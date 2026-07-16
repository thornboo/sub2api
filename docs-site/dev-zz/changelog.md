# 变更记录

## 2026-07-17

- 修复企业成员列表在历史接口返回 `group_ids: null` 时白屏的问题：后端读取和创建成员时统一返回非 `nil` 空数组，前端 API 边界兼容旧版本响应并在数据进入页面前规范化为 `[]`。
- 新增 Repository 公共读取 / 创建 JSON 契约、全部成员变更 API 的旧响应兼容契约，以及待配置无分组成员页面渲染回归测试；发布版本提升为 `v1.7.6`。
- 同步上游 `main`（`bc2244c83`）到 `dev-zz-develop`：吸收异步图片任务与 S3 兼容对象存储、API Key 计费倍率自省、上游 Sub2API 倍率探测、图片输入 Token 独立计费、操作审计 / 会话绑定 / step-up 2FA、分组与渠道监控复制、管理员批量用户限额，以及 OpenAI / Grok / WebSocket 正确性修复。
- 冲突合并继续保留 dev-zz 企业成员路由、预算预留与用量归因、供应商成本池、`schedule_strategy`、stone / emerald 视觉、隐藏认证入口、长期数据保留和 `1.7.4` 版本线；同步接入上游低倍率账号优先、图片输入价格与用量字段、审计中间件和新管理端入口。
- OpenAI APIKey 的参数 400 不写持久化模型冷却；502/503/504 等瞬时上游错误采用上游新的 account+model 连续失败运行时冷却，404、明确模型限流及其它平台的模型级错误仍沿用 dev-zz 持久化冷却边界。
- 新增同号迁移 `178_channel_image_input_price.sql`、`179_usage_log_image_input_tokens.sql`、`180_audit_logs.sql`、`181_group_duplicate_operation_id.sql`，均与既有企业成员迁移按完整文件名并存，不改写已应用迁移。

## 2026-07-16

- 企业成员 CSV/XLSX 导入的六个外部聚合 Token 字段支持非负且最多两位小数；`421.63` 会在预览、不可变迁移基线、导入结果、成员预算汇总和页面展示中保持为 `421.63`，超过两位有效小数时明确拒绝而不静默取整。新增 migration 191 将对应基线列升级为 `NUMERIC(21,2)`；迁移 Token API 改用精确十进制字符串，页面不再把百万级值 compact 缩写或经 JavaScript `number` 改写，真实请求日志 Token 仍保持整数语义。
- 发布 v1.7.3：在 v1.7.2 基础上补齐企业成员请求回执、结算 outbox 与结果不明保护，避免成员请求归因丢失、重复执行或 Batch image 在上游结果未知时被错误退款；同时吸收截至 `eb2b8632d` 的上游账号、计费、Grok 和 Agent Identity 更新。
- 发布前修复上游 `NewAdminService` 构造契约变化造成的 integration fixture 漂移：测试改用与生产 wiring 一致的管理型账号仓储并补齐返佣服务依赖，完整 integration、CI、Security Scan 和正式分支镜像门禁均以精确发布候选提交为准。
- 增量同步上游 `main`（`eb2b8632d`）到 `dev-zz-develop`：吸收 Grok 自定义上游地址 / 请求头覆写、OpenAI Agent Identity 独立导入与 Codex 能力、订阅套餐币种、管理员充值返佣设置和 locale 消息编译保护；继续保留企业成员路由 / 预算 / 归因、owner / admin 数据边界、`1.7.2` 版本线和 stone / emerald 视觉。
- Grok OAuth 官方地址保持可信端点，自定义地址受全局出站 URL 安全策略约束，认证头和会话路由头不得覆写；账号创建、编辑与批量编辑共用请求头 JSON 导入 / 复制工具。新增订阅币种迁移与企业成员同号 `177` 迁移按完整文件名并存，不修改既有迁移。
- 修复上游 locale 编译契约测试缺少直接依赖的问题：显式声明 `@intlify/message-compiler@9.14.5`，确保 pnpm 严格依赖环境可运行；新账号控件同步使用 dev-zz 色板并补齐开关无障碍状态。

## 2026-07-15

- 增量同步上游 `main`（`d515c3045`）到 `dev-zz-develop`：吸收 Grok OAuth 池主动刷新 / 对账、OpenAI 首输出与 WebSocket 首消息超时、Chat 直接响应桥、Codex 工具流终止、调度 outbox latch 和 XAI URL 安全修复；继续保留企业成员路由、严格 Tool Search 契约、fork 镜像、`1.7.2` 版本线和 stone / emerald 视觉。
- 管理端账号菜单新增静态凭据账号的幂等复制入口：复制配置、凭据和有序分组，重置用量 / 错误 / 限流等运行态并默认不可调度；OAuth、setup-token 和影子账号不允许复制。用户 Key 表新增默认隐藏的可排序 ID 列，且不削弱既有批量选择、标签和列偏好。
- 网关新增根路径 `/models` 别名并保持企业成员授权链；OpenAI native Responses 首输出、high-effort 覆盖、WebSocket 首消息和 token refresh 池的并发 / QPS / 熔断 / 周期超时均改为显式配置，默认值保持原有行为或选择禁用。
- 同步上游 `main`（`4355861ef`）到 `dev-zz-develop`：吸收 OpenAI Agent Identity、Codex models 跨账号重试、Grok SSO / 自动探测 / 凭据 failover、长上下文计费、系统日志 host、可选 Server-Timing、调度器增量刷新与请求取消修复；继续保留企业成员归因、owner / admin 隐私边界、`schedule_meta`、fork 镜像、`1.7.1` 版本线和 stone / emerald 视觉。
- 普通用户用量响应同时展示成员归因与长上下文计费证据，但不返回上游账号 ID；系统日志 host 进入筛选、列表、清理确认和索引。上游新增 `174/175/176` 迁移按完整文件名与 dev-zz 同号迁移并存，不修改已应用迁移。
- 企业账号新增独立“成员使用记录”入口，以成员筛选和成员排行作为一级交互，统一驱动统计、图表、请求明细与错误记录；Key 只作为明细来源展示，不提供 Key 维度切换或主筛选。原“使用记录”固定只展示企业账号普通 Key 请求，避免企业 owner 自用与成员用量混在同一模块。
- 修复 OpenAI 专用计费路径遗漏 `usage_logs.member_id` 与成员快照的问题；新增迁移只依据不可变成员预算账本回填可证明归属的历史记录，并将账本关联回真实 usage 事实。无法证明请求时名称或编号的旧记录保持空快照，不使用成员当前资料改写历史。
- 企业能力关闭后继续保留“成员使用记录”历史只读入口，但成员管理和成员 Key 新流量仍保持禁用；普通用户用量 DTO 不再返回上游账号 ID，管理员接口继续保留调度调查所需字段。
- 企业成员路由与结算进一步收紧：simple 调度也只查询当前成员授权分组；请求写入后连接中断、首输出超时和异步任务持久化失败统一视为结果不明，不再跨组重放或自动释放预算。WebSocket 在 `response.create` 进入上游写入边界后如发生写入结果不明、读取中断或缺少终态，会停止换连/HTTP 重放并保留成员预算待对账；明确的上游拒绝仍按原恢复策略处理。真实用量已取得但本地统一计费失败时，版本化 settlement outbox 会保留完整结算命令并幂等恢复，且数据库复合外键禁止跨成员/owner 归属；Batch image 在 provider 调用前先持久化 `provider_submitting`，进程中断或创建结果不明时只会转为保留 hold 的 `submission_unknown`，已完成外部工作后的结算失败也继续低频重试，两者都不再误退款。

## 2026-07-13

- 管理端使用记录改为服务端分页下的自然表格渲染，移除页面与 `DataTable` 的双重滚动容器，并使用 usage ID 作为稳定行键，修复大页容量下只有 sticky 用户列可见、其余单元格空白的虚拟滚动回归。
- 使用记录单页容量独立限制为 10/20/50/100；即使系统全局表格配置为 1000 条，该模块也最多请求并渲染 100 条，且不会覆盖其他列表的共享页容量偏好。公共 `Pagination` 同步修复为真实遵守调用方选项，并支持关闭全局偏好写入。
- 使用记录的端点、模型映射、Token 与费用展示增加历史数据类型防御，字符串数值、空值或无效数值不再中断整行渲染。
- 企业成员控制从单一自然月预算扩展为成员级 5h、1d、7d 与自然月聚合限额；成员名下所有 Key 共享同一套限额和持久化预留，在并发请求下不能各自获得一份独立额度。单 Key quota/限额继续作为额外控制层，以最先耗尽的一层为准。
- 编辑成员统一展示四个窗口的限额与已用金额，无需额外填写调整原因；自然月差额写入不可变 `manual_adjustment` 账本，窗口投影与带系统来源说明的 before/after 审计在同一事务中更新，不覆盖真实请求记录。
- 创建成员同步支持填写 5h/1d/7d/月初始已用额度，无需额外填写开账说明；成员、分组、月度 `migration_opening`、窗口起点和系统归因审计证据在同一事务中提交，禁止“先建成员、后补用量”的部分成功状态。
- “稳定成员编号”改为面向用户的“成员编号”，编辑态只读；后端普通更新接口同步拒绝修改，继续作为不可复用的导入匹配和历史审计身份。
- “有序分组候选”改为“成员可访问的分组”：候选来源与“我可访问的分组”共用 owner 当前分组授权，勾选表达成员授权，已选顺序表达调用优先级和 fallback 顺序。
- CSV/XLSX 企业成员导入模板、解析、校验和事务写入同步支持成员 5h/1d/7d 限额；新增迁移 `179_enterprise_member_rate_limits.sql`、认证缓存版本和后端/前端契约测试。

## 2026-07-12

- 企业成员长期架构由 ADR-0003 正式取代“Key 即员工”的临时结论：企业能力使用独立 `account_type`，`role` 继续只表达 admin/user 授权；成员不可登录，但拥有稳定 code、多把 Key、有序分组、成员预算和历史归属。
- 新增完整企业成员目标设计，覆盖请求级 `ActiveGroup`、跨协议 handler 分派、分组内账号 failover 与受控跨分组 fallback、响应提交边界、全入口能力矩阵和普通 Key 兼容。
- 成员月预算采用持久化预留、预算账本、幂等结算、崩溃恢复和对账；迁移期开账、人工调整与真实 usage 分开保存，不用调用后累加或伪造请求记录冒充可靠硬限额。
- 明确成员/企业/Key 租户不变量、管理员撤权传播、缓存版本失效、归档优先生命周期、服务器权威 CSV/XLSX 预览与确认令牌、TOCTOU 防护、审计、安全、可访问性和完整测试合同。
- 企业成员完整运行时正在按最终合同实施：已落地独立成员实体与租户约束、多 Key/有序分组、请求级 ActiveGroup、受控跨组故障转移、严格成员预算预留/结算/恢复/对账、owner/member 用量分析、服务器权威 CSV/XLSX 持久化慢导入 job、一次性加密 Key 结果交付、append-only 数据库审计、无高基数标签的 Ops 指标和企业成员管理前端；真实 PostgreSQL/Redis 已覆盖多 worker 租约 fencing 与跨实例认证缓存失效，剩余浏览器 E2E、集群指标汇总、容量和故障注入证明继续收口。
- Grok 异步视频任务新增持久化路由身份：创建响应提交前保存 owner/member/Key、实际 group 和实际 account，状态查询恢复原 group/account，并在成员撤权或身份不匹配时 fail closed。
- Responses WebSocket 在首帧读取后、上游 turn 提交前按模型选择成员分组，并按每个 `response.create` 独立预留和结算预算；长连接不再因无 body 的 HTTP upgrade 被误判为不可估算请求。
- Batch image 只使用显式启用能力的 Gemini 成员分组；无可用账号只在 Job/外部任务创建前允许切换候选。Job 持久化实际 group/member 快照，企业余额冻结与成员预算预留、异步捕获或释放分别在同一事务中完成。
- 企业 owner 可以在成员 Key 弹窗中预览并显式迁移仍带固定分组的历史普通 Key；提交事务锁定成员与 Key、复检版本和实时分组授权、保留原分组并写入 `member_key.adopted` 审计，失败时不产生部分迁移。
- 成员预算与用量详情补齐请求记录分页表，展示请求 ID、Key、对客模型、公开分组、类型、token、耗时和对客费用，并继续隔离上游账号、渠道、供应商成本和利润字段。
- 企业成员控制台全部可见文案与错误/确认提示迁入独立 zh/en locale 模块，动态数量和实体名称使用参数插值；自动化测试保证两种语言键集合对称、每个页面引用都存在，并阻止页面重新引入双语硬编码 helper。
- 企业成员导入 worker 增加租约 fencing：只有当前 `lock_owner` 可以提交或写失败状态，旧 worker 被接管后无法污染新处理；Redis Pub/Sub 集成测试同时证明用户级失效会清除另一实例已确认存在的旧 L1 认证快照。
- 企业成员导入领取和处理改用独立 timeout，默认 15 分钟处理窗口内持续心跳续租；短暂续租错误容忍重试，确认失租或错误超过租约期限后取消处理。5000 行 CSV 解析约 2.56 ms，真实 PostgreSQL 创建 5000 成员并写逐成员审计约 7.9 s；容量测试同时修复软删除 Key 冲突检查引用不存在表的问题。
- 企业成员进程级故障测试新增 Redis 容器重启和 PostgreSQL 活跃事务连接强杀：Redis 恢复订阅后单次广播可清除重启前旧 L1；PostgreSQL 中断时 5000 行事务零部分提交，租约过期后可被新 worker 接管。worker Stop/timeout 也补齐取消、等待和独立失败 context 覆盖。

## 2026-07-11

- 修正 Responses → Chat Completions 工具发现桥的协议边界：缺省 `execution` 的 type-only `tool_search` 保持官方 hosted 语义，不再静默改写为客户端执行；只有显式 `execution: "client"` 才会生成代理 function，旧客户端缺省行为需账号级兼容开关明确启用。
- 新增 request-local `ResponsesToolRegistry`，按输入顺序合并顶层、`additional_tools` 和 `tool_search_output`，保留载体来源、加载状态与回程身份；顶层 `defer_loading: true` 工具在真正加载前不再暴露给 Chat 上游。
- 动态加载的顶层 function 在非流式、流式和下一轮历史中保留官方 namespace 身份；重复 `tool_search_output.call_id` 以后者替换前者，避免旧定义被 union 回当前工具集合。
- Chat `allowed_tools`、旧式隐式 client tool search 与有损 custom grammar 降级改为账号级显式标记；hosted/server-only 工具、摊平名称冲突或跨来源回程身份冲突统一返回 typed capability mismatch，由 Responses handler 排除当前账户并继续换号，不把 Chat transport 的表达限制记成账号健康故障。
- 重复 Tool Search output 在 Chat 历史中只保留首个 tool result，后续副本只更新当前 callable set；历史 function call 按其所在输入位置解析身份。流式 added/done/completed 共用同一 output item ID。
- 工具定义比较改用保留 JSON number 的解码方式，避免超大数值精度折叠；原始载荷预检拒绝重复 JSON key，并把声明工具、动态工具和 `allowed_tools` 引用统一计入数量/字节/深度预算。历史 identity 在输入 replay 时缓存，避免按历史调用重复扫描工具；Responses input 和单项 content/summary parts 各最多 16384 项，根对象、工具对象、tool choice、input item、content/summary part 与嵌套 image URL 对象最多 64 个字段，转换器只解码实际使用的 part 字段；上游 custom arguments 也改为无通用 map 的按字段读取。Chat fallback 流式工具参数使用线性 buffer，单调用最多 16 MiB、单响应合计最多 32 MiB，超限时 Responses 客户端收到 `response.failed`、Anthropic Messages 客户端收到 `event: error`，随后终止上游读取且不伪装成正常完成，防止大请求或异常上游返回造成 CPU / 内存放大。请求本地 400 使用 typed client error，不再污染所选账号健康。本轮只更新 `dev-zz-develop`，不提升 `dev-zz`、不打 tag、不发布。

## 2026-07-10

- 增量同步上游 `main`（`e316ebf5`）：补齐 Codex custom/freeform 工具、`tool_search` 和 namespace MCP 子工具在 Responses → Chat Completions 降级路径中的请求转换、历史往返、非流式响应与流式事件还原。
- 合并复审补齐 `tool_search_output.tools` 与 `additional_tools.tools` 的下一轮动态加载；2026-07-11 follow-up 进一步按载体来源区分 searchable / loaded / callable，并补齐动态顶层 function 的 namespace 身份。
- namespace 摊平名采用稳定长度限制与哈希后缀，function/custom、代理名、同名不同完整定义和跨 namespace 撞名显式拒绝；JSON 语义等价的同名定义去重，未知扩展字段也参与比较。2026-07-11 后，`allowed_tools` 和有损 custom grammar 受账号能力门控，无法保真时换号而不再宣称无条件等价。
- 本轮 10 个上游提交、8 个后端文件自动合入且无冲突；没有迁移、前端、依赖、workflow 或版本变化，继续保留 dev-zz `1.5.1`，不提升正式分支、不打 tag、不发布。
- 增量同步上游 `main`（`07fac347`）：修复 ops capture writer 释放后晚到访问的 nil panic；合并复审同时阻止被 compact keepalive 包装的 writer 回池复用，确保外层 middleware 仍读取本请求状态且不会跨请求串用 writer。
- Responses ↔ Anthropic 非流式和流式转换完整保留 `cache_creation_input_tokens`；Anthropic 普通输入扣除 cache read / creation，Responses 总输入加回两类缓存 token，避免缓存写入用量丢失或重复计入输入。
- 本轮 7 个上游提交、6 个文件只涉及后端正确性；唯一版本冲突继续保留 dev-zz `1.5.1`，不提升正式分支、不打 tag、不发布。
- 合并后复审修正 Fast / Flex 设置的失败原子性：无效用户 ID 在写入前被拒绝，普通设置、认证来源默认值和策略在同一次批量写入中保存；策略变更补入审计，前端和 zh/en 文案同步增加校验与契约覆盖。
- `Codex ` 家族大小写变体统一规范化为上游接受的前缀；文档明确用户专属规则的模型白名单 fallback 是终止结果，以及 WebSocket 设置在新建会话时生效。
- 增量同步上游 `main`（`deff3123`）：Fast / Flex 策略新增用户 ID 范围，用户专属规则优先于全局规则且只使用 API Key 认证注入的可信 owner 身份；管理端可维护用户 ID，服务端拒绝非正数和重复值。
- Codex OAuth 请求按最终 User-Agent 配对 `originator` 并校正过低版本头，compat messages bridge 继续维持无 `originator` 语义；Grok Responses 同步保留 OpenAI-compatible `reasoning_effort`。
- 本轮 7 个上游提交、30 个文件自动合入且无冲突；复核确认 dev-zz 管理员 7 项运行设置、管理员用量证据 guard、供应商成本、账号归档、模型自检、用户/admin DTO 隔离和 `1.5.1` 版本线保持不变。
- 同步上游 `main`（`6dd3274a`）到 `dev-zz-develop`：吸收 GPT-5.6 reasoning / cache write / usage / 计费修复、API Key 最近使用 IP 与当前并发排序、管理端用户 Token 排行、版本回退、Grok 视频计费元数据和 Go 1.26.5。
- 接受上游 Go 服务 / 仓储和 zh/en i18n 的模块拆分，同时保留 dev-zz 的模型自检无重试边界、用量 `schedule_meta`、cache-read 计费口径、账号归档与倍率变更 Key 失效、成本优先调度、fork release 链接和 stone / emerald 管理端视觉。
- 管理端供应商新增 / 编辑统一改为 Modal，并在供应商默认资金池维护低频的默认充值换算和默认参考汇率；修改默认值只影响后续流水，不重算历史充值记录或成本快照。
- 供应商标签页改用账号页顶部的同一套操作栏：隐藏不影响供应商数据的账号筛选、自动刷新和账号工具；主操作按标签页切换为“添加账号”或“添加供应商”，卡片内部不再重复显示刷新 / 新增按钮。
- 普通充值记录默认只需输入支付金额，到账额度按供应商默认换算自动展示，参考汇率自动带入；“本次与默认不同”时才展开实际到账和本次汇率覆盖。赠送直接录到账额度但不定义独立单位成本，赠送和调整都不刷新当前成本快照。
- 新增 `upstream_cost_pools.default_effective_cny_per_usd` / `default_reference_fx_rate` / `is_default`，把稳定默认配置、默认池身份与最近一次真实成本拆开；没有真实快照时当前成本保持为空，不进入账号成本排序或 `cost_first` 调度。
- 供应商创建改为严格语义：重名返回 `SUPPLIER_NAME_CONFLICT`，不再复用并覆盖已有供应商默认配置。系统供应商不再进入账号成本 DTO / 排序；充值成本变化会主动刷新绑定账号的调度快照。
- 已归档供应商继续保留已有账号绑定，并在账号编辑中以禁用历史项展示；只有管理员明确点叉号才解绑，所有新绑定入口都拒绝已归档供应商。
- 供应商硬删除收紧为“从未使用”：active 绑定、历史绑定、充值 / 快照或非默认池都会阻止硬删除，避免为了删除供应商而丢失归属审计链。

## 2026-07-09

- 管理端「供应商」标签页补齐供应商编辑、归档 / 恢复和受限硬删除；2026-07-10 复审后，硬删除进一步收紧为无任何账号绑定历史、无非默认资金池、无充值记录、无成本快照。
- 供应商删除不再清理历史绑定；曾被账号使用或已有成本事实的供应商应归档而不是硬删。
- 新增 `upstream_suppliers.is_system` 稳定标志，后端和前端都用该字段保护旧迁移遗留的系统供应商；系统供应商退出正常业务路径，不再出现在供应商 / 资金池列表、账号候选、active 绑定查询或按账号充值兜底逻辑中。
- 账号编辑弹窗继续不承载真实充值比例、参考汇率或资金池基础成本；它现在维护供应商归属、上游分组名和这把 key 的上游分组倍率，综合折扣按供应商充值折扣乘以账号分组倍率展示。
- 新增 `upstream_account_cost_bindings.upstream_group_name`；`default_multiplier` 继续作为兼容存储列承载 `upstream_group_multiplier`。
- 新增供应商更新 / 删除和账号编辑边界的 Go / Vitest 覆盖，并更新 docs-site 的功能页、接口索引和验证矩阵。

## 2026-07-08

- 发布 v1.4.10：将上游 `main`（`e8e23425`）同步到 `dev-zz-develop` 后提升到正式 `dev-zz`，吸收批量生图 MVP、OpenAI Responses / Chat Completions fallback 共享 CC 管线、网关文件拆分、Grok / web-search / image namespace 等兼容修复。
- 批量生图新增任务、队列、冻结余额、结算、下载、清理、worker runtime、Gemini / Vertex provider、分组权限、管理端 pricing / gate / hold ratio 配置，以及用户侧批量生图入口和指南页。
- 网关同步上游拆分结构，同时保留 dev-zz 的 model self-check probe 安全边界、OpenAI cache-read usage 口径、ScheduleMeta、真实 `UpstreamEndpoint` 记录和 messages 后置 fallback 顺序。
- 修正合并后的 rate-limit 顺序边界：5xx 显式 temp-unsched 规则优先于通用模型级失败，非模型级 4xx / 429 仍保留账号自定义 temp-unsched 兜底，404 / model_not_found 继续走模型级冷却，Anthropic 429 官方窗口仍优先。
- 本次同步继续保留 dev-zz 的 docs-site 文档中心、stone / emerald 控制台视觉方向、用户/admin 字段边界、供应商成本与模型自检策略。

## 2026-07-07

- 管理端账号页把「供应商成本」标签改为「供应商」：供应商新增和充值记录入口集中到该标签页；账号创建 / 编辑弹窗不再承担新增供应商或高级成本 / Key 配额查询配置，账号编辑仅保留供应商选择并支持清空绑定。
- 发布 v1.4.9 follow-up：刷新 `xlsx` audit exception 的风险说明和到期日，使 Security Scan 不再因 2026-07-06 过期的例外阻断；运行时行为不变，仍只在导出时动态加载 `xlsx`，不解析用户上传的 XLSX 文件。
- 管理端账号列表新增供应商成本上下文列，并把「供应商、综合折扣、充值比例、倍率」放到「分组」列后方，便于管理员在同一行同时查看账号分组和上游成本归属；参考汇率保留在供应商 / 资金池详情中查看。
- 「综合折扣」和「倍率」支持账号列表服务端排序；综合折扣排序按当前供应商成本、参考汇率和账号绑定供应商的默认倍率计算，未配置成本的账号排在末尾。
- 成本对比页保持供应商列表视角，作为供应商级充值记录入口；普通用户侧接口和页面仍不暴露供应商、上游账号、资金池、真实成本或利润字段。

## 2026-07-06

- 管理端上游成本池阶段 1 后端兼容层落地：新增供应商、资金池、账号成本绑定和成本快照表，现有账号会获得默认资金池，旧账号级充值记录接口继续可用并写入账号绑定资金池。
- 新增管理端资金池 API：供应商/资金池列表与详情、资金池账本、资金池账号绑定、账号成本绑定读取与替换。普通用户侧接口不暴露供应商、资金池、上游余额、真实成本或利润字段。
- 成本池 review follow-up：`adjustment` 只保存账本不刷新当前成本快照；历史默认资金池创建改为事务内账号级锁；账号成本绑定 `GET` 保持只读；历史未归类供应商补 active 名称唯一约束。
- 成本池页面方向修正：后续前端主交互应在账号编辑页选择或新建“上游供应商”，供应商只有一个资金池时自动绑定默认资金池；资金池 / 钱包选择只在多钱包或高级运营场景展示。
- 本阶段不自动合并共享钱包，不改变普通用户扣费，不启用成本优先调度；资金池管理页、余额查询迁移、usage 上游成本证据和调度联动仍是后续阶段。

## 2026-07-02

- 同步上游 `main`（`a632cb00`）到 `dev-zz-develop`：吸收订阅分组高峰时段倍率全链路支持，包括 group schema / DTO、管理端分组配置、API Key auth cache、gateway 计费记录、订阅套餐和可用渠道展示。
- 新增分组字段 `peak_rate_enabled`、`peak_start`、`peak_end`、`peak_rate_multiplier`。高峰倍率仅订阅分组可启用，时间格式为 `HH:MM`，区间为同日左闭右开，不支持跨天；高峰因子只叠加 token 计费倍率，图片按次计费不受影响。
- 本次合并保留 dev-zz 的 docs-site 文档中心、fork release / 镜像策略、账号归档语义、模型自检状态快照，以及用户/admin 用量字段边界。用户侧可看到公开分组的高峰倍率提示，但仍不暴露上游账号、渠道、内部成本或管理员字段。
- 上游新增迁移 `backend/migrations/158_add_group_peak_rate_multiplier.sql`；与本分支既有 `158_add_usage_log_schedule_meta.sql` 按文件名并存，沿用此前同号迁移并存口径。
- 同步上游 `main`（`7dc7cfce`）到 `dev-zz-develop`：吸收 Spark shadow 账号体系、Grok media / xAI media 路由、OpenAI-compatible Grok 转发、`/count_tokens` 兼容、用量 dashboard snapshot-v2、`billing_mode` / `request_type` 过滤、支付 refund pending/resume 修复、OAuth 邮箱补全、risk-control matched keyword、订阅撤销缓存、dateline fingerprint 归一化、GPT-5.5 / Codex 相关逻辑以及 README / Docker / deploy 更新。
- 本次合并保留 dev-zz 的 `1.4.1` 发布线版本号、docs-site 文档中心、stone / emerald 控制台主题、账号归档语义、模型自检状态快照、fork release 链接策略，以及用户/admin 用量数据边界。
- 用户 `/usage/dashboard/models` 与 snapshot-v2 模型列表继续返回用户安全字段，不返回 `cost` / `account_cost`；用户模型分布表同步隐藏 Standard / Account Cost 列，避免把管理员计费字段暴露到用户页。
- 管理端账号页在保留“停用 -> 归档 -> 恢复为停用”的二开语义基础上，吸收 Spark shadow parent 展示和 shadow 账号操作；账号编辑弹窗保留模型映射模式，同时兼容 Spark shadow credentials。
- 管理端渠道定价、自检、用量图表、系统日志和账号列表继续沿用 stone / emerald 二开主题，并吸收上游新增字段、breakdown、排序辅助和 i18n 修复。

## 2026-06-29

- 同步上游 `main`（`c99112a9`）到 `dev-zz-develop`：吸收 Grok / xAI OAuth 与订阅配额探测、Codex / ChatGPT 账号检测加固、OpenAI PAT auth mode、Responses / Chat Completions 兼容修复、OpenAI 图片 bridge 与 overloaded 错误识别修复、支付金额/币种显示修复、用户 API Key 列设置、运维系统日志 API Key 筛选和 sponsor / README 更新。
- 运维系统日志新增按 API Key ID 查询和清理的后端字段与索引；上游迁移在 dev-zz 中顺延为 `162_add_ops_system_logs_api_key_id.sql` 与 `163_add_ops_system_logs_api_key_id_index_notx.sql`，避免与既有 dev-zz 迁移编号冲突。
- 本次合并保留 dev-zz 的 `1.4.0` 发布线版本号、docs-site 文档中心、stone / emerald 控制台视觉方向、企业 Key 标签/批量/用量下钻语义、模型自检 runner、OpenAI usage 真实上游端点记录和系统状态保护。
- 用户 API Key 页面在保留标签、批量创建/批量操作和单 Key 用量下钻的基础上，吸收上游列设置能力；管理员系统日志表在保留二次确认清理弹窗的基础上，吸收 `api_key_id` 筛选。

## 2026-06-28

- 用户侧模型服务状态改为**定价驱动的站点自检**：在渠道定价里按模型开启「自检」开关后，系统对该模型解析出可服务的上游账号（跨分组去重），用合成请求走本站网关真实链路探测，结果写入 `model_self_check_histories`。探针请求带专用上下文标记，**不写 `usage_logs`、不计费，且不触发生产账号的限流封禁 / runtime-block / 重试 / failover**；用户侧 `/monitor` 由此按 **分组 / 模型** 维度展示健康状态、24h/7d/30d 可用率和降级比例。
- 新增管理员设置：`model_self_check_enabled`（软开关）、`self_check_default_interval_seconds`（默认探测间隔）、`self_check_max_concurrency`（全局并发上限）、`self_check_max_tasks_per_round`（单轮去重任务上限，成本护栏）。
- 用户 `/api/v1/model-status` 响应改为按分组返回，新增 `group_id` / `group_name` / `degraded_ratio_24h` 字段；仍不返回 `account_id`、`provider`、`endpoint`、`channel_id`、成本等内部字段。上游「渠道监控」子系统（`channel_monitor_*`）保持现状，仅管理员用于排查上游。

## 2026-06-26

- 用户侧 `/monitor` 从“渠道监控”切换为“模型服务状态”：按公开模型名展示当前状态、24h / 7d / 30d 可用率、平均延迟和最近时间线；新增 `/api/v1/model-status` 与 `/api/v1/model-status/detail?model=...`，并撤下旧用户侧 `/api/v1/channel-monitors` 探针路由，避免普通用户看到上游 monitor、provider、group、endpoint 等内部字段。管理员渠道监控配置与排障入口保持不变。
- 同步上游 `main`（`ce6af413`）到 `dev-zz-develop`：新增 GPT-5.5 codex instructions 并作为 codex 最新指令 fallback；修复 codex spark 路径剥离 `image_generation` 工具导致的上游 502；管理端账号「重置 OpenAI 周限」增加二次确认；sponsor / 合作方 logo 与多语言 README 更新。本次为干净合并，无冲突，未触及 dev-zz 视觉、认证入口、数据保留与用量字段边界等已记录策略。

## 2026-06-25

- 时间范围选择器（共享 `DateRangePicker`）的「开始/结束日期」旁新增时间输入，可精确到秒，默认开始 00:00:00、结束 23:59:59；结束按「含当秒」处理（发给后端时 +1 秒转为排他上界），所以默认值等价于按整天，与原行为一致。预设（今天/近7天等）重置为整天默认时间，可再手动微调。覆盖 admin 全站用量分析/仪表盘、user 用量分析/仪表盘 4 个页面。
- 后端 `start_time/end_time`（datetime）优先于 `start_date/end_date`（日期），命中时不做整天补偿；底层按 `created_at` 时间戳半开区间查询。
- 说明：精确时间对统计卡片、模型分布、日志列表为秒级精度；趋势图在「按小时」粒度下仍按整点聚合（预聚合固有行为）。

- 运维监控总览新增“客户可见失败”口径，用 `error_count_total / request_count_total` 展示客户实际收到失败响应的比例；SLA 卡片继续保留排除客户侧限制后的稳定性口径。
- 运维错误明细入口支持从“客户可见失败”“SLA 错误”“客户侧限制”“非限流上游错误”“上游限流/过载”直接带筛选进入，减少客户投诉排查时反复手动切筛选。
- 错误明细视图文案从“错误 / 排除项 / 全部”调整为“SLA 错误 / 客户侧限制 / 全部失败”，上游错误文案从“错误数（排除429/529）/ 429/529”调整为“非限流上游错误 / 上游限流/过载”。
- 错误列表接口新增 `status_codes_exclude` 筛选参数，用于查看非 429/529 的上游错误明细；现有 SLA、客户侧限制和状态码筛选口径保持不变。
- 修复自定义时间范围下，运维总览和错误 / 请求明细使用不同时间窗口导致“卡片有数、明细为空”的问题。
- 修复上游错误卡片按 `provider` 归因统计、明细却强制 `phase=upstream` 的口径错位；上游错误明细默认改为 provider 归因口径，避免 network/provider 类失败被漏查。
- 账号列表的模型限流徽标新增「解除」按钮，管理员可精细到单个模型手动解除限流（仅清除该 scope，其它模型限流和账号级状态不受影响），无需再用一刀切的「恢复状态」。
- 新增管理员设置「模型级限流策略」：可配置失败阈值（连续 N 次失败才限流）、统计窗口和回退冷却时长。默认关闭，保持历史「首次失败即限流、回退冷却 1 分钟」的行为。
- 失败阈值基于 per-(账号, 模型) 的 Redis 滑动窗口计数（复用 OpenAI 403 计数器同款实现），窗口内无新失败自动衰减；上游返回明确 reset 时间时仍优先使用上游时间，配置冷却仅作为回退。

## 2026-06-22

- 同步上游 `main`（`85a3b122`）到 `dev-zz-develop`：合并缓存 Token 明细展示、OpenAI 图片 incomplete 故障转移、Gemini / Vertex Anthropic schema 兼容修复、Claude Code / CC Switch 识别更新、调度优先最早重置账号能力、订阅 affiliate rebate、promo code 过期时间清空、SELinux bind mount 标记和 sponsor 资料更新。
- 管理端 usage 统计卡片吸收上游缓存 Token tooltip，同时保留 dev-zz 当前 stone / emerald 视觉方向。
- OpenAI usage 记录端点冲突保留 dev-zz 的真实 result endpoint 口径，避免 chat-only API Key fallback 的上游端点记录回退为按请求路径猜测。
- `backend/cmd/server/VERSION` 合并冲突按 dev-zz 发布线保留 `1.2.1`，未采用上游 `0.1.138` 版本号。

## 2026-06-21

- 同步上游 `main`（`945b9b20`）：合并邮箱绑定后缀白名单校验、API Key IP ACL 拒绝提示携带客户端 IP、SSE `event:error` 响应体保留到运维日志、Anthropic 官方 5h/7d 窗口限流冷却保护、thinking block 协议识别与 retry 过滤收敛，以及 DeepSeek / GLM / Kimi / MiniMax / Doubao 多模态 embedding 的兜底定价。
- 管理端账号列表吸收上游账号 ID 列展示与排序能力，同时保留 dev-zz 当前表格多选按钮样式和 stone / emerald 控制台视觉方向。
- `backend/cmd/server/VERSION` 合并冲突按 dev-zz 发布线保留 `1.1.6`，未采用上游 `0.1.137` 版本号。

## 2026-06-19

- 发布 v1.1.5 patch release：修复管理员访问「可用渠道」时，管理端全量目录中 `groups` / `platforms` / `intervals` 等数组字段为 `null` 导致前端执行 `.filter()` 崩溃、页面主体空白的问题。
- 可用渠道接口前端入口现在会把后端返回的 `null` 数组归一为空数组；后端管理端全量目录也避免把空 platform / group 切片编码为 JSON `null`。
- `backend/cmd/server/VERSION` 更新为 `1.1.5`，固定版本镜像示例同步为 `thornboo/sub2api:1.1.5`。
- 发布 v1.1.4 patch release：将 v1.1.3 中额外加入的前端启动失败兜底页移除，使 2026-06-17 白屏事故修复重新收敛到根因修复，即删除危险的手写 `manualChunks` 拆包并保持 Rollup/Vite 默认 chunk graph。
- `backend/cmd/server/VERSION` 更新为 `1.1.4`，固定版本镜像示例同步为 `thornboo/sub2api:1.1.4`。

## 2026-06-17

- 管理员用量证据完整性阶段 1 落地：`/admin/usage` 在管理员证据上下文中穿透软删除解析已删除 Key 的名称和删除状态，明细、画像和导出会展示原 Key 名称加“已删除”标识，导出补充 Key ID、名称和删除时间。用户侧 `/usage` 和普通 `/keys` 列表仍只解析活跃 Key，不改变用户侧展示语义；DTO 不暴露已删除 Key 的明文。
- `/admin/usage` 的显式日期范围选择现在会回写到路由 query，刷新页面或分享链接时保留所选时间范围；首次无参数加载仍保持干净 URL 并使用内部默认日期。
- 同步上游 `main`（`b8a482e1`）：OpenAI `cyber_policy` 硬阻断透传与计费、OpenAI 账号 rate-limit quota 查询/重置、scheduler outbox 去重与 pending dedup 索引恢复、网关非 JSON/zstd/图片故障转移修复、渠道监控检测间隔 jitter、账号过期自动暂停索引等。用户用量页冲突已解决并补入 `cyber` 请求类型分支，沿用 dev-zz 深色主题 badge。
- 发布 v1.1.2 patch release，`backend/cmd/server/VERSION` 更新为 `1.1.2`，固定版本镜像示例同步为 `thornboo/sub2api:1.1.2`。
- dev-zz 镜像更新流程改为备份优先：部署文档明确日常更新先执行 `deploy/backup-dev-zz.sh` 备份，再 `docker compose pull` 并只重建应用容器，不删除数据目录和 `.env`。

## 2026-06-15

- 企业 owner 用量分析已从设计阶段进入实现阶段：用户侧 Usage 页面新增分析视图，后端提供 `/api/v1/usage/analytics/summary`、`leaderboard`、`models`、`groups`、`tags`、`trend` 六个用户认证域接口，统计范围始终绑定当前登录用户。
- owner 用量分析接口只返回用户可见字段：请求数、Token、实际扣费、Key 名称、标签、分组、状态、最后使用时间等；不返回 `account_cost`、上游账号、渠道、`upstream_model` 或其它管理员专属运营字段。
- API Key 禁用状态统一为 `disabled`；旧的 `inactive` 仅作为兼容输入归一化，不再作为持久化状态写入。
- 编辑 API Key 时，如果 `group_id` 没有变化，不再重新执行分组绑定授权；这允许用户继续编辑标签、额度、限流、IP ACL 等无关字段，即使该 Key 历史绑定的分组当前已不再可绑定。
- 前端 API Key 页面在普通编辑、批量编辑和筛选中使用 `disabled` 作为禁用状态，并在编辑 `quota_exhausted` / `expired` 系统状态 Key 时避免无意把系统状态覆盖成禁用。
- `api_keys.tags` 在仓储写入边界强制保持数组形态，`nil` tags 会写成 `[]`，避免 PostgreSQL `jsonb` 列出现 JSON `null` 并破坏标签筛选契约。
- 发布与部署默认切换到 fork 镜像：`thornboo/sub2api:latest` 或 `ghcr.io/thornboo/sub2api:latest`。上游镜像 `weishaw/sub2api:latest` 不包含 dev-zz 二开内容，不再作为本分支默认部署镜像。
- Docker 部署脚本默认从 `thornboo/sub2api` 的 `dev-zz` 分支下载部署文件，已部署服务日常更新推荐 `docker compose pull sub2api` + 只重建应用容器，不删除数据目录和 `.env`。
- 新增 v1.1.1 patch release 记录，并补充发布镜像、滚动更新、本地构建镜像迁移到发布镜像的部署说明。
- GitHub Actions 增加 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true`，用于验证 JavaScript actions runtime 对 Node 24 的兼容；项目自身前端构建 Node 版本仍保持 20。
- 文档站补齐 dev-zz 变更地图、接口索引、配置与迁移索引、验证矩阵，使 docs-site 可以从“几篇功能文档”升级为完整分支档案。

## 2026-06-14

- 新增企业用量分析中心设计文档，明确企业 owner 与平台管理员的用量分析边界、员工 Key 排行、分组/标签/模型分析、多供应商员工 Key 的长期方案，以及用户可见字段与管理员专属字段的权限矩阵；经审查补充 `AllowedGroups` / fallback group 授权约束、tags 重复计入契约和实时快照/历史聚合区分。
- 用户侧 API Key 列表新增单 Key 用量详情，可按小时、天、周、月查看趋势汇总，查看模型分布，并在弹窗内查看该 Key 的请求记录；用户侧模型统计只返回本人 Key 的请求数、Token 与实际扣费，不暴露管理员成本字段。
- 用户侧 API Key 批量修改与批量删除后端支持作用到当前筛选结果，并在无筛选条件或匹配数量超过 500 时阻止执行，降低误操作风险；当前前端仍以列表勾选作为批量操作入口。
- 用户侧 API Key 标签筛选改为加载当前用户完整标签候选，避免分页未浏览标签无法在下拉中选择。

## 2026-06-13

- 为用户侧 API Key 增加结构化标签字段和标签筛选，支持在创建、编辑、批量创建和批量修改时维护标签。
- 批量修改 API Key 新增标签操作，可对已选择的 Key 统一追加、覆盖、移除或清空标签。
- 为用户侧 API Key 管理新增批量创建能力，可按名称模板或名称列表一次生成多把 Key，并统一配置分组、额度、有效期、限流和 IP ACL。
- 批量创建结果只在首次响应展示完整 Key，提供复制全部和 CSV 导出；幂等重放不再返回完整 Key。
- 为用户侧 Key 列表新增按 ID 勾选后的批量更新与批量删除，支持统一修改分组、状态、quota、过期时间、限流、限流用量和 IP ACL。
- 批量更新 / 删除在后端使用事务一次完成，任一 Key 不属于当前用户或任一写入失败都会整批拒绝或回滚。
- 新增公共 Key 状态查询接口，作为企业 Key 管理的一阶段补充能力，方便只有 Key、没有站点账号的员工查询本人 Key 的可用状态、额度用量、过期时间、最近使用和限流配置。
- 优化运维监控明细弹窗体验：父级明细列表与单条错误详情支持叠加打开，关闭子详情不再连带关闭父弹窗。
- 修复多层弹窗下 Escape、遮罩点击、关闭按钮和页面滚动锁定的层级判定，使交互始终只作用于视觉最上层弹窗。
- 优化运维错误明细筛选区，增加明确筛选标签和用户可读搜索占位文案，并修复下拉菜单在弹窗内点击空白处无法自动收起的问题。
- 让错误详情中的响应内容和关联上游响应预览自动换行，避免长 JSON 或长错误文本需要横向滚动阅读。

## 2026-06-12

- 同步上游 `main` 的部署与运营合规确认：管理端继续使用前需确认合规承诺，法律文档可通过公开文档路由查看。
- 同步上游网关、Bedrock 兼容、idempotency、错误透传和账号分组调度索引修复。

## 2026-06-10

- 将 dev-zz 二开文档迁移到 `docs-site/dev-zz/`，使 VitePress 文档站成为本分支的完整文档中心。
- 把 dev-zz 源码构建部署脚本从 `secondary-dev/deploy-dev-zz.sh` 移到 `deploy/deploy-dev-zz.sh`。
- 新增完整的本地开发指南，涵盖前端、后端、PostgreSQL、Redis、可选的 Air 后端自动重启、验证命令和重置步骤。
- 保留 `docs/` 下的上游兼容项目文档，同时在 `docs-site/project/` 增加结构化项目文档。

## 2026-05-06

- 将首页热门模型的展示价格恢复为官方价格，同时保留折扣分组价格作为实际计费说明。
- 将首页热门模型展示价格从官方价的 80% 调整为 85%，并把中英文分组价格说明明确为折扣分组价格。
- 扩展账号模型探测功能：在创建/编辑模型映射区追加探测到的上游模型，以同名映射对的形式供管理员调整。
- 优化账号模型探测与映射设置：探测结果与上游目标模型比对，标记新增和缺失的模型，保留显式的白名单/映射模式，并使用所选渠道配置生成映射建议。
- 在自定义模型输入框旁新增 models.dev 目录搜索，方便管理员查找公开模型 ID 并填入白名单或同名映射行，同时保留手动输入。
- 为创建/编辑账号模型映射区新增“清空全部模型”操作，管理员无需切回白名单模式即可批量清空映射行。

## 2026-05-05

- 优化首页视觉体系，并从首页 / 页脚 / 页头移除公开的 GitHub 入口。
- 将控制台布局、侧边栏、页头、卡片、表格、对话框、下拉菜单、公告、用量视图和运维监控页统一向新的 stone / neutral / emerald 主题调整。
- 通过 body 级 portal 渲染日期范围和列设置下拉菜单，修复被裁切的问题。
- 修复运维监控帮助提示在页面滚动后的定位问题。
- 把首页新增的中文可见文案移入语言文件，使英文模式不再出现中文回落文案。
- 让日期范围和用量列设置下拉菜单在关闭时不再保持全局的滚动 / 缩放 / 点击监听。
- 重新设计登录和注册入口页，与首页的 stone / emerald 明暗视觉方向保持一致。
- 在前端登录、注册、资料绑定和管理端认证设置展示中隐藏 LinuxDo 和微信第三方认证平台入口。
- 更新资料身份绑定测试，以匹配仅前端隐藏 LinuxDo / 微信的行为，同时保留 OIDC 绑定的测试覆盖。
- 清理未使用的首页 i18n key，把剩余的推荐语首字母移入语言数据，并让页脚联系链接使用配置的联系信息而非 FAQ 锚点。
- 默认关闭仪表盘数据的自动保留清理，使用量日志、计费去重数据和用量仪表盘聚合在管理员手动删除前一直保留。
- 默认关闭运维数据的自动保留清理，使运维日志、指标、预聚合和渠道监控历史不被计划维护删除。
- 把运维系统日志清理的浏览器原生确认替换为项目弹窗确认，并展示当前筛选条件摘要。
- 为 `dev-zz` 分支新增二开 Docker 部署文档和源码构建部署脚本。
- 为 `dev-zz` 源码构建脚本新增启动前的自动部署备份。
- 新增带 SSRF 防护的管理端账号模型探测操作，通过后端拉取 OpenAI 兼容的 `/v1/models` 结果，并追加到创建/编辑模型白名单。
