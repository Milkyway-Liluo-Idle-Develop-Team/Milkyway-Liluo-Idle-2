// Package gameconfig holds the static game data parsed from actions.json.
// Everything here is read-only after Load(); models use value types so
// callers can safely copy them without aliasing mutable state.
package gameconfig

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
	Items  []Item  `json:"items"`
	Events []Event `json:"events"`
}

// --- Item tree ---

type Item struct {
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
	Damage      DamageProfile  `json:"damage"`
	IsBasic     bool           `json:"is_basic"`
}

type DamageProfile struct {
	Type       string  `json:"type"`
	Flat       float64 `json:"flat"`
	Multiplier float64 `json:"multiplier"`
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
	LoopTime     *float64      `json:"loop_time,omitempty"`    // seconds per cycle; set when Type==Loop
	RepeatTime   *float64      `json:"repeat_time,omitempty"`  // seconds between repeats; set on repeatable upgrades
	Rewards      []Reward      `json:"rewards,omitempty"`
}

// --- Requirement ---

type Requirement struct {
	Type            string   `json:"type"`
	ID              string   `json:"id"`
	ComparisonTypes *string  `json:"comparison_types,omitempty"`
	Value           *float64 `json:"value,omitempty"`
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
	Type    string  `json:"type"`    // "" for item, "experience" for XP
	ID      string  `json:"id"`      // item id when Type=="" ; skill id context when Type=="experience"
	Num     float64 `json:"num"`     // item quantity (preferred)
	Value   float64 `json:"value"`   // fallback quantity or XP value
	SkillID string  `json:"skill_id"`// target skill for XP rewards
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
