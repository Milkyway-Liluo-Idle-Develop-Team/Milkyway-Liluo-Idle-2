package session_test

import (
	"testing"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
	"github.com/google/uuid"
)

func TestBattleInstanceAttachDetach(t *testing.T) {
	s := session.New(uuid.New(), 1, testLogger())
	if s.Battle() != nil {
		t.Fatal("expected no battle initially")
	}
	b := session.NewBattleInstance(1)
	s.SetBattle(b)
	if s.Battle() != b {
		t.Fatal("battle should be attached")
	}
	s.SetBattle(nil)
	if s.Battle() != nil {
		t.Fatal("battle should be detached")
	}
}

func TestRLockSession(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	sess, ok := mgr.RLockSession(1)
	if !ok {
		t.Fatal("expected session")
	}
	r := attribute.Get()
	id, _ := r.AttrID("physical_power")
	_ = sess.Attr().GetFinal(id) // read under RLock
	mgr.RUnlockSession(sess)
}

func TestGraceExtendedDuringBattle(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s)

	// Set active battle
	b := session.NewBattleInstance(1)
	b.SetActive(true)
	s.SetBattle(b)

	// Attach then detach conn to enter grace
	conn := &wsx.Conn{ID: uuid.New(), UserID: 1}
	s.AttachConn(conn)
	s.DetachConn()

	s.StartGraceTimer(100 * time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	if s.State() != session.StateGrace {
		t.Fatalf("expected StateGrace during battle, got %v", s.State())
	}

	// Deactivate battle and re-start grace
	b.SetActive(false)
	s.StartGraceTimer(100 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	if s.State() != session.StateClosed {
		t.Fatalf("expected StateClosed after battle ended, got %v", s.State())
	}
}
