package session

import (
	"sync/atomic"
	"time"
)

type sessionState int32

const (
	StateActive sessionState = iota
	StateGrace
	StateClosing
	StateClosed
)

func (s *PlayerSession) State() sessionState {
	return sessionState(atomic.LoadInt32((*int32)(&s.state)))
}

func (s *PlayerSession) setState(st sessionState) {
	atomic.StoreInt32((*int32)(&s.state), int32(st))
}

func (s *PlayerSession) StartGraceTimer(duration time.Duration) {
	s.graceMu.Lock()
	defer s.graceMu.Unlock()

	if s.battleSession != nil && s.battleSession.Running {
		duration = 5 * time.Minute
	}

	if s.graceTimer != nil {
		s.graceTimer.Stop()
	}
	s.setState(StateGrace)
	s.graceTimer = time.AfterFunc(duration, func() {
		s.graceMu.Lock()
		defer s.graceMu.Unlock()
		if s.State() != StateGrace {
			return
		}
		if s.battleSession != nil && s.battleSession.Running {
			s.graceTimer.Reset(5 * time.Minute)
			return
		}
		s.setState(StateClosing)
		if s.onGraceExpire != nil {
			s.onGraceExpire()
		}
		s.setState(StateClosed)
	})
}

func (s *PlayerSession) StopGraceTimer() {
	s.graceMu.Lock()
	defer s.graceMu.Unlock()
	if s.graceTimer != nil {
		s.graceTimer.Stop()
		s.graceTimer = nil
	}
	s.setState(StateActive)
}

func (s *PlayerSession) SetOnGraceExpire(fn func()) {
	s.onGraceExpire = fn
}
