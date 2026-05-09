package battle

import "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"

// EffectMode describes how an effect modifies its target attribute.
type EffectMode int

const (
	EffectModeFlat EffectMode = iota       // flat addition / subtraction
	EffectModePercentMult                  // percent multiplier (1 + value)
)

// ActiveEffect is a runtime buff or debuff attached to a BattleEntity.
type ActiveEffect struct {
	SourceSkillID string                // source skill, used for overwrite semantics
	Attribute     attribute.AttributeID // target attribute
	Mode          EffectMode
	Value         float64
	ExpiresAt     *float64 // nil = permanent until battle end
}

// effectKey returns a deterministic key for deduplication / overwrite.
func (e ActiveEffect) effectKey() string {
	return e.SourceSkillID + ":" + e.Attribute.String() + ":" + modeString(e.Mode)
}

func modeString(m EffectMode) string {
	switch m {
	case EffectModeFlat:
		return "flat"
	case EffectModePercentMult:
		return "percent_mult"
	default:
		return "unknown"
	}
}

// rescaleResources scales current HP/MP/SP proportionally when maxima change.
// oldMax values must be captured *before* any modifiers that affect maxima are applied.
func rescaleResources(e BattleEntity, oldMaxHP, oldMaxMP, oldMaxSP float64) {
	oldMaxHP = max(1e-6, oldMaxHP)
	oldMaxMP = max(1e-6, oldMaxMP)
	oldMaxSP = max(1e-6, oldMaxSP)

	newMaxHP := max(0.0, e.MaxHP())
	newMaxMP := max(0.0, e.MaxMP())
	newMaxSP := max(0.0, e.MaxSP())

	e.SetHP(scaleCurrent(e.HP(), oldMaxHP, newMaxHP))
	e.SetMP(scaleCurrent(e.MP(), oldMaxMP, newMaxMP))
	e.SetSP(scaleCurrent(e.SP(), oldMaxSP, newMaxSP))
}

func scaleCurrent(cur, oldMax, newMax float64) float64 {
	if newMax <= 0 {
		return 0
	}
	ratio := cur / oldMax
	v := newMax * ratio
	if v < 0 {
		return 0
	}
	if v > newMax {
		return newMax
	}
	return v
}
