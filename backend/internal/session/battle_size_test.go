package session

import (
	"fmt"
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"google.golang.org/protobuf/proto"
)

func init() {
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic(err)
		}
	}
	battle.LoadAttrIDs()
}

func makeSizeTestPlayer(userID int64, name string, pp float64) *battle.PlayerBattleEntity {
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

func TestMeasureBattlePacketSizes(t *testing.T) {
	fmt.Printf("\n=== Battle Packet Size Summary ===\n")

	// ── Scenario 1: Solo, 1 enemy, high HP (typical attack, no death) ──
	p1 := makeSizeTestPlayer(1, "Hero", 200)
	e1 := battle.NewEnemyBattleEntity(1, 0, "Goblin", map[string]float64{
		"hp":              500, "defense": 0, "evade": 1, "attack_interval": 5,
	})
	e1.SetHP(e1.MaxHP())
	e1.SetNextReadyTime(5.0)
	e1.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
		gameconfig.BattleSkillID(1): {ID: gameconfig.BattleSkillID(1), Name: "基础攻击", Damage: &battle.DamageProfile{Type: "physical", Flat: 0, Multiplier: 1.0}, CastTime: 2.0, IsBasic: true},
	})
	e1.SetBasicSkillID(gameconfig.BattleSkillID(1))
	e1.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})

	s1 := battle.NewBattleSession(battle.BattleConfig{NumericID: 1, ID: "test", Name: "Test", Map: "village", Interval: 10.0}, []*battle.PlayerBattleEntity{p1})
	s1.Enemies = append(s1.Enemies, e1)
	s1.NextWaveTime = nil
	p1.SetNextReadyTime(2.0)

	var logs1 []battle.BattleLog
	for s1.Running && len(logs1) == 0 {
		logs1 = s1.AdvanceOneEvent()
	}
	batch1 := battleLogsToProto(s1, logs1)
	batch1Bytes, _ := proto.Marshal(batch1)
	snap1 := s1.BuildSnapshot()
	snap1Bytes, _ := proto.Marshal(BattleSnapshotToProto(&snap1))
	t.Logf("[Solo1v1] EventBatch (%d logs, %d entities): %d bytes", len(batch1.Logs), len(batch1.AffectedEntities), len(batch1Bytes))
	t.Logf("[Solo1v1] Snapshot  (1 player, 1 enemy): %d bytes", len(snap1Bytes))
	fmt.Printf("Solo 1v1 (1 attack, no death):\n")
	fmt.Printf("  EventBatch: %d bytes (%d logs, %d entities)\n", len(batch1Bytes), len(batch1.Logs), len(batch1.AffectedEntities))
	fmt.Printf("  Snapshot:   %d bytes\n", len(snap1Bytes))

	// ── Scenario 2: Solo, 1 enemy, low HP (attack + death) ──
	p2 := makeSizeTestPlayer(1, "Hero", 200)
	e2 := battle.NewEnemyBattleEntity(1, 0, "Goblin", map[string]float64{
		"hp": 50, "defense": 0, "evade": 1, "attack_interval": 5,
	})
	e2.SetHP(e2.MaxHP())
	e2.SetNextReadyTime(5.0)
	e2.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
		gameconfig.BattleSkillID(1): {ID: gameconfig.BattleSkillID(1), Name: "基础攻击", Damage: &battle.DamageProfile{Type: "physical", Flat: 0, Multiplier: 1.0}, CastTime: 2.0, IsBasic: true},
	})
	e2.SetBasicSkillID(gameconfig.BattleSkillID(1))
	e2.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})

	s2 := battle.NewBattleSession(battle.BattleConfig{NumericID: 1, ID: "test", Name: "Test", Map: "village", Interval: 10.0}, []*battle.PlayerBattleEntity{p2})
	s2.Enemies = append(s2.Enemies, e2)
	s2.NextWaveTime = nil
	p2.SetNextReadyTime(2.0)

	var logs2 []battle.BattleLog
	for s2.Running && len(logs2) == 0 {
		logs2 = s2.AdvanceOneEvent()
	}
	batch2 := battleLogsToProto(s2, logs2)
	batch2Bytes, _ := proto.Marshal(batch2)
	snap2 := s2.BuildSnapshot()
	snap2Bytes, _ := proto.Marshal(BattleSnapshotToProto(&snap2))
	fmt.Printf("Solo 1v1 (attack + enemy death):\n")
	fmt.Printf("  EventBatch: %d bytes (%d logs, %d entities)\n", len(batch2Bytes), len(batch2.Logs), len(batch2.AffectedEntities))
	fmt.Printf("  Snapshot:   %d bytes\n", len(snap2Bytes))

	// ── Scenario 3: 2 players, 3 enemies (multiplayer) ──
	p3a := makeSizeTestPlayer(1, "Alice", 100)
	p3b := makeSizeTestPlayer(2, "Bob", 100)
	cfg3 := battle.BattleConfig{NumericID: 1, ID: "test", Name: "Test", Map: "village", Interval: 10.0}
	s3 := battle.NewBattleSession(cfg3, []*battle.PlayerBattleEntity{p3a, p3b})
	for i := 0; i < 3; i++ {
		e := battle.NewEnemyBattleEntity(int64(i+1), i, fmt.Sprintf("Goblin-%d", i), map[string]float64{
			"hp": 200, "defense": 10, "evade": 5, "attack_interval": 3,
		})
		e.SetHP(e.MaxHP())
		e.SetNextReadyTime(5.0)
		e.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
			gameconfig.BattleSkillID(1): {ID: gameconfig.BattleSkillID(1), Name: "基础攻击", Damage: &battle.DamageProfile{Type: "physical", Flat: 0, Multiplier: 1.0}, CastTime: 2.0, IsBasic: true},
		})
		e.SetBasicSkillID(gameconfig.BattleSkillID(1))
		e.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})
		s3.Enemies = append(s3.Enemies, e)
	}
	s3.NextWaveTime = nil
	p3a.SetNextReadyTime(2.0)
	p3b.SetNextReadyTime(2.0)

	var logs3 []battle.BattleLog
	for s3.Running && len(logs3) == 0 {
		logs3 = s3.AdvanceOneEvent()
	}
	batch3 := battleLogsToProto(s3, logs3)
	batch3Bytes, _ := proto.Marshal(batch3)
	snap3 := s3.BuildSnapshot()
	snap3Bytes, _ := proto.Marshal(BattleSnapshotToProto(&snap3))
	fmt.Printf("Party 2p + 3 enemies (first tick):\n")
	fmt.Printf("  EventBatch: %d bytes (%d logs, %d entities)\n", len(batch3Bytes), len(batch3.Logs), len(batch3.AffectedEntities))
	fmt.Printf("  Snapshot:   %d bytes (players=%d, enemies=%d)\n", len(snap3Bytes), len(snap3.Players), len(snap3.Enemies))

	// ── Component sizes ──
	for _, l := range batch1.Logs {
		b, _ := proto.Marshal(l)
		fmt.Printf("  LogEntry[%s]: %d bytes\n", l.Type, len(b))
	}
	for _, e := range batch1.AffectedEntities {
		b, _ := proto.Marshal(e)
		fmt.Printf("  EntityState[%d]: %d bytes\n", e.EntityId, len(b))
	}

	// ── Envelope overhead ──
	fmt.Printf("Envelope overhead: ~%d bytes\n", len("battle.event_batch")+8)

	// ── Bandwidth estimate ──
	eventsPerSec := 3.0 // player + 2 enemies attacking
	snapshotRate := 0.5 // every 2s
	fmt.Printf("\n=== Downstream Bandwidth Estimate (solo 1v1, active combat) ===\n")
	fmt.Printf("Event batches: %.0f events/s × %d bytes ≈ %.0f bytes/s\n", eventsPerSec, len(batch1Bytes), eventsPerSec*float64(len(batch1Bytes)))
	fmt.Printf("Snapshots:     %.1f/s × %d bytes ≈ %.0f bytes/s\n", snapshotRate, len(snap1Bytes), snapshotRate*float64(len(snap1Bytes)))
	fmt.Printf("Total:         ≈ %.1f KB/s per player\n", (eventsPerSec*float64(len(batch1Bytes))+snapshotRate*float64(len(snap1Bytes)))/1024)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
