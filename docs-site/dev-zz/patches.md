# 补丁记录

## 2026-06-29 - 上游 main 同步到 dev-zz-develop：Grok、Codex 检测、系统日志 Key 筛选与支付修复

范围：
- 上游同步：`origin/main` `c99112a9` 合并到 `dev-zz-develop`
- 后端：Grok / xAI OAuth、quota probe、网关转发、OpenAI/Codex PAT 与 app-server 检测、quota platform 后扣、no-account 错误、Responses / Chat Completions 兼容、支付和系统日志
- 前端：账号创建/编辑弹窗、设置页、用户 Key 列设置、系统日志表、支付页面、平台图标/i18n
- 迁移：`backend/migrations/162_add_ops_system_logs_api_key_id.sql`、`163_add_ops_system_logs_api_key_id_index_notx.sql`
- 文档：`docs-site/dev-zz/{changelog.md,patches.md,maintenance/merge-log.md}`

改动：
- 吸收上游 Grok / xAI OAuth 和订阅配额探测链路，管理端账号配置、OAuth 授权、token refresh、quota probe 和 OpenAI-compatible Grok 网关转发均纳入本分支。
- 吸收 Codex / ChatGPT 账号检测加固：PAT auth mode、app-server client、engine fingerprint 信号、Codex 白名单设置与相关测试。
- 吸收 OpenAI / Responses / Chat Completions 兼容修复，包括 tool schema 规范化、passthrough function args 去重、图片 bridge `tool_choice=auto`、overloaded 错误识别、no-account `model_not_found` 和 token refresh 非重试错误。
- 吸收支付显示与订单金额修复，保留订阅 CNY 换算和支付二维码弹窗修复。
- 运维系统日志新增 `api_key_id` 字段、索引、查询筛选和清理筛选；在 dev-zz 中顺延迁移编号为 `162/163`，避免与既有 `154/155` 撞号。
- 用户 API Key 页面吸收上游列设置，同时保留 dev-zz 的标签筛选、批量创建/批量操作、单 Key 用量下钻、`disabled` 状态语义和系统状态保护。
- 账号创建/编辑弹窗吸收 Grok OAuth 模型映射与 Antigravity project ID，同时保留 dev-zz 的模型目录、上游成本设置、模型自检相关策略和 stone / emerald 视觉。
- OpenAI usage 记录同时保留 dev-zz `ScheduleMeta` 与上游 `QuotaPlatform`，并继续优先使用真实转发结果里的上游 endpoint。

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

## 2026-06-28 - 定价驱动的站点自检模型监控（取代 2026-06-26 方案）

> 本条取代下方 2026-06-26 的实现：用户侧模型状态的数据来源由「上游渠道探针聚合」改为「站点自检」。旧的 `channel_monitor_model_status.go` 已删除，旧设计文档 `features/model-service-status-page.md` 已删除并由 `features/pricing-driven-self-check-monitoring-design.md` 取代。

范围：
- `backend/migrations/161_model_self_check.sql`（`model_self_check_config` + `model_self_check_histories`）
- `backend/internal/service/model_self_check_{status,probe,runner}.go` + 测试
- `backend/internal/repository/model_self_check_repo.go`
- `backend/internal/handler/channel_monitor_user_handler.go`（改接自检 service）+ `_test.go`
- `backend/internal/pkg/ctxkey/ctxkey.go`（新增 `ModelSelfCheckProbe` 标记）
- 热路径探针守卫：`gateway_service.go`、`ratelimit_service.go`、`openai_account_runtime_block_fastpath.go`、`antigravity_gateway_service.go`、`gemini_messages_compat_service.go`、`gemini_chat_completions_compat_service.go`
- 设置：`domain_constants.go`、`setting_service.go`、`settings_view.go`、`handler/admin/setting_handler.go`、`handler/dto/settings.go`
- 定价开关：`handler/admin/channel_handler.go`、`repository/channel_repo_pricing.go`、`service/channel.go`
- 前端：`views/admin/SettingsView.vue`、`views/admin/ChannelsView.vue`、`components/admin/channel/PricingEntryCard.vue`、`views/user/ChannelStatusView.vue`、`components/user/monitor/MonitorTimeline.vue`、`api/admin/{channels,settings}.ts`、`api/modelStatus.ts`、`i18n/locales/{zh,en}.ts`
- `docs-site/dev-zz/{changelog.md,patches.md,index.md,reference/api-surface.md}` + `features/pricing-driven-self-check-monitoring-design.md` + `.vitepress/config.ts`

改动：
- 在渠道定价里按模型开启「自检」开关（`model_self_check_config(channel_id, model, enabled)`）。
- 自检 runner 对开启的模型解析「可服务的上游账号」（跨分组去重），用合成 `gin.Context` 走真实网关 `Forward`，`max_tokens=1`，结果写 `model_self_check_histories`。
- 探针请求带 `ctxkey.ModelSelfCheckProbe` 标记；限流封禁、runtime-block、重试、failover 在该标记下全部跳过（默认安全：无标记时原逻辑不变），且不调 `RecordUsage`——**不写用量、不计费、不影响生产账号调度**。
- 用户侧 `/monitor` 改为按 **分组 / 模型** 展示，`/api/v1/model-status` 新增 `group_id` / `group_name` / `degraded_ratio_24h`；状态按 (分组,模型) 对覆盖账号 OR 聚合，含陈旧检测（超新鲜窗口→`unknown`）。
- 新增管理员设置：`model_self_check_enabled`、`self_check_default_interval_seconds`、`self_check_max_concurrency`、`self_check_max_tasks_per_round`。

验证：
- `cd backend && go test ./internal/service ./internal/handler ./internal/server/routes`（含 4 平台真实 Forward 集成测试、禁止字段断言、去重/聚合/陈旧检测、429 不封账号）
- `cd backend && go build ./...`
- `pnpm --dir frontend run typecheck && pnpm --dir frontend run lint:check`

未验证：
- 全量 `go test ./...` 与各平台 staging 实测（合成 context 探针真实跑通、用户页视觉）由仓库所有者本地确认。

## 2026-06-26 - 用户侧模型服务状态页（已被 2026-06-28 取代）

范围：
- `backend/internal/service/channel_monitor_model_status.go` + `_test.go`
- `backend/internal/handler/channel_monitor_user_handler.go`
- `backend/internal/server/routes/user.go` + `user_routes_test.go`
- `frontend/src/api/modelStatus.ts`
- `frontend/src/views/user/ChannelStatusView.vue`
- `frontend/src/composables/useChannelMonitorFormat.ts`
- `frontend/src/api/index.ts`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `docs-site/dev-zz/{changelog.md,patches.md,index.md,reference/api-surface.md,features/model-service-status-page.md}`
- `docs-site/.vitepress/config.ts`

改动：
- 用户侧 `/monitor` 从旧的 monitor / provider / group 视图切换为模型服务状态视图，只展示公开模型名、聚合状态、24h / 7d / 30d 可用率、延迟、最后检测时间和脱敏时间线。
- 新增用户接口 `GET /api/v1/model-status` 与 `GET /api/v1/model-status/detail?model=...`，响应 DTO 不包含 monitor ID、monitor 名称、provider、endpoint、group、API mode、原始错误、账号、渠道 ID 或成本字段。
- 新增 `ChannelMonitorService.ListUserModelStatus` / `GetUserModelStatus`：复用 enabled channel monitor、latest history、`ComputeAvailabilityForMonitors` 和最近历史查询，按公开模型名跨多个隐藏探针聚合状态；24h / 7d / 30d 当前全部直接读取 `channel_monitor_histories`。
- 聚合口径：所有探针无历史为 `unknown`；至少一个成功但存在失败、降级或缺失探针为 `degraded`；无可用探针且有失败历史为 `failed`；全部可用为 `operational`。
- 撤下旧用户侧 `/api/v1/channel-monitors` 探针路由，避免普通登录用户继续通过 API 看到上游 monitor / provider / group 等内部字段；管理员 `/api/v1/admin/channel-monitors` 不变。
- 前端新增模型状态 API wrapper，`/monitor` 支持窗口切换、搜索、自动刷新、详情弹窗和无数据总体状态；导航文案从“渠道状态”调整为“模型状态”。
- dev-zz 文档补齐模型状态页实现状态、接口边界、验证记录和侧边栏入口。

验证：
- `cd backend && go test ./internal/service -run 'TestListUserModelStatus|TestChannelMonitor'`
- `cd backend && go test ./internal/handler ./internal/server/routes ./internal/service`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run lint:check`
- `pnpm --dir frontend run build`
- `pnpm --dir docs-site build`

未验证：
- 浏览器人工 smoke（模型状态页实际视觉、详情弹窗和刷新交互），由管理员本地验证。
- 完整仓库级 `go test ./...` 与完整前端测试套件。

## 2026-06-25 - 时间范围选择器支持可选「精确到秒」（DateRangePicker）

范围：
- `backend/internal/pkg/timezone/timezone.go`（新增 `ParseUserDateOrDateTime`）+ `timezone_test.go`
- `backend/internal/handler/admin/dashboard_handler.go`（`parseTimeRange`）
- `backend/internal/handler/admin/usage_handler.go`（`List`/`Stats`）
- `backend/internal/handler/usage_handler.go`（`parseOwnerAPIKeyAnalyticsRange` 及其调用、user `List`/`Stats`、`parseUserTimeRange`（user 仪表盘 trend/models）、user `ListErrors`）
- `frontend/src/components/common/DateRangePicker.vue`（开始/结束日期旁加可选 `<input type="time" step="1">`；emit `update:startTime/endTime` + `change` 负载加 startTime/endTime；预设清空时间=整天）
- `frontend/src/views/admin/{UsageView.vue,DashboardView.vue}`、`frontend/src/views/user/{UsageView.vue,DashboardView.vue}`、`frontend/src/components/user/{UsageAnalyticsPanel.vue,dashboard/UserDashboardCharts.vue}`（接 `v-model:start-time/end-time`，非空时注入各接口 `start_time/end_time`）
- `frontend/src/types/index.ts`、`frontend/src/api/admin/{usage.ts,dashboard.ts}`、`frontend/src/api/usage.ts`（参数类型加 `start_time?/end_time?`；`getStatsByDateRange` 加可选 opts）
- `frontend/src/i18n/locales/{zh,en}.ts`（`dates.startTime/endTime`）

改动：
- 新增 `timezone.ParseUserDateOrDateTime(value, userTZ) (t, hasTime, err)`：依次按 RFC3339 / `2006-01-02 15:04:05` / `2006-01-02` 解析，`hasTime` 标记是否带时分秒。
- 后端各解析点（admin `parseTimeRange`/`List`/`Stats`、user owner-analytics range/`List`/`Stats`/`parseUserTimeRange`/`ListErrors`）统一为：`start_time/end_time` 优先于 `start_date/end_date`；**仅在纯日期口径下保留 `+1 天` 整天补偿，带时间口径跳过**。服务层与仓储 SQL（`created_at` timestamptz 半开区间）无需改动。
- 前端把时分秒做进共享 `DateRangePicker`（每个边界一个 `time` 输入，默认开始 00:00:00 / 结束 23:59:59），4 个消费页接住并注入 `start_time/end_time`。**结束按「含当秒」语义**：发出 ISO 时 +1 秒转为半开排他上界，故默认 23:59:59 等价于次日 00:00（与原按整天零回归）；预设重置为整天默认时间。时间被清空时该端回退纯日期口径。**未引入页面级外挂控件**（上一轮的 datetime-local 外挂方案已撤销）。
- 修复 `DateRangePicker` 时间 v-model 的 round-trip 缺陷：`startTime/endTime` 改为单向 emit（ISO），不再把 ISO 回灌进 `type=time` 输入框（此前会导致 apply 后重新打开时间框显示异常）。

已知限制：
- 趋势图预聚合快路径在 `day` 粒度按 `::date`、`hour` 粒度按整点桶化，故精确时间对趋势图在小时粒度下聚合到整点；统计卡片/模型分布/日志列表为秒级精度。

验证：
- `mise x -C backend -- go build ./...`；`go test -tags unit ./internal/handler/... ./internal/pkg/timezone/...`（全部 ok，含新增 `TestParseUserDateOrDateTime`）。
- `pnpm --dir frontend typecheck`、`eslint`（改动文件）、`DateRangePicker.spec`（已更新 `change` 负载断言，通过）。

未验证：
- 浏览器人工 smoke（在选择器里选时分秒后各图表/列表的秒级过滤效果），由管理员本地验证。
- 完整 `go test ./...` 与完整前端测试套件。

## 2026-06-25 - 模型级限流：单模型手动解除与失败阈值配置

范围：
- `backend/internal/service/model_fail_counter.go`（新增）
- `backend/internal/repository/model_fail_counter_cache.go`（新增）
- `backend/internal/service/{ratelimit_service.go,settings_view.go,domain_constants.go,setting_service.go,account_service.go,wire.go}`
- `backend/internal/repository/{account_repo.go,wire.go}`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/admin/{account_handler.go,setting_handler.go}`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/service/*_test.go`（新增 `model_rate_limit_threshold_test.go`、各 mock 补 stub）
- `frontend/src/api/admin/{accounts.ts,settings.ts}`
- `frontend/src/components/account/AccountStatusIndicator.vue`
- `frontend/src/views/admin/{AccountsView.vue,SettingsView.vue}`
- `frontend/src/i18n/locales/{zh,en}.ts`

改动：
- 单模型手动解除：新增 `accountRepository.ClearModelRateLimit(id, scope)`，用 jsonb `#-` 仅删除 `extra.model_rate_limits[scope]`，并同步调度器 outbox/快照；服务层 `RateLimitService.ClearModelRateLimit` 同时重置该 scope 的失败计数器；新增路由 `POST /admin/accounts/:id/clear-model-rate-limit`。
- 前端账号列表的「普通模型限流」徽标新增「×」解除按钮，复用现有 `patchAccountInList` 局部刷新；积分耗尽/走积分（AICredits）徽标不显示该按钮。
- 失败阈值策略：新增 `ModelFailCounterCache`（Redis 滑动窗口，key 为 `model_fail_count:account:<id>:<scope>`，镜像 OpenAI 403 计数器）。`HandleOpenAIModelRateLimit` 和 `handleProviderModelUpstreamFailure` 在打限流标记前先经过 `shouldTripModelRateLimit` 闸门：未达阈值时仅返回 handled（仍触发账号切换）而不打标记。
- 冷却注入：`openAIModelRateLimitResetAt` / `modelUpstreamFailureResetAt` 重构出带 override 版本，配置冷却仅作为最末回退，上游 header retry-after / body reset 仍优先。
- 新增管理员设置 `model_rate_limit_settings`（Enabled / FailureThreshold / WindowMinutes / CooldownSeconds），读时 clamp、写时校验；新增 `GET/PUT /admin/settings/model-rate-limit` 及前端设置卡片。
- 默认 `Enabled=false`，闸门、nil 计数器、设置读取失败均降级为「首次失败即限流」，完全保持历史行为（有回归测试守护）。

验证：
- `mise x -C backend -- go build ./...`
- `mise x -C backend -- go test -tags unit ./internal/service ./internal/repository ./internal/handler/admin ./internal/server/...`（全部 ok）
- 新增测试 `mise x -C backend -- go test -tags unit -race -run 'ModelRateLimit|ClearModelRateLimit|HandleOpenAIModelRateLimit'`（通过）
- `pnpm --dir frontend typecheck`、`pnpm --dir frontend exec eslint`（改动文件）、`pnpm --dir frontend test:run AccountStatusIndicator.spec`

未验证：
- 浏览器人工 smoke（解除按钮交互、设置页阈值生效），由管理员本地验证。
- 完整 `go test ./...` 与完整前端测试套件；注意仓库已存在与本改动无关的 `-race` flake（`TestIsNonRetryableGeminiOAuthError`、`TestUpdateProviderInstance...`，去掉 `-race` 即通过）。

## 2026-06-25 - 运维监控客户可见失败排障入口

范围：
- `backend/internal/handler/admin/ops_handler.go`
- `backend/internal/repository/ops_repo.go`
- `backend/internal/service/ops_models.go`
- `frontend/src/api/admin/ops.ts`
- `frontend/src/views/admin/ops/{OpsDashboard.vue,components/OpsDashboardHeader.vue,components/OpsErrorDetailsModal.vue,composables/useOpsModalStack.ts}`
- `frontend/src/views/admin/ops/{components,composables}/__tests__/*`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `docs-site/dev-zz/{changelog.md,patches.md,index.md,features/ops-customer-visible-error-triage.md}`
- `docs-site/.vitepress/config.ts`

改动：
- 运维总览新增“客户可见失败”口径，展示所有 `status_code >= 400` 的客户可见失败比例，并把 SLA 错误和客户侧限制拆开展示。
- SLA 卡片继续沿用 `error_count_sla` / `request_count_sla` 口径，只把卡片明细入口改为“SLA 错误”。
- 上游错误卡片拆成“非限流上游错误”和“上游限流/过载”，两个数字都可以直接进入对应错误明细。
- 错误明细弹窗新增 preset 链路，支持从不同卡片打开时自动设置标题、视图、归因和状态码筛选。
- 错误明细和请求明细在自定义时间范围下统一透传 `start_time` / `end_time`，不再让弹窗退回默认最近 1 小时。
- 上游错误明细默认对齐卡片的 provider 归因口径，不再强制 `phase=upstream`，避免 network/provider 类失败被卡片统计但明细漏查。
- 请求明细自定义时间范围的窗口文案改为真实起止时间，避免显示成默认 1 小时。
- 错误列表接口新增 `status_codes_exclude` 参数，前端用于查询非 429/529 的上游错误；原有 `status_codes` 和 `status_codes_other` 继续保留。
- 运维错误明细文案调整为“SLA 错误 / 客户侧限制 / 全部失败”，降低客服排查客户报错时的理解成本。

验证：
- `git diff --check`
- `pnpm --dir frontend test:run src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts src/views/admin/ops/composables/__tests__/useOpsModalStack.spec.ts`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `mise x -C backend -- go test ./internal/handler/admin ./internal/repository ./internal/service`
- `pnpm --dir docs-site docs:build`

未验证：
- 浏览器人工 smoke，由管理员在本地页面验证交互和视觉细节。
- 完整仓库级前端测试套件和完整 `go test ./...`。

## 2026-06-22 - 上游 main 同步到 dev-zz-develop：缓存 Token 明细与兼容修复

范围：
- `.github/workflows/{backend-ci.yml,cla.yml,release.yml,security-scan.yml}`
- `backend/internal/{config,handler,service}/**`
- `deploy/{config.example.yaml,docker-compose.dev.yml,docker-compose.local.yml}`
- `frontend/src/{App.vue,api/admin/usage.ts,components/admin/usage,i18n,router,utils}`
- `assets/partners/logos/*`
- `docs-site/dev-zz/{changelog.md,patches.md,maintenance/merge-log.md}`

改动：
- 合并上游 `main` 到 `dev-zz-develop`，上游 head 为 `85a3b122`。
- 吸收管理端 usage 缓存 Token 明细展示，统计卡片可以查看缓存创建和缓存读取拆分。
- 吸收 OpenAI 图片 `response.incomplete` 软失败识别、OpenAI / Chat Completions endpoint 记录修复，以及 Gemini / Vertex Anthropic schema 与 beta header 兼容修复。
- 吸收 Claude Code / CC Switch 新版识别逻辑、默认模型更新和新版 CLI billing block 测试。
- 吸收账号调度“优先选择最早重置账号”能力，订阅 affiliate rebate，promo code 过期时间清空，以及部署 SELinux bind mount `:Z` 标记。
- 更新 sponsor 资料和合作方 logo。
- `backend/cmd/server/VERSION` 冲突按 dev-zz 发布线保留 `1.2.1`，没有采用上游 `0.1.138`。
- `backend/internal/handler/openai_gateway_handler.go` 冲突保留 dev-zz 的 `openAIUsageUpstreamEndpoint` 口径，继续优先使用真实转发结果中的上游端点。
- `frontend/src/components/admin/usage/UsageStatsCards.vue` 吸收上游缓存 tooltip 功能，同时保留 dev-zz 的 stone / emerald 样式。

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

## 2026-06-21 - 上游 main 同步：thinking 协议、兜底定价与账号 ID

范围：
- `backend/internal/handler/{gateway_handler.go,gateway_handler_intercept_test.go,auth_oauth_pending_flow_test.go}`
- `backend/internal/server/middleware/{api_key_auth.go,api_key_auth_test.go}`
- `backend/internal/service/{auth_email_binding.go,billing_service.go,gateway_*.go,openai_*.go,ratelimit_service.go,thinking_protocol.go}`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `docs-site/dev-zz/{changelog.md,patches.md,maintenance/merge-log.md}`

改动：
- 合并上游 `main` 到 `dev-zz`，上游 head 为 `945b9b20`。
- 吸收邮箱绑定后缀白名单校验，使发送绑定验证码和实际绑定都走注册邮箱策略。
- API Key IP ACL 拒绝响应现在包含客户端 IP；空 IP 以 `unknown` 展示。
- 网关保留 SSE `event:error` 真实响应体用于运维日志，并补强 haiku 探针、OpenAI/Gemini/WebSocket/Responses 兼容路径。
- 新增 thinking 协议识别：Anthropic 官方 strict 路径继续剥离无效签名 thinking block，DeepSeek / Kimi / GLM / MiniMax / Qwen thinking 等 passback-required 上游保留历史 thinking block，避免破坏第三方 Anthropic 兼容协议。
- 合并 DeepSeek V4、GLM、Kimi、MiniMax、Kimi coding 和 Doubao embedding vision 的兜底定价，并为图文不同价 embedding 增加图片输入 token 单价。
- Anthropic 官方 5h / 7d 窗口耗尽时优先持久化真实 reset 冷却，避免被宽泛 429 临时不可调度规则缩短。
- 管理端账号列表新增账号 ID 列和排序能力；dev-zz 的表格选择按钮样式保持不变。
- `backend/cmd/server/VERSION` 冲突按 dev-zz 发布线保留 `1.1.6`，没有采用上游 `0.1.137`。

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

## 2026-06-19 - v1.1.5 可用渠道 null 数组容错

范围：
- `backend/cmd/server/VERSION`
- `backend/internal/handler/admin/channel_handler.go`
- `frontend/src/api/{channels.ts,admin/channels.ts}`
- `frontend/src/components/channels/AvailableChannelsTable.vue`
- `frontend/src/views/user/AvailableChannelsView.vue`
- `frontend/src/utils/{availableChannelsCatalog.ts,__tests__/availableChannelsCatalog.spec.ts}`
- `docs-site/dev-zz/{changelog.md,patches.md,deployment/deploy-dev-zz.md,reference/configuration-and-migrations.md}`

改动：
- 修复管理员进入「可用渠道」时，管理端全量目录响应中 `groups` / `platforms` / `intervals` 为 `null` 导致前端 `.filter()` 崩溃、页面主体空白的问题。
- 用户侧与管理端可用渠道 API 响应在前端入口统一归一化数组字段，历史或异常响应里的 `null` 会被当作空数组处理。
- 可用渠道搜索、表格分组和导出行构建逻辑增加容错，避免绕过 API 入口的数据再次触发空白。
- 后端管理端全量目录对空 `platforms` / `groups` 返回 `[]`，不再编码成 JSON `null`。
- `VERSION` 更新为 `1.1.5`，固定版本镜像示例同步为 `thornboo/sub2api:1.1.5`。

验证：
- `pnpm --dir frontend test:run src/utils/__tests__/availableChannelsCatalog.spec.ts`
- `pnpm --dir frontend run build`
- `go test ./internal/handler/admin ./internal/handler -run 'Available|Channel' -count=1`
- `git diff --check`

未验证：
- 新发布镜像的正式环境浏览器 smoke，待 Release workflow 构建完成后在生产容器更新后验证。

## 2026-06-19 - v1.1.4 白屏修复边界收敛

范围：
- `backend/cmd/server/VERSION`
- `frontend/index.html`
- `frontend/src/main.ts`
- `docs-site/dev-zz/{changelog.md,patches.md,maintenance/frontend-white-screen-2026-06-17.md}`
- `docs-site/dev-zz/{deployment/deploy-dev-zz.md,reference/configuration-and-migrations.md}`

改动：
- 移除 v1.1.3 中额外加入的 HTML 级“前端加载失败”兜底页，避免把网络慢、资源中断等非本次事故问题表现为错误页。
- 恢复 `frontend/src/main.ts` 为单纯 `bootstrap()`，删除 `sub2api:bootstrap-error` 自定义事件链路。
- 将 2026-06-17 白屏事故复盘收敛为根因修复：删除错误的手写 `manualChunks` 拆包，避免生产构建出现 ESM chunk 循环初始化错误。
- `VERSION` 更新为 `1.1.4`，固定版本镜像示例同步为 `thornboo/sub2api:1.1.4`。

验证：
- `pnpm -C frontend run build`
- `git diff --check`

未验证：
- 新发布镜像的浏览器 smoke，待 Release workflow 构建完成后在测试或正式环境验证。

## 2026-06-17 - 已删除 Key 证据展示阶段 1

范围：
- `backend/internal/handler/admin/usage_handler.go`
- `backend/internal/handler/dto/{mappers.go,types.go}`
- `backend/internal/repository/{api_key_repo.go,usage_log_repo.go}`
- `backend/internal/service/api_key.go`
- `backend/internal/handler/admin/usage_handler_search_users_test.go`
- `backend/internal/handler/dto/mappers_deleted_api_key_test.go`
- `backend/internal/repository/usage_log_repo_deleted_user_integration_test.go`
- `frontend/src/views/admin/UsageView.vue` 及 admin usage 组件
- `docs-site/dev-zz/features/usage-ledger-evidence-integrity.md`

改动：
- 仅在管理员证据视图（`/admin/usage`）穿透软删除解析 Key 名称和删除状态，hydrate 已删除 Key 时返回 `deleted` / `deleted_at`，不改变用户侧 `/usage` 的 hydration 口径。
- DTO 隐藏已删除 Key 的明文 key，仅向管理员证据上下文暴露删除元数据；导出补充 Key ID、名称和删除时间。
- usage_logs 被定位为不可变消费账本，维度对象软删除不影响历史明细数值；阶段 2（快照字段）和阶段 3（外键约束）保持设计阶段。

验证：
- `go test ./internal/repository ./internal/handler/admin ./internal/handler/dto ./internal/service ./internal/server/middleware -count=1`
- `pnpm -C frontend test:run src/components/admin/usage/__tests__/UsageObjectFilterPicker.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts src/views/admin/__tests__/UsageView.spec.ts`
- `pnpm -C frontend typecheck`
- `git diff --check`

未验证：
- 依赖 testcontainers/Postgres 的 repository 集成测试本地无 rootless Docker 未跑，新增集成断言以 CI 或带 Docker 环境为准。

## 2026-06-17 - 管理员用量日期范围可共享

范围：
- `frontend/src/views/admin/UsageView.vue`
- `frontend/src/views/admin/__tests__/UsageView.spec.ts`

改动：
- `/admin/usage` 显式修改日期范围时把所选区间回写到路由 query，刷新和分享链接保留时间口径。
- 首次无 query 加载保持干净 URL，内部使用默认日期，不把默认值写进 URL。
- 初始 route 规范化与用户显式筛选改动保持分离，避免干净 URL 行为被意外改变。

验证：
- `pnpm --dir frontend test:run src/views/admin/__tests__/UsageView.spec.ts`
- `pnpm --dir frontend typecheck`
- `git diff --check`

未验证：
- 浏览器运行时冒烟，由用户在自有前后端服务验证。

## 2026-06-17 - v1.1.2 发布与镜像备份优先更新

范围：
- `backend/cmd/server/VERSION`
- `docs-site/dev-zz/deployment/deploy-dev-zz.md`
- `docs-site/dev-zz/reference/{change-map.md,configuration-and-migrations.md}`
- `docs-site/dev-zz/testing/verification-matrix.md`

改动：
- `VERSION` 更新为 `1.1.2`，固定版本镜像示例同步为 `thornboo/sub2api:1.1.2`。
- 部署文档把 dev-zz 镜像更新流程改为备份优先：先 `deploy/backup-dev-zz.sh` 备份，再 `docker compose pull sub2api` 并只重建应用容器，不执行 `down -v`，不删除 `.env` 和数据目录。
- 同步配置/迁移索引、变更地图和验证矩阵中的镜像版本与备份脚本口径。

验证：
- `git diff --check`
- 文档复核镜像名、版本号、备份脚本和数据目录保护口径

## 2026-06-15 - docs-site 全量重构与 dev-zz 变更索引

范围：
- `docs-site/.vitepress/config.ts`
- `docs-site/index.md`
- `docs-site/project/{index.md,overview.md}`
- `docs-site/dev-zz/{index.md,branch-policy.md,changelog.md,patches.md}`
- `docs-site/dev-zz/reference/{change-map.md,api-surface.md,configuration-and-migrations.md}`
- `docs-site/dev-zz/testing/verification-matrix.md`

改动：
- 基于 `origin/main...dev-zz` 重新盘点分支差异，记录当前 HEAD `3a7d0474`、上游 `origin/main` `e34ad2b1`、差异规模和变更分布。
- 新增 `change-map.md`，按企业 API Key、owner 用量分析、模型/渠道、UI/运维、部署发布、CI/运行时归纳 dev-zz 相对上游的主要二开范围。
- 新增 `api-surface.md`，把用户侧 API Key、公共 Key 状态查询、单 Key 用量下钻、owner analytics、可用渠道模型和管理端模型探测的接口路径、参数、权限和字段边界集中记录。
- 新增 `configuration-and-migrations.md`，记录 Go/Node/pnpm/docs-site 运行时口径、API Key 批量/标签配置、`151/152` 迁移、数据保留默认值、fork 镜像和 CI runtime 约束。
- 新增 `verification-matrix.md`，按文档、API Key、用量分析、可用渠道、模型探测、运维弹窗和分支级变更列出最小验证组合。
- 重写文档站首页、项目文档入口、项目说明、dev-zz 总览和分支策略，使 docs-site 明确承担“源项目文档 + dev-zz 二开档案”的职责。
- 更新 VitePress 顶部导航和侧边栏，新增变更地图、接口索引、配置/迁移索引和验证矩阵入口。

验证：
- `pnpm --dir docs-site docs:build`
- `git diff --check`

## 2026-06-15 - API Key 状态与分组更新语义

范围：
- `backend/internal/handler/api_key_handler.go`
- `backend/internal/handler/api_key_handler_test.go`
- `backend/internal/service/api_key_service.go`
- `backend/internal/service/api_key_batch_test.go`
- `frontend/src/api/keys.ts`
- `frontend/src/types/index.ts`
- `frontend/src/views/user/KeysView.vue`
- `docs-site/dev-zz/{changelog.md,patches.md,features/enterprise-key-member-management.md,reference/api-surface.md,testing/verification-matrix.md}`

改动：
- API Key 可写禁用状态统一为 `disabled`；`inactive` 只作为 legacy alias 接收并归一化为 `disabled`。
- 单把更新 handler 的 `status` binding 增加 `disabled`，前端状态选项、筛选项和 toggle 操作也改用 `disabled`。
- 新增 `ErrAPIKeyStatusInvalid` 和状态归一化函数，避免在 service 层继续散落 `inactive` 判断。
- `quota_exhausted` 与 `expired` 作为系统派生状态保留；前端编辑这些 Key 时，如用户没有显式改状态，不会把它们保存成 `disabled`。
- 单把 Key 编辑时，仅当 `group_id` 真实变化才重新检查 owner 是否可绑定目标分组。只改标签、额度、过期、限流或 IP ACL 时，不会因为历史绑定分组当前不可绑定而失败。
- 批量筛选状态同样归一化：`inactive` 作为输入别名，最终筛选 `disabled`。
- 错误提示优先展示后端返回的 `detail` 或 `message`，方便用户看到 `GROUP_NOT_ALLOWED` 等具体原因。

验证：
- `mise x -C backend -- go test ./internal/service -run 'APIKeyServiceUpdate|APIKeyServiceBatchUpdate' -count=1`
- `mise x -C backend -- go test ./internal/handler -run 'TestAPIKeyHandlerUpdateAcceptsDisabledStatus' -count=1`
- `pnpm --dir frontend typecheck`
- `git diff --check`
- 用户手动验证 API Key 禁用和标签编辑流程

未验证：
- 完整后端测试套件
- 前端 lint
- 浏览器 e2e

## 2026-06-15 - owner 用量分析落地

范围：
- `backend/internal/handler/usage_handler.go`
- `backend/internal/repository/usage_log_repo.go`
- `backend/internal/server/routes/user.go`
- `backend/internal/service/{api_key_analytics.go,usage_service.go}`
- `frontend/src/api/usage.ts`
- `frontend/src/components/user/UsageAnalyticsPanel.vue`
- `frontend/src/views/user/UsageView.vue`
- `frontend/src/components/{common/DataTable.vue,layout/TablePageLayout.vue}`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `docs-site/dev-zz/{changelog.md,patches.md,features/enterprise-usage-analytics.md,reference/api-surface.md}`

改动：
- 用户认证域新增 `/api/v1/usage/analytics/summary`、`leaderboard`、`models`、`groups`、`tags`、`trend`。
- owner analytics 统一从当前 `subject.UserID` 构造过滤条件，支持 `start_date`、`end_date`、`timezone`、`granularity`、`api_key_id`、`group_id`、`tags`、`status`、`search`、`limit`。
- 后端按当前 owner 的 `usage_logs` 和 `api_keys` 做聚合，不接受外部传入 `user_id`。
- summary 将历史时间范围聚合与当前 Key 实时治理快照分离，避免用户把当前 quota/限流状态误解为历史快照。
- leaderboard 返回 Key 名称、标签、分组、状态、请求数、Token、实际扣费、占比、环比和最后使用时间。
- models / groups / tags / trend 分别提供模型分布、分组统计、标签归因和趋势。
- tags 统计不返回 `share_percent`，因为多标签 Key 会重复计入每个标签。
- 前端用户 Usage 页面新增 analytics tab 和 `UsageAnalyticsPanel`，复用现有图表/表格风格，不引入新图表依赖。

验证：
- `go test ./internal/handler ./internal/server/routes ./internal/service`
- `go test ./internal/repository -run 'TestUsageLogRepositoryGetAPIKeyUsageTrendForUser'`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run lint:check`
- `git diff --check`

## 2026-06-15 - API Key 标签仓储契约与 CI 修复

范围：
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/api_key_repo_integration_test.go`
- `backend/internal/handler/api_key_handler_test.go`
- `backend/internal/service/api_key_batch_test.go`
- `.github/workflows/{backend-ci.yml,security-scan.yml}`
- `docs-site/dev-zz/{changelog.md,patches.md,reference/configuration-and-migrations.md}`

改动：
- 仓储层新增写入前归一化：`nil` tags 写成空数组，保证 `api_keys.tags` 持续满足 `jsonb` 数组约束。
- 修复 `ListTagsByUserID` 的 `rows.Close()` errcheck 问题，满足 golangci-lint 配置。
- 补齐 unit build tag 下 APIKeyRepository 扩展后的测试 stub，恢复 CI 对扩展 repository contract 的覆盖。
- GitHub Actions 在 backend CI 和 security scan 中设置 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true`，验证 JavaScript actions runtime 的 Node 24 兼容性。
- 保持前端构建 `setup-node` 为 Node 20，不把 action runtime 验证和项目 runtime 升级混在一起。

验证：
- `go test -tags=unit ./...`
- `gofmt`
- `git diff --check`

未验证：
- 部分提交在本地未重新跑完整 Go 集成测试，后续以 GitHub Actions 作为最终验证面。

## 2026-06-15 - fork 镜像发布与滚动部署口径

范围：
- `.github/workflows/release.yml`
- `.goreleaser.yaml`
- `README.md`
- `README_JA.md`
- `deploy/{.env.example,DOCKER.md,Dockerfile,README.md,config.example.yaml,docker-compose*.yml,docker-deploy.sh,install.sh,sub2api.service}`
- `docs-site/dev-zz/deployment/deploy-dev-zz.md`
- `docs-site/dev-zz/{changelog.md,patches.md,reference/configuration-and-migrations.md}`

改动：
- fork 镜像默认值改为 `thornboo/sub2api:latest`，并保留 `ghcr.io/thornboo/sub2api:latest` 作为可选镜像源。
- 部署脚本默认从 `thornboo/sub2api` 的 `dev-zz` 分支拉取部署文件，避免安装脚本继续指向上游或旧分支名。
- 明确上游镜像 `weishaw/sub2api:latest` 不包含 dev-zz 二开，不应作为本分支默认部署镜像。
- 补充已部署服务日常更新方式：拉取镜像并只重建 `sub2api` 容器，不执行 `down -v`，不删除 `.env`、`data/`、`postgres_data/`、`redis_data/`。
- 补充从早期本地源码构建镜像 `sub2api:dev-zz` 切换到发布镜像的备份、override、compose config、启动和回滚步骤。
- 将 `deploy/deploy-dev-zz.sh` 定位为开发验证、临时测试未发布代码或远程镜像不可用时的本地构建路径。
- 记录 v1.1.1 patch release 和固定版本镜像只适合验收、回滚或锁版本场景。

验证：
- `git diff --check`
- 文档说明复核镜像名、分支名和数据目录保护口径

## 2026-06-14 - 企业用量分析中心设计

范围：
- `DESIGN.md`
- `docs-site/.vitepress/config.ts`
- `docs-site/dev-zz/{index.md,changelog.md,patches.md}`
- `docs-site/dev-zz/features/{enterprise-key-member-management.md,enterprise-usage-analytics.md}`

改动：
- 新增根部 `DESIGN.md`，作为 dev-zz 后续 UI/UX 和分析型功能的设计取舍索引，记录产品目标、角色、视觉语言、组件复用、权限边界和实现约束。
- 新增 `enterprise-usage-analytics.md`，把企业 owner 自助分析、平台管理员全站分析、Key-only 员工查询和单 Key 下钻分层说明。
- 明确 owner 新接口应挂在用户认证域，强制绑定当前 `subject.UserID`，并使用独立 DTO 排除 `account_cost`、上游账号、渠道、`upstream_model` 等管理员专属字段。
- 明确员工 Key 排行、分组/标签统计、模型调用分布、趋势和异常面板作为下一阶段 owner 用量总览范围。
- 对“员工需要同时使用 OpenAI / Anthropic / Gemini”给出阶段性方案：短期可用标签归并多把物理 Key，长期推荐 Key Access Profile / 多分组访问范围，让一把员工 Key 绑定多个可用分组，同时保留 `api_keys.group_id` 兼容旧逻辑。
- 根据设计审查补强多分组 Key 的授权前置条件：阶段四设计取舍文档必须先梳理 `AllowedGroups`、订阅型分组、`api_keys.group_id`、auth snapshot 和 fallback group 的现有关系，禁止 Key 绑定到 owner 自身无权访问的分组。
- 明确 owner 统计契约：tags 聚合第一版不返回 `share_percent`，避免多标签重复计入时被误画为总和 100% 的占比；summary 将历史时间范围聚合与当前 quota / 限流实时快照分开。
- 修正 usage log 索引表述：现有 `user_id, created_at` 支撑 owner 时间范围扫描，但 `GROUP BY api_key_id` 等聚合仍需在 owner 时间窗内计算，真实数据量证明瓶颈后再评估复合索引或预聚合。
- 将企业 Key 成员管理、API Key 用量下钻、企业用量分析中心和设计取舍 0002 加入 docs-site 侧边栏，便于后续审查和实现查找。

验证：
- `pnpm --dir docs-site docs:build`
- `git diff --check`

## 2026-06-14 - API Key 用量下钻

范围：
- `backend/internal/{handler,repository,server,service}/**`
- `frontend/src/{api,components/keys,i18n,views/user}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,features/api-key-usage-drilldown.md}`

改动：
- 新增用户侧 `GET /api/v1/user/api-keys/:id/usage/trend`，支持按 `hour` / `day` / `week` / `month` 聚合单把 API Key 的请求数、Token 和实际扣费。
- 新趋势接口复用当前用户认证主体，并在 handler 层校验 Key 所有权、granularity 白名单和日期范围上限；绕过前端直接请求超大范围会返回 400。
- repository 新增单 Key 专用查询方法，使用 `created_at AT TIME ZONE $tz` 做分桶，不修改共享 `GetUsageTrendWithFilters` 路径，避免影响 dashboard 等既有调用点。
- 用户侧新增 `GET /api/v1/user/api-keys/:id/usage/models`，只在校验 Key 属于当前用户后返回脱敏模型统计；用户模型统计响应不包含 `cost` / `account_cost` 等管理员成本字段。
- 用户侧 Key 列表的用量列新增详情入口，弹窗内提供趋势图表、模型分布和请求记录表；请求记录面板直接复用已有 `/usage` 查询接口并绑定 `api_key_id`。
- 趋势表、模型表和请求记录表对大 Token 数使用 K/M/B 紧凑展示，并保留完整数值悬停提示。
- 周粒度展示 ISO 周编号并补充自然日期区间，便于定位具体周范围。
- 根据审查反馈补强前端面板请求竞态防护，快速切换粒度、刷新模型分布或翻页请求记录时会丢弃过期响应。
- `GetAPIKeyModelStats` service 方法改为同时接收 `userID` 和 `apiKeyID`，与趋势和日用量路径保持双重过滤的纵深防御。
- 本轮复用项目已有图表依赖，不实现 API Key 列表按用量排序。

验证：
- `go test ./internal/handler ./internal/server/routes ./internal/service`
- `go test ./internal/repository -run 'TestUsageLogRepositoryGetAPIKeyUsageTrendForUser'`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run lint:check`
- `git diff --check`

## 2026-06-14 - 企业 Key 筛选批量操作

范围：
- `backend/internal/{handler,service}/**`
- `frontend/src/{i18n,types,views/user}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,features/enterprise-key-member-management.md}`

改动：
- 用户侧 `POST /api/v1/keys/batch-update` 和 `POST /api/v1/keys/batch-delete` 支持 `apply_to=filtered`，可对当前筛选条件匹配的 Key 执行批量改/删。
- 筛选批量支持 `search` / `status` / `group_id` / `tags`，要求至少一个筛选条件，避免空筛选误操作全量 Key。
- 后端先将筛选结果解析为当前 owner 名下的 Key ID 集合，并限制单次最多 500 个，再复用现有按 ID 批量事务、越权检查和缓存失效链路。
- 当前用户侧 Key 页面仍以列表勾选作为批量修改 / 删除入口，不在筛选下拉选择时自动显示批量操作。
- 本轮不引入子账号 / 员工登录实体，也不改变设计取舍 0002 的 Key-as-member 边界。

验证：
- `mise x -C backend -- go test ./internal/service -run 'TestAPIKeyServiceBatch(Update|Delete)'`

## 2026-06-14 - 企业 Key 标签候选

范围：
- `backend/internal/{handler,repository,server,service}/**`
- `frontend/src/{api,types,views/user}/**`

改动：
- 新增用户侧只读接口 `GET /api/v1/keys/tags`，返回当前 owner 未删除 Key 的去重标签候选。
- 标签候选查询绑定当前 `user_id`，过滤软删除 Key，并限制单次最多返回 500 个标签。
- 用户侧 Key 页面进入时加载完整标签候选，标签筛选下拉不再依赖当前分页已加载过的标签。

验证：
- `mise x -C backend -- go test ./internal/handler ./internal/repository ./internal/server/routes ./internal/service`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `mise x -C backend -- go test ./internal/...`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

## 2026-06-13 - 企业 Key 标签管理

范围：
- `backend/{ent,migrations,internal}/**`
- `frontend/src/{api,i18n,types,views/user}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,features/enterprise-key-member-management.md}`

改动：
- 在 `api_keys` 新增 `tags` jsonb 字段，默认空数组，并用 `_notx` migration 创建部分 GIN 索引支持 owner 侧标签筛选。
- 用户侧 Key 列表支持 `tags` 查询参数，多个标签按“同时包含”过滤。
- 单把创建 / 编辑、批量创建和批量更新均支持标签；批量更新支持 `set` / `add` / `remove` / `clear` 四种标签操作。
- 后端统一规范化标签：trim、小写化、去重，最多 20 个标签，单个最多 40 个字符。
- 用户侧 `KeysView.vue` 增加标签筛选、表格标签展示、批量创建结果标签列和 CSV 导出标签字段。
- 本轮不引入子账号 / 员工登录实体，也不实现“对全部筛选结果执行批量操作”；批量维护仍限定为已选择的 Key ID。

验证：
- `mise x -C backend -- go test ./internal/service -run 'Test(APIKeyServiceBatch|BuildBatchAPIKeyNames|NormalizeAPIKeyTags)'`
- `mise x -C backend -- go test ./internal/...`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

## 2026-06-13 - 企业 Key 批量维护

范围：
- `backend/internal/{handler,repository,server,service}/**`
- `frontend/src/{api,i18n,types,views/user}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,features/enterprise-key-member-management.md}`

改动：
- 用户侧 Key 列表新增按 `api_keys.id` 勾选的批量操作栏，批量动作只提交 ID，不依赖名称或脱敏 Key，避免同名 Key 或 Key 展示脱敏导致误操作。
- 新增 `POST /api/v1/keys/batch-update`，支持统一修改分组、状态、quota、过期时间、5h/1d/7d 限流、限流窗口用量和 IP 黑白名单。
- quota 批量更新支持设置固定额度、追加额度和改为无限制；过期时间支持统一设置或清空。
- 新增 `POST /api/v1/keys/batch-delete`，对选中 Key 做批量软删除。
- 批量更新和批量删除均先校验全部 ID 属于当前用户，再在单个事务内执行；任一写入失败时整批回滚。
- 事务提交后再失效认证缓存；重置限流用量时同步失效 Redis 限流缓存。
- 前端批量创建结果表为每把新 Key 增加单独复制按钮，保留复制全部与 CSV 导出。
- 本轮不引入 `api_keys.tags`，也不实现按筛选条件批量操作；当前批量维护范围限定为页面勾选的 ID 集合。

验证：
- `go test ./internal/service -run 'Test(APIKeyServiceBatch|BuildBatchAPIKeyNames)'`
- `go test ./internal/server/routes -run 'TestUserRoutesAPIKeyBatchPathsAreRegisteredBeforeIDRoute'`
- `go test ./...`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend build`
- `git diff --check`

## 2026-06-13 - 企业 Key 批量创建

范围：
- `backend/internal/{handler,repository,server,service}/**`
- `frontend/src/{api,i18n,types,views/user}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,decisions/adr-0002-key-as-enterprise-member.md,features/enterprise-key-member-management.md}`

改动：
- 新增用户侧 `POST /api/v1/keys/batch`，支持按名称模板或名称列表批量创建 API Key，并统一配置分组、quota、有效期、5h/1d/7d 限流和 IP 黑白名单。
- 批量创建在 service 层集中校验并通过 repository 事务一次性写入，任意一把失败时整批回滚；Key 唯一冲突做有界重试，事务提交后再失效认证缓存和编译 IP 规则。
- 新增设置项 `api_key_batch_create_max_count`，默认 `200`，服务端硬上限 `500`。
- 批量创建使用用户写幂等，但成功记录落库前会脱敏完整 Key；首次响应展示完整 Key，幂等重放只返回不可再次展示明文的摘要。
- 用户侧 Key 页面新增批量创建弹窗、结果弹窗、一次性明文提示、复制全部和包含完整字段的 CSV 导出。
- 阶段一不修改 `api_keys` schema，不引入子账号实体，不影响个人用户已有 Key 的认证、扣费、限流和使用链路。

验证：
- `go test ./internal/service ./internal/handler ./internal/server/routes ./internal/repository`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run lint:check`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

## 2026-06-13 - Key 自助状态查询

范围：
- `backend/internal/{handler,repository,server,service}/**`
- `frontend/src/{api,types}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,decisions/adr-0002-key-as-enterprise-member.md,features/enterprise-key-member-management.md}`

改动：
- 作为企业 Key 成员管理阶段一的补充需求，新增公共只读 `POST /api/v1/key/status`，允许只有 Key、没有站点账号的员工查询本人 Key 状态、quota 用量、过期时间、最近使用和限流配置。
- 查询结果只返回当前 Key 自身信息，不返回 owner 账号余额、邮箱、角色、其它 Key 或企业全局数据。
- 查询不走网关认证缓存，不更新 `last_used_at`，不扣 quota，不改限流窗口，只做读查询和状态推导。
- 同一 Key 10 秒内限查一次，限流标识使用 Key 哈希；Redis 冷却写入失败时 fail-close 返回不可用，不静默降级为多实例不一致的进程内限流。
- 路由层叠加 IP 级 `30/min` fail-close 限流，降低暴力枚举风险。

验证：
- `go test ./internal/service ./internal/handler ./internal/server/routes ./internal/repository`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run lint:check`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

## 2026-06-13 - 运维明细弹窗栈与筛选体验优化

范围：
- `frontend/src/components/common/{BaseDialog,Select}.vue`
- `frontend/src/components/common/__tests__/{BaseDialog,Select}.spec.ts`
- `frontend/src/views/admin/ops/**`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `docs-site/dev-zz/{changelog.md,patches.md}`

改动：
- 将通用 `BaseDialog` 升级为模块级弹窗栈，自动按有效 z-index 分层，并确保 Escape、遮罩点击、关闭按钮和 body 滚动锁只由视觉最上层弹窗接管。
- 将 Ops 运维看板的请求详情、错误列表和单条错误详情状态抽取到 `useOpsModalStack`，支持父级明细弹窗与子级错误详情叠加打开，关闭子级不再连带关闭父级。
- 修复通用 `Select` 在弹窗等 `@click.stop` 容器内点击外部无法收起的问题，改用捕获阶段监听和真实 DOM ref 判断外部点击。
- 优化错误明细筛选区布局，为搜索、状态码、错误阶段、归属方和显示范围提供明确标签，并将搜索占位文案改为用户可读描述。
- 为错误列表取数增加请求序号，快速切换请求/上游错误类型时丢弃过期响应，避免旧数据覆盖新数据。
- 让单条错误详情的响应内容和关联上游响应预览使用阅读型自动换行代码块，保留 JSON 缩进和纵向滚动，移除横向阅读负担。

验证：
- `pnpm --dir frontend test:run src/components/common/__tests__/BaseDialog.spec.ts src/components/common/__tests__/Select.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts src/views/admin/ops/composables/__tests__/useOpsModalStack.spec.ts`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `git diff --check`

## 2026-06-12 - 上游 main 同步：合规确认与网关修复

范围：
- `.gitignore`
- `backend/internal/{handler,server,service,pkg}/**`
- `backend/migrations/150_account_group_scheduler_indexes_notx.sql`
- `docs/legal/**`
- `frontend/src/{api,components,composables,i18n,router,stores,views}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,maintenance/merge-log.md}`

改动：
- 合并上游管理端部署与运营合规确认 gate，包括后端接口/中间件、前端确认弹窗、合规状态 store、公开法律文档和中英文文案。
- 合并上游网关正确性修复：错误透传/非流式错误帧重复写入保护、`MarkResponseCommitted` 覆盖、OpenAI failover 模型请求体替换，以及 idempotency 响应 UTF-8 截断。
- 合并上游 Bedrock / Claude 兼容修复、账号分组调度索引优化、调度日志循环优化、`claude-fable-5` 常量与 sponsor 资料更新。
- 解决 `.gitignore` 冲突时同时保留 dev-zz 的 `docs-site` 缓存忽略规则和上游 `docs/legal/*.md` 反忽略规则。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$"`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/keys/__tests__/UseKeyModal.spec.ts src/api/__tests__/client.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/server/middleware ./internal/handler ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/pkg/apicompat ./internal/pkg/openai`

## 2026-06-10 - dev-zz 文档中心迁移

范围：
- `.gitignore`
- `deploy/deploy-dev-zz.sh`
- `docs-site/package.json`
- `docs-site/index.md`
- `docs-site/.vitepress/config.ts`
- `docs-site/project/**`
- `docs-site/dev-zz/**`
- `docs/LOCAL_DEVELOPMENT.md`
- `docs/AVAILABLE_CHANNELS_MODEL_MARKETPLACE_PLAN.md`
- `secondary-dev/**`

改动：
- 把 `docs-site/` 从一个生成的镜像目录改造为 `dev-zz` 的源文档中心。
- 在 `docs-site/project/` 下新增结构化项目文档。
- 将二开记录迁移到 `docs-site/dev-zz/`，包括变更记录、补丁说明、分支策略、部署文档、合并流程、合并记录、功能规划，以及文档中心的设计取舍文档。
- 把 dev-zz 源码构建部署脚本移到 `deploy/deploy-dev-zz.sh`。
- 移除生成内容的同步脚本，并取消 `secondary-dev/` 作为独立文档目录。
- 把本地开发和可用渠道模型广场规划文档移入 `docs-site/dev-zz/`。

验证：
- `pnpm --dir docs-site docs:build`
- `bash -n deploy/deploy-dev-zz.sh`
- `git diff --check`

## 2026-05-06 - 首页官方模型价格

范围：
- `frontend/src/views/HomeView.vue`
- `docs-site/dev-zz/changelog.md`
- `docs-site/dev-zz/patches.md`

改动：
- 把首页热门模型展示价格从 85% 折扣值恢复为官方价格。
- 保留原有的中英文价格说明：实际价格以折扣后的分组价格为准。

验证：
- `rg -n -F '$5/M input tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$30/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$25/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$2/M input tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$12/M output tokens' frontend/src/views/HomeView.vue`
- `git diff --check -- frontend/src/views/HomeView.vue docs-site/dev-zz/changelog.md docs-site/dev-zz/patches.md`

## 2026-05-06 - 首页折扣模型价格

范围：
- `frontend/src/views/HomeView.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`

改动：
- 把首页热门模型展示价格从官方价的 80% 调整为 85%。
- 把中文价格说明从“实际以分组定价为准”改为“实际以优惠后分组价格为准”。
- 把英文价格说明从 "Actual price follows group pricing" 改为 "Actual price follows discounted group pricing"。

验证：
- `cd frontend && pnpm run typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check -- frontend/src/views/HomeView.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts`

## 2026-05-06 - 映射模式清空全部模型

范围：
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal}.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`

改动：
- 为创建/编辑账号模型映射区新增“清除所有模型” / "Clear all models" 操作。
- 覆盖普通账号映射区、Bedrock 映射区，以及 Antigravity 的仅映射账号区。
- 清空映射时保持当前映射模式 UI 激活，移除所有映射行，清空映射目录输入状态，并清除探测的“新增/缺失”标记。
- 新增一个编辑弹窗回归测试：清空映射行后，验证保存的凭据不再包含 `model_mapping` 或 `model_restriction_mode`。

验证：
- `cd frontend && pnpm test:run src/components/account/__tests__/EditAccountModal.spec.ts`
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check`

## 2026-05-06 - 模型探测映射填充

范围：
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal}.vue`
- `frontend/src/components/account/ModelWhitelistSelector.vue`
- `frontend/src/components/account/ModelCatalogSearch.vue`
- `frontend/src/components/account/channelModelRecommendations.ts`
- `frontend/src/components/account/modelCatalog.ts`
- `frontend/src/i18n/locales/{zh,en}.ts`

改动：
- 为创建/编辑账号模型映射区新增已有的“获取支持模型” / "Fetch supported models" 操作。
- 探测到的上游模型 ID 以同名映射行（`model -> model`）追加，不覆盖已有的源模型映射，管理员可手动调整目标侧。
- 复用已有的后端探测接口、凭据解析、加载状态、去重处理和失败提示。
- 映射模式下的探测比对现在评估右侧的上游目标模型，标记新增的行，以及最新上游模型列表未返回的行。
- 当存在模型映射数据时，保存的凭据会包含 `model_restriction_mode`，使同名映射行能以映射模式重新打开，而不被推断为白名单。
- 映射快速添加的推荐现在来自所选分组的渠道配置：优先用渠道模型映射目标，未配置映射时回落到渠道定价模型。
- 自定义模型输入框新增基于公开 models.dev 目录的“查询” / "Search" 操作。选中结果会填入输入框；管理员仍需显式点击“填入”或“添加同名映射”才会应用。

验证：
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check`

## 2026-05-05 - 账号模型探测

范围：
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/account_handler_probe_models_test.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal,ModelWhitelistSelector}.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`

改动：
- 新增 `POST /api/v1/admin/accounts/probe-models`，用于管理员专属、不持久化地探测 OpenAI 兼容的上游模型列表。
- 后端从传入的 HTTPS Base URL 构造 `/v1/models` 请求，为防御 SSRF 拦截解析到私有/本地/链路本地地址的主机，以 bearer token 发送当前 API key，解析 `data[].id`，并返回去重后的模型 ID，不记录也不持久化凭据。
- 在创建/编辑账号白名单选择器中，于“填入相关模型” / "Fill related models" 之前新增“获取支持模型” / "Fetch supported models" 按钮。
- 创建/编辑对话框会尽量使用当前表单凭据，对 Bedrock/服务账号流程隐藏探测操作，把探测到的模型追加到当前白名单，并在失败时回落到清晰的提示，让管理员可以继续手动填模型。

验证：
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `mise x -C backend -- go test ./internal/handler/admin ./internal/server`
- `git diff --check`

## 2026-05-05 - 首页与控制台 UI 焕新

范围：
- `frontend/src/views/HomeView.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `frontend/src/views/auth/{LoginView,RegisterView}.vue`
- `frontend/src/components/auth/*OAuthSection.vue`
- `frontend/src/style.css`
- `frontend/src/components/common/*`
- `frontend/src/components/layout/*`
- `frontend/src/views/admin/*`
- `frontend/src/views/admin/ops/*`
- `frontend/src/views/user/*`

改动：
- 把首页改造为当前的明暗视觉方向，包含模型卡片、快捷入口、用户推荐、FAQ 折叠面板和简化的页脚。
- 从首页相关入口移除公开的 GitHub 导航。
- 将“查看更多模型”指向 `/available-channels`。
- 用 stone/neutral/emerald 主题重新设计控制台布局基础组件和高频的管理端/用户端页面。
- 将 `DateRangePicker` 和管理端用量列设置通过 portal 渲染到 `body`，避免在可滚动的表格/卡片容器内被裁切。
- 修正 `HelpTooltip` 的 fixed 定位坐标，使滚动位置不再偏移运维监控卡片的提示。
- 把首页可见的硬编码中文文案移入 i18n key，并让代码示例使用当前站点 origin。
- 仅在日期范围和用量列设置菜单打开时绑定全局监听，并对位置更新器保留关闭状态的守卫。
- 重做共享认证布局以及登录/注册页的强调色，使其匹配首页的 stone/emerald 主题，包括主题/语言控件。
- 仅在前端隐藏 LinuxDo 和微信认证平台 UI：登录/注册 OAuth 按钮、资料绑定卡片/来源提示，以及管理端认证设置/来源默认值。后端路由和设置数据保持不变。

验证：
- `cd frontend && pnpm vitest run src/components/common/__tests__/HelpTooltip.spec.ts`
- `cd frontend && pnpm vitest run src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts`
- `cd frontend && pnpm typecheck`
