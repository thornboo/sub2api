# dev-zz 部署

`dev-zz` 分支必须从当前源码构建部署，不应直接拉取上游公开镜像。

不要使用：

```bash
docker pull weishaw/sub2api:latest
```

该镜像来自上游项目，不包含 dev-zz 的二开修改。

## 推荐脚本

在 `dev-zz` 分支的仓库根目录运行：

```bash
./deploy/deploy-dev-zz.sh
```

脚本会：

- 从仓库根目录 `Dockerfile` 构建本地镜像 `sub2api:dev-zz`
- 基于 `deploy/.env.example` 创建 `deploy/.env`
- 为新 `.env` 生成 `POSTGRES_PASSWORD`、`JWT_SECRET` 和 `TOTP_ENCRYPTION_KEY`
- 创建 `deploy/data`、`deploy/postgres_data` 和 `deploy/redis_data`
- 创建 `deploy/docker-compose.override.yml`，让 Compose 使用 `sub2api:dev-zz`
- 在启动已有栈前创建备份
- 使用 `deploy/docker-compose.local.yml` 和 override 文件启动服务

脚本不会默认覆盖已有 `deploy/.env`。拉取更新后重复执行脚本，会重新构建本地镜像并用原有数据和密钥重启服务。

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

`deploy/docker-compose.local.yml` 默认使用上游镜像。创建本地 override：

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

### 6. 更新服务

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
