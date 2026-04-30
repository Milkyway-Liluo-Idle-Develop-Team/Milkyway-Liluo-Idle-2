package event_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/event"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	_ "modernc.org/sqlite"
)

func init() {
	if err := gameconfig.Load(); err != nil {
		panic("gameconfig: " + err.Error())
	}
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic("attribute: " + err.Error())
		}
	}
}

// mockCtx implements event.SettlementCtx for testing settle in isolation.
type mockCtx struct {
	items     map[item.Item]float64
	attrVals  map[attribute.AttributeID]float64
	skillLvl  map[gameconfig.SkillID]float64
	xpAdded   map[gameconfig.SkillID]float64
	unlocked    []gameconfig.EventID
	unlockedSet map[gameconfig.EventID]bool
}

func newMockCtx() *mockCtx {
	ctx := &mockCtx{
		items:       make(map[item.Item]float64),
		attrVals:    make(map[attribute.AttributeID]float64),
		skillLvl:    make(map[gameconfig.SkillID]float64),
		xpAdded:     make(map[gameconfig.SkillID]float64),
		unlockedSet: make(map[gameconfig.EventID]bool),
	}
	// Most events require starting_dialog_5 as a prerequisite.
	if sid, ok := gameconfig.StringToEventID("starting_dialog_5"); ok {
		ctx.unlockedSet[sid] = true
	}
	return ctx
}

func (m *mockCtx) HasItem(it item.Item, qty float64) bool { return m.items[it] >= qty }
func (m *mockCtx) GetItemQty(it item.Item) float64         { return m.items[it] }
func (m *mockCtx) AddItem(it item.Item, qty float64)       { m.items[it] += qty }
func (m *mockCtx) DeductItem(it item.Item, qty float64)     { m.items[it] -= qty }
func (m *mockCtx) AddXP(sid gameconfig.SkillID, xp float64) { m.xpAdded[sid] += xp }
func (m *mockCtx) GetAttr(id attribute.AttributeID) float64 { return m.attrVals[id] }
func (m *mockCtx) GetSkillLevel(sid gameconfig.SkillID) float64 {
	return m.skillLvl[sid]
}
func (m *mockCtx) UnlockEvent(id gameconfig.EventID) {
	m.unlocked = append(m.unlocked, id)
	m.unlockedSet[id] = true
}
func (m *mockCtx) IsEventUnlocked(id gameconfig.EventID) bool { return m.unlockedSet[id] }

func TestLoadEmpty(t *testing.T) {
	db, q := openEventDB(t)
	defer db.Close()
	st, err := event.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		t.Fatal("nil state")
	}
}

func TestSettleLoopProduces(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(event.ExecProvider)
	reg.Register(event.QueueProvider)

	db, q := openEventDB(t)
	defer db.Close()

	st, err := event.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}

	// felling_oak_tree: loop_time=2, rewards: 1 oak_logs + 20 XP (felling)
	// Requirements: felling >= 1, event starting_dialog_5 unlocked
	eid, _ := gameconfig.StringToEventID("felling_oak_tree")

	rec := record.NewRecorder(reg)
	st.SetRecorder(rec)

	// Add felling_oak_tree to queue.
	st.Enqueue(0, eid, -1) // -1 = infinite

	ctx := newMockCtx()
	// Fulfill requirements.
	fellingID, _ := gameconfig.StringToSkillID("felling")
	ctx.skillLvl[fellingID] = 1

	// Settle 2 seconds → exactly 1 cycle.
	rec.PushNamespace("tick")
	st.Settle(ctx, 2.0)
	rec.PopNamespace()
	st.ClearRecorder()

	// Check rewards.
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	oak := item.Item{ID: oakID}
	if ctx.items[oak] != 1 {
		t.Errorf("want 1 oak_logs, got %v", ctx.items[oak])
	}

	// Check XP.
	if ctx.xpAdded[fellingID] != 20 {
		t.Errorf("want 20 XP, got %v", ctx.xpAdded[fellingID])
	}

	// Check diff payload.
	diff, _ := reg.BuildDiff(rec)
	var m map[string]json.RawMessage
	json.Unmarshal(diff, &m)
	if _, ok := m["event_execution_changes"]; !ok {
		t.Error("missing event_execution_changes")
	}
	if _, ok := m["event_queue_changes"]; !ok {
		t.Error("missing event_queue_changes")
	}
}

func TestSwapWhenRequirementUnmet(t *testing.T) {
	_, q := openEventDB(t)
	st, _ := event.Load(context.Background(), q, 1)

	// felling_oak_tree requires felling >= 1 → unsatisfied
	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	// making_oak_plank requires crafting >= 1
	plankID, _ := gameconfig.StringToEventID("making_oak_plank")

	st.Enqueue(0, fellingID, -1) // position 0 — requires felling (unmet)
	st.Enqueue(0, plankID, -1)   // position 1 — requires crafting

	ctx := newMockCtx()
	// Only crafting is met, not felling.
	craftSkill, _ := gameconfig.StringToSkillID("crafting")
	ctx.skillLvl[craftSkill] = 5

	reg := record.NewRegistry()
	reg.Register(event.ExecProvider)
	reg.Register(event.QueueProvider)
	rec := record.NewRecorder(reg)
	st.SetRecorder(rec)
	rec.PushNamespace("tick")
	st.Settle(ctx, 2.0) // plankID loop_time=4, so <1 cycle
	rec.PopNamespace()

	// The swap should have occurred: felling unmet → swap with plank
	// Then plank advances partially (2s of 4s loop_time = 0 cycles).
	// But the progress should be saved.
	// Swap moved plank to front. plank loop_time=4, delta=2 → 0 cycles, nothing produced.
	// But verify the swap didn't crash and we can still operate.
	if len(ctx.unlocked) != 0 {
		t.Error("0 cycles should not unlock event")
	}
}

func TestFiniteTargetCycles(t *testing.T) {
	_, q := openEventDB(t)
	st, _ := event.Load(context.Background(), q, 1)

	eid, _ := gameconfig.StringToEventID("felling_oak_tree")
	st.Enqueue(0, eid, 2) // exactly 2 cycles, then remove

	ctx := newMockCtx()
	fellingID, _ := gameconfig.StringToSkillID("felling")
	ctx.skillLvl[fellingID] = 1

	// 4 seconds → 2 cycles (loop_time=2).
	st.Settle(ctx, 4.0)

	oakID, _ := gameconfig.StringToItemID("oak_logs")
	oak := item.Item{ID: oakID}
	if ctx.items[oak] != 2 {
		t.Errorf("want 2 oak_logs, got %v", ctx.items[oak])
	}

	// After 2 cycles, event should be consumed.
	st.Settle(ctx, 10.0)
	if ctx.items[oak] != 2 {
		t.Errorf("should still be 2 after consumption, got %v", ctx.items[oak])
	}
}

func TestConsumedPositionSkipped(t *testing.T) {
	_, q := openEventDB(t)
	st, _ := event.Load(context.Background(), q, 1)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	dirtID, _ := gameconfig.StringToEventID("mining_dirt")

	st.Enqueue(0, fellingID, 1)  // position 0, 1 cycle
	st.Enqueue(0, dirtID, -1)    // position 1, infinite

	ctx := newMockCtx()
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	ctx.skillLvl[fellingSkill] = 1
	ctx.skillLvl[miningSkill] = 1

	// First settle: consume felling (1 cycle), then dirt production starts.
	st.Settle(ctx, 10.0)

	oakID, _ := gameconfig.StringToItemID("oak_logs")
	dirtItemID, _ := gameconfig.StringToItemID("dirt")
	oak := item.Item{ID: oakID}
	dirt := item.Item{ID: dirtItemID}

	if ctx.items[oak] != 1 {
		t.Errorf("felling: want 1 oak, got %v", ctx.items[oak])
	}
	if ctx.items[dirt] == 0 {
		t.Error("dirt should have been produced after felling consumed")
	}

	// Second settle: position 0 is consumed (EventID=0), should skip to dirt.
	before := ctx.items[dirt]
	st.Settle(ctx, 10.0)
	if ctx.items[dirt] <= before {
		t.Error("dirt should continue after consumed position is skipped")
	}
}

func TestEmptyQueueNoPanic(t *testing.T) {
	_, q := openEventDB(t)
	st, _ := event.Load(context.Background(), q, 1)

	ctx := newMockCtx()
	// Should not panic on empty queue.
	st.Settle(ctx, 100.0)
}

func TestFlushAndReloadKeepsEnqueued(t *testing.T) {
	db, q := openEventDB(t)
	st, _ := event.Load(context.Background(), q, 1)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	dirtID, _ := gameconfig.StringToEventID("mining_dirt")
	st.Enqueue(0, fellingID, 1)   // 1 cycle, then consumed
	st.Enqueue(0, dirtID, -1)    // infinite

	if err := st.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	// Reload and verify both are present.
	st2, _ := event.Load(context.Background(), q, 1)
	ctx := newMockCtx()
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	ctx.skillLvl[fellingSkill] = 1
	ctx.skillLvl[miningSkill] = 1

	// 4s: felling (1 cycle=2s) consumed, dirt (loop_time=2) gets 2s → 1 cycle.
	st2.Settle(ctx, 4.0)
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	dirtItemID, _ := gameconfig.StringToItemID("dirt")
	if ctx.items[item.Item{ID: oakID}] != 1 {
		t.Errorf("felling should produce 1 after reload, got %v", ctx.items[item.Item{ID: oakID}])
	}
	if ctx.items[item.Item{ID: dirtItemID}] == 0 {
		t.Errorf("dirt should produce after reload (position 1), got %v", ctx.items[item.Item{ID: dirtItemID}])
	}
	db.Close()
}

func TestFlushAfterMixOfConsumeAndEnqueue(t *testing.T) {
	db, q := openEventDB(t)
	st, _ := event.Load(context.Background(), q, 1)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	dirtID, _ := gameconfig.StringToEventID("mining_dirt")
	plankID, _ := gameconfig.StringToEventID("making_oak_plank")

	// [felling@0: 1 cycle, dirt@1: 2 cycles, plank@2:2 cycles]
	st.Enqueue(0, fellingID, 1)
	st.Enqueue(0, dirtID, 2)
	st.Enqueue(0, plankID, 2)

	ctx := newMockCtx()
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	craftSkill, _ := gameconfig.StringToSkillID("crafting")
	ctx.skillLvl[fellingSkill] = 1
	ctx.skillLvl[miningSkill] = 1
	ctx.skillLvl[craftSkill] = 10
	// Plank needs oak_logs.
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	ctx.items[item.Item{ID: oakID}] = 100

	// Settle: felling (2s) consumed, dirt (2*2=4s) consumed, plank (2*4=8s) consumed.
	st.Settle(ctx, 20.0)

	// Verify plank was produced in the first settlement.
	plankItemID, _ := gameconfig.StringToItemID("oak_plank")
	if ctx.items[item.Item{ID: plankItemID}] == 0 {
		t.Error("plank should be produced before flush")
	}

	// Flush → reload.
	if err := st.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	st2, _ := event.Load(context.Background(), q, 1)

	// All three consumed → reloaded queue should be empty.
	ctx2 := newMockCtx()
	ctx2.skillLvl[fellingSkill] = 1
	ctx2.skillLvl[miningSkill] = 1
	ctx2.skillLvl[craftSkill] = 10
	ctx2.items[item.Item{ID: oakID}] = 100

	before := ctx2.items[item.Item{ID: plankItemID}]
	st2.Settle(ctx2, 100.0)
	if ctx2.items[item.Item{ID: plankItemID}] != before {
		t.Error("reloaded queue should be empty, nothing new produced")
	}
	db.Close()
}

func TestFlushRoundTrip(t *testing.T) {
	db, q := openEventDB(t)

	st, _ := event.Load(context.Background(), q, 1)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	dirtID, _ := gameconfig.StringToEventID("mining_dirt")
	st.Enqueue(0, fellingID, 1)
	st.Enqueue(0, dirtID, -1)

	ctx := newMockCtx()
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	ctx.skillLvl[fellingSkill] = 1
	ctx.skillLvl[miningSkill] = 1

	// Consume felling position.
	st.Settle(ctx, 10.0)

	// Flush then reload from same DB.
	if err := st.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	st2, err := event.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Dirt production should still be active; felling consumed.
	ctx2 := newMockCtx()
	ctx2.skillLvl[miningSkill] = 1
	st2.Settle(ctx2, 10.0)
	dirtItemID, _ := gameconfig.StringToItemID("dirt")
	dirt := item.Item{ID: dirtItemID}
	if ctx2.items[dirt] == 0 {
		t.Error("dirt should produce after reload (felling consumed)")
	}
}

func openEventDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS player_active_events (
			user_id INTEGER NOT NULL, queue_id INTEGER NOT NULL DEFAULT 0,
			event_id INTEGER NOT NULL, position INTEGER NOT NULL,
			target_cycles INTEGER NOT NULL DEFAULT -1, progress REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, queue_id, position)
		)
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db, dbgen.New(db)
}
