package battle_test

import (
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

func TestBattleSessionWaveSpawnAndAttack(t *testing.T) {
	inst := attribute.NewInstance()
	p := battle.NewPlayerBattleEntity(1, "Hero", inst)
	p.SetHP(p.MaxHP())

	// Give the player a basic attack skill.
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
			Cooldown: 0,
			IsBasic:  true,
		},
	})
	p.SetBasicSkillID(gameconfig.BattleSkillID(1))
	p.SetSkillPlan([]battle.SkillPlanEntry{
		{SkillID: gameconfig.BattleSkillID(1), Priority: 0},
	})

	cfg := battle.BattleConfig{
		ID:              "test_battle",
		Name:            "Test Battle",
		Map:             "village",
		Interval:        3.0,
		CombinationLoop: []string{"weak"},
		WeakEnemyCombinations: []battle.EnemyWaveCombination{
			{Enemies: []string{"goblin"}, Weight: 100},
		},
	}

	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})

	// Advance to first event (should be wave spawn at t=3).
	logs := sess.AdvanceOneEvent()
	if len(logs) == 0 {
		t.Fatal("expected logs from first event")
	}
	foundSpawn := false
	for _, l := range logs {
		if l.Type == battle.BattleLogTypeWaveSpawned {
			foundSpawn = true
		}
	}
	if !foundSpawn {
		t.Fatalf("expected wave_spawned, got logs: %+v", logs)
	}

	// Should have 1 enemy now.
	if len(sess.Enemies) != 1 {
		t.Fatalf("expected 1 enemy, got %d", len(sess.Enemies))
	}

	// Advance until player attacks.
	for sess.Running && len(sess.AliveEnemies()) > 0 {
		logs = sess.AdvanceOneEvent()
		for _, l := range logs {
			if l.Type == battle.BattleLogTypePlayerAttack || l.Type == battle.BattleLogTypeEnemyAttack {
				t.Logf("combat log: %+v", l)
				return // success
			}
		}
		if len(logs) > 0 && logs[0].Type == battle.BattleLogTypeStopped {
			break
		}
	}
	t.Fatal("no combat log produced")
}
