package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config holds all runtime configuration. Fields are loaded from environment
// variables (with optional .env support). Keep it a single struct so it is
// trivial to wire and easy to inject during tests.
type Config struct {
	Env  string `env:"APP_ENV"          envDefault:"development"`
	HTTP HTTP
	DB   DB
	Auth Auth
	WS   WS
	Log  Log
}

type HTTP struct {
	Addr            string        `env:"HTTP_ADDR"             envDefault:":8080"`
	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT"     envDefault:"15s"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT"    envDefault:"15s"`
	IdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT"     envDefault:"60s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`
	// CORSAllowedOrigins. Use "*" only in dev; in prod restrict to known origins.
	CORSAllowedOrigins []string `env:"HTTP_CORS_ALLOWED_ORIGINS" envDefault:"*" envSeparator:","`
}

type DB struct {
	// URL is the SQLite connection string. Examples:
	//   file:data.db?_pragma=foreign_keys(1)
	//   file::memory:?cache=shared        (in-memory, for tests)
	URL string `env:"DATABASE_URL" envDefault:"file:data.db?_pragma=foreign_keys(1)"`
	// MaxOpenConns caps total connections. SQLite is single-writer, so a
	// small number (e.g. 4) avoids locking errors under WAL.
	MaxOpenConns int32 `env:"DATABASE_MAX_OPEN_CONNS" envDefault:"4"`
	// MaxIdleConns caps idle connections kept around for reuse.
	MaxIdleConns int32 `env:"DATABASE_MAX_IDLE_CONNS" envDefault:"4"`
	// ConnMaxLifetime recycles long-lived connections. 0 disables.
	ConnMaxLifetime time.Duration `env:"DATABASE_CONN_MAX_LIFETIME" envDefault:"0"`
	// AutoMigrate runs embedded goose migrations on startup.
	AutoMigrate bool `env:"DATABASE_AUTO_MIGRATE" envDefault:"true"`
}

type Auth struct {
	// SessionTTL is how long a session stays valid after creation.
	SessionTTL time.Duration `env:"AUTH_SESSION_TTL" envDefault:"720h"` // 30 days
	// CookieName is the name of the session cookie.
	CookieName string `env:"AUTH_COOKIE_NAME" envDefault:"sid"`
	// CookieDomain restricts the cookie to a domain. Leave empty to use request host.
	CookieDomain string `env:"AUTH_COOKIE_DOMAIN" envDefault:""`
	// CookieSecure controls the Secure flag (HTTPS only). Defaults true; set false for local http dev.
	CookieSecure bool `env:"AUTH_COOKIE_SECURE" envDefault:"false"`
	// CookieSameSite: "lax" | "strict" | "none". When "none", Secure must be true.
	CookieSameSite string `env:"AUTH_COOKIE_SAMESITE" envDefault:"lax"`
	// BcryptCost. 10-12 for prod, 4 for tests.
	BcryptCost int `env:"AUTH_BCRYPT_COST" envDefault:"12"`
}

type WS struct {
	// MaxMessageSize is the max accepted size of a single client message.
	MaxMessageSize int64 `env:"WS_MAX_MESSAGE_SIZE" envDefault:"65536"` // 64 KiB
	// WriteTimeout per outbound message.
	WriteTimeout time.Duration `env:"WS_WRITE_TIMEOUT" envDefault:"10s"`
	// PingInterval is the keepalive ping interval. Should be < server idle timeout.
	PingInterval time.Duration `env:"WS_PING_INTERVAL" envDefault:"30s"`
	// SendBuffer is the per-client outgoing buffer. Drops slow clients past this.
	SendBuffer int `env:"WS_SEND_BUFFER" envDefault:"64"`
	// AllowAnonymous allows unauthenticated WebSocket connections.
	AllowAnonymous bool `env:"WS_ALLOW_ANONYMOUS" envDefault:"false"`
	// Codec: "proto" (binary, prod default) or "json" (protojson text, for dev).
	Codec string `env:"WS_CODEC" envDefault:"json"`
	// SessionGracePeriod is how long a session stays in memory after disconnect.
	SessionGracePeriod time.Duration `env:"WS_SESSION_GRACE_PERIOD" envDefault:"30s"`
	// GameLoopTick is the main session ticker interval.
	GameLoopTick time.Duration `env:"WS_GAME_LOOP_TICK" envDefault:"50ms"`
}

type Log struct {
	// Level: "debug" | "info" | "warn" | "error"
	Level string `env:"LOG_LEVEL" envDefault:"info"`
	// Format: "json" | "text"
	Format string `env:"LOG_FORMAT" envDefault:"text"`
	// File is the path to a log file. When non-empty, logs are tee'd to both
	// stdout and the file (like Minecraft's latest.log). The file is truncated
	// on each startup.
	File string `env:"LOG_FILE" envDefault:""`
}

// Load reads .env (if present) then parses environment variables into a Config.
// Missing .env is not an error; missing required env vars are.
func Load() (Config, error) {
	_ = godotenv.Load() // best-effort

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}

// IsDev reports whether we're running in a development environment.
func (c Config) IsDev() bool { return c.Env == "development" || c.Env == "dev" }
