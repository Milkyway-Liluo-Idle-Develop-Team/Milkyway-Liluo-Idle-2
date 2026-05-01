package session_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/inventory"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/skill"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func testLogger() *slog.Logger { return slog.Default() }

func init() {
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic(err)
		}
	}
}

func newTestManager(t *testing.T) *session.Manager {
	t.Helper()
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	return session.NewManager(reg, nil)
}

// newLockedSession creates a session, adds it to the manager, locks it,
// and returns a cleanup function.
func newLockedSession(t *testing.T, mgr *session.Manager, userID int64) (*session.PlayerSession, func()) {
	t.Helper()
	id := uuid.New()
	s := session.New(id, userID, testLogger())
	mgr.Add(s)
	locked, ok := mgr.LockSession(id)
	if !ok {
		t.Fatal("LockSession failed")
	}
	return locked, func() {
		mgr.UnlockSession(locked)
		mgr.Remove(id)
	}
}

func TestAddRemove(t *testing.T) {
	mgr := newTestManager(t)
	id := uuid.New()

	s := session.New(id, 42, testLogger())
	mgr.Add(s)

	if mgr.Count() != 1 {
		t.Fatalf("want 1, got %d", mgr.Count())
	}

	got, ok := mgr.Get(id)
	if !ok || got.ID != id {
		t.Fatal("session not found")
	}

	mgr.Remove(id)
	if mgr.Count() != 0 {
		t.Fatalf("want 0, got %d", mgr.Count())
	}
}

func TestGetByUser(t *testing.T) {
	mgr := newTestManager(t)

	s1 := session.New(uuid.New(), 1, testLogger())
	s2 := session.New(uuid.New(), 1, testLogger())
	s3 := session.New(uuid.New(), 2, testLogger())
	mgr.Add(s1)
	mgr.Add(s2)
	mgr.Add(s3)

	byUser := mgr.GetByUser(1)
	if len(byUser) != 2 {
		t.Fatalf("user 1: want 2 sessions, got %d", len(byUser))
	}
}

func TestConcurrentAccess(t *testing.T) {
	mgr := newTestManager(t)

	var wg sync.WaitGroup
	n := 100
	wg.Add(n * 2)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id := uuid.New()
			s := session.New(id, int64(i%10), testLogger())
			mgr.Add(s)
		}(i)
	}

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = mgr.GetByUser(int64(i % 10))
			_ = mgr.Count()
		}(i)
	}

	wg.Wait()
}

func TestRecorderLifecycle(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	rec := mgr.NewRecorder()
	s.SetRecorder(rec)

	cleared := s.ClearRecorder()
	if cleared != rec {
		t.Error("ClearRecorder should return the recorder")
	}
}

func TestAttributeInstanceOnSession(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	r := attribute.Get()
	id, _ := r.AttrID("physical_power")
	val := s.Attr().GetFinal(id)

	def, _ := r.Def(id)
	if val != def.DefaultValue {
		t.Errorf("want default %v, got %v", def.DefaultValue, val)
	}
}

// --- Integration tests ---

func TestExecutionCycle(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")
	accID, _ := r.AttrID("accuracy")

	rec := mgr.NewRecorder()
	s.SetRecorder(rec)
	rec.PushNamespace("event_execution")

	s.Attr().AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
		{AttrID: accID, Op: attribute.OpAdd, Value: 10, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	pVal := s.Attr().GetFinal(physID)
	aVal := s.Attr().GetFinal(accID)

	rec.PopNamespace()
	s.ClearRecorder()

	if pVal != 25 {
		t.Errorf("physical_power: want 25, got %v", pVal)
	}
	if aVal != 10 {
		t.Errorf("accuracy: want 10, got %v", aVal)
	}

	diff, _ := mgr.Registry().BuildDiff(rec)
	if string(diff) == "{}" {
		t.Fatal("diff should not be empty")
	}
}

func TestMultipleExecutionCycles(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	rec1 := mgr.NewRecorder()
	s.SetRecorder(rec1)
	rec1.PushNamespace("tick_1")
	s.Attr().AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	rec1.PopNamespace()
	s.ClearRecorder()

	diff1, _ := reg.BuildDiff(rec1)
	if string(diff1) == "{}" {
		t.Error("cycle 1 diff should not be empty")
	}

	rec2 := mgr.NewRecorder()
	s.SetRecorder(rec2)
	rec2.PushNamespace("tick_2")
	s.Attr().UpdateModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 30, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	rec2.PopNamespace()
	s.ClearRecorder()

	val := s.Attr().GetFinal(physID)
	if val != 40 {
		t.Errorf("after update: want 40, got %v", val)
	}
}

func TestExecutionCycleNoChanges(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	rec := mgr.NewRecorder()
	s.SetRecorder(rec)
	rec.PushNamespace("tick_idle")

	r := attribute.Get()
	id, _ := r.AttrID("physical_power")
	_ = s.Attr().GetFinal(id)

	rec.PopNamespace()
	s.ClearRecorder()

	diff, _ := reg.BuildDiff(rec)
	if string(diff) != "{}" {
		t.Errorf("idle cycle should produce empty diff, got %s", diff)
	}
}

func TestSessionLifecycleSimulation(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)

	connID := uuid.New()
	s := session.New(connID, 42, testLogger())
	mgr.Add(s)

	if mgr.Count() != 1 {
		t.Fatal("session should be added")
	}

	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")
	fellingID, _ := r.AttrID("felling_production_multiplier")

	// Lock → operate → unlock
	{
		locked, ok := mgr.LockSession(connID)
		if !ok {
			t.Fatal("LockSession failed")
		}
		rec := mgr.NewRecorder()
		locked.SetRecorder(rec)
		rec.PushNamespace("event_execution")

		locked.Attr().AddModifiers("tool:axe", []attribute.Modifier{
			{AttrID: fellingID, Op: attribute.OpAdd, Value: 0.1, Display: attribute.DisplayPercent, Source: "tool:axe"},
		})
		locked.Attr().AddModifiers("equipment:armor", []attribute.Modifier{
			{AttrID: physID, Op: attribute.OpAdd, Value: 5, Display: attribute.DisplayFixed, Source: "equipment:armor"},
		})

		rec.PopNamespace()
		locked.ClearRecorder()

		diff, err := reg.BuildDiff(rec)
		if err != nil {
			t.Fatal(err)
		}

		var m map[string]json.RawMessage
		json.Unmarshal(diff, &m)
		raw, ok := m["attribute_changes"]
		if !ok {
			t.Fatal("missing attribute_changes")
		}
		var changes []struct{ AttrID string `json:"attr_id"` }
		json.Unmarshal(raw, &changes)

		attrIDs := make(map[string]bool)
		for _, c := range changes {
			attrIDs[c.AttrID] = true
		}
		if !attrIDs["physical_power"] {
			t.Error("diff should include physical_power")
		}
		if !attrIDs["felling_production_multiplier"] {
			t.Error("diff should include felling_production_multiplier")
		}

		mgr.UnlockSession(locked)
	}

	mgr.Remove(connID)
	if mgr.Count() != 0 {
		t.Fatal("session should be removed after disconnect")
	}
}

func TestContextInSettlement(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	s.Attr().AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 20, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})

	ctx := attribute.NewContext()
	ctx.AddMult(physID, 0.5)

	buffed := s.Attr().GetFinalWithContext(physID, ctx)
	if buffed != 45 {
		t.Errorf("buffed: want 45, got %v", buffed)
	}

	normal := s.Attr().GetFinal(physID)
	if normal != 30 {
		t.Errorf("persistent: want 30, got %v", normal)
	}
}

func TestFullSnapshotOnConnect(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	s.Attr().AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})

	data, err := reg.BuildFullSnapshot(map[string]any{
		"attribute": s.Attr(),
	})
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	json.Unmarshal(data, &m)
	var attrs []struct {
		AttrID     string  `json:"attr_id"`
		FinalValue float64 `json:"final_value"`
	}
	json.Unmarshal(m["attribute"], &attrs)

	if len(attrs) != r.Count() {
		t.Fatalf("full snapshot: want %d attrs, got %d", r.Count(), len(attrs))
	}
}

func TestRecorderIsCleanBetweenCycles(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	rec1 := mgr.NewRecorder()
	s.SetRecorder(rec1)
	rec1.PushNamespace("cycle_1")
	s.Attr().AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 10, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	rec1.PopNamespace()
	s.ClearRecorder()

	rec2 := mgr.NewRecorder()
	s.SetRecorder(rec2)
	diff, _ := reg.BuildDiff(rec2)
	if string(diff) != "{}" {
		t.Errorf("fresh recorder should produce empty diff, got %s", diff)
	}

	val := s.Attr().GetFinal(physID)
	if val != 20 {
		t.Errorf("dirty state should persist: want 20, got %v", val)
	}
	s.ClearRecorder()
}

func TestLockSession(t *testing.T) {
	mgr := newTestManager(t)
	id := uuid.New()

	s := session.New(id, 42, testLogger())
	mgr.Add(s)

	locked, ok := mgr.LockSession(id)
	if !ok {
		t.Fatal("LockSession should find the session")
	}

	locked.SetRecorder(mgr.NewRecorder())
	locked.ClearRecorder()
	mgr.UnlockSession(locked)
}

func TestLockSessionNotFound(t *testing.T) {
	mgr := newTestManager(t)
	_, ok := mgr.LockSession(uuid.New())
	if ok {
		t.Fatal("LockSession should return false for unknown id")
	}
}

// --- Integration tests with inventory ---

func openInvDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS player_inventory (
			user_id INTEGER NOT NULL, item_id INTEGER NOT NULL,
			item_state INTEGER NOT NULL DEFAULT 0,
			quantity REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, item_id, item_state)
		)
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db, dbgen.New(db)
}

// TestSessionWithInventory runs a full tick with attribute and inventory
// changes, then verifies both systems appear in the diff packet.
func TestSessionWithInventory(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	reg.Register(inventory.Provider)
	mgr := session.NewManager(reg, nil)

	_, q := openInvDB(t)
	invSt, err := inventory.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}

	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)

	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	// Lock and operate.
	locked, ok := mgr.LockSession(s.ID)
	if !ok {
		t.Fatal("lock failed")
	}
	defer mgr.UnlockSession(locked)

	rec := mgr.NewRecorder()
	locked.SetRecorder(rec)
	rec.PushNamespace("tick")

	// Equip sword → attribute change.
	locked.Attr().AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	// Produce some items.
	locked.Inv().Add(item.Item{ID: 1, State: 0}, 5)
	locked.Inv().Add(item.Item{ID: 2, State: 0}, 3.5)

	rec.PopNamespace()
	locked.ClearRecorder()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	json.Unmarshal(diff, &m)

	if _, ok := m["attribute_changes"]; !ok {
		t.Error("missing attribute_changes")
	}
	if _, ok := m["inventory_changes"]; !ok {
		t.Error("missing inventory_changes")
	}
}

// TestInventoryFlushInCycle tests that dirty inventory can be flushed.
func TestInventoryFlushInCycle(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(inventory.Provider)
	mgr := session.NewManager(reg, nil)

	_, q := openInvDB(t)
	invSt, err := inventory.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}

	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)

	locked, ok := mgr.LockSession(s.ID)
	if !ok {
		t.Fatal("lock failed")
	}
	defer mgr.UnlockSession(locked)

	locked.Inv().Add(item.Item{ID: 10, State: 0}, 7)

	// Flush dirty items.
	if err := locked.Inv().Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	// Reload and verify.
	invSt2, err := inventory.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := invSt2.Get(item.Item{ID: 10, State: 0}); got != 7 {
		t.Errorf("after flush+reload: want 7, got %v", got)
	}
}

// --- three-system integration test ---

func openSkillDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS player_skills (
			user_id INTEGER NOT NULL, skill_id INTEGER NOT NULL,
			level REAL NOT NULL DEFAULT 0, xp REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, skill_id)
		)
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db, dbgen.New(db)
}

// TestFullCycleAllSystems runs a tick with attribute, inventory, and skill
// changes, then verifies all three appear in the diff packet.
func TestFullCycleAllSystems(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	reg.Register(inventory.Provider)
	reg.Register(skill.Provider)
	mgr := session.NewManager(reg, nil)

	// Load inventory.
	_, invQ := openInvDB(t)
	invSt, err := inventory.Load(context.Background(), invQ, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Load skill curve and state.
	curve, err := skill.LoadCurve()
	if err != nil {
		t.Fatal(err)
	}
	_, skillQ := openSkillDB(t)
	skillSt, err := skill.Load(context.Background(), skillQ, 1, curve)
	if err != nil {
		t.Fatal(err)
	}

	// Build session.
	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	s.SetSkill(skillSt)
	mgr.Add(s)

	locked, ok := mgr.LockSession(s.ID)
	if !ok {
		t.Fatal("lock failed")
	}
	defer mgr.UnlockSession(locked)

	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	rec := mgr.NewRecorder()
	locked.SetRecorder(rec)
	rec.PushNamespace("tick")

	// Attribute: equip sword.
	locked.Attr().AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	// Inventory: produce logs, consume planks.
	locked.Inv().Add(item.Item{ID: 1, State: 0}, 5)
	locked.Inv().Add(item.Item{ID: 2, State: 0}, -2)
	// Skill: gain XP.
	locked.Skill().AddXP(gameconfig.SkillID(3), 200) // felling

	rec.PopNamespace()
	locked.ClearRecorder()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	json.Unmarshal(diff, &m)

	if _, ok := m["attribute_changes"]; !ok {
		t.Error("missing attribute_changes")
	}
	if _, ok := m["inventory_changes"]; !ok {
		t.Error("missing inventory_changes")
	}
	if _, ok := m["skill_xp_changes"]; !ok {
		t.Error("missing skill_xp_changes")
	}
}
