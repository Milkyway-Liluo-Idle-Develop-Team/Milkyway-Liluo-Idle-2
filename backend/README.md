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
| Game data           | `actions.json` embedded, loaded at startup    |

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
│   ├── gameconfig/         # actions.json loader, models, indexes
│   │   └── data/           # embedded copy of actions.json
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

## 游戏配置 (`internal/gameconfig`)

`actions.json`（物品、事件、需求、奖励等）在启动时通过
`gameconfig.Load()` 解析。该文件通过 `//go:embed` 嵌入二进制，因此服
务器对游戏数据没有外部文件依赖。

访问接口：
- `gameconfig.GetItem(id)` / `GetEvent(id)` — O(1) 查询
- `gameconfig.AllItems()` / `AllEvents()` — 完整列表，顺序确定
- `gameconfig.EventsBySkill(skillID)` / `EventsByMap(mapID)` — 预构建索引
- `gameconfig.ItemsByClassification(class)` — 按分类筛选

校验在启动时一次性完成：重复 ID、需求/奖励中悬空的物品/事件引用、缺失
必填字段等。校验失败是致命的，确保部署时立即发现坏数据。

### 稳定数字 ID 注册表 (`id_registry.json`)

内部代码和数据库使用数字 ID 而非字符串进行存储和计算。字符串 ID → 数
字 ID 的映射定义在 `internal/gameconfig/data/id_registry.json` 中，该文
件同样被嵌入并纳入版本控制。

**为什么需要它：** 如果数字 ID 按 `actions.json` 的出现顺序分配，那么调
整条目顺序或插入新条目都会导致所有 ID 偏移，从而破坏已有存档 / 数据库
记录。注册表保证：
- ID **稳定** — 一旦分配，永不改变。
- ID **单调递增** — 新条目获得 `max(已有) + 1`，已删除 ID 不会被复用，
  因此旧数据库行不会产生歧义。

#### `actions.json` 变更时的操作流程

1. 将更新后的 `actions.json` 拷贝到
   `internal/gameconfig/data/actions.json`。
2. 重新生成注册表，为新增的字符串 ID 分配数字 ID：
   ```bash
   go run ./cmd/genregistry
   ```
   该命令保留所有已有映射，仅追加新映射。
3. 重新编译服务器。启动时 `gameconfig.Load()` 会：
   - 解析 `actions.json`
   - 加载 `id_registry.json`
   - 校验 `actions.json` 中的**每一个**字符串 ID 在注册表中都有对应项
   - 构建查询索引
   如果有任何 ID 缺失，服务器会直接退出并提示你回到第 2 步。

#### 在代码中使用数字 ID

```go
// 字符串 → 数字（用于存储 / 数据库）
id, ok := gameconfig.StringToItemID("oak_logs")

// 数字 → 字符串（用于展示 / 日志）
s, ok := gameconfig.ItemIDToString(id)

// 直接用数字 ID 查询
it, ok := gameconfig.GetItemByID(id)
```

`EventID`、`SkillID`、`MapID`、`BattleSkillID` 都有类似的辅助
函数。每个类型 ID 都实现了 `.String()` 方法，便于打印调试。

#### 新增实体类型

如果 `actions.json` 后续引入了新的类别（例如 `monsters`），你需要：
1. 在 `internal/gameconfig/models.go` 的 `IDRegistry` 中新增字段。
2. 在 `internal/gameconfig/generate.go` 的 `GenerateRegistry` 中处理该类别。
3. 在 `internal/gameconfig/registry.go` 中补充 `checkDup` 和
   `checkConsistency` 校验逻辑。
4. 在 `internal/gameconfig/id.go` 和 `loader.go` 中新增类型 ID
   （如 `MonsterID`）和对应的访问函数。
5. 运行 `go run ./cmd/genregistry` 生成初始映射。

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
