package battle

import (
	"math/rand"
)

// DamageResult holds the full outcome of a single damage calculation.
type DamageResult struct {
	Damage           float64 // actual damage dealt after all mitigation
	RawDamage        float64 // damage before block/crit (for logs)
	DamageType       string  // "physical" | "magic"
	Evaded           bool
	Blocked          bool
	BlockedReduction float64 // amount reduced by block
}

// CalcDamage computes the damage from attacker to defender using the given skill.
// rng is the random source; if nil, the global default is used.
func CalcDamage(attacker, defender BattleEntity, skill *BattleSkill, rng *rand.Rand) DamageResult {
	if rng == nil {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}

	dmgCfg := skill.Damage
	if dmgCfg == nil {
		return DamageResult{DamageType: "none"}
	}

	damageType := dmgCfg.Type
	if damageType == "" {
		damageType = "physical"
	}

	// ----- Evade check -----
	// evade_rate = 1 / (1 + (clamp(acc/evade, 0, 10)^2))
	acc := max(1e-6, attacker.GetFinal(AttrAccuracy))
	evade := max(1e-6, defender.GetFinal(AttrEvade))
	evadeRatio := acc / evade
	if evadeRatio > 10 {
		evadeRatio = 10
	}
	evadePossibility := 1.0 / (1.0 + evadeRatio*evadeRatio)

	// apply possibility multipliers
	accMult := 1.0 + attacker.GetFinal(AttrAccuracyPossibilityMultiplier)
	evadeMult := 1.0 + defender.GetFinal(AttrEvadePossibilityMultiplier)
	evadePossibility = evadePossibility * evadeMult / accMult
	if evadePossibility < 0 {
		evadePossibility = 0
	}
	if evadePossibility > 1 {
		evadePossibility = 1
	}

	if rng.Float64() < evadePossibility {
		return DamageResult{
			Damage:     0,
			RawDamage:  0,
			DamageType: damageType,
			Evaded:     true,
		}
	}

	// ----- Random variance -----
	randRatio := 0.9 + rng.Float64()*0.2

	// ----- Base damage -----
	var originalDamage float64

	if damageType == "magic" {
		// Magic: rand * magic_power / (1 + magic_instance) * (1+fdm) / (1+fdr)
		magicPower := max(0.0, dmgCfg.Flat+attacker.GetFinal(AttrMagicPower)*dmgCfg.Multiplier)
		magicInstance := max(0.0, defender.GetFinal(AttrMagicInstance))
		originalDamage = randRatio * magicPower / (1.0 + magicInstance)
	} else {
		// Physical: rand * attack_power^2 / (attack_power + defense) * (1+fdm) / (1+fdr)
		attackPower := max(0.0, dmgCfg.Flat+attacker.GetFinal(AttrPhysicalPower)*dmgCfg.Multiplier)
		defense := max(0.0, defender.GetFinal(AttrDefense))
		originalDamage = randRatio * (attackPower * attackPower) / (attackPower + defense)
	}

	fdm := attacker.GetFinal(AttrFinalDamageMultiplier)
	fdr := defender.GetFinal(AttrFinalDamageReduce)
	originalDamage *= (1.0 + fdm) / (1.0 + fdr)
	originalDamage = max(0.0, originalDamage)

	// ----- Block check (physical only) -----
	blocked := false
	blockedReduction := 0.0
	if damageType != "magic" {
		block := max(0.0, defender.GetFinal(AttrBlock))
		blockPosMult := max(0.0, defender.GetFinal(AttrBlockPossibilityMultiplier))
		blockPossibility := 1.0 - 100.0/(100.0+block)/(1.0+blockPosMult)
		if blockPossibility < 0 {
			blockPossibility = 0
		}
		if blockPossibility > 1 {
			blockPossibility = 1
		}

		if rng.Float64() < blockPossibility {
			blocked = true
			blockRate := max(1.0, defender.GetFinal(AttrBlockRate))
			blockedDamage := max(0.0, originalDamage/blockRate)
			blockedReduction = originalDamage - blockedDamage
			originalDamage = blockedDamage
		}
	}

	// ----- Critical check (physical only) -----
	if damageType != "magic" {
		crit := attacker.GetFinal(AttrCritical)
		if crit < 0 {
			crit = 0
		}
		if crit > 1 {
			crit = 1
		}
		if rng.Float64() < crit {
			critRate := max(1.0, attacker.GetFinal(AttrCriticalRate))
			originalDamage *= critRate
		}
	}

	originalDamage = max(0.0, originalDamage)

	return DamageResult{
		Damage:           originalDamage,
		RawDamage:        originalDamage + blockedReduction,
		DamageType:       damageType,
		Evaded:           false,
		Blocked:          blocked,
		BlockedReduction: blockedReduction,
	}
}
