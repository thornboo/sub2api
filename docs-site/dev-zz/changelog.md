# 变更记录

## 2026-07-10

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
