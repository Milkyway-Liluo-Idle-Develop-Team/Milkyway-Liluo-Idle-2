package server

import (
	"net/http"

	"github.com/edrowsluo/new-mli/backend/internal/auth"
	"github.com/edrowsluo/new-mli/backend/internal/config"
	"github.com/edrowsluo/new-mli/backend/internal/httpx"
	"github.com/edrowsluo/new-mli/backend/internal/logging"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

// wsHandler builds the http.Handler for the WebSocket upgrade endpoint.
// Auth happens here over plain HTTP before the upgrade — passing the userID
// to the hub via ServeOptions. OnConnect/OnDisconnect manage the
// PlayerSession lifecycle.
func wsHandler(hub *wsx.Hub, mw *auth.Middleware, httpCfg config.HTTP, wsCfg config.WS, sessMgr *session.Manager) http.Handler {
	originPatterns := allowedOriginPatterns(httpCfg)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID int64
		u, _, err := mw.AuthenticateRequest(r)
		if err != nil {
			if !wsCfg.AllowAnonymous {
				httpx.Error(w, r, err)
				return
			}
		} else {
			userID = u.ID
		}

		logger := logging.FromContext(r.Context())

		// Hub.Serve blocks until the connection ends. The HTTP request's
		// context is fine here — chi/server keeps it alive until we return.
		if err := hub.Serve(w, r, wsx.ServeOptions{
			UserID:         userID,
			OriginPatterns: originPatterns,
			OnConnect: func(c *wsx.Conn) {
				sess := session.New(c.ID, userID, logger)
				sessMgr.Add(sess)
				logger.Info("player session created",
					"conn", c.ID,
					"user_id", userID,
					"total", sessMgr.Count(),
				)
			},
			OnDisconnect: func(c *wsx.Conn) {
				if s, ok := sessMgr.LockSession(c.ID); ok {
					s.ClearRecorder()
					sessMgr.UnlockSession(s)
				}
				sessMgr.Remove(c.ID)
				logger.Info("player session removed",
					"conn", c.ID,
					"total", sessMgr.Count(),
				)
			},
		}); err != nil {
			logger.Debug("ws serve ended", "err", err)
		}
	})
}
