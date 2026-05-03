package session_test

import (
	"sync"
	"testing"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/google/uuid"
)

// TestCommandPanic_ReturnsError verifies that when a submitted command
// panics inside drainCommands, the error propagated back to SubmitCommand
// is non-nil. Before the fix, recover() in drainCommands logged the panic
// but left err == nil, so the caller thought the command succeeded.
//
// This is a real production bug: a panicking command silently appeared to
// succeed, potentially leaving the caller to act on corrupt/unset state.
func TestCommandPanic_ReturnsError(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	errCh := make(chan error, 1)
	go func() {
		err := s.SubmitCommand(func(sess *session.PlayerSession) error {
			panic("intentional command panic")
		})
		errCh <- err
	}()

	// Wait for the goroutine to enqueue before ticking.
	time.Sleep(20 * time.Millisecond)

	// Tick to trigger drainCommands; otherwise the command sits forever.
	mgr.ManualTick(time.Now())

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected non-nil error from panicking command, got nil; " +
				"drainCommands recover() is not propagating the panic as an error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SubmitCommand blocked forever — drainCommands did not unblock it")
	}
}

// TestClose_DrainsPendingCommands verifies that Close() unblocks goroutines
// blocked in SubmitCommand instead of leaking them forever.
//
// Before the fix: Close() set StateClosed but never drained or failed
// commands already sitting in the command channel. Those goroutines
// blocked on <-cmd.resp until the process exited.
func TestClose_DrainsPendingCommands(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

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

	// Give goroutines time to enqueue into the channel.
	time.Sleep(20 * time.Millisecond)

	// Close the session.
	s.Close()

	// All pending commands MUST return within a short timeout.
	// Before the fix they would block forever because Close() did not
	// drain the channel or close the done channel in a way that unblocks
	// the resp wait.
	for i := 0; i < n; i++ {
		select {
		case <-done:
			// ok — goroutine was unblocked
		case <-time.After(2 * time.Second):
			t.Fatalf("goroutine %d leaked: SubmitCommand still blocked after Close", i)
		}
	}
}

// TestSubmitCommand_RaceWithClose stresses the race between SubmitCommand
// and Close(). A goroutine may pass the StateClosed check, be preempted,
// then Close() runs, then the goroutine resumes and blocks forever on
// cmd.resp because no tick will ever drain it.
//
// Before the fix: occasional goroutine leaks under heavy contention.
func TestSubmitCommand_RaceWithClose(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = s.SubmitCommand(func(sess *session.PlayerSession) error {
					sess.Attr().GetFinal(1)
					return nil
				})
			}
		}()
	}

	// Close while submissions are still in flight.
	time.Sleep(5 * time.Millisecond)
	s.Close()

	// All goroutines must finish; if any deadlocked, wg.Wait() times out.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("deadlock: SubmitCommand goroutines leaked after Close")
	}
}
