// Package item defines the runtime item model: identity (Item), definition
// (ItemDef), and modifier parsing. It owns no mutable state, imports only
// attribute for modifier types, and does not depend on gameconfig.
//
// Lookups are provided by gameconfig, which returns item.ItemDef values.
package item

import (
	"fmt"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
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
}

// NewDef creates an ItemDef from parsed config data.
// The maps are consumed (caller should not retain references).
func NewDef(
	id ID, stringID, name string,
	tool, equipment, upgradable bool,
	classification string,
	equipBasic, equipUpgrade, toolBasic, toolUpgrade map[string]float64,
) ItemDef {
	d := ItemDef{
		id: id, stringID: stringID, name: name,
		tool: tool, equipment: equipment, upgradable: upgradable,
		classification: classification,
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

// Modifiers extracts attribute modifiers for this item definition at the
// given instance identity.
//
// When it.State == 0, returns the base modifiers from equipment_*_data /
// tool_*_data. Non-zero State is reserved for future upgrade/forge
// encoding (currently identical to State == 0).
//
// Source labels encode ownership:
//
//	equipment_basic:{stringID}  /  equipment_upgrade:{stringID}
//	tool_basic:{stringID}       /  tool_upgrade:{stringID}
//
// Attribute string IDs that are not registered in attrReg produce an error.
func (d ItemDef) Modifiers(it Item, attrReg *attribute.Registry) ([]attribute.Modifier, error) {
	// Future: use it.State to index upgrade_curve and apply ability_multiplier.
	_ = it.State

	var out []attribute.Modifier

	collect := func(m map[string]float64, source string) error {
		for k, v := range m {
			aid, ok := attrReg.AttrID(k)
			if !ok {
				return fmt.Errorf("item %q: attribute %q not registered", d.stringID, k)
			}
			out = append(out, attribute.Modifier{
				AttrID:  aid,
				Op:      attribute.OpAdd,
				Value:   v,
				Display: displayHint(k),
				Source:  source,
			})
		}
		return nil
	}

	if err := collect(d.equipBasic, "equipment_basic:"+d.stringID); err != nil {
		return nil, err
	}
	if err := collect(d.equipUpgrade, "equipment_upgrade:"+d.stringID); err != nil {
		return nil, err
	}
	if err := collect(d.toolBasic, "tool_basic:"+d.stringID); err != nil {
		return nil, err
	}
	if err := collect(d.toolUpgrade, "tool_upgrade:"+d.stringID); err != nil {
		return nil, err
	}

	return out, nil
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
