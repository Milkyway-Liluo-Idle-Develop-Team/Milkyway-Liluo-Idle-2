package record_test

import (
	"encoding/json"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// --- Mock inventory system ---

type invBucket struct {
	// keyed by {ItemID, ItemState}
	items map[invKey]*invEntry
}

type invKey struct {
	ItemID    int64
	ItemState int32
}

type invEntry struct {
	QtyDelta  int
	FracDelta float64
}

func newInvBucket() *invBucket {
	return &invBucket{items: make(map[invKey]*invEntry)}
}

func (b *invBucket) SystemName() string { return "inventory" }

func (b *invBucket) Add(itemID int64, itemState int32, qty int, frac float64) {
	k := invKey{itemID, itemState}
	if e, ok := b.items[k]; ok {
		e.QtyDelta += qty
		e.FracDelta += frac
	} else {
		b.items[k] = &invEntry{QtyDelta: qty, FracDelta: frac}
	}
}

func (b *invBucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*invBucket)
	for k, e := range ob.items {
		if existing, ok := b.items[k]; ok {
			existing.QtyDelta += e.QtyDelta
			existing.FracDelta += e.FracDelta
		} else {
			b.items[k] = &invEntry{QtyDelta: e.QtyDelta, FracDelta: e.FracDelta}
		}
	}
}

func (b *invBucket) SerializeDiff() (json.RawMessage, error) {
	type wire struct {
		ItemID    int64   `json:"item_id"`
		ItemState int32   `json:"item_state"`
		QtyDelta  int     `json:"quantity_delta"`
		FracDelta float64 `json:"fraction_delta,omitempty"`
	}
	out := make([]wire, 0, len(b.items))
	for k, e := range b.items {
		out = append(out, wire{k.ItemID, k.ItemState, e.QtyDelta, e.FracDelta})
	}
	return json.Marshal(out)
}

func (b *invBucket) IsEmpty() bool { return len(b.items) == 0 }

type invProvider struct{}

func (p invProvider) SystemName() string     { return "inventory" }
func (p invProvider) NewBucket() record.RecordBucket { return newInvBucket() }
func (p invProvider) SerializeFull(state any) (json.RawMessage, error) {
	return json.Marshal(state)
}

// --- Mock skill_xp system ---

type skillBucket struct {
	items map[int64]*skillEntry // skillID -> entry
}

type skillEntry struct {
	XpDelta  float64
	NewLevel float64
}

func newSkillBucket() *skillBucket {
	return &skillBucket{items: make(map[int64]*skillEntry)}
}

func (b *skillBucket) SystemName() string { return "skill_xp" }

func (b *skillBucket) Add(skillID int64, xp float64, newLevel float64) {
	if e, ok := b.items[skillID]; ok {
		e.XpDelta += xp
		e.NewLevel = newLevel
	} else {
		b.items[skillID] = &skillEntry{XpDelta: xp, NewLevel: newLevel}
	}
}

func (b *skillBucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*skillBucket)
	for k, e := range ob.items {
		if existing, ok := b.items[k]; ok {
			existing.XpDelta += e.XpDelta
			existing.NewLevel = e.NewLevel
		} else {
			b.items[k] = &skillEntry{XpDelta: e.XpDelta, NewLevel: e.NewLevel}
		}
	}
}

func (b *skillBucket) SerializeDiff() (json.RawMessage, error) {
	type wire struct {
		SkillID  int64   `json:"skill_id"`
		XpDelta  float64 `json:"xp_delta"`
		NewLevel float64 `json:"new_level"`
	}
	out := make([]wire, 0, len(b.items))
	for k, e := range b.items {
		out = append(out, wire{k, e.XpDelta, e.NewLevel})
	}
	return json.Marshal(out)
}

func (b *skillBucket) IsEmpty() bool { return len(b.items) == 0 }

type skillProvider struct{}

func (p skillProvider) SystemName() string     { return "skill_xp" }
func (p skillProvider) NewBucket() record.RecordBucket { return newSkillBucket() }
func (p skillProvider) SerializeFull(state any) (json.RawMessage, error) {
	return json.Marshal(state)
}

// --- Tests ---

func TestSingleNamespace(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(skillProvider{})

	rec := record.NewRecorder(reg)
	rec.PushNamespace("event_execution")

	invB := record.Get[*invBucket](rec)
	invB.Add(1, 0, 5, 0)
	invB.Add(1, 0, 3, 0) // same identity → auto-deduped inside bucket
	invB.Add(2, 0, -2, 0)

	skillB := record.Get[*skillBucket](rec)
	skillB.Add(3, 20, 4)

	rec.PopNamespace()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(diff, &m); err != nil {
		t.Fatal(err)
	}

	// Both systems should appear.
	if _, ok := m["inventory_changes"]; !ok {
		t.Error("missing inventory_changes")
	}
	if _, ok := m["skill_xp_changes"]; !ok {
		t.Error("missing skill_xp_changes")
	}

	// ItemID=1 should be merged to QtyDelta=8.
	var inv []struct {
		ItemID   int64 `json:"item_id"`
		QtyDelta int   `json:"quantity_delta"`
	}
	json.Unmarshal(m["inventory_changes"], &inv)
	if len(inv) != 2 {
		t.Fatalf("expected 2 inventory entries, got %d", len(inv))
	}
	for _, r := range inv {
		if r.ItemID == 1 && r.QtyDelta != 8 {
			t.Errorf("item 1: want 8, got %d", r.QtyDelta)
		}
	}
}

func TestNestedNamespaces(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(skillProvider{})

	rec := record.NewRecorder(reg)

	rec.PushNamespace("outer")
	record.Get[*invBucket](rec).Add(1, 0, 10, 0)

	rec.PushNamespace("inner")
	record.Get[*invBucket](rec).Add(1, 0, 5, 0)
	record.Get[*skillBucket](rec).Add(3, 50, 5)
	rec.PopNamespace()

	rec.PopNamespace()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	json.Unmarshal(diff, &m)

	// ItemID=1 across two namespaces → merged to 15
	var inv []struct {
		ItemID   int64 `json:"item_id"`
		QtyDelta int   `json:"quantity_delta"`
	}
	json.Unmarshal(m["inventory_changes"], &inv)
	if len(inv) != 1 {
		t.Fatalf("expected 1 merged entry, got %d", len(inv))
	}
	if inv[0].QtyDelta != 15 {
		t.Errorf("want 15, got %d", inv[0].QtyDelta)
	}
}

func TestNoActiveNamespace(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})

	rec := record.NewRecorder(reg)

	// Bucket with no active namespace returns nil.
	if b := rec.Bucket("inventory"); b != nil {
		t.Error("expected nil bucket when no namespace is active")
	}

	// BuildDiff on empty recorder returns {}.
	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}
	if string(diff) != "{}" {
		t.Errorf("expected {}, got %s", diff)
	}
}

func TestFullSnapshot(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(skillProvider{})

	states := map[string]any{
		"inventory": map[string]int{"oak_logs": 42},
		"skill_xp":  map[string]float64{"felling": 4.0},
	}

	data, err := reg.BuildFullSnapshot(states)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	json.Unmarshal(data, &m)
	if _, ok := m["inventory"]; !ok {
		t.Error("missing inventory")
	}
	if _, ok := m["skill_xp"]; !ok {
		t.Error("missing skill_xp")
	}
}

func TestEmptyBucketNotSerialized(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(skillProvider{})

	rec := record.NewRecorder(reg)
	rec.PushNamespace("test")
	// Only write to inventory, leave skill_xp bucket empty.
	record.Get[*invBucket](rec).Add(1, 0, 1, 0)
	rec.PopNamespace()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	json.Unmarshal(diff, &m)
	if _, ok := m["skill_xp_changes"]; ok {
		t.Error("empty skill_xp bucket should not appear in diff")
	}
	if _, ok := m["inventory_changes"]; !ok {
		t.Error("non-empty inventory should appear")
	}
}

func TestDuplicateRegisterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(invProvider{})
}
