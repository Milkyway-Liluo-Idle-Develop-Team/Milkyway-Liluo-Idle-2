package inventory_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	dbgen "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/inventory"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	_ "modernc.org/sqlite"
)

func init() {
	if err := gameconfig.Load(); err != nil {
		panic("gameconfig: " + err.Error())
	}
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic("attribute: " + err.Error())
		}
	}
}

func openTestDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Run the schema.
	schema := `
	CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL, password_hash TEXT NOT NULL);
	CREATE TABLE player_inventory (
	    user_id INTEGER NOT NULL, item_id INTEGER NOT NULL, item_state INTEGER NOT NULL DEFAULT 0,
	    quantity REAL NOT NULL DEFAULT 0, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    PRIMARY KEY (user_id, item_id, item_state)
	);
	`
	_, err = db.ExecContext(context.Background(), schema)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	return db, dbgen.New(db)
}

func TestLoadEmpty(t *testing.T) {
	_, q := openTestDB(t)
	s, err := inventory.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.All()) != 0 {
		t.Errorf("want 0 slots, got %d", len(s.All()))
	}
}

func TestAddAndGet(t *testing.T) {
	_, q := openTestDB(t)
	s, err := inventory.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}

	it := item.Item{ID: 1, State: 0}
	s.Add(it, 5)
	s.Add(it, 3)

	if got := s.Get(it); got != 8 {
		t.Errorf("want 8, got %v", got)
	}
	if s.Display(it) != 8 {
		t.Errorf("display: want 8, got %d", s.Display(it))
	}
}

func TestFractionalAdd(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := inventory.Load(context.Background(), q, 1)

	it := item.Item{ID: 1, State: 0}
	s.Add(it, 1.3)
	s.Add(it, 1.4)

	if got := s.Get(it); got != 2.7 {
		t.Errorf("want 2.7, got %v", got)
	}
	if s.Display(it) != 2 {
		t.Errorf("display: want 2, got %d", s.Display(it))
	}
}

func TestDeduct(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := inventory.Load(context.Background(), q, 1)

	it := item.Item{ID: 1, State: 0}
	s.Add(it, 10)
	s.Deduct(it, 4)

	if got := s.Get(it); got != 6 {
		t.Errorf("want 6, got %v", got)
	}
}

func TestHas(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := inventory.Load(context.Background(), q, 1)

	it := item.Item{ID: 1, State: 0}
	s.Add(it, 5)

	if !s.Has(it, 5) {
		t.Error("should have 5")
	}
	if !s.Has(it, 3) {
		t.Error("should have 3")
	}
	if s.Has(it, 6) {
		t.Error("should not have 6")
	}
}

func TestMultipleStates(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := inventory.Load(context.Background(), q, 1)

	defaultAxe := item.Item{ID: 1, State: 0}
	upgradedAxe := item.Item{ID: 1, State: 10}

	s.Add(defaultAxe, 5)
	s.Add(upgradedAxe, 2)

	if got := s.Get(defaultAxe); got != 5 {
		t.Errorf("default: want 5, got %v", got)
	}
	if got := s.Get(upgradedAxe); got != 2 {
		t.Errorf("upgraded: want 2, got %v", got)
	}
}

func TestFlush(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := inventory.Load(context.Background(), q, 1)

	it := item.Item{ID: 3, State: 0}
	s.Add(it, 42)

	if err := s.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	// Reload and verify persistence.
	s2, err := inventory.Load(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Get(it); got != 42 {
		t.Errorf("after reload: want 42, got %v", got)
	}
}

func TestFlushNoDirty(t *testing.T) {
	_, q := openTestDB(t)
	s, _ := inventory.Load(context.Background(), q, 1)
	// No adds —Flush should be a no-op.
	if err := s.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}
}

func TestRecordBucket(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(inventory.Provider)
	reg.Register(attribute.Provider)

	rec := record.NewRecorder(reg)
	_, q := openTestDB(t)
	s, _ := inventory.Load(context.Background(), q, 1)
	s.SetRecorder(rec)

	rec.PushNamespace("event_execution")
	s.Add(item.Item{ID: 1, State: 0}, 5)
	s.Add(item.Item{ID: 1, State: 0}, 3) // same identity —merged in bucket
	s.Add(item.Item{ID: 2, State: 0}, -2)
	rec.PopNamespace()
	s.ClearRecorder()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	if len(diff.Inventory) != 2 {
		t.Fatalf("want 2 changes (1 merged, 1 subtract), got %d", len(diff.Inventory))
	}
	for _, c := range diff.Inventory {
		if c.ItemId == 1 && c.QuantityDelta != 8 {
			t.Errorf("item 1: want qty_delta=8, got %v", c.QuantityDelta)
		}
	}
}

func TestFullSnapshot(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(inventory.Provider)

	_, q := openTestDB(t)
	s, _ := inventory.Load(context.Background(), q, 1)
	s.Add(item.Item{ID: 1, State: 0}, 10)
	s.Add(item.Item{ID: 2, State: 0}, 20)

	data, err := reg.BuildFullSnapshot(map[string]any{
		"inventory": s,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(data.Inventory) != 2 {
		t.Fatalf("want 2 slots, got %d", len(data.Inventory))
	}
}
