package battle_test

import (
	"math/rand"
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

func init() {
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic(err)
		}
	}
	if err := gameconfig.Load(); err != nil {
		panic(err)
	}
	battle.LoadAttrIDs()
}

func TestEnemyEntityMaxHP(t *testing.T) {
	// Enemy with base HP = 200.
	e := battle.NewEnemyBattleEntity("goblin", "goblin_0", "Goblin", map[string]float64{
		"hp": 200,
	})
	if e.MaxHP() != 200 {
		t.Errorf("MaxHP want 200, got %v", e.MaxHP())
	}
}

func TestPlayerEntityMaxHPWithModifiers(t *testing.T) {
	inst := attribute.NewInstance()
	// Default HP is 100.
	p := battle.NewPlayerBattleEntity(1, "Player", inst)
	if p.MaxHP() != 100 {
		t.Errorf("MaxHP want 100, got %v", p.MaxHP())
	}

	// Add +50 HP from equipment.
	hpID, _ := attribute.Get().AttrID("hp")
	inst.AddModifiers("equipment:armor", []attribute.Modifier{
		{AttrID: hpID, Op: attribute.OpAdd, Value: 50, Source: "equipment:armor"},
	})
	if p.MaxHP() != 150 {
		t.Errorf("MaxHP want 150, got %v", p.MaxHP())
	}
}

func TestRefreshStatsScalesHP(t *testing.T) {
	inst := attribute.NewInstance()
	p := battle.NewPlayerBattleEntity(1, "Player", inst)
	p.SetHP(50) // 50% of default 100

	// Increase max HP by 100 → new max 200.
	hpID, _ := attribute.Get().AttrID("hp")
	inst.AddModifiers("buff:giant", []attribute.Modifier{
		{AttrID: hpID, Op: attribute.OpAdd, Value: 100, Source: "buff:giant"},
	})

	p.RefreshStats(0)
	if p.MaxHP() != 200 {
		t.Errorf("MaxHP want 200, got %v", p.MaxHP())
	}
	// HP should scale proportionally: 50/100 * 200 = 100.
	if p.HP() != 100 {
		t.Errorf("HP want 100, got %v", p.HP())
	}
}

func TestCalcDamagePhysical(t *testing.T) {
	// Create two enemies: attacker and defender.
	attacker := battle.NewEnemyBattleEntity("a", "a_0", "Attacker", map[string]float64{
		"physical_power": 100,
		"accuracy":       100,
		"critical":       0,
		"final_damage_multiplier": 0,
	})
	attacker.SetHP(100)

	defender := battle.NewEnemyBattleEntity("d", "d_0", "Defender", map[string]float64{
		"defense":               50,
		"evade":                 1, // very low evade
		"block":                 0,
		"final_damage_reduce":   0,
		"magic_instance":        0,
	})
	defender.SetHP(100)

	skill := &battle.BattleSkill{
		ID:   "basic_attack",
		Name: "Basic Attack",
		Damage: &battle.DamageProfile{
			Type:       "physical",
			Flat:       0,
			Multiplier: 1.0,
		},
	}

	// Deterministic RNG: 0.5 for all rolls.
	// randRatio = 0.9 + 0.5*0.2 = 1.0
	// attackPower = 100
	// damage = 1.0 * 100^2 / (100+50) = 10000/150 ≈ 66.67
	rng := rand.New(rand.NewSource(42))
	result := battle.CalcDamage(attacker, defender, skill, rng)

	if result.Evaded {
		t.Fatal("should not evade with high accuracy vs low evade")
	}
	if result.Damage <= 0 {
		t.Fatalf("damage should be positive, got %v", result.Damage)
	}
	// With seed 42, we can verify exact value if needed.
	t.Logf("physical damage: %v", result.Damage)
}

func TestCalcDamageMagic(t *testing.T) {
	attacker := battle.NewEnemyBattleEntity("a", "a_0", "Attacker", map[string]float64{
		"magic_power":             80,
		"accuracy":                100,
		"final_damage_multiplier": 0,
	})
	defender := battle.NewEnemyBattleEntity("d", "d_0", "Defender", map[string]float64{
		"magic_instance":        0.5,
		"evade":                 1,
		"final_damage_reduce":   0,
	})

	skill := &battle.BattleSkill{
		ID:   "fireball",
		Name: "Fireball",
		Damage: &battle.DamageProfile{
			Type:       "magic",
			Flat:       0,
			Multiplier: 1.0,
		},
	}

	rng := rand.New(rand.NewSource(42))
	result := battle.CalcDamage(attacker, defender, skill, rng)

	if result.Evaded {
		t.Fatal("should not evade")
	}
	if result.Damage <= 0 {
		t.Fatalf("magic damage should be positive, got %v", result.Damage)
	}
	// Magic damage should NOT be affected by block or crit.
	if result.Blocked {
		t.Error("magic damage should not be blocked")
	}
	t.Logf("magic damage: %v", result.Damage)
}

func TestApplyEffectSyncsToAttr(t *testing.T) {
	inst := attribute.NewInstance()
	p := battle.NewPlayerBattleEntity(1, "Player", inst)

	ppID, _ := attribute.Get().AttrID("physical_power")
	before := p.GetFinal(ppID)

	eff := battle.ActiveEffect{
		SourceSkillID: "buff:strength",
		Attribute:     ppID,
		Mode:          battle.EffectModeFlat,
		Value:         20,
	}
	p.ApplyEffect(eff, 0)

	after := p.GetFinal(ppID)
	if after != before+20 {
		t.Errorf("physical_power want %v, got %v", before+20, after)
	}
}
