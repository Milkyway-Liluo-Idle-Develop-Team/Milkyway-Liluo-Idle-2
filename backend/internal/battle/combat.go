package battle

import (
	"math"
	"math/rand"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
)

// processPlayerAttack resolves the player's next attack.
func (s *BattleSession) processPlayerAttack(target *EnemyBattleEntity) []BattleLog {
	p := s.Player
	skill := s.choosePlayerSkill(target)
	return s.resolveAttack(p, target, skill, true)
}

// processEnemyAttack resolves a single enemy's attack on the player.
func (s *BattleSession) processEnemyAttack(enemy *EnemyBattleEntity) []BattleLog {
	p := s.Player
	skill := s.chooseEnemySkill(enemy, p)
	return s.resolveAttack(enemy, p, skill, false)
}

// resolveAttack handles the full attack flow: costs, effects, damage, XP, death.
func (s *BattleSession) resolveAttack(attacker, defender BattleEntity, skill *BattleSkill, isPlayerAttacking bool) []BattleLog {
	var logs []BattleLog

	// Deduct costs.
	mpCost := max(0.0, skill.MPCost)
	spCost := max(0.0, skill.SPCost)
	if mpCost > 0 {
		attacker.SetMP(clamp(attacker.MP()-mpCost, 0.0, attacker.MaxMP()))
	}
	if spCost > 0 {
		attacker.SetSP(clamp(attacker.SP()-spCost, 0.0, attacker.MaxSP()))
	}

	// Apply skill effects.
	appliedEffects := s.applySkillEffects(attacker, defender, skill)

	// Calculate damage.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := CalcDamage(attacker, defender, skill, rng)

	// Apply damage.
	if result.Damage > 0 {
		defender.SetHP(clamp(defender.HP()-result.Damage, 0.0, defender.MaxHP()))
	}

	// Set attacker timing.
	castTime := max(0.1, skill.CastTime)
	if castTime <= 0 {
		castTime = max(0.1, attacker.GetFinal(AttrAttackInterval))
	}
	cooldown := max(0.0, skill.Cooldown)

	attacker.SetNextReadyTime(s.Time + castTime)
	attacker.SetLastActionDuration(castTime)
	attacker.SetLastSkillID(skill.ID)
	attacker.SetLastSkillName(skill.Name)
	if cooldown > 0 {
		attacker.SetCooldown(skill.ID, s.Time+cooldown)
	}

	// Build log.
	log := BattleLog{
		Type:             "player_attack",
		SkillID:          skill.ID,
		SkillName:        skill.Name,
		Damage:           result.Damage,
		RawDamage:        result.RawDamage,
		DamageType:       result.DamageType,
		Evaded:           result.Evaded,
		Blocked:          result.Blocked,
		BlockedReduction: result.BlockedReduction,
		MPCost:           mpCost,
		SPCost:           spCost,
		Effects:          appliedEffects,
	}

	if isPlayerAttacking {
		log.Type = "player_attack"
		log.TargetID = defender.EntityID()
		log.TargetHP = round3(defender.HP())
	} else {
		log.Type = "enemy_attack"
		log.EnemyID = attacker.EntityID()
		log.EnemyInstanceID = attacker.EntityID()
		log.EnemyName = attacker.Name()
		log.PlayerHP = round3(defender.HP())
	}
	logs = append(logs, log)

	// Skill XP (1 XP per skill use).
	if isPlayerAttacking && skill.ID != "" {
		s.addPendingSkillXP(skill.ID, 1.0)
	}

	// Combat XP from damage dealt.
	if isPlayerAttacking && result.Damage > 0 {
		s.awardCombatXP(result, skill)
	}

	// Combat XP from damage received (evade / block / hit).
	if !isPlayerAttacking {
		s.awardDefenseXP(result)
	}

	// Check death.
	if defender.HP() <= 0 && defender.Alive() {
		defender.SetAlive(false)
		if isPlayerAttacking {
			logs = append(logs, s.handleEnemyDeath(defender.(*EnemyBattleEntity))...)
		} else {
			logs = append(logs, s.handlePlayerDown()...)
		}
	}

	return logs
}

// choosePlayerSkill selects the highest-priority usable skill.
func (s *BattleSession) choosePlayerSkill(target *EnemyBattleEntity) *BattleSkill {
	p := s.Player
	aliveEnemies := s.AliveEnemies()

	for _, entry := range p.SkillPlan() {
		skill, ok := p.Skills()[entry.SkillID]
		if !ok {
			continue
		}
		if !s.canUseSkill(p, skill) {
			continue
		}
		if entry.Condition != nil && !evaluateCondition(entry.Condition, p, target, toEntities(aliveEnemies)) {
			continue
		}
		if !skillHasDamage(skill) && skillRequiresEffectChange(skill) && !skillWouldApplyEffect(skill, p, target, s.Time) {
			continue
		}
		return skill
	}

	// Fallback to basic attack.
	if basic, ok := p.Skills()[p.BasicSkillID()]; ok {
		return basic
	}
	// Ultimate fallback.
	return &BattleSkill{
		ID:   "__basic_attack__",
		Name: "基础攻击",
		Damage: &DamageProfile{
			Type:       "physical",
			Flat:       0,
			Multiplier: 1.0,
		},
		CastTime: max(0.1, p.GetFinal(AttrAttackInterval)),
	}
}

// chooseEnemySkill selects the highest-priority usable enemy skill.
func (s *BattleSession) chooseEnemySkill(enemy *EnemyBattleEntity, target *PlayerBattleEntity) *BattleSkill {
	aliveAllies := s.AliveEnemies()

	for _, entry := range enemy.SkillPlan() {
		skill, ok := enemy.Skills()[entry.SkillID]
		if !ok {
			continue
		}
		if !s.canUseSkill(enemy, skill) {
			continue
		}
		if entry.Condition != nil && !evaluateCondition(entry.Condition, enemy, target, toEntities(aliveAllies)) {
			continue
		}
		if !skillHasDamage(skill) && skillRequiresEffectChange(skill) && !skillWouldApplyEffect(skill, enemy, target, s.Time) {
			continue
		}
		return skill
	}

	if basic, ok := enemy.Skills()[enemy.BasicSkillID()]; ok {
		return basic
	}
	return &BattleSkill{
		ID:   "__enemy_basic_attack__",
		Name: "基础攻击",
		Damage: &DamageProfile{
			Type:       "physical",
			Flat:       0,
			Multiplier: 1.0,
		},
		CastTime: max(0.1, enemy.GetFinal(AttrAttackInterval)),
	}
}

func (s *BattleSession) canUseSkill(e BattleEntity, skill *BattleSkill) bool {
	if s.Time < e.Cooldowns()[skill.ID] {
		return false
	}
	if e.MP() < skill.MPCost {
		return false
	}
	if e.SP() < skill.SPCost {
		return false
	}
	return true
}

func skillHasDamage(skill *BattleSkill) bool {
	return skill.Damage != nil && (skill.Damage.Flat != 0 || skill.Damage.Multiplier != 0)
}

func skillRequiresEffectChange(skill *BattleSkill) bool {
	return len(skill.Effects) > 0
}

func skillWouldApplyEffect(skill *BattleSkill, caster, target BattleEntity, now float64) bool {
	for _, eff := range skill.Effects {
		dst := caster
		if eff.Target == "target" {
			dst = target
		}
		if dst == nil {
			continue
		}

		// Build the effect key as ApplyEffect would.
		candidate := ActiveEffect{
			SourceSkillID: skill.ID,
			Attribute:     eff.Attribute,
			Mode:          eff.Mode,
		}
		key := candidate.effectKey()

		found := false
		for _, ae := range dst.ActiveEffects() {
			if ae.effectKey() != key {
				continue
			}
			found = true
			// If the existing effect has expired, it will be cleaned up soon,
			// so re-applying makes sense.
			if ae.ExpiresAt != nil && *ae.ExpiresAt <= now {
				return true
			}
			// If the value differs (stronger buff, weaker debuff, etc.),
			// applying it would change the entity's stats.
			if ae.Value != eff.Value {
				return true
			}
			// Same value, still alive → this particular effect would change nothing.
		}
		if !found {
			// No existing effect with this key → applying creates a new modifier.
			return true
		}
	}
	return false
}

func (s *BattleSession) applySkillEffects(caster, target BattleEntity, skill *BattleSkill) []AppliedEffect {
	var applied []AppliedEffect
	for _, eff := range skill.Effects {
		dst := caster
		if eff.Target == "target" {
			dst = target
		}
		if dst == nil {
			continue
		}

		var expires *float64
		if eff.Duration > 0 {
			t := s.Time + eff.Duration
			expires = &t
		}

		modeStr := "flat"
		mode := EffectModeFlat
		if eff.Mode == EffectModePercentMult {
			mode = EffectModePercentMult
			modeStr = "percent_mult"
		}

		dst.ApplyEffect(ActiveEffect{
			SourceSkillID: skill.ID,
			Attribute:     eff.Attribute,
			Mode:          mode,
			Value:         eff.Value,
			ExpiresAt:     expires,
		}, s.Time)

		attrName, _ := attribute.Get().AttrString(eff.Attribute)
		applied = append(applied, AppliedEffect{
			Target:    eff.Target,
			Attribute: attrName,
			Mode:      modeStr,
			Value:     eff.Value,
			Duration:  eff.Duration,
		})
	}
	return applied
}

func (s *BattleSession) awardCombatXP(result DamageResult, skill *BattleSkill) {
	if result.Damage <= 0 {
		return
	}
	 dmg := result.Damage
	if skill.Damage != nil && skill.Damage.Type == "magic" {
		s.addPendingSkillXP("magic", 1.0*dmg)
		return
	}

	style := skill.PhysicalStyle
	if style == "" {
		style = "melee"
	}
	if style == "ranged" {
		s.addPendingSkillXP("strength", 0.5*dmg)
		s.addPendingSkillXP("ranging", 0.5*dmg)
	} else {
		s.addPendingSkillXP("strength", 0.8*dmg)
		s.addPendingSkillXP("ranging", 0.2*dmg)
	}
}

func (s *BattleSession) awardDefenseXP(result DamageResult) {
	if result.Evaded {
		s.addPendingSkillXP("ranging", 2.0*result.RawDamage)
		return
	}
	if result.Damage > 0 {
		s.addPendingSkillXP("resilience", 1.5*result.Damage)
		s.addPendingSkillXP("defense", 0.5*result.Damage)
	}
	if result.BlockedReduction > 0 {
		s.addPendingSkillXP("defense", 2.0*result.BlockedReduction)
	}
}

func (s *BattleSession) addPendingSkillXP(skillID string, xp float64) {
	if xp <= 0 {
		return
	}
	s.PendingSkillExp[skillID] += xp
}

func (s *BattleSession) handleEnemyDeath(enemy *EnemyBattleEntity) []BattleLog {
	return []BattleLog{{
		Type:            "enemy_died",
		EnemyID:         enemy.enemyID,
		EnemyInstanceID: enemy.instanceID,
		EnemyName:       enemy.name,
		WaveNumber:      s.WaveNumber,
	}}
}

func (s *BattleSession) handlePlayerDown() []BattleLog {
	p := s.Player
	p.SetAlive(false)
	respawn := s.Time + max(0.1, s.Config.Interval)
	s.RespawnTime = &respawn
	return []BattleLog{{
		Type:       "player_downed",
		PlayerHP:   0,
		NextWaveIn: s.Config.Interval,
	}}
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// evaluateCondition evaluates a skill condition against the current battle state.
// Stub: full implementation in a follow-up step.
func toEntities(enemies []*EnemyBattleEntity) []BattleEntity {
	out := make([]BattleEntity, len(enemies))
	for i, e := range enemies {
		out[i] = e
	}
	return out
}

func evaluateCondition(cond *SkillCondition, player, target BattleEntity, aliveEnemies []BattleEntity) bool {
	if cond == nil {
		return true
	}
	// TODO: implement full condition evaluation.
	return true
}
