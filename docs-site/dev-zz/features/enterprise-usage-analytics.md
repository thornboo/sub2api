# 企业用量分析中心

## 已落地情况

- owner 用量分析第一版已落地；管理员全站增强、异常治理和多分组 Key 仍在后续阶段。
- 最近更新：2026-06-15。
- 前置成果：
  - API Key 批量创建、批量维护、标签、筛选批量操作已落地。
  - 单 Key 用量下钻已落地，包含趋势、模型分布和请求记录。
  - ADR 0002 已明确：用 API Key 承载企业成员管理，不引入员工登录实体。
- 已落地成果：
  - 用户侧 Usage 页面新增分析视图，前端组件为 `frontend/src/components/user/UsageAnalyticsPanel.vue`。
  - 用户认证域新增 `/api/v1/usage/analytics/summary`、`leaderboard`、`models`、`groups`、`tags`、`trend`。
  - 后端所有 owner 查询都绑定当前登录用户，不接收外部 `user_id`。
- 本页记录 owner 自助分析的真实实现边界，同时保留平台管理员全站增强、异常治理和多供应商员工 Key 的后续设计边界。

## 背景

当前用户侧 `API 密钥` 已经可以把一把 Key 当作一个员工席位来管理。最近完成的单 Key 用量下钻解决了“点开某个员工 Key，看它在小时/天/周/月上的消耗、模型分布、请求记录”的问题。

但企业客户还会继续追问更高一层的问题：

- 老板想看所有员工 Key 的用量排行榜，而不是一个个点开。
- 财务或团队负责人想按部门、标签、分组、模型拆分成本。
- 管理员想识别异常用量、长期空闲 Key、接近 quota 或限流窗口的 Key。
- 平台管理员想看所有用户、所有分组、所有账号的全站运营数据和真实上游成本。
- 如果员工 A 同时需要 OpenAI、Anthropic、Gemini，当前“一把 Key 只绑一个 `group_id`”会逼迫 owner 创建多把员工 A 的 Key，管理体验不够企业级。

参考项目 `ding113/claude-code-hub` 把“实时监控与统计、日志审计、消耗排行榜、按用户统计请求数/Token/成本”作为企业网关核心功能，并提供多供应商统一接入、限流、负载均衡等能力。sub2api 可以借鉴这些产品形态，但不能照搬数据模型：claude-code-hub 是平台管理员管理多个 user 的视角，而 sub2api 的企业客户通常只是一个普通 `users` 行，企业员工目前映射为该账号名下的多把 `api_keys`。

## 现有基础

### 用户侧已有功能

- `frontend/src/views/user/KeysView.vue`：owner 管理自己名下 API Key。
- `frontend/src/components/keys/ApiKeyUsageModal.vue`：单 Key 用量详情 Modal。
- 用户侧单 Key 接口：
  - `GET /api/v1/user/api-keys/:id/usage/trend`
  - `GET /api/v1/user/api-keys/:id/usage/models`
- 用户侧请求记录：`GET /api/v1/usage` 支持 `api_key_id` 过滤，并校验 Key 属于当前用户。
- 用户侧批量摘要：`POST /api/v1/usage/dashboard/api-keys-usage` 返回当前页 Key 的今日/近 30 天实际扣费。
- 用户侧 owner analytics 已落地：
  - `GET /api/v1/usage/analytics/summary`
  - `GET /api/v1/usage/analytics/leaderboard`
  - `GET /api/v1/usage/analytics/models`
  - `GET /api/v1/usage/analytics/groups`
  - `GET /api/v1/usage/analytics/tags`
  - `GET /api/v1/usage/analytics/trend`

### 管理员已有功能

- `frontend/src/views/admin/UsageView.vue` 已有全站 Usage 页面，包含统计卡片、趋势、模型分布、分组分布、端点分布、请求记录和导出。
- `frontend/src/api/admin/dashboard.ts` 已有：
  - `/admin/dashboard/trend`
  - `/admin/dashboard/models`
  - `/admin/dashboard/groups`
  - `/admin/dashboard/api-keys-trend`
  - `/admin/dashboard/users-trend`
  - `/admin/dashboard/users-ranking`
  - `/admin/dashboard/user-breakdown`
  - `/admin/dashboard/snapshot-v2`
- 管理员 DTO 可包含 `account_cost`、上游账号、渠道、真实路由、用户邮箱等运营字段。

### 数据模型事实

- `api_keys` 当前核心字段：
  - `user_id`
  - `group_id`，可空，但只有单个分组绑定。
  - `name`
  - `tags`
  - `quota` / `quota_used`
  - `rate_limit_5h` / `rate_limit_1d` / `rate_limit_7d`
  - `expires_at`
- `groups` 有 `platform`、`rate_multiplier`、`supported_model_scopes`、模型路由、OpenAI Messages 调度配置等路由/计费语义。
- `usage_logs` 已记录：
  - `user_id`
  - `api_key_id`
  - `account_id`
  - `group_id`
  - `model` / `requested_model` / `upstream_model`
  - token 字段
  - `total_cost`
  - `actual_cost`
  - `account_cost` 的计算来源字段和账号倍率字段
  - `created_at`
- `usage_logs` 已有 `user_id, created_at`、`api_key_id, created_at`、`group_id, created_at` 等索引。第一版 owner 聚合可依赖 `user_id, created_at` 做 owner + 时间范围扫描；`GROUP BY api_key_id`、`GROUP BY requested_model`、`GROUP BY group_id` 仍会在该 owner 时间窗内聚合，不应描述为已被完整复合索引覆盖。只有当真实数据量证明单 owner 范围聚合成为瓶颈时，再评估 `(user_id, api_key_id, created_at)` 等复合索引或预聚合表。
- `users.AllowedGroups` 已存在，表示用户可绑定的专属分组授权范围；`APIKeyAuthUserSnapshot` 也会携带该字段。
- `APIKeyAuthGroupSnapshot` 已有 `FallbackGroupID` / `FallbackGroupIDOnInvalidRequest`，说明网关侧已经存在分组 fallback 语义。

### 关键约束

- ADR 0002 当前结论是“一把 Key 等于一个员工席位”，不引入员工登录主体。
- `APIKeyAuthSnapshot` 当前只有单个 `GroupID` 和单个 `Group` 快照。
- API Key 绑定分组不能只看 `api_keys.group_id`。现有授权语义包括：
  - 非专属分组：普通用户可绑定。
  - 专属分组：用户必须在 `AllowedGroups` 中。
  - 订阅型分组：还需要满足当前订阅/可绑定规则。
- 因此，未来一把 Key 多分组绑定必须是 owner 当前可绑定分组集合的子集，不能让 Key 获得 owner 自身无权访问的分组。
- 网关计费、调度、粘性会话、订阅用量更新、usage log 写入等路径大量使用 `apiKey.GroupID`。
- 因此，多供应商员工 Key 不是简单前端改动；它会影响 auth cache、路由选择、计费倍率、订阅用量和 usage log 归属。

## 产品目标

1. 给企业 owner 一个总览页，看自己名下所有员工 Key 的排行、趋势、模型分布、分组/标签拆分和异常信号。
2. 给平台管理员一个更清晰的全站分析入口，看用户、分组、账号、模型、成本和异常之间的关系。
3. 让单 Key 下钻成为所有排行和图表的共同 drilldown 目标。
4. 明确普通用户/企业 owner 与平台管理员的字段边界，防止用户侧接口泄露全站数据或上游账号成本。
5. 解决“员工 A 需要多供应商能力却被迫创建多把 Key”的企业级管理模型问题，但把它作为单独阶段实现。

## 非目标

- 不把平台管理员分析能力暴露给企业 owner。
- 不改变已经落地的单 Key 下钻接口。
- 不把平台管理员 `/admin` 接口开放给企业 owner。
- 不在第一版 owner 分析里暴露 `account_cost`、上游账号、渠道、真实调度链路或利润字段。
- 不引入员工登录账号。
- 不在未验证查询成本前增加复杂预聚合表。

## 权限与隐私边界

### 角色定义

| 角色 | 身份 | 可见范围 |
| --- | --- | --- |
| Key-only 员工 | 只有完整 API Key，没有站点登录 | 只能看该 Key 自身状态和有限用量摘要 |
| 企业 owner | 普通登录用户，管理自己名下多把 Key | 只能看 `user_id = 自己` 的 Key、标签、分组绑定和用量 |
| 平台管理员 | `role=admin` | 可看全站用户、Key、账号、分组、渠道、上游成本和运营数据 |

### 字段矩阵

| 字段/信息 | Key-only 员工 | 企业 owner | 平台管理员 |
| --- | --- | --- | --- |
| Key 名称、状态、过期、quota、限流 | 仅自己这把 Key 的有限状态 | 自己名下全部 Key | 全站 |
| Key 标签 | 不展示或仅展示当前 Key 的非敏感标签 | 自己名下 Key | 全站 Key |
| 请求数、Token、实际扣费 `actual_cost` | 当前 Key 有限摘要 | 自己名下 Key 聚合和明细 | 全站 |
| 标准计费 `total_cost` | 不展示 | 谨慎展示；第一版 owner 排行主口径用 `actual_cost` | 可展示 |
| 上游账号成本 `account_cost` | 禁止 | 禁止 | 允许 |
| 上游账号 ID/名称、渠道、账号池 | 禁止 | 禁止 | 允许 |
| `upstream_model`、模型映射链 | 禁止 | 第一版禁止；后续需单独评估 | 允许 |
| 请求 ID、IP、User-Agent | 禁止或极简 | 仅自己请求记录中已有用户可见字段 | 允许 |
| 其他用户邮箱、余额、角色 | 禁止 | 禁止 | 允许 |
| 利润、成本差、账号倍率 | 禁止 | 禁止 | 允许 |

### 接口边界

- 企业 owner 接口已经挂在用户认证域 `/api/v1/usage/analytics/*`。
- 所有 owner 查询都必须从当前认证主体取 `subject.UserID`，后端忽略或拒绝外部传入的 `user_id`。
- owner DTO 必须独立定义，不能直接返回 `usagestats.ModelStat`、`GroupStat`、`UserBreakdownItem` 等含 `account_cost` 的 admin DTO。
- 管理员分析继续使用 `/api/v1/admin/dashboard/*` 或新增 `/api/v1/admin/enterprise-analytics/*`，并保持 admin middleware。

## 信息架构

### 企业 owner：API Key 用量总览

当前入口：

- 用户侧 `用量` 页面新增 `分析` Tab。
- `分析` Tab 聚合当前 owner 名下所有 Key 的用量；请求记录、错误记录仍保留在同一页面的其它 Tab。
- 排行榜和单 Key 详情继续回到 `ApiKeyUsageModal` 的 drilldown 形态。

早期曾考虑放在 `API 密钥` 页面二级 Tab。实际落地放在 Usage 页，原因是现有 Usage 页已经承载用户侧统计、请求记录、错误记录、导出和图表组件，复用成本更低。

页面结构：

1. 顶部筛选栏
   - 日期范围：今天、昨天、7 天、30 天、90 天、自定义。
   - 粒度：小时、天、周、月。
   - 标签筛选。
   - 分组筛选。
   - 状态筛选。
   - Key 名称搜索。
   - 指标切换：实际扣费、总 Token、请求数。
2. 摘要卡片
   - 总实际扣费。
   - 总请求数。
   - 总 Token。
   - 活跃 Key 数（当前实时快照）。
   - 近 7 天环比变化。
   - 接近 quota 或限流的 Key 数（当前实时快照，不随历史日期范围回溯）。
3. 员工 Key 排行榜
   - 默认按 `actual_cost desc`。
   - 可切换请求数、Token、增长率。
   - 行字段：Key 名、标签、分组、请求数、Token、实际扣费、占比、环比、最后使用。
   - 点击行打开已有 `ApiKeyUsageModal`。
4. 分组/标签拆分
   - 分组成本占比。
   - 标签成本占比。
   - 同一 Key 多标签时，第一版标签聚合可采用“每个标签重复计入”并在 UI 说明；严格成本分摊可后续增加 `primary_tag`。
5. 模型分析
   - 请求模型分布。
   - 模型调用次数排行。
   - 模型 Token 排行。
   - 模型实际扣费排行。
   - owner 第一版只按 `requested_model` 展示，避免泄露上游模型映射。
6. 趋势图
   - 全部 Key 总趋势。
   - Top N Key 趋势对比。
   - 图表下方表格保留完整数值。
7. 异常与治理面板（后续阶段）
   - 用量突然升高。
   - 长期未使用。
   - 接近 quota。
   - 接近 5h/1d/7d 限流。
   - 最近失败请求较多（仅使用用户可见错误数据）。

### 平台管理员：全站企业分析

现有 `/admin/usage` 和 `/admin/dashboard` 已具备很多基础能力，不建议另起一套重复页面。更合理的方向是增强现有 admin 分析：

- 用户排行榜：已有 `/admin/dashboard/users-ranking`，继续作为全站用户消费榜。
- 用户拆分：已有 `/admin/dashboard/user-breakdown`，用于模型、分组、端点维度下看用户贡献。
- API Key 趋势：已有 `/admin/dashboard/api-keys-trend`，后续可扩展为 API Key 排行榜。
- 模型/分组/端点分布：已有 admin UsageView 图表。

建议新增管理员视图能力：

- “用户 -> Key -> 分组 -> 模型”的 drilldown 面板。
- API Key 排行榜，支持按用户、分组、标签、模型过滤。
- 账号成本 vs 实际扣费 vs 标准计费的差额分析。
- 高风险用户/Key 列表：异常增长、失败率高、疑似共享 Key、短时间高并发。
- 管理员专属导出，包含 `account_cost`、账号、渠道、上游模型等字段。

### Key-only 员工：仅状态查询

Key-only 员工不进入分析中心。

保留现有公共 Key 状态查询定位：

- 当前 Key 是否可用。
- quota / quota_used。
- 过期时间。
- 最近使用时间。
- 限流配置与当前窗口用量。

不能展示：

- owner 总余额。
- 同一企业其他 Key。
- 分组成本、模型成本、请求明细。
- 全站任何信息。

## Owner API 已落地

所有接口都要求用户登录。所有查询都必须绑定 `subject.UserID`。

### 统一查询参数

```text
start_date=YYYY-MM-DD
end_date=YYYY-MM-DD
timezone=Asia/Shanghai
granularity=hour|day|week|month
group_id=123
tags=team-a,frontend
status=active|disabled|quota_exhausted|expired
search=alice
limit=20
```

日期范围必须后端硬校验：

| 粒度 | owner 第一版最大范围 |
| --- | --- |
| hour | 31 天 |
| day | 180 天 |
| week | 104 周 |
| month | 60 月 |

### 汇总

```text
GET /api/v1/usage/analytics/summary
```

返回：

```json
{
  "total_actual_cost": 22.8054,
  "total_requests": 417,
  "total_tokens": 25447549,
  "used_key_count": 3,
  "current_key_snapshot": {
    "active_key_count": 3,
    "near_quota_key_count": 1,
    "near_rate_limit_key_count": 0,
    "snapshot_at": "2026-06-14T18:30:00+08:00"
  },
  "start_date": "2026-05-16",
  "end_date": "2026-06-14",
  "timezone": "Asia/Shanghai"
}
```

语义约束：

- `total_actual_cost`、`total_requests`、`total_tokens`、`used_key_count` 是所选历史时间范围内的聚合。
- `current_key_snapshot` 是当前实时治理快照，统计现有 Key 状态、当前 quota 使用和当前 5h/1d/7d 限流窗口，不随 `start_date` / `end_date` 回溯。前端应把它与历史聚合卡片分区展示，避免用户误解为“上个月某一刻接近限流的 Key 数”。

### 员工 Key 排行

```text
GET /api/v1/usage/analytics/leaderboard
```

返回字段：

```json
{
  "items": [
    {
      "api_key_id": 123,
      "key_name": "alice",
      "tags": ["frontend"],
      "group_id": 9,
      "group_name": "openai",
      "status": "active",
      "requests": 218,
      "input_tokens": 3844208,
      "output_tokens": 213892,
      "cache_creation_tokens": 0,
      "cache_read_tokens": 16173184,
      "total_tokens": 20231284,
      "actual_cost": 16.8122,
      "share_percent": 73.72,
      "previous_actual_cost": 8.11,
      "change_percent": 107.30,
      "last_used_at": "2026-06-10T12:34:56Z"
    }
  ],
  "total": 3
}
```

明确不返回：

- 完整 Key。
- `account_id` / `account_name`。
- `account_cost`。
- `upstream_model`。
- 其他用户字段。

### 模型统计

```text
GET /api/v1/usage/analytics/models
```

第一版按 `requested_model` 聚合。返回字段与用户侧 `UserModelStat` 保持一致：

- `model`
- `requests`
- token 字段
- `actual_cost`

不返回 `cost` / `account_cost`。如果未来需要展示标准计费 `cost`，必须确认它不会暴露平台成本或利润口径，并单独改 DTO。

### 分组/标签统计

```text
GET /api/v1/usage/analytics/groups
GET /api/v1/usage/analytics/tags
```

分组统计：

- 按 `usage_logs.group_id` 聚合。
- 只允许返回当前 owner 名下 Key 产生的用量。
- 分组名称来自当前用户可见的分组绑定或历史 usage log。

标签统计：

- 需要 join 当前 `api_keys.tags`。
- 多标签 Key 第一版可以重复计入每个标签；UI 需明确“标签聚合用于归因观察，不是严格财务分摊”。
- 第一版 `tags` 统计 DTO 不返回 `share_percent`，前端使用条形图或表格展示 `actual_cost` / `requests` / `key_count`，不要渲染成要求总和为 100% 的饼图。
- 如果未来确实要展示百分比，字段名必须明确分母，例如 `of_total_actual_cost_percent` 或 `of_tag_covered_cost_percent`，并在 API 文档和 UI 上说明多标签重复计入会导致按总成本口径求和超过 100%。
- 若企业要求严格分摊，后续增加 `primary_tag` 或成本中心字段。

### 趋势

```text
GET /api/v1/usage/analytics/trend
```

支持：

- 当前 owner 名下全部 Key 的总趋势。
- `api_key_id` / `group_id` / `tags` / `status` / `search` 过滤。
- hour / day / week / month 粒度。

repository 必须使用用户 timezone 分桶，不再使用裸 `TO_CHAR(created_at, ...)`。

### 异常（未落地）

```text
GET /api/v1/usage/analytics/anomalies
```

该接口尚未实现。后续第一版只做确定性规则，不做 ML：

- `spike`: 当前周期实际扣费超过前一周期 N 倍。
- `quota_near_limit`: `quota > 0` 且 `quota_used / quota >= 80%`。
- `rate_limit_near_limit`: 5h/1d/7d 任一窗口超过 80%。
- `idle`: 30 天未使用但仍 active。
- `new_key_high_usage`: 新 Key 在 24 小时内超过阈值。

所有异常项都必须指向某把 owner 自己的 Key，并可打开单 Key 下钻。

## Admin API 后续草案（未落地）

管理员已有 `/api/v1/admin/dashboard/*`。短期优先扩展现有 DashboardService/UsageView，而不是新增平行体系。

推荐新增或增强：

```text
GET /api/v1/admin/dashboard/api-keys-ranking
GET /api/v1/admin/dashboard/users/:id/key-breakdown
GET /api/v1/admin/dashboard/groups-ranking
GET /api/v1/admin/dashboard/cost-margin
```

管理员可返回：

- `user_id`、`email`、`username`。
- `api_key_id`、Key 名称、状态、分组。
- `account_id`、账号名称、渠道。
- `requested_model`、`upstream_model`、`model_mapping_chain`。
- `total_cost`、`actual_cost`、`account_cost`、差额。
- 请求 IP、User-Agent、request_id。

管理员接口仍需要范围上限和缓存，但缓存 key 必须包含所有筛选条件和日期。

## 多供应商员工 Key 方案

### 问题

如果一个企业 owner 要给员工 A 同时开放 OpenAI、Anthropic、Gemini，而当前 API Key 只能绑定一个 `group_id`，owner 往往需要创建：

- `员工A-openai`
- `员工A-anthropic`
- `员工A-gemini`

这会带来问题：

- 员工要保存多把 Key。
- owner 的排行榜会把员工 A 拆成多行。
- quota、限流、过期、IP ACL 要重复维护。
- 失效、离职、审计都要处理多把 Key。
- 标签可以缓解归并，但不能解决授权和限额重复。

### 现有授权模型约束

阶段四不能只新增 `api_key_group_bindings` 表就结束，因为现有系统已经有三层分组语义：

- `api_keys.group_id`：当前 Key 的 legacy/default 单分组绑定。
- `users.AllowedGroups`：用户可绑定的专属分组授权范围。
- `groups.FallbackGroupID` / `FallbackGroupIDOnInvalidRequest`：请求失败或不匹配时可能进入的分组 fallback 语义。

未来 Key 级多分组绑定必须满足：

- 绑定列表中的每个 group 都必须是 owner 当时可绑定的分组，不能绕过 `AllowedGroups`、订阅型分组和现有 `CanBindGroup` / `canUserBindGroupInternal` 规则。
- auth snapshot v2 应复用并扩展现有 `AllowedGroups`、`GroupID`、`Group`、fallback group 的语义，而不是定义一套平行授权模型。
- fallback group 如何计入 Key 的访问范围必须写入 ADR：默认 group fallback 是否自动允许、fallback 目标是否也要在 owner 可绑定范围内、usage log 最终写入哪个 group。

### 方案对比

| 方案 | 说明 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- | --- |
| 多把物理 Key + 共享标签 | 继续每个供应商一把 Key，用 `employee:alice` 标签归并 | 无 schema 风险，立刻可用 | 员工体验差，quota/限流重复，排行榜需二次归并 | 只作为短期过渡 |
| 新增员工/成员实体 | 新建不一定能登录的 `members`，一个成员挂多把 Key | 员工概念清晰，可做严格成本中心 | 与 ADR 0002 “Key 即员工席位”冲突，模型变重 | 暂不推荐 |
| API Key 多分组访问范围 | 一把 Key 仍是员工席位，但可绑定多个分组/能力包 | 员工只有一把 Key，quota/限流统一，usage log 仍可按 group 归因 | 需要改 auth cache、网关选组、计费和 UI | 推荐作为企业级目标方案 |

### 推荐方向：Key Access Profile

保留 `api_keys` 作为认证主体，但把“这个 Key 可以访问哪些供应商/分组/模型能力”从单个 `group_id` 扩展成访问范围。

建议概念：

- `api_key_access_profiles` 或直接 `api_key_group_bindings`。
- 一把 Key 可以绑定多个 group。
- 一把 Key 的 group binding 必须是该 Key owner 可绑定分组集合的子集。
- 每个绑定可选：
  - `group_id`
  - `platform`
  - `model_scope`
  - `sort_order`
  - `status`
  - 可选 per-binding quota/rate limit，第一版不做。

兼容策略：

- 保留 `api_keys.group_id` 作为 legacy/default group。
- 如果没有多分组绑定，完全走旧逻辑。
- 如果存在绑定，认证快照升级为 v2：

```json
{
  "api_key_id": 123,
  "user_id": 42,
  "group_id": 9,
  "allowed_groups": [9, 10, 11],
  "group_bindings": [
    { "group_id": 9, "platform": "openai", "model_scope": "gpt" },
    { "group_id": 10, "platform": "anthropic", "model_scope": "claude" },
    { "group_id": 11, "platform": "gemini", "model_scope": "gemini_text" }
  ]
}
```

路由策略：

1. 根据入口协议和请求模型判断目标平台/模型族。
2. 从 Key 的 `group_bindings` 中选择匹配分组。
3. 再进入现有按 group 的账号选择、模型限制、倍率、订阅用量逻辑。
4. usage log 继续写入最终 `group_id`，从而支持“员工 Key 总用量”和“员工 Key 按供应商/分组拆分”。

计费和限额策略：

- Key 的 `quota` 和 5h/1d/7d 限流默认是跨所有绑定共享。
- 分组倍率仍按最终选中的 group 计算。
- 订阅用量仍按最终 group 更新。
- 如果未来需要“员工 A 的 OpenAI 额度”和“员工 A 的 Gemini 额度”分开限制，再增加 binding 级 quota；不要第一版就做。

UI 策略：

- 创建/编辑 Key 时，把“分组”升级为“访问范围”。
- 默认仍可选择一个分组，兼容个人用户简单场景。
- 企业模式下可选择多个能力包：
  - OpenAI 分组
  - Anthropic 分组
  - Gemini 文本分组
  - Gemini 图片分组
- 列表展示为多个 provider chips，而不是把员工拆成多行。
- 批量创建员工 Key 时可以统一选择访问范围。

风险：

- 网关服务大量路径使用单 `GroupID`，不能小改小补。
- auth cache 版本升级必须兼容旧缓存和失效逻辑。
- 多分组绑定必须与 `AllowedGroups`、订阅型分组、fallback group 语义一致，否则会产生 Key 越权访问分组或错误 fallback。
- 模型到平台/分组的解析必须有确定性，否则同一请求可能路由到错误分组。
- 管理员和 owner 的用量统计要同时支持“按 Key 合计”和“按 Key + group 拆分”。

因此，多分组 Key 应作为单独阶段，先写 ADR，再实现。

## 实施阶段建议

### 阶段 0：文档和边界确认

- 完成这份设计文档。
- 给 Claude 审查权限矩阵、API 形态和多供应商 Key 方向。
- 不改代码。

### 阶段 1：Owner 用量总览后端（已完成）

不改 schema，基于现有 `usage_logs` + `api_keys`：

- 已新增 summary。
- 已新增 Key leaderboard。
- 已新增 owner model stats。
- 已新增 group/tag stats。
- 已新增 total trend。
- 所有接口硬绑定 `subject.UserID`。
- 所有 DTO 去除 `account_cost`、账号、渠道、upstream model。

推荐测试：

- owner 只能看到自己的 Key。
- 传入别人的 `api_key_id` 或构造筛选不会返回数据。
- 响应 JSON 不含 `account_cost`。
- 超范围日期返回 400。
- timezone 分桶正确。

### 阶段 2：Owner 用量总览前端（已完成）

- 在用户侧 Usage 页面增加 `分析` tab。
- 复用现有图表组件和 `ApiKeyUsageModal`。
- 数字使用 K/M/B 紧凑显示，悬停保留完整数值。
- leaderboard 点击打开单 Key Modal。
- 不引入新图表依赖。

### 阶段 3：Admin 全站分析增强

- 增强 admin Usage/Dashboard 的 Key 排行和用户 drilldown。
- 管理员可看 `account_cost`、账号、渠道、upstream model。
- 增加全站异常列表。
- 可考虑导出。

### 阶段 4：Key Access Profile / 多分组 Key

- 先写 ADR。
- ADR 必须先梳理 `users.AllowedGroups`、订阅型分组、`api_keys.group_id`、auth snapshot `GroupID` / `Group`、group fallback 之间的现有关系。
- 设计 `api_key_group_bindings` 或 access profile schema。
- 绑定校验必须复用现有分组可绑定规则，禁止把 Key 绑定到 owner 自身无权访问的分组。
- auth snapshot v2。
- gateway 选择最终 group。
- usage log 写最终 group。
- UI 把单分组选择升级为访问范围。

### 阶段 5：预聚合与异常治理

当 owner/admin 查询在真实数据量下出现压力，再增加：

- `usage_api_key_daily` 或 owner 级聚合。
- tag/group/model 维度的周期汇总。
- 异常规则配置和通知。

第一版不先上预聚合，避免过早设计一套难维护的数据管道。

## 验证清单

### 文档审查

- 权限矩阵是否覆盖用户、owner、admin。
- owner DTO 是否明确排除 admin-only 字段。
- 多供应商 Key 是否不破坏 ADR 0002。
- 多供应商 Key 是否明确继承 `AllowedGroups`、订阅型分组和 fallback group 的现有授权语义。
- 阶段划分是否避免把 schema 重构混入 owner analytics。
- tags 统计是否明确“重复计入”契约，且没有把 `share_percent` 暗示为总和 100% 的占比。
- summary 是否区分历史时间范围聚合与当前实时治理快照。

### 后端实现时

- owner 接口 cross-user 403/空结果测试。
- owner 接口响应字段白名单测试。
- admin 接口仍受 admin middleware 保护。
- 日期范围上限测试。
- timezone 分桶测试。
- tag/group/model 过滤组合测试。
- tags 统计响应字段白名单测试：第一版不返回 `share_percent`；如未来增加百分比字段，测试必须覆盖多标签 Key 下的分母语义。
- 大 Key 数量下的 SQL explain 或至少索引命中检查。
- 阶段 4 另加多分组 Key 授权测试：owner 不能把 Key 绑定到自己不可绑定的专属分组、未订阅分组或非法 fallback 目标。

### 前端实现时

- 类型检查和 lint。
- owner 页面过滤、排序、空态、错误态。
- leaderboard drilldown 到已完成的单 Key Modal。
- 快速切换筛选时过期响应不会覆盖当前状态。
- 移动端表格不挤压关键按钮。

## 结论

owner 级 API Key 用量总览第一版已经落地：

- 对企业 owner：看自己名下所有员工 Key 的排行、趋势、模型分布、分组/标签拆分和异常。
- 对平台管理员：继续增强 admin Usage/Dashboard，包含全站用户/Key/分组/账号/成本视角。
- 对多供应商员工：短期用标签归并，长期做一把 Key 多分组访问范围，不建议长期让员工保存多把供应商 Key。

这个拆法保持了权限边界清晰，也让后续工作可以单独推进异常治理、admin 全站增强和 Key Access Profile，而不是把高风险网关认证和 schema 重构混入 owner analytics 第一版。
