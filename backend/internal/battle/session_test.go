package battle_test

import (
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
)

func TestBattleSessionWaveSpawnAndAttack(t *testing.T) {
	inst := attribute.NewInstance()
	p := battle.NewPlayerBattleEntity(1, "Hero", inst)
	p.SetHP(p.MaxHP())

	// Give the player a basic attack skill.
	p.SetSkills(map[string]*battle.BattleSkill{
		"basic_attack": {
			ID:   "basic_attack",
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
	p.SetBasicSkillID("basic_attack")
	p.SetSkillPlan([]battle.SkillPlanEntry{
		{SkillID: "basic_attack", Priority: 0},
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

	sess := battle.NewBattleSession(cfg, p)

	// Advance to first event (should be wave spawn at t=3).
	logs := sess.AdvanceOneEvent()
	if len(logs) == 0 {
		t.Fatal("expected logs from first event")
	}
	foundSpawn := false
	for _, l := range logs {
		if l.Type == "wave_spawned" {
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
			if l.Type == "player_attack" || l.Type == "enemy_attack" {
				t.Logf("combat log: %+v", l)
				return // success
			}
		}
		if len(logs) > 0 && logs[0].Type == "stopped" {
			break
		}
	}
	t.Fatal("no combat log produced")
}
