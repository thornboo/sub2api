# 企业用量分析中心

> 状态：已按增量兼容方案落地。现有 owner API Key 用量分析保持兼容；企业账号在同一个 Usage 模块中增加成员默认视角、成员筛选、成员排行、请求与错误记录归因，并继续允许下钻到 Key。

## 1. 文档边界

本文记录：

- 已落地 owner API Key analytics 的真实接口和字段边界。
- 现有 Key analytics 与未来成员 analytics 的兼容关系。
- 企业 owner、Key-only 使用者和平台管理员之间的可见性边界。

本文不重复定义成员、成员 Key、多分组路由、成员预算、导入或缓存语义；这些内容只以 [企业用户成员管理](./enterprise-member-management.md) 为权威来源。

旧的“一把 Key 等于一个员工席位”和 Key Access Profile 方案已经被 ADR-0003 取代。现有 Key 分析继续作为普通用户与历史普通 Key 的稳定能力，但不再承担企业成员身份。

## 2. 已落地能力

- 用户侧 Usage 页面已经提供 owner 分析视图，前端组件为 `frontend/src/components/user/UsageAnalyticsPanel.vue`。
- 用户认证域已经提供：
  - `GET /api/v1/usage/members`
  - `GET /api/v1/usage/analytics/summary`
  - `GET /api/v1/usage/analytics/leaderboard`
  - `GET /api/v1/usage/analytics/members`
  - `GET /api/v1/usage/analytics/models`
  - `GET /api/v1/usage/analytics/groups`
  - `GET /api/v1/usage/analytics/tags`
  - `GET /api/v1/usage/analytics/trend`
- 所有 owner 查询绑定当前认证用户，不接受外部 `user_id`。
- 单 Key 趋势、模型分布和请求记录下钻已经落地。
- 用户侧 DTO 不返回 `account_cost`、上游账号、渠道或真实上游路由。
- 管理员已有全站 Usage/Dashboard、用户/Key 下钻和管理员专属成本字段。

最近同步：2026-07-13。

## 3. 领域关系

### 3.1 现有普通 Key

当前已落地分析仍按：

```text
users
  -> api_keys
      -> usage_logs
```

适用于：

- `account_type=individual` 的普通用户。
- 企业账号升级前创建、尚未显式迁移的历史普通 Key。
- 成员功能上线后的普通 Key 兼容入口。

### 3.2 完整成员分析

成员功能实现后增加：

```text
enterprise user
  -> enterprise member
      -> member keys
          -> actual groups
              -> models / requests
```

三层下钻同时成立：

- 按成员：解决一名成员多 Key 的总量归集。
- 按 Key：继续复用单 Key 趋势、模型和请求记录。
- 按实际分组：记录请求级 `ActiveGroup`，支持多分组成员的成本和用量拆分。

成员改名、换 Key、调整分组、禁用或归档，不改变历史 `usage_logs.member_id` 与成员名称快照。

## 4. 权限与隐私边界

### 4.1 身份矩阵

| 身份 | 定义 | 可见范围 |
| --- | --- | --- |
| Key-only 使用者 | 只有完整 API Key，没有站点登录 | 仅当前 Key 的有限状态和用量摘要 |
| 普通用户 | `role=user, account_type=individual` | 自己名下普通 Key 和用量 |
| 企业 owner | `role=user, account_type=enterprise` | 自己名下成员、成员 Key、公开分组、预算和用量 |
| 平台管理员 | `role=admin` | 全站用户、Key、分组、账号、渠道和运营成本；默认不看成员明细 |

### 4.2 字段矩阵

| 信息 | Key-only | 普通用户 / 企业 owner | 平台管理员 |
| --- | --- | --- | --- |
| Key 名称、状态、过期、quota、限流 | 当前 Key 的有限状态 | 自己名下 Key | 全站 |
| 成员名称/code | 禁止 | 企业 owner 仅自己成员 | 默认禁止 |
| 请求数、Token、用户应付金额 | 当前 Key 有限摘要 | 自己范围聚合和明细 | 全站 |
| 成员预算、开账、调整、预留 | 禁止 | 企业 owner 仅自己成员 | 默认禁止 |
| 上游账号 `account_cost` | 禁止 | 禁止 | 允许 |
| 上游账号、渠道、provider endpoint | 禁止 | 禁止 | 允许 |
| `upstream_model`、映射链 | 禁止 | 默认禁止 | 允许 |
| 其他用户邮箱、余额、角色 | 禁止 | 禁止 | 允许 |
| 利润、成本差、账号倍率 | 禁止 | 禁止 | 允许 |

成员预算使用企业用户最终实际应付金额，不能混入 `account_cost` 或利润字段。

### 4.3 接口边界

- owner API 从认证主体读取 user ID。
- owner DTO 独立定义，禁止直接返回 admin analytics DTO。
- 企业成员 API 位于 `/api/v1/enterprise/*`，仍属于用户认证域。
- 管理员 API 继续受 admin middleware 保护。
- 平台管理员若未来需要成员级排障，必须另立权限和审计决策。

## 5. 现有 Owner API 口径

### 5.1 查询参数

```text
start_date=YYYY-MM-DD
end_date=YYYY-MM-DD
timezone=Asia/Shanghai
granularity=hour|day|week|month
member_id=123
member_scope=all|assigned|unassigned
group_id=123
tags=team-a,frontend
status=active|disabled|quota_exhausted|expired
search=alice
limit=20
```

日期范围必须由后端校验。历史聚合与当前实时 Key 状态必须分开展示，不能把“当前接近 quota 的 Key 数”误认为历史时间点快照。

`member_id` 与 `member_scope` 是附加、可选参数：

- 普通用户不传时，所有旧接口保持原语义。
- 企业账号选择某个成员时，成功记录、统计、趋势、模型、实际分组和错误记录使用同一成员范围。
- `member_scope=assigned` 表示所有已归属成员的事实；`member_scope=unassigned` 表示 `member_id IS NULL` 的企业自身或历史普通 Key 事实。
- 同时选择 `member_id` 与 `api_key_id` 时，后端必须验证 Key 当前归属与成员一致；不能只依赖前端联动。

### 5.2 Summary

`GET /api/v1/usage/analytics/summary` 返回所选时间范围的：

- 用户应付金额。
- 请求数。
- Token。
- 使用过的 Key 数。
- 当前 active/接近 quota/接近限流的 Key 实时快照。

### 5.3 Leaderboard

`GET /api/v1/usage/analytics/leaderboard` 按 Key 返回：

- Key ID、名称、标签、状态。
- 实际分组。
- 请求数、Token、用户应付金额。
- 最后使用时间和趋势比较。

点击排行项继续进入单 Key 下钻。

### 5.4 Models / Groups / Tags / Trend

- models 按用户安全的请求模型聚合，不暴露真实上游映射链。
- groups 只返回用户可见的分组 ID/名称和用户应付金额。
- tags 延续多标签重复归因语义，不作为严格财务分摊。
- trend 按选定时区和粒度返回历史聚合。

## 6. 成员 Analytics 合同

### 6.1 聚合维度

成员功能必须提供：

- 企业成员汇总和排行。
- 成员用量趋势。
- 成员模型分布。
- 成员实际分组分布。
- 成员 Key 拆分。
- 成员请求记录。
- 成员预算进度和预算账本。

成员排行按 `usage_logs.member_id` 聚合；成员当前名称用于列表，历史名称快照用于审计。

`GET /api/v1/usage/analytics/members` 的汇总字段基于完整筛选结果计算，不受前端 `limit` 截断：

- `total`：成员行与“未归属”事实桶合计的排行对象数。
- `member_count`：真实成员数，不包含“未归属”事实桶。
- `budget_risk_member_count`：未归档成员中，当前已用加预留达到自然月预算 80% 的成员数。
- `total_reserved_usd`：完整筛选范围的当前预留总额。
- `total_actual_cost`：所选请求时间范围的用户应付金额总额。

成员视角即使选择“全部”，也显式使用 `member_scope=all` 作为分析口径标记：分组按 `usage_logs.group_id` 的请求事实统计，已删除 Key 不会让历史请求消失。切换到 Key 兼容视角且未选择成员范围时，继续保留原有 Key 当前元数据口径。

### 6.2 预算与请求用量分离

成员页面同时展示但不能混淆：

| 指标 | 事实来源 |
| --- | --- |
| 真实请求金额/Token/次数 | `usage_logs` |
| 迁移当月开账 | `enterprise_member_budget_entries(kind=migration_opening)` |
| 人工调整 | `enterprise_member_budget_entries(kind=manual_adjustment)` |
| 在途预留 | `enterprise_member_budget_periods.reserved_usd` |
| 预算合计已用 | 当月预算账本投影 |

因此，导入了当月已用额度但没有导入请求明细时，“预算已用”可以大于“本站请求用量”；UI 必须通过开账项解释差异。

### 6.3 成员接口

成员管理页的单成员预算与快速用量入口仍以成员设计文档为准；企业完整分析统一进入 `/usage`，避免形成两套互不一致的统计中心：

```text
GET /api/v1/usage/members
GET /api/v1/usage?member_id=:id
GET /api/v1/usage/errors?member_id=:id
GET /api/v1/usage/stats?member_id=:id
GET /api/v1/usage/analytics/summary?member_id=:id
GET /api/v1/usage/analytics/members
GET /api/v1/usage/analytics/models?member_id=:id
GET /api/v1/usage/analytics/groups?member_id=:id
GET /api/v1/usage/analytics/trend?member_id=:id
GET /api/v1/enterprise/members/usage/summary
GET /api/v1/enterprise/members/usage/trend
GET /api/v1/enterprise/members/:id/usage
GET /api/v1/enterprise/members/:id/budget
GET /api/v1/enterprise/members/:id/budget/entries
```

筛选和日期行为应复用现有 owner analytics 的半开时间区间和时区语义，预算周期则始终使用站点权威计费时区。

### 6.4 兼容关系

- 普通 Key analytics 不因成员功能删除。
- 现有 `/usage/analytics/leaderboard` 始终保持 Key 排行语义；新增 `/usage/analytics/members` 承担成员排行，避免静默改变旧客户端结果。
- 企业账号升级后，历史普通 Key 继续出现在兼容区。
- 成员 Key 同时支持成员聚合与单 Key 下钻。
- 一把成员 Key 跨分组时，usage 记录实际执行分组。
- 普通 Key 的 `member_id=NULL`，不能被错误归入虚构成员。

## 7. 信息架构

### 7.1 普通用户

Usage 页面继续提供：

- 摘要。
- Key 排行。
- 模型、分组、标签。
- 趋势。
- 单 Key 下钻。

### 7.2 企业 owner

企业账号默认进入成员视角：

1. 企业总用量摘要。
2. 成员排行和预算风险。
3. 成员趋势、模型和分组。
4. 成员详情下钻。
5. 历史普通 Key 兼容分析。

成员详情再下钻到 Key 和请求，不把一名成员多 Key 拆成多名员工。

企业 Usage 页的交互约束：

- 默认选择“成员”分析维度，仍可切换到“Key”兼容维度。
- 成员筛选位于 Key 筛选之前；选择成员后，Key 候选只保留该成员当前所属 Key。
- “企业自身 / 未归属”作为显式事实桶展示，不伪造成成员实体。
- 已归档成员保留在历史筛选目录中并标注归档，不重新激活，也不丢失历史事实。
- 成员管理页可携带 `member_id` 跳转到 Usage，日期、成员和后续 Key 下钻在同一 URL 状态中表达。

### 7.3 平台管理员

管理员继续使用现有 `/admin/usage` 与 `/admin/dashboard`：

- 用户、Key、模型、分组、账号和成本分析。
- 可见管理员专属 `account_cost` 和上游信息。
- 企业账号仍作为一个 user 聚合。
- 默认不增加成员排行榜或成员详情。

## 8. 聚合、索引与重建

- `usage_logs(user_id, created_at)` 继续支撑 owner 总范围。
- 新增 `usage_logs(member_id, created_at)` 支撑成员时间窗。
- `ops_error_logs` 新增可空的 `member_id`、`member_code_snapshot`、`member_name_snapshot`；旧错误记录保持 `NULL`，不做基于当前 Key 的追溯性回填。
- `ops_error_logs(user_id, member_id, created_at)` 使用独立的并发索引迁移，避免在主事务迁移中长时间锁表。
- 新字段和索引只通过 `180`、`181_notx` 追加迁移交付；已经应用的 `175`、`177`、`178`、`179` 迁移保持校验和不变。
- Key、group、model 聚合必须保持 owner/member 过滤在最外层权限条件内。
- 当真实数据量超过查询 SLO 时，可以增加成员日聚合，但聚合必须可从 usage log 重建。
- 成员预算实时控制读取 budget period 投影；投影必须可从预算账本重建。
- 缓存 key 必须包含 owner/member、筛选、日期、时区、粒度和数据版本。

## 9. 权限与统计测试

### 9.1 现有 Key analytics

- owner 只能看到自己 Key。
- 传入他人 Key、group 或构造筛选不能泄露数据。
- 响应 JSON 不含 admin-only 字段。
- 日期范围、时区分桶、标签重复归因和实时快照语义正确。
- 已删除 Key 的用户侧/admin 侧证据边界保持现状。

### 9.2 成员 analytics

- 跨企业成员 ID 被拒绝。
- 一成员多 Key 正确合并。
- 未归属 Key 只进入 `unassigned` 桶，不污染任意成员。
- 一 Key 多实际分组正确拆分。
- 成员改名、禁用、归档后历史仍归属同一 member ID。
- 错误请求在认证已解析成员 Key 时写入成员 ID 与名称快照；认证前无法可靠识别成员时保持空值。
- 成功记录与错误记录使用相同的 owner/member 越权校验，跨企业成员 ID 必须返回拒绝结果。
- 迁移开账只影响预算，不生成请求数、Token 或伪造 usage。
- 人工调整和对账修复可解释且有 actor/reason。
- 管理员撤回企业分组后，新请求不再进入该分组，历史统计不被改写。
- owner/member DTO 不出现 `account_cost`、账号、渠道、endpoint 或利润。

### 9.3 前端

- individual/enterprise/admin 三视角正确。
- 成员 → Key → request 下钻保持筛选和日期。
- 预算进度区分 usage、opening、adjustment 和 reservation。
- 空态、错误态、慢查询、过期响应和窄屏行为正确。
- 图表有表格或可读数值替代，不只靠颜色传达。

## 10. 结论

现有 owner Key analytics 是已落地且继续维护的兼容能力；ADR-0003 引入的成员 analytics 是企业长期模型：

- 普通 Key 继续按 Key 分析。
- 企业成员按成员聚合多把 Key。
- 成员 Key 按实际 ActiveGroup 拆分多分组用量。
- 预算账本与真实请求用量分开保存、联合解释。
- 平台管理员仍按企业 user 看总量，成员明细不进入默认 admin 视图。

这个边界既保留已经上线的 Key analytics，也消除了“Key 即员工”在身份、预算和多分组路由上的长期限制。
