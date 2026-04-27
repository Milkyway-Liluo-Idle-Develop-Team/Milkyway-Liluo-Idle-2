package gameconfig

import (
	"testing"
)

func TestLoadAndAccessors(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}

	if ItemCount() != 37 {
		t.Errorf("ItemCount() = %d, want 37", ItemCount())
	}
	if EventCount() != 61 {
		t.Errorf("EventCount() = %d, want 61", EventCount())
	}

	// Item lookup
	it, ok := GetItem("oak_logs")
	if !ok {
		t.Fatal("GetItem(oak_logs) not found")
	}
	if it.Name != "橡木原木" {
		t.Errorf("oak_logs name = %q, want 橡木原木", it.Name)
	}
	if it.Classification != "resources" {
		t.Errorf("oak_logs classification = %q, want resources", it.Classification)
	}

	// Equipment item with nested data
	sword, ok := GetItem("wooden_sword")
	if !ok {
		t.Fatal("GetItem(wooden_sword) not found")
	}
	if !sword.Equipment {
		t.Error("wooden_sword should be equipment")
	}
	if sword.EquipmentDetails == nil {
		t.Fatal("wooden_sword missing equipment_details")
	}
	if len(sword.EquipmentDetails.BattleSkills) == 0 {
		t.Error("wooden_sword should have battle_skills")
	}

	// Event lookup
	ev, ok := GetEvent("felling_oak_tree")
	if !ok {
		t.Fatal("GetEvent(felling_oak_tree) not found")
	}
	if ev.Type != EventTypeLoop {
		t.Errorf("felling_oak_tree type = %q, want loop", ev.Type)
	}
	if ev.LoopTime == nil || *ev.LoopTime != 2 {
		t.Errorf("felling_oak_tree loop_time = %v, want 2", ev.LoopTime)
	}
	if ev.NeedSkill != "felling" {
		t.Errorf("felling_oak_tree need_skill = %q, want felling", ev.NeedSkill)
	}

	// Index: by skill
	fellingEvents := EventsBySkill("felling")
	if len(fellingEvents) == 0 {
		t.Error("EventsBySkill(felling) should not be empty")
	}

	// Index: by map
	villageEvents := EventsByMap("village")
	if len(villageEvents) != EventCount() {
		t.Errorf("EventsByMap(village) = %d, want %d", len(villageEvents), EventCount())
	}

	// Index: by classification
	resources := ItemsByClassification("resources")
	if len(resources) == 0 {
		t.Error("ItemsByClassification(resources) should not be empty")
	}

	// Event with fluid requirement
	condensing, ok := GetEvent("condensing_black_stone")
	if !ok {
		t.Fatal("GetEvent(condensing_black_stone) not found")
	}
	if condensing.Type != EventTypeLoop {
		t.Errorf("condensing_black_stone type = %q, want loop", condensing.Type)
	}

	// Loop vs upgrade split
	loops := LoopEvents()
	upgrades := UpgradeEvents()
	if len(loops)+len(upgrades) != EventCount() {
		t.Errorf("loop(%d)+upgrade(%d) = %d, want %d", len(loops), len(upgrades), len(loops)+len(upgrades), EventCount())
	}

	// Reward format
	makingPlank, ok := GetEvent("making_oak_plank")
	if !ok {
		t.Fatal("GetEvent(making_oak_plank) not found")
	}
	if len(makingPlank.Rewards) != 2 {
		t.Fatalf("making_oak_plank rewards count = %d, want 2", len(makingPlank.Rewards))
	}
	// First reward should be experience
	if !makingPlank.Rewards[0].IsExperience() {
		t.Error("making_oak_plank first reward should be experience")
	}
	if makingPlank.Rewards[0].Value != 20.0 {
		t.Errorf("making_oak_plank XP value = %v, want 20", makingPlank.Rewards[0].Value)
	}
	// Second reward should be item
	if !makingPlank.Rewards[1].IsItem() {
		t.Error("making_oak_plank second reward should be item")
	}
	if makingPlank.Rewards[1].ItemQuantity() != 4 {
		t.Errorf("making_oak_plank item qty = %v, want 4", makingPlank.Rewards[1].ItemQuantity())
	}
}

func TestRequirementConsumption(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}

	// felling_oak_tree requirements:
	// - skill felling >= 1 (threshold)
	// - event starting_dialog_5 (threshold)
	ev, _ := GetEvent("felling_oak_tree")
	if len(ev.Requirements) != 2 {
		t.Fatalf("felling_oak_tree requirements count = %d, want 2", len(ev.Requirements))
	}
	if ev.Requirements[0].IsConsumption() {
		t.Error("skill requirement should not be consumption")
	}
	if ev.Requirements[1].IsConsumption() {
		t.Error("event requirement should not be consumption")
	}

	// making_oak_plank requirements:
	// - skill crafting >= 1 (threshold)
	// - item oak_logs x1 (consumption, no comparison_types)
	craft, _ := GetEvent("making_oak_plank")
	if len(craft.Requirements) != 2 {
		t.Fatalf("making_oak_plank requirements count = %d, want 2", len(craft.Requirements))
	}
	if craft.Requirements[0].IsConsumption() {
		t.Error("skill requirement should not be consumption")
	}
	if !craft.Requirements[1].IsConsumption() {
		t.Error("item requirement without comparison_types should be consumption")
	}
}
