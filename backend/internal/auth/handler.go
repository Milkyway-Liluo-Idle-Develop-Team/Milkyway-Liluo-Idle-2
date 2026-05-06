package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/apperror"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/config"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Handler exposes the auth HTTP routes.
type Handler struct {
	svc *Service
	cfg config.Auth
}

// NewHandler builds a Handler.
func NewHandler(svc *Service, cfg config.Auth) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

// Mount registers routes on r. Convention: routes that do not require auth
// are registered at the top level; routes that require auth are wrapped by
// the caller via Middleware.RequireAuth.
func (h *Handler) Mount(r chi.Router, mw *Middleware) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.register)
		r.Post("/login", h.login)
		r.Post("/logout", h.logout)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAuth)
			r.Get("/me", h.me)
			r.Post("/logout-all", h.logoutAll)
		})
	})
}

type registerReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.svc.Register(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, u)
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	User    User      `json:"user"`
	Session Session   `json:"session"`
	Expires time.Time `json:"expires_at"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	sess, err := h.svc.Login(r.Context(), req.Username, req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	// Look up the user (cheap) so we can return both in one response.
	u, _, err := h.svc.Authenticate(r.Context(), sess.Token)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.setSessionCookie(w, sess.Token, sess.ExpiresAt)
	httpx.JSON(w, http.StatusOK, loginResp{User: u, Session: sess, Expires: sess.ExpiresAt})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r, h.cfg.CookieName)
	if err := h.svc.Logout(r.Context(), token); err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.clearSessionCookie(w)
	httpx.NoContent(w)
}

func (h *Handler) logoutAll(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	if err := h.svc.LogoutAll(r.Context(), u.ID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.clearSessionCookie(w)
	httpx.NoContent(w)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, apperror.Unauthorized("not authenticated"))
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

// --- cookie helpers ---

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.CookieName,
		Value:    token,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: parseSameSite(h.cfg.CookieSameSite),
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.CookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: parseSameSite(h.cfg.CookieSameSite),
	})
}

func parseSameSite(s string) http.SameSite {
	switch strings.ToLower(s) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// extractToken reads the session token from the cookie or the Authorization
// header. Cookie wins so that browser sessions never compete with stale
// bearer tokens kept in URLs/scripts.
func extractToken(r *http.Request, cookieName string) string {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		return c.Value
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return ""
}

// clientIP returns the best-effort client IP from the request. Behind a
// reverse proxy with X-Forwarded-For, configure it to strip and rewrite to
// RemoteAddr; we deliberately do not blindly trust untrusted headers here.
func clientIP(r *http.Request) string {
	// host:port -> host
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}
