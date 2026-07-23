# dev-zz 接口索引

这页只记录 dev-zz 新增或语义有差异的接口。上游通用接口仍以项目源码和 `docs/` 兼容文档为准。

## 管理端账号复制与 Grok OAuth 对账

| 方法 | 路径 | 用途 | 关键语义 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/admin/accounts/:id/duplicate` | 复制静态凭据账号 | 要求 `Idempotency-Key`；复制配置、凭据和有序分组，重置运行态并默认不可调度；重复请求按管理员作用域返回同一结果 |
| `POST` | `/api/v1/admin/grok/oauth/reconcile` | 对账 Grok OAuth 账号 | 触发 Grok OAuth 账号状态 / 凭据协调；仅管理员可用 |

账号复制只允许 `apikey`、`upstream`、`bedrock` 和 `service_account` 等静态凭据类型。OAuth、setup-token、未知旧凭据类型以及带 `parent_account_id` 的影子账号会被拒绝，避免复制共享 refresh token 或凭据池身份。复制不会继承错误、限流、过载、临时不可调度、会话窗口、用量投影、被动探测和 CRS 远端绑定等运行态。

## Key 计费倍率自省与上游探测

| 方法 | 路径 | 用途 | 关键语义 |
| --- | --- | --- | --- |
| `GET` | `/v1/sub2api/billing` | 当前 API Key 的倍率自省 | 只需 Key 鉴权，不消耗并发 / 计费配额；返回分组、用户、高峰和最终生效倍率，不暴露账号或供应商成本 |
| `GET` | `/api/v1/admin/accounts/upstream-billing-probe/settings` | 读取周期探测设置 | 管理员接口；默认开启、30 分钟间隔 |
| `PUT` | `/api/v1/admin/accounts/upstream-billing-probe/settings` | 更新周期探测设置 | 间隔限制 `5-1440` 分钟 |
| `POST` | `/api/v1/admin/accounts/upstream-billing-probe/batch` | 手工批量探测 | 单批最多 20 个 OpenAI APIKey 账号 |
| `PUT` | `/api/v1/admin/accounts/:id/upstream-billing-probe` | 开关单账号探测 | 写入账号 `extra`，关闭时清除旧探测快照 |
| `POST` | `/api/v1/admin/accounts/:id/upstream-billing-probe` | 立即探测单账号 | 使用账号当前凭据、代理和 TLS 配置；身份变化时 CAS 拒绝旧结果覆盖 |

## 管理端 Ollama Cloud 官方用量

全部路径只允许管理员访问。该能力读取 Ollama 官方设置页的脱敏用量快照，不参与账号健康、调度、计费或用户模型目录。

| 方法 | 路径 | 用途 | 关键语义 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/accounts/ollama-cloud-usage/settings` | 读取全局 runner 设置 | 默认关闭、默认间隔 60 分钟 |
| `PUT` | `/api/v1/admin/accounts/ollama-cloud-usage/settings` | 更新 runner 开关与间隔 | `interval_minutes` 限制 `15-1440` |
| `GET` | `/api/v1/admin/accounts/:id/ollama-cloud-usage` | 读取单账号状态 | 返回资格、是否已配置、自动刷新和脱敏快照 |
| `PUT` | `/api/v1/admin/accounts/:id/ollama-cloud-usage/session` | 保存 Ollama Web session | 需要固定 `TOTP_ENCRYPTION_KEY`；响应不回显 session |
| `DELETE` | `/api/v1/admin/accounts/:id/ollama-cloud-usage/session` | 删除 session 与托管状态 | 不删除账号或影响账号凭据 |
| `PUT` | `/api/v1/admin/accounts/:id/ollama-cloud-usage/auto-refresh` | 开关单账号自动刷新 | 只影响观察 runner |
| `POST` | `/api/v1/admin/accounts/:id/ollama-cloud-usage/refresh` | 立即刷新 | 单账号手工刷新 30 秒冷却 |

账号必须是指向 `https://ollama.com` 的 OpenAI 或 Anthropic API Key 账号。持久化快照最多包含套餐、5 小时 / 7 天窗口、余额、模型请求数、抓取时间和脱敏错误；原始 HTML、Cookie / session、账号凭据和上游响应头不得进入 DTO、审计或普通日志。

## 异步图片任务

| 方法 | 路径 | 用途 | 关键语义 |
| --- | --- | --- | --- |
| `POST` | `/v1/images/generations/async` | 异步提交图片生成 | 返回 `202`、task ID、`Location`、任务阶段和可选 `budget`；成员限额下明确标注“任务冻结金额”，不混同实际已用 |
| `POST` | `/v1/images/edits/async` | 异步提交图片编辑 | 支持同步编辑端点的 multipart / JSON 载荷；流式请求拒绝 |
| `GET` | `/v1/images/tasks/:task_id` | 查询任务状态 / 结果 | 只能使用创建任务的 Key；返回 `phase` 与 `budget.status`，可区分 `held`、`settled`、`released`、`needs_review`；对象存储临时禁用时仍可查询既有任务 |

以上路径同时提供既有无 `/v1` 别名。任务只支持 OpenAI / Grok 分组，图片结果转存 S3 兼容对象存储后才写入紧凑 Redis 结果。Redis 私有快照保存预算 receipt 关联和恢复期限，PostgreSQL receipt 保存 task ID 与 `queued/executing` 执行栅栏：未发往上游的过期排队任务自动释放冻结，已进入执行但结果不明的任务不重放并转待核对。Redis task key 丢失时，轮询接口按 PostgreSQL receipt 返回不泄露内部 request ID 的恢复终态，而不是让仍在占用预算的任务直接变成 `404`。完整响应合同见 `docs/ASYNC_IMAGE_TASKS.md`。

## 操作审计与 step-up 2FA

| 方法 | 路径 | 用途 | 关键语义 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/audit-logs` | 管理操作审计列表 | 仅管理员；支持按动作、操作者、目标和时间筛选 |
| `GET` | `/api/v1/admin/audit-logs/:id` | 审计详情 | 敏感头、token、secret 和密码字段在持久化前脱敏 |
| `POST` | `/api/v1/admin/audit-logs/clear` | 清空审计 | 必须现场 TOTP 验证，不复用已有 step-up 窗口 |
| `POST` | `/api/v1/user/totp/step-up` | 为当前会话建立短时二次验证 | 会话绑定 IP / UA，敏感操作仍可要求更强的现场验证 |

管理员角色提升、用户敏感变更和下载 / 清理类操作由路由或 handler 施加 step-up；普通登录态不能绕过该层。

## 企业成员 Key 明文按需读取

| 方法 | 路径 | 用途 | 关键语义 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/enterprise/members/:id/keys` | 查看成员 Key 列表 | 只返回脱敏 Key；归档成员可查看历史列表但不能读取明文或修改 |
| `POST` | `/api/v1/enterprise/members/:id/keys/:key_id/reveal` | owner 主动复制一把当前成员 Key | 同时限定企业 owner、当前成员、Key ID、成员归属和未删除状态；审计成功后才返回最小明文响应 |

`reveal` 成功响应只包含 `id`、`member_id` 和 `key`，并设置 `Cache-Control: no-store` 与 `Pragma: no-cache`。应用服务在返回前写入 `member_key.reveal_authorized` append-only 审计，记录 owner/member/actor/Key ID 而不记录凭据；审计不可用时 fail closed。普通 `/api/v1/keys/:id` 继续拒绝成员 Key，不能作为旁路。

当前认证基线与普通 Key 明文详情一致：要求有效 owner 登录态并写通用审计，不单独强制 TOTP step-up。未来如提升凭据揭示安全等级，普通 Key 和成员 Key 必须同步实施 step-up、专用限流与异常告警，避免形成单侧安全策略。

## 管理端提示词输入审计

全部路径都要求管理员身份。提示词审计与原内容审计通过协调器串联，但使用独立配置、任务、事件和管理页面；默认配置为关闭。

| 方法 | 路径 | 用途 | 关键语义 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/prompt-audit/config` | 读取公开配置 | 返回 token 是否已配置，不回显 Guard token |
| `PUT` | `/api/v1/admin/prompt-audit/config` | 更新配置 | 要求 `expected_config_version`，使用版本比较阻止并发覆盖 |
| `POST` | `/api/v1/admin/prompt-audit/endpoints/probe` | 探测审计节点 | 使用本次提交的节点配置，返回状态、延迟和可重试信息 |
| `GET` | `/api/v1/admin/prompt-audit/runtime` | 查看运行状态 | 返回进程、队列、Redis、节点和审计指标快照 |
| `GET` | `/api/v1/admin/prompt-audit/events` | 筛选事件 | 支持用户、Key、分组、判定、风险、时间和文本条件 |
| `GET` | `/api/v1/admin/prompt-audit/events/:id` | 查看事件详情 | 命中事件可包含管理员复核所需的 `snapshot.full_prompt` |
| `DELETE` | `/api/v1/admin/prompt-audit/events/:id` | 删除单事件 | 同步清理关联任务和临时 payload |
| `POST` | `/api/v1/admin/prompt-audit/events/batch-delete` | 按 ID 批量删除 | 单次 1-500 个正整数事件 ID |
| `POST` | `/api/v1/admin/prompt-audit/events/delete-preview` | 预览筛选删除 | 固化 filter hash、最高事件 ID 和短时确认 token |
| `POST` | `/api/v1/admin/prompt-audit/events/delete-by-filter` | 确认筛选删除 | `confirm=true` 且预览快照、管理员与确认 token 必须一致 |

配置同时受 `risk_control_enabled` 总入口影响，支持全部分组或显式 `group_ids`、priority 节点策略、异步审计与可选 blocking。任务表只保留 hash、脱敏预览和身份快照；完整提示词只进入最终事件表，Guard 凭据不进入任务/事件表或普通日志。

## 用户侧 API Key

所有 `/api/v1/keys/*` 接口都要求登录用户身份，并且只能操作当前用户自己的 Key。

| 方法 | 路径 | 用途 | 关键语义 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/keys` | 当前用户 Key 列表 | 支持 `search`、`status`、`group_id`、`tags`、分页和排序 |
| `GET` | `/api/v1/keys/tags` | 当前用户标签候选 | 只看未删除 Key，最多返回 500 个去重标签 |
| `POST` | `/api/v1/keys` | 创建单把 Key | 支持 `tags`、分组、quota、过期、5h/1d/7d 限流、IP ACL |
| `PUT` | `/api/v1/keys/:id` | 更新单把 Key | `status` 接收 `active`、`disabled`；`inactive` 仅兼容旧别名 |
| `DELETE` | `/api/v1/keys/:id` | 删除单把 Key | 软删除并写 deleted API key audit |
| `POST` | `/api/v1/keys/batch` | 批量创建 Key | 要求 `Idempotency-Key`，全有或全无事务 |
| `POST` | `/api/v1/keys/batch-update` | 批量更新 Key | 支持 selected/filtered 目标和多字段更新 |
| `POST` | `/api/v1/keys/batch-delete` | 批量删除 Key | 支持 selected/filtered 目标，事务软删除 |

### 列表筛选

`GET /api/v1/keys` 支持：

| 参数 | 说明 |
| --- | --- |
| `search` | Key 名称或脱敏 Key 搜索，后端截断到 100 字符 |
| `status` | `active`、`disabled`、`quota_exhausted`、`expired`；旧值 `inactive` 会归一到 `disabled` |
| `group_id` | `0` 表示无分组，正数表示指定分组 |
| `tags` | 逗号分隔标签，语义为同时包含 |
| `tag` | 可重复参数，和 `tags` 合并 |

标签会在服务层统一 `trim`、小写、去重，最多 20 个，每个最多 40 个字符。

### 批量创建

`POST /api/v1/keys/batch` 请求主体：

```json
{
  "count": 3,
  "name_template": "team-a-{seq}",
  "names": [],
  "tags": ["team-a", "frontend"],
  "group_id": 9,
  "quota": 10,
  "expires_in_days": 30,
  "rate_limit_5h": 1,
  "rate_limit_1d": 3,
  "rate_limit_7d": 10,
  "ip_whitelist": ["203.0.113.0/24"],
  "ip_blacklist": []
}
```

约束：

- `name_template` 与 `names` 二选一。
- 模板必须包含 `{seq}`，生成序号宽度至少 3 位。
- 默认最多 200 把，服务端硬上限 500。
- 任一 Key 写入失败会整批回滚。
- 首次成功响应包含完整明文 Key；同一 `Idempotency-Key` 重放只返回脱敏 Key。

### 批量更新 / 删除目标

`batch-update` 与 `batch-delete` 共用目标选择：

```json
{
  "ids": [1, 2, 3],
  "apply_to": "selected"
}
```

或：

```json
{
  "apply_to": "filtered",
  "filters": {
    "search": "alice",
    "status": "active",
    "group_id": 9,
    "tags": ["team-a"]
  }
}
```

`filtered` 模式要求至少一个筛选条件，且匹配数量不能超过 500。后端会先解析成当前用户名下的 Key ID，再复用 selected 路径的所有权校验、事务和缓存失效。

批量更新支持：

| 字段 | 语义 |
| --- | --- |
| `update_group` + `group_id` | 批量改分组；`null` 可清空分组 |
| `update_status` + `status` | 批量启用/禁用；`inactive` 会写成 `disabled` |
| `update_quota` + `quota_mode` | `set`、`add`、`unlimited` |
| `update_expiration` + `expires_at` | 设置或清空过期时间 |
| `update_rate_limit` | 设置 5h/1d/7d 限流 |
| `reset_rate_limit_usage` | 清空限流窗口用量并失效 Redis 限流缓存 |
| `update_ip_access_control` | 覆盖 IP 白名单/黑名单 |
| `update_tags` + `tags_mode` | `set`、`add`、`remove`、`clear` |

## 公共 Key 状态查询

`POST /api/v1/key/status` 不要求站点登录，但要求持有完整 API Key。

请求：

```json
{
  "key": "sk-..."
}
```

返回范围：

- Key 名称、状态、是否 active。
- 分组 ID、分组名、平台。
- quota、quota_used、quota_remaining。
- 过期时间、创建时间、最近使用时间。
- 5h/1d/7d 限流配置、当前窗口用量和预计重置时间。

安全边界：

- 不返回 owner 账号余额、邮箱、角色或其它 Key。
- 不返回请求记录、模型分布或企业聚合数据。
- 不更新 `last_used_at`，不扣 quota，不改变限流窗口。
- IP 维度限流为 30/min；同一 Key 10 秒内只能查一次。Redis 冷却写入失败时 fail-close。

## 单 Key 用量下钻

所有路径都要求登录用户身份，并校验 `:id` 属于当前用户。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/user/api-keys/:id/usage/daily` | 兼容日粒度旧接口 |
| `GET` | `/api/v1/user/api-keys/:id/usage/trend` | 按 hour/day/week/month 返回趋势 |
| `GET` | `/api/v1/user/api-keys/:id/usage/models` | 当前 Key 的用户可见模型分布 |

`trend` 参数：

| 参数 | 说明 |
| --- | --- |
| `start_date` / `end_date` | 日期范围 |
| `granularity` | `hour`、`day`、`week`、`month` |
| `timezone` | 默认使用服务端解析后的时区；常用 `Asia/Shanghai` |

用户侧模型分布只返回 `actual_cost`，不返回 `cost`、`account_cost` 或上游账号字段。

## 管理端分组与高峰倍率

管理端分组接口沿用上游 `/api/v1/admin/groups` 系列路径。本分支在文档中明确高峰倍率的接口边界，避免和 dev-zz 用户/admin 成本边界混淆。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/groups` | 管理端分组分页列表，返回 `AdminGroup` |
| `GET` | `/api/v1/admin/groups/all` | 管理端分组候选列表，可带 `include_inactive=true` |
| `POST` | `/api/v1/admin/groups` | 创建分组，支持高峰倍率字段 |
| `PUT` | `/api/v1/admin/groups/:id` | 更新分组，支持高峰倍率字段 |

高峰倍率字段：

| 字段 | 说明 |
| --- | --- |
| `peak_rate_enabled` | 是否启用高峰时段倍率 |
| `peak_start` | 高峰开始时间，格式 `HH:MM` |
| `peak_end` | 高峰结束时间，格式 `HH:MM`，左闭右开区间的结束点 |
| `peak_rate_multiplier` | 高峰时段叠加倍率；允许 `0`，不能为负 |

语义约束：

- 仅 `subscription_type=subscription` 的分组允许启用高峰倍率。
- 启用后 `peak_start` / `peak_end` 必填，且 `peak_end > peak_start`；当前不支持跨天区间，例如 `22:00-02:00`。
- 高峰时间按服务端全局时区判定。
- 高峰倍率只叠加到 token 计费倍率；token 模式下的图片 token 同样适用，图片按次计费不受高峰倍率影响。
- 用户侧可用渠道和订阅展示可返回公开分组的高峰倍率提示，但不得因此暴露上游账号、渠道、内部成本或管理员专属计费字段。

## 管理端 OpenAI Fast / Flex 策略

管理员通过通用设置接口读取和保存 OpenAI Fast / Flex 策略：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/settings` | 返回 `openai_fast_policy_settings` 及其规则 |
| `PUT` | `/api/v1/admin/settings` | 更新系统设置，可携带 `openai_fast_policy_settings` |

规则的 dev-zz / 上游增量字段：

| 字段 | 说明 |
| --- | --- |
| `openai_fast_policy_settings.rules[].user_ids` | 可选用户 ID 数组；为空或省略时是全局规则，非空时只匹配指定 API Key 所属用户 |

匹配和安全语义：

- 用户专属规则整体先于全局规则计算；用户专属组和全局组内部都保持管理员配置顺序，首条命中后停止。
- 用户专属规则命中 scope / tier 后，其模型白名单结果也是终止结果：模型不在白名单时直接执行该规则的 `fallback_action`（默认 `pass`），不会继续落到全局规则。需要与全局规则相同的兜底时，应在用户专属规则中显式配置相同动作。
- 用户 ID 必须为正整数，同一条规则内不能重复；服务端会在写入前完成策略校验，并把普通系统设置、认证来源默认值和 Fast / Flex 策略作为同一次批量写入。支付配置仍由该接口中的独立服务更新，不属于这次批量写入边界。
- 匹配身份来自 API Key 认证中间件写入请求 context 的可信 owner ID，不能用客户端请求体、header 或 query 中的自报用户标识替代。
- HTTP 与 OpenAI WebSocket 转发使用同一规则语义；没有可信 API Key owner context 时，用户专属规则不匹配，只能进入适用的全局规则。
- WebSocket 会话在建立时读取一次策略快照；设置变更会作用于新会话，已有长连接需重连后才读取新策略。
- 该配置只存在于管理员设置接口和管理端页面，不向普通用户接口暴露用户 ID 列表或其它管理员策略细节。

## Owner 用量分析

owner analytics 已落地在用户认证域 `/api/v1/usage/analytics/*`。接口不接受外部 `user_id`，后端始终绑定当前 `subject.UserID`。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/v1/usage/analytics/summary` | 当前用户 Key 的历史聚合和实时治理快照 |
| `GET` | `/api/v1/usage/analytics/leaderboard` | 员工 Key 排行 |
| `GET` | `/api/v1/usage/analytics/models` | 请求模型分布 |
| `GET` | `/api/v1/usage/analytics/groups` | 分组维度统计 |
| `GET` | `/api/v1/usage/analytics/tags` | 标签维度统计 |
| `GET` | `/api/v1/usage/analytics/trend` | owner 总趋势 |

统一查询参数：

| 参数 | 说明 |
| --- | --- |
| `start_date` / `end_date` | 统计时间范围 |
| `timezone` | 分桶时区 |
| `granularity` | `hour`、`day`、`week`、`month` |
| `api_key_id` | 限定当前用户名下单把 Key |
| `group_id` | 限定分组 |
| `tags` | 逗号分隔标签 |
| `status` | `active`、`disabled`、`quota_exhausted`、`expired` |
| `search` | Key 名称搜索，最长 100 字符 |
| `limit` | 1-100，默认服务端常量 |

字段边界：

- 允许返回请求数、Token、`actual_cost`、Key 名称、标签、分组名称、状态、最后使用时间。
- 禁止返回完整 Key、其它用户信息、上游账号、渠道、`upstream_model`、`account_cost`、利润或账号倍率。
- `summary.current_key_snapshot` 是当前实时状态，不随历史时间范围回溯。
- `tags` 统计使用重复计入语义，不返回 `share_percent`。

## 可用渠道模型

用户侧可用渠道仍使用：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/channels/available` | 返回当前用户可见渠道及模型信息 |

模型条目可选返回 `route_group_ids` 和 `supported_endpoints`。前者只包含当前用户可见、运行时仍可选择的分组；后者每项只包含协议、公共路径和已确认可发布该端点的可见 `group_ids`。只有定价模型能通过 active、可调度且模型匹配的账号形成稳定可调用路由时才返回。原有 Messages 兼容路径可提供 `/v1/messages`，Chat / Responses 则按账号实际选定的上游协议与明确能力证据形成原生或兼容交付。能力为 `unknown` 可保留既有兼容模型，但不发布未经证明的新端点；明确 `unsupported` 不允许被旧选择器绕过。不会返回账号、供应商、上游地址或故障转移拓扑。`GET /v1/models` 同步使用 new-api 兼容的可选字段 `supported_endpoint_types`，且复用同一稳定路由解析器。

dev-zz 前端基于该接口构建模型级表格和导出视图。具体展示口径见 [可用渠道模型广场与报价导出](../features/available-channels-model-marketplace.md)。

## 模型服务状态（定价驱动站点自检）

用户侧模型服务状态要求登录用户身份，按 **分组 / 模型** 维度展示公开模型健康状态。数据来源是**站点自检探针**：对在渠道定价中开启「自检」的模型，系统用合成请求走本站网关真实链路（标记为探针，**不写用量、不计费、不触发生产账号封禁/重试/failover**），结果写入 `model_self_check_histories`，按 (分组, 模型) 对覆盖账号做 OR 聚合（任一账号成功即视为该分组下模型可用）。与管理员侧「渠道监控」（上游探针，写 `channel_monitor_histories`）完全分离。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/model-status` | 模型状态列表，按分组分区，含当前状态、24h / 7d / 30d 可用率、降级比例、最近延迟和最后检测时间 |
| `GET` | `/api/v1/model-status/detail?model=...` | 单个模型详情，含同口径指标和最近时间线 |

字段边界：

- 允许返回 `group_id`、`group_name`、`model`、`display_name`、`status`、公开 `message_code`、`availability_24h/7d/30d`、`degraded_ratio_24h`、平均延迟、最近延迟、`last_checked_at` 和脱敏 `timeline`。（分组名对用户本就可见，故允许返回。）
- 禁止返回 `account_id`、`provider`、`endpoint`、API mode、上游模型映射、原始错误、`channel_id`、成本或其它内部路由字段。
- 旧用户侧 `/api/v1/channel-monitors` 探针路由已撤下；管理员仍通过 `/api/v1/admin/channel-monitors` 管理和排查上游探针。

配置入口：在**渠道定价**编辑界面按模型开启「自检」开关；自检节流由管理员设置控制——软开关 `model_self_check_enabled`、`self_check_default_interval_seconds`、`self_check_max_concurrency`、`self_check_max_tasks_per_round`。

具体设计见 [定价驱动的站点自检模型监控](../features/pricing-driven-self-check-monitoring-design.md)。

## 管理端模型探测

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/admin/accounts/probe-models` | 管理员用当前表单凭据探测 OpenAI 兼容 `/v1/models` |
| `GET` | `/api/v1/admin/accounts/:id/model-protocol-capabilities` | 读取账号的模型协议观察、覆盖和有效状态，并返回其影响的公开渠道模型 |
| `PUT` | `/api/v1/admin/accounts/:id/model-protocol-capabilities/overrides` | 只更新管理员覆盖；`auto` 恢复跟随观察结果 |
| `POST` | `/api/v1/admin/accounts/:id/model-protocol-capabilities/sync` | 从账号配置的上游模型列表同步模型和协议观察 |
| `GET` | `/api/v1/admin/channels/:id/model-delivery` | 从渠道公开模型反查分组、稳定账号路由、最终上游模型和公共 API 端点 |

该接口不持久化凭据，后端带 SSRF 防护，拒绝解析到本地、私有或链路本地地址的目标主机。前端只把探测结果追加到白名单或同名映射行，管理员仍需保存账号表单。

协议能力同步复用已保存账号的认证、base URL、代理和 SSRF 校验。观察列与管理员覆盖列是两条独立事务写路径；客户端不能写观察来源、观察时间或有效状态。上游无法提供能力元数据时，管理员仍可在独立能力界面手工添加精确上游模型并设置覆盖；模型名只允许 1–255 个字符，不接受 `*` 以外的通配符或控制字符。

账号能力响应中的 `public_model_impacts` / `orphan_upstream_models` 和渠道交付接口中的账号、映射、证据来源都属于管理员诊断数据。用户侧 `/channels/available` 只消费其脱敏后的模型与公共端点投影。

## 管理端上游成本池

阶段 1 只落地后端兼容层：旧账号级充值记录接口仍保留，但账号已有 active 成本绑定时会读取/写入对应资金池账本。本期账本只支持 `recharge` / `bonus` / `adjustment` 三类非负金额记录；只有具有有效实付/到账单位成本的 `recharge` 会生成成本快照，`bonus` 和 `adjustment` 不单独改写当前成本。普通用户侧接口不得返回供应商、资金池、上游余额、真实成本或利润字段。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/upstream-suppliers` | 列出真实上游供应商；`is_system=true` 的旧迁移系统供应商不进入正常列表 |
| `POST` | `/api/v1/admin/upstream-suppliers` | 新增供应商和默认资金池，可同时写入默认充值换算与默认参考汇率 |
| `PATCH` | `/api/v1/admin/upstream-suppliers/:supplier_id` | 更新供应商名称、备注、状态或默认资金池结算配置；`is_system=true` 的旧迁移系统供应商禁止修改 |
| `DELETE` | `/api/v1/admin/upstream-suppliers/:supplier_id` | 受限硬删除供应商；只允许从未绑定账号、无非默认资金池、无充值记录和无成本快照的供应商 |
| `GET` | `/api/v1/admin/upstream-cost-pools` | 列出资金池，返回供应商、稳定默认池标志 `is_default`、当前真实基础成本、绑定账号数和账本记录数 |
| `GET` | `/api/v1/admin/upstream-cost-pools/:pool_id` | 查看单个资金池详情 |
| `GET` | `/api/v1/admin/upstream-cost-pools/:pool_id/accounts` | 查看资金池当前 active 账号绑定 |
| `GET` | `/api/v1/admin/upstream-cost-pools/:pool_id/recharge-records` | 查看资金池充值账本 |
| `POST` | `/api/v1/admin/upstream-cost-pools/:pool_id/recharge-records` | 给资金池新增充值记录；可选 `account_id` 只作为账号快照来源 |
| `GET` | `/api/v1/admin/accounts/:id/upstream-cost-binding` | 查看账号当前 active 成本绑定；没有绑定时返回未找到，不产生写副作用 |
| `PUT` | `/api/v1/admin/accounts/:id/upstream-cost-binding` | 替换账号 active 成本绑定，旧绑定归档保留历史 |
| `PATCH` | `/api/v1/admin/accounts/:id/upstream-cost-profile` | 兼容更新历史账号 `extra` 中的上游成本参数；不作为新版账号编辑主入口，长期应迁移到资金池 / 绑定 / 快照接口 |
| `GET` | `/api/v1/admin/accounts/:id/recharge-records` | 兼容旧入口；有资金池绑定时返回资金池账本并带 `deprecated` / `cost_pool_id` |
| `POST` | `/api/v1/admin/accounts/:id/recharge-records` | 兼容旧入口；写入账号 active 真实供应商资金池，没有绑定时返回 `UPSTREAM_SUPPLIER_BINDING_REQUIRED` |

`PATCH /upstream-suppliers/:supplier_id` 请求主体：

```json
{
  "name": "供应商 A",
  "note": "共享钱包",
  "status": "archived",
  "default_effective_cny_per_usd": 1,
  "default_reference_fx_rate": 7.2
}
```

约束：

- `name`、`note`、`status`、`default_effective_cny_per_usd`、`default_reference_fx_rate` 都是可选字段；`status` 只接受 `active` 或 `archived`，两个默认成本字段必须大于 0。管理端清空备注时发送空字符串，后端会归一化为无备注。
- 默认成本字段写入 `is_default=true` 的供应商默认资金池。它们只用于以后新增充值记录的自动计算，不写入当前真实成本，不覆盖已有充值记录或成本快照。
- `POST` 是严格创建语义；active 名称重复时返回 `SUPPLIER_NAME_CONFLICT`，不会复用现有供应商，也不会修改其默认配置。
- 改名会校验 active 供应商名称唯一，冲突返回 `SUPPLIER_NAME_CONFLICT`。
- `is_system=true` 的旧迁移系统供应商不从列表返回；若通过历史 ID 直接请求修改 / 删除，返回 `SUPPLIER_RESERVED`。

`DELETE /upstream-suppliers/:supplier_id` 约束：

- 供应商必须没有任何账号绑定历史；active 绑定返回 `SUPPLIER_HAS_BOUND_ACCOUNTS`，已解绑 / 已归档绑定返回 `SUPPLIER_HAS_BINDING_HISTORY`。
- 供应商下不能有非默认资金池。
- 供应商下不能有充值记录或成本快照；这两类历史成本事实应通过归档保留。
- 删除事务不会清理历史绑定；曾被账号使用的供应商必须归档，以保留可解释的归属审计链。

常见错误码：

| 错误码 | 说明 |
| --- | --- |
| `SUPPLIER_NAME_CONFLICT` | active 供应商重名 |
| `SUPPLIER_RESERVED` | 旧迁移系统供应商不允许修改或删除 |
| `SUPPLIER_HAS_BOUND_ACCOUNTS` | 仍有 active 账号绑定 |
| `SUPPLIER_HAS_BINDING_HISTORY` | 已有账号绑定历史，必须归档保留审计链 |
| `SUPPLIER_HAS_COST_DATA` | 仍有非默认资金池、充值记录或成本快照 |

`PUT /accounts/:id/upstream-cost-binding` 和账号编辑使用的 `PUT /accounts/:id/upstream-supplier-binding` 是兼容层绑定接口；账号编辑主流程传供应商 / 资金池归属、上游分组名和这把 key 的上游分组倍率，不暴露真实充值比例、参考汇率或资金池基础成本。两条入口都拒绝给已归档供应商建立新绑定；已有归档供应商绑定继续返回给账号编辑，并以禁用历史项展示，只有管理员明确清空时才解绑。底层 `default_multiplier` 仍作为兼容存储列，API / UI 应优先使用 `upstream_group_multiplier`：

```json
{
  "supplier_id": 1,
  "cost_pool_id": 1,
  "upstream_group_name": "claude-sale",
  "price_reference_currency": "USD",
  "upstream_group_multiplier": 1.4
}
```

`price_reference_currency` 只接受 `CNY` / `USD`，含义是上游分组价目表的实际币种，不按供应商所在地或模型名称推断。显式提交后，响应中的 `price_reference_confirmed` 为 `true`。迁移前历史绑定保留 `USD` 旧公式但返回 `price_reference_confirmed=false`，管理端显示“待确认”，并排除出成本优先排序 / 调度。

为了兼容旧客户端，更新同一资金池且省略 `price_reference_currency` 时保留当前绑定的币种与确认状态；新绑定或切换资金池时仍省略，则创建暂按 `USD` 旧口径、但未确认的绑定。当前账号编辑 UI 要求管理员明确选择后才保存 active 供应商绑定。

完整兼容接口仍接受 `default_multiplier` 和 `model_families`，但账号编辑页不应把模型族倍率或供应商基础成本作为主流程字段。首版 `cost_first` 仍使用账号级标量 `default_multiplier`，尚未按请求模型族消费 `model_families`。

`POST /upstream-cost-pools/:pool_id/recharge-records` 沿用旧充值记录字段：`type`、`paid_amount`、`paid_currency`、`received_credit_amount`、`received_credit_currency`、`reference_fx_rate`、`recorded_at`、`note`；阶段 1 只接受 `recharge` / `bonus` / `adjustment`，仍固定支持 CNY 实付和 USD 上游额度，金额必须非负。管理端 UI 会从资金池 `default_effective_cny_per_usd` / `default_reference_fx_rate` 自动生成普通充值的到账额度和参考汇率，但请求仍提交并固化本次实际值。只有 `recharge` 定义单位成本并更新当前快照；`bonus` 只增加额度，`adjustment` 只保存账本调整，两者都不单独刷新资金池当前成本。充值新增、修改或删除提交后会刷新该资金池所有 active 绑定账号的调度快照。

## 运维失败分类与结构化钻取

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/ops/dashboard/overview` | 返回失败分类 v2 总览、归因分布和固定最近 15 分钟当前状态；旧 headline 字段继续兼容返回 |
| `GET` | `/api/v1/admin/ops/errors` | 按结构化分类筛选所有 Ops 错误日志 |
| `GET` | `/api/v1/admin/ops/request-errors` | 请求失败明细；默认兼容旧 `view`，也接受 v2 精确筛选 |
| `GET` | `/api/v1/admin/ops/upstream-errors` | 上游终态或 recovered attempt 明细；未指定 `view` 时保留 provider-health 证据 |
| `GET` | `/api/v1/admin/ops/errors/:id` | 返回单条错误详情及完整分类 v2 字段 |

overview 新增字段：

- `classification_version`：当前聚合合同版本，v2 为 `2`；
- `customer_visible_failure_count/rate`：客户最终收到失败的逻辑请求数和比例；
- `platform_sla_failure_count`：平台未履约失败数；
- `sla_excluded_failure_count`：客户可见但不计平台 SLA 的失败数；
- `classification_unknown_count`：证据不足、不能可靠判责的 v2 行，以及滚动部署晚写、缺少结构化归因的 v1 行数量；
- `failure_breakdown[]`：按 `domain/category` 返回可对账归因数量；
- `current_window`：固定 900 秒窗口及 `active/recovered/quiet/unknown` 状态，状态按管理员平台 SLA 失败率阈值判定。

错误日志和列表筛选新增：

```text
event_scope
customer_visible=true|false
failure_domain
failure_category
failure_reason
resolution_owner
pool_ownership
sla_impact=true|false|unknown
classification_version
```

兼容规则：旧 `view=errors|excluded|all` 继续可用，分别映射到平台 SLA、SLA 排除和不按 SLA 过滤；v2 行的 `sla_impact=NULL` 始终进入 unknown，不能回退到 `is_business_limited` 猜测。v1 行为保持旧客户端 headline 连续性，仍用 `is_business_limited` 推导 SLA 兼容值，但同时进入 unknown，因此混合窗口中的 unknown 是数据质量覆盖层，可能与兼容 headline 重叠。旧实例晚写的 `status<400 + stream=true` 非 recovered 行按客户可见终态进入列表和 unknown；只有 `error_phase=upstream/account_auth` 且错误消息以 `Recovered ` 开头的旧行被识别为已恢复尝试。`cyber_policy` 与 `cyber_policy_session_blocked` 即使 HTTP 状态为 200 也保持客户可见、SLA 排除。前端从 overview 下钻时应发送响应中的绝对 `start_time/end_time`，不能重新计算相对 1h/6h 窗口。

`failure_category=non_routing` 是只读虚拟筛选组，不会写入数据库；它表示平台域内除 `routing_capacity` 以外的 dependency/internal 等故障，用于保证“平台内部故障”汇总与明细守恒。
