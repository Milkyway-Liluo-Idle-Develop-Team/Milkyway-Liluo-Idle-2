package session_test

import (
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
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
