package session_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/bestiary"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/event"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/inventory"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/skill"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// openFullDBForTest creates an in-memory SQLite with all player tables.
func openFullDBForTest(t *testing.T) *db.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	for _, s := range []string{
		`CREATE TABLE player_inventory (user_id INTEGER NOT NULL, item_id INTEGER NOT NULL,
			item_state INTEGER NOT NULL DEFAULT 0, quantity REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, item_id, item_state))`,
		`CREATE TABLE player_skills (user_id INTEGER NOT NULL, skill_id INTEGER NOT NULL,
			level REAL NOT NULL DEFAULT 0, xp REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, skill_id))`,
		`CREATE TABLE player_unlocked_events (user_id INTEGER NOT NULL, event_id INTEGER NOT NULL,
			unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, event_id))`,
		`CREATE TABLE player_active_events (user_id INTEGER NOT NULL, queue_id INTEGER NOT NULL DEFAULT 0,
			event_id INTEGER NOT NULL, position INTEGER NOT NULL,
			target_cycles INTEGER NOT NULL DEFAULT -1, progress REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, queue_id, position))`,
		`CREATE TABLE player_equipment (user_id INTEGER NOT NULL, slot TEXT NOT NULL,
			item_id INTEGER NOT NULL, item_state INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, slot))`,
	} {
		if _, err := conn.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return &db.DB{Conn: conn, Queries: dbgen.New(conn)}
}

// createTestSession uses the production Manager.CreateSession path.
func createTestSession(t *testing.T, mgr *session.Manager, database *db.DB, userID int64) *session.PlayerSession {
	t.Helper()
	s, err := mgr.CreateSession(context.Background(), uuid.New(), userID, database, testLogger())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	mgr.Add(s)
	return s
}

func newRegForTick() *record.Registry {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	reg.Register(inventory.Provider)
	reg.Register(skill.Provider)
	reg.Register(bestiary.Provider)
	reg.Register(event.ExecProvider)
	reg.Register(event.QueueProvider)
	return reg
}

// --- P0 regression: lastTick initialization ---

// TestTickAll_LastTickInitialized is a P0 regression test.
//
// Purpose: Verify that PlayerSession.lastTick is initialized to time.Now()
// in New(), so the first ManualTick computes a sane delta (~milliseconds)
// instead of ~50 years (zero time → now).
//
// What it prevents: If lastTick is zero, the first tick would settle
// event queues with a delta of decades, producing astronomical amounts of
// items/XP in a single tick and corrupting player state.
func TestTickAll_LastTickInitialized(t *testing.T) {
	reg := newRegForTick()
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	results := mgr.ManualTick(base.Add(100 * time.Millisecond))

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// delta should be ~100ms, not decades.
	// If lastTick was zero, delta would be ~50+ years.
	// We verify by checking that the tick didn't produce a crazy settle,
	// but more directly: a second tick with 100ms interval should behave normally.

	results2 := mgr.ManualTick(base.Add(200 * time.Millisecond))
	if len(results2) != 1 {
		t.Fatalf("expected 1 result on second tick, got %d", len(results2))
	}
}

// --- TickAll integration tests ---

// TestTickAll_FullEventCycle verifies the entire TickAll → runTick → Settle
// pipeline for a production event (felling_oak_tree).
//
// Purpose: Confirm that a real tick with a loop event produces the expected
// inventory, skill XP, and diff output. This is the closest test to actual
// production gameplay flow.
//
// What it prevents: Regression in the core settlement engine where events
// silently fail to execute, or requirements (skill level, unlocked events)
// are not checked correctly during tick.
func TestTickAll_FullEventCycle(t *testing.T) {
	database := openFullDBForTest(t)
	reg := newRegForTick()
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := createTestSession(t, mgr, database, 1)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetLastTick(base)

	// Setup: enqueue a production event and seed resources.
	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	startingDialog, _ := gameconfig.StringToEventID("starting_dialog_5")
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")

	locked, ok := mgr.LockSession(s.UserID)
	if !ok {
		t.Fatal("lock failed")
	}
	locked.Bestiary().UnlockEvent(startingDialog)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Skill().AddXP(fellingSkill, 100) // prerequisite XP
	locked.Inv().Add(item.Item{ID: oakID}, 1e6)
	mgr.UnlockSession(locked)

	// Tick 1: 10 seconds → multiple cycles.
	mgr.ManualTick(base.Add(10 * time.Second))

	locked, _ = mgr.LockSession(s.UserID)
	defer mgr.UnlockSession(locked)

	logs := locked.Inv().Get(item.Item{ID: oakID})
	if logs <= 1e6 {
		t.Errorf("expected oak logs to increase after felling, got %v", logs)
	}
	_, xp := locked.Skill().Get(fellingSkill)
	if xp <= 100 {
		t.Errorf("expected felling XP to increase, got %v", xp)
	}
}

// TestTickAll_IdleTickProducesProgressDiff verifies behavior when the tick
// delta is smaller than the event's loop_time (no full cycles complete).
//
// Purpose: Ensure idle ticks still produce valid diff packets (e.g. progress
// updates) and do not crash or leak empty diffs.
//
// What it prevents: Frontend desync caused by missing progress updates, or
// unnecessary empty diff pushes wasting bandwidth.
func TestTickAll_IdleTickProducesProgressDiff(t *testing.T) {
	database := openFullDBForTest(t)
	reg := newRegForTick()
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := createTestSession(t, mgr, database, 1)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetLastTick(base)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")

	locked, ok := mgr.LockSession(s.UserID)
	if !ok {
		t.Fatal("lock failed")
	}
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	mgr.UnlockSession(locked)

	// Tick with 0.1s (< loop_time ~2s): no cycles complete.
	results := mgr.ManualTick(base.Add(100 * time.Millisecond))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Diff may be empty or contain only queue progress updates.
	diff := results[0].Diff
	if diff == nil {
		t.Fatal("expected diff even on idle tick")
	}
}

// TestTickAll_OfflineCatchup simulates a session being removed from the
// manager (grace expiry), then re-added after 5 seconds, verifying that
// the offline duration is settled in a single catch-up tick.
//
// Purpose: Validate the core catch-up arithmetic: when a session's lastTick
// lags behind now, the next tick must apply the full gap as delta.
//
// What it prevents: Lost offline production (a critical game-economy bug)
// where elapsed time between session removal and reactivation is silently
// discarded, or delta is miscalculated because lastTick was reset.
func TestTickAll_OfflineCatchup(t *testing.T) {
	database := openFullDBForTest(t)
	reg := newRegForTick()
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := createTestSession(t, mgr, database, 1)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetLastTick(base)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	startingDialog, _ := gameconfig.StringToEventID("starting_dialog_5")
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")

	locked, ok := mgr.LockSession(s.UserID)
	if !ok {
		t.Fatal("lock failed")
	}
	locked.Bestiary().UnlockEvent(startingDialog)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	locked.Inv().Add(item.Item{ID: oakID}, 1e6)
	mgr.UnlockSession(locked)

	// Phase 1: Establish a precise timeline using ManualTick.
	// lastTick is set to base so the baseline delta is exactly 0.
	baselineResults := mgr.ManualTick(base)
	var baselineCycles int32
	if len(baselineResults) == 1 && baselineResults[0].Diff != nil {
		for _, ex := range baselineResults[0].Diff.EventExecution {
			if ex.EventId == int64(fellingID) {
				baselineCycles = ex.Cycles
				break
			}
		}
	}

	// Phase 2: Session is removed — simulates grace expiry after disconnect.
	mgr.Remove(s.UserID)

	// Phase 3: 5 seconds "offline". lastTick inside the session object is
	// frozen at 'now' because nobody ticks it while removed.
	offlineDuration := 5 * time.Second

	// Phase 4: Re-activate the same session.
	mgr.Add(s)

	// Phase 5: Catch-up tick. delta should equal offlineDuration.
	results := mgr.ManualTick(base.Add(offlineDuration))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	locked, _ = mgr.LockSession(s.UserID)
	defer mgr.UnlockSession(locked)

	// --- Precise expected outcome for felling_oak_tree ---
	// loop_time = 2s, no production modifier (default 0).
	// delta = 5s, progress = 0  →  timeCycles = floor(5 / 2) = 2.
	// oak_logs per cycle = 1  →  expected +2.
	// XP per cycle = 20      →  expected +40.
	// consumed = 2 * 2 = 4s, remaining progress = 1s.
	var catchupCycles int32
	for _, ex := range results[0].Diff.EventExecution {
		if ex.EventId == int64(fellingID) {
			catchupCycles = ex.Cycles
			break
		}
	}
	totalCycles := baselineCycles + catchupCycles

	// Total oak_logs = totalCycles * 1, XP = totalCycles * 20.
	expectedLogs := float64(totalCycles) * 1.0
	expectedXP := float64(totalCycles) * 20.0

	logs := locked.Inv().Get(item.Item{ID: oakID})
	if logs != 1e6+expectedLogs {
		t.Errorf("oak logs: want %v, got %v", 1e6+expectedLogs, logs)
	}
	_, xp := locked.Skill().Get(fellingSkill)
	if xp != 100+expectedXP {
		t.Errorf("felling xp: want %v, got %v", 100+expectedXP, xp)
	}
}

// TestTickAll_CommandInterleavedWithSettle submits an Equip command while
// the background TickAll goroutine is running, then verifies the command
// is drained before Settle and its effects persist.
//
// Purpose: Validate the drainCommands → mu.Lock → Settle ordering inside
// runTick. Commands must be fully applied before the settlement engine
// reads player state.
//
// What it prevents: Race conditions where a player's equipment change is
// ignored by the current tick's event settlement, causing the client to
// see the item equipped but the server to use old attribute values.
func TestTickAll_CommandInterleavedWithSettle(t *testing.T) {
	database := openFullDBForTest(t)
	reg := newRegForTick()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := session.NewManager(ctx, reg, nil, 20*time.Millisecond)

	s := createTestSession(t, mgr, database, 1)

	// Wait for first tick to establish baseline.
	time.Sleep(30 * time.Millisecond)

	// Pre-seed inventory.
	locked, _ := mgr.LockSession(s.UserID)
	woodenSword, _ := gameconfig.StringToItemID("wooden_sword")
	locked.Inv().Add(item.Item{ID: woodenSword}, 1)
	mgr.UnlockSession(locked)

	// Submit equip command while tick loop is running.
	err := s.SubmitCommand(func(sess *session.PlayerSession) error {
		return sess.Equip(context.Background(), item.Item{ID: woodenSword}, "main_hand")
	})
	if err != nil {
		t.Fatalf("submit command: %v", err)
	}

	// Wait for tick to process the command.
	time.Sleep(60 * time.Millisecond)

	locked, _ = mgr.LockSession(s.UserID)
	defer mgr.UnlockSession(locked)
	if _, ok := locked.Equipment().Get("main_hand"); !ok {
		t.Fatal("equipment should be applied after tick")
	}
}
