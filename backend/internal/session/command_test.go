package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/equipment"
	"github.com/edrowsluo/new-mli/backend/internal/inventory"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/google/uuid"
)

func TestSubmitCommand(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.RunLoop(ctx, mgr, nil, 50*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	var executed bool
	err := s.SubmitCommand(func(s *session.PlayerSession) error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if !executed {
		t.Fatal("command was not executed")
	}
}

func TestHandleCommandEquip(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	reg.Register(inventory.Provider)
	reg.Register(equipment.Provider)
	mgr := session.NewManager(reg, nil)

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	sword := item.Item{ID: 35, State: 0}
	invSt.Add(sword, 1)

	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.RunLoop(ctx, mgr, nil, 50*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	err := s.SubmitCommand(func(sess *session.PlayerSession) error {
		return sess.Equip(context.Background(), sword, "main_hand")
	})
	if err != nil {
		t.Fatalf("equip command: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	s.RLock()
	_, ok := s.Equipment().Get("main_hand")
	s.RUnlock()
	if !ok {
		t.Fatal("main_hand should be equipped")
	}
}
