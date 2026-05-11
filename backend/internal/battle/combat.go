package battle

import (
	"math"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

// Fallback skill IDs for runtime-generated basic attacks.
// Negative values avoid collision with registry-assigned positive IDs.
const (
	FallbackBasicAttackID      gameconfig.BattleSkillID = -1
	FallbackEnemyBasicAttackID gameconfig.BattleSkillID = -2
)

// processPlayerAttack resolves a single player's next attack.
func (s *BattleSession) processPlayerAttack(player *PlayerBattleEntity, target *EnemyBattleEntity) []BattleLog {
	skill := s.choosePlayerSkill(player, target)
	return s.resolveAttack(player, target, skill)
}

// processEnemyAttack resolves a single enemy's attack on its chosen target.
func (s *BattleSession) processEnemyAttack(enemy *EnemyBattleEntity, target *PlayerBattleEntity) []BattleLog {
	skill := s.chooseEnemySkill(enemy, target)
	return s.resolveAttack(enemy, target, skill)
}

// resolveAttack handles the full attack flow: costs, effects, damage, XP, death.
func (s *BattleSession) resolveAttack(attacker, defender BattleEntity, skill *BattleSkill) []BattleLog {
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
	result := CalcDamage(attacker, defender, skill, s.rng)

	// Apply damage.
	if result.Damage > 0 {
		defender.SetHP(clamp(defender.HP()-result.Damage, 0.0, defender.MaxHP()))
	}

	// Update hate: player dealing damage to enemy generates hate on that enemy.
	if attacker.Team() == TeamPlayer && defender.Team() == TeamEnemy && result.Damage > 0 {
		s.addHate(defender.EntityID(), attacker.EntityID(), result.Damage)
	}

	// Set attacker timing.
	castTime := skill.CastTime
	if castTime <= 0 {
		castTime = max(0.1, attacker.GetFinal(AttrAttackInterval))
	}
	cooldown := max(0.0, skill.Cooldown)

	attacker.SetNextReadyTime(s.Time + castTime)
	attacker.SetLastActionDuration(castTime)
	attacker.SetLastSkillID(skill.ID)
	if cooldown > 0 {
		attacker.SetCooldown(skill.ID, s.Time+cooldown)
	}

	// Build log.
	var logType BattleLogType
	if attacker.Team() == TeamPlayer {
		logType = BattleLogTypePlayerAttack
	} else {
		logType = BattleLogTypeEnemyAttack
	}

	log := BattleLog{
		Type:             logType,
		SkillID:          skill.ID,
		Damage:           result.Damage,
		RawDamage:        result.RawDamage,
		Evaded:           result.Evaded,
		Blocked:          result.Blocked,
		BlockedReduction: result.BlockedReduction,
		MPCost:           mpCost,
		SPCost:           spCost,
		Effects:          appliedEffects,
		AttackerEntityID: attacker.EntityID(),
		DefenderEntityID: defender.EntityID(),
		DefenderHP:       round3(defender.HP()),
	}
	logs = append(logs, log)

	// Skill XP (1 XP per skill use) for player attacks.
	if attacker.Team() == TeamPlayer && skill.ID > 0 {
		s.addPendingBattleSkillXP(skill.ID, 1.0)
	}

	// Combat XP from damage dealt.
	if attacker.Team() == TeamPlayer && result.Damage > 0 {
		s.awardCombatXP(result, skill)
	}

	// Combat XP from damage received (evade / block / hit).
	if defender.Team() == TeamPlayer {
		s.awardDefenseXP(result)
	}

	// Check death.
	if defender.HP() <= 0 && defender.Alive() {
		defender.SetAlive(false)
		if defender.Team() == TeamEnemy {
			logs = append(logs, s.handleEnemyDeath(defender.(*EnemyBattleEntity))...)
		} else {
			logs = append(logs, s.handlePlayerDown(defender.(*PlayerBattleEntity))...)
		}
	}

	return logs
}

// choosePlayerSkill selects the highest-priority usable skill for a specific player.
func (s *BattleSession) choosePlayerSkill(player *PlayerBattleEntity, target *EnemyBattleEntity) *BattleSkill {
	aliveEnemies := s.AliveEnemies()

	for _, entry := range player.SkillPlan() {
		skill, ok := player.Skills()[entry.SkillID]
		if !ok {
			continue
		}
		if !s.canUseSkill(player, skill) {
			continue
		}
		if entry.Condition != nil && !evaluateCondition(entry.Condition, player, target, toEntities(aliveEnemies)) {
			continue
		}
		if !skillHasDamage(skill) && skillRequiresEffectChange(skill) && !skillWouldApplyEffect(skill, player, target, s.Time) {
			continue
		}
		return skill
	}

	// Fallback to basic attack.
	if basic, ok := player.Skills()[player.BasicSkillID()]; ok {
		return basic
	}
	// Ultimate fallback.
	return &BattleSkill{
		ID:   FallbackBasicAttackID,
		Name: "基础攻击",
		Damage: &DamageProfile{
			Type:       "physical",
			Flat:       0,
			Multiplier: 1.0,
		},
		CastTime: max(0.1, player.GetFinal(AttrAttackInterval)),
	}
}

// chooseEnemySkill selects the highest-priority usable enemy skill.
func (s *BattleSession) chooseEnemySkill(enemy *EnemyBattleEntity, target BattleEntity) *BattleSkill {
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
		ID:   FallbackEnemyBasicAttackID,
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
		if id, ok := gameconfig.StringToSkillID("magic"); ok {
			s.addPendingSkillXP(id, 1.0*dmg)
		}
		return
	}

	style := skill.PhysicalStyle
	if style == "" {
		style = "melee"
	}
	if style == "ranged" {
		if id, ok := gameconfig.StringToSkillID("strength"); ok {
			s.addPendingSkillXP(id, 0.5*dmg)
		}
		if id, ok := gameconfig.StringToSkillID("ranging"); ok {
			s.addPendingSkillXP(id, 0.5*dmg)
		}
	} else {
		if id, ok := gameconfig.StringToSkillID("strength"); ok {
			s.addPendingSkillXP(id, 0.8*dmg)
		}
		if id, ok := gameconfig.StringToSkillID("ranging"); ok {
			s.addPendingSkillXP(id, 0.2*dmg)
		}
	}
}

func (s *BattleSession) awardDefenseXP(result DamageResult) {
	if result.Evaded {
		if id, ok := gameconfig.StringToSkillID("ranging"); ok {
			s.addPendingSkillXP(id, 2.0*result.RawDamage)
		}
		return
	}
	if result.Damage > 0 {
		if id, ok := gameconfig.StringToSkillID("resilience"); ok {
			s.addPendingSkillXP(id, 1.5*result.Damage)
		}
		if id, ok := gameconfig.StringToSkillID("defense"); ok {
			s.addPendingSkillXP(id, 0.5*result.Damage)
		}
	}
	if result.BlockedReduction > 0 {
		if id, ok := gameconfig.StringToSkillID("defense"); ok {
			s.addPendingSkillXP(id, 2.0*result.BlockedReduction)
		}
	}
}

func (s *BattleSession) addPendingBattleSkillXP(skillID gameconfig.BattleSkillID, xp float64) {
	if xp <= 0 {
		return
	}
	s.PendingBattleSkillExp[skillID] += xp
}

func (s *BattleSession) addPendingSkillXP(skillID gameconfig.SkillID, xp float64) {
	if xp <= 0 {
		return
	}
	s.PendingSkillExp[skillID] += xp
}

func (s *BattleSession) handleEnemyDeath(enemy *EnemyBattleEntity) []BattleLog {
	return []BattleLog{{
		Type:             BattleLogTypeEnemyDied,
		AttackerEntityID: enemy.EntityID(),
		WaveNumber:       s.WaveNumber,
	}}
}

func (s *BattleSession) handlePlayerDown(player *PlayerBattleEntity) []BattleLog {
	respawn := s.Time + max(0.1, s.Config.Interval)
	s.RespawnTimes[player.EntityID()] = &respawn
	return []BattleLog{{
		Type:             BattleLogTypePlayerDowned,
		DefenderEntityID: player.EntityID(),
		DefenderHP:       0,
		NextWaveIn:       s.Config.Interval,
	}}
}

// addHate accumulates hate for a player against a specific enemy.
func (s *BattleSession) addHate(enemyEntityID, playerEntityID int64, amount float64) {
	if s.HateMap[enemyEntityID] == nil {
		s.HateMap[enemyEntityID] = make(map[int64]float64)
	}
	s.HateMap[enemyEntityID][playerEntityID] += amount
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
