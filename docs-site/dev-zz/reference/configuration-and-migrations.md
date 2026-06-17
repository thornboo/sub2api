# 配置与迁移索引

本文记录 dev-zz 相对上游新增或语义明显不同的配置、迁移、镜像和 CI 约束。

## 运行时版本

| 项 | 当前口径 |
| --- | --- |
| Go | `backend/go.mod` 声明 `go 1.26.4`，CI 会校验 `go1.26.4` |
| 前端构建 Node | GitHub Actions 仍使用 `node-version: '20'` |
| GitHub JavaScript actions runtime | CI 设置 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true`，用于验证 actions runtime 兼容性 |
| pnpm | 前端和 docs-site 独立 lockfile，CI 前端使用 pnpm 9 |
| docs-site | VitePress 1.x，命令见 `docs-site/package.json` |

Node 24 runtime 变量只验证 GitHub action 执行环境，不等价于项目构建 Node 升级。升级前端构建 Node 版本前，需要单独验证依赖兼容。

## API Key 相关配置

| 配置 | 默认值 | 说明 |
| --- | ---: | --- |
| `api_key_batch_create_max_count` | 200 | 批量创建 Key 默认上限 |
| 服务端硬上限 | 500 | 批量创建、filtered 批量更新/删除的最大目标数量 |
| 标签数量 | 20 | 单把 Key 最多标签数 |
| 标签长度 | 40 | 单个标签最多字符数 |
| 标签候选 | 500 | `/api/v1/keys/tags` 最多返回数量 |

标签规范化在服务层完成：trim、小写、去重、过滤空字符串。仓储层在写入前还会把 `nil` tags 归一成空数组，避免 PostgreSQL `jsonb` 列写入 JSON `null`。

## 数据库迁移

| 文件 | 作用 |
| --- | --- |
| `backend/migrations/151_add_api_key_tags.sql` | 给 `api_keys` 增加 `tags jsonb NOT NULL DEFAULT '[]'::jsonb`，补齐空值，并添加 `api_keys_tags_json_array` 约束 |
| `backend/migrations/152_add_api_key_tags_index_notx.sql` | 以 `_notx` 方式创建 `idx_api_keys_tags_gin` 部分 GIN 索引 |

`152` 使用 `CREATE INDEX CONCURRENTLY`，不能放进普通事务迁移。后续合并上游迁移时，需保留 `_notx` 约定，避免长事务锁表。

## 数据保留默认值

dev-zz 默认关闭自动数据清理，保留管理员显式清理入口。

| 配置 | 默认值 | 说明 |
| --- | --- | --- |
| `DASHBOARD_AGGREGATION_RETENTION_AUTO_CLEANUP_ENABLED` | `false` | usage logs、billing dedup、hourly/daily aggregation 不自动删 |
| `DASHBOARD_AGGREGATION_RETENTION_USAGE_LOGS_DAYS` | `90` | 仅在自动清理开启时生效 |
| `DASHBOARD_AGGREGATION_RETENTION_USAGE_BILLING_DEDUP_DAYS` | `365` | 必须大于等于 usage logs 保留天数 |
| `DASHBOARD_AGGREGATION_RETENTION_HOURLY_DAYS` | `180` | 仅在自动清理开启时生效 |
| `DASHBOARD_AGGREGATION_RETENTION_DAILY_DAYS` | `730` | 仅在自动清理开启时生效 |
| `OPS_CLEANUP_AUTO_CLEANUP_ENABLED` | `false` | 运维日志、指标和监控历史不自动删 |
| `OPS_CLEANUP_ERROR_LOG_RETENTION_DAYS` | `30` | 仅在自动清理开启时生效 |
| `OPS_CLEANUP_MINUTE_METRICS_RETENTION_DAYS` | `30` | 仅在自动清理开启时生效 |
| `OPS_CLEANUP_HOURLY_METRICS_RETENTION_DAYS` | `30` | 仅在自动清理开启时生效 |

如果自动清理关闭，保留天数允许为 0；如果开启，则必须是正数并通过配置校验。

## 部署镜像

| 项 | 当前值 |
| --- | --- |
| Docker Hub | `thornboo/sub2api:latest` |
| GitHub Container Registry | `ghcr.io/thornboo/sub2api:latest` |
| 固定版本示例 | `thornboo/sub2api:1.1.2` |
| 上游镜像 | `weishaw/sub2api:latest`，不包含 dev-zz 二开 |

`deploy/.env.example` 和各 compose 文件默认使用 `SUB2API_IMAGE=thornboo/sub2api:latest`。本地源码构建镜像 `sub2api:dev-zz` 只作为开发验证、应急和未发布代码测试路径。

## 部署文件来源

`deploy/docker-deploy.sh` 默认：

```bash
GITHUB_REPO=thornboo/sub2api
GITHUB_BRANCH=dev-zz
GITHUB_RAW_URL=https://raw.githubusercontent.com/thornboo/sub2api/dev-zz/deploy
```

脚本会下载 `docker-compose.local.yml` 为部署目录中的 `docker-compose.yml`，并下载 `.env.example` 生成 `.env`。如果维护 fork，需要同时检查脚本默认仓库、镜像默认值和 README/部署文档是否一致。

## CI 与发布

| 文件 | dev-zz 差异 |
| --- | --- |
| `.github/workflows/backend-ci.yml` | actions runtime 走 Node 24 验证；Go 版本校验 1.26.4；前端构建 Node 仍是 20 |
| `.github/workflows/security-scan.yml` | actions runtime 走 Node 24 验证；前端 audit 仍使用 Node 20 |
| `.github/workflows/release.yml` | release 产物推送 fork 镜像命名 |
| `.goreleaser.yaml` | 镜像仓库和版本标签按 fork 口径维护 |

CI 相关文档应区分三件事：

- actions runtime：GitHub 执行 JavaScript action 的运行时。
- 项目构建 Node：前端依赖和 Vite 构建使用的 Node 版本。
- Go toolchain：后端测试、lint、release 使用的 Go 版本。
