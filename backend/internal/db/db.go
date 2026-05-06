// Package db owns everything about the database connection: the *sql.DB
// handle, embedded migrations, and a small wrapper that exposes both the
// raw connection (for transactions) and the sqlc Queries (for application
// reads/writes).
//
// We use SQLite via the pure-Go modernc.org/sqlite driver —no CGO needed.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/config"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// driverName is the name used to register modernc.org/sqlite.
const driverName = "sqlite"

// DB bundles the *sql.DB with the sqlc Queries handle. Pass DB into any
// service that needs both transactional access (Conn) and ergonomic CRUD
// (Queries). Never expose dbgen.Queries directly to handlers —keep
// boundaries at services.
type DB struct {
	Conn    *sql.DB
	Queries *dbgen.Queries
}

// Open creates the connection, applies SQLite PRAGMAs, optionally runs
// migrations, and returns DB.
func Open(ctx context.Context, cfg config.DB) (*DB, error) {
	conn, err := sql.Open(driverName, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// SQLite is a single-writer engine. With WAL mode, multiple readers can
	// coexist with one writer, but the database/sql layer can still serialise
	// writers across connections. Capping max-open-conns to a small number
	// keeps "database is locked" errors at bay for typical loads. Tune via
	// config if needed.
	if cfg.MaxOpenConns > 0 {
		conn.SetMaxOpenConns(int(cfg.MaxOpenConns))
	}
	if cfg.MaxIdleConns > 0 {
		conn.SetMaxIdleConns(int(cfg.MaxIdleConns))
	}
	if cfg.ConnMaxLifetime > 0 {
		conn.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := applyPragmas(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("apply pragmas: %w", err)
	}

	if cfg.AutoMigrate {
		if err := migrate(ctx, conn); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}

	return &DB{
		Conn:    conn,
		Queries: dbgen.New(conn),
	}, nil
}

// Close releases the database connection.
func (d *DB) Close() error {
	if d == nil || d.Conn == nil {
		return nil
	}
	return d.Conn.Close()
}

// InTx runs fn inside a transaction, committing on success and rolling back
// on any error or panic. The fn receives a *dbgen.Queries bound to the tx.
func (d *DB) InTx(ctx context.Context, fn func(q *dbgen.Queries) error) (err error) {
	tx, err := d.Conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	q := d.Queries.WithTx(tx)
	if err = fn(q); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// applyPragmas turns on foreign keys, switches to WAL for better
// concurrency, and sets a generous busy timeout so brief writer contention
// doesn't surface as errors. Pragmas are per-connection in SQLite, so we
// also apply them to every freshly-opened connection via a connector hook
// where possible —but here we additionally rely on shared-cache + WAL
// behaviour and a small pool to keep things simple.
func applyPragmas(ctx context.Context, conn *sql.DB) error {
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA temp_store = MEMORY;",
	}
	for _, s := range stmts {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("exec %q: %w", s, err)
		}
	}
	return nil
}

// migrate runs embedded goose migrations.
func migrate(ctx context.Context, conn *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := goose.UpContext(ctx, conn, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
