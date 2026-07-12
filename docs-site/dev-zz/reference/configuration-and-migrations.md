# 配置与迁移索引

这页记录 dev-zz 相对上游新增或语义明显不同的配置、迁移、镜像和 CI 约束。

## 运行时版本

| 项 | 当前约定 |
| --- | --- |
| Go | `backend/go.mod` 声明 `go 1.26.5`，CI 会校验 `go1.26.5` |
| 前端构建 Node | GitHub Actions 仍使用 `node-version: '20'` |
| GitHub JavaScript actions runtime | CI 设置 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true`，用于验证 actions runtime 兼容性 |
| pnpm | 前端和 docs-site 独立 lockfile，CI 前端使用 pnpm 9 |
| docs-site | VitePress 1.x，命令见 `docs-site/package.json` |

Node 24 runtime 变量只验证 GitHub action 执行环境，不等价于项目构建 Node 升级。升级前端构建 Node 前，需要单独验证依赖兼容。

## API Key 相关配置

| 配置 | 默认值 | 说明 |
| --- | ---: | --- |
| `api_key_batch_create_max_count` | 200 | 批量创建 Key 默认上限 |
| 服务端硬上限 | 500 | 批量创建、filtered 批量更新/删除的最大目标数量 |
| 标签数量 | 20 | 单把 Key 最多标签数 |
| 标签长度 | 40 | 单个标签最多字符数 |
| 标签候选 | 500 | `/api/v1/keys/tags` 最多返回数量 |

标签规范化在服务层完成：trim、小写、去重、过滤空字符串。仓储层在写入前还会把 `nil` tags 归一成空数组，避免 PostgreSQL `jsonb` 列写入 JSON `null`。

## OpenAI Chat fallback 账号标记

以下键保存在 OpenAI APIKey 账号的 `accounts.extra` 中，只影响 `openai_responses_mode=force_chat_completions` 或自动探测确认不支持 Responses 的 Chat-only fallback。它们默认均为 `false`。第一项是上游能力；后两项是有损/旧客户端兼容授权，不应误写成上游原生支持。

| extra 键 | 类别 | 默认值 | 说明 |
| --- | --- | ---: | --- |
| `openai_chat_allowed_tools_supported` | 上游能力 | `false` | 上游明确支持 Chat `tool_choice.type=allowed_tools` 时启用；否则多子工具 namespace / Responses `allowed_tools` 触发 capability 换号 |
| `openai_chat_implicit_client_tool_search_enabled` | 兼容授权 | `false` | 仅兼容已验证的旧客户端：把缺省 execution 的 `tool_search` 当作 client；正常请求必须显式 `execution: "client"`，type-only 按 hosted 处理 |
| `openai_chat_lossy_custom_tool_grammar_enabled` | 有损授权 | `false` | 允许把带 grammar/format 的 Responses custom tool 有损包装为 Chat function；默认拒绝并换到能保真的账户 |

这些键属于高级兼容配置，不应根据 host 名自动猜测。后两项只应配置在客户端语义固定的专用账号池中，避免同一 type-only/grammar 请求因随机选中不同账号而改变解释。账号进入 scheduler cache 时会保留三项标记。能力不匹配不会把账号记为运行故障；handler 会排除当前账号并继续选择。只有所有尝试都在访问上游前因 capability mismatch 退出时才返回 `unsupported_feature`；一旦已有账号访问上游并失败，最终优先返回真实 upstream 错误。

## 数据库迁移

| 文件 | 作用 |
| --- | --- |
| `backend/migrations/151_add_api_key_tags.sql` | 给 `api_keys` 增加 `tags jsonb NOT NULL DEFAULT '[]'::jsonb`，补齐空值，并添加 `api_keys_tags_json_array` 约束 |
| `backend/migrations/152_add_api_key_tags_index_notx.sql` | 以 `_notx` 方式创建 `idx_api_keys_tags_gin` 部分 GIN 索引 |
| `backend/migrations/158_add_usage_log_schedule_meta.sql` | dev-zz 用量日志调度证据字段，保留调度来源和模型/账号选择证据 |
| `backend/migrations/158_add_group_peak_rate_multiplier.sql` | 上游分组高峰倍率字段：`peak_rate_enabled`、`peak_start`、`peak_end`、`peak_rate_multiplier` |
| `backend/migrations/165_usage_logs_restrict_dimension_fks.sql` | 将 `usage_logs.user_id`、`api_key_id`、`account_id` 的外键固定为 `ON DELETE RESTRICT NOT VALID`，防止未来物理删除用户、Key 或账号时级联删除用量账本 |
| `backend/migrations/166_upstream_cost_pools.sql` | 阶段 1 上游成本池兼容迁移：新增供应商、资金池、账号成本绑定和成本快照表，给现有账号创建默认资金池，并把旧账号充值记录回填到资金池 |
| `backend/migrations/172_upstream_suppliers_system_flag.sql` | 给上游供应商增加 `is_system` 标志，稳定保护系统保留供应商，避免用本地化名称作为控制位 |
| `backend/migrations/173_upstream_account_binding_group_name.sql` | 给账号成本绑定增加 `upstream_group_name`，把供应商侧分组名与兼容存储列 `default_multiplier` 承载的分组倍率配套保存 |
| `backend/migrations/174_upstream_cost_pool_defaults.sql` | 给资金池增加稳定的默认充值成本、默认参考汇率和 `is_default` 身份；从当前成本 / 汇率回填默认值，并清理配置性快照留下的伪当前成本 |
| `backend/migrations/175_enterprise_members.sql` | 新增企业成员、多 Key 归属、有序分组、月预算预留/账本与成员用量证据基础结构 |
| `backend/migrations/177_enterprise_member_audit_logs.sql` | 新增企业成员管理的 append-only 审计日志与字段白名单触发器 |
| `backend/migrations/178_enterprise_member_import_jobs.sql` | 新增服务器权威导入 job、租约、预览和一次性结果交付状态 |
| `backend/migrations/179_enterprise_member_rate_limits.sql` | 为成员增加所有成员 Key 共享的 5h/1d/7d 限额，并新增独立窗口用量投影表；同时刷新成员审计字段白名单 |

`152` 使用 `CREATE INDEX CONCURRENTLY`，不能放进普通事务迁移。后续合并上游迁移时，需保留 `_notx` 约定，避免长事务锁表。

`158` 当前存在两个文件名不同的迁移：一个来自 dev-zz 调度证据字段，一个来自上游高峰倍率字段。按本分支既有合并口径，迁移以文件名为准并存；后续新增迁移时不要仅按数字判断是否可复用编号。

`165` 是账本完整性迁移：它不会扫描已有 `usage_logs` 全表验证历史行，但会约束后续写入和父表物理删除行为。若需要验证历史 orphan 行，应在低峰期单独执行 `VALIDATE CONSTRAINT` 或只读审计查询。

`166` 是等价迁移，不删除旧账号级充值字段，也不自动合并多个账号的共享钱包。迁移会补 active 供应商名称唯一约束，并在历史回填后验证新增资金池外键。迁移后旧 `/admin/accounts/:id/recharge-records` 兼容入口仍可用；后端新增资金池 API，前端主入口切换和真实合池需要后续显式实现。

`172` 只新增系统供应商保护标志，不改变供应商名称或绑定关系。`is_system=true` 仅用于识别旧迁移遗留的系统行：正常供应商 / 资金池列表、账号绑定候选和 active 绑定查询都应过滤它；若历史 ID 被直接请求修改 / 删除，后端仍返回 `SUPPLIER_RESERVED`。新业务不得再用“未归类供应商”作为账号兜底来源。

`173` 是账号绑定语义迁移：`upstream_group_name` 保存这把上游 key 在供应商侧所属分组；`default_multiplier` 继续作为兼容存储列承载对外 API / UI 的 `upstream_group_multiplier`。

`174` 把低频默认配置与最近一次真实成本分离：`default_effective_cny_per_usd` / `default_reference_fx_rate` 只作为以后新增流水的默认值，`current_effective_cny_per_usd` / `reference_fx_rate` 继续表示当前真实成本快照。迁移优先从已有当前成本回填默认值，缺失时回退现有参考汇率；修改默认值不得重写历史充值记录、当前真实成本或成本快照。`is_default` 为每个未归档供应商提供至多一个稳定默认池身份，后端和前端不再依赖“主余额池”中文名称识别控制位。迁移还会清理早期实现生成的“供应商默认资金池创建时”配置性初始快照，并同步清空其伪 `current_effective_cny_per_usd`；只匹配精确备注、`source_record_id IS NULL` 且整个资金池没有任何充值记录的行，真实成本事实不受影响。

## 破坏性迁移与升级前检查

下列迁移会丢弃旧表、旧列、重复审计行或整块覆盖已有配置。升级前应先完成数据库备份，并在发布说明里确认这些历史数据不再需要保留。

| 文件 | 不可逆行为 | 影响口径 |
| --- | --- | --- |
| `backend/migrations/019_migrate_wechat_to_attributes.sql` | 将 `users.wechat` 迁移到用户属性后删除旧列 | 已软删除用户的旧 `wechat` 值不会被迁移 |
| `backend/migrations/033_ops_monitoring_vnext.sql` | `DROP TABLE IF EXISTS ops_* ... CASCADE` | 旧 Ops 日志、指标、告警和 heartbeat 数据会被丢弃 |
| `backend/migrations/054_drop_legacy_cache_columns.sql` | 删除 legacy cache token 列 | 依赖 `009` 已把值复制到 canonical cache token 列 |
| `backend/migrations/090_drop_sora.sql` | 删除 Sora 表和 Sora 相关列，包括 `usage_logs.media_type` | 旧 Sora 任务、生成、账号和媒体类型字段不再保留 |
| `backend/migrations/127_drop_channel_monitor_deleted_at.sql` | 删除渠道监控历史/聚合表的 `deleted_at` 列 | 监控历史回到物理删除保留策略 |
| `backend/migrations/131_affiliate_rebate_hardening.sql` | 删除重复 `payment_audit_logs(order_id, action)` 行 | 保留每组最早一条审计行，用于建立唯一约束 |
| `backend/migrations/136_remove_ops_retry_replay.sql` | 删除 Ops retry 表和 request body/header/retry 字段 | 旧请求重放证据不再保留 |
| `backend/migrations/049_unify_antigravity_model_mapping.sql` | 覆盖 Antigravity `credentials.model_mapping` 并删除 `model_whitelist` | 自定义账号模型映射会被默认映射替换 |
| `backend/migrations/051_migrate_opus45_to_opus46_thinking.sql` | 整块覆盖 Antigravity `credentials.model_mapping` | 自定义账号模型映射会被默认映射替换 |
| `backend/migrations/058_add_sonnet46_to_model_mapping.sql` | 整块覆盖 Antigravity `credentials.model_mapping` | 自定义账号模型映射会被默认映射替换 |
| `backend/migrations/060_add_gemini31_flash_image_to_model_mapping.sql` | 整块覆盖 Antigravity `credentials.model_mapping` | 自定义账号模型映射会被默认映射替换 |

后续新增迁移若包含 `DROP TABLE`、`DROP COLUMN`、`DELETE FROM`、`TRUNCATE`，或会整块覆盖用户可编辑 JSON 配置，应同步更新本表。能用 key-level merge 的配置迁移，不应再整块覆盖 `credentials.model_mapping`。

## 数据保留默认值

dev-zz 默认关闭 dashboard / ops 自动数据清理，保留管理员显式清理入口。模型状态快照是独立的时序证据表，默认按 90 天保留以避免每分钟快照无限增长。

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
| `model_self_check_status_snapshot_retention_days` | `90` | DB setting；模型状态快照默认每 24 小时清理超过该天数的数据，`0` 表示关闭自动清理，正数小于 `30` 时按 `30` 执行 |

如果自动清理关闭，保留天数允许为 0；如果开启，则必须是正数并通过配置校验。

## 部署镜像

| 项 | 当前值 |
| --- | --- |
| 测试环境分支镜像 | `ghcr.io/thornboo/sub2api:dev-zz-develop`、`ghcr.io/thornboo/sub2api:dev-zz-develop-<shortsha>`，默认多架构 `linux/amd64` + `linux/arm64` |
| dev-zz 正式线候选镜像 | `ghcr.io/thornboo/sub2api:dev-zz`、`ghcr.io/thornboo/sub2api:dev-zz-<shortsha>`，默认多架构 `linux/amd64` + `linux/arm64` |
| 正式发布 Docker Hub | `thornboo/sub2api:latest` |
| 正式发布 GitHub Container Registry | `ghcr.io/thornboo/sub2api:latest` |
| 固定版本示例 | `thornboo/sub2api:1.1.6` |
| 上游镜像 | `weishaw/sub2api:latest`，不包含 dev-zz 二开 |

`deploy/.env.example` 和各 compose 文件默认使用 `SUB2API_IMAGE=thornboo/sub2api:latest`，该值代表正式发布镜像。测试环境应显式改为 `ghcr.io/thornboo/sub2api:dev-zz-develop` 或带 `<shortsha>` 的测试镜像。本地源码构建镜像 `sub2api:dev-zz` 只作为开发验证、应急和未发布代码测试路径。

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
| `.github/workflows/backend-ci.yml` | actions runtime 走 Node 24 验证；Go 版本校验 1.26.5；前端构建 Node 仍是 20 |
| `.github/workflows/security-scan.yml` | actions runtime 走 Node 24 验证；前端 audit 仍使用 Node 20 |
| `.github/workflows/dev-zz-branch-images.yml` | `dev-zz-develop` / `dev-zz` push 构建 GHCR 多架构分支镜像，不更新 `latest` |
| `.github/workflows/release.yml` | release 产物推送 fork 镜像命名 |
| `.goreleaser.yaml` | 镜像仓库和版本标签按 fork 口径维护 |

CI 相关文档应区分三件事：

- actions runtime：GitHub 执行 JavaScript action 的运行时。
- 项目构建 Node：前端依赖和 Vite 构建使用的 Node 版本。
- Go toolchain：后端测试、lint、release 使用的 Go 版本。
