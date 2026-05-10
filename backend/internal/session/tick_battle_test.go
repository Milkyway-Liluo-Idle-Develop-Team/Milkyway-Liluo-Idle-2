package session_test

import (
	"testing"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/session"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/wsx"
	"github.com/google/uuid"
)

// TestTickBattlesAdvancesBattleOnce verifies that ManualTick advances a
// registered battle session by exactly one tick delta.
func TestTickBattlesAdvancesBattleOnce(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	p := makeTestBattlePlayer(1, "Hero")
	bs := battle.NewBattleSession(battle.BattleConfig{
		NumericID: 1,
		ID:        "test",
		Name:      "Test",
		Map:       "village",
		Interval:  10.0,
	}, []*battle.PlayerBattleEntity{p})

	s.SetBattleSession(bs)
	mgr.AddBattle(bs)

	if bs.Time != 0 {
		t.Fatalf("expected time 0, got %v", bs.Time)
	}

	mgr.ManualTick(time.Now())

	if bs.Time != 0.05 {
		t.Fatalf("expected time 0.05 after one tick, got %v", bs.Time)
	}
}

// TestTickBattlesNoDuplicateWithSharedBattle verifies that when two
// PlayerSessions share the same BattleSession, it is advanced exactly once
// per tick round (not twice).
func TestTickBattlesNoDuplicateWithSharedBattle(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s1 := session.New(uuid.New(), 1, testLogger())
	s2 := session.New(uuid.New(), 2, testLogger())
	mgr.Add(s1)
	mgr.Add(s2)

	p1 := makeTestBattlePlayer(1, "P1")
	p2 := makeTestBattlePlayer(2, "P2")
	bs := battle.NewBattleSession(battle.BattleConfig{
		NumericID: 1,
		ID:        "test",
		Name:      "Test",
		Map:       "village",
		Interval:  10.0,
	}, []*battle.PlayerBattleEntity{p1, p2})

	// Both sessions point to the same BattleSession.
	s1.SetBattleSession(bs)
	s2.SetBattleSession(bs)
	mgr.AddBattle(bs)

	mgr.ManualTick(time.Now())

	if bs.Time != 0.05 {
		t.Fatalf("shared battle time want 0.05, got %v (double-ticked?)", bs.Time)
	}
}

// TestTickBattlesBroadcastsEvents verifies that combat logs produced during
// a tick are broadcast to every participating player.
// Because NewBattleSession clamps the interval to a minimum of 0.1s, a single
// 50ms tick does not reach the wave spawn point. We tick twice to cross the
// threshold and trigger the wave.
func TestTickBattlesBroadcastsEvents(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s1 := session.New(uuid.New(), 1, testLogger())
	s2 := session.New(uuid.New(), 2, testLogger())
	mgr.Add(s1)
	mgr.Add(s2)

	conn1, recv1 := wsx.NewTestConn(1, 16)
	conn2, recv2 := wsx.NewTestConn(2, 16)
	s1.AttachConn(conn1)
	s2.AttachConn(conn2)

	p1 := makeTestBattlePlayer(1, "P1")
	p2 := makeTestBattlePlayer(2, "P2")
	bs := battle.NewBattleSession(battle.BattleConfig{
		NumericID: 1,
		ID:        "test",
		Name:      "Test",
		Map:       "village",
		Interval:  0.05, // clamped to 0.1 internally
		CombinationLoop: []string{"weak"},
		WeakEnemyCombinations: []battle.EnemyWaveCombination{
			{Enemies: []string{"goblin"}, Weight: 100},
		},
	}, []*battle.PlayerBattleEntity{p1, p2})

	s1.SetBattleSession(bs)
	s2.SetBattleSession(bs)
	mgr.AddBattle(bs)

	// First tick: Time 0 → 0.05 (no event yet).
	mgr.ManualTick(time.Now())
	// Second tick: Time 0.05 → 0.10 (crosses NextWaveTime=0.1).
	mgr.ManualTick(time.Now())

	var got1, got2 bool
	deadline := time.AfterFunc(500*time.Millisecond, func() {
		conn1.Close()
		conn2.Close()
	})
	defer deadline.Stop()

	for !got1 || !got2 {
		select {
		case msg := <-recv1:
			if msg.Type == "battle.event_batch" {
				got1 = true
			}
		case msg := <-recv2:
			if msg.Type == "battle.event_batch" {
				got2 = true
			}
		case <-time.After(200 * time.Millisecond):
			goto done
		}
	}
done:

	if !got1 {
		t.Error("player 1 did not receive battle.event_batch")
	}
	if !got2 {
		t.Error("player 2 did not receive battle.event_batch")
	}
}

// TestTickBattlesSnapshotHeartbeat verifies that heartbeat snapshots are
// broadcast every ~2s (40 ticks at 50ms).
func TestTickBattlesSnapshotHeartbeat(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	conn, recv := wsx.NewTestConn(1, 64)
	s.AttachConn(conn)

	p := makeTestBattlePlayer(1, "Hero")
	bs := battle.NewBattleSession(battle.BattleConfig{
		NumericID: 1,
		ID:        "test",
		Name:      "Test",
		Map:       "village",
		Interval:  10.0,
	}, []*battle.PlayerBattleEntity{p})

	s.SetBattleSession(bs)
	mgr.AddBattle(bs)

	var snapshotCount int
	deadline := time.AfterFunc(500*time.Millisecond, func() { conn.Close() })
	defer deadline.Stop()

	// Tick 40 times; the 40th tick should broadcast a snapshot.
	for i := 0; i < 40; i++ {
		mgr.ManualTick(time.Now())
		for {
			select {
			case msg := <-recv:
				if msg.Type == "battle.snapshot" {
					snapshotCount++
				}
			default:
				goto nextTick
			}
		}
	nextTick:
	}

	if snapshotCount != 1 {
		t.Fatalf("expected 1 heartbeat snapshot, got %d", snapshotCount)
	}
}
