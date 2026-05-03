package session_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/bestiary"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/event"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/inventory"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/skill"
	"github.com/google/uuid"
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

func openFullDB(b *testing.B) *db.DB {
	b.Helper()
	conn, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	b.Cleanup(func() { conn.Close() })
	for _, s := range []string{
		`CREATE TABLE player_inventory (user_id INTEGER NOT NULL, item_id INTEGER NOT NULL,
			item_state INTEGER NOT NULL DEFAULT 0, quantity REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, item_id, item_state))`,
		`CREATE TABLE player_skills (user_id INTEGER NOT NULL, skill_id INTEGER NOT NULL,
			level REAL NOT NULL DEFAULT 0, xp REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, skill_id))`,
		`CREATE TABLE player_unlocked_events (user_id INTEGER NOT NULL, event_id INTEGER NOT NULL,
			unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, event_id))`,
		`CREATE TABLE player_active_events (user_id INTEGER NOT NULL, queue_id INTEGER NOT NULL DEFAULT 0,
			event_id INTEGER NOT NULL, position INTEGER NOT NULL,
			target_cycles INTEGER NOT NULL DEFAULT -1, progress REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, queue_id, position))`,
	} {
		if _, err := conn.Exec(s); err != nil {
			b.Fatalf("schema: %v", err)
		}
	}
	return &db.DB{Conn: conn, Queries: dbgen.New(conn)}
}

func newSession(b *testing.B, database *db.DB, userID int64) *session.PlayerSession {
	b.Helper()
	q := database.Queries
	invSt, err := inventory.Load(context.Background(), q, userID)
	if err != nil {
		b.Fatal(err)
	}
	curve, err := skill.LoadCurve()
	if err != nil {
		b.Fatal(err)
	}
	skillSt, err := skill.Load(context.Background(), q, userID, curve)
	if err != nil {
		b.Fatal(err)
	}
	bestSt := bestiary.New(userID)
	rows, _ := q.LoadUnlockedEvents(context.Background(), userID)
	ids := make([]gameconfig.EventID, len(rows))
	for i, r := range rows {
		ids[i] = gameconfig.EventID(r.EventID)
	}
	bestSt.LoadEvents(ids)
	evSt, err := event.Load(context.Background(), q, userID)
	if err != nil {
		b.Fatal(err)
	}
	s := session.New(uuid.New(), userID, testLogger())
	s.SetInv(invSt)
	s.SetSkill(skillSt)
	s.SetBestiary(bestSt)
	s.SetEvents(evSt)
	return s
}

func newReg() *record.Registry {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	reg.Register(inventory.Provider)
	reg.Register(skill.Provider)
	reg.Register(bestiary.Provider)
	reg.Register(event.ExecProvider)
	reg.Register(event.QueueProvider)
	return reg
}

// BenchmarkTick_Producing measures a tick where events complete cycles and produce items.
func BenchmarkTick_Producing(b *testing.B) {
	database := openFullDB(b)
	reg := newReg()
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := newSession(b, database, 1)
	mgr.Add(s)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	miningID, _ := gameconfig.StringToEventID("mining_dirt")
	plankID, _ := gameconfig.StringToEventID("making_oak_plank")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	craftSkill, _ := gameconfig.StringToSkillID("crafting")
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	dirtID, _ := gameconfig.StringToItemID("dirt")
	plankItemID, _ := gameconfig.StringToItemID("oak_plank")
	startingDialog, _ := gameconfig.StringToEventID("starting_dialog_5")

	// One-time setup: enqueue 3 production events, set skills, seed items.
	locked, _ := mgr.LockSession(s.UserID)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Events().Enqueue(0, miningID, -1)
	locked.Events().Enqueue(0, plankID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	locked.Skill().AddXP(miningSkill, 50)
	locked.Skill().AddXP(craftSkill, 500)
	locked.Bestiary().UnlockEvent(startingDialog)
	// Seed enough raw materials for many cycles.
	locked.Inv().Add(item.Item{ID: oakID}, 1e6)
	locked.Inv().Add(item.Item{ID: dirtID}, 1e6)
	locked.Inv().Add(item.Item{ID: plankItemID}, 1e6)
	mgr.UnlockSession(locked)

	// produce events DON'T re-enqueue, settle just accumulates cycles.
	// But felling consumes oak_logs (wait, no —felling PRODUCES oak_logs, doesn't consume).
	// making_oak_plank typically consumes oak_logs. So we need to keep refilling.
	// For simplicity, use a delta that's large enough to produce every time.

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		locked, _ := mgr.LockSession(s.UserID)

		rec := mgr.NewRecorder()
		locked.SetRecorder(rec)
		rec.PushNamespace("tick")

		locked.Events().Settle(locked, 10.0) // fire ~5 cycles of each event

		rec.PopNamespace()
		locked.ClearRecorder()

		_ = locked.FlushAll(context.Background(), database)
		_, _ = reg.BuildDiff(rec)

		mgr.UnlockSession(locked)
	}
}

// BenchmarkTick_Idle measures a 1-second tick with no production (just progress updates).
func BenchmarkTick_Idle(b *testing.B) {
	database := openFullDB(b)
	reg := newReg()
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := newSession(b, database, 1)
	mgr.Add(s)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	miningID, _ := gameconfig.StringToEventID("mining_dirt")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")

	locked, _ := mgr.LockSession(s.UserID)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Events().Enqueue(0, miningID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	locked.Skill().AddXP(miningSkill, 50)
	mgr.UnlockSession(locked)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		locked, _ := mgr.LockSession(s.UserID)

		rec := mgr.NewRecorder()
		locked.SetRecorder(rec)
		rec.PushNamespace("tick")

		locked.Events().Settle(locked, 1.0) // <1 cycle for typical loop_time (2s+)

		rec.PopNamespace()
		locked.ClearRecorder()

		_ = locked.FlushAll(context.Background(), database)
		_, _ = reg.BuildDiff(rec)

		mgr.UnlockSession(locked)
	}
}

// BenchmarkTick_OfflineReturn measures an 8-hour offline catch-up.
func BenchmarkTick_OfflineReturn(b *testing.B) {
	database := openFullDB(b)
	reg := newReg()
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := newSession(b, database, 1)
	mgr.Add(s)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	miningID, _ := gameconfig.StringToEventID("mining_dirt")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	dirtID, _ := gameconfig.StringToItemID("dirt")

	// Setup: enqueue events, set skills, seed items.
	locked, _ := mgr.LockSession(s.UserID)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Events().Enqueue(0, miningID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	locked.Skill().AddXP(miningSkill, 50)
	locked.Inv().Add(item.Item{ID: oakID}, 1e6)
	locked.Inv().Add(item.Item{ID: dirtID}, 1e6)
	mgr.UnlockSession(locked)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		locked, _ := mgr.LockSession(s.UserID)

		rec := mgr.NewRecorder()
		locked.SetRecorder(rec)
		rec.PushNamespace("tick")

		locked.Events().Settle(locked, 28800.0) // 8 hours

		rec.PopNamespace()
		locked.ClearRecorder()

		_ = locked.FlushAll(context.Background(), database)
		_, _ = reg.BuildDiff(rec)

		mgr.UnlockSession(locked)
	}
}

// BenchmarkTick_OnDisk measures a producing tick on a disk-backed SQLite file.
func BenchmarkTick_OnDisk(b *testing.B) {
	dir := b.TempDir()
	path := dir + "/bench.db"
	conn, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	b.Cleanup(func() { conn.Close() })
	for _, s := range []string{
		`CREATE TABLE IF NOT EXISTS player_inventory (user_id INTEGER NOT NULL, item_id INTEGER NOT NULL,
			item_state INTEGER NOT NULL DEFAULT 0, quantity REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, item_id, item_state))`,
		`CREATE TABLE IF NOT EXISTS player_skills (user_id INTEGER NOT NULL, skill_id INTEGER NOT NULL,
			level REAL NOT NULL DEFAULT 0, xp REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, skill_id))`,
		`CREATE TABLE IF NOT EXISTS player_unlocked_events (user_id INTEGER NOT NULL, event_id INTEGER NOT NULL,
			unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, event_id))`,
		`CREATE TABLE IF NOT EXISTS player_active_events (user_id INTEGER NOT NULL, queue_id INTEGER NOT NULL DEFAULT 0,
			event_id INTEGER NOT NULL, position INTEGER NOT NULL,
			target_cycles INTEGER NOT NULL DEFAULT -1, progress REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, queue_id, position))`,
	} {
		if _, err := conn.Exec(s); err != nil {
			b.Fatalf("schema: %v", err)
		}
	}
	database := &db.DB{Conn: conn, Queries: dbgen.New(conn)}
	reg := newReg()
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := newSession(b, database, 1)
	mgr.Add(s)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	miningID, _ := gameconfig.StringToEventID("mining_dirt")
	plankID, _ := gameconfig.StringToEventID("making_oak_plank")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	craftSkill, _ := gameconfig.StringToSkillID("crafting")
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	dirtID, _ := gameconfig.StringToItemID("dirt")
	plankItemID, _ := gameconfig.StringToItemID("oak_plank")
	startingDialog, _ := gameconfig.StringToEventID("starting_dialog_5")

	locked, _ := mgr.LockSession(s.UserID)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Events().Enqueue(0, miningID, -1)
	locked.Events().Enqueue(0, plankID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	locked.Skill().AddXP(miningSkill, 50)
	locked.Skill().AddXP(craftSkill, 500)
	locked.Bestiary().UnlockEvent(startingDialog)
	locked.Inv().Add(item.Item{ID: oakID}, 1e6)
	locked.Inv().Add(item.Item{ID: dirtID}, 1e6)
	locked.Inv().Add(item.Item{ID: plankItemID}, 1e6)
	mgr.UnlockSession(locked)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		locked, _ := mgr.LockSession(s.UserID)

		rec := mgr.NewRecorder()
		locked.SetRecorder(rec)
		rec.PushNamespace("tick")

		locked.Events().Settle(locked, 10.0)

		rec.PopNamespace()
		locked.ClearRecorder()

		_ = locked.FlushAll(context.Background(), database)
		_, _ = reg.BuildDiff(rec)

		mgr.UnlockSession(locked)
	}
}

// BenchmarkTick_OnDisk_FullSync measures a producing tick with synchronous=FULL
// (every commit fsyncs). This is the "safest" setting for crash resilience.
func BenchmarkTick_OnDisk_FullSync(b *testing.B) {
	dir := b.TempDir()
	path := dir + "/bench_full.db"
	conn, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)")
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	b.Cleanup(func() { conn.Close() })
	for _, s := range []string{
		`CREATE TABLE IF NOT EXISTS player_inventory (user_id INTEGER NOT NULL, item_id INTEGER NOT NULL,
			item_state INTEGER NOT NULL DEFAULT 0, quantity REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, item_id, item_state))`,
		`CREATE TABLE IF NOT EXISTS player_skills (user_id INTEGER NOT NULL, skill_id INTEGER NOT NULL,
			level REAL NOT NULL DEFAULT 0, xp REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, skill_id))`,
		`CREATE TABLE IF NOT EXISTS player_unlocked_events (user_id INTEGER NOT NULL, event_id INTEGER NOT NULL,
			unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, event_id))`,
		`CREATE TABLE IF NOT EXISTS player_active_events (user_id INTEGER NOT NULL, queue_id INTEGER NOT NULL DEFAULT 0,
			event_id INTEGER NOT NULL, position INTEGER NOT NULL,
			target_cycles INTEGER NOT NULL DEFAULT -1, progress REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, queue_id, position))`,
	} {
		if _, err := conn.Exec(s); err != nil {
			b.Fatalf("schema: %v", err)
		}
	}
	database := &db.DB{Conn: conn, Queries: dbgen.New(conn)}
	reg := newReg()
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := newSession(b, database, 1)
	mgr.Add(s)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	miningID, _ := gameconfig.StringToEventID("mining_dirt")
	plankID, _ := gameconfig.StringToEventID("making_oak_plank")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	craftSkill, _ := gameconfig.StringToSkillID("crafting")
	oakID, _ := gameconfig.StringToItemID("oak_logs")
	dirtID, _ := gameconfig.StringToItemID("dirt")
	plankItemID, _ := gameconfig.StringToItemID("oak_plank")
	startingDialog, _ := gameconfig.StringToEventID("starting_dialog_5")

	locked, _ := mgr.LockSession(s.UserID)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Events().Enqueue(0, miningID, -1)
	locked.Events().Enqueue(0, plankID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	locked.Skill().AddXP(miningSkill, 50)
	locked.Skill().AddXP(craftSkill, 500)
	locked.Bestiary().UnlockEvent(startingDialog)
	locked.Inv().Add(item.Item{ID: oakID}, 1e6)
	locked.Inv().Add(item.Item{ID: dirtID}, 1e6)
	locked.Inv().Add(item.Item{ID: plankItemID}, 1e6)
	mgr.UnlockSession(locked)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		locked, _ := mgr.LockSession(s.UserID)

		rec := mgr.NewRecorder()
		locked.SetRecorder(rec)
		rec.PushNamespace("tick")

		locked.Events().Settle(locked, 10.0)

		rec.PopNamespace()
		locked.ClearRecorder()

		_ = locked.FlushAll(context.Background(), database)
		_, _ = reg.BuildDiff(rec)

		mgr.UnlockSession(locked)
	}
}

// BenchmarkSettle_Only measures pure in-memory settle (no DB, no diff).
func BenchmarkSettle_Only(b *testing.B) {
	database := openFullDB(b)
	reg := record.NewRegistry()
	reg.Register(event.ExecProvider)
	reg.Register(event.QueueProvider)
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := newSession(b, database, 1)
	mgr.Add(s)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	miningID, _ := gameconfig.StringToEventID("mining_dirt")
	plankID, _ := gameconfig.StringToEventID("making_oak_plank")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	miningSkill, _ := gameconfig.StringToSkillID("mining")
	craftSkill, _ := gameconfig.StringToSkillID("crafting")
	oakID, _ := gameconfig.StringToItemID("oak_logs")

	locked, _ := mgr.LockSession(s.UserID)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Events().Enqueue(0, miningID, -1)
	locked.Events().Enqueue(0, plankID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	locked.Skill().AddXP(miningSkill, 50)
	locked.Skill().AddXP(craftSkill, 500)
	locked.Inv().Add(item.Item{ID: oakID}, 1e6)
	mgr.UnlockSession(locked)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		locked, _ := mgr.LockSession(s.UserID)
		rec := mgr.NewRecorder()
		locked.SetRecorder(rec)
		rec.PushNamespace("tick")

		locked.Events().Settle(locked, 10.0)

		rec.PopNamespace()
		locked.ClearRecorder()
		mgr.UnlockSession(locked)
	}
}
