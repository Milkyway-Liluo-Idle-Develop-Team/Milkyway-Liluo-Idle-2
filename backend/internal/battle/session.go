package battle

import (
	"math/rand"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

// BattleConfig is the static definition of a battle instance.
type BattleConfig struct {
	NumericID               int64
	ID                      string // human-readable string id (e.g. "pasture")
	Name                    string
	Map                     string
	Interval                float64 // seconds between waves
	CombinationLoop         []string
	WeakEnemyCombinations   []EnemyWaveCombination
	StrongEnemyCombinations []EnemyWaveCombination
	BossEnemyCombinations   []EnemyWaveCombination
}

// EnemyWaveCombination is a single weighted enemy group for a wave.
type EnemyWaveCombination struct {
	Enemies []string
	Weight  float64
}

// BattleSession holds the runtime state of an active battle.
type BattleSession struct {
	Config BattleConfig
	Time   float64
	Running bool

	Players []*PlayerBattleEntity
	Enemies []*EnemyBattleEntity

	WaveNumber   int
	WaveType     string
	NextWaveTime *float64

	// Per-player respawn timers: key = player EntityID (int64 userID).
	RespawnTimes map[int64]*float64

	// Accumulated battle-skill XP (1 XP per skill use).
	PendingBattleSkillExp map[gameconfig.BattleSkillID]float64

	// Accumulated general-skill XP from combat (strength, defense, magic, etc.).
	PendingSkillExp map[gameconfig.SkillID]float64

	// HateMap[enemyEntityID][playerEntityID] = accumulated hate value.
	HateMap map[int64]map[int64]float64

	// rng is the shared random source for all non-deterministic combat
	// decisions (damage rolls, evade/block/crit checks, target selection).
	// Tests may replace this with a seeded source for reproducibility.
	rng *rand.Rand
}

// NewBattleSession creates a battle session with the given config and players.
func NewBattleSession(cfg BattleConfig, players []*PlayerBattleEntity) *BattleSession {
	s := &BattleSession{
		Config:          cfg,
		Time:            0,
		Running:         true,
		Players:         players,
		PendingBattleSkillExp: make(map[gameconfig.BattleSkillID]float64),
		PendingSkillExp:       make(map[gameconfig.SkillID]float64),
		RespawnTimes:    make(map[int64]*float64),
		HateMap:         make(map[int64]map[int64]float64),
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Each player starts ready after the first wave interval + their attack interval.
	interval := max(0.1, cfg.Interval)
	for _, p := range players {
		attackInterval := max(0.1, p.GetFinal(AttrAttackInterval))
		p.SetNextReadyTime(interval + attackInterval)
		p.SetLastActionDuration(attackInterval)
	}

	s.NextWaveTime = &interval
	return s
}

// UserIDs returns the user IDs of all participating players.
func (s *BattleSession) UserIDs() []int64 {
	out := make([]int64, len(s.Players))
	for i, p := range s.Players {
		out[i] = p.UserID()
	}
	return out
}

// SetRNG replaces the internal random source (used by tests for deterministic
// combat outcomes).
func (s *BattleSession) SetRNG(r *rand.Rand) {
	s.rng = r
}

// AlivePlayers returns players that are still alive.
func (s *BattleSession) AlivePlayers() []*PlayerBattleEntity {
	var out []*PlayerBattleEntity
	for _, p := range s.Players {
		if p.Alive() {
			out = append(out, p)
		}
	}
	return out
}

// NextEventTime returns the next time anything happens, or nil if idle.
func (s *BattleSession) NextEventTime() *float64 {
	var candidates []float64

	for _, rt := range s.RespawnTimes {
		if rt != nil {
			candidates = append(candidates, *rt)
		}
	}
	if s.NextWaveTime != nil {
		candidates = append(candidates, *s.NextWaveTime)
	}

	aliveEnemies := s.AliveEnemies()
	alivePlayers := s.AlivePlayers()

	if len(alivePlayers) == 0 {
		// No alive players: only respawn times matter.
		if len(candidates) == 0 {
			return nil
		}
	} else {
		if len(aliveEnemies) > 0 {
			for _, p := range alivePlayers {
				candidates = append(candidates, p.NextReadyTime())
			}
			for _, e := range aliveEnemies {
				candidates = append(candidates, e.NextReadyTime())
			}
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
		logs = append(logs, BattleLog{Type: BattleLogTypeStopped})
		return logs
	}

	s.advanceTime(*next)

	// Respawn checks.
	for pid, rt := range s.RespawnTimes {
		if rt != nil && s.Time >= *rt {
			s.RespawnTimes[pid] = nil
			p := s.playerByID(pid)
			if p != nil {
				p.SetAlive(true)
				p.SetHP(p.MaxHP())
				p.SetMP(p.MaxMP())
				p.SetSP(p.MaxSP())
				p.SetNextReadyTime(s.Time + max(0.1, p.GetFinal(AttrAttackInterval)))
				logs = append(logs, BattleLog{
					Type:             BattleLogTypePlayerRespawn,
					NextWaveIn:       s.Config.Interval,
					AttackerEntityID: pid,
				})
			}
		}
	}

	// Wave spawn check.
	alivePlayers := s.AlivePlayers()
	if s.NextWaveTime != nil && s.Time >= *s.NextWaveTime && len(alivePlayers) > 0 {
		logs = append(logs, s.spawnWave()...)
	}

	// If everyone is dead, only respawn matters; emit the event once.
	if len(alivePlayers) == 0 {
		logs = append(logs, BattleLog{Type: BattleLogTypeAllPlayersDowned})
		return logs
	}

	// Player attacks.
	aliveEnemies := s.AliveEnemies()
	if len(aliveEnemies) > 0 {
		for _, p := range alivePlayers {
			if s.Time >= p.NextReadyTime() {
				logs = append(logs, s.processPlayerAttack(p, aliveEnemies[0])...)
			}
		}
	}

	// Enemy attacks.
	aliveEnemies = s.AliveEnemies()
	alivePlayers = s.AlivePlayers()
	if len(aliveEnemies) > 0 && len(alivePlayers) > 0 {
		for _, e := range aliveEnemies {
			if s.Time >= e.NextReadyTime() {
				target := s.chooseEnemyTarget(e, alivePlayers)
				if target != nil {
					logs = append(logs, s.processEnemyAttack(e, target)...)
				}
			}
		}
	}

	// After enemy attacks, check if the last player just died.
	if len(s.AlivePlayers()) == 0 {
		logs = append(logs, BattleLog{Type: BattleLogTypeAllPlayersDowned})
	}

	// Wave completion check.
	if len(s.AlivePlayers()) > 0 {
		logs = append(logs, s.finishWaveIfNeeded()...)
	}

	return logs
}

// AdvanceByDelta advances the battle by a fixed wall-clock delta,
// processing every event whose scheduled time falls within the interval.
// The session clock ends at s.Time+delta.
func (s *BattleSession) AdvanceByDelta(delta float64) []BattleLog {
	var allLogs []BattleLog
	if !s.Running {
		return allLogs
	}

	target := s.Time + delta
	for {
		next := s.NextEventTime()
		if next == nil || *next > target {
			break
		}
		allLogs = append(allLogs, s.AdvanceOneEvent()...)
		if !s.Running {
			return allLogs
		}
	}

	// No more events before target — just apply natural recovery.
	s.advanceTime(target)
	return allLogs
}

// advanceTime moves the battle clock forward and applies natural recovery.
func (s *BattleSession) advanceTime(target float64) {
	if target <= s.Time {
		return
	}
	elapsed := target - s.Time

	// Natural recovery for alive players.
	for _, p := range s.Players {
		if p.Alive() {
			s.applyNaturalRecovery(p, elapsed)
			p.RefreshStats(target)
		}
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
		Type:       BattleLogTypeWaveCleared,
		WaveNumber: s.WaveNumber,
		NextWaveIn: s.Config.Interval,
	}}
}

// playerByID finds a player by their EntityID.
func (s *BattleSession) playerByID(id int64) *PlayerBattleEntity {
	for _, p := range s.Players {
		if p.EntityID() == id {
			return p
		}
	}
	return nil
}

// hasPendingRespawn returns true if any player has a pending respawn timer.
func (s *BattleSession) hasPendingRespawn() bool {
	for _, rt := range s.RespawnTimes {
		if rt != nil {
			return true
		}
	}
	return false
}
