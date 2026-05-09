package battle

import (
	"fmt"
	"math/rand"
	"time"

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
	combo := pickWeightedCombination(combos)
	if combo == nil {
		return logs
	}

	// Spawn enemies.
	for idx, enemyID := range combo.Enemies {
		instanceID := fmt.Sprintf("%s_%d", enemyID, idx)
		enemy := s.createEnemyFromDef(enemyID, instanceID)
		if enemy == nil {
			continue
		}
		enemy.SetHP(enemy.MaxHP())
		enemy.SetNextReadyTime(s.Time + max(0.1, enemy.GetFinal(AttrAttackInterval)))
		s.Enemies = append(s.Enemies, enemy)
	}

	logs = append(logs, BattleLog{
		Type:       "wave_spawned",
		WaveNumber: s.WaveNumber,
	})
	return logs
}

// createEnemyFromDef builds an EnemyBattleEntity from gameconfig data.
func (s *BattleSession) createEnemyFromDef(enemyID, instanceID string) *EnemyBattleEntity {
	def, ok := gameconfig.GetEnemy(enemyID)
	if !ok {
		return nil
	}

	e := NewEnemyBattleEntity(enemyID, instanceID, def.Name, def.BattleData)

	// Set basic damage type.
	basicDamageType := def.BasicDamageType
	if basicDamageType == "" {
		basicDamageType = "physical"
	}

	// Build skills map and skill plan.
	skills := make(map[string]*BattleSkill)
	var skillPlan []SkillPlanEntry

	for _, entry := range def.BattleSkills {
		bs := entry.Skill
		skill := &BattleSkill{
			ID:            bs.ID,
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

		skills[bs.ID] = skill
		skillPlan = append(skillPlan, SkillPlanEntry{
			SkillID:  bs.ID,
			Priority: entry.Priority,
		})
	}

	// If no skills defined, create a basic attack fallback.
	basicSkillID := def.BasicSkillID
	if basicSkillID == "" {
		basicSkillID = "__enemy_basic_attack__"
	}
	if _, ok := skills[basicSkillID]; !ok {
		fallback := &BattleSkill{
			ID:   basicSkillID,
			Name: "基础攻击",
			Damage: &DamageProfile{
				Type:       basicDamageType,
				Flat:       0,
				Multiplier: 1.0,
			},
			CastTime: max(0.1, e.GetFinal(AttrAttackInterval)),
		}
		skills[basicSkillID] = fallback
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
func pickWeightedCombination(combos []EnemyWaveCombination) *EnemyWaveCombination {
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

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
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
