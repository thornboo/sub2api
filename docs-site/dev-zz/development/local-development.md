# 完全本地开发指南

这页记录 `dev-zz` 推荐的本地开发方式：前端、后端、PostgreSQL、Redis 都跑在本机，前端用 Vite 热更新，后端可以直接 `go run`，也可以用 Air 自动重启。

## 目标拓扑

```text
Browser
  -> Vite dev server: http://localhost:3000
      -> proxy /api, /v1, /setup
          -> Go backend: http://127.0.0.1:8080
              -> PostgreSQL: 127.0.0.1:5433
              -> Redis: 127.0.0.1:6380
```

这套方式和 `deploy/docker-compose.dev.yml` 不一样：Compose 开发文件会从源码构建并运行完整容器栈，但不会把本机 Go 源码挂进容器做后端热重启。日常开发更建议只用 Docker 跑数据库和 Redis，前后端代码直接在宿主机上运行。

## 前置要求

- Docker 或兼容的容器运行时
- pnpm
- Go 1.26.5，或通过仓库的 mise 配置启动 Go
- Node 20+ 用于本地前端和文档站构建；GitHub Actions 里额外用 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` 验证 JavaScript actions runtime 的 Node 24 兼容性。
- 可选：Air，用于后端文件变化后自动重启

## 1. 启动本地 PostgreSQL 和 Redis

PostgreSQL 绑定宿主机 `5433`，避免和本机已有数据库冲突。Redis 绑定宿主机 `6380`。

```bash
docker run -d \
  --name sub2api-postgres-dev \
  -e POSTGRES_USER=sub2api \
  -e POSTGRES_PASSWORD=sub2api_dev_password \
  -e POSTGRES_DB=sub2api \
  -p 127.0.0.1:5433:5432 \
  -v sub2api-postgres-dev:/var/lib/postgresql/data \
  postgres:18-alpine

docker run -d \
  --name sub2api-redis-dev \
  -p 127.0.0.1:6380:6379 \
  -v sub2api-redis-dev:/data \
  redis:8-alpine redis-server --save 60 1 --appendonly yes
```

检查服务状态：

```bash
docker exec sub2api-postgres-dev pg_isready -U sub2api -d sub2api
docker exec sub2api-redis-dev redis-cli ping
```

重复启动时如果容器已经存在，直接启动即可：

```bash
docker start sub2api-postgres-dev sub2api-redis-dev
```

## 2. 启动后端

后端会优先读取 `DATA_DIR`。建议把开发数据放在仓库根目录的 `.dev/backend-data`，`.dev/` 已被忽略，不会进入版本控制。

```bash
mkdir -p .dev/backend-data

DATA_DIR="$PWD/.dev/backend-data" \
AUTO_SETUP=true \
SERVER_HOST=127.0.0.1 \
SERVER_PORT=8080 \
SERVER_MODE=debug \
DATABASE_HOST=127.0.0.1 \
DATABASE_PORT=5433 \
DATABASE_USER=sub2api \
DATABASE_PASSWORD=sub2api_dev_password \
DATABASE_DBNAME=sub2api \
DATABASE_SSLMODE=disable \
REDIS_HOST=127.0.0.1 \
REDIS_PORT=6380 \
REDIS_PASSWORD= \
REDIS_DB=0 \
ADMIN_EMAIL=admin@sub2api.local \
ADMIN_PASSWORD=sub2api_admin_password \
JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
TOTP_ENCRYPTION_KEY=abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789 \
mise x -C backend -- go run ./cmd/server
```

如果不使用 mise，需要确保当前 shell 中的 `go` 是 1.26.5，然后改为：

```bash
cd backend
go run ./cmd/server
```

保留同一组环境变量即可，不需要额外配置。

## 3. 后端自动重启

可选安装 Air：

```bash
mise x -C backend -- go install github.com/air-verse/air@latest
export PATH="$(mise x -C backend -- go env GOPATH)/bin:$PATH"
```

从仓库根目录运行：

```bash
mkdir -p .dev/backend-data .dev/tmp

DATA_DIR="$PWD/.dev/backend-data" \
AUTO_SETUP=true \
SERVER_HOST=127.0.0.1 \
SERVER_PORT=8080 \
SERVER_MODE=debug \
DATABASE_HOST=127.0.0.1 \
DATABASE_PORT=5433 \
DATABASE_USER=sub2api \
DATABASE_PASSWORD=sub2api_dev_password \
DATABASE_DBNAME=sub2api \
DATABASE_SSLMODE=disable \
REDIS_HOST=127.0.0.1 \
REDIS_PORT=6380 \
REDIS_PASSWORD= \
REDIS_DB=0 \
ADMIN_EMAIL=admin@sub2api.local \
ADMIN_PASSWORD=sub2api_admin_password \
JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
TOTP_ENCRYPTION_KEY=abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789 \
air \
  --build.cmd "cd backend && go build -p 2 -o ../.dev/tmp/sub2api ./cmd/server" \
  --build.bin "./.dev/tmp/sub2api"
```

Air 只负责监听文件变化、重新编译并重启后端进程。数据库和 Redis 容器不需要跟着重启。

## 4. 启动前端

前端 Vite 默认把 `/api`、`/v1`、`/setup` 代理到 `http://localhost:8080`，也可以用 `VITE_DEV_PROXY_TARGET` 显式指定。

```bash
cd frontend
pnpm install
VITE_DEV_PROXY_TARGET=http://localhost:8080 VITE_DEV_PORT=3000 pnpm run dev
```

访问：

```text
http://localhost:3000
```

默认本地管理员账号来自后端启动环境变量：

```text
admin@sub2api.local
sub2api_admin_password
```

## 5. 日常开发命令

前端类型检查：

```bash
pnpm --dir frontend typecheck
```

前端 lint：

```bash
pnpm --dir frontend lint:check
```

后端局部测试示例：

```bash
mise x -C backend -- go test ./internal/server ./internal/handler ./internal/config
```

`dev-zz` 企业 Key、owner 用量分析和渠道报价相关的重点测试：

```bash
mise x -C backend -- go test ./internal/service ./internal/handler ./internal/server
pnpm --dir frontend test:run src/utils/__tests__/availableChannelsCatalog.spec.ts
```

文档站本地查看：

```bash
pnpm --dir docs-site install
pnpm --dir docs-site docs:dev
```

然后访问：

```text
http://localhost:5173
```

文档站构建检查：

```bash
pnpm --dir docs-site docs:build
```

更完整的验证组合见 [验证矩阵](../testing/verification-matrix.md)。

## 6. 重置本地数据

下面这些操作会删除本地开发数据库、Redis 数据和后端 `.dev` 数据目录。只有需要从零初始化时再执行：

```bash
docker rm -f sub2api-postgres-dev sub2api-redis-dev
docker volume rm sub2api-postgres-dev sub2api-redis-dev
rm -rf .dev/backend-data
```

重置后重新执行第 1 步和第 2 步。

## 常见问题

### 前端请求仍然打到旧后端

确认前端启动命令里设置了：

```bash
VITE_DEV_PROXY_TARGET=http://localhost:8080
```

并检查后端实际监听的是：

```bash
SERVER_HOST=127.0.0.1
SERVER_PORT=8080
```

### AUTO_SETUP 没有创建管理员

确认后端使用的是新的 `DATA_DIR`，并且数据库是空库。已有数据库不会重复初始化管理员。

### 端口被占用

可以换端口，但前后端配置要一起改：

- 后端端口：`SERVER_PORT`
- 前端代理目标：`VITE_DEV_PROXY_TARGET`
- 前端端口：`VITE_DEV_PORT`
- PostgreSQL 宿主机端口：`-p 127.0.0.1:<host-port>:5432`
- Redis 宿主机端口：`-p 127.0.0.1:<host-port>:6379`
