# Backend

Go + sqlc + SQLite backend with HTTP and WebSocket support.

This module provides only the foundation — user accounts, sessions, request
authentication, and a WebSocket framework for game messages. No game
business logic yet.

## Stack

| Concern             | Choice                                        |
|---------------------|-----------------------------------------------|
| HTTP router         | [chi v5](https://github.com/go-chi/chi)       |
| WebSocket           | [coder/websocket](https://github.com/coder/websocket) |
| Database            | SQLite via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go, no CGO) |
| SQL → Go            | [sqlc](https://sqlc.dev/) (in `internal/db/gen`) |
| Migrations          | [goose](https://github.com/pressly/goose) (embedded, auto-run) |
| Password hashing    | bcrypt (`golang.org/x/crypto/bcrypt`)         |
| Session token       | 256-bit random, SHA-256 in DB                 |
| Config              | env vars (`.env` supported)                   |
| Logger              | `log/slog`                                    |

The whole thing builds without CGO. A single `go run ./cmd/server` brings up
the server with the database file auto-created and migrations applied.

## Layout

```
backend/
├── cmd/server/             # main entry point
├── internal/
│   ├── apperror/           # transport-agnostic error type
│   ├── auth/               # user system: service, HTTP handlers, middleware, WS handlers
│   ├── config/             # env-driven config struct
│   ├── db/
│   │   ├── migrations/     # goose-style *.sql, embedded
│   │   ├── queries/        # sqlc query files
│   │   ├── gen/            # sqlc generated code (DO NOT EDIT)
│   │   └── db.go           # *sql.DB + tx helper + PRAGMA setup
│   ├── httpx/              # JSON envelope, error → HTTP, body decoding
│   ├── logging/            # slog setup + ctx propagation
│   ├── server/             # router, middleware, server lifecycle
│   └── wsx/                # WebSocket framework: hub, connection, routing
├── sqlc.yaml
├── .env.example
└── go.mod
```

## Quick start

```bash
cp .env.example .env       # optional; defaults work for local dev
go run ./cmd/server
```

The first run creates `data.db` (and `.db-wal`/`.db-shm` from WAL mode) in
the working directory. Migrations are embedded in the binary and run on
startup.

The server listens on `:8080` by default. Health check: `GET /healthz`.

### SQLite notes

- WAL mode is enabled at startup so reads don't block writes.
- `foreign_keys = ON` so `ON DELETE CASCADE` in the schema actually fires.
- `busy_timeout = 5000` to ride out brief writer contention.
- Max open connections defaults to 4 — small enough to avoid lock errors,
  large enough for typical request concurrency. Tune via `DATABASE_MAX_OPEN_CONNS`.
- For tests, set `DATABASE_URL=file::memory:?cache=shared` for an
  in-memory database.

## API

All HTTP responses use the same envelope:

```json
{ "data": ... }                     // success
{ "error": { "code": "...", "message": "..." } }   // failure
```

### Auth (HTTP, all under `/api/v1`)

| Method | Path           | Auth    | Body                                  | Returns                  |
|--------|----------------|---------|---------------------------------------|--------------------------|
| POST   | `/auth/register` | none  | `{ "username", "password" }`          | `{ "id", "username", ... }` |
| POST   | `/auth/login`    | none  | `{ "username", "password" }`          | `{ "user", "session" }`; sets `sid` cookie |
| POST   | `/auth/logout`   | any   | —                                     | 204; clears cookie       |
| POST   | `/auth/logout-all` | required | —                                | 204                      |
| GET    | `/auth/me`       | required | —                                  | current user             |

Authenticated requests may use either:
- the `sid` HttpOnly cookie set on login, or
- `Authorization: Bearer <token>` (the `session.token` returned by `/login`).

### WebSocket

`GET /ws` upgrades to a WebSocket. Auth follows the same rules as HTTP, plus
a `?token=` query param fallback for browser clients that can't set
headers (cookies are still preferred).

Wire format (JSON, both directions):

```jsonc
// client → server (request)
{ "id": "req-1", "type": "ping", "payload": {} }

// server → client (matching reply)
{ "id": "req-1", "type": "ping.ok", "payload": { "server_time": "..." } }

// server → client (error reply)
{ "id": "req-1", "type": "ping.err", "error": { "code": "...", "message": "..." } }

// server → client (push, no id)
{ "type": "some.event", "payload": { ... } }
```

Built-in WS message types:

| Type           | Purpose                                  |
|----------------|------------------------------------------|
| `ping`         | Returns `pong` (`ping.ok`) with server time |
| `auth.whoami`  | Returns the connection's user_id (or null) |

## Adding a new module

The auth module is the template. To add a feature module `foo`:

1. SQL: write migrations in `internal/db/migrations/` and queries in `internal/db/queries/foo.sql`.
2. Run `sqlc generate` to refresh `internal/db/gen/`.
3. Code: create `internal/foo/` with `service.go` (business logic), `handler.go` (HTTP), `ws.go` (WebSocket message handlers). Keep handlers thin — services own the rules.
4. Wire it in `cmd/server/main.go`: build the service, register HTTP routes inside `server.New`, and call `foo.RegisterWS(hub, ...)`.

A few rules of thumb the framework relies on:

- **Errors:** services return `*apperror.AppError`. Transports translate them — never write transport-specific status codes from a service.
- **Auth in HTTP:** wrap a chi `Group` with `authMW.RequireAuth`. Read the user via `auth.UserFromContext(r.Context())`.
- **Auth in WebSocket:** the connection's `UserID` is set at upgrade time. Anonymous connections have `UserID == 0` only when `WS_ALLOW_ANONYMOUS=true`.
- **Lifecycle:** background workers spawn from `main.run` so they share the root context and shut down cleanly.
- **Transactions:** for multi-statement writes, use `db.InTx(ctx, func(q *dbgen.Queries) error { ... })`.

## SQLite-specific gotchas to keep in mind as you add tables

- SQLite has no real `BIGSERIAL`/`UUID`/`TIMESTAMPTZ`. Use:
  - `INTEGER PRIMARY KEY AUTOINCREMENT` for surrogate ids
  - `TEXT` for UUIDs (string form) or `BLOB` (16 bytes)
  - `DATETIME` for timestamps; sqlc maps it to `time.Time`
- Use `CURRENT_TIMESTAMP` instead of `NOW()` and `datetime('now', '-7 days')`
  instead of `INTERVAL '7 days'`.
- Parameter placeholders are `?`, not `$N`.
- `RETURNING` is supported (SQLite ≥ 3.35), so sqlc `:one` queries with
  `RETURNING *` work as expected.

## Regenerating sqlc

```bash
sqlc generate
```

Re-run after adding/editing files in `internal/db/queries/` or `internal/db/migrations/`.

## Dev tools

- `go build ./...` — compile everything
- `go vet ./...` — static checks
- `go run ./cmd/server` — run locally
- `sqlc generate` — regenerate DB code
