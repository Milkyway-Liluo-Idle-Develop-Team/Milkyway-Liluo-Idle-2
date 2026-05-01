package wsx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/edrowsluo/new-mli/backend/internal/apperror"
	"github.com/edrowsluo/new-mli/backend/internal/logging"
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Conn is a single client WebSocket connection managed by a Hub. It is
// safe to call Send/Reply/Close from any goroutine. Reads happen on the
// connection's read loop only.
//
// UserID == 0 means anonymous.
type Conn struct {
	ID     uuid.UUID
	UserID int64

	hub  *Hub
	ws   *websocket.Conn
	send chan Outbound
	done chan struct{} // closed by Close to signal the writer to stop

	jsonCodec bool // use protojson+text frames instead of proto+binary
	closeOnce sync.Once
}

// Send queues msg for delivery. Returns true if queued, false if the
// connection is closed or the send buffer is full (slow consumer). When
// false on a "buffer full" path, the framework will close the connection
// shortly to avoid memory growth.
func (c *Conn) Send(msg Outbound) bool {
	// Fast pre-check; the second select is the authoritative one.
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- msg:
		return true
	case <-c.done:
		return false
	default:
		c.closeWithReason("send buffer full")
		return false
	}
}

// Reply sends a typed response to an inbound request, reusing its ID.
// The response type is in.Type + ".ok"; use Send directly for custom types.
func (c *Conn) Reply(in Inbound, payload proto.Message) bool {
	return c.Send(Outbound{
		ID:      in.ID,
		Type:    in.Type + ".ok",
		Payload: payload,
	})
}

// ReplyError sends an error response that matches the inbound request id.
// Non-AppError values are wrapped as internal errors.
func (c *Conn) ReplyError(in Inbound, err error) bool {
	ae, ok := apperror.As(err)
	if !ok {
		ae = apperror.Internal("internal error").WithCause(err)
	}
	return c.Send(Outbound{
		ID:    in.ID,
		Type:  in.Type + ".err",
		Error: ae,
	})
}

// Close terminates the connection. Idempotent and safe to call from any
// goroutine.
func (c *Conn) Close() {
	c.closeWithReason("closed by server")
}

func (c *Conn) closeWithReason(reason string) {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.ws.Close(websocket.StatusNormalClosure, reason)
	})
}

func convertFields(fields map[string]any) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for k, v := range fields {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// run owns the read/write goroutines for the lifetime of the connection.
// It returns when both loops have exited.
func (c *Conn) run(ctx context.Context) {
	writerDone := make(chan struct{})
	go c.writeLoop(ctx, writerDone)
	c.readLoop(ctx)
	c.closeWithReason("read ended") // make sure the writer wakes up
	<-writerDone
}

func (c *Conn) writeLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	cfg := c.hub.cfg
	pingTicker := time.NewTicker(cfg.PingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			pingCtx, cancel := context.WithTimeout(ctx, cfg.WriteTimeout)
			err := c.ws.Ping(pingCtx)
			cancel()
			if err != nil {
				c.closeWithReason("ping failed")
				return
			}
		case msg := <-c.send:
			var data []byte
			var err error
			msgType := websocket.MessageBinary

			if c.jsonCodec {
				// Build JSON-friendly envelope with inline payload.
				envMap := map[string]any{
					"id":   msg.ID,
					"type": msg.Type,
				}
				if msg.Error != nil {
					envMap["error"] = map[string]any{
						"code":    string(msg.Error.Code),
						"message": msg.Error.Message,
						"fields":  convertFields(msg.Error.Fields),
					}
				} else if msg.Payload != nil {
					payloadJSON, jerr := protojson.Marshal(msg.Payload)
					if jerr != nil {
						logging.FromContext(ctx).Error("ws marshal", "err", jerr, "type", msg.Type)
						continue
					}
					envMap["payload"] = json.RawMessage(payloadJSON)
				}
				data, err = json.Marshal(envMap)
				msgType = websocket.MessageText
			} else {
				env := &pb.Envelope{
					Id:   msg.ID,
					Type: msg.Type,
				}
				if msg.Error != nil {
					env.Error = &pb.Error{
						Code:    string(msg.Error.Code),
						Message: msg.Error.Message,
						Fields:  convertFields(msg.Error.Fields),
					}
				} else if msg.Payload != nil {
					var payload []byte
					payload, err = proto.Marshal(msg.Payload)
					if err != nil {
						logging.FromContext(ctx).Error("ws marshal", "err", err, "type", msg.Type)
						continue
					}
					env.Payload = payload
				}
				data, err = proto.Marshal(env)
			}
			if err != nil {
				logging.FromContext(ctx).Error("ws envelope marshal", "err", err, "type", msg.Type)
				continue
			}
			wctx, cancel := context.WithTimeout(ctx, cfg.WriteTimeout)
			err = c.ws.Write(wctx, msgType, data)
			cancel()
			if err != nil {
				c.closeWithReason("write failed")
				return
			}
		}
	}
}

func (c *Conn) readLoop(ctx context.Context) {
	c.ws.SetReadLimit(c.hub.cfg.MaxMessageSize)
	for {
		typ, data, err := c.ws.Read(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logging.FromContext(ctx).Debug("ws read end", "conn", c.ID, "err", err)
			}
			return
		}
		var env pb.Envelope
		if c.jsonCodec {
			if typ != websocket.MessageText {
				c.closeWithReason("unexpected binary frame")
				return
			}
			var raw struct {
				ID      string          `json:"id"`
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				c.Send(Outbound{
					Type:  "error",
					Error: apperror.BadRequest("invalid JSON message").WithCause(err),
				})
				continue
			}
			env.Id = raw.ID
			env.Type = raw.Type
			env.Payload = raw.Payload
		} else {
			if typ != websocket.MessageBinary {
				c.closeWithReason("unexpected text frame")
				return
			}
			if err := proto.Unmarshal(data, &env); err != nil {
				c.Send(Outbound{
					Type:  "error",
					Error: apperror.BadRequest("invalid protobuf message").WithCause(err),
				})
				continue
			}
		}
		in := Inbound{
			ID:      env.Id,
			Type:    env.Type,
			Payload: env.Payload,
		}
		c.hub.dispatch(ctx, c, in)
	}
}
