# 运维失败归因、SLA 与事件视图重构

> 状态：核心重构已实现，待发布后生产对账。2026-07-18 已落地 v2 分类双写、确定性回填、raw/preagg 统一统计、结构化钻取、固定 15 分钟当前状态，以及健康评分、告警和报表的新口径；主要故障事件聚合与 HTTP 200 后流式终态去重仍属于阶段 5。第一版兼容背景见 [运维监控客户可见错误排障](./ops-customer-visible-error-triage.md)。

## 结论

运维监控不能继续使用 `is_business_limited` 一个布尔值同时决定：

- 客户是否看到了失败；
- 错误由客户、企业配置、平台、上游还是客户端造成；
- 是否计入平台 SLA；
- 明细默认展示在哪个入口；
- 健康评分、告警和定时报表是否受影响。

当前实现采用结构化失败分类，并把以下三个问题拆开：

```text
客户可见性：客户最终是否失败？
失败归因：失败发生在哪里、谁能处理？
SLA 影响：是否属于平台未能履约？
```

顶部看板同时提供：

1. 客户可见失败率；
2. 平台可用性；
3. 账户/权限/请求、平台容量、上游服务、客户端中断等归因分布；
4. 固定 15 分钟当前状态，区分“故障仍在持续”与“历史窗口尚未消退”；
5. 失败请求次数，并明确注明包含客户端重试；
6. 后续的主要故障事件聚合，不用一万条重试掩盖两次真实事故。

## 事实基础

### 重构前数据路径

以下旧路径是本轮重构的事实起点。它已经具备较完整的错误证据，但分类维度仍然混杂：

- `ops_error_logs` 保存 `error_phase`、`error_type`、`error_owner`、`error_source`、上游状态和 `is_business_limited`；
- `classifyOpsErrorLog` 会根据显式 marker、上游上下文、错误码和错误消息计算阶段与业务限制；
- `routingCapacityLimited` 当前会直接令 `is_business_limited=true`；
- dashboard raw、preagg、trend 和 metrics collector 都按 `is_business_limited` 计算 `business_limited_count` 与 `error_count_sla`；
- 健康评分、告警、定时报表继续消费同一 SLA 和 error rate；
- 前端把 `business_limited_count` 展示为“客户侧限制”，把 `error_count_sla` 展示为“SLA 错误”；
- 错误明细的 `view=errors|excluded|all` 继续映射到 `is_business_limited=false|true|不筛选`。

因此这不是一个局部卡片问题，而是贯穿写入、存储、聚合、展示和告警的指标合同问题。

### 2026-07-17 线上六小时参考样本

截图时：

| 当前指标 | 数量 |
| --- | ---: |
| 全部失败 | 10,014 |
| 客户侧限制 | 9,702 |
| SLA 错误 | 312 |
| 客户可见失败率 | 60.50% |

只读查询时滚动窗口已经前移，得到：

| 当前指标 | 数量 |
| --- | ---: |
| 全部失败 | 9,907 |
| 客户侧限制 | 9,595 |
| SLA 错误 | 312 |
| 成功请求 | 约 6,507 |

重新归因后：

| 目标分类 | 数量 | 主要原因 |
| --- | ---: | --- |
| 账户、企业策略与请求限制 | 4,990 | 主账号余额、成员预算、Key、分组、模型和协议 |
| 平台路由容量不足 | 4,813 | 平台托管 OpenAI 分组无可用账号 |
| 上游服务异常 | 65 | 最终上游 502 等错误 |
| 客户端中断 | 39 | 499、取消或连接断开 |
| 合计 | 9,907 | |

这组数据暴露了两个方向相反的分类错误：

- 4,813 条平台路由容量失败被排除出 SLA，并显示成客户侧限制；
- 账户、成员预算、模型、协议和客户端中断却有 247 条进入当前 SLA 错误。

参考样本必须固化为后续实现的回归 fixture，不作为永久业务阈值硬编码。

## 目标

- 让管理员一眼回答“当前是否仍在故障”。
- 让客户可见失败、平台 SLA 和上游健康分别有清晰口径。
- 让每条终态错误具有稳定、可过滤、可审计的原因码。
- 让卡片、趋势、明细、导出、告警、健康评分和定时报表共享同一个分类器。
- 让平台托管和企业自管上游池在发生时快照责任归属。
- 让已恢复的上游失败保留排障证据，但不污染客户失败和 SLA。
- 让 HTTP 200 后的流式终态失败进入客户体验指标，且不与成功重复计数。
- 保留旧 API 和预聚合兼容窗口，支持小步上线和回滚。

## 非目标

- 本设计不修改计费、扣费、企业成员预算或账号调度的业务规则。
- 本设计不把运维监控变成通用 BI 系统。
- 第一阶段不自动执行故障修复、账号充值或路由切换。
- 第一阶段不对客户公开管理员级上游账号、成本、原始错误或平台内部信息。
- 第一阶段不删除 `is_business_limited`、旧 API 字段或旧明细视图。
- 第一阶段不把重试请求物理合并或删除；故障事件只是额外视图。

## 术语

### 失败请求

一次逻辑请求最终向客户返回失败，或在已经开始的流中终止为错误。

页面文案使用：

```text
失败请求次数（包含客户端重试）
```

### 已恢复的上游失败

某次上游尝试失败，但系统通过账号或分组 failover 最终向客户返回成功。

这类事件影响上游健康，不属于客户可见失败，也不属于平台 SLA 失败。

### 客户可见失败率

回答“客户最终看见了多少失败”，不表达责任归属。

### 平台可用性

回答“对于符合服务条件的有效请求，平台是否成功提供服务”。这是平台 SLA 的运维口径。

### 处理方

能够采取主要修复动作的角色，不等同于技术错误发生位置。

例如上游供应商返回 503 时，技术发生位置是 upstream，但客户只能向平台反馈，因此主要处理方仍是 `platform_ops`。

## 核心设计：正交分类

### 1. `event_scope`

闭集：

| 值 | 含义 |
| --- | --- |
| `request_terminal` | 普通 HTTP 请求最终失败 |
| `stream_terminal` | HTTP 200 或已 flush 后，流最终以错误终止 |
| `upstream_attempt_recovered` | 中间上游尝试失败，最终请求成功 |

### 2. `customer_visible`

| 值 | 含义 |
| --- | --- |
| `true` | 客户最终收到失败或流终止错误 |
| `false` | 客户最终成功；当前记录仅是中间上游健康证据 |

`event_scope` 与 `customer_visible` 都显式存储，避免仅靠 HTTP 状态推断流式和 failover 场景。

### 3. `failure_domain`

闭集：

| 值 | 含义 | 典型处理方 |
| --- | --- | --- |
| `customer` | 客户账户或 Key 自身状态 | customer |
| `enterprise` | 企业内部预算、权限和分组策略 | enterprise_admin |
| `client` | 请求、协议、客户端或网络终止 | client |
| `platform` | 平台路由、依赖、实现或内部故障 | platform_ops |
| `upstream` | 上游供应商最终错误或退化 | platform_ops |
| `unknown` | 历史或新路径缺少可靠分类证据 | unknown |

### 4. `failure_category`

第一版闭集：

```text
authentication
balance
budget
quota
rate_limit
concurrency
permission
capability
protocol
routing_capacity
credential
overload
timeout
network
dependency
internal
cancellation
unknown
```

新增类别必须同时更新：

- 分类器；
- API 枚举；
- 中英文文案；
- 聚合与明细测试；
- 本文分类矩阵。

### 5. `failure_reason`

原因码是比 category 更具体的稳定闭集，例如：

```text
user_balance_exhausted
enterprise_member_budget_exhausted
api_key_invalid
api_key_quota_exhausted
group_unavailable
model_not_authorized
unsupported_protocol
no_available_accounts
all_accounts_cooldown
provider_rate_limited
provider_balance_exhausted
provider_timeout
provider_4xx
provider_5xx
provider_error_unknown
client_cancelled
client_disconnected
database_unavailable
redis_unavailable
internal_error
legacy_unknown
```

原因码不能直接使用供应商原始错误文案，也不能把可变的人类文案作为查询合同。

### 6. `resolution_owner`

闭集：

```text
customer
enterprise_admin
platform_ops
client
unknown
```

上游错误默认由 `platform_ops` 对客户负责；供应商责任继续由 `failure_domain=upstream` 表达。

### 7. `pool_ownership`

闭集：

```text
platform
enterprise
unknown
```

仅在路由容量、上游账号凭据、账号余额和账号池状态相关错误中有意义。发生时写入快照，配置变化不得回写历史。

当前产品只允许平台管理员维护上游账号，因此现有池默认为 `platform`。未来引入企业自管账号前必须先保证该字段全链路可用。

### 8. `sla_impact`

三态：

| 值 | 含义 |
| --- | --- |
| `true` | 计入平台 SLA 失败 |
| `false` | 不计入平台 SLA |
| `NULL` | 缺少可靠证据，不允许静默归入任一侧 |

出现 `NULL` 时：

- 看板显示“未分类”数量；
- SLA 卡片显示数据质量提示；
- 健康评分不能把未知当成健康；
- 触发分类完整性观察；
- 明细提供 `sla_impact=unknown` 筛选。

### 9. `classification_version`

记录写入时使用的分类规则版本。初始新口径为 `2`；现有 `is_business_limited` 口径视为版本 `1`。

后续修改归因规则时必须增加版本，不能悄悄改变历史解释。

## 分类决策优先级

分类器按以下优先级工作：

1. 明确终态：普通终态、流式终态、已恢复上游尝试；
2. 显式 typed marker 或领域错误码；
3. 请求发生时的账号池 ownership；
4. 上游尝试链和 account-auth / provider 上下文；
5. 稳定错误码；
6. 仅为旧路径保留受限的错误文本回退；
7. 无法可靠判断时写 `unknown`。

禁止：

- 仅凭 `status_code` 决定责任；
- 仅凭中英文 message 决定新路径分类；
- 查询时使用当前账号或分组状态重算历史责任；
- 前端根据标题或颜色自行推断 SLA。

## 第一版分类矩阵

| 场景 | domain | category | reason | owner | SLA | 说明 |
| --- | --- | --- | --- | --- | ---: | --- |
| 用户余额不足 | customer | balance | `user_balance_exhausted` | customer | 否 | 企业主账户余额也属于客户账户层 |
| API Key 无效 | customer | authentication | `api_key_invalid` | customer | 否 | 包含缺少、失效或找不到 |
| API Key 停用/过期 | customer | authentication | `api_key_disabled` / `api_key_expired` | customer | 否 | |
| API Key 额度耗尽 | customer | quota | `api_key_quota_exhausted` | customer | 否 | |
| 用户或 Key RPM/并发限制 | customer | rate_limit / concurrency | 对应稳定 reason | customer | 否 | |
| 企业成员月预算耗尽 | enterprise | budget | `enterprise_member_budget_exhausted` | enterprise_admin | 否 | 必须修复当前误入 SLA 的行为 |
| 企业分组未授权/未分配 | enterprise | permission | `group_unavailable` / `group_unassigned` | enterprise_admin | 否 | |
| 企业策略不允许模型/入口 | enterprise | permission | `model_not_authorized` / `endpoint_not_allowed` | enterprise_admin | 否 | |
| 客户请求体或协议不兼容 | client | protocol | `invalid_request` / `unsupported_protocol` | client | 否 | |
| 客户请求不存在的公开模型 | client | capability | `model_not_found` | client | 否 | 仅当公开能力本就不提供该模型 |
| 平台已承诺模型但账号池不支持 | platform | routing_capacity | `model_capacity_missing` | platform_ops | 是 | 不能与客户拼错模型混为一谈 |
| 平台托管池无可用账号 | platform | routing_capacity | `no_available_accounts` | platform_ops | 是 | 修复 2026-07-17 主误分类 |
| 平台托管池所有账号 cooldown | platform | routing_capacity | `all_accounts_cooldown` | platform_ops | 是 | |
| 企业自管池无可用账号 | enterprise | routing_capacity | `enterprise_pool_unavailable` | enterprise_admin | 否 | 未来能力 |
| 平台内部依赖不可用 | platform | dependency | `database_unavailable` / `redis_unavailable` | platform_ops | 是 | |
| 平台内部 500/panic | platform | internal | `internal_error` | platform_ops | 是 | |
| 平台托管上游凭据失败 | upstream | credential | `provider_auth_failed` | platform_ops | 是 | 保留 upstream 根因 |
| 平台托管上游余额耗尽 | upstream | balance | `provider_balance_exhausted` | platform_ops | 是 | |
| 最终上游 429 | upstream | rate_limit | `provider_rate_limited` | platform_ops | 是 | 与内部限流分开 |
| 最终上游 529/过载 | upstream | overload | `provider_overloaded` | platform_ops | 是 | |
| 最终上游其它 4xx | upstream | protocol | `provider_4xx` | platform_ops | 是 | 不能把供应商协议/能力拒绝伪装成 5xx |
| 最终上游 5xx | upstream | internal | `provider_5xx` | platform_ops | 是 | |
| 最终上游超时/网络错误 | upstream | timeout / network | `provider_timeout` / `provider_network_error` | platform_ops | 是 | |
| 最终上游状态不足以判定类别 | upstream | unknown | `provider_error_unknown` | platform_ops | 是 | 保留上游终态责任，不猜成 5xx |
| 上游失败后 failover 成功 | upstream | 对应类别 | 对应 reason | platform_ops | 否 | `customer_visible=false` |
| 客户端主动取消 | client | cancellation | `client_cancelled` | client | 否 | 包含 499 的明确取消 |
| 客户端连接断开 | client | network | `client_disconnected` | client | 否 | 与上游超时分开 |
| 成员预算结果不确定且本地提交失败 | platform | internal | `budget_outcome_ambiguous` | platform_ops | 是 | 上游副作用与本地事实不一致风险 |
| 无法确定的历史行 | unknown | unknown | `legacy_unknown` | unknown | 未知 | 单独展示 |

## 指标合同

### 逻辑请求终态

一个逻辑请求在选定时间窗口内只能有一个终态：

```text
success
customer_visible_failure
```

已恢复上游尝试不创建第二个逻辑请求终态。

如果流式请求已经写入用量事实，但随后以错误终止：

- 用量事实继续保留，不能因失败删除；
- 运维终态为失败；
- dashboard 不能同时把该 request id 计为成功和失败；
- request id 缺失时进入数据质量计数，不允许无提示双计。

### 客户可见失败率

```text
customer_visible_failure_rate
= customer_visible_failure_count
  / (success_terminal_count + customer_visible_failure_count)
```

它包含客户、企业、客户端、平台和上游所有最终失败，只回答客户体验，不表达责任。

### 平台 SLA

```text
platform_sla
= success_terminal_count
  / (success_terminal_count + platform_sla_failure_count)
```

其中：

```text
platform_sla_failure_count
= count(customer_visible = true AND sla_impact = true)
```

客户账户、企业策略、客户端问题不进入分子或分母。

### 未分类数量

```text
classification_unknown_count
= count(customer_visible = true AND (
    classification_version < 2
    OR sla_impact IS NULL
  ))
```

其中全 v2 窗口可简化为 `customer_visible=true AND sla_impact IS NULL`。滚动发布期间，旧实例晚写的 v1 行即使能用 `is_business_limited` 提供临时 headline 兼容值，也仍进入 unknown 数据质量覆盖层；非 recovered 的 HTTP 200 流式终态失败必须客户可见，但不得在证据不足时猜测 SLA 责任。

当该值大于 0 时：

- SLA 仍可展示已分类范围的 provisional 值；
- 必须同时显示“存在 N 条未分类失败”；
- 卡片不得显示绿色“数据完整”状态；
- 定时报表必须包含未知数量。

### 上游健康

上游健康同时观察：

- 最终导致客户失败的上游错误；
- 被 failover 恢复的上游尝试。

但只有前者进入客户可见失败和平台 SLA。

## 参考样本的目标结果

对 2026-07-17 查询时的样本：

```text
success_terminal_count = 6,507
customer_visible_failure_count = 9,907
platform_sla_failure_count = 4,878
sla_excluded_failure_count = 4,990 + 39
classification_unknown_count = 0
```

其中：

```text
platform_sla_failure_count
= 4,813 平台路由容量不足
 +    65 最终上游错误
```

目标断言：

- 4,813 条 `no_available_accounts` 不得再进入客户/企业限制；
- 4,707 条主账号余额不足不进入平台 SLA；
- 72 条成员预算耗尽不进入平台 SLA；
- 65 条最终上游错误进入平台 SLA；
- 39 条客户端中断不进入平台 SLA；
- 所有分类之和必须等于 9,907；
- 使用相同过滤打开详情时总数必须一致。

## 数据模型

### `ops_error_logs` 新增字段

实施时分阶段增加以下可空字段，避免一次迁移强制重写高写入表：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `event_scope` | VARCHAR(32) | `request_terminal` / `stream_terminal` / `upstream_attempt_recovered` |
| `customer_visible` | BOOLEAN | 客户最终是否失败 |
| `failure_domain` | VARCHAR(32) | 发生领域 |
| `failure_category` | VARCHAR(32) | 稳定类别 |
| `failure_reason` | VARCHAR(64) | 稳定原因码 |
| `resolution_owner` | VARCHAR(32) | 主要处理方 |
| `pool_ownership` | VARCHAR(16) | 账号池责任快照 |
| `sla_impact` | BOOLEAN NULL | true / false / unknown |
| `classification_version` | SMALLINT | 分类规则版本 |

`error_phase`、`error_owner`、`error_source` 和 `is_business_limited` 暂时保留。

### 为什么保留旧技术字段

- `error_phase` 适合定位 auth / routing / upstream / internal 技术阶段；
- `error_owner` 当前表达 client / provider / platform 的技术归属；
- 新 `resolution_owner` 表达谁应采取主要行动；
- `is_business_limited` 只用于旧 API 和旧预聚合兼容，迁移完成后不再作为新指标来源。

### 索引

索引在回填后使用 `CREATE INDEX CONCURRENTLY` 创建，优先覆盖时间窗口、详情入口和滚动发布探针：

```text
(created_at DESC) WHERE <customer_visible 兼容表达式>
(<sla_impact 兼容表达式>, created_at DESC)
(failure_domain, created_at DESC)
(failure_category, created_at DESC)
(failure_reason, created_at DESC)
(created_at DESC) WHERE COALESCE(classification_version, 0) < 2
```

维度索引不能只使用 `WHERE customer_visible IS TRUE`，否则迁移后仍由旧实例写入、结构化列为空的兼容行会被索引排除。customer-visible 与 SLA 索引必须复用读取侧同一表达式；最后一个 partial index与 preagg 的 v1 `EXISTS` 探针使用完全相同的谓词。真实 PostgreSQL 集成测试在关闭顺序扫描后用 `EXPLAIN` 验证这些查询分别命中目标索引。

最终索引数量必须以 `EXPLAIN ANALYZE` 和线上写入压力为依据；不为每个枚举组合建立索引。

## 写入分类器

### 显式 marker 优先

现有 `MarkOpsClientBusinessLimited`、`MarkOpsGroupRetry` 和上游错误上下文继续提供信号，但需要升级为结构化 marker，例如：

```go
type OpsFailureClassification struct {
    Domain          OpsFailureDomain
    Category        OpsFailureCategory
    Reason          OpsFailureReason
    ResolutionOwner OpsResolutionOwner
    PoolOwnership   OpsPoolOwnership
    SLAImpact       *bool
}
```

领域代码在产生错误时设置明确原因，logger 只负责补全和验证，不再通过长字符串列表承担主要分类。

### 旧路径文本回退

旧错误路径可以暂时继续通过标准错误码和受限文本回退，但必须满足：

- 新增路径不得只依赖 message；
- 回退命中写入分类版本和回退来源，便于统计迁移进度；
- 未命中写 `unknown` 并计入分类完整性告警；
- 不允许前端再次解析 message。

### 兼容字段派生

迁移期 `is_business_limited` 从新分类派生：

```text
true:
  customer / enterprise / 明确的 client 前置条件失败

false:
  platform / upstream / unknown
```

但新聚合绝不能再反向从 `is_business_limited` 推断 domain 或 SLA。

## 预聚合与查询

### Raw 与 preagg 必须同义

当前 `ops_repo_dashboard.go`、`ops_repo_preagg.go`、`ops_repo_trends.go` 和 `ops_metrics_collector.go` 分别实现相似 SQL。第二阶段必须先抽出共享聚合合同或 SQL 片段，避免四处复制新规则。

最低要求：

- raw 与 preagg 对同一 fixture 返回完全一致的计数；
- 自定义时间窗口的 head / preagg / tail 合并保持相同分类；
- 卡片和详情使用同一枚举值，不使用不同字符串解释；
- 历史 v1 与新 v2 数据不得无提示混算。

### `ops_metrics_hourly`

第一阶段继续复用现有小时预聚合，增加闭集计数：

```text
customer_visible_failure_count
platform_sla_failure_count
sla_excluded_failure_count
classification_unknown_count
customer_failure_count
enterprise_failure_count
client_request_failure_count
client_transport_failure_count
platform_routing_failure_count
platform_internal_failure_count
upstream_terminal_failure_count
upstream_recovered_attempt_count
```

原因码级明细第一阶段仍查 raw 表；如果真实长窗口查询证明需要，再引入按 reason 维度的伴随预聚合表，不提前扩张。

小时与日预聚合都保存 `classification_version`：只有窗口内全部桶均为 v2 时，dashboard 才使用预聚合结果；遇到 v1 桶会显式回退 raw 查询，避免 headline 已更新但详细归因仍为零的混算。桶版本取参与错误行的最小分类版本，因此滚动部署中旧实例晚写的 v1 行会进入未分类并保持 raw 回退，不能把混合桶误标为 v2。读取预聚合前还会对同一稳定段执行原始 v1 `EXISTS` 校验，防止旧版本聚合器没有维护新版本列而留下伪 v2 桶。聚合任务以 31 天保留窗口重建迁移已回填数据的 v2 小时桶，再由小时桶生成日桶；迁移后晚写的 v1 原始行不会被聚合器猜测升级，需保留 raw 回退直至离开查询窗口或由后续确定性回填处理。

读取侧对晚写 v1 流式记录使用严格兼容规则：`status<400 + stream=true` 且不匹配 `upstream/account_auth + message 前缀 Recovered` 的行，视为客户可见终态并进入 unknown；严格 recovered 行只保留 provider-health 证据。`cyber_policy` 与 `cyber_policy_session_blocked` 是确定性客户策略结果，始终不计平台 SLA。

## API 合同

### Dashboard overview

现有路由保持不变，新增字段：

```json
{
  "classification_version": 2,
  "customer_visible_failure_count": 9907,
  "customer_visible_failure_rate": 0.6036,
  "platform_sla_failure_count": 4878,
  "sla_excluded_failure_count": 5029,
  "classification_unknown_count": 0,
  "failure_breakdown": [
    { "domain": "customer", "count": 4782 },
    { "domain": "enterprise", "count": 155 },
    { "domain": "client", "count": 92 },
    { "domain": "platform", "category": "routing_capacity", "count": 4813 },
    { "domain": "upstream", "count": 65 }
  ],
  "current_window": {
    "seconds": 900,
    "state": "recovered",
    "success_count": 381,
    "customer_visible_failure_count": 1,
    "platform_sla_failure_count": 0
  }
}
```

示例数字只用于说明结构，不作为固定响应。

旧字段：

```text
business_limited_count
error_count_sla
request_count_sla
sla
error_rate
```

至少保留一个正式发布周期，并在类型和文档中标注 deprecated。前端切换完成后，旧字段仅服务兼容客户端。

### 错误明细过滤

错误列表新增精确过滤参数：

```text
event_scope
customer_visible
failure_domain
failure_category
failure_reason
resolution_owner
pool_ownership
sla_impact=true|false|unknown
classification_version
```

旧 `view=errors|excluded|all` 在兼容期映射为：

```text
errors   -> sla_impact=true
excluded -> sla_impact=false
all      -> customer_visible=true
```

该映射必须在 API 文档中标为 legacy；新卡片直接发送结构化参数，不再通过标题或 view 名称拼条件。

### 详情返回

详情新增：

- 客户是否最终失败；
- 是否计入平台 SLA；
- 为什么计入或排除；
- 发生领域；
- 处理方；
- 分类原因码和分类版本；
- 账号池 ownership；
- 已恢复上游尝试与最终终态的关系。

## 页面信息架构

### 顶部卡片

保留现有紧凑控制台风格，不新增装饰性大卡或新的视觉系统。

推荐结构：

```text
┌ 平台可用性 ─────────────────────┐
│ 57.154%               当前已恢复 │
│ 平台失败 4,878 · 未分类 0         │
└─────────────────────────────────┘

┌ 客户可见失败 ───────────────────┐
│ 60.50%                           │
│ 失败请求 9,907（包含客户端重试）  │
│ 账户/权限/请求       4,990        │
│ 平台路由容量         4,813        │
│ 上游服务异常            65        │
│ 客户端中断              39        │
└─────────────────────────────────┘
```

颜色不是唯一语义：

| 类型 | 视觉建议 |
| --- | --- |
| 账户/企业策略/请求 | amber + 明确文字 |
| 平台路由/内部 | red + 平台需处理 |
| 上游 | orange + 上游原因 |
| 客户端中断 | neutral/gray |
| 未分类 | neutral + 数据质量警告，不显示绿色健康 |

### 当前状态

固定使用最近 15 分钟计算：

| 状态 | 规则 |
| --- | --- |
| `active` | 当前窗口平台失败率超过现有 SLA 告警阈值 |
| `recovered` | 所选窗口存在平台失败，但当前窗口已回到阈值以内 |
| `quiet` | 所选窗口和当前窗口都没有平台失败 |
| `unknown` | 当前窗口没有足够终态数据，或存在未分类证据且尚未证明 active |

状态判断必须先确认已达到 `active` 阈值，再处理 unknown；未分类记录可以阻止系统宣称 quiet/recovered，但不能遮蔽已经由平台失败率证明的持续故障。自定义历史窗口若不与固定最近 15 分钟重叠，不显示 recovered。
| `unknown` | 当前窗口无数据或存在影响判断的未分类记录 |

文案使用：

```text
故障持续中
当前已恢复
当前无平台故障
状态无法判断
```

不能只用颜色或一个圆点表达。

### 选定窗口与当前窗口

- 大数字继续表示用户选择的 1h / 6h / 24h / 自定义窗口；
- 卡片固定附带最近 15 分钟当前状态；
- 自定义窗口若完全位于历史，不展示“当前已恢复”，改为“当前状态不在所选历史窗口内”，并提供单独当前摘要；
- tooltip 必须说明两个窗口，避免把历史累计误认为实时状态。

### 详情入口

每个归因数字可点击，打开同一时间窗口和同一结构化筛选：

```text
账户、权限与请求
平台路由容量
平台内部错误
上游最终失败
客户端中断
未分类
```

详情标题下显示：

```text
共 4,813 条 · 与打开卡片时快照一致
```

如果滚动窗口已经变化：

```text
当前窗口已滚动；打开时 4,813 条，现查询 4,706 条
```

前端在打开卡片时保存 `start_time`、`end_time` 和显示计数，不只保存 `time_range=6h`，避免明细查询时窗口继续漂移。

## 故障事件视图

第一阶段先完成准确分类；第二阶段增加“主要故障事件”。

### 事件指纹

初始指纹：

```text
failure_domain
+ failure_category
+ failure_reason
+ platform
+ group_id
+ account_id（仅平台/上游）
+ requested_model
+ inbound_endpoint
```

相同指纹连续事件间隔不超过 5 分钟时归为同一候选事件；超过间隔开启新事件。

### 事件摘要

展示：

- 开始和结束时间；
- 是否仍在持续；
- 失败请求次数；
- 成功/失败趋势；
- 受影响用户、Key、成员、模型和端点数量；
- 主要错误原因；
- 平台托管或企业自管；
- 是否计入 SLA；
- 首次恢复时间和最近一次出现时间。

事件聚合不修改底层日志，不作为计费或审计事实。

## 明细体验

错误明细顶部筛选改为：

```text
客户最终失败 / 已恢复上游尝试
发生领域
失败类别
具体原因
处理方
SLA：计入 / 排除 / 未知
平台 / 分组 / 账号 / 用户 / Key / 成员 / 模型 / 端点
```

详情必须用自然语言解释：

```text
计入平台 SLA：平台托管账号池没有可调度账号，客户无法通过自身配置恢复。
```

或：

```text
不计入平台 SLA：企业成员月预算已耗尽，需要企业管理员调整预算或等待周期重置。
```

不能只展示 `true/false`。

## 健康评分、告警与报表

### 健康评分

`computeDashboardHealthScore` 的业务健康部分切换到新平台 SLA error rate 和上游最终失败率。

- 客户余额、企业预算和客户端取消不降低平台健康评分；
- 平台托管池归零必须降低健康评分；
- 未分类数量大于 0 时增加数据质量惩罚或上限，不能显示满分健康；
- 已恢复上游尝试影响上游健康趋势，但不按最终客户失败的同等权重扣分。

### 告警

新增或重构：

| 告警 | 口径 |
| --- | --- |
| 平台可用性下降 | `platform_sla` 低于阈值并持续 |
| 路由容量归零 | platform + routing_capacity 持续或突增 |
| 上游最终错误 | upstream + customer_visible=true |
| 上游退化但已恢复 | upstream_attempt_recovered 异常增长 |
| 分类完整性 | unknown 数量大于阈值 |
| 企业策略拒绝 | 默认不发平台 P1；可通知企业管理员 |

### 定时报表

替换含混字段：

```text
Errors (SLA)
Business Limited
```

改为：

```text
Customer-visible failures
Platform SLA failures
Customer/account/policy exclusions
Client interruptions
Upstream terminal failures
Recovered upstream attempts
Unclassified failures
```

## 历史兼容与迁移

### 阶段 0：文档与 fixture（已完成）

- 本文和 ADR-0004 生效；
- 建立 2026-07-17 固定分类 fixture；
- 固定 fixture 以 9,907 条客户可见失败为守恒基线，其中 4,813 条平台路由容量和 65 条最终上游失败计入平台 SLA；
- 本阶段本身不改变线上口径。

### 阶段 1：可空列与双写（已完成）

- 给 `ops_error_logs` 增加可空新字段；
- 写路径同时维护 v2 字段和兼容字段 `is_business_limited`；
- API 和看板返回分类版本及 unknown 数量，未分类记录不再被静默判给客户或平台。

### 阶段 2：确定性回填与预聚合（已完成）

- 迁移 `192` 在 31 天保留窗口内执行确定性回填，并设置锁等待和语句超时；
- 当前平台托管边界下的 `no_available_accounts` 回填为 platform/routing_capacity/SLA；
- 明确余额、预算、Key、上游和客户端错误按稳定码回填；
- 不明确记录写 unknown；
- 并发创建必要索引；
- hourly/daily 预聚合新增 v2 计数列，旧 headline 列继续作为兼容别名；
- hourly/daily 同步写入参与错误行的最小分类版本；任一 v1 桶都会令 dashboard 回退 raw，滚动部署晚写的 v1 行单列未分类；
- 迁移 `193` 以 `_notx` 并发建立分类查询索引。

### 阶段 3：API 与看板切换（已完成）

- API 返回新字段，保留旧字段；
- 前端切换新卡片、当前状态和结构化明细筛选；
- raw、preagg、trend、distribution 和 metrics collector 共用同一 SQL 分类合同；
- 卡片钻取冻结总览响应的绝对起止时间，避免刷新后出现“卡片有数、明细为空”；
- 旧 API 字段继续返回兼容值；当前没有单独的 v2 功能开关，应用回滚依赖旧字段兼容读取。

### 阶段 4：健康评分、告警和报表切换（代码已完成，待生产观察）

- 健康评分、告警 evaluator 和定时报表统一消费平台 SLA 失败率；
- 健康评分对未分类证据增加上限保护，不能把数据质量缺口显示成健康绿色；
- 本次变更没有发送外部通知；发布后需对比至少一个完整业务周期，并核对正式告警阈值；
- 旧字段进入弃用期。

### 阶段 5：流式终态与事件视图（待实现）

- 完成 HTTP 200 后流式终态去重；
- 上线主要故障事件聚合；
- 根据实际性能决定是否增加 reason 级预聚合。

## 回滚

- 新列和双写为向后兼容，回滚时旧字段仍可读；
- 新 API 字段只增不删；
- 当前没有独立运行时功能开关；如需回退，部署上一应用版本即可继续读取旧字段；
- 发布前后应保存同窗口 raw/preagg 与旧字段对账结果，发现语义异常时先回退应用版本；
- 不通过回滚删除已写入的新分类证据；
- 数据库列和索引的物理删除必须另开迁移，不与功能回滚绑定。

## 测试合同

### 分类器

表驱动覆盖分类矩阵中的每个 reason：

- domain/category/reason/owner/SLA 全字段断言；
- 平台托管与企业自管 ownership 分支；
- 终态失败与 recovered upstream 分支；
- 流式终态；
- typed marker 优先于 message；
- 未知路径不会默认为客户责任。

### 聚合

- raw 与 preagg 同值；
- 自定义窗口 head/preagg/tail 同值；
- trend、distribution 和 overview 同值；
- 旧 v1 与新 v2 分段窗口有明确版本标识；
- 未分类不会静默进入任一侧。

### 卡片与明细

- 每个 breakdown 点击后的明细总数一致；
- 打开时传递绝对 `start_time/end_time`；
- 滚动窗口变化时显示差异提示；
- 自定义历史窗口不误报当前状态；
- 错误详情解释 SLA 原因；
- 键盘、焦点、颜色之外的文字语义符合根级 `DESIGN.md`。

### 健康评分与告警

- 账户余额和成员预算不降低平台 SLA；
- 平台托管池归零降低健康评分并触发容量告警；
- recovered upstream 不进入客户失败告警；
- unknown 阻止虚假绿色健康；
- 定时报表数字与 dashboard 相同。

### 2026-07-17 回归 fixture

固定断言：

```text
customer_visible_failure_count = 9,907
platform_routing_failure_count = 4,813
upstream_terminal_failure_count = 65
platform_sla_failure_count = 4,878
client_transport_failure_count = 39
customer/enterprise/client request exclusions = 4,990
classification_unknown_count = 0
```

### 验证命令范围

实现阶段至少运行：

```bash
cd backend && go test ./internal/handler ./internal/handler/admin ./internal/repository ./internal/service
cd backend && DOCKER_HOST=unix:///path/to/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/repository -run OpsFailureClassificationV2
pnpm --dir frontend test:run
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check
pnpm --dir docs-site docs:build
git diff --check
```

涉及并发索引和大表回填时，还需要在生产规模副本上验证锁等待、语句时间和写入压力。

## 验收标准

1. 管理员能区分客户最终失败、已恢复上游尝试和平台 SLA 失败。
2. 平台托管 `no_available_accounts` 不再显示成客户侧限制，并计入平台 SLA。
3. 用户余额、Key 额度和企业成员预算不计入平台 SLA。
4. 最终上游失败计入平台 SLA，但保留 upstream 归因。
5. 企业自管账号池有独立 ownership，不默认归责平台。
6. 499/客户端取消可查，但不计入平台 SLA。
7. 卡片、趋势、明细、导出、告警、健康评分和报表数字一致。
8. 选定六小时窗口很差但最近十五分钟恢复时，页面明确显示“当前已恢复”。
9. 页面把近一万条记录称为“失败请求次数（包含客户端重试）”，并能进一步查看主要故障事件。
10. 未分类记录单独展示，不制造虚假精确度或虚假健康。
11. 流式终态失败不与成功重复计数。
12. 旧客户端和旧前端在兼容周期内仍能读取现有字段。

## 已确认的产品取舍

| 决策点 | 采用方案 |
| --- | --- |
| 平台托管池无可用账号 | 计入平台 SLA |
| 最终第三方上游错误 | 计入平台对客户 SLA，同时单列 upstream 原因 |
| 企业自管池不可用 | 默认不计入平台 SLA，归企业管理员 |
| 客户端 499 | 进入诊断，不计入平台 SLA |
| 客户端重试 | 保留原始请求数，同时增加事件聚合 |
| 历史回填 | 只做确定性迁移，不明确的标记 unknown |
| 当前状态窗口 | 固定最近 15 分钟，与用户选定历史窗口并列 |
| 旧字段 | 至少保留一个正式发布周期，迁移期双写 |

## 后续可选增强

- 按客户或企业配置独立通知渠道。
- 自动关联账号暂停、余额和模型 cooldown 时间线。
- 事件级确认、负责人、备注和恢复复盘。
- 对外状态页只发布脱敏后的平台可用性，不公开客户或上游账号信息。
- 根据服务套餐定义不同 SLA 范围，但仍复用同一底层分类事实。
