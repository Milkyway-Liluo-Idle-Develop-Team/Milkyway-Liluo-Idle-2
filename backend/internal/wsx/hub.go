package wsx

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/apperror"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/config"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/logging"
	"github.com/google/uuid"
)

// Handler handles a single inbound message. Returning an error causes the
// framework to send a typed error reply (using in.ID/in.Type) and log the
// cause.
type Handler func(ctx context.Context, c *Conn, in Inbound) error

// Hub owns all live connections and routes messages by type. Construct one
// per server (the framework expects a single hub).
type Hub struct {
	cfg config.WS
	log func(ctx context.Context) any // optional, currently unused

	handlersMu sync.RWMutex
	handlers   map[string]Handler

	mu         sync.RWMutex
	conns      map[uuid.UUID]*Conn
	byUserID   map[int64]map[uuid.UUID]*Conn
	closed     bool
	allDone    sync.WaitGroup
}

// NewHub constructs a Hub.
func NewHub(cfg config.WS) *Hub {
	return &Hub{
		cfg:      cfg,
		handlers: map[string]Handler{},
		conns:    map[uuid.UUID]*Conn{},
		byUserID: map[int64]map[uuid.UUID]*Conn{},
	}
}

// Handle registers a handler for the given message type. Last write wins.
// Register all handlers at startup before serving connections.
func (h *Hub) Handle(typ string, fn Handler) {
	h.handlersMu.Lock()
	defer h.handlersMu.Unlock()
	h.handlers[typ] = fn
}

// ServeOptions tweak how a single connection is upgraded and registered.
type ServeOptions struct {
	// UserID > 0 means the connection is authenticated as that user.
	UserID int64
	// OriginPatterns passed to websocket.Accept. nil means same-origin.
	OriginPatterns []string
	// OnConnect/OnDisconnect run synchronously around the connection's life
	// without blocking the read loop. Use them to publish presence events.
	OnConnect    func(c *Conn)
	OnDisconnect func(c *Conn)
}

// Serve upgrades the HTTP request to a WebSocket and runs the connection
// until it closes. Blocks until the connection ends. Auth must be checked
// by the caller before invoking Serve; pass UserID via ServeOptions.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, opts ServeOptions) error {
	acceptOpts := &websocket.AcceptOptions{
		OriginPatterns:     opts.OriginPatterns,
		InsecureSkipVerify: len(opts.OriginPatterns) == 0,
	}
	ws, err := websocket.Accept(w, r, acceptOpts)
	if err != nil {
		return err
	}

	c := &Conn{
		ID:        uuid.New(),
		UserID:    opts.UserID,
		hub:       h,
		ws:        ws,
		send:      make(chan Outbound, h.cfg.SendBuffer),
		done:      make(chan struct{}),
		jsonCodec: h.cfg.Codec == "json",
	}

	if !h.register(c) {
		_ = ws.Close(websocket.StatusGoingAway, "server shutting down")
		return errors.New("hub closed")
	}

	if opts.OnConnect != nil {
		opts.OnConnect(c)
	}

	// Use a background-derived ctx so the lifetime of the connection is
	// independent of the HTTP request handler. The hub cancels these via
	// Close().
	ctx, cancel := context.WithCancel(context.Background())
	// Bridge the request logger into the conn lifetime.
	ctx = logging.WithLogger(ctx, logging.FromContext(r.Context()))

	h.allDone.Add(1)
	defer h.allDone.Done()
	defer cancel()
	defer h.unregister(c)
	defer func() {
		if opts.OnDisconnect != nil {
			opts.OnDisconnect(c)
		}
	}()

	c.run(ctx)
	return nil
}

func (h *Hub) register(c *Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	h.conns[c.ID] = c
	if c.UserID != 0 {
		set, ok := h.byUserID[c.UserID]
		if !ok {
			set = map[uuid.UUID]*Conn{}
			h.byUserID[c.UserID] = set
		}
		set[c.ID] = c
	}
	return true
}

func (h *Hub) unregister(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, c.ID)
	if c.UserID != 0 {
		if set, ok := h.byUserID[c.UserID]; ok {
			delete(set, c.ID)
			if len(set) == 0 {
				delete(h.byUserID, c.UserID)
			}
		}
	}
}

// Broadcast sends msg to every live connection. Returns the number of
// connections it was queued to (some may still drop on slow consumers).
func (h *Hub) Broadcast(msg Outbound) int {
	h.mu.RLock()
	conns := make([]*Conn, 0, len(h.conns))
	for _, c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	n := 0
	for _, c := range conns {
		if c.Send(msg) {
			n++
		}
	}
	return n
}

// SendToUser delivers msg to every connection owned by userID. Returns the
// number of connections it was queued to.
func (h *Hub) SendToUser(userID int64, msg Outbound) int {
	h.mu.RLock()
	set := h.byUserID[userID]
	conns := make([]*Conn, 0, len(set))
	for _, c := range set {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	n := 0
	for _, c := range conns {
		if c.Send(msg) {
			n++
		}
	}
	return n
}

// SendToConn delivers msg to a specific connection by id.
func (h *Hub) SendToConn(connID uuid.UUID, msg Outbound) bool {
	h.mu.RLock()
	c, ok := h.conns[connID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	return c.Send(msg)
}

// CountConns returns the number of currently live connections.
func (h *Hub) CountConns() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// Close stops accepting new connections and closes existing ones. It blocks
// until every connection's goroutines have exited or shutdownTimeout elapses.
func (h *Hub) Close(shutdownTimeout time.Duration) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	conns := make([]*Conn, 0, len(h.conns))
	for _, c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.Unlock()

	for _, c := range conns {
		c.closeWithReason("server shutting down")
	}

	done := make(chan struct{})
	go func() {
		h.allDone.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownTimeout):
	}
}

func (h *Hub) dispatch(ctx context.Context, c *Conn, in Inbound) {
	start := time.Now()

	h.handlersMu.RLock()
	fn, ok := h.handlers[in.Type]
	h.handlersMu.RUnlock()
	if !ok {
		c.ReplyError(in, apperror.NotFound("unknown message type").WithField("type", in.Type))
		return
	}
	if err := fn(ctx, c, in); err != nil {
		c.ReplyError(in, err)
		if ae, ok := apperror.As(err); ok && ae.Code == apperror.CodeInternal {
			logging.FromContext(ctx).Error("ws handler error",
				"type", in.Type, "conn", c.ID, "err", err)
		}
	}

	dur := time.Since(start)
	if dur > 200*time.Millisecond {
		logging.FromContext(ctx).Warn("ws handler slow",
			"type", in.Type, "conn", c.ID, "dur_ms", dur.Milliseconds())
	} else {
		logging.FromContext(ctx).Debug("ws handler",
			"type", in.Type, "conn", c.ID, "dur_ms", dur.Milliseconds())
	}
}
