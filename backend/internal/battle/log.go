package battle

import "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"

// BattleLogType categorises a single combat event.
type BattleLogType int

const (
	BattleLogTypeUnspecified      BattleLogType = 0
	BattleLogTypePlayerAttack     BattleLogType = 1
	BattleLogTypeEnemyAttack      BattleLogType = 2
	BattleLogTypeEnemyDied        BattleLogType = 3
	BattleLogTypePlayerDowned     BattleLogType = 4
	BattleLogTypePlayerRespawn    BattleLogType = 5
	BattleLogTypeWaveSpawned      BattleLogType = 6
	BattleLogTypeWaveCleared      BattleLogType = 7
	BattleLogTypeAllPlayersDowned BattleLogType = 8
	BattleLogTypeStopped          BattleLogType = 9
)

func (t BattleLogType) String() string {
	switch t {
	case BattleLogTypePlayerAttack:
		return "player_attack"
	case BattleLogTypeEnemyAttack:
		return "enemy_attack"
	case BattleLogTypeEnemyDied:
		return "enemy_died"
	case BattleLogTypePlayerDowned:
		return "player_downed"
	case BattleLogTypePlayerRespawn:
		return "player_respawn"
	case BattleLogTypeWaveSpawned:
		return "wave_spawned"
	case BattleLogTypeWaveCleared:
		return "wave_cleared"
	case BattleLogTypeAllPlayersDowned:
		return "all_players_downed"
	case BattleLogTypeStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// BattleLog is a single event produced by the combat engine.
// It is converted to protobuf before leaving the server.
type BattleLog struct {
	Type BattleLogType

	// Attack logs.
	SkillID          gameconfig.BattleSkillID
	Damage           float64
	RawDamage        float64
	Evaded           bool
	Blocked          bool
	BlockedReduction float64
	MPCost           float64 // internal only, not transmitted
	SPCost           float64 // internal only, not transmitted
	Effects          []AppliedEffect

	// Attacker / Defender identifiers (numeric, supports multiplayer).
	AttackerEntityID int64
	DefenderEntityID int64
	DefenderHP       float64

	// Meta.
	WaveNumber int
	NextWaveIn float64
	BattleID   string // internal only, not transmitted
}

// AppliedEffect records an effect that was actually applied.
type AppliedEffect struct {
	Target    string
	Attribute string
	Mode      string
	Value     float64
	Duration  float64
}
