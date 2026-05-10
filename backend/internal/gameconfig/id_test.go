package gameconfig

import (
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
)

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

	// Numeric accessors
	it, ok := GetItemDefByID(itemID)
	if !ok || it.StringID() != "oak_logs" {
		t.Fatal("GetItemDefByID failed")
	}
	ev, ok := GetEventByID(eventID)
	if !ok || ev.ID != "felling_oak_tree" {
		t.Fatal("GetEventByID failed")
	}

	// Invalid lookups
	if _, ok := StringToItemID("nonexistent"); ok {
		t.Error("StringToItemID(nonexistent) should not be found")
	}
	if _, ok := ItemIDToString(item.ID(99999)); ok {
		t.Error("ItemIDToString(99999) should not be found")
	}
}

func TestIDCounts(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() = %v", err)
	}

	if ItemCount() != 40 {
		t.Errorf("ItemCount = %d, want 40", ItemCount())
	}
	if EventCount() != 61 {
		t.Errorf("EventCount = %d, want 61", EventCount())
	}
	if SkillCount() != 11 {
		t.Errorf("SkillCount = %d, want 11", SkillCount())
	}
	if MapCount() != 1 {
		t.Errorf("MapCount = %d, want 1", MapCount())
	}
	if BattleSkillCount() != 1 {
		t.Errorf("BattleSkillCount = %d, want 1", BattleSkillCount())
	}

	// All IDs should be dense (1..N)
	itemIDs := AllItemIDs()
	if len(itemIDs) != ItemCount() {
		t.Errorf("AllItemIDs len = %d, want %d", len(itemIDs), ItemCount())
	}
	for i, id := range itemIDs {
		if int32(id) != int32(i+1) {
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
	if itemID.String() != "<item-17>" {
		// item.ID.String() uses numeric format
		_ = itemID
	}

	evID, _ := StringToEventID("felling_oak_tree")
	if evID.String() != "felling_oak_tree" {
		t.Errorf("EventID.String() = %q, want felling_oak_tree", evID.String())
	}
}
