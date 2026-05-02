package session

import (
	"context"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/db"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

func (s *PlayerSession) RunLoop(ctx context.Context, mgr *Manager, database *db.DB, tickInterval time.Duration) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	var elapsedAccum float64
	lastTick := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			delta := now.Sub(lastTick).Seconds()
			lastTick = now
			elapsedAccum += delta

			s.drainCommands()

			s.mu.Lock()
			rec := mgr.NewRecorder()
			s.SetRecorder(rec)
			rec.PushNamespace("action_queue")
			if s.ev != nil {
				s.ev.Settle(s, elapsedAccum)
			}
			rec.PopNamespace()
			elapsedAccum = 0

			// Battle hook placeholder for Phase 5
			// if s.battle != nil {
			// 	// will be filled in Phase 5
			// }

			s.ClearRecorder()

			if database != nil {
				if err := s.FlushAll(ctx, database); err != nil {
					s.logger.Error("flush failed", "err", err)
				}
			}

			diff, err := mgr.Registry().BuildDiff(rec)
			if err != nil {
				s.logger.Error("build diff failed", "err", err)
			} else if !isStateDiffEmpty(diff) {
				if c := s.Conn(); c != nil {
					c.Send(wsx.Outbound{Type: "state.diff", Payload: diff})
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *PlayerSession) drainCommands() {
	for {
		select {
		case cmd := <-s.commandCh:
			err := cmd.fn(s)
			cmd.resp <- err
		default:
			return
		}
	}
}
