package battle

import (
	"fmt"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

// EnemyBattleEntity represents an enemy in combat.
// It owns a temporary attribute.Instance that is discarded after the battle.
type EnemyBattleEntity struct {
	numericID   int64 // numeric enemy definition id from gameconfig
	instanceIdx int   // index within the current wave
	name        string // retained for internal debugging, not transmitted

	attr *attribute.Instance

	// Runtime resources.
	hp    float64
	mp    float64
	sp    float64
	alive bool

	// Cached maxima for proportional scaling on buff expiry.
	cachedMaxHP float64
	cachedMaxMP float64
	cachedMaxSP float64

	nextReadyTime      float64
	lastActionDuration float64

	cooldowns     map[gameconfig.BattleSkillID]float64
	activeEffects []ActiveEffect

	// Skills.
	skills       map[gameconfig.BattleSkillID]*BattleSkill
	skillPlan    []SkillPlanEntry
	basicSkillID gameconfig.BattleSkillID

	// Bookkeeping.
	lastSkillID gameconfig.BattleSkillID

	// Rewards.
	Drops     []DropEntry
	ExpReward float64
}

// DropEntry is a single possible drop from an enemy.
type DropEntry struct {
	ItemID    int32
	ItemState int32
	Chance    float64 // 0.0 - 1.0
	MinQty    float64
	MaxQty    float64
}

// NewEnemyBattleEntity creates an enemy from its definition.
// baseStats maps attribute string IDs to their base values.
func NewEnemyBattleEntity(numericID int64, instanceIdx int, name string, baseStats map[string]float64) *EnemyBattleEntity {
	attr := attribute.NewInstance()
	reg := attribute.Get()

	// Inject base stats as OVERRIDE modifiers so they participate in the
	// same computation pipeline as player attributes.
	var mods []attribute.Modifier
	for attrName, val := range baseStats {
		aid, ok := reg.AttrID(attrName)
		if !ok {
			continue
		}
		mods = append(mods, attribute.Modifier{
			AttrID: aid,
			Op:     attribute.OpOverride,
			Value:  val,
			Source: "enemy:base",
		})
	}
	if len(mods) > 0 {
		attr.AddModifiers("enemy:base", mods)
	}

	e := &EnemyBattleEntity{
		numericID:   numericID,
		instanceIdx: instanceIdx,
		name:        name,
		attr:        attr,
		alive:       true,
		cooldowns:   make(map[gameconfig.BattleSkillID]float64),
		skills:      make(map[gameconfig.BattleSkillID]*BattleSkill),
	}
	e.cachedMaxHP = e.MaxHP()
	e.cachedMaxMP = e.MaxMP()
	e.cachedMaxSP = e.MaxSP()
	return e
}

// --- Identity ---

// EntityID returns a unique numeric id for this enemy instance.
// Encoding: -(numericID << 32 | instanceIdx)
// Players use positive ids (userID), so negative values identify enemies.
func (e *EnemyBattleEntity) EntityID() int64 {
	return -(e.numericID<<32 | int64(e.instanceIdx))
}

func (e *EnemyBattleEntity) Team() Team { return TeamEnemy }

// --- Life ---

func (e *EnemyBattleEntity) Alive() bool     { return e.alive }
func (e *EnemyBattleEntity) SetAlive(v bool) { e.alive = v }

// --- Resources ---

func (e *EnemyBattleEntity) HP() float64         { return e.hp }
func (e *EnemyBattleEntity) SetHP(v float64)     { e.hp = v }
func (e *EnemyBattleEntity) MaxHP() float64      { return e.attr.GetFinal(mustAttrID("hp")) }
func (e *EnemyBattleEntity) MP() float64         { return e.mp }
func (e *EnemyBattleEntity) SetMP(v float64)     { e.mp = v }
func (e *EnemyBattleEntity) MaxMP() float64      { return e.attr.GetFinal(mustAttrID("mp")) }
func (e *EnemyBattleEntity) SP() float64         { return e.sp }
func (e *EnemyBattleEntity) SetSP(v float64)     { e.sp = v }
func (e *EnemyBattleEntity) MaxSP() float64      { return e.attr.GetFinal(mustAttrID("sp")) }

// --- Timing ---

func (e *EnemyBattleEntity) NextReadyTime() float64              { return e.nextReadyTime }
func (e *EnemyBattleEntity) SetNextReadyTime(v float64)          { e.nextReadyTime = v }
func (e *EnemyBattleEntity) LastActionDuration() float64         { return e.lastActionDuration }
func (e *EnemyBattleEntity) SetLastActionDuration(v float64)     { e.lastActionDuration = v }

// --- Attributes ---

func (e *EnemyBattleEntity) GetFinal(attrID attribute.AttributeID) float64 {
	return e.attr.GetFinal(attrID)
}

// --- Effects ---

func (e *EnemyBattleEntity) ActiveEffects() []ActiveEffect {
	out := make([]ActiveEffect, len(e.activeEffects))
	copy(out, e.activeEffects)
	return out
}

func (e *EnemyBattleEntity) SetActiveEffects(effs []ActiveEffect) {
	e.activeEffects = make([]ActiveEffect, len(effs))
	copy(e.activeEffects, effs)
}

func (e *EnemyBattleEntity) ApplyEffect(effect ActiveEffect, now float64) {
	replaced := false
	for i := range e.activeEffects {
		if e.activeEffects[i].effectKey() == effect.effectKey() {
			e.activeEffects[i] = effect
			replaced = true
			break
		}
	}
	if !replaced {
		e.activeEffects = append(e.activeEffects, effect)
	}
	e.syncEffectsForSource(effect.SourceSkillID)
}

// syncEffectsForSource rebuilds all attribute modifiers for a given skill source.
func (e *EnemyBattleEntity) syncEffectsForSource(sourceSkillID gameconfig.BattleSkillID) {
	var mods []attribute.Modifier
	for _, eff := range e.activeEffects {
		if eff.SourceSkillID != sourceSkillID {
			continue
		}
		var op attribute.OpType
		switch eff.Mode {
		case EffectModeFlat:
			op = attribute.OpAdd
		case EffectModePercentMult:
			op = attribute.OpMultiply
		default:
			op = attribute.OpAdd
		}
		mods = append(mods, attribute.Modifier{
			AttrID: eff.Attribute,
			Op:     op,
			Value:  eff.Value,
			Source: fmt.Sprintf("battle:%d", sourceSkillID),
		})
	}
	e.attr.AddModifiers(fmt.Sprintf("battle:%d", sourceSkillID), mods)
}

func (e *EnemyBattleEntity) RefreshStats(now float64) {
	all := e.activeEffects
	kept := make([]ActiveEffect, 0, len(all))
	for _, eff := range all {
		if eff.ExpiresAt != nil && *eff.ExpiresAt <= now {
			continue
		}
		kept = append(kept, eff)
	}
	e.activeEffects = kept

	rescaleResources(e, e.cachedMaxHP, e.cachedMaxMP, e.cachedMaxSP)

	e.cachedMaxHP = e.MaxHP()
	e.cachedMaxMP = e.MaxMP()
	e.cachedMaxSP = e.MaxSP()
}

// --- Skills ---

func (e *EnemyBattleEntity) Skills() map[gameconfig.BattleSkillID]*BattleSkill {
	out := make(map[gameconfig.BattleSkillID]*BattleSkill, len(e.skills))
	for k, v := range e.skills {
		out[k] = v
	}
	return out
}

func (e *EnemyBattleEntity) SkillPlan() []SkillPlanEntry                           { return e.skillPlan }
func (e *EnemyBattleEntity) BasicSkillID() gameconfig.BattleSkillID                { return e.basicSkillID }
func (e *EnemyBattleEntity) Cooldowns() map[gameconfig.BattleSkillID]float64       { return e.cooldowns }
func (e *EnemyBattleEntity) SetCooldown(skillID gameconfig.BattleSkillID, expiresAt float64) {
	e.cooldowns[skillID] = expiresAt
}

// --- Bookkeeping ---

func (e *EnemyBattleEntity) LastSkillID() gameconfig.BattleSkillID              { return e.lastSkillID }
func (e *EnemyBattleEntity) SetLastSkillID(v gameconfig.BattleSkillID)          { e.lastSkillID = v }

// Setters for skill data.

func (e *EnemyBattleEntity) SetSkills(skills map[gameconfig.BattleSkillID]*BattleSkill) { e.skills = skills }
func (e *EnemyBattleEntity) SetSkillPlan(plan []SkillPlanEntry)                         { e.skillPlan = plan }
func (e *EnemyBattleEntity) SetBasicSkillID(id gameconfig.BattleSkillID)                { e.basicSkillID = id }
