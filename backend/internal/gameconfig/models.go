// Package gameconfig holds the static game data parsed from actions.json.
// Everything here is read-only after Load(); models use value types so
// callers can safely copy them without aliasing mutable state.
//
// Item identity and runtime definition live in the item package
// (item.ID, item.Item, item.ItemDef). This file only defines the JSON
// parsing shape —itemJSON —plus the Event/Reward/Requirement types.
package gameconfig

import "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"

// --- enum-like types ---

type EventType string

const (
	EventTypeLoop    EventType = "loop"
	EventTypeInstant EventType = "instant"
	EventTypeUpgrade EventType = "upgrade"
)

type ReqType string

const (
	ReqTypeSkill ReqType = "skill"
	ReqTypeItem  ReqType = "item"
	ReqTypeFluid ReqType = "fluid"
	ReqTypeEvent ReqType = "event"
)

// --- raw config ---

type ActionConfig struct {
	Items   []itemJSON   `json:"items"`
	Events  []Event      `json:"events"`
	Enemies []EnemyDef   `json:"enemies"`
	Battles []BattleDef  `json:"battles"`
}

// --- Item JSON shape ---

// toDef converts the parsed JSON into the runtime item.ItemDef.
// Attribute values in _basic_data / _upgrade_data are expected to be
// numeric (JSON numbers). Non-numeric values are silently skipped.
func (ij itemJSON) toDef(id item.ID) item.ItemDef {
	asFloat := func(m map[string]any) map[string]float64 {
		if len(m) == 0 {
			return nil
		}
		out := make(map[string]float64, len(m))
		for k, v := range m {
			switch n := v.(type) {
			case float64:
				out[k] = n
			case int:
				out[k] = float64(n)
			}
		}
		return out
	}

	var eb, eu, tb, tu map[string]float64
	if ij.EquipmentDetails != nil {
		eb = asFloat(ij.EquipmentDetails.EquipmentBasicData)
		eu = asFloat(ij.EquipmentDetails.EquipmentUpgradeData)
	}
	if ij.ToolDetails != nil {
		tb = asFloat(ij.ToolDetails.ToolBasicData)
		tu = asFloat(ij.ToolDetails.ToolUpgradeData)
	}

	// Build upgrade curve nodes.
	var upgradeCurve []item.UpgradeCurveNode
	if ij.UpgradeDetails != nil {
		for _, n := range ij.UpgradeDetails.UpgradeCurve {
			upgradeCurve = append(upgradeCurve, item.UpgradeCurveNode{
				Level:             n.Level,
				RecommendLevel:    n.RecommendLevel,
				BasicSuccessRate:  n.BasicSuccessRate,
				AbilityMultiplier: n.AbilityMultiplier,
			})
		}
	}

	// Build position requirements.
	var equipReqs, toolReqs []item.PositionReq
	if ij.EquipmentDetails != nil {
		for _, r := range ij.EquipmentDetails.EquipmentPositionRequirements {
			v := r.Value
			if v < 1 {
				v = 1
			}
			equipReqs = append(equipReqs, item.PositionReq{Position: r.Position, Value: v})
		}
	}
	if ij.ToolDetails != nil {
		for _, r := range ij.ToolDetails.ToolPositionRequirement {
			v := r.Value
			if v < 1 {
				v = 1
			}
			toolReqs = append(toolReqs, item.PositionReq{Position: r.ToolPosition, Value: v})
		}
	}

	maxUpgrade := 0
	if ij.UpgradeDetails != nil {
		maxUpgrade = ij.UpgradeDetails.MaxUpgrade
	}

	return item.NewDef(
		id, ij.ID, ij.Name,
		ij.Tool, ij.Equipment, ij.Upgradable,
		ij.Classification,
		eb, eu, tb, tu,
		upgradeCurve,
		equipReqs, toolReqs,
		maxUpgrade,
	)
}

type itemJSON struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Tool             bool              `json:"tool"`
	Equipment        bool              `json:"equipment"`
	Upgradable       bool              `json:"upgradable"`
	Classification   string            `json:"classification"`
	ToolDetails      *ToolDetails      `json:"tool_details,omitempty"`
	EquipmentDetails *EquipmentDetails `json:"equipment_details,omitempty"`
	UpgradeDetails   *UpgradeDetails   `json:"upgrade_details,omitempty"`
	Extra            []ExtraProperty   `json:"extra,omitempty"`
}

// ExtraProperty captures open-ended key/value pairs attached to items
// (e.g. coal -> [{id:"heat", value:8}]).
type ExtraProperty struct {
	ID    string  `json:"id"`
	Value float64 `json:"value"`
}

// ToolDetails is present when Item.Tool == true.
type ToolDetails struct {
	ToolPositionRequirement []ToolPositionReq `json:"tool_position_requirement"`
	ToolBasicData           map[string]any    `json:"tool_basic_data"`
	ToolType                string            `json:"tool_type"`
	ToolUpgradeData         map[string]any    `json:"tool_upgrade_data"`
	Requirements            []Requirement     `json:"requirements"`
}

type ToolPositionReq struct {
	ToolPosition string `json:"tool_position"`
	Value        int    `json:"value"`
}

// EquipmentDetails is present when Item.Equipment == true.
type EquipmentDetails struct {
	Type                         string                 `json:"type"` // weapon, wear, relics, ...
	EquipmentPositionRequirements []EquipPositionReq    `json:"equipment_position_requirements"`
	Element                      string                 `json:"element"` // physical, magic, ...
	BattleSkills                 []BattleSkill          `json:"battle_skills"`
	EquipmentBasicData           map[string]any         `json:"equipment_basic_data"`
	EquipmentUpgradeData         map[string]any         `json:"equipment_upgrade_data"`
	Requirements                 []Requirement          `json:"requirements"`
}

type EquipPositionReq struct {
	Position string `json:"position"`
	Value    int    `json:"value"`
}

type BattleSkill struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	TargetType  string         `json:"target_type"`
	Damage      *DamageProfile `json:"damage,omitempty"`
	IsBasic     bool           `json:"is_basic"`

	// Extended fields used by enemy battle skills.
	Cooldown      float64    `json:"cooldown"`
	CastTime      float64    `json:"cast_time"`
	MPCost        float64    `json:"mp_cost"`
	SPCost        float64    `json:"sp_cost"`
	IsSupport     bool       `json:"is_support"`
	Effects       []EffectDef `json:"effects,omitempty"`
	PhysicalStyle string     `json:"physical_style"`
}

type DamageProfile struct {
	Type       string  `json:"type"`
	Flat       float64 `json:"flat"`
	Multiplier float64 `json:"multiplier"`
}

type EffectDef struct {
	Target    string  `json:"target"` // "self" | "target"
	Attribute string  `json:"attribute"`
	Mode      string  `json:"mode"` // "flat" | "percent_mult"
	Value     float64 `json:"value"`
	Duration  float64 `json:"duration,omitempty"`
}

// UpgradeDetails is present when Item.Upgradable == true.
type UpgradeDetails struct {
	MaxUpgrade    int                `json:"max_upgrade"`
	EnhanceSlot   int                `json:"enhance_slot"`
	ForgeSlot     int                `json:"forge_slot"`
	UpgradeCurve  []UpgradeCurveNode `json:"upgrade_curve"`
}

type UpgradeCurveNode struct {
	Level             int     `json:"level"`
	RecommendLevel    int     `json:"recommend_level"`
	BasicSuccessRate  float64 `json:"basic_success_rate"`
	AbilityMultiplier float64 `json:"ability_multiplier"`
}

// --- Event tree ---

type Event struct {
	ID           string        `json:"id"`
	Type         EventType     `json:"type"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	NeedSkill    string        `json:"need_skill"` // "none" if no skill required
	Requirements []Requirement `json:"requirements"`
	Map          string        `json:"map"`
	LoopTime     *float64      `json:"loop_time,omitempty"`
	RepeatTime   *float64      `json:"repeat_time,omitempty"`
	Rewards      []Reward      `json:"rewards,omitempty"`

	// Pre-resolved at load time —never looked up at runtime.
	ResolvedSkillID      SkillID
	ProductionAttrName   string // "{NeedSkill}_production_multiplier", set during Load
}

// --- Requirement ---

type Requirement struct {
	Type            string   `json:"type"`
	ID              string   `json:"id"`
	ComparisonTypes *string  `json:"comparison_types,omitempty"`
	Value           *float64 `json:"value,omitempty"`

	// Pre-resolved at load time.
	ResolvedID   int64      // skill ID or event ID (from StringToSkillID / StringToEventID)
	ResolvedItem item.Item  // item identity (from StringToItemID) for item/fluid types
}

// IsConsumption reports whether this requirement deducts resources on
// execution (item/fluid with no comparison operator).
func (r Requirement) IsConsumption() bool {
	return (r.Type == string(ReqTypeItem) || r.Type == string(ReqTypeFluid)) &&
		r.ComparisonTypes == nil
}

// IsThreshold reports whether this requirement is a gate that must be
// satisfied but does not consume resources.
func (r Requirement) IsThreshold() bool {
	return !r.IsConsumption()
}

// --- Reward ---

type Reward struct {
	Type    string  `json:"type"`     // "" for item, "experience" for XP
	ID      string  `json:"id"`       // item id when Type=="" ; skill id context when Type=="experience"
	Num     float64 `json:"num"`      // item quantity (preferred)
	Value   float64 `json:"value"`    // fallback quantity or XP value
	SkillID string  `json:"skill_id"` // target skill for XP rewards

	// Pre-resolved at load time.
	ResolvedItem    item.Item // item identity for item rewards
	ResolvedSkillID SkillID   // from StringToSkillID for XP rewards
}

// IsExperience reports whether this reward grants skill experience.
func (r Reward) IsExperience() bool { return r.Type == "experience" }

// IsItem reports whether this reward grants an item.
func (r Reward) IsItem() bool { return r.Type == "" }

// ItemQuantity returns the resolved quantity for an item reward.
// If Num is zero, falls back to Value for legacy format compatibility.
func (r Reward) ItemQuantity() float64 {
	if r.Num != 0 {
		return r.Num
	}
	return r.Value
}

// Experience returns the XP value and target skill for an experience reward.
func (r Reward) Experience() (value float64, skillID string) {
	return r.Value, r.SkillID
}

// --- Enemy definitions ---

// EnemyDef is a read-only enemy definition loaded from actions.json.
type EnemyDef struct {
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	BattleData      map[string]float64      `json:"enemy_battle_data"`
	BasicDamageType string                  `json:"basic_damage_type"`
	Rewards         []EnemyReward           `json:"rewards"`
	BattleSkills    []EnemyBattleSkillEntry `json:"battle_skill"`
	BasicSkillID    string                  `json:"basic_skill_id"`
}

// EnemyReward is a single drop or XP reward from an enemy.
type EnemyReward struct {
	Type           string  // "" for item, "experience" for XP
	ID             string
	Num            float64
	Value          float64
	ResolvedItem   item.Item
	ResolvedSkillID SkillID
}

// IsExperience reports whether this reward grants skill experience.
func (r EnemyReward) IsExperience() bool { return r.Type == "experience" }

// IsItem reports whether this reward grants an item.
func (r EnemyReward) IsItem() bool { return r.Type == "" }

// ItemQuantity returns the resolved quantity.
func (r EnemyReward) ItemQuantity() float64 {
	if r.Num != 0 {
		return r.Num
	}
	return r.Value
}

// Experience returns the XP value and target skill.
func (r EnemyReward) Experience() (value float64, skillID string) {
	return r.Value, r.ID
}

// EnemyBattleSkillEntry is a single skill in an enemy's battle plan.
type EnemyBattleSkillEntry struct {
	Skill     BattleSkill
	Priority  int
	Condition *SkillConditionDef
}

// SkillConditionDef defines when a skill may be used.
type SkillConditionDef struct {
	LogicType        string               `json:"logic_type"`
	NormalCondition  []SimpleConditionDef `json:"normal_condition"`
	ComplexCondition *SkillConditionDef   `json:"complex_condition"`
}

// SimpleConditionDef is a single numeric comparison.
type SimpleConditionDef struct {
	Key            string  `json:"key"`
	ComparisonType string  `json:"comparison_type"`
	Value          float64 `json:"value"`
}


// --- Battle definitions ---

// BattleDef is a read-only battle scene definition loaded from actions.json.
type BattleDef struct {
	ID                       string                  `json:"id"`
	Name                     string                  `json:"name"`
	Map                      string                  `json:"map"`
	Interval                 float64                 `json:"interval"`
	CombinationLoop          []string                `json:"combination_loop"`
	WeakEnemyCombinations    []EnemyWaveCombination  `json:"weak_enemy_combinations,omitempty"`
	StrongEnemyCombinations  []EnemyWaveCombination  `json:"strong_enemy_combinations,omitempty"`
	BossEnemyCombinations    []EnemyWaveCombination  `json:"boss_enemy_combinations,omitempty"`
}

// EnemyWaveCombination is a single weighted enemy group for a wave.
type EnemyWaveCombination struct {
	Enemies []string `json:"enemies"`
	Weight  float64  `json:"weight"`
}

// CombinationsMap returns the combinations grouped by wave type.
func (b BattleDef) CombinationsMap() map[string][]EnemyWaveCombination {
	return map[string][]EnemyWaveCombination{
		"weak":   b.WeakEnemyCombinations,
		"strong": b.StrongEnemyCombinations,
		"boss":   b.BossEnemyCombinations,
	}
}
