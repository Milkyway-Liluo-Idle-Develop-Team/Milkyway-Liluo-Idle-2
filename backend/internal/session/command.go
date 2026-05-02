package session

import "github.com/edrowsluo/new-mli/backend/internal/apperror"

type command struct {
	fn   func(*PlayerSession) error
	resp chan error
}

func (s *PlayerSession) SubmitCommand(fn func(*PlayerSession) error) error {
	cmd := command{fn: fn, resp: make(chan error, 1)}
	select {
	case s.commandCh <- cmd:
		return <-cmd.resp
	default:
		return apperror.Unavailable("session command channel full")
	}
}
