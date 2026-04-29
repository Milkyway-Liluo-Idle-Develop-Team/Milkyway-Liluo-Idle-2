// Package session manages PlayerSession instances, one per WebSocket
// connection. Each session holds the per-player game state (attribute
// instance, inventory, etc.) and provides the recorder lifecycle for
// execution cycles.
package session

import (
	"log/slog"
	"sync"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/google/uuid"
)

// PlayerSession is the in-memory game state for one connected player.
// Created on WebSocket connect, destroyed on disconnect.
type PlayerSession struct {
	ID     uuid.UUID // equals wsx.Conn.ID
	UserID int64
	Attr   *attribute.Instance

	logger *slog.Logger

	mu       sync.Mutex
	recorder *record.Recorder
}

// New creates a PlayerSession. The attribute instance is constructed bare
// (with static modifiers from the registry); player-specific modifiers
// (equipment, skills, buffs) are loaded and applied by the caller after
// creation once the corresponding systems are available.
func New(connID uuid.UUID, userID int64, logger *slog.Logger) *PlayerSession {
	return &PlayerSession{
		ID:     connID,
		UserID: userID,
		Attr:   attribute.NewInstance(),
		logger: logger,
	}
}

// SetRecorder attaches a Recorder for the current execution cycle.
// Subsequent markDirty / inventory change / skill XP writes will flow
// into this recorder.
func (s *PlayerSession) SetRecorder(rec *record.Recorder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorder = rec
	s.Attr.SetRecorder(rec)
}

// ClearRecorder detaches and returns the current Recorder, if any.
func (s *PlayerSession) ClearRecorder() *record.Recorder {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attr.ClearRecorder()
	rec := s.recorder
	s.recorder = nil
	return rec
}

// Recorder returns the current recorder without detaching it.
func (s *PlayerSession) Recorder() *record.Recorder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recorder
}

// Manager is the thread-safe registry of all active PlayerSessions,
// keyed by WebSocket connection ID.
type Manager struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]*PlayerSession
	reg      *record.Registry
}

// NewManager creates a Manager backed by the given record Registry,
// which is used when creating Recorders for execution cycles.
func NewManager(reg *record.Registry) *Manager {
	return &Manager{
		sessions: make(map[uuid.UUID]*PlayerSession),
		reg:      reg,
	}
}

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

// Get returns the PlayerSession for the given connection ID.
func (m *Manager) Get(id uuid.UUID) (*PlayerSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// GetByUser returns all sessions belonging to a user. A user may have
// multiple connections (e.g. browser + mobile).
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
// Convenience for execution cycles that need a fresh recorder.
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
