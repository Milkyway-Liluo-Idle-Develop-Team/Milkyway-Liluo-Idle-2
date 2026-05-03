package bestiary_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/bestiary"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
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

func TestUnlockEvent(t *testing.T) {
	s := bestiary.New(0)
	id, _ := gameconfig.StringToEventID("felling_oak_tree")
	s.UnlockEvent(id)
	s.UnlockEvent(id) // idempotent —no panic
}

func TestUnlockItem(t *testing.T) {
	s := bestiary.New(0)
	s.UnlockItem(item.Item{ID: 1, State: 0})
	s.UnlockItem(item.Item{ID: 1, State: 5})
	s.UnlockItem(item.Item{ID: 1, State: 0}) // duplicate
}

func TestUnlockArea(t *testing.T) {
	s := bestiary.New(0)
	id, _ := gameconfig.StringToMapID("village")
	s.UnlockArea(id)
	s.UnlockArea(id) // idempotent
}

func TestRecordBucket(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(bestiary.Provider)

	s := bestiary.New(0)
	rec := record.NewRecorder(reg)
	s.SetRecorder(rec)

	eid, _ := gameconfig.StringToEventID("felling_oak_tree")
	mid, _ := gameconfig.StringToMapID("village")

	rec.PushNamespace("tick")
	s.UnlockEvent(eid)
	s.UnlockItem(item.Item{ID: 19, State: 0})
	s.UnlockArea(mid)
	// duplicates in same namespace
	s.UnlockEvent(eid)
	s.UnlockItem(item.Item{ID: 19, State: 0})
	rec.PopNamespace()
	s.ClearRecorder()

	diff, _ := reg.BuildDiff(rec)

	if len(diff.Bestiary) != 3 {
		t.Fatalf("want 3 unique entries, got %d", len(diff.Bestiary))
	}
}

func TestLoadRebuild(t *testing.T) {
	s := bestiary.New(0)

	eid, _ := gameconfig.StringToEventID("felling_oak_tree")
	mid, _ := gameconfig.StringToMapID("village")

	s.LoadEvents([]gameconfig.EventID{eid})
	s.LoadItems([]item.Item{{ID: 1, State: 0}, {ID: 2, State: 0}})
	s.LoadAreas([]gameconfig.MapID{mid})

	// Load methods should not write records (recorder is nil here, but
	// the point is they populate state without side effects).
	// Verify: calling Unlock on same data is no-op (already known).
	reg := record.NewRegistry()
	reg.Register(bestiary.Provider)
	rec := record.NewRecorder(reg)
	s.SetRecorder(rec)
	rec.PushNamespace("tick")

	s.UnlockEvent(eid)                   // already loaded →no record
	s.UnlockItem(item.Item{ID: 1, State: 0}) // already loaded →no record

	rec.PopNamespace()
	s.ClearRecorder()

	diff, _ := reg.BuildDiff(rec)
	// Diff should be empty —no new discoveries.
	if len(diff.Bestiary) != 0 {
		t.Errorf("expected empty diff after loading pre-known data, got %d entries", len(diff.Bestiary))
	}
}

func TestFlushUnlockedEvents(t *testing.T) {
	db, q := openDB(t)
	s := bestiary.New(1)
	eid, _ := gameconfig.StringToEventID("felling_oak_tree")
	s.UnlockEvent(eid)

	if err := s.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	// Verify persisted.
	rows, err := q.LoadUnlockedEvents(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.EventID == int64(eid) {
			found = true
		}
	}
	if !found {
		t.Error("unlocked event not persisted after flush")
	}

	// Second flush should be no-op (dirty cleared).
	if err := s.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	db.Close()
}

func openDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS player_unlocked_events (
			user_id INTEGER NOT NULL, event_id INTEGER NOT NULL,
			unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, event_id)
		)
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db, dbgen.New(db)
}

func TestFullSnapshot(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(bestiary.Provider)

	s := bestiary.New(0)
	eid, _ := gameconfig.StringToEventID("felling_oak_tree")
	s.UnlockEvent(eid)
	s.UnlockItem(item.Item{ID: 19, State: 0})

	data, _ := reg.BuildFullSnapshot(map[string]any{"bestiary": s})
	if len(data.Bestiary) != 2 {
		t.Fatalf("want 2 entries in full snapshot, got %d", len(data.Bestiary))
	}
}
