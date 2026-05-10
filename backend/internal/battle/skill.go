package battle

import (
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

// BattleSkill is a runtime combat skill.
type BattleSkill struct {
	ID          gameconfig.BattleSkillID
	Name        string
	Description string
	TargetType  string // "single" | "aoe" | "self"

	// Damage configuration. Nil for support skills.
	Damage *DamageProfile

	// Resource costs.
	MPCost float64
	SPCost float64

	// Timing.
	CastTime float64 // action duration in seconds
	Cooldown float64 // cooldown in seconds

	// Effects applied on cast.
	Effects []SkillEffect

	// Activation condition (player skill plan only).
	Condition *SkillCondition

	// Flags.
	IsBasic       bool   // default basic attack
	IsSupport     bool   // no damage, only effects
	PhysicalStyle string // "melee" | "ranged" — affects XP distribution
}

// DamageProfile describes how a skill computes its base damage.
type DamageProfile struct {
	Type       string  // "physical" | "magic"
	Flat       float64 // flat bonus added before multiplier
	Multiplier float64 // multiplier on attack power
}

// SkillEffect is an effect applied by a skill on cast or on hit.
type SkillEffect struct {
	Target    string                // "self" | "target"
	Attribute attribute.AttributeID
	Mode      EffectMode
	Value     float64
	Duration  float64 // seconds; 0 = instantaneous
}

// SkillPlanEntry is a single entry in a skill priority plan.
type SkillPlanEntry struct {
	SkillID   gameconfig.BattleSkillID
	Priority  int
	Condition *SkillCondition
}

// SkillCondition defines when a skill may be used.
// Logic may nest via Complex.
type SkillCondition struct {
	Logic   string            // "and" | "or" | "nor"
	Normal  []SimpleCondition // flat list of simple checks
	Complex *SkillCondition   // nested condition
}

// SimpleCondition is a single numeric comparison.
type SimpleCondition struct {
	Key        string  // e.g. "self_hp", "target_hp_ratio", "any_enemy_hp"
	Comparison string  // ">" | ">=" | "<" | "<=" | "="
	Value      float64
}
