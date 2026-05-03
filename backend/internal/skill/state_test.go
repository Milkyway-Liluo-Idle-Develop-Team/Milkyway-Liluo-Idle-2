package skill_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/skill"
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

func testCurve(t *testing.T) skill.LevelCurve {
	t.Helper()
	c, err := skill.LoadCurve()
	if err != nil {
		t.Fatalf("load curve: %v", err)
	}
	return c
}

func openSkillDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS player_skills (
			user_id INTEGER NOT NULL, skill_id INTEGER NOT NULL,
			level REAL NOT NULL DEFAULT 0, xp REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, skill_id)
		)
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db, dbgen.New(db)
}

func TestLoadEmpty(t *testing.T) {
	_, q := openSkillDB(t)
	s, err := skill.Load(context.Background(), q, 1, testCurve(t))
	if err != nil {
		t.Fatal(err)
	}
	level, xp := s.Get(1)
	if level != 0 || xp != 0 {
		t.Errorf("want 0/0, got %v/%v", level, xp)
	}
}

func TestAddXPLevelUp(t *testing.T) {
	_, q := openSkillDB(t)
	s, err := skill.Load(context.Background(), q, 1, testCurve(t))
	if err != nil {
		t.Fatal(err)
	}

	var leveledUp bool
	var oldLvl, newLvl float64
	s.OnLevelUp = func(id gameconfig.SkillID, oldLevel, newLevel float64) {
		leveledUp = true
		oldLvl = oldLevel
		newLvl = newLevel
	}

	// Level 1 requires 80 XP. Add enough to reach it.
	s.AddXP(1, 100)
	level, xp := s.Get(1)
	if level != 1 {
		t.Errorf("want level 1, got %v", level)
	}
	if xp != 100 {
		t.Errorf("want xp 100, got %v", xp)
	}
	if !leveledUp {
		t.Error("OnLevelUp should have fired")
	}
	if oldLvl != 0 || newLvl != 1 {
		t.Errorf("OnLevelUp: want 0→1, got %v→%v", oldLvl, newLvl)
	}
}

func TestAddXPNoLevelUp(t *testing.T) {
	_, q := openSkillDB(t)
	s, _ := skill.Load(context.Background(), q, 1, testCurve(t))

	fired := false
	s.OnLevelUp = func(id gameconfig.SkillID, _, _ float64) {
		fired = true
	}

	// 50 XP is below the 80 XP threshold for level 1.
	s.AddXP(1, 50)
	level, _ := s.Get(1)
	if level != 0 {
		t.Errorf("want level 0, got %v", level)
	}
	if fired {
		t.Error("OnLevelUp should not fire without level-up")
	}
}

func TestFlush(t *testing.T) {
	_, q := openSkillDB(t)
	s, _ := skill.Load(context.Background(), q, 1, testCurve(t))
	s.AddXP(3, 500)

	if err := s.Flush(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	// Reload and verify.
	s2, _ := skill.Load(context.Background(), q, 1, testCurve(t))
	level, xp := s2.Get(3)
	if level == 0 {
		t.Errorf("level should be > 0 after 500 XP")
	}
	if xp != 500 {
		t.Errorf("want 500 XP, got %v", xp)
	}
}

func TestRecordBucket(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(skill.Provider)

	_, q := openSkillDB(t)
	s, _ := skill.Load(context.Background(), q, 1, testCurve(t))

	rec := record.NewRecorder(reg)
	s.SetRecorder(rec)

	rec.PushNamespace("tick")
	s.AddXP(4, 120)
	s.AddXP(4, 30) // same skill →merged
	rec.PopNamespace()
	s.ClearRecorder()

	diff, _ := reg.BuildDiff(rec)

	if len(diff.SkillXp) != 1 {
		t.Fatalf("want 1 merged change, got %d", len(diff.SkillXp))
	}
	if diff.SkillXp[0].XpDelta != 150 {
		t.Errorf("want xp_delta=150, got %v", diff.SkillXp[0].XpDelta)
	}
}
