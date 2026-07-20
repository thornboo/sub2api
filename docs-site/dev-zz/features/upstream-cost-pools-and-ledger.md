# 上游供应商资金池与成本账本

> 状态：部分落地。阶段 1 后端兼容层已实现：`166_upstream_cost_pools.sql` 新增供应商、资金池、账号成本绑定和成本快照表；旧账号级充值接口保留，但会写入账号默认资金池。本期账本只支持 `recharge` / `bonus` / `adjustment` 三类非负金额记录，只有具有实付与到账金额的 `recharge` 定义单位成本并生成成本快照；`bonus` 增加额度但不单独改写当前成本。2026-07-09 已补齐供应商重命名 / 备注 / 归档 / 受限硬删除与账号供应商归属；2026-07-10 又把稳定的默认充值换算 / 参考汇率收回供应商默认资金池配置，并把供应商新增 / 编辑改为 Modal、日常充值收敛为金额输入和自动到账。资金池高级管理页、显式合池、退款 / 冲正 / 作废账本、余额查询迁移和 usage 成本证据仍是后续阶段。

## 这份设计解决什么

当前 `/admin/accounts` 的“成本对比”已经能按账号展示上游充值成本、参考汇率、分组倍率和综合折扣。但真实运营里经常不是“一个账号一个钱包”：

- 同一个供应商可能同时提供 OpenAI、Anthropic、Gemini 等平台的 key。
- 多个不同平台账号可能消耗同一个供应商余额池。
- 同一平台也可能因为供应商分组、钱包、套餐或结算账户不同，被拆成多个独立余额池。
- 一笔真实充值可能同时支撑多个账号；把它记到任意一个账号都会造成账本偏移，重复记到多个账号则会造成充值和成本重复。

这份文档的目标是把“真实充值发生在哪里”和“哪个账号在消费”拆开：

```text
供应商 -> 资金池 / 钱包 -> 账号成本绑定 -> 请求时成本快照
```

长期产品语义应该是：

- 充值账本属于资金池，不属于账号。
- 余额查询属于资金池，不属于账号。
- 账号只表达它如何消费某个资金池，例如平台、供应商侧分组、这把 key 的分组倍率、模型族覆盖和调度参数。
- 历史 usage 写入时必须固化当时的上游成本证据，避免后续修改成本配置污染历史利润。

## 非目标

- 不改变普通用户侧扣费口径。
- 不向普通用户暴露供应商、资金池、上游余额、充值成本或利润。
- 不在请求转发热路径实时查询上游余额。
- 不把旧的账号级充值记录接口作为长期主入口。
- 不要求第一阶段自动按成本改写账号 priority 或启用成本优先调度。

## 当前问题

现有实现把“充值记录”和“成本配置”挂在账号上，适合“一个账号一个独立上游钱包”的简单场景，但不适合共享钱包：

```text
OpenAI 账号 A   -> 供应商 X / 主余额池
Anthropic 账号 B -> 供应商 X / 主余额池
```

如果供应商 X 的主余额池充值 1000 CNY：

- 记在 OpenAI 账号 A：Anthropic 账号 B 看不到真实充值账本。
- 记在 Anthropic 账号 B：OpenAI 账号 A 看不到真实充值账本。
- 两边都记：总充值、加权成本、余额对账都会重复。

所以长期不能再让管理员选择“这笔充值记在哪个账号里”。正确问题应该是：

1. 这笔钱充给哪个供应商？
2. 是该供应商下哪个钱包、分组、余额池或结算账户？
3. 哪些账号共享这个资金池？
4. 每个账号消费这个资金池时按什么倍率折算成本？

## 2026-07-09 / 2026-07-10 已落地补充：供应商管理与账号编辑边界

这次补齐的是 Phase 1 兼容层之上的管理端操作面，并收紧默认配置、真实快照和调度成本的边界。2026-07-20 起综合折扣增加绑定级人民币 / 美元计价基准，普通用户侧返回字段不变。

### 管理端供应商操作

供应商标签页现在支持：

- 新增供应商。
- 编辑供应商名称和备注。
- 归档 / 恢复供应商。归档后不再出现在账号编辑的供应商下拉候选中。
- 受限硬删除。删除是破坏性操作，只允许“完全干净”的供应商执行。

硬删除前置条件：

| 条件 | 口径 |
| --- | --- |
| 无账号绑定历史 | active 绑定返回 `SUPPLIER_HAS_BOUND_ACCOUNTS`；已解绑 / 已归档绑定返回 `SUPPLIER_HAS_BINDING_HISTORY`，两者都要求改用归档以保留审计链 |
| 无非默认资金池 | 默认池可以随供应商一起删除，非默认池表示已有显式资金池结构，需改用归档 |
| 无充值记录 | 包括已软删除 / 历史充值记录；因为 `upstream_recharge_records.cost_pool_id` 是 `ON DELETE RESTRICT` |
| 无成本快照 | 有快照说明该供应商已经产生过成本事实，需保留历史 |
| 不是系统保留供应商 | `upstream_suppliers.is_system = true` 只用于保护旧迁移遗留系统行；正常业务列表和账号绑定候选都过滤它，不再把本地化名称当兜底控制位 |

实现细节：

- `172_upstream_suppliers_system_flag.sql` 为供应商增加 `is_system` 标志，并把既有默认供应商标记为系统行；后端和前端都读该字段保护历史行，同时从正常供应商 / 资金池列表、账号候选和 active 绑定查询中过滤它。
- 后端只允许删除从未绑定过账号的供应商；历史绑定不再为硬删除而清理，避免供应商归属证据消失。
- 前端本地删除预检使用列表里的 active 绑定数量；最终是否可删仍以后端事务内校验为准。
- 前端归档仍有 active 绑定的供应商时会先确认：存量绑定继续生效，但该供应商会从新账号绑定候选中隐藏。
- 删除失败时按错误码提示：`SUPPLIER_HAS_BOUND_ACCOUNTS`、`SUPPLIER_HAS_BINDING_HISTORY`、`SUPPLIER_HAS_COST_DATA`、`SUPPLIER_RESERVED`、`SUPPLIER_NAME_CONFLICT`。

### 账号编辑边界

账号编辑弹窗不挂载旧 `UpstreamCostSettings`，也不在账号主流程里编辑真实充值比例、参考汇率或资金池基础成本；它只维护账号与供应商 / 默认资金池的归属绑定，以及这把上游 key 在供应商侧的分组名、分组倍率和分组计价基准：

- 账号编辑负责选择 / 清空 active 供应商绑定；这一步表达“这把 key 消费哪个供应商 / 默认资金池”。
- 当前绑定供应商归档后仍以禁用历史项展示，存量绑定继续生效；无关字段保存不会解绑，管理员明确点叉号才清空。已归档供应商不能用于新绑定。
- 账号编辑负责保存 `upstream_group_name`、`upstream_group_multiplier` 和 `price_reference_currency`；这一步表达“这把 key 在供应商侧属于哪个分组、按什么倍率、相对人民币还是美元价目表计价”。
- 默认充值换算和默认参考汇率属于供应商默认资金池的低频运营配置；真实支付、到账、汇率和基础成本属于账本与成本快照。调整默认值只影响以后录入，实际成本变化必须通过新 `recharge` 记录和快照固化，而不是覆盖账号 `extra`。
- 账号列表、排序和 `cost_first` 使用同一公式：人民币价基准按 `current_effective_cny_per_usd * upstream_group_multiplier`，美元价基准按 `current_effective_cny_per_usd / reference_fx_rate * upstream_group_multiplier`。历史绑定保留 `USD` 旧口径，但 `price_reference_confirmed=false`，页面显示“待确认”，并且在管理员确认前不进入 `cost_first`。
- 旧 `PATCH /admin/accounts/:id/upstream-cost-profile` 保留为兼容接口，用于读取或迁移历史账号 `extra` 成本字段；不作为新版账号编辑主入口。

后续供应商 / 资金池详情页应继续承担这些高级入口：

- 供应商成本档案：维护供应商公开报价、模型族覆盖和生效时间等高级成本解释；不替代账号/key 绑定上的当前分组倍率。
- 资金池账本：充值、赠送、调整、退款 / 冲正 / 作废。
- 成本快照：按充值或手动重算生成“从某时刻起生效”的基础成本事实。
- 账号绑定列表：展示哪些账号消费该资金池，并允许从供应商成本档案选择适用口径。
- usage 成本证据：请求落账时记录当时绑定、资金池和快照，避免后续供应商调价污染历史利润。

边界：

- `172_upstream_suppliers_system_flag.sql` 增加稳定的系统供应商标志，`173_upstream_account_binding_group_name.sql` 增加供应商侧分组名，`174_upstream_cost_pool_defaults.sql` 把低频默认充值配置与最近一次真实成本拆开保存。
- 综合折扣公式增加绑定级计价基准；不按供应商所在地、模型名或分组名自动推断。
- 不把供应商、资金池、上游余额、真实成本或利润暴露给普通用户侧接口。
- 不把账号 `extra` 成本口径继续扩成新版主流程；历史字段迁移到资金池 / 绑定 / 快照仍是后续专项。

### 供应商默认结算与日常充值

供应商新增 / 编辑统一使用 Modal。普通管理员看到的是供应商级默认结算配置，底层仍写入该供应商的默认资金池：

- `default_effective_cny_per_usd`：默认每 1 USD 额度需要多少 CNY，UI 反向展示为“支付 1 CNY 默认到账多少 USD”。
- `default_reference_fx_rate`：默认参考汇率。
- `current_effective_cny_per_usd` / `reference_fx_rate`：最近一次真实成本快照继续使用的当前口径，不作为下一次录入的稳定默认值。

日常新增 `recharge` 记录只要求输入支付金额；到账额度按默认充值换算自动计算，参考汇率自动带入。只有“本次与默认不同”时才展开实际到账与本次参考汇率覆盖。`bonus` 直接录到账额度，但因为没有独立实付单价，不生成成本快照；`adjustment` 继续保留手工金额入口且不刷新成本快照。

历史保护：

- 每条充值记录仍保存本次实际支付、实际到账和当时参考汇率。
- 修改供应商默认结算配置只影响以后新建的记录，不重算旧流水和旧成本快照。
- 新建供应商的默认配置只是运营配置，不自动生成成本快照，也不写入 `current_effective_cny_per_usd`。只有从未绑定账号、没有非默认资金池、充值记录或快照的干净供应商才可硬删除。

## 领域模型

### 供应商

供应商表示上游服务提供方，例如：

- 自建 New API。
- 某个 OpenAI / Claude 中转站。
- 某个统一多平台供应商。
- 企业合同供应商。

供应商本身是组织层，通常不直接记账。真实账本在资金池上。

### 资金池

资金池是实际充值、余额查询和基础成本计算的对象。

一个供应商可以有多个资金池：

| 供应商 | 资金池 | 说明 |
| --- | --- | --- |
| 供应商 A | 主余额池 | OpenAI、Anthropic 共用余额 |
| 供应商 A | Claude 企业组 | Claude 单独套餐或分组 |
| 供应商 A | OpenAI 高并发组 | OpenAI 单独计价或单独余额 |
| 供应商 B | 默认池 | 单供应商单钱包 |

拆分资金池的判断标准：

- 余额共享：同一个资金池。
- 充值账单共享：同一个资金池。
- 余额、充值、折扣、倍率、有效期或结算账户隔离：拆成不同资金池。
- 同一供应商但不同分组独立扣费：拆成不同资金池。
- 不确定是否共享余额时，先拆开；后续通过合并流程归并。

### 账号成本绑定

账号成本绑定表示某个上游账号消费哪个资金池，以及消费时的成本倍率。

同一个资金池可以绑定多个账号：

```text
供应商 A / 主余额池
  -> OpenAI apikey 账号 1，倍率 1.0
  -> Anthropic apikey 账号 1，倍率 1.4
  -> Gemini apikey 账号 1，倍率 0.8
```

账号自身仍然保留调度和平台属性：

- 平台：OpenAI / Anthropic / Gemini / Grok。
- 类型：apikey / oauth / setup-token。
- 分组、模型限制、优先级、并发限制。
- 账号状态、错误状态、模型级冷却。

账号成本绑定只负责成本和账本解释，不应该替代调度状态。

### 充值账本

充值账本记录资金池的资金变化。

它应该是追加式账本，而不是可以随意改写的普通配置：

- 充值：`recharge`
- 赠送：`bonus`
- 调整：`adjustment`
- 退款：`refund`
- 冲正：`correction`
- 作废：`void`

阶段 1 已落地的兼容接口只接受 `recharge` / `bonus` / `adjustment`，金额仍要求非负；`refund` / `correction` / `void` 和负向调整属于后续账本语义扩展。成本快照只从具有有效单位成本的 `recharge` 计算；`bonus` 和 `adjustment` 都不会单独改写资金池当前成本。

长期原则：

- 金额类错误优先用冲正 / 调整记录表达。
- 作废必须保留操作者、时间和原因。
- 汇总成本可以重算，但原始账本事实应可追溯。

### 成本快照

成本快照记录某个时间点资金池的基础成本。

为什么需要快照：

- 今天充值成本可能是 5 CNY/USD。
- 下个月新充值后加权成本可能变成 4 CNY/USD。
- 如果历史 usage 每次都读取最新资金池成本，上个月利润会被新充值改写。

所以请求发生时必须固化当时命中的成本：

```text
usage -> account -> cost binding -> cost pool -> active cost snapshot
```

历史报表用请求时的快照，当前成本对比用最新快照。

## 推荐数据结构

### `upstream_suppliers`

```text
id
name
status
website
support_contact
note
created_by
created_at
updated_at
deleted_at
```

说明：

- `name` 是管理员可读名称，不进入用户侧接口。
- `status` 可先支持 `active`、`inactive`、`archived`。
- 供应商归档不应删除资金池账本。

### `upstream_cost_pools`

```text
id
supplier_id
name
status
base_currency
credit_currency
reference_fx_rate
cost_method
current_effective_cny_per_usd
current_snapshot_id
balance_query_enabled
balance_provider
balance_endpoint
balance_auth_mode
balance_auth_header
balance_low_threshold
last_balance_snapshot
note
created_by
created_at
updated_at
archived_at
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `supplier_id` | 所属供应商 |
| `name` | 资金池名称，例如“主余额池” |
| `is_default` | 稳定默认池身份；每个未归档供应商至多一个，不依赖中文显示名称识别 |
| `base_currency` | 实付币种，第一阶段可先固定 CNY |
| `credit_currency` | 上游额度币种，第一阶段可先固定 USD |
| `reference_fx_rate` | 参考汇率，例如 7 |
| `cost_method` | 当前成本算法：`latest`、`weighted`、`manual` |
| `current_effective_cny_per_usd` | 当前真实基础成本；只有 `current_snapshot_id` 非空时才进入展示和调度 |
| `current_snapshot_id` | 当前成本快照 |
| `last_balance_snapshot` | 最近一次余额查询结果，后端脱敏保存 |

`current_effective_cny_per_usd` 和 `current_snapshot_id` 是反范式冗余，只应在生成新成本快照的同一个事务里更新；其它读取路径只读不写，避免这两个字段和快照表出现不一致。

### `upstream_account_cost_bindings`

```text
id
account_id
cost_pool_id
status
upstream_group_name
price_reference_currency
price_reference_confirmed
default_multiplier
model_family_multipliers
note
valid_from
valid_to
created_by
created_at
updated_at
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `account_id` | 绑定的上游账号 |
| `cost_pool_id` | 绑定的资金池 |
| `upstream_group_name` | 这把上游 key 在供应商侧所属的分组，例如 `claude-sale` |
| `price_reference_currency` | 上游分组价目表基准，`CNY` 表示人民币官方价，`USD` 表示美元官方价；历史数据暂存 `USD` 仅为保持旧公式 |
| `price_reference_confirmed` | 管理员是否明确确认过计价基准；历史数据为 `false`，不参与成本优先排序或调度 |
| `default_multiplier` | 兼容存储列，当前承载这把上游 key 的分组倍率；API / UI 对外命名为 `upstream_group_multiplier` |
| `model_family_multipliers` | 模型族覆盖，例如 haiku / sonnet / opus |
| `valid_from` / `valid_to` | 绑定历史区间 |

第一阶段可以限制“一个账号同一时刻只有一个 active 绑定”，用 partial unique index（例如 `WHERE status = 'active'`）在库层强制，避免并发写出两条 active 绑定。保留绑定历史是为了后续解释历史 usage。

如果实现上希望降低复杂度，也可以先在 `accounts.extra` 或 `accounts` 新字段中保存当前 `cost_pool_id` 和倍率；但长期仍要把请求时成本固化到 usage，不能依赖当前绑定重算历史。

### `upstream_recharge_records`

长期应从账号归属迁到资金池归属：

```text
id
cost_pool_id
type
paid_amount
paid_currency
received_credit_amount
received_credit_currency
reference_fx_rate
effective_cny_per_usd
recharge_discount
recorded_at
note
source_account_id_snapshot
merged_from_pool_id
source
external_order_id
created_by
created_at
updated_at
voided_at
voided_by
void_reason
```

关键点：

- `cost_pool_id` 是主归属。
- `account_id` 不应再作为账本主归属。
- 如果需要保留旧数据来源，可以加 `source_account_id_snapshot`、`merged_from_pool_id` 或迁移备注。
- 退款、冲正和调整不能被非负金额约束卡死；需要明确方向，或用 `type + signed_amount` 表达。

### `upstream_cost_snapshots`

```text
id
cost_pool_id
effective_cny_per_usd
reference_fx_rate
calculation_method
source_record_id
included_record_ids
valid_from
valid_to
created_by
created_at
note
```

说明：

- `latest` 快照来自最新有效充值记录。
- `weighted` 快照来自有效充值记录的加权成本。
- `manual` 快照来自管理员手动覆盖。
- 新快照生效时，应关闭旧快照的 `valid_to`。

### usage 成本证据字段

长期应在 usage 写入路径固化上游成本证据。字段可以落在 `usage_logs`、`usage_billing` 或专门的 usage cost detail 表中：

```text
upstream_supplier_id
upstream_cost_pool_id
upstream_cost_snapshot_id
upstream_cost_binding_id
upstream_official_cost_usd
upstream_effective_cny_per_usd
upstream_cost_multiplier
upstream_actual_cost_cny
upstream_cost_method
```

这样后续可以稳定回答：

- 用户扣费是多少。
- 官方模型成本是多少。
- 上游真实成本是多少。
- 毛利是多少。
- 当时使用的是哪个资金池和哪版成本快照。

## 成本计算

资金池基础成本：

```text
effective_cny_per_usd = paid_amount_cny / received_credit_amount_usd
```

加权基础成本：

```text
weighted_effective_cny_per_usd =
  sum(paid_amount_cny) / sum(received_credit_amount_usd)
```

账号最终成本：

```text
account_upstream_cost_cny =
  official_cost_usd
  × pool_effective_cny_per_usd
  × account_multiplier
```

折扣展示：

```text
reference_divisor = price_reference_currency == CNY ? 1 : reference_fx_rate
effective_discount = pool_effective_cny_per_usd / reference_divisor × account_multiplier
display_discount = effective_discount × 10
```

只有存在真实 `current_snapshot_id` 且 `price_reference_confirmed=true` 的绑定才生成可用于排序 / 调度的综合折扣。历史未确认绑定继续保留旧美元公式用于兼容解释，但账号列表显示“待确认”，不会把它包装成准确成本。

示例：

```text
资金池充值：1000 CNY -> 200 USD
参考汇率：7
资金池基础成本：5 CNY/USD

OpenAI 账号倍率：1.0
OpenAI 有效折扣：5 / 7 × 1.0 × 10 = 7.1 折

Anthropic 账号倍率：1.4
Anthropic 有效折扣：5 / 7 × 1.4 × 10 = 10.0 折

Kimi 人民币价分组倍率：0.8
Kimi 有效折扣：1 / 1 × 0.8 × 10 = 8.0 折（资金池基础成本为 1 CNY/USD 时）
```

同一资金池下不同账号可以拥有不同倍率，但真实充值仍然只记录一次。

## 页面设计

### 核心交互原则：供应商优先，资金池后置

后台页面不能把数据库模型直接摊给管理员。底层可以是“供应商 -> 资金池 -> 账号绑定 -> 成本快照”，但普通配置流程应该优先表达成：

```text
编辑账号 -> 选择上游供应商 -> 录入这把 key 的上游分组和分组倍率 -> 必要时选择该供应商下的钱包 / 资金池
```

产品契约：

- 管理员第一次配置账号时，先在供应商标签页把“供应商 A”新增到供应商列表。
- 新增 / 编辑供应商使用统一 Modal；名称、备注、默认充值换算和默认参考汇率在这里维护。
- 新增供应商时，系统默认创建一个“主余额池”承载默认结算配置；账号是否绑定到该供应商，仍以账号编辑表单里最终选中的下拉值和账号保存动作为准。
- 后续同一供应商的账号，在账号编辑弹窗里直接从下拉框选择“供应商 A”即可；如果该供应商只有一个 active 资金池，不应强迫管理员理解或选择资金池。
- 只有当同一供应商存在多个钱包、套餐、分组、余额池或结算账户时，页面才展示“资金池 / 钱包”选择器。
- 阶段 1 账号编辑页的主字段是“供应商、上游分组、上游分组倍率、分组计价基准”。真实充值金额、到账额度、参考汇率和资金池基础成本来自供应商 / 资金池账本，不再让管理员在每个账号上重复录入。
- “资金池、成本快照、绑定历史”等术语应尽量隐藏在详情页、审计页或高级设置里，普通账号配置流程只看到供应商和必要的钱包名称。

推荐的账号编辑区域：

```text
上游供应商

供应商：
[ 供应商 A v ]

上游分组：
[ claude-sale ]

上游分组倍率：
[ 1.4 ]

分组计价基准：
[ 美元官方价 v ]

说明：
后续充值记录会归到这个供应商；人民币价分组直接按资金池人民币成本叠加倍率，美元价分组再除以参考汇率。
```

资金池 / 钱包、当前成本、账本入口应放在供应商或资金池详情页；只有当同一供应商存在多个 active 钱包 / 资金池且当前账号必须区分时，账号编辑才补充展示钱包选择器。

第一次使用流程：

```text
供应商标签页顶部新增供应商 A -> Modal 配置默认充值换算 / 参考汇率 -> 编辑账号 -> 选择供应商 A -> 填上游分组和分组倍率 -> 保存账号
```

供应商标签页使用账号页顶部的同一套操作栏：供应商视图隐藏账号搜索、账号筛选、自动刷新和账号工具，只保留手动刷新，并把主操作切换为“添加供应商”；不会同时显示“添加账号”或第二个“新增”按钮。供应商卡片内部不再重复放刷新 / 新增入口。

再次使用流程：

```text
编辑另一个账号 -> 选择供应商 A -> 填这把 key 的上游分组和分组倍率 -> 默认绑定供应商 A / 主余额池 -> 保存
```

如果供应商 A 后续被拆成多个钱包：

```text
编辑账号 -> 选择供应商 A -> 选择“主余额池 / Claude 企业组 / OpenAI 高并发组”
```

### “上游资金池”管理入口

建议新增管理员入口：

```text
/admin/upstream-cost-pools
```

或先在 `/admin/accounts` 下增加“资金池”视图。

这个入口面向高级运营和审计，不是账号首次配置的唯一入口。账号编辑页应支持把新供应商保存到列表，再从同一个供应商下拉框选择归属；资金池管理页用于处理多个钱包、共享余额、账本核对、归档和后续合并。

资金池列表展示：

| 字段 | 说明 |
| --- | --- |
| 供应商 | 所属供应商 |
| 资金池 | 钱包 / 分组 / 结算账户名称 |
| 绑定账号 | 绑定账号数和平台摘要 |
| 当前余额 | 最近一次余额快照 |
| 累计实付 | 有效账本实付合计 |
| 累计额度 | 有效账本获得额度合计 |
| 当前基础成本 | 当前快照 CNY/USD |
| 成本算法 | latest / weighted / manual |
| 状态 | active / inactive / archived |

资金池详情页：

- 基本信息。
- 余额查询配置。
- 充值账本。
- 成本快照历史。
- 绑定账号列表。
- 操作审计。

### `/admin/accounts` 成本对比

账号成本对比仍然保留，但语义调整为“账号最终有效成本”：

| 字段 | 说明 |
| --- | --- |
| 账号 | 当前上游账号 |
| 平台 / 类型 | OpenAI / Anthropic / apikey / oauth |
| 供应商 | 账号绑定的上游供应商 |
| 资金池 / 钱包 | 绑定的供应商资金池；供应商只有一个资金池时可弱化展示 |
| 基础成本 | 来自资金池当前快照 |
| 上游分组 | 这把 key 在供应商侧所属分组 |
| 上游分组倍率 | 来自账号成本绑定；数据库兼容存储列为 `default_multiplier` |
| 分组计价基准 | 来自账号成本绑定；明确显示人民币价基准或美元价基准 |
| 模型族覆盖 | 当前选择模型族的覆盖倍率 |
| 最终折扣 | 资金池成本叠加上游分组倍率后的结果 |
| 余额 | 来自资金池余额快照 |

“充值记录”按钮不应再表达为账号级账本，应该改为“资金池账本”或“供应商账本”。如果一个账号未绑定供应商 / 资金池，入口应引导在账号编辑弹窗中先新增供应商到列表，或直接从下拉框选择已有供应商；若管理员从该入口新增充值，系统应明确提示“这笔充值会记到供应商 A / 主余额池，不只属于当前账号”。

### 账号编辑弹窗

账号编辑的主流程从“录入上游充值成本”改成“选择供应商并记录这把 key 的供应商侧分组”：

- 选择已有上游供应商；新增供应商从供应商标签页完成。
- 录入上游分组名，例如 `claude-sale`。
- 录入上游分组倍率，例如 `1.4`。
- 选择分组计价基准；国产模型不等于人民币定价，必须以上游分组价目表实际币种为准。历史绑定在管理员确认前显示“待确认”，且不参与成本优先调度。
- 如果供应商只有一个 active 资金池，自动绑定默认资金池。
- 如果供应商有多个 active 资金池，展示资金池 / 钱包选择器。

不要在账号编辑里直接新增真实充值记录，避免管理员误以为充值属于账号。可以提供“查看 / 新增资金池账本”的跳转按钮，但打开后的标题必须展示供应商和资金池名称，例如：

```text
供应商 A / 主余额池账本
```

旧版“充值人民币/美元额度、参考汇率、模型系列倍率”等字段可以短期保留在兼容代码和历史数据里，但不应展示在新版账号编辑主流程里。`default_multiplier` 作为底层兼容存储列继续存在，但 UI / API 应使用 `upstream_group_multiplier` 表达“这把 key 的供应商侧分组倍率”。新版主交互应让管理员先选供应商和上游分组倍率，再由系统处理默认资金池、绑定、账本和快照。

## API 设计

本节分为当前已落地接口和长期目标接口。未列入“当前已落地”的编辑资金池、作废账本、余额刷新和快照重算接口仍是后续工作。

当前已落地接口：

```text
GET    /api/v1/admin/upstream-suppliers
POST   /api/v1/admin/upstream-suppliers
PATCH  /api/v1/admin/upstream-suppliers/:supplier_id
DELETE /api/v1/admin/upstream-suppliers/:supplier_id
GET    /api/v1/admin/upstream-cost-pools
GET    /api/v1/admin/upstream-cost-pools/:id
GET    /api/v1/admin/upstream-cost-pools/:id/accounts
GET    /api/v1/admin/upstream-cost-pools/:id/recharge-records
POST   /api/v1/admin/upstream-cost-pools/:id/recharge-records
GET    /api/v1/admin/accounts/:id/upstream-cost-binding
PUT    /api/v1/admin/accounts/:id/upstream-cost-binding
GET    /api/v1/admin/accounts/:id/recharge-records
POST   /api/v1/admin/accounts/:id/recharge-records
```

后续供应商高级接口：

```text
GET    /api/v1/admin/upstream-suppliers
POST   /api/v1/admin/upstream-suppliers
GET    /api/v1/admin/upstream-suppliers/:id
PATCH  /api/v1/admin/upstream-suppliers/:id
POST   /api/v1/admin/upstream-suppliers/:id/archive
```

长期资金池接口：

```text
GET    /api/v1/admin/upstream-cost-pools
POST   /api/v1/admin/upstream-cost-pools
GET    /api/v1/admin/upstream-cost-pools/:id
PATCH  /api/v1/admin/upstream-cost-pools/:id
POST   /api/v1/admin/upstream-cost-pools/:id/archive
POST   /api/v1/admin/upstream-cost-pools/:id/restore
```

长期资金池账本接口：

```text
GET    /api/v1/admin/upstream-cost-pools/:id/recharge-records
POST   /api/v1/admin/upstream-cost-pools/:id/recharge-records
GET    /api/v1/admin/upstream-cost-pools/:id/recharge-records/:record_id
PATCH  /api/v1/admin/upstream-cost-pools/:id/recharge-records/:record_id
POST   /api/v1/admin/upstream-cost-pools/:id/recharge-records/:record_id/void
```

长期成本快照接口：

```text
GET    /api/v1/admin/upstream-cost-pools/:id/cost-snapshots
POST   /api/v1/admin/upstream-cost-pools/:id/cost-snapshots/recalculate
POST   /api/v1/admin/upstream-cost-pools/:id/cost-snapshots/manual
```

长期账号成本绑定接口：

```text
GET    /api/v1/admin/accounts/:id/upstream-cost-binding
PUT    /api/v1/admin/accounts/:id/upstream-cost-binding
GET    /api/v1/admin/upstream-cost-pools/:id/accounts
POST   /api/v1/admin/upstream-cost-pools/:id/accounts/:account_id/bind
DELETE /api/v1/admin/upstream-cost-pools/:id/accounts/:account_id
```

长期余额查询接口：

```text
POST   /api/v1/admin/upstream-cost-pools/:id/balance/refresh
POST   /api/v1/admin/upstream-cost-pools/balance/refresh
```

旧账号级接口兼容：

```text
GET    /api/v1/admin/accounts/:id/recharge-records
POST   /api/v1/admin/accounts/:id/recharge-records
PATCH  /api/v1/admin/accounts/:id/upstream-cost-profile
```

兼容策略：

- 第一阶段保留旧接口，避免前端和外部调用立刻中断。
- 如果账号已绑定资金池，旧 `GET` 可返回该资金池账本，并附带 deprecated 标记。
- 旧 `POST` 不应长期继续创建账号级充值记录；迁移期可以创建到账号绑定的资金池。
- 新实现稳定后，前端主入口只使用资金池接口。

## 迁移方案

### 阶段 1：等价迁移

目标：不改变当前行为，只把模型变成可扩展结构。

当前已落地后端兼容层：

- 新增 `upstream_suppliers`、`upstream_cost_pools`、`upstream_account_cost_bindings`、`upstream_cost_snapshots`。
- `upstream_recharge_records` 新增 `cost_pool_id`、`source_account_id_snapshot`、`merged_from_pool_id`、作废字段和来源字段。
- 历史等价迁移曾为已有账号创建“未归类供应商”下的账号默认资金池和 active 绑定；当前产品路径不再使用该兜底，账号应显式绑定真实供应商。
- 旧 `/api/v1/admin/accounts/:id/recharge-records` 仍可用；账号已绑定资金池时返回该资金池账本，并在响应里带 `deprecated` 与 `cost_pool_id`。
- 新增资金池列表、详情、账本和账号绑定后端接口，供后续管理页切换主入口。
- `GET /api/v1/admin/accounts/:id/upstream-cost-binding` 只读取已有绑定；新账号没有绑定时由显式绑定或旧充值兼容写路径创建默认资金池。

步骤：

1. 创建供应商、资金池、绑定、快照相关表。
2. 历史等价迁移为旧账号补默认资金池 / 绑定；升级后的正常业务不再给未绑定账号自动创建“未归类供应商”兜底，管理员应为账号绑定真实供应商。
3. 将账号 `extra` 中的上游成本字段迁入账号绑定和资金池当前配置。
4. 将 `upstream_recharge_records.account_id` 的历史记录迁到对应默认资金池。
5. 为每个资金池生成初始成本快照。
6. `/admin/accounts` 成本对比继续显示等价结果。

迁移后默认状态仍等价于：

```text
一个账号 -> 一个默认资金池
```

这样不会破坏现有管理员的成本配置。

### 阶段 2：资金池合并

目标：支持管理员把多个账号默认资金池合并为真实供应商资金池。

合并流程：

1. 选择目标资金池。
2. 选择要合并进来的源资金池。
3. 展示影响预览：
   - 源资金池账本数量。
   - 源资金池绑定账号。
   - 合并后的加权成本。
   - 是否存在冲突的余额查询配置。
4. 管理员确认。
5. 源资金池账本迁入目标资金池：账本记录保留原资金池引用（例如 `merged_from_pool_id`），只改当前归属、不物理抹除来源，保证合并后仍能追溯每笔充值原本充给哪个池。
6. 源资金池账号绑定改到目标资金池。
7. 生成新的成本快照。
8. 源资金池归档。

注意：历史 usage 如果已经写入成本快照，不应因为合并而重算，除非管理员显式发起重算任务。

### 阶段 3：页面切换

目标：让管理员从产品语义上使用供应商，必要时再进入资金池 / 钱包细分。

改动：

- 账号编辑页新增“上游供应商”选择器，支持搜索已有供应商；新增供应商从供应商标签页完成。
- 新增供应商到列表时自动创建“主余额池”；账号绑定只在管理员从下拉框选定供应商并保存账号时发生。
- 供应商只有一个 active 资金池时，账号编辑页自动选择并弱化资金池概念；供应商有多个 active 资金池时，才展示资金池 / 钱包选择器。
- 账号编辑页阶段 1 展示供应商归属、上游分组名和上游分组倍率；真实充值成本、参考汇率和模型族倍率覆盖从供应商 / 资金池账本或后续供应商成本档案维护。
- 新增资金池管理入口，作为高级运营和审计入口，不作为账号首次配置的唯一入口。
- 成本对比表展示供应商列，并在需要时展示资金池 / 钱包列。
- 账号级“充值记录”改名为“资金池账本”或“供应商账本”，标题展示供应商和资金池名称。
- 旧账号级新增充值入口下线或降级为兼容入口。

### 阶段 4：请求成本快照落账

目标：让历史利润和成本可审计。

改动：

- 网关写 usage 时解析账号成本绑定。
- 命中当前资金池成本快照。
- 固化 `upstream_cost_pool_id`、`upstream_cost_snapshot_id`、`upstream_effective_cny_per_usd`、`upstream_cost_multiplier`、`upstream_actual_cost_cny`。
- 管理员使用分析支持收入、上游成本、毛利视图。

### 阶段 5：调度联动

目标：为成本感知调度提供可靠输入。

前置条件：

- 资金池基础成本稳定。
- 账号绑定上的上游分组倍率稳定。
- 模型级健康和调度解释已经可用。
- 成本快照不会污染历史 usage。

联动方式见 [上游供应商成本感知与模型级调度](./upstream-provider-cost-aware-scheduling.md)。

## 权限和安全边界

供应商、资金池、余额和真实成本都属于管理员运营信息：

- 用户侧接口不得返回供应商名称、资金池名称、上游余额、真实充值成本、调度候选、利润或成本快照。
- 管理员导出可以包含资金池和成本字段，但不得包含余额查询 token、Authorization、cookie 或自定义密钥。
- 余额查询请求必须设置超时、响应大小限制和脱敏日志。
- 默认禁止余额查询访问内网地址；如需访问内网地址，必须由管理员显式开启。
- 资金池归档不得删除历史账本。

## 验收口径

### 数据模型

- 同一资金池可以绑定 OpenAI 和 Anthropic 两个平台账号。
- 一笔充值只记录在资金池，不需要复制到绑定账号。
- 账号删除或归档后，资金池账本仍然可读。
- 资金池合并不会自动改写已固化的历史 usage 成本。

### 成本计算

- `1000 CNY -> 200 USD` 生成 `5 CNY/USD` 基础成本。
- 同一资金池下，上游分组倍率不同的账号会得到不同最终折扣。
- 模型族倍率覆盖只影响对应模型族的账号最终成本，不改变资金池基础成本。
- 手动快照、最新快照、加权快照能清楚展示来源。

### 页面

- `/admin/accounts` 编辑账号时能选择已有上游供应商，也能先新增供应商到列表再选择。
- 新增供应商到列表会自动创建默认资金池；账号只有在表单最终选择该供应商并保存后才绑定到该默认资金池。
- 当供应商只有一个资金池时，账号编辑页不强迫管理员选择资金池；当供应商有多个资金池时，才要求选择钱包 / 资金池。
- `/admin/accounts` 成本对比能显示账号绑定的供应商，并在需要时显示资金池。
- 管理员能从账号跳转查看带有供应商和资金池标题的资金池账本。
- 管理员不能在账号上重复录入真实充值。
- 资金池列表能按供应商、状态、余额和绑定账号筛选。

### 历史报表

- 新充值不会改变已发生请求的上游成本。
- 资金池合并不会改变旧 usage 的成本快照。
- 后续利润报表能区分用户扣费、官方成本、上游真实成本和毛利。

## 风险和取舍

### 增加模型复杂度

供应商、资金池、绑定、快照比账号级字段复杂。但这是为了换取长期账本正确性。共享钱包场景无法用账号级字段可靠表达。

### 旧数据迁移需要谨慎

旧记录只有 `account_id`，无法自动判断哪些账号实际共享同一供应商钱包。第一阶段应保持等价迁移，不自动合并；合并必须由管理员显式确认。

### 余额接口归属变化

余额查询从账号迁到资金池后，需要处理“用哪个凭据查余额”的问题。推荐支持：

| 模式 | 说明 |
| --- | --- |
| `pool_secret` | 资金池独立 token |
| `bound_account_api_key` | 使用某个绑定账号的 API key 查询 |
| `custom_header` | 自定义 header |

如果不同账号查到不同余额，就说明它们不应该属于同一个资金池。

### 调度不能直接只看成本

资金池模型只提供可靠成本输入。是否按成本调度，还必须依赖模型级健康、失败率、延迟、并发和调度解释。默认策略仍应保持保守。

## 推荐落地顺序

1. 新增资金池和绑定模型，做等价迁移。
2. 把账号级充值记录迁到资金池账本。
3. 新增资金池管理页和账号绑定 UI。
4. 写入请求时成本快照。
5. 增加利润 / 毛利报表。
6. 再把资金池成本输入成本感知调度。

这条顺序的核心是先保证账本正确，再做展示和统计，最后才让成本参与调度。
