package item_test

import (
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/item"
)

func init() {
	if err := gameconfig.Load(); err != nil {
		panic("gameconfig.Load: " + err.Error())
	}
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic("attribute.Load: " + err.Error())
		}
	}
}

func TestItemDefLookup(t *testing.T) {
	it, ok := gameconfig.GetItemDef("oak_logs")
	if !ok {
		t.Fatal("oak_logs not found")
	}
	if it.Name() != "橡木原木" {
		t.Errorf("unexpected name: %q", it.Name())
	}
	if it.Classification() != "resources" {
		t.Errorf("unexpected class: %q", it.Classification())
	}
	if it.ID() == 0 {
		t.Error("numeric id should not be zero")
	}
}

func TestItemDefMethods(t *testing.T) {
	it, ok := gameconfig.GetItemDef("wooden_sword")
	if !ok {
		t.Fatal("wooden_sword not found")
	}
	if !it.IsEquipment() {
		t.Error("wooden_sword should be equipment")
	}
	if it.IsTool() {
		t.Error("wooden_sword should not be a tool")
	}
	if !it.IsUpgradable() {
		t.Error("wooden_sword should be upgradable")
	}
	if it.IsFluid() {
		t.Error("wooden_sword should not be fluid")
	}
}

func TestItemDefsByClassification(t *testing.T) {
	equips := gameconfig.ItemDefsByClassification("equipment")
	if len(equips) == 0 {
		t.Fatal("no equipment items found")
	}
	for _, it := range equips {
		if it.Classification() != "equipment" {
			t.Errorf("item %q has wrong class %q", it.StringID(), it.Classification())
		}
	}
}

func TestModifiersEquipment(t *testing.T) {
	it, ok := gameconfig.GetItemDef("wooden_sword")
	if !ok {
		t.Fatal("wooden_sword not found")
	}

	reg := attribute.Get()
	mods, err := it.Modifiers(item.Item{ID: it.ID()}, reg)
	if err != nil {
		t.Fatal(err)
	}

	if len(mods) != 3 {
		t.Fatalf("wooden_sword: want 3 modifiers, got %d", len(mods))
	}

	sources := map[string]int{}
	for _, m := range mods {
		sources[m.Source]++
	}
	if sources["equipment_basic:wooden_sword"] != 1 {
		t.Error("missing equipment_basic source")
	}
	if sources["equipment_upgrade:wooden_sword"] != 2 {
		t.Error("wrong count for equipment_upgrade source")
	}
}

func TestModifiersTool(t *testing.T) {
	it, ok := gameconfig.GetItemDef("wooden_axe")
	if !ok {
		t.Fatal("wooden_axe not found")
	}

	reg := attribute.Get()
	mods, err := it.Modifiers(item.Item{ID: it.ID()}, reg)
	if err != nil {
		t.Fatal(err)
	}

	if len(mods) != 1 {
		t.Fatalf("wooden_axe: want 1 modifier, got %d", len(mods))
	}
	m := mods[0]
	if m.Source != "tool_upgrade:wooden_axe" {
		t.Errorf("want tool_upgrade source, got %q", m.Source)
	}
	if m.Value != 0.1 {
		t.Errorf("want 0.1, got %v", m.Value)
	}
}

func TestModifiersNoDetails(t *testing.T) {
	it, ok := gameconfig.GetItemDef("oak_logs")
	if !ok {
		t.Fatal("oak_logs not found")
	}

	reg := attribute.Get()
	mods, err := it.Modifiers(item.Item{ID: it.ID()}, reg)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 0 {
		t.Errorf("oak_logs should have no modifiers, got %d", len(mods))
	}
}

func TestKey(t *testing.T) {
	k := item.Item{ID: 1}
	if k.State != 0 {
		t.Error("zero-value State should be 0")
	}

	k2 := item.Item{ID: 1, State: 5}
	if k2.State != 5 {
		t.Error("Key should preserve explicit State")
	}
}
