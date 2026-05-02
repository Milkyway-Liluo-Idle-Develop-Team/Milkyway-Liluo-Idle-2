// Package session manages PlayerSession instances, one per user.
// A session survives WebSocket disconnect for a configurable grace period
// and is destroyed only on grace expiry or explicit eviction.
// Each session holds the per-player game state and provides the recorder
// lifecycle for execution cycles.
//
// Concurrency: game-state writes are handled exclusively by the session's
// RunLoop goroutine via the command channel. External reads may use
// Manager.RLockSession / RUnlockSession.
package session

import (
	"context"
	"log/slog"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/apperror"
	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/bestiary"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/equipment"
	"github.com/edrowsluo/new-mli/backend/internal/event"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/inventory"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/skill"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
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
	eq     *equipment.State
	battle *Instance

	logger *slog.Logger

	mu       sync.RWMutex
	recorder *record.Recorder

	conn   *wsx.Conn
	connMu sync.RWMutex

	commandCh chan command

	done chan struct{} // closed on session close; kept for Close() compat

	state         sessionState
	graceTimer    *time.Timer
	graceMu       sync.Mutex
	onGraceExpire func()

	lastTick     time.Time
	elapsedAccum float64
}

// New creates a PlayerSession. The attribute instance is constructed bare;
// subsystems (inventory, etc.) are attached later via setters once loaded
// from the database.
func New(connID uuid.UUID, userID int64, logger *slog.Logger) *PlayerSession {
	return &PlayerSession{
		ID:        connID,
		UserID:    userID,
		attr:      attribute.NewInstance(),
		eq:        equipment.NewState(userID),
		logger:    logger,
		commandCh: make(chan command, 64),
		done:      make(chan struct{}),
		state:     StateActive,
	}
}

// Close shuts down the push loop and releases resources. Idempotent.
func (s *PlayerSession) Close() {
	s.graceMu.Lock()
	defer s.graceMu.Unlock()
	if s.graceTimer != nil {
		s.graceTimer.Stop()
		s.graceTimer = nil
	}
	s.setState(StateClosed)
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

// ---- conn attach / detach ----

func (s *PlayerSession) AttachConn(c *wsx.Conn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if old := s.conn; old != nil && old.ID != c.ID {
		old.Close()
	}
	s.conn = c
}

func (s *PlayerSession) DetachConn() {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.conn = nil
}

func (s *PlayerSession) Conn() *wsx.Conn {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.conn
}

func (s *PlayerSession) HasConn() bool {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.conn != nil
}

// ---- lock (unexported — only Manager touches these) ----

func (s *PlayerSession) lock()   { s.mu.Lock() }
func (s *PlayerSession) unlock() { s.mu.Unlock() }

func (s *PlayerSession) RLock()   { s.mu.RLock() }
func (s *PlayerSession) RUnlock() { s.mu.RUnlock() }

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

// Equipment returns the equipment state, or nil if not yet loaded.
func (s *PlayerSession) Equipment() *equipment.State { return s.eq }

// Battle returns the battle instance, or nil if not yet attached.
func (s *PlayerSession) Battle() *Instance { return s.battle }

// SetBattle attaches a battle instance.
func (s *PlayerSession) SetBattle(b *Instance) { s.battle = b }

// SetEquipment attaches an equipment state (called after DB load).
func (s *PlayerSession) SetEquipment(st *equipment.State) {
	s.eq = st
	// Replay equipment modifiers on the attribute system. equipment.Load is
	// pure DB → State; attribute coupling is the session's job.
	for _, it := range st.All() {
		def, ok := gameconfig.GetItemDefByID(it.ID)
		if !ok {
			continue
		}
		mods, err := def.Modifiers(it, attribute.Get())
		if err != nil {
			// Best-effort: log and skip broken definitions.
			s.logger.Warn("equipment modifier replay failed", "item_id", it.ID, "err", err)
			continue
		}
		s.attr.AddModifiers("equipment:"+def.StringID(), mods)
	}
}

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
	if s.eq != nil {
		s.eq.SetRecorder(rec)
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
	if s.eq != nil {
		s.eq.ClearRecorder()
	}
	rec := s.recorder
	s.recorder = nil
	return rec
}

// FlushAll persists every dirty subsystem inside a single SQLite transaction.
func (s *PlayerSession) FlushAll(ctx context.Context, database *db.DB) error {
	return database.InTx(ctx, func(q *dbgen.Queries) error {
		if s.inv != nil {
			if err := s.inv.Flush(ctx, q); err != nil {
				return err
			}
		}
		if s.skill != nil {
			if err := s.skill.Flush(ctx, q); err != nil {
				return err
			}
		}
		if s.best != nil {
			if err := s.best.Flush(ctx, q); err != nil {
				return err
			}
		}
		if s.ev != nil {
			if err := s.ev.Flush(ctx, q); err != nil {
				return err
			}
		}
		if s.eq != nil {
			if err := s.eq.Flush(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}

// Equip moves an item from inventory to the given slot, applying attribute modifiers.
// If the slot is occupied, the old item is unequipped first.
func (s *PlayerSession) Equip(ctx context.Context, it item.Item, slot string) error {
	def, ok := gameconfig.GetItemDefByID(it.ID)
	if !ok {
		return apperror.NotFound("item not found")
	}
	if !def.IsEquipment() {
		return apperror.BadRequest("item is not equipment")
	}
	if !s.inv.Has(it, 1) {
		return apperror.BadRequest("item not in inventory")
	}

	// Replace existing item in slot, if any.
	if old, ok := s.eq.Get(slot); ok {
		if oldDef, ok := gameconfig.GetItemDefByID(old.ID); ok {
			s.attr.RemoveModifiers("equipment:" + oldDef.StringID())
		}
		s.inv.AddEquipChange(old, 1, false) // returns to inventory, reason=UNEQUIP
		s.eq.Unequip(slot)                  // clears + records UNEQUIP
	}

	// Deduct from inventory with EQUIP reason.
	s.inv.AddEquipChange(it, -1, true)

	// Apply attribute modifiers.
	mods, err := def.Modifiers(it, attribute.Get())
	if err != nil {
		return err
	}
	s.attr.AddModifiers("equipment:"+def.StringID(), mods)

	// Record slot mount.
	s.eq.Equip(slot, it)
	return nil
}

// Unequip removes the item from the given slot and returns it to inventory.
func (s *PlayerSession) Unequip(ctx context.Context, slot string) error {
	it, ok := s.eq.Get(slot)
	if !ok {
		return apperror.NotFound("no item equipped in slot")
	}
	if def, ok := gameconfig.GetItemDefByID(it.ID); ok {
		s.attr.RemoveModifiers("equipment:" + def.StringID())
	}
	s.inv.AddEquipChange(it, 1, false)
	s.eq.Unequip(slot)
	return nil
}

func isStateDiffEmpty(d *pb.StateDiff) bool {
	return len(d.Inventory) == 0 &&
		len(d.Attribute) == 0 &&
		len(d.SkillXp) == 0 &&
		len(d.Bestiary) == 0 &&
		len(d.EventExecution) == 0 &&
		len(d.EventQueue) == 0 &&
		len(d.Equipment) == 0
}

// Manager is the thread-safe registry of all active PlayerSessions,
// keyed by user ID. It owns all session locking.
type Manager struct {
	mu          sync.RWMutex
	sessions    map[int64]*PlayerSession
	reg         *record.Registry
	database    *db.DB
	workerCount int
}

// NewManager creates a Manager backed by the given record Registry.
// It auto-starts the global TickAll goroutine if tickInterval > 0.
// database is used for flush and may be nil in tests.
func NewManager(ctx context.Context, reg *record.Registry, database *db.DB, tickInterval time.Duration) *Manager {
	m := &Manager{
		sessions:    make(map[int64]*PlayerSession),
		reg:         reg,
		database:    database,
		workerCount: runtime.NumCPU(),
	}
	if tickInterval > 0 {
		go m.TickAll(ctx, database, tickInterval)
	}
	return m
}

// NewManagerWithoutTick is a convenience constructor for tests and callers
// that do not need the global tick loop.
func NewManagerWithoutTick(reg *record.Registry, database *db.DB) *Manager {
	return &Manager{
		sessions:    make(map[int64]*PlayerSession),
		reg:         reg,
		database:    database,
		workerCount: runtime.NumCPU(),
	}
}

// LockSession returns the session for userID, already locked for exclusive
// access. Call UnlockSession after the operation.
func (m *Manager) LockSession(userID int64) (*PlayerSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[userID]
	if !ok {
		return nil, false
	}
	s.lock()
	return s, true
}

// UnlockSession releases the lock acquired by LockSession.
func (m *Manager) UnlockSession(s *PlayerSession) { s.unlock() }

// RLockSession returns the session for userID, already read-locked.
// Call RUnlockSession after the operation.
func (m *Manager) RLockSession(userID int64) (*PlayerSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[userID]
	if !ok {
		return nil, false
	}
	s.RLock()
	return s, true
}

// RUnlockSession releases the read lock acquired by RLockSession.
func (m *Manager) RUnlockSession(s *PlayerSession) { s.RUnlock() }

// Add registers a new PlayerSession. Called on WebSocket connect.
// If a session already exists for the same user, the old one is closed
// and replaced (single-online-per-user).
func (m *Manager) Add(s *PlayerSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.sessions[s.UserID]; ok {
		old.Close()
	}
	m.sessions[s.UserID] = s
}

// Remove unregisters a PlayerSession. Called on WebSocket disconnect.
func (m *Manager) Remove(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, userID)
}

// Get returns the session without locking. Prefer LockSession.
func (m *Manager) Get(userID int64) (*PlayerSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[userID]
	return s, ok
}

// GetByUser returns the session for a user.
func (m *Manager) GetByUser(userID int64) (*PlayerSession, bool) {
	return m.Get(userID)
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

// CommandHandler is a WS message handler that submits a command to the session's RunLoop.
type CommandHandler func(ctx context.Context, c *wsx.Conn, sess *PlayerSession, in wsx.Inbound) error

// HandleCommand registers a WS message type that is processed as a session command.
func (m *Manager) HandleCommand(hub *wsx.Hub, typ string, fn CommandHandler) {
	hub.Handle(typ, func(ctx context.Context, c *wsx.Conn, in wsx.Inbound) error {
		s, ok := m.Get(c.UserID)
		if !ok {
			c.ReplyError(in, apperror.NotFound("session not found"))
			return nil
		}
		return s.SubmitCommand(func(sess *PlayerSession) error {
			return fn(ctx, c, sess, in)
		})
	})
}

// TypedCommandHandler is a CommandHandler that receives a pre-decoded payload.
type TypedCommandHandler[T proto.Message] func(ctx context.Context, c *wsx.Conn, sess *PlayerSession, req T) error

// HandleCommandTyped registers a WS message type that is processed as a session command
// with a typed payload. The payload is automatically decoded and validated before fn is called.
func HandleCommandTyped[T proto.Message](m *Manager, hub *wsx.Hub, typ string, fn TypedCommandHandler[T]) {
	hub.Handle(typ, func(ctx context.Context, c *wsx.Conn, in wsx.Inbound) error {
		var req T
		req = reflect.New(reflect.TypeOf(req).Elem()).Interface().(T)
		if err := in.DecodePayload(req); err != nil {
			return err
		}
		s, ok := m.Get(c.UserID)
		if !ok {
			c.ReplyError(in, apperror.NotFound("session not found"))
			return nil
		}
		return s.SubmitCommand(func(sess *PlayerSession) error {
			return fn(ctx, c, sess, req)
		})
	})
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
	eqSt, err := equipment.Load(ctx, database.Queries, userID)
	if err != nil {
		return nil, err
	}
	sess.SetEquipment(eqSt)
	return sess, nil
}
