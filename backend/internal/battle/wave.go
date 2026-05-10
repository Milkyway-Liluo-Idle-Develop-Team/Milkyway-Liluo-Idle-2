package battle

import (
	"math/rand"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

// spawnWave generates enemies for the current wave.
func (s *BattleSession) spawnWave() []BattleLog {
	s.WaveNumber++
	waveType := s.chooseWaveType()
	s.WaveType = waveType
	s.NextWaveTime = nil

	var combos []EnemyWaveCombination
	switch waveType {
	case "weak":
		combos = s.Config.WeakEnemyCombinations
	case "strong":
		combos = s.Config.StrongEnemyCombinations
	case "boss":
		combos = s.Config.BossEnemyCombinations
	}

	var logs []BattleLog
	if len(combos) == 0 {
		return logs
	}

	// Pick a weighted combination.
	combo := pickWeightedCombination(combos, s.rng)
	if combo == nil {
		return logs
	}

	// Spawn enemies.
	for idx, enemyID := range combo.Enemies {
		enemy := s.createEnemyFromDef(enemyID, idx)
		if enemy == nil {
			continue
		}
		enemy.SetHP(enemy.MaxHP())
		enemy.SetNextReadyTime(s.Time + max(0.1, enemy.GetFinal(AttrAttackInterval)))
		s.Enemies = append(s.Enemies, enemy)
	}

	logs = append(logs, BattleLog{
		Type:       BattleLogTypeWaveSpawned,
		WaveNumber: s.WaveNumber,
	})
	return logs
}

// createEnemyFromDef builds an EnemyBattleEntity from gameconfig data.
func (s *BattleSession) createEnemyFromDef(enemyDefID string, instanceIdx int) *EnemyBattleEntity {
	def, ok := gameconfig.GetEnemy(enemyDefID)
	if !ok {
		return nil
	}

	numericID, _ := gameconfig.StringToEnemyID(enemyDefID)
	e := NewEnemyBattleEntity(numericID, instanceIdx, def.Name, def.BattleData)

	// Set basic damage type.
	basicDamageType := def.BasicDamageType
	if basicDamageType == "" {
		basicDamageType = "physical"
	}

	// Build skills map and skill plan.
	skills := make(map[gameconfig.BattleSkillID]*BattleSkill)
	var skillPlan []SkillPlanEntry

	for _, entry := range def.BattleSkills {
		bs := entry.Skill
		skillID, _ := gameconfig.StringToBattleSkillID(bs.ID)
		if skillID == 0 {
			continue // skip skills with no registry entry
		}
		skill := &BattleSkill{
			ID:            skillID,
			Name:          bs.Name,
			Description:   bs.Description,
			TargetType:    bs.TargetType,
			Cooldown:      bs.Cooldown,
			CastTime:      bs.CastTime,
			MPCost:        bs.MPCost,
			SPCost:        bs.SPCost,
			IsBasic:       bs.IsBasic,
			IsSupport:     bs.IsSupport,
			PhysicalStyle: bs.PhysicalStyle,
		}
		if bs.Damage != nil {
			skill.Damage = &DamageProfile{
				Type:       bs.Damage.Type,
				Flat:       bs.Damage.Flat,
				Multiplier: bs.Damage.Multiplier,
			}
		}
		for _, eff := range bs.Effects {
			aid, ok := attribute.Get().AttrID(eff.Attribute)
			if !ok {
				continue
			}
			mode := EffectModeFlat
			if eff.Mode == "percent_mult" || eff.Mode == "percent_multiplier" {
				mode = EffectModePercentMult
			}
			skill.Effects = append(skill.Effects, SkillEffect{
				Target:    eff.Target,
				Attribute: aid,
				Mode:      mode,
				Value:     eff.Value,
				Duration:  eff.Duration,
			})
		}

		skills[skillID] = skill
		skillPlan = append(skillPlan, SkillPlanEntry{
			SkillID:  skillID,
			Priority: entry.Priority,
		})
	}

	// Resolve basic skill ID to numeric.
	basicSkillID := FallbackEnemyBasicAttackID
	if def.BasicSkillID != "" {
		if nid, ok := gameconfig.StringToBattleSkillID(def.BasicSkillID); ok {
			basicSkillID = nid
		}
	}
	if _, ok := skills[basicSkillID]; !ok {
		fallback := &BattleSkill{
			ID:   FallbackEnemyBasicAttackID,
			Name: "基础攻击",
			Damage: &DamageProfile{
				Type:       basicDamageType,
				Flat:       0,
				Multiplier: 1.0,
			},
			CastTime: max(0.1, e.GetFinal(AttrAttackInterval)),
		}
		skills[FallbackEnemyBasicAttackID] = fallback
		basicSkillID = FallbackEnemyBasicAttackID
	}

	e.SetSkills(skills)
	e.SetSkillPlan(skillPlan)
	e.SetBasicSkillID(basicSkillID)

	// Rewards.
	for _, rew := range def.Rewards {
		e.Drops = append(e.Drops, DropEntry{
			ItemID:    int32(rew.ResolvedItem.ID),
			ItemState: int32(rew.ResolvedItem.State),
			Chance:    1.0, // TODO: use proper drop chance
			MinQty:    rew.ItemQuantity(),
			MaxQty:    rew.ItemQuantity(),
		})
		if rew.IsExperience() {
			e.ExpReward += rew.ItemQuantity()
		}
	}

	return e
}

// pickWeightedCombination selects a combination using weight-based random roll.
func pickWeightedCombination(combos []EnemyWaveCombination, rng *rand.Rand) *EnemyWaveCombination {
	if len(combos) == 0 {
		return nil
	}
	var total float64
	for _, c := range combos {
		total += max(0.0, c.Weight)
	}
	if total <= 0 {
		return &combos[0]
	}

	roll := rng.Float64() * total

	var accum float64
	for i := range combos {
		accum += max(0.0, combos[i].Weight)
		if roll <= accum {
			return &combos[i]
		}
	}
	return &combos[len(combos)-1]
}
