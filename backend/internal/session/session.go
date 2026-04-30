// Package session manages PlayerSession instances, one per WebSocket
// connection. Each session holds the per-player game state and provides
// the recorder lifecycle for execution cycles.
//
// Concurrency: game-state access is gated by Manager.LockSession /
// Manager.UnlockSession. The session's mutex is not exposed — the Manager
// is the sole choke point for locking.
package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/apperror"
	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/bestiary"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	"github.com/edrowsluo/new-mli/backend/internal/event"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/inventory"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/skill"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
	"github.com/google/uuid"
)

// PlayerSession is the in-memory game state for one connected player.
// Created on WebSocket connect, destroyed on disconnect.
//
// All game-state access must happen between Lock / Unlock, obtained via
// Manager.LockSession.
type PlayerSession struct {
	ID     uuid.UUID
	UserID int64
	attr   *attribute.Instance
	inv    *inventory.State
	skill  *skill.State
	best   *bestiary.State
	ev     *event.State

	logger *slog.Logger

	mu       sync.Mutex
	recorder *record.Recorder

	done chan struct{} // closed on session close
}

// New creates a PlayerSession. The attribute instance is constructed bare;
// subsystems (inventory, etc.) are attached later via setters once loaded
// from the database.
func New(connID uuid.UUID, userID int64, logger *slog.Logger) *PlayerSession {
	return &PlayerSession{
		ID:     connID,
		UserID: userID,
		attr:   attribute.NewInstance(),
		logger: logger,
		done:   make(chan struct{}),
	}
}

// Close shuts down the push loop and releases resources. Idempotent.
func (s *PlayerSession) Close() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

// ---- lock (unexported — only Manager touches these) ----

func (s *PlayerSession) lock()   { s.mu.Lock() }
func (s *PlayerSession) unlock() { s.mu.Unlock() }

// ---- game state (caller must hold lock) ----

// Attr returns the attribute instance.
func (s *PlayerSession) Attr() *attribute.Instance { return s.attr }

// Inv returns the inventory state, or nil if not yet loaded.
func (s *PlayerSession) Inv() *inventory.State { return s.inv }

// SetInv attaches an inventory state (called after DB load).
func (s *PlayerSession) SetInv(st *inventory.State) { s.inv = st }

// Skill returns the skill state, or nil if not yet loaded.
func (s *PlayerSession) Skill() *skill.State { return s.skill }

// SetSkill attaches a skill state (called after DB load).
func (s *PlayerSession) SetSkill(st *skill.State) { s.skill = st }

// Bestiary returns the bestiary state.
func (s *PlayerSession) Bestiary() *bestiary.State { return s.best }

// SetBestiary attaches a bestiary state.
func (s *PlayerSession) SetBestiary(st *bestiary.State) { s.best = st }

// Events returns the event queue state.
func (s *PlayerSession) Events() *event.State { return s.ev }

// SetEvents attaches an event state.
func (s *PlayerSession) SetEvents(st *event.State) { s.ev = st }

// --- SettlementCtx (implements event.SettlementCtx) ---

func (s *PlayerSession) HasItem(it item.Item, qty float64) bool    { return s.inv.Has(it, qty) }
func (s *PlayerSession) GetItemQty(it item.Item) float64           { return s.inv.Get(it) }
func (s *PlayerSession) AddItem(it item.Item, qty float64)         { s.inv.Add(it, qty) }
func (s *PlayerSession) DeductItem(it item.Item, qty float64)      { s.inv.Deduct(it, qty) }
func (s *PlayerSession) AddXP(sid gameconfig.SkillID, xp float64)  { s.skill.AddXP(sid, xp) }
func (s *PlayerSession) GetAttr(id attribute.AttributeID) float64  { return s.attr.GetFinal(id) }
func (s *PlayerSession) GetSkillLevel(sid gameconfig.SkillID) float64 {
	lvl, _ := s.skill.Get(sid)
	return lvl
}
func (s *PlayerSession) UnlockEvent(id gameconfig.EventID)  { s.best.UnlockEvent(id) }
func (s *PlayerSession) IsEventUnlocked(id gameconfig.EventID) bool { return s.best.HasEvent(id) }

// SetRecorder attaches a Recorder for the current execution cycle.
func (s *PlayerSession) SetRecorder(rec *record.Recorder) {
	s.recorder = rec
	s.attr.SetRecorder(rec)
	if s.inv != nil {
		s.inv.SetRecorder(rec)
	}
	if s.skill != nil {
		s.skill.SetRecorder(rec)
	}
	if s.best != nil {
		s.best.SetRecorder(rec)
	}
	if s.ev != nil {
		s.ev.SetRecorder(rec)
	}
}

// ClearRecorder detaches and returns the current Recorder, if any.
func (s *PlayerSession) ClearRecorder() *record.Recorder {
	s.attr.ClearRecorder()
	if s.inv != nil {
		s.inv.ClearRecorder()
	}
	if s.skill != nil {
		s.skill.ClearRecorder()
	}
	if s.best != nil {
		s.best.ClearRecorder()
	}
	if s.ev != nil {
		s.ev.ClearRecorder()
	}
	rec := s.recorder
	s.recorder = nil
	return rec
}

// Manager is the thread-safe registry of all active PlayerSessions,
// keyed by WebSocket connection ID. It owns all session locking.
type Manager struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]*PlayerSession
	reg      *record.Registry
}

// NewManager creates a Manager backed by the given record Registry.
func NewManager(reg *record.Registry) *Manager {
	return &Manager{
		sessions: make(map[uuid.UUID]*PlayerSession),
		reg:      reg,
	}
}

// LockSession returns the session for connID, already locked for exclusive
// access. Call UnlockSession after the operation.
func (m *Manager) LockSession(id uuid.UUID) (*PlayerSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, false
	}
	s.lock()
	return s, true
}

// UnlockSession releases the lock acquired by LockSession.
func (m *Manager) UnlockSession(s *PlayerSession) { s.unlock() }

// Add registers a new PlayerSession. Called on WebSocket connect.
func (m *Manager) Add(s *PlayerSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
}

// Remove unregisters a PlayerSession. Called on WebSocket disconnect.
func (m *Manager) Remove(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

// Get returns the session without locking. Prefer LockSession.
func (m *Manager) Get(id uuid.UUID) (*PlayerSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// GetByUser returns all sessions belonging to a user.
func (m *Manager) GetByUser(userID int64) []*PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*PlayerSession
	for _, s := range m.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out
}

// NewRecorder creates a Recorder backed by this manager's Registry.
func (m *Manager) NewRecorder() *record.Recorder {
	return record.NewRecorder(m.reg)
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Registry returns the record Registry owned by this Manager.
func (m *Manager) Registry() *record.Registry {
	return m.reg
}

// SessionHandler is a WS message handler that receives a pre-locked session.
type SessionHandler func(ctx context.Context, c *wsx.Conn, sess *PlayerSession, in wsx.Inbound) error

// HandleSession registers a WS message type that requires a locked session.
// The session is locked before fn is called and unlocked afterwards.
func (m *Manager) HandleSession(hub *wsx.Hub, typ string, fn SessionHandler) {
	hub.Handle(typ, func(ctx context.Context, c *wsx.Conn, in wsx.Inbound) error {
		s, ok := m.LockSession(c.ID)
		if !ok {
			c.ReplyError(in, apperror.NotFound("session not found"))
			return nil
		}
		defer m.UnlockSession(s)
		return fn(ctx, c, s, in)
	})
}

// StartLoop launches the push goroutine for a session. It ticks on a
// fixed interval (placeholder until the event system provides smart
// prediction), builds a diff packet, and pushes it to the client.
// Exits when the session is closed.
func (m *Manager) StartLoop(sess *PlayerSession, conn *wsx.Conn) {
	go func() {
		const tick = 1 * time.Second
		ticker := time.NewTicker(tick)
		defer ticker.Stop()
		lastTick := time.Now()

		for {
			select {
			case <-sess.done:
				return
			case now := <-ticker.C:
				elapsed := now.Sub(lastTick).Seconds()
				lastTick = now

				s, ok := m.LockSession(sess.ID)
				if !ok {
					return
				}

				func() {
					defer m.UnlockSession(s)

					rec := m.NewRecorder()
					s.SetRecorder(rec)
					rec.PushNamespace("action_queue")
					s.Events().Settle(s, elapsed)
					rec.PopNamespace()
					s.ClearRecorder()

					diff, err := m.reg.BuildDiff(rec)
					if err != nil {
						conn.Send(wsx.Outbound{Type: "error", Error: apperror.Internal("build diff").WithCause(err)})
						return
					}
					if string(diff) != "{}" {
						conn.Send(wsx.Outbound{Type: "state.diff", Payload: diff})
					}
				}()
			}
		}
	}()
}

// CreateSession builds a fully-loaded PlayerSession from the database.
// All subsystems (inventory, skill, bestiary) are loaded and attached.
// The session is not added to the Manager — caller must call Add.
func (m *Manager) CreateSession(ctx context.Context, connID uuid.UUID, userID int64, database *db.DB, logger *slog.Logger) (*PlayerSession, error) {
	q := database.Queries

	// Inventory.
	invSt, err := inventory.Load(ctx, q, userID)
	if err != nil {
		return nil, err
	}

	// Skills.
	curve, err := skill.LoadCurve()
	if err != nil {
		return nil, err
	}
	skillSt, err := skill.Load(ctx, q, userID, curve)
	if err != nil {
		return nil, err
	}

	// Bestiary from unlocked events.
	best := bestiary.New(userID)
	eventRows, err := q.LoadUnlockedEvents(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]gameconfig.EventID, len(eventRows))
	for i, r := range eventRows {
		ids[i] = gameconfig.EventID(r.EventID)
	}
	best.LoadEvents(ids)

	// Active event queues.
	evSt, err := event.Load(ctx, q, userID)
	if err != nil {
		return nil, err
	}

	sess := New(connID, userID, logger)
	sess.SetInv(invSt)
	sess.SetSkill(skillSt)
	sess.SetBestiary(best)
	sess.SetEvents(evSt)
	return sess, nil
}
