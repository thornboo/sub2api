# 上游合并记录

## 2026-07-18 - 将上游 `main` 合并到 `dev-zz`：提示词审计、安全开关与 Grok 媒体路由合流

分支：
- 目标：`dev-zz`
- 上游：`origin/main`
- Base：`bc2244c83`
- 合并前目标：`78da3e513`
- 上游 head：`b1a6b8026`
- 结果提交：本次合并提交

上游要点：
- 新增独立的 OpenAI 兼容提示词输入审计：管理端配置、节点探测、运行状态、事件筛选/详情/删除，异步审计和可选阻断模式，以及 PostgreSQL 任务/事件证据与 Redis 临时载荷。
- 将 `step_up_enabled` 和 `session_binding_enabled` 明确为默认关闭的安全开关；备份 S3 保存、管理员角色提升等敏感操作继续在开关启用时执行 TOTP step-up，并统一审计日志与会话绑定的客户端 IP 信任口径。
- Grok 媒体调度新增账号资格覆盖与探测隔离，被动 `image_gen` namespace 不再误触发显式图片权限；Grok 媒体缓存、alpha/search APIKey 调度、Stripe 懒加载和账号上游站点入口同步修正。

合并策略：
- 合并前完整读取 `docs-site/dev-zz` 的分支策略、上游同步流程、历史合并记录、补丁/变更记录、变更地图、API/迁移索引和验证矩阵；刷新 `origin/main` 后，使用 `git merge-tree --write-tree --messages --name-only --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 只读预演，再执行 `git merge --no-commit origin/main`。
- 预演和真实合并均得到 7 个内容冲突。接受上游安全审计、Grok 媒体资格、协议正确性和支付懒加载修复；继续保留 dev-zz 企业成员路由/预算/归因、Ops 分类 v2、fork 镜像、数据保留、默认 Rollup chunk graph 和 `1.7.8` 版本线。
- `181_prompt_audit.sql`、`182_prompt_audit_full_prompt.sql` 与既有同号迁移按完整文件名并存；没有修改任何已应用迁移。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/grok_media.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/service/account.go`
- `deploy/docker-compose.yml`
- `frontend/vite.config.ts`

解决说明：
- `VERSION` 保持 `1.7.8`，Compose 默认镜像保持 `thornboo/sub2api:latest`；不采用上游 `0.1.160` 和仅本机构建可用的 `sub2api:latest`。
- Grok 新任务按 `grok_media_generation` 资格筛选账号；已持久化的异步视频状态查询仍固定回到原 group/account，不因资格变化或普通 failover 切换到其它凭据租户。
- Responses WebSocket 每个 turn 先按企业成员模型与请求体预留预算，首 turn 复用握手阶段审计，后续 turn 使用新的安全审计协调器；阻断不会绕过预算回收/结果不明保护。
- `OpenAICacheTokenUsageMode` 与 `GrokMediaEligibleExtraKey` 同时保留；Wire 从合并后的 `wire.go` 重生成，并补齐 `PromptAdminService` 绑定以及安全审计、step-up、企业预算、Grok 任务仓储、模型自检和企业导入 worker 的联合注入/清理。
- 前端继续使用默认 Rollup chunk graph，避免恢复曾导致生产循环 chunk 白屏的手工 vendor 分包；Stripe 三个消费入口仍通过 `@stripe/stripe-js/pure` 动态加载，回归测试改为验证动态加载和禁止 `manualChunks`，而不是锁定冲突的 `vendor-stripe` 实现。

验证：
- `mise x -C backend -- go run github.com/google/wire/cmd/wire ./cmd/server`
- `mise x -C backend -- go test ./cmd/server ./internal/handler ./internal/service ./internal/server ./internal/server/middleware ./internal/securityaudit ./migrations -count=1`
- `mise x -C backend -- go test ./... -run '^$' -count=1`
- `make -C backend test-unit`
- `mise x -C backend -- golangci-lint run --timeout=30m`（0 issues）
- `mise x -C backend -- go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`（211 个测试文件、1413 个测试通过）
- `pnpm --dir frontend build`
- `pnpm --dir docs-site docs:build`
- 排除上游只读 source-freeze patch/tar 归档后的 whitespace 检查、未合并索引与冲突标记扫描。

未验证：
- 浏览器人工 smoke。
- Docker / Testcontainers 运行时集成测试；本轮只完成 repository integration 测试二进制编译。

## 2026-07-17 - 将上游 `main` 合并到 `dev-zz-develop`：异步图片、倍率探测、图片计费与操作审计合流

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`eb2b8632d`
- 合并前目标：`991fcc829`
- 上游 head：`bc2244c83`
- 结果提交：本次合并提交

上游要点：
- 异步图片生成 / 编辑任务、S3 兼容结果转存、任务轮询接口，以及图片输入 Token 的独立定价、用量和费用字段。
- API Key 计费倍率自省、上游 Sub2API 倍率探测、低上游倍率优先和调度快照批量刷新优化。
- 操作审计日志、会话 IP/UA 绑定、敏感操作 step-up 2FA、管理员角色提升加固和管理员批量用户限额。
- 分组 / 渠道监控幂等复制、Grok 上游端点快捷切换、Codex Responses WebSocket v2、图片模型路由、body-limit failover、Responses rejected-field retry 与 WebSocket ingress 修复。

合并策略：
- 合并前完整读取 `docs-site/dev-zz/branch-policy.md`、`maintenance/merge-main.md`、`maintenance/merge-log.md`、`patches.md`、`changelog.md`、`reference/change-map.md`、`reference/api-surface.md`、`reference/configuration-and-migrations.md` 和 `testing/verification-matrix.md`。
- 先把目标分支从 `414287721` 快进到正式 `origin/dev-zz@991fcc829`，使 `VERSION` 与已发布 `1.7.4` 一致；随后刷新 `origin/main`，使用 `git merge-tree --write-tree --messages --name-only --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 只读预演，再执行 `git merge --no-commit origin/main`。预演和真实合并均得到同一组 33 个内容冲突。
- 接受上游安全、图片、计费和协议正确性修复；继续保留 dev-zz 企业成员路由 / 预算 / 归因、owner / admin 数据隔离、供应商成本池、`schedule_strategy`、隐藏认证入口、数据保留、stone / emerald 视觉和 `1.7.4` 版本线。
- `178`、`179`、`180`、`181` 同号迁移按完整文件名并存；没有修改任何已应用迁移。

冲突文件：
- `.gitignore`
- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/admin/channel_handler.go`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/grok_media.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/repository/usage_log_repo_insert.go`
- `backend/internal/repository/usage_log_repo_query.go`
- `backend/internal/repository/usage_log_repo_request_type_test.go`
- `backend/internal/server/http.go`
- `backend/internal/server/router.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/openai_account_runtime_block_fastpath.go`
- `backend/internal/service/openai_gateway_scheduling.go`
- `backend/internal/service/openai_images_test.go`
- `backend/internal/service/openai_ws_forwarder_ingress.go`
- `backend/internal/service/setting_parse.go`
- `backend/internal/service/setting_service_update_test.go`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/admin/channel/PricingEntryCard.vue`
- `frontend/src/components/admin/user/UserCreateModal.vue`
- `frontend/src/components/common/DataTable.vue`
- `frontend/src/components/keys/UseKeyModal.vue`
- `frontend/src/i18n/locales/en/common.ts`
- `frontend/src/i18n/locales/zh/common.ts`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`
- `frontend/src/views/admin/__tests__/AccountsView.schedulerScore.spec.ts`

解决说明：
- `wire_gen.go` 从合并后的 `wire.go` 重新生成，同时注入上游审计 / step-up / 异步图片 / 倍率探测和 dev-zz 企业成员预算、模型自检、成本池服务。
- 网关入口为同步图片、异步图片、batch image、Responses / Chat / Embeddings 和无前缀别名统一保留成员分组解析、预算保护与组内编排；`/v1/sub2api/billing` 只走 Key 鉴权，不误占计费并发。
- usage log SQL 同时保留 `enterprise_member_id`、`schedule_meta`、真实 `upstream_endpoint` 和上游新增的 `image_input_tokens` / `image_input_cost`；单条、批量和 best-effort insert 的列、类型、参数与查询顺序保持一致。
- 账号调度同时保留 dev-zz `strict_priority` / `cost_first`、供应商成本和上游新增的低倍率优先；账号列表、编辑弹窗与设置页同时展示供应商成本和倍率探测，不向普通用户 DTO 暴露上游成本。
- OpenAI APIKey 参数 400 不写持久化模型冷却，502/503/504 等瞬时错误采用上游 account+model 连续失败运行时冷却；404、明确模型限流和其它平台模型错误继续走 dev-zz 持久化模型级冷却。
- `DataTable` 保留 stone / emerald 与 BaseCheckbox 可访问控件，合入上游选择列、选中 Key 和横向滚动修复；`UseKeyModal` 保留 dev-zz 视觉，同时恢复窄屏 client tabs 的滚动合同。
- `VERSION` 保持 `1.7.4`；`.gitignore` 继续忽略 docs-site 构建产物，但显式跟踪上游 `docs/ASYNC_IMAGE_TASKS.md`。

验证：
- `mise x -C backend -- go run github.com/google/wire/cmd/wire ./cmd/server`
- `mise x -C backend -- go test ./... -run '^$' -count=1`
- `make -C backend test-unit`
- `mise x -C backend -- golangci-lint run --timeout=30m`
- `mise x -C backend -- go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`（204 个测试文件、1371 个测试通过）
- `pnpm --dir frontend build`
- `pnpm --dir docs-site docs:build`
- `git diff --check`、`git diff --cached --check`、未合并索引与冲突标记扫描。

未验证：
- 浏览器人工 smoke。
- Docker / testcontainers 运行时集成测试。

## 2026-07-16 - 增量合并上游 `main`：Grok 自定义上游、Agent Identity、订阅币种与管理员充值返佣

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`d515c3045`
- 合并前目标：`85f3dcda4`
- 上游 head：`eb2b8632d`
- 结果提交：本次合并提交

上游要点：
- Grok API Key / OAuth 账号支持自定义转发 `base_url` 与请求头覆写；OAuth 官方地址继续走可信端点，自定义地址受全局 URL allowlist / HTTPS 策略约束，认证头和会话路由头不能被覆写。
- OpenAI Agent Identity 增加独立导入入口、Codex 能力、过期校验和授权层级修正；models / quota 相关服务同步补齐 Agent Identity 行为。
- 订阅套餐新增币种字段及迁移，用户套餐卡和管理员套餐编辑显示真实币种；管理员手工充值可以按设置决定是否计入 affiliate rebate。
- 账号请求头编辑器增加 JSON 导入 / 复制与全部 locale 消息编译测试，Grok 建号表单补齐上游配置入口并修复 JSON 示例导致的运行时 i18n 编译崩溃。

合并策略：
- 合并前完整读取 `docs-site/dev-zz` 分支策略、上游同步流程、补丁 / 变更记录、变更地图与验证矩阵，刷新 `origin/main` 后使用 `git merge-tree --write-tree --messages --name-only` 只读预演，再执行 `git merge --no-commit origin/main`。
- 接受上游 14 个提交、79 个文件中的 Grok、Agent Identity、订阅币种、充值返佣和 i18n 正确性改进；继续保留 dev-zz 企业成员路由 / 预算 / 归因、owner / admin 数据边界、`1.7.2` 版本线与 stone / emerald 视觉契约。
- 上游新增 `177_add_subscription_plan_currency.sql` 与既有 `177_enterprise_member_audit_logs.sql` 按完整文件名并存；没有修改任何已应用迁移。

冲突文件：
- `frontend/src/components/account/CreateAccountModal.vue`

解决说明：
- 账号创建表单同时保留 dev-zz 的 `ModelCatalogSearch` / `buildChannelModelRecommendations` 与上游的 `HeaderOverrideEditor`，避免在唯一 import 冲突中丢失任一侧能力。
- 上游新增 locale 编译测试直接导入 `@intlify/message-compiler`，但没有声明直接依赖；补充与现有 `vue-i18n` 一致的 `9.14.5` 开发依赖并更新 lockfile，确保 pnpm 严格依赖布局和干净 CI 环境可解析测试。
- 新增请求头编辑器、JSON 工具和 Grok OAuth 开关从上游 `primary / blue / dark-*` 色板收敛为 dev-zz 的 stone / emerald / rose 体系；开关补充 `role="switch"` 与 `aria-checked`。

验证：
- `mise x -C backend -- go test ./internal/pkg/xai ./internal/service ./internal/handler/admin ./internal/handler ./internal/server -count=1`
- `mise x -C backend -- go test ./... -run '^$' -count=1`
- `make -C backend test-unit`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`
- `pnpm --dir frontend build`
- `pnpm --dir docs-site docs:build`
- `git diff --check`、`git diff --cached --check`、未合并索引与冲突标记扫描。

未验证：
- 浏览器人工 smoke。
- Docker / testcontainers 集成测试。

## 2026-07-15 - 增量合并上游 `main`：Grok OAuth 池、Chat bridge、账号复制与 Key ID

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`3f605c354`
- 合并前目标：`d4899ac77`
- 上游 head：`d515c3045`
- 结果提交：本次合并提交

上游要点：
- Grok OAuth 刷新改为分页候选、平台级并发 / QPS / 熔断与周期超时控制，新增 OAuth 账号对账、主动刷新、Free 缓存账号识别、函数工具缓存、vision 图片桥和通用账号刷新路由修复。
- OpenAI / Codex 增加 native Responses 首输出超时、WebSocket 首消息超时、Messages 流错误事件、Codex 图片函数工具保留、Responses Lite 工具归一化，以及 Read tool 完整 / 不完整流的安全终止。
- 新增 Anthropic Messages 与 Chat Completions 的直接转换桥；管理员可幂等复制静态凭据账号，复制结果保留配置和有序分组但重置运行态；账号复制重试按管理员作用域隔离。
- `/models` 增加无 `/v1` 根路径别名；用户 Key 表新增默认隐藏、可排序的 ID 列；调度快照 outbox 在降级重建期间保持 latch，XAI OAuth 拒绝带不安全组件的 base URL。

合并策略：
- 合并前读取 `docs-site/dev-zz` 文档中心、分支策略、上游同步流程、补丁 / 变更记录、变更地图与验证矩阵；刷新 `origin/main`，并把无本地独有提交的 `main` 快进到 `d515c3045`。
- 使用 `git merge-tree --write-tree --messages --name-only --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 只读预演，再执行 `git merge --no-commit origin/main`；预演和真实合并均得到同一组 7 个内容冲突。
- 接受上游 Grok OAuth 池健康、Responses / WebSocket 超时、直接响应桥、账号复制、根路径 models、Key ID 与调度正确性修复；继续保留 dev-zz 企业成员路由、Tool Search / hosted-tool 无损契约、成本池测试桩、批量 Key / 标签能力、stone / emerald 视觉和 `1.7.2` 版本线。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go`
- `backend/internal/pkg/apicompat/chatcompletions_responses_bridge_custom_tools_test.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/service/openai_gateway_messages_chat_fallback.go`
- `frontend/src/views/admin/__tests__/AccountsView.sparkShadow.spec.ts`
- `frontend/src/views/user/KeysView.vue`

解决说明：
- `VERSION` 保留已发布 dev-zz `1.7.2`，不采用上游 `0.1.156`。
- Responses → Chat 工具解析继续统一走 `BuildResponsesToolRegistry`，保留 deferred / namespace / tool_search 历史身份和 hosted-tool capability mismatch；吸收上游“非载体字段不误解析、畸形 additional_tools 明确失败”的回归测试，不引入功能更弱的平行 `EffectiveResponsesTools`。
- Messages 的请求侧继续保留 Anthropic → Responses → Chat 链，以维持 dev-zz prompt cache、replay guard、Fast/Flex 和工具注册策略；响应侧采用上游 Chat → Anthropic 单状态机，减少每个流式 chunk 的重复转换，并适配 dev-zz `scanCCStream` 的可失败回调合同。
- 完整 unit 闸门发现直接 Chat → Anthropic 状态机未继承 dev-zz 的工具参数资源上限；现与 Responses 桥统一执行单调用 16 MiB、单响应 32 MiB 限制，超限发送标准 Anthropic `event: error`、停止读取上游并禁止正常 `message_stop` 收尾。
- `/v1/models` 和 `/models` 复用同一个企业成员分组编排 handler；根路径别名补齐成员分组解析、预算保护和 fallback 中间件，不能绕开成员授权。
- 账号页测试同时保留上游成本池接口桩和上游新增的账号复制桩；Key 表同时保留批量选择、标签列和新增 ID 列，ID 默认隐藏并使用 stone 色板。
- 后端全包编译发现两个 dev-zz 既有测试未跟随 `NewAccountHandler` 新增依赖参数，补齐显式 `nil`；同时清理一个上游测试文件尾部多余空行，使 `git diff --check` 恢复干净。

验证：
- `mise x -C backend -- go test ./... -run '^$' -count=1`
- `mise x -C backend -- go test ./internal/pkg/apicompat ./internal/server/routes ./internal/service`
- `make -C backend test-unit`
- `mise x -C backend -- golangci-lint run --timeout=30m`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`（192 个测试文件、1288 个测试通过）
- `pnpm --dir frontend build`
- `pnpm --dir docs-site docs:build`
- `git diff --check`、`git diff --cached --check`、未合并索引与冲突标记扫描。

未验证：
- 浏览器人工 smoke。
- Docker / testcontainers 集成测试。

## 2026-07-15 - 将上游 `main` 合并到 `dev-zz-develop`：Agent Identity、Grok 运行时、长上下文计费与 Ops 可观测性合流

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`7d239d62`
- 合并前目标：`9934b830`
- 上游 head：`4355861e`
- 结果提交：本次合并提交

上游要点：
- OpenAI 新增 Agent Identity 认证与任务失效恢复，Codex models manifest 支持跨账号重试，并补齐 Responses namespace、WebSocket / HTTP bridge、图片生成和请求取消边界。
- Grok 新增 SSO device OAuth、导入后自动探测、渠道监控、滚动 24h 免费额度估算、凭据级 failover、上游 URL 归一化和图片 / 视频实际计费修复。
- OpenAI 账号新增可选长上下文计费，usage log 保存是否应用长上下文倍率；账号创建 / 编辑和管理端用量表展示对应配置与证据。
- 系统日志新增 `host` 持久化、筛选、清理条件与索引；管理员 UI 请求可选输出 `Server-Timing`，并新增 SQL / Redis timing 汇总。
- 调度器吸收 auto-pause / proxy expiry 增量刷新、pending lag / rebuild coalescing、请求取消感知 failover 和账号投影性能修复。
- 管理端分组列表新增可选 ID 列，账号页补充 OpenAI 认证模式；内容审核、Ops 队列投影与 content seed 扫描获得热路径优化。

合并策略：
- 合并前完整阅读 `docs-site/dev-zz` 分支政策、补丁记录、变更地图、配置 / API 索引、验证矩阵与历史 merge-log，并刷新 `origin/main`。
- 使用 `git merge-tree --write-tree --messages --name-only --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 只读预演，再用 `git merge --no-commit origin/main` 展开真实合并；预演与实际均得到同一组 20 个内容冲突。
- 接受上游 Agent Identity、Grok、长上下文计费、Server-Timing、系统日志 host、调度器和网关正确性修复；保留 dev-zz 企业成员归因、owner / admin 隐私边界、`schedule_meta`、fork 镜像、`1.7.1` 版本线和 stone / emerald 视觉。
- Ent 生成文件不手工维持冲突结果：先合成 `UsageLog` schema 的成员字段与 `long_context_billing_applied`，再执行 `go generate ./ent` 重新生成。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `backend/internal/handler/admin/ops_handler.go`
- `backend/internal/handler/dto/credentials_redact_test.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/gateway_handler_chat_completions.go`
- `backend/internal/handler/grok_media.go`
- `backend/internal/handler/openai_codex_models_handler.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/repository/usage_log_repo_insert.go`
- `backend/internal/repository/usage_log_repo_query.go`
- `backend/internal/service/ops_models.go`
- `deploy/.env.example`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/admin/usage/UsageTable.vue`
- `frontend/src/views/admin/ops/components/OpsSystemLogTable.vue`

解决说明：
- `VERSION` 保留 dev-zz `1.7.1`，不采用上游 `0.1.155`；部署示例同时保留 `SUB2API_IMAGE=thornboo/sub2api:latest` 和新增 `ENABLE_SERVER_TIMING=false`。
- 用量写入 / 查询同时保留成员 ID、成员编号 / 名称快照、`schedule_meta` 与上游 `long_context_billing_applied`；单条、批量和 best-effort SQL 的 58 个参数及扫描顺序统一维护。
- 普通用户用量 DTO 保留成员归因和长上下文证据，但继续不返回 `account_id`；管理员 DTO 仍保留完整调度调查字段。
- Ops provider-health 默认同时读取 `upstream` / `account_auth` 和 recovered 行，用户请求错误接口仍受 `status>=400` 与 owner/member 范围约束；`StatusCodesExclude` 继续保留。
- Responses / Chat Completions / Codex models / Grok media 同时保留 dev-zz group failover 证据、capability mismatch 换号、WebSocket turn 预算与持久化异步任务账号，并吸收上游取消感知、凭据错误脱敏、Retry-After 和 OAuth 429 failover 边界。
- 账号调度快照同时保留成本池显式刷新接口和上游脱离请求取消的短超时刷新，避免请求结束后丢失状态传播。
- 系统日志 UI 保留 dev-zz 确认弹窗与 stone 视觉，并把 host 纳入列表、查询、清理 payload 和确认摘要；账号长上下文开关同步改用 stone / emerald 样式。
- `origin/main@4355861e` 的 `openai_gateway_messages.go` 使用 `xai.ParseQuotaHeaders` 却漏导入 `internal/pkg/xai`，合并后编译闸门实证失败；本次同步补入 import，避免把已知上游红灯带入 dev-zz。
- 上游新增的 failover 单元测试按 dev-zz 扩展后的 gateway handler 构造函数补齐企业成员预算服务与 Grok 任务仓储占位参数；Ops 参数契约测试同步按成员归因新增的 3 列校验 44 参数及正确索引。
- 系统日志清理测试改为验证 ConfirmDialog 的显式确认契约，不再依赖已经移除的 `window.confirm`。
- 上游 `174/175/176` 迁移与 dev-zz 同号文件按完整文件名并存，不修改任何已应用迁移。

验证：
- `go generate ./ent`
- `go test ./... -run '^$' -count=1`
- `go test ./internal/service ./internal/handler ./internal/repository ./internal/server -run '^$' -count=1`
- `make -C backend test-unit`
- `golangci-lint run --timeout=30m`（`backend`）
- `go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`（190 个测试文件、1223 个测试通过）
- `pnpm --dir frontend build`
- `pnpm --dir docs-site docs:build`
- `git diff --check`、`git diff --cached --check`、未合并索引和冲突标记扫描。

未验证：
- 浏览器人工 smoke。
- Docker / testcontainers 集成测试。

这里记录二开分支吸收上游变更的同步工作。

## 2026-07-13 - `bee874106` 合并后 Codex 套餐限流语义复审修复

复审结论：
- `dev-zz` 精确提交 `bee874106` 的 GitHub Actions 中，frontend、golangci-lint 和 shell job 通过，test job 的 `make test-unit` 唯一失败于 `TestRateLimitService_HandleUpstreamError_CodexPlanGatedModelIgnoresAPIKeyAccount`。
- 失败不是 CI 环境或时序抖动；该测试文件带有 `//go:build unit`，合并前执行的普通 `go test ./...` 不会编译它，使用 CI 同款 `go test -tags=unit` 可在本地稳定复现。
- 上游专用处理只允许 OpenAI OAuth 账号把 ChatGPT/Codex 套餐限制 400 转成模型冷却，但 dev-zz 的通用供应商模型失败处理随后又捕获了同一 400，导致 API Key 账号被错误冷却并返回 failover。

修复策略：
- 在通用供应商模型失败处理入口识别该专用错误；当账号不是 OpenAI OAuth 时直接跳过通用模型冷却，交回普通 400 处理。
- OpenAI OAuth 的 30 分钟账号/模型冷却、模型映射、请求 failover 保持不变；其他 OpenAI API Key 错误、其他平台和其他 4xx/5xx 通用冷却策略不变。
- 不通过修改测试接受错误行为；保留上游新增的 OAuth/API Key 边界测试作为长期回归契约。

验证：
- `go test -tags=unit ./internal/service -run '^TestRateLimitService_HandleUpstreamError_CodexPlanGatedModel' -count=1 -v`
- `make -C backend test-unit`
- `mise x -C backend -- go test ./...`
- `mise x -C backend -- golangci-lint run --timeout=30m`
- `git diff --check` 和 `git diff --cached --check`。

流程修正：
- 后续 `main` 合并只要上游或冲突范围包含带 build tag 的测试，最终门禁必须同时执行 `make -C backend test-unit`，不能以普通 `go test ./...` 代替 tagged 单元测试。
- 本条作为 `dev-zz` CI follow-up 提交；修复推送后再将 `dev-zz-develop` 快进到同一提交，不打 tag、不发布。

## 2026-07-13 - 增量合并上游 `main`：Grok 媒体、Alpha Search、WebSocket 生命周期与 Apple Container

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`e316ebf5`
- 合并前目标：`d2a8f4c4`
- 上游 head：`7d239d62`
- 结果提交：本条所在合并提交

上游要点：
- Grok 增加 OAuth/API Key 账号配置、第三方 Base URL、媒体能力、视频编辑与扩展路由，并补充缓存和配额处理。
- OpenAI 增加 Alpha Search 端点、按次计费配置和分组字段；WebSocket 转发补齐生命周期、连接池、重试和失败边界。
- Responses/Chat/Anthropic 兼容层继续修正工具、流式终态、usage 和 Codex identity 转换；调度、冷却、并发与上游错误分类同步吸收正确性修复。
- 前端 DataTable 对小数据集跳过虚拟滚动，管理员设置增加 Fast/Flex 用户搜索选择器，并修正日期输入、账号/Grok 配置和用量展示。
- 部署侧增加 Apple Container 脚本、示例环境和生命周期夹具测试；仓储侧增加 API Key 最近 IP 查询索引。

合并策略：
- 合并前完整阅读 `docs-site` 的分支策略、合并流程、补丁目录、变更映射、配置与迁移约束、验证矩阵以及企业成员/企业用量设计；刷新 `origin/main` 后以 `git merge-tree` 预检，再执行 `git merge --no-commit origin/main`。
- 接受上游网关、媒体、计费、WebSocket、仓储索引、前端性能与部署正确性改进；继续保留 dev-zz `1.6.0`、二开镜像、stone/emerald 视觉体系、显式数据保留策略、企业成员路由和 owner/admin 字段边界。
- 上游新增两个文件名前缀均为 `174` 的迁移；仓库迁移规则按完整文件名区分且已有同前缀先例，因此两个新迁移与本地 `175` 至 `181` 并存，不修改任何已经应用的迁移内容。
- Alpha Search 和新增 Grok 路由纳入现有企业成员解析、分组资格、限额和用量编排；Responses 工具转换继续使用 dev-zz 的 request-local registry 与严格 capability mismatch 语义，同时吸收上游 `additional_tools` 正确性覆盖。

冲突文件：
- `backend/cmd/server/VERSION`：保留 dev-zz `1.6.0`，不采用上游 `0.1.153`。
- `deploy/.env.example`：保留 `thornboo/sub2api:latest`，同时加入上游 Apple Container 镜像配置。
- `backend/internal/handler/ops_capture_writer_nil_test.go`、`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/handler/openai_gateway_endpoint_normalization_test.go`：合并测试依赖，保留本地账号耗尽语义，并采用上游覆盖更完整的 endpoint 解析。
- `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go`、`backend/internal/pkg/apicompat/chatcompletions_responses_bridge_custom_tools_test.go`、`backend/internal/service/openai_gateway_responses_chat_fallback.go`：保留共享工具 registry 与严格无损转换边界，吸收上游 `additional_tools` 测试。
- `backend/internal/service/api_key_auth_cache_impl.go`：缓存版本提升至 `18`，同时失效上游 Web Search 定价和 dev-zz 企业成员聚合限额缓存。
- `backend/internal/service/openai_gateway_grok_test.go`、`backend/internal/server/routes/gateway.go`、`backend/internal/server/middleware/enterprise_member_group.go`：合并 Grok/Alpha Search 路由，并补齐企业成员平台资格检查。
- `frontend/src/components/common/DataTable.vue`、`frontend/src/i18n/locales/en/admin/settings.ts`、`frontend/src/i18n/locales/zh/admin/settings.ts`、`frontend/src/views/admin/SettingsView.vue`、`frontend/src/views/user/DashboardView.vue`：吸收小列表性能、用户选择器和本地日期格式修复，同时保留二开视觉和兼容文案键。

合并复审修复：
- 删除自动合并后 `OpenAIGatewayResult.UpstreamEndpoint` 的重复字段，避免编译失败。
- `opsCaptureWriter` 增加显式 retained 标记；compact SSE keepalive 持有 wrapper 时不再把已逃逸对象放回 `sync.Pool`，并以定向 race 测试锁定跨请求生命周期。
- 新增 Alpha Search 企业成员分组资格回归，确认 OpenAI 分组允许、Grok 分组拒绝；新增路由均经过企业成员预算和用量编排。
- 保留旧 Fast/Flex 用户 ID 文案键，避免现有 locale contract 测试和旧调用点因上游重命名回归。

验证：
- `mise x -C backend -- go test ./...`
- `mise x -C backend -- go test -race ./internal/handler -run '^TestOpsErrorLoggerMiddleware_DownstreamWriterDoesNotEscapeIntoPool$' -count=1`
- `mise x -C backend -- go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`（175 个测试文件、1105 个测试通过）
- `pnpm --dir frontend build`
- `pnpm --dir docs-site docs:build`
- `bash -n deploy/apple-container.sh deploy/tests/apple-container-test.sh deploy/tests/fixtures/bin/container deploy/tests/fixtures/bin/curl`
- `bash deploy/tests/apple-container-test.sh`
- `git diff --check`、`git diff --cached --check`、未合并索引和冲突标记扫描。

未验证：
- 浏览器人工 smoke。
- 真实 Docker/testcontainers 集成测试运行；本地只编译 repository integration 测试二进制，运行交给远端 CI。
- 远端 CI、镜像构建和生产升级；本轮只合并并提交 `dev-zz-develop`，不推送、不提升 `dev-zz`、不打 tag、不发布。

## 2026-07-11 - `e316ebf5` 合并后 Tool Search 协议复审修复

复审结论：
- 合并结构、dev-zz 边界和 CI 均正常，但第二轮独立复审确认提交 `cca6a16c` 对 Tool Search 的“可逆”描述过强：type-only hosted 请求被改写为 client execution，deferred 工具提前暴露，动态顶层 function 丢失 namespace 身份。
- 本条 follow-up 在 `dev-zz-develop` 修复上述阻断项；不重写已经推送的合并提交，不提升 `dev-zz`、不打 tag、不发布。

修复策略：
- 使用 request-local `ResponsesToolRegistry` 保留载体来源、加载状态、输入顺序和 Chat/Responses 双向名称；service 与 converter 共享同一实例。
- 只有显式 `execution: "client"` 的 tool search 默认可进入 Chat fallback；hosted/server 或无法保真的 custom grammar 返回 typed capability mismatch。
- capability mismatch 由 handler 排除当前账号继续换号，但不调用账号失败评分；全部候选不支持时向客户端返回 `unsupported_feature`。
- `allowed_tools`、旧客户端隐式 client 兼容和有损 custom grammar 使用账号 extra 显式声明；默认不假设第三方 OpenAI-compatible Chat 实现支持这些能力。
- hosted/server-only 工具与 Chat 名称/回程 identity 冲突同样触发 capability 换号；若已有可表达请求的账号真正访问上游并失败，最终优先保留 upstream failover，而不被后续 capability miss 改写成 400。
- 工具定义比较保留 JSON number，并在账号调度和完整工具树解码前执行重复-key-aware 原始载荷预检；顶层、动态载体与 `tool_choice.allowed_tools` 共享数量、单定义、总定义和 namespace 深度预算。
- Registry replay 同步缓存每个历史 function call 的 Chat 名，消息转换阶段不再按 item 回扫全部工具；原始载荷预检只保留安全相关字段，并限制 input item、content/summary part 总数及关键/嵌套对象字段数；part 转换改用最小字段结构，上游 custom arguments 改为无 map 的字段读取；流式工具参数改用线性 buffer，并设置单调用 16 MiB / 单响应 32 MiB 上限，超限按 Responses / Anthropic 各自协议发送失败终态、禁止正常 message stop/finalize 并停止读取，封闭大请求和异常上游返回的 CPU / 内存放大路径。
- fallback 内其余客户端校验错误返回 typed `OpenAIClientRequestError`；handler 不把未访问上游的 400 计入账号调度错误率。

验证重点：
- type-only hosted 请求不得生成 Chat proxy；显式 client 仍完整恢复 `tool_search_call execution=client`。
- 顶层和 namespace deferred 工具加载前不可调用，出现在 `additional_tools` / client `tool_search_output` 后才进入当前集合。
- 动态顶层 function 的历史、非流式、流式 added/done/completed 均恢复 `namespace=name`。
- 重复 call ID 更新只产生一个 Chat tool result，历史身份按 item 位置解析；流式 added/done/completed 的 item ID 必须一致。
- 能力换号、hosted 工具拒绝、定义/identity 冲突、非法 execution、重复 JSON key、`allowed_tools` 资源预算、历史 identity replay cache、对象字段上限、input/content part 数量上限、嵌套 image URL、最小字段 part 解码、大 unknown-field custom arguments、流式单调用/总参数上限和转换错误停止读取均有回归覆盖；最终验证命令与未验证范围在本轮交付记录中报告。

## 2026-07-10 - 增量合并上游 `main`：Codex MCP、custom 与 tool_search bridge 补全

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`07fac347`
- 合并前目标：`fb9a3324`
- 上游 head：`e316ebf5`
- 结果提交：本条所在合并提交

上游要点：
- Responses → Chat Completions bridge 支持 custom / freeform 工具，把自由文本输入降级为单字段 function schema，并在非流式和流式回程中还原为 `custom_tool_call` 与 `custom_tool_call_input.*` 事件，修复 Codex `exec` 等工具在 chat-only 上游丢失的问题。
- 显式 client `tool_search` 降级为同名代理 function，历史调用、结果和强制 `tool_choice` 保持可往返；2026-07-11 follow-up 明确 type-only 为 hosted，chat-only 账户不能把它改写成 client。
- namespace 子工具按 `<namespace>__<name>` 摊平到 Chat Completions；超长名字使用稳定哈希后缀，顶层/跨 namespace 撞名显式拒绝，回程重新写入原始 `namespace` 和子工具名，避免 Codex 把 MCP 调用判为 unsupported call。
- `tool_choice` 只指向实际保留下来的工具；simple custom 和显式 client tool_search 的具名选择转换为 function 选择，namespace / `allowed_tools` 受上游能力门控，无法保真时触发 capability 换号。
- 流式 wire 补齐 custom tool input 的 zero-value index、done/input 字段，以及 namespace / tool_search 输出项的必需字段。

合并策略：
- 合并前完整阅读 `branch-policy.md`、`maintenance/merge-main.md`、`patches.md`、`maintenance/merge-log.md`、`changelog.md`、`reference/change-map.md` 和 `testing/verification-matrix.md`；刷新远端后确认本地 `main` 与 `origin/main` 同为 `e316ebf5`。
- 用 `git merge-tree --write-tree --merge-base 07fac347 HEAD origin/main` 做只读预检，结果为干净合并树；真实合并使用 `git merge --no-commit origin/main`，无文本冲突。
- 本轮上游增量为 10 个提交、8 个后端文件，仅涉及 `internal/pkg/apicompat` 与两个 OpenAI chat fallback 文件；不含迁移、依赖、前端、部署、workflow 或版本变化。
- 接受上游 Codex MCP/custom/tool_search 正确性修复；继续保留 dev-zz 的 Responses Fast / Flex 策略、billing/upstream model 归一化、真实 usage 与 endpoint 证据、messages fallback 顺序、用户/admin 字段隔离和模型自检边界。
- Anthropic Messages fallback 调用新的 converter 签名时显式传入空 custom/tool_search/namespace 元数据，保持既有 Anthropic 工具和 usage 转换语义；Responses fallback 才携带原请求工具映射完成回程还原。

冲突文件：
- 无。

合并复审修复：
- 对照 OpenAI Tool Search 文档补齐真实第二轮形态：`tool_search_output.tools` 与 `additional_tools.tools` 都会并入下一轮可调用工具；`tool_search_output` 同时生成与原 `call_id` 配对的 Chat tool result，不再读取并不存在的 `output` 字段。
- 客户端 `tool_search` 自带的 `description` / `parameters` 原样用于代理 function；2026-07-11 follow-up 要求显式 `execution=client`，type-only hosted 与显式 server 都由 chat-only 账户提前返回 capability mismatch。
- namespace 强制选择在单一子工具时映射为具名 function，多子工具时映射为 `mode=required` 的 Chat `allowed_tools`；已丢弃托管工具、不存在的工具名、源类型不匹配（function / custom）和不可转换的 `allowed_tools` 项显式失败，不再静默放宽或重新解释。
- function / custom 同名、`tool_search` 代理同名、namespace 摊平名碰撞统一拒绝；同类型同名工具只有完整定义等价时去重，schema / description / custom grammar `format` 乃至尚未建模的原始字段存在差异时显式失败。namespace 流式 arguments delta、added 与 done 均使用原始裸子工具名，避免同一调用生命周期内名称不一致。
- 新增官方第二轮回归：不重复声明顶层 `tool_search`，仅重放 tool search call 与 `tool_search_output.tools`，仍能生成下一轮 function / namespace 工具声明，并在 Chat 回程恢复 namespace 与裸工具名。

边界复审：
- `backend/cmd/server/VERSION` 未被上游修改；继续保留 dev-zz `1.5.1`，上游仍为 `0.1.151`。
- 供应商成本、账号归档、管理员设置原子保存、管理员用量证据 guard、模型自检和普通用户 DTO 均不在本轮变更范围。
- 本轮只更新 `dev-zz-develop`，不提升 `dev-zz`、不打 tag、不发布。

验证：
- `go test ./internal/pkg/apicompat -run 'ToolSearch|AllowedTools|UnrepresentableToolChoice|NamespaceToolChoice|FunctionCustomNameConflict|NamespacedTool(CallStream|NameArrivesLate)|ResponsesRequestTools' -count=1`
- `go test ./internal/pkg/apicompat -count=1`
- `go test -tags=unit ./internal/pkg/apicompat ./internal/service -count=1`
- `make -C backend test-unit`
- `go test ./... -count=1`
- `golangci-lint run --timeout=30m`
- `go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`
- `pnpm --dir frontend run lint:check`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir docs-site run docs:build`
- `git diff --check`、`git diff --cached --check` 和冲突标记扫描。
- 远端 `CI`、`Security Scan`、`dev-zz Branch Images` 在推送最终 head 后检查；运行结果记录在本轮交付报告。

未验证：
- 浏览器人工 smoke。
- 本机 Docker / testcontainers 运行时集成测试；本地只编译 integration 测试二进制，运行由 GitHub Actions integration job 验证。
- 本机既有开发数据库的后端启动仍受先前已诊断的 `174_upstream_cost_pool_defaults.sql` 中间版本 checksum 不一致阻断；该本地数据库历史状态不是本轮上游增量引入，且本轮不修改迁移或数据库。

## 2026-07-10 - 增量合并上游 `main`：ops writer 释放安全与 cache creation usage 补全

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`deff3123`
- 合并前目标：`a3a3bb5f`
- 上游 head：`07fac347`
- 结果提交：本条所在合并提交

上游要点：
- `opsCaptureWriter` 在内部 writer 已释放时，为 Gin `ResponseWriter` 的状态、header、写入、flush、hijack、close-notify 和 HTTP/2 pusher 委托补齐 nil 守卫，避免 compact keepalive 等释放后访问触发 panic。
- Responses → Anthropic 非流式和流式转换保留 `cache_creation_input_tokens`，并从 Responses 总输入中同时扣除 cache read 与 cache creation，恢复 Anthropic `input_tokens` 的非缓存输入语义。
- Anthropic → Responses 非流式和流式转换在总输入中加回 cache creation，同时把该字段显式写入 Responses usage，保证双向转换和后续计费证据不丢缓存写入 token。
- 上游版本从 `0.1.150` 更新到 `0.1.151`。

合并策略：
- 合并前阅读 `branch-policy.md`、`maintenance/merge-main.md`、`patches.md`、`maintenance/merge-log.md`、`changelog.md`、`reference/change-map.md` 和 `testing/verification-matrix.md`，刷新远端后确认 `deff3123` 是当前目标与新上游 head 的 merge base。
- 用 `git merge-tree --write-tree --merge-base deff3123 HEAD origin/main` 做只读预检；预检只发现 `backend/cmd/server/VERSION` 一个文本冲突，真实合并使用 `git merge --no-commit origin/main`。
- 接受上游 ops writer 释放安全和 cache creation usage 正确性修复；版本冲突继续保留 dev-zz `1.5.1`，不采用上游 `0.1.151`。
- 本轮上游增量为 7 个提交、6 个文件，不含数据库迁移、依赖、前端、部署或 workflow 变更，不触碰 dev-zz 的供应商成本、账号归档、模型自检、管理员设置原子保存和用户/admin 字段边界。

冲突文件：
- `backend/cmd/server/VERSION`：保留 dev-zz `1.5.1`。

合并复审修复：
- 上游 nil guard 只覆盖 writer 已释放但尚未被复用的窗口；compact keepalive 会把 `opsCaptureWriter` 包在下游 writer 中，原实现仍可能把这个已逃逸对象放回 `sync.Pool`，导致外层 Logger 读到状态 `0`，并在并发复用时观察到另一请求的 writer。
- ops middleware 现在无条件恢复进入时的原始 writer；只有 `c.Writer` 仍等于自身 wrapper 时才允许回池，下游 wrapper 持有时只重置并退役该对象，避免跨请求复用。
- 已释放 writer 的非空 `Write` / `WriteString` 返回 `io.ErrClosedPipe`，不再用 `(0, nil)` 把丢失写入伪装成成功。
- 新增真实嵌套回归：外层观察 middleware + `OpsErrorLoggerMiddleware` + compact keepalive 连续两次请求，断言外层读到各自真实状态且被下游持有的 writer 不会进入下一请求；race 定向测试通过。

验证：
- `go test ./internal/handler -run 'OpsCaptureWriter|OpsErrorLoggerMiddleware_DoesNotBreakOuterMiddlewares' -count=1`
- `go test -race ./internal/handler -run '^TestOpsErrorLoggerMiddleware_DownstreamWriterDoesNotEscapeIntoPool$' -count=1`
- `go test ./internal/pkg/apicompat -run 'CacheCreation|CacheTokensUseOpenAIInputSemantics|ResponsesEventToAnthropicEvents_TopLevelTerminalUsage' -count=1`
- `make -C backend test-unit`
- `go test ./... -count=1`
- `golangci-lint run --timeout=30m`
- `go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`
- `pnpm --dir frontend run lint:check`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run test:run`
- `pnpm --dir frontend run build`
- `pnpm --dir docs-site run docs:build`
- `git diff --check`、`git diff --cached --check` 和冲突标记扫描。
- 远端 `CI`、`Security Scan`、`dev-zz Branch Images` 在推送最终 head 后检查；运行结果记录在本轮交付报告，避免为了回填运行编号再触发一轮 docs-only 工作流。

未验证：
- 浏览器人工 smoke。
- 本机 Docker / testcontainers 运行时集成测试；本地只编译 integration 测试二进制，运行由 GitHub Actions integration job 验证。
- 本轮不提升 `dev-zz`、不打 tag、不发布。

## 2026-07-10 - 增量合并上游 `main`：用户级 Fast/Flex、Grok reasoning 与 Codex 身份配对

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`6dd3274a`
- 合并前目标：`33c32717`
- 上游 head：`deff3123`
- 结果提交：`838e4094`

上游要点：
- OpenAI Fast / Flex 策略规则新增 `user_ids`，支持先按 API Key 所属 Sub2API 用户匹配专属规则，再回退到全局规则。
- API Key 认证把可信用户 ID 写入请求 context；HTTP、WebSocket 与预取策略路径统一使用该身份，不读取客户端请求体中的用户标识。
- Grok Responses 路径保留 OpenAI-compatible `reasoning_effort`，不再只读取补丁后 body 的 `reasoning.effort`。
- Codex OAuth 上游请求按最终出站 User-Agent 配对 `originator`，校正 override 后的身份错配，并把过低的 `version` 头提升到上游可接受版本。

合并策略：
- 合并前重读 dev-zz 分支策略、合并流程、变更地图和验证矩阵，刷新远程引用后确认 `6dd3274a` 是当前分支与新上游 head 的 merge base。
- 用 `git merge-tree --write-tree --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 做只读预检，再用 `git merge --no-commit origin/main` 执行真实合并。
- 本轮 7 个上游提交、30 个文件自动合入，无文本冲突；随后按大文件拆分回归模式复核管理员设置、用量证据 hydration、认证上下文、OpenAI gateway 和版本边界。
- 接受上游 Fast / Flex 用户范围、Grok reasoning 与 Codex 身份配对实现，不改变 dev-zz 的供应商成本、账号归档、用户/admin DTO 隔离、模型自检和调度证据边界。

冲突文件：
- 无。

边界复审：
- `schedule_strategy`、模型自检 5 项设置和 `disable_keys_on_rate_change` 的 GET、PUT 省略保留、响应与审计检测仍完整存在。
- usage log 关联 hydration 仍只有显式管理员 evidence context 可以解析已删除 API Key 和已归档账号；普通用户查询不会穿透软删除边界。
- 用户专属 Fast / Flex 规则只接受 API Key 认证中间件注入的 `ctxkey.UserID`；规则内只允许正整数且不能重复，用户专属规则优先于全局规则，组内保持配置顺序首条命中。
- Codex 身份配对只收口带 `originator` 的 OAuth 内部接口请求；compat messages bridge 继续不带 `originator`，第三方或不合法身份整体回退到默认官方 Codex CLI 身份。
- `backend/cmd/server/VERSION` 未被本轮上游改动，继续保留 dev-zz `1.5.1`；本轮不提升 `dev-zz`、不打 tag、不发布。

合并后复审修复：
- 管理员保存 Fast / Flex 用户规则时，先在服务层完成规则规范化和校验，再把普通系统设置、认证来源默认值与策略 JSON 合并进同一次 `SetMultiple`；无效 `user_ids` 返回 400 时不再留下已保存但未审计的普通设置。
- Fast / Flex 策略的成功变更进入设置审计字段列表；前端同时拦截非正整数、非整数和单条规则内重复用户 ID。
- zh/en 用户 ID 文案从误放的 `betaPolicy` 移回页面实际读取的 `openaiFastPolicy` 命名空间，并增加 locale 契约测试。
- 大小写变体的 `Codex ` 家族前缀统一恢复为上游大小写敏感校验需要的规范前缀；用户规则白名单 fallback 的终止语义和 WebSocket 建连快照边界已补测试及文档。

验证：
- `go test ./internal/pkg/openai -run '^TestPairCodexClientIdentity$' -count=1`
- `go test ./internal/server/middleware -run '^(TestAPIKeyAuthForwardsUserScopedOpenAIFastPolicyToUpstream|TestAPIKeyAuthSetsGroupContext)$' -count=1`
- `go test -tags=unit ./internal/service -run 'OpenAIFastPolicy|CodexIdentity|GrokResponsesReasoningEffort|OAuthPassthrough_CodexTuiIdentity|OAuthOfficialClientOriginatorCompatibility|WSv2_OAuthOriginatorCompatibility' -count=1`
- `go test -tags=unit ./internal/server ./internal/handler/admin -run '^(TestAPIContracts|TestSettingHandler_UpdateSettings_PreservesOmittedDevZZOperationalSettings|TestDiffSettings_DetectsDevZZOperationalSettingChanges)$' -count=1`
- `go test -tags=unit ./internal/handler/admin -run 'OpenAIFastPolicy|SettingsAuditChanges' -count=1`
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/SettingsView.spec.ts src/i18n/__tests__/localesNoKeyCollision.spec.ts`
- `make -C backend test-unit`
- `go test ./... -count=1`
- `golangci-lint run --timeout=30m`（`0 issues`）
- `go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`
- `make test-frontend`（ESLint、typecheck、6 个测试文件 / 93 个关键用例）
- `pnpm --dir frontend run test:run`（163 个测试文件、1030 个用例）
- `pnpm --dir frontend run build`
- `pnpm --dir docs-site run docs:build`
- `git diff --check`、`git diff --cached --check` 和冲突标记扫描。
- 远端 `CI`、`Security Scan`、`dev-zz Branch Images` 在推送最终 head 后检查；运行结果记录在本轮交付报告，避免为了回填运行编号再触发一轮 docs-only 工作流。

未验证：
- 浏览器人工 smoke。
- 本机 Docker / testcontainers 运行时集成测试；已完成 integration 测试二进制编译，运行由 GitHub Actions integration job 验证。

## 2026-07-10 - 将上游 `main` 合并到 `dev-zz-develop`：GPT-5.6 计费、用量排行与模块拆分合流

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`e8e23425`
- 合并前目标：`9b8d19c9`
- 上游 head：`6dd3274a`
- 结果提交：`a1b8b657`

上游要点：
- GPT-5.6 Responses / Chat Completions 路径补充 reasoning effort、usage、cache write 和计费兼容修复，并继续完善 compact、WebSocket 和失败响应处理。
- API Key 新增最近使用 IP，账号 / Key 列表新增当前并发排序；管理端用量页新增用户 Token 排行。
- 管理端版本提示新增版本回退入口，发布检查、系统 API 与前端交互同步补齐。
- Grok 视频计费补充分辨率、时长与 usage metadata；cyber 失败请求类型迁移同步进入上游 schema。
- `setting_handler`、`admin_service`、`gateway_service`、`antigravity_gateway_service`、`usage_log_repo`、`setting_service` 和前端 i18n 接受上游模块拆分，Go toolchain 同步到 1.26.5。

合并策略：
- 合并前阅读 `branch-policy.md`、`maintenance/merge-main.md`、`maintenance/merge-log.md`、`reference/change-map.md`、`testing/verification-matrix.md`、`patches.md` 和 `changelog.md`，随后刷新 `origin/main` 并用 `git merge-tree --write-tree` 做只读预检。
- 用 `git merge --no-commit origin/main` 展开真实合并，接受上游模块拆分后的文件结构，再把 dev-zz 的账号归档、倍率变更 Key 失效、模型自检、成本优先调度、用量调度证据和供应商页面边界补回拆分文件。
- 用量日志同时保留上游视频字段和 dev-zz `schedule_meta`，所有 INSERT、批处理 CTE、SELECT 与扫描顺序统一维护。
- i18n 接受上游按域拆分的 zh/en 目录，并用独立 dev-zz overlay 深合并二开文案，避免重新制造两个超大单文件。
- 前端继续保留 stone / neutral / emerald 视觉、fork release 链接、用户/admin 字段边界和供应商成本入口，同时吸收上游用户 Token 排行、最近使用 IP、当前并发排序和版本回退。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/usage_log_repo.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/antigravity_gateway_service.go`
- `backend/internal/service/api_key_service.go`
- `backend/internal/service/api_key_service_delete_test.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_messages_chat_fallback.go`
- `backend/internal/service/openai_gateway_usage.go`
- `backend/internal/service/setting_service.go`
- `backend/internal/service/update_service_test.go`
- `backend/migrations/auth_identity_payment_migrations_regression_test.go`
- `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`
- `frontend/src/components/admin/usage/UsageFilters.vue`
- `frontend/src/components/common/VersionBadge.vue`
- `frontend/src/types/index.ts`
- `frontend/src/views/HomeView.vue`
- `frontend/src/views/KeyUsageView.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/admin/UsageView.vue`
- `frontend/src/views/admin/__tests__/UsageView.spec.ts`
- `frontend/src/views/user/KeysView.vue`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`

解决说明：
- `VERSION` 保留 dev-zz `1.5.1` 发布线，不采用上游版本号。
- 后端列表 DTO 同时保留 dev-zz tag / disabled reason / deleted audit 字段和上游 `last_used_ip` / current concurrency 排序能力。
- 上游服务与仓储大文件拆分后，通过 `*_devzz.go` 隔离保留账号归档、owner analytics、模型自检、模型限流配置和成本优先调度，减少未来上游同文件冲突。
- 分组默认倍率、用户专属倍率发生变化时继续按设置停用受影响 Key，并保持更新与停用的事务边界；上游新增的视频分组计费字段不受影响。
- 模型自检探针继续跳过已有 probe guard 覆盖的 Gateway / Antigravity retry、限流写入和账号惩罚分支；Antigravity INTERNAL 500 / credits exhausted 等未覆盖的既存副作用不在本次合并修复范围，后续单独审计。
- `openai_gateway_usage.go` 保留 API Key / OAuth 不同的 cache-read input 口径；messages fallback 保留 dev-zz 转换后 body 与真实 upstream endpoint。
- 管理端用量页合入用户 Token 排行，保留路由时间范围、对象下钻和 dev-zz popover；供应商 tab 继续使用专用“添加供应商”操作与 Modal。
- 版本回退采用上游交互，但仓库与 release 链接固定到 `thornboo/sub2api`，并继续使用 dev-zz 视觉样式。

合并复审修复：
- 恢复 `schedule_strategy`、模型自检 5 项设置和 `disable_keys_on_rate_change` 的 GET、PUT 省略字段保留、PUT 响应与审计变更检测，避免管理员保存无关设置时静默改写 dev-zz 运行配置。
- 恢复 usage log hydration 拆分时遗漏的管理员证据 guard：只有显式管理员 evidence context 才可解析已软删除 API Key 和已归档账号，普通 / 用户侧查询继续不穿透软删除边界。
- 将 `dev-zz-branch-images.yml` 的 Go 版本校验同步到 `go1.26.5`，与 `backend/go.mod`、其他 CI 和镜像构建入口保持一致。
- 合并提交已推送到 `origin/dev-zz-develop`；复审修复完成前不提升 `dev-zz`、不打 tag、不发布。
- 初次推送的 CI `29083279239` 和 dev-zz Branch Images `29083279250` 分别暴露上述设置链路与 Go 版本问题；修复提交推送后以新的远端工作流结果为准。
- 首轮修复提交 `b1d96889` 的 CI `29087638485` 已通过 unit、lint 和前端，但 integration 暴露 usage log hydration guard 漏回；该合并回归随本轮一并修复。

验证：
- `go test ./internal/handler/admin ./internal/handler/dto ./internal/repository ./internal/service ./migrations -run '^$' -count=1`
- `go test ./internal/repository -count=1`
- `go test ./internal/handler/admin ./migrations -count=1`
- `go test ./internal/service -run '^TestGatewayModelSelfCheckProbeExecutorAntigravityForwardPath$' -count=1`
- `go test -tags unit ./internal/repository ./internal/handler/admin ./migrations -count=1`
- `go test -tags unit ./internal/service -count=1`
- `go test ./... -count=1`
- `golangci-lint run --timeout=30m`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`（163 个测试文件、1026 个用例）
- `pnpm --dir frontend build`
- `pnpm --dir docs-site build`
- `git diff --check`
- `rg -n '^(<<<<<<<|=======|>>>>>>>)' backend frontend docs-site`

复审修复验证：
- `go test -tags=unit ./internal/server -run '^TestAPIContracts$' -count=1`
- `go test -tags=unit ./internal/handler/admin -run '^(TestSettingHandler_UpdateSettings_PreservesOmittedDevZZOperationalSettings|TestDiffSettings_DetectsDevZZOperationalSettingChanges)$' -count=1`
- `go test -tags=unit ./internal/repository -count=1`
- `go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`（编译 integration 测试二进制，不启动 testcontainers）
- `make -C backend test-unit`
- `golangci-lint run --timeout=30m`（`0 issues`）
- `make test-frontend`（lint、typecheck、91 条关键 Vitest）
- `pnpm --dir frontend run test:run`（163 个测试文件、1026 个用例）
- `pnpm --dir frontend run build`
- `pnpm --dir docs-site run docs:build`
- 本机尝试 `make -C backend test-integration`，非容器包通过，repository testcontainers 因本机无 Docker 以 `rootless Docker not found` 退出；该项由修复提交对应的 GitHub Actions integration job 完成验证。

未验证：
- 浏览器人工 smoke。
- Docker / testcontainers 集成测试。

## 2026-07-08 - 将上游 `main` 合并到 `dev-zz-develop`：批量生图、网关拆分与 Chat Completions 回退合流

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`67e945f8`
- 合并前目标：`ff3b347f`
- 上游 head：`e8e23425`
- 结果提交：`eeb45334`
- 补充修复：`a714ddab`
- 发布提交：本条所在提交
- 发布标签：`v1.4.10`

上游要点：
- 批量生图 MVP：新增 batch image ent/schema/migrations/repository/service/handler、分组生图权限、用户冻结余额、批量生图用户入口和指南页。
- OpenAI Responses / Chat Completions fallback 拆分共享 CC 管线，补充 messages fallback、非流式 / 流式测试、GLM reasoning effort 归一化和错误处理。
- OpenAI、Grok、Responses、Chat Completions 路径新增 prompt / function-call / video-text / web-search / image namespace 等兼容修复。
- 网关 Anthropic passthrough 和 Bedrock 逻辑从 `gateway_service.go` 拆出到独立文件。
- 管理端分组、套餐和设置补充批量生图 pricing、gate、hold ratio、下载与用户删除相关配置。
- 安全扫描 `xlsx` exception 到期日刷新，并补充 README、部署和赞助商镜像同步。

合并策略：
- 合并前阅读 `docs-site/dev-zz/branch-policy.md`、`maintenance/merge-main.md`、`maintenance/merge-log.md`、`reference/change-map.md`、`testing/verification-matrix.md`、`patches.md`、`api-surface.md`、`configuration-and-migrations.md` 和 `changelog.md`。
- 用 `git fetch origin` 刷新远程引用，用 `git merge-tree --write-tree --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 做只读预检，再用 `git merge --no-commit origin/main` 执行真实合并。
- 接受上游拆分后的网关文件结构，移除旧大文件重复块；再把 dev-zz 的 context-aware retry、OpenAI usage cache-read 口径、ScheduleMeta、UpstreamEndpoint 和 messages 后置 fallback 顺序补回新文件。
- messages API-key force Chat Completions fallback 保持 dev-zz 的后置路径，避免绕过 prompt cache、Claude Code todo guard、fast policy、Grok patch、billing model 和 upstream model 归一化。
- 余额 quick action、modal 和 header 冲突保留 dev-zz stone / emerald UI，合入上游批量生图入口、余额 tooltip 和 focus-visible 细节。
- `xlsx` audit exception 保留 dev-zz “仅导出、不解析用户上传 XLSX”的风险说明，并采用上游更晚到期日。
- 合并后修正 rate-limit 顺序回归：5xx 显式 temp-unsched 规则优先于通用模型级上游失败，非模型级 4xx / 429 仍保留账号自定义 temp-unsched 兜底，404 / model_not_found 仍保持模型级冷却，Anthropic 429 官方窗口仍优先于 temp-unsched。

冲突文件：
- `.github/audit-exceptions.yml`
- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/repository/migrations_runner.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_messages_chat_fallback.go`
- `backend/internal/service/openai_gateway_responses_chat_fallback.go`
- `backend/internal/service/openai_gateway_service.go`
- `frontend/src/components/common/BaseDialog.vue`
- `frontend/src/components/layout/AppHeader.vue`
- `frontend/src/components/user/dashboard/UserDashboardQuickActions.vue`

解决说明：
- `wire_gen.go` 同时注入上游 `BatchImageCleanupService`、`BatchImageWorkerRuntime` 和 dev-zz `ModelSelfCheckRunner`。
- `migrations_runner.go` 保留 `159_create_upstream_recharge_records.sql`、`159_batch_image_foundation.sql`、`161_batch_image_pricing_snapshot.sql` 三条 checksum 兼容记录。
- `gateway_service.go` 采用上游拆分结构，并在 `gateway_anthropic_passthrough.go`、`gateway_bedrock.go`、`gateway_service.go` 统一保留 model self-check probe 不触发上游重试的 guard。
- `openai_gateway_usage.go` 保留 ScheduleMeta 透传和 OpenAI cache-read token 是否计入 input 的 dev-zz 口径。
- `openai_gateway_messages_chat_fallback.go` / `openai_gateway_responses_chat_fallback.go` 使用上游共享 CC fallback 管线，同时补齐 `UpstreamEndpoint: "/v1/chat/completions"`。
- `openai_gateway_messages.go` 去掉上游前置 force-CC fallback，保留 dev-zz 后置 fallback，确保请求先经过转换、策略和计费归一化。
- `BaseDialog.vue`、`AppHeader.vue` 和 `UserDashboardQuickActions.vue` 保留 dev-zz 视觉边界，同时合入上游批量生图入口和可访问性细节。
- `openai_gateway_messages_chat_fallback_test.go` 的非流式 body 断言改为搜索 user message，因为 dev-zz 转换路径会在用户消息前注入 Claude Code todo guard。
- `ratelimit_service.go` 将 5xx temp-unsched 规则提前到通用模型级失败之前，修复 502 非 JSON 响应被错误写成模型冷却的问题；同时补回模型级处理之后的非 401 temp-unsched 兜底，避免 403 等账号自定义规则被静默跳过，并通过 403 / 404 / Anthropic 429 回归测试确认边界。

验证：
- `gofmt -w backend/internal/service/gateway_service.go backend/internal/service/gateway_anthropic_passthrough.go backend/internal/service/gateway_bedrock.go backend/internal/service/openai_gateway_usage.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_messages.go backend/internal/service/openai_gateway_messages_chat_fallback.go backend/internal/service/openai_gateway_responses_chat_fallback.go backend/internal/service/ratelimit_service.go backend/cmd/server/wire_gen.go`
- `rg -n "^(<<<<<<< .+|=======|>>>>>>> .+)$" .`
- `git diff --check`
- `go test -tags unit ./internal/service -run 'ForceChatCompletions|RecordUsage|OpenAIAPIKeyDefaultIncludesCacheRead|OpenAIOAuthIgnoresSeparatedCacheUsageMode|ScheduleMeta|ModelSelfCheck' -count=1`
- `go test -tags unit ./internal/repository -run 'MigrationChecksumCompatibility|IsMigrationChecksumCompatible' -count=1`
- `go test -tags unit ./internal/service -run 'Custom403TempUnschedulableRule|OpenAI403|HandleUpstreamError_ModelNotFound|HandleUpstreamError_Bare404|NonJSON2xxMatchesTempUnschedulableRule|HandleUpstreamError_AnthropicWindowLimitPreemptsTempUnschedRule|HandleModelScopedFailure' -count=1`
- `go test -tags unit ./internal/handler ./internal/server ./internal/repository ./internal/service -count=1`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir docs-site docs:build`

未验证：
- 未运行浏览器人工 smoke。
- 未运行完整前端测试套件。

## 2026-07-07 - 账号管理供应商入口简化

分支：
- 目标：`dev-zz`
- 来源：当前工作区
- 发布提交：本条所在提交
- 发布标签：待后续 patch release 确定

发布要点：
- 管理端账号页第三个标签从「供应商成本」改为「供应商」，让供应商新增和供应商级充值记录集中在供应商 tab。
- 账号编辑弹窗只保留供应商选择，支持清空供应商绑定，不再承担新增供应商或高级成本 / Key 配额查询配置。
- 创建账号弹窗同步移除历史高级成本 / Key 配额查询配置，避免创建和编辑表单能力不一致。

合并策略：
- 不修改供应商、资金池、充值账本和成本快照后端语义。
- 不改变账号列表供应商成本列、排序口径、调度逻辑或普通用户侧返回字段。
- 保留已有历史 `extra` 字段，不做数据迁移；如后续恢复余额查询，应在供应商或资金池级入口重新设计。

验证：
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `cd backend && go test ./internal/service`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

未验证：
- 浏览器人工 smoke。

## 2026-07-07 - 将 Security Scan exception follow-up 提升到 `dev-zz`

分支：
- 目标：`dev-zz`
- 来源：`dev-zz-develop`
- 合并前目标：`edb14865`
- 发布提交：本条所在提交
- 发布标签：`v1.4.9`

发布要点：
- `v1.4.8` release workflow 已成功发布镜像和 release 产物，但 Security Scan 因 `xlsx` audit exception 在 `2026-07-06` 到期而失败。
- 本次只刷新 `xlsx` 两个 high advisory 的例外说明与到期日，保留“只导出、不解析用户上传 XLSX”的风险接受前提。
- `v1.4.9` 作为 CI follow-up patch，覆盖 `v1.4.8` 的红色 Security Scan 状态。

合并策略：
- 不同步新的上游 `main`。
- 不修改业务代码、不调整供应商成本排序行为。
- 先在 `dev-zz-develop` 提交 security metadata 和版本记录，再快进 `dev-zz` 并打 `v1.4.9`。

验证：
- `python tools/check_pnpm_audit_exceptions.py --audit frontend/audit.json --exceptions .github/audit-exceptions.yml`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

未验证：
- 未替换 `xlsx` 依赖。
- 浏览器人工 smoke。

## 2026-07-07 - 将 `dev-zz-develop` 供应商成本排序修正提升到 `dev-zz`

分支：
- 目标：`dev-zz`
- 来源：`dev-zz-develop`
- 合并前目标：`cde90d58`
- 功能提交：`ee2a8d20`
- 发布提交：本条所在提交
- 发布标签：`v1.4.8`

发布要点：
- 账号列表供应商成本列移到「分组」列后方。
- 「综合折扣」和「倍率」按页面实际展示口径支持服务端排序。
- 成本对比页保持供应商视角，继续作为供应商级充值记录入口。
- 补齐 `changelog.md` 和 `patches.md` 记录后再提升到 `dev-zz` 并发布 patch tag。

合并策略：
- 以 `dev-zz-develop` 的供应商成本排序提交作为来源，先补本文档、补丁记录、变更记录和版本号。
- `dev-zz` 与 `dev-zz-develop` 保持同一发布提交后打 `v1.4.8`。
- 只发布 dev-zz 二开 patch，不同步新的上游 `main`。

验证：
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `go test ./internal/repository`
- `golangci-lint run --timeout=30m`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

未验证：
- Docker / testcontainers 依赖的数据库集成排序用例未能在本机运行；本地环境报 `rootless Docker not found`。
- 浏览器人工 smoke。

## 2026-07-07 - 将上游 `main` 合并到 `dev-zz-develop`：供应商成本口径与上游调度/错误请求能力合流

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`a632cb00`
- 合并前目标：`e9079d92`
- 上游 head：`67e945f8`
- 结果提交：本次合并提交

上游要点：
- API Key 账号新增 OpenAI/Anthropic 请求头覆写能力，并补充覆写审计修复。
- OpenAI 新模型 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna` 进入模型常量和前后端展示。
- OpenAI 高级调度器新增管理端控制、调度评分展示和审计修复。
- 用量/错误请求页新增错误列设置、排序、IP 地理信息批量查询，以及 CSV BOM 修复。
- Anthropic Fable `7d_oi` 限流按模型级窗口处理，避免误伤账号其他模型。
- 支付侧新增 EasyPay 自定义支付方式、CNY 换算显式 opt-in、内置支付方法精确匹配等修复。
- 部署示例和文档吸收上游安全默认值、README、赞助商和版本同步更新。

合并策略：
- 合并前阅读 `docs-site/dev-zz/branch-policy.md`、`maintenance/merge-main.md`、`patches.md`、`reference/change-map.md`、`testing/verification-matrix.md`、`maintenance/merge-log.md` 和 `changelog.md`。
- 用 `git fetch origin` 刷新远程引用，以上游 `origin/main` 的 `67e945f8` 作为合并目标。
- 用 `git merge-tree --write-tree --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 只读预检，确认账号、用量、ops、i18n、wire、API Key 等冲突范围。
- 用 `git merge --no-commit origin/main` 执行真实合并。
- 保留 dev-zz 的供应商优先成本口径：账号编辑只绑定供应商，充值记录和成本对比仍按供应商聚合；账号列表继续展示综合折扣、充值/汇率、倍率。
- 接受上游调度评分、请求头覆写、错误请求 DataTable/IP 地理信息、支付和模型常量更新；冲突处按“保留 dev-zz 产品边界，合入上游新增能力”解决。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/account_handler_list_test.go`
- `backend/internal/handler/admin/admin_service_stub_test.go`
- `backend/internal/handler/admin/ops_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/service/api_key_service.go`
- `backend/internal/service/ratelimit_service_anthropic_window_limit_test.go`
- `backend/internal/service/wire.go`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/admin/usage/UsageFilters.vue`
- `frontend/src/components/common/DataTable.vue`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/UsageView.vue`
- `frontend/src/views/admin/ops/components/OpsErrorDetailsModal.vue`
- `frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`
- `frontend/src/views/user/UsageView.vue`

解决说明：
- `ProvideAPIKeyService` 同时注入 `SettingService` 和 `ConcurrencyService`，保留 dev-zz 的 Key 标签/删除态映射，并合入上游当前并发数。
- 账号列表 handler/repository 同时保留 dev-zz 的归档/过滤路径和上游调度评分过滤池，调度评分不受分页截断。
- Anthropic 429 专用窗口优先于通用 model-scoped failure：5h/7d 仍账号级，`7d_oi` 仅 Fable 模型级，其余 Anthropic 429 维持旧账号级限流。
- `EditAccountModal` 保留供应商绑定 UI 和高级成本/配额查询分区，同时把上游 setup-token/Grok OAuth 模型映射范围合入。
- 管理端账号列表保留供应商成本三列，同时合入调度分数类型和相关展示。
- 管理端/用户用量页保留 dev-zz 的对象筛选和时间参数，同时合入错误请求模式、列设置、IP 地理信息和排序。
- `OpsErrorLogTable` 统一采用上游 `DataTable` 结构，保留用户/Key/账号归因和 dev-zz 详情弹窗的 error type 传递。
- 中英文 i18n 同时保留用量分析 `analytics` 文案和上游 `ipGeo` 文案。

验证：
- `gofmt -w backend/cmd/server/wire_gen.go backend/internal/handler/admin/account_handler.go backend/internal/handler/admin/account_handler_list_test.go backend/internal/handler/admin/admin_service_stub_test.go backend/internal/handler/admin/ops_handler.go backend/internal/handler/dto/mappers.go backend/internal/repository/account_repo.go backend/internal/service/api_key_service.go backend/internal/service/ratelimit_service.go backend/internal/service/ratelimit_service_anthropic_window_limit_test.go backend/internal/service/wire.go`
- `rg -n "^(<<<<<<< .+|=======|>>>>>>> .+)$"`
- `git diff --check`
- `go test -tags unit ./internal/handler/admin ./internal/handler/dto ./internal/repository ./internal/service -run 'TestAccountHandler|TestHandleUpstreamError_Anthropic|TestAPIKey|TestNonExistent'`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/views/admin/__tests__/AccountsView.schedulerScore.spec.ts src/views/user/__tests__/UsageView.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts src/components/common/__tests__/DataTable.spec.ts`
- `pnpm --dir docs-site docs:build`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-07-02 - 将上游 `main` 合并到 `dev-zz-develop`：分组高峰倍率、订阅计费透传与可用渠道展示

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`7dc7cfce`
- 合并前目标：`24df52c3`
- 上游 head：`a632cb00`
- 结果提交：本次合并提交

上游要点：
- 订阅分组新增高峰时段倍率配置：`peak_rate_enabled`、`peak_start`、`peak_end`、`peak_rate_multiplier`。
- 高峰倍率全链路透传到 group DTO、API Key auth cache、计费服务、OpenAI / generic gateway 用量记录、订阅套餐和可用渠道展示。
- 高峰倍率只叠加到 token 计费倍率；token 模式下的图片 token 同样受影响，图片按次倍率不受高峰倍率影响。
- 管理端分组页新增高峰时段配置和校验；用户侧可用渠道、订阅计划与 Key 相关展示会显示高峰倍率提示。
- 新增迁移 `158_add_group_peak_rate_multiplier.sql`；本分支迁移目录已有同号并存惯例，本次按文件名直接吸收，未顺延。

合并策略：
- 合并前阅读 `docs-site/dev-zz/index.md`、`maintenance/merge-log.md`、`branch-policy.md`、`maintenance/merge-main.md`、`testing/verification-matrix.md`、`patches.md` 和 `changelog.md`。
- 用 `git fetch origin` 刷新远程引用，以上游 `origin/main` 的 `a632cb00` 作为合并目标。
- 用 `git merge-tree --write-tree --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 只读预检，预测到 1 个内容冲突。
- 用 `git merge --no-commit origin/main` 执行真实合并。
- 接受上游分组高峰倍率 schema/API/UI/计费链路；保留 dev-zz 的 docs-site 文档中心、fork release/镜像策略、用户/admin 用量字段边界、账号归档语义和模型自检状态快照。

冲突文件：
- `backend/internal/service/openai_gateway_record_usage_test.go`

解决说明：
- `openai_gateway_record_usage_test.go` 同时保留 dev-zz 的 OpenAI API Key cache token 口径测试和上游新增的高峰倍率 token-mode 图片输出 token 计费测试。
- 高峰倍率字段沿用上游语义：仅订阅分组可启用，时间格式为 `HH:MM`，区间为同日左闭右开 `[peak_start, peak_end)`，不支持跨天，`peak_rate_multiplier=0` 允许作为高峰 token 免费/折扣策略。
- dev-zz 用户/admin 用量边界不变：用户侧仍只展示自己的实际扣费和公开分组/模型信息，不暴露上游账号、渠道、内部成本或管理员字段。
- 迁移编号 `158` 与既有 `158_add_usage_log_schedule_meta.sql` 并存；本分支此前已允许上游同号迁移按文件名并存，未做重编号。

验证：
- `gofmt -w backend/internal/service/openai_gateway_record_usage_test.go`
- `rg -n "^(<<<<<<< .+|=======|>>>>>>> .+)$" .`
- `git diff --check`
- `mise x -C backend -- go test -tags unit ./internal/service -run 'PeakRate|CacheUsageMode|OpenAIAPIKeyDefaultIncludesCacheRead|OpenAIOAuthIgnoresSeparatedCacheUsageMode' -count=1`
- `mise x -C backend -- go build ./...`
- `mise x -C backend -- go test -tags unit ./migrations -count=1`
- `mise x -C backend -- go test -tags unit ./internal/server ./internal/handler ./internal/handler/admin ./internal/config ./internal/repository ./internal/service ./internal/pkg/openai ./internal/pkg/apicompat ./internal/pkg/xai -count=1`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir docs-site docs:build`

未验证：
- 浏览器人工 smoke。
- 完整前端测试套件和完整仓库级 `go test ./...`。

## 2026-07-02 - 将上游 `main` 合并到 `dev-zz-develop`：Spark shadow、Grok media、用量快照与支付/认证修复

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`c99112a9`
- 合并前目标：`925a5db3`
- 上游 head：`7dc7cfce`
- 结果提交：本次合并提交

上游要点：
- 新增 Spark shadow 账号体系：账号 schema、父子账号展示、调度跳过 shadow 凭据、Spark 窗口配额、账号测试与前端账号操作入口。
- 新增 Grok media / xAI media 路由、OpenAI-compatible Grok 请求处理、`/count_tokens` 兼容路径和相关网关测试。
- 用户用量页吸收 dashboard snapshot-v2、`billing_mode`、`request_type`、reasoning intensity、用户用量图表与导出修复。
- 修复支付 refund pending / resume、OAuth 邮箱补全、隐私 toast、risk-control matched keyword、订阅撤销缓存、dateline fingerprint 归一化和 GPT-5.5 / Codex 相关逻辑。
- 更新 README、多语言资料、合作方 logo、Docker/deploy 脚本和 fork/upstream 版本同步工具。
- 新增迁移 `154_account_spark_shadow.sql`、`154a_account_spark_shadow_indexes_notx.sql`、`156_content_moderation_matched_keyword.sql`、`157_user_platform_quotas_add_grok.sql`；本分支迁移目录已有同号并存惯例，本次按文件名直接吸收，未顺延。

合并策略：
- 合并前阅读 `docs-site/dev-zz/branch-policy.md`、`maintenance/merge-main.md`、`reference/change-map.md`、`changelog.md`、`patches.md`、`maintenance/merge-log.md` 和 `testing/verification-matrix.md`。
- 用 `git fetch origin` 刷新远程引用，以上游 `origin/main` 的 `7dc7cfce` 作为合并目标。
- 用 `git merge-tree --write-tree --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 只读预检，预测到内容冲突。
- 用 `git merge --no-commit origin/main` 执行真实合并。
- 接受上游后端正确性、Spark shadow、Grok media、payment/refund、OAuth、risk-control、dateline、count_tokens、dashboard snapshot-v2 与前端用量增强；保留 dev-zz 的 `1.4.1` 发布线、docs-site 文档中心、stone / emerald 二开主题、账号归档语义、模型自检状态快照、用户/admin 用量字段边界和 fork release 链接策略。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/dto/mappers_usage_test.go`
- `backend/internal/handler/usage_handler.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/repository/usage_log_repo.go`
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/usage_service.go`
- `frontend/src/api/usage.ts`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/admin/channel/IntervalRow.vue`
- `frontend/src/components/admin/channel/PricingEntryCard.vue`
- `frontend/src/components/admin/usage/UsageStatsCards.vue`
- `frontend/src/components/admin/usage/UsageTable.vue`
- `frontend/src/components/charts/EndpointDistributionChart.vue`
- `frontend/src/components/charts/GroupDistributionChart.vue`
- `frontend/src/components/charts/ModelDistributionChart.vue`
- `frontend/src/components/charts/__tests__/GroupDistributionChart.spec.ts`
- `frontend/src/components/common/DataTable.vue`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/ChannelsView.vue`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/admin/ops/components/OpsSystemLogTable.vue`
- `frontend/src/views/user/UsageView.vue`

解决说明：
- `backend/cmd/server/VERSION` 保留 dev-zz 发布线 `1.4.1`，不采用上游 `0.1.142`。
- `account_handler.go` 同时保留 dev-zz ETag / 归档列表过滤和上游 Spark shadow parent enrichment。
- `account_repo.go` 吸收上游 `Count` 使用 `Clone()` 的修复，避免列表计数污染主查询。
- `usage_handler.go` 同时吸收上游 `billing_mode`、dashboard snapshot-v2、request type 和模型来源过滤，并保留 dev-zz 用户域安全边界：用户 `/usage/dashboard/models` 与 snapshot-v2 模型列表继续返回脱敏 DTO，不返回 `cost` / `account_cost`。
- `usage_log_repo.go` 同时保留 dev-zz owner analytics 与上游 `billing_mode` 聚合快路径判断。
- `ratelimit_service.go` 的 401 分支吸收上游 `authAccount` 处理，同时保留 dev-zz 可故障转移语义。
- `openai_gateway_messages.go` 同时保留 `openai_compat` 与 xAI/Grok media 依赖。
- 前端账号编辑弹窗保留 dev-zz 模型映射模式与上游 Spark shadow credentials 语义；Spark shadow 提交时只发送模型映射凭据。
- 管理端账号页保留归档/恢复语义和 disabled 前置，同时吸收 Spark shadow 创建/更新/删除入口。
- 用量图表保留 dev-zz 排名列、stone / emerald 主题和用户安全展示，同时吸收上游 breakdown、`showAccountCost`、snapshot-v2 和 `billing_mode` 支持；用户模型分布关闭 Standard / Account Cost 列，避免显示未返回的管理员字段。
- 运维系统日志表保留 dev-zz 确认弹窗和主题，同时吸收上游新增筛选字段与 i18n。

验证：
- `gofmt -w backend/internal/handler/admin/account_handler.go backend/internal/handler/dto/mappers_usage_test.go backend/internal/handler/usage_handler.go backend/internal/handler/usage_handler_request_type_test.go backend/internal/repository/account_repo.go backend/internal/repository/usage_log_repo.go backend/internal/service/openai_gateway_messages.go backend/internal/service/ratelimit_service.go backend/internal/service/usage_service.go`
- `rg -n "^(<<<<<<<|>>>>>>>|=======$)" .`
- `git diff --check`
- `git diff --cached --check`
- `mise x -C backend -- go build ./...`
- `mise x -C backend -- go test -tags unit ./migrations`
- `mise x -C backend -- go test -tags unit ./internal/server ./internal/handler ./internal/handler/admin ./internal/config ./internal/repository ./internal/service ./internal/pkg/openai ./internal/pkg/apicompat ./internal/pkg/xai`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/common/__tests__/DataTable.spec.ts src/components/charts/__tests__/GroupDistributionChart.spec.ts src/components/charts/__tests__/ModelDistributionChart.spec.ts src/views/user/__tests__/UsageView.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/views/admin/__tests__/AccountsView.sparkShadow.spec.ts src/views/admin/ops/components/__tests__/OpsSystemLogTable.spec.ts`
- `pnpm --dir docs-site docs:build`

未验证：
- 浏览器人工 smoke。
- 完整前端测试套件和完整仓库级 `go test ./...`。

## 2026-06-29 - 将上游 `main` 合并到 `dev-zz-develop`：Grok 订阅、Codex 检测加固、系统日志 Key 筛选与支付修复

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`ce6af413`
- 合并前目标：`76c78835`
- 上游 head：`c99112a9`
- 结果提交：本次合并提交

上游要点：
- 新增 Grok / xAI OAuth、订阅配额探测、账号刷新、网关转发与管理端授权入口。
- 加固 OpenAI Codex / ChatGPT 账号检测：新增 PAT auth mode、app-server client 识别、engine fingerprint 统一信号和 Codex 白名单设置。
- 运维系统日志新增 `api_key_id` 持久列、筛选条件和索引，便于按 Key 排查生产日志。
- 用户 API Key 页面新增列设置；管理员账号表、设置页、Grok 配额探测、支付订单金额和二维码弹窗获得多处修复。
- OpenAI 用量后扣保留请求期解析出的 quota platform，避免 worker 池背景上下文丢失 ForcePlatform；无可用账号时返回更精确的 `model_not_found`。
- 修复 Responses / Chat Completions 兼容路径中的工具 schema、passthrough function args、图片 bridge `tool_choice`、overloaded 错误识别和 token refresh 非重试错误。
- 更新 sponsor 资料、合作方 logo、README 多语言内容和 `sub2api-admin` 技能说明。

合并策略：
- 合并前阅读 `docs-site/dev-zz/branch-policy.md`、`maintenance/merge-main.md`、`patches.md`、`maintenance/merge-log.md`、`changelog.md`、`reference/change-map.md` 和 `testing/verification-matrix.md`。
- 用 `git fetch origin` 刷新远程引用，以上游 `origin/main` 的 `c99112a9` 作为合并目标。
- 用 `git merge-tree --write-tree --merge-base ce6af413577a6d012e334baad5069a02a80d48b6 HEAD origin/main` 只读预检，预测到 12 个内容冲突。
- 用 `git merge --no-commit origin/main` 执行真实合并，冲突文件与预检一致。
- 接受上游后端正确性、Grok 支持、支付修复、Codex 检测加固、系统日志 Key 筛选和用户 Key 列设置；保留 dev-zz 的发布版本号、docs-site 文档中心、stone / emerald 视觉方向、企业 Key 标签/批量/用量下钻语义、模型自检 runner 和 OpenAI usage 真实 result endpoint 口径。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/service/account.go`
- `backend/internal/service/openai_gateway_service.go`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/utils/billingMode.ts`
- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/views/admin/ops/components/OpsSystemLogTable.vue`
- `frontend/src/views/user/KeysView.vue`
- `frontend/src/views/user/UsageView.vue`

解决说明：
- `backend/cmd/server/VERSION` 的 base 为 `0.1.138`，dev-zz-develop 为 `1.4.0`，上游为 `0.1.139`；按 dev-zz 发布线保留 `1.4.0`。
- `wire_gen.go` 和 `provideCleanup` 同时保留上游 `grokOAuthService` 与 dev-zz `modelSelfCheckRunner`。
- `openai_gateway_handler.go` 继续使用 `openAIUsageUpstreamEndpoint(c, account, result)`，保留真实转发结果中的上游端点；同时提前解析并传入 `QuotaPlatform`，保证异步后扣平台口径。
- `service/account.go` 同时保留 dev-zz cache token usage mode 与上游 OpenAI PAT auth mode。
- `OpenAIRecordUsageInput` 同时保留 dev-zz `ScheduleMeta` 与上游 `QuotaPlatform`。
- 账号创建/编辑弹窗保留 dev-zz 边框和暗色视觉，同时接入上游 Grok OAuth 模型映射和 Antigravity project ID 写入逻辑。
- `billingMode.ts` 保留 dev-zz 的 `isImageUsage()` 图片用量识别口径，避免只凭空 `billing_mode` 判断导致图片记录被误归类。
- `DashboardView.vue` 接受上游数值归零保护，避免旧统计快照缺字段时显示 `NaN`。
- `OpsSystemLogTable.vue` 保留 dev-zz 二次确认清理弹窗，并把上游 `api_key_id` 筛选接入查询、清理 payload 和确认摘要。
- `KeysView.vue` 同时保留 dev-zz 标签、批量创建/批量操作、单 Key 用量下钻和系统状态保护，并接入上游列设置下拉。
- `UsageView.vue` 保留 dev-zz stone 文案样式和图片优先展示结构，避免重复插入上游灰色 token 单价区块。
- 上游新增 `154/155` 系统日志 Key 迁移与 dev-zz 既有 `154/155` 撞号，已顺延为 `162_add_ops_system_logs_api_key_id.sql` 和 `163_add_ops_system_logs_api_key_id_index_notx.sql`。

验证：
- `gofmt -w backend/cmd/server/wire_gen.go backend/internal/handler/openai_gateway_handler.go backend/internal/service/account.go backend/internal/service/openai_gateway_service.go`
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|>>>>>>>|=======$)" .`
- `mise x -C backend -- go test ./migrations`
- `mise x -C backend -- go test ./internal/server ./internal/handler ./internal/handler/admin ./internal/config ./internal/repository ./internal/service ./internal/pkg/openai ./internal/pkg/apicompat ./internal/pkg/xai`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/views/user/__tests__/KeysView.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts src/views/admin/__tests__/SettingsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/PaymentQRDialog.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts`
- `pnpm --dir docs-site docs:build`

未验证：
- 浏览器人工 smoke。
- 完整前端测试套件和完整仓库级 `go test ./...`。

## 2026-06-26 - 将上游 `main` 合并到 `dev-zz-develop`：GPT-5.5 codex、codex spark 502 修复与 OpenAI 周限重置确认

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`85a3b122`
- 合并前目标：`b6791ba2`
- 上游 head：`ce6af413`
- 结果提交：本次合并提交

上游要点：
- 新增 GPT-5.5 codex instructions（`instructions_gpt5_5.txt`），并作为 codex 最新 instructions 的 fallback。
- 修复 codex spark 路径：剥离 `image_generation` 工具，修复上游 502。
- 管理端账号「重置 OpenAI 周限」操作增加二次确认（`OpenAIQuotaResetCell.vue`）。
- 更新 sponsor 资料与合作方 logo（byteplus / huoshan），新增 `README_CN.md`，README 多语言更新。

合并策略：
- 合并前阅读 `branch-policy.md`、`maintenance/merge-main.md`、`maintenance/merge-log.md`、`patches.md`、`changelog.md`、`reference/change-map.md`、`testing/verification-matrix.md`。
- `git fetch origin` 后以 `origin/main`（`ce6af413`）为合并目标。
- `git merge-tree --write-tree HEAD origin/main` 只读预检：未预测到冲突。
- `git merge --no-commit --no-ff origin/main` 自动合并成功，无冲突文件。
- 上游本批为 OpenAI codex/gpt-5.5、ws forwarder、管理端确认弹窗、i18n 与 README/资源更新，均不触及 dev-zz 已记录策略（视觉、认证入口、数据保留、用量字段边界、部署线），全部按上游接受。

冲突文件：
- 无（自动合并干净）。

验证：
- `grep -rnE '^(<<<<<<<|=======|>>>>>>>)$'`（无标记）
- `git diff --check`（仅上游新增 `README_CN.md` 的 markdown 行尾空格告警，属上游内容，未改写）
- `pnpm --dir frontend typecheck`、`pnpm --dir frontend lint:check`
- `mise x -C backend -- go build ./...`、`go test ./internal/server ./internal/handler ./internal/config ./internal/service ./internal/pkg/openai`

未验证：
- 未运行 `pnpm --dir docs-site docs:build` 与完整前后端测试套件（镜像/构建由维护者本地执行）。



分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`945b9b20`
- 合并前目标：`2fa893bf`
- 上游 head：`85a3b122`
- 结果提交：本次合并提交

上游要点：
- 管理端 usage 统计卡片新增缓存 Token 总量提示，可查看缓存创建与缓存读取拆分。
- 新增账号调度“优先选择最早重置账号”能力，用于 rate-limit reset 场景的可选调度策略。
- 修复 OpenAI 图片 `response.incomplete` 软失败识别与故障转移记录。
- 修复 Gemini / Vertex Anthropic 兼容路径中的不支持 schema 字段和 `anthropic-beta` 过滤。
- 更新 Claude Code / CC Switch 识别逻辑与默认模型，识别新的 IDE entrypoint 和新版 CLI billing block。
- 新增订阅支付 affiliate rebate，允许清空 promo code 过期时间。
- 部署 compose bind mount 增加 SELinux `:Z` 标记，CI/CLA workflow 补充 Node 24 actions runtime 相关更新。
- 更新 sponsor 资料和合作方 logo。

合并策略：
- 合并前阅读 `docs-site/dev-zz/branch-policy.md`、`docs-site/dev-zz/maintenance/merge-main.md`、`docs-site/dev-zz/maintenance/merge-log.md`、`docs-site/dev-zz/patches.md`、`docs-site/dev-zz/changelog.md`、`docs-site/dev-zz/reference/change-map.md` 和 `docs-site/dev-zz/testing/verification-matrix.md`。
- 当前存在未提交的 new-api 缓存 Token 口径修复，合并前用 `git stash push -u -m "wip new-api cache token usage before main merge"` 暂存保护。
- 用 `git fetch origin` 刷新远程引用，以上游 `origin/main` 的 `85a3b122` 作为合并目标。
- 本机 Git 需要使用 `git merge-tree --write-tree --merge-base "$(git merge-base HEAD origin/main)" HEAD origin/main` 预检；预检预测到三处内容冲突。
- 用 `git merge --no-commit origin/main` 把上游 `main` 合并进 `dev-zz-develop`。
- 接受上游后端正确性、调度、Claude Code 识别、Gemini/Vertex 兼容、支付 rebate、缓存 Token 展示和部署 SELinux 修复；保留 dev-zz 的发布版本号、docs-site 文档中心、stone / emerald 管理端视觉方向，以及 OpenAI usage 上游端点记录的真实 result endpoint 口径。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/internal/handler/openai_gateway_handler.go`
- `frontend/src/components/admin/usage/UsageStatsCards.vue`

解决说明：
- `backend/cmd/server/VERSION` 的 base 为 `0.1.137`，dev-zz-develop 为 `1.2.1`，上游为 `0.1.138`；按 dev-zz 发布线保留 `1.2.1`。
- `backend/internal/handler/openai_gateway_handler.go` 三处 usage 记录端点冲突保留 `openAIUsageUpstreamEndpoint(c, account, result)`，继续优先使用 `OpenAIForwardResult.UpstreamEndpoint`，避免 chat-only API Key fallback 被错误记录为 `/v1/responses`。
- `frontend/src/components/admin/usage/UsageStatsCards.vue` 吸收上游缓存 Token 明细 tooltip，同时保留 dev-zz 的 stone / emerald 卡片样式。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|>>>>>>>|=======$)" .`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/admin/usage/__tests__/UsageStatsCards.spec.ts src/utils/__tests__/ccswitchImport.spec.ts`
- `go test ./internal/service ./internal/handler`
- `pnpm --dir docs-site docs:build`

未验证：
- 浏览器人工 smoke。
- 完整前端测试套件。
- 完整仓库级 `go test ./...`。

## 2026-06-21 - 将上游 `main` 合并到 `dev-zz`：thinking 协议、国产模型兜底定价与账号 ID 展示

分支：
- 目标：`dev-zz`
- 上游：`origin/main`
- Base：`b8a482e1`
- 合并前目标：`e5027c48`
- 上游 head：`945b9b20`
- 结果提交：本次合并提交

上游要点：
- 新增邮箱绑定后缀白名单校验，发送绑定验证码和绑定提交都复用注册邮箱策略。
- 新增 SSE `event:error` 响应体保留，运维错误日志可以看到真实上游错误内容。
- 新增 thinking 协议识别、DeepSeek `max` reasoning effort 归一化、MiniMax M 系列 `enabled` thinking 自适应处理，以及 Anthropic 兼容上游 thinking block passback 保护。
- 新增 DeepSeek V4、GLM、Kimi、MiniMax、Kimi coding 和 Doubao embedding vision 兜底定价，并支持图片输入 token 单独计价。
- 修复 Anthropic 官方 5h / 7d 窗口限流冷却被通用 429 临时不可调度规则缩短的问题。
- API Key IP ACL 拒绝响应携带客户端 IP；管理端账号列表展示并支持排序账号 ID。

合并策略：
- 合并前阅读 `docs-site/dev-zz/branch-policy.md`、`docs-site/dev-zz/patches.md`、`docs-site/dev-zz/maintenance/merge-main.md`、`docs-site/dev-zz/maintenance/merge-log.md`、`docs-site/dev-zz/changelog.md`、`docs-site/dev-zz/reference/change-map.md`、`docs-site/dev-zz/reference/api-surface.md`、`docs-site/dev-zz/reference/configuration-and-migrations.md` 和 `docs-site/dev-zz/testing/verification-matrix.md`，并扫描 `docs-site/` 全站结构与关键词。
- 用 `git fetch origin` 刷新远程引用；本地 `main` 与 `origin/main` 均为 `945b9b20`。
- 在正式合并前用 `git merge-tree --write-tree --merge-base b8a482e127c58dce1441bd14042793524b760867 HEAD origin/main` 预检，预测到一处内容冲突。
- 用 `git merge --no-commit origin/main` 把上游 `main` 合并进 `dev-zz`。
- 接受上游后端正确性、安全策略、计费、网关兼容性和账号 ID 展示改动；保留 dev-zz 的发布版本号、控制台视觉方向、认证入口隐藏策略、企业 Key 和 docs-site 文档中心。

冲突文件：
- `backend/cmd/server/VERSION`

解决说明：
- `backend/cmd/server/VERSION` 的 base 为 `0.1.136`，dev-zz 为 `1.1.6`，上游为 `0.1.137`；按 dev-zz 正式发布线保留 `1.1.6`。
- `frontend/src/views/admin/AccountsView.vue` 自动合并后仅新增账号 ID 列与排序 key，保留 dev-zz 表格多选按钮和当前 stone / emerald 样式。
- `frontend/src/i18n/locales/{zh,en}.ts` 自动合并新增账号 ID 列文案，没有改变 dev-zz 隐藏 LinuxDo / 微信入口的认证展示策略。
- 本次没有新增数据库迁移，未改变 `151/152/153` 之后的 dev-zz 迁移编号。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$" .`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts src/views/admin/__tests__/AccountsView.usageWindowsHint.spec.ts`
- `pnpm --dir docs-site docs:build`
- `mise x -C backend -- go test ./internal/handler ./internal/server/middleware ./internal/service`

未验证：
- 浏览器人工 smoke。
- 完整仓库级 `go test ./...` 和完整前端测试套件。

## 2026-06-17 - 将上游 `main` 合并到 `dev-zz`：Cyber 策略、OpenAI 配额与调度修复

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`e34ad2b1`
- 合并前目标：`0fd01ef6`
- 上游 head：`b8a482e1`
- 结果提交：本次合并提交

上游要点：
- 新增 OpenAI `cyber_policy` 硬阻断的透传、审计、计费、用户错误分类和会话本地屏蔽能力。
- 新增 OpenAI 账号 rate-limit quota 查询/重置支持，并加强 `/responses` 能力探测的工具调用校验。
- 修复 scheduler outbox 去重、合并、清理和 pending dedup index 恢复流程。
- 修复网关非 JSON 2xx、zstd 响应体、图片服务器错误故障转移、Responses fallback input 锚定，以及默认 tool strict 兼容行为。
- 新增渠道监控检测间隔 jitter 配置、账号过期自动暂停索引、OAuth 注册 promo code 修复、Anthropic system role 合并、Claude OAuth system prompt blocks，以及 `form-data` 安全 override。

合并策略：
- 合并前阅读 `docs-site/dev-zz/branch-policy.md`、`docs-site/dev-zz/patches.md`、`docs-site/dev-zz/maintenance/merge-log.md` 和 `docs-site/dev-zz/changelog.md`。
- 用 `git fetch origin` 刷新本地远程引用，以上游 `origin/main` 的 `b8a482e1` 作为合并目标。
- 在正式合并前用 `git merge-tree --write-tree HEAD origin/main` 预检，预测到一处内容冲突。
- 用 `git merge --no-commit origin/main` 把上游 `main` 合并进 `dev-zz`。
- 接受上游后端正确性、安全策略、调度器和配额能力修复；保留 `dev-zz` 的用户用量页视觉方向、管理员用量下钻、已删除 Key 证据链、图表展开、日期 URL 同步和 `docs-site` 二开文档体系。

冲突文件：
- `frontend/src/views/user/UsageView.vue`

解决说明：
- 保留 `dev-zz` 用户用量页的 stone/emerald 视觉、分析/表格切换和现有展示结构。
- 接受上游新增的 `cyber` 请求类型、i18n 文案和类型解析；在 `dev-zz` 的请求类型 label、badge、导出文本函数中补入 `cyber` 分支。
- 未采用上游默认红色 badge 样式，改为适配当前深色主题的 `rose` 半透明 badge。
- 保留 `dev-zz` 已发布的 `151_add_api_key_tags.sql`、`152_add_api_key_tags_index_notx.sql` 和 `153_normalize_api_key_inactive_status.sql`。
- 将上游新增迁移顺延为 `154_account_autopause_expiry_index_notx.sql`、`155_channel_monitor_jitter.sql`、`156_scheduler_outbox_dedup_key.sql` 和 `157_scheduler_outbox_pending_dedup_key_index_notx.sql`，并同步更新 migration runner/test 中的文件名引用。
- 全量前端测试暴露出合并后的几处小兼容问题，已一并修复：OpenAI OAuth 账号行刷新绕过旧 usage 缓存、pending OAuth 创建账号测试保留 affiliate payload、日期范围测试适配 Teleport 下拉、旧 `table-page-size-source` 分页偏好清理，以及 Dashboard 对旧统计快照缺少账号成本字段时归零显示。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|>>>>>>>)|^=======$" .`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`
- `mise x -C backend -- go test ./migrations`
- `mise x -C backend -- go test ./internal/repository`
- `mise x -C backend -- go test ./internal/server ./internal/server/middleware ./internal/handler ./internal/handler/admin ./internal/config ./internal/service ./internal/pkg/apicompat ./internal/pkg/openai`
- `mise x -C backend -- go test ./...`

未验证：
- 无。

## 2026-06-12 - 将上游 `main` 合并到 `dev-zz`：合规确认、网关修复与 Bedrock 兼容

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`434af38f`
- 合并前目标：`a7dc462f`
- 上游 head：`e34ad2b1`
- 结果提交：本次合并提交

上游要点：
- 新增管理端部署与运营合规确认 gate，包括后端状态/确认接口、中间件守卫、前端弹窗、状态 store、公开法律文档路由，以及中英文 `docs/legal/admin-compliance.*.md`。
- 新增网关正确性修复：避免错误透传/非流式错误帧重复写入、完整覆盖 `MarkResponseCommitted`、修复 OpenAI failover 模型请求体替换，以及 idempotency 响应 UTF-8 截断。
- 修复 Bedrock / Claude 兼容路径：过滤不支持的顶层字段、清理 beta token、合并 header filtering，并修复管理端 `bedrock_cc_compat` 开关回显。
- 优化账号分组调度索引、调度日志循环开销，新增 `claude-fable-5` 常量与 sponsor 资料更新。

合并策略：
- 合并前阅读 `docs-site/dev-zz/branch-policy.md`、`docs-site/dev-zz/patches.md`、`docs-site/dev-zz/maintenance/merge-log.md` 和 `docs-site/dev-zz/changelog.md`。
- 用 `git fetch origin` 刷新本地远程引用；本地 `main` 与 `origin/main` 在 `e34ad2b1` 一致。
- 在正式合并前用 `git merge-tree --write-tree dev-zz main` 预检，预测到一处内容冲突。
- 用 `git merge --no-commit main` 把上游 `main` 合并进 `dev-zz`。
- 接受上游合规确认 gate、后端网关/运行时正确性修复、Bedrock 兼容修复和调度索引优化，因为它们不替换二开的前端视觉方向、认证入口可见性策略、永久保留默认值、账号模型探测/映射行为，或源码构建部署策略。

冲突文件：
- `.gitignore`

解决说明：
- 保留 dev-zz 的 `docs-site` 依赖、缓存和构建产物忽略规则。
- 同时接受上游 `docs/legal/` 和 `docs/legal/*.md` 的反忽略规则，使新增合规法律文档可纳入版本控制。
- `backend/internal/server/routes/admin.go`、`frontend/src/components/common/BaseDialog.vue`、`frontend/src/i18n/locales/{zh,en}.ts` 和 `frontend/src/views/admin/ChannelsView.vue` 自动合并；检查后保留上游合规确认、可隐藏关闭按钮文案、法律文档文案和 Bedrock 开关修复。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$"`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/keys/__tests__/UseKeyModal.spec.ts src/api/__tests__/client.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/server/middleware ./internal/handler ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/pkg/apicompat ./internal/pkg/openai`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-06-10 - 将上游 `main` 合并到 `dev-zz`：代理回落、缓存 token 用量与 OpenAI 修复

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`1cecd271`
- 合并前目标：`c293f3f4`
- 上游 head：`434af38f`
- 结果提交：本次合并提交

上游要点：
- 新增代理过期与回落行为，包括后端 schema/service/repository 支持、代理 UI 更新、账号代理回落展示，以及回退到源站的处理。
- 新增管理端按 API Key 分组过滤用户，并收紧 API Key 的专属分组访问校验。
- 拆分用量的缓存创建/缓存命中 token 统计，并新增图片输出 token/费用展示辅助函数。
- 新增 OpenAI 网关与兼容性修复：传输错误故障转移、粘性分组校验、跨分组 `previous_response_id` 处理、非流式 JSON content type、响应失败保留，以及 prompt 缓存 key 传递。
- 新增多实例后台任务的 leader 锁、setup/bootstrap 修复、Go/OpenAI prompt 指令更新、版本/文档更新，以及上游 `skills/sub2api-admin` 辅助工具。

合并策略：
- 合并前先阅读 `secondary-dev/README.md`、`secondary-dev/PATCHES.md`、`secondary-dev/MERGELOG.md` 和 `secondary-dev/CHANGELOG.md`。
- 用 `git fetch origin` 刷新本地远程引用；本地 `main` 与 `origin/main` 在 `434af38f` 一致。
- 在正式合并前用 `git merge-tree --write-tree dev-zz main` 预检，预测到两处内容冲突。
- 用 `git merge --no-commit main` 把上游 `main` 合并进 `dev-zz`。
- 接受上游后端/运行时正确性修复和代理/用量/管理端新增功能，因为它们不替换二开的前端认证可见性策略、源码构建部署策略、永久保留默认值，或账号模型探测/映射行为。

冲突文件：
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/user/UsageView.vue`

解决说明：
- 保留二开的账号表格空状态样式，同时接受上游的代理过期展示、回落标记和回退操作。
- 让用户用量页保持二开的 stone/emerald 视觉方向，同时接受上游的缓存命中/缓存创建总量、缓存命中率、图片输出 token/费用明细，以及文本输出 token 价格拆分。
- 保留二开的图片用量展示语义：除非行被显式标为 token 计费，否则图片行仍归类为图片计费，使缺失 `billing_mode` 的旧图片行仍渲染为图片用量。
- 移除上游 `backend/internal/pkg/openai/instructions_gpt5_2.txt` 的一个行尾空格，使暂存的合并通过 `git diff --cached --check`。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$"`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/views/user/__tests__/UsageView.spec.ts src/views/admin/__tests__/UsageView.spec.ts src/views/admin/__tests__/apiKeyGroupFilterOptions.spec.ts src/utils/__tests__/proxyExpiry.spec.ts src/components/account/__tests__/UsageProgressBar.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/server/middleware ./internal/handler ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/usagestats`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-06-06 - 将上游 `main` 合并到 `dev-zz`：失败请求可见性与网关修复

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`f1aa5896`
- 合并前目标：`a3997b07`
- 上游 head：`1cecd271`
- 结果提交：本次合并提交

上游要点：
- 在运维/管理端/用户端用量界面新增失败请求的持久化与可见性，包括 API Key/账号归属、已删除 Key 的审计查询，以及受公开设置控制的面向用户的错误请求表。
- 新增网关正确性修复：Responses 转 Anthropic 的工具配对、chat-completions 失败响应、缺失的流式终止输出、OpenAI 图片限流冷却故障转移，以及 Claude Code 客户端识别。
- 新增数据库连接池生命周期下限、调度器粘性健康逃逸、EasyPay 查单状态处理、管理端审核自动封禁豁免、分组描述清空、Go 1.26.4 工具链更新，以及相关回归测试。

合并策略：
- 合并前先阅读 `secondary-dev/README.md`、`secondary-dev/PATCHES.md`、`secondary-dev/MERGELOG.md` 和 `secondary-dev/CHANGELOG.md`。
- 用 `git fetch origin` 刷新本地远程引用；本地 `main` 与 `origin/main` 在 `1cecd271` 一致。
- 在正式合并前用 `git merge-tree --write-tree dev-zz main` 预检，预测到两处内容冲突。
- 用 `git merge --no-commit main` 把上游 `main` 合并进 `dev-zz`。
- 接受上游失败请求可见性和后端网关/运行时修复，因为它们是增量或正确性导向的，不改变二开的账号模型探测/映射行为、前端认证可见性策略、永久保留默认值，或源码构建部署策略。

冲突文件：
- `frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`
- `frontend/src/views/user/UsageView.vue`

解决说明：
- 保留二开的 stone/emerald 管理端运维表格样式，同时新增上游的 API Key 和账号归属列，包括已删除 Key 标记和加宽的空状态 colspan。
- 让用户用量页保持二开的 stone/emerald 样式和图片用量展示语义，同时接受上游的 null 安全 token/费用渲染和新的用户错误请求标签页。
- 冻结的 `secondary-dev/DEV_SEED_DESIGN.md` 文档仍隔离在 `stash@{0}`，未包含在本次合并中。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$"`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts src/views/admin/__tests__/UsageView.spec.ts src/views/user/__tests__/UsageView.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/server/middleware ./internal/handler ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/pkg/apicompat ./internal/payment/provider`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-06-05 - 将上游 `main` 合并到 `dev-zz`：OpenAI 5 小时用量语义

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`aa69e394`
- 合并前目标：`437e2df5`
- 上游 head：`f1aa5896`
- 结果提交：本次合并提交

上游要点：
- 回退 OpenAI Codex 5 小时用量百分比归一化，使存储的 5 小时用量重新遵循上游直接的 `used_percent` 语义。
- 移除已废弃的 5 小时“剩余转已用”归一化辅助函数及相关的快照/账号用量测试。
- 更新 OpenAI 限流和账号用量测试，以匹配回退后的 5 小时百分比行为。

合并策略：
- 合并前先阅读 `secondary-dev/README.md`、`secondary-dev/PATCHES.md` 和 `secondary-dev/MERGELOG.md`。
- 用 `git fetch origin` 刷新本地远程引用。
- 用 `git merge --no-commit main` 把上游 `main` 合并进 `dev-zz`。
- `git merge-tree --write-tree dev-zz main` 和正式合并均未报告内容冲突。
- 接受上游 OpenAI 5 小时用量语义回退，因为它仅限于后端 OpenAI Codex 用量统计，不改变二开的前端认证可见性、账号模型探测/映射行为、可用渠道导出行为、保留默认值，或部署策略。

冲突文件：
- 无。

解决说明：
- 自动合并应用了上游对 `normalizeCodexFiveHourUsedPercent` 的移除，并恢复了从 OpenAI Codex 主/次 `used_percent` 头的直接赋值。
- 自动合并保留了上次上游同步之后的二开提交，包括可用渠道表格/导出工作和二开部署文档对齐。
- 之前暂存的计费维度定价设计文档仍留在 stash，未包含在本次合并中。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$"`
- `mise x -C backend -- go test ./internal/service`

未验证：
- 未运行前端 typecheck、lint 和测试，因为本次上游同步只改动了后端 OpenAI service 文件和测试。
- 未运行完整后端测试套件。

## 2026-06-02 - 将上游 `main` 合并到 `dev-zz`：Codex bridge 后续

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`bc5813f0`
- 合并前目标：`8bd61b24`
- 上游 head：`aa69e394`
- 结果提交：本次合并提交

上游要点：
- 重新设计 Codex Responses 转 Chat Completions 的 bridge，包括请求不变量覆盖和响应流事件 wire 辅助函数。
- WebSocket Codex 图片 bridge 工具注入，以及额外的 WebSocket ingress-session 覆盖。
- Antigravity Gemini 限流、调度器缓存、配额范围和账号调度修复。
- 管理端用户余额处理改为基于指针的输入，使零余额和未填余额可以区分。

合并策略：
- 用 `git merge --no-commit main` 把上游 `main` 合并进 `dev-zz`。
- `git merge-tree --write-tree dev-zz main` 和正式合并均未报告内容冲突。
- 接受上游后端兼容性、调度器、Antigravity 和管理端用户修复，因为它们是增量或正确性导向的，不改变二开的前端认证可见性策略。
- 保留已有的二开记录、账号模型探测/映射行为、永久数据保留默认值,以及源码构建部署指引。

冲突文件：
- 无。

解决说明：
- 自动合并保持上次 `dev-zz` 同步结果不变，同时新增上游 `apicompat` bridge 重新设计和新的回归测试。
- 自动合并接受了管理端创建用户的余额指针改动及其 API 类型更新，因为它保留了显式的零余额输入行为。
- 自动合并接受了 Antigravity 调度器/限流修复，未触碰二开的 UI 策略或部署记录。

验证：
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/views/admin/__tests__/UsersView.spec.ts`
- `mise x -C backend -- go test ./internal/pkg/apicompat ./internal/service ./internal/repository ./internal/handler/admin ./internal/handler`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-06-02 - 将上游 `main` 合并到 `dev-zz`

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`f18451e5`
- 合并前目标：`e6e7e5b9`
- 上游 head：`bc5813f0`
- 结果提交：本次合并提交

上游要点：
- OpenAI 请求体保留/重构、OOM 处理、故障转移缓存体重映射覆盖、WebSocket 用量去重修复、超大 WebSocket 请求桥接，以及 WebSocket 转 HTTP bridge 恢复。
- 账号创建流程可在账号持久化前，从已输入的凭据同步上游模型。
- 管理端用量性能/查询缓存更新、从当前统计加载模型筛选选项,以及支持查看已删除用户的历史用量。
- OpenAI OAuth 刷新增强、Claude Code count_tokens 配额、OpenAI 5 小时用量窗口百分比修复,以及账号用量窗口提示文案。

合并策略：
- 用 `git merge --no-commit main` 把上游 `main` 合并进 `dev-zz`。
- 保留二开的账号模型探测、models.dev 搜索、映射填充、同名映射持久化、清空全部模型映射、永久数据保留默认值、首页/控制台视觉方向,以及源码构建部署记录。
- 接受上游创建账号时的上游模型同步，因为它是增量的，可与二开基于凭据的探测流程共存。
- 接受上游管理端用量的已删除用户和用量窗口改进，同时保留二开的 popover/stone UI 样式。

冲突文件：
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/admin/usage/UsageFilters.vue`

解决说明：
- 在创建账号白名单区合并 `ModelWhitelistSelector` 的 props，使二开的 `/probe-models` 加载/新增/缺失标记和上游的 `syncCredentials` 预览都可用。
- 让 Bedrock/Anthropic 白名单行为与已有的二开探测流程保持一致，同时在存在凭据时启用上游预览同步。
- 让 `UsageFilters` 保持二开的 `popover-item` 样式，并新增上游的已删除用户标记和已删除用户排序。

验证：
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts src/components/admin/usage/__tests__/UsageFilters.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts src/views/admin/__tests__/AccountsView.usageWindowsHint.spec.ts src/views/admin/__tests__/UsageView.spec.ts src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/handler`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-06-01 - 将上游 `main` 合并到 `dev-zz`

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`bebc0823`
- 合并前目标：`f1ca9454`
- 上游 head：`f18451e5`
- 结果提交：本次合并提交

上游要点：
- 用户平台配额的数据库聚合 flusher 和哨兵回填，减少重复的预检数据库读取。
- 账号 5 小时/7 天用量阈值自动暂停、同账号重试状态码配置,以及账号创建时间展示。
- 自定义分组级 `/v1/models` 列表配置和候选模型加载。
- OpenAI embeddings 网关支持、端点能力门控、Codex CLI/Claude Code 允许客户端处理,以及 Responses/WebSocket 兼容性修复。
- 用量请求上下文保留、并发错误分类、模型未找到冷却行为,以及本地业务限制原因分类。
- 计费、长上下文缓存定价、Gemini messages、Anthropic/Responses 转换、Bedrock 上下文管理、定价元数据,以及直到 `0.1.133` 的版本更新。

合并策略：
- 用 `git merge --no-commit main` 把上游 `main` 合并进 `dev-zz`，`git merge-tree --write-tree dev-zz main` 和正式的 `git merge --no-commit main` 均未报告内容冲突。
- 保留二开的账号模型探测、models.dev 搜索、映射填充、同名映射模式持久化、清空全部模型映射、永久数据保留默认值、首页/控制台视觉方向,以及源码构建部署记录。
- 接受上游的配额、自动暂停、分组模型列表、embeddings、端点能力、请求上下文、重试状态、账号创建时间、定价、计费、兼容性,以及运维/风控更新。
- 延续二开前端隐藏 LinuxDo 和微信认证入口的策略，同时保持上游可见的其他提供商和后端设置/数据不变。

冲突文件：
- 无。

解决说明：
- 自动合并保留了两条账号模型发现路径：二开基于凭据的 `/probe-models` 和上游基于已保存账号的 `/models/sync-upstream`。
- 自动合并保留了显式的 `model_restriction_mode: mapping` 处理和二开的清空全部映射回归覆盖，同时接受上游的账号自动暂停和重试状态配置。
- 自动合并在登录/资料/管理端界面保持仅前端的 LinuxDo/微信可见性策略，同时在适用处接受上游的 DingTalk 和微信后端/设置/支付代码。
- 接受上游自定义分组 `/v1/models` 列表配置，因为它是增量的，不改变二开的账号白名单、映射或调度器行为。

验证：
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts src/views/admin/__tests__/groupsModelsList.spec.ts src/views/admin/__tests__/groupsModelsListCandidates.spec.ts src/views/admin/__tests__/groupsModelsListLayout.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts src/views/admin/__tests__/RiskControlView.spec.ts src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/handler`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-05-27 - 将上游 `main` 合并到 `dev-zz`

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`18790386`
- 合并前目标：`68877dcc`
- 上游 head：`bebc0823`
- 结果提交：本次合并提交

上游要点：
- 用户平台美元配额支持。
- DingTalk OAuth 和提供商默认授权支持。
- 内容审核的按模型控制和风险阈值配置。
- 账号上游模型同步,以及 OpenAI API Key 的 Responses 支持控制。
- 渠道监控的 API 模式/模板更新。
- 用户/管理端用量的图片计费元数据和每日用量视图。
- 兑换码批量更新、邮件模板、订阅提醒邮件,以及相关管理端 UI 更新。
- OpenAI Responses/WebSocket/工具输出续传修复、HTTP/2 超时修复,以及依赖/安全更新。

合并策略：
- 把上游 `main` 合并进 `dev-zz`。
- 保留二开的账号模型探测、models.dev 搜索、映射填充、同名映射模式持久化、清空全部模型映射、永久数据保留默认值、首页/控制台视觉方向,以及源码构建部署记录。
- 接受上游的 DingTalk、用户平台配额、账号上游模型同步、OpenAI Responses 控制、风控、渠道监控、图片用量、兑换、邮件模板、订阅提醒,以及网关兼容性更新。
- 延续二开前端隐藏 LinuxDo 和微信认证入口的策略，同时接受上游可见的 DingTalk 入口。

冲突文件：
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/ModelWhitelistSelector.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/user/profile/ProfileIdentityBindingsSection.vue`
- `frontend/src/components/user/profile/ProfileInfoCard.vue`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/views/admin/ChannelsView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/UsersView.vue`
- `frontend/src/views/auth/LoginView.vue`
- `frontend/src/views/user/UsageView.vue`

解决说明：
- 保留两条账号模型发现路径：二开基于凭据的 `/probe-models` 和上游基于已保存账号的 `/models/sync-upstream`。
- 保留二开的“填入相关模型” / "Fill related models" 操作标签，并新增独立的上游同步标签。
- 保留显式的 `model_restriction_mode: mapping`，使同名映射以映射模式重新打开，同时对没有显式模式的旧映射保留上游混合白名单/映射行为。
- 保留二开的清空全部映射回归覆盖和上游的 OpenAI Responses 覆写回归覆盖。
- 在前端登录/资料/管理端默认来源界面保持隐藏 LinuxDo/微信；DingTalk 保持可见，因为它是现有隐藏策略之外的上游功能。
- 把上游的渠道定价同步和用户列强制可见行为合并进二开的 popover/emerald 视觉风格。
- 修复用户用量提示的解析，使存在图片用量时始终显示图片元数据，即使存储的 `billing_mode` 为空。

验证：
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/views/user/__tests__/UsageView.spec.ts src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/handler/admin ./internal/config`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-05-14 - 将上游 `main` 合并到 `dev-zz`

分支：
- 目标：`dev-zz`
- 上游：`main`
- Base：`a1106e81`
- 合并前目标：`6bdd4f1b`
- 上游 head：`18790386`
- 结果提交：本次合并提交

上游提交：
- `af550fa6` feat: 增加 GitHub 和 Google 邮箱快捷登录
- `e872cbec` feat: 添加登录注册条款确认
- `b23055af` feat: add Airwallex payments and multi-currency support
- `fff4a300` feat(risk-control): add content moderation audit
- `7a9c1d7e` feat(frontend): add account Codex image bridge control
- `18790386` fix(deploy): 移除数据库与 Redis 宿主机端口映射

合并策略：
- 把上游 `main` 合并进 `dev-zz`。
- 保留二开的账号模型探测、映射填充、清空全部模型映射、首页定价文案、永久数据保留默认值,以及源码构建部署记录。
- 接受上游的支付、邮箱 OAuth、登录协议、内容审核、Codex 图片 bridge、OpenAI/Gemini 兼容性,以及部署更新。

冲突文件：
- `backend/internal/server/routes/admin.go`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/user/profile/ProfileInfoCard.vue`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/auth/LoginView.vue`
- `frontend/src/views/auth/RegisterView.vue`

解决说明：
- 保留两条管理端账号路由：二开的 `/probe-models` 和上游的 `/import/codex-session`。
- 保留两个编辑账号回归测试：清空映射模式模型,以及上游 Codex 图片 bridge 覆写。
- 保留上游的 GitHub/Google 资料、登录、注册和认证来源默认支持，同时继续在前端登录/注册/资料/设置界面隐藏 LinuxDo/微信。
- 保留二开的账号模型探测语言 key 和已有的“填入相关模型” / "Fill related models" 操作标签。
- 把上游的账号工具菜单操作合并进二开的 popover 样式。
- 保留上游的登录协议门控，同时保持二开认证页的视觉风格。

验证：
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/handler/admin ./internal/config`

未验证：
- 未运行完整前端测试套件。
- 未运行完整后端测试套件。

## 2026-05-05 - 将上游 WebSocket 恢复修复合并到 `dev-zz`

分支：
- 目标：`dev-zz`
- 上游：`main`
- 结果提交：`2d6e114a`

上游提交：
- `e71b55ec` fix: skip previous_response_id recovery when payload has function_call_output
- `94e49431` Merge pull request #2197 from learnerLj/fix/ws-preflight-ping-fc-output-recovery

合并策略：
- 把 `main` 合并进 `dev-zz`。
- 保留 `dev-zz` 上已有的二开提交。
- 未发生冲突。

解决说明：
- 接受上游在 `backend/internal/service/openai_ws_forwarder.go` 的后端改动。
- 已有的首页/认证/控制台 UI 二开改动保持不变。

验证：
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `git diff --check`

未验证：
- 未运行后端 Go 测试，因为当前 shell 中没有 `go`。

备注：
- `stash@{0}: On main: 数据永久保存` 仍保留在本地，未被合并。
