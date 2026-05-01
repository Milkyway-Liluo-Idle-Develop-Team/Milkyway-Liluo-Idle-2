package record_test

import (
	"testing"

	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// --- Mock inventory system ---

type invBucket struct {
	items []*pb.InventoryDiff
}

func newInvBucket() *invBucket { return &invBucket{} }

func (b *invBucket) SystemName() string { return "inventory" }

func (b *invBucket) Add(itemID, itemState int32, qty float64) {
	for _, e := range b.items {
		if e.ItemId == itemID && e.ItemState == itemState {
			e.QuantityDelta += qty
			return
		}
	}
	b.items = append(b.items, &pb.InventoryDiff{ItemId: itemID, ItemState: itemState, QuantityDelta: qty})
}

func (b *invBucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*invBucket)
	for _, e := range ob.items {
		b.Add(e.ItemId, e.ItemState, e.QuantityDelta)
	}
}

func (b *invBucket) SerializeDiff() (proto.Message, error) {
	if len(b.items) == 0 {
		return nil, nil
	}
	return &pb.StateDiff{Inventory: b.items}, nil
}

func (b *invBucket) IsEmpty() bool { return len(b.items) == 0 }

type invProvider struct{}

func (p invProvider) SystemName() string              { return "inventory" }
func (p invProvider) NewBucket() record.RecordBucket  { return newInvBucket() }
func (p invProvider) SerializeFull(state any) (proto.Message, error) {
	return &pb.StateFull{}, nil
}

// --- Mock skill_xp system ---

type skillBucket struct {
	items []*pb.SkillXPDiff
}

func newSkillBucket() *skillBucket { return &skillBucket{} }

func (b *skillBucket) SystemName() string { return "skill_xp" }

func (b *skillBucket) Add(skillID int64, xpDelta, newLevel float64) {
	for _, e := range b.items {
		if e.SkillId == skillID {
			e.XpDelta += xpDelta
			e.NewLevel = newLevel
			return
		}
	}
	b.items = append(b.items, &pb.SkillXPDiff{SkillId: skillID, XpDelta: xpDelta, NewLevel: newLevel})
}

func (b *skillBucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*skillBucket)
	for _, e := range ob.items {
		b.Add(e.SkillId, e.XpDelta, e.NewLevel)
	}
}

func (b *skillBucket) SerializeDiff() (proto.Message, error) {
	if len(b.items) == 0 {
		return nil, nil
	}
	return &pb.StateDiff{SkillXp: b.items}, nil
}

func (b *skillBucket) IsEmpty() bool { return len(b.items) == 0 }

type skillProvider struct{}

func (p skillProvider) SystemName() string              { return "skill_xp" }
func (p skillProvider) NewBucket() record.RecordBucket  { return newSkillBucket() }
func (p skillProvider) SerializeFull(state any) (proto.Message, error) {
	return &pb.StateFull{}, nil
}

// --- Tests ---

func TestSingleNamespace(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(skillProvider{})

	rec := record.NewRecorder(reg)
	rec.PushNamespace("event_execution")

	invB := record.Get[*invBucket](rec)
	invB.Add(1, 0, 5)
	invB.Add(1, 0, 3) // same identity → merged
	invB.Add(2, 0, -2)

	skillB := record.Get[*skillBucket](rec)
	skillB.Add(3, 20, 4)

	rec.PopNamespace()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	if len(diff.Inventory) == 0 {
		t.Error("missing inventory")
	}
	if len(diff.SkillXp) == 0 {
		t.Error("missing skill_xp")
	}

	// ItemID=1 should be merged to QtyDelta=8.
	for _, r := range diff.Inventory {
		if r.ItemId == 1 && r.QuantityDelta != 8 {
			t.Errorf("item 1: want 8, got %v", r.QuantityDelta)
		}
	}
}

func TestNestedNamespaces(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(skillProvider{})

	rec := record.NewRecorder(reg)

	rec.PushNamespace("outer")
	record.Get[*invBucket](rec).Add(1, 0, 10)

	rec.PushNamespace("inner")
	record.Get[*invBucket](rec).Add(1, 0, 5)
	record.Get[*skillBucket](rec).Add(3, 50, 5)
	rec.PopNamespace()

	rec.PopNamespace()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	if len(diff.Inventory) != 1 {
		t.Fatalf("expected 1 merged inventory entry, got %d", len(diff.Inventory))
	}
	if diff.Inventory[0].QuantityDelta != 15 {
		t.Errorf("want 15, got %v", diff.Inventory[0].QuantityDelta)
	}
}

func TestNoActiveNamespace(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})

	rec := record.NewRecorder(reg)
	if b := rec.Bucket("inventory"); b != nil {
		t.Error("expected nil bucket when no namespace is active")
	}

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}
	if !isStateDiffEmpty(diff) {
		t.Error("expected empty diff")
	}
}

func TestFullSnapshot(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(skillProvider{})

	data, err := reg.BuildFullSnapshot(map[string]any{
		"inventory": struct{}{},
		"skill_xp":  struct{}{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if data == nil {
		t.Fatal("expected non-nil snapshot")
	}
}

func TestEmptyBucketNotSerialized(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(invProvider{})
	reg.Register(skillProvider{})

	rec := record.NewRecorder(reg)
	rec.PushNamespace("test")
	record.Get[*invBucket](rec).Add(1, 0, 1)
	rec.PopNamespace()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.SkillXp) != 0 {
		t.Error("empty skill_xp bucket should not appear in diff")
	}
	if len(diff.Inventory) == 0 {
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

func isStateDiffEmpty(d *pb.StateDiff) bool {
	return len(d.Inventory) == 0 &&
		len(d.Attribute) == 0 &&
		len(d.SkillXp) == 0 &&
		len(d.Bestiary) == 0 &&
		len(d.EventExecution) == 0 &&
		len(d.EventQueue) == 0
}
