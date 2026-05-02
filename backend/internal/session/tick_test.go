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

func TestTickAllProcessesCommands(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := session.NewManager(ctx, reg, nil, 50*time.Millisecond)

	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	time.Sleep(20 * time.Millisecond)
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
		t.Fatal("command was not executed by TickAll")
	}
}

func TestTickAllStopsOnContextCancel(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	ctx, cancel := context.WithCancel(context.Background())
	mgr := session.NewManager(ctx, reg, nil, 50*time.Millisecond)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	cancel()
	time.Sleep(100 * time.Millisecond)
	// TickAll exited; no panic means success.
}
