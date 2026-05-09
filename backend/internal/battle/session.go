package battle

// BattleConfig is the static definition of a battle instance.
type BattleConfig struct {
	ID                string
	Name              string
	Map               string
	Interval          float64 // seconds between waves
	CombinationLoop   []string
	Combinations      map[string][]EnemyCombination
}

// EnemyCombination defines a group of enemies that can spawn together.
type EnemyCombination struct {
	EnemyID string
	Count   int
}

// BattleSession holds the runtime state of an active battle.
type BattleSession struct {
	Config BattleConfig
	Time   float64
	Running bool

	Player  *PlayerBattleEntity
	Enemies []*EnemyBattleEntity

	WaveNumber   int
	WaveType     string
	NextWaveTime *float64
	RespawnTime  *float64

	// Accumulated rewards / exp pending settlement.
	PendingSkillExp map[string]float64 // skill_id -> xp
}

// NewBattleSession creates a battle session with the given config and player.
func NewBattleSession(cfg BattleConfig, player *PlayerBattleEntity) *BattleSession {
	s := &BattleSession{
		Config:          cfg,
		Time:            0,
		Running:         true,
		Player:          player,
		PendingSkillExp: make(map[string]float64),
	}

	// Player starts ready after the first wave interval + attack interval.
	interval := max(0.1, cfg.Interval)
	attackInterval := max(0.1, player.GetFinal(AttrAttackInterval))
	player.SetNextReadyTime(interval + attackInterval)
	player.SetLastActionDuration(attackInterval)

	s.NextWaveTime = &interval
	return s
}

// NextEventTime returns the next time anything happens, or nil if idle.
func (s *BattleSession) NextEventTime() *float64 {
	var candidates []float64

	if s.RespawnTime != nil {
		candidates = append(candidates, *s.RespawnTime)
	}
	if s.NextWaveTime != nil {
		candidates = append(candidates, *s.NextWaveTime)
	}

	aliveEnemies := s.AliveEnemies()
	if s.Player.Alive() && len(aliveEnemies) > 0 {
		candidates = append(candidates, s.Player.NextReadyTime())
		for _, e := range aliveEnemies {
			candidates = append(candidates, e.NextReadyTime())
		}
	}

	if len(candidates) == 0 {
		return nil
	}
	minT := candidates[0]
	for _, t := range candidates[1:] {
		if t < minT {
			minT = t
		}
	}
	return &minT
}

// AliveEnemies returns enemies that are still alive.
func (s *BattleSession) AliveEnemies() []*EnemyBattleEntity {
	var out []*EnemyBattleEntity
	for _, e := range s.Enemies {
		if e.Alive() {
			out = append(out, e)
		}
	}
	return out
}

// AdvanceOneEvent moves the battle forward by exactly one event.
// Returns the combat logs generated.
func (s *BattleSession) AdvanceOneEvent() []BattleLog {
	var logs []BattleLog
	if !s.Running {
		return logs
	}

	next := s.NextEventTime()
	if next == nil {
		s.Running = false
		logs = append(logs, BattleLog{Type: "stopped"})
		return logs
	}

	s.advanceTime(*next)

	// Respawn check.
	if s.RespawnTime != nil && s.Time >= *s.RespawnTime {
		s.RespawnTime = nil
		p := s.Player
		p.SetAlive(true)
		p.SetHP(p.MaxHP())
		p.SetMP(p.MaxMP())
		p.SetSP(p.MaxSP())
		p.SetNextReadyTime(s.Time + max(0.1, p.GetFinal(AttrAttackInterval)))
		nextWave := s.Time + max(0.1, s.Config.Interval)
		s.NextWaveTime = &nextWave
		logs = append(logs, BattleLog{Type: "player_respawn", NextWaveIn: s.Config.Interval})
	}

	// Wave spawn check.
	if s.NextWaveTime != nil && s.Time >= *s.NextWaveTime && s.Player.Alive() {
		logs = append(logs, s.spawnWave()...)
	}

	// Player attack.
	aliveEnemies := s.AliveEnemies()
	if len(aliveEnemies) > 0 && s.Player.Alive() && s.Time >= s.Player.NextReadyTime() {
		logs = append(logs, s.processPlayerAttack(aliveEnemies[0])...)
	}

	// Enemy attacks.
	aliveEnemies = s.AliveEnemies()
	if len(aliveEnemies) > 0 && s.Player.Alive() {
		for _, e := range aliveEnemies {
			if s.Time >= e.NextReadyTime() {
				logs = append(logs, s.processEnemyAttack(e)...)
				if !s.Player.Alive() {
					break
				}
			}
		}
	}

	// Wave completion check.
	if s.Player.Alive() {
		logs = append(logs, s.finishWaveIfNeeded()...)
	}

	return logs
}

// advanceTime moves the battle clock forward and applies natural recovery.
func (s *BattleSession) advanceTime(target float64) {
	if target <= s.Time {
		return
	}
	elapsed := target - s.Time

	// Natural recovery for player.
	if s.Player.Alive() {
		s.applyNaturalRecovery(s.Player, elapsed)
		s.Player.RefreshStats(target)
	}

	// Natural recovery for alive enemies.
	for _, e := range s.Enemies {
		if e.Alive() {
			s.applyNaturalRecovery(e, elapsed)
			e.RefreshStats(target)
		}
	}

	s.Time = target
}

func (s *BattleSession) applyNaturalRecovery(e BattleEntity, elapsed float64) {
	if elapsed <= 0 {
		return
	}
	maxHP := max(0.0, e.MaxHP())
	maxMP := max(0.0, e.MaxMP())
	maxSP := max(0.0, e.MaxSP())

	hpRec := e.GetFinal(AttrHPRecovery)
	mpRec := e.GetFinal(AttrMPRecovery)
	spRec := e.GetFinal(AttrSPRecovery)

	e.SetHP(clamp(e.HP()+hpRec*elapsed, 0.0, maxHP))
	e.SetMP(clamp(e.MP()+mpRec*elapsed, 0.0, maxMP))
	e.SetSP(clamp(e.SP()+spRec*elapsed, 0.0, maxSP))
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// chooseWaveType picks the next wave type from the combination loop.
func (s *BattleSession) chooseWaveType() string {
	loop := s.Config.CombinationLoop
	if len(loop) == 0 {
		return "weak"
	}
	idx := s.WaveNumber % len(loop)
	return loop[idx]
}

// finishWaveIfNeeded checks if all enemies are dead and queues the next wave.
func (s *BattleSession) finishWaveIfNeeded() []BattleLog {
	if s.NextWaveTime != nil {
		return nil
	}
	for _, e := range s.Enemies {
		if e.Alive() {
			return nil
		}
	}

	nextWave := s.Time + max(0.1, s.Config.Interval)
	s.NextWaveTime = &nextWave
	return []BattleLog{{
		Type:       "wave_cleared",
		WaveNumber: s.WaveNumber,
		NextWaveIn: s.Config.Interval,
	}}
}
