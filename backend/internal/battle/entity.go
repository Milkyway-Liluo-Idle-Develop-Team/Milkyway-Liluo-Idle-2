// Package battle implements the combat system: entities, skills, damage
// calculation, and the event-driven battle loop.
package battle

import (
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

// Team identifies which side an entity belongs to.
type Team int

const (
	TeamPlayer Team = iota
	TeamEnemy
)

// BattleEntity is the common interface for every combatant (player or enemy).
type BattleEntity interface {
	// --- Identity ---
	EntityID() int64
	Team() Team

	// --- Life ---
	Alive() bool
	SetAlive(bool)

	// --- Resources (current values; max comes from the attribute system) ---
	HP() float64
	SetHP(float64)
	MaxHP() float64

	MP() float64
	SetMP(float64)
	MaxMP() float64

	SP() float64
	SetSP(float64)
	MaxSP() float64

	// --- Action timing ---
	NextReadyTime() float64
	SetNextReadyTime(float64)
	LastActionDuration() float64
	SetLastActionDuration(float64)

	// --- Attributes ---
	// GetFinal reads the computed final value from the underlying attribute system.
	GetFinal(attrID attribute.AttributeID) float64

	// --- Effects ---
	ActiveEffects() []ActiveEffect
	SetActiveEffects([]ActiveEffect)
	ApplyEffect(effect ActiveEffect, now float64)
	RefreshStats(now float64)

	// --- Skills ---
	Skills() map[gameconfig.BattleSkillID]*BattleSkill
	SkillPlan() []SkillPlanEntry
	BasicSkillID() gameconfig.BattleSkillID
	Cooldowns() map[gameconfig.BattleSkillID]float64
	SetCooldown(skillID gameconfig.BattleSkillID, expiresAt float64)

	// --- Combat log bookkeeping ---
	LastSkillID() gameconfig.BattleSkillID
	SetLastSkillID(gameconfig.BattleSkillID)
}
