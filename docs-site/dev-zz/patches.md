# 补丁记录

## 2026-07-20 - 供应商综合折扣计价基准确认

### 问题

- 账号综合折扣原先固定按美元价目表公式除以参考汇率，导致人民币官方价分组也被错误换汇；例如资金池充值比例 `1:1`、Kimi 分组倍率 `0.8` 时，页面显示约 `1.1 折`，而真实口径应为 `8.0 折`。
- 仅凭供应商所在地、模型名或分组名无法可靠判断价目表币种；迁移前历史绑定如果直接标记为人民币或美元，会把未经管理员确认的推断包装成准确成本。
- 未确认成本若继续参与账号排序和 `cost_first`，不仅展示错误，还会实际改变上游账号选择顺序。

### 修复

- 账号成本绑定新增显式 `price_reference_currency`（`CNY` / `USD`）与 `price_reference_confirmed`。人民币价分组按“资金池人民币成本 × 分组倍率”计算，美元价分组继续按“资金池人民币成本 ÷ 参考汇率 × 分组倍率”计算。
- 账号编辑页在存在 active 供应商绑定时要求管理员明确选择分组计价基准；账号列表展示确认后的综合折扣，历史未确认绑定显示“待确认”。
- 列表排序、成本比较与调度快照共用同一计算口径；没有真实资金池成本快照或计价基准未确认的绑定，不生成可排序成本，也不进入 `cost_first`。
- 兼容旧客户端：更新同一资金池并省略新字段时保留原币种和确认状态；新绑定或切换资金池仍省略时，保留旧美元公式但标记为未确认。
- 供应商只有一个 active 资金池时，账号列表不再额外展示“主余额池”标签；默认资金池已归档时优先使用仍 active 的资金池。

### 数据与兼容性

- 新增 migration `196_upstream_binding_price_reference_currency.sql`，为既有绑定写入兼容默认值 `USD` 和 `price_reference_confirmed=false`，不根据业务名称重写历史事实。
- 管理员显式保存 `CNY` 或 `USD` 后才把绑定标记为已确认。供应商资金池、充值账本、真实成本快照和普通用户接口不变。
- 首版 `cost_first` 仍使用账号绑定的标量分组倍率；模型族倍率尚未进入请求级成本调度，文档不再把它描述成已生效能力。

### 验证

- 后端 service、repository、admin handler 与 migrations 测试通过；集成测试覆盖人民币价 `0.8` 倍率得到 `8.0 折`，以及历史未确认成本不进入调度。
- 前端 214 个测试文件 / 1448 条测试、typecheck、ESLint 和生产构建通过；docs-site 构建与 `git diff --check` 通过。
- 浏览器人工 smoke 未执行。

## 2026-07-20 - 上游 main 同步：入口安全、热配置与媒体路由

### 目标

- 将 `origin/main@bfabfe60c` 合入 `dev-zz`，吸收入口安全、鉴权缓存、客户端 IP、对象存储、Grok 媒体、上游倍率和 WebSocket 生命周期修复。
- 保持企业成员路由/预算/归因、owner/admin 数据边界、永久留存、供应商成本、fork 发布和 stone/emerald 视觉合同不回退。

### 主要变化

- 无效 Key、缺失 Key 和分组拒绝等入口失败进入聚合记录与滥用限制，不再为高频无效请求逐条放大数据库错误日志；新增鉴权缓存失效 outbox、worker、订阅健康状态及清理工具。
- 客户端 IP 来源改为显式配置的可信代理和请求头列表，设置保存会刷新运行时快照并写审计；部署示例补齐 Caddy / Docker 边缘安全说明。
- 异步图片对象存储配置进入管理端备份页并支持保存即生效，环境变量启动仍可用；图片任务继续保留企业成员预算恢复、task fence、失败释放和结果不明处理。
- Grok 视频生成、状态和内容查询绑定持久化的 group/account，受保护签名内容经本站同源代理返回并验证请求所有者，不能通过普通 failover 跨凭据读取。
- OpenAI 首输出超时按预算上下文区分：普通 Key 在未输出语义内容时允许切换账号；企业成员已有预算 receipt 时禁止重放并保留结果不明 receipt。
- OpenAI 模型失败合并双方优先级：明确的 model-not-found 先进入专用模型冷却，管理员临时规则随后按 account+model 隔离，剩余错误再走通用模型策略；配置了 OAuth 429 规则但未匹配时保持账号级短冷却。
- 账号管理同时展示并排序供应商成本、上游有效倍率和峰值倍率；设置页增加客户端 IP 与倍率探测配置，表格继续使用 dev-zz 的可访问复选框和 stone/emerald 主题。

### 数据与兼容性

- 新增 `183_ops_ingress_reject_aggregates.sql` 和 `184_auth_cache_invalidation_outbox.sql`；按完整文件名追加，不修改任何既有迁移。
- 运维错误 insert 合同删除已删除 Key 明文归属字段，保留企业成员快照与分类 v2；owner 明细查询要求当前 `user_id`，管理员审计查询继续可见完整证据。
- 正式版本号保持 `1.7.13`，Compose 默认镜像保持 `thornboo/sub2api:latest`。

### 验证

- Wire 从合并后的 provider graph 重新生成；后端 handler、repository、middleware、routes、service、server 和入口清理命令测试通过，全量 unit 与 golangci-lint（0 issues）通过，repository integration 测试二进制编译成功。
- 前端 ESLint、typecheck、214 个测试文件 / 1444 条测试和生产构建通过；docs-site 构建与最终冲突/whitespace 检查通过。
- Compose 配置校验通过；浏览器人工 smoke 和 Docker/Testcontainers 运行时集成测试未执行。

## 2026-07-20 - v1.7.11 企业成员 Key 按需复制修复

### 问题

- 企业成员 Key 列表按安全合同只返回脱敏值，但复制按钮错误复用了普通 `GET /api/v1/keys/:id` 详情接口。
- 普通 Key 详情接口会按设计拒绝所有 `member_id != NULL` 的成员 Key，因此企业 owner 点击复制稳定得到 `API key not found`；Key 本身、成员绑定和网关调用不受影响。

### 修复

- 新增 `POST /api/v1/enterprise/members/:id/keys/:key_id/reveal`，只允许启用状态的企业 owner 按当前成员读取一把未删除成员 Key；Repository 查询同时限定 owner ID、member ID、Key ID 和 `deleted_at IS NULL`。
- 普通 `/api/v1/keys/:id` 继续拒绝成员 Key，避免把成员身份和明文暴露到普通 Key 管理边界。
- “鉴权、读取、append-only 审计、返回明文”统一收敛到 `EnterpriseMemberService`；审计写入动作使用 `member_key.reveal_authorized`，只记录 owner/member/actor/Key ID 和固定来源，不记录 Key 值。审计 repository 缺失或写入失败时不返回明文。
- 成功响应仅返回 `id`、`member_id`、`key`，并禁止 HTTP 缓存；已归档成员不显示复制入口，服务端独立拒绝归档成员和已删除 Key。
- 前端在请求前冻结 member ID 与 Key ID，迟到响应遇到成员切换时直接丢弃；正常响应必须同时匹配请求的 member ID 和 Key ID 才能进入剪贴板。
- 当前与普通 Key 明文详情保持一致，要求有效 owner 登录态并写通用审计与企业成员授权审计，不单独强制 TOTP step-up。未来若提升明文凭据读取基线，必须同时覆盖普通 Key 和成员 Key。

### 兼容性与验证

- 不修改数据库结构、已有 Key、成员绑定、网关鉴权、计费或普通 Key 接口响应。
- 后端 handler/repository/service 定向测试覆盖成功最小响应、禁止缓存、跨 owner/成员拒绝、归档成员/已删除 Key 拒绝和审计失败关闭；前端覆盖真实复制调用、错误响应 ID 和成员切换迟到响应。
- 前端定向 Vitest、typecheck、ESLint，后端相关包测试、Wire 生成和 `git diff --check` 通过；远端 CI、Security Scan 和正式分支镜像以发布候选提交为准。

## 2026-07-19 - v1.7.10 Key 自助查询

### 目标

- 为无法登录站点的企业成员及普通 Key 持有者提供独立的自助查询入口，在不暴露完整 Key、其他 Key、上游账号或管理员成本的前提下，查询额度、静态可用状态、可访问分组与模型、统计、请求记录、详情和 CSV 导出。

### 主要变化

- 首页新增 Key 查询入口；浏览器使用一次性 Bearer Key 换取短时 `HttpOnly` 查询会话，完整 Key 不进入 URL、本地存储、业务接口参数或日志。
- 摘要区分当前 Key 额度与企业成员共享预算，展示成员有序分组及完整模型列表；成功记录与失败记录都强制 owner + API Key 双重归属，公开 DTO 排除上游账号、账号成本和内部错误字段。
- 查询会话采用 15 分钟空闲、1 小时绝对过期，Redis 只保存随机令牌哈希和最小身份快照；读取接口共享单 IP 60 次/分钟限流，详情和导出叠加更严格限制，Redis 故障时 fail closed。
- 前端以 session epoch 和 `AbortController` 隔离摘要、记录、详情与导出请求；退出时立即清空旧数据，撤销完成前禁止建立下一把 Key 会话，避免迟到响应和 Cookie 时序重新展示上一会话数据。
- Key 静态状态同步校验 owner、企业能力、成员状态、分组完整性及普通/独占分组授权；模型、端点、订阅/余额、IP 和实时上游资格继续留在具体请求路径判断。
- 错误 CSV 按 Repository 实际页大小继续分页到 5,000 行；成员分组返回 binding 的真实排序值；新增 `(api_key_id, created_at)` 错误记录索引支持单 Key 时间范围查询。

### 数据与兼容性

- 新增 migration `194_ops_error_logs_api_key_time_index_notx.sql`，只增加并发索引，不改写历史错误记录。
- Cookie 使用 `SameSite=Strict`，当前部署合同要求前端与 API 属于浏览器意义上的 same-site；跨站部署必须先补 Origin 白名单与 CSRF 设计。
- 正式发布版本提升为 `1.7.10`，Compose 继续默认 `thornboo/sub2api:latest`。

### 验证

- 后端 handler、service、repository、routes、middleware 与 server wiring 测试通过；新增会话生命周期、字段白名单、跨 Key 边界、导出分页、静态状态和成员 binding 排序回归测试。
- 前端 Key 查询/API 定向测试、typecheck、ESLint 和生产构建通过；覆盖原始 Key 提前清除、会话恢复/退出、迟到摘要/详情隔离和 DELETE 未完成时禁止下一次查询。
- `git diff --check` 通过；严格快照导出仍可在后续将 OFFSET 分页升级为 `(created_at, id)` keyset pagination。

## 2026-07-18 - v1.7.9 上游 main 同步：提示词审计、安全开关与 Grok 媒体资格

### 目标

- 将 `origin/main@b1a6b8026` 合入正式 `dev-zz`，吸收上游安全审计、Grok 媒体、调度和支付加载修复，同时不回退企业成员、Ops 分类、fork 发布和生产分包边界。

### 主要变化

- 新增独立提示词审计服务和 `/admin/prompt-audit` 管理页面，支持 OpenAI 兼容审计节点、指定分组/全部分组、异步审计/可选阻断、运行状态、事件详情及带快照确认的批量筛选删除；配置默认关闭，Guard token 不从管理 API 回显。
- 新增 `prompt_audit_jobs` / `prompt_audit_events` 证据表；任务只保存脱敏预览，命中事件可以保存管理员复核所需的完整提示词，审计节点凭据不写入这两张表。事件删除同时清理对应临时载荷。
- `step_up_enabled` 和 `session_binding_enabled` 在缺失配置时默认关闭；开关写入保持旧客户端省略字段即保留现值，启用后的高风险操作继续执行现有 TOTP 与会话绑定合同。
- Grok 新媒体请求使用资格探测/覆盖筛选；已创建异步视频的状态查询仍只回到原始账号。Responses WebSocket 同时保留每 turn 企业预算预留和新的安全审计阶段。
- Stripe 支付入口改为 side-effect-free 动态加载；构建继续使用 dev-zz 默认 chunk graph，不恢复会导致循环 vendor chunk 白屏的手工分包。

### 数据与兼容性

- `181_prompt_audit.sql` 与 `181_group_duplicate_operation_id.sql`、`181_ops_error_logs_member_time_index_notx.sql` 并存。
- `182_prompt_audit_full_prompt.sql` 与 `182_enterprise_member_import_baselines.sql` 并存。
- 正式发布版本提升为 `1.7.9`，Compose 继续默认 `thornboo/sub2api:latest`；没有修改既有迁移或线上数据。

### 验证

- Wire 重生成、后端全包编译、完整 unit-tag 测试、重点包普通测试、golangci-lint 和 repository integration 编译通过。
- 前端 typecheck、完整 ESLint、211 个测试文件 / 1413 条测试和生产构建通过；docs-site 构建通过。
- 真实浏览器 smoke 与 Docker/Testcontainers 运行时集成测试未执行。

## 2026-07-18 - 运维失败分类与平台 SLA 口径重构

问题：
- 原 `is_business_limited` 同时承担客户可见性、责任归因、SLA 排除和明细分流，导致平台无可用路由可能被排除出 SLA、recovered 上游尝试可能与最终客户失败混在一起。
- 总览、趋势、预聚合、健康评分和明细筛选各自拼接 SQL，字段含义容易漂移；相对时间钻取还可能在刷新边界得到与卡片不同的结果。
- 最近 6 小时的大量请求失败只能逐条查看，缺少稳定归因、处理责任和未分类数据质量入口。

修复：
- 新增分类 v2 及稳定 reason code，独立保存 `event_scope`、`customer_visible`、`failure_domain/category`、`resolution_owner`、`pool_ownership` 和可空 `sla_impact`；正常请求、流式终态、recovered attempt 和 Cyber Policy 直写路径统一双写。
- 以共享 SQL 合同驱动 raw、preagg、趋势、状态码分布和 metrics collector；旧 `error_count_total`、`business_limited_count`、`error_count_sla` 保留为新口径兼容别名，v2 unknown 不回退成主观责任判断。
- 总览新增归因分布、未分类入口和固定 15 分钟当前状态；当前状态使用管理员已有的平台 SLA 失败率阈值，所有钻取冻结 overview 的绝对起止时间并携带结构化筛选。
- 健康评分、告警和定时报表切换到平台 SLA 失败率；未分类记录会限制健康评分上限。迁移只回填最近 31 天可确定证据，索引以 `_notx` 并发创建。

验证：
- 分类矩阵和 9,907 条生产 fixture 守恒测试通过，其中 4,878 条计入平台 SLA；Repository 参数、迁移合同、raw/preagg 共享 SQL、当前状态阈值和健康评分回归测试通过。
- 后端目标包普通与 `unit` tag 测试、前端类型检查及完整 Vitest 套件通过；完整构建、全量 Go 测试和 docs build 结果见本轮最终验证记录。

剩余边界：
- 主要故障事件聚合和 HTTP 200 后流式终态去重尚未实现；本轮继续按逻辑失败请求计数，并明确包含客户端重试。
- 当前没有独立 v2 运行时功能开关；如上线后对账异常，回退上一应用版本继续读取兼容字段，不删除已经写入的 v2 分类证据。

## 2026-07-17 - v1.7.8 企业成员预算信息与调账交互收敛

问题：
- 成员预算弹窗同时展示“预算占用”“已结算”“处理中预占”“可用余额”和“本期活动”，客户难以直接回答预算、已用和剩余分别是多少。
- 小额用量会被整数百分比四舍五入为 `0%`，与已用金额互相矛盾；导入历史用量和未配置的短周期限额长期占据一级页面空间。
- 单成员调账表单默认展开并使用浏览器原生确认框，不能在写入不可变账本前清楚展示和冻结实际提交内容。

修复：
- 一级摘要只保留月预算、本月已用和剩余预算；处理中预占仅在实际存在时以说明提示出现，进度条区分已结算与预占但不改变严格预授权语义。
- 使用率按数值范围保留必要精度，`US$0.09 / US$100` 显示为 `0.09%`；请求数、Token 和导入历史记录进入默认折叠的“本月用量明细”，全部未配置时隐藏短周期限额区域。
- 调账入口进入高级折叠区并使用项目统一确认对话框；第一次提交只冻结成员、金额和说明，明确确认后才调用现有调账接口写入不可变账本。
- 不修改成员预算计算、预占、结算、请求体、数据库结构或后端 API 契约，继续保持严格预算方案。

验证：
- 前端预算布局、国际化和调账确认回归测试共 35 条通过，覆盖小额百分比与确认前不写账本、确认内容冻结。
- `vue-tsc --noEmit`、定向 ESLint、前端生产构建和 `git diff --check` 通过。
- 浏览器截图工具调用被取消，因此未将源码检查标记为交互式视觉验收通过。

## 2026-07-17 - v1.7.7 企业成员模型统计筛选闭合

问题：
- 企业“成员使用记录”会把成员范围、指定成员、模型和计费模式等条件传给用量统计接口，但模型分布端点存在手工维护的“是否带筛选”判断，未覆盖新增的成员字段。
- Service 在 Repository 未显式实现扩展接口时还会回退到较窄的旧统计方法，静默丢失完整筛选条件，使成员页面的模型分布可能展示账户全局数据。

修复：
- 用户模型统计统一调用完整 `UsageLogFilters` Repository 契约；该能力成为 `UsageLogRepository` 的强制接口，不再通过运行时类型断言选择会丢字段的兼容回退。
- 删除 handler 中容易随筛选字段增长而漂移的手工判断，普通使用记录与成员使用记录都走同一条完整筛选路径。
- 前端 `DashboardModelParams` 从公共 `UsageQueryParams` 派生，确保成员范围和成员 ID 等合法筛选可以透传，同时继续排除分页、排序和管理员专用字段。
- 不修改 usage 数据、成员归属、计费结果或数据库结构；本补丁只修正统计查询的筛选边界。

验证：
- handler HTTP 契约覆盖无成员筛选、全部成员、已分配、未分配、指定成员和不属于当前 owner 的成员，并验证完整筛选进入模型统计 Repository。
- 前端 API 契约验证 `member_scope` 与 `member_id` 会透传到模型统计请求。
- 后端 handler / repository / service / server 测试与 vet，前端目标 Vitest、typecheck、ESLint，`git diff --check`。

## 2026-07-17 - v1.7.6 企业成员无分组响应兼容

问题：
- 企业成员导入可以合法创建尚未绑定分组的待配置成员；旧返回路径会把 Go 的 `nil` 切片序列化为 `group_ids: null`。
- 成员页面按数组使用 `group_ids`，历史 `null` 响应会在渲染阶段触发异常并导致整个页面白屏。

修复：
- Repository 的实体转换与创建返回统一初始化非 `nil` 空切片，权威 API 对无分组成员输出 `group_ids: []`。
- 前端领域类型继续保持 `group_ids: number[]`；只在 Wire 边界兼容旧后端的 `null` / 缺失字段，并在列表、创建、更新、启停、恢复、单成员分组和批量分组响应进入页面前统一规范化。
- 不修改成员状态、授权分组、导入数据或数据库结构；无分组成员继续保持合法的“待配置”语义。

验证：
- Repository 私有映射、公共 `ListByOwner` 完整 enrich 链路与公共 `Create` 路径的 JSON 契约测试。
- 前端成员响应 contract spec 覆盖所有规范化入口；页面回归测试确认待配置无分组成员可渲染且不触发全局错误。
- 后端 repository / service / handler 测试与 vet，前端完整 Vitest、typecheck、ESLint 和生产构建，`git diff --check`。

## 2026-07-17 - 上游 main 同步：异步图片、倍率探测、图片计费与操作审计

### 目标

- 将 `origin/main@bc2244c83` 合入 `dev-zz-develop`，继续以 `docs-site/dev-zz` 的分支策略、接口边界和历史合并记录作为冲突裁决依据。
- 接受上游安全、计费、图片、调度和 OpenAI / Grok 兼容修复，同时不回退企业成员预算 / 归因、供应商成本、调度策略、数据保留、视觉和 fork 版本线。

### 主要变化

- 新增异步图片提交 / 查询 API；任务结果必须落 S3 兼容对象存储，Redis 只保存紧凑结果，功能默认关闭。完整协议见 `docs/ASYNC_IMAGE_TASKS.md`。
- 新增 `/v1/sub2api/billing` Key 倍率自省和管理端上游倍率探测；探测快照只保存在账号 `extra`，低倍率优先只扩展旧调度，不覆盖 dev-zz `cost_first` / `strict_priority` 策略。
- 渠道价格和 usage log 新增图片输入 Token 单价、数量与费用；SQL insert / batch insert / query、DTO、管理端表格和定价卡保持同一字段顺序。
- 新增操作审计、会话 IP/UA 绑定和敏感操作 step-up 2FA；管理员角色提升、审计清空等高风险操作保持更严格的现场验证边界。
- 分组与渠道监控复制、管理员批量用户限额、Grok 上游端点快捷切换、OpenAI WebSocket / body-limit / Responses 字段重试等能力随上游合入。
- 合并复审修正两处上游/分支语义碰撞：OpenAI APIKey 的参数 400 不进入通用持久化模型冷却，瞬时 5xx 采用 account+model 连续失败运行时冷却；DataTable / UseKeyModal 继续使用 dev-zz stone 视觉和可访问控件，同时恢复上游横向滚动与选择测试合同。

### 数据与兼容性

- `178_channel_image_input_price.sql` 与 `178_enterprise_member_import_jobs.sql` 并存。
- `179_usage_log_image_input_tokens.sql` 与 `179_enterprise_member_rate_limits.sql` 并存。
- `180_audit_logs.sql` 与 `180_ops_error_logs_enterprise_member_attribution.sql` 并存。
- `181_group_duplicate_operation_id.sql` 与 `181_ops_error_logs_member_time_index_notx.sql` 并存。
- `VERSION` 保持 dev-zz `1.7.4`，不采用上游 `0.1.158`；没有改写任何既有迁移。

### 验证

- 后端全包编译、带 `unit` build tag 的完整测试、golangci-lint 和 repository integration 编译。
- 前端 typecheck、ESLint、204 个测试文件 / 1371 个测试、生产构建。
- docs-site 构建、Wire 重新生成、冲突标记 / 未合并索引 / whitespace 检查与双父祖先校验。

## 2026-07-16 - 企业成员导入小数 Token 精确保留

实现：
- 企业成员 CSV/XLSX 导入的总量、输入、输出、缓存、缓存写入和缓存读取 Token 改为精确十进制定点值，接受非负且最多两位有效小数；`421.63` 在预览、完成结果和成员预算汇总中保持 `421.63`，第三位有效小数直接拒绝，不进行静默四舍五入。
- 单行持久化上限与多行聚合范围分离：缺省总量的输入 + 输出若无法写入基线，会在预览阶段直接拒绝；合法单行的更大聚合仍可被结果 JSON 和预算摘要读取。迁移 Token API 使用规范化十进制字符串，页面通过 `BigInt` 分组整数部分并保留小数，不经过 JavaScript `number`，百万级值也不再 compact 缩写。
- migration `191_enterprise_member_fractional_token_baselines.sql` 将六个外部迁移基线列从 `BIGINT` 升级为 `NUMERIC(21,2)`；真实请求 `usage_logs` 的 Token 计数继续保持整数，不用迁移聚合值伪造请求明细。
- 企业成员页面 Token 格式化和中英文校验提示同步支持两位小数；升级文档明确 migration 191 需要排空旧导入 worker 并停止旧实例，不能放入新旧二进制并存的滚动窗口。

验证：
- `go test -tags=unit ./... -count=1`
- `golangci-lint run ./...`
- `pnpm --dir frontend test:run`、`typecheck`、`lint:check`、`build`
- `pnpm --dir docs-site docs:build`
- `git diff --check`
- 本机未启动 Docker，真实 PostgreSQL Testcontainers schema/持久化集成测试未执行；对应 migration、精确 JSON/SQL 往返和 repository 汇总合同测试已通过。

## 2026-07-16 - v1.7.3 企业成员可靠性与上游同步发布

发布范围：
- 企业成员请求回执、usage 归因和版本化 settlement outbox，确保成功请求在本地结算故障后仍可幂等恢复，且不会重复写入成员预算或 usage 事实。
- OpenAI WebSocket、普通请求与 Batch image 的结果不明边界：上游可能已接收工作时禁止跨组重放、自动退款或释放成员预算，保留后续对账证据。
- 同步上游 `main@eb2b8632d` 的 Grok 自定义上游、OpenAI Agent Identity、账号复制、订阅币种、充值返佣和网关兼容性修复，同时保留 dev-zz 企业成员权限、预算和 owner/admin 数据边界。

发布门禁：
- `backend/cmd/server/VERSION` 提升为 `1.7.3`，正式 tag 使用不可变 annotated tag `v1.7.3`。
- 修复 integration fixture 对 `NewAdminService` 新依赖与管理型账号仓储接口的构造漂移，不修改生产运行时逻辑。
- tag 只允许建立在 `dev-zz` 精确版本提交的 CI、Security Scan 和 dev-zz Branch Images 全绿之后；发布后继续验证 GitHub Release 与 Docker Hub / GHCR 多架构镜像。

验证：
- `DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock make -C backend test-integration`
- `mise x -C backend -- go test -tags=integration ./internal/service -run '^(TestUpstream|TestArchivedUpstream|TestUsedUpstream)' -count=1 -v`
- `git diff --check`
- 修复提交 `2ebcb294f` 的远端 CI、Security Scan 和 dev-zz Branch Images 全部通过。

## 2026-07-16 - 上游 main 增量同步：Grok 自定义上游、Agent Identity 与订阅币种

实现：
- 合并 `origin/main@eb2b8632d` 的 14 个提交，覆盖 Grok 自定义 `base_url` / 请求头覆写、Agent Identity 独立导入与 Codex 能力、订阅套餐币种、管理员充值返佣设置和 locale 运行时编译保护。
- Grok OAuth 官方地址继续使用可信端点；自定义转发地址统一受 operator URL 策略校验，认证与会话头不可覆写，billing / quota / media / Responses / Chat 请求使用同一账号上游解析口径。
- 账号创建、编辑和批量编辑共享请求头覆写数据结构；JSON 导入拒绝无效对象，复制只输出具名项，OAuth 建号三条路径在消耗授权凭据前完成自定义上游配置校验。
- 订阅计划新增 `currency`，迁移、Ent schema、支付配置、DTO 和前端展示保持一致；管理员充值返佣开关进入设置审计与保存合同。

冲突与兼容：
- 唯一内容冲突位于 `CreateAccountModal.vue` import 区；同时保留 dev-zz 模型目录推荐 / 搜索与上游请求头编辑器。
- 新增 migration `177_add_subscription_plan_currency.sql` 与既有企业成员 `177` 迁移按完整文件名并存，不修改已应用迁移；版本继续保留 `1.7.2`。
- 为上游 locale 编译测试补齐直接开发依赖 `@intlify/message-compiler@9.14.5`；新增账号控件继续采用 stone / emerald / rose 视觉，并补充 switch 无障碍状态。

验证：
- 后端目标包测试、全包编译、完整 tagged unit 闸门。
- 前端 typecheck、ESLint、全量 Vitest、生产构建和 docs-site 构建。
- `git diff --check`、冲突标记与未合并索引扫描。

## 2026-07-15 - 上游 main 增量同步：Grok OAuth 池、Chat bridge、账号复制与 Key ID

实现：
- 合并 `origin/main@d515c3045` 的 52 个提交，覆盖 Grok OAuth refresh pool / reconcile / Free cache、OpenAI 首输出与 WebSocket 首消息超时、Codex / Responses 工具兼容、调度 outbox latch、XAI URL 校验、账号复制、根路径 models 和 Key ID 列。
- 管理员账号复制采用 `Idempotency-Key`、管理员作用域 operation key 与原子“账号 + 有序分组”写入；只允许静态凭据类型，复制后清理运行态、配额投影和远端绑定证据并默认不可调度。
- Messages Chat fallback 保留 dev-zz 请求侧 Responses 工具 / 策略链，响应侧吸收上游直接 Chat → Anthropic 转换；畸形 additional_tools 继续 fail closed，hosted/server-only 工具不得静默丢弃。
- 直接 Chat → Anthropic 流式状态机与 Responses 桥共享工具参数资源边界：单调用最多 16 MiB、单响应最多 32 MiB；超限立即输出 Anthropic error 事件并停止读取，不得伪装为正常完成。
- `/v1/models` 与 `/models` 复用同一个成员分组编排 handler；Key ID 作为可选列接入现有列偏好版本 3，默认隐藏且遵循 stone 视觉。

冲突与兼容：
- 7 个内容冲突逐项合并，没有数据库迁移冲突；`VERSION` 保留 `1.7.2`。
- 保留企业成员分组 / 预算 / fallback、Key 批量选择与标签、账号成本池测试桩；吸收账号复制、ID 列和上游工具解析测试。
- 合并后补齐两个 `NewAccountHandler` 测试调用的新依赖占位，避免只在全包编译或 CI 中暴露构造函数错位。

验证：
- `mise x -C backend -- go test ./... -run '^$' -count=1`
- `mise x -C backend -- go test ./internal/pkg/apicompat ./internal/server/routes ./internal/service`
- `make -C backend test-unit`
- `mise x -C backend -- golangci-lint run --timeout=30m`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run`（192 文件、1288 测试通过）
- `pnpm --dir frontend build`
- `pnpm --dir docs-site docs:build`
- `git diff --check`、冲突标记扫描。

## 2026-07-13 - 管理端使用记录稳定渲染

范围：
- 前端：管理端使用记录表、公共分页器、分页容量策略和历史用量字段格式化。
- 测试与文档：`DataTable` 自然模式、`Pagination` 调用方选项、使用记录渲染契约、1000 条全局配置收敛测试，以及 changelog / 组件说明。

决策：
- 使用记录已经由服务端分页，不再叠加浏览器虚拟行；页面只保留 `DataTable` 一个横向滚动所有者，sticky 用户列与普通列共享同一批真实 `<tr>`。
- 使用记录页容量固定为 10/20/50/100，模块上限 100；该模块的选择不写入共享表格偏好，其他固定高度大数据列表继续保留虚拟化能力。
- `Pagination.pageSizeOptions` 由调用方传入时必须真实生效，新增 `persistPageSize` 显式控制共享偏好写入，默认行为保持兼容。
- 用量展示先将 Token、费用、端点和模型映射字段收敛为可渲染值；单条历史脏数据不能让后续单元格或整行停止渲染。

验证：
- 前端完整 176 个 Vitest 文件 / 1111 项测试、ESLint、typecheck 和生产构建通过。
- `git diff --check` 通过；未修改后端、API、数据库迁移或依赖。

## 2026-07-12 - 企业成员完整目标架构

范围：
- 文档与长期决策：ADR-0003、企业成员设计、企业用量分析、旧 Key 成员方案退役说明、dev-zz 首页/侧边栏/分支策略/变更地图。
- 后端与迁移：企业账号生命周期、成员实体/分组/Key、ActiveGroup、预算预留/账本/恢复/对账、CSV/XLSX 导入、Grok 异步视频任务路由身份、append-only 操作审计与企业成员 Ops 指标。
- 前端：企业成员控制台、导航、成员/分组/Key/预算/用量/导入、全企业与单成员审计视图，以及管理员企业能力停用/恢复开关。

决策：
- `users.role` 保持 `admin/user`；企业能力使用独立 `users.account_type=enterprise`，避免把产品类型混入授权角色。
- 企业成员是不可登录的稳定主体，聚合多把成员 Key、有序分组、成员预算和用量证据；普通 Key 批量/标签/analytics 继续保留。
- 成员 Key 使用请求级 `ActiveGroup`，由统一 orchestrator 在协议 handler 之前完成入口/模型解析、候选资格、跨平台分派和受控 fallback。
- 成员月预算使用持久化 reservation、不可变预算账本、幂等结算、恢复与对账；请求 usage、迁移开账和人工调整分开记录。
- 导入以 member code 为稳定键，支持一成员多 Key、多分组、CSV 与受限 XLSX；服务器保存权威 preview，commit 在事务内重新校验并防重复。

新增稳定性修复：
- 已产生成员事实的企业账号禁止破坏性降级；管理员改用 `enterprise_enabled` 停用/恢复能力，并立即失效认证缓存。
- 不限成员预算不创建 reservation，但成功请求仍幂等写入预算账本；Batch image 不再把不限预算误判为预算耗尽。
- 后台 reconciliation 修复 usage/reservation/ledger 证据关联，并从账本和在途 reservation 重建月度投影。
- migration 176 持久化 Grok 视频上游任务 ID 对应的 owner/member/Key/group/account；查询只使用原任务 account。
- migration 177 使用同事务数据库触发器记录账号能力、成员、分组、成员 Key、非用量预算账本和导入任务变更；审计表禁止 update/delete，载荷按字段白名单生成，不复制明文 Key、导入 preview/result 或上传原文件。
- 企业 owner 可读取 owner-scoped 全局审计和 member-scoped 审计；前端在现有控制台的全局审计弹窗和成员预算详情内展示，不新增脱离 AppLayout 的页面。
- 管理员 Ops 新增无 tenant/member 高基数标签的进程内快照，覆盖成员鉴权、候选/跨组路由、预算预留/结算/释放/恢复/对账与导入解析/回滚。
- 导入 commit 改为持久化 `queued/processing/completed/failed` job；多实例 worker 用 `SKIP LOCKED` 和超时租约领取，进程退出进入统一 Stop 生命周期。前端轮询 job，失败可下载无敏感字段的 CSV 报告。
- 导入租约增加 `lock_owner` fencing：接管后旧 worker 不能再提交或标记失败；缺失 `locked_at` 的异常 processing 记录也可恢复领取。真实 PostgreSQL 并发测试覆盖唯一领取、超时接管、迟到写入隔离和无时间戳恢复。
- 导入 worker 将领取 timeout 与处理 timeout 分离，默认处理窗口提升为 15 分钟，并按租约三分之一间隔续租；短暂数据库错误继续重试，确认失租或续租错误持续超过租约期限时取消当前处理。Ops 快照新增续租成功、续租错误和失租三个无租户标签计数。
- 5000 行 CSV 解析边界和 benchmark 已固定；真实 PostgreSQL 可在约 7.9 秒内事务创建 5000 成员并生成 5000 条 append-only 审计。本轮容量测试同时修复了导入校验误引用不存在的 `deleted_api_keys`：软删除 Key 本就在 `api_keys` 原表，继续作为不可复用的历史凭证参与冲突检查。
- 进程级故障注入覆盖 Redis Stop/Start 和 PostgreSQL `pg_terminate_backend`：远端认证实例在 Redis 恢复后重新建立 Pub/Sub 订阅，恢复后的单次广播清除重启前旧 L1；导入事务在成员 INSERT 期间被终止后零部分写入，原 Job 可在租约过期后由新 worker 接管。
- worker 生命周期测试证明 Stop 会取消活跃 commit/heartbeat 并等待 goroutine 退出；处理 timeout 后使用新的 failure context 写状态，不再复用已过期 context。
- 两个独立 APIKeyService 实例以各自 L1、共享 Redis 和真实 PostgreSQL 状态验证用户级认证缓存失效：发布实例删除 L2 并广播后，订阅实例会清除旧 L1 并重新加载当前用户状态。
- 导入结果 Key 以应用加密密文短暂保存，owner 使用 preview token 一次性消费后原子清除；失败任务立即清除 preview Key 密文，未消费成功密文 24 小时后由 cleanup 清除。
- 历史普通 Key 新增显式成员迁移：UI 预览原分组的路由影响，后端用成员/Key 行锁、expected version 和提交时分组授权复检保证原子性；原分组只会追加或复用，不会静默丢失，迁移审计不包含 Key 明文。
- 成员预算详情新增独立请求记录分页投影，只返回 Key 名称、对客模型、公开分组、token、耗时和对客费用，不复用含上游账号/渠道字段的管理员 DTO。
- 企业成员控制台 264 组静态文案和 12 组动态插值全部迁移到独立 zh/en locale namespace；新增语言键对称、页面引用完整性和“禁止恢复页面内双语 helper”的回归测试，并显式合并原有导航 title/description，避免 namespace 覆盖。
- 成员主体从低密度卡片墙修正为桌面数据表和窄屏连续紧凑行：金额不再省略，成员名、稳定编号、Key 数、分组数、有序路由、更新时间与全部操作可横向比较；名称/编号和 Key/分组分别拆为独立列，桌面行压缩为 `py-2` 并同步收紧状态、预算条与路由胶囊，操作组固定单行。表头、桌面行和移动行的选择控件统一复用二开 `tableSelectionCheckboxClasses`，提供 emerald 勾选、半选横线、键盘和读屏语义，不再显示浏览器原生白色 checkbox。布局契约测试禁止恢复旧卡片网格、指标纵向堆叠、原生选择框或操作按钮折行。
- 成员筛选栏的状态、预算风险和排序从浏览器原生 select 迁移到二开共享 `Select.vue`，统一暗色触发器、emerald 打开态、旋转箭头、Teleport 浮层、选中勾号和键盘导航；筛选值与原有查询逻辑保持不变。
- “查看归档”眼睛按钮改为共享 Select 的成员范围筛选（仅当前成员 / 包含已归档）；归档状态选项只在范围允许时出现，切回当前成员会自动清除已归档状态并重新加载，避免两个控件组合出无结果的矛盾状态。

边界：
- 这是完整最终状态的设计合同，不是 MVP；实现可以按依赖顺序拆分，但完成口径不缩减。
- 平台管理员仍按企业 user 查看总量，默认不读取成员明细；企业 owner 不接触 account/channel/provider/account_cost 等管理员字段。
- 已产生成员事实后不允许直接降回 individual；成员、Key、预算和 usage 有历史事实时优先归档，不硬删除。
- 本记录不表示功能已经上线；浏览器 E2E、指标跨实例聚合、包含分组/Key/开账的混合负载容量测试，以及网络分区/长时间数据库不可用等持续性故障验证仍未完成。

验证：
- 后端完整 `go test ./...`，包含审计仓储/迁移、Ops 指标、慢导入队列规范化、worker 生命周期、handler、route、middleware 与 Wire cleanup 覆盖。
- 企业成员 migration 175–178 新增真实 schema integration 合同；本机 Colima 上以 PostgreSQL 18.1、Redis 8.4 Testcontainers 验证复合外键、约束、索引、审计 trigger、导入多 worker fencing/心跳续租、5000 成员事务、Redis 重启订阅恢复、PostgreSQL 中断回滚和跨实例认证缓存失效全部通过。
- 前端完整 ESLint、typecheck、166 个 Vitest 文件/1044 项测试和生产构建。
- `git diff --check`、文档交叉引用与 VitePress build。

## 2026-07-11 - Tool Search 状态机与 Chat fallback 能力边界修复

范围：
- 后端：Responses 工具注册表、Responses → Chat request/history/response/stream bridge、OpenAI Responses force-Chat fallback、账户换号和 scheduler account extra。
- 前端类型：补齐三项高级 Chat fallback 账号标记，不新增普通用户可见入口。
- 文档：changelog、patches、merge log、配置索引和验证矩阵。

改动：
- `BuildResponsesToolRegistry` 单次解析请求工具载体，并按输入顺序 replay 顶层 `tools`、`additional_tools` 和 `tool_search_output.tools`；当前可调用集合与回程 identity map 从同一份 immutable registry 派生，不再由 service 和 converter 各自解析。
- 顶层 function/custom 的 `defer_loading: true` 和 namespace 内 deferred 子工具在加载前隐藏；`additional_tools` 与 client `tool_search_output` 明确加入的工具在后续当前轮可调用。
- type-only `tool_search` 按官方 hosted 默认处理，Chat-only 账户不能承载时返回 capability mismatch；显式 `execution: "client"` 才映射为代理。旧客户端若确实省略 execution，只能通过 `openai_chat_implicit_client_tool_search_enabled=true` 明确兼容。
- `tool_search_output` 动态加载的顶层 function 使用 function 名作为 Chat 名，同时记录 Responses 回程的 `namespace=name`；输入历史、非流式、流式 added/done/completed 使用同一映射。
- 重复 `tool_search_output.call_id` 视为同一历史输出的更新版本：Chat 历史只保留首个 tool result，后续副本更新最终 callable set；历史 function call 使用所在 input item 之前的 identity 状态，不再被最终 map 反向污染。
- 普通顶层 function、动态直连 function 与 namespace child 的 Responses identity 全部参与双向冲突检查；同一 Chat 名无法区分两个 Responses identity 时触发 capability 换号。流式 added/done/completed 复用同一 item ID。
- hosted/server-only 工具不再静默丢弃；非法 `execution` 在账号调度前返回 `invalid_request_error`，合法但 Chat 无法保真的 hosted/identity/allowed_tools/grammar 场景才是 capability mismatch。若 capability 与真实 upstream failover 交错，最终优先返回已经发生的 upstream 失败。
- 工具定义冲突比较使用 `json.Decoder.UseNumber`；原始 body 在账号调度和完整工具树解码前拒绝重复 JSON key，并把顶层工具、动态载体与 `tool_choice.allowed_tools.tools` 统一纳入数量、单定义字节数、总字节数和 namespace 深度预检，registry 内仍保留第二层防御。`allowed_tools` converter 改为流式解码轻量引用，不再构造完整 `ResponsesTool` 树。
- 历史 function call 的 Chat 名在 Registry 按 input 顺序 replay 时一次解析并缓存，转换阶段为 O(1) 查询，不再形成“历史项 × 工具节点”的乘法扫描；Responses input 和单项 content/summary parts 各限制为最多 16384 项，关键对象、part 与嵌套 image URL 对象限制为最多 64 个字段，预检只保留安全相关字段值并继续对全部字段做有界重复键检测；reasoning/content part 转换只解码 `type`、`text`、`image_url` 等实际字段，上游 custom arguments 只读取根 `input` 字段，不再把未知字段扩张为通用 Go map。流式工具 arguments 使用 `strings.Builder` 按调用线性累积，单调用上限 16 MiB、单响应总上限 32 MiB；超限不生成不完整的 done/completed，Responses 回退发送稳定 `response.failed`，Anthropic Messages 回退发送标准 `event: error`，两者都立即停止读取上游流。
- fallback 内仍可能发生的 call ID、定义冲突、tool choice 等客户端校验使用 typed `OpenAIClientRequestError`；handler 在账号健康上报前终止，不把未访问上游的 400 写入 error-rate EWMA。
- `allowed_tools`、隐式 client tool search 和有损 custom grammar wrapper 均为账号级 opt-in；能力不匹配不提前写 HTTP 400，而是返回 `AccountCapabilityMismatchError`，Responses handler 排除当前账号继续调度。只有全部尝试都在访问上游前能力不匹配时才返回稳定的 `unsupported_feature`。

边界：
- 普通 custom/freeform 工具仍可用 `input` wrapper 走旧 Chat 兼容路径；带 grammar/format 的 custom 默认拒绝有损转换，只有显式账号开关允许旧行为。
- `additional_tools` 的当前可调用集合按历史顺序 replay；Chat 顶层 tools 无法复刻 Responses 的 prompt-cache 插入位置，文档不再把缓存布局称为完全可逆。
- Fast / Flex、billing/upstream model、usage、endpoint、Anthropic Messages fallback 和用户/admin 字段隔离不变。
- 只更新 `dev-zz-develop`，不提升 `dev-zz`、不打 tag、不发布。

验证：
- hosted/client execution、deferred-before-load、namespace 混合加载、动态顶层 function 非流/流/历史回程、重复 call ID 替换、allowed_tools capability、custom grammar capability、超大 JSON number、重复 key、历史 identity replay cache、对象字段上限、input/content part 数量上限、嵌套 image URL、最小字段 part 解码、大 unknown-field custom arguments、流式单调用/总参数上限和转换错误停止读取、allowed-tools 总预算与资源上限均有 Go 回归测试。
- backend `make test-unit`、`go test ./... -count=1`、`golangci-lint run --timeout=30m` 与 repository integration test 编译通过。
- frontend `pnpm run lint:check`、`pnpm run typecheck` 通过；docs-site `pnpm run docs:build` 通过（仅保留既有的大 chunk warning）。

## 2026-07-10 - Codex MCP、custom 与 tool_search Chat bridge 增量同步

范围：
- 上游：`origin/main` `e316ebf5` 增量合并到 `dev-zz-develop`，merge base 为 `07fac347`。
- 后端：Responses ↔ Chat Completions bridge、Responses stream wire、OpenAI Responses / Messages chat-only fallback。
- 文档：changelog、patches 和 merge log。

改动：
- custom / freeform 工具降级为带 `input` 字符串 schema 的 function 工具；历史调用、非流式响应和流式事件回程还原为 Responses `custom_tool_call`，使 Codex `exec` 等工具可在 chat-only 上游工作。
- 显式 `execution=client` 的 `tool_search` 使用同名 function 代理，保留客户端自定义 description / schema，回程恢复 `tool_search_call` 与 `execution=client`；2026-07-11 follow-up 明确 type-only 为 hosted，不能由 chat-only 账户静默改写。
- `tool_search_output.tools` 与 Responses Lite `additional_tools.tools` 进入后续当前可调用集合；2026-07-11 follow-up 用来源感知 registry 保持 deferred-before-load 和动态顶层 function identity。Chat 顶层 tools 不承诺复刻 Responses 的 prompt-cache 插入位置。
- namespace 子工具摊平后转发，使用稳定的长度限制/哈希命名并拒绝不可消歧的碰撞；回程恢复 namespace 与原始子工具名，修复 MCP 工具 unsupported call。
- custom / function 同名、代理名与摊平名碰撞均显式拒绝；同类型同名工具按完整原始定义比较，JSON key 顺序不同但语义等价时去重，schema、custom grammar `format` 或未来未知字段不同时拒绝；namespace arguments delta、added 和 done 使用一致的裸子工具名。
- `tool_choice` 的 function / simple custom、显式 client tool_search 和单子工具 namespace 在可保真时转为 Chat 形态；多子工具 namespace 只有账号声明支持时才转为 Chat `allowed_tools`。托管工具、不存在工具、源类型错配和无能力账号显式失败或换号。
- custom input、namespace function 和 tool_search 的非流式/流式 wire 字段与生命周期由集中测试覆盖。

边界：
- 本轮 10 个提交、8 个文件均为后端兼容性改动；无迁移、依赖、前端、部署、workflow 或版本变化。
- Responses fallback 保留 dev-zz 的 Fast / Flex、billing/upstream model、usage、endpoint 和故障转移链路；Anthropic Messages fallback 不启用 Responses 专属工具回程映射。
- 继续保留 dev-zz `1.5.1`，只更新 `dev-zz-develop`，不提升 `dev-zz`、不打 tag、不发布。

验证：
- `internal/pkg/apicompat` custom、官方 tool search 第二轮、Responses Lite additional tools、namespace、allowed tool choice、碰撞边界、历史消息、非流式和流式定向测试。
- backend unit / 全包测试、golangci-lint、docs-site 构建、补丁检查和冲突标记扫描纳入本轮分支验证。

未验证：
- 浏览器人工 smoke。
- 本机后端启动仍受既有开发数据库的迁移 174 checksum 历史不一致阻断；本轮没有修改迁移或数据库。

## 2026-07-10 - 上游 ops writer 与 cache creation usage 增量同步

范围：
- 上游：`origin/main` `07fac347` 增量合并到 `dev-zz-develop`，merge base 为 `deff3123`。
- 后端：ops error capture writer、Responses / Anthropic 双向转换及流式 usage 状态。
- 文档：changelog、patches 和 merge log。

改动：
- 为已释放的 `opsCaptureWriter` 补齐 Gin `ResponseWriter` 全部委托方法的 nil 安全行为；合并复审进一步修正对象所有权：ops middleware 无条件恢复原 writer，下游 wrapper 持有时不把对象放回池，避免晚到访问读到状态 `0` 或串到另一请求。
- 已释放 writer 的非空写入返回 `io.ErrClosedPipe`，遵守 `io.Writer` 的短写错误契约，不再静默丢弃数据。
- Responses → Anthropic 转换保留缓存写入 token，并从总输入中扣除 cache read / creation，避免把缓存 token 重复计入普通输入。
- Anthropic → Responses 转换把 cache read / creation 加回 Responses 总输入，同时显式输出 `cache_creation_input_tokens`；非流式、流式完成事件和异常结束兜底使用同一语义。

边界：
- 唯一冲突是 `backend/cmd/server/VERSION`；继续保留 dev-zz `1.5.1`，不采用上游 `0.1.151`。
- 本轮没有迁移、依赖、前端、部署或 workflow 变化，不改变供应商成本、账号归档、模型自检、Fast / Flex 设置原子写入和普通用户字段隔离。
- 仅更新 `dev-zz-develop`，不提升 `dev-zz`、不打 tag、不发布。

验证：
- ops capture writer 释放安全、真实 compact keepalive 嵌套、pool 隔离、race 和 Responses / Anthropic cache creation 定向测试通过。
- 后端 unit / 全包测试、golangci-lint、前端 lint / typecheck / 全量测试 / 构建和 docs-site 构建纳入本轮分支验证。

未验证：
- 浏览器人工 smoke。
- 本机 Docker / testcontainers 运行时集成测试；该项由 GitHub Actions integration job 验证。

## 2026-07-10 - Fast/Flex 设置原子保存与合并后复审加固

范围：
- 后端：管理员设置写入边界、Fast / Flex 策略校验与审计、Codex 家族身份规范化。
- 前端：Fast / Flex 用户 ID 校验与 zh/en i18n 命名空间。
- 文档：策略优先级、WebSocket 快照和合并验证记录。

改动：
- 无效 Fast / Flex 用户规则在任何设置落库前返回结构化 400；普通设置、认证来源默认值和 Fast / Flex 策略改为同一次批量写入，消除失败响应下的静默部分保存。
- 策略变更进入管理员设置审计；前端提前拒绝 0、负数、非整数和单条规则内重复用户 ID。
- 修正 Fast / Flex 用户 ID 文案命名空间，并用组件测试和 locale 契约测试覆盖实际读取路径。
- `Codex ` 家族身份即使客户端传入大小写变体，也会规范化为上游所需前缀；用户专属规则的白名单 fallback 终止语义由回归测试锁定。

边界：
- 支付配置仍由通用设置接口中的独立服务更新，不属于普通设置 / 认证默认值 / Fast / Flex 策略的原子批量写入。
- WebSocket 会话继续使用建连时策略快照；运行中的连接需要重连才读取新设置。
- 仅加固 `dev-zz-develop`，不提升 `dev-zz`、不打 tag、不发布。

验证：
- 管理员设置原子写入、审计、Fast / Flex fallback、Codex identity 定向 Go 测试通过。
- SettingsView Fast / Flex 保存与 locale 命名空间 Vitest 通过。

未验证：
- 浏览器人工 smoke。
- 本机 Docker / testcontainers 运行时集成测试；继续由 GitHub Actions integration job 验证。

## 2026-07-10 - 上游 Fast/Flex 用户范围与 Codex 身份修复增量同步

范围：
- 上游：`origin/main` `deff3123` 增量合并到 `dev-zz-develop`，merge base 为 `6dd3274a`。
- 后端：API Key 认证上下文、OpenAI Fast / Flex 策略、Codex OAuth 身份头、Grok reasoning usage。
- 前端：管理员 Fast / Flex 规则新增用户 ID 范围配置和中英文提示。
- 文档：管理员设置 API 语义、changelog、patches 和 merge log。

改动：
- Fast / Flex 规则新增 `user_ids`：非空时仅匹配指定 API Key owner，用户专属规则整体优先于全局规则，每组继续按配置顺序首条命中。
- 用户身份只来自 API Key 认证中间件写入的可信请求 context；HTTP 与 WebSocket 转发共用该语义，不接受客户端请求体中的用户标识替代。
- 管理端规则编辑支持添加 / 删除用户 ID；服务端拒绝非正整数和单条规则内重复 ID。
- Codex OAuth 转发根据最终 User-Agent 生成配套 `originator`，处理客户端 override 后的尾部真实身份，并把不合法或不可识别身份回退到默认官方 Codex CLI。
- Grok Responses 使用兼容提取逻辑保留 `reasoning_effort`，覆盖标准字段和已支持的模型兼容路径。

边界：
- 本轮真实合并无冲突；仍按拆分热点复核并保留 dev-zz 管理员 7 项运行设置、管理员用量证据 guard、供应商成本、账号归档、模型自检和用户/admin DTO 隔离。
- compatibility messages bridge 继续保持无 `originator` 请求，不被新的 Codex 身份收口改写。
- 继续保留 dev-zz `1.5.1`；仅更新 `dev-zz-develop`，不提升 `dev-zz`、不打 tag、不发布。
- 合并提交：`838e4094`。

验证：
- Fast / Flex 用户匹配、API Key auth context、Codex identity、Grok reasoning 与 dev-zz 管理员设置定向 Go 测试通过。
- 后端 `make test-unit`、不带 build tag 的 `go test ./... -count=1`、`golangci-lint`（0 issues）和 repository integration 测试二进制编译通过。
- 前端 ESLint / typecheck / 93 条关键测试、完整 Vitest（163 个文件 / 1030 个用例）和生产构建通过。
- docs-site VitePress 构建、`git diff --check` 和冲突标记扫描通过；GitHub Actions 在推送最终 head 后检查，运行结果记录在本轮交付报告。

未验证：
- 浏览器人工 smoke。
- 本机 Docker / testcontainers 运行时集成测试；该项由 GitHub Actions integration job 验证。

## 2026-07-10 - 上游 GPT-5.6、排行与结构拆分同步

范围：
- 上游：`origin/main` `6dd3274a` 合并到 `dev-zz-develop`。
- 后端：GPT-5.6 / OpenAI gateway、API Key、admin、settings、usage log、Grok 视频计费与迁移。
- 前端：管理端用量排行、账号 / Key 列表、版本回退、i18n 模块拆分。
- 文档：`changelog.md`、`patches.md`、`maintenance/merge-log.md`。

改动：
- 吸收 GPT-5.6 reasoning effort、cache write token、usage 和计费口径修复，以及 compact、WebSocket、messages fallback 的上游兼容更新。
- API Key 增加最近使用 IP，账号和 Key 列表支持按当前并发排序；管理端用量页增加用户 Token 排行。
- 版本提示增加管理员回退能力，但 release API 与跳转链接继续固定到 fork `thornboo/sub2api`。
- 接受上游 Go 大文件和 i18n 的按职责拆分；dev-zz 功能以小型补充文件和 locale overlay 保留，减少后续 merge 冲突面。
- 用量日志新增上游视频分辨率 / 时长字段时，继续完整保存 dev-zz 调度诊断 `schedule_meta`；插入、批处理和扫描列序保持一致。
- 用量日志关联 hydration 继续按管理员 evidence context 受控解析已删除 Key 和已归档账号；普通 / 用户侧查询不穿透软删除边界。
- 模型自检继续跳过已有 probe guard 覆盖的 Gateway / Antigravity retry、限流写入和账号惩罚分支；未覆盖的 Antigravity 既存副作用另列专项审计。分组 / 用户倍率变更继续按设置停用受影响 Key。
- 供应商 Modal、成本事实、账号归档、普通用户字段隔离和 stone / neutral / emerald UI 保持不变。

边界：
- 合并提交 `a1b8b657` 已推送到 `origin/dev-zz-develop`；复审修复和远端 CI 全绿前不提升 `dev-zz`、不打 tag、不发布。
- 不采用上游版本号，继续保留 dev-zz `1.5.1`。
- 不把管理员排行、供应商成本、上游账号或调度诊断字段暴露到普通用户接口。

验证：
- 后端冲突包编译、全仓 `go test ./...`、带 `unit` 标签的完整 service 测试与 `golangci-lint`。
- 前端 typecheck、lint、生产构建和完整 Vitest（163 个测试文件、1026 个用例）。
- docs-site VitePress 构建、冲突标记扫描和 `git diff --check`。

未验证：
- 浏览器人工 smoke。
- Docker / testcontainers 集成测试。

## 2026-07-10 - 供应商默认结算与充值录入简化

范围：
- 后端：迁移 `174_upstream_cost_pool_defaults.sql`、供应商 create/update、默认资金池创建与成本池 DTO。
- 前端：账号页顶部操作、供应商列表、供应商新增 / 编辑 Modal、充值记录 Modal、zh/en i18n。
- 测试：供应商默认配置 service / migration 回归，供应商列表、供应商 Modal、充值 Modal 和账号编辑 Vitest。
- 文档：成本池功能页、API / 迁移索引、验证矩阵、changelog / patches。

改动：
- `upstream_cost_pools` 新增 `default_effective_cny_per_usd` / `default_reference_fx_rate` / `is_default`；迁移从当前成本和参考汇率回填稳定默认值，并用数据库唯一索引固定每个供应商至多一个未归档默认池。
- `POST` / `PATCH /api/v1/admin/upstream-suppliers` 可保存默认充值成本和默认参考汇率，实际存储仍归供应商默认资金池。
- 默认配置只作为以后新增流水的输入默认值，不再写入 `current_effective_cny_per_usd`；只有真实 `current_snapshot_id` 才进入账号成本展示、排序和 `cost_first` 调度。
- 新建供应商默认池不再自动生成手工成本快照，避免“只有配置、没有成本事实”的供应商被永久挡在受限硬删除之外。
- `174` 会清理早期实现留下的配置性初始快照，但只处理精确自动备注、无来源记录且资金池从未产生充值流水的行；真实快照保持不变。
- 供应商新增 / 编辑统一使用 `BaseDialog` Modal；页面顶部主操作按标签页切换为“添加账号”或“添加供应商”，供应商卡片内部移除重复刷新 / 新增入口。
- 供应商视图隐藏账号搜索 / 筛选、自动刷新和账号更多操作；顶部手动刷新继续按当前视图刷新供应商和资金池。
- 普通充值默认按供应商配置自动计算到账额度和参考汇率，只在“本次与默认不同”时展开覆盖字段；赠送只增加额度，不定义独立单位成本，赠送和调整都不刷新当前成本快照。
- 供应商创建改为严格冲突语义，同名提交不会复用或覆盖已有配置；备注可通过显式空字符串清除。
- 系统供应商和无真实快照的配置不再进入账号成本 DTO / 排序；充值成本变化会主动刷新绑定账号的调度快照。
- 归档供应商的现有绑定在账号编辑中以禁用历史项保留，所有新绑定入口拒绝已归档供应商；硬删除要求供应商从未产生任何账号绑定历史。
- integration 用例不再假设未绑定账号首次充值会自动创建“未归类供应商”，统一先建立真实供应商绑定。

边界：
- 默认换算是后续流水的输入默认值，不是当前或历史成本事实；每条记录仍固化本次实际支付、实际到账和本次参考汇率。
- 不把默认成本字段移动到供应商表；多钱包场景仍可在不同资金池维护各自默认值。
- 不改变普通用户扣费、普通用户 DTO、账号分组倍率或成本感知调度公式。

验证：
- `go test ./internal/service -run 'Test(ApplyUpstreamSupplierUpdate|EnsureUpstreamSupplierDeletable|UpdateDefaultUpstreamCostPoolConfig|NormalizeUpstreamCostPoolDefault)' -count=1`
- `go test ./migrations -run 'TestMigration(166|172|173|174)' -count=1`
- `go test ./internal/server ./internal/service`
- `go test ./... -count=1`
- `go test -tags=integration ./internal/service -run TestUpstream -count=1`
- `go test -tags=integration ./internal/repository -run 'TestAccountRepoSuite/TestListWithFilters_(SortByUpstreamEffectiveDiscount|UpstreamDiscountRequiresRealNonSystemSnapshot)$' -count=1`
- `go build ./...`
- `go vet ./internal/service ./internal/server`
- `pnpm exec vitest run src/components/admin/account/__tests__/UpstreamCostComparison.spec.ts src/components/admin/account/__tests__/UpstreamSupplierModal.spec.ts src/components/admin/account/__tests__/UpstreamRechargeRecordsModal.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts`
- `pnpm run test:run`（154 个测试文件、969 个测试全部通过）
- `pnpm run typecheck`
- `pnpm exec eslint`（目标改动文件）
- `pnpm run build`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

未验证：
- 浏览器人工 smoke。

## 2026-07-09 - 供应商编辑 / 删除与账号编辑边界收敛

范围：
- 后端：迁移 `172_upstream_suppliers_system_flag.sql` / `173_upstream_account_binding_group_name.sql`、`upstream_cost_pool_service`、管理端 supplier handler、管理端路由。
- 前端：管理端供应商标签页、账号编辑弹窗供应商绑定边界、管理端账号 API、zh/en i18n。
- 测试：供应商 service 单测、`UpstreamCostComparison` 和 `EditAccountModal` Vitest。
- 文档：`docs-site/dev-zz/features/upstream-cost-pools-and-ledger.md`、`changelog.md`、`patches.md`、`reference/api-surface.md`、`testing/verification-matrix.md`。

改动：
- 新增 `PATCH /api/v1/admin/upstream-suppliers/:supplier_id`，支持供应商改名、备注和 `active` / `archived` 状态切换。
- 新增 `DELETE /api/v1/admin/upstream-suppliers/:supplier_id`，只允许硬删除完全干净的供应商。
- 新增 `upstream_suppliers.is_system`，API 下发 `is_system`；后端 update/delete 和前端按钮显隐都读该稳定标志，不再用供应商名称字面量判断旧迁移系统供应商。
- `is_system=true` 的旧迁移系统供应商退出正常业务路径：供应商 / 资金池列表、账号绑定候选、active 绑定查询和按账号新增充值记录都不再把它作为兜底来源；未绑定真实供应商的账号新增充值记录会提示先绑定供应商。
- 2026-07-10 复审后，删除前置校验收紧为无任何账号绑定历史；active 绑定和已归档绑定分别返回明确冲突，曾被使用的供应商改用归档。
- 删除事务不再为硬删除清理历史绑定行，避免破坏供应商归属审计链。
- 删除仍会拦截非默认资金池、任意充值记录和任意成本快照；已有成本事实的供应商应归档保留历史。
- 前端供应商列表新增编辑、归档 / 恢复、删除按钮；删除使用二次确认，并优先提示 active 绑定数量；归档仍有 active 绑定的供应商时会先确认“存量绑定继续生效，新绑定候选隐藏”。
- 账号编辑弹窗仍不挂回旧 `UpstreamCostSettings`，不编辑真实充值比例、参考汇率或资金池基础成本。
- 账号编辑弹窗保留供应商归属选择，并新增这把上游 key 的供应商侧分组名与分组倍率；综合折扣按 `current_effective_cny_per_usd / reference_fx_rate * upstream_group_multiplier` 展示。
- `default_multiplier` 继续作为兼容存储列承载上游分组倍率；`model_family_multipliers` 不进入本轮账号编辑主流程。
- 账号列表成本上下文列把「充值/汇率」改为「充值比例」，只展示 `current_effective_cny_per_usd` 换算出的 CNY:USD 额度比例；参考汇率留在供应商 / 资金池详情查看。
- 既有 `PATCH /api/v1/admin/accounts/:id/upstream-cost-profile` 保留为兼容接口，不作为新版账号编辑成本入口。

边界：
- 新增幂等迁移 `172_upstream_suppliers_system_flag.sql` 和 `173_upstream_account_binding_group_name.sql`；不改普通用户侧表和 DTO。
- 不改变普通用户扣费、调度逻辑、资金池成本快照算法或用户侧 DTO。
- 不把账号 `extra` 成本参数继续扩成新版主流程；历史字段迁移到资金池 / 绑定 / 快照仍是后续专项。
- `is_system=true` 的旧迁移系统供应商不进入正常列表；若通过历史 ID 直接请求，禁止编辑、归档和删除。

验证：
- `go test ./internal/service -run 'Test(ApplyUpstreamSupplierUpdate|EnsureUpstreamSupplierDeletable)'`
- `go test ./migrations -run 'TestMigration(166|172|173)'`
- `go test ./internal/server ./internal/service`
- `go build ./...`
- `go vet ./internal/service ./internal/server`
- `pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts src/components/admin/account/__tests__/UpstreamCostComparison.spec.ts`
- `pnpm run typecheck`
- `pnpm exec eslint src/components/account/EditAccountModal.vue src/components/account/__tests__/EditAccountModal.spec.ts src/components/admin/account/UpstreamCostComparison.vue src/components/admin/account/__tests__/UpstreamCostComparison.spec.ts src/views/admin/AccountsView.vue src/api/admin/accounts.ts src/i18n/locales/zh.ts src/i18n/locales/en.ts`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

未验证：
- `staticcheck ./...`：当前 shell 找不到 `staticcheck` 可执行文件，`/Users/thornboo/go/bin/staticcheck` 和 `/Users/thornboo/.local/share/go/bin/staticcheck` 也不存在。
- 浏览器人工 smoke。

## 2026-07-08 - v1.4.10 上游 main 同步发布

范围：
- 上游同步：`origin/main` `e8e23425` 合并到 `dev-zz-develop`，并提升到正式 `dev-zz`。
- 后端：批量生图 ent/schema/migrations/repository/service/handler、网关拆分、OpenAI / Anthropic / Grok fallback 与 usage 记录。
- 前端：批量生图用户入口、管理端分组 / 套餐 / 设置配置、dashboard quick action、sidebar / router / i18n。
- 文档：`docs-site/dev-zz/changelog.md`、`patches.md`、`maintenance/merge-log.md`。
- 发布：`backend/cmd/server/VERSION` 更新为 `1.4.10`，用于 `v1.4.10` release。

改动：
- 吸收上游批量生图 MVP：任务、队列、冻结余额、结算、下载、清理、worker runtime、Gemini / Vertex provider、分组 gate、pricing snapshot 和用户侧指南页。
- 接受上游网关拆分结构，把 Anthropic passthrough、Bedrock、OpenAI passthrough、OpenAI scheduling、usage 和 CC fallback 管线拆到独立文件。
- 合入 OpenAI Responses / Chat Completions 共享 fallback 管线，并保留 dev-zz 的 prompt cache、Claude Code todo guard、fast policy、billing / upstream model 归一化和 `UpstreamEndpoint` 记录。
- 保留 dev-zz 的 OpenAI cache-read 计费口径、ScheduleMeta、model self-check probe 不触发生产账号 retry / failover 的 guard。
- 修正 rate-limit 合并边界：5xx temp-unsched 优先于通用模型级失败；非模型级 4xx / 429 自定义 temp-unsched 兜底保留；404 / model_not_found 和 Anthropic 429 官方窗口维持专用优先级。
- `xlsx` audit exception 保留 dev-zz “仅导出、不解析用户上传 XLSX”的风险说明，并采用上游更晚的 `2026-10-06` 到期日。

边界：
- 普通用户侧仍不暴露上游账号、渠道、供应商、成本、利润或管理员字段。
- `dev-zz` 继续使用 docs-site 文档中心和 fork release / 镜像策略，不采用上游版本号。
- 前端继续保留 dev-zz stone / emerald 控制台方向，批量生图入口按当前二开视觉接入。

验证：
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
- 浏览器人工 smoke。
- 完整前端测试套件。

## 2026-07-07 - 账号管理供应商入口简化

范围：
- 前端：管理端账号页供应商标签、供应商列表新增入口、账号创建表单、账号编辑供应商绑定区。
- 后端：供应商创建默认备注文案。
- 文档：`docs-site/dev-zz/changelog.md`、`patches.md`。

改动：
- 将账号页第三个标签从「供应商成本」改为「供应商」，避免把供应商管理入口误读成单纯成本对比。
- 在供应商标签页顶部新增「新增」入口，新增成功后刷新供应商列表；供应商页继续作为供应商级充值记录入口。
- 账号编辑弹窗只保留供应商下拉选择和绑定说明，移除这里的新建供应商表单以及高级成本 / Key 配额查询配置组件，并允许清空供应商绑定。
- 创建账号弹窗同步移除历史高级成本 / Key 配额查询配置组件，避免出现“创建时能配置、编辑时不能维护”的半入口。
- 供应商创建的默认备注从“通过账号编辑新增”调整为“通过管理端新增”，匹配新的入口位置。

边界：
- 不修改后端供应商、资金池、充值账本和成本快照逻辑。
- 不改变账号列表供应商成本列、排序口径或普通用户侧返回字段。
- 本次不迁移已有账号 `extra` 中的历史高级成本 / Key 配额查询字段；后续如要恢复余额查询，应在供应商或资金池级入口重新设计。

验证：
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `cd backend && go test ./internal/service`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

未验证：
- 浏览器人工 smoke。

## 2026-07-07 - v1.4.9 Security Scan exception follow-up

范围：
- CI：`.github/audit-exceptions.yml` 中 `xlsx` 两个 high advisory 的例外说明和到期日。
- 发布：`backend/cmd/server/VERSION` 更新为 `1.4.9`，用于 `v1.4.9` patch release。
- 文档：`docs-site/dev-zz/changelog.md`、`patches.md`、`maintenance/merge-log.md`。

改动：
- 将 `xlsx` 的 `GHSA-4r6h-8v6p-xvw6` 和 `GHSA-5pgg-2g8v-p4x9` 例外到期日从 `2026-07-06` 延长到 `2026-08-07`。
- 更新例外理由：当前代码只用 `xlsx` 生成导出文件，不调用 `xlsx.read` / `readFile` 解析用户上传的 XLSX 文件；相关功能仍通过动态 import 仅在导出时加载。
- 本次不改变前端导出行为、不引入依赖升级、不修改业务代码。

边界：
- 这不是漏洞修复，只是对现有已接受风险的有效期和说明做续期；后续仍应评估替换 `xlsx` 或迁移到可维护的表格导出库。
- `v1.4.8` release 已成功发布，但 Security Scan 因过期例外失败；`v1.4.9` 作为 CI follow-up patch supersede `v1.4.8`。

验证：
- `python tools/check_pnpm_audit_exceptions.py --audit frontend/audit.json --exceptions .github/audit-exceptions.yml`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

未验证：
- 未替换 `xlsx` 依赖。
- 浏览器人工 smoke。

## 2026-07-07 - 账号列表供应商成本列与排序

范围：
- 前端：管理端账号列表供应商成本列位置、综合折扣排序、倍率排序。
- 后端：账号列表仓储的 `upstream_effective_discount` / `upstream_multiplier` 服务端排序。
- 文档：`docs-site/dev-zz/changelog.md`、`patches.md`、`maintenance/merge-log.md`。
- 发布：`backend/cmd/server/VERSION` 更新为 `1.4.8`，用于 `v1.4.8` patch release。

改动：
- 账号列表把「供应商、综合折扣、充值/汇率、倍率」移动到「分组」列后方，保留账号基础信息和调度字段原有顺序。
- 「综合折扣」列启用服务端排序，排序值与页面展示保持一致：`current_effective_cny_per_usd / reference_fx_rate * default_multiplier`。
- 「倍率」列启用服务端排序，读取账号 active 供应商绑定的默认倍率；未绑定、供应商归档或成本未配置的账号排在排序末尾。
- 后端排序 JOIN 只读取 active 账号成本绑定、active 且未归档的资金池和 active 供应商，避免旧绑定或归档供应商影响列表排序。
- 补充账号仓储 SQL 形态单测和数据库集成测试用例，覆盖综合折扣与倍率排序口径。

边界：
- 不改变普通用户扣费。
- 不改变调度逻辑或供应商成本快照计算。
- 不把供应商、资金池、上游余额、真实成本或利润字段暴露给普通用户侧接口。

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

## 2026-07-06 - 上游成本池 Phase 1 后端兼容层

范围：
- 后端：上游供应商、资金池、账号成本绑定、成本快照和资金池账本兼容服务。
- 迁移：`backend/migrations/166_upstream_cost_pools.sql`。
- 路由：新增 `/api/v1/admin/upstream-suppliers`、`/api/v1/admin/upstream-cost-pools/*` 和 `/api/v1/admin/accounts/:id/upstream-cost-binding`。
- 文档：`docs-site/dev-zz/features/upstream-cost-pools-and-ledger.md`、接口索引、迁移索引、变更记录和侧边栏。

改动：
- 新增 `upstream_suppliers`、`upstream_cost_pools`、`upstream_account_cost_bindings`、`upstream_cost_snapshots`。
- `upstream_recharge_records` 新增 `cost_pool_id`、`source_account_id_snapshot`、`merged_from_pool_id`、作废字段和来源字段。
- 历史等价迁移为每个现有账号创建“未归类供应商”下的账号默认资金池和 active 绑定，并把旧账号充值记录回填到资金池；后续正常业务已不再使用该兜底。
- 旧账号级充值记录接口继续兼容；账号有 active 成本绑定时读取/写入对应资金池账本，并返回 `deprecated` / `cost_pool_id`。
- 新增账号成本绑定接口，替换绑定时归档旧 active 绑定，保留绑定历史。
- 新增充值记录后会生成最新成本快照并更新资金池当前基础成本。
- 2026-07-10 复审后，只有具有有效单位成本的 `recharge` 生成资金池当前成本快照；`bonus` 和 `adjustment` 都不单独刷新当前成本。
- 账号默认资金池创建改为事务内账号级 advisory lock，避免并发首次创建留下孤儿资金池。
- 供应商补 active 名称唯一索引；历史未归类供应商创建改为唯一约束驱动，后续正常业务不再自动创建或使用该兜底供应商。
- 页面设计方向修正为“供应商优先，资金池后置”：账号编辑页应支持选择 / 新建供应商，并在供应商只有一个资金池时自动绑定默认资金池；资金池选择器只在多钱包或高级运营场景展示。

边界：
- 不自动合并多个账号的共享钱包。
- 不改变普通用户扣费或用户侧返回字段。
- 不启用成本优先调度。
- 本期账本只支持 `recharge` / `bonus` / `adjustment` 三类非负金额记录；暂不实现退款、冲正、作废、供应商优先的账号编辑 UI、完整资金池管理页、余额查询迁移和 usage 上游成本证据落账。

验证：
- `gofmt -w backend/internal/service/upstream_recharge_service_test.go backend/internal/service/upstream_cost_pool_service.go backend/internal/handler/admin/upstream_cost_pool_handler.go backend/internal/server/routes/admin.go`
- `git diff --check`
- `mise x -C backend -- go test -tags unit ./internal/service -run 'Upstream(Recharge|Cost)' -count=1`
- `mise x -C backend -- go test -tags unit ./migrations -run 'Migration166|Migration165' -count=1`
- `mise x -C backend -- go test -tags unit ./internal/handler/admin ./internal/server -run 'Upstream|TestAPIContracts' -count=1`
- `mise x -C backend -- go test -tags unit ./migrations ./internal/service ./internal/handler/admin ./internal/server -count=1`
- `pnpm --dir docs-site docs:build`

未验证：
- 完整仓库级 `go test ./...`。
- 前端管理页切换主入口和浏览器人工 smoke；本阶段未实现资金池管理页。

## 2026-07-02 - 上游 main 同步到 dev-zz-develop：分组高峰倍率与订阅计费展示

范围：
- 上游同步：`origin/main` `a632cb00` 合并到 `dev-zz-develop`
- 后端：分组 schema / DTO、API Key auth cache、billing/gateway 用量记录、订阅套餐配置、可用渠道响应、相关单测
- 前端：管理端分组页、用户 Key/订阅/支付页面、可用渠道表格、分组 badge、i18n、类型定义
- 迁移：`backend/migrations/158_add_group_peak_rate_multiplier.sql`
- 文档：`docs-site/dev-zz/{changelog.md,patches.md,maintenance/merge-log.md,reference/api-surface.md,reference/configuration-and-migrations.md}`

改动：
- 订阅分组新增高峰时段倍率字段：`peak_rate_enabled`、`peak_start`、`peak_end`、`peak_rate_multiplier`。
- 管理端分组创建/编辑支持配置高峰倍率；启用时要求分组为订阅类型、时间为 `HH:MM`、`peak_end > peak_start`，不支持跨天区间。
- 计费链路在基础倍率上叠加高峰因子；token 计费和 token 模式下的图片 token 受高峰倍率影响，图片按次计费保持原图片倍率。
- API Key auth cache 和订阅套餐配置会携带高峰倍率字段，避免鉴权缓存或套餐展示使用旧倍率口径。
- 用户侧可用渠道、订阅和 Key 相关展示增加高峰倍率提示；展示范围仍限公开分组/计费提示，不暴露上游账号、渠道、内部成本或管理员运营字段。
- 解决 `openai_gateway_record_usage_test.go` 合并冲突时同时保留 dev-zz cache token 口径测试和上游高峰倍率图片 token 计费测试。
- 上游新增迁移编号 `158` 与 dev-zz 既有 `158_add_usage_log_schedule_meta.sql` 并存，沿用本分支同号迁移按文件名并存的既有口径。

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

## 2026-07-02 - 上游 main 同步到 dev-zz-develop：Spark shadow、Grok media、用量快照与支付修复

范围：
- 上游同步：`origin/main` `7dc7cfce` 合并到 `dev-zz-develop`
- 后端：Spark shadow 账号、Grok media / xAI media、OpenAI-compatible Grok、`/count_tokens`、dashboard snapshot-v2、支付 refund pending/resume、OAuth 邮箱补全、risk-control matched keyword、订阅撤销缓存、dateline fingerprint 归一化、GPT-5.5 / Codex
- 前端：账号管理、账号编辑、渠道定价、自检配置、用量图表、用户用量页、运维系统日志、支付/订单、设置页、i18n、主题类名
- 迁移：`154_account_spark_shadow.sql`、`154a_account_spark_shadow_indexes_notx.sql`、`156_content_moderation_matched_keyword.sql`、`157_user_platform_quotas_add_grok.sql`
- 文档：`docs-site/dev-zz/{changelog.md,patches.md,maintenance/merge-log.md}`

改动：
- 吸收 Spark shadow 账号体系：schema 字段、父子账号展示、shadow 凭据跳过、Spark 窗口配额、调度路由、账号测试、管理端账号操作与测试覆盖。
- 吸收 Grok media / xAI media 和 OpenAI-compatible Grok 网关路径，新增 media 处理、模型路由、账号测试和 `/count_tokens` 兼容。
- 吸收上游用户用量 dashboard snapshot-v2、`billing_mode`、`request_type`、reasoning intensity、图表 breakdown 与导出修复。
- 保留 dev-zz 用户/admin 用量边界：用户 `/usage/dashboard/models` 和 snapshot-v2 模型列表只返回用户安全字段，不返回 `cost` / `account_cost`；用户模型分布图同步关闭 Standard / Account Cost 列。
- 管理端账号页保留 dev-zz 账号归档语义（仅 disabled 可归档、恢复为 disabled），同时接入 Spark shadow 账号操作和 parent 展示。
- 账号编辑弹窗保留 dev-zz 模型映射模式、模型探测和二开主题，同时兼容 Spark shadow credentials 的最小提交。
- 管理端渠道、用量图表、DataTable、系统日志和设置页继续沿用 stone / emerald 二开主题，并吸收上游新增字段、i18n 和排序/可访问性修复。
- 后端使用量仓储保留 owner analytics 和用户安全 DTO，吸收上游 billing mode 快路径、模型来源过滤和 group stats 聚合。
- `backend/cmd/server/VERSION` 保留 dev-zz 发布线 `1.4.1`，不采用上游 `0.1.142`。

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
