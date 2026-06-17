# dev-zz 部署

`dev-zz` 二开版本已经发布到当前 fork 的镜像仓库，生产部署默认跟随最新镜像即可，不需要手动修改 `docker-compose.yml`。

推荐镜像：

```bash
docker pull thornboo/sub2api:latest
docker pull ghcr.io/thornboo/sub2api:latest
```

不要使用上游镜像 `weishaw/sub2api:latest`。该镜像来自上游项目，不包含 `dev-zz` 的二开修改。

固定版本镜像（例如 `thornboo/sub2api:1.1.2`）只建议用于验收、回滚或需要锁定版本的场景；日常更新应使用 `latest`，拉取最新镜像后重启服务。

## 测试环境镜像

测试环境使用 `dev-zz-develop` 分支镜像，不使用 `latest`：

```dotenv
SUB2API_IMAGE=ghcr.io/thornboo/sub2api:dev-zz-develop
```

需要精确锁定一次验证时，使用带 short SHA 的镜像：

```dotenv
SUB2API_IMAGE=ghcr.io/thornboo/sub2api:dev-zz-develop-<shortsha>
```

`dev-zz-develop` 和 `dev-zz` push 只构建 GHCR 分支镜像，不更新正式 `latest`。正式 `latest` 仍由 `v*` tag / Release workflow 发布。

## 推荐 Docker 部署脚本

Docker 部署准备脚本会从当前 fork 的 `dev-zz` 分支拉取 `docker-compose.local.yml` 和 `.env.example`，在当前目录生成 `docker-compose.yml` / `.env`，并使用 `thornboo/sub2api:latest` 作为默认镜像：

```bash
mkdir -p sub2api-deploy
cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/thornboo/sub2api/dev-zz/deploy/docker-deploy.sh | bash
docker compose up -d
```

已部署服务器更新到最新版本时，先看下面的“已部署服务器日常更新”。如果是用这个脚本创建的独立部署目录，最短流程是：

```bash
cd /path/to/sub2api-deploy
./backup-dev-zz.sh
docker compose pull sub2api
docker compose up -d --no-deps --force-recreate sub2api
```

关键点是进入实际保存 `docker-compose.yml` 和 `.env` 的部署目录执行。

## 已部署服务器日常更新

如果服务器已经使用 `SUB2API_IMAGE=thornboo/sub2api:latest`，日常更新就是：

1. 备份数据。
2. 如果部署目录本身是 git 仓库，拉取最新 `dev-zz` 代码。
3. 拉取最新 Docker 镜像 `thornboo/sub2api:latest`。
4. 只重建应用容器。

`git pull` 和 `docker compose pull` 的作用不同：`git pull` 更新部署脚本、Compose 模板和文档；真正更新运行中应用的是 `docker compose pull sub2api` 拉到的新镜像。

### 仓库式部署目录

如果服务器上保留的是完整仓库，例如 `/opt/sub2api-zz`，推荐按这个流程更新：

```bash
cd /opt/sub2api-zz
git fetch origin
git switch dev-zz
git pull --ff-only origin dev-zz

cd deploy

if [ ! -x ./backup-dev-zz.sh ]; then
  chmod +x backup-dev-zz.sh
fi

./backup-dev-zz.sh
docker compose -f docker-compose.local.yml -f docker-compose.override.yml pull sub2api
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --no-deps --force-recreate sub2api

docker inspect --format '{{.Config.Image}}' sub2api
docker inspect --format '{{.State.Health.Status}}' sub2api
docker logs --tail=100 sub2api
```

### 独立部署目录

如果部署目录是通过 `docker-deploy.sh` 创建的，例如 `/path/to/sub2api-deploy`，这个目录通常不是 git 仓库，所以没有代码可拉。直接备份、拉镜像、重建应用容器即可。

更新前先备份。用量记录、用户余额、API Key 和账号配置都是消费证据，尤其是包含数据库迁移的版本，不应在没有备份的情况下更新。

如果部署目录是旧版本创建的，可能还没有 `backup-dev-zz.sh`。先下载一次；以后如果文档或备份逻辑更新，也可以重复执行这两行刷新脚本：

```bash
cd /path/to/sub2api-deploy

curl -sSL https://raw.githubusercontent.com/thornboo/sub2api/dev-zz/deploy/backup-dev-zz.sh -o backup-dev-zz.sh
chmod +x backup-dev-zz.sh
```

之后每次更新都先备份，再拉取发布镜像并重建应用容器：

```bash
cd /path/to/sub2api-deploy

./backup-dev-zz.sh
docker compose pull sub2api
docker compose up -d --no-deps --force-recreate sub2api

docker inspect --format '{{.Config.Image}}' sub2api
docker inspect --format '{{.State.Health.Status}}' sub2api
docker logs --tail=100 sub2api
```

如果这个独立目录里仍使用 `docker-compose.local.yml` + `docker-compose.override.yml`，使用同一组 compose 文件更新：

```bash
cd /path/to/sub2api-deploy

./backup-dev-zz.sh
docker compose -f docker-compose.local.yml -f docker-compose.override.yml pull sub2api
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --no-deps --force-recreate sub2api

docker inspect --format '{{.Config.Image}}' sub2api
docker inspect --format '{{.State.Health.Status}}' sub2api
docker logs --tail=100 sub2api
```

更新过程中不要执行 `docker compose down -v`，也不要删除 `data/`、`postgres_data/`、`redis_data/` 或 `.env`。

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
SUB2API_IMAGE=thornboo/sub2api:1.1.2
```

回滚完成后再执行：

```bash
docker compose pull sub2api
docker compose up -d sub2api
```

## 更新后验证

确认应用容器仍使用发布镜像，且服务健康：

```bash
docker ps -a
docker inspect --format '{{.Config.Image}}' sub2api
docker inspect --format '{{.State.Health.Status}}' sub2api
docker logs --tail=200 sub2api
```

如果本次版本包含数据库迁移，可检查迁移记录：

```bash
docker exec -it sub2api-postgres sh -c '
  : "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is not set in the PostgreSQL container}"
  export PGPASSWORD="$POSTGRES_PASSWORD"
  exec psql \
    -U "${POSTGRES_USER:-sub2api}" \
    -d "${POSTGRES_DB:-sub2api}" \
    -c "SELECT filename, applied_at FROM schema_migrations ORDER BY applied_at DESC LIMIT 10;"
'
```

如果 `docker image ls` 里还残留 `sub2api:dev-zz`，但 `docker ps` 中运行的 `sub2api` 容器已经是 `thornboo/sub2api:latest`，这个旧镜像只是未使用缓存，不再属于当前更新流程。
