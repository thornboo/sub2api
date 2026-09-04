# 变更记录

## 2026-09-05

- 账号管理的供应商列表支持按供应商、绑定账号、当前成本、充值比例、资金池折扣、累计支付、记录数量和状态进行本地升降序排列；未选择排序时保留原有默认顺序，缺失成本始终置后。累计支付继续按原币种展示，排序时使用充值记录逐笔参考汇率折算出的参考 CNY 总额，避免隐藏的币种优先级。供应商充值趋势弹窗默认按日展示，并在每次重新打开时恢复日粒度。账号列表综合折扣和供应商资金池折扣统一显示两位小数，底层计算精度保持不变。

## 2026-09-04

- 管理员消费统计新增独立的“上游应扣成本”证据：按实际成功账号、最终上游计费模型和完整请求用量重新解析价目表金额，再乘实际命中的默认或模型族倍率。账号显式绑定官方价目表渠道，运行时不再通过调用用户分组找价；人民币绑定必须选择明确价目表，美元可回落内置官方价。单条 usage 保留原始 CNY/USD 扣额、绑定 ID、实际倍率及参考汇率；跨账号聚合使用生成的参考 USD 金额，避免混加币种，并返回证据覆盖数和缺失数；全无证据时返回 `null`，普通用户接口不暴露供应商成本证据。
- 同步上游 `main`（`b1748c4ea`）到正式线 `dev-zz`：Ops 上游错误按每次请求尝试固化受管代理 ID / 名称，直接连接与未知路径分开表达；历史事件不再根据账号当前代理反推，且不会保存代理 URL、主机或凭据。
- 上游错误队列恢复 256 次尝试和约 512 KiB 的双重边界，保留最新失败与有限诊断正文，并记录被裁掉的更早尝试；dev-zz 企业成员实际分组、预算结果不明不重放和 Ops 分类 v2 边界保持不变。
- 同步赞助商清理；本轮没有数据库迁移、配置、依赖或版本号变化，`VERSION` 保持 fork `1.7.43`。

## 2026-09-02

- 同步上游 `main`（`5097b3145`）到正式线 `dev-zz`：分组新增强制 OpenAI Fast 和 Fast 按 Standard 计费开关，OpenAI / Composite 分组的管理端表单、认证快照、duplicate、网关转发与计费口径同步接入，同时保留 dev-zz 的企业成员最终分组、预算 / usage 原子归因和 sticky / no-replay 边界。
- OpenAI / Codex reasoning effort 映射支持按模型 exact / prefix / suffix 分组配置，超过分组上限时可选择自动降档或直接拒绝；WebSocket、Responses、Messages fallback 和 API Key chat cache identity 同步吸收上游修复。
- 价格目录支持 `pricing.override_file` 覆盖补丁，长上下文阶梯改为从目录 above-tier 字段数据驱动，并新增 1h cache write 价格字段；account stats 成本、图片 output token 计费、Codex OAuth / shadow credential tier 和 OpenAI service tier 分离同步修复。
- usage 新增 native compaction v2 标记与用户 / 管理端筛选；Kimi 原生 Responses 同步覆盖显式协议和 adaptive 能力路由，Claude Fable 5.1、Anthropic fallback beta 守卫、Ollama Cloud 国产平台用量窗口、账号 reauth / cooldown、兑换码本地时间解析、临时数据库启动重试和赞助商更新同步进入本分支。
- 新增迁移 `231_add_usage_log_native_compaction_v2.sql`、`232_channel_cache_write_1h_pricing.sql`、`232_group_force_openai_fast.sql`、`232_group_reasoning_effort_over_limit.sql` 和 `233_group_free_openai_fast.sql`；`VERSION` 保持 fork `1.7.43`，上游 `0.2.0` 不进入 fork 发布线。

## 2026-08-29

- 同步上游 OpenAI 流 / WebSocket 模型级限流：普通模型 429 不再扩大为整账号限流，OAuth Spark 配额窗口继续保留真实 reset，首输出前 keepalive 与 failover 模型归因同步增强。
- 更新 DeepSeek 官方高峰 / 低峰价格与渠道后缀模型定价优先级，并收紧 Fable 模型调度阈值作用域。
- 管理端账号页改用轻量 ETag 刷新上游账单倍率，保留 dev-zz 供应商 / 资金池视图；批量编辑可显式关闭 Codex fingerprint。
- 同步 Claude / Grok / Antigravity 兼容、订阅 reset、智谱配额、SMTP TLS、支付币种、monitor singleflight、分组错误提示和版本比较修复。

## 2026-08-28

- 同步上游 `main`（`7b693ae42`）到 `dev-zz`：usage 保存请求推理强度与实际发送强度，用户只见自己的请求值，管理员可审计映射差异；管理导出继续保留已删除 Key 证据。
- OpenAI raw Chat 无终态截断不再伪装成功；非流式 HTTP 200 失败、WebSocket 正常断开、跨供应商 reasoning / tools、图片能力冷却和 quota refresh 同步修复，并继续遵守企业成员预算结果不明与 no-replay 边界。
- 管理员可限制用户只能绑定授权的公开分组；新增用户字段、管理 UI、auth cache 和迁移。公开模型广场继续保持匿名 / 登录一致的客户安全目录，不展示专属分组、订阅分组或用户倍率。
- OAuth 注册保留 promo code，EasyPay 支持相对支付 URL；LinuxDo / 微信入口继续隐藏。新增 `231_add_usage_log_requested_reasoning_effort.sql` 与 `231_user_restrict_public_groups.sql`，`VERSION` 保持 `1.7.41`。

## 2026-08-26

- 同步上游 `main`（`efb46db0a`）到 `dev-zz`：Codex 模型目录按当前分组、平台、账号 mapping、实际路由和 capability 生成，并补齐 ETag、配置模型优先与 routed metadata。
- 模型同步把 models.dev 能力、账号 extra 快照和显式 mapping fallback 合并为受限目录；API Key `/models` 返回 404/405 时不再让已配置模型消失，也不会把临时不可调度扩大成新能力。
- OpenAI / WebSocket 同步 Lite、session-id、stale tool ID、上游 endpoint 和 quota 429 修复；Composite / sticky 继续遵守企业成员最终 `ActiveGroup`、durable binding、route lock 与未知结果不重放边界。
- 管理端账号模型选择器和 Key 使用页增加 Codex catalog 搜索 / 配置提示，同时保留 probe 与保存账号 sync 的明确分工；邮箱绑定、支付结果、Kimi 403 和 Antigravity token clamp 同步修复。
- `VERSION` 保持 `1.7.40`，上游 `0.1.183` 不进入 fork 发布线；本轮没有迁移、配置或依赖变化，未推送、发布或部署。

## 2026-08-25

- 同步上游 `main`（`e2d9b823f`）到 `dev-zz`：Gemini 工具 schema 移除不支持的嵌套字段，标量 enum 转为字符串，复合 enum 安全丢弃。
- Responses Lite 的 `additional_tools` 保留 `parallel_tool_calls:false`；上游拒绝 input `status` 时一次清理全部同类型 item，避免逐项重试耗尽预算。
- Grok OAuth / CLI proxy 使用官方 workspace User-Agent 与 `0.2.120` identity；普通 API Key 和非 CLI 目标不受影响。
- `VERSION` 保持 `1.7.39`，上游 `0.1.181` 不进入 fork 发布线；本轮无迁移、配置、依赖或前端运行时代码变化。

## 2026-08-24

- 同步上游 `main`（`03e8ab413`）到 `dev-zz`：新增默认停用的独立进程插件框架和管理页面，当前只允许 OpenAI OAuth 出站传输；插件不能拥有账号选择、企业成员最终分组、预算 / usage、sticky 或 replay 决策，生产默认拒绝未签名包。
- OpenAI Responses、Chat Completions 与 WebSocket 支持 `fast` service tier，并按上游实际结果安全降档计费；普通请求不能因上游回显被升级收费。另同步 quota 自动重置、Codex identity、WebSocket v2、工具 identity 和模型目录读取上限。
- 渠道分时价格继续使用 dev-zz 单一 `TimePricing`，吸收仅工作日规则并在周末回落默认倍率 / 标签；公开模型价格补充 token 阶梯，不恢复已经删除的旧 ChannelTimePricing 或旧模型广场组件。
- 新增迁移 `229_plugins.sql`、`230_plugin_artifacts.sql`，Go / CI 工具链更新到 1.27.0，前端依赖和安全 override 更新；`VERSION` 保持 `1.7.39`，上游 `0.1.180` 不进入 fork 发布线。
- 全量回归覆盖后端 unit / Testcontainers integration、vet、build、lint，前端 297 个测试文件 / 2032 条用例、typecheck、ESLint、生产构建，以及 docs、audit 和 Compose 合同；未推送、发布或部署。

- 同步上游 Chat / Responses 工具正确性修复：文件 part 不再静默丢失，输出上限截断出的非法 function arguments 不再被标记完成，DeepSeek 原生 Responses 可以正确承接 Codex 客户端工具。
- 修复 Responses WebSocket / HTTP bridge 重复 replay 与孤立 tool call；已有 call context 的后续轮次不重复补历史，未知上游结果继续禁止整轮重放。
- 稳定 OpenAI OAuth 图片生成：区分内容政策拒绝、普通文本 fallback 和空响应，并对工具不可用账号执行短期冷却与受控 failover。
- Guardian / review 自动审查优先复用父 thread 在当前分组内的账号，同时继续遵守隐私、能力、利润、sticky、预算和实际分组归因边界。
- 补齐 Ollama Cloud raw Chat Completions reasoning / `max_tokens` 兼容，并收紧 Google One OAuth 可发布模型目录。

## 2026-08-21

- 继续同步 `origin/main@67380eafd`：国产供应商账号测试按平台和显式协议选择 OpenAI、Responses 或原生 Anthropic 入口；DeepSeek 无效中继余额不再被解释为零余额，管理端余额 / 配额刷新改为明确的主动探测按钮。
- Composite 分组可显式开启 Messages dispatch，并能把视频生成请求路由到实际 Grok 候选；OpenAI family / model 详细映射仍只属于 OpenAI 分组，企业成员候选、最终 `ActiveGroup`、预算 / usage 原子归因和不明确结果禁止重放边界不变。
- OpenAI Chat sticky 只使用请求开头连续的 system / developer 前缀；空 `openai_capabilities` 恢复为未配置语义。前端同时修复 token refresh 锁循环，Home 的三个模型广场入口统一服从 feature flag 和登录要求。
- 本轮没有数据库迁移、配置或依赖变化；fork 版本继续保持 `1.7.37`。

- 继续同步 `origin/main@f646a1f97`：OpenAI-compatible Chat Completions / Responses 的 pool 账号在可重试状态且未被停调时恢复同账号重试；该路径继续服从既有账号级次数、sticky、企业成员候选、预算归因和未知结果不重放边界。
- Responses 转 Chat Completions 的流式 arguments-only delta 不再发送空工具名；Antigravity 改用官方 daily 域名，并只让 `pro` / `ultra` 账号默认走 daily，免费或未知 plan 保持生产端点。
- CN 额度探测测试 fake 消除并发 append race；nanoID 审计例外和支付集成文档链接同步修正。无数据库迁移、配置、依赖或前端运行时变化，fork 版本继续保持 `1.7.37`。

- 继续同步 `origin/main@9f74eb57f`：OpenAI Responses 增加输入 / terminal usage / tool schema 兼容、被拒字段有限同号重试、compact fallback、WebSocket 会话抢占与 API Key 健康熔断；Grok 默认模型和计费目录推进到 4.6，并收紧媒体、Realtime、stream idle、compaction 和容量重试语义。
- Ops 错误详情增加上游状态、根因优先级、失败尝试快照和诊断 payload 去重；后台 capture writer 使用 generation-bound lease，避免池化复用后的旧 writer 触达新请求，同时保留二开 route trace、外层 writer 生命周期和 stone 阅读型布局。
- WebSocket 继续锁定连接级公开模型、渠道 / 账号映射、平台、分组和账号；每轮分别保存完整映射链、预算和 usage。后续轮次未知传输结果仍进入 enterprise member budget ambiguous，对已可能发生的上游消费不做整连接重放。
- 国产供应商原生 Anthropic 路径补齐 `reasoning_effort`，`gpt-5.6-*` 保留 `max` effort；prompt guard 的 `config_loaded` 不再每 5 秒重复写日志，只在首次加载、配置变化或错误恢复时记录。
- 新增 `gateway.grok_response_header_timeout`（默认 120 秒）和默认关闭的 OpenAI pool API Key 跨实例健康熔断 setting；无数据库迁移，发布版本继续保持 `1.7.37`。

- 同步 `origin/main@2bc139ab5` 到正式线 `dev-zz`：国产供应商账号新增 `adaptive` API 协议与分协议 base URL，Composite 扩展到 Kimi / 智谱 / DeepSeek 及 Codex 控制入口；显式账号协议仍优先于异步 Responses 探测结果。
- OpenAI / Codex 吸收 WebSocket 后续 turn 恢复、当前 turn 安全 failover、客户端工具跨 turn 保留、input token 预检、请求级容量恢复、Chat 缓冲读取故障转移和 reasoning item 缓存回注；Grok 增加 tool-search discoveries 晋升、客户端 tool search 与内联图片工具适配。
- 渠道价格增加 Fast / Flex 服务层倍率和长上下文区间的输入、输出、缓存读写倍率；`dev-zz` 的 `time_pricing` 继续是唯一分时定价合同，保留 IANA 时区、自定义类型名称、跨午夜、`0x`、请求开始时结算和替代其它倍率的语义。账号统计成本入口不接受渠道倍率。
- Channel Monitor 收紧配额数据源与模式组合校验，配额占位模型统一本地化；usage 汇总改为单次 `GROUPING SETS` 扫描，并保留企业成员 / owner 过滤；本地模型配置错误从上游 SLA 归因中排除，但仍保留路由尝试证据。
- 新增可配置代理出口探测目标；迁移 `226_add_usage_log_effective_model_indexes_notx.sql`、`227_composite_routes_add_cn_providers.sql`、`228_channel_pricing_multipliers.sql` 按完整文件名追加。fork 版本继续保持 `1.7.37`，没有采用上游 `0.1.179`。

## 2026-08-18

- 同步 `origin/main@e0c48a19` 到正式线 `dev-zz`：Channel Monitor 增加配额模式、快照、远程账号搜索和多平台展示；吸收 OpenAI / Codex 客户端工具、WebSocket、fingerprint、批量账号设置与 Team 熔断修复，以及 Gemini / Antigravity、国产供应商、Grok 用量、邀请码原子兑换、Ops 和暗色界面的上游改进。
- 上游同期提供的简化渠道分时倍率没有作为第二套实现并存；`dev-zz` 继续以现有按量分时定价为唯一合同，保留自定义类型名称、IANA 时区、其余时段倍率、跨午夜、`0x`、请求开始时结算和“启用后替代其他倍率”的规则。客户目录仍只显示最终价格，不增加“当前有效倍率”标签。
- OpenAI Chat / Responses / Messages / WebSocket 在吸收 partial usage、终态事件和工具适配修复时，继续携带企业成员最终分组、渠道 usage、请求开始定价快照和交付元数据；模型状态用户路由继续使用授权分组过滤，不恢复上游旧用户监控端点。
- 完善按量模型分时定价：时区从自由文本改为可搜索、可保留自定义合法值的 IANA 选择器；“其余时段”可配置类型名称和倍率，允许没有显式窗口时作为全天规则使用；每条显式规则也由管理员独立配置类型名称和倍率，客户目录原样展示名称，绝不根据倍率数值擅自推断高峰、低谷或平时。启用后当前分时倍率直接替代分组默认、用户专属和旧 Group peak 倍率，报价与扣费不再隐藏叠加；关闭后原倍率链保持不变。客户模型卡片和价格表格只展示按当前规则算出的最终价格，不再额外显示“当前有效倍率”或分组倍率标签；时段详情与管理端规则仍保留倍率。旧 JSON 缺失 `default_multiplier` 时继续按 `1x` 兼容。

## 2026-08-17

- 新增按量模型分时定价：管理员可在 Group / Channel 的 token 模型价格条目中配置 IANA 时区、多条高峰/低谷或跨午夜窗口；标准分组直接可用，不再把能力绑定到订阅。最终命中的模型价格同时决定基础价和时段规则，模型规则与旧 Group peak 互斥，按次图片、视频、音频和搜索价格不受影响。
- 公开模型列表和登录后的可用模型广场新增实际时段价格表，明确显示其余时段、当前命中行、时区、输入/输出/缓存实际价格、零价与缺失价；分组模型价覆盖会按当前可见分组安全投影，用户专属倍率只影响本人目录。用量审计在既有 `schedule_meta` 中保存定价时刻、倍率、时区和规则快照。
- 新增迁移 `225_channel_model_time_pricing.sql`，只扩展渠道模型价格；Group 规则随既有 `groups.model_pricing` JSONB 保存，不新增订阅结构。
- 同步 `origin/main@e330c243a` 到正式线 `dev-zz`，吸收 Kimi、智谱和 DeepSeek 多协议国产供应商、余额 / 配额检测、分组日用量预聚合与昨日用量、Codex turn-state / fingerprint / compaction 修复，以及 OpenAI Fast / Flex 设置说明；fork 版本继续保持 `1.7.36`。
- 企业成员请求级 `ActiveGroup`、有序候选、预算结果不明确禁止换组、成功 usage / 预算 / 最终分组原子归因、sticky 与利润准入均保持不变；CN 平台进入现有协议选择和调度链，不能用账号探测覆盖显式模型交付决策。
- Anthropic 原生协议账号优先直达供应商原生端点；只支持 Chat Completions 的 OpenAI-compatible 账号继续经过 dev-zz 的 Messages 转换、reasoning / Fast 策略、prompt-cache 与计费模型解析后 fallback。
- 管理端账号页同时保留供应商成本绑定与 CN provider 模式、协议、base URL、余额和配额；分组页沿用 stone / neutral 视觉并新增昨日用量。
- 新增迁移 `222_group_usage_daily_rollups.sql`、`223_group_usage_rollup_timezone.sql` 和 `224_user_platform_quotas_add_cn_providers.sql`，按完整文件名追加且不改写历史迁移。Wire 从合并后的 provider graph 重新生成。

## 2026-08-13

- 修复 HTTP 200 流内 `response.failed` 未结束企业成员预算回执的问题：明确失败现在立即释放，结果 / 用量不明仍优先保持 `ambiguous`，cyber 拒绝继续按真实 token 计费；WebSocket `response_id` 的 Redis 粘性绑定也不再被已经结束的请求 context 直接取消，并继续受 3 秒超时约束。企业成员候选分组与账号调度逻辑未修改。
- 同步 `origin/main@fbfdcef8` 到正式线 `dev-zz`，吸收 Channel Monitor V2、Grok 视频 / Voice / Realtime / Web Search / X Search、账号调度阈值、上游响应模型审计、安全计费来源和逐模型 / 媒体 / 搜索价格；fork 版本继续保持 `1.7.32`。
- 企业成员有序分组、模型感知候选、Composite 每次候选重解析、预算结果不明确禁止重试、成功 usage / 预算 / 最终分组原子归因与共享模型目录均保持不变；新增 Grok 路由也进入同一解析、预算和候选编排链。
- 模型状态页增加 V1 / V2 模式外壳；默认 V1 继续展示 dev-zz 的分组模型自检、可选历史 fail-soft 和管理员 Token 用量，V2 仅显式启用。升级不会用上游旧探针页面覆盖现有模型状态能力。
- 用量与 Ops 增加 requested / sent / upstream response model 和 mismatch 审计；过滤、导出和详情继续保持 admin / owner / enterprise member 字段隔离。分组与渠道增加逐模型、视频、语音和搜索定价，但公开展示继续复用共享 available-channel catalog。
- Ent / Wire 由合并后的 schema 和 provider graph 重新生成；新增迁移按完整文件名并存，不改写已应用 SQL。合并验证重点覆盖企业成员路由 / 预算、usage SQL、迁移、模型状态、设置和兼容层。

## 2026-08-12

- 加固 v1.7.29 事故后的预算失败归因：企业成员预算预留的事务启动、已有预留查询、消费限额预留、新记录写入、提交和死锁重试退出点现在携带稳定阶段标签并保留原始 error chain / PostgreSQL SQLSTATE；预算超限、冲突、成员不存在等既有领域错误继续保持原来的 400 / 404 / 409 / 429 分类。
- 用户“模型状态”页面对可选历史、时间线或快照存储故障改为 fail-soft：授权分组查询和当前状态主数据仍 fail-closed，只有可选可用率 / 时间线降级为空，页面继续展示已取得的当前状态。前端同时丢弃畸形列表行、拒绝畸形详情并取消过期详情请求，避免坏数据或请求竞态拖垮整页。
- 修复 `Security Scan` 报告的 `nanoid` 高危公告 `GHSA-2v37-7h3g-55p8`：仅把 PostCSS 允许范围内的传递锁定版本从 `3.3.16` 提升到 `3.3.17`，不增加 audit 例外、不新增直接依赖。修复后 frozen-lockfile 安装和仓库 audit exception 校验通过。
- 修复 v1.7.29 企业成员 admission runtime 缓存命中前仍执行 readiness 数据库聚合的问题：热命中不再读 readiness 证据，冷缓存重建继续由 singleflight 合并，避免模型路由前置查询争抢连接池并把预算事务启动失败放大为平台 500。
- v1.7.29 故障后把 `gateway.enterprise_member_model_admission_mode`、新安装、空值、非法值与设置数据库故障的默认路径回退到 `legacy_order_only`；shadow / enforce 仍可显式开启，但升级默认不再执行预算前账号 / 分组 / 协议能力投影或 readiness 聚合，legacy WARN 也按进程限为每分钟最多一条。
- 管理端保存 legacy / shadow 设置时跳过仅 enforce 校验需要的 readiness 聚合；显式保存 enforce 时仍完整执行 readiness、rollout 与 auto-stop gate。
- readiness 评估现在复用同一份 admission evidence summary 计算 auto-stop，不再重复执行相同的 30 天聚合证据查询；evidence 不可用仍 fail-closed 到 `shadow_published`，rollout、预算原子结算和最终分组归因语义不变。
- 新增调用次数回归，修复前单次冷缓存 / 热缓存 / 32 并发热命中为 `2 / 1 / 32`，修复后为 `1 / 0 / 0`，并验证 32 个并发冷 miss 也只重建 1 次；同一次 readiness 的 evidence repository 调用由 2 次降为 1 次。本轮本地全量验证包含后端测试、vet、golangci-lint，前端 261 个测试文件 / 1764 条测试、typecheck、lint、build，以及依赖审计；事故中预算底层失败与模型状态页最初故障的生产根因仍未被证明消失，正式候选仍需新提交对应的 hosted CI / Security Scan / Branch Images 全绿。

## 2026-08-05

- 企业成员模型感知路由治理阶段 0-4 在本地实现并通过最终本地验证：精确发布模型规划、routing eligibility revision/outbox/PubSub/atomic mirror、generation LKG、Composite 文本预览、非文本 evaluator coverage、typed attempt、Ops routing attempts、alias review ledger、readiness/rollout/auto-stop 管理能力和 legacy 退役准备已进入当前工作区。
- 最终本地验证已通过：后端 `go generate`、`make test-unit`、`go test ./... -count=1`、`go vet ./...`、`golangci-lint run ./...`（0 issues），18 个 Colima/Testcontainers PostgreSQL 16/Redis 顶层测试（14 个 routing eligibility + 4 个 migration runner），focused race，前端 typecheck/lint/完整测试（259 个测试文件、1758 条测试）/build，docs build，`git diff --check`，隐私/脱敏专项测试及新增差异的高置信度密钥模式扫描。
- 新增迁移 `199_routing_eligibility_revision.sql`、`200_enterprise_member_alias_review_ledger.sql`、`201_ops_routing_attempts.sql`、`201a_ops_routing_attempts_indexes_notx.sql`、`202_account_model_protocol_capabilities_non_text.sql`。这些迁移支撑本地治理代码和管理证据，但不代表生产数据库已应用、生产 enforce 已灰度或新安装默认值已切到 enforce；`201` 历史 CHECK 验证和 `201a` 生产同量级并发索引演练仍是发布门。
- `gateway.enterprise_member_model_admission_mode` / DB setting 继续默认 `shadow_published`；`enforce_published` 仍必须通过服务端 readiness、rollout、auto-stop、alias/evidence 门和真实生产发布窗口。当前尚未 commit、release、deploy，也尚未取得生产 7d/30d/canary release-window 证据；阶段 5 只完成本地退役准备，shadow 双算和旧候选分支不得在生产窗口验证前删除。
- 同步上游 `main`（`00b859617`）到 `dev-zz-develop`：认证验证码扩展为互斥的 Cloudflare Turnstile、腾讯天御与阿里云验证码 2.0，认证动作、OAuth 登录启动和待建账号使用统一 proof；管理端 secret 继续只暴露 configured 状态，局部保存审计不会继承已存凭据。
- Codex OAuth 出站身份统一由生效版本重建，默认自动同步官方最新稳定版并允许管理员覆写；同时吸收 WebSocket terminal event、Responses 提示词审计解析、订阅续费串行化、消费排行用户名、Anthropic OAuth endpoint 和 Grok CLI 版本修复。
- 模型广场图片报价修复迁入 dev-zz 共享目录：分组图片档位按“分组价 > 渠道同档价 > 渠道默认按次价”回落，图片独立倍率替代普通分组 / 用户倍率；模型广场、价格表格与导出使用同一实现，不恢复已删除的旧 `channel_plaza` 和价格组件，也不把图片输出 Token 单价当作单张图片价格。
- 登录 / 注册页继续隐藏 LinuxDo 与微信入口，但保留后端、回调和共享 OAuth 组件能力。无数据库迁移；新增腾讯 / 阿里验证码 SDK 依赖与 CSP 来源；`VERSION` 保持 `1.7.27`，上游 `0.1.171` 不进入 fork 发布线。

## 2026-08-03

- 可用渠道“价格表格”和 Excel 不再直接展示未乘倍率的渠道基础价；报价按可调用计价分组拆行，使用与模型广场相同的“用户专属倍率优先、否则分组默认倍率”计算客户生效价，价格排序同步按生效价执行。模型广场、表格与导出统一新旧响应的可调用分组判断；管理员全量目录为每个模型返回权威 `route_group_ids`（无路由时为 `[]`），只对确有可调用路由的分组使用默认倍率生成报价。
- 同步上游 `main`（`825ca7b1f`）到 `dev-zz-develop`：新增分组级利润准入与预览能力，按客户倍率、账号成本倍率、最低利润率和安全缓冲过滤候选；普通、OpenAI、图片、WebSocket 和 failover 路径共用 gate，并新增迁移 `192_group_profit_control.sql`、`193_group_profit_control_auth_cache_invalidation.sql`。
- 上游账单探测扩展到多 API Key 平台，可在显式开关、倍率上限与可信探测约束下同步账号倍率；账号编辑、批量探测与列表刷新使用同一口径。认证刷新、退款 / Stripe、SMTP、内容审核代理和窄范围提示词阻断同步增强。
- OpenAI 吸收 reset-credit 缓存 / 恢复、Messages 临时错误切换、SSE `429`、取消、负载削峰、namespace 与工具输出媒体修复；Anthropic 中断流会保留已观察 usage。企业成员预算、usage 与最终实际分组继续原子结算，事务失败不会退化为独立 usage 写入。
- 管理端账号支持按完整筛选结果全选并继续作用于 dev-zz 批量编辑、探测和归档，不恢复页面硬删除；Home 增加 compact preset。公开模型列表继续使用共享 marketplace、Select 筛选和 stone / neutral / emerald 视觉，不恢复已删除旧价格组件或倍率筛选。
- 认证快照提升到 `v21` 并合并利润、计价、企业成员与 Live 字段；`VERSION` 保持 `1.7.25`，上游版本 `0.1.170` 不进入 fork 发布线。本轮没有新增依赖。

## 2026-07-31

- 同步上游 `main`（`d29acc29a`）到 `dev-zz-develop`：Responses / Gemini 上游路径增加安全校验；OpenAI 代理断流熔断在全部候选被隔离时 fail-open，Pool 模式补齐流式容量重试，Grok billing ping 统一为 SSE 注释并避免 pool 模式 entitlement `403` 误冷却。
- 网关继续保留 dev-zz 企业成员候选编排、预算门禁、最终实际分组归因和 Responses WebSocket 首轮路由锁。Responses 子路径安全守卫会先于候选调度拒绝非法路径；代理 fail-open 只放宽隔离偏好，不跳过渠道定价限制或 sticky 路由合同。
- 订阅日 / 周 / 月额度窗口改为服从订阅真实开始与到期边界，前后端重置时间和剩余标签保持同一口径。支付设置恢复 patch 语义，未提交字段不会清空可见支付方式；支付选择器与长套餐标题修复窄屏溢出。
- 可用渠道目录把 Composite 分组展开到实际配置模型的平台 section，同时继续使用 dev-zz 的稳定路由和协议能力投影；另同步标准 SMTP 格式、图片 data URL 转存、GPT-5.6 Luna / Terra 与 GLM-5.2 价格、Composite 模型展示及结构化清理日志。
- Docker / Compose 增加 `no-new-privileges`，CI 加入部署安全合同脚本。本轮没有数据库迁移或依赖变更；`VERSION` 保持 `1.7.25`，Ops 自动清理默认关闭和长期数据保留合同不变。

## 2026-07-30

- `/model-plaza` 收敛为注册前可访问的公开模型列表：只返回 active、standard、非专属分组，匿名与登录请求内容一致；专属分组、订阅分组和用户专属倍率继续只属于登录后的 `/available-channels`。
- 公开模型列表与用户可用渠道复用同一稳定交付投影，模型分组归属、公共 API 端点和多账号协议能力不再维护两套判断。页面复用现有模型卡片，按公开分组默认倍率展示客户价格，并支持模型搜索、平台/分组下拉筛选和失败重试；公开页视觉统一为 stone / neutral / emerald，桌面筛选栏保持单行紧凑布局。
- 公开模型列表右上角改为返回型导航：登录用户返回对应控制台，未登录访客返回首页，不再把该位置作为登录入口。
- Home 公共导航与 Models 区域在功能启用时链接 `/model-plaza`，功能关闭时不展示入口；保留 `model_plaza_require_auth` 兼容门禁，但登录 token 不改变目录内容。本轮没有数据库迁移、运行时转发或计费逻辑变化。
- 控制台顶栏不再重复展示公开“模型列表”入口；登录用户通过“可用渠道”查看自身完整模型、专属分组和实际倍率，直接访问 `/model-plaza` 及其“返回控制台”导航仍然保留。
- 同步上游 `main`（`5a6143097`）到 `dev-zz`：OpenAI Live observer 在 Redis / store 抖动时有限重试，持续失败后按会话到期时间兜底 finalize，不再因为 controller 或 call record 读取异常静默丢失租约释放与 usage 证据。
- Live usage 改为 best-effort 队列优先、同步写入回退；生产 Redis call record 同时补齐企业成员 ID、成员编号 / 名称快照的保存与恢复，usage 继续保留这些成员证据、最终实际分组和 call hash request ID，最终写入失败保留 dev-zz 脱敏结构化事件。
- 管理端账号状态补充 Claude Sonnet 5 短别名；Passkey 功能关闭时资料页跳过凭据查询，设置切换竞态返回禁用错误时不再显示误导性加载失败。
- 本轮没有迁移、依赖、接口或配置变化；`VERSION` 保持 `1.7.23`，企业成员 Live 身份、调度、计费和分组 gate 合同不变。

## 2026-07-28

- 同步上游 `main`（`8fd01c281`）到 `dev-zz`：新增默认关闭的 Passkey 登录与管理能力，以及默认关闭、可选强制登录的 `/model-plaza` 分组价格橱窗。价格橱窗不会替换 `/available-channels`；后者仍按 dev-zz 的真实账号路由和端点能力展示可用模型。
- User / API Key 更新改为显式字段集合，避免管理员资料保存、余额变更、额度重置、状态切换、标签和企业能力并发修改时互相覆盖；合并继续保留 `account_type`、企业停用标记、API Key 标签和企业成员归属隔离。
- OpenAI Messages 桥接补齐 GPT-5.6 `max` reasoning effort，另同步 Kimi K3 / 1M 后缀、Codex Web Search、Anthropic cache breakpoint、安全审计配置恢复、setup bypass 和模型 ID 复制修复。
- 新增 `191_passkey_credentials.sql`，与既有同号迁移按完整文件名并存；版本保持 `1.7.21`，企业成员路由 / 预算 / 归因、模型多协议调度、Messages 显式映射和模型状态授权不变。
- OpenAI 分组的 Messages 模型名处理改为显式策略：新分组默认原样透传，管理员可选择自定义 Opus / Sonnet / Haiku 系列映射，且单个系列留空时继续透传；精确模型覆盖始终优先。旧分组缺少策略字段时继续沿用原 GPT 默认映射，避免升级后静默改变存量流量。
- 分组编辑页的目标模型输入会建议当前分组可调度 OpenAI 账号 `model_mapping` 中的具体逻辑模型，并对不在候选范围内的手填值给出提示；不再把平台全局模型目录当作该分组的目标模型。新增候选接口，无数据库迁移。
- 收紧账号“上游模型协议能力”的模型范围：账号配置了 `model_mapping` 时，同步、接口响应和弹窗只处理映射右侧的最终上游模型，不再把同一上游 `/v1/models` 中未参与该账号路由的模型混入能力矩阵；空映射账号继续保持全模型透传语义。
- 协议能力保存成功后现在会立即关闭弹窗。映射受限账号不再显示“手工添加任意上游模型”入口；已存在的无关历史观察不删除、不参与当前配置视图或覆盖保存，避免修复过程破坏审计数据。
- 收紧用户侧模型状态的分组可见性：`GET /api/v1/model-status` 与详情接口现在从认证身份读取当前用户，并复用 API Key 可用分组口径，只返回公开标准分组、已授权专属分组和有效订阅分组；未授权分组详情按不存在处理。
- 授权分组查询失败时模型状态接口 fail-closed，不再回退到全站状态。站点自检 runner 仍按全部启用目标生成后台快照，不改变探测范围、历史数据或管理员上游监控。
- 补充列表过滤、详情越权、授权失败和未认证请求回归测试；后端全量测试、CI unit-tag 测试、定向 `go vet` 和 golangci-lint 2.9 均通过。本轮无数据库迁移和前端代码变化。

## 2026-07-27

- 同步上游 `main`（`dc893dd0b`）到 `dev-zz-develop`：新增可由管理员热配置的面板 API 限流，认证接口按用户 ID、重查询接口按更严格档位、公开设置接口按安全客户端 IP 计数，降低高频面板请求拖垮数据库的风险。
- 面板限流默认启用，默认每用户 240 RPM、重查询 60 RPM、公开 IP 300 RPM，并默认豁免管理员；阈值设为 `0` 可单独关闭对应档位。Redis 异常时 fail-open，运行配置使用 60 秒进程缓存，当前节点保存后立即生效。
- 合并继续保留 dev-zz 企业成员预算服务、API Key 自助查询、模型级限流和 owner 用量分析；Key 日/趋势/模型统计及 `/usage` 聚合查询统一叠加 Heavy 限流，不把新增分析端点留在保护范围之外。
- 管理端“系统设置”同时展示模型级限流与面板 API 限流，沿用 dev-zz 现有设置保存合同和 stone / neutral / emerald 视觉。本轮没有数据库迁移、依赖声明、版本号或 GitHub Actions workflow 变化；上游同时更新了 README 赞助商列表和对应静态资源。
- 同步上游 `main`（`d96b6a31f`）到 `dev-zz-develop`：补齐 Antigravity OAuth 账号对 OpenAI Chat Completions / Responses 请求的原生 `v1internal:streamGenerateContent` 兼容转发，并吸收 Gemini Hermes Web Search 判定、分组说明排版和通用下拉框视口边界修复。
- Antigravity 兼容层会在请求进入原生 Gemini 链路前转换 OpenAI 请求，在流式与非流式响应返回时恢复 OpenAI 语义；仅收到 usage、没有任何可交付内容的响应会按失败处理并进入既有账号切换机制，组内账号耗尽且响应尚未提交时会继续尝试企业成员的下一候选组，避免把空结果或单组容量问题直接返回给用户。
- 账号失败与冷却继续使用真正尝试过的上游 endpoint 作为证据；凭据拒绝信息保持脱敏。网关仍通过 dev-zz 的 `DeliveryDecision` 选择最终上游协议，企业成员路由、预算和最终分组归因边界不变。
- Gemini Messages 兼容只把显式声明为服务端 Google Search 的工具转换为 `googleSearch`；Hermes 风格的普通 `web_search` function 继续作为客户端函数保留，避免工具定义被误吞。
- 分组说明支持换行、长单词断行和三行截断；通用 Select 会根据视口左右边界收缩并夹紧下拉层，同时保留捕获阶段 outside-click 监听。本轮没有数据库迁移、依赖声明、版本号或 CI workflow 变化。
- 同步上游 `main`（`95590b553`）到 `dev-zz-develop`：吸收系统设置局部更新保护、显式 `CONFIG_FILE` 路径、管理员用量 `request_id` 筛选、Responses / Anthropic 兼容修复、最终上游模型统计、支付统计分币种和监控时间线窄卡片溢出修复。
- 设置保存继续保留 dev-zz OpenAI Fast/Flex 策略的原子校验、序列化和审计；局部 payload 没有携带的设置键不会被零值覆盖。用量筛选同时保留企业成员 owner 可见性边界和新增请求 ID 精确查询。
- OpenAI Responses failover 同时保留模型协议能力选择和上游 reasoning 透传边界；跨账号切换前会基于当前账号派生请求体，避免不兼容 reasoning 形态污染后续账号。
- 支付后台看板按币种展示收入、支付方式和用户排行；前端继续使用 dev-zz stone / neutral / emerald 视觉。用户模型状态时间线保留自定义 tooltip 与无障碍语义，同时在窄卡片中收敛宽度。
- GitHub Actions 保持 Node 20，并统一固定到 pnpm 10.34.5：既兼容当前 `pnpm-workspace.yaml` overrides / lockfile，又避免 pnpm 11.17 对 Node 22.13 以上版本的运行时要求。
- 管理员渠道编辑新增“模型映射 → 模型定价”覆盖核对：按计费基准区分请求模型和渠道映射后模型，展示已覆盖、缺失和额外定价。填写或修改映射不会自动生成定价；管理员确认映射后可逐条补定价，或显式点击“快速定价”补齐缺失项。开启“限制模型”时，漏配会在保存前阻止提交，未限制时只提示而不删除合法的独立定价。
- 模型映射与模型定价均支持拖动调整显示顺序，也可按模型名称或映射顺序整理。定价 `sort_order` 明确为平台内展示顺序，只影响管理端展示，不改变通配符匹配优先级；新增迁移 `198_channel_pricing_display_order.sql` 持久化映射顺序和定价顺序，旧渠道按既有行 ID 与自然模型名获得确定顺序。
- 同步上游 `main`（`eb6e3d1f1`）到 `dev-zz-develop`：补齐 OpenAI Responses WebSocket 逐 turn 模型、渠道映射、计费和调度证据，并吸收提示词审计配置可用性、Grok `402` 冷却、管理员用量筛选、注册返佣码与 Caddy SSE 修复。
- WebSocket 继续遵守 dev-zz 首轮路由锁：连接内可以省略或重复同一公共模型，但切换模型、平台或渠道目标必须重连。企业成员逐 turn 预算、结果不明保护、同步 usage 落账和最终实际分组归因保持不变。
- 管理员通过带 `user_id` 的用量链接进入时会看到对应用户标签；迟到标签查询不会覆盖新的搜索输入。返佣开启且强制邀请码关闭时，注册页会展示可选邀请码，并继续使用 stone / emerald 视觉。
- 提示词审计没有可信运行配置时明确返回不可用，不再展示可能误导管理员的默认值；Grok 手工测试收到 `402` 会临时暂停账号。
- Caddy 压缩明确排除 SSE；旧 `AvailableChannelsTable.vue` 继续保持删除，用户模型广场仍是 dev-zz 的可用模型入口。本轮无数据库迁移和版本提升，版本保持 `1.7.18`。

## 2026-07-26

- 同步上游 `main`（`2730c1c43`）到 `dev-zz-develop`：新增 OpenAI Live HTTP / sideband 网关、macOS attestation 和分组级 `allow_live`，并吸收 Session ID 请求证据、注册邮箱别名安全、Ollama Cloud 刷新、公告预览、postcss 安全升级及 OpenAI / Grok / Gemini 正确性修复。
- Live 默认关闭，只允许管理员在 OpenAI 分组显式开启；管理端会先探测当前服务端 attestation 能力，unsupported 环境仍需二次确认。既有 Responses、Alpha Search、WebSocket、Composite 和企业成员候选编排保持不变。
- 企业成员与 Composite Live create 复用有序候选编排，只在最终解析到 OpenAI 且 `allow_live=true` 时创建；sideband 绑定原 call 身份。Live 最终 usage 保留成员与实际分组证据，仓储失败会记录结构化错误而不是静默忽略。
- usage 与 batch image 新增经过清洗的显式客户端 `session_id`；该字段只用于当前 owner / 管理员请求证据关联，不改变 sticky、调度、计费、request ID 或 prompt cache。企业成员同步落账、预算结果不明、最终 `ActiveGroup` 与成员快照继续作为权威归因。
- 公告管理新增预览动作，Bell 与 Popup 共用 Markdown 样式；功能修复合流后继续使用 dev-zz stone / neutral / emerald 主题。
- 新增同号迁移 `187_add_usage_log_session_id.sql`、`188_allow_live_usage_request_type.sql`、`189_add_group_allow_live.sql`、`190_add_users_email_alias_dedup_index_notx.sql`；与既有企业成员迁移按完整文件名并存，不修改历史迁移。版本保持 `1.7.17`。

## 2026-07-23

- 再次同步上游 `main`（`cd8bb98c4`）到 `dev-zz-develop`：新增 Ollama Cloud 官方用量观察与定时刷新、支付宝官方移动端当面付唤起，并吸收 OpenAI passthrough / 流式隔离、模型限流、渠道定价名称、Codex identity 导入和 Grok 调度修复。
- Ollama Cloud 用量只作为管理员观察：支持符合条件的 OpenAI / Anthropic API Key 账号，展示 5 小时、7 天、余额和模型请求窗口；Web session 使用固定 `TOTP_ENCRYPTION_KEY` 加密保存，不进入用户 DTO、审计明文、账号健康、计费或调度。
- 支付宝移动端当面付唤起默认关闭。启用后服务端使用 `alipay.trade.precreate` 获取动态二维码，前端先尝试唤起支付宝 App，页面未离开时回退到二维码；桌面端和未启用时的既有流程不变。
- 新增 `186_alipay_mobile_precreate_deep_link.sql` 和 `186_group_auth_cache_image_generation.sql`；与 dev-zz 既有同号迁移按完整文件名并存，不改写历史迁移。
- 同步上游 `main`（`ba88cc239`）到 `dev-zz-develop`：接入 Composite 分组 / 模型路由注册表、分组级 reasoning effort 映射与上限、Grok / OpenAI Responses 与 WebSocket 正确性、系统更新长请求和响应式管理端布局；继续保留企业成员路由 / 预算 / 归因、模型原生多协议、供应商成本、stone / emerald 视觉、fork 镜像和 `1.7.16` 版本线。
- Composite 与企业成员组合改为“每个候选组独立解析”：候选切换会清除上一组路由决策并从原始公开模型重新解析，HTTP 模型改写不会串组；Responses WebSocket 首帧支持显式别名，切组后同样重新解析。首 turn 未产生下游事件时可以安全 failover，后续 turn 结果不明时继续保留预算并禁止重放。
- 模型目录合并企业成员跨组并集、Composite 可调度模型和原生协议元数据；Ops 记录使用最终具体平台归因。Ent、Wire 和 pnpm lock 均按合并后的 schema / provider graph / 依赖声明重新生成。
- Composite 精确公开别名会进入模型目录并携带端点能力，前缀和禁用规则不会误发布；Gemini 模型目录会跳过不匹配的 Composite 候选。Ops 恢复记录固定归因到真实失败 attempt，不会被最终成功分组覆盖。
- Responses WebSocket 在首 turn 固定公开模型到最终上游模型的映射，连接内切换模型或平台会要求重新建连；只有首 turn 可以做整连接 failover，后续 turn 的 429 / 未知结果禁止重放。成员预算延后到最终分组和账号稳定后预留，任何不明确预算结果都会阻止跨组重试。
- 多协议链路收敛为统一 `DeliveryDecision`：渠道定价只负责发布和计费，账号能力只记录最终上游模型协议事实，渠道页改为只读“API 端点就绪度”；模型目录与 Chat、Responses、Messages 运行时共享候选判定。`openai_responses_mode` 仅选择 Chat / Responses 上游传输，不再要求统一设为 `auto`，也不会错误禁用独立的原生 Messages 能力。
- 管理端“系统设置 → 网关 → 请求转发行为”新增“模型多 API 端点路由”全局开关；后台设置可在不重启服务的情况下统一控制用户模型广场端点发布与真实原生协议调度，未保存后台覆盖时继续继承 `gateway.native_model_protocol_routing_enabled` 部署默认值。
- 账号“模型与协议能力”弹窗会明确显示全局路由是否启用。关闭状态下仍可同步、查看和保存协议能力，但这些能力不会追加到用户目录或参与原生调度，避免管理员把“能力已配置”误解为“路由已生效”。

## 2026-07-22

- 用户侧“可用渠道”目录改为只展示存在稳定账号交付路由的“分组 + 模型”组合，并按分组分区展示；模型是否仍可调用由 `route_group_ids` 表达，已经确认可公开的 API 端点由更严格的 `supported_endpoints[].group_ids` 表达。能力证据暂时未知时不会误删仍可走存量兼容合同的模型，也不会凭空发布未经证明的新端点；同一分组内的多渠道继续合并，不同分组不再混用渠道、报价或协议能力。
- 这是一次用户可见的目录口径收紧：升级后，如果某个已配置渠道报价的模型只关联不可调度、停用或不支持该模型的账号，它会从用户模型广场消失。管理员可在渠道定价的模型交付详情中查看“无可用路由 / 无可用端点”诊断；这类消失表示当前不可稳定交付，不代表渠道定价数据被删除。

## 2026-07-20

- 修复供应商账号综合折扣固定使用美元换汇口径的问题：账号成本绑定显式记录上游分组价目表是 `CNY` 还是 `USD`；人民币价分组直接叠加资金池人民币成本与分组倍率，美元价分组继续除以参考汇率。充值比例 `1:1`、Kimi 分组倍率 `0.8` 时现在正确显示 `8.0 折`。
- 历史绑定保留旧美元公式但标记为“待确认”，在管理员明确选择计价基准前不参与列表成本排序或 `cost_first` 调度；旧客户端更新同一资金池且省略新字段时继续保留当前状态，避免无关编辑静默改写成本语义。
- 新增 migration `196_upstream_binding_price_reference_currency.sql`，只增加计价币种和确认状态，不根据供应商、模型或分组名称推断历史事实；供应商只有一个 active 资金池时，账号列表不再重复展示“主余额池”。
- 同步上游 `main`（`bfabfe60c`）到 `dev-zz`：新增入口拒绝聚合与鉴权缓存失效 outbox、可信代理/客户端 IP 请求头设置、异步图片对象存储热配置、Grok 受保护视频内容代理，以及上游有效倍率/峰值倍率排序。
- 合并继续保留企业成员有序分组、预算/归因和 owner/admin 数据隔离：普通 Key 的 OpenAI 首输出超时可在尚未产生语义输出时切换账号，企业成员已有预算 receipt 时则停止重放并进入结果不明闭环，避免重复副作用或重复计费。
- 运维错误明细不再为已删除 Key 保存或查询明文归属；无效鉴权流量进入低基数聚合入口，owner 只能读取当前用户及未永久移除成员的记录，管理员审计查询不受影响。
- `VERSION` 保持 `1.7.13`，Compose 继续使用 `thornboo/sub2api:latest`；上游新增迁移按完整文件名追加，没有改写已应用迁移。
- 发布 `v1.7.11`：修复企业成员管理中复制成员 Key 固定返回 `API key not found` 的问题。成员 Key 列表继续只返回脱敏值，owner 主动复制时改走企业成员专用按需读取接口，不放宽普通 `/keys/:id` 对成员 Key 的隔离。
- 专用接口在应用服务内同时校验企业 owner、当前成员、Key ID、成员归属和未删除状态，并在返回明文前写入不含凭据的 append-only `member_key.reveal_authorized` 审计；审计缺失或写入失败时 fail closed。
- 明文响应只包含 `id`、`member_id` 和 `key`，设置 `Cache-Control: no-store` / `Pragma: no-cache`；前端冻结请求成员与 Key 身份，切换成员时丢弃迟到响应，并同时校验响应中的成员 ID 和 Key ID。
- 已归档成员不显示复制入口，服务端继续拒绝归档成员和已删除 Key；中英文错误及审计动作均使用稳定本地化文案。

## 2026-07-19

- 发布 `v1.7.10` Key 自助查询：企业成员及普通 Key 持有者无需站点登录即可通过短时 `HttpOnly` 会话查询当前 Key 额度、成员共享预算、可访问分组/模型、统计、成功/失败记录、详情和 CSV 导出。
- 完整 Key 仅用于一次性会话交换并立即从页面状态清除；Redis 只保存令牌哈希与最小身份快照，所有数据读取强制 owner + API Key 双重归属，公开 DTO 不返回其他 Key、上游账号、管理员成本或内部调度字段。
- 查询页面增加跨会话 epoch、请求取消和退出过渡状态，阻止迟到摘要/详情回填旧数据，以及旧 DELETE 响应清除下一把 Key 会话 Cookie；服务端撤销失败会明确告警。
- Key 静态状态补齐 owner、企业成员、有效分组和独占分组授权判断；错误导出按实际页大小分页至 5,000 行，读取路由共享单 IP 60 次/分钟限流，新增 migration `194_ops_error_logs_api_key_time_index_notx.sql`。

## 2026-07-18

- 发布 `v1.7.9`：在保持企业成员预算、路由、归因、Ops 分类 v2、fork 镜像和默认 Rollup chunk graph 的前提下，同步上游提示词审计、安全开关、Grok 媒体资格与协议正确性修复。
- 同步上游 `main`（`b1a6b8026`）到正式 `dev-zz`：新增独立提示词输入审计控制台、OpenAI 兼容 Guard 节点配置/探测、异步审计与可选阻断、事件筛选/详情/确认删除，以及对应任务/事件迁移；安全审计功能默认关闭。
- `step_up_enabled` 与 `session_binding_enabled` 明确默认关闭；Grok 媒体调度新增账号资格覆盖并保持异步任务固定原账号，显式图片意图、alpha/search APIKey 调度、统一客户端 IP 和 Stripe 按需加载等正确性修复同步合入。
- 合并继续保留企业成员路由/预算/归因、Ops 分类 v2、fork 镜像、长期数据保留和默认 Rollup chunk graph；`181/182` 提示词审计迁移与既有同号迁移按完整文件名并存，发布版本提升为 `1.7.9`。
- 运维监控引入失败分类 v2，把“客户是否看到失败”“失败发生在哪里”和“是否影响平台 SLA”拆成独立字段；平台路由容量、最终上游失败、企业策略、客户账户/权限、客户端中断与 recovered upstream attempt 不再由单一 `is_business_limited` 混合表达。
- 总览升级为客户可见失败、平台可用性、SLA 排除和未分类证据四组可对账指标，并展示账户/企业、平台路由、平台内部、最终上游和客户端中断的归因分布；每个数字以总览响应的绝对时间快照进入同口径结构化明细。
- 最近 15 分钟当前状态按管理员已有的平台 SLA 失败率阈值判断“故障持续中 / 当前已恢复 / 当前无平台故障 / 状态无法判断”，历史自定义窗口不会伪装成实时恢复状态。
- raw、hourly/daily preagg、趋势、分布、metrics collector、健康评分、告警和定时报表统一使用分类 v2；旧 overview 字段继续返回兼容值，未分类证据会限制健康评分，不能静默显示为健康。
- 新增迁移 `192_ops_failure_classification_v2.sql` 和 `193_ops_failure_classification_v2_indexes_notx.sql`，执行 31 天确定性回填、预聚合扩列和并发索引创建；主要故障事件聚合及 HTTP 200 后流式终态去重仍留待阶段 5。

## 2026-07-17

- 优化企业成员“预算与用量”弹窗：一级信息收敛为月预算、本月已用和剩余预算；处理中预占仅在存在时解释展示，本月请求数、Token 和导入历史用量改为折叠明细，未配置的短周期限额不再占用页面空间。
- 小额预算使用率不再被四舍五入为 `0%`；不可变调账入口改为高级折叠操作和统一确认对话框，确认前冻结成员、金额与说明且不写账本；发布版本提升为 `v1.7.8`。
- 修复企业“成员使用记录”的模型分布错误复用账户全局统计的问题：模型统计统一执行完整 `UsageLogFilters`，不再因 Repository 能力回退而丢失 `member_id`、成员范围、模型或计费模式等筛选条件。
- 前端模型统计参数直接从公共用量查询参数派生，并新增已分配、未分配、指定成员和外部成员隔离的 API / handler 回归测试；发布版本提升为 `v1.7.7`。
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
