# dev-zz 变更地图

这页按 `origin/main...dev-zz` 的真实差异整理正式二开范围。最近一次盘点基于 2026-09-04 将 `main@b1748c4ea` 合入 `dev-zz` 后的工作树：

| 项 | 值 |
| --- | --- |
| dev-zz | 本次合并提交及验证修复（合并前 `dfc1265f9`） |
| origin/dev-zz | `dfc1265f9` |
| origin/main | `b1748c4ea` |
| merge-base | `b1748c4ea`（本次合并完成后） |
| 差异规模 | 1166 个文件，约 201266 行新增、15086 行删除 |

## 变更分布

| 区域 | 文件数 | 说明 |
| --- | ---: | --- |
| `frontend/` | 360 | 用户/API Key、企业成员、owner 用量分析、管理员用量下钻、可用渠道模型、CN provider、V1/V2 模型状态、运维弹窗栈、主题与控制台 UI |
| `backend/` | 722 | 企业成员、模型/网关路由、API Key、用量/计费、分组日 rollup、CN provider、监控、Grok、安全、配置、测试、生成物与迁移 |
| `docs-site/` | 54 | dev-zz 文档中心、功能文档、部署/开发/维护记录 |
| `docs/` | 2 | 上游通用协议与 Channel Monitor V2 安全默认值文档 |
| `deploy/` | 14 | fork 镜像默认值、源码构建脚本、备份脚本、Compose/安装脚本与部署样例 |
| `.github/` | 5 | CI、release、security scan 和分支镜像的 Node 24 actions runtime 验证 |
| 根目录 / README / Dockerfile | 9 | release 镜像、项目说明、分布式 Dockerfile 与设计索引 |

## 已落地功能

### API Key 批量与标签管理

- 批量、标签和单 Key 分析是已落地的普通 Key 能力；“Key 即企业成员”的临时领域结论已由 ADR-0003 取代。
- 用户侧 Key 支持结构化 `tags`，新增 `jsonb` 字段和 GIN 索引。
- 创建、编辑、列表筛选、批量创建、批量更新、批量删除均支持标签。
- 批量创建支持模板名或名称列表；响应首次展示明文 Key，幂等重放会脱敏。
- 批量更新和删除支持 `apply_to=selected` 与 `apply_to=filtered` 两种目标。
- 筛选批量目标支持 `search`、`status`、`group_id`、`tags`，且要求至少一个筛选条件，单次最多 500 把 Key。
- Key 状态以 `active` / `disabled` 为可写状态，`inactive` 仅作为旧别名归一化到 `disabled`；`quota_exhausted`、`expired` 是系统状态，不作为普通保存状态写入。
- 编辑 Key 时，未改变 `group_id` 不会重新触发分组绑定授权，避免“只改标签”被历史不可绑定分组阻断。

### 企业成员完整目标架构（实现中）

- 企业能力使用独立 `account_type`，不把 `enterprise` 混入 admin/user 授权角色。
- 引入不可登录的稳定成员实体；成员聚合多把 Key，并继承有序分组集合。
- 成员 Key 使用请求级 `ActiveGroup` 完成跨协议分派、分组内账号 failover 和受控跨分组 fallback。
- 成员 usage、预算结算和最终实际分组归因必须由统一计费事务原子提交；普通请求计费失败可以保留未结算 usage，成员请求不得退化为独立 usage 写入。
- 成员 5h、1d、7d 与自然月聚合限额由成员名下全部 Key 共享；同步请求按当前实际用量授权并创建零金额回执，异步任务按明确预计费用创建真实占用，二者共用预算账本、幂等结算、崩溃恢复和对账证据链。
- 成员编号作为不可复用的导入/审计身份，创建后普通编辑不可修改；成员编辑可以带原因调整各窗口已用投影，自然月差额保留不可变账本证据。
- 成员分组编辑候选来自企业 owner 当前可访问分组；勾选表达授权，排序表达调用优先级和受控 fallback 顺序。
- 迁移当月开账与真实 usage 分开保存；普通 Key 行为和现有 owner analytics 保持兼容。
- 当前已落地成员实体、多 Key、有序分组、普通 Key 显式事务迁移、ActiveGroup、同步请求零金额 receipt、异步任务预算 hold、账本/恢复/对账、成员级请求记录、CSV/XLSX 持久化慢导入 job、一次性加密 Key 结果交付、append-only 审计、无高基数标签的 Ops 指标、正式 zh/en i18n 和企业成员控制台；真实 PostgreSQL/Redis 已验证多 worker 唯一领取、心跳续租、租约 fencing、异常 processing 恢复、5000 成员事务、Redis 重启订阅恢复、PostgreSQL 事务中断回滚和跨实例认证 L1 失效，浏览器 E2E、集群指标汇总、混合负载容量与持续性网络故障仍按设计合同继续收口。
- Grok 异步视频任务在返回上游任务 ID 前持久化 owner/member/Key/实际 group/account；状态查询只回到原任务账号，不重新选择成员候选或跨账号 failover。
- Composite 分组可以把公开模型按端点映射到 OpenAI、Anthropic、Gemini、Antigravity 或 Grok 的具体上游模型；与企业成员有序分组组合时，路由决策属于单个候选 attempt，切组必须清除并重新解析，不能继承上一组的目标平台或模型改写。
- HTTP 只在响应完全未提交且失败被显式标记为容量、瞬时上游或能力不匹配时进入下一候选；预算结果不明确是统一的禁止重试条件。Responses WebSocket 在首 turn 固定公开模型到实际上游模型的映射，只在首 turn 尚未产生下游事件时允许安全切换账号/分组；连接内切换模型、平台或更新后的渠道目标必须重连。每个 turn 分别保存实际请求模型、上游模型、渠道映射链、计费模型和调度结果，后续 turn 的 429 与未知传输结果禁止整连接重放并进入预算结果不明闭环。
- 权威设计见 [企业用户成员管理](../features/enterprise-member-management.md) 与 [ADR-0003](../decisions/adr-0003-enterprise-member-entity.md)。

### 用量分析

- 用户侧单 Key 下钻已落地：趋势、模型分布、请求记录三块功能。
- Gateway、OpenAI、Gemini、图片和异步 batch image 会把经过统一清洗的显式客户端 `session_id` 保存为请求证据；该字段与企业成员快照、最终 `ActiveGroup`、调度证据和预算请求 ID 并存，但不参与 sticky、调度、计费、幂等或 prompt cache。
- 企业 owner 级用量分析已落地在用户 Usage 页的分析 Tab，包括 summary、leaderboard、models、groups、tags、trend。
- owner analytics 接口在 `/api/v1/usage/analytics/*`，所有查询绑定当前登录用户，不接收外部 `user_id`。
- owner DTO 不返回 `account_cost`、上游账号、渠道、`upstream_model` 等管理员字段。
- 标签聚合采用“多标签重复计入”的归因语义，不作为严格财务分摊。
- 管理员用量下钻已落地：`/admin/usage` 顶部对象选择器、用户/Key 双栏下钻、路由 query 同步（含显式日期范围回写，刷新和分享链接保留时间口径）、路由用户标签回填与迟到响应隔离、趋势月粒度和密集图表展开。
- 已删除 Key 证据展示阶段 1 已落地：`/admin/usage` 管理员证据视图穿透软删除 hydrate Key 名称与删除状态，DTO 隐藏明文 key，导出保留已删除 Key 的 ID、名称和删除时间；用户侧 `/usage` 和普通 Key 列表仍只解析活跃 Key。

### 可用渠道模型与账号模型维护

- Home 公共导航和 Models 区域在模型列表功能启用时进入 `/model-plaza`；页面无需登录，只展示 active、standard、非专属分组，登录身份不会扩展结果。
- 公开模型列表与 `/available-channels` 复用客户安全渠道 DTO、稳定交付路线和分组级 API 端点投影；公开页使用分组默认倍率展示价格，用户页使用当前用户在该分组的生效倍率。
- Group / Channel 逐模型 token 价格支持多窗口分时倍率；标准按量分组无需订阅即可使用。最终价格来源同时决定基础价和时段规则；规则启用时当前分时倍率直接替代分组默认、用户专属和旧 Group peak 倍率，公开与登录目录按同一最终倍率展示，关闭时保留原倍率链。
- 用户侧可用渠道增加模型级表格视图和导出功能。
- 账号模型配置支持从上游 `/v1/models` 探测、将探测结果写入白名单或同名映射行。
- 自定义模型输入可以查询 models.dev 目录。
- 映射模式支持清空全部模型，并保留映射模式语义。
- 符合条件的 Ollama Cloud OpenAI / Anthropic API Key 账号支持管理员用量观察：session 加密保存，快照只展示官方 5 小时 / 7 天窗口、余额和模型请求数；该观察不进入调度、计费、账号健康或用户 DTO。
- OpenAI 分组支持默认关闭的 Live gate；macOS DeviceCheck attestation、Live HTTP / sideband 路由、并发租约、最长会话时间和独立 usage 类型已接入，管理员可先探测当前实例能力再决定是否开启。
- Live observer 在 controller / call store 抖动时有限重试并按会话到期时间兜底 finalize；usage 采用 best-effort 队列与同步回退，同时保留企业成员快照、最终实际分组和脱敏失败证据。
- 分组支持按平台配置最低利润率与安全缓冲；普通、OpenAI、图片和 WebSocket 调度在账号 slot 复核后使用同一利润 gate，倍率证据无效或低于阈值时切换候选。
- 多平台上游账单探测可在显式允许、可信声明和倍率上限内同步账号倍率；同步结果只改变成本证据，不绕过分组、模型、能力或企业成员调度边界。
- 分组支持逐模型、长上下文、视频、语音与搜索定价；公开报价继续由共享 available-channel catalog 投影，不恢复已删除的第二套模型目录。
- Channel Monitor V2 作为显式 opt-in 的被动聚合视图接入；默认 V1 继续使用 dev-zz 分组模型自检、可选历史 fail-soft 与管理员 Token 用量页面。
- usage / Ops 保存 requested、sent 与 upstream response model 及 mismatch 证据；渠道只有在响应模型通过安全准入和价格解析时才可按 response model 计费。
- OpenAI pool-mode API Key 提供默认关闭的跨实例健康熔断，只累计可归因于账号的 429 / 5xx；Responses HTTP / WebSocket 共用 rejected-field compatibility、失败分类和账号健康证据，但连接内路由锁定与企业成员预算结果边界不变。
- 管理端支持上传、校验、灰度启用和停用独立进程 `.s2plugin`；当前只开放 OpenAI OAuth 出站传输能力。插件默认停用且生产默认拒绝未签名包，宿主继续控制账号选择、企业成员最终分组、预算、usage、sticky 与未知结果禁止重放。
- OpenAI Responses、Chat 和 WebSocket 支持 `fast` service tier 的透传与按上游实际响应档位计费；计费只允许从请求档位降级到上游实际档位，不能因上游回显把普通请求升级收费。
- Chat / Responses fallback 会拒绝被输出上限截断成非法 JSON 的普通 function arguments，同时保留单调用 16 MiB / 单响应 32 MiB 的线性参数预算；DeepSeek 原生 Responses 可以降级并恢复 Codex client tools，WebSocket HTTP bridge 不重复 replay 已带 call context 的后续轮次，也不保留没有 output 配对的孤立历史调用。
- `codex-auto-review` Guardian / review 请求可以用父 thread sticky hash 在当前分组内优先选择父账号；lineage hint 不携带账号 ID，仍受分组、隐私、传输、能力、利润和数据库二次复核约束，失败时不删除父 thread 的有效 sticky。
- OpenAI OAuth 图片生成区分内容政策拒绝、普通文本 fallback 与完全无输出；普通文本 fallback 可短期冷却当前账号并受控切换同组候选，响应已提交或结果不明确时不得重放。
- Ollama Cloud raw Chat Completions 对 `reasoning` / `thinking` 与 `reasoning_content` 做受限双向兼容并按模型族收紧 `max_tokens`；Google One OAuth 只发布保守目录或显式 mapping。
- Kimi、智谱和 DeepSeek API Key 账号支持 OpenAI / Anthropic 协议、平台 base URL、余额 / 配额检测和可恢复停调；显式模型交付协议继续优先于账号探测结果。
- 上游模型目录读取增加独立字节上限；模型广场公开 token 阶梯和渠道工作日分时信息，但仍以客户安全 DTO 与 dev-zz 的单一 TimePricing 合同为准。
- 分组用量支持今日、昨日与总量汇总；日 rollup 使用显式业务时区并由触发器与周期任务幂等维护，生产升级仍需在真实数据副本验证历史回填和跨午夜边界。

### UI 与运维体验

- 首页、认证页、控制台布局、通用表单/表格/弹窗、管理端/用户端页面统一到当前 stone / neutral / emerald 视觉方向。
- Home 支持自定义内容优先、compact preset 次之、默认首页兜底；管理端账号列表支持按完整筛选结果全选，但删除生命周期继续以归档为主。
- 返佣开启且强制邀请码关闭时，注册页展示可选邀请码；强制邀请码、推广码、Turnstile 与既有注册禁用边界保持不变。
- 公告管理支持直接预览尚未发布的内容；Bell 与 Popup 共用富文本样式和滚动锁生命周期，预览不会写公告已读状态，共享样式继续使用 stone / neutral / emerald 规则。
- 前端隐藏 LinuxDo / 微信登录、注册、资料绑定和管理端认证显示入口；后端 OAuth 能力保留。
- 运维明细弹窗支持父子层叠，Escape、遮罩和滚动锁只作用于最上层弹窗。
- 运维错误详情和上游响应预览改为阅读型自动换行，降低长 JSON 横向滚动负担。
- 上游错误尝试保存事件时代理 ID / 名称，区分受管代理、明确直连和未知路径；legacy 详情只补齐代理归因键，不根据账号当前绑定改写历史。
- 单请求上游错误队列最多保留 256 次尝试和约 512 KiB；最新 16 次保留受限诊断正文，较早尝试保留核心归因并记录裁剪数量。
- 运维错误详情按 upstream root cause 优先展示 status、失败尝试与诊断 payload，并保留 route trace；capture writer 通过 generation-bound lease 隔离池化 state，旧请求不能访问复用后的新请求状态。
- 管理端新增独立提示词输入审计工作台，覆盖 Guard 节点配置、运行态、事件筛选/详情和确认删除；功能、阻断和通过事件保存默认均关闭，Guard token 不从管理 API 回显。
- 提示词审计没有可信运行配置快照时，管理接口返回 `prompt_audit_config_unavailable`，不把默认配置伪装成当前生效状态。
- `step_up_enabled` 与 `session_binding_enabled` 作为默认关闭的显式安全开关；启用后继续沿用 TOTP、会话绑定和操作审计合同。
- 支付宝官方移动端可选择当面付 `precreate` + App Scheme 唤起；功能默认关闭，唤起失败回退动态二维码，桌面和旧 WAP 流程保持兼容。

### 数据保留与运维策略

- dashboard aggregation 自动清理默认关闭。
- ops cleanup 自动清理默认关闭。
- 管理员仍可手动创建用量清理任务、取消任务或清理运维日志。
- 默认保留运行数据，删除动作必须是显式管理操作。

### 部署、发布与 CI

- fork 镜像默认值为 `thornboo/sub2api:latest`，也支持 `ghcr.io/thornboo/sub2api:latest`。
- 上游镜像 `weishaw/sub2api:latest` 不包含 dev-zz 二开，不应用于本分支部署。
- `deploy/docker-deploy.sh` 默认从 `thornboo/sub2api` 的 `dev-zz` 分支拉取部署文件。
- `deploy/backup-dev-zz.sh` 作为发布镜像更新前的备份入口；`deploy/build-image-dev-zz.sh` 仅保留为本地源码构建、开发验证和远程镜像不可用时的镜像打包路径。
- CI 引入 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` 验证 GitHub JavaScript actions runtime；项目构建 Node 版本仍是 20。
- Caddy 压缩规则明确排除 `text/event-stream`，避免 SSE 被压缩缓冲；便携检查脚本在后端 CI 中验证该合同。

## 文档归档规则

- 功能说明放在 `docs-site/dev-zz/features/`。
- 稳定接口和实现口径放在 [接口索引](./api-surface.md)。
- 配置、迁移、镜像和 CI 约束放在 [配置与迁移索引](./configuration-and-migrations.md)。
- 每次用户可见变化先写 [变更记录](../changelog.md)，再在 [补丁记录](../patches.md) 记录实现和验证。
