package session_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/bestiary"
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

// sessionWithData is a pre-built session with its manager and DB connection,
// used for multi-player benchmarks.
type sessionWithData struct {
	session *session.PlayerSession
	mgr     *session.Manager
	reg     *record.Registry
	db      *sql.DB
	q       *dbgen.Queries
}

func openMultiPlayerDB(b *testing.B) *sql.DB {
	b.Helper()
	conn, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	b.Cleanup(func() { conn.Close() })
	_, err = conn.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS player_inventory (
			user_id INTEGER NOT NULL, item_id INTEGER NOT NULL,
			item_state INTEGER NOT NULL DEFAULT 0, quantity REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, item_id, item_state));
		CREATE TABLE IF NOT EXISTS player_skills (
			user_id INTEGER NOT NULL, skill_id INTEGER NOT NULL,
			level REAL NOT NULL DEFAULT 0, xp REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, skill_id));
		CREATE TABLE IF NOT EXISTS player_unlocked_events (
			user_id INTEGER NOT NULL, event_id INTEGER NOT NULL,
			unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, event_id));
		CREATE TABLE IF NOT EXISTS player_active_events (
			user_id INTEGER NOT NULL, queue_id INTEGER NOT NULL DEFAULT 0,
			event_id INTEGER NOT NULL, position INTEGER NOT NULL,
			target_cycles INTEGER NOT NULL DEFAULT -1, progress REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, queue_id, position));
	`)
	if err != nil {
		b.Fatalf("schema: %v", err)
	}
	return conn
}

func buildPlayer(b *testing.B, db *sql.DB, userID int64) *sessionWithData {
	b.Helper()
	q := dbgen.New(db)

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
	evSt, err := event.Load(context.Background(), q, userID)
	if err != nil {
		b.Fatal(err)
	}

	s := session.New(uuid.New(), userID, testLogger())
	s.SetInv(invSt)
	s.SetSkill(skillSt)
	s.SetBestiary(bestSt)
	s.SetEvents(evSt)

	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	reg.Register(inventory.Provider)
	reg.Register(skill.Provider)
	reg.Register(bestiary.Provider)
	reg.Register(event.ExecProvider)
	reg.Register(event.QueueProvider)

	mgr := session.NewManager(reg, nil)
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

	locked, _ := mgr.LockSession(s.ID)
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

	return &sessionWithData{
		session: s,
		mgr:     mgr,
		reg:     reg,
		db:      db,
		q:       q,
	}
}

// doTick runs settle + flush on one session. Keeps the recorder for diff building.
func doTick(sw *sessionWithData) {
	s, _ := sw.mgr.LockSession(sw.session.ID)

	rec := sw.mgr.NewRecorder()
	s.SetRecorder(rec)
	rec.PushNamespace("tick")
	s.Events().Settle(s, 10.0) // producing
	rec.PopNamespace()
	s.ClearRecorder()

	// FlushAll-like: one transaction per player.
	_ = flushOne(context.Background(), sw.db, s)

	sw.mgr.UnlockSession(s)
}

// doSettleOnly runs settle without flush.
func doSettleOnly(sw *sessionWithData) {
	s, _ := sw.mgr.LockSession(sw.session.ID)

	rec := sw.mgr.NewRecorder()
	s.SetRecorder(rec)
	rec.PushNamespace("tick")
	s.Events().Settle(s, 10.0)
	rec.PopNamespace()
	s.ClearRecorder()

	sw.mgr.UnlockSession(s)
}

func flushOne(ctx context.Context, db *sql.DB, s *session.PlayerSession) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	q := dbgen.New(db).WithTx(tx)
	if s.Inv() != nil {
		if err = s.Inv().Flush(ctx, q); err != nil {
			return err
		}
	}
	if s.Skill() != nil {
		if err = s.Skill().Flush(ctx, q); err != nil {
			return err
		}
	}
	if s.Bestiary() != nil {
		if err = s.Bestiary().Flush(ctx, q); err != nil {
			return err
		}
	}
	if s.Events() != nil {
		if err = s.Events().Flush(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// batchFlush flushes all sessions inside a single transaction.
func batchFlush(ctx context.Context, db *sql.DB, players []*sessionWithData) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	q := dbgen.New(db).WithTx(tx)
	for _, sw := range players {
		s, _ := sw.mgr.LockSession(sw.session.ID)
		if s.Inv() != nil {
			if err = s.Inv().Flush(ctx, q); err != nil {
				sw.mgr.UnlockSession(s)
				return err
			}
		}
		if s.Skill() != nil {
			if err = s.Skill().Flush(ctx, q); err != nil {
				sw.mgr.UnlockSession(s)
				return err
			}
		}
		if s.Bestiary() != nil {
			if err = s.Bestiary().Flush(ctx, q); err != nil {
				sw.mgr.UnlockSession(s)
				return err
			}
		}
		if s.Events() != nil {
			if err = s.Events().Flush(ctx, q); err != nil {
				sw.mgr.UnlockSession(s)
				return err
			}
		}
		sw.mgr.UnlockSession(s)
	}
	return nil
}

// Benchmark_MultiPlayer_Individual measures N players doing settle+flush individually.
// Uses :memory: to isolate transaction overhead from disk I/O.
func Benchmark_MultiPlayer_Individual(b *testing.B) {
	for _, n := range []int{1, 10, 100, 500, 1000} {
		b.Run(fmt.Sprintf("%dplayers", n), func(b *testing.B) {
			db := openMultiPlayerDB(b)
			players := make([]*sessionWithData, n)
			for i := range players {
				players[i] = buildPlayer(b, db, int64(i+1))
				// First flush to clear initial setup dirt.
				doTick(players[i])
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, sw := range players {
					doTick(sw)
				}
			}
		})
	}
}

// Benchmark_MultiPlayer_Batched measures N players settling individually,
// then all flushing in one transaction.
func Benchmark_MultiPlayer_Batched(b *testing.B) {
	for _, n := range []int{1, 10, 100, 500, 1000} {
		b.Run(fmt.Sprintf("%dplayers", n), func(b *testing.B) {
			db := openMultiPlayerDB(b)
			players := make([]*sessionWithData, n)
			for i := range players {
				players[i] = buildPlayer(b, db, int64(i+1))
				doTick(players[i]) // clear setup dirt
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Phase 1: all settle (in-memory only)
				for _, sw := range players {
					doSettleOnly(sw)
				}
				// Phase 2: batch flush (one transaction)
				if err := batchFlush(context.Background(), db, players); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark_MultiPlayer_OnDisk compares individual vs batched on a real disk file.
func Benchmark_MultiPlayer_OnDisk(b *testing.B) {
	for _, tc := range []struct {
		name string
		n    int
	}{
		{"individual_100", 100},
		{"individual_500", 500},
		{"batched_100", 100},
		{"batched_500", 500},
	} {
		b.Run(tc.name, func(b *testing.B) {
			dir := b.TempDir()
			path := dir + "/bench.db"
			conn, err := sql.Open("sqlite", "file:"+strings.ReplaceAll(path, "\\", "/")+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
			if err != nil {
				b.Fatalf("open db: %v", err)
			}
			b.Cleanup(func() { conn.Close() })
			_, err = conn.ExecContext(context.Background(), `
				CREATE TABLE IF NOT EXISTS player_inventory (
					user_id INTEGER NOT NULL, item_id INTEGER NOT NULL,
					item_state INTEGER NOT NULL DEFAULT 0, quantity REAL NOT NULL DEFAULT 0,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (user_id, item_id, item_state));
				CREATE TABLE IF NOT EXISTS player_skills (
					user_id INTEGER NOT NULL, skill_id INTEGER NOT NULL,
					level REAL NOT NULL DEFAULT 0, xp REAL NOT NULL DEFAULT 0,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (user_id, skill_id));
				CREATE TABLE IF NOT EXISTS player_unlocked_events (
					user_id INTEGER NOT NULL, event_id INTEGER NOT NULL,
					unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (user_id, event_id));
				CREATE TABLE IF NOT EXISTS player_active_events (
					user_id INTEGER NOT NULL, queue_id INTEGER NOT NULL DEFAULT 0,
					event_id INTEGER NOT NULL, position INTEGER NOT NULL,
					target_cycles INTEGER NOT NULL DEFAULT -1, progress REAL NOT NULL DEFAULT 0,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (user_id, queue_id, position));
			`)
			if err != nil {
				b.Fatalf("schema: %v", err)
			}

			n := tc.n
			players := make([]*sessionWithData, n)
			for i := range players {
				players[i] = buildPlayer(b, conn, int64(i+1))
				doTick(players[i]) // clear setup dirt
			}

			batched := strings.HasPrefix(tc.name, "batched")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if batched {
					for _, sw := range players {
						doSettleOnly(sw)
					}
					if err := batchFlush(context.Background(), conn, players); err != nil {
						b.Fatal(err)
					}
				} else {
					for _, sw := range players {
						doTick(sw)
					}
				}
			}

			// Report DB file size.
			if fi, err := os.Stat(path); err == nil {
				b.ReportMetric(float64(fi.Size()), "db_bytes")
			}
		})
	}
}
