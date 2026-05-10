package session_test

import (
	"testing"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/session"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/wsx"
	"github.com/google/uuid"
)

func makeTestBattlePlayer(userID int64, name string) *battle.PlayerBattleEntity {
	inst := attribute.NewInstance()
	p := battle.NewPlayerBattleEntity(userID, name, inst)
	p.SetHP(p.MaxHP())
	p.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
		gameconfig.BattleSkillID(1): {
			ID: gameconfig.BattleSkillID(1),
			Name: "基础攻击",
			Damage: &battle.DamageProfile{
				Type:       "physical",
				Flat:       0,
				Multiplier: 1.0,
			},
			CastTime: 2.0,
			IsBasic:  true,
		},
	})
	p.SetBasicSkillID(gameconfig.BattleSkillID(1))
	p.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})
	return p
}

func TestBattleSessionAttachDetach(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	if s.BattleSession() != nil {
		t.Fatal("expected no battle initially")
	}
	bs := battle.NewBattleSession(battle.BattleConfig{
		NumericID: 99,
		ID:        "test",
		Name:      "Test Battle",
		Map:       "test_map",
		Interval:  5.0,
	}, []*battle.PlayerBattleEntity{makeTestBattlePlayer(1, "Player1")})
	s.SetBattleSession(bs)
	mgr.AddBattle(bs)
	if s.BattleSession() != bs {
		t.Fatal("battle session should be attached")
	}
	retrieved, ok := mgr.GetBattle(99)
	if !ok || retrieved != bs {
		t.Fatal("battle should be registered in manager")
	}
	s.SetBattleSession(nil)
	mgr.RemoveBattle(99)
	if s.BattleSession() != nil {
		t.Fatal("battle session should be detached")
	}
}

func TestRLockSession(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	sess, ok := mgr.RLockSession(1)
	if !ok {
		t.Fatal("expected session")
	}
	r := attribute.Get()
	id, _ := r.AttrID("physical_power")
	_ = sess.Attr().GetFinal(id) // read under RLock
	mgr.RUnlockSession(sess)
}

func TestGraceExtendedDuringBattle(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	// Set active battle
	bs := battle.NewBattleSession(battle.BattleConfig{
		NumericID: 99,
		ID:        "test",
		Name:      "Test Battle",
		Map:       "test_map",
		Interval:  5.0,
	}, []*battle.PlayerBattleEntity{makeTestBattlePlayer(1, "Player1")})
	s.SetBattleSession(bs)
	mgr.AddBattle(bs)

	// Attach then detach conn to enter grace
	conn := &wsx.Conn{ID: uuid.New(), UserID: 1}
	s.AttachConn(conn)
	s.DetachConn()

	s.StartGraceTimer(100 * time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	if s.State() != session.StateGrace {
		t.Fatalf("expected StateGrace during battle, got %v", s.State())
	}

	// Deactivate battle and re-start grace
	bs.Running = false
	s.StartGraceTimer(100 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	if s.State() != session.StateClosed {
		t.Fatalf("expected StateClosed after battle ended, got %v", s.State())
	}
}
