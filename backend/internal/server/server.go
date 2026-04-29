// Package server wires together the HTTP router, middleware stack, and the
// WebSocket hub. It owns the *http.Server lifetime; cmd/server/main.go is
// responsible for graceful shutdown.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/auth"
	"github.com/edrowsluo/new-mli/backend/internal/config"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	"github.com/edrowsluo/new-mli/backend/internal/httpx"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
	"github.com/go-chi/chi/v5"
)

// Deps bundles all the cross-cutting things server.New needs.
type Deps struct {
	Config config.Config
	DB     *db.DB
	Logger *slog.Logger
	Hub    *wsx.Hub

	AuthSvc   *auth.Service
	AuthMW    *auth.Middleware
	AuthH     *auth.Handler
	SessMgr   *session.Manager
}

// Server is the HTTP server with its background goroutines.
type Server struct {
	httpSrv *http.Server
	hub     *wsx.Hub
	cfg     config.Config
	logger  *slog.Logger
}

// New builds a Server. It does not start listening; call Start.
func New(d Deps) *Server {
	r := chi.NewRouter()

	r.Use(requestID)
	r.Use(requestLogger(d.Logger))
	r.Use(recoverer)
	r.Use(cors(d.Config.HTTP.CORSAllowedOrigins))

	// Public routes
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Versioned API mount point. All future modules hang off /api/v1.
	r.Route("/api/v1", func(r chi.Router) {
		d.AuthH.Mount(r, d.AuthMW)
	})

	// WebSocket endpoint. Auth is enforced inside the handler so we can
	// emit a clean JSON error instead of upgrading then closing.
	r.Handle("/ws", wsHandler(d.Hub, d.AuthMW, d.Config.HTTP, d.Config.WS, d.SessMgr))

	httpSrv := &http.Server{
		Addr:         d.Config.HTTP.Addr,
		Handler:      r,
		ReadTimeout:  d.Config.HTTP.ReadTimeout,
		// WriteTimeout = 0 because long-lived WebSocket connections are
		// served via this same Handler. We rely on the WS framework's own
		// per-message timeouts instead.
		WriteTimeout: 0,
		IdleTimeout:  d.Config.HTTP.IdleTimeout,
	}

	return &Server{
		httpSrv: httpSrv,
		hub:     d.Hub,
		cfg:     d.Config,
		logger:  d.Logger,
	}
}

// Start blocks until the server stops listening (or fails to start).
func (s *Server) Start() error {
	s.logger.Info("http listening", "addr", s.httpSrv.Addr)
	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the HTTP server and the WebSocket hub. It
// returns once both have stopped, or when the timeout elapses.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("http shutting down")
	if err := s.httpSrv.Shutdown(ctx); err != nil {
		s.logger.Error("http shutdown", "err", err)
	}
	// Close the hub after HTTP so no new upgrades happen mid-flight.
	timeout := s.cfg.HTTP.ShutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	s.hub.Close(timeout)
	return nil
}
