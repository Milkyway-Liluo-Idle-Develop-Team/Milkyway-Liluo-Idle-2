package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/session"
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
	mgr := session.NewManager(ctx, reg, nil, 20*time.Millisecond)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	// Let at least one tick fire to confirm the goroutine is running.
	time.Sleep(30 * time.Millisecond)

	cancel()

	// Wait for the tick goroutine to observe cancellation and exit.
	time.Sleep(50 * time.Millisecond)

	// After TickAll exits, ManualTick must still work (no panic, no deadlock).
	mgr.ManualTick(time.Now())
}
