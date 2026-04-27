// Package gameconfig ID types and allocation.
//
// Every string identifier in actions.json (items, events, skills, fluids,
// maps, battle-skills) is assigned a stable numeric int64 defined in
// id_registry.json.  Internal code and database schemas should use these
// numeric IDs for storage and computation; string IDs are reserved for the
// wire format (frontend protocols) and human-facing output.
package gameconfig

import "fmt"

// --- typed IDs (prevent accidental mixing) ---

// ItemID is a stable numeric id for an item defined in id_registry.json.
// Zero means invalid / not found.
type ItemID int64

// EventID is a stable numeric id for an event.
type EventID int64

// SkillID is a stable numeric id for a skill (e.g. "felling", "crafting").
type SkillID int64

// MapID is a stable numeric id for a map / scene.
type MapID int64

// BattleSkillID is a stable numeric id for a battle skill inside equipment.
type BattleSkillID int64

// --- String helpers ---

func (id ItemID) String() string {
	if id == 0 {
		return "<invalid-item>"
	}
	if s, ok := ItemIDToString(id); ok {
		return s
	}
	return fmt.Sprintf("<item-%d>", id)
}

func (id EventID) String() string {
	if id == 0 {
		return "<invalid-event>"
	}
	if s, ok := EventIDToString(id); ok {
		return s
	}
	return fmt.Sprintf("<event-%d>", id)
}

func (id SkillID) String() string {
	if id == 0 {
		return "<invalid-skill>"
	}
	if s, ok := SkillIDToString(id); ok {
		return s
	}
	return fmt.Sprintf("<skill-%d>", id)
}

func (id MapID) String() string {
	if id == 0 {
		return "<invalid-map>"
	}
	if s, ok := MapIDToString(id); ok {
		return s
	}
	return fmt.Sprintf("<map-%d>", id)
}

func (id BattleSkillID) String() string {
	if id == 0 {
		return "<invalid-battle-skill>"
	}
	if s, ok := BattleSkillIDToString(id); ok {
		return s
	}
	return fmt.Sprintf("<battle-skill-%d>", id)
}
