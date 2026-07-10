# dev-zz 接口索引

这页只记录 dev-zz 新增或语义有差异的接口。上游通用接口仍以项目源码和 `docs/` 兼容文档为准。

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
- 用户 ID 必须为正整数，同一条规则内不能重复；无效设置由服务端拒绝，不会部分写入。
- 匹配身份来自 API Key 认证中间件写入请求 context 的可信 owner ID，不能用客户端请求体、header 或 query 中的自报用户标识替代。
- HTTP 与 OpenAI WebSocket 转发使用同一规则语义；没有可信 API Key owner context 时，用户专属规则不匹配，只能进入适用的全局规则。
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

该接口不持久化凭据，后端带 SSRF 防护，拒绝解析到本地、私有或链路本地地址的目标主机。前端只把探测结果追加到白名单或同名映射行，管理员仍需保存账号表单。

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
  "upstream_group_multiplier": 1.4
}
```

完整兼容接口仍接受 `default_multiplier` 和 `model_families`，但账号编辑页不应把模型族倍率或供应商基础成本作为主流程字段。

`POST /upstream-cost-pools/:pool_id/recharge-records` 沿用旧充值记录字段：`type`、`paid_amount`、`paid_currency`、`received_credit_amount`、`received_credit_currency`、`reference_fx_rate`、`recorded_at`、`note`；阶段 1 只接受 `recharge` / `bonus` / `adjustment`，仍固定支持 CNY 实付和 USD 上游额度，金额必须非负。管理端 UI 会从资金池 `default_effective_cny_per_usd` / `default_reference_fx_rate` 自动生成普通充值的到账额度和参考汇率，但请求仍提交并固化本次实际值。只有 `recharge` 定义单位成本并更新当前快照；`bonus` 只增加额度，`adjustment` 只保存账本调整，两者都不单独刷新资金池当前成本。充值新增、修改或删除提交后会刷新该资金池所有 active 绑定账号的调度快照。
