package battle

import (
	"fmt"
	"math/rand"
	"time"
)

// spawnWave generates enemies for the current wave.
func (s *BattleSession) spawnWave() []BattleLog {
	s.WaveNumber++
	waveType := s.chooseWaveType()
	s.WaveType = waveType
	s.NextWaveTime = nil

	comboKey := waveType
	combos, ok := s.Config.Combinations[comboKey]
	if !ok || len(combos) == 0 {
		combos = s.Config.Combinations["weak"]
	}

	var logs []BattleLog
	if len(combos) == 0 {
		return logs
	}

	// Pick a random combination.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	combo := combos[rng.Intn(len(combos))]

	// Spawn enemies.
	for i := 0; i < combo.Count; i++ {
		instanceID := fmt.Sprintf("%s_%d", combo.EnemyID, i)
		// TODO: look up enemy definition from gameconfig to get base stats.
		// For now, spawn a placeholder enemy.
		enemy := NewEnemyBattleEntity(combo.EnemyID, instanceID, combo.EnemyID, map[string]float64{
			"hp":              100,
			"physical_power":  20,
			"accuracy":        20,
			"evade":           20,
			"defense":         10,
			"attack_interval": 2.0,
		})
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
