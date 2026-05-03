package session

import "github.com/edrowsluo/new-mli/backend/internal/apperror"

type command struct {
	fn   func(*PlayerSession) error
	resp chan error
}

func (s *PlayerSession) SubmitCommand(fn func(*PlayerSession) error) error {
	if s.State() == StateClosed {
		return apperror.Unavailable("session closed")
	}
	cmd := command{fn: fn, resp: make(chan error, 1)}
	select {
	case s.commandCh <- cmd:
		select {
		case err := <-cmd.resp:
			return err
		case <-s.done:
			return apperror.Unavailable("session closed")
		}
	case <-s.done:
		return apperror.Unavailable("session closed")
	default:
		return apperror.Unavailable("session command channel full")
	}
}
