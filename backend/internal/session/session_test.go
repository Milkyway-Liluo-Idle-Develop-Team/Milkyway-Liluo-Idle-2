package session_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/google/uuid"
)

func init() {
	if err := attribute.Load(); err != nil {
		panic(err)
	}
}

func newTestManager(t *testing.T) *session.Manager {
	t.Helper()
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	return session.NewManager(reg)
}

func TestAddRemove(t *testing.T) {
	mgr := newTestManager(t)
	id := uuid.New()

	s := session.New(id, 42, nil)
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

	_, ok = mgr.Get(id)
	if ok {
		t.Fatal("session should be gone")
	}
}

func TestGetByUser(t *testing.T) {
	mgr := newTestManager(t)

	s1 := session.New(uuid.New(), 1, nil)
	s2 := session.New(uuid.New(), 1, nil)
	s3 := session.New(uuid.New(), 2, nil)
	mgr.Add(s1)
	mgr.Add(s2)
	mgr.Add(s3)

	byUser := mgr.GetByUser(1)
	if len(byUser) != 2 {
		t.Fatalf("user 1: want 2 sessions, got %d", len(byUser))
	}

	byUser2 := mgr.GetByUser(2)
	if len(byUser2) != 1 {
		t.Fatalf("user 2: want 1 session, got %d", len(byUser2))
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
			s := session.New(id, int64(i%10), nil)
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
	// Just verify no race (run with -race).
}

func TestRecorderLifecycle(t *testing.T) {
	mgr := newTestManager(t)

	rec := mgr.NewRecorder()
	if rec == nil {
		t.Fatal("NewRecorder returned nil")
	}

	s := session.New(uuid.New(), 1, nil)
	s.SetRecorder(rec)

	if s.Recorder() != rec {
		t.Error("Recorder() should return the set recorder")
	}

	cleared := s.ClearRecorder()
	if cleared != rec {
		t.Error("ClearRecorder should return the recorder")
	}
	if s.Recorder() != nil {
		t.Error("Recorder should be nil after ClearRecorder")
	}
}

func TestAttributeInstanceOnSession(t *testing.T) {
	s := session.New(uuid.New(), 1, nil)

	if s.Attr == nil {
		t.Fatal("session.Attr is nil")
	}

	r := attribute.Get()
	id, _ := r.AttrID("physical_power")
	val := s.Attr.GetFinal(id)

	def, _ := r.Def(id)
	if val != def.DefaultValue {
		t.Errorf("want default %v, got %v", def.DefaultValue, val)
	}
}

// --- Integration tests: full execution cycle simulation ---

// TestExecutionCycle simulates a single game tick: equip an item, run a
// settlement cycle, and verify the diff packet carries attribute changes.
func TestExecutionCycle(t *testing.T) {
	mgr := newTestManager(t)
	s := session.New(uuid.New(), 1, nil)
	mgr.Add(s)
	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")
	accID, _ := r.AttrID("accuracy")

	// --- Start execution cycle ---
	rec := mgr.NewRecorder()
	s.SetRecorder(rec)

	rec.PushNamespace("event_execution")

	// Simulate: settlement reads GetFinal, which triggers markDirty
	// because modifiers changed below.
	s.Attr.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
		{AttrID: accID, Op: attribute.OpAdd, Value: 10, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})

	// Simulate: settlement reads final values (this also caches).
	pVal := s.Attr.GetFinal(physID)
	aVal := s.Attr.GetFinal(accID)

	rec.PopNamespace()
	s.ClearRecorder()

	// --- Verify computed values ---
	if pVal != 25 { // 10 default + 15
		t.Errorf("physical_power: want 25, got %v", pVal)
	}
	if aVal != 10 { // 0 default + 10
		t.Errorf("accuracy: want 10, got %v", aVal)
	}

	// --- Build diff packet ---
	diff, err := mgr.Registry().BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	// Verify diff contains attr_changes with modifiers.
	diffStr := string(diff)
	if diffStr == "{}" {
		t.Fatal("diff should not be empty after an execution cycle with changes")
	}

	// Verify the diff round-trips and contains the expected data.
	var m map[string]json.RawMessage
	json.Unmarshal(diff, &m)

	changes, ok := m["attribute_changes"]
	if !ok {
		t.Fatal("missing attribute_changes in diff")
	}

	var attrs []struct {
		AttrID     string  `json:"attr_id"`
		FinalValue float64 `json:"final_value"`
		Modifiers  []struct {
			Source string  `json:"source"`
			Op     string  `json:"op"`
			Value  float64 `json:"value"`
		} `json:"modifiers"`
	}
	json.Unmarshal(changes, &attrs)

	if len(attrs) == 0 {
		t.Fatal("no attribute changes in diff")
	}
	for _, a := range attrs {
		if a.AttrID == "physical_power" && a.FinalValue != 25 {
			t.Errorf("diff physical_power: want 25, got %v", a.FinalValue)
		}
	}
}

// TestMultipleExecutionCycles runs two ticks on the same session and
// verifies that each produces independent diffs.
func TestMultipleExecutionCycles(t *testing.T) {
	// This test uses our own registry since we need to inspect diffs
	// from multiple cycles.
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg)
	s := session.New(uuid.New(), 1, nil)
	mgr.Add(s)
	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	// --- Cycle 1: equip sword (+15) ---
	rec1 := mgr.NewRecorder()
	s.SetRecorder(rec1)
	rec1.PushNamespace("tick_1")
	s.Attr.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	rec1.PopNamespace()
	s.ClearRecorder()

	diff1, _ := reg.BuildDiff(rec1)
	if string(diff1) == "{}" {
		t.Error("cycle 1 diff should not be empty")
	}

	// --- Cycle 2: unequip sword, equip axe (+30) ---
	rec2 := mgr.NewRecorder()
	s.SetRecorder(rec2)
	rec2.PushNamespace("tick_2")
	s.Attr.UpdateModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 30, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	rec2.PopNamespace()
	s.ClearRecorder()

	diff2, _ := reg.BuildDiff(rec2)
	if string(diff2) == "{}" {
		t.Error("cycle 2 diff should not be empty")
	}

	// The final value after update should be 40 (10 + 30).
	val := s.Attr.GetFinal(physID)
	if val != 40 {
		t.Errorf("after update: want 40, got %v", val)
	}

	// Cycles should be independent — diff1 should not have been modified.
	var m1 map[string]json.RawMessage
	json.Unmarshal(diff1, &m1)
	if _, ok := m1["attribute_changes"]; !ok {
		t.Error("cycle 1 diff should still have attribute_changes")
	}
}

// TestExecutionCycleNoChanges verifies that a tick with no attribute
// modifications produces an empty diff.
func TestExecutionCycleNoChanges(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg)
	s := session.New(uuid.New(), 1, nil)
	mgr.Add(s)

	rec := mgr.NewRecorder()
	s.SetRecorder(rec)
	rec.PushNamespace("tick_idle")

	// Read some attributes but don't change anything.
	r := attribute.Get()
	id, _ := r.AttrID("physical_power")
	_ = s.Attr.GetFinal(id) // just a read, no markDirty triggered

	rec.PopNamespace()
	s.ClearRecorder()

	diff, _ := reg.BuildDiff(rec)
	if string(diff) != "{}" {
		t.Errorf("idle cycle should produce empty diff, got %s", diff)
	}
}

// TestSessionLifecycleSimulation replicates the full connect → play → disconnect flow.
func TestSessionLifecycleSimulation(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg)

	// --- Connect ---
	connID := uuid.New()
	s := session.New(connID, 42, nil)
	mgr.Add(s)

	if mgr.Count() != 1 {
		t.Fatal("session should be added")
	}

	// --- Play: equip and execute one cycle ---
	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")
	fellingID, _ := r.AttrID("felling_production_multiplier")

	rec := mgr.NewRecorder()
	s.SetRecorder(rec)
	rec.PushNamespace("event_execution")

	// Equip axe — buffs woodcutting.
	s.Attr.AddModifiers("tool:axe", []attribute.Modifier{
		{AttrID: fellingID, Op: attribute.OpAdd, Value: 0.1, Display: attribute.DisplayPercent, Source: "tool:axe"},
	})
	// Equip armor — buffs attack.
	s.Attr.AddModifiers("equipment:armor", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 5, Display: attribute.DisplayFixed, Source: "equipment:armor"},
	})

	rec.PopNamespace()
	s.ClearRecorder()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	// Verify diff has changes for both attributes.
	var m map[string]json.RawMessage
	json.Unmarshal(diff, &m)

	raw, ok := m["attribute_changes"]
	if !ok {
		t.Fatal("missing attribute_changes in diff")
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

	// --- Disconnect ---
	mgr.Remove(connID)
	if mgr.Count() != 0 {
		t.Fatal("session should be removed after disconnect")
	}
}

// TestContextInSettlement demonstrates using temporary Context during
// settlement to preview a buff effect without polluting the cache.
func TestContextInSettlement(t *testing.T) {
	s := session.New(uuid.New(), 1, nil)
	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	// Base: 10 (default) + 20 (equipment) = 30 persistent.
	s.Attr.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 20, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})

	// Settlement: simulate a one-time buff that gives +50% damage.
	ctx := attribute.NewContext()
	ctx.AddMult(physID, 0.5)

	buffed := s.Attr.GetFinalWithContext(physID, ctx)
	if buffed != 45 { // (10 + 20) * 1.5
		t.Errorf("buffed: want 45, got %v", buffed)
	}

	// After settlement: persistent value unaffected.
	normal := s.Attr.GetFinal(physID)
	if normal != 30 {
		t.Errorf("persistent after context: want 30, got %v", normal)
	}
	// Cache should not be dirty.
	if s.Attr.Dirty(physID) {
		t.Error("context should not mark persistent cache dirty")
	}
}

// TestFullSnapshotOnConnect verifies the full snapshot construction
// for the initial state push on connect.
func TestFullSnapshotOnConnect(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)

	s := session.New(uuid.New(), 1, nil)
	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	s.Attr.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	// Force compute so cache is populated.
	s.Attr.GetFinal(physID)

	data, err := reg.BuildFullSnapshot(map[string]any{
		"attribute": s.Attr,
	})
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	json.Unmarshal(data, &m)

	var attrs []struct {
		AttrID     string  `json:"attr_id"`
		FinalValue float64 `json:"final_value"`
		Modifiers  []struct {
			Source string `json:"source"`
		} `json:"modifiers"`
	}
	json.Unmarshal(m["attribute"], &attrs)

	// All 11 attributes should be present.
	if len(attrs) != r.Count() {
		t.Fatalf("full snapshot: want %d attrs, got %d", r.Count(), len(attrs))
	}

	// Find physical_power and verify its state.
	for _, a := range attrs {
		if a.AttrID == "physical_power" {
			if a.FinalValue != 25 {
				t.Errorf("full snapshot physical_power: want 25, got %v", a.FinalValue)
			}
			if len(a.Modifiers) == 0 {
				t.Error("physical_power should have modifiers attached")
			}
		}
	}
}

// TestRecorderIsCleanBetweenCycles verifies that a recorder from a
// previous cycle doesn't leak into the next.
func TestRecorderIsCleanBetweenCycles(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg)
	s := session.New(uuid.New(), 1, nil)
	mgr.Add(s)
	r := attribute.Get()
	physID, _ := r.AttrID("physical_power")

	// Cycle 1
	rec1 := mgr.NewRecorder()
	s.SetRecorder(rec1)
	rec1.PushNamespace("cycle_1")
	s.Attr.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: physID, Op: attribute.OpAdd, Value: 10, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	rec1.PopNamespace()
	s.ClearRecorder()

	// Cycle 2: should start clean, no leftover recorder state.
	rec2 := mgr.NewRecorder()
	s.SetRecorder(rec2)
	if s.Recorder() != rec2 {
		t.Error("recorder should be rec2")
	}

	// Rec2 is clean — no namespaces yet.
	diff, _ := reg.BuildDiff(rec2)
	if string(diff) != "{}" {
		t.Errorf("fresh recorder should produce empty diff, got %s", diff)
	}

	// Dirty state from cycle 1 should still be reflected in GetFinal.
	val := s.Attr.GetFinal(physID)
	if val != 20 {
		t.Errorf("dirty state should persist across cycles: want 20, got %v", val)
	}

	s.ClearRecorder()
}

