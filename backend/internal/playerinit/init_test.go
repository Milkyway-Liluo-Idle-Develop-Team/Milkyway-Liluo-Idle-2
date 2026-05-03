package playerinit_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/db"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/playerinit"
	"github.com/edrowsluo/new-mli/backend/internal/skill"
	_ "modernc.org/sqlite"
)

func init() {
	if err := gameconfig.Load(); err != nil {
		panic("gameconfig: " + err.Error())
	}
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	schema := `
		CREATE TABLE IF NOT EXISTS player_skills (
			user_id INTEGER NOT NULL,
			skill_id INTEGER NOT NULL,
			level REAL NOT NULL DEFAULT 0,
			xp REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, skill_id)
		);
		CREATE TABLE IF NOT EXISTS player_init (
			user_id INTEGER NOT NULL PRIMARY KEY,
			initialized INTEGER NOT NULL DEFAULT 0,
			initialized_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	if _, err := conn.ExecContext(context.Background(), schema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	return &db.DB{
		Conn:    conn,
		Queries: dbgen.New(conn),
	}
}

func TestInitPlayer_SeedsAllSkillsAtLevel1(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()
	userID := int64(42)

	if err := playerinit.InitPlayer(ctx, userID, database); err != nil {
		t.Fatalf("InitPlayer failed: %v", err)
	}

	curve, err := skill.LoadCurve()
	if err != nil {
		t.Fatalf("load curve: %v", err)
	}
	wantXP := curve.XPForLevel(1)

	allIDs := gameconfig.AllSkillIDs()
	if len(allIDs) == 0 {
		t.Fatal("no skills in game config")
	}

	rows, err := database.Queries.LoadSkills(ctx, userID)
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}

	if len(rows) != len(allIDs) {
		t.Fatalf("want %d skill rows, got %d", len(allIDs), len(rows))
	}

	got := make(map[int64]struct{ Level, XP float64 }, len(rows))
	for _, r := range rows {
		got[r.SkillID] = struct{ Level, XP float64 }{Level: r.Level, XP: r.Xp}
	}

	for _, sid := range allIDs {
		slot, ok := got[int64(sid)]
		if !ok {
			t.Errorf("missing skill %d", sid)
			continue
		}
		if slot.Level != 1 {
			t.Errorf("skill %d: want level 1, got %v", sid, slot.Level)
		}
		if slot.XP != wantXP {
			t.Errorf("skill %d: want xp %v, got %v", sid, wantXP, slot.XP)
		}
	}
}

func TestInitPlayer_IsIdempotent(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()
	userID := int64(99)

	// First init.
	if err := playerinit.InitPlayer(ctx, userID, database); err != nil {
		t.Fatalf("first InitPlayer failed: %v", err)
	}

	// Second init should succeed without error (Upsert semantics).
	if err := playerinit.InitPlayer(ctx, userID, database); err != nil {
		t.Fatalf("second InitPlayer failed: %v", err)
	}

	rows, err := database.Queries.LoadSkills(ctx, userID)
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}
	if len(rows) != len(gameconfig.AllSkillIDs()) {
		t.Fatalf("want %d rows after idempotent init, got %d", len(gameconfig.AllSkillIDs()), len(rows))
	}
}

func TestInitPlayer_DifferentUsersAreIsolated(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()

	if err := playerinit.InitPlayer(ctx, 1, database); err != nil {
		t.Fatalf("user 1 init: %v", err)
	}
	if err := playerinit.InitPlayer(ctx, 2, database); err != nil {
		t.Fatalf("user 2 init: %v", err)
	}

	rows1, _ := database.Queries.LoadSkills(ctx, 1)
	rows2, _ := database.Queries.LoadSkills(ctx, 2)

	if len(rows1) != len(gameconfig.AllSkillIDs()) {
		t.Errorf("user 1: want %d rows, got %d", len(gameconfig.AllSkillIDs()), len(rows1))
	}
	if len(rows2) != len(gameconfig.AllSkillIDs()) {
		t.Errorf("user 2: want %d rows, got %d", len(gameconfig.AllSkillIDs()), len(rows2))
	}
}
