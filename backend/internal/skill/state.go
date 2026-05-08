// Package skill manages player skill levels and XP. It tracks per-skill
// experience, detects level-ups, and exposes an OnLevelUp hook for
// external systems (e.g. attribute modifiers) to react.
package skill

import (
	"context"
	"fmt"
	"sort"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
)

// LevelCurve maps cumulative XP to level.
type LevelCurve []float64

// Level returns the level for the given cumulative XP (binary search).
// Prefer nextLevel for AddXP where the old level is known.
func (c LevelCurve) Level(xp float64) float64 {
	if len(c) == 0 || xp < c[0] {
		return 0
	}
	idx := sort.Search(len(c), func(i int) bool { return c[i] > xp })
	return float64(idx)
}

// nextLevel walks forward from oldLevel to find the new level for the
// given total XP. Since XP growth is small between ticks, this is
// typically 0— steps —effectively O(1) amortized.
func (c LevelCurve) nextLevel(oldLevel, totalXP float64) float64 {
	n := len(c)
	for l := int(oldLevel); l < n; l++ {
		if totalXP < c[l] {
			return float64(l)
		}
	}
	return float64(n)
}

// XPForLevel returns the cumulative XP required to reach the given level.
func (c LevelCurve) XPForLevel(level float64) float64 {
	if level <= 0 {
		return 0
	}
	l := int(level) - 1
	if l >= len(c) {
		l = len(c) - 1
	}
	return c[l]
}

// State holds the in-memory skill data for one player.
type State struct {
	userID int64
	skills map[gameconfig.SkillID]*skillSlot
	dirty  map[gameconfig.SkillID]bool
	curve  LevelCurve

	// OnLevelUp is called when AddXP causes a level increase.
	// Set by the caller after construction to wire attribute updates.
	OnLevelUp func(skillID gameconfig.SkillID, oldLevel, newLevel float64)

	recorder *record.Recorder
}

type skillSlot struct {
	Level float64
	XP    float64
}

// Load reads all skill rows for the given user into a new State.
func Load(ctx context.Context, q *dbgen.Queries, userID int64, curve LevelCurve) (*State, error) {
	rows, err := q.LoadSkills(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("skill: load: %w", err)
	}

	skills := make(map[gameconfig.SkillID]*skillSlot, len(rows))
	for _, r := range rows {
		skills[gameconfig.SkillID(r.SkillID)] = &skillSlot{Level: r.Level, XP: r.Xp}
	}

	return &State{
		userID: userID,
		skills: skills,
		dirty:  make(map[gameconfig.SkillID]bool),
		curve:  curve,
	}, nil
}

// Flush writes every dirty skill to the database.
func (s *State) Flush(ctx context.Context, q *dbgen.Queries) error {
	if len(s.dirty) == 0 {
		return nil
	}

	for id := range s.dirty {
		slot := s.skills[id]
		err := q.UpsertSkill(ctx, dbgen.UpsertSkillParams{
			UserID:  s.userID,
			SkillID: int64(id),
			Level:   slot.Level,
			Xp:      slot.XP,
		})
		if err != nil {
			return fmt.Errorf("skill: upsert %d: %w", id, err)
		}
	}

	s.dirty = make(map[gameconfig.SkillID]bool)
	return nil
}

// AddXP adds experience to the given skill. The XP value must already
// include all external bonuses (equipment, buffs) —this method only
// handles accumulation and level-up detection.
func (s *State) AddXP(skillID gameconfig.SkillID, xp float64) {
	slot := s.mustSlot(skillID)
	oldLevel := slot.Level
	slot.XP += xp
	newLevel := s.curve.nextLevel(oldLevel, slot.XP)
	slot.Level = newLevel
	s.dirty[skillID] = true

	s.record(skillID, xp, newLevel)

	if newLevel > oldLevel && s.OnLevelUp != nil {
		s.OnLevelUp(skillID, oldLevel, newLevel)
	}
}

// Get returns level and XP for a skill. Returns 0, 0 if the skill has
// never been trained.
func (s *State) Get(skillID gameconfig.SkillID) (level, xp float64) {
	slot := s.mustSlot(skillID)
	return slot.Level, slot.XP
}

// All returns all skills with progress for snapshot serialization.
func (s *State) All() map[gameconfig.SkillID]skillSlot {
	out := make(map[gameconfig.SkillID]skillSlot, len(s.skills))
	for id, slot := range s.skills {
		if slot.Level > 0 || slot.XP > 0 {
			out[id] = *slot
		}
	}
	return out
}

func (s *State) mustSlot(skillID gameconfig.SkillID) *skillSlot {
	slot, ok := s.skills[skillID]
	if !ok {
		slot = &skillSlot{Level: 1}
		s.skills[skillID] = slot
	}
	return slot
}

func (s *State) record(skillID gameconfig.SkillID, xpDelta, newLevel float64) {
	if s.recorder == nil {
		return
	}
	b := s.recorder.Bucket("skill_xp")
	if b != nil {
		b.(*Bucket).add(skillID, xpDelta, newLevel)
	}
}

// SetRecorder attaches a Recorder for the current execution cycle.
func (s *State) SetRecorder(rec *record.Recorder) { s.recorder = rec }

// ClearRecorder detaches the current Recorder.
func (s *State) ClearRecorder() { s.recorder = nil }
