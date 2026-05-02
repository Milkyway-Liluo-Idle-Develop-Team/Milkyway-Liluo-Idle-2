package session

import (
	"context"
	"sync"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/db"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

type tickResult struct {
	sess *PlayerSession
	diff *pb.StateDiff
}

func (m *Manager) TickAll(ctx context.Context, database *db.DB, tickInterval time.Duration) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			sessions := m.getAllSessions()
			if len(sessions) == 0 {
				continue
			}

			results := m.parallelTick(sessions, now)

			if database != nil {
				m.batchFlush(ctx, database, results)
			}

			for _, r := range results {
				if r.diff == nil || isStateDiffEmpty(r.diff) {
					continue
				}
				if c := r.sess.Conn(); c != nil {
					c.Send(wsx.Outbound{Type: "state.diff", Payload: r.diff})
				}
			}
		}
	}
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

func (m *Manager) parallelTick(sessions []*PlayerSession, now time.Time) []tickResult {
	workerCount := m.workerCount
	if workerCount > len(sessions) {
		workerCount = len(sessions)
	}
	batchSize := (len(sessions) + workerCount - 1) / workerCount

	results := make([]tickResult, 0, len(sessions))
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

				diff := runTick(s, m, s.elapsedAccum)
				s.elapsedAccum = 0

				if diff != nil {
					mu.Lock()
					results = append(results, tickResult{sess: s, diff: diff})
					mu.Unlock()
				}
			}
		}(sessions[start:end])
	}

	wg.Wait()
	return results
}

func runTick(s *PlayerSession, mgr *Manager, delta float64) *pb.StateDiff {
	s.drainCommands()

	s.mu.Lock()
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

	s.ClearRecorder()

	diff, _ := mgr.Registry().BuildDiff(rec)
	s.mu.Unlock()

	return diff
}

func (m *Manager) batchFlush(ctx context.Context, database *db.DB, results []tickResult) {
	err := database.InTx(ctx, func(q *dbgen.Queries) error {
		for _, r := range results {
			if err := flushSession(ctx, q, r.sess); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// Best-effort: log once per tick. Production should wire to metrics.
	}
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
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Error("command panic", "recover", r)
					}
				}()
				err = cmd.fn(s)
			}()
			cmd.resp <- err
		default:
			return
		}
	}
}
