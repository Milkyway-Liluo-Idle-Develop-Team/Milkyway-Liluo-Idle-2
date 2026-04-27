package gameconfig

import (
	"encoding/json"
	"fmt"
	"sort"
)

// registry holds the parsed config and its indexes.
type registry struct {
	items   map[string]Item
	events  map[string]Event

	// indexes
	itemsByClass   map[string][]Item
	eventsBySkill  map[string][]Event
	eventsByMap    map[string][]Event
	loopEvents     []Event   // all loop events
	upgradeEvents  []Event   // all upgrade events
}

var reg *registry

// Load parses the embedded actions.json, validates it, and builds the
// in-memory indexes. It is safe to call multiple times (idempotent).
func Load() error {
	if reg != nil {
		return nil
	}

	var cfg ActionConfig
	if err := json.Unmarshal(actionsJSON, &cfg); err != nil {
		return fmt.Errorf("unmarshal actions.json: %w", err)
	}

	r := &registry{
		items:         make(map[string]Item, len(cfg.Items)),
		events:        make(map[string]Event, len(cfg.Events)),
		itemsByClass:  make(map[string][]Item),
		eventsBySkill: make(map[string][]Event),
		eventsByMap:   make(map[string][]Event),
	}

	if err := indexItems(r, cfg.Items); err != nil {
		return err
	}
	if err := indexEvents(r, cfg.Events); err != nil {
		return err
	}
	if err := validate(r); err != nil {
		return err
	}

	// sort every slice so iteration order is deterministic
	for k := range r.itemsByClass {
		sort.Slice(r.itemsByClass[k], func(i, j int) bool {
			return r.itemsByClass[k][i].ID < r.itemsByClass[k][j].ID
		})
	}
	for k := range r.eventsBySkill {
		sort.Slice(r.eventsBySkill[k], func(i, j int) bool {
			return r.eventsBySkill[k][i].ID < r.eventsBySkill[k][j].ID
		})
	}
	for k := range r.eventsByMap {
		sort.Slice(r.eventsByMap[k], func(i, j int) bool {
			return r.eventsByMap[k][i].ID < r.eventsByMap[k][j].ID
		})
	}
	sort.Slice(r.loopEvents, func(i, j int) bool {
		return r.loopEvents[i].ID < r.loopEvents[j].ID
	})
	sort.Slice(r.upgradeEvents, func(i, j int) bool {
		return r.upgradeEvents[i].ID < r.upgradeEvents[j].ID
	})

	reg = r
	return nil
}

func indexItems(r *registry, items []Item) error {
	for _, it := range items {
		if it.ID == "" {
			return fmt.Errorf("item with empty id (name=%q)", it.Name)
		}
		if _, ok := r.items[it.ID]; ok {
			return fmt.Errorf("duplicate item id %q", it.ID)
		}
		r.items[it.ID] = it
		r.itemsByClass[it.Classification] = append(r.itemsByClass[it.Classification], it)
	}
	return nil
}

func indexEvents(r *registry, events []Event) error {
	for _, ev := range events {
		if ev.ID == "" {
			return fmt.Errorf("event with empty id (name=%q)", ev.Name)
		}
		if _, ok := r.events[ev.ID]; ok {
			return fmt.Errorf("duplicate event id %q", ev.ID)
		}
		r.events[ev.ID] = ev

		if ev.Type == EventTypeLoop {
			r.loopEvents = append(r.loopEvents, ev)
		} else if ev.Type == EventTypeUpgrade {
			r.upgradeEvents = append(r.upgradeEvents, ev)
		}

		skill := ev.NeedSkill
		if skill == "" {
			skill = "none"
		}
		r.eventsBySkill[skill] = append(r.eventsBySkill[skill], ev)
		r.eventsByMap[ev.Map] = append(r.eventsByMap[ev.Map], ev)
	}
	return nil
}

func validate(r *registry) error {
	knownSkills := map[string]struct{}{
		"none":      {},
		"felling":   {},
		"mining":    {},
		"crafting":  {},
		"forging":   {},
		"enhancing": {},
	}

	for _, ev := range r.events {
		if ev.NeedSkill != "" {
			if _, ok := knownSkills[ev.NeedSkill]; !ok {
				// Not a hard error; new skills may be added without updating this list.
				// We just note it silently or could log a warning.
			}
		}

		if ev.Type == EventTypeLoop && ev.LoopTime == nil {
			return fmt.Errorf("event %q (type=loop) missing loop_time", ev.ID)
		}

		for _, req := range ev.Requirements {
			switch req.Type {
			case string(ReqTypeItem):
				if _, ok := r.items[req.ID]; !ok {
					return fmt.Errorf("event %q requires unknown item %q", ev.ID, req.ID)
				}
			case string(ReqTypeFluid):
				// Fluids are not declared in the items array; they are ad-hoc
				// resource identifiers. No existence check possible here.
			case string(ReqTypeEvent):
				if _, ok := r.events[req.ID]; !ok {
					return fmt.Errorf("event %q requires unknown event %q", ev.ID, req.ID)
				}
			case string(ReqTypeSkill):
				// skill IDs are open-ended; skip check
			default:
				return fmt.Errorf("event %q has unknown requirement type %q", ev.ID, req.Type)
			}
		}

		for _, rew := range ev.Rewards {
			if rew.IsItem() {
				if _, ok := r.items[rew.ID]; !ok {
					return fmt.Errorf("event %q rewards unknown item %q", ev.ID, rew.ID)
				}
			}
			if rew.IsExperience() && rew.SkillID == "" {
				return fmt.Errorf("event %q experience reward missing skill_id", ev.ID)
			}
		}
	}

	for _, it := range r.items {
		if it.Tool && it.ToolDetails == nil {
			return fmt.Errorf("item %q has tool=true but no tool_details", it.ID)
		}
		if it.Equipment && it.EquipmentDetails == nil {
			return fmt.Errorf("item %q has equipment=true but no equipment_details", it.ID)
		}
		if it.Upgradable && it.UpgradeDetails == nil {
			return fmt.Errorf("item %q has upgradable=true but no upgrade_details", it.ID)
		}
	}

	return nil
}

// --- accessors ---

// GetItem returns an item by its id.
func GetItem(id string) (Item, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetItem")
	}
	it, ok := reg.items[id]
	return it, ok
}

// GetEvent returns an event by its id.
func GetEvent(id string) (Event, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetEvent")
	}
	ev, ok := reg.events[id]
	return ev, ok
}

// AllItems returns every item in deterministic order (sorted by id).
func AllItems() []Item {
	if reg == nil {
		panic("gameconfig: Load() must be called before AllItems")
	}
	out := make([]Item, 0, len(reg.items))
	for _, it := range reg.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// AllEvents returns every event in deterministic order (sorted by id).
func AllEvents() []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before AllEvents")
	}
	out := make([]Event, 0, len(reg.events))
	for _, ev := range reg.events {
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// EventsBySkill returns events whose NeedSkill matches the given skill id.
// Use "none" for skill-less events.
func EventsBySkill(skillID string) []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before EventsBySkill")
	}
	return append([]Event(nil), reg.eventsBySkill[skillID]...)
}

// EventsByMap returns events located on the given map.
func EventsByMap(mapID string) []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before EventsByMap")
	}
	return append([]Event(nil), reg.eventsByMap[mapID]...)
}

// ItemsByClassification returns items of a given classification.
func ItemsByClassification(class string) []Item {
	if reg == nil {
		panic("gameconfig: Load() must be called before ItemsByClassification")
	}
	return append([]Item(nil), reg.itemsByClass[class]...)
}

// LoopEvents returns all loop-type events.
func LoopEvents() []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before LoopEvents")
	}
	return append([]Event(nil), reg.loopEvents...)
}

// UpgradeEvents returns all upgrade-type events.
func UpgradeEvents() []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before UpgradeEvents")
	}
	return append([]Event(nil), reg.upgradeEvents...)
}

// ItemCount returns the number of defined items.
func ItemCount() int {
	if reg == nil {
		panic("gameconfig: Load() must be called before ItemCount")
	}
	return len(reg.items)
}

// EventCount returns the number of defined events.
func EventCount() int {
	if reg == nil {
		panic("gameconfig: Load() before EventCount")
	}
	return len(reg.events)
}
