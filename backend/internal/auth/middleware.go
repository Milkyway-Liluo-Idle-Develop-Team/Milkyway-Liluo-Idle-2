package auth

import (
	"context"
	"net/http"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/apperror"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/config"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/httpx"
)

type ctxKey struct{ name string }

var (
	userKey    = ctxKey{"user"}
	sessionKey = ctxKey{"session"}
)

// UserFromContext returns the authenticated user attached to ctx by
// Middleware.RequireAuth (or LoadOptional). The bool reports presence.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey).(User)
	return u, ok
}

// SessionFromContext returns the current session, if attached.
func SessionFromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey).(Session)
	return s, ok
}

// withAuth attaches the user/session pair to a child context.
func withAuth(ctx context.Context, u User, s Session) context.Context {
	ctx = context.WithValue(ctx, userKey, u)
	ctx = context.WithValue(ctx, sessionKey, s)
	return ctx
}

// Middleware bundles the HTTP middlewares that depend on the auth Service.
type Middleware struct {
	svc *Service
	cfg config.Auth
}

// NewMiddleware constructs a Middleware.
func NewMiddleware(svc *Service, cfg config.Auth) *Middleware {
	return &Middleware{svc: svc, cfg: cfg}
}

// RequireAuth rejects requests without a valid session. On success, the
// downstream handler can call UserFromContext to retrieve the user.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r, m.cfg.CookieName)
		if token == "" {
			httpx.Error(w, r, apperror.Unauthorized("missing session token"))
			return
		}
		u, sess, err := m.svc.Authenticate(r.Context(), token)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(withAuth(r.Context(), u, sess)))
	})
}

// LoadOptional populates the user/session on the request if a valid token
// is present, but never rejects the request. Useful for endpoints that
// behave differently for guests vs. logged-in users.
func (m *Middleware) LoadOptional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r, m.cfg.CookieName)
		if token != "" {
			if u, sess, err := m.svc.Authenticate(r.Context(), token); err == nil {
				r = r.WithContext(withAuth(r.Context(), u, sess))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// AuthenticateRequest is a transport-agnostic helper used by the WebSocket
// upgrade endpoint. It reads the session token from the request (cookie,
// Authorization header, or ?token= query param as a last resort) and
// returns the user. Returns Unauthorized when missing/invalid.
//
// We accept a query-param token for WS only because some browser clients
// can't set custom headers when calling new WebSocket(...). Cookies are
// preferred.
func (m *Middleware) AuthenticateRequest(r *http.Request) (User, Session, error) {
	token := extractToken(r, m.cfg.CookieName)
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return User{}, Session{}, apperror.Unauthorized("missing session token")
	}
	return m.svc.Authenticate(r.Context(), token)
}
