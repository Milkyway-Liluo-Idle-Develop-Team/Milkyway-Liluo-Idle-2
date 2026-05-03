package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/apperror"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	pb "github.com/edrowsluo/new-mli/backend/pb"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

// TickResult holds a session that produced dirty state during a tick.
// Diff is no longer produced at the tick-round level; individual diffs
// are pushed immediately from inside runTick (command diff right after
// commands drain, settle diff right after settlement).
type TickResult struct {
	Sess *PlayerSession
}

func (m *Manager) TickAll(ctx context.Context, database *db.DB, tickInterval time.Duration) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			m.tickRound(ctx, database, now)
		}
	}
}

// tickRound executes a single round of tick+flush for all sessions.
// Diff pushing is now done inside runTick; tickRound only batches flushes.
func (m *Manager) tickRound(ctx context.Context, database *db.DB, now time.Time) {
	sessions := m.getAllSessions()
	if len(sessions) == 0 {
		return
	}

	results := m.parallelTick(sessions, now)

	if database != nil {
		if err := m.BatchFlush(ctx, database, results); err != nil {
			slog.Default().Error("batch flush failed", "err", err)
		}
	}
}

// ManualTick triggers one tick round immediately. Used by tests.
func (m *Manager) ManualTick(now time.Time) []TickResult {
	sessions := m.getAllSessions()
	if len(sessions) == 0 {
		return nil
	}
	return m.parallelTick(sessions, now)
}

func (m *Manager) getAllSessions() []*PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessions := make([]*PlayerSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

func (m *Manager) parallelTick(sessions []*PlayerSession, now time.Time) []TickResult {
	workerCount := m.workerCount
	if workerCount > len(sessions) {
		workerCount = len(sessions)
	}
	batchSize := (len(sessions) + workerCount - 1) / workerCount

	results := make([]TickResult, 0, len(sessions))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		start := i * batchSize
		end := start + batchSize
		if end > len(sessions) {
			end = len(sessions)
		}
		if start >= len(sessions) {
			break
		}

		wg.Add(1)
		go func(batch []*PlayerSession) {
			defer wg.Done()
			for _, s := range batch {
				if s.State() == StateClosed {
					continue
				}
				delta := now.Sub(s.lastTick).Seconds()
				s.lastTick = now
				s.elapsedAccum += delta

				if runTick(s, m, s.elapsedAccum) {
					mu.Lock()
					results = append(results, TickResult{Sess: s})
					mu.Unlock()
				}
				s.elapsedAccum = 0
			}
		}(sessions[start:end])
	}

	wg.Wait()
	return results
}

// runTick processes commands and settlement for one session.
// It returns true if the session has dirty state that needs flushing.
// Diffs are pushed immediately from inside this function:
//   1. After draining commands —command diff is built and pushed.
//   2. After settlement —settle diff is built and pushed only if it
//      contains actual rewards (not progress-only).
func runTick(s *PlayerSession, mgr *Manager, delta float64) (dirty bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Phase 1: drain commands (inside lock so changes are recorded).
	// Only create a recorder and build diff if there were actually commands.
	rec := mgr.NewRecorder()
	s.SetRecorder(rec)
	hasCmds := s.drainCommandsLocked()
	s.ClearRecorder()
	if hasCmds {
		cmdDiff, _ := mgr.Registry().BuildDiff(rec)
		if cmdDiff != nil && !isStateDiffEmpty(cmdDiff) {
			dirty = true
			if c := s.Conn(); c != nil {
				c.Send(wsx.Outbound{Type: "state.diff", Payload: cmdDiff})
			}
		}
	}

	// Phase 2: settle (only when there are active events).
	if s.ev == nil || !s.ev.HasActive() {
		return dirty
	}

	rec = mgr.NewRecorder()
	s.SetRecorder(rec)
	rec.PushNamespace("action_queue")

	settleDiff, err := func() (diff *pb.StateDiff, err error) {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("tick panic", "recover", r)
				err = apperror.Internal("tick panic")
			}
		}()

		s.ev.Settle(s, delta)

		if s.battle != nil && s.battle.Active() {
			// Placeholder: battle simulation runs here.
		}

		return mgr.Registry().BuildDiff(rec)
	}()

	s.ClearRecorder()

	if err != nil {
		s.logger.Error("tick failed", "err", err)
	}

	if settleDiff != nil && !isStateDiffEmpty(settleDiff) {
		dirty = true
		if !isProgressOnlyDiff(settleDiff) {
			if c := s.Conn(); c != nil {
				c.Send(wsx.Outbound{Type: "state.diff", Payload: settleDiff})
			}
		}
		// Progress-only diffs are intentionally NOT pushed.
		// The client can locally interpolate progress between ticks.
	}

	return dirty
}

// isProgressOnlyDiff returns true when the diff contains only event-queue
// progress updates with no actual rewards (items, XP, executions, etc.) and
// no queue structural changes (enqueue, consume, reorder).
//
// A "current" scope EventQueueDiff only updates the head entry's progress.
// A "full" scope EventQueueDiff means the queue structure changed and must be
// pushed to the client immediately.
func isProgressOnlyDiff(d *pb.StateDiff) bool {
	if d == nil || isStateDiffEmpty(d) {
		return true
	}
	// Any tangible reward or state change means it is NOT progress-only.
	if len(d.EventExecution) > 0 || len(d.Inventory) > 0 ||
		len(d.SkillXp) > 0 || len(d.Attribute) > 0 ||
		len(d.Bestiary) > 0 || len(d.Equipment) > 0 {
		return false
	}
	// If any EventQueue diff has "full" scope (enqueue, consume, reorder),
	// it is NOT progress-only.
	for _, qd := range d.EventQueue {
		if qd.Scope != "current" {
			return false
		}
	}
	// Only "current" scope EventQueue changes = pure progress update.
	return len(d.EventQueue) > 0
}

type dbFlusher interface {
	InTx(ctx context.Context, fn func(q *dbgen.Queries) error) error
}

func (m *Manager) BatchFlush(ctx context.Context, database dbFlusher, results []TickResult) error {
	return database.InTx(ctx, func(q *dbgen.Queries) error {
		for _, r := range results {
			if err := flushSession(ctx, q, r.Sess); err != nil {
				return err
			}
		}
		return nil
	})
}

func flushSession(ctx context.Context, q *dbgen.Queries, s *PlayerSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inv != nil {
		if err := s.inv.Flush(ctx, q); err != nil {
			return err
		}
	}
	if s.skill != nil {
		if err := s.skill.Flush(ctx, q); err != nil {
			return err
		}
	}
	if s.best != nil {
		if err := s.best.Flush(ctx, q); err != nil {
			return err
		}
	}
	if s.ev != nil {
		if err := s.ev.Flush(ctx, q); err != nil {
			return err
		}
	}
	if s.eq != nil {
		if err := s.eq.Flush(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *PlayerSession) drainCommands() {
	for {
		select {
		case cmd := <-s.commandCh:
			err := func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Error("command panic", "recover", r)
						err = apperror.Internal("command panic")
					}
				}()
				return cmd.fn(s)
			}()
			cmd.resp <- err
		default:
			return
		}
	}
}

// drainCommandsLocked is the lock-holding variant used by runTick.
// s.mu must be held by the caller.
// It returns true if at least one command was processed.
func (s *PlayerSession) drainCommandsLocked() bool {
	processed := false
	for {
		select {
		case cmd := <-s.commandCh:
			processed = true
			err := func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Error("command panic", "recover", r)
						err = apperror.Internal("command panic")
					}
				}()
				return cmd.fn(s)
			}()
			cmd.resp <- err
		default:
			return processed
		}
	}
}
