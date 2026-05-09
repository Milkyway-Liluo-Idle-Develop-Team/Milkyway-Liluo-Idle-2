package battle

import (
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
)

// PlayerBattleEntity wraps a PlayerSession for combat.
// It references the player's attribute.Instance directly; buffs are injected
// as temporary modifiers with a "battle:" source prefix.
type PlayerBattleEntity struct {
	userID int64
	name   string

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
}

// NewPlayerBattleEntity creates a combat-ready player entity.
func NewPlayerBattleEntity(userID int64, name string, attr *attribute.Instance) *PlayerBattleEntity {
	p := &PlayerBattleEntity{
		userID:    userID,
		name:      name,
		attr:      attr,
		alive:     true,
		cooldowns: make(map[string]float64),
		skills:    make(map[string]*BattleSkill),
	}
	p.cachedMaxHP = p.MaxHP()
	p.cachedMaxMP = p.MaxMP()
	p.cachedMaxSP = p.MaxSP()
	return p
}

// --- Identity ---

func (p *PlayerBattleEntity) EntityID() string { return p.name }
func (p *PlayerBattleEntity) Name() string      { return p.name }
func (p *PlayerBattleEntity) Team() Team        { return TeamPlayer }

// --- Life ---

func (p *PlayerBattleEntity) Alive() bool     { return p.alive }
func (p *PlayerBattleEntity) SetAlive(v bool) { p.alive = v }

// --- Resources ---

func (p *PlayerBattleEntity) HP() float64         { return p.hp }
func (p *PlayerBattleEntity) SetHP(v float64)     { p.hp = v }
func (p *PlayerBattleEntity) MaxHP() float64      { return p.attr.GetFinal(mustAttrID("hp")) }
func (p *PlayerBattleEntity) MP() float64         { return p.mp }
func (p *PlayerBattleEntity) SetMP(v float64)     { p.mp = v }
func (p *PlayerBattleEntity) MaxMP() float64      { return p.attr.GetFinal(mustAttrID("mp")) }
func (p *PlayerBattleEntity) SP() float64         { return p.sp }
func (p *PlayerBattleEntity) SetSP(v float64)     { p.sp = v }
func (p *PlayerBattleEntity) MaxSP() float64      { return p.attr.GetFinal(mustAttrID("sp")) }

// --- Timing ---

func (p *PlayerBattleEntity) NextReadyTime() float64              { return p.nextReadyTime }
func (p *PlayerBattleEntity) SetNextReadyTime(v float64)          { p.nextReadyTime = v }
func (p *PlayerBattleEntity) LastActionDuration() float64         { return p.lastActionDuration }
func (p *PlayerBattleEntity) SetLastActionDuration(v float64)     { p.lastActionDuration = v }

// --- Attributes ---

func (p *PlayerBattleEntity) GetFinal(attrID attribute.AttributeID) float64 {
	return p.attr.GetFinal(attrID)
}

// --- Effects ---

func (p *PlayerBattleEntity) ActiveEffects() []ActiveEffect {
	out := make([]ActiveEffect, len(p.activeEffects))
	copy(out, p.activeEffects)
	return out
}

func (p *PlayerBattleEntity) SetActiveEffects(effs []ActiveEffect) {
	p.activeEffects = make([]ActiveEffect, len(effs))
	copy(p.activeEffects, effs)
}

// ApplyEffect adds or replaces an active effect and syncs attribute modifiers.
func (p *PlayerBattleEntity) ApplyEffect(effect ActiveEffect, now float64) {
	// Replace existing effect with same source + attribute + mode.
	replaced := false
	for i := range p.activeEffects {
		if p.activeEffects[i].effectKey() == effect.effectKey() {
			p.activeEffects[i] = effect
			replaced = true
			break
		}
	}
	if !replaced {
		p.activeEffects = append(p.activeEffects, effect)
	}
	p.syncEffectsForSource(effect.SourceSkillID)
}

// syncEffectsForSource rebuilds all attribute modifiers for a given skill source.
func (p *PlayerBattleEntity) syncEffectsForSource(sourceSkillID string) {
	var mods []attribute.Modifier
	for _, eff := range p.activeEffects {
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
	p.attr.AddModifiers("battle:"+sourceSkillID, mods)
}

// RefreshStats purges expired effects and rescales current HP/MP/SP.
func (p *PlayerBattleEntity) RefreshStats(now float64) {
	// Purge expired effects.
	all := p.activeEffects
	kept := make([]ActiveEffect, 0, len(all))
	for _, eff := range all {
		if eff.ExpiresAt != nil && *eff.ExpiresAt <= now {
			continue
		}
		kept = append(kept, eff)
	}
	p.activeEffects = kept

	rescaleResources(p, p.cachedMaxHP, p.cachedMaxMP, p.cachedMaxSP)

	// Update caches for next refresh.
	p.cachedMaxHP = p.MaxHP()
	p.cachedMaxMP = p.MaxMP()
	p.cachedMaxSP = p.MaxSP()
}

// --- Skills ---

func (p *PlayerBattleEntity) Skills() map[string]*BattleSkill {
	out := make(map[string]*BattleSkill, len(p.skills))
	for k, v := range p.skills {
		out[k] = v
	}
	return out
}

func (p *PlayerBattleEntity) SkillPlan() []SkillPlanEntry   { return p.skillPlan }
func (p *PlayerBattleEntity) BasicSkillID() string          { return p.basicSkillID }
func (p *PlayerBattleEntity) Cooldowns() map[string]float64 { return p.cooldowns }
func (p *PlayerBattleEntity) SetCooldown(skillID string, expiresAt float64) {
	p.cooldowns[skillID] = expiresAt
}

// --- Bookkeeping ---

func (p *PlayerBattleEntity) LastSkillID() string              { return p.lastSkillID }
func (p *PlayerBattleEntity) SetLastSkillID(v string)          { p.lastSkillID = v }
func (p *PlayerBattleEntity) LastSkillName() string            { return p.lastSkillName }
func (p *PlayerBattleEntity) SetLastSkillName(v string)        { p.lastSkillName = v }

// Setters for skill data (called during entity construction).

func (p *PlayerBattleEntity) SetSkills(skills map[string]*BattleSkill) { p.skills = skills }
func (p *PlayerBattleEntity) SetSkillPlan(plan []SkillPlanEntry)       { p.skillPlan = plan }
func (p *PlayerBattleEntity) SetBasicSkillID(id string)                { p.basicSkillID = id }

// mustAttrID panics if the attribute is missing; used for well-known attrs.
func mustAttrID(name string) attribute.AttributeID {
	id, ok := attribute.Get().AttrID(name)
	if !ok {
		panic("battle: missing attribute " + name)
	}
	return id
}
