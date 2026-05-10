package battle_test

import (
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

func TestRealEnemyDataFromGameConfig(t *testing.T) {
	def, ok := gameconfig.GetEnemy("goblin")
	if !ok {
		t.Fatal("goblin not found in gameconfig")
	}
	if def.Name == "" {
		t.Error("goblin name should not be empty")
	}
	if def.BattleData["hp"] != 200 {
		t.Errorf("goblin hp want 200, got %v", def.BattleData["hp"])
	}
	if def.BattleData["physical_power"] != 60 {
		t.Errorf("goblin physical_power want 60, got %v", def.BattleData["physical_power"])
	}

	// Build an enemy entity from the definition.
	inst := attribute.NewInstance()
	p := battle.NewPlayerBattleEntity(1, "Hero", inst)
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
	logs := sess.AdvanceOneEvent() // wave spawn at t=3

	if len(sess.Enemies) != 1 {
		t.Fatalf("expected 1 enemy, got %d", len(sess.Enemies))
	}

	enemy := sess.Enemies[0]
	if enemy.MaxHP() != 200 {
		t.Errorf("enemy MaxHP want 200, got %v", enemy.MaxHP())
	}
	if enemy.GetFinal(battle.AttrPhysicalPower) != 60 {
		t.Errorf("enemy physical_power want 60, got %v", enemy.GetFinal(battle.AttrPhysicalPower))
	}
	if enemy.GetFinal(battle.AttrDefense) != 30 {
		t.Errorf("enemy defense want 30, got %v", enemy.GetFinal(battle.AttrDefense))
	}

	t.Logf("wave logs: %+v", logs)
	t.Logf("enemy stats: HP=%v, PP=%v, DEF=%v", enemy.MaxHP(), enemy.GetFinal(battle.AttrPhysicalPower), enemy.GetFinal(battle.AttrDefense))
}

// TestActionsJSONWeakCombinations verifies that weak_enemy_combinations from
// actions.json are correctly loaded into the WeakEnemyCombinations field.
func TestActionsJSONWeekCombinations(t *testing.T) {
	pasture, ok := gameconfig.GetBattle("pasture")
	if !ok {
		t.Fatal("pasture battle not found in gameconfig")
	}

	if len(pasture.WeakEnemyCombinations) == 0 {
		t.Fatal("pasture should have weak (week) enemy combinations loaded from actions.json")
	}

	// actions.json pasture has 3 weak combinations with weights 40, 30, 30.
	var totalWeight float64
	for _, c := range pasture.WeakEnemyCombinations {
		totalWeight += c.Weight
	}
	if totalWeight != 100 {
		t.Errorf("pasture weak total weight want 100, got %v", totalWeight)
	}

	// Check one of the combinations contains expected enemies.
	foundPig := false
	for _, c := range pasture.WeakEnemyCombinations {
		for _, e := range c.Enemies {
			if e == "pig" {
				foundPig = true
			}
		}
	}
	if !foundPig {
		t.Error("expected 'pig' to appear in pasture weak combinations")
	}

	// Verify strong and boss combinations are also present.
	if len(pasture.StrongEnemyCombinations) == 0 {
		t.Error("pasture should have strong enemy combinations")
	}
	if len(pasture.BossEnemyCombinations) == 0 {
		t.Error("pasture should have boss enemy combinations")
	}

	// Verify outskirts_of_village also has data.
	outskirts, ok := gameconfig.GetBattle("outskirts_of_village")
	if !ok {
		t.Fatal("outskirts_of_village battle not found")
	}
	if len(outskirts.WeakEnemyCombinations) == 0 {
		t.Error("outskirts_of_village should have weak enemy combinations")
	}
	if len(outskirts.StrongEnemyCombinations) == 0 {
		t.Error("outskirts_of_village should have strong enemy combinations")
	}
	if len(outskirts.BossEnemyCombinations) == 0 {
		t.Error("outskirts_of_village should have boss enemy combinations")
	}

	// Verify cave only has boss (no weak or strong per the JSON).
	cave, ok := gameconfig.GetBattle("cave")
	if !ok {
		t.Fatal("cave battle not found")
	}
	if len(cave.WeakEnemyCombinations) != 0 {
		t.Errorf("cave should have no weak combinations, got %d", len(cave.WeakEnemyCombinations))
	}
	if len(cave.StrongEnemyCombinations) != 0 {
		t.Errorf("cave should have no strong combinations, got %d", len(cave.StrongEnemyCombinations))
	}
	if len(cave.BossEnemyCombinations) == 0 {
		t.Error("cave should have boss enemy combinations")
	}

	t.Logf("pasture weak combos count: %d", len(pasture.WeakEnemyCombinations))
	t.Logf("outskirts weak combos count: %d", len(outskirts.WeakEnemyCombinations))
	t.Logf("cave boss combos count: %d", len(cave.BossEnemyCombinations))
}
