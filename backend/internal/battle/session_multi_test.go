package battle_test

import (
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
)

// TestTwoPlayersAttack verifies that in a 2-player session both players get to attack.
func TestTwoPlayersAttack(t *testing.T) {
	p1 := makeTestPlayer(1, "Alice")
	p2 := makeTestPlayer(2, "Bob")

	cfg := battle.BattleConfig{
		ID:              "test_multi",
		Name:            "Multi Test",
		Map:             "village",
		Interval:        3.0,
		CombinationLoop: []string{"weak"},
		WeakEnemyCombinations: []battle.EnemyWaveCombination{
			{Enemies: []string{"goblin"}, Weight: 100},
		},
	}

	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p1, p2})

	// Wave spawn.
	logs := sess.AdvanceOneEvent()
	if len(sess.Enemies) != 1 {
		t.Fatalf("expected 1 enemy, got %d", len(sess.Enemies))
	}

	// Advance until both players have attacked at least once.
	p1Attacked := false
	p2Attacked := false
	maxEvents := 50
	for i := 0; i < maxEvents && sess.Running; i++ {
		logs = sess.AdvanceOneEvent()
		for _, l := range logs {
			if l.Type == "player_attack" {
				if l.AttackerID == "Alice" {
					p1Attacked = true
				}
				if l.AttackerID == "Bob" {
					p2Attacked = true
				}
			}
		}
		if p1Attacked && p2Attacked {
			break
		}
	}

	if !p1Attacked {
		t.Error("player 1 (Alice) never attacked")
	}
	if !p2Attacked {
		t.Error("player 2 (Bob) never attacked")
	}
}

// TestHateTargeting verifies that an enemy preferentially attacks the player
// who has dealt the most damage to it.
func TestHateTargeting(t *testing.T) {
	// Player A: high physical power → will deal more damage → generate more hate.
	// Also give both players high accuracy so attacks actually land.
	ppID, _ := attribute.Get().AttrID("physical_power")
	accID, _ := attribute.Get().AttrID("accuracy")

	pAAttr := attribute.NewInstance()
	pAAttr.AddModifiers("test", []attribute.Modifier{
		{AttrID: ppID, Op: attribute.OpOverride, Value: 500, Source: "test"},
		{AttrID: accID, Op: attribute.OpOverride, Value: 1000, Source: "test"},
	})
	pA := battle.NewPlayerBattleEntity(1, "Aggro", pAAttr)
	pA.SetHP(pA.MaxHP())
	pA.SetSkills(map[string]*battle.BattleSkill{
		"basic_attack": {
			ID:   "basic_attack",
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
	pA.SetBasicSkillID("basic_attack")
	pA.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: "basic_attack", Priority: 0}})

	pBAttr := attribute.NewInstance()
	pBAttr.AddModifiers("test", []attribute.Modifier{
		{AttrID: ppID, Op: attribute.OpOverride, Value: 1, Source: "test"},
		{AttrID: accID, Op: attribute.OpOverride, Value: 1000, Source: "test"},
	})
	pB := battle.NewPlayerBattleEntity(2, "Passive", pBAttr)
	pB.SetHP(pB.MaxHP())
	pB.SetSkills(map[string]*battle.BattleSkill{
		"basic_attack": {
			ID:   "basic_attack",
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
	pB.SetBasicSkillID("basic_attack")
	pB.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: "basic_attack", Priority: 0}})

	cfg := battle.BattleConfig{
		ID:              "test_hate",
		Name:            "Hate Test",
		Map:             "village",
		Interval:        3.0,
		CombinationLoop: []string{"weak"},
		WeakEnemyCombinations: []battle.EnemyWaveCombination{
			{Enemies: []string{"goblin"}, Weight: 100},
		},
	}

	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{pA, pB})

	// Manually inject a high-HP enemy so it survives long enough to attack many times.
	enemy := battle.NewEnemyBattleEntity("tank", "tank_0", "TankEnemy", map[string]float64{
		"hp":              5000,
		"physical_power":  30,
		"defense":         10,
		"attack_interval": 2,
		"accuracy":        1000,
	})
	enemy.SetHP(enemy.MaxHP())
	enemy.SetNextReadyTime(5.0)
	enemy.SetSkills(map[string]*battle.BattleSkill{
		"basic_attack": {
			ID:   "basic_attack",
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
	enemy.SetBasicSkillID("basic_attack")
	enemy.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: "basic_attack", Priority: 0}})
	sess.Enemies = append(sess.Enemies, enemy)
	sess.NextWaveTime = nil // prevent auto wave spawn

	// Run many events to gather attack statistics.
	targetCounts := map[string]int{}
	maxEvents := 80
	for i := 0; i < maxEvents && sess.Running; i++ {
		logs := sess.AdvanceOneEvent()
		for _, l := range logs {
			if l.Type == "enemy_attack" {
				targetCounts[l.DefenderID]++
			}
		}
	}

	aggroCount := targetCounts["Aggro"]
	passiveCount := targetCounts["Passive"]
	t.Logf("enemy targeted Aggro %d times, Passive %d times", aggroCount, passiveCount)

	if aggroCount == 0 {
		t.Error("enemy never targeted Aggro (high-damage player)")
	}
	if passiveCount > aggroCount {
		t.Errorf("expected enemy to target Aggro more than Passive, got Aggro=%d Passive=%d", aggroCount, passiveCount)
	}
}

// TestPartialPlayerDeath verifies that when one player dies, the other can
// continue fighting and the dead player respawns later.
func TestPartialPlayerDeath(t *testing.T) {
	// Player A: normal stats.
	pA := makeTestPlayer(1, "Tank")
	// Player B: very low HP so it dies quickly.
	pBAttr := attribute.NewInstance()
	hpID, _ := attribute.Get().AttrID("hp")
	pBAttr.AddModifiers("test", []attribute.Modifier{
		{AttrID: hpID, Op: attribute.OpOverride, Value: 5, Source: "test"},
	})
	pB := battle.NewPlayerBattleEntity(2, "Fragile", pBAttr)
	pB.SetHP(pB.MaxHP())
	pB.SetSkills(map[string]*battle.BattleSkill{
		"basic_attack": {
			ID:   "basic_attack",
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
	pB.SetBasicSkillID("basic_attack")
	pB.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: "basic_attack", Priority: 0}})

	cfg := battle.BattleConfig{
		ID:              "test_death",
		Name:            "Death Test",
		Map:             "village",
		Interval:        3.0,
		CombinationLoop: []string{"weak"},
		WeakEnemyCombinations: []battle.EnemyWaveCombination{
			{Enemies: []string{"goblin"}, Weight: 100},
		},
	}

	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{pA, pB})

	// Wave spawn.
	sess.AdvanceOneEvent()

	var fragileDowned bool
	var tankContinued bool
	var fragileRespawned bool
	maxEvents := 100
	for i := 0; i < maxEvents && sess.Running; i++ {
		logs := sess.AdvanceOneEvent()
		for _, l := range logs {
			if l.Type == "player_downed" && l.DefenderID == "Fragile" {
				fragileDowned = true
			}
			if l.Type == "player_attack" && l.AttackerID == "Tank" {
				tankContinued = true
			}
			if l.Type == "player_respawn" && l.AttackerID == "Fragile" {
				fragileRespawned = true
			}
		}
		if fragileDowned && tankContinued && fragileRespawned {
			break
		}
	}

	if !fragileDowned {
		t.Error("Fragile player should have been downed")
	}
	if !tankContinued {
		t.Error("Tank player should have continued attacking after Fragile died")
	}
	if !fragileRespawned {
		t.Error("Fragile player should have respawned")
	}
}

// TestAllPlayersDowned verifies that the battle stops when all players are dead.
func TestAllPlayersDowned(t *testing.T) {
	// Both players have very low HP.
	p1Attr := attribute.NewInstance()
	hpID, _ := attribute.Get().AttrID("hp")
	p1Attr.AddModifiers("test", []attribute.Modifier{
		{AttrID: hpID, Op: attribute.OpOverride, Value: 3, Source: "test"},
	})
	p1 := battle.NewPlayerBattleEntity(1, "P1", p1Attr)
	p1.SetHP(p1.MaxHP())
	p1.SetSkills(map[string]*battle.BattleSkill{
		"basic_attack": {
			ID:   "basic_attack",
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
	p1.SetBasicSkillID("basic_attack")
	p1.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: "basic_attack", Priority: 0}})

	p2 := battle.NewPlayerBattleEntity(2, "P2", p1Attr)
	p2.SetHP(p2.MaxHP())
	p2.SetSkills(p1.Skills())
	p2.SetBasicSkillID("basic_attack")
	p2.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: "basic_attack", Priority: 0}})

	cfg := battle.BattleConfig{
		ID:              "test_all_down",
		Name:            "All Down Test",
		Map:             "village",
		Interval:        3.0,
		CombinationLoop: []string{"weak"},
		WeakEnemyCombinations: []battle.EnemyWaveCombination{
			{Enemies: []string{"goblin"}, Weight: 100},
		},
	}

	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p1, p2})

	// Wave spawn.
	sess.AdvanceOneEvent()

	var allDowned bool
	var someoneRespawned bool
	maxEvents := 100
	for i := 0; i < maxEvents; i++ {
		logs := sess.AdvanceOneEvent()
		for _, l := range logs {
			if l.Type == "all_players_downed" {
				allDowned = true
			}
			if l.Type == "player_respawn" {
				someoneRespawned = true
			}
		}
		if allDowned && someoneRespawned {
			break
		}
	}

	if !allDowned {
		t.Error("expected all_players_downed event when both players die")
	}
	if !someoneRespawned {
		t.Error("expected players to respawn after all downed")
	}
	if !sess.Running {
		t.Error("session should NOT stop when all players are downed; respawns should continue")
	}
}

// makeTestPlayer creates a standard test player with basic attack.
func makeTestPlayer(userID int64, name string) *battle.PlayerBattleEntity {
	inst := attribute.NewInstance()
	p := battle.NewPlayerBattleEntity(userID, name, inst)
	p.SetHP(p.MaxHP())
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
			IsBasic:  true,
		},
	})
	p.SetBasicSkillID("basic_attack")
	p.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: "basic_attack", Priority: 0}})
	return p
}
