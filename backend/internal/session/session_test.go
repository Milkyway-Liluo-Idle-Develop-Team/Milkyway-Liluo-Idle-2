package session_test

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/equipment"
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
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
	locked, ok := mgr.LockSession(userID)
	if !ok {
		t.Fatal("LockSession failed")
	}
	return locked, func() {
		mgr.UnlockSession(locked)
		mgr.Remove(userID)
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

	got, ok := mgr.Get(42)
	if !ok || got.ID != id {
		t.Fatal("session not found")
	}

	mgr.Remove(42)
	if mgr.Count() != 0 {
		t.Fatalf("want 0, got %d", mgr.Count())
	}
}

func TestGetByUser(t *testing.T) {
	mgr := newTestManager(t)

	s1 := session.New(uuid.New(), 1, testLogger())
	s2 := session.New(uuid.New(), 2, testLogger())
	mgr.Add(s1)
	mgr.Add(s2)

	got, ok := mgr.GetByUser(1)
	if !ok || got != s1 {
		t.Fatal("user 1 session not found")
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
			_, _ = mgr.GetByUser(int64(i % 10))
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
	if len(diff.Attribute) == 0 {
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
	if len(diff1.Attribute) == 0 {
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
	if len(diff.Attribute) != 0 {
		t.Errorf("idle cycle should produce empty diff")
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
		locked, ok := mgr.LockSession(42)
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

				_ = diff // was json.Unmarshal
		// was raw ok
		if !ok {
			t.Fatal("missing attribute_changes")
		}
		// was changes struct AttrID string `json:"attr_id"` }
		// was raw unmarshal

		attrIDs := make(map[string]bool)
		for _, c := range diff.Attribute {
			attrIDs[c.AttrId] = true
		}
		if !attrIDs["physical_power"] {
			t.Error("diff should include physical_power")
		}
		if !attrIDs["felling_production_multiplier"] {
			t.Error("diff should include felling_production_multiplier")
		}

		mgr.UnlockSession(locked)
	}

	mgr.Remove(42)
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

	
	if len(data.Attribute) != r.Count() {
		t.Fatalf("full snapshot: want %d attrs, got %d", r.Count(), len(data.Attribute))
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
	if len(diff.Attribute) != 0 {
		t.Errorf("fresh recorder should produce empty diff")
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

	locked, ok := mgr.LockSession(42)
	if !ok {
		t.Fatal("LockSession should find the session")
	}

	locked.SetRecorder(mgr.NewRecorder())
	locked.ClearRecorder()
	mgr.UnlockSession(locked)
}

func TestLockSessionNotFound(t *testing.T) {
	mgr := newTestManager(t)
	_, ok := mgr.LockSession(9999)
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
	locked, ok := mgr.LockSession(s.UserID)
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

		_ = diff // was json.Unmarshal

	if len(diff.Attribute) == 0 {
		t.Error("missing attribute_changes")
	}
	if len(diff.Inventory) == 0 {
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

	locked, ok := mgr.LockSession(s.UserID)
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

	locked, ok := mgr.LockSession(s.UserID)
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

		_ = diff // was json.Unmarshal

	if len(diff.Attribute) == 0 {
		t.Error("missing attribute_changes")
	}
	if len(diff.Inventory) == 0 {
		t.Error("missing inventory_changes")
	}
	if len(diff.SkillXp) == 0 {
		t.Error("missing skill_xp_changes")
	}
}

// --- Equipment tests ---

func TestEquip(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0} // wooden_sword
	invSt.Add(sword, 1)
	s.SetInv(invSt)

	err := s.Equip(context.Background(), sword, "main_hand")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.Equipment().Get("main_hand")
	if !ok {
		t.Fatal("main_hand should be equipped")
	}
	if got.ID != 35 {
		t.Errorf("want wooden_sword(35), got %v", got.ID)
	}
	if s.Inv().Get(sword) != 0 {
		t.Error("inventory should have 0 after equip")
	}
	physID, _ := attribute.Get().AttrID("physical_power")
	val := s.Attr().GetFinal(physID)
	def, _ := attribute.Get().Def(physID)
	if val <= def.DefaultValue {
		t.Errorf("physical_power should increase after equip: want > %v, got %v", def.DefaultValue, val)
	}
}

func TestEquipUnequip(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}
	invSt.Add(sword, 1)
	s.SetInv(invSt)

	physID, _ := attribute.Get().AttrID("physical_power")
	beforeUnequip := s.Attr().GetFinal(physID)

	s.Equip(context.Background(), sword, "main_hand")
	equippedVal := s.Attr().GetFinal(physID)
	if equippedVal <= beforeUnequip {
		t.Error("physical_power should increase after equip")
	}

	if err := s.Unequip(context.Background(), "main_hand"); err != nil {
		t.Fatal(err)
	}
	_, ok := s.Equipment().Get("main_hand")
	if ok {
		t.Error("main_hand should be empty after unequip")
	}
	if s.Inv().Get(sword) != 1 {
		t.Error("inventory should have item back after unequip")
	}
	afterUnequip := s.Attr().GetFinal(physID)
	if afterUnequip != beforeUnequip {
		t.Errorf("physical_power should return to %v (before equip), got %v", beforeUnequip, afterUnequip)
	}
}

func TestEquipReplaceSlot(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}
	staff := item.Item{ID: 33, State: 0}
	invSt.Add(sword, 1)
	invSt.Add(staff, 1)
	s.SetInv(invSt)

	s.Equip(context.Background(), sword, "main_hand")
	s.Equip(context.Background(), staff, "main_hand")

	got, ok := s.Equipment().Get("main_hand")
	if !ok || got.ID != 33 {
		t.Errorf("want staff(33) equipped, got %v", got.ID)
	}
	if s.Inv().Get(sword) != 1 {
		t.Error("sword should return to inventory on replace")
	}
	if s.Inv().Get(staff) != 0 {
		t.Error("staff should be consumed")
	}
}

func TestEquipInventoryDiffReason(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(inventory.Provider)
	reg.Register(attribute.Provider)
	reg.Register(equipment.Provider)

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}  // wooden_sword
	boots := item.Item{ID: 15, State: 0}  // leather_boots
	invSt.Add(sword, 1)
	invSt.Add(boots, 1)

	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)

	locked, _ := mgr.LockSession(s.UserID)
	defer mgr.UnlockSession(locked)

	rec := mgr.NewRecorder()
	locked.SetRecorder(rec)
	rec.PushNamespace("action")

	// Equip sword to main_hand (EQUIP), equip boots to feet (EQUIP).
	locked.Equip(context.Background(), sword, "main_hand")
	locked.Equip(context.Background(), boots, "feet")

	rec.PopNamespace()
	locked.ClearRecorder()

	diff, _ := reg.BuildDiff(rec)
	if len(diff.Inventory) != 2 {
		t.Fatalf("want 2 changes, got %d", len(diff.Inventory))
	}
	for _, c := range diff.Inventory {
		if c.Reason != pb.InventoryChangeReason_EQUIP {
			t.Errorf("item %d: want EQUIP, got %v", c.ItemId, c.Reason)
		}
	}
	if len(diff.Equipment) != 2 {
		t.Fatalf("expected 2 equipment diffs, got %d", len(diff.Equipment))
	}
	for _, ed := range diff.Equipment {
		if ed.Action != pb.EquipAction_EQUIP_ACTION_EQUIP {
			t.Errorf("expected EQUIP action, got %v", ed.Action)
		}
	}
}

func TestEquipNonEquipment(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	logs := item.Item{ID: 19, State: 0} // oak_logs
	invSt.Add(logs, 1)
	s.SetInv(invSt)

	err := s.Equip(context.Background(), logs, "main_hand")
	if err == nil {
		t.Fatal("expected error equipping non-equipment item")
	}
}

func TestEquipNotInInventory(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	s.SetInv(invSt)

	sword := item.Item{ID: 35, State: 0}
	err := s.Equip(context.Background(), sword, "main_hand")
	if err == nil {
		t.Fatal("expected error equipping item not in inventory")
	}
}

func TestUnequipEmptySlot(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	err := s.Unequip(context.Background(), "main_hand")
	if err == nil {
		t.Fatal("expected error unequipping empty slot")
	}
}

func TestEquipMultipleSlots(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}       // main_hand
	helmet := item.Item{ID: 17, State: 0}       // head
	breastplate := item.Item{ID: 16, State: 0}  // chest
	legarmor := item.Item{ID: 18, State: 0}     // leg
	boots := item.Item{ID: 15, State: 0}        // feet
	for _, it := range []item.Item{sword, helmet, breastplate, legarmor, boots} {
		invSt.Add(it, 1)
	}
	s.SetInv(invSt)

	slots := map[string]item.Item{
		"main_hand": sword,
		"head":      helmet,
		"chest":     breastplate,
		"leg":       legarmor,
		"feet":      boots,
	}
	for slot, it := range slots {
		if err := s.Equip(context.Background(), it, slot); err != nil {
			t.Fatalf("equip %s: %v", slot, err)
		}
	}

	// All slots occupied.
	for slot := range slots {
		if _, ok := s.Equipment().Get(slot); !ok {
			t.Errorf("slot %s should be occupied", slot)
		}
	}
	// Inventory empty.
	for _, it := range slots {
		if s.Inv().Get(it) != 0 {
			t.Errorf("item %v should have count 0", it.ID)
		}
	}
	// Check modifiers from each equipment.
	physID, _ := attribute.Get().AttrID("physical_power")
	if s.Attr().GetFinal(physID) < 20 {
		t.Error("physical_power should increase from equipment modifiers")
	}
}

func TestEquipUnequipDiffReason(t *testing.T) {
	// Separate namespace for unequip to verify UNEQUIP reason independently.
	reg := record.NewRegistry()
	reg.Register(inventory.Provider)
	reg.Register(attribute.Provider)
	reg.Register(equipment.Provider)

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}
	invSt.Add(sword, 1)

	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)

	locked, _ := mgr.LockSession(s.UserID)
	defer mgr.UnlockSession(locked)

	// First: equip in its own namespace.
	rec1 := mgr.NewRecorder()
	locked.SetRecorder(rec1)
	rec1.PushNamespace("equip")
	locked.Equip(context.Background(), sword, "main_hand")
	rec1.PopNamespace()
	locked.ClearRecorder()

	// Second: unequip in a separate namespace.
	rec2 := mgr.NewRecorder()
	locked.SetRecorder(rec2)
	rec2.PushNamespace("unequip")
	locked.Unequip(context.Background(), "main_hand")
	rec2.PopNamespace()
	locked.ClearRecorder()

	diff1, _ := reg.BuildDiff(rec1)
	if len(diff1.Inventory) != 1 || diff1.Inventory[0].Reason != pb.InventoryChangeReason_EQUIP {
		t.Error("equip should produce EQUIP reason")
	}
	if len(diff1.Equipment) != 1 {
		t.Fatalf("expected 1 equipment diff for equip tick, got %d", len(diff1.Equipment))
	}
	if diff1.Equipment[0].Action != pb.EquipAction_EQUIP_ACTION_EQUIP {
		t.Errorf("expected EQUIP action in equip tick, got %v", diff1.Equipment[0].Action)
	}

	diff2, _ := reg.BuildDiff(rec2)
	if len(diff2.Inventory) != 1 || diff2.Inventory[0].Reason != pb.InventoryChangeReason_UNEQUIP {
		t.Error("unequip should produce UNEQUIP reason")
	}
	if len(diff2.Equipment) != 1 {
		t.Fatalf("expected 1 equipment diff for unequip tick, got %d", len(diff2.Equipment))
	}
	if diff2.Equipment[0].Action != pb.EquipAction_EQUIP_ACTION_UNEQUIP {
		t.Errorf("expected UNEQUIP action in unequip tick, got %v", diff2.Equipment[0].Action)
	}
}

func TestEquipRepeatedSlot(t *testing.T) {
	// Equipping to same slot twice should return old item and show both in diff.
	reg := record.NewRegistry()
	reg.Register(inventory.Provider)
	reg.Register(attribute.Provider)

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}
	staff := item.Item{ID: 33, State: 0}
	invSt.Add(sword, 1)
	invSt.Add(staff, 1)

	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)

	locked, _ := mgr.LockSession(s.UserID)
	defer mgr.UnlockSession(locked)

	rec := mgr.NewRecorder()
	locked.SetRecorder(rec)
	rec.PushNamespace("action")
	locked.Equip(context.Background(), sword, "main_hand")
	locked.Equip(context.Background(), staff, "main_hand")
	rec.PopNamespace()
	locked.ClearRecorder()

	diff, _ := reg.BuildDiff(rec)
	// sword: EQUIP (-1), then returned (+1) = net 0 → dropped
	// staff: EQUIP (-1)
	if len(diff.Inventory) != 1 {
		t.Fatalf("want 1 net change (staff equip), got %d", len(diff.Inventory))
	}
	if diff.Inventory[0].ItemId != 33 {
		t.Errorf("want staff(33), got %d", diff.Inventory[0].ItemId)
	}
	if diff.Inventory[0].Reason != pb.InventoryChangeReason_EQUIP {
		t.Errorf("want EQUIP, got %v", diff.Inventory[0].Reason)
	}
}

func TestEquipPersistsAcrossTicks(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	reg.Register(inventory.Provider)

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}
	invSt.Add(sword, 1)

	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)

	locked, _ := mgr.LockSession(s.UserID)
	defer mgr.UnlockSession(locked)

	physID, _ := attribute.Get().AttrID("physical_power")
	valBefore := locked.Attr().GetFinal(physID)

	// Tick 1: equip sword.
	rec1 := mgr.NewRecorder()
	locked.SetRecorder(rec1)
	rec1.PushNamespace("tick_1")
	locked.Equip(context.Background(), sword, "main_hand")
	rec1.PopNamespace()
	locked.ClearRecorder()

	valAfterEquip := locked.Attr().GetFinal(physID)
	if valAfterEquip <= valBefore {
		t.Errorf("modifier should increase after equip: %v → %v", valBefore, valAfterEquip)
	}

	// Tick 2: idle — no equipment changes.
	rec2 := mgr.NewRecorder()
	locked.SetRecorder(rec2)
	rec2.PushNamespace("tick_2")
	rec2.PopNamespace()
	locked.ClearRecorder()

	if locked.Attr().GetFinal(physID) != valAfterEquip {
		t.Error("modifiers should persist across idle ticks")
	}
	diff2, _ := reg.BuildDiff(rec2)
	if len(diff2.Inventory) != 0 {
		t.Errorf("idle tick should have no inventory changes, got %d", len(diff2.Inventory))
	}
}

func TestEquipPersistsAcrossSessions(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	reg.Register(inventory.Provider)

	db, q := openInvDB(t)
	// Create equipment table and insert a record to simulate a prior session.
	db.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS player_equipment (user_id INTEGER NOT NULL, slot TEXT NOT NULL, item_id INTEGER NOT NULL, item_state INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (user_id, slot))`)
	db.ExecContext(context.Background(), `INSERT OR REPLACE INTO player_equipment (user_id, slot, item_id, item_state) VALUES (1, 'main_hand', 35, 0)`)

	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}
	invSt.Add(sword, 1)
	invSt.Flush(context.Background(), q)

	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)
	// Simulate reconnect: reload equipment from DB.
	equipSt, err := equipment.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	s.SetEquipment(equipSt)

	locked, ok := mgr.LockSession(s.UserID)
	if !ok {
		t.Fatal("LockSession failed")
	}
	defer mgr.UnlockSession(locked)

	got, ok := locked.Equipment().Get("main_hand")
	if !ok || got.ID != 35 {
		t.Fatalf("main_hand should have sword(35) after reconnect, got %v", got)
	}

	physID, _ := attribute.Get().AttrID("physical_power")
	val := locked.Attr().GetFinal(physID)
	def, _ := attribute.Get().Def(physID)
	if val <= def.DefaultValue {
		t.Errorf("physical_power should have equipment modifier: want > %v, got %v", def.DefaultValue, val)
	}
}
