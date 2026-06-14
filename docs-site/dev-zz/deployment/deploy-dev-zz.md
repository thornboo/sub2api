# dev-zz 部署

`dev-zz` 二开版本已经发布到当前 fork 的镜像仓库，生产部署默认跟随最新镜像即可，不需要手动修改 `docker-compose.yml`。

推荐镜像：

```bash
docker pull thornboo/sub2api:latest
docker pull ghcr.io/thornboo/sub2api:latest
```

不要使用上游镜像 `weishaw/sub2api:latest`。该镜像来自上游项目，不包含 `dev-zz` 的二开修改。

固定版本镜像（例如 `thornboo/sub2api:1.1.1`）只建议用于验收、回滚或需要锁定版本的场景；日常更新应使用 `latest`，拉取最新镜像后重启服务。

## 推荐 Docker 部署脚本

Docker 部署准备脚本会从当前 fork 的 `dev-zz` 分支拉取 `docker-compose.local.yml` 和 `.env.example`，在当前目录生成 `docker-compose.yml` / `.env`，并使用 `thornboo/sub2api:latest` 作为默认镜像：

```bash
mkdir -p sub2api-deploy
cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/thornboo/sub2api/dev-zz/deploy/docker-deploy.sh | bash
docker compose up -d
```

已部署服务器更新到最新版本：

```bash
cd /path/to/sub2api-deploy
docker compose pull sub2api
docker compose up -d sub2api
```

关键点是进入实际保存 `docker-compose.yml` 和 `.env` 的部署目录执行，不需要重新下载或手动改 Compose 文件。

## 镜像配置

`deploy/.env` 中可以显式设置镜像源：

```dotenv
SUB2API_IMAGE=thornboo/sub2api:latest
```

如需使用 GitHub Container Registry：

```dotenv
SUB2API_IMAGE=ghcr.io/thornboo/sub2api:latest
```

如需临时回滚到指定版本：

```dotenv
SUB2API_IMAGE=thornboo/sub2api:1.1.1
```

回滚完成后再执行：

```bash
docker compose pull sub2api
docker compose up -d sub2api
```

## 本地源码构建

只有在开发验证、临时测试未发布代码，或需要绕过远程镜像仓库时，才需要从源码构建本地镜像。

在 `dev-zz` 分支的仓库根目录运行：

```bash
./deploy/deploy-dev-zz.sh
```

本地构建脚本会：

- 从仓库根目录 `Dockerfile` 构建本地镜像 `sub2api:dev-zz`
- 基于 `deploy/.env.example` 创建 `deploy/.env`
- 为新 `.env` 生成 `POSTGRES_PASSWORD`、`JWT_SECRET` 和 `TOTP_ENCRYPTION_KEY`
- 创建 `deploy/data`、`deploy/postgres_data` 和 `deploy/redis_data`
- 创建 `deploy/docker-compose.override.yml`，让 Compose 使用 `sub2api:dev-zz`
- 在启动已有栈前创建备份
- 使用 `deploy/docker-compose.local.yml` 和 override 文件启动服务

脚本不会默认覆盖已有 `deploy/.env`。拉取源码更新后重复执行脚本，会重新构建本地镜像并用原有数据和密钥重启服务。

## 常用选项

```bash
./deploy/deploy-dev-zz.sh --no-start
./deploy/deploy-dev-zz.sh --build-only
./deploy/deploy-dev-zz.sh --no-build
./deploy/deploy-dev-zz.sh --skip-backup
./deploy/deploy-dev-zz.sh --force-env
./deploy/deploy-dev-zz.sh --force-override
```

谨慎使用 `--force-env`。它会备份并重新生成 `deploy/.env`，如果新文件投入使用，可能导致登录会话或 TOTP 设置失效。

## 手动部署步骤

### 1. 克隆 dev-zz 分支

```bash
git clone -b dev-zz https://github.com/thornboo/sub2api.git
cd sub2api
```

已有仓库：

```bash
git fetch origin
git switch dev-zz
git pull --ff-only origin dev-zz
```

### 2. 构建本地镜像

```bash
docker build -t sub2api:dev-zz .
```

### 3. 准备部署目录

```bash
cd deploy
cp .env.example .env
mkdir -p data postgres_data redis_data
```

至少设置：

```dotenv
POSTGRES_PASSWORD=change_this_to_a_strong_password
JWT_SECRET=change_this_to_a_fixed_32_byte_hex_secret
TOTP_ENCRYPTION_KEY=change_this_to_a_fixed_32_byte_hex_secret
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=change_this_to_a_strong_admin_password
SERVER_PORT=8080
TZ=Asia/Shanghai
```

生成固定密钥：

```bash
openssl rand -hex 32
```

`JWT_SECRET` 和 `TOTP_ENCRYPTION_KEY` 需要跨重启保持稳定。

### 4. 覆盖镜像名

本地源码构建时，创建本地 override，让 Compose 使用刚构建的 `sub2api:dev-zz`：

```bash
cat > docker-compose.override.yml <<'EOF'
services:
  sub2api:
    image: sub2api:dev-zz
EOF
```

`deploy/docker-compose.override.yml` 是本地部署状态，已被仓库忽略。

### 5. 启动服务

```bash
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d
```

旧版 Compose：

```bash
docker-compose -f docker-compose.local.yml -f docker-compose.override.yml up -d
```

### 6. 更新本地构建服务

```bash
git fetch origin
git switch dev-zz
git pull --ff-only origin dev-zz
./deploy/deploy-dev-zz.sh
```

如需手动更新：

```bash
docker build -t sub2api:dev-zz .
cd deploy
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --no-deps --force-recreate sub2api
```
