package server

import (
	"context"
	"net/http"

	"github.com/edrowsluo/new-mli/backend/internal/auth"
	"github.com/edrowsluo/new-mli/backend/internal/config"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	"github.com/edrowsluo/new-mli/backend/internal/httpx"
	"github.com/edrowsluo/new-mli/backend/internal/logging"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

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
				// Single-online: look up existing session
				if existing, ok := sessMgr.Get(userID); ok {
					// Kick old connection if present
					if old := existing.Conn(); old != nil {
						old.Close()
					}
					existing.AttachConn(c)
					existing.StopGraceTimer()
					logger.Info("player session reconnected",
						"conn", c.ID,
						"user_id", userID,
					)
					return
				}

				// No existing session — create new
				sess, err := sessMgr.CreateSession(r.Context(), c.ID, userID, database, logger)
				if err != nil {
					logger.Error("create session", "err", err)
					return
				}
				sess.AttachConn(c)
				sessMgr.Add(sess)

				// Grace expire callback: flush and remove
				sess.SetOnGraceExpire(func() {
					if err := sess.FlushAll(context.Background(), database); err != nil {
						logger.Error("flush on grace expire", "err", err)
					}
					sessMgr.Remove(userID)
					logger.Info("player session grace expired",
						"user_id", userID,
					)
				})

				logger.Info("player session created",
					"conn", c.ID,
					"user_id", userID,
					"total", sessMgr.Count(),
				)
			},
			OnDisconnect: func(c *wsx.Conn) {
				sess, ok := sessMgr.Get(userID)
				if !ok {
					return
				}

				// Only detach if this is the current conn
				if cur := sess.Conn(); cur != nil && cur.ID == c.ID {
					sess.DetachConn()
				}

				if !sess.HasConn() {
					sess.StartGraceTimer(wsCfg.SessionGracePeriod)
					logger.Info("player session detached, grace started",
						"conn", c.ID,
						"user_id", userID,
						"grace", wsCfg.SessionGracePeriod,
					)
				}
			},
		}); err != nil {
			logger.Debug("ws serve ended", "err", err)
		}
	})
}
