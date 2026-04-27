package auth

import (
	"context"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

// RegisterWS registers built-in auth-related WebSocket message handlers on
// the hub. Currently:
//
//   - "auth.whoami" → returns the user attached to the connection (or null)
//   - "ping"        → responds with "pong" carrying server time
//
// Registering more message types should follow this pattern: a small
// public function that takes the hub and the dependencies it needs.
func RegisterWS(hub *wsx.Hub, svc *Service) {
	hub.Handle("ping", func(ctx context.Context, c *wsx.Conn, in wsx.Inbound) error {
		return pong(c, in)
	})

	hub.Handle("auth.whoami", func(ctx context.Context, c *wsx.Conn, in wsx.Inbound) error {
		if c.UserID == 0 {
			c.Reply(in, map[string]any{"user": nil})
			return nil
		}
		// We don't look up the user again — UserID was authenticated at
		// upgrade time. If a downstream consumer needs more, they can
		// call svc.LookupUser via a separate method.
		c.Reply(in, map[string]any{"user_id": c.UserID})
		return nil
	})
}

func pong(c *wsx.Conn, in wsx.Inbound) error {
	c.Reply(in, map[string]any{"server_time": time.Now().UTC().Format(time.RFC3339Nano)})
	return nil
}
