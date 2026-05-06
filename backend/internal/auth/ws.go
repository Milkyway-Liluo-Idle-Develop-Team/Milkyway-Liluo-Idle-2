package auth

import (
	"context"
	"time"

	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/wsx"
)

// RegisterWS registers built-in auth-related WebSocket message handlers on
// the hub. Currently:
//
//   - "auth.whoami" →returns the user attached to the connection (or null)
//   - "ping"        →responds with "pong" carrying server time
//
// Registering more message types should follow this pattern: a small
// public function that takes the hub and the dependencies it needs.
func RegisterWS(hub *wsx.Hub, svc *Service) {
	hub.Handle("ping", func(ctx context.Context, c *wsx.Conn, in wsx.Inbound) error {
		return pong(c, in)
	})

	hub.Handle("auth.whoami", func(ctx context.Context, c *wsx.Conn, in wsx.Inbound) error {
		if c.UserID == 0 {
			c.Reply(in, &pb.WhoamiResponse{})
			return nil
		}
		c.Reply(in, &pb.WhoamiResponse{UserId: &c.UserID})
		return nil
	})
}

func pong(c *wsx.Conn, in wsx.Inbound) error {
	c.Reply(in, &pb.Pong{ServerTime: time.Now().UTC().Format(time.RFC3339Nano)})
	return nil
}
