package session_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/google/uuid"
)

// TestParallelTick_NoRaceUnderLoad runs TickAll with many sessions while
// goroutines perform concurrent operations. Must be run with -race.
//
// Purpose: Detect data races on PlayerSession fields (conn, state, lastTick,
// elapsedAccum) when multiple goroutines touch the same session concurrently.
//
// What it prevents: Hidden data races that only manifest under production
// load, such as concurrent read/write on connMu, or lastTick being updated
// by one worker while another worker reads it.
func TestParallelTick_NoRaceUnderLoad(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Use real background TickAll so commands are drained promptly.
	mgr := session.NewManager(ctx, reg, nil, 10*time.Millisecond)

	// Create more sessions than CPU cores to force parallel workers.
	const n = 20
	sessions := make([]*session.PlayerSession, n)
	for i := range n {
		s := session.New(uuid.New(), int64(i+1), testLogger())
		mgr.Add(s)
		sessions[i] = s
	}

	// Concurrent mutators.
	var mutWG sync.WaitGroup
	for i := range n {
		mutWG.Add(2)
		s := sessions[i]

		// Goroutine A: rapid commands.
		go func() {
			defer mutWG.Done()
			for j := 0; j < 10; j++ {
				_ = s.SubmitCommand(func(sess *session.PlayerSession) error {
					sess.Attr().GetFinal(1)
					return nil
				})
				time.Sleep(2 * time.Millisecond)
			}
		}()

		// Goroutine B: attach/detach conn + occasional close.
		go func() {
			defer mutWG.Done()
			for j := 0; j < 5; j++ {
				s.AttachConn(nil)
				time.Sleep(3 * time.Millisecond)
				s.DetachConn()
				time.Sleep(3 * time.Millisecond)
			}
		}()
	}

	mutWG.Wait()
}

// TestParallelTick_ReplaceSessionRace verifies that replacing a session
// under tick does not panic or corrupt the manager.
//
// Purpose: Simulate the "same user logs in from a new device" scenario:
// Manager.Add(s2) evicts s1 while parallelTick may still be processing s1.
//
// What it prevents: Panic or use-after-free when the old session is Closed
// mid-tick, or the manager map ending up with zero/duplicate entries.
func TestParallelTick_ReplaceSessionRace(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s1 := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s1)

	// Start a tick that will iterate over s1.
	var tickWG sync.WaitGroup
	tickWG.Add(1)
	go func() {
		defer tickWG.Done()
		for i := 0; i < 100; i++ {
			mgr.ManualTick(time.Now().Add(time.Duration(i) * time.Millisecond))
		}
	}()

	// Concurrently replace s1 with s2 many times.
	var replaceWG sync.WaitGroup
	replaceWG.Add(1)
	go func() {
		defer replaceWG.Done()
		for i := 0; i < 50; i++ {
			s2 := session.New(uuid.New(), 1, testLogger())
			mgr.Add(s2)
			time.Sleep(time.Millisecond)
			s1.Close()
			s1 = s2
		}
	}()

	replaceWG.Wait()
	tickWG.Wait()

	// Manager should still be consistent.
	if mgr.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", mgr.Count())
	}
}

// TestCloseWhileCommandsPending verifies that closing a session with
// pending commands does not panic, unblocks pending goroutines, and causes
// subsequent submissions to fail fast.
//
// Purpose: Ensure graceful degradation when a session is Closed while its
// command channel contains unconsumed commands. After Close, SubmitCommand
// must fail fast instead of blocking forever.
//
// What it prevents: Deadlock in WebSocket handlers where SubmitCommand
// hangs because the tick loop has stopped draining the channel.
func TestCloseWhileCommandsPending(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	// Fill command channel without consuming.
	const n = 10
	done := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			err := s.SubmitCommand(func(sess *session.PlayerSession) error {
				return nil
			})
			done <- err
		}()
	}
	time.Sleep(10 * time.Millisecond)

	// Close while commands may still be pending.
	s.Close()

	// All pending commands must be unblocked (not leaked).
	for i := 0; i < n; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("goroutine %d leaked after Close", i)
		}
	}

	// Subsequent command must fail fast.
	err := s.SubmitCommand(func(sess *session.PlayerSession) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error from SubmitCommand after Close, got nil")
	}

	if s.State() != session.StateClosed {
		t.Fatalf("expected StateClosed, got %v", s.State())
	}
}

// TestGraceExpireDuringTick verifies that grace timer expiry concurrent
// with a tick does not deadlock or panic.
//
// Purpose: Stress-test the interaction between graceMu + graceTimer callback
// and the tick loop. The callback may try to Flush/Close while runTick holds
// session.mu or iterates over the session.
//
// What it prevents: Deadlock between graceMu and session.mu, or panic from
// double-close of the done channel when grace expires during tick processing.
func TestGraceExpireDuringTick(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	// Simulate: disconnect starts grace, then tick runs while grace expires.
	s.DetachConn()
	s.StartGraceTimer(20 * time.Millisecond)

	// Run ticks concurrently with grace expiry.
	var tickWG sync.WaitGroup
	tickWG.Add(1)
	go func() {
		defer tickWG.Done()
		for i := 0; i < 20; i++ {
			mgr.ManualTick(time.Now().Add(time.Duration(i) * 10 * time.Millisecond))
			time.Sleep(5 * time.Millisecond)
		}
	}()

	tickWG.Wait()

	// After grace + some ticks, session may be closed. Either is acceptable.
	st := s.State()
	if st != session.StateGrace && st != session.StateClosed {
		t.Fatalf("unexpected state %v after grace+tick race", st)
	}
}
