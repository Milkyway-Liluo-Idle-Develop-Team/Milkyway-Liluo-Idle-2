package server

import (
	"net/http"

	"github.com/edrowsluo/new-mli/backend/internal/auth"
	"github.com/edrowsluo/new-mli/backend/internal/config"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	"github.com/edrowsluo/new-mli/backend/internal/httpx"
	"github.com/edrowsluo/new-mli/backend/internal/logging"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

// wsHandler builds the http.Handler for the WebSocket upgrade endpoint.
func wsHandler(hub *wsx.Hub, mw *auth.Middleware, httpCfg config.HTTP, wsCfg config.WS, sessMgr *session.Manager, database *db.DB) http.Handler {
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

		if err := hub.Serve(w, r, wsx.ServeOptions{
			UserID:         userID,
			OriginPatterns: originPatterns,
			OnConnect: func(c *wsx.Conn) {
				sess, err := sessMgr.CreateSession(r.Context(), c.ID, userID, database, logger)
				if err != nil {
					logger.Error("create session", "err", err)
					return
				}
				sessMgr.Add(sess)
				sessMgr.StartLoop(sess, c)
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
