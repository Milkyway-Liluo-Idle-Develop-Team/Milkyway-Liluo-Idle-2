# Milkyway Liluo Idle 2

银河梨落放置 2 服务端与客户端。

## 项目结构

```
.
├── backend/        # Go 后端（HTTP + WebSocket）
│   ├── cmd/server/ # 服务器入口
│   └── internal/   # 业务模块
└── web-client/     # Vue 3 前端（Vite）
```

| 组件 | 技术栈 | 说明 |
|------|--------|------|
| 后端 | Go 1.25 + SQLite (pure-Go) | 单二进制部署，无需 CGO |
| 前端 | Vue 3 + Vite + TypeScript | 静态资源 |
| 通信 | HTTP REST + WebSocket | JSON / Protobuf 双协议 |
| 数据库 | SQLite (WAL 模式) | 自动迁移，零外部依赖 |

---

## 开发部署

### 1. 启动后端

```bash
cd backend

# 可选：创建本地环境配置（已有合理默认值）
cp .env.example .env

# 安装依赖并运行
go mod download
go run ./cmd/server
```

后端默认监听 `:8080`，首次运行会在工作目录创建 `data.db` 并自动执行数据库迁移。

### 2. 启动前端

```bash
cd web-client

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

前端开发服务器默认运行在 `http://localhost:5173`，已配置允许所有 host，Vite 会自动代理到后端（生产环境由反向代理处理跨域）。

### 3. 访问

打开浏览器访问 `http://localhost:5173`。

> 后端 `.env` 中的 `HTTP_CORS_ALLOWED_ORIGINS` 默认允许 `http://localhost:5173`，开发时无需额外配置。

---

## 生产部署

生产环境的目标：**单个后端二进制文件 + 前端静态文件**，可部署到任意 Linux/Windows 服务器或容器。

---

## 环境变量

后端通过环境变量读取配置，支持 `.env` 文件。完整变量见 `backend/.env.example`。

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `APP_ENV` | `development` | 运行环境：development / production |
| `HTTP_ADDR` | `:8080` | HTTP 监听地址 |
| `DATABASE_URL` | `file:data.db?_pragma=foreign_keys(1)` | SQLite 连接串 |
| `DATABASE_AUTO_MIGRATE` | `true` | 启动时是否自动执行数据库迁移 |
| `AUTH_COOKIE_SECURE` | `false` | Cookie Secure 标志，生产环境（HTTPS）应设为 `true` |
| `AUTH_COOKIE_SAMESITE` | `lax` | Cookie SameSite 策略 |
| `WS_CODEC` | `json` | WebSocket 编码：`json`（开发）或 `proto`（生产） |
| `WS_ALLOW_ANONYMOUS` | `false` | 是否允许匿名 WebSocket 连接 |

**生产环境最小配置示例：**

```bash
APP_ENV=production
HTTP_ADDR=:8080
HTTP_CORS_ALLOWED_ORIGINS=https://your-domain.com
DATABASE_URL=file:./data/app.db?_pragma=foreign_keys(1)
DATABASE_AUTO_MIGRATE=true
AUTH_COOKIE_SECURE=true
AUTH_COOKIE_SAMESITE=lax
WS_CODEC=proto
WS_ALLOW_ANONYMOUS=false
```

---

## 数据库迁移

迁移文件位于 `backend/internal/db/migrations/`，使用 [goose](https://github.com/pressly/goose) 管理，已嵌入二进制。

- **自动迁移**：设置 `DATABASE_AUTO_MIGRATE=true`，服务启动时自动执行
- **手动迁移**：如需独立控制，可在启动前运行
  ```bash
  cd backend
  goose -dir internal/db/migrations sqlite3 "data.db" up
  ```

---

## 常用命令

```bash
# 后端
cd backend
go run ./cmd/server              # 开发运行
go build -o server ./cmd/server  # 编译二进制
go test ./...                    # 运行测试
go vet ./...                     # 静态检查
sqlc generate                    # 重新生成数据库代码

# 前端
cd web-client
npm run dev                      # 开发服务器
npm run build                    # 生产构建
npm run type-check               # TypeScript 类型检查
```

---

## 生成 Protocol Buffers

若修改了 `backend/proto/*.proto`，需重新生成 Go 代码：

```bash
cd backend
buf generate
```

要求安装：
- [buf](https://buf.build/docs/installation)
- `protoc-gen-go`

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

---

