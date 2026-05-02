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

func TestGracePeriodExpires(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	sess := session.New(uuid.New(), 1, testLogger())
	mgr.Add(sess)

	conn := &wsx.Conn{ID: uuid.New(), UserID: 1}
	sess.AttachConn(conn)
	if sess.State() != session.StateActive {
		t.Fatalf("expected StateActive, got %v", sess.State())
	}

	sess.DetachConn()
	sess.StartGraceTimer(100 * time.Millisecond)
	if sess.State() != session.StateGrace {
		t.Fatalf("expected StateGrace, got %v", sess.State())
	}

	time.Sleep(200 * time.Millisecond)
	if sess.State() != session.StateClosed {
		t.Fatalf("expected StateClosed, got %v", sess.State())
	}
}

func TestReconnectDuringGrace(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(attribute.Provider)
	mgr := session.NewManagerWithoutTick(reg, nil)

	sess := session.New(uuid.New(), 1, testLogger())
	mgr.Add(sess)

	conn1 := &wsx.Conn{ID: uuid.New(), UserID: 1}
	sess.AttachConn(conn1)
	sess.DetachConn()
	sess.StartGraceTimer(200 * time.Millisecond)

	time.Sleep(50 * time.Millisecond)
	if sess.State() != session.StateGrace {
		t.Fatal("should be in grace")
	}

	sess.StopGraceTimer()
	if sess.State() != session.StateActive {
		t.Fatalf("expected StateActive after reconnect, got %v", sess.State())
	}

	conn2 := &wsx.Conn{ID: uuid.New(), UserID: 1}
	sess.AttachConn(conn2)
	if !sess.HasConn() {
		t.Fatal("should have conn after reconnect")
	}

	// Ensure grace timer doesn't fire later
	time.Sleep(300 * time.Millisecond)
	if sess.State() != session.StateActive {
		t.Fatal("should still be active after grace timer would have fired")
	}
}
