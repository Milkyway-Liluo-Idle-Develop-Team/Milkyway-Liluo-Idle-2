package session_test

import (
	"sync"
	"testing"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/session"
	"github.com/google/uuid"
)

// TestTickDuringStateClosing verifies that a tick executing while the grace
// timer callback is in StateClosing does not panic and eventually stops
// ticking the session after it becomes StateClosed.
//
// Purpose: Stress-test the narrow window between setState(StateClosing) and
// setState(StateClosed) inside the grace timer callback.
//
// What it prevents: Panic or data corruption when parallelTick races with
// the grace timer callback. The callback runs FlushAll (holding s.mu) while
// runTick may also be trying to acquire s.mu.
func TestTickDuringStateClosing(t *testing.T) {
	database := openFullDBForTest(t)
	reg := newRegForTick()
	mgr := session.NewManagerWithoutTick(reg, database)

	s := createTestSession(t, mgr, database, 1)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetLastTick(base)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	startingDialog, _ := gameconfig.StringToEventID("starting_dialog_5")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	locked, _ := mgr.LockSession(s.UserID)
	locked.Bestiary().UnlockEvent(startingDialog)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	mgr.UnlockSession(locked)

	// Synchronize with the grace callback: signal when it enters StateClosing,
	// then hold it open so we can tick inside the window.
	enteredClosing := make(chan struct{})
	var closingWG sync.WaitGroup
	closingWG.Add(1)
	s.SetOnGraceExpire(func() {
		close(enteredClosing)
		// Hold StateClosing open briefly to widen the race window.
		time.Sleep(50 * time.Millisecond)
		mgr.Remove(s.UserID)
		closingWG.Done()
	})

	s.DetachConn()
	s.StartGraceTimer(0)

	// Wait until the callback has set StateClosing and is inside onGraceExpire.
	<-enteredClosing

	// Tick while session is in StateClosing. This must not panic.
	// Because runTick acquires s.mu and onGraceExpire also acquires s.mu,
	// they serialize; the test verifies no deadlock.
	mgr.ManualTick(base.Add(1 * time.Second))

	// Wait for callback to finish (StateClosed + Remove).
	closingWG.Wait()
	time.Sleep(20 * time.Millisecond)

	// After grace expiry the session must be removed from the manager.
	if _, ok := mgr.Get(s.UserID); ok {
		t.Error("session should be removed after grace expiry completes")
	}
}

// TestTickAfterRemove verifies that removing a session from the manager
// immediately stops it from being ticked in subsequent tick rounds.
//
// Purpose: Ensure that getAllSessions (which copies the current session
// slice) does not resurrect a removed session.
//
// What it prevents: Use-after-remove where a session pointer lingering in
// a worker goroutine continues to be settled after it has been removed.
func TestTickAfterRemove(t *testing.T) {
	database := openFullDBForTest(t)
	reg := newRegForTick()
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := createTestSession(t, mgr, database, 1)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetLastTick(base)

	fellingID, _ := gameconfig.StringToEventID("felling_oak_tree")
	startingDialog, _ := gameconfig.StringToEventID("starting_dialog_5")
	fellingSkill, _ := gameconfig.StringToSkillID("felling")
	locked, _ := mgr.LockSession(s.UserID)
	locked.Bestiary().UnlockEvent(startingDialog)
	locked.Events().Enqueue(0, fellingID, -1)
	locked.Skill().AddXP(fellingSkill, 100)
	mgr.UnlockSession(locked)

	// Tick 1: session is present, 3s > loop_time (~2s) so event triggers.
	r1 := mgr.ManualTick(base.Add(3 * time.Second))
	if len(r1) != 1 {
		t.Fatalf("expected 1 result before remove, got %d", len(r1))
	}

	// Remove session.
	mgr.Remove(s.UserID)

	// Tick 2: session is gone, should produce no results.
	r2 := mgr.ManualTick(base.Add(2 * time.Second))
	if len(r2) != 0 {
		t.Fatalf("expected 0 results after remove, got %d", len(r2))
	}
}

// TestCommandDuringStateClosing verifies that SubmitCommand behaves safely
// when the session is in StateClosing (grace callback running).
//
// Purpose: SubmitCommand only checks StateClosed, not StateClosing. A command
// submitted during StateClosing may enter the channel but never be drained
// because the tick loop stops after StateClosed. The test ensures the caller
// does not deadlock.
//
// What it prevents: Deadlock in WebSocket handlers where SubmitCommand
// blocks forever because the command channel is never drained after grace.
func TestCommandDuringStateClosing(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	// Hook the grace callback to hold StateClosing open.
	enteredClosing := make(chan struct{})
	s.SetOnGraceExpire(func() {
		close(enteredClosing)
		time.Sleep(100 * time.Millisecond)
	})

	s.DetachConn()
	s.StartGraceTimer(0)

	// Wait until callback is inside StateClosing.
	<-enteredClosing

	// Start a background ticker to drain commands while in StateClosing.
	stopTick := make(chan struct{})
	var tickWG sync.WaitGroup
	tickWG.Add(1)
	go func() {
		defer tickWG.Done()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopTick:
				return
			case <-ticker.C:
				mgr.ManualTick(time.Now())
			}
		}
	}()

	// Try to submit a command while in StateClosing.
	// Because the tick loop may still drain commands (parallelTick does not
	// check StateClosing), we cannot guarantee execution. We only verify
	// that it does not deadlock.
	done := make(chan error, 1)
	go func() {
		err := s.SubmitCommand(func(sess *session.PlayerSession) error {
			sess.Attr().GetFinal(1)
			return nil
		})
		done <- err
	}()

	select {
	case err := <-done:
		// Acceptable outcomes: executed successfully, or returned error.
		_ = err
	case <-time.After(2 * time.Second):
		close(stopTick)
		tickWG.Wait()
		t.Fatal("SubmitCommand deadlocked during StateClosing")
	}

	close(stopTick)
	tickWG.Wait()
}
