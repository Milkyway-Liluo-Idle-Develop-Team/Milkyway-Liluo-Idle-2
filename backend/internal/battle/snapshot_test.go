package battle_test

import (
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

func TestAdvanceByDeltaEmptyTick(t *testing.T) {
	p := makeTestPlayer(1, "Hero")
	cfg := battle.BattleConfig{
		ID:              "test",
		Name:            "Test",
		Map:             "village",
		Interval:        10.0,
		CombinationLoop: []string{"weak"},
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})

	// At t=0, next event is wave spawn at t=10.
	// Advance 0.05s — no events should fire.
	logs := sess.AdvanceByDelta(0.05)
	if len(logs) != 0 {
		t.Errorf("expected no logs for empty tick, got %d", len(logs))
	}
	if sess.Time != 0.05 {
		t.Errorf("time want 0.05, got %v", sess.Time)
	}
}

func TestAdvanceByDeltaProcessesEvent(t *testing.T) {
	p := makeTestPlayer(1, "Hero")
	cfg := battle.BattleConfig{
		ID:              "test",
		Name:            "Test",
		Map:             "village",
		Interval:        3.0,
		CombinationLoop: []string{"weak"},
		WeakEnemyCombinations: []battle.EnemyWaveCombination{
			{Enemies: []string{"goblin"}, Weight: 100},
		},
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})

	// Wave spawn at t=3. Advance 5s to capture it.
	logs := sess.AdvanceByDelta(5.0)

	foundSpawn := false
	for _, l := range logs {
		if l.Type == battle.BattleLogTypeWaveSpawned {
			foundSpawn = true
		}
	}
	if !foundSpawn {
		t.Errorf("expected wave_spawned in logs, got %+v", logs)
	}
	if sess.Time != 5.0 {
		t.Errorf("time want 5.0, got %v", sess.Time)
	}
	if len(sess.Enemies) != 1 {
		t.Fatalf("expected 1 enemy after spawn, got %d", len(sess.Enemies))
	}
}

func TestAdvanceByDeltaMultipleEvents(t *testing.T) {
	// Use a custom high-HP enemy so it survives multiple rounds.
	p := makeTestPlayer(1, "Hero")
	cfg := battle.BattleConfig{
		ID:              "test",
		Name:            "Test",
		Map:             "village",
		Interval:        10.0,
		CombinationLoop: []string{"weak"},
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})

	// Manually inject a high-HP enemy.
	enemy := battle.NewEnemyBattleEntity(1, 0, "Tank", map[string]float64{
		"hp":              5000,
		"physical_power":  10,
		"defense":         100,
		"attack_interval": 2,
		"accuracy":        1000,
	})
	enemy.SetHP(enemy.MaxHP())
	enemy.SetNextReadyTime(5.0)
	enemy.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
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
	enemy.SetBasicSkillID(gameconfig.BattleSkillID(1))
	enemy.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})
	sess.Enemies = append(sess.Enemies, enemy)
	sess.NextWaveTime = nil

	// Align player ready time with enemy so both attack at t=5.
	p.SetNextReadyTime(5.0)

	// Advance 5s should capture both player_attack and enemy_attack.
	logs := sess.AdvanceByDelta(5.0)

	var playerAttacks, enemyAttacks int
	for _, l := range logs {
		if l.Type == battle.BattleLogTypePlayerAttack {
			playerAttacks++
		}
		if l.Type == battle.BattleLogTypeEnemyAttack {
			enemyAttacks++
		}
	}

	if playerAttacks == 0 {
		t.Error("expected at least one player_attack")
	}
	if enemyAttacks == 0 {
		t.Error("expected at least one enemy_attack")
	}
	if sess.Time != 5.0 {
		t.Errorf("time want 5.0, got %v", sess.Time)
	}
}

func TestBuildSnapshot(t *testing.T) {
	p := makeTestPlayer(1, "Hero")
	p.SetHP(p.MaxHP())
	p.SetMP(p.MaxMP())
	p.SetSP(p.MaxSP())

	cfg := battle.BattleConfig{
		ID:              "test_snap",
		Name:            "Snapshot Test",
		Map:             "village",
		Interval:        3.0,
		CombinationLoop: []string{"weak"},
		WeakEnemyCombinations: []battle.EnemyWaveCombination{
			{Enemies: []string{"goblin"}, Weight: 100},
		},
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})

	snap := sess.BuildSnapshot()

	if snap.BattleID != "test_snap" {
		t.Errorf("battle_id want test_snap, got %s", snap.BattleID)
	}
	if snap.Status != "between_waves" {
		// No enemies spawned yet, next_wave_time is set, so status is between_waves.
		t.Errorf("status want between_waves, got %s", snap.Status)
	}
	if len(snap.Players) != 1 {
		t.Fatalf("players want 1, got %d", len(snap.Players))
	}

	playerState := snap.Players[0]
	if playerState.EntityID != p.EntityID() {
		t.Errorf("player entity_id want %d, got %d", p.EntityID(), playerState.EntityID)
	}
	if playerState.HP <= 0 {
		t.Errorf("player hp should be > 0, got %v", playerState.HP)
	}
	if playerState.MaxHP <= 0 {
		t.Errorf("player max_hp should be > 0, got %v", playerState.MaxHP)
	}
	if playerState.ActionCooldownProgress < 0 || playerState.ActionCooldownProgress > 1 {
		t.Errorf("progress should be in [0,1], got %v", playerState.ActionCooldownProgress)
	}

	// After spawning a wave, status should be fighting.
	sess.AdvanceByDelta(3.0)
	snap2 := sess.BuildSnapshot()
	if snap2.Status != "fighting" {
		t.Errorf("status after spawn want fighting, got %s", snap2.Status)
	}
	if len(snap2.Enemies) == 0 {
		t.Error("expected enemies in snapshot after spawn")
	}
}

func TestBuildSnapshotStatuses(t *testing.T) {
	p1 := makeTestPlayer(1, "P1")
	p2 := makeTestPlayer(2, "P2")

	cfg := battle.BattleConfig{
		ID:              "test_status",
		Name:            "Status Test",
		Map:             "village",
		Interval:        3.0,
		CombinationLoop: []string{"weak"},
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p1, p2})

	// Running, no enemies, next_wave_time set -> between_waves.
	if s := sess.BuildSnapshot().Status; s != "between_waves" {
		t.Errorf("initial status want between_waves, got %s", s)
	}

	// All players down, with respawn pending -> respawn.
	p1.SetAlive(false)
	p2.SetAlive(false)
	sess.RespawnTimes[p1.EntityID()] = ptr(10.0)
	if s := sess.BuildSnapshot().Status; s != "respawn" {
		t.Errorf("all dead status want respawn, got %s", s)
	}

	// Not running -> stopped.
	sess.Running = false
	if s := sess.BuildSnapshot().Status; s != "stopped" {
		t.Errorf("not running status want stopped, got %s", s)
	}
}

func ptr(v float64) *float64 {
	return &v
}
