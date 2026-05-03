package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/apperror"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

type TickResult struct {
	Sess *PlayerSession
	Diff *pb.StateDiff
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

// tickRound executes a single round of tick+flush+push for all sessions.
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

	for _, r := range results {
		if r.Diff == nil || isStateDiffEmpty(r.Diff) {
			continue
		}
		if c := r.Sess.Conn(); c != nil {
			c.Send(wsx.Outbound{Type: "state.diff", Payload: r.Diff})
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

				diff, err := runTick(s, m, s.elapsedAccum)
				s.elapsedAccum = 0
				if err != nil {
					s.logger.Error("runTick failed", "err", err)
				}

				if diff != nil {
					mu.Lock()
					results = append(results, TickResult{Sess: s, Diff: diff})
					mu.Unlock()
				}
			}
		}(sessions[start:end])
	}

	wg.Wait()
	return results
}

func runTick(s *PlayerSession, mgr *Manager, delta float64) (*pb.StateDiff, error) {
	s.drainCommands()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.ClearRecorder()

	diff, err := func() (diff *pb.StateDiff, err error) {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("tick panic", "recover", r)
				err = apperror.Internal("tick panic")
			}
		}()

		rec := mgr.NewRecorder()
		s.SetRecorder(rec)
		rec.PushNamespace("action_queue")
		if s.ev != nil {
			s.ev.Settle(s, delta)
		}
		rec.PopNamespace()

		if s.battle != nil && s.battle.Active() {
			// Placeholder: battle simulation runs here.
		}

		return mgr.Registry().BuildDiff(rec)
	}()

	if err != nil {
		s.logger.Error("tick failed", "err", err)
	}
	return diff, err
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
