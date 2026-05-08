// Package item defines the runtime item model: identity (Item), definition
// (ItemDef), and modifier parsing. It owns no mutable state, imports only
// attribute for modifier types, and does not depend on gameconfig.
//
// Lookups are provided by gameconfig, which returns item.ItemDef values.
package item

import (
	"fmt"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
)

// --- item identity ---

// ID is a stable numeric identifier for an item, assigned by genregistry.
// Zero means invalid.
type ID int32

// String returns a debug representation.
func (id ID) String() string {
	if id == 0 {
		return "<invalid-item>"
	}
	return fmt.Sprintf("<item-%d>", id)
}

// State is the 32-bit variant descriptor for an item instance.
// Zero is the canonical default (base item); non-zero encodes
// upgrade level, forge affix, or other game-defined variation.
type State int32

// Item is the complete identity of an item instance.
// Both ID and State are required —State=0 is the default state,
// not a sentinel meaning "absent".
type Item struct {
	ID    ID
	State State
}

// String returns a compact representation like "19/0" or "19/10".
func (it Item) String() string {
	return it.ID.String() + "/" + stateStr(it.State)
}

func stateStr(s State) string {
	if s == 0 {
		return "0"
	}
	return fmt.Sprintf("%d", s)
}

// --- item definition ---

// UpgradeCurveNode defines a single point on an item's upgrade curve.
type UpgradeCurveNode struct {
	Level             int
	RecommendLevel    int
	BasicSuccessRate  float64
	AbilityMultiplier float64
}

// PositionReq defines how many slots of a given base position an item needs.
type PositionReq struct {
	Position string
	Value    int
}

// ItemDef is a read-only item definition loaded from actions.json.
// It bridges the config layer with runtime modifier parsing.
type ItemDef struct {
	id             ID
	stringID       string
	name           string
	tool           bool
	equipment      bool
	upgradable     bool
	classification string

	// Pre-parsed attribute data for Modifiers. Keys are attribute string IDs.
	equipBasic   map[string]float64
	equipUpgrade map[string]float64
	toolBasic    map[string]float64
	toolUpgrade  map[string]float64

	// Upgrade curve for ability_multiplier interpolation.
	upgradeCurve []UpgradeCurveNode

	// Position requirements for multi-slot support.
	equipPositionReqs []PositionReq
	toolPositionReqs  []PositionReq

	// Maximum upgrade level (0 = not upgradable).
	maxUpgrade int
}

// NewDef creates an ItemDef from parsed config data.
// The maps are consumed (caller should not retain references).
func NewDef(
	id ID, stringID, name string,
	tool, equipment, upgradable bool,
	classification string,
	equipBasic, equipUpgrade, toolBasic, toolUpgrade map[string]float64,
	upgradeCurve []UpgradeCurveNode,
	equipPositionReqs, toolPositionReqs []PositionReq,
	maxUpgrade int,
) ItemDef {
	d := ItemDef{
		id: id, stringID: stringID, name: name,
		tool: tool, equipment: equipment, upgradable: upgradable,
		classification: classification,
		upgradeCurve:   upgradeCurve,
		maxUpgrade:     maxUpgrade,
	}
	if len(equipBasic) > 0 {
		d.equipBasic = equipBasic
	}
	if len(equipUpgrade) > 0 {
		d.equipUpgrade = equipUpgrade
	}
	if len(toolBasic) > 0 {
		d.toolBasic = toolBasic
	}
	if len(toolUpgrade) > 0 {
		d.toolUpgrade = toolUpgrade
	}
	if len(equipPositionReqs) > 0 {
		d.equipPositionReqs = equipPositionReqs
	}
	if len(toolPositionReqs) > 0 {
		d.toolPositionReqs = toolPositionReqs
	}
	return d
}

func (d ItemDef) ID() ID                 { return d.id }
func (d ItemDef) StringID() string       { return d.stringID }
func (d ItemDef) Name() string           { return d.name }
func (d ItemDef) Classification() string { return d.classification }
func (d ItemDef) IsEquipment() bool      { return d.equipment }
func (d ItemDef) IsTool() bool           { return d.tool }
func (d ItemDef) IsUpgradable() bool     { return d.upgradable }
func (d ItemDef) IsFluid() bool          { return d.classification == "fluid" }
func (d ItemDef) MaxUpgrade() int        { return d.maxUpgrade }

// EquipPositionReqs returns the equipment slot position requirements.
func (d ItemDef) EquipPositionReqs() []PositionReq { return d.equipPositionReqs }

// ToolPositionReqs returns the tool slot position requirements.
func (d ItemDef) ToolPositionReqs() []PositionReq { return d.toolPositionReqs }

// AbilityMultiplier returns the interpolated ability multiplier for the given
// enhance level.  Defaults to 1.0 at level 0 when no curve is defined.
func (d ItemDef) AbilityMultiplier(enhanceLevel int) float64 {
	if len(d.upgradeCurve) == 0 {
		return 1.0
	}
	lv := enhanceLevel
	if lv <= d.upgradeCurve[0].Level {
		return d.upgradeCurve[0].AbilityMultiplier
	}
	for i := 1; i < len(d.upgradeCurve); i++ {
		left := d.upgradeCurve[i-1]
		right := d.upgradeCurve[i]
		if lv > right.Level {
			continue
		}
		if right.Level == left.Level {
			return right.AbilityMultiplier
		}
		ratio := float64(lv-left.Level) / float64(right.Level-left.Level)
		return left.AbilityMultiplier + (right.AbilityMultiplier-left.AbilityMultiplier)*ratio
	}
	return d.upgradeCurve[len(d.upgradeCurve)-1].AbilityMultiplier
}

// Modifiers extracts attribute modifiers for this item definition at the
// given instance identity.
//
// Values are scaled by ability_multiplier derived from it.State (enhance
// level) and the item's upgrade_curve:
//
//	final = basic_data[attr] + upgrade_data[attr] * ability_multiplier
//
// Source labels encode ownership:
//
//	equipment_basic:{stringID}  /  equipment_upgrade:{stringID}
//	tool_basic:{stringID}       /  tool_upgrade:{stringID}
//
// Attribute string IDs that are not registered in attrReg produce an error.
func (d ItemDef) Modifiers(it Item, attrReg *attribute.Registry) ([]attribute.Modifier, error) {
	ability := d.AbilityMultiplier(int(it.State))

	var out []attribute.Modifier

	collect := func(basic, upgrade map[string]float64, sourceBase string) error {
		allKeys := make(map[string]struct{}, len(basic)+len(upgrade))
		for k := range basic {
			allKeys[k] = struct{}{}
		}
		for k := range upgrade {
			allKeys[k] = struct{}{}
		}
		for k := range allKeys {
			aid, ok := attrReg.AttrID(k)
			if !ok {
				return fmt.Errorf("item %q: attribute %q not registered", d.stringID, k)
			}
			baseVal := basic[k]
			upVal := upgrade[k]
			finalVal := baseVal + upVal*ability
			if abs(finalVal) < 1e-12 {
				continue
			}
			out = append(out, attribute.Modifier{
				AttrID:  aid,
				Op:      attribute.OpAdd,
				Value:   finalVal,
				Display: displayHint(k),
				Source:  sourceBase + ":" + d.stringID,
			})
		}
		return nil
	}

	if err := collect(d.equipBasic, d.equipUpgrade, "equipment"); err != nil {
		return nil, err
	}
	if err := collect(d.toolBasic, d.toolUpgrade, "tool"); err != nil {
		return nil, err
	}

	return out, nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func displayHint(attrID string) attribute.DisplayMode {
	switch attrID {
	case "felling_production_multiplier",
		"mining_production_multiplier",
		"crafting_production_multiplier",
		"exp_gain_multiplier",
		"hatred_multiplier":
		return attribute.DisplayPercent
	default:
		return attribute.DisplayFixed
	}
}
