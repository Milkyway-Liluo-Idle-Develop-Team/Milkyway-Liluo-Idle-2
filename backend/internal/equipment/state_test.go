package equipment_test

import (
	"context"
	"database/sql"
	"testing"

	dbgen "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/equipment"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := `
	CREATE TABLE player_equipment (
	    user_id INTEGER NOT NULL, slot TEXT NOT NULL,
	    item_id INTEGER NOT NULL, item_state INTEGER NOT NULL DEFAULT 0,
	    anchor_slot TEXT NOT NULL DEFAULT '',
	    PRIMARY KEY (user_id, slot)
	);
	`
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db, dbgen.New(db)
}

func TestLoadEmpty(t *testing.T) {
	_, q := openTestDB(t)
	s, err := equipment.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.All()) != 0 {
		t.Errorf("want 0 slots, got %d", len(s.All()))
	}
}

func TestEquipAndGet(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := equipment.Load(context.Background(), q, 1)

	it := item.Item{ID: 5, State: 0}
	s.Equip("main_hand", it, "main_hand")

	got, ok := s.Get("main_hand")
	if !ok {
		t.Fatal("slot should be occupied")
	}
	if got.Item != it {
		t.Errorf("want %v, got %v", it, got)
	}
	if len(s.All()) != 1 {
		t.Errorf("want 1 entry, got %d", len(s.All()))
	}
}

func TestUnequip(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := equipment.Load(context.Background(), q, 1)

	it := item.Item{ID: 5, State: 0}
	s.Equip("feet", it, "feet")
	s.Unequip("feet")

	if _, ok := s.Get("feet"); ok {
		t.Error("slot should be empty after unequip")
	}
	if len(s.All()) != 0 {
		t.Errorf("want 0 entries, got %d", len(s.All()))
	}
}

func TestEquipReplaceSlot(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := equipment.Load(context.Background(), q, 1)

	a := item.Item{ID: 1, State: 0}
	b := item.Item{ID: 2, State: 7}
	s.Equip("main_hand", a, "main_hand")
	s.Equip("main_hand", b, "main_hand")

	got, ok := s.Get("main_hand")
	if !ok || got.Item != b {
		t.Errorf("want %v, got %v ok=%v", b, got, ok)
	}
}

func TestFlushPersists(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := equipment.Load(context.Background(), q, 1)

	it := item.Item{ID: 9, State: 3}
	s.Equip("main_hand", it, "main_hand")
	if err := s.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	s2, err := equipment.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get("main_hand")
	if !ok || got.Item != it {
		t.Errorf("after reload: want %v, got %v ok=%v", it, got, ok)
	}
}

func TestFlushDeletes(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := equipment.Load(context.Background(), q, 1)

	it := item.Item{ID: 9, State: 0}
	s.Equip("main_hand", it, "main_hand")
	if err := s.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	s.Unequip("main_hand")
	if err := s.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	s2, _ := equipment.Load(context.Background(), q, 1)
	if _, ok := s2.Get("main_hand"); ok {
		t.Error("slot should be absent after flush+unequip+flush")
	}
}

func TestRecordBucket(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(equipment.Provider)

	rec := record.NewRecorder(reg)
	_, q := openTestDB(t)
	s, _ := equipment.Load(context.Background(), q, 1)
	s.SetRecorder(rec)

	rec.PushNamespace("event_execution")
	s.Equip("main_hand", item.Item{ID: 1, State: 0}, "main_hand")
	s.Equip("feet", item.Item{ID: 2, State: 0}, "feet")
	// Same slot, last action wins.
	s.Unequip("main_hand")
	rec.PopNamespace()
	s.ClearRecorder()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Equipment) != 2 {
		t.Fatalf("want 2 equipment diffs (last action per slot), got %d", len(diff.Equipment))
	}
	for _, d := range diff.Equipment {
		switch d.Slot {
		case "main_hand":
			if d.Action != pb.EquipAction_EQUIP_ACTION_UNEQUIP {
				t.Errorf("main_hand: want UNEQUIP, got %v", d.Action)
			}
		case "feet":
			if d.Action != pb.EquipAction_EQUIP_ACTION_EQUIP {
				t.Errorf("feet: want EQUIP, got %v", d.Action)
			}
		default:
			t.Errorf("unexpected slot %q", d.Slot)
		}
	}
}

func TestFullSnapshot(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(equipment.Provider)

	_, q := openTestDB(t)
	s, _ := equipment.Load(context.Background(), q, 1)
	s.Equip("main_hand", item.Item{ID: 1, State: 0}, "main_hand")
	s.Equip("feet", item.Item{ID: 2, State: 5}, "feet")

	full, err := reg.BuildFullSnapshot(map[string]any{
		"equipment": s,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Equipment) != 2 {
		t.Fatalf("want 2 slots, got %d", len(full.Equipment))
	}
	if full.Equipment["feet"].ItemState != 5 {
		t.Errorf("feet state: want 5, got %d", full.Equipment["feet"].ItemState)
	}
}
