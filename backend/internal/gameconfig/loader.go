package gameconfig

import (
	"encoding/json"
	"fmt"
	"sort"
)

// registry holds the parsed config, numeric ID mappings, and indexes.
type registry struct {
	// --- raw string maps (external API / wire format) ---
	items  map[string]Item
	events map[string]Event

	// --- id registry (source of truth for numeric ids) ---
	idReg *IDRegistry

	// --- numeric-id indexes (internal / DB) ---
	itemsByID      map[ItemID]Item
	eventsByID     map[EventID]Event
	itemsByClass   map[string][]Item
	eventsBySkill  map[SkillID][]Event
	eventsByMap    map[MapID][]Event
	loopEvents     []Event
	upgradeEvents  []Event
}

var reg *registry

// Load parses the embedded actions.json and id_registry.json, validates
// consistency, and builds the in-memory indexes.  It is safe to call
// multiple times (idempotent).
func Load() error {
	if reg != nil {
		return nil
	}

	var cfg ActionConfig
	if err := json.Unmarshal(actionsJSON, &cfg); err != nil {
		return fmt.Errorf("unmarshal actions.json: %w", err)
	}

	idReg, err := loadRegistry()
	if err != nil {
		return err
	}

	if err := checkConsistency(idReg, &cfg); err != nil {
		return err
	}

	r := &registry{
		items:         make(map[string]Item, len(cfg.Items)),
		events:        make(map[string]Event, len(cfg.Events)),
		idReg:         idReg,
		itemsByID:     make(map[ItemID]Item),
		eventsByID:    make(map[EventID]Event),
		itemsByClass:  make(map[string][]Item),
		eventsBySkill: make(map[SkillID][]Event),
		eventsByMap:   make(map[MapID][]Event),
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
		id := ItemID(r.idReg.Items[it.ID])
		r.itemsByID[id] = it
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
		eid := EventID(r.idReg.Events[ev.ID])
		r.eventsByID[eid] = ev

		if ev.Type == EventTypeLoop {
			r.loopEvents = append(r.loopEvents, ev)
		} else if ev.Type == EventTypeUpgrade {
			r.upgradeEvents = append(r.upgradeEvents, ev)
		}

		skill := SkillID(r.idReg.Skills[ev.NeedSkill])
		r.eventsBySkill[skill] = append(r.eventsBySkill[skill], ev)

		mid := MapID(r.idReg.Maps[ev.Map])
		r.eventsByMap[mid] = append(r.eventsByMap[mid], ev)
	}
	return nil
}

func validate(r *registry) error {
	for _, ev := range r.events {
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
				// ad-hoc identifiers; no existence table
			case string(ReqTypeEvent):
				if _, ok := r.events[req.ID]; !ok {
					return fmt.Errorf("event %q requires unknown event %q", ev.ID, req.ID)
				}
			case string(ReqTypeSkill):
				// open-ended
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

// =========================
// String-based accessors (for external API / wire format)
// =========================

// GetItem returns an item by its string id.
func GetItem(id string) (Item, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetItem")
	}
	it, ok := reg.items[id]
	return it, ok
}

// GetEvent returns an event by its string id.
func GetEvent(id string) (Event, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetEvent")
	}
	ev, ok := reg.events[id]
	return ev, ok
}

// AllItems returns every item in deterministic order (sorted by string id).
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

// AllEvents returns every event in deterministic order (sorted by string id).
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

// ItemsByClassification returns items of a given classification.
func ItemsByClassification(class string) []Item {
	if reg == nil {
		panic("gameconfig: Load() must be called before ItemsByClassification")
	}
	out := append([]Item(nil), reg.itemsByClass[class]...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
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

// =========================
// Numeric ID accessors (for internal settlement / DB)
// =========================

// --- ItemID ---

// StringToItemID converts a string item id to its numeric id.
func StringToItemID(s string) (ItemID, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before StringToItemID")
	}
	id, ok := reg.idReg.Items[s]
	return ItemID(id), ok
}

// ItemIDToString converts a numeric item id back to its string id.
func ItemIDToString(id ItemID) (string, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before ItemIDToString")
	}
	// Linear scan is fine for development/debug; for hot paths use a reverse map.
	for s, v := range reg.idReg.Items {
		if v == int64(id) {
			return s, true
		}
	}
	return "", false
}

// GetItemByID returns an item by its numeric id.
func GetItemByID(id ItemID) (Item, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetItemByID")
	}
	it, ok := reg.itemsByID[id]
	return it, ok
}

// AllItemIDs returns all numeric item ids in ascending order.
func AllItemIDs() []ItemID {
	if reg == nil {
		panic("gameconfig: Load() must be called before AllItemIDs")
	}
	out := make([]ItemID, 0, len(reg.itemsByID))
	for id := range reg.itemsByID {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// --- EventID ---

// StringToEventID converts a string event id to its numeric id.
func StringToEventID(s string) (EventID, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before StringToEventID")
	}
	id, ok := reg.idReg.Events[s]
	return EventID(id), ok
}

// EventIDToString converts a numeric event id back to its string id.
func EventIDToString(id EventID) (string, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before EventIDToString")
	}
	for s, v := range reg.idReg.Events {
		if v == int64(id) {
			return s, true
		}
	}
	return "", false
}

// GetEventByID returns an event by its numeric id.
func GetEventByID(id EventID) (Event, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetEventByID")
	}
	ev, ok := reg.eventsByID[id]
	return ev, ok
}

// AllEventIDs returns all numeric event ids in ascending order.
func AllEventIDs() []EventID {
	if reg == nil {
		panic("gameconfig: Load() must be called before AllEventIDs")
	}
	out := make([]EventID, 0, len(reg.eventsByID))
	for id := range reg.eventsByID {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// --- SkillID ---

// StringToSkillID converts a string skill id to its numeric id.
func StringToSkillID(s string) (SkillID, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before StringToSkillID")
	}
	id, ok := reg.idReg.Skills[s]
	return SkillID(id), ok
}

// SkillIDToString converts a numeric skill id back to its string id.
func SkillIDToString(id SkillID) (string, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before SkillIDToString")
	}
	for s, v := range reg.idReg.Skills {
		if v == int64(id) {
			return s, true
		}
	}
	return "", false
}

// AllSkillIDs returns all numeric skill ids in ascending order.
func AllSkillIDs() []SkillID {
	if reg == nil {
		panic("gameconfig: Load() must be called before AllSkillIDs")
	}
	out := make([]SkillID, 0, len(reg.idReg.Skills))
	for _, id := range reg.idReg.Skills {
		out = append(out, SkillID(id))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// --- MapID ---

// StringToMapID converts a string map id to its numeric id.
func StringToMapID(s string) (MapID, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before StringToMapID")
	}
	id, ok := reg.idReg.Maps[s]
	return MapID(id), ok
}

// MapIDToString converts a numeric map id back to its string id.
func MapIDToString(id MapID) (string, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before MapIDToString")
	}
	for s, v := range reg.idReg.Maps {
		if v == int64(id) {
			return s, true
		}
	}
	return "", false
}

// --- BattleSkillID ---

// StringToBattleSkillID converts a string battle skill id to its numeric id.
func StringToBattleSkillID(s string) (BattleSkillID, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before StringToBattleSkillID")
	}
	id, ok := reg.idReg.BattleSkills[s]
	return BattleSkillID(id), ok
}

// BattleSkillIDToString converts a numeric battle skill id back to its string id.
func BattleSkillIDToString(id BattleSkillID) (string, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before BattleSkillIDToString")
	}
	for s, v := range reg.idReg.BattleSkills {
		if v == int64(id) {
			return s, true
		}
	}
	return "", false
}

// =========================
// Numeric-indexed helpers
// =========================

// EventsBySkill returns events whose NeedSkill matches the given skill string id.
func EventsBySkill(skillID string) []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before EventsBySkill")
	}
	if skillID == "" {
		skillID = "none"
	}
	sid, _ := StringToSkillID(skillID)
	return append([]Event(nil), reg.eventsBySkill[sid]...)
}

// EventsByMap returns events located on the given map string id.
func EventsByMap(mapID string) []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before EventsByMap")
	}
	mid, _ := StringToMapID(mapID)
	return append([]Event(nil), reg.eventsByMap[mid]...)
}

// EventsBySkillID returns events whose NeedSkill matches the given numeric skill id.
func EventsBySkillID(skillID SkillID) []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before EventsBySkillID")
	}
	return append([]Event(nil), reg.eventsBySkill[skillID]...)
}

// EventsByMapID returns events located on the given numeric map id.
func EventsByMapID(mapID MapID) []Event {
	if reg == nil {
		panic("gameconfig: Load() must be called before EventsByMapID")
	}
	return append([]Event(nil), reg.eventsByMap[mapID]...)
}

// --- counts ---

// SkillCount returns the number of distinct skills.
func SkillCount() int {
	if reg == nil {
		panic("gameconfig: Load() must be called before SkillCount")
	}
	return len(reg.idReg.Skills)
}

// MapCount returns the number of distinct maps.
func MapCount() int {
	if reg == nil {
		panic("gameconfig: Load() must be called before MapCount")
	}
	return len(reg.idReg.Maps)
}

// BattleSkillCount returns the number of distinct battle skills.
func BattleSkillCount() int {
	if reg == nil {
		panic("gameconfig: Load() must be called before BattleSkillCount")
	}
	return len(reg.idReg.BattleSkills)
}
