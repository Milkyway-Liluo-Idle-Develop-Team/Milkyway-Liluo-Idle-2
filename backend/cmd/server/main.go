// Command server is the application entry point. It loads config, opens
// the database, wires modules, and runs the HTTP server. Background
// goroutines (e.g. session cleanup) are owned here so we can shut them
// down deterministically.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/auth"
	"github.com/edrowsluo/new-mli/backend/internal/bestiary"
	"github.com/edrowsluo/new-mli/backend/internal/config"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	"github.com/edrowsluo/new-mli/backend/internal/equipment"
	"github.com/edrowsluo/new-mli/backend/internal/event"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/inventory"
	"github.com/edrowsluo/new-mli/backend/internal/logging"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"github.com/edrowsluo/new-mli/backend/internal/server"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/skill"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
)

func main() {
	if err := run(); err != nil {
		// Already-logged errors return as a sentinel; otherwise print.
		if !errors.Is(err, errExitLogged) {
			slog.Error("fatal", "err", err)
		}
		os.Exit(1)
	}
}

var errExitLogged = errors.New("exit logged")

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.New(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	slog.SetDefault(logger)
	logger.Info("starting", "env", cfg.Env)

	// --- Game config ---
	if err := gameconfig.Load(); err != nil {
		logger.Error("load game config", "err", err)
		return errExitLogged
	}
	logger.Info("game config loaded",
		"items", gameconfig.ItemCount(),
		"events", gameconfig.EventCount(),
		"skills", gameconfig.SkillCount(),
		"maps", gameconfig.MapCount(),
		"battle_skills", gameconfig.BattleSkillCount(),
	)

	// --- Attribute system ---
	if err := attribute.Load(); err != nil {
		logger.Error("load attribute system", "err", err)
		return errExitLogged
	}
	logger.Info("attribute system loaded",
		"attrs", attribute.Get().Count(),
	)

	// --- Data record registry ---
	recordReg := record.NewRegistry()
	recordReg.Register(attribute.Provider)
	recordReg.Register(inventory.Provider)
	recordReg.Register(skill.Provider)
	recordReg.Register(bestiary.Provider)
	recordReg.Register(event.ExecProvider)
	recordReg.Register(event.QueueProvider)
	recordReg.Register(equipment.Provider)

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- DB ---
	openCtx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	database, err := db.Open(openCtx, cfg.DB)
	cancel()
	if err != nil {
		logger.Error("open db", "err", err)
		return errExitLogged
	}
	defer database.Close()
	logger.Info("db ready")

	// --- WS hub ---
	hub := wsx.NewHub(cfg.WS)

	// --- Auth module ---
	authSvc := auth.NewService(database, cfg.Auth)
	authMW := auth.NewMiddleware(authSvc, cfg.Auth)
	authH := auth.NewHandler(authSvc, cfg.Auth)
	auth.RegisterWS(hub, authSvc)

	// --- Session manager ---
	sessMgr := session.NewManager(rootCtx, recordReg, database, cfg.WS.GameLoopTick)

	// --- WS game handlers ---
	registerGameHandlers(hub, sessMgr)

	// --- Background workers ---
	go runSessionCleanup(rootCtx, logger, authSvc)

	// --- HTTP server ---
	srv := server.New(server.Deps{
		Config:  cfg,
		DB:      database,
		Logger:  logger,
		Hub:     hub,
		AuthSvc: authSvc,
		AuthMW:  authMW,
		AuthH:   authH,
		SessMgr: sessMgr,
	})

	serverErrCh := make(chan error, 1)
	go func() { serverErrCh <- srv.Start() }()

	select {
	case <-rootCtx.Done():
		logger.Info("signal received, shutting down")
	case err := <-serverErrCh:
		if err != nil {
			logger.Error("http server", "err", err)
			return errExitLogged
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("bye")
	return nil
}

// runSessionCleanup periodically removes expired/old-revoked sessions.
// Failures are logged and the loop continues —the row volume is bounded
// and the next tick will retry.
func runSessionCleanup(ctx context.Context, log *slog.Logger, svc *auth.Service) {
	const interval = time.Hour
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := svc.CleanupExpired(ctx); err != nil {
				log.Warn("session cleanup failed", "err", err)
			}
		}
	}
}
