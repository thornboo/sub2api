# 上游合并记录

## 2026-08-29 - 继续同步上游 `main`：模型级限流、计费口径与账号列表轻量刷新

分支：

- 目标：`dev-zz`
- 上游：本地 `main` / `origin/main`
- Base：`7b693ae4295e20329f18ff451b29a38879cb4705`
- 合并前目标：`73753278f8e071cf782341870307b0c8de213cf3`
- 上游 head：`b5827cfd54d58c248a9480b800444d0b40f0c6ea`
- 备份：`backup/dev-zz-pre-main-20260829-73753278`
- 结果：本次最终合并提交

上游要点：

- OpenAI HTTP / WebSocket 流式 429 保留最终模型归因，普通模型只进入模型级冷却；OAuth Spark 继续读取明确的 5h / 7d reset。Responses passthrough 在首个输出前发送 SSE keepalive，并保留 failover 时的模型 scope。
- DeepSeek 官方高峰 / 低峰价格、带后缀渠道模型的定价优先级、Fable 模型专属调度阈值、Claude Code sticky / attribution / tool arguments、Grok prompt cache identity 和 Antigravity mixed built-in tools 同步修复。
- 账号页增加上游账单倍率轻量 ETag 刷新，避免定时刷新整页账号列表；批量编辑可显式关闭 Codex fingerprint。账号创建 / 编辑继续保留 dev-zz 的供应商、资金池、模型 mapping / probe / sync 合同。
- 同步订阅周 / 月 reset 展示、智谱 GLM Coding Plan 配额、monitor singleflight、SMTP TLS 测试、支付币种、版本比较和分组错误展示修复。

合并策略与冲突：

- 合并前重新读取 `docs-site/dev-zz` 分支策略、合并流程、补丁 / 合并记录、变更地图和验证矩阵；fetch 后以 `7b693ae42` 为 merge-base 执行 `git merge-tree` 预演，创建本地恢复分支后运行 `git merge --no-commit origin/main`，不推送、不发布、不部署。
- 上游范围共 40 个提交，最终业务增量修改 74 个文件；预演和真实合并均报告 6 个文本冲突：`backend/internal/server/routes/admin.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/service/openai_ws_forwarder_ingress.go`、`backend/internal/service/openai_ws_forwarder_support.go`、`frontend/src/api/admin/accounts.ts`、`frontend/src/views/admin/AccountsView.vue`。
- 管理路由与前端 API 导出取并集，同时保留归档、供应商 / 资金池接口和新增账单倍率接口。账号页保留 dev-zz 供应商成本视图、迟到响应隔离和局部成本上下文刷新，并接入上游 ETag 快照刷新，不恢复整页轮询。
- Passthrough / WebSocket 同时保留 dev-zz 首轮可 failover、后续轮次 no-replay / 客户端重连合同，并吸收 canonical model 归因与 Spark 429 header 语义。普通 API Key 握手 429 的旧测试夹具补齐模型级写入断言，确认不会扩大成账号级限流或写入 OAuth 配额快照。
- 首次 Hosted CI 暴露无冲突标记的 Spark 429 判定顺序回归：通用 provider-model failure 抢先返回，使专用 Spark 配额分支无法运行。后续修复保持管理员显式 temp-unsched 规则优先，将 Spark 专用 429 提前到通用模型故障策略之前，恢复配额窗口、瞬时短冷却和不误报 failover 的既有合同。
- `VERSION` 保持 fork `1.7.42`；本轮没有数据库迁移、配置项或依赖变化。

验证：

- Git whitespace、真实冲突标记和索引检查通过；WebSocket 429 合流回归、前端 5 个目标测试文件共 97 条用例及前端 typecheck 通过。
- 后端相关包全量测试、前端完整 ESLint 和 docs-site 生产构建在创建合并提交前执行并以最终命令结果为准。
- CI 修复后的 Spark HTTP / 流式 / 影子账号 429 四条定向回归与完整 `make test-unit` 通过；修复尚未推送，新的 Hosted CI 结果待提交后验证。

未验证：

- 未连接真实 provider，未运行 Testcontainers integration、浏览器人工 smoke 或完整 Docker 镜像；修复后尚无新的 Hosted CI 结果，未发布或生产部署。

## 2026-08-28 - 继续同步上游 `main`：推理强度证据、公开分组限制与流终态修复

分支：

- 目标：`dev-zz`
- 上游：`origin/main`
- Base：`efb46db0a960fdad94502b1c3a982a0051cf5245`
- 合并前目标：`097c0178671594f3a03ae07fc25be095fd659225`
- 上游 head：`7b693ae4295e20329f18ff451b29a38879cb4705`
- 备份：`backup/dev-zz-pre-main-20260828-097c0178`
- 结果：本次最终合并提交

上游要点：

- usage 同时保存客户端请求的推理强度和策略映射后实际发送的推理强度；普通用户只见请求值，管理员可对照请求值与上游值。
- OpenAI raw Chat 流在没有终态前被截断时不再伪装成功；未向客户端输出前允许受控 failover，已经输出后记录失败并禁止重放。HTTP 200 内的终止失败事件、WebSocket 客户端正常关闭和 cyber 策略检测同步修复。
- Anthropic / Bedrock 传输故障进入统一 Ops 归因与持久故障临时停调；企业成员先标记预算结果不明，既有 no-replay gate 继续阻止可能已产生上游消费的跨候选重放。
- 管理员可显式限制某个用户能绑定的公开分组；新增用户字段、管理 UI、认证缓存快照和两份追加迁移。模型广场继续使用 dev-zz 的公开客户安全目录，不因登录态扩展专属分组或用户倍率。
- 同步 OAuth 注册 promo code、EasyPay 相对支付 URL、OpenAI 图片能力冷却、Grok / DeepSeek 工具与 reasoning 兼容、quota refresh 和多模态客户端工具修复。

合并策略与冲突：

- 合并前重新读取 `docs-site/dev-zz` 分支策略、上游合并流程、补丁 / 合并记录、变更地图和验证矩阵；以 `efb46db0a` 为 merge-base 执行 `git merge-tree` 预演，创建本地恢复分支后运行 `git merge --no-commit origin/main`，不推送、不发布、不部署。
- 本轮上游范围共 59 个提交（24 个非 merge 提交）、137 个上游变更文件；预演与真实合并均报告 24 个文本冲突，位于 Ent 生成物、用户管理、usage SQL / DTO、Gateway 计费与错误归因、模型广场、管理用量页和注册页。
- 用户管理冲突合并 `account_type` / `enterprise_enabled` 与 `restrict_public_groups`，保留用户倍率变更事务、Key 停用和认证缓存失效；Ent 按最终 schema 重新生成，避免上游字段下标覆盖 fork 企业字段。
- usage 冲突保留 `session_id`、最终 `ActiveGroup`、企业成员 usage / budget 原子归因和已删除 Key 审计，同时加入 `requested_reasoning_effort` 的单条、批量、best-effort SQL 与 admin-only 上游映射展示。
- Gateway 冲突保留企业成员 capability mismatch、预算结果不明、`submitGatewayUsageRecordTask` 和 no-replay 边界，同时吸收 requested effort 绑定、raw stream 截断、跨供应商 reasoning、HTTP 200 失败终态和传输故障观测修复。
- 模型广场继续返回匿名 / 登录一致的 active、standard、非专属客户安全渠道目录；上游“登录后展示授权专属分组 / 用户倍率”的分支不进入该公开接口。公开分组限制只影响实际 Key 绑定和登录控制台的授权面。
- 注册页只吸收 Email / GitHub / Google OAuth 的 promo code 保留；LinuxDo / 微信注册入口继续按 dev-zz 文档隐藏。`VERSION` 保持 fork `1.7.41`，不采用上游 `0.1.183`。

冲突路径：

- Ent / 用户：`backend/ent/mutation.go`、`backend/ent/runtime/runtime.go`、`backend/internal/handler/admin/user_handler.go`、`backend/internal/handler/dto/mappers.go`、`backend/internal/service/admin_service.go`、`backend/internal/service/admin_user.go`、`backend/internal/service/api_key_service.go`、`backend/internal/service/user.go`。
- Gateway / usage：`backend/internal/handler/gateway_handler_chat_completions.go`、`backend/internal/handler/gateway_handler_responses.go`、`backend/internal/repository/usage_log_repo_insert.go`、`backend/internal/repository/usage_log_repo_query.go`、`backend/internal/repository/usage_log_repo_request_type_test.go`、`backend/internal/repository/usage_log_session_id_unit_test.go`、`backend/internal/service/gateway_anthropic_apikey_passthrough_test.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/openai_gateway_chat_completions_raw.go`、`backend/internal/service/openai_gateway_usage.go`、`backend/internal/service/openai_upstream_transport_error.go`、`backend/internal/service/ratelimit_service.go`。
- 前端 / 目录：`backend/internal/handler/model_plaza_handler.go`、`backend/internal/handler/model_plaza_handler_test.go`、`frontend/src/views/admin/UsageView.vue`、`frontend/src/views/auth/RegisterView.vue`。

验证：

- 按最终 schema 重新生成 Ent；后端 handler / DTO / repository / service / payment / apicompat 定向包测试通过。
- 后端全仓 unit、`go vet ./...`、server build，前端全量 Vitest（301 个测试文件、2066 条用例）、typecheck / ESLint，以及 docs-site 生产构建通过。
- Git 无未解决索引项、无真实冲突标记，whitespace、合并父链和 `origin/main` 祖先检查通过。

未验证：

- 未连接真实 provider，未运行 Testcontainers integration、浏览器人工 smoke、完整 Docker 镜像或 Hosted CI；未推送、发布或生产部署。

## 2026-08-26 - 继续同步上游 `main`：Codex 目录、模型元数据与 WebSocket 工具续链

分支：

- 目标：`dev-zz`
- 上游：本地 `main` / `origin/main`
- Base：`e2d9b823f63dc4e8f4014be3fd24a0a73e339867`
- 合并前目标：`170266d603e5b16de93150051a9343f901fd2aab`
- 上游 head：`efb46db0a960fdad94502b1c3a982a0051cf5245`
- 备份：`backup/dev-zz-pre-main-20260826-170266d6`
- 结果：本次合并提交

上游要点：

- Codex 模型目录改为按实际分组、平台、账号映射和 capabilities 生成 manifest，并补齐 ETag、CLI 版本、公开 / 控制端点与默认模型目录。
- 上游模型同步增加 routed catalog 元数据、models.dev capability 补全、账号 extra 快照保存，以及 404/405 时按已配置 mapping 模型继续同步 capability。
- OpenAI Responses / WebSocket 路径补充 Lite 请求规范化、stale native tool ID 清理、session-id header、quota 429 停用和工具 item ID 兼容测试。
- 同步邮箱别名并发守卫、Composite 平台聚合 SQL、Kimi concurrency 403 恢复、Antigravity token limit clamp、赞助商 logo 与前端 Codex 配置提示。

合并策略与冲突：

- 合并前重新读取 `docs-site/dev-zz` 的分支策略、合并流程、补丁 / 合并记录、变更地图和验证矩阵；以 `e2d9b823f` 为 merge-base 执行 `git merge-tree` 预演，创建 `backup/dev-zz-pre-main-20260826-170266d6` 后运行 `git merge --no-commit origin/main`，不推送、不发布、不部署。
- 本轮上游范围共 59 个提交（38 个非 merge 提交），最终 merge diff 为 117 个文件（含本次补丁、变更和合并记录）；预演与真实合并均报告 12 个文本冲突，集中在 Gateway handler / routes、Composite resolver、OpenAI 调度、模型白名单 UI 与上游模型同步。
- `upstream_models.go` 保留 dev-zz 的 `UpstreamModelDescriptor`、`supported_endpoint_types`、模型映射和账号模型探测语义，同时吸收上游 `SyncUpstreamModelCatalog`、models.dev 元数据补全、账号 extra 快照和 404/405 configured-mapping fallback。
- WebSocket HTTP bridge 同时保留 dev-zz 后续 turn 结果不明进入 enterprise member budget ambiguous 的边界，并吸收上游 compatible endpoint 上清理 `reasoning.effort=none` 与 stale tool-ID regression。
- `/backend-api/codex/models` 继续通过统一 `codexModelsHandler` dispatch：官方 OpenAI 仍可走 OpenAI live metadata，非 OpenAI / Composite 分组返回 Codex manifest，并保持企业成员候选编排、Composite 逐候选重解析和 route-lock / no-replay 合同。
- 前端账号创建、模型白名单和 Key 使用页吸收 Codex catalog UI / 配置提示；保留 dev-zz 模型 mapping 模式、余额 / 配额主动探测、账号保存后同步、stone 控制台方向和隐藏认证入口策略。
- `VERSION` 保持 fork `1.7.40`，不采用上游 `0.1.183`；本轮没有数据库迁移、配置项或依赖变化。

验证：

- 已完成 Git 结构检查：无未解决索引项、无真实冲突标记，`git diff --check` 与 `git diff --cached --check` 通过。
- 后端 Codex、Gateway、Composite、sticky、WebSocket、模型同步定向测试以及全仓 unit、`go vet ./...` 和 server build 通过。
- 前端 typecheck、完整 ESLint、生产构建与全量 Vitest 通过，共 299 个测试文件、2055 条用例。
- 文档站生产构建通过：`pnpm --dir docs-site docs:build`。

未验证：

- 未连接真实 provider，未运行 Testcontainers integration、浏览器人工 smoke、完整 Docker 镜像或 Hosted CI；未推送、发布或生产部署。

## 2026-08-25 - 继续同步上游 `main`：Gemini schema、Responses Lite 与 Grok CLI 身份

分支：

- 目标：`dev-zz`
- Base：`03e8ab41346b42de9ece4e3e5bfcb6ca2b8cb57e`
- 合并前目标：`32e2c5307594c7329b4b8ab00b29227198267cf7`
- 上游 head：`e2d9b823f63dc4e8f4014be3fd24a0a73e339867`
- 结果：本次合并提交

上游要点：

- Gemini Messages 工具 schema 递归移除不支持的 `deprecated`，把字符串、布尔、数字和 `null` enum 统一编码为 Gemini 可接受的字符串；包含对象或数组的 enum 整体丢弃，避免发送半有效定义。
- Responses Lite 工具已经被搬入 `input[].type=additional_tools` 后，仍视为存在工具，保留必须为 `false` 的 `parallel_tool_calls`，避免被后续无工具规范化误删并触发上游 `unsupported_value`。
- Responses rejected-field retry 在上游拒绝某类 input item 的 `status` 时，一次删除所有同类型 item 的 status，其他类型不受影响，避免逐项重试耗尽有限预算。
- Grok OAuth / CLI proxy 使用官方 workspace User-Agent 与 `0.2.120` identity；普通 API Key 和非 CLI 目标继续保留自身 User-Agent，不被伪装为 CLI。

合并策略与冲突：

- 合并前重新读取当前 `docs-site/dev-zz` 分支策略、合并流程、补丁 / 合并记录、变更地图和验证矩阵；fetch 后以 `03e8ab413` 为 merge-base 执行 `git merge-tree` 预演，并创建 `backup/dev-zz-pre-main-20260825-32e2c530`。
- 本轮上游增量共 9 个提交（5 个非 merge 提交）、16 个文件、245 行新增和 58 行删除；预演与真实 `git merge --no-commit origin/main` 均只有 `backend/cmd/server/VERSION` 一个文本冲突。
- `VERSION` 继续保留 fork `1.7.39`，不吸收上游 `0.1.181`。其余 15 个文件自动合并后仍逐项复审；改动没有触及企业成员候选 / `ActiveGroup`、预算 / usage、sticky、插件、TimePricing、数据库、配置、依赖或前端运行时代码。
- Grok identity 只覆盖官方 CLI proxy / OAuth 路径；Responses status 批量清理只作用于上游已经证明不接受 status 的同一 item type；Lite 工具识别只检查实际非空 `additional_tools`，空载体仍按无工具处理。

验证：

- 定向后端测试覆盖 Gemini schema、Grok CLI identity / observed models、Responses Lite `parallel_tool_calls` 与 rejected-status 整类清理，相关 `xai`、repository 和 service 包通过。
- 后端全仓 unit、`go vet ./...`、server build 和 golangci-lint v2.13（0 issues）通过；前端 typecheck、完整 ESLint 与 docs-site 生产构建通过。
- Git whitespace、真实冲突标记、索引、合并父链与 `origin/main` 祖先检查通过。

未验证：

- 本轮没有迁移、schema、配置、依赖或前端运行时代码变化，因此未重复运行 Testcontainers integration、前端全量 Vitest 或完整 Docker 镜像；未连接真实 Gemini / Grok / OpenAI，未运行浏览器人工 smoke、Hosted CI、推送、发布或生产部署。

## 2026-08-24 - 继续同步上游 `main`：插件传输、Fast 计费与统一分时价格

分支：

- 目标：`dev-zz`
- Base：`d45135d87df16d48637f04ccd245727bc955ba54`
- 合并前目标：`67e91fc7e9d308709f1fd71805733b3954adcd78`
- 上游 head：`03e8ab41346b42de9ece4e3e5bfcb6ca2b8cb57e`
- 结果：本次合并提交

上游要点：

- 新增默认停用的本地进程插件框架、管理员插件页面、独立进程 gRPC 合同与迁移 `229_plugins.sql`、`230_plugin_artifacts.sql`；当前只开放 OpenAI OAuth 出站传输能力。
- OpenAI Responses、Chat Completions 与 WebSocket 增加 `fast` service tier 透传和实际档位计费，另补齐 OAuth quota 自动重置、Codex identity、WebSocket v2 与工具调用 identity 修复。
- 上游模型目录增加独立读取上限；公开模型价格补充 token 阶梯，渠道分时价格增加仅工作日规则；前后端依赖和 Go 工具链同步更新到 1.27.0。
- 管理端吸收插件、Ops 详情、账号优先级、IPv6 代理和用户能力编辑；安全工作流更新前端高危依赖 override 与部署环境检查。

合并策略与冲突：

- 合并前重新读取 `docs-site/dev-zz` 的分支策略、合并流程、补丁 / 合并记录、变更地图、配置迁移、API 合同和验证矩阵；以 `d45135d87` 为 merge-base 执行 `git merge-tree` 预演，并创建 `backup/dev-zz-pre-main-20260824-67e91fc7`。
- 本轮上游增量共 70 个提交（45 个非 merge 提交）、276 个文件、18347 行新增和 1121 行删除；预演与真实 `git merge --no-commit origin/main` 均报告 46 个冲突路径，全部按合同逐项合流，没有整仓选边。
- 保留 dev-zz 单一 `TimePricing` 与现有模型广场架构，不恢复已经删除的 `ChannelTimePricing`、旧价格区域和旧模型广场组件；上游 `weekdays_only` 拼入现有规则，周末跳过显式规则并回落默认倍率与标签。
- 插件不能拥有账号选择、企业成员最终分组、预算 / usage、sticky 或 replay 决策；生产默认拒绝未签名包，插件 `request_sent` 继续作为未知结果禁止跨账号重放的证据。Fast 计费只允许按上游实际结果降档，不能把普通请求抬升为 Fast 计费。
- 保留企业成员 `ActiveGroup`、sticky、预算 / usage 原子归因、未知结果禁止重放、stone UI、隐藏 LinuxDo / 微信默认项和 fork `1.7.39` 版本线；上游 `0.1.180` 不进入 fork 版本文件。
- 全量单测发现一次无冲突标记的语义回归：兼容桥提前拒绝同名工具，破坏既有“等价定义去重、冲突定义拒绝”合同；已恢复定义级比较，并删除仍引用旧模型广场组件的孤立测试。

验证：

- 后端全仓 unit、Testcontainers integration、`go vet ./...`、server build、Wire 稳定再生成和 golangci-lint v2.13 验证通过；定向覆盖统一 TimePricing、工具定义去重、Fast service tier、插件安全、quota reset 和 OpenAI 转发路径。
- 前端 typecheck、完整 ESLint、生产构建和全量 Vitest 通过，共 297 个测试文件、2032 条用例；pnpm frozen-lockfile 与高危 audit exception 校验通过。
- 三组 Compose 安全 / 资源合同脚本与 docs-site 生产构建通过；Git whitespace、真实冲突标记、索引、Wire 生成稳定性、合并父链与 `origin/main` 祖先检查通过。

未验证：

- 未连接真实 OpenAI 或第三方插件进程，未运行浏览器人工 smoke、完整 Docker 镜像、Hosted CI、推送、发布或生产部署。

## 2026-08-24 - 继续同步上游 `main`：工具续链、图片生成与 Guardian 亲和性

分支：

- 目标：`dev-zz`
- Base：`67380eafd5ae2eaa8db910ae738199c3dac62e37`
- 合并前目标：`e54b79e9cd0031f013386459aca528a6b88ecb67`
- 上游 head：`d45135d87df16d48637f04ccd245727bc955ba54`
- 结果：本次合并提交

上游要点：

- Chat Completions → Responses 兼容层补齐 `file` part，并在普通 function arguments 被输出上限截断为非法 JSON 时停止正常完成，避免把坏调用持久化到下一轮；DeepSeek 原生 Responses 现在能把 Codex 客户端工具降级后再恢复回程 identity。
- Responses WebSocket / HTTP bridge 只有在当前 tool output 缺少对应 call context 时才补历史 replay，并清除没有配对 output 的孤立历史 tool call，避免重复执行或让下一轮携带无效调用。
- OpenAI OAuth 图片路径增加 experimental Responses header、文本 fallback 分类、工具不可用冷却与受控同组 failover；内容策略拒绝继续作为明确客户端错误，普通建议文本不再被误报为内容审核。
- `codex-auto-review` Guardian / review 请求可以用父 thread 的 sticky hash 在当前分组内优先选择父账号；客户端只提供 lineage hint，不传账号 ID，候选仍受分组、隐私、传输、能力、利润和调度复核约束。
- Ollama Cloud raw Chat Completions 补齐 `reasoning` / `thinking` 与 `reasoning_content` 的请求响应兼容，并按模型族收紧 `max_tokens`；Google One OAuth 模型目录只发布保守白名单或显式 mapping。

合并策略与冲突：

- 合并前重新读取 `docs-site/dev-zz` 的分支策略、合并流程、补丁 / 合并记录、变更地图、验证矩阵，以及企业成员、预算、Responses / WebSocket 和模型路由 ADR；以 `67380eafd` 为 merge-base 执行 `git merge-tree` 预演。
- 本轮共 21 个上游提交（12 个非 merge 提交）、41 个上游变更文件、2781 行新增和 78 行删除；预演与 `git merge --no-commit origin/main` 均报告 3 个文本冲突：`chatcompletions_responses_bridge.go`、`openai_account_scheduler.go` 和 `openai_gateway_responses_chat_fallback_test.go`。
- 兼容桥同时保留二开的流式工具参数线性累计、单调用 16 MiB / 单响应 32 MiB 上限和 `response.failed` 终态，并吸收上游 malformed JSON 校验；校验直接读取二开 builder 中的累计参数，不能退回已经不再增量拼接的 `Function.Arguments` 字段。
- 调度器保留二开“渠道映射后的模型选择不得清理原 sticky”的保护，同时吸收 Guardian 父账号亲和、分组隐私复核和数据库二次 capability 校验；最终 `PreserveStickyBinding` 取两个保护条件的并集，Guardian fallback 不读取或改写普通 session sticky，也不能跨出当前 `groupID`。
- 测试冲突是两个独立新增用例占用同一插入位置，最终同时保留 Messages → Chat fallback 和输出上限造成非法 tool arguments 两个场景。没有整文件采用 `ours` / `theirs`，最终无未解决索引项。
- 自动合并的 WebSocket replay、DeepSeek 客户端工具、OAuth 图片生成和 Ollama Cloud 路径按企业成员预算结果、响应提交、sticky / 路由锁和未知结果禁止重放合同复审；本轮没有数据库迁移、配置项、依赖、前端运行时代码或版本文件变化，`VERSION` 继续保持 `1.7.38`。

验证：

- 定向后端测试覆盖兼容桥 file part / malformed arguments / 参数上限、DeepSeek 客户端工具、Guardian 亲和与隐私、OAuth 图片、Ollama Cloud、Google One 目录和 WebSocket replay，相关包全部通过。
- 后端 `mise x -C backend -- make test-unit` 全仓通过，其中 `internal/service` 约 153 秒；`go vet ./...`、独立 `go build ./cmd/server` 和 golangci-lint v2.9.0（0 issues）通过。
- 前端 typecheck、完整 ESLint 和 docs-site 生产构建通过；Git whitespace、真实冲突标记、索引、合并父链与 `origin/main` 祖先检查通过。

未验证：

- 本轮没有迁移、schema 或数据访问合同变化，因此未运行 Testcontainers integration；未连接真实 OpenAI、DeepSeek、Ollama Cloud、Google One 或图片上游，未运行浏览器人工 smoke、完整 Docker 镜像、Hosted CI、推送、发布或生产部署。

## 2026-08-21 - 继续同步上游 `main`：国产供应商探测、Composite 入口与 sticky 稳定性

分支：

- 目标：`dev-zz`
- Base：`f646a1f974c26152160ef8327a7d6b9e3488ee83`
- 合并前目标：`e0839d9a707bc9c29b63fde841a47c3bf235f9e2`
- 上游 head：`67380eafd5ae2eaa8db910ae738199c3dac62e37`
- 结果：本次合并提交

上游要点：

- Composite 分组新增 Grok 视频生成入口，并允许按分组开关进入 Messages dispatch；OpenAI 专属的详细 family / model 映射仍只属于 OpenAI 分组。
- 国产供应商账号测试按平台和显式协议选择入口：Anthropic 协议使用供应商原生端点与协议 API Key，DeepSeek Responses 走 OpenAI probe；无效中继余额 payload 不再被误认成零余额，管理端余额 / 配额刷新改为明确的主动探测操作。
- OpenAI Chat sticky seed 只使用请求开头连续的 system / developer 前缀，避免后续动态系统消息打散粘性；空 `openai_capabilities` 恢复为“未配置”，不再排除 OAuth 文本账号，而非空且全 false 或格式错误的能力声明继续保持限制性。
- 前端 token refresh 删除会造成等待者重复取得旧结果的锁循环；启用模型广场时，Home 的 compact、默认导航和 Models CTA 都能按登录要求正确进入 `/model-plaza`。

合并策略与冲突：

- 合并前重新读取 `docs-site/dev-zz` 的分支策略、合并流程、补丁 / 合并记录、变更地图、验证矩阵，以及 Composite、企业成员路由、Messages 和 sticky 相关合同；以 `f646a1f97` 为 merge-base 执行 `git merge-tree` 预演。
- 本轮共 21 个上游提交（11 个非 merge 提交）、30 个上游变更文件、962 行新增和 98 行删除；预演与 `git merge --no-commit origin/main` 均报告 3 个文本冲突：`HomeView.vue`、`GroupsView.vue` 和 `groupsMessagesDispatch.spec.ts`。
- Home 保留二开的 stone / neutral / emerald 顶栏、compact/default 布局和现有文档入口，同时统一采用带认证要求判断的 `showModelPlazaEntry`，不恢复上游旧灰色顶栏或重复导航。Groups 同时保留二开 stone 边框与上游 Composite Messages 开关，详细映射继续限制为 OpenAI；测试冲突合并两个独立类型 / helper import。
- 全量前端测试另外暴露 1 个没有冲突标记的源码合同问题：Home 新增第三个模型广场入口后，旧测试仍假定只有两个链接，且默认 CTA 仍只判断 feature flag。现已让三个入口统一服从 feature flag + 登录要求，并同步更新源码合同测试。
- Composite 视频与 Messages 只扩展匹配入口，候选账号仍按实际平台重新解析；企业成员有序候选、最终 `ActiveGroup`、sticky、预算 / usage 原子归因和结果不明确禁止重放均未放宽。本轮没有数据库迁移、配置项或依赖变化，`VERSION` 继续保持 `1.7.37`。

验证：

- 定向后端测试覆盖 Composite Messages / 视频、CN 协议账号测试、DeepSeek 余额、sticky seed 和空 capabilities；定向前端测试覆盖 token refresh、CN 余额 / 配额、Home 三种入口和 Groups Messages，两批共 49 条测试执行通过。
- 后端 `make test-unit` 全仓通过，service 包约 154 秒；`go vet ./...`、`go build ./cmd/server` 和 golangci-lint v2.9.0（0 issues）通过。
- 前端 typecheck、完整 ESLint、生产构建和全量 Vitest 通过，共 293 个测试文件、2003 条用例；docs-site 生产构建和 Git whitespace、冲突标记、索引、父链及 `origin/main` 祖先检查通过。

未验证：

- 未运行 Testcontainers integration，未连接真实国产供应商 / OpenAI / Grok，未运行浏览器人工 smoke、完整 Docker 镜像、Hosted CI、推送、发布或生产部署。

## 2026-08-21 - 继续同步上游 `main`：Pool 同账号重试、Antigravity daily 与流式工具名

分支：

- 目标：`dev-zz`
- Base：`9f74eb57f45cbc0f81961382e3207bfc37ad72b8`
- 合并前目标：`fe65d78e80f52ff35fa480c2a0cb3a0a0272643b`
- 上游 head：`f646a1f974c26152160ef8327a7d6b9e3488ee83`
- 结果：本次合并提交

上游要点：

- OpenAI-compatible Chat Completions / Responses 在 pool 模式遇到可重试状态码、且现有 rate-limit 处理没有停调账号时，补齐 `RetryableOnSameAccount`，先按账号配置在原账号重试再决定切号。
- Responses 转 Chat Completions 的流式 arguments-only delta 不再输出空 `name`，避免客户端用后续 `"name":""` 覆盖首个 delta 已累计的工具名。
- Antigravity daily 地址改为官方 `daily-cloudcode-pa.googleapis.com`；未显式配置环境变量时，`pro` / `ultra` 账号使用 daily，免费、未知或异常 plan 继续使用生产端点，显式 prod / daily 仍优先。
- CN 供应商额度探测测试 fake 增加互斥锁，消除并发 append race；安全审计为 nanoid 公告补充有期限的例外，支付集成文档修正迁移到 `docs/` 后的链接。

合并策略与冲突：

- 合并前重新核对 `docs-site/dev-zz` 的分支策略、合并流程、补丁 / 合并记录、变更地图和验证矩阵，并以 `9f74eb57f` 为 merge-base 执行 `git merge-tree` 预演。
- 本轮共 13 个上游提交（7 个非 merge 提交）、12 个变更文件、218 行新增和 25 行删除；预演与 `git merge --no-commit origin/main` 均自动合并，没有文本冲突、未解决索引项或 modify/delete 冲突。
- Pool 同账号重试仅补齐既有 failover 信号，继续受 handler 的账号级重试上限、sticky、已写响应判定、企业成员候选和预算状态机约束；没有新增跨账号、跨分组或结果不明确后的重放路径。
- 本轮没有数据库迁移、配置项、依赖或前端变化；`VERSION` 继续保持 `1.7.37`。

验证：

- 定向测试覆盖 Antigravity OAuth / plan 端点选择、Responses arguments delta、Chat / Responses pool-mode 429 同账号重试和 CN 额度探测；CN 并发用例额外通过 `go test -race`。
- 后端 `make test-unit`、`go vet ./...`、`go build ./cmd/server` 和 golangci-lint 通过；pnpm audit exception 校验通过。
- 前端 typecheck / lint 和 docs-site 生产构建通过；Git whitespace、冲突标记、索引、父链和 `origin/main` 祖先检查通过。

未验证：

- 本轮没有数据库、配置、依赖或前端运行时代码变化，因此未重复执行 Testcontainers integration、前端全量 Vitest 或完整 Docker 镜像构建；未连接真实 provider，未运行浏览器人工 smoke、Hosted CI、推送、发布或生产部署。

## 2026-08-21 - 继续同步上游 `main`：Responses 兼容、Grok 4.6 与 Ops 根因证据

分支：

- 目标：`dev-zz`
- 初始 Base：`2bc139ab527b4a687546d145dc7bb9063cf14510`
- 合并前目标：`e147efa2f147ee6da94565f91f29dc120531a029`
- 第一段上游 head：`9d5171c5d1d345f7e0cdacf0e3bc0aa360a15015`
- 最终上游 head：`9f74eb57f45cbc0f81961382e3207bfc37ad72b8`
- 第一段结果：`7fee26124`；最终结果：本次合并提交

上游要点：

- OpenAI Responses 增加输入兼容、客户端工具 schema / 名称归一化、terminal usage 兼容、被拒字段的有限同号重试、compact fallback，以及 HTTP / WebSocket 共用的失败分类和 API Key 健康熔断。
- WebSocket 增加同会话抢占、后续轮次恢复和失败证据；Grok 默认目录推进到 4.6，补齐媒体尺寸、Realtime、限流 / 容量重试、stream idle、compaction 与用量兼容。
- Ops 错误记录增加 upstream status、root cause、失败尝试快照和 generation-bound capture writer，管理端错误详情按根因优先展示并去重诊断 payload。
- 最后追加的 6 个提交补齐国产供应商原生 Anthropic 路径的 `reasoning_effort`，保留 `gpt-5.6-*` 的 `max` effort，并让 prompt guard 的 `config_loaded` 只在首次加载、配置变化或错误恢复时记录。
- 新增 `gateway.grok_response_header_timeout` 和默认关闭的 `openai_apikey_health_breaker_settings`；没有数据库迁移，`dev-zz` 继续保留 `VERSION=1.7.37`。

合并策略与冲突：

- 合并前完整读取 `docs-site/dev-zz` 的分支策略、合并流程、补丁 / 合并记录、变更地图、验证矩阵，以及企业成员路由、WebSocket 预算和 Ops 失败分类文档；先以 merge-base 执行 `git merge-tree`，再使用 `git merge --no-commit`。
- 第一段从 `2bc139ab5` 到 `9d5171c5d` 的真实合并产生 16 个冲突：`.gitignore`；Handler 的 Responses、Chat、cyber、gateway 与 Ops logger 7 个文件；repository rollup 测试 1 个；service 的 Grok、调度、三个 WebSocket 文件和 rate-limit 6 个；Ops 详情组件及测试 2 个。
- 第一段解决期间，远端 `origin/main` 从 `9d5171c5d` 前进到 `9f74eb57f`。为避免伪造 merge parent，先把已解决树提交为 `7fee26124`，再对最终 head 做第二次 `merge-tree` 和 `--no-commit` 合并；新增 6 个提交自动合并，未产生冲突。
- 初始 Base 到最终上游 head 共 51 个提交（43 个非 merge 提交）、208 个上游变更文件、17729 行新增和 1738 行删除。第一段实际合流树为 197 个文件、17376 行新增和 1753 行删除。
- 所有冲突均按字段、状态机和测试合同逐项合流；没有整组采用 `ours` / `theirs`，最终没有未解决索引项或真实冲突标记。

关键解决说明：

- Responses HTTP / WebSocket 吸收 rejected-field retry、stream-start guard、会话抢占和 API Key health 观测，同时保留企业成员候选标记、首轮安全 failover、后续轮次不跨账号重放、逐轮预算 context 和未知传输结果进入 ambiguous 对账的边界。
- WebSocket 连接继续锁定公开模型、渠道映射、账号映射、平台、分组和账号；每轮 usage 保存 `requested -> channel -> account` 的完整映射链。合并测试暴露 `UpstreamModel` 一度停在渠道模型，已改为在建连时冻结最终账号映射模型。
- cyber 阻断同时使用上游逐轮请求体和二开的 billing context / pricing snapshot；调度健康报告吸收上游具体账号与错误对象，但不放弃二开的实际交付模型。
- Ops capture writer 采用上游 generation-bound 非池化 handle，防止旧 lease 访问复用后的 state；保留二开外层 middleware writer 恢复和 inactive writer 行为。流式失败快照、root cause、失败账号 / 分组 / 平台和 enterprise member 归因与现有 route evidence 合流。
- Grok 529 / 429 处理保留二开的 Anthropic / 模型级分类优先级，并采用上游的模型容量与同号重试边界；分组 rollup 测试统一使用显式业务时区 helper。
- 前端保留 stone / neutral / emerald 视觉、route trace 和长文本自动换行，增加 upstream status、根因优先级和诊断 payload 去重；`.gitignore` 同时保留 `.omx` 并加入上游 `.codegraph/`。
- 全仓 unit 暴露 3 个无冲突标记的语义拼接问题并已修复：Responses 流只提交 heartbeat 时必须补一个 `response.failed` 且不得跨账号重试；Grok model self-check 不得写 team-model cooldown；OpenAI WS 流内 429 必须沿用实际 upstream model 做模型级限流，不能退化为账号级停调。

验证：

- 冲突回归覆盖 WebSocket 锁定模型 / 逐轮 usage、后续轮次预算 ambiguous、Ops stale lease / nil writer、Responses 流终态、Grok self-check、模型级 rate-limit、rejected-field / scheduling、rollup repository 和 prompt guard 配置日志。
- CI 同款后端 `make test-unit` 与 `make test-integration` 全仓通过；`go vet ./...`、`go build ./cmd/server` 和 `golangci-lint v2.9.0`（0 issues）通过。
- 前端 typecheck、完整 ESLint、生产构建和全量 Vitest 通过，共 292 个测试文件、1992 条用例；docs-site 生产构建通过。

未验证：

- integration 使用 Testcontainers 临时数据服务验证；未连接生产同量级 PostgreSQL / Redis 或真实 OpenAI、Grok、Kimi、智谱、DeepSeek 上游。未运行浏览器人工 smoke、完整 Docker 镜像、Hosted CI、发布、推送或生产部署。

## 2026-08-21 - 将上游 `main` 合并到正式线 `dev-zz`：自适应协议、Codex 恢复与渠道倍率

分支：

- 目标：`dev-zz`
- 上游：`origin/main`（本地 `main` 与 `origin/main` 均为 `2bc139ab527b4a687546d145dc7bb9063cf14510`）
- Base：`49504adc98d2b6d539491e865a340e644548979e`
- 合并前目标：`3d1ee78ab527b4a687546d145dc7bb9063cf14510`
- 上游 head：`2bc139ab527b4a687546d145dc7bb9063cf14510`
- 结果提交：本次合并提交

上游要点：

- 国产供应商增加 Adaptive API 协议和按协议独立 base URL；Composite 分组扩展 Kimi、智谱、DeepSeek，并为 Codex Responses、WebSocket 和控制类端点补齐组合平台路由。
- OpenAI / Codex 增加跨轮客户端工具状态恢复、WebSocket later-turn resume、当前轮 failover、容量恢复、输入 token 预检、buffered-read failover 和 reasoning cache；Grok 增加 tool search discoveries、可调用工具与内联图片工具。
- 渠道计费增加 Fast / Flex 服务层倍率和缓存创建 / 缓存命中 / 音频输入 / 音频输出倍率；usage 聚合改为单次扫描，迁移 `226` / `227` / `228` 分别补充有效模型索引、Composite 国产供应商和渠道倍率字段。
- 代理探测目标可配置并增加严格校验；Channel Monitor 补齐配额来源、检查模式和界面本地化；Ops 将本地模型配置明确归类为业务受限并排除在上游 SLA 之外。
- 上游版本推进到 `0.1.179`；`dev-zz` 按独立发布线继续保留 `1.7.37`。

合并策略与冲突：

- 合并前完整读取 `docs-site/dev-zz` 的分支策略、合并流程、补丁记录、历史合并记录、变更记录、变更地图、验证矩阵，以及分时定价、多协议、企业成员路由和 Ops 失败分类文档；以这些合同作为冲突裁决依据。
- 以 merge-base `49504adc` 执行只读 `git merge-tree` 预演，再执行 `git merge --no-commit origin/main`。本次上游范围为 79 个提交、214 个文件、9567 行新增和 1482 行删除；预演与真实合并均报告 37 个冲突文件。
- 版本、README、渠道定价与渠道界面共 14 个冲突：`README.md`、`README_JA.md`、`backend/cmd/server/VERSION`、`channel_handler.go`、`channel_handler_test.go`、`channel_repo_pricing.go`、`channel_repo_pricing_time_test.go`、`channel.go`、`model_pricing_resolver.go`、`IntervalRow.vue`、`PricingEntryCard.vue`、`PricingEntryCard.timePricing.spec.ts`、`types.spec.ts`、`ChannelsView.vue`。
- Gateway、协议桥、Composite、usage 与 Ops 共 18 个冲突：`gateway_handler.go`、`no_account_error.go`、`openai_alpha_search.go`、`openai_codex_models_handler.go`、`openai_gateway_count_tokens.go`、`openai_gateway_handler.go`、`openai_live.go`、`ops_error_logger.go`、`chatcompletions_responses_bridge.go`、`usage_log_repo_stats.go`、`composite_platform_test.go`、`gateway.go`、`admin_service_composite_group_test.go`、`openai_apikey_responses_probe_test.go`、`openai_gateway_chat_completions.go`、`openai_gateway_messages.go`、`openai_gateway_responses_chat_fallback.go`、`ops_upstream_context.go`。
- 管理端账号与监控界面共 5 个冲突：`CreateAccountModal.vue`、`EditAccountModal.vue`、`CreateAccountModal.grok.spec.ts`、`UserEditModal.vue`、`MonitorCard.vue`。
- 所有冲突都按字段和控制流逐项合流，没有整批采用 `ours` / `theirs`；`channel_repo_pricing_time_test.go` 是唯一 modify/delete 冲突，因同一合同已有更新后的分层测试覆盖而保持删除。最终未留下未解决索引项或真实冲突标记。

关键解决说明：

- 渠道定价只保留 `dev-zz` 的一套 `TimePricing` 合同：IANA 时区、默认时段名称和倍率、最多 16 条具名规则、跨午夜、`0x`、`[0,100]`、按请求开始时间定价，以及启用后替代分组 / 用户 / 旧 peak 倍率。没有引入上游功能较弱且语义重叠的 `ChannelTimePricing`；同时吸收 Fast / Flex 和四类 interval multiplier。账号统计定价明确忽略渠道倍率与分时规则，避免把渠道实际结算策略误用到成本统计。
- Composite / 国产供应商路由按每个候选账号重新解析真实平台，继续保留企业成员 `ActiveGroup`、sticky、预算 / usage 原子归因和“归因不明确就不重放”的边界；Codex 控制端点和 `count_tokens` 对 OpenAI 与支持的 Composite / 国产供应商开放，Live 则依据解析后的目标平台校验。
- Responses HTTP 支持 OpenAI、Grok 和国产供应商，WebSocket 仍限 OpenAI / Grok。Adaptive 模式下显式国产供应商协议优先于陈旧探测结果：`responses` 固定走 Responses，`chat_completions` / `anthropic` 不被探测覆盖，只有 DeepSeek 的 adaptive 模式按探测决定是否走 Responses。
- Responses 转 Chat bridge 同时保留 `dev-zz` 的 capability / tool registry 与上游 reasoning cache；WebSocket 合流 later-turn resume、当前轮 failover、重试 payload 和逐次模型映射。OpenAI Messages fallback 在转换后进入既有调用链，不绕过 fork 的企业归因和交付决策。
- Usage 统计吸收上游单扫描 `GROUPING SETS` 优化，同时保留企业成员、有效模型和 billing type 过滤。Ops 本地模型配置保留路由证据，但标记为业务受限并排除上游归因和 SLA，避免把本地配置问题误报为供应商故障。
- 前端保留 `dev-zz` 的 stone / neutral / emerald 视觉、分时定价编辑器、企业用户字段和禁用搜索的角色选择器，同时吸收 Adaptive base URL、长上下文计费门控、Fast / Flex 和 interval multiplier 控件、配额本地化。
- 自动拼接后还额外修复了无冲突标记但会导致编译或语义回退的调用签名、`count_tokens` 平台判断、定价 SQL mock、测试 helper 平台透传和 Composite 公开模型列表；Wire 根据最终 provider graph 重新生成且再次生成无差异。

验证：

- `mise x -C backend -- go generate ./cmd/server` 通过且重新生成无差异；`go test -tags=unit ./... -count=1` 全仓通过，其中 `service` 包约 152 秒。定向验证覆盖 Adaptive / 固定协议、Composite / Codex / Live、无账号错误、Ops、本地模型分类、`count_tokens`、admin handler、repository 和渠道倍率。
- 前端 typecheck、完整 ESLint 和 10 个冲突相关测试文件的 115 条 Vitest 通过；全量 Vitest 共 292 个测试文件、1973 条用例通过。
- 后端 `go vet ./...`、`go build ./cmd/server`、前端生产构建和 docs-site 生产构建通过；staged whitespace、真实冲突标记、未解决索引、合并双父节点与 `origin/main` 祖先关系检查通过。

未验证：

- 未连接真实 PostgreSQL / Redis 验证迁移 `226` / `227` / `228` 和实际查询计划；未向真实 OpenAI、Codex、Grok、Kimi、智谱或 DeepSeek 发流量。
- 未运行 Playwright 或浏览器人工 smoke，未构建 Docker 镜像，也未运行 Hosted CI、发布、推送或生产部署。

## 2026-08-18 - 继续合并上游 `main`：版本同步提交

分支：

- 目标：`dev-zz`
- 上游：`origin/main`（与本地 `main` 同为 `49504adc98d2b6d539491e865a340e644548979e`）
- Base：`e0c48a19ed794a565e3858662520afe0a1f9f0ba`
- 合并前目标：`66138ae7c114cdf48719e1cb1f21bc5c3fef7e06`
- 上游 head：`49504adc98d2b6d539491e865a340e644548979e`
- 结果提交：本次合并提交

上游内容：

- 上游仅新增 GitHub Actions 自动版本同步提交 `chore: sync VERSION to 0.1.178 [skip ci]`，只把 `backend/cmd/server/VERSION` 从 `0.1.177` 改为 `0.1.178`，没有业务代码、前端、数据库、配置、依赖或接口变化。

冲突与解决：

- 以 merge-base `e0c48a19` 执行只读 `git merge-tree`，预演和真实 `git merge --no-commit origin/main` 都只报告 `backend/cmd/server/VERSION` 这 1 个内容冲突。
- 按 `dev-zz` 的独立发布线策略保留 `VERSION=1.7.36`，不采用上游 `0.1.178`。该取舍只隔离两条发布版本线，不丢弃任何上游运行时功能；最终树除本合并记录外与合并前业务代码一致。

验证：

- 未解决索引、真实冲突标记、whitespace 和 `VERSION=1.7.36` 检查通过。
- 后端 `cmd/server` 与 `internal/server` 测试通过，docs-site 生产构建通过；提交后验证双父节点、`origin/main` 祖先关系和洁净工作区。

未验证：

- 因上游只有一个未采用的版本文本变化，没有重复运行前一合并已经通过的后端全仓 unit、前端 1924 条 Vitest、typecheck、ESLint 和前端生产构建。
- 本次仍未使用 Playwright，未推送、发布或部署。

## 2026-08-18 - 将上游 `main` 合并到正式线 `dev-zz`：配额监控、网关兼容与分时定价合同合流

分支：

- 目标：`dev-zz`
- 上游：`origin/main`（本地 `main` 与 `origin/main` 均为 `e0c48a19ed794a565e3858662520afe0a1f9f0ba`）
- Base：`e330c243a8f142f8963d784916da0093ab7084ee`
- 合并前目标：`9e11478302bc9dfeb30b293c0273f691562ef851`
- 上游 head：`e0c48a19ed794a565e3858662520afe0a1f9f0ba`
- 结果提交：本次合并提交

上游要点：

- Channel Monitor 增加配额检查模式、配额快照、8 类平台展示和管理员开关；账号选择器支持服务端搜索与已选项回填，避免大账号池一次性加载。
- OpenAI / Codex 增强客户端工具恢复、WebSocket HTTP bridge、终态事件、真实客户端 fingerprint、批量账号设置、Team 联动熔断和 passthrough 模型发现；Gemini / Antigravity 补齐 typed tool config、server-side tool invocation 与 skipped 错误策略。
- 国产供应商修复 Kimi / 智谱 / DeepSeek 的渠道定价、分组入口、调度闸门、计费、断开漏记、`count_tokens` 和 `403` 处理；Grok 增加本站 24h / 7d / 30d 用量汇总并收紧空额度展示。
- 注册邀请码消费改为与用户创建原子提交；账号页增加批量设置与 Ollama 用量查询；Ops、公告、仪表盘、订阅提醒和暗色原生控件获得一组正确性与体验修复。
- 上游也加入了一个仅支持简单时间段的渠道分时倍率实现；本次未直接采用该实现，而是与 `dev-zz` 已完成的按量分时定价合同做了显式取舍。

合并策略与冲突：

- 合并前完整读取 `docs-site/dev-zz` 的分支策略、合并流程、补丁、合并记录、变更记录、变更地图和验证矩阵；确认工作区干净、本地 `main` 与 `origin/main` 一致，再以 merge-base `e330c243` 执行只读 `git merge-tree` 预演和 `git merge --no-commit origin/main`。
- 本次上游范围包含 98 个提交、237 个文件，真实合并产生 36 个冲突文件。冲突集中在分时定价 / 渠道配置、OpenAI gateway / WebSocket / usage、Channel Monitor 设置与 Wire、账号仓储与调度缓存，以及管理端通用界面。
- 分时定价与渠道配置冲突：`channel_handler.go`、`channel_repo_pricing.go`、`billing_service.go`、`channel.go`、`channel_service_test.go`、`225_channel_model_time_pricing.sql`、`channels.ts`、`PricingEntryCard.vue`、`types.spec.ts`、`types.ts`、`ChannelsView.vue`、`GroupsView.vue` 和中英文渠道文案。
- 网关、WebSocket 与 usage 冲突：`openai_chat_completions.go`、`openai_gateway_handler.go`、`gateway_service.go`、`gateway_usage_billing.go`、`openai_gateway_usage.go`、`openai_ws_forwarder.go`、`openai_ws_http_bridge_test.go`、`openai_ws_v2_passthrough_adapter.go`、`api_key_auth_cache_impl.go` 和利润测试。
- 监控、设置、仓储与界面冲突：`channel_monitor_user_handler.go`、`setting_public.go`、`wire.go` / `wire_gen.go`、`account_repo.go`、`scheduler_cache.go` 及测试、`SettingsView.vue`、`AppHeader.vue`、`OpsDashboardHeader.vue`、`OpsErrorDetailsModal.vue` 和共享前端类型。
- 所有冲突均按字段和控制流逐项合流，没有用全仓或整组 `ours` / `theirs` 覆盖。合并后未保留未解决索引项或真实冲突标记。

关键解决说明：

- `dev-zz` 的分时定价是唯一生效合同：保留 IANA 时区、自定义“其余时段”名称与倍率、每条规则独立名称、最多 16 条、跨午夜、`0x`、`[0,100]`、请求开始时定价、启用后替代分组 / 用户 / 旧 peak 倍率，以及客户目录只展示最终价格的语义。上游 `ChannelTimePricing + Periods` 不支持这些合同，因此没有引入第二套运行时、编辑器或测试；迁移继续使用 `JSONB NOT NULL DEFAULT '{}'`。Group / account-stats 输入中的空禁用对象会兼容丢弃，启用的分时规则则在这些非渠道入口明确拒绝。
- 用户侧模型状态继续使用 `dev-zz` 的 `/api/v1/model-status` 与授权分组过滤，没有恢复已被替代的 `/api/v1/channel-monitors` 用户路由；管理员 Channel Monitor 则吸收上游配额 fetcher、快照、负缓存、singleflight、公开设置和远程账号搜索。
- OpenAI Chat / Responses / Messages / WebSocket 保留企业成员 `ActiveGroup`、请求开始定价时刻、交付元数据、渠道 usage 归因和预算持久化失败标记，同时吸收上游在转发错误或客户端断开后提交已观察 partial usage、客户端工具适配、终态事件和 turn 时间戳修复。
- passthrough 的公开模型发现不再被过期映射误当白名单；Codex fingerprint seed、chat tool flags、账号成本别名和 scheduler cache 字段取并集。Wire 根据最终 provider graph 重新生成。
- 设置同时保留 `dev-zz` 的模型自检与上游 `channel_monitor_show_quota`；渠道平台颜色增加 Kimi / 智谱 / DeepSeek，顶栏角色完成本地化，Ops SLA 空窗口仍使用更精确的失败 / 未分类口径，自定义错误时间范围与预设范围可以共同工作。
- 自动拼接处额外发现并修复 Gemini compatibility 的 `policy` 作用域问题；这不是带标记冲突，但会导致合并后编译失败，已通过后端编译和单元测试验证。

验证：

- Wire 从合并后的 provider graph 重新生成；后端 handler / repository / service 编译通过，全仓 `go vet ./...` 通过。
- 后端 `go test -tags=unit ./... -count=1` 全仓通过，其中 `service` 包约 153 秒；分时定价、Group / account-stats 拒绝与空对象兼容用例也单独通过。
- 前端 typecheck 与完整 ESLint 通过；分时定价、可用渠道目录、Select 远程搜索、配额视图、用户监控卡片、监控表单账号选择器和账号批量设置定向 Vitest 共 8 个文件、102 条用例通过；全量 Vitest 共 282 个文件、1924 条用例通过；前端生产构建与 docs-site 构建通过。
- staged whitespace、真实冲突标记、未解决索引、合并父节点与 `origin/main` 祖先关系检查通过。

未验证：

- 未连接真实 PostgreSQL / Redis 验证 migration 225 / 226、配额负缓存和邀请码并发；未向真实 OpenAI、Codex、Gemini、Kimi、智谱、DeepSeek、Grok 或 Ollama 发流量。
- 按本次会话约束未使用 Playwright，也未做浏览器人工 smoke；未运行 Docker 镜像、Hosted CI、发布、推送或生产部署。

## 2026-08-17 - 将上游 `main` 合并到正式线 `dev-zz`：国产供应商、多协议与分组日用量汇总

分支：

- 目标：`dev-zz`
- 上游：`origin/main`（与规范上游 `Wei-Shaw/sub2api` 的 `main` 同为 `e330c243a8f142f8963d784916da0093ab7084ee`）
- Base：`fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710`
- 合并前目标：`3c6621bc5611c337af9c1e491ff5014123d21acc`
- 上游 head：`e330c243a8f142f8963d784916da0093ab7084ee`
- 结果提交：本次合并提交

上游要点：

- Kimi、智谱和 DeepSeek 账号扩展为 OpenAI / Anthropic 多协议国产供应商，增加平台专用 base URL、余额 / 配额查询、周期检测、限流和管理端展示。
- 分组用量增加按自然日预聚合、时区修正、昨日用量和汇总接口；上游迁移 `222` / `223` 建立 rollup 表、触发器和时区口径。
- Codex Responses 增强 turn-state 回显隔离、fingerprint 可选透传、session / prompt-cache 绑定和 native / legacy compaction 选择，减少跨账号状态串联与错误 fallback。
- 管理设置明确 OpenAI Fast / Flex 模式文案，Docker / CI Go builder 对齐 `1.26.6`；上游版本推进到 `0.1.177`。

合并策略与冲突：

- 合并前完整读取 `docs-site/dev-zz` 的分支策略、合并流程、最近补丁 / 合并 / 变更记录、变更地图、配置迁移索引和验证矩阵；确认工作区干净，先 fetch 再基于实时 `origin/main` 合并。
- 以 merge-base `fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710` 执行 `git merge-tree --write-tree --messages --name-only --merge-base` 只读预演，再执行 `git merge --no-commit origin/main`；预演与真实合并均得到 11 个冲突。
- 冲突文件为 `DEV_GUIDE.md`、`backend/cmd/server/VERSION`、`backend/cmd/server/wire_gen.go`、`backend/internal/service/account.go`、`dashboard_aggregation_service_test.go`、`openai_account_scheduler.go`、`openai_gateway_forward.go`、`openai_gateway_messages.go`、`openai_gateway_scheduling.go`、`frontend/src/components/account/EditAccountModal.vue` 和 `frontend/src/views/admin/GroupsView.vue`。
- 没有整文件选择 `ours` / `theirs`。每个冲突按 fork 合同与上游新增能力取并集；首次编译另外发现并修复 Responses fallback 别名、Messages fallback 调用时机和 CN 余额不足 failover 变量这 3 个无标记语义拼接问题。

关键解决说明：

- `VERSION` 保持 fork 发布线 `1.7.36`，不采用上游 `0.1.177`；开发指南同时保留 `dev-zz-branch-images.yml` 与上游三份 Dockerfile 的 Go builder 一致性约束。
- Wire 根据合并后的 provider graph 重新生成；CN provider handler / balance checker 与 dev-zz 的 model self-check、routing eligibility、企业导入和 Channel Monitor V2 生命周期同时保留。
- OpenAI 调度继续保留企业成员请求级 `ActiveGroup`、sticky / profit control 和 `preserveStickyBinding`，同时吸收 CN 平台精确归一化。Anthropic 原生协议优先走供应商原生端点；只支持 Chat Completions 的账号仍先经过 dev-zz 的 Messages 转换、reasoning / Fast 策略和计费模型解析，再进入 fallback。
- Codex turn-state、fingerprint 和 compaction 修复进入现有 Responses / Messages 链；显式协议选择继续优先于账号探测结果，避免新增 CN 探测覆盖既有模型交付决策。
- 管理端账号编辑同时保留供应商成本绑定字段与 CN provider 的 account mode、API protocol、base URL preset、余额和配额能力；分组页继续使用 stone / neutral 视觉，并增加昨日用量列。
- `222_group_usage_daily_rollups.sql`、`223_group_usage_rollup_timezone.sql` 和 `224_user_platform_quotas_add_cn_providers.sql` 按完整文件名追加，没有改写任何已应用迁移。

验证：

- Wire 从合并后的源图重新生成且再次生成无差异；冲突索引、严格冲突标记和 staged whitespace 检查通过。
- 后端分组 rollup、迁移、CN provider、多协议路由、Codex turn-state / compaction、Handler / route 定向测试通过；前端冲突相关 5 个测试文件共 119 条用例和 typecheck 通过。
- 后端 `go test ./... -count=1`、`make test-unit`、`go vet ./...` 和 `go build ./cmd/server` 通过；前端全量 Vitest 278 个测试文件、1868 条用例、typecheck、完整 ESLint 和生产构建通过；docs-site 生产构建通过。全量前端首次运行暴露的 Grok placeholder 静态合同已改为匹配等价 switch 实现，修复后聚焦与全量用例均通过。

未验证：

- 真实 PostgreSQL / Redis 上的 rollup 迁移、触发器时区边界和周期任务；真实 Kimi / 智谱 / DeepSeek 的余额、配额及 OpenAI / Anthropic 上游流量。
- 浏览器人工账号编辑、分组昨日用量和 Fast / Flex 设置 smoke；Docker 镜像、Hosted CI、tag、Release 和生产部署。
- 当前本机未安装 `golangci-lint`，因此没有执行该检查；以 `go vet`、全仓测试和构建作为本地静态 / 编译门，后续仍需 Hosted CI 补齐正式 lint 证据。
- 本次只创建本地合并提交，不推送远端、不发布、不部署。

## 2026-08-13 - 将上游 `main` 合并到正式线 `dev-zz`：监控 V2、Grok 能力与计费审计合流

分支：

- 目标：`dev-zz`
- 上游：`origin/main`（与本地 `main` 同为 `fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710`）
- Base：`00b8596176809906993169c283671811ad04f58d`
- 合并前目标：`4b8b4159c8ef3292ab7a1df7fb5aab604a5b3469`
- 上游 head：`fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710`
- 结果提交：本次合并提交

上游要点：

- 新增 Channel Monitor V2 被动聚合、用户/管理端页面、隐私默认值和温和回填；V2 保持显式 opt-in，缺失或非法配置继续回到 V1。
- Grok 扩展视频、Voice、Realtime、Web Search、X Search、订阅档位、模型目录和媒体计费；账号调度增加可配置用量阈值。
- 用量日志增加上游响应模型与模型不一致审计，渠道可选择 `response_model` 计费来源；分组增加逐模型定价、长上下文阶梯开关以及视频、语音、搜索价格。
- 吸收 OpenAI / Gemini / Antigravity 的兼容、failover、错误处罚、WebSocket、图片计数、安全与备份正确性修复；上游版本推进到 `0.1.176`。

合并策略与冲突：

- 按用户明确要求直接合入正式线 `dev-zz`；合并前完整读取分支策略、合并流程、补丁 / 变更 / 合并记录、变更地图、配置迁移索引和验证矩阵，并确认工作区干净、本地 `main` 与 `origin/main` 一致。
- 以 merge-base `00b8596176809906993169c283671811ad04f58d` 执行 `git merge-tree --write-tree --messages --name-only --merge-base` 只读预演，再执行 `git merge --no-commit origin/main`；预演与真实合并均得到 82 个冲突。
- 冲突主要分布在 gateway / 企业成员路由、设置与账号、usage / Ops、API compatibility、Ent / Wire 生成物、迁移协调、分组定价、模型状态页与相关前端测试。没有使用全局 `ours` / `theirs` 覆盖；每一组均按双方字段、控制流和测试合同逐项合流。

关键解决说明：

- 企业成员继续由有序候选编排、请求级 `ActiveGroup`、模型 / 端点能力裁剪和每次候选重新解析 Composite 路由；新视频、Voice、Realtime、Search 与无前缀 OpenAI 路由也通过同一成员解析、预算门禁和候选编排链。预算结果不明确仍禁止换组，成功 usage、预算结算和最终实际分组继续原子归因。
- 公开模型与报价继续使用 dev-zz 的共享 available-channel catalog；已被共享目录取代的 `channel_plaza` 与旧 `PlazaModelPricingTable` 保持删除。上游逐模型、视频、语音和搜索价格迁入共享 Group / Channel schema 与定价卡片，不恢复第二套模型目录。
- `/monitor` 保留新的 V1 / V2 模式外壳，但默认 V1 内容继续使用 dev-zz 已发布的“按分组 + 模型站点自检、可选历史 fail-soft、管理员 Token 用量”页面；上游 Channel Monitor V2 仅在显式选择 V2 时展示，避免升级后默认页面语义倒退。
- settings、公开 DTO、账号页和缓存版本取并集：保留企业成员 admission、模型原生多协议、自检和 schedule strategy，同时吸收 Channel Monitor V2、Grok mapping、账号调度阈值与 throughput 隐私开关。敏感凭据脱敏键也取并集。
- usage insert / query 同时保留成员 tombstone、`schedule_meta`、`route_plan_*`、`session_id` 和上游响应模型审计；insert 参数数量由合并后的类型表统一计算，过滤器继续经过 owner / member 隔离，避免新增 mismatch 筛选绕过权限边界。
- Ent 从合并后的 schema 重新生成，Wire 从合并后的 provider graph 重新生成；`ModelSelfCheckRunner` 与 `ChannelMonitorV2Aggregator` 均进入启动 / 停止生命周期。
- 上游迁移按完整文件名原样追加，包括同号但不同文件名的 `194` / `195`；没有重命名、覆盖或改写已应用迁移。并发索引预处理同时覆盖企业成员基线、Ops 路由和上游模型 mismatch 索引。
- `VERSION` 保持 fork 发布线 `1.7.32`，不采用上游 `0.1.176`。

验证：

- Ent / Wire 重新生成后无差异；`make test-unit`、`go test ./... -count=1`、`go vet ./...`、`golangci-lint run ./...`（0 issues）和 `go build ./cmd/server` 全部通过。server / route / repository / migration / compatibility 定向测试以及企业成员路由、预算、模型状态定向测试也通过。
- 前端 typecheck、完整 ESLint、生产构建和全量 Vitest 通过：275 个测试文件、1853 条用例。首次全量暴露的重复 mock、V1/V2 i18n 扫描和固定 Ops 快照参数等 4 个合并测试合同均已修复；V1 模型状态组件与合并前 `dev-zz` 源逐字一致。
- `git diff --check`、冲突标记扫描和独立高风险代码复核通过；独立复核没有发现企业成员路由、预算 / usage 归因、gateway 中间件顺序、usage SQL、Ent / Wire 或迁移的 P0 / P1 阻塞项。
- docs-site 生产构建通过；只有既有的大 chunk、动态 / 静态重复导入、Browserslist 数据陈旧和测试环境刻意错误路径告警，没有构建或测试失败。

未验证：

- 真实 PostgreSQL / Redis 迁移与 Channel Monitor V2 长周期回填，真实 Grok / OpenAI / Gemini / Antigravity 媒体、语音、搜索和 WebSocket 流量。
- 浏览器人工 V1 / V2 模型状态、分组逐模型定价、账号阈值和用量 mismatch 下钻；Docker 镜像、Hosted CI、tag、Release 和生产部署。
- 本次只创建本地合并提交，不推送远端、不发布、不部署。

## 2026-08-12 - 将正式线 `dev-zz` 恢复到 `dev-zz-develop` 以修复 v1.7.29 故障

分支：

- 目标：`dev-zz-develop`
- 来源：`dev-zz`（与 `origin/dev-zz` 同为 `381275bd6`）
- Base：`6b92daf42`
- 合并前目标：`92e039c66`
- 来源 head：`381275bd6`
- 结果提交：`c124b90c2`

合并目的：

- `dev-zz-develop` 在生产回退后位于 v1.7.28 基线，只包含 `fix/budget-error-attribution-1728` 的错误归因补丁；要修复 v1.7.29 的真实运行时放大问题，必须先在开发候选线恢复 v1.7.29 的模型感知路由实现。
- 保持已发布 tag 不可变，在 `dev-zz-develop` 上建立可测试、可回退的修复候选；本次没有 push、release、镜像发布或服务器操作。

合并策略与冲突：

- 合并前使用 `git merge-tree --write-tree --messages --name-only --merge-base` 预演；预演与真实 `git merge --no-ff --no-commit dev-zz` 均只在 `backend/internal/server/middleware/enterprise_member_group.go` 及其测试产生冲突。
- 两处冲突来自 v1.7.28 回退线和 v1.7.29 正式线分别携带等价的预算错误归因修复；解决结果保留 v1.7.29 的路由结构、底层错误日志和 `gatewayErrorCodeHeader` 测试合同。
- v1.7.29 的迁移 `199`-`202` 按正式 tag 内容原样恢复，没有改写已经可能被数据库记录为已应用的迁移文件。

验证与边界：

- 合并冲突后的企业成员预算错误归因定向测试通过，`git diff --check` 通过。
- 合并提交仅建立根因修复基线；admission 热路径修复和完整验证由后续提交承担。
- 未执行 push、正式分支合并、tag、Release、镜像或生产数据库 / 服务器操作。

## 2026-08-05 - 将上游 `main` 合并到 `dev-zz-develop`：认证验证码、Codex 身份与图片报价口径合流

分支：

- 目标：`dev-zz-develop`
- 上游：`origin/main`（与本地 `main` 同为 `00b859617`）
- Base：`825ca7b1f`
- 合并前目标：`88f24f161`
- 上游 head：`00b859617`
- 结果提交：本次合并提交

上游要点：

- 认证人机验证扩展为互斥的 Cloudflare Turnstile、腾讯天御与阿里云验证码 2.0；认证动作和 OAuth 登录启动 / 待建账号流程统一取得并校验 proof，管理端可保存 provider 凭据且审计不继承已存 secret。
- Codex 出站身份统一由生效版本派生，默认自动跟随官方最新稳定版并允许管理员覆写；版本同步使用 latest 主路径、列表回退、6 小时间隔和启动防抖。WebSocket 租约丢失保留 terminal event，提示词审计补齐 Responses 文本解析。
- 模型广场图片报价修复为与实收一致的分组档位和独立倍率；订阅续费串行化，管理端消费排行补充用户名，Anthropic OAuth authorize endpoint 切换到 `claude.com/cai`，Grok CLI 固定版本更新。
- 上游版本推进到 `0.1.171`，并更新赞助商列表与静态资源。

合并策略：

- 合并前完整读取 `docs-site/dev-zz` 的分支策略、上游合并流程、变更地图、最近补丁 / 合并 / 变更记录、共享模型目录设计和验证矩阵；确认工作区干净、本地 `main` 与 `origin/main` 一致，`origin/dev-zz` 没有待吸收提交。
- 先以 merge-base `825ca7b1f` 执行 `git merge-tree --write-tree --messages --name-only` 预演，再执行 `git merge --no-commit origin/main`；预演与真实合并得到相同的 17 个冲突。
- 接受上游验证码、Codex 身份 / 版本同步、订阅并发、WebSocket、提示词审计、OAuth、排行和依赖更新；继续保留 dev-zz 共享公开模型目录、LinuxDo / 微信登录注册入口隐藏策略、企业成员合同、stone / neutral / emerald 视觉、长期数据保留和 `1.7.27` 版本线。

冲突文件：

- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire_gen.go`
- `backend/go.sum`
- `backend/internal/handler/admin/setting_handler_platform_quota_test.go`
- `backend/internal/handler/admin/setting_handler_update.go`
- `backend/internal/handler/model_plaza_handler.go`
- `backend/internal/handler/model_plaza_handler_test.go`
- `backend/internal/service/channel_plaza.go`
- `backend/internal/service/channel_plaza_test.go`
- `backend/internal/service/setting_service.go`
- `frontend/src/api/modelPlaza.ts`
- `frontend/src/components/modelPlaza/PlazaGroupSection.vue`
- `frontend/src/components/modelPlaza/PlazaModelPricingTable.vue`
- `frontend/src/components/modelPlaza/PlazaModelPricingTable.spec.ts`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/auth/LoginView.vue`
- `frontend/src/views/auth/RegisterView.vue`

解决说明：

- `VERSION` 保持 dev-zz `1.7.27`，不采用上游 `0.1.171`；Wire 根据合并后的 provider graph 重新生成，`go.sum` 同时保留 dev-zz 与新增腾讯 / 阿里验证码 SDK 的依赖校验。
- 设置更新继续使用 dev-zz 的提前审计和额外变更记录，同时改用上游的脱敏 `auditReq`，避免未提交的已存腾讯 secret 被误计入审计 diff。设置服务同时保留模型原生多协议缓存和新增 Codex 版本缓存 / singleflight。
- 公开模型列表和登录可用渠道继续复用 `buildAvailableChannelCatalog`、`AvailableModelMarketplace` 与共享报价工具；旧 `channel_plaza` service、`PlazaGroupSection` 和 `PlazaModelPricingTable` 保持删除，不恢复第二套模型 / 价格 / 端点规则。
- 上游图片报价修复迁入共享目录：用户分组 DTO 增加图片独立倍率和 `1K / 2K / 4K` 档位价；模型广场、价格表格与导出统一按“分组档位 > 渠道同档位 > 渠道默认按次价”回落，并在独立图片倍率启用时只乘 `image_rate_multiplier`。`image_output_price` 不再被模型卡片误当单张图片价格。
- 管理端设置页同时保留模型原生多协议保存状态和新增验证码 provider 主选择器。登录 / 注册页吸收 action captcha 与 OAuth start API，但继续隐藏 LinuxDo 和微信入口；对应后端能力、回调页和可复用 OAuth 组件不删除。
- 上游订阅续费锁、WebSocket terminal event、Responses 审计解析、Codex 身份和 OAuth endpoint 修复按原设计吸收；本轮没有数据库迁移，新增依赖仅来自腾讯与阿里云验证码服务端 SDK。

验证：

- 前端全量 Vitest 通过：259 个测试文件、1739 条用例；覆盖共享图片档位 / 独立倍率、公开模型卡片、验证码和设置冲突面。
- 后端 `mise x -C backend -- go test -tags unit ./... -count=1` 与 `mise x -C backend -- go test ./... -count=1` 全量通过；`go vet ./...` 和 `go build ./cmd/server` 通过。
- 本次冲突 / 上游能力的聚焦 `-race` 用例通过，覆盖共享图片价格投影、Codex 版本同步、订阅续期、验证码服务以及 OAuth / 模型广场 handler。
- 前端 typecheck、全量 ESLint 与生产构建通过；输出只有既有 browserslist、动态 / 静态重复导入和大 chunk 提示。
- Wire 根据合并后 provider graph 重新生成完成；docs-site 生产构建通过。Go 格式、staged whitespace、冲突标记与未解决索引检查通过。

未验证：

- 真实腾讯天御 / 阿里云验证码、Cloudflare Turnstile、外部 OAuth、GitHub 官方 release API 与上游 Codex / WebSocket 流量。
- 真实 PostgreSQL / Redis 与 integration testcontainers 运行时；integration 标签测试二进制编译通过，但当前环境在 `TestMain` 因找不到 rootless Docker 而无法启动容器。
- service 包宽范围组合 `-race` 仍会命中既有测试基建竞争：多个并行测试共同调用全局 `gin.SetMode`，与同时创建 Gin engine 的用例发生读写竞争；本次变更对应的聚焦 race 用例已单独通过，本轮不扩展到无关测试基建重构。
- 浏览器人工登录 / 注册验证码、管理设置、公开模型页与图片模型报价 smoke；Docker 镜像 / Compose 运行时未验证。

## 2026-08-03 - 将上游 `main` 合并到 `dev-zz-develop`：利润控制、计费可靠性与管理端批量能力合流

分支：

- 目标：`dev-zz-develop`
- 上游：`main`（与 `origin/main` 同为 `825ca7b1f`）
- Base：`d29acc29a`
- 合并前目标：`7d0201e49`
- 上游 head：`825ca7b1f`
- 结果提交：本次合并提交

上游要点：

- 新增分组级利润控制：按平台、分组倍率、账号成本倍率和安全缓冲计算准入阈值，并把 veto 传播到普通、OpenAI、图片与 WebSocket 调度；配套迁移 `192_group_profit_control.sql`、`193_group_profit_control_auth_cache_invalidation.sql`、预览命令与管理端配置。
- 上游账单探测扩展到多 API Key 平台，可在受控开关、倍率上限和抑制策略下把官方声明倍率回写账号；账号编辑、批量探测和列表显示同步刷新倍率。
- OpenAI 增强 reset-credit 恢复、刷新缓存、SSE `429`、Messages 临时错误 failover、请求取消、负载削峰、namespace 工具、工具输出媒体、WebSocket turn 计价和 compact 恢复；Anthropic 中断流保留已观察 usage。
- 认证刷新 token 轮换避免并发竞态；支付退款补充余额不足强制确认和 Stripe 幂等；SMTP 测试与发送统一连接路径，内容审核可走配置代理，提示词审计增加窄范围阻断。
- 管理端账号支持按完整筛选结果全选，Home 增加 compact preset；模型广场视觉与排序调整。上游版本推进到 `0.1.170`。

合并策略：

- 合并前读取 `branch-policy.md`、`maintenance/merge-main.md`、最近补丁 / 合并 / 变更记录、变更地图和验证矩阵；确认本地 `main` 与 `origin/main` 一致、目标工作区干净且 `origin/dev-zz` 没有待吸收提交。
- 先以 merge-base `d29acc29a` 执行 `git merge-tree --write-tree --messages --name-only` 预演，再执行 `git merge --no-commit main`；预演与真实合并得到相同的 19 个冲突。
- 接受上游利润控制、倍率回写治理、网关 / 支付 / 认证 / 邮件正确性、compact Home 和筛选结果全选；继续保留 dev-zz 企业成员原子结算、候选路由与最终归因、模型多协议交付、账号归档、共享模型目录、stone / neutral / emerald 视觉、长期数据保留和 `1.7.25` 版本线。

冲突文件：

- `backend/cmd/server/VERSION`
- `backend/ent/client.go`
- `backend/internal/handler/admin/setting_handler_platform_quota_test.go`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/grok_media.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go`
- `backend/internal/service/admin_group.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/openai_gateway_messages_chat_fallback.go`
- `backend/internal/service/openai_gateway_scheduling.go`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/modelPlaza/PlazaFilterBar.vue`
- `frontend/src/components/modelPlaza/PlazaGroupSection.vue`
- `frontend/src/components/modelPlaza/PlazaModelPricingTable.vue`
- `frontend/src/components/modelPlaza/PlazaModelPricingTable.spec.ts`
- `frontend/src/views/HomeView.vue`
- `frontend/src/views/admin/AccountsView.vue`

解决说明：

- `VERSION` 保持 dev-zz `1.7.25`；Ent client 保留企业成员实体 hooks / interceptors，同时吸收新 Group 利润字段。认证快照提升到 `v21`，强制刷新并合并利润、计价、企业成员与 OpenAI Live 字段。
- 普通网关、OpenAI 与 Grok media 同时接入利润 gate / veto 统计和上游 partial usage 记录，并保留企业成员候选编排、预算、最终 `ActiveGroup` 与媒体任务持久化。完整服务测试发现上游“计费失败写未结算日志”会破坏成员结算原子性，最终限定为普通请求保留未结算日志、企业成员继续 fail-closed 且不做独立 usage 写入。
- OpenAI 调度同时保留 sticky 绑定与利润 veto；代理隔离 fail-open 只放宽隔离偏好，不丢失定价预检 bypass。Messages fallback 叠加 reasoning effort、Fast / Flex 和 business-limited 策略；WebSocket 继续锁定首 turn 的公共模型 / 上游模型映射。
- Responses → Chat 工具桥保留 dev-zz 显式 `execution=client`、namespace、动态注册与资源边界，同时吸收上游工具输出文本 / 图片提取；带 `.tools` 的 `tool_search_output` 继续注册动态工具，只有 `.output` 时按工具结果回放。
- Group 设置同时执行平台规范化、利润字段校验和 dev-zz OpenAI Messages dispatch 校验。账号编辑同时保存供应商、cache token 与受控账单倍率回写配置。
- 模型广场继续使用 dev-zz 共享 marketplace 组件、Select 筛选和紧凑 stone / neutral / emerald 布局；已被共享目录替代的旧 Group/Pricing 组件保持删除，不恢复倍率筛选。Home 同时遵守自定义内容、compact preset 与模型广场入口开关。
- 账号页吸收筛选结果全选和批量操作状态，但继续使用 dev-zz 归档生命周期，不把上游批量硬删除作为当前页面操作；成本视图、普通列表、供应商上下文和批量归档保持隔离。
- 合并后的上游测试按当前构建边界修正：依赖 unit-only channel repository 的调度用例移入 `unit` 测试文件，构造器补齐企业预算 / Grok repository 参数，Home 与账号选择用例使用当前 Pinia、视觉根节点和共享 checkbox 合同。

验证：

- `apicompat` 工具输出媒体 / Tool Search / namespace 定向测试通过；service 默认与 `unit` 利润、reasoning、sticky、scheduler、Messages、账单探测、退款和取消定向测试通过。
- handler、admin handler、`apicompat` 和 migrations 定向测试通过；覆盖利润 gate、平台设置、quota reset、工具输出、compact / cancellation 等冲突影响面。
- `go test -tags=unit ./internal/service -count=1` 全包通过；handler、admin handler、routes、`apicompat` 和 migrations 的 `unit` 全包通过。
- 前端冲突相关 8 个测试文件、109 条用例与关键回归 8 个测试文件、133 条用例通过；typecheck、ESLint 与生产构建通过，输出只有既有 browserslist、动态导入和大 chunk 提示。
- Ent / Wire 重新生成完成，后端 `1.7.25` 二进制构建通过；docs-site 生产构建通过。Go 格式、staged whitespace 和冲突标记检查通过。

未验证：

- 真实 PostgreSQL 迁移、Redis 认证缓存失效、上游账单探测与倍率回写、OpenAI / Anthropic 流式错误、支付 / SMTP / 内容审核代理运行时。
- 浏览器人工 compact Home、筛选结果全选、账号归档、模型广场与利润控制表单 smoke；Docker 镜像 / Compose 运行时未验证。

## 2026-07-31 - 将上游 `main` 合并到 `dev-zz-develop`：网关安全、订阅窗口与支付配置正确性合流

分支：

- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`5a6143097`
- 合并前目标：`be7384503`
- 上游 head：`d29acc29a`
- 结果提交：本次合并提交

上游要点：

- OpenAI Responses 子路径与 Gemini 上游 URL 增加路径片段校验；OpenAI 代理断流熔断在全部候选都被隔离时 fail-open，并合并突发断流事件。Pool 模式可重试流式容量错误，Grok billing ping 统一转换为 SSE 注释且不再因 pool 模式的 entitlement `403` 误冷却账号。
- 订阅日 / 周 / 月额度窗口以订阅真实开始时间和到期时间为边界推进，旧版首个午夜锚点可安全归一到订阅开始时间；前后端剩余时间与重置标签使用同一口径。
- 支付设置恢复严格 patch 语义，省略字段不会清空已保存的可见支付方式；支付方式选择器和长套餐标题修复窄屏溢出。SMTP 邮件改为标准 CRLF、折行与 dot-stuffing 格式，异步图片转存支持解码 data URL。
- 用户可用渠道会把 Composite 分组展开到其实际配置模型的平台 section；官方价格更新 GPT-5.6 Luna / Terra 与 GLM-5.2 fallback，管理员渠道列表可显示 Composite 模型。
- Docker / Compose 增加 `no-new-privileges`，release 产物补齐价格 fallback 资源，CI 增加部署安全合同脚本。上游版本推进到 `0.1.169`。

合并策略：

- 合并前读取 `branch-policy.md`、`maintenance/merge-main.md`、最近补丁 / 合并 / 变更记录、变更地图和验证矩阵；确认 `dev-zz-develop@be7384503` 与 `origin/dev-zz-develop`、`dev-zz`、`origin/dev-zz` 完全一致且工作区干净。
- `git fetch --prune origin` 后确认上游自 `5a6143097` 新增 56 个提交，其中 32 个非合并提交；先执行 `git merge-tree --write-tree --messages --name-only --merge-base` 预演，再执行 `git merge --no-commit origin/main`，两者均得到相同的 4 个内容冲突。
- 接受上游网关路径安全、流式容错、订阅窗口、支付 patch、邮件 / 图片、Composite 目录、价格和容器权限修复；继续保留 dev-zz 企业成员候选编排、预算、最终分组归因、WebSocket 首轮路由锁、真实模型交付目录、长期数据保留和 `1.7.25` 版本线。

冲突文件：

- `backend/cmd/server/VERSION`
- `backend/internal/handler/available_channel_handler_test.go`
- `backend/internal/handler/openai_gateway_handler_test.go`
- `backend/internal/server/routes/gateway.go`

解决说明：

- `VERSION` 保持 dev-zz `1.7.25`，不采用上游 `0.1.169`。
- Responses 子路径在进入企业成员 / Composite 候选编排前执行上游安全守卫；合法路径继续经过 dev-zz 的成员分组解析、预算门禁与最终分组归因，非法路径以本地策略拒绝且不进入上游调度。
- 可用渠道测试同时保留 dev-zz 的稳定交付 / 协议端点投影与上游 Composite section 展开合同；Composite 分组不会被错误压缩为单一平台，普通分组仍保持平台隔离。
- Responses WebSocket 三处冲突继续遵守文档化的 dev-zz 首轮路由锁：连接内可以省略或重复同一公共模型，模型、平台或渠道目标变化必须重连；上游价格更新仍由共享价格资源吸收。
- 编译验证发现上游新抽取的代理熔断 fail-open 包装层没有把 dev-zz 的 `skipChannelPricingRestriction` 与 `preserveStickyBinding` 加入内部选择签名并传入两次选择。最终两次选择都透传原参数，使 fail-open 只放宽代理隔离，不放宽渠道定价或首轮 sticky 合同。
- Ops 自动清理仍由 `ops.cleanup.auto_cleanup_enabled` 门禁且默认关闭；本轮只把成功日志切换为结构化 info，不改变保留期或删除行为。

验证：

- 冲突相关 routes / handler 定向 Go 测试通过，覆盖 Responses 子路径安全守卫、可用渠道 Composite 展开、稳定交付投影和 WebSocket 首轮路由锁。
- 代理熔断 fail-open 定向测试通过，覆盖全部代理隔离时恢复容量，以及第二次选择继续遵守调用方显式渠道定价预检策略。
- 后端 `go test -tags=unit ./... -count=1` 全包通过。
- 前端支付方式、套餐卡、订阅额度口径、用户支付页和管理设置共 5 个测试文件 / 62 条测试通过；测试输出只有既有 jsdom navigation 与 i18n compiler 警告。
- 前端 typecheck、ESLint 与生产构建通过；仅有既有 browserslist、动态导入和大 chunk 提示。
- `docker-compose-security-test.sh` 与 `docker-runtime-resources-test.sh` 通过。
- docs-site 生产构建通过；仅有既有 chunk-size 提示。
- Go 格式、staged whitespace 与冲突标记检查通过。

未验证：

- 浏览器人工支付布局 / 订阅到期标签 smoke。
- 真实 SMTP、代理断流、Grok ping、上游 URL 拒绝和 Docker 容器运行时 smoke。

## 2026-07-30 - 将上游 `main` 合并到 `dev-zz`：OpenAI Live store 容错与前端状态修复合流

分支：

- 目标：`dev-zz`
- 上游：`origin/main`
- Base：`8fd01c281`
- 合并前目标：`db4eb63e2`
- 上游 head：`5a6143097`
- 结果提交：本次合并提交

上游要点：

- OpenAI Live observer 在 controller claim、call record 和 controller 状态读取遇到 Redis / store 抖动时不再静默退出；有限重试耗尽后按会话 `ExpiresAt` 兜底 finalize，保证租约释放与 usage 证据最迟在会话到期时完成。
- Live finalize 的 usage 写入改为 best-effort 队列优先、同步 `Create` 回退，避免该会话唯一一次落库机会因队列超时或故障永久丢失。
- 管理端账号状态为 Claude Sonnet 5 增加稳定短别名；Passkey 功能关闭时资料页不再请求凭据列表，设置变更竞态返回 `PASSKEY_DISABLED` 时也不会显示误导性错误提示。
- 上游版本推进到 `0.1.168`。

合并策略：

- 合并前读取 `branch-policy.md`、`maintenance/merge-main.md`、最近补丁/合并/变更记录、变更地图和验证矩阵；确认 `dev-zz@db4eb63e2` 与 `origin/dev-zz` 对齐、工作区干净，且本地 `main` 与 `origin/main` 都指向 `5a6143097`。
- 先执行 `git merge-tree --write-tree --messages --name-only --merge-base` 预演，再执行 `git merge --no-commit origin/main`；预演和真实合并都只得到 `VERSION` 与 `openai_live.go` 两个内容冲突。
- 接受上游 Live store 容错、usage 同步回退、Claude Sonnet 5 状态别名和 Passkey 禁用态修复；继续保留 dev-zz 企业成员 Live 身份校验、成员快照、最终实际分组证据、脱敏结构化失败事件和 `1.7.23` 发布线。

冲突文件：

- `backend/cmd/server/VERSION`
- `backend/internal/service/openai_live.go`

解决说明：

- `VERSION` 保持 dev-zz `1.7.23`，不采用上游 `0.1.168`。
- Live observer 接受上游把完整 `LiveCallRecord` 交给 observer、store 故障有限重试、到期兜底 finalize 和幂等关闭语义；原有企业成员 identity / snapshot 字段继续随 call record 持久化并在 sideband 身份匹配时校验。合并审查同时发现生产 Redis gateway cache 原先没有序列化这三个字段，已补齐保存 / 读取和 miniredis 跨实例往返测试，避免测试 fake 掩盖成员归因丢失。
- Live usage 在写入前继续补齐 `MemberID`、成员编号 / 名称快照、最终 `GroupID` 与 call hash request ID；写入改为 best-effort 队列优先、同步回退。定向测试发现直接使用上游通用 helper 会丢失 dev-zz 的 `openai_live.usage_log_insert_failed` 结构化事件，因此最终解法在同步回退仍失败时保留 call hash、account、Key 和 user 数字 ID，且不记录原始 call ID、凭据或 attestation。
- Claude Sonnet 5 只增加状态徽标短别名，不改变模型匹配或限流状态；Passkey 修复只收紧 disabled 路径，不改变已启用情况下的注册、登录和撤销合同。

验证：

- `mise x -C backend -- go test ./internal/repository ./internal/service -run 'Live' -count=1` 通过，覆盖 Redis call record 成员字段跨实例往返、store claim / read 故障、到期 finalize、usage best-effort 同步回退、企业成员快照、身份校验和脱敏失败日志。
- `mise x -C backend -- go test ./internal/service -count=1` 全包通过。
- `pnpm --dir frontend test:run src/components/account/__tests__/AccountStatusIndicator.spec.ts` 通过，共 6 条测试。
- `pnpm --dir frontend typecheck` 与 `pnpm --dir frontend lint:check` 通过。
- `pnpm --dir docs-site docs:build` 通过；仅有既有 VitePress chunk-size 警告。
- Go 格式、staged whitespace 和冲突标记检查通过。

未验证：

- 浏览器人工 Passkey disabled 设置切换与 Claude Sonnet 5 状态徽标 smoke。
- 真实 Redis 故障期间的长连接到期恢复；Redis 字段往返仅使用 miniredis 验证。

## 2026-07-28 - 将上游 `main` 合并到 `dev-zz`：Passkey、模型价格橱窗与字段级更新合流

分支：

- 目标：`dev-zz`
- 上游：`origin/main`
- Base：`dc893dd0b`
- 合并前目标：`eddb60257`
- 上游 head：`8fd01c281`
- 结果提交：本次合并提交

上游要点：

- 新增 Passkey 注册、登录、撤销和管理端开关；注册与撤销要求当前账户密码，WebAuthn 配置不完整时 fail-closed。
- 新增默认关闭的 `/model-plaza` 价格橱窗，按分组聚合渠道模型与官方参考价，并支持公开访问或强制登录。
- User / API Key 仓储更新改为显式字段集合，避免余额、额度、状态、标签或企业字段被并发的无关保存覆盖。
- OpenAI Messages 桥接保留 GPT-5.6 `max` reasoning effort；补充 Kimi K3 / 1M 后缀、Codex Web Search、Anthropic cache breakpoint、安全审计配置恢复和 setup bypass 修复。
- 管理端模型白名单增加模型 ID 复制能力。

合并策略：

- 合并前读取 `branch-policy.md`、`maintenance/merge-main.md`、最新补丁/合并/变更记录、变更地图和验证矩阵；确认 `dev-zz@eddb60257` 与 `origin/dev-zz` 对齐且工作区干净。
- `git fetch --prune origin` 后确认 `origin/main@8fd01c281` 比上一轮上游增加 18 个非合并提交；先执行 `git merge-tree --write-tree --messages --name-only` 预演，再执行 `git merge --no-commit origin/main`，两者均得到相同的 12 个冲突。
- 接受上游 Passkey、字段级仓储更新、模型价格橱窗和协议正确性修复；继续保留 dev-zz 企业成员能力、模型多协议调度、Messages 显式映射、模型状态授权、现有可用渠道模型广场、stone / neutral / emerald 视觉、长期数据保留和 `1.7.21` 版本线。

冲突文件：

- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire_gen.go`
- `backend/go.mod`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/user_repo.go`
- `backend/internal/server/http.go`
- `backend/internal/server/router.go`
- `backend/internal/service/admin_user.go`
- `backend/internal/service/api_key_service.go`
- `backend/internal/service/openai_gateway_messages_chat_fallback.go`
- `frontend/src/components/account/ModelWhitelistSelector.vue`
- `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

解决说明：

- `VERSION` 保持 dev-zz `1.7.21`；Go 依赖同时保留 dev-zz `google/subcommands` 与上游 WebAuthn 所需 `google/go-tpm`，重新执行 `go mod tidy`。
- Wire / HTTP / Router 重新生成并做并集：保留企业成员预算、模型交付服务和恢复任务，同时接入 Passkey、模型价格橱窗和 Optional JWT；前端服务器的静态检查抑制保持上游拆分后的边界。
- User 更新字段同时覆盖上游的邮箱、资料、状态、限额等列与 dev-zz 的 `account_type` / `enterprise_disabled_at`；企业费率事务不再用完整实体覆盖余额或其它并发字段。
- API Key 更新字段同时覆盖上游的名称、状态、额度、限流等列与 dev-zz `tags`；状态更新继续同步 `disabled_reason`，普通更新不会触碰企业成员归属，批量更新按请求内容构造字段集合。
- Messages 强制 Chat Completions 回退继续使用 dev-zz 的 Responses 中间表示；上游 GPT-5.6 `max` effort 修复在进入 Responses 序列化前生效，不回退已落地的原生 Messages / Responses 协议选择。
- 模型白名单组件同时保留 dev-zz 目录搜索、自定义模型加入语义和上游复制按钮；SettingsView 测试同时保留 OpenAI Fast/Flex 与 Passkey 配置合同。
- `/model-plaza` 保持上游独立且默认关闭的价格橱窗，不替换 `/available-channels`：前者可展示授权专属分组的报价，后者仍是 dev-zz 按真实可调度能力收敛的用户可用模型入口。
- 字段级仓储接口合流后同步修正 dev-zz API Key 批量与 handler 测试桩签名，避免测试替身继续实现旧的全实体更新合同。

验证：

- 冲突相关 repository / service / handler / middleware / routes 定向测试通过。
- 后端 `go test -tags=unit ./... -count=1` 与 `go test ./... -count=1` 全包通过；`go mod tidy -diff` 无依赖元数据漂移，`golangci-lint run ./...` 返回 `0 issues`。
- 前端定向测试、typecheck、ESLint、全量 Vitest 242 个测试文件 / 1602 条测试和生产构建通过。
- docs-site 生产构建通过；Go 格式、whitespace、冲突路径和冲突标记检查通过。

未验证：

- 浏览器人工 Passkey / 模型价格橱窗 smoke。
- 真实 WebAuthn HTTPS 域名与硬件认证器流程。
- Docker / Testcontainers 运行时集成测试。

## 2026-07-27 - 将上游 `main` 合并到 `dev-zz-develop`：面板 API 分层限流与 dev-zz 路由合同合流

分支：

- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`d96b6a31f`
- 合并前目标：`255eebdcd`
- 上游 head：`dc893dd0b`
- 结果提交：本次合并提交

上游要点：

- 新增面板 API 限流：认证接口按用户 ID、重查询按 Heavy 档位、公开设置按公网客户端 IP 计数，管理员默认豁免。
- 管理端可以热配置总开关、每用户 RPM、Heavy RPM、管理员豁免和公开 IP RPM；配置有进程缓存，Redis 错误 fail-open。
- 系统设置页面新增面板限流卡片、双语文案和保存回归测试；README 赞助商列表移除 BytePlus / 火山引擎条目和对应 logo。

合并策略：

- 合并前重新读取 `branch-policy.md`、`maintenance/merge-main.md`、最新补丁/合并/变更记录、变更地图和验证矩阵；发现工作区位于 `dev-zz` 后先切回与正式线完全对齐的 `dev-zz-develop@255eebdcd`。
- `git fetch --prune origin` 后确认 `origin/main@dc893dd0b` 比上一轮上游增加 3 个提交；先用 `git merge-tree --write-tree --messages --name-only` 预演，再执行 `git merge --no-commit origin/main`，预演与真实合并均得到 7 个内容冲突。
- 接受上游面板限流和赞助商列表更新；继续保留 dev-zz 企业成员预算、Key 自助查询、模型级限流、owner 用量分析、设置保存合同和现有视觉。

冲突文件：

- `backend/internal/handler/dto/settings.go`
- `backend/internal/server/router.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/server/routes/auth.go`
- `backend/internal/server/routes/user.go`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/views/admin/SettingsView.vue`

解决说明：

- `router.go` 同时传递 dev-zz 的 `memberBudgetService` 和上游 `panelRateLimiter`；Gateway 企业成员预算链路不回退，Auth、User、Admin 和 Payment 路由获得面板 limiter。
- 语义复核补齐独立注册的 `/admin/payment` 路由组：管理员豁免关闭后，该组也会和常规 `/admin` 路由一样进入 Global 档，不再绕过面板限流。
- Admin 设置同时注册 `/model-rate-limit` 和 `/panel-rate-limit`，DTO、前端 API、页面状态、加载和保存方法做并集，不删除任何一套限流能力。
- Auth 路由保留 `/api/v1/key/*` 自助查询的原 fail-close 专用限流，同时为公开设置接入 PublicIP、为登录后接口接入 Global 面板限流。
- User 路由保留 dev-zz 的 Key 日/趋势/模型统计、企业成员和 owner analytics；三条 Key 统计路由及 `/usage` 聚合组统一叠加 Heavy 档位。
- dev-zz 独有的 `channel_model_delivery_route_test.go` 和 `user_routes_test.go` 补入新的 limiter 参数，继续在 nil limiter 下验证非限流路由合同。

验证：

- 面板限流 middleware、设置 service/handler、Auth/User/Admin/Payment/Gateway 路由定向测试通过。
- 后端 `go test -tags=unit ./... -count=1` 与 `go test ./... -count=1` 全包通过；`go mod tidy -diff` 无依赖元数据漂移，`golangci-lint run ./...` 返回 `0 issues`。
- 前端 SettingsView 定向测试、typecheck、ESLint、全量 Vitest 和生产构建通过。
- docs-site 生产构建、Go 格式、whitespace、冲突路径和冲突标记检查通过。

未验证：

- 浏览器人工限流 smoke。
- 真实 Redis 并发/吞吐压测。
- Docker / Testcontainers 运行时集成测试。

## 2026-07-27 - 将上游 `main` 合并到 `dev-zz-develop`：Antigravity 原生兼容与下拉边界修复合流

分支：

- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`95590b553`
- 合并前目标：`252b3f649`
- 上游 head：`d96b6a31f`
- 结果提交：本次合并提交

上游要点：

- Antigravity OAuth 新增 OpenAI Chat Completions / Responses 到原生 `v1internal:streamGenerateContent` 的请求、流式事件和非流式响应转换。
- Antigravity 只有 usage、没有可交付内容的响应按失败处理；账号凭据拒绝消息脱敏，并使用真正尝试过的 endpoint 更新账号状态。
- Gemini Messages 兼容区分显式服务端 Google Search 与 Hermes 风格客户端 `web_search` function，避免普通函数被错误转换。
- 分组说明支持换行、长文本断行和三行截断；通用 Select 根据视口边界夹紧并收缩下拉层。

合并策略：

- 合并前完整读取 `branch-policy.md`、`maintenance/merge-main.md`、历史合并记录、补丁/变更记录、变更地图和验证矩阵；确认 `dev-zz-develop@252b3f649` 工作区干净并与 `origin/dev-zz-develop` 对齐。
- 刷新远端后先执行 `git merge-tree --write-tree --messages --name-only` 预演，再执行 `git merge --no-commit origin/main`；预演和真实合并都只产生一个 add/add 测试冲突。
- 接受上游 Antigravity 原生兼容、Hermes Web Search 正确性和组件边界修复；继续保留 dev-zz 企业成员路由 / 预算 / 归因、模型原生多协议 `DeliveryDecision`、owner / admin 边界、长期证据、stone / neutral / emerald 视觉和现有版本线。

冲突文件：

- `frontend/src/components/common/__tests__/Select.spec.ts`

解决说明：

- 保留 dev-zz 的 outside-click 回归合同：document 继续使用捕获阶段监听，即使祖先节点通过 `@click.stop` 阻止冒泡，下拉框也会正确关闭。
- 同时接入上游四个视口边界合同：空间充足时保持最小宽度、靠近右边界时收缩、左侧越界时夹紧到 padding、完全位于右侧视口外时按可用宽度收敛。
- 两组测试使用独立 wrapper，并统一清理 DOM、`innerWidth` 和 Vitest mock，避免跨测试状态泄漏。
- 提交前语义审查补齐 Responses failover 耗尽后的企业成员重试标记：凭据拒绝、429 或组内无最终错误且响应尚未提交时允许编排器切换下一候选组；流式响应已开始后仍不设置标记，避免不安全重放。

验证：

- Antigravity / Gemini / endpoint / credential / Web Search 后端定向测试通过。
- 后端 `go test -tags=unit ./... -count=1` 与 `go test ./... -count=1` 全包通过；`go mod tidy -diff` 无依赖元数据漂移，`golangci-lint run ./...` 返回 `0 issues`。
- 前端 Select / GroupOptionItem 定向测试、typecheck、ESLint、239 个测试文件 / 1579 条 Vitest 和生产构建通过。
- docs-site 生产构建通过；`git diff --check`、`git diff --cached --check`、冲突路径与冲突标记检查通过。

未验证：

- 浏览器人工 smoke。
- Docker / Testcontainers 运行时集成测试。

## 2026-07-27 - 将上游 `main` 合并到 `dev-zz-develop`：设置局部更新、用量筛选、协议兼容与支付统计合流

分支：

- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`eb6e3d1f1`
- 合并前目标：`d35a45d2c`
- 上游 head：`95590b553`
- 结果提交：本次合并提交

上游要点：

- 系统设置 PUT 只保留请求实际携带字段，避免局部表单把未提交设置写成零值；`CONFIG_FILE` 显式路径继续优先生效。
- 管理员用量记录支持按 `request_id` 精确筛选，并补充 mapped/final upstream model 统计修正。
- OpenAI Responses 与 Anthropic 兼容层吸收 reasoning、tool output、prompt cache 和 Claude Code 伪装流量识别修复；跨账号 failover 会移除不适合目标账号的外部 reasoning。
- 支付统计按币种分组，后台支付看板、图表和排行榜增加币种维度；渠道定价补充 Antigravity Gemini 3.6 Flash。
- 用户模型状态时间线在窄卡片下不再横向溢出。

合并策略：

- 合并前读取 `docs-site/dev-zz` 分支策略、上游合并流程、补丁记录、历史合并记录、变更记录、变更地图和验证矩阵；在 `dev-zz-develop@d35a45d2c` 上继续未完成的 `origin/main` 合并。
- 接受上游设置局部更新、用量 `request_id` 筛选、Responses / Anthropic 正确性、CONFIG_FILE、支付统计和监控时间线修复。
- 继续保留 dev-zz 的 OpenAI Fast/Flex 策略原子保存、企业成员使用记录可见性、模型多协议调度、stone / neutral / emerald 视觉和账号行操作密度调整。

冲突文件：

- `backend/go.sum`
- `backend/internal/handler/admin/setting_handler_update.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/pkg/apicompat/types.go`
- `backend/internal/pkg/usagestats/usage_log_types.go`
- `backend/internal/repository/usage_log_repo_query.go`
- `backend/internal/service/setting_update.go`
- `frontend/src/components/user/monitor/MonitorTimeline.vue`

解决说明：

- `SettingService` 新增合并后的 `UpdateSettingsWithAuthSourceDefaultsAndOpenAIFastPolicyOmitting`，让局部 payload 的 omitted keys 与 OpenAI Fast/Flex 策略校验、序列化和单次 repository 写入并存；缓存刷新在局部更新后回读存储值。
- 管理员设置 handler 改为调用带 omitted keys 的 Fast/Flex 方法，保留策略变更审计和支付配置后续处理。
- usage filters 同时保留企业成员 `MemberID` / `MemberScope` / owner visible member 边界和上游 `RequestID` 精确筛选；SQL where 先追加成员可见性约束，再追加 `request_id` 条件。
- Responses handler 的协议选择与上游 passthrough failover body 派生合流，使用每次尝试派生后的 `attemptBody` 进入 `ForwardWithSelectedProtocol`。
- `apicompat` 类型同时保留 dev-zz 的 Responses tool 大小限制 / raw definition 比较和上游 function_call_output JSON / string 兼容解析。
- 监控时间线保留 dev-zz 自定义 tooltip、i18n 和无障碍语义，同时把外层条目改为 `min-w-0`，内层柱体保留最小可见宽度，避免窄卡片撑宽。

验证：

- `CI=true corepack pnpm@10.34.5 --dir frontend install --frozen-lockfile` 通过；GitHub Actions 固定到兼容 Node 20 的 pnpm 10，既能读取 `pnpm-workspace.yaml` 中的 overrides，也不会触发 pnpm 11 对 Node `>=22.13` 的运行时要求。
- 后端冲突相关包定向测试通过：`mise x -C backend -- go test ./internal/handler/admin ./internal/repository ./internal/service ./internal/pkg/apicompat ./internal/config -run 'Test(Settings|OpenAIFastPolicy|Usage|Payment|Responses|Anthropic|Reasoning|Probe|RequestType|Partial|Composite|Pricing|ConfigFile|FinalUpstream|Mapped)' -count=1`。
- 后端主要包测试通过：`mise x -C backend -- go test ./internal/handler ./internal/repository ./internal/service ./internal/pkg/apicompat ./internal/config -count=1`。
- 后端 unit 全包通过：`mise x -C backend -- go test -tags=unit ./... -count=1`。
- 后端默认标签全包通过：`mise x -C backend -- go test ./... -count=1`；`go mod tidy -diff` 无依赖元数据漂移。
- `mise x -C backend -- golangci-lint run ./...` 返回 `0 issues`。
- 前端 `pnpm --dir frontend typecheck`、`pnpm --dir frontend lint:check`、238 个测试文件 / 1574 条 Vitest 和 `pnpm --dir frontend build` 通过。
- docs-site `pnpm --dir docs-site docs:build` 通过；仅出现既有大 chunk 警告。
- `git diff --check`、`git diff --cached --check` 和冲突路径检查通过；未发现剩余 unmerged path。

未验证：

- 浏览器人工 smoke。
- Docker / Testcontainers 运行时集成测试。

## 2026-07-27 - 将上游 `main` 合并到 `dev-zz-develop`：WebSocket 轮次计费、审计配置与管理端筛选正确性合流

分支：

- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`2730c1c43`
- 合并前目标：`2fbaf5a23`
- 上游 head：`eb6e3d1f1`
- 结果提交：本次合并提交

上游要点：

- OpenAI Responses WebSocket 补齐每个 turn 的请求模型、上游模型、渠道映射快照、计费模型和调度结果，避免长连接后续 turn 沿用首轮计费证据。
- 提示词审计配置在运行快照不可用时明确返回服务不可用，不再伪装成默认配置；Grok 管理员手工测试遇到 `402 Payment Required` 时会临时暂停账号。
- 管理员用量页从路由 `user_id` 进入时回填用户标签，并防止迟到查询覆盖新的筛选输入；注册页在返佣开启、强制邀请码关闭时展示可选邀请码。
- Caddy 压缩规则排除 `text/event-stream`，避免 SSE 被压缩缓冲；对应检查脚本兼容不同 awk 实现并进入后端 CI。
- 上游旧版可用渠道表增加移动端卡片，但该组件在 dev-zz 已由模型广场替代。

合并策略：

- 合并前完整读取 `branch-policy.md`、`maintenance/merge-main.md`、历史合并记录、补丁/变更记录、变更地图和验证矩阵；把合并前已有的 `frontend/pnpm-lock.yaml` 修改单独保存后，确认 `dev-zz-develop@2fbaf5a23` 与 `origin/dev-zz` 完全对齐。
- 刷新远端并先用 `git merge-tree --write-tree --messages --name-only` 预演，再执行 `git merge --no-commit origin/main`；预演和真实合并均得到 6 个冲突。
- 接受上游 WebSocket turn 级计费证据、提示词审计可用性、Grok `402` 冷却、用量筛选、可选返佣码和 Caddy SSE 修复；继续保留 dev-zz 企业成员同步落账与预算结果不明、Composite 候选路由、WebSocket 首轮路由锁、模型广场、stone / emerald 视觉和 `1.7.18` 版本线。

冲突文件：

- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/service/openai_ws_forwarder_ingress.go`
- `frontend/src/components/channels/AvailableChannelsTable.vue`
- `frontend/src/components/channels/__tests__/AvailableChannelsTable.spec.ts`
- `frontend/src/views/admin/UsageView.vue`
- `frontend/src/views/admin/__tests__/UsageView.spec.ts`

解决说明：

- WebSocket 同时保留 dev-zz 的连接级公共模型 / 上游路由锁和企业成员逐 turn 预算上下文，并接入上游每轮实际上游模型、渠道证据、计费模型与调度反馈。连接内省略或重复同一模型可以继续，切换模型、平台或更新后的渠道目标仍要求重连；客户端策略拒绝不会误伤账号健康。
- `recordCyberPolicyIfMarked` 和异步 usage 继续使用对应 turn 的企业预算 context、payload hash 与同步落账失败标记；渠道映射链按该连接真正使用的锁定路由记录，不用连接期间变化的配置改写历史证据。
- 管理员用量页同时执行 dev-zz 路由 query 清洗 / 顶部对象上下文恢复和上游用户标签回填；异步标签查询以用户 ID 与输入 revision 双重校验，不能覆盖管理员后续输入。
- 已由模型广场替代的 `AvailableChannelsTable.vue` 及旧测试继续保持删除，不恢复上游旧表移动端实现。注册页可选返佣码使用 dev-zz stone 色板。

验证：

- 后端 `go test -tags=unit ./... -count=1`、受影响的 `handler` / `securityaudit` / `service` / `service/openai_ws_v2` 完整包测试与最终冲突定向回归通过；WebSocket passthrough / ctx-pool 的连接路由锁、逐 turn usage、渠道映射变化、图片计费模型和客户端策略拒绝均包含在内。
- `golangci-lint run ./...` 返回 `0 issues`；Caddy SSE / 非 SSE 压缩合同脚本通过。
- 前端 typecheck、ESLint、237 个测试文件 / 1567 条 Vitest 和生产构建通过；路由用户标签、迟到响应隔离、可选返佣码与 Turnstile 互斥均有定向覆盖。
- docs-site 生产构建、冲突标记、Go 格式和 whitespace 检查通过。

未验证：

- 浏览器人工 smoke。
- Docker / Testcontainers 运行时集成测试。

## 2026-07-26 - 将上游 `main` 合并到 `dev-zz-develop`：OpenAI Live、会话证据与网关正确性合流

分支：

- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`cd8bb98c4`
- 合并前目标：`135720a6b`
- 上游 head：`2730c1c43`
- 结果提交：本次合并提交

上游要点：

- 新增 OpenAI Live HTTP / sideband 网关、分组级 `allow_live`、Live usage 类型、租约失效收口和 macOS attestation；管理员可以先探测当前运行环境能力，再决定是否开启分组。
- usage 与异步 batch image 新增经过统一清洗的客户端 `session_id` 请求证据；该字段只来自显式会话头，不从 `prompt_cache_key` 或内容哈希派生，也不改变 sticky、调度、计费或 request ID 语义。
- Ollama Cloud 用量刷新增加按模型请求活动、抓取 debounce、公平候选和 PostgreSQL 16 兼容修复；仍只作为管理员观察，不进入账号健康、调度或计费。
- 注册邮箱别名查重收紧根点、加号和并发边界，新增表达式并发索引；公告管理增加预览动作并共享富文本样式，前端依赖同时吸收 postcss 安全升级。
- 同步 OpenAI Responses item ID / namespace、同账号重试、Grok / Gemini 媒体、图片请求证据、远端定价和移动端返佣文案等正确性修复。

合并策略：

- 在干净且与 `origin/dev-zz` 对齐的 `dev-zz-develop@135720a6b` 上刷新远端，先用 `git merge-tree --write-tree --messages --name-only` 预演，再执行 `git merge --no-commit origin/main`；预演和真实合并均得到 20 个内容冲突。
- 接受上游 OpenAI Live、客户端会话证据、Ollama Cloud、注册安全、公告预览与网关正确性修复；继续保留 dev-zz 企业成员同步落账、预算结果不明、请求级 `ActiveGroup` 归因、Composite / 原生多协议、长期证据、stone / emerald 视觉和 `1.7.17` 版本线。
- `session_id` 与 `request_payload_hash`、企业成员快照、`schedule_meta`、`member_budget_request_id` 并存；普通请求继续走有界 worker，企业成员请求同步持久化，图片用量在队列丢弃时继续走 mandatory 同步回退。
- `187`–`190` 的上游迁移与 dev-zz 既有企业成员迁移按完整文件名并存，不重命名、覆盖或修改任何已应用迁移。

冲突文件：

- `backend/cmd/server/VERSION`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/gateway_handler_chat_completions.go`
- `backend/internal/handler/gateway_handler_responses.go`
- `backend/internal/handler/gemini_v1beta_handler.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_embeddings.go`
- `backend/internal/handler/openai_images.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/usage_log_repo_insert.go`
- `backend/internal/repository/usage_log_repo_query.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/internal/service/batch_image_public_test.go`
- `backend/internal/service/gateway_usage_billing.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_usage.go`
- `backend/internal/service/openai_upstream_transport_error.go`
- `frontend/src/components/common/AnnouncementBell.vue`
- `frontend/src/components/common/AnnouncementPopup.vue`

解决说明：

- 所有 Gateway / OpenAI / Gemini 用量入口都保留企业成员感知的提交 helper，并加入上游清洗后的 `SessionID`；OpenAI Chat / Embeddings 同时保留请求载荷指纹，图片继续保留企业成员同步落账和 mandatory fallback。
- usage / batch image SQL 按最终字段并集重新对齐列、占位符、参数、扫描顺序和批量容量；`GroupID` 继续取请求上下文中的最终 `ActiveGroup`，不回退到 Key 的静态分组。
- Live create 路由加入专用的企业成员候选与 Composite `session.model` 解析：只有最终目标为 OpenAI 且候选分组 `allow_live=true` 才创建调用；sideband 不重放创建请求，而是用当前仍授权的候选分组逐一匹配持久化 call 身份。Responses、Alpha Search、WebSocket 和模型目录的既有 Composite / 企业成员编排保持不变。
- Live 调用记录和最终零用量证据同时保存企业成员 ID、编号 / 名称快照与实际 group；usage 写入失败不再静默丢弃，会用 call hash 和内部数字 ID 记录结构化错误。鉴权缓存使用包含 dev-zz v19 与 Live gate 的新快照版本。
- `OpenAIGatewayService` 同时保留模型原生协议能力和 Live attestation；传输错误处理复用一次分类结果，并补充 Ollama Cloud 活动刷新。
- 公告预览接受上游 `displayedAnnouncement`、关闭生命周期和共享 CSS；共享样式使用 dev-zz stone / emerald 规则，未采用上游 gray / blue / amber 视觉。

验证：

- `mise x -C backend -- make generate` 通过且生成物无漂移；`go test -tags=unit ./... -count=1` 与 `go test ./... -count=1` 全绿，Live / Session ID / batch image 结算回归包含在内。
- `golangci-lint run ./...` 返回 `0 issues`；冲突标记、Go 格式和 staged / unstaged whitespace 检查通过。
- Colima / PostgreSQL 上的 repository integration-tagged 迁移与 `session_id` 测试通过，证明 `187`–`190` 上游迁移能与 dev-zz 既有完整文件名迁移共同执行。
- 前端 typecheck、ESLint、236 个测试文件 / 1560 个 Vitest 和生产构建通过；公告预览、分组 Live 能力 mock 与用量展示包含在回归内。
- docs-site 生产构建通过。

未验证：

- 浏览器人工 smoke。

## 2026-07-23 - 将上游 `main` 合并到 `dev-zz-develop`：Ollama Cloud 用量、支付宝移动唤起与网关修复合流

分支：

- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`ba88cc239`
- 合并前目标：`b0f785038`
- 上游 head：`cd8bb98c4`
- 结果提交：本次合并提交

上游要点：

- 新增 Ollama Cloud 官方用量读取和定时刷新：管理员为符合条件的 Ollama OpenAI / Anthropic API Key 账号保存 Web session 后，可查看 5 小时、7 天、余额和模型请求窗口；会话使用固定加密密钥保存，API、审计和普通日志不回显明文。
- 支付宝官方支付新增移动端 `alipay.trade.precreate` + App Scheme 唤起；功能默认关闭，唤起失败时展示动态二维码，桌面端和既有 WAP 流程保持不变。
- OpenAI passthrough 输入归一化、流式代理隔离、模型限流重置展示、渠道定价模型名归一化、Codex identity 导入索引、Grok 402 冷却和简单模式图片能力继续修正。
- Ollama 账号仓储读取改为深拷贝，凭据清理边界和审计脱敏收紧；组级图片权限进入 API Key 鉴权缓存失效合同。

合并策略：

- 在干净的 `dev-zz-develop@b0f785038` 上刷新远端并执行 `git merge --no-commit origin/main`，实际产生 9 个内容冲突。
- 接受上游 Ollama Cloud 用量、支付宝移动唤起和网关正确性修复；继续保留 dev-zz 模型原生多协议、供应商成本、账号归档、企业成员路由/预算/归因、stone / emerald 视觉、fork 镜像和 `1.7.16` 版本线。
- Ollama 用量是管理员只读观察，不参与账号健康、调度、计费或用户 DTO；Web session 只有配置固定 `TOTP_ENCRYPTION_KEY` 后才能持久化。
- `186_alipay_mobile_precreate_deep_link.sql`、`186_group_auth_cache_image_generation.sql` 与既有 `186_enterprise_member_removal_lifecycle.sql` 按完整文件名并存，不修改已应用迁移。

冲突文件：

- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/dto/types.go`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/components/account/AccountStatusIndicator.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/common/DataTable.vue`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/AccountsView.vue`

解决说明：

- `AccountHandler` 和前端账号 DTO 同时保留 dev-zz 模型协议能力、供应商成本/归档字段，并接入 Ollama Cloud 用量状态；没有把管理员观察字段扩散到用户目录。
- 账号列表继续保留供应商成本和协议操作，只增加 Ollama 用量列；状态组件吸收上游统一倒计时格式，同时保留 dev-zz 模型限流清理操作。
- `DataTable` 接受上游宽度约束，继续使用 dev-zz stone 主题；账号编辑同时保留 OpenAI cache token 用量模式和 Ollama 用量状态。
- Wire 从合并后的 provider graph 重新生成，模型自检、企业 worker、异步图片和 Ollama 用量后台服务同时存在。

验证：

- 后端目标包测试、`make test-unit`、全仓编译检查和 integration-tagged service / repository 测试二进制编译通过；`golangci-lint` 为 `0 issues`。
- 前端 typecheck、ESLint、234 个测试文件 / 1547 个 Vitest 和生产构建通过。
- Ollama Cloud 管理 API、会话加密/脱敏、定时刷新、账号状态 UI，以及支付宝移动唤起、二维码回退和支付流程均有定向回归测试。
- Wire 重生成无漂移；docs-site 构建、部署脚本语法、Compose 配置、冲突标记和 whitespace 检查通过。

未验证：

- 浏览器人工 smoke。
- Docker / Testcontainers 运行时集成测试。

## 2026-07-23 - 将上游 `main` 合并到 `dev-zz-develop`：Composite 路由、推理策略与现有企业交付架构合流

分支：
- 目标：`dev-zz-develop`
- 上游：`origin/main`
- Base：`bfabfe60c`
- 合并前目标：`34b41f559`
- 上游 head：`ba88cc239`
- 结果提交：本次合并提交

上游要点：
- 新增 Composite 分组与模型路由注册表，支持按公开模型、端点和优先级映射到具体平台与上游模型，并补齐 Grok / OpenAI / Gemini、媒体和计费归因。
- 分组新增 OpenAI reasoning effort 映射与上限策略；OpenAI / Grok Responses、WebSocket、客户端工具、模型目录和上游错误处理继续修正。
- 管理端订阅套餐、系统更新 / 回滚长请求、移动端 Ops 布局、账号操作浮层、站点 Logo 和部署安装流程更新。
- axios 与 Go `x/text` 等依赖更新，包含安全修复。

合并策略：
- 合并前完整读取 `branch-policy.md`、`maintenance/merge-main.md`、历史合并记录、补丁/变更记录、变更地图和验证矩阵；在干净的 `dev-zz-develop@34b41f559` 上执行 `git merge --no-commit origin/main`，实际产生 56 个内容冲突。
- 接受上游 Composite 路由、推理强度策略、Grok / OpenAI 正确性、安全依赖与响应式布局；继续保留 dev-zz 企业成员有序分组、预算/归因、模型原生多协议、供应商成本、长期留存、stone / emerald 视觉、fork 镜像和 `1.7.16` 版本线。
- Composite 解析不作为企业成员编排外层的单次中间件：每个候选分组都从原始公开请求重新解析，切换候选前清除上一组的目标平台、上游模型和公开模型决策；只有响应尚未提交且失败被显式分类为可重试时才允许进入下一组。
- Ent 和 Wire 均从合并后的 schema / provider graph 重新生成；`pnpm-lock.yaml` 从合并后的依赖声明重新计算。

关键冲突：
- 后端：`gateway.go`、`gateway_handler.go`、`openai_gateway_handler.go`、企业成员中间件、API Key DTO、上游模型目录、Ops 归因、WebSocket HTTP bridge、Ent 生成物和 Wire。
- 前端：`AccountsView.vue`、`AppHeader.vue`、认证 / 首页、用量图表、Ops 卡片与弹窗、订阅页、`AvailableChannelsTable.vue` 删除冲突、`package.json` 和 `pnpm-lock.yaml`。
- 部署：`.env.example`、`docker-compose.yml`。

解决说明：
- HTTP Composite 路由按企业成员候选组内执行；第一组的模型改写不会污染第二组。无匹配路由以 typed capability mismatch 结束当前候选，允许事务响应仍未提交时继续下一个授权组。
- Responses WebSocket 在读取首个 `response.create` 后解析显式 Composite 路由；切组后重新解析，并把最终上游模型写入首帧。连接在首 turn 固定公开模型到最终上游模型的映射，后续 turn 省略模型或重复同一公开 / 上游模型均可继续，切换模型或平台必须重新建连。
- WebSocket 仅允许首 turn 在尚未产生下游事件时安全切换账号 / 分组；后续 turn 的 429 或未知传输结果不得触发整连接首帧重放。成员预算只在最终分组、账号和上游模型稳定后预留，未知结果进入结果不明闭环并阻止候选重试。
- 模型目录同时保留企业成员跨组模型并集、Composite 精确公开别名、可调度平台模型、大小写不敏感去重和 dev-zz `supported_endpoint_types` 元数据；前缀规则和禁用规则不发布为静态目录项。
- Ops 恢复事件按实际失败 attempt 的账号、分组和具体平台归因，不被最终成功候选覆盖；企业成员身份继续取当前请求身份。Gemini 模型目录遇到不匹配的 Composite 候选时退出当前候选并继续后续 Gemini 分组。
- 候选请求重放会同步恢复 body、`ContentLength` 和 `Content-Length`；一旦预算结果被标记为不明确，编排器统一禁止跨组重试。
- API Key 更新继续保留 tags、IP ACL 与状态的“省略即不修改”指针语义；鉴权缓存版本升级并同时覆盖推理策略、Web Search 计费和企业成员限制。
- 前端继续使用 dev-zz stone / neutral / emerald 主题，吸收上游浮层定位、响应式移动卡片和无障碍状态；已由模型广场替代的 `AvailableChannelsTable.vue` 及上游新增的旧组件测试保持删除。
- Compose 默认镜像保持 `thornboo/sub2api:latest`，并接受上游 `UPDATE_GITHUB_TOKEN`；`VERSION` 保持 `1.7.16`，不采用上游 `0.1.163`。

验证：
- Ent / Wire 重生成后无漂移；后端 `make test-unit` 全绿，核心 Handler / Service / Middleware / Routes 包测试全绿，`golangci-lint` 为 `0 issues`，全仓编译检查通过。
- 前端 typecheck、lint、230 个测试文件 / 1516 个 Vitest 和生产构建全绿；文档构建、Compose 配置、部署脚本语法、冲突标记与 whitespace 检查通过。
- Composite 精确别名目录、企业候选逐组解析、请求元数据恢复、预算结果不明禁重试、Gemini 候选回退、Ops 失败 attempt 归因、WebSocket 固定模型映射及仅首 turn failover 均有定向回归测试。

未验证：
- 浏览器人工 smoke。
- Docker / Testcontainers 运行时集成测试。

## 2026-07-20 - 将上游 `main` 合并到 `dev-zz`：入口安全、鉴权缓存、运行时对象存储与 Grok 媒体闭环合流

分支：
- 目标：`dev-zz`
- 上游：`origin/main`
- Base：`b1a6b8026`
- 合并前目标：`8a7b65f54`
- 上游 head：`bfabfe60c`
- 结果提交：本次合并提交

上游要点：
- 新增入口鉴权拒绝聚合、无效凭据滥用限制和鉴权缓存失效 outbox，减少无效 Key 对数据库与运维错误明细的放大，同时提供清理命令和健康状态。
- 客户端 IP 解析改为显式可信代理 / 请求头设置，并将配置、审计、部署示例和 Caddy 边缘安全说明串成同一合同。
- 异步图片对象存储改为后台热配置并支持环境变量启动；Grok 视频内容使用同源代理、请求所有者隔离、签名地址校验和已持久化任务路由。
- 上游倍率探测和账号列表新增有效倍率 / 峰值倍率排序，OpenAI WebSocket、流式错误、模型级临时冷却及 Responses 兼容继续修正。

合并策略：
- 合并前完整读取 `docs-site/dev-zz` 的分支策略、上游同步流程、历史合并记录、补丁/变更记录、变更地图和验证矩阵；刷新 `origin/main` 后先用 `git merge-tree --write-tree` 只读预演，再执行 `git merge --no-commit origin/main`。
- 预演和真实合并均得到 38 个内容冲突。接受上游入口安全、鉴权缓存、客户端 IP、对象存储热配置、Grok 媒体和倍率探测修复；继续保留 dev-zz 企业成员有序分组、预算/归因、owner/admin 数据边界、永久留存、供应商成本、stone/emerald 视觉、fork 镜像和 `1.7.13` 版本线。
- `183_ops_ingress_reject_aggregates.sql`、`184_auth_cache_invalidation_outbox.sql` 与 dev-zz 既有更高编号迁移按完整文件名并存；没有修改任何已应用迁移。

冲突文件：
- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/admin/setting_handler_update.go`
- `backend/internal/handler/grok_media.go`
- `backend/internal/handler/image_task_handler.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/ops_error_logger.go`
- `backend/internal/handler/ops_error_logger_test.go`
- `backend/internal/handler/wire.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/repository/account_repo_sort_integration_test.go`
- `backend/internal/repository/ops_error_where_test.go`
- `backend/internal/repository/ops_repo.go`
- `backend/internal/repository/ops_repo_args_test.go`
- `backend/internal/repository/ops_repo_get_error_log_by_id_integration_test.go`
- `backend/internal/server/middleware/api_key_auth.go`
- `backend/internal/server/middleware/api_key_auth_google.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/service/api_key_service.go`
- `backend/internal/service/gemini_chat_completions_compat_service.go`
- `backend/internal/service/grok_media.go`
- `backend/internal/service/image_task.go`
- `backend/internal/service/openai_account_runtime_block_fastpath.go`
- `backend/internal/service/openai_gateway_grok_test.go`
- `backend/internal/service/openai_ws_v2_passthrough_adapter.go`
- `backend/internal/service/ops_port.go`
- `backend/internal/service/ops_service.go`
- `backend/internal/service/ops_service_user_error_test.go`
- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/ratelimit_service_model_not_found_test.go`
- `backend/internal/service/wire.go`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/common/DataTable.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`
- `frontend/src/views/admin/__tests__/SettingsView.spec.ts`
- `frontend/src/views/admin/ops/components/OpsSettingsDialog.vue`

解决说明：
- Wire 从合并后的 `wire.go` 重新生成，同时注入上游入口拒绝聚合、鉴权缓存失效 worker、Ops 运行时刷新和 Grok quota，以及 dev-zz 企业成员预算、图片预算恢复、媒体任务仓储和导入 worker。
- API Key / Google 鉴权保留企业成员状态与分组验证，同时把入口拒绝写入聚合管线；删除 Key 的明文归属不再进入通用错误日志，owner 查询严格要求当前 `user_id`，管理员审计仍保留未过滤查询。
- 图片任务继续执行企业成员预算恢复、异步任务 fence 和结果不明闭环，同时对象存储解析器改为保存即生效的运行时设置；Grok 视频状态和内容优先使用持久化 group/account 路由，不能跨凭据租户回退。
- OpenAI 首输出超时在普通 Key 尚未写出语义内容时允许复用客户端连接切换账号；存在企业成员预算凭据时停止重放并把 receipt 标记为结果不明，避免同一 turn 产生重复上游副作用或重复计费。
- OpenAI 错误处理按“明确模型不存在、管理员临时规则、通用模型冷却”依次判定；OAuth 账号配置了 429 规则时，匹配项只冷却 account+model，未匹配项继续走账号级短冷却，未配置规则的普通模型 429 保持模型级 failover。
- 账号列表同时保留 dev-zz 供应商成本排序和上游有效倍率/峰值倍率排序；`DataTable`、账号页和设置页继续使用 stone/emerald 与可访问复选框，并吸收上游表头 slot、倍率列和客户端 IP 设置。
- `VERSION` 保持 `1.7.13`，Compose 默认镜像保持 `thornboo/sub2api:latest`，不采用上游 `0.1.161` 版本线。

验证：
- `mise x -C backend -- go run github.com/google/wire/cmd/wire ./cmd/server`
- `mise x -C backend -- go test ./internal/handler ./internal/repository ./internal/server/middleware ./internal/server/routes ./internal/service ./cmd/cleanup-ingress-reject-logs ./cmd/server -count=1`
- `mise x -C backend -- make test-unit`
- `mise x -C backend -- golangci-lint run --timeout=30m`（0 issues）
- `mise x -C backend -- go test -tags=integration -c -o /tmp/sub2api-repository-integration.test ./internal/repository`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend test:run`（214 个测试文件、1444 个测试通过）
- `pnpm --dir frontend build`
- `pnpm --dir docs-site docs:build`
- `docker-compose -f deploy/docker-compose.yml config -q`
- `git diff --check`、`git diff --cached --check`、Wire 重生成、未合并索引与冲突标记扫描。

未验证：
- 浏览器人工 smoke。
- Docker / Testcontainers 运行时集成测试；本轮只校验 Compose 配置并编译 repository integration 测试二进制。

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
