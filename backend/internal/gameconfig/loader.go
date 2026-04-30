package gameconfig

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/edrowsluo/new-mli/backend/internal/item"
)

// registry holds the parsed config, numeric ID mappings, and indexes.
type registry struct {
	// --- item defs (runtime type) ---
	itemsByString map[string]item.ItemDef
	itemsByID     map[item.ID]item.ItemDef
	itemsByClass  map[string][]item.ItemDef

	// --- events ---
	events map[string]Event

	// --- id registry (source of truth for numeric ids) ---
	idReg *IDRegistry

	// --- numeric-id indexes ---
	eventsByID    map[EventID]Event
	eventsBySkill map[SkillID][]Event
	eventsByMap   map[MapID][]Event
	loopEvents    []Event
	upgradeEvents []Event
}

var reg *registry

// Load parses the embedded actions.json and id_registry.json, validates
// consistency, and builds the in-memory indexes. Safe to call multiple
// times (idempotent).
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
		itemsByString: make(map[string]item.ItemDef, len(cfg.Items)),
		events:        make(map[string]Event, len(cfg.Events)),
		idReg:         idReg,
		itemsByID:     make(map[item.ID]item.ItemDef),
		itemsByClass:  make(map[string][]item.ItemDef),
		eventsByID:    make(map[EventID]Event),
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

func indexItems(r *registry, items []itemJSON) error {
	for _, ij := range items {
		if ij.ID == "" {
			return fmt.Errorf("item with empty id (name=%q)", ij.Name)
		}
		if _, ok := r.itemsByString[ij.ID]; ok {
			return fmt.Errorf("duplicate item id %q", ij.ID)
		}
		id := item.ID(r.idReg.Items[ij.ID])
		def := ij.toDef(id)
		r.itemsByString[ij.ID] = def
		r.itemsByID[id] = def
		r.itemsByClass[def.Classification()] = append(r.itemsByClass[def.Classification()], def)
	}
	return nil
}

func indexEvents(r *registry, events []Event) error {
	for i := range events {
		ev := &events[i]
		if ev.ID == "" {
			return fmt.Errorf("event with empty id (name=%q)", ev.Name)
		}
		if _, ok := r.events[ev.ID]; ok {
			return fmt.Errorf("duplicate event id %q", ev.ID)
		}

		// Resolve skill/event/item IDs.
		ev.ResolvedSkillID = SkillID(r.idReg.Skills[ev.NeedSkill])
		ev.ProductionAttrName = ev.NeedSkill + "_production_multiplier"

		for j := range ev.Requirements {
			req := &ev.Requirements[j]
			switch req.Type {
			case string(ReqTypeSkill):
				req.ResolvedID = r.idReg.Skills[req.ID]
			case string(ReqTypeEvent):
				req.ResolvedID = r.idReg.Events[req.ID]
			case string(ReqTypeItem), string(ReqTypeFluid):
				req.ResolvedItem = item.Item{ID: item.ID(r.idReg.Items[req.ID])}
			}
		}
		for j := range ev.Rewards {
			rew := &ev.Rewards[j]
			if rew.IsItem() {
				rew.ResolvedItem = item.Item{ID: item.ID(r.idReg.Items[rew.ID])}
			} else if rew.IsExperience() {
				rew.ResolvedSkillID = SkillID(r.idReg.Skills[rew.SkillID])
			}
		}

		r.events[ev.ID] = *ev
		eid := EventID(r.idReg.Events[ev.ID])
		r.eventsByID[eid] = *ev

		if ev.Type == EventTypeLoop {
			r.loopEvents = append(r.loopEvents, *ev)
		} else if ev.Type == EventTypeUpgrade {
			r.upgradeEvents = append(r.upgradeEvents, *ev)
		}

		r.eventsBySkill[ev.ResolvedSkillID] = append(r.eventsBySkill[ev.ResolvedSkillID], *ev)

		mid := MapID(r.idReg.Maps[ev.Map])
		r.eventsByMap[mid] = append(r.eventsByMap[mid], *ev)
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
				if _, ok := r.itemsByString[req.ID]; !ok {
					return fmt.Errorf("event %q requires unknown item %q", ev.ID, req.ID)
				}
			case string(ReqTypeFluid):
				// handled as items
			case string(ReqTypeEvent):
				if _, ok := r.events[req.ID]; !ok {
					return fmt.Errorf("event %q requires unknown event %q", ev.ID, req.ID)
				}
			case string(ReqTypeSkill):
			default:
				return fmt.Errorf("event %q has unknown requirement type %q", ev.ID, req.Type)
			}
		}
		for _, rew := range ev.Rewards {
			if rew.IsItem() {
				if _, ok := r.itemsByString[rew.ID]; !ok {
					return fmt.Errorf("event %q rewards unknown item %q", ev.ID, rew.ID)
				}
			}
			if rew.IsExperience() && rew.SkillID == "" {
				return fmt.Errorf("event %q experience reward missing skill_id", ev.ID)
			}
		}
	}

	for _, it := range r.itemsByString {
		// Validate was called on itemJSON. Since we now store ItemDef,
		// the JSON-level validation (tool/equipment/upgrade flags) is
		// handled in toDef / NewDef.
		_ = it
	}
	return nil
}

// =========================
// Item accessors (returns item.ItemDef)
// =========================

// GetItemDef returns an item definition by string id.
func GetItemDef(s string) (item.ItemDef, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetItemDef")
	}
	it, ok := reg.itemsByString[s]
	return it, ok
}

// GetItemDefByID returns an item definition by numeric id.
func GetItemDefByID(id item.ID) (item.ItemDef, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetItemDefByID")
	}
	it, ok := reg.itemsByID[id]
	return it, ok
}

// AllItemDefs returns every item definition in deterministic order.
func AllItemDefs() []item.ItemDef {
	if reg == nil {
		panic("gameconfig: Load() must be called before AllItemDefs")
	}
	out := make([]item.ItemDef, 0, len(reg.itemsByString))
	for _, it := range reg.itemsByString {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StringID() < out[j].StringID() })
	return out
}

// ItemDefsByClassification returns items of a given classification.
func ItemDefsByClassification(class string) []item.ItemDef {
	if reg == nil {
		panic("gameconfig: Load() must be called before ItemDefsByClassification")
	}
	out := append([]item.ItemDef(nil), reg.itemsByClass[class]...)
	sort.Slice(out, func(i, j int) bool { return out[i].StringID() < out[j].StringID() })
	return out
}

// ItemCount returns the number of defined items.
func ItemCount() int {
	if reg == nil {
		panic("gameconfig: Load() must be called before ItemCount")
	}
	return len(reg.itemsByString)
}

// =========================
// Item ID mapping
// =========================

// StringToItemID converts a string item id to its numeric id.
func StringToItemID(s string) (item.ID, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before StringToItemID")
	}
	id, ok := reg.idReg.Items[s]
	return item.ID(id), ok
}

// ItemIDToString converts a numeric item id back to its string id.
func ItemIDToString(id item.ID) (string, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before ItemIDToString")
	}
	for s, v := range reg.idReg.Items {
		if item.ID(v) == id {
			return s, true
		}
	}
	return "", false
}

// AllItemIDs returns all numeric item ids in ascending order.
func AllItemIDs() []item.ID {
	if reg == nil {
		panic("gameconfig: Load() must be called before AllItemIDs")
	}
	out := make([]item.ID, 0, len(reg.itemsByID))
	for id := range reg.itemsByID {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// =========================
// String-based accessors (for external API / wire format)
// =========================

// GetEvent returns an event by its string id.
func GetEvent(id string) (Event, bool) {
	if reg == nil {
		panic("gameconfig: Load() must be called before GetEvent")
	}
	ev, ok := reg.events[id]
	return ev, ok
}

// AllEvents returns every event in deterministic order.
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

// EventCount returns the number of defined events.
func EventCount() int {
	if reg == nil {
		panic("gameconfig: Load() before EventCount")
	}
	return len(reg.events)
}

// =========================
// EventID
// =========================

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

// =========================
// SkillID
// =========================

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

// SkillCount returns the number of distinct skills.
func SkillCount() int {
	if reg == nil {
		panic("gameconfig: Load() must be called before SkillCount")
	}
	return len(reg.idReg.Skills)
}

// =========================
// MapID
// =========================

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

// MapCount returns the number of distinct maps.
func MapCount() int {
	if reg == nil {
		panic("gameconfig: Load() must be called before MapCount")
	}
	return len(reg.idReg.Maps)
}

// =========================
// BattleSkillID
// =========================

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

// BattleSkillCount returns the number of distinct battle skills.
func BattleSkillCount() int {
	if reg == nil {
		panic("gameconfig: Load() must be called before BattleSkillCount")
	}
	return len(reg.idReg.BattleSkills)
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
