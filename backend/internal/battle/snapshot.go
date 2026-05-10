package battle

import "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"

// BattleSnapshot is the full state of a battle at a single point in time.
type BattleSnapshot struct {
	BattleID   string
	Status     string // "fighting" | "between_waves" | "respawn" | "stopped"
	Time       float64
	WaveNumber int
	NextStepIn float64
	Players    []BattleEntityState
	Enemies    []BattleEntityState
}

// BattleEntityState captures the runtime state of a single combatant for the snapshot.
type BattleEntityState struct {
	EntityID               int64
	Alive                  bool
	HP                     float64
	MaxHP                  float64
	MP                     float64
	MaxMP                  float64
	SP                     float64
	MaxSP                  float64
	NextReadyIn            float64
	ActionCooldownSeconds  float64
	ActionCooldownProgress float64
	LastSkillID            gameconfig.BattleSkillID
}

// BuildSnapshot creates a BattleSnapshot from the current session state.
// If logs is non-nil the caller may embed them elsewhere (e.g. EventBatch);
// this method does NOT store logs inside the snapshot.
func (s *BattleSession) BuildSnapshot() BattleSnapshot {
	status := s.deriveStatus()

	next := s.NextEventTime()
	var nextStepIn float64
	if next != nil {
		nextStepIn = max(0, *next-s.Time)
	}

	var players []BattleEntityState
	for _, p := range s.Players {
		players = append(players, BuildEntityState(p, s.Time))
	}
	var enemies []BattleEntityState
	for _, e := range s.Enemies {
		enemies = append(enemies, BuildEntityState(e, s.Time))
	}

	return BattleSnapshot{
		BattleID:   s.Config.ID,
		Status:     status,
		Time:       s.Time,
		WaveNumber: s.WaveNumber,
		NextStepIn: nextStepIn,
		Players:    players,
		Enemies:    enemies,
	}
}

func (s *BattleSession) deriveStatus() string {
	if !s.Running {
		return "stopped"
	}
	if len(s.AlivePlayers()) == 0 {
		return "respawn"
	}
	if len(s.AliveEnemies()) == 0 && s.NextWaveTime != nil {
		return "between_waves"
	}
	return "fighting"
}

func BuildEntityState(e BattleEntity, now float64) BattleEntityState {
	cooldownSeconds := e.GetFinal(AttrAttackInterval)
	var progress float64
	if cooldownSeconds > 0 {
		remaining := e.NextReadyTime() - now
		progress = 1 - clamp(remaining/cooldownSeconds, 0, 1)
	}

	return BattleEntityState{
		EntityID:               e.EntityID(),
		Alive:                  e.Alive(),
		HP:                     round3(e.HP()),
		MaxHP:                  round3(e.MaxHP()),
		MP:                     round3(e.MP()),
		MaxMP:                  round3(e.MaxMP()),
		SP:                     round3(e.SP()),
		MaxSP:                  round3(e.MaxSP()),
		NextReadyIn:            round3(max(0, e.NextReadyTime()-now)),
		ActionCooldownSeconds:  round3(cooldownSeconds),
		ActionCooldownProgress: round3(progress),
		LastSkillID:            e.LastSkillID(),
	}
}
