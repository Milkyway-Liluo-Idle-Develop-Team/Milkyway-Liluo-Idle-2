package server

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/apperror"
	"github.com/edrowsluo/new-mli/backend/internal/config"
	"github.com/edrowsluo/new-mli/backend/internal/httpx"
	"github.com/edrowsluo/new-mli/backend/internal/logging"
	"github.com/google/uuid"
)

// requestIDHeader is the header we both read and emit. Reading lets a
// reverse proxy/edge correlate, emitting lets clients log it.
const requestIDHeader = "X-Request-ID"

// requestID generates an id when the upstream didn't supply one, attaches
// it to the response, and threads it through the request context.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := withRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder remembers the response status so we can log it after the
// handler runs.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Hijack exposes the underlying http.Hijacker so WebSocket upgrades work
// through the logging middleware.
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("http.ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

func (s *statusRecorder) status0() int {
	if s.status == 0 {
		return http.StatusOK
	}
	return s.status
}

// requestLogger logs one structured line per request and injects a
// per-request logger into the context.
func requestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rid, _ := RequestIDFromContext(r.Context())

			reqLog := base.With("request_id", rid)
			ctx := logging.WithLogger(r.Context(), reqLog)

			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r.WithContext(ctx))

			reqLog.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status0(),
				"dur_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// recoverer turns panics into 500s without crashing the server.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logging.FromContext(r.Context()).Error("panic", "value", rec, "path", r.URL.Path)
				httpx.Error(w, r, apperror.Internal("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// cors is a minimal, configurable CORS middleware. For richer needs,
// swap to github.com/go-chi/cors.
func cors(allowed []string) func(http.Handler) http.Handler {
	allowAll := len(allowed) == 1 && allowed[0] == "*"
	allowSet := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		allowSet[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else if _, ok := allowSet[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// allowedOriginPatterns translates the CORS allow list into the patterns
// expected by github.com/coder/websocket. "*" means "skip origin check".
func allowedOriginPatterns(cfg config.HTTP) []string {
	out := make([]string, 0, len(cfg.CORSAllowedOrigins))
	for _, o := range cfg.CORSAllowedOrigins {
		if o == "*" {
			return nil // websocket.AcceptOptions.InsecureSkipVerify path
		}
		// strip scheme; coder/websocket matches host[:port] patterns
		o = strings.TrimPrefix(o, "https://")
		o = strings.TrimPrefix(o, "http://")
		out = append(out, o)
	}
	return out
}
