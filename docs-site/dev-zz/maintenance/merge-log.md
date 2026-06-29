# 上游合并记录

这里记录二开分支吸收上游变更的同步工作。

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
