package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/google/uuid"
)

func TestLoopProcessesCommands(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.RunLoop(ctx, mgr, nil, 50*time.Millisecond)

	var executed bool
	err := s.SubmitCommand(func(sess *session.PlayerSession) error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if !executed {
		t.Fatal("command not executed by loop")
	}
}

func TestLoopStopsOnContextCancel(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	ctx, cancel := context.WithCancel(context.Background())
	go s.RunLoop(ctx, mgr, nil, 50*time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)
	// If RunLoop doesn't stop, this test will leak a goroutine (detected by goleak if installed).
}
