// Package session manages PlayerSession instances, one per WebSocket
// connection. Each session holds the per-player game state and provides
// the recorder lifecycle for execution cycles.
//
// Concurrency: game-state access is gated by Manager.LockSession /
// Manager.UnlockSession. The session's mutex is not exposed — the Manager
// is the sole choke point for locking.
package session

import (
	"log/slog"
	"sync"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/inventory"
	"github.com/edrowsluo/new-mli/backend/internal/record"
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

	logger *slog.Logger

	mu       sync.Mutex
	recorder *record.Recorder
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

// SetRecorder attaches a Recorder for the current execution cycle.
func (s *PlayerSession) SetRecorder(rec *record.Recorder) {
	s.recorder = rec
	s.attr.SetRecorder(rec)
	if s.inv != nil {
		s.inv.SetRecorder(rec)
	}
}

// ClearRecorder detaches and returns the current Recorder, if any.
func (s *PlayerSession) ClearRecorder() *record.Recorder {
	s.attr.ClearRecorder()
	if s.inv != nil {
		s.inv.ClearRecorder()
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
