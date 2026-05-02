package session_test

import (
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
	"github.com/google/uuid"
)

func TestSessionConnAttachDetach(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)
	sess := session.New(uuid.New(), 42, testLogger())
	mgr.Add(sess)

	if sess.HasConn() {
		t.Fatal("expected no conn initially")
	}

	conn := &wsx.Conn{ID: uuid.New(), UserID: 42}
	sess.AttachConn(conn)
	if !sess.HasConn() || sess.Conn() != conn {
		t.Fatal("conn should be attached")
	}

	sess.DetachConn()
	if sess.HasConn() {
		t.Fatal("conn should be detached")
	}
}

func TestManagerSingleOnline(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	s1 := session.New(uuid.New(), 1, testLogger())
	s2 := session.New(uuid.New(), 1, testLogger())
	mgr.Add(s1)
	mgr.Add(s2)

	if mgr.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", mgr.Count())
	}
	got, ok := mgr.GetByUser(1)
	if !ok || got != s2 {
		t.Fatal("latest session should win")
	}
}
