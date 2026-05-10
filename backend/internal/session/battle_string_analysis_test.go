package session

import (
	"fmt"
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"google.golang.org/protobuf/proto"
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
)

func init() {
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic(err)
		}
	}
	battle.LoadAttrIDs()
}

func makeStringAnalysisPlayer(userID int64, name string, pp float64) *battle.PlayerBattleEntity {
	inst := attribute.NewInstance()
	reg := attribute.Get()
	var mods []attribute.Modifier
	if id, ok := reg.AttrID("physical_power"); ok {
		mods = append(mods, attribute.Modifier{AttrID: id, Op: attribute.OpOverride, Value: pp, Source: "test"})
	}
	if id, ok := reg.AttrID("accuracy"); ok {
		mods = append(mods, attribute.Modifier{AttrID: id, Op: attribute.OpOverride, Value: 1000, Source: "test"})
	}
	if len(mods) > 0 {
		inst.AddModifiers("test", mods)
	}
	p := battle.NewPlayerBattleEntity(userID, name, inst)
	p.SetHP(p.MaxHP())
	p.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
		gameconfig.BattleSkillID(1): {
			ID: gameconfig.BattleSkillID(1),
			Name: "基础攻击",
			Damage: &battle.DamageProfile{
				Type:       "physical",
				Flat:       0,
				Multiplier: 1.0,
			},
			CastTime: 2.0,
			IsBasic:  true,
		},
	})
	p.SetBasicSkillID(gameconfig.BattleSkillID(1))
	p.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})
	return p
}

// stripStrings returns a copy of the message with all string fields set to empty.
func stripStringsBattleLogEntry(e *pb.BattleLogEntry) *pb.BattleLogEntry {
	return &pb.BattleLogEntry{
		Type:             pb.BattleLogType_BATTLE_LOG_TYPE_UNSPECIFIED,
		AttackerEntityId: 0,
		DefenderEntityId: 0,
		SkillId: 0,
		Damage:           e.Damage,
		RawDamage:        e.RawDamage,
		Evaded:           e.Evaded,
		Blocked:          e.Blocked,
		BlockedReduction: e.BlockedReduction,
		DefenderHp:       e.DefenderHp,
		WaveNumber:       e.WaveNumber,
		NextWaveIn:       e.NextWaveIn,
	}
}

func stripStringsEntityState(e *pb.BattleEntityState) *pb.BattleEntityState {
	return &pb.BattleEntityState{
		EntityId:               0,
		Alive:                  e.Alive,
		Hp:                     e.Hp,
		MaxHp:                  e.MaxHp,
		Mp:                     e.Mp,
		MaxMp:                  e.MaxMp,
		Sp:                     e.Sp,
		MaxSp:                  e.MaxSp,
		NextReadyIn:            e.NextReadyIn,
		ActionCooldownSeconds:  e.ActionCooldownSeconds,
		ActionCooldownProgress: e.ActionCooldownProgress,
		LastSkillId: 0,
	}
}

func stripStringsSnapshot(s *pb.BattleSnapshot) *pb.BattleSnapshot {
	out := &pb.BattleSnapshot{
		BattleId:   "",
		Status:     s.Status,
		Time:       s.Time,
		WaveNumber: s.WaveNumber,
		NextStepIn: s.NextStepIn,
	}
	for _, p := range s.Players {
		out.Players = append(out.Players, stripStringsEntityState(p))
	}
	for _, e := range s.Enemies {
		out.Enemies = append(out.Enemies, stripStringsEntityState(e))
	}
	return out
}

func stripStringsEventBatch(b *pb.BattleEventBatch) *pb.BattleEventBatch {
	out := &pb.BattleEventBatch{Time: b.Time}
	for _, l := range b.Logs {
		out.Logs = append(out.Logs, stripStringsBattleLogEntry(l))
	}
	for _, e := range b.AffectedEntities {
		out.AffectedEntities = append(out.AffectedEntities, stripStringsEntityState(e))
	}
	return out
}

func TestAnalyzeStringUsage(t *testing.T) {
	p := makeStringAnalysisPlayer(1, "Hero", 200)
	enemy := battle.NewEnemyBattleEntity(1, 0, "Goblin", map[string]float64{
		"hp": 500, "defense": 0, "evade": 1, "attack_interval": 5,
	})
	enemy.SetHP(enemy.MaxHP())
	enemy.SetNextReadyTime(5.0)
	enemy.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
		gameconfig.BattleSkillID(1): {ID: gameconfig.BattleSkillID(1), Name: "基础攻击", Damage: &battle.DamageProfile{Type: "physical", Flat: 0, Multiplier: 1.0}, CastTime: 2.0, IsBasic: true},
	})
	enemy.SetBasicSkillID(gameconfig.BattleSkillID(1))
	enemy.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})

	sess := battle.NewBattleSession(battle.BattleConfig{NumericID: 1, ID: "test", Name: "Test", Map: "village", Interval: 10.0}, []*battle.PlayerBattleEntity{p})
	sess.Enemies = append(sess.Enemies, enemy)
	sess.NextWaveTime = nil
	p.SetNextReadyTime(2.0)

	var logs []battle.BattleLog
	for sess.Running && len(logs) == 0 {
		logs = sess.AdvanceOneEvent()
	}

	batch := battleLogsToProto(sess, logs)
	snap := sess.BuildSnapshot()
	snapPb := BattleSnapshotToProto(&snap)

	fmt.Printf("\n=== String Field Analysis ===\n\n")

	// --- BattleLogEntry ---
	for _, e := range batch.Logs {
		full, _ := proto.Marshal(e)
		stripped, _ := proto.Marshal(stripStringsBattleLogEntry(e))
		strBytes := len(full) - len(stripped)
		pct := float64(strBytes) * 100 / float64(len(full))
		fmt.Printf("BattleLogEntry[%s]: %d bytes total, %d bytes strings (%.1f%%)\n",
			e.Type, len(full), strBytes, pct)
		fmt.Printf("  string fields: skill_id=%q\n", e.SkillId)
		fmt.Printf("  numeric fields: attacker=%d defender=%d\n", e.AttackerEntityId, e.DefenderEntityId)
	}

	// --- BattleEntityState ---
	for _, e := range batch.AffectedEntities {
		full, _ := proto.Marshal(e)
		stripped, _ := proto.Marshal(stripStringsEntityState(e))
		strBytes := len(full) - len(stripped)
		pct := float64(strBytes) * 100 / float64(len(full))
		fmt.Printf("\nBattleEntityState[%d]: %d bytes total, %d bytes strings (%.1f%%)\n",
			e.EntityId, len(full), strBytes, pct)
		fmt.Printf("  string fields: last_skill_id=%q\n", e.LastSkillId)
		fmt.Printf("  numeric fields: entity_id=%d\n", e.EntityId)
	}

	// --- BattleEventBatch total ---
	batchFull, _ := proto.Marshal(batch)
	batchStripped, _ := proto.Marshal(stripStringsEventBatch(batch))
	batchStrBytes := len(batchFull) - len(batchStripped)
	fmt.Printf("\nBattleEventBatch: %d bytes total, %d bytes strings (%.1f%%)\n",
		len(batchFull), batchStrBytes, float64(batchStrBytes)*100/float64(len(batchFull)))

	// --- BattleSnapshot total ---
	snapFull, _ := proto.Marshal(snapPb)
	snapStripped, _ := proto.Marshal(stripStringsSnapshot(snapPb))
	snapStrBytes := len(snapFull) - len(snapStripped)
	fmt.Printf("BattleSnapshot: %d bytes total, %d bytes strings (%.1f%%)\n",
		len(snapFull), snapStrBytes, float64(snapStrBytes)*100/float64(len(snapFull)))
	fmt.Printf("  snapshot strings: battle_id=%q\n", snapPb.BattleId)

	// --- Envelope ---
	fmt.Printf("\n=== Envelope Type Strings ===\n")
	fmt.Printf("  'battle.event_batch' = %d bytes per packet\n", len("battle.event_batch"))
	fmt.Printf("  'battle.snapshot'    = %d bytes per packet\n", len("battle.snapshot"))
}
