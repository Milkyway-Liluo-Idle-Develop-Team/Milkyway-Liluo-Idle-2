package server

import (
	"net/http"

	"github.com/edrowsluo/new-mli/backend/internal/auth"
	"github.com/edrowsluo/new-mli/backend/internal/config"
	"github.com/edrowsluo/new-mli/backend/internal/httpx"
	"github.com/edrowsluo/new-mli/backend/internal/logging"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

// wsHandler builds the http.Handler for the WebSocket upgrade endpoint.
// Auth happens here over plain HTTP before the upgrade — passing the userID
// to the hub via ServeOptions.
func wsHandler(hub *wsx.Hub, mw *auth.Middleware, httpCfg config.HTTP, wsCfg config.WS) http.Handler {
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

		// Hub.Serve blocks until the connection ends. The HTTP request's
		// context is fine here — chi/server keeps it alive until we return.
		if err := hub.Serve(w, r, wsx.ServeOptions{
			UserID:         userID,
			OriginPatterns: originPatterns,
		}); err != nil {
			logging.FromContext(r.Context()).Debug("ws serve ended", "err", err)
		}
	})
}
