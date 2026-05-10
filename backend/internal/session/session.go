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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/apperror"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/bestiary"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db"
	dbgen "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/equipment"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/event"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/inventory"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/playerinit"
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/skill"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/wsx"
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
	battleSession *battle.BattleSession

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

// SetLastTick overrides lastTick for deterministic tests.
func (s *PlayerSession) SetLastTick(t time.Time) {
	s.lastTick = t
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
		lastTick:  time.Now(),
	}
}

// GraceExpireNow synchronously runs the full grace-expiry flow:
// StateClosing → FlushAll → StateClosed. Used by tests to simulate
// a natural session shutdown without waiting for the grace timer.
func (s *PlayerSession) GraceExpireNow(ctx context.Context, database *db.DB) error {
	s.graceMu.Lock()
	if s.graceTimer != nil {
		s.graceTimer.Stop()
		s.graceTimer = nil
	}
	s.graceMu.Unlock()

	s.setState(StateClosing)
	if err := s.FlushAll(ctx, database); err != nil {
		return err
	}
	s.Close()
	return nil
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
	// Fail any pending commands so their goroutines don't leak.
	for {
		select {
		case cmd := <-s.commandCh:
			cmd.resp <- apperror.Unavailable("session closed")
		default:
			return
		}
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

// ---- lock (unexported —only Manager touches these) ----

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

// BattleSession returns the active battle session, or nil if not in battle.
func (s *PlayerSession) BattleSession() *battle.BattleSession { return s.battleSession }

// SetBattleSession attaches a battle session.
func (s *PlayerSession) SetBattleSession(b *battle.BattleSession) { s.battleSession = b }

// SetEquipment attaches an equipment state (called after DB load).
func (s *PlayerSession) SetEquipment(st *equipment.State) {
	s.eq = st
	// Replay equipment modifiers on the attribute system. equipment.Load is
	// pure DB →State; attribute coupling is the session's job.
	seen := make(map[string]struct{})
	for _, entry := range st.All() {
		def, ok := gameconfig.GetItemDefByID(entry.Item.ID)
		if !ok {
			continue
		}
		// Deduplicate by anchor to avoid double-counting multi-slot pieces.
		key := entry.AnchorSlot + ":" + def.StringID()
		if _, done := seen[key]; done {
			continue
		}
		seen[key] = struct{}{}
		mods, err := def.Modifiers(entry.Item, attribute.Get())
		if err != nil {
			// Best-effort: log and skip broken definitions.
			s.logger.Warn("equipment modifier replay failed", "item_id", entry.Item.ID, "err", err)
			continue
		}
		s.attr.AddModifiers("equipment:"+def.StringID(), mods)
	}
}

// --- SettlementCtx (implements event.SettlementCtx) ---

func (s *PlayerSession) HasItem(it item.Item, qty float64) bool    { return s.inv.Has(it, qty) }
func (s *PlayerSession) GetItemQty(it item.Item) float64           { return s.inv.Get(it) }
func (s *PlayerSession) AddItem(it item.Item, qty float64) {
	s.inv.Add(it, qty)
	if qty > 0 {
		s.best.UnlockItem(it)
	}
}
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
// For multi-slot items, the system automatically occupies all required slots
// anchored to the clicked slot.
func (s *PlayerSession) Equip(ctx context.Context, it item.Item, clickedSlot string) error {
	def, ok := gameconfig.GetItemDefByID(it.ID)
	if !ok {
		return apperror.NotFound("item not found")
	}
	if !def.IsEquipment() && !def.IsTool() {
		return apperror.BadRequest("item is not equipment")
	}
	if !s.inv.Has(it, 1) {
		return apperror.BadRequest("item not in inventory")
	}

	slotType := resolveSlotType(clickedSlot)

	// Parse position requirements.
	reqMap := positionRequirements(def, slotType)
	if len(reqMap) == 0 {
		reqMap = map[string]int{slotBase(clickedSlot): 1}
	}
	if _, ok := reqMap[slotBase(clickedSlot)]; !ok {
		return apperror.BadRequest("item cannot be equipped in this slot")
	}

	// Build dynamic slot instances.
	allSlots := s.buildSlotInstances(slotType)
	found := false
	for _, sl := range allSlots {
		if sl == clickedSlot {
			found = true
			break
		}
	}
	if !found {
		return apperror.BadRequest("invalid slot")
	}

	// If clicked slot is occupied, remove the entire anchored piece first.
	if old, ok := s.eq.Get(clickedSlot); ok {
		if err := s.unequipByAnchor(old.AnchorSlot); err != nil {
			return err
		}
	}

	// Collect occupied slots (excluding the clicked slot area which we just freed).
	occupied := make(map[string]struct{})
	for sl := range s.eq.All() {
		occupied[sl] = struct{}{}
	}

	// Greedy target slot selection.
	targetSlots, err := chooseTargetSlots(allSlots, reqMap, clickedSlot, occupied)
	if err != nil {
		return apperror.BadRequest(err.Error())
	}

	// Deduct from inventory with EQUIP reason.
	s.inv.AddEquipChange(it, -1, true)

	// Apply attribute modifiers (once per piece, not per slot).
	mods, err := def.Modifiers(it, attribute.Get())
	if err != nil {
		return err
	}
	s.attr.AddModifiers("equipment:"+def.StringID(), mods)

	// Record slot mounts.
	for _, sl := range targetSlots {
		s.eq.Equip(sl, it, clickedSlot)
	}
	return nil
}

// Unequip removes the entire anchored piece that occupies the given slot.
func (s *PlayerSession) Unequip(ctx context.Context, slot string) error {
	entry, ok := s.eq.Get(slot)
	if !ok {
		return apperror.NotFound("no item equipped in slot")
	}
	return s.unequipByAnchor(entry.AnchorSlot)
}

func (s *PlayerSession) unequipByAnchor(anchor string) error {
	removed := s.eq.UnequipByAnchor(anchor)
	if len(removed) == 0 {
		return nil
	}
	// All removed slots belong to the same piece; grab item from any.
	var it item.Item
	for _, v := range removed {
		it = v
		break
	}
	if def, ok := gameconfig.GetItemDefByID(it.ID); ok {
		s.attr.RemoveModifiers("equipment:" + def.StringID())
	}
	s.inv.AddEquipChange(it, 1, false)
	s.best.UnlockItem(it)
	return nil
}

// ---------------------------------------------------------------------------
// Equipment helpers
// ---------------------------------------------------------------------------

var (
	toolSlotBases      = []string{"felling", "mining", "planting", "crafting", "forging", "enhancing"}
	equipmentSlotBases = []string{"main_hand", "side_hand", "head", "chest", "leg", "feet", "necklace", "treasure"}
)

func slotBase(slotID string) string {
	return strings.Split(slotID, "#")[0]
}

func resolveSlotType(slotID string) string {
	base := slotBase(slotID)
	for _, b := range toolSlotBases {
		if b == base {
			return "tool"
		}
	}
	for _, b := range equipmentSlotBases {
		if b == base {
			return "equipment"
		}
	}
	return ""
}

func (s *PlayerSession) buildSlotInstances(slotType string) []string {
	var bases []string
	if slotType == "tool" {
		bases = toolSlotBases
	} else {
		bases = equipmentSlotBases
	}
	var out []string
	for _, base := range bases {
		count := 1
		if s.attr != nil {
			attrName := base + "_slot_count"
			if aid, ok := attribute.Get().AttrID(attrName); ok {
				count = int(s.attr.GetFinal(aid))
			}
		}
		if count <= 1 {
			out = append(out, base)
			continue
		}
		for i := 1; i <= count; i++ {
			out = append(out, fmt.Sprintf("%s#%d", base, i))
		}
	}
	return out
}

func positionRequirements(def item.ItemDef, slotType string) map[string]int {
	out := make(map[string]int)
	if slotType == "tool" {
		for _, r := range def.ToolPositionReqs() {
			v := r.Value
			if v < 1 {
				v = 1
			}
			out[r.Position] += v
		}
	} else {
		for _, r := range def.EquipPositionReqs() {
			v := r.Value
			if v < 1 {
				v = 1
			}
			out[r.Position] += v
		}
	}
	return out
}

func chooseTargetSlots(allSlots []string, reqMap map[string]int, clickedSlot string, occupied map[string]struct{}) ([]string, error) {
	clickedBase := slotBase(clickedSlot)
	if _, ok := reqMap[clickedBase]; !ok {
		return nil, fmt.Errorf("item cannot be equipped in this slot")
	}

	slotsByBase := make(map[string][]string)
	for _, sl := range allSlots {
		b := slotBase(sl)
		slotsByBase[b] = append(slotsByBase[b], sl)
	}

	var selected []string
	for base, need := range reqMap {
		candidates := slotsByBase[base]
		if len(candidates) == 0 {
			return nil, fmt.Errorf("missing slot: %s", base)
		}

		var chosen []string
		if base == clickedBase {
			if _, taken := occupied[clickedSlot]; taken {
				return nil, fmt.Errorf("clicked slot is occupied")
			}
			chosen = append(chosen, clickedSlot)
			for _, sl := range candidates {
				if len(chosen) >= need {
					break
				}
				if sl == clickedSlot {
					continue
				}
				if _, taken := occupied[sl]; taken {
					continue
				}
				chosen = append(chosen, sl)
			}
		} else {
			for _, sl := range candidates {
				if len(chosen) >= need {
					break
				}
				if _, taken := occupied[sl]; taken {
					continue
				}
				chosen = append(chosen, sl)
			}
		}

		if len(chosen) < need {
			return nil, fmt.Errorf("not enough free slots for %s", base)
		}
		selected = append(selected, chosen...)
	}
	return selected, nil
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
	battlesMu   sync.RWMutex
	battles     map[int64]*battle.BattleSession // keyed by BattleConfig.NumericID
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
		battles:     make(map[int64]*battle.BattleSession),
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
		battles:     make(map[int64]*battle.BattleSession),
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

// =========================
// Battle management
// =========================

// AddBattle registers a battle session. The caller must ensure the NumericID is unique.
func (m *Manager) AddBattle(bs *battle.BattleSession) {
	m.battlesMu.Lock()
	defer m.battlesMu.Unlock()
	m.battles[bs.Config.NumericID] = bs
}

// RemoveBattle unregisters a battle session by its NumericID.
func (m *Manager) RemoveBattle(numericID int64) {
	m.battlesMu.Lock()
	defer m.battlesMu.Unlock()
	delete(m.battles, numericID)
}

// GetBattle returns a registered battle session by its NumericID.
func (m *Manager) GetBattle(numericID int64) (*battle.BattleSession, bool) {
	m.battlesMu.RLock()
	defer m.battlesMu.RUnlock()
	bs, ok := m.battles[numericID]
	return bs, ok
}

// Evict forcibly closes and removes an in-memory session.
// Used by test cleanup to ensure a user can be deleted safely.
func (m *Manager) Evict(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[userID]; ok {
		s.Close()
		delete(m.sessions, userID)
	}
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

// SendFullState builds a full state snapshot and sends it to the session's connection.
func (m *Manager) SendFullState(sess *PlayerSession) {
	states := map[string]any{
		"inventory":      sess.Inv(),
		"attribute":      sess.Attr(),
		"skill_xp":       sess.Skill(),
		"bestiary":       sess.Bestiary(),
		"event_execution": sess.Events(),
		"equipment":      sess.Equipment(),
	}
	full, err := m.reg.BuildFullSnapshot(states)
	if err != nil {
		sess.logger.Error("build full snapshot failed", "err", err)
		return
	}
	if c := sess.Conn(); c != nil {
		c.Send(wsx.Outbound{Type: "state.full", Payload: full})
	}
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
// The session is not added to the Manager —caller must call Add.
func (m *Manager) CreateSession(ctx context.Context, connID uuid.UUID, userID int64, database *db.DB, logger *slog.Logger) (*PlayerSession, error) {
	q := database.Queries

	// Lazy player initialization: if this user has never entered the game,
	// seed the default player data (skills, etc.) before loading subsystems.
	initStatus, err := q.IsPlayerInit(ctx, userID)
	if err != nil {
		// sql.ErrNoRows means the row doesn't exist →not initialized yet.
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	if initStatus != 1 {
		if err := playerinit.InitPlayer(ctx, userID, database); err != nil {
			return nil, apperror.Internal("initialize player").WithCause(err)
		}
	}

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

	// Bestiary from unlocked events + discovered items.
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

	itemRows, err := q.LoadDiscoveredItems(ctx, userID)
	if err != nil {
		return nil, err
	}
	best.LoadDiscoveredItems(itemRows)

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
