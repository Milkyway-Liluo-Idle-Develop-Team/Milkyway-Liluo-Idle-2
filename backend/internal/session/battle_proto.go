package session

import (
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
)

var statusToProto = map[string]pb.BattleStatus{
	"fighting":       pb.BattleStatus_BATTLE_STATUS_FIGHTING,
	"between_waves":  pb.BattleStatus_BATTLE_STATUS_BETWEEN_WAVES,
	"respawn":        pb.BattleStatus_BATTLE_STATUS_RESPAWN,
	"stopped":        pb.BattleStatus_BATTLE_STATUS_STOPPED,
}

var logTypeToProto = map[battle.BattleLogType]pb.BattleLogType{
	battle.BattleLogTypeUnspecified:      pb.BattleLogType_BATTLE_LOG_TYPE_UNSPECIFIED,
	battle.BattleLogTypePlayerAttack:     pb.BattleLogType_BATTLE_LOG_TYPE_PLAYER_ATTACK,
	battle.BattleLogTypeEnemyAttack:      pb.BattleLogType_BATTLE_LOG_TYPE_ENEMY_ATTACK,
	battle.BattleLogTypeEnemyDied:        pb.BattleLogType_BATTLE_LOG_TYPE_ENEMY_DIED,
	battle.BattleLogTypePlayerDowned:     pb.BattleLogType_BATTLE_LOG_TYPE_PLAYER_DOWNED,
	battle.BattleLogTypePlayerRespawn:    pb.BattleLogType_BATTLE_LOG_TYPE_PLAYER_RESPAWN,
	battle.BattleLogTypeWaveSpawned:      pb.BattleLogType_BATTLE_LOG_TYPE_WAVE_SPAWNED,
	battle.BattleLogTypeWaveCleared:      pb.BattleLogType_BATTLE_LOG_TYPE_WAVE_CLEARED,
	battle.BattleLogTypeAllPlayersDowned: pb.BattleLogType_BATTLE_LOG_TYPE_ALL_PLAYERS_DOWNED,
	battle.BattleLogTypeStopped:          pb.BattleLogType_BATTLE_LOG_TYPE_STOPPED,
}

func battleLogsToProto(s *battle.BattleSession, logs []battle.BattleLog) *pb.BattleEventBatch {
	batch := &pb.BattleEventBatch{
		Time: round3(s.Time),
		Logs: make([]*pb.BattleLogEntry, 0, len(logs)),
	}

	affected := make(map[int64]struct{})
	for _, l := range logs {
		protoType := pb.BattleLogType_BATTLE_LOG_TYPE_UNSPECIFIED
		if t, ok := logTypeToProto[l.Type]; ok {
			protoType = t
		}

		entry := &pb.BattleLogEntry{
			Type:             protoType,
			AttackerEntityId: l.AttackerEntityID,
			DefenderEntityId: l.DefenderEntityID,
			SkillId:          int64(l.SkillID),
			Damage:           l.Damage,
			RawDamage:        l.RawDamage,
			Evaded:           l.Evaded,
			Blocked:          l.Blocked,
			BlockedReduction: l.BlockedReduction,
			DefenderHp:       l.DefenderHP,
			WaveNumber:       int32(l.WaveNumber),
			NextWaveIn:       l.NextWaveIn,
		}
		batch.Logs = append(batch.Logs, entry)

		if l.AttackerEntityID != 0 {
			affected[l.AttackerEntityID] = struct{}{}
		}
		if l.DefenderEntityID != 0 {
			affected[l.DefenderEntityID] = struct{}{}
		}
	}

	// Include latest state for every entity that appears in the logs.
	for _, p := range s.Players {
		if _, ok := affected[p.EntityID()]; ok {
			batch.AffectedEntities = append(batch.AffectedEntities, EntityStateToProto(battle.BuildEntityState(p, s.Time)))
		}
	}
	for _, e := range s.Enemies {
		if _, ok := affected[e.EntityID()]; ok {
			batch.AffectedEntities = append(batch.AffectedEntities, EntityStateToProto(battle.BuildEntityState(e, s.Time)))
		}
	}

	return batch
}

func BattleSnapshotToProto(snap *battle.BattleSnapshot) *pb.BattleSnapshot {
	status := pb.BattleStatus_BATTLE_STATUS_UNSPECIFIED
	if s, ok := statusToProto[snap.Status]; ok {
		status = s
	}

	p := &pb.BattleSnapshot{
		BattleId:   snap.BattleID,
		Status:     status,
		Time:       snap.Time,
		WaveNumber: int32(snap.WaveNumber),
		NextStepIn: snap.NextStepIn,
		Players:    make([]*pb.BattleEntityState, 0, len(snap.Players)),
		Enemies:    make([]*pb.BattleEntityState, 0, len(snap.Enemies)),
	}

	for _, es := range snap.Players {
		p.Players = append(p.Players, EntityStateToProto(es))
	}
	for _, es := range snap.Enemies {
		p.Enemies = append(p.Enemies, EntityStateToProto(es))
	}

	return p
}

func EntityStateToProto(es battle.BattleEntityState) *pb.BattleEntityState {
	return &pb.BattleEntityState{
		EntityId:               es.EntityID,
		Alive:                  es.Alive,
		Hp:                     es.HP,
		MaxHp:                  es.MaxHP,
		Mp:                     es.MP,
		MaxMp:                  es.MaxMP,
		Sp:                     es.SP,
		MaxSp:                  es.MaxSP,
		NextReadyIn:            es.NextReadyIn,
		ActionCooldownSeconds:  es.ActionCooldownSeconds,
		ActionCooldownProgress: es.ActionCooldownProgress,
		LastSkillId:            int64(es.LastSkillID),
	}
}

func round3(v float64) float64 {
	return float64(int64(v*1000+0.5)) / 1000
}
