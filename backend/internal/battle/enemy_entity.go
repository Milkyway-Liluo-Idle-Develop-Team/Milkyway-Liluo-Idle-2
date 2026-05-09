package battle

import (
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
)

// EnemyBattleEntity represents an enemy in combat.
// It owns a temporary attribute.Instance that is discarded after the battle.
type EnemyBattleEntity struct {
	enemyID    string
	instanceID string
	name       string

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

	cooldowns     map[string]float64
	activeEffects []ActiveEffect

	// Skills.
	skills       map[string]*BattleSkill
	skillPlan    []SkillPlanEntry
	basicSkillID string

	// Bookkeeping.
	lastSkillID   string
	lastSkillName string

	// Rewards.
	Drops     []DropEntry
	ExpReward float64
}

// DropEntry is a single possible drop from an enemy.
type DropEntry struct {
	ItemID   int32
	ItemState int32
	Chance   float64 // 0.0 - 1.0
	MinQty   float64
	MaxQty   float64
}

// NewEnemyBattleEntity creates an enemy from its definition.
// baseStats maps attribute string IDs to their base values.
func NewEnemyBattleEntity(enemyID, instanceID, name string, baseStats map[string]float64) *EnemyBattleEntity {
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
		enemyID:    enemyID,
		instanceID: instanceID,
		name:       name,
		attr:       attr,
		alive:      true,
		cooldowns:  make(map[string]float64),
		skills:     make(map[string]*BattleSkill),
	}
	e.cachedMaxHP = e.MaxHP()
	e.cachedMaxMP = e.MaxMP()
	e.cachedMaxSP = e.MaxSP()
	return e
}

// --- Identity ---

func (e *EnemyBattleEntity) EntityID() string { return e.instanceID }
func (e *EnemyBattleEntity) Name() string      { return e.name }
func (e *EnemyBattleEntity) Team() Team        { return TeamEnemy }

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
func (e *EnemyBattleEntity) syncEffectsForSource(sourceSkillID string) {
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
			Source: "battle:" + sourceSkillID,
		})
	}
	e.attr.AddModifiers("battle:"+sourceSkillID, mods)
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

func (e *EnemyBattleEntity) Skills() map[string]*BattleSkill {
	out := make(map[string]*BattleSkill, len(e.skills))
	for k, v := range e.skills {
		out[k] = v
	}
	return out
}

func (e *EnemyBattleEntity) SkillPlan() []SkillPlanEntry   { return e.skillPlan }
func (e *EnemyBattleEntity) BasicSkillID() string          { return e.basicSkillID }
func (e *EnemyBattleEntity) Cooldowns() map[string]float64 { return e.cooldowns }
func (e *EnemyBattleEntity) SetCooldown(skillID string, expiresAt float64) {
	e.cooldowns[skillID] = expiresAt
}

// --- Bookkeeping ---

func (e *EnemyBattleEntity) LastSkillID() string              { return e.lastSkillID }
func (e *EnemyBattleEntity) SetLastSkillID(v string)          { e.lastSkillID = v }
func (e *EnemyBattleEntity) LastSkillName() string            { return e.lastSkillName }
func (e *EnemyBattleEntity) SetLastSkillName(v string)        { e.lastSkillName = v }

// Setters for skill data.

func (e *EnemyBattleEntity) SetSkills(skills map[string]*BattleSkill) { e.skills = skills }
func (e *EnemyBattleEntity) SetSkillPlan(plan []SkillPlanEntry)       { e.skillPlan = plan }
func (e *EnemyBattleEntity) SetBasicSkillID(id string)                { e.basicSkillID = id }
