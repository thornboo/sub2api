# 补丁记录

## 2026-08-25 - 上游 main 增量同步：工具 schema、Lite 并行开关与 CLI 身份

### 目标

- 将 `origin/main@e2d9b823f` 继续合入正式线 `dev-zz`，取得 Gemini tool schema、Responses Lite、rejected-field retry 和 Grok CLI identity 修复。
- 保留企业成员 `ActiveGroup`、预算 / usage、sticky、插件 / TimePricing 合同和 fork `1.7.39` 发布线。

### 主要变化

- Gemini schema 移除嵌套 `deprecated`，把标量 enum 规范为字符串并丢弃包含复合值的 enum。
- Responses Lite 的非空 `additional_tools` 继续保留 `parallel_tool_calls:false`；Responses status rejection 一次清理全部同类型 item，避免有限重试预算被逐项消耗。
- Grok OAuth / CLI proxy 更新到官方 workspace User-Agent 和 `0.2.120`，普通 API Key / 非 CLI 目标不接受该身份覆盖。

### 冲突与兼容性

- 9 个上游提交修改 16 个文件；唯一冲突是 `backend/cmd/server/VERSION`，继续保留 `1.7.39`，不采用上游 `0.1.181`。
- 自动合并文件按控制流复审；本轮没有改变企业成员路由 / 结算、未知结果禁止重放、插件边界、分时价格、数据库、配置、依赖或前端运行时。

### 验证

- 受影响后端定向测试、全仓 unit、vet、server build 和 golangci-lint v2.13 通过；前端 typecheck / ESLint、docs build 与 Git 结构检查通过。
- 未重复运行 integration、前端全量 Vitest、完整镜像、真实 provider 或 Hosted CI；未推送、发布或部署。

## 2026-08-24 - 上游 main 增量同步：插件、Fast service tier 与工作日分时

### 目标

- 将 `origin/main@03e8ab413` 继续合入正式线 `dev-zz`，取得插件出站传输、Fast service tier、OpenAI quota reset / WebSocket、模型价格和运维安全修复。
- 保留企业成员最终 `ActiveGroup`、预算 / usage 原子归因、sticky、未知结果禁止重放、单一 TimePricing / 模型广场合同、stone UI 和 `1.7.39` 发布线。

### 主要变化

- 管理员可以上传、校验、配置和灰度启用独立进程 `.s2plugin`；安装和绑定默认停用，生产默认拒绝未签名包。当前插件能力只覆盖 OpenAI OAuth 出站传输，宿主继续控制认证、调度、计费和重试边界。
- OpenAI 三种入口透传 `fast` service tier，并按请求和上游实际档位的安全交集计费；上游响应只能触发降档，不能把普通请求升级收费。
- 渠道公开价格沿用 dev-zz 的 `TimePricing`，增加 `weekdays_only`；周末应用默认倍率 / 标签。模型目录增加独立响应大小限制，模型价格和上下文数据同步更新。
- 新增插件迁移 229/230、quota 自动重置、插件管理 UI、账号优先级、IPv6 代理与 Ops 详情改进；Go 版本和相关 CI 合同统一到 1.27.0，前端安全 override 同步到 lockfile。

### 冲突与兼容性

- 70 个上游提交修改 276 个文件；预演与真实合并均产生 46 个冲突路径，按 dev-zz 文档合同逐项解决。删除上游重新引入的旧 ChannelTimePricing / 模型广场组件及其孤立测试，不形成双实现。
- 插件返回的发送状态只作为宿主 retry 安全证据；插件不能改变账号候选、企业成员分组、预算回执、usage、sticky 或未知结果处理。现有品牌、认证入口默认值和 fork 版本保持不变。
- 全量测试发现同名等价工具被兼容桥过早拒绝，已恢复“等价去重、不同定义报错”；Wire 按最终 provider 集重新生成并二次生成稳定。

### 验证

- 后端全仓 unit / integration、vet、server build、Wire 再生成和 golangci-lint v2.13 通过；前端 typecheck、ESLint、生产构建和全量 Vitest（297 个文件、2032 条用例）通过。
- pnpm frozen-lockfile、audit exception、Compose 安全合同、docs build 和 Git 结构 / 冲突 / whitespace 检查通过。
- 未连接真实 provider / 插件，未运行浏览器 smoke、完整镜像或 Hosted CI；未推送、发布或部署。

## 2026-08-24 - 上游 main 增量同步：工具续链、图片生成与 Guardian 亲和性

### 目标

- 将 `origin/main@d45135d87` 继续合入正式线 `dev-zz`，取得 Chat / Responses 工具参数正确性、DeepSeek 客户端工具、WebSocket replay、OAuth 图片、Ollama Cloud、Google One 目录和 Guardian 父账号亲和修复。
- 保留企业成员产品授权 / 路由计划边界、最终 `ActiveGroup`、预算 / usage 原子归因、结果不明确禁止重放、sticky / WebSocket 锁定、stone 视觉和 `1.7.38` 发布线。

### 主要变化

- Chat file part 转为 Responses `input_file`；普通 function arguments 被截断为非法 JSON 时不再发出完成事件。二开的工具参数 builder 继续按 16 MiB 单调用、32 MiB 单响应限额线性累计。
- DeepSeek 原生 Responses 支持 Codex client tools 的请求降级与非流式 / SSE 回程恢复；WebSocket HTTP bridge 不再重复补齐已经自带 call context 的 tool output，并移除无 output 配对的孤立历史调用。
- OpenAI OAuth 图片对“内容审核拒绝、普通文本 fallback、完全无输出”分别分类，普通文本 fallback 触发短期工具冷却和受控账号 failover；上游 experimental Responses header 与流式 / 非流式错误证据保持一致。
- Guardian / review 自动审查通过父 thread sticky hash 在当前分组内优先命中父账号，并保留分组隐私、传输、能力、利润、数据库二次复核和原 sticky 不被误删的约束。
- Ollama Cloud 对 raw Chat Completions 的 reasoning 字段双向兼容并限制 `max_tokens`；Google One OAuth 目录不再把通配模型当成可交付模型。

### 冲突与兼容性

- 3 个冲突分别位于 Responses 兼容桥、OpenAI 调度器和 fallback 测试；均按字段 / 控制流合流，没有整文件选边。
- malformed arguments 校验改为读取二开 builder 的真实累计值；Guardian 与渠道映射两种 sticky 保护合并为逻辑或，不能因为父账号失效清理正常父 thread 绑定，也不能跨组选择账号。
- 没有 migration、schema、配置、依赖、前端运行时代码或版本变化；普通 Key、企业成员预算回执、异步任务和未知上游结果的既有合同不变。

### 验证

- 定向 apicompat / service / admin handler 回归通过；后端全仓 unit、vet、server build、golangci-lint v2.9.0（0 issues）通过。
- 前端 typecheck / ESLint、docs-site build、whitespace / 冲突标记 / 索引和最终 merge topology 检查通过。
- 未运行 Testcontainers integration、真实 provider、浏览器 smoke、Docker 镜像、Hosted CI、推送、发布或生产部署。

## 2026-08-21 - 上游 main 增量同步：CN 探测、Composite 入口与 Home 可发现性

### 目标

- 将 `origin/main@67380eafd` 继续合入正式线 `dev-zz`，取得国产供应商账号测试 / 余额修复、Composite Messages / 视频入口、OpenAI sticky 与 capabilities 修复，以及 Home / token refresh 改进。
- 保留企业成员有序候选、最终 `ActiveGroup`、预算 / usage 原子归因、结果不明确禁止重放、stone 视觉和 `1.7.37` 发布线。

### 主要变化

- CN 账号测试按实际平台和显式协议选择 OpenAI、Responses 或原生 Anthropic 入口；DeepSeek 无效中继余额不再写成零余额，余额 / 配额刷新在管理端显示为明确的主动探测按钮。
- Composite 分组可开启 Messages dispatch 并进入 Grok 视频生成路由，但 OpenAI family / model 详细映射仍只对 OpenAI 分组开放；每个 Composite 候选继续重新解析实际目标平台。
- Chat sticky 只散列请求开头连续的 system / developer 前缀；空 OpenAI capabilities 作为未配置处理，明确的非空限制继续生效。
- token refresh 移除陈旧 peer 结果循环；Home 的 compact、默认导航和 Models CTA 统一按模型广场 feature flag 与登录要求显示。

### 冲突与兼容性

- 21 个上游提交修改 30 个文件；`merge-tree` 和真实 `--no-commit` 合并都产生 3 个冲突：Home、Groups 和 Groups Messages 测试。
- Home 保留二开的视觉与布局并吸收认证感知的模型广场入口；Groups 保留 stone 样式并扩展 Composite allow toggle，OpenAI 专属映射不外溢；测试合并两个必要 import。
- 全量测试另发现 Home 第三个入口与旧源码合同不一致，已统一三个入口的可见性条件并更新测试。无数据库迁移、配置和依赖变化；`VERSION` 保持 `1.7.37`。

### 验证

- 冲突与新行为定向测试通过；后端全仓 unit、vet、server build、golangci-lint v2.9.0（0 issues）通过。
- 前端 typecheck、完整 ESLint、生产构建和全量 Vitest（293 个文件、2003 条用例）通过；docs build 和最终 Git 结构检查通过。
- 未运行 Testcontainers integration、真实 provider、浏览器 smoke、完整镜像或 Hosted CI；未推送、发布或部署。

## 2026-08-21 - 上游 main 增量同步：Pool 重试、Antigravity 与工具流修复

### 目标

- 将 `origin/main@f646a1f97` 继续合入正式线 `dev-zz`，吸收 OpenAI-compatible pool 重试、Antigravity 官方 daily 端点、流式工具名兼容和并发测试修复。
- 保留企业成员有序候选、预算 / usage 原子归因、sticky、未知结果禁止重放以及 `1.7.37` 发布线。

### 主要变化

- Chat Completions / Responses compatibility forwarder 在 pool 账号收到可重试状态且账号未被 rate-limit 处理停调时，返回同账号重试信号；既有 handler 继续负责重试次数、延时、sticky 与最终换号。
- Responses arguments-only 流式 delta 省略空工具名，防止 OpenAI-compatible 客户端覆盖已累计的 function name。
- Antigravity 使用官方 daily 域名，并只让 `pro` / `ultra` 账号默认进入 daily；免费、未知、异常 plan 和显式 prod override 保持生产端点。
- CN 额度探测测试 fake 的并发记录改为互斥保护；nanoID 高危公告加入限期审计例外；支付集成文档链接指向迁移后的 `docs/` 路径。

### 冲突与兼容性

- 13 个上游提交修改 12 个文件；`merge-tree` 和真实 `--no-commit` 合并均无冲突，没有使用 `ours` / `theirs`。
- 同账号重试没有修改企业成员候选、预算回执、最终 `ActiveGroup`、跨分组 fallback 或 WebSocket 后续轮次边界。
- 无数据库迁移、配置项、依赖和前端运行时代码变化；`VERSION` 保持 `1.7.37`。

### 验证

- 新增 / 受影响测试、CN `-race`、后端全仓 unit、vet、server build、golangci-lint 和 audit exception 校验通过。
- 前端 typecheck / lint、docs build、Git whitespace / conflict-marker / ancestry 检查通过。
- 未重复运行 Testcontainers integration、前端全量 Vitest、完整 Docker 镜像、真实 provider、Hosted CI、推送、发布或部署。

## 2026-08-21 - 上游 main 继续同步：Responses 兼容、Grok 4.6 与 Ops 根因

### 目标

- 将初始 `origin/main@9d5171c5d` 以及合并期间继续前进的最终 head `9f74eb57f` 合入 `dev-zz`，取得 OpenAI / Grok 兼容、健康熔断、Ops 根因证据和 reasoning effort 修复。
- 保留企业成员有序候选、最终 `ActiveGroup`、逐轮预算 / usage、WebSocket 连接锁定路由、未知结果不重放、stone 视觉和 `1.7.37` 发布线。

### 主要变化

- Responses 输入、tool schema / name、terminal usage、item ID 和 rejected-field retry 统一兼容；compact fallback 和 WebSocket 同号重试使用有限循环，不因递归重置单次重试状态。
- OpenAI pool API Key 可通过默认关闭的 DB setting 启用跨实例健康熔断；只累计可归因于账号的 429 / 5xx，不把凭据错误、request / provider scope、context cancel 或 deadline 误计入账号健康。
- WebSocket 增加会话抢占和恢复，但二开仍以建连时选定的最终公开模型、渠道映射、账号映射、平台、分组和账号作为连接合同。逐轮预算和 usage 保留；后续轮次未知传输结果标记 ambiguous，不跨账号重放。
- Grok 默认目录迁移到 4.6，补齐媒体尺寸、usage、Realtime 预握手 / 冷却、stream idle、compaction、429 / 529 分类与有限重试；文本 Responses 使用独立响应头超时。
- Ops backend 保存 upstream status、root cause、失败事件和实际 route evidence；capture state 以 generation 隔离复用。前端优先展示真实上游根因并去重重复 payload，保留 stone 主题和 route trace。
- 原生 Anthropic / Chat / Responses 转换按最终映射模型规范化 `reasoning_effort`，避免把 `gpt-5.6-*` 的 `max` 降级；prompt guard 配置成功日志改为 change / recovery driven。

### 冲突与兼容性

- 第一段合并有 16 个冲突，集中在 OpenAI Handler、WebSocket、Grok、rate-limit、Ops backend / frontend 和 rollup 测试；全部按最终状态机与文档合同合流。第二段新增 6 个提交自动合并，无冲突。
- Responses failover 同时保留 stream-start guard、enterprise group retry 标记和上游 rejected-field retry；WebSocket later-turn 不因新重试逻辑放宽跨账号重放边界。
- Ops writer 同时满足 generation 隔离、外层 writer 恢复和 inactive write 合同；Grok 模型容量不隐藏 sibling credentials，rate-limit 仍先执行二开模型 / Anthropic 特定分类。
- 自动拼接后额外修复 Responses heartbeat-only 流缺少失败终态、Grok model self-check 污染 team-model cooldown、OpenAI WS 429 丢失模型级限流上下文这 3 个无标记回归。
- 无数据库迁移；`.gitignore` 保留 `.omx` 并加入 `.codegraph/`，`VERSION` 保持 `1.7.37`。

### 验证

- 定向回归覆盖 WebSocket 锁定模型与逐轮 usage、enterprise budget ambiguous、Ops stale lease / nil writer、Responses 流终态、Grok self-check、模型级 rate-limit、scheduling、rollup、reasoning effort、prompt guard 和 Ops 详情组件。
- 后端全仓 unit / integration、vet、server build 与 golangci-lint v2.9.0（0 issues）通过；前端完整 ESLint、生产构建、292 个文件 / 1992 条 Vitest，以及 docs build 通过。
- integration 使用临时 Testcontainers 数据服务；未连接生产数据或真实 provider，未运行浏览器人工 smoke、完整镜像、Hosted CI、推送、发布或部署。

## 2026-08-21 - 上游 main 同步：自适应协议、Codex 恢复与渠道倍率

### 目标

- 将 `origin/main@2bc139ab5` 合入正式线 `dev-zz`，取得国产供应商自适应协议、Composite CN / Codex 路由、OpenAI WebSocket 恢复、Grok 工具、渠道倍率、监控校验和 usage 查询优化。
- 继续以 `docs-site/dev-zz` 为冲突裁决依据，守住企业成员候选编排、预算 / usage 原子归因、模型交付协议、唯一分时定价合同、stone / neutral / emerald 视觉和 `1.7.37` 发布线。

### 主要变化

- Kimi、智谱和 DeepSeek 账号支持 `adaptive` 协议与 Chat Completions / Anthropic / Responses 分协议 base URL；Composite 可把文本与 Codex 控制入口解析到 CN provider。显式 `api_protocol` 是同步管理员意图，优先于可能滞后的 Responses probe `extra`。
- OpenAI Responses / Messages / WebSocket 合并后续 turn resume、当前 turn 缓冲重试、客户端工具跨 turn 保留、input token 预检、容量恢复、Chat 缓冲读取 failover 和 reasoning item ID 缓存回注；企业成员首次激活、每候选 Composite 重解析、最终 `ActiveGroup`、sticky、请求级预算和“结果不明确不重放”保持不变。
- Grok 把客户端 tool-search discoveries 转成可调用工具，保留延迟加载 / namespace / capability registry 边界，并合并内联图片工具；Responses→Chat 同时使用二开的工具注册表 / 上游能力矩阵和新增 reasoning cache，不以其中一套覆盖另一套。
- 渠道模型价格增加 Fast / Flex 倍率，长上下文区间增加 input / output / cache write / cache read 倍率；既有 `time_pricing` 仍是唯一分时合同。账号统计价格入口丢弃渠道倍率和分时规则，继续只表达供应商成本。
- usage 总览使用一条 `GROUPING SETS` 查询同时得到总计、入站、上游和路径聚合，并保留 enterprise member、owner、模型来源、billing mode 等过滤；本地模型配置错误不再携带上游账号 / endpoint 归因或计入上游 SLA，路由计划和实际尝试证据仍保留。
- Channel Monitor 在保存时拒绝不可用配额数据源和非法模式组合；用户 / 管理端把纯配额占位模型 `quota` 显示为本地化标签。代理出口探测允许配置有序 URL 与 `ip-api` / `ipify` / `chatgpt-trace` parser，并在启动时严格校验。

### 冲突与兼容性

- `git merge-tree` 预演和真实 `git merge --no-commit origin/main` 均报告 37 个冲突文件。没有使用整仓或整组 `ours` / `theirs`；每个文件按最终数据结构、控制流和测试合同合流。
- 定价冲突保留 `TimePricing` 的 IANA 时区、自定义默认 / 规则名称、最多 16 条、跨午夜、`0x` 和请求开始时结算，同时加入 Fast / Flex 与区间倍率。未引入上游较简单的第二套 `ChannelTimePricing`。
- 网关冲突允许 Composite / CN / Codex 新入口，但继续按请求实际解析平台执行 Live / WebSocket gate；企业成员能力不匹配才可切下一候选，已提交响应、外部任务、WebSocket turn 或结果不明确时不能重放。
- 管理端冲突保留企业账号能力、stone 视觉和二开表单字段，同时加入自适应 base URL、长上下文显示门控、角色 Select 行为和配额模型格式化。`VERSION` 保持 `1.7.37`，不采用上游 `0.1.179`。
- 新增迁移 `226_add_usage_log_effective_model_indexes_notx.sql`、`227_composite_routes_add_cn_providers.sql`、`228_channel_pricing_multipliers.sql`，全部按完整文件名追加；没有改写历史迁移。

### 验证

- 冲突相关后端 Handler / route / protocol 定向测试通过；CN adaptive / 显式协议覆盖和管理端渠道 handler 包测试通过。前端冲突相关 10 个文件、115 条用例通过。
- 前端 typecheck、完整 ESLint 和全量 Vitest 通过：292 个测试文件、1973 条用例。全量后端 unit、vet、build、前端生产构建、docs build 与最终 Git 检查见同日合并记录。
- 未连接真实 PostgreSQL / Redis、真实 provider 或浏览器人工 smoke；未运行 Docker / Hosted CI，未推送、发布或部署。

## 2026-08-18 - 按量模型分时定价

### 目标

- 把峰谷价格建模为逐模型 token 销售价的时间维度，让以钱包按量扣费为主的标准分组直接使用，不借用订阅资格或订阅配额。
- 让管理端配置、运行时扣费、公开/登录目录报价和历史审计使用同一价格来源、时区、边界和倍率语义。

### 主要变化

- Group / Channel 价格条目新增 `time_pricing`：支持可搜索 IANA 时区、其余时段的自定义类型名称与倍率、最多 16 个带自定义类型名称的分钟精度显式窗口、跨午夜与 `0x` 免费；类型名称是独立的客户文案，不根据倍率推断，显式规则可为空，浏览器和服务端均拒绝空名称、过长名称、非法时区、越界倍率、相同起止和循环日重叠。
- 统一价格解析继续遵循 Group → Channel → LiteLLM → fallback。最终获胜的 Group 或 Channel 条目同时提供基础价和时段规则；新规则启用时当前分时倍率直接替代分组默认、用户专属和旧 Group peak 倍率，缺失时保留旧行为，避免 `1.1 × 1.1 = 1.21` 一类隐藏叠加。
- generic / OpenAI token 计费使用请求冻结的 `pricing_at`，实际倍率写回 usage 的 `rate_multiplier`；定价时刻、时段倍率、管理员配置的类型名称、时区和规则快照复用管理员可见的 `schedule_meta` 持久化。
- 管理端复用模型价格卡片编辑规则，账号统计成本入口隐藏该能力。客户目录只投影可见分组的最终 Group 模型价，原样返回 `default_label` 与每条规则的 `label`；卡片、表格和导出以分时倍率为完整最终倍率展示当前及各时段输入/输出/缓存价格，与服务端扣费一致。

### 数据与兼容性

- `225_channel_model_time_pricing.sql` 给 `channel_model_pricing` 增加 `time_pricing jsonb NOT NULL DEFAULT '{}'::jsonb`；Group 规则使用既有 `groups.model_pricing` JSONB，不新增订阅列。
- 仅 token 模式消费新规则；图片按次、视频、音频、Web Search 和 Account Stats 价格不变。持久化或 API 脏倍率在运行时安全回退，不用于扣费。

### 验证

- 后端完整 unit-tag 测试与 `go vet ./...` 通过，覆盖边界分钟、跨午夜、重叠拒绝、分组价格优先、分时倍率替代分组/用户/旧 peak、非法时区回退、审计快照和 repository JSON 往返。
- 前端 4 个目标测试文件 41 条用例通过；全量 Vitest 279 个测试文件、1881 条用例，以及 typecheck、完整 ESLint 和生产构建通过。覆盖表单转换/校验、分组最终价格投影、分时倍率替代用户倍率、客户时段表、表格/导出与实际价格。
- 迁移内嵌测试、docs build 和最终 diff 检查通过；按本次会话约束未使用 Playwright。

## 2026-08-17 - 上游 main 同步：国产供应商、多协议与分组日用量汇总

### 目标

- 将 `origin/main@e330c243a` 合入正式线 `dev-zz`，取得 CN provider、分组日用量 rollup 和 Codex transport 正确性更新，同时保持企业成员、共享目录、视觉与 `1.7.36` 发布线。
- 对 11 个文本冲突和编译暴露的 3 个无标记语义拼接问题逐项合流，不用整文件选择掩盖双方合同。

### 主要变化

- Kimi、智谱和 DeepSeek 支持 OpenAI / Anthropic 协议、平台 base URL preset、余额 / 配额查询、周期检测和可恢复停调；管理端账号创建、编辑、批量编辑与用量单元格同步识别这些平台。
- 分组用量增加日 rollup repository / service、启动 leader lock、时区修正、昨日用量和汇总 API；dashboard retention 关闭时继续跳过全部清理目标，但不会阻止独立 rollup 同步。
- Codex Responses 合并 turn-state echo guard、fingerprint opt-in、session / prompt-cache binding 和 native / legacy compaction 决策；CN Anthropic 原生端点与 Chat Completions fallback 接入现有模型映射、Fast 策略和计费链。
- OpenAI 调度合流后继续携带 `ActiveGroup`、sticky / profit control 与显式 delivery protocol；前端冲突取供应商成本字段、CN provider 字段和 stone / neutral 视觉的并集。

### 数据与兼容性

- `222_group_usage_daily_rollups.sql` 新增分组日用量聚合结构与增量维护；`223_group_usage_rollup_timezone.sql` 固化 rollup 时区口径；`224_user_platform_quotas_add_cn_providers.sql` 扩展平台配额约束到 Kimi、智谱和 DeepSeek。
- 三个迁移均按完整文件名追加，不重命名、覆盖或改写旧 SQL。发布前仍需在生产同量级副本验证 migration lock、历史回填、触发器增量和时区切换边界。
- `VERSION` 保持 `1.7.36`；企业成员预算 / usage 原子归因、共享 available-channel catalog、模型状态 V1 默认、长期数据保留和隐藏 LinuxDo / 微信入口策略不变。

### 验证

- `git merge-tree` 预演与真实 `git merge --no-commit` 均报告相同 11 个冲突；解决后 Wire 重新生成一致，未合并索引、冲突标记和 whitespace 检查通过。
- 分组 rollup、repository / migration、CN provider、多协议、Codex turn-state / compaction、Handler / route 定向后端测试通过；前端冲突相关 119 条用例和 typecheck 通过。
- 后端默认标签全仓测试、unit tag、vet 和 server build 通过；前端全量 Vitest 278 个测试文件、1868 条用例、typecheck、完整 ESLint 和生产构建通过；docs-site build 通过。当前本机未安装 `golangci-lint`，真实 provider、真实数据服务、浏览器、镜像与 Hosted CI 不在本地验证范围内。

## 2026-08-13 - 流式失败预算终态与 WebSocket 粘性绑定生命周期修复

### 目标

- 修复 Responses 流已经以 HTTP 200 提交、随后收到或合成 `response.failed` 时，企业成员零金额预算回执一直停留在 `reserved` 直到过期的问题。
- 修复 OpenAI WebSocket 已取得 `response_id` 后，请求 context 恰好取消会让 Redis 粘性账号绑定立即以 `context canceled` 失败的问题；不改变企业成员候选分组、账号选择、重试或计费规则。

### 主要变化

- 为 HTTP 请求增加显式的企业预算“确定失败”终态。已经传达给客户端的上游 `response.failed`、以及 HTTP 200 流内合成的确定错误会标记该终态；预算中间件在请求返回后立即释放回执，不再依赖已经固化的 HTTP 状态码推断结果。
- 终态优先级保持 `async task owner > ambiguous > definitive failure > HTTP status`：上游结果或用量落库结果不明时仍进入对账，不能被确定失败标记误释放；cyber 拒绝继续走带真实 token 的专用用量计费路径。
- WebSocket `response_id -> account_id` 的 Redis 写入保留请求 context values，但解除请求取消传播，并重新附加 3 秒硬超时。本地热绑定仍先写入；Redis 读取和删除继续服从原请求取消，不扩大后台工作范围。

### 数据与兼容性

- 无数据库迁移、配置键、公开 API 或依赖变化；企业成员分组候选与路由编排代码未修改。
- 普通非流式 4xx / 5xx 仍按 HTTP 状态释放，未知上游结果仍保持 `ambiguous`，正常成功请求仍由统一 usage 结算；修复只补齐 HTTP 200 流内确定失败的缺口。

### 验证

- 回归覆盖 HTTP 200 `response.failed` 释放、ambiguous 优先级、cyber 专用计费隔离、普通传输错误不误判，以及已取消请求 context 下 Redis 绑定仍执行且保留短超时。
- 受影响 Handler / Middleware / Service 完整包测试、CI 同款 `go test -tags=unit ./... -count=1`、默认标签 `go test ./... -count=1`、聚焦 `-race`、`go vet ./...`、`golangci-lint run ./...`（0 issues）和 server build 通过。
- `go mod tidy -diff` 报告基线 `go.sum` 存在与本补丁无关的冗余历史校验项；本补丁未修改 `go.mod` / `go.sum`，也未顺带清理依赖元数据。

## 2026-08-13 - 上游 main 同步：监控 V2、Grok 能力与计费审计

### 目标

- 将 `origin/main@fbfdcef8` 合入正式线 `dev-zz`，吸收上游正确性、安全性、兼容性和用户可见能力，同时守住企业成员路由、原子预算 / 计费 / 归因、共享模型目录、模型自检和 `1.7.32` 发布线。
- 明确解决 82 个双向冲突，不以整文件 `ours` / `theirs` 掩盖语义差异；生成代码从合并后的源 schema / provider graph 重建。

### 主要变化

- Channel Monitor V2 增加被动 minute rollup、隐私开关、用户 / 管理端视图和温和回填；默认仍为 V1。`/monitor` 外壳按 mode 选择页面，V1 继续承载 dev-zz 分组模型自检和管理员 Token 用量，V2 不取代现有默认能力。
- Grok 增加视频创建 / 编辑 / 扩展、Voice、Realtime、Web Search、X Search、订阅档位与媒体计费；所有企业成员入口继续经过候选编排、Composite 解析、预算门禁和最终实际分组归因。
- 渠道可使用经安全准入的 upstream response model 计费；usage 保存 requested / sent / response / mismatch 证据。分组增加逐模型 pricing、长上下文开关、视频模型、Voice 和 search price；公开目录仍使用共享 catalog。
- 设置和账号调度吸收平台用量阈值、Grok 默认模型 / base URL mapping、Channel Monitor mode / throughput 隐私，同时保留企业 admission、native protocol、自检、schedule strategy 和敏感字段脱敏合同。
- API compatibility 保留 dev-zz Tool Search / namespace / deferred 边界并吸收 `x_search`、reasoning alias 和 Gemini / Antigravity 修复；OpenAI HTML 403 不再处罚账号，pool-mode request-local retry 不会被新模型冷却提前截断。

### 数据与兼容性

- 新增 upstream migration `194_add_usage_log_upstream_response_model.sql`、Channel Monitor V2 `194_channel_monitor_v2.sql` 到 `206_channel_monitor_v2_privacy_defaults.sql`，以及分组定价 `217` 到 `221`。同号迁移以完整文件名独立执行，历史 checksum 兼容规则保留，不重命名或改写已应用文件。
- Group / UsageLog Ent schema 和生成物取双方字段并集；Wire 同时启动 / 停止 Model Self Check 与 Channel Monitor V2 aggregator。
- `VERSION`、企业成员 API、共享模型 catalog、owner/admin 隐私和默认数据保留策略不变。

### 验证

- 合并前预演和真实合并均报告 82 个冲突；解决后无未合并索引或冲突标记，Ent / Wire 重新生成一致，`git diff --check` 通过。
- 企业成员候选 / Composite / 预算、gateway routes、usage insert / query、migration runner、API compatibility、模型状态和设置冲突面定向测试通过；Go 全仓测试编译通过。
- 前端 typecheck、完整 ESLint、生产构建和全量 Vitest 通过（275 个测试文件、1853 条用例）；首次完整 Vitest 暴露的重复 mock、V1/V2 i18n 扫描和固定 Ops 快照参数等 4 个合并合同均已修正。`make test-unit`、`go test ./... -count=1`、`go vet ./...`、`golangci-lint run ./...`（0 issues）、后端构建和 docs build 也全部通过。
- 未执行真实外部 provider、生产数据库迁移、长周期 V2 回填、浏览器人工 smoke、镜像、Hosted CI、tag、Release 或生产部署。

## 2026-08-12 - v1.7.29 事故后发布候选加固

### 目标

- 让企业成员预算基础设施错误在下次复发时可精确定位到事务阶段，同时不改变预算超限、冲突等客户端可行动错误的既有 HTTP 分类。
- 让用户模型状态页面在可选历史 / 时间线存储不可用或响应局部畸形时保留当前状态，而不是由一个非关键依赖或坏行造成整页失败；授权与主状态读取继续 fail-closed。
- 清除正式分支 `Security Scan` 已报告的 `nanoid` 高危锁文件版本，不以 audit 例外掩盖已存在补丁的漏洞。
- 明确边界：这些修复提高可观测性和页面韧性，但不会让已经失败的预算请求自动成功，也不证明事故当时的六个底层 SQL 失败点或模型状态页面故障根因已经全部定位。

### 主要变化

- 企业成员预算预留为事务启动、已有预留查询、消费限额预留、新预留写入和事务提交增加稳定阶段标签；死锁重试耗尽、退避前取消和退避期间取消也带操作阶段。全部包装使用 Go error chain，保留 `errors.Is`、`errors.As` 和 PostgreSQL SQLSTATE。
- 领域错误白名单保持预算超限、请求冲突、无界金额、receipt / 成员不存在等既有语义；唯一键冲突继续转换为 Conflict。只有未分类的仓储 / 事务错误获得阶段上下文，并由现有中间件按平台故障返回 500、记录原始错误。
- 模型状态列表在可选 30 天历史不可用时返回当前状态并把可用率留空；单目标时间线 / 快照不可用时保留该目标的基础状态。详情页在快照失败后回退旧时间线，两种可选来源都失败时仍返回基础详情。用户可见分组查询、目标、账号和 latest 主数据读取仍是硬失败边界。
- 前端 API 边界把服务端 payload 视为未知输入：非法状态回落为 `unknown`，非有限数值回落为空，缺少合法分组或模型的行被丢弃，畸形详情明确失败并保留列表已取得的详情基线。详情请求使用独立取消信号，旧请求不能覆盖新选择。
- `frontend/pnpm-lock.yaml` 中 PostCSS 的传递依赖 `nanoid` 从 `3.3.16` 更新为同一兼容范围内的 `3.3.17`；没有新增直接依赖、workspace override 或 audit exception。

### 数据与兼容性

- 无数据库迁移、配置键或公开 API schema 变更。模型状态可选数据缺失时字段沿用既有 `null` / 空数组形态，授权边界不放宽。
- 企业成员预算原子预留 / 结算合同不变；请求遇到真实基础设施故障仍失败，只是由错误阶段和原始 error chain 保留可诊断证据。
- `nanoid` 只做 lock-only patch 升级；应用源码没有新增 nanoid 调用。

### 验证

- 远端旧 head `2aac5e9e` 的 `CI` run `31583772673` 与 `dev-zz Branch Images` run `31583772697` 成功；同一 head 的 `Security Scan` run `31583772643` 仅因 `nanoid@3.3.16` / `GHSA-2v37-7h3g-55p8` 失败。当前 lock-only 修复通过 frozen-lockfile 安装、`pnpm audit` 和仓库 audit exception 校验，但仍需新提交对应的 hosted Security Scan 证明正式门已转绿。
- 后端 `go test ./... -count=1`、`go vet ./...`、`golangci-lint run ./...`（0 issues）通过；预算阶段回归验证 error chain、SQLSTATE `40P01` / `53300` 和 context 取消仍可识别，模型状态回归覆盖可选历史 / 快照 / 时间线同时故障及授权查询 fail-closed。
- 前端完整 Vitest 261 个测试文件 / 1764 条测试、`vue-tsc --noEmit`、完整 ESLint 和生产构建通过；新增 API 畸形 payload、详情取消信号、中英文模型状态静态 / 动态文案及 `/monitor` 路由合同测试。
- 当前没有独立测试环境，也没有执行生产流量回放、生产 PostgreSQL 故障注入或浏览器连真实生产 API 的 smoke。发布前必须让新提交的 hosted CI、Security Scan 和 Branch Images 全绿；本地 PASS 不能证明未知生产根因不存在。

## 2026-08-12 - v1.7.29 admission 热路径数据库放大修复

### 目标

- 修复 v1.7.29 企业成员请求在预算预留前重复执行 admission readiness 聚合的问题，避免前置数据库查询争抢连接池后把预算事务启动失败暴露为平台 500。
- 把未经生产负载证明的 shadow 规划改为显式 opt-in，同时保持 enforce readiness、rollout、auto-stop 和证据不可用时 fail-closed 的既有安全语义；不放宽预算门禁掩盖故障。
- 本次默认回退收口只在本地 `dev-zz-develop` 继续提交；不再推送、不发布、不部署，也不修改已经发布的 `v1.7.29` tag。

### 主要变化

- `GetEnterpriseMemberModelAdmissionRuntime` 先检查 5 秒 runtime cache，再在 singleflight 冷缓存路径计算 readiness；热缓存命中不再触发 alias/evidence 聚合，冷缓存并发只允许一次重建，并与设置读取共用 5 秒重建超时。
- 同一次 readiness 评估复用已经取得的 admission evidence summary 来计算 auto-stop，不再通过包装 provider 第二次读取相同的 30 天聚合证据。
- 部署默认、新安装默认、空值、非法配置和设置数据库读取失败统一回到 `legacy_order_only`，因此未经显式配置的升级不会在预算前进入账号 / 分组 / 协议能力投影；测试环境仍可显式启用 `shadow_published` 收集证据。
- gateway 的 legacy runtime 使用无数据库的保守 readiness 占位，不触发 alias/evidence 聚合；legacy 使用计数仍按请求记录，但告警日志按进程每分钟最多一条，避免安全回退本身形成日志放大。
- 管理端保存 legacy / shadow 设置时不再计算只供 enforce 校验使用的 readiness；显式保存 enforce 时仍执行完整 gate，拒绝不满足条件的扩大。
- 显式 enforce 在 readiness、rollout 或 evidence 不满足时仍降级为 `shadow_published`，不会扩大 enforce 范围；显式 shadow 的缓存过期和并发重建继续遵守上述有界路径。

### 数据与兼容性

- 无新增迁移、API 或依赖；`gateway.enterprise_member_model_admission_mode` 的缺省值从 `shadow_published` 回退到 `legacy_order_only`，管理端缺省与非法值回退同步更新。v1.7.29 的 `199`-`202` 迁移文件保持原样。
- 预算预留、结算、usage 原子落账和最终实际分组归因不变；`fix/budget-error-attribution-1728` 的平台错误归因继续保留。

### 验证

- 新增 readiness 调用次数回归：修复前单次冷缓存 / 热缓存 / 32 并发热命中分别稳定为 `2 / 1 / 32` 次，修复后为 `1 / 0 / 0` 次；另显式验证 32 个并发冷 miss 也只重建 1 次。
- 新增同一次 readiness 只读取一次 admission evidence repository 的断言；修复前稳定为 2 次，修复后为 1 次。
- 新增默认 admission 不调用 model-aware planner、默认 legacy、设置缓存刷新与非 enforce 设置保存不调用 readiness provider 的回归，并覆盖缺失、空值、非法值、resolver 非法返回和设置数据库故障均回到 `legacy_order_only`；legacy WARN 限频和显式 shadow / enforce 的既有语义测试继续保留。
- 后端默认与 `unit` 全量测试、service 全包复跑、聚焦 `-race`、`go vet ./...`、`golangci-lint run ./...`（0 issues）、server build 和 Ent / Wire 生成物一致性通过；前端 259 个测试文件 / 1758 个测试、typecheck / lint / build 与 docs build 通过，`git diff --check` 通过。
- repository / service 的 integration 测试二进制以 `integration` tag 编译通过；当前 Colima Docker socket 不存在，真实 PostgreSQL / Redis Testcontainers 未执行。本地验证不替代正式分支 CI、测试环境流量回放或生产发布门。
- 独立 code review 未发现 correctness、安全、并发、缓存、fail-closed、rollout、readiness 或 auto-stop 阻塞项。

## 2026-08-05 - 企业成员模型感知路由治理本地实现

### 目标

- 修复 2026-08-05 企业成员多分组路由样本暴露的模型无感知候选问题：未知或未发布模型不得按成员绑定顺序横扫无关分组。
- 建立发布模型准入、稳定资格投影、revision/LKG、typed attempt、Ops 证据、alias review 和 enforce readiness/rollout/auto-stop 的本地治理闭环。
- 明确保留生产边界：当前是本地实现与最终本地验证完成，不是 commit、release、deploy、生产灰度、默认 enforce 或 shadow 删除完成。

### 主要变化

- 阶段 0-4 本地代码已实现并通过最终本地验证：企业成员规划器按当前授权顺序只裁剪不扩权；三态 admission mode 继续以 `shadow_published` 为默认；服务端 readiness、rollout policy 和 auto-stop 共同决定 enforce 是否可扩大。
- 管理设置更新在 JSON 解码前限制为 4 MiB；rollout policy 还分别限制 raw/normalized ID 数量、显式目标总数、salt 和 JSON 大小，避免管理员凭据被滥用时形成无界解码或持久化压力。
- routing eligibility 使用 PostgreSQL revision/outbox + Redis Pub/Sub + 启动/定期全量对账 + atomic mirror；LKG 使用短 TTL、generation key、当前授权交集和实时 scheduler 复核，规划依赖失败时不恢复全授权分组扫描。
- Composite 文本协议使用无副作用预览；Embeddings、Images、Live、Batch Images、Grok Video 和 Gemini Native 进入非文本 evaluator coverage，并通过 typed local gate 防御规划/执行竞态。
- Ops 错误详情与成功 usage 增加脱敏计划来源、LKG 年龄和 bounded routing attempts 证据；alias review ledger 保存 `legacy_success_new_pruned` 的人工处置状态，但不参与运行时准入。
- 管理端系统设置展示 admission mode、来源、readiness 条件、rollout、auto-stop、legacy retirement target 和 alias review；阶段 5 只完成本地退役准备，`phase5_production_gate_pending` 仍阻止新安装默认 enforce。

### 数据与兼容性

- 新增迁移：
  - `backend/migrations/199_routing_eligibility_revision.sql`
  - `backend/migrations/200_enterprise_member_alias_review_ledger.sql`
  - `backend/migrations/201_ops_routing_attempts.sql`
  - `backend/migrations/201a_ops_routing_attempts_indexes_notx.sql`
  - `backend/migrations/202_account_model_protocol_capabilities_non_text.sql`
- 新增 / 变更 setting：`gateway.enterprise_member_model_admission_mode`、`enterprise_member_model_admission_mode`、`enterprise_member_model_admission_rollout_policy`、`enterprise_member_model_admission_legacy_retirement_target`。
- 普通 Key、单分组 Key、企业成员预算结算、owner 可见 usage、Key 自助查询和旧 Ops 行保持兼容；新 routing attempts 和 route plan 来源字段为空时按旧终态字段展示。

### 验证

- 最终本地验证通过：后端 `go generate`、`make test-unit`、`go test ./... -count=1`、`go vet ./...`、`golangci-lint run ./...`（0 issues）；18 个 Colima/Testcontainers PostgreSQL 16/Redis 顶层测试（14 个 routing eligibility + 4 个 migration runner）；focused race；前端 typecheck、lint、完整测试（259 个测试文件、1758 条测试）和 build；docs build；`git diff --check`；隐私/脱敏专项测试；新增差异的高置信度密钥模式扫描。未把环境中未安装的 gitleaks 等专用扫描器计作已执行证据。
- 尚未完成 commit、正式发布流水线、生产数据库迁移、生产 7d/30d alias/canary release-window 证据收集、生产 enforce 灰度、默认值切换或 shadow 双算删除。
- `201` 的 CHECK 约束以 `NOT VALID` 安装，历史行必须在低峰维护窗口审计并验证；`201a` 的并发索引重试会先清理同名 invalid index，但仍必须在生产同量级副本演练耗时、磁盘峰值和中断恢复。本地迁移 PASS 不能关闭这两项生产门。
- 发布前仍必须按 [企业成员模型感知路由实施测试规格](./testing/enterprise-member-model-aware-routing-test-spec.md) 和 [验证矩阵](./testing/verification-matrix.md) 复核 release gate；本地 PASS 不能替代真实生产窗口证据。

## 2026-08-05 - 上游 main 同步：认证验证码、Codex 身份与共享图片报价

### 目标

- 将 `main@00b859617` 合入 `dev-zz-develop`，吸收多 provider 人机验证、Codex 官方版本同步 / 出站身份、订阅续费并发、WebSocket、提示词审计、OAuth 和管理排行修复。
- 保持 dev-zz 的共享公开模型目录、登录 / 注册入口策略、企业成员合同、长期数据保留和 `1.7.27` 发布线不回退。

### 主要变化

- 管理端验证码配置收敛为 Cloudflare Turnstile、腾讯天御、阿里云验证码 2.0 三选一；服务端拒绝同时启用多家，secret 只返回 configured 状态且不会被局部设置请求带入审计。登录、注册、找回 / 验证动作以及 OAuth 登录启动 / 待建账号继续使用统一 action proof 合同。
- Codex OAuth 出站的 User-Agent、originator 和版本字段由同一生效版本重建；默认每 6 小时同步官方最新稳定版，管理员手工版本优先。新增 `gateway.disable_codex_identity_enforcement` 回滚开关，旧 `disable_codex_originator_normalization` 继续兼容。
- 图片模型报价修复落在 dev-zz 共享可用渠道目录，而不是恢复上游旧 `channel_plaza`：分组图片档位、独立倍率通过客户安全 DTO 投影，模型广场、价格表格和导出复用同一价格解析；单张图片价格只来自按次 / 档位字段，不把图片输出 Token 单价当按次价。
- 订阅续费使用仓储锁串行化；OpenAI WebSocket 租约丢失保留 terminal event，提示词审计解析 Responses output text；管理消费排行显示用户名，Anthropic OAuth endpoint 和 Grok CLI 固定版本同步更新。
- 登录 / 注册页面继续不展示 LinuxDo 和微信入口，但保留后端、回调页与共享组件能力，避免入口策略被上游模板冲突覆盖。

### 数据与兼容性

- 无数据库迁移。后端新增腾讯云、阿里云验证码 SDK 依赖；默认 CSP 增加对应官方脚本、样式与 frame 来源。
- `VERSION` 保持 `1.7.27`；上游 `0.1.171` 不进入 fork 发布线。
- 客户目录分组 DTO 新增图片独立倍率与档位价字段；旧响应字段缺失时继续按既有分组倍率和渠道定价工作。

### 验证

- 前端全量 259 个测试文件、1739 条测试通过；新增共享图片报价单测覆盖分组 `1K` 覆盖、`2K` 默认按次回落、`4K` 渠道档位回落、独立倍率和输入不可变。
- 后端默认与 `unit` 全量测试、`go vet ./...` 和 server build 通过；共享目录 handler / service 用例覆盖图片配置投影。
- 前端 typecheck、全量 ESLint、生产构建和 docs-site 生产构建通过；Wire 已按合并后 provider graph 重新生成。

## 2026-08-03 - 可用渠道价格表格统一客户生效价

### 目标

- 让登录后的“模型广场”和“价格表格”使用同一价格计算口径，不再把未乘分组倍率的渠道基础价直接展示为用户价格。
- 保持同一模型跨渠道、计价分组和阶梯的报价可独立比较，避免多分组共用一行时任意选择倍率。

### 主要变化

- `buildAvailableChannelCatalogRows` 按“渠道 + 模型 + 可调用计价分组 + 阶梯”拆行，并使用 `route_group_ids` 排除当前模型不可调用的分组。
- 表格输入、输出、缓存、图片和按次价格统一应用“用户专属倍率优先、否则分组默认倍率”；价格列排序与 Excel 导出复用同一生效价格 getter。
- 模型广场与表格共享分组生效倍率解析函数，并进一步共享可调用分组兼容逻辑：`route_group_ids` 权威优先，旧响应才回退 `supported_endpoints[].group_ids`，两个字段均缺失时保留回滚兼容行为。
- 管理员全量目录为每个模型投影不省略的 `route_group_ids`，并与当前平台区段分组取交集；无稳定路由模型继续保留为诊断数据但返回 `[]`，生效报价导出不再为其生成不可调用分组价格。管理员导出没有目标用户上下文，因此对可调用分组使用默认倍率。
- 中英文表头、提示和导出列明确标注计价分组与生效价，功能文档同步更新。

### 数据与兼容性

- 管理员全量目录响应新增权威 `route_group_ids` 数组；数据库、真实计费和依赖不变。渠道基础定价继续作为前端计算输入，但不再直接作为用户价格展示。
- 峰值倍率行为保持与现有模型广场一致，本补丁不改变请求最终落账规则。

### 验证

- 可调用分组共享逻辑、可用渠道价格行、价格表格组件和模型广场 4 个定向前端测试文件共 24 条测试通过；新增组件回归直接验证用户专属 `0.5x` 时表格显示与模型广场一致的 `$0.4 / $2`，而不是基础价 `$0.8 / $4`。管理员 handler 单测覆盖投影输入去重排序、平台区段交集与无路由模型序列化为 `[]`。
- 前端全量 254 个测试文件、1695 条测试通过；typecheck、全量 ESLint 和生产构建通过。
- 后端全部 handler 单测、docs-site 生产构建和 whitespace 检查通过。

## 2026-08-03 - 上游 main 同步：利润控制、账单倍率治理与管理端批量能力

### 目标

- 将 `main@825ca7b1f` 合入 `dev-zz-develop`，吸收分组利润准入、账单探测倍率回写、OpenAI / Anthropic usage 可靠性、认证 / 支付 / SMTP 正确性、compact Home 和筛选结果全选。
- 保持 dev-zz 企业成员原子预算 / 计费 / 归因、模型多协议交付、账号归档、共享公开模型目录、长期证据、stone / neutral / emerald 视觉和 `1.7.25` 版本线不回退。

### 主要变化

- 分组可按平台开启利润控制并配置最低利润率与安全缓冲；调度在账号 slot 复核后按分组客户倍率和账号成本倍率 veto 不合格候选，普通、OpenAI、图片、WebSocket 和 failover 路径共用同一 gate。新增预览命令、管理端表单和 activity 指标。
- 上游账单探测扩展到多 API Key 平台；只有显式允许、探测可信且倍率在上限内时才写回账号 `rate_multiplier`，列表、编辑和批量探测会刷新最新倍率。
- OpenAI 补齐 reset-credit 缓存 / 恢复、Messages 临时错误切换、SSE `429`、请求取消、负载削峰、namespace 工具和工具输出媒体；Anthropic 中断流保留已观察 usage。Responses → Chat 仍要求 client 工具显式声明 `execution=client`。
- 普通计费失败继续写 `ActualCost=0` 的未结算 usage 证据；企业成员因预算结算、usage 和最终归因必须原子化，任何事务仓储缺失 / 失败继续 fail-closed，不做独立 usage 写入。
- 管理端账号支持按完整筛选结果全选并作用于既有批量编辑、探测和归档；Home 可启用 compact preset。账号仍以归档代替页面硬删除，公开模型列表仍复用共享 marketplace，不恢复上游已被替代的旧价格组件或倍率筛选。
- 刷新 token 轮换避免并发竞态；退款余额不足要求显式强制确认，Stripe refund 保持幂等；SMTP 发送 / 测试共用连接路径，内容审核可使用配置代理，提示词审计支持窄范围阻断。

### 数据与兼容性

- 新增 `192_group_profit_control.sql` 与 `193_group_profit_control_auth_cache_invalidation.sql`；Group schema、Ent 生成物与 API DTO 增加利润字段。
- 认证快照版本为 `v21`，合并保留利润控制、计价、企业成员和 OpenAI Live 字段并强制旧缓存回源。
- `VERSION` 保持 `1.7.25`，不采用上游 `0.1.170`；没有新增前端或 Go 依赖。

### 验证

- 后端 service `unit` 全包通过；handler、admin handler、routes、`apicompat` 和 migrations 的关键定向 / `unit` 测试通过。
- 前端冲突相关 8 个测试文件、109 条测试与关键回归 8 个测试文件、133 条测试通过；typecheck、ESLint 和生产构建通过。
- Ent / Wire 重新生成、后端二进制构建、docs-site 生产构建、Go 格式、whitespace 和冲突标记检查通过。
- 真实 PostgreSQL / Redis、上游账单 / 模型流、支付 / SMTP / 代理、浏览器和 Docker 运行时 smoke 未执行。

## 2026-07-31 - 上游 main 同步：网关安全、订阅窗口与支付配置正确性

### 目标

- 将 `origin/main@d29acc29a` 合入 `dev-zz-develop`，吸收网关路径安全、OpenAI / Grok 流式容错、订阅额度窗口、支付 patch、邮件 / 图片、Composite 目录、价格和容器权限修复。
- 保持 dev-zz 企业成员路由 / 预算 / 归因、WebSocket 首轮路由锁、真实模型交付目录、长期证据和 `1.7.25` 版本线不回退。

### 主要变化

- Responses wildcard 子路径与 Gemini 上游 URL 拒绝不可转发路径片段；安全守卫在企业成员和 Composite 候选调度前执行，合法请求仍走原有成员预算、能力选择和最终分组归因。
- OpenAI 代理断流熔断增加配置开关、突发错误合并和容量耗尽 fail-open；fail-open 第二次选择继续透传渠道定价限制和 sticky 绑定。Pool 模式可重试流式容量错误，缺失 passthrough instructions 时补默认值。
- Grok billing ping 被转换为标准 SSE 注释，并对过滤缓冲和 body close 错误增加保护；pool 模式的默认冷却与 entitlement `403` 处理统一，不再错误暂停可继续使用的账号。
- 订阅日 / 周 / 月额度窗口从真实订阅锚点推进，并在到期边界停止自动重置；旧首个午夜锚点可归一到开始时间。前端用同一窗口计算展示剩余时间与到期标签。
- 支付配置只写入请求中显式携带的字段，保留未修改的可见支付方式；支付方式选择器和套餐卡约束窄屏布局。SMTP 输出标准 CRLF / 折行 / dot-stuffing，图片任务可解码 data URL 后转存。
- Composite 分组在用户目录按已配置模型的平台展开，但仍服从稳定账号交付和协议端点证据；同步 GPT-5.6 Luna / Terra、GLM-5.2 fallback 价格和管理员 Composite 模型展示。

### 数据与兼容性

- 没有数据库迁移、Go / 前端依赖变更；新增 `gateway.openai_proxy_stream_circuit.disabled`，默认 `false`，即继续启用现有熔断。
- Docker / Compose 增加 `no-new-privileges`，release 资源包含模型价格 fallback，CI 执行对应部署安全脚本。
- Ops cleanup 仍要求 `auto_cleanup_enabled=true` 才会删除数据，默认关闭；本轮仅把成功日志改为结构化 info。
- `VERSION` 保持 `1.7.25`，不采用上游 `0.1.169`。

### 验证

- 冲突相关 routes / handler 测试通过，覆盖 Responses 子路径守卫、Composite 目录展开、稳定交付投影和 WebSocket 路由锁。
- 代理熔断 fail-open 定向测试通过，覆盖全部代理隔离时恢复容量和第二次选择的渠道定价预检策略透传。
- 后端 `go test -tags=unit ./... -count=1` 全包通过。
- 支付 / 订阅 / 设置前端定向测试 5 个文件、62 条测试通过；两个部署安全合同脚本通过。
- 前端 typecheck、ESLint 与生产构建、docs-site 生产构建通过；Go 格式、whitespace 和冲突标记检查通过。
- 真实 SMTP、代理 / Grok 上游、浏览器与 Docker 运行时 smoke 未执行。

## 2026-07-30 - 公开模型列表复用真实可交付目录

### 目标

- 让访客注册前即可在独立 `/model-plaza` 页面查看本站公开提供的模型、价格和客户可调用 API 端点。
- 保持公开目录和登录后的 `/available-channels` 使用同一套稳定交付判断，同时严格隔离专属、订阅和用户专属倍率。

### 主要变化

- 后端从可用渠道 handler 提取共享客户目录构建逻辑；公开接口只筛选 active、standard、非专属分组，再复用同一 `route_group_ids` 与 `supported_endpoints` 投影。
- `GET /api/v1/model-plaza` 不再按用户计算分组或倍率。匿名与登录请求返回相同 `description + channels` DTO；存量 `model_plaza_require_auth` 只控制访问门禁。
- 公开页复用 `AvailableModelMarketplace` 和同组同模型聚合工具，按公开分组默认倍率展示价格；登录后的可用渠道页按当前用户的生效倍率展示。多价格方案继续明确展示方案数量，不擅自挑最低价。
- 删除已被共享目录投影完全取代的旧 `channel_plaza` 分组聚合与官方参考价服务，避免后端继续保留第二套模型目录规则。
- Home 导航和 Models 区域在功能启用时链接 `/model-plaza`；功能关闭时隐藏入口。公开页保留独立导航、搜索、平台/分组下拉筛选、加载、空状态与失败重试，复用通用 Select 的键盘和禁用态能力，并使用 stone / neutral / emerald 视觉；桌面筛选栏保持单行，倍率只作为价格信息而不再占用筛选项。
- 公开页独立导航只承担返回语义：登录用户显示“返回控制台”并按角色进入用户或管理员控制台，未登录访客显示“回到首页”并进入 `/`。

### 数据与兼容性

- 没有数据库迁移、依赖或真实网关转发变化；渠道定价仍负责发布和价格来源，账号/分组能力仍负责证明稳定交付。
- 公开响应不包含上游账号、上游 URL、供应商、管理员成本、故障转移拓扑、用户专属分组或用户专属倍率。
- `model_plaza_enabled=false` 时接口继续返回 404；要实现注册前可见，生产配置需要 `model_plaza_enabled=true` 且 `model_plaza_require_auth=false`。

### 验证

- 后端覆盖公开分组严格过滤、匿名/登录响应相等、共享端点投影、DTO 白名单和关闭时 fail-closed；`make test-unit` 通过。
- 前端覆盖 Home 双入口和功能开关、公开分组过滤、搜索、倍率价格、端点及错误重试；全量 Vitest、typecheck、ESLint 和生产构建通过。

## 2026-07-30 - 上游 main 同步：OpenAI Live store 容错与前端状态修复

### 目标

- 将 `origin/main@5a6143097` 合入 `dev-zz`，吸收 OpenAI Live store 故障恢复、usage 可靠落库、Claude Sonnet 5 状态别名和 Passkey 禁用态提示修复。
- 保持 dev-zz 企业成员 Live 身份与归因、脱敏结构化失败证据、现有 Passkey 开关边界和 `1.7.23` 版本线不回退。

### 主要变化

- Live observer 在 controller claim 或 call / controller 读取遇到 store 故障时有限重试；持续失败后持有原始 call record 等待 `ExpiresAt`，再通过幂等 finalize 释放租约并补写 usage，避免 Redis 抖动让会话静默消失。
- Live finalize 先尝试仓储 best-effort writer，队列超时或失败时换用同步 `Create` 兜底；最终失败继续记录脱敏的 call hash、account、API Key 和 user 数字 ID，不记录原始 call ID、凭据或 attestation。
- 企业成员 Live call 的 `MemberID`、成员编号 / 名称快照现在由生产 Redis gateway cache 显式保存并恢复，最终继续写入 usage；sideband 仍要求调用身份与原 call 的成员身份一致。
- 管理端账号状态增加 Claude Sonnet 5 短别名 `CSon5`；Passkey 功能关闭时资料页跳过凭据查询，设置切换竞态返回 `PASSKEY_DISABLED` 时不再弹出加载失败。

### 数据与兼容性

- 本轮没有数据库迁移、依赖声明、API payload 或配置键变化。
- Live 仍保持零金额 usage 证据与默认关闭的分组 gate；本轮只加固 controller / store 故障恢复，不改变调度、计费、并发限制或最大会话时长。
- `VERSION` 保持 `1.7.23`，不采用上游 `0.1.168`。

### 验证

- Live repository / service 定向测试及 service 包全量测试通过，包含 Redis 成员字段跨实例往返、store fault、到期 finalize、best-effort 同步回退、企业成员快照与失败日志合同。
- AccountStatusIndicator 6 条定向测试、前端 typecheck 与 ESLint 通过。
- docs-site 生产构建、Go 格式、whitespace 和冲突标记检查通过。

## 2026-07-28 - 上游 main 同步：Passkey、模型价格橱窗与字段级更新

### 目标

- 将 `origin/main@8fd01c281` 合入 `dev-zz`，吸收 Passkey、模型价格橱窗、User / API Key 并发更新保护和协议正确性修复。
- 保持 dev-zz 企业成员路由 / 预算 / 归因、模型多协议能力、Messages 显式模型映射、模型状态授权、现有可用渠道目录和 `1.7.21` 版本线不回退。

### 主要变化

- 用户可在资料页注册和撤销 Passkey，并使用 Passkey 登录；敏感管理动作要求当前密码，配置不完整或部署条件不满足时 fail-closed。管理员可独立控制 Passkey 登录入口。
- 新增默认关闭的 `/model-plaza` 价格橱窗，可按分组展示渠道价和官方参考价，并选择公开访问或强制登录；它不替换 `/available-channels`，后者继续按真实可调度账号和端点展示用户可用模型。
- User / API Key 更新改为显式列集合，避免并发的资料、余额、额度、状态、标签或企业设置互相覆盖。合并后继续支持 dev-zz `account_type`、企业停用标记和 API Key `tags`。
- OpenAI Messages 桥接会为最终 GPT-5.6 模型保留 `max` reasoning effort；Kimi K3 / 1M 后缀、Codex Web Search、Anthropic cache breakpoint、安全审计配置恢复、setup bypass 和模型 ID 复制修复同步合入。

### 数据与兼容性

- 新增 `backend/migrations/191_passkey_credentials.sql`；它与 dev-zz 既有同号迁移按完整文件名并存，不修改任何历史迁移。
- `VERSION` 保持 `1.7.21`；模型价格橱窗和 Passkey 登录默认关闭，升级不会自动新增公开入口或改变现有登录方式。
- 字段级更新只改变仓储写入范围，不改变现有 API payload；企业成员归属仍不能通过普通 API Key 更新路径修改。

### 验证

- 后端冲突相关定向测试、unit / 默认标签全包、依赖元数据检查和 golangci-lint 通过。
- 前端定向测试、全量 242 个测试文件 / 1602 条 Vitest、typecheck、ESLint 和生产构建通过。
- docs-site 生产构建、生成代码、Go 格式、whitespace 和冲突标记检查通过。

## 2026-07-28 - OpenAI Messages 分组模型映射显式化

### 问题

- 分组编辑弹窗会在 Opus、Sonnet、Haiku 映射为空时重新填入固定 GPT 模型，管理员无法表达“不要系列改写，保留客户端模型名”。
- 固定默认值适合旧 OpenAI / Codex 分组，但会误导 DeepSeek、MiniMax 等通过 OpenAI 账号接入、同时原生支持 `/v1/messages` 的上游。
- 目标模型只能自由输入，没有按当前分组账号路由范围提供候选，容易保存一个本分组没有账号能够匹配的逻辑模型。

### 修复

- 为 `messages_dispatch_model_config` 增加 `family_mapping_mode`：`passthrough` 表示未命中精确覆盖时原样透传，`custom` 表示使用显式系列映射；自定义模式下某个系列留空也会透传。
- 精确模型覆盖继续具有最高优先级。运行时保持确定性，不在映射目标失败后隐式改用原始模型重放。
- 新分组默认显式透传；旧分组缺少模式字段时继续使用历史 GPT 默认映射，并在编辑界面提示管理员选择迁移方式，避免升级后改变存量请求。
- 透传配置在服务端规范化时会清空无效的系列目标字段，避免存储“看似配置、实际不生效”的残留值。
- 新增 `GET /api/v1/admin/groups/:id/messages-dispatch-model-candidates`，只聚合当前分组可调度 OpenAI 账号 `model_mapping` 的具体左侧键，排除通配规则和其它平台账号。编辑页以这些逻辑模型提供输入建议，仍允许手工输入，并对不在候选中的值显示提示。

### 兼容性

- 配置继续存放在现有分组 JSONB 中，不新增数据库列或迁移。
- 缺失 `family_mapping_mode` 的历史数据保持原有 `gpt-5.4`、`gpt-5.3-codex`、`gpt-5.4-mini` 系列映射；只有管理员显式保存新模式后才采用新语义。
- `AllowMessagesDispatch` 仍是 `/v1/messages` 的分组准入开关；本补丁只调整模型名如何进入后续账号映射与最终上游模型解析。

### 验证

- 增加旧版兼容、显式透传、部分自定义、精确覆盖优先、无效模式拒绝、候选范围和完整映射链回归测试。
- 增加前端默认值、旧配置水合、新模式序列化、预设应用和候选范围提示测试；完整前后端验证结果以提交前实际执行记录为准。

## 2026-07-28 - 账号协议能力按模型映射收敛

### 问题

- 账号配置了少量模型映射后，协议能力同步仍把上游 `/v1/models` 返回的完整目录写入观察结果，前端又直接以全部观察记录生成矩阵，导致同分组但未参与该账号映射的模型也出现在配置弹窗中。
- 保存覆盖成功后前端只刷新数据和提示成功，没有触发关闭事件，因此管理员点击保存后弹窗仍停留在页面上。
- 既有测试只校验覆盖请求字段，没有覆盖保存后的弹窗生命周期，也没有约束能力列表必须服从账号模型映射。

### 修复

- 账号存在显式 `model_mapping` 时，以映射右侧的最终上游模型作为协议能力范围；同步只写入这些目标模型的观察，GET / PUT / POST 响应也只返回目标模型与账号默认 `*` 能力。
- 空映射仍表示账号允许全部模型：同步和手工精确模型入口保持原行为，不把本轮收敛错误应用到透传账号。
- 映射范围由服务端通过 `models` 和 `mapping_restricted` 返回；前端以该范围生成矩阵，并在映射受限时隐藏“手工添加任意上游模型”入口。
- 旧的无关观察记录不执行数据删除；它们不再进入当前弹窗或覆盖 payload，也不会影响只按最终上游模型精确查询的路由判定。
- 覆盖保存成功后显示成功提示并发出 `close`，失败时继续保留弹窗和错误信息。

### 验证

- 新增“同步忽略未映射上游模型”“响应范围只包含映射目标”“前端不渲染服务端范围之外的记录”和“保存成功关闭弹窗”回归测试。
- 本轮不新增数据库迁移；完整前后端验证结果以提交前实际执行记录为准。

## 2026-07-28 - 用户模型状态按可用分组授权

### 问题

- 用户侧模型状态接口虽然要求登录，但服务层被实现为全站状态视图；列表会返回所有启用自检的活跃分组，详情也允许按任意 `group_id + model` 读取。
- 现有 DTO 只隐藏了账号、供应商、上游地址和原始错误，没有执行专属分组与订阅分组授权，因此新用户也能看到未授权分组名称、模型目录和健康时间线。

### 修复

- 列表与详情从认证上下文取得当前 `user_id`，由 `ModelSelfCheckService` 复用 `APIKeyService.GetAvailableGroups` 解析可见范围。
- 可见性与 API Key 绑定及用户可用渠道保持一致：公开标准分组可见；专属分组需要 `user_allowed_groups` 授权；订阅分组需要有效订阅。
- 状态目标在加载账号、历史与时间线之前按可用分组过滤；直接读取未授权详情返回 not found，授权查询失败时 fail-closed。
- 后台自检任务与状态快照刷新继续读取全部启用目标，不受用户读取范围影响；没有迁移或前端兼容变更。

### 验证

- 新增列表只返回当前用户分组、详情拒绝越权分组、授权查询失败不返回数据、列表与详情要求认证身份的回归测试。
- `go test ./... -count=1`、`make test-unit`、`go vet ./internal/service ./internal/handler ./cmd/server`、`golangci-lint run ./...` 与 `git diff --check` 通过。

## 2026-07-27 - 上游 main 同步：面板 API 分层限流

### 目标

- 将 `origin/main@dc893dd0b` 合入 `dev-zz-develop`，吸收按用户和公开客户端 IP 保护面板 API 的可配置限流，避免高频列表、用量和仪表盘查询持续占用数据库。
- 保持 dev-zz 企业成员预算与候选组编排、API Key 自助查询、模型级限流、owner 用量分析、设置保存边界和管理端视觉不回退。

### 主要变化

- 认证面板接口使用用户 ID 作为全局限流主体，不按反向代理或共享出口 IP 合并不同用户；usage 等聚合查询在全局档位之外叠加 Heavy 档位。管理员默认豁免，可在设置中关闭豁免。
- 无需认证的公开设置接口只对安全解析后的公网单播 IP 计数；回环、内网、链路本地和未指定地址跳过，避免把反向代理内部地址当成所有访问者的共同限流主体。
- 默认配置为启用、每用户 240 RPM、Heavy 60 RPM、公开 IP 300 RPM、管理员豁免。任一 RPM 设为 `0` 可关闭对应档位，非法负数或超过 100000 的保存请求会被拒绝。
- Redis 限流错误 fail-open；配置通过 60 秒进程缓存和 singleflight 避免热路径逐请求访问数据库，读取失败保留最近成功值并用 5 秒短 TTL 重试，当前节点保存后立即刷新缓存。
- 冲突合并后同时保留模型级限流和面板 API 限流设置；API Key 日、趋势、模型统计与 owner `/usage` 聚合查询都进入 Heavy 档位，企业成员预算服务继续传入 Gateway 路由。
- 独立注册的 `/admin/payment` 路由组也接入 Global 档，关闭管理员豁免后不会形成绕过路径。

### 数据与兼容性

- 配置保存在 DB setting `panel_rate_limit_settings`，不需要数据库迁移；缺失、空值或无效 JSON 回退到安全默认值。
- API Key 网关请求不进入面板限流；`/api/v1/key/*` 自助查询继续使用原有 fail-close 专用限流，不改成用户 ID 桶。
- 本轮没有依赖声明、版本号或 GitHub Actions workflow 变化；上游 README 赞助商列表与静态 logo 同步更新。

### 验证

- 面板限流 middleware、设置 service、handler 和 route 定向测试通过；dev-zz 路由测试已适配新的显式 limiter 参数。
- 后端 `go test -tags=unit ./... -count=1`、`go test ./... -count=1`、`go mod tidy -diff` 和 `golangci-lint run ./...` 通过。
- 前端 SettingsView 定向测试、typecheck、ESLint、全量 Vitest 和生产构建通过；docs-site 生产构建通过。
- `git diff --check`、冲突路径和冲突标记检查通过。浏览器人工限流 smoke、真实 Redis 压测和 Docker / Testcontainers 运行时集成测试未执行。

## 2026-07-27 - 上游 main 同步：Antigravity 原生兼容与下拉边界

### 目标

- 将 `origin/main@d96b6a31f` 合入 `dev-zz-develop`，吸收 Antigravity OAuth 的 OpenAI Chat Completions / Responses 原生兼容转发、Hermes Web Search 判定、分组说明排版和 Select 视口边界修复。
- 保持 dev-zz 企业成员路由、预算和归因、模型原生多协议 `DeliveryDecision`、账号失败证据、stone / neutral / emerald 视觉及版本线不回退。

### 主要变化

- Antigravity OAuth 账号可把 OpenAI Chat Completions / Responses 请求转换到原生 `v1internal:streamGenerateContent`，并分别把流式事件和非流式结果还原为 OpenAI 兼容响应；响应 usage 继续进入既有计费和用量记录。
- Antigravity 响应只有 usage、没有文本、工具调用或其他可交付内容时不再被当成成功，而是进入现有账号 failover；Responses 在组内账号因凭据或容量问题耗尽且响应尚未提交时会标记企业成员跨组重试，已经开始流式输出时禁止重放。
- 上游凭据拒绝消息保持脱敏，账号冷却和错误归因使用实际尝试的 endpoint。
- Gemini Messages 兼容层区分显式服务端 Google Search 与 Hermes 风格的客户端 `web_search` function：前者转换为 `googleSearch`，后者保留函数声明和参数。
- 分组说明支持显式换行、超长连续文本断行和最多三行截断。Select 下拉层会根据视口 padding 夹紧左边界并收缩右边界，同时继续使用捕获阶段 document 监听保证祖先节点阻止冒泡时 outside click 仍然生效。

### 数据与兼容性

- 本轮没有新增数据库迁移、依赖声明、版本号或 GitHub Actions workflow。
- Antigravity 兼容只作用于对应 OAuth 平台；其他 OpenAI / Gemini 转发仍沿用原有协议选择、调度、冷却和计费路径。
- 企业成员候选编排、预算结果不明保护、最终 `ActiveGroup` 归因以及 owner / admin 数据边界没有放宽。

### 验证

- 后端 Antigravity / Gemini / endpoint / credential / Web Search 定向测试、`go test -tags=unit ./... -count=1`、`go test ./... -count=1`、`go mod tidy -diff` 和 `golangci-lint run ./...` 通过。
- 前端 Select / GroupOptionItem 定向测试、typecheck、ESLint、239 个测试文件 / 1579 条 Vitest 和生产构建通过；docs-site 生产构建通过。
- `git diff --check`、`git diff --cached --check`、冲突路径和冲突标记检查通过。
- 浏览器人工 smoke 与 Docker / Testcontainers 运行时集成测试未执行。

## 2026-07-27 - 上游 main 同步：设置局部更新、协议兼容与支付统计

### 目标

- 将 `origin/main@95590b553` 合入 `dev-zz-develop`，吸收设置局部更新保护、显式 `CONFIG_FILE`、管理员用量 `request_id` 筛选、Responses / Anthropic 兼容修复、支付统计分币种和监控时间线窄卡片修复。
- 保持 dev-zz OpenAI Fast/Flex 策略原子保存、企业成员可见性边界、模型协议能力选择、stone / neutral / emerald 视觉和账号行操作密度调整。

### 主要变化

- 设置保存合并为“omitted keys + auth source defaults + OpenAI Fast/Flex policy”单次写入；局部 payload 不再把未提交字段写成零值，策略变更仍会先校验、规范化并写审计。
- 管理员用量查询新增 `request_id` 精确筛选，同时继续执行企业成员 `MemberID` / `MemberScope` / owner visible member 约束；mapped model 与最终上游模型统计沿用上游修复。
- Responses / Anthropic 兼容层接入 function_call_output 的 JSON / string 双形态解析、tool / prompt cache 修复和跨账号 failover 的 reasoning 清理；OpenAI Responses 转发继续通过协议能力选择器决定上游协议。
- 支付统计接口与后台图表按币种分组展示收入、支付方式和用户排行；监控时间线在保留自定义 tooltip 的同时吸收 `min-w-0` 溢出修复。

### 数据与兼容性

- 本轮没有新增数据库迁移。
- 依赖更新来自上游 `go.mod` / `go.sum`；版本线不在本轮提升。
- 局部设置 payload 的 omitted keys 只表示“不修改”，显式传入的 false、0 或空集合仍按原设置语义保存。

### 验证

- pnpm 10.34.5 frozen install、前端 typecheck、ESLint、238 个测试文件 / 1574 条 Vitest、生产构建和 docs-site 构建通过；CI 保持 Node 20，并使用能够读取 workspace overrides 的 pnpm 10。
- 后端冲突相关包定向测试、主要包测试、`go test -tags=unit ./... -count=1`、`go test ./... -count=1`、`go mod tidy -diff` 和 `golangci-lint run ./...` 通过。
- `git diff --check`、`git diff --cached --check` 和冲突路径检查通过。
- 浏览器人工 smoke 与 Docker / Testcontainers 运行时集成测试未执行。

## 2026-07-27 - 渠道映射与定价一致性及显示排序

### 问题

- 渠道模型映射和模型定价分开维护，保存前只校验各自内部的重复与通配符冲突；映射数量较多时，管理员容易遗漏定价模型，开启“限制模型”后会导致对应请求被拒绝。
- 原“同步最新模型”面向 LiteLLM 定价目录，不会核对当前渠道映射。模型映射使用 JSONB 对象、模型定价按数据库 ID 返回，页面没有可靠的自定义显示顺序。

### 修复

- 每个平台展示映射模型、已覆盖、缺失和额外定价数量；映射行直接标记“已定价 / 缺失定价 / 无法判断”，缺失模型可逐条补齐，或在确认映射后显式点击“快速定价”一次性处理。
- 覆盖计算跟随计费基准：requested 使用映射源模型，channel_mapped 使用映射目标模型；upstream 不根据渠道映射伪造结论。精确模型、尾部通配符和大小写复用运行时兼容语义。
- 填写、修改或删除映射都不会隐式增删定价，避免错误映射留下孤立定价。只有管理员主动触发“快速定价”时才追加尚未覆盖的模型，并尝试复用默认定价、合并价格完全一致的模型；开启“限制模型”时缺失覆盖阻止保存，未开启时只显示警告。
- 映射和定价加入拖动手柄，并提供按名称、按映射顺序整理；定价顺序在每个平台内独立编号，只是管理端展示属性，不改变模型匹配或定价优先级。

### 数据与兼容性

- 新增迁移 `198_channel_pricing_display_order.sql`：`channels.model_mapping_order` 单独保存各平台映射键顺序，`channel_model_pricing.sort_order` 保存平台内定价展示顺序。
- 旧渠道定价按原 ID 顺序回填；缺少映射顺序的旧客户端或旧数据会保留仍存在的键，并把新增键按自然模型名追加。旧客户端省略新字段时仍可继续创建和更新渠道。

### 验证

- 前端覆盖推导、计费基准、通配符、限制模式和自然排序单元测试通过；全量 238 个测试文件、1574 个测试通过，TypeScript 类型检查、ESLint 与生产构建通过。
- 后端 `go test ./...` 以及 service、repository、admin handler 和 migration 的 `-tags=unit` 测试通过；迁移合同覆盖向后兼容字段、旧数据排序回填及不影响匹配优先级的约束。
- `docs-site` 生产构建通过。
- 浏览器人工拖动与真实 PostgreSQL 迁移 smoke 未执行。

## 2026-07-27 - 上游 main 同步：WebSocket 轮次计费与运行时正确性

### 目标

- 将 `origin/main@eb6e3d1f1` 合入 `dev-zz-develop`，吸收 OpenAI Responses WebSocket 逐 turn 模型 / 计费证据、提示词审计配置可用性、Grok `402` 冷却、管理员用量筛选、注册返佣码和 Caddy SSE 修复。
- 保持 dev-zz 企业成员同步落账与预算结果不明、Composite 候选路由、WebSocket 首轮路由锁、模型广场、长期证据、fork 发布、stone / emerald 视觉和 `1.7.18` 版本线不回退。

### 主要变化

- OpenAI Responses WebSocket 为每个 turn 记录实际请求模型、上游模型、渠道映射、计费模型和调度结果；图片工具返回的独立计费模型继续优先，渠道可按 requested / upstream / mapped 口径覆盖。
- dev-zz 继续把一个 WebSocket 连接固定到首轮最终公共模型和上游路由：后续 turn 可以省略或重复同一模型，但模型、平台或渠道目标变化必须重连。逐 turn 统计使用连接真正转发的路由，客户端策略拒绝不计为账号故障。
- 企业成员每轮预算 context、payload hash、结果不明标记、同步 usage 落账和最终 `ActiveGroup` 归因保持不变，并与新的 turn 级模型证据共同写入。
- 提示词审计配置没有可信运行快照时返回 `prompt_audit_config_unavailable`；Grok 手工连接测试遇到 `402` 后暂停账号 30 分钟。
- 管理员用量路由筛选会回填用户邮箱，并以筛选 ID 和搜索 revision 防止迟到响应覆盖新输入。注册页在返佣开启且强制邀请码关闭时展示可选邀请码，沿用 stone 视觉。
- Caddy 不再压缩 `text/event-stream`，避免 SSE 响应被缓冲；便携检查脚本纳入 CI。旧 `AvailableChannelsTable.vue` 继续保持删除，用户模型广场仍是唯一目录入口。

### 数据与兼容性

- 本轮没有新增数据库迁移、依赖声明或版本号变化。
- `VERSION` 保持 `1.7.18`；不采用上游旧版可用渠道表，也不放宽 WebSocket 连接内切换模型 / 平台的边界。

### 验证

- 后端完整 unit、Handler / 提示词审计 / Service / WebSocket v2 包测试和 `golangci-lint` 通过；Caddy SSE 合同脚本通过。
- 前端 typecheck、ESLint、237 个测试文件 / 1567 条 Vitest 和生产构建通过；docs-site 生产构建、冲突标记、Go 格式和 whitespace 检查通过。
- 浏览器人工 smoke 与 Docker / Testcontainers 运行时集成测试未执行。

## 2026-07-26 - 上游 main 同步：OpenAI Live、会话证据与注册安全

### 目标

- 将 `origin/main@2730c1c43` 合入 `dev-zz-develop`，吸收 OpenAI Live、客户端会话证据、注册邮箱别名安全、Ollama Cloud 刷新、公告预览和网关正确性修复。
- 保持 dev-zz 企业成员同步落账、预算结果不明、请求级实际分组归因、Composite / 原生多协议、长期证据、fork 发布、stone / emerald 视觉和 `1.7.17` 版本线不回退。

### 主要变化

- OpenAI 分组新增默认关闭的 `allow_live`；支持 `/v1/live`、`/backend-api/codex/realtime/calls` 及 sideband 查询，Live usage 使用独立请求类型，租约失效会终止会话并补齐可证明的 usage 记录。macOS attestation 只在当前运行环境具备能力时生成，管理端开启前可以探测并确认。
- 企业成员 Live create 会按有序候选逐组解析，Composite 分组从 JSON / multipart 的 `session.model` 解析并重写最终 OpenAI 模型；非 OpenAI 或未开启 Live 的候选只在响应尚未提交时安全切换。sideband 不重新创建调用，只按当前授权候选匹配已持久化的 call 身份。
- Live 调用和最终 usage 证据保留成员 ID、成员编号 / 名称快照与实际 group；usage 仓储失败会记录不含原始 call ID、凭据或 attestation 的结构化错误，不再静默形成证据缺口。
- Gateway、OpenAI、Gemini、图片和 batch image 路径统一提取显式客户端会话头，经 UTF-8、控制字符和 255 字符上限校验后写入 `usage_logs.session_id`；该证据不参与 sticky、调度、计费、请求幂等或上游 prompt cache。
- session 证据与企业成员 `member_id` / 名称快照、最终 `ActiveGroup`、`schedule_meta`、请求载荷指纹和预算请求 ID 同时保存；企业成员与图片用量的同步 / mandatory 持久化合同不因上游异步写入改动而回退。
- Ollama Cloud 按模型请求活动刷新、debounce、公平候选与 PostgreSQL 16 due 判断修复合流；注册邮箱别名查重增加规范化、并发保护和表达式索引。
- 公告管理增加预览动作；Bell 与 Popup 共用富文本样式，同时继续采用 dev-zz stone / emerald 主题。同步 postcss 安全升级、移动端返佣复制和 OpenAI / Grok / Gemini 兼容修复。

### 数据与兼容性

- 新增 `187_add_usage_log_session_id.sql`、`188_allow_live_usage_request_type.sql`、`189_add_group_allow_live.sql` 和 `190_add_users_email_alias_dedup_index_notx.sql`。
- 四个迁移与 dev-zz 既有同号企业成员迁移按完整文件名并存，不改写已应用迁移；`session_id` 和 batch image session 均为 nullable，Live 与分组开关默认关闭。
- `VERSION` 保持 `1.7.17`，不采用上游 `0.1.165`。

### 验证

- 冲突标记、whitespace、Go 格式与生成物、后端 Handler / Repository / Service / Routes、迁移 schema、前端公告 / 分组 / 用量回归、typecheck、lint、生产构建和 docs-site 构建纳入本轮合并验证。
- 浏览器人工 smoke 和 Docker / Testcontainers 运行时集成测试未执行；若本机容器可用，迁移 schema 另以真实 PostgreSQL 集成测试确认。

## 2026-07-23 - 上游 main 同步：Ollama Cloud 用量与支付宝移动唤起

### 目标

- 将 `origin/main@cd8bb98c4` 合入 `dev-zz-develop`，吸收 Ollama Cloud 官方用量观察、支付宝移动端当面付唤起和网关正确性修复。
- 保持 dev-zz 模型原生多协议、供应商成本、账号归档、企业成员、长期留存、fork 镜像和 stone / emerald 视觉合同不回退。

### 主要变化

- 管理端账号列表为符合条件的 Ollama OpenAI / Anthropic API Key 账号增加官方用量状态；可保存/删除 Web session、手工刷新、开关单账号自动刷新，并通过全局设置控制 runner 和 `15-1440` 分钟刷新间隔。
- Ollama session 使用现有 secret encryptor 保存；没有固定 `TOTP_ENCRYPTION_KEY` 时拒绝持久化。响应、账号列表 DTO、审计和普通日志只返回是否已配置及脱敏快照，不返回 session 或原始设置页面。
- 用量刷新只写账号 `extra` 中的观察快照，不修改账号调度健康、可调度状态、计费、额度或用户目录；同一 Ollama 身份的账号共享托管状态，仓储返回独立深拷贝，避免并发修改共享 map。
- 支付宝官方支付可选择移动端 `precreate` + App Scheme；唤起失败时回退动态二维码，订单轮询和到账确认沿用原合同。环境变量可强制开启，未设置时使用后台设置。
- 同步 OpenAI passthrough 输入归一化、流式代理隔离、模型限流重置展示、Grok 402 冷却、简单模式 Grok 图片、渠道定价模型名归一化和 Codex identity 导入修复。

### 数据与兼容性

- 新增 `186_alipay_mobile_precreate_deep_link.sql`，只写入默认关闭的 `ALIPAY_MOBILE_PRECREATE_DEEP_LINK` setting。
- 新增 `186_group_auth_cache_image_generation.sql`，把 `allow_image_generation` 纳入既有组级鉴权缓存失效触发器。
- 两个迁移与 `186_enterprise_member_removal_lifecycle.sql` 按完整文件名并存；没有重命名、覆盖或修改任何已应用迁移。

### 验证

- 后端目标包、完整 unit、全仓编译和 integration-tagged service / repository 编译通过；`golangci-lint` 为 `0 issues`。
- 前端 typecheck、ESLint、234 个测试文件 / 1547 条测试和生产构建通过。
- Wire 重生成、docs-site 构建、部署脚本语法、Compose 配置和最终冲突/whitespace 检查通过。
- 浏览器人工 smoke 和 Docker / Testcontainers 运行时集成测试未执行。

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
