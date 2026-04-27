package gameconfig

import "testing"

func TestIDAllocation(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}

	// Item string -> numeric -> string round-trip
	itemID, ok := StringToItemID("oak_logs")
	if !ok {
		t.Fatal("StringToItemID(oak_logs) not found")
	}
	if itemID <= 0 {
		t.Fatalf("itemID should be > 0, got %d", itemID)
	}
	s, ok := ItemIDToString(itemID)
	if !ok || s != "oak_logs" {
		t.Fatalf("ItemIDToString(%d) = %q, want oak_logs", itemID, s)
	}

	// Event string -> numeric -> string round-trip
	eventID, ok := StringToEventID("felling_oak_tree")
	if !ok {
		t.Fatal("StringToEventID(felling_oak_tree) not found")
	}
	s, ok = EventIDToString(eventID)
	if !ok || s != "felling_oak_tree" {
		t.Fatalf("EventIDToString(%d) = %q, want felling_oak_tree", eventID, s)
	}

	// Skill
	skillID, ok := StringToSkillID("crafting")
	if !ok {
		t.Fatal("StringToSkillID(crafting) not found")
	}
	s, ok = SkillIDToString(skillID)
	if !ok || s != "crafting" {
		t.Fatalf("SkillIDToString(%d) = %q, want crafting", skillID, s)
	}

	// Map
	mapID, ok := StringToMapID("village")
	if !ok {
		t.Fatal("StringToMapID(village) not found")
	}
	s, ok = MapIDToString(mapID)
	if !ok || s != "village" {
		t.Fatalf("MapIDToString(%d) = %q, want village", mapID, s)
	}

	// Battle skill
	bsID, ok := StringToBattleSkillID("basic_attack")
	if !ok {
		t.Fatal("StringToBattleSkillID(basic_attack) not found")
	}
	s, ok = BattleSkillIDToString(bsID)
	if !ok || s != "basic_attack" {
		t.Fatalf("BattleSkillIDToString(%d) = %q, want basic_attack", bsID, s)
	}

	// Numeric accessors
	it, ok := GetItemByID(itemID)
	if !ok || it.ID != "oak_logs" {
		t.Fatal("GetItemByID failed")
	}
	ev, ok := GetEventByID(eventID)
	if !ok || ev.ID != "felling_oak_tree" {
		t.Fatal("GetEventByID failed")
	}

	// Invalid lookups
	if _, ok := StringToItemID("nonexistent"); ok {
		t.Error("StringToItemID(nonexistent) should not be found")
	}
	if _, ok := ItemIDToString(ItemID(99999)); ok {
		t.Error("ItemIDToString(99999) should not be found")
	}
}

func TestIDCounts(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}

	if ItemCount() != 40 { // 37 items + 3 fluids (heat, stone_fluid, iron_fluid)
		t.Errorf("ItemCount = %d, want 40", ItemCount())
	}
	if EventCount() != 61 {
		t.Errorf("EventCount = %d, want 61", EventCount())
	}
	if SkillCount() != 8 { // crafting, enhancing, felling, forging, magic, mining, none, strength
		t.Errorf("SkillCount = %d, want 8", SkillCount())
	}
	if MapCount() != 1 { // village
		t.Errorf("MapCount = %d, want 1", MapCount())
	}
	if BattleSkillCount() != 1 { // basic_attack
		t.Errorf("BattleSkillCount = %d, want 1", BattleSkillCount())
	}

	// All IDs should be dense (1..N)
	itemIDs := AllItemIDs()
	if len(itemIDs) != ItemCount() {
		t.Errorf("AllItemIDs len = %d, want %d", len(itemIDs), ItemCount())
	}
	for i, id := range itemIDs {
		if int(id) != i+1 {
			t.Errorf("item id at index %d = %d, expected %d", i, id, i+1)
		}
	}
}

func TestNumericIndexedHelpers(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}

	skillID, _ := StringToSkillID("felling")
	events := EventsBySkillID(skillID)
	if len(events) == 0 {
		t.Error("EventsBySkillID(felling) should not be empty")
	}

	mapID, _ := StringToMapID("village")
	events = EventsByMapID(mapID)
	if len(events) != EventCount() {
		t.Errorf("EventsByMapID(village) = %d, want %d", len(events), EventCount())
	}
}

func TestIDStringer(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}

	itemID, _ := StringToItemID("oak_logs")
	if itemID.String() != "oak_logs" {
		t.Errorf("ItemID.String() = %q, want oak_logs", itemID.String())
	}

	if ItemID(0).String() != "<invalid-item>" {
		t.Errorf("ItemID(0).String() = %q", ItemID(0).String())
	}
	if ItemID(99999).String() != "<item-99999>" {
		t.Errorf("ItemID(99999).String() = %q", ItemID(99999).String())
	}
}
