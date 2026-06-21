# 用量账本与已删除 Key 证据完整性

> 状态：部分落地。阶段 1 的管理员证据视图已经实现；阶段 2、阶段 3 仍是方案。

## 已落地情况

- 阶段 1 已落地（对应提交 `7bd82caf`）；阶段 2、阶段 3 仍是设计草案，待审查。
- 讨论的是 API Key 删除后，管理员和用户侧用量明细、统计分析、导出和对账证据是否还能说得清。
- 相关页面：
  - 管理员使用记录：`/admin/usage`
  - 用户侧 API Key 管理：`/keys`
  - 用户侧用量分析：`/usage`
- 不调整：扣费算法、网关转发、限流、余额变动、订阅用量更新或历史清理策略。

这页的核心判断是：`usage_logs` 应被视为不可变消费账本，`api_keys`、`users`、`groups`、`accounts` 等表只是可变维度。消费证据不能因为维度对象后续改名、禁用或删除，就变得说不清。

阶段 1 已按这份设计落地：`/admin/usage` 管理员证据视图会穿透软删除，解析已删除 Key 的名称和删除状态；DTO 隐藏明文 key，只向管理员证据上下文暴露删除元数据；导出会补充 Key ID、名称和删除时间。用户侧 `/usage` 和普通 `/keys` 列表仍只解析活跃 Key。阶段 2（快照字段）和阶段 3（外键约束）仍是设计草案，实施前需要按各阶段的前置核验执行。

## 当前情况

### API Key 删除是软删除

API Key 表有 `deleted_at` 字段。当前删除逻辑不会物理删除 `api_keys` 行，而是：

1. 写入 `deleted_api_key_audits`，保存原始 key、`api_key_id`、`user_id`、`key_name` 和删除时间。
2. 把 `api_keys.key` 改成 tombstone，释放唯一键。
3. 设置 `api_keys.deleted_at`。

因此正常业务删除不会触发数据库外键的 `ON DELETE CASCADE`，历史 `usage_logs` 不会被一起删除。

### usage_logs 仍保留 api_key_id

`usage_logs` 持有 `api_key_id`、`user_id`、`account_id`、`group_id`、模型、token、费用、端点、请求类型和时间等字段。管理员 `/admin/usage` 列表、趋势和统计大部分直接从 `usage_logs` 按这些字段过滤和聚合。

这意味着：即使 Key 已软删除，只要知道 `api_key_id`，管理员仍能按 `usage_logs.api_key_id` 查到历史明细和统计。

### usage_logs 外键删除策略存在来源差异

`backend/migrations/001_init.sql` 中 `usage_logs.user_id`、`usage_logs.api_key_id`、`usage_logs.account_id` 最初都声明为 `ON DELETE CASCADE`。这说明如果某个环境仍沿用该约束，物理删除用户、Key 或账号会级联删除消费记录。

但当前生成的 Ent schema 中，`usage_logs` 指向 `api_keys`、`accounts`、`users` 的外键是 `NO ACTION`，指向 `groups` 和 `user_subscriptions` 的外键是 `SET NULL`。因此阶段 3 不能只引用迁移文件或生成代码中的任一单一来源，必须先核实实际数据库约束。

阶段 3 的前置核验应查询 `pg_constraint.confdeltype`，确认生产、测试和本地库的真实 `ON DELETE` 行为。如果实际库已经是 `NO ACTION` / `RESTRICT`，阶段 3 的重点应从“改约束”调整为“把该约束写进测试和迁移防回归”；如果实际库仍是 `CASCADE`，才执行约束迁移。

### 普通 API Key 查询默认排除已删除 Key

API Key repository 的 `activeQuery()` 默认加 `deleted_at IS NULL`。因此已删除 Key 会从以下场景消失：

- 用户侧 Key 列表。
- 管理员用量页的 Key 搜索下拉。
- 用户 / Key 画像对象选择器。
- owner 侧 API Key analytics 的 Key 排行、模型、分组、标签和趋势。

这对“当前可管理 Key”是正确的，但对“历史消费证据查询”不完整。

### 已删除 Key 的名称关联会降级

管理员使用记录会 hydrate 用户、Key、账号、分组等关联对象。用户关联当前会穿透软删除，用于显示已删除用户；API Key 关联没有穿透软删除。

结果是：已删除 Key 的历史 usage log 数值仍在，但 `api_key` 对象可能为空。前端目前用 `#api_key_id` 兜底，导出里 Key 名称也可能为空。

这不是数值错误，但会损害对账和争议处理中的可解释性。

### 用户总用量和 Key 维度分析存在口径差异

用户总用量通常从 `usage_logs WHERE user_id = ?` 聚合，因此包含已删除 Key 的历史消耗。

但 owner API Key analytics 会 join `api_keys` 并要求 `api_keys.deleted_at IS NULL`，因此已删除 Key 不再出现在 Key 排行、模型分布、分组分布、标签分布和趋势中。

这会产生一个可解释但需要明确标注的差异：

- 用户总消费：历史真实消费，包含已删除 Key。
- 当前 Key 画像：当前仍存在的 Key，不包含已删除 Key。

## 问题定义

如果一个用户删除了某把已有使用记录的 Key，目前系统不会丢失原始消费数值，但有三类证据问题：

1. **名称证据不稳定**
   - 历史明细可能只能显示 `#9`，而不是原 Key 名称。
   - 导出可能缺少 Key 名称。
   - 管理员难以把一笔消费和用户认识的 Key 名称对应起来。

2. **查询入口不完整**
   - 已删除 Key 不再出现在 Key 搜索和对象选择器中。
   - 管理员如果不知道 `api_key_id`，很难主动筛出这把 Key 的历史消费。

3. **统计口径容易误读**
   - 用户总消费包含已删除 Key。
   - Key 维度分析默认排除已删除 Key。
   - 如果 UI 不说明，用户或管理员会误以为“总额对不上”。

## 产品原则

### 1. usage_logs 是账本，不是普通日志

用量记录直接关系到用户余额、扣费、对账和争议处理。它应满足：

- 不因 Key、用户、账号、分组删除而消失。
- 不因 Key、用户、账号、分组改名而改写历史语义。
- 能在导出、客服排查、用户申诉中解释“谁、用哪把 Key、在什么时候、调用了什么、产生了多少费用”。

### 2. 当前管理视图和历史证据视图必须分开

普通 Key 管理页应该只展示当前可管理 Key，不应把已删除 Key 混进日常操作。

用量明细、审计、导出和对账页面应该可以包含已删除 Key，并明确标识其状态。

### 3. ID 是证据链，快照是证据解释

`api_key_id`、`user_id`、`account_id`、`group_id` 是稳定追溯链路，但它们本身不能解释历史记录。

历史消费展示还需要写入时的快照信息，例如 Key 名称、用户邮箱、分组名称、账号名称。否则对象后续删除或改名后，历史账本会失去可读性。

## 推荐方案总览

建议分三阶段处理。

| 阶段 | 目标 | 改动类型 | 风险 |
| --- | --- | --- | --- |
| 阶段 1 | 修复已删除 Key 的管理员证据展示 | 后端 hydrate + 前端展示 + 导出字段 | 低 |
| 阶段 2 | 为 usage_logs 增加历史快照字段 | migration + 写入路径 + 回填 | 中 |
| 阶段 3 | 收紧账本完整性约束，避免物理删除级联清除消费证据 | migration + 清理任务审计 | 中高 |

## 阶段 1：管理员证据展示补全

### 目标

管理员在 `/admin/usage` 查看历史 usage log 时，即使关联 Key 已删除，也应看到：

- 原 Key 名称。
- API Key ID。
- 已删除状态。
- 删除时间，如果可得。
- 用户、账号、分组等其它维度照常展示。

### 后端设计

新增一个只在管理员用量 / 审计场景使用的 Key 解析能力，不改变普通 API Key 查询默认排除已删除记录的行为。

推荐方向：

1. `usage_log_repo.hydrateUsageLogAssociations` 对 API Key 关联支持穿透软删除。
2. 对软删除 Key，返回 DTO 时带 `deleted: true` 和 `deleted_at`。
3. 如果 `api_keys` 行仍在，Key 名称直接来自 `api_keys.name`。
4. 如果未来存在物理删除或历史残缺，再从 `deleted_api_key_audits` 按 `api_key_id` 补 `key_name`。

示例响应：

```json
{
  "api_key_id": 9,
  "api_key": {
    "id": 9,
    "name": "ClaudeCode",
    "deleted": true,
    "deleted_at": "2026-06-16T08:30:00Z"
  }
}
```

注意：

- 不返回完整明文 key。
- 不让普通 Key 列表、授权校验、用户侧 Key 管理复用这个穿透软删除查询。
- 管理员 DTO 可以比用户 DTO 多 `deleted`、`deleted_at`、`historical` 等字段。

### 前端设计

`/admin/usage` 表格、画像标题、对象选择器和导出都要明确展示已删除状态。

建议 UI：

- 表格 Key 列：`ClaudeCode` + `已删除` badge。
- hover/title：`该 API Key 已删除，当前展示的是历史用量。`
- 画像顶部：`htzh@... / ClaudeCode（已删除）`。
- 导出列：
  - `API Key ID`
  - `API Key 名称`
  - `API Key 状态`
  - `API Key 删除时间`

### Key 搜索和筛选

管理员用量页的对象选择器可以增加一个证据查询专用选项：

- 默认：只搜索当前 Key。
- 开关：`包含已删除 Key`。
- 已删除 Key 搜索结果带 badge，不允许进入编辑，只允许进入用量分析。

前端落点是 `frontend/src/components/admin/usage/UsageObjectFilterPicker.vue`。该组件当前负责用户 / Key 双栏对象选择，后续可以在右侧 API Key 列表区增加“包含已删除 Key”开关，并把该开关传给管理员历史 Key 搜索接口。普通 `UsageFilters` 和用户侧 Key 管理页不应复用这个开关。

这样可以同时保持：

- `/keys` 页面不受污染。
- `/admin/usage` 能完整查证据。

### 阶段 1 验收

必须覆盖以下用例：

1. 创建 Key，写入 usage log，删除 Key。
2. `/admin/usage?api_key_id=<id>` 仍返回历史明细。
3. 明细中的 `api_key.name` 仍为原 Key 名称。
4. 明细中的 `api_key.deleted === true`。
5. 导出包含 Key ID、Key 名称和已删除状态。
6. 管理员对象选择器默认不显示已删除 Key。
7. 打开“包含已删除 Key”后能搜到该 Key。
8. 用户侧普通 Key 列表仍不显示已删除 Key。
9. 用户侧 `/usage` 明细第一阶段不穿透已删除 Key 解析名称或删除时间；只有管理员证据上下文会 hydrate 已删除 Key。

## 阶段 2：usage_logs 历史快照

### 目标

从根上避免历史消费记录依赖维度表当前状态。

新增快照字段后，即使 Key 改名、用户改邮箱、分组改名、账号改名，历史 usage log 仍可解释当时的消费上下文。

### 建议新增字段

阶段 2 实施前必须先做 `usage_logs` 现有列审计，避免重复保存已存在的费用和倍率事实。审计输出至少要列出：

- 当前所有费用字段，例如 `input_cost`、`output_cost`、`cache_*_cost`、`total_cost`、`actual_cost`、`account_stats_cost`。
- 当前所有倍率字段，例如 `rate_multiplier`、`account_rate_multiplier`。
- 当前模型和路由字段，例如 `model`、`requested_model`、`upstream_model`、`channel_id`、`model_mapping_chain`、`billing_tier`、`billing_mode`。
- 每个拟新增 snapshot 字段是否已有等价列、是否是写入时事实、是否会被后续维度改名污染。

只有完成逐列判定后，才能确认最终 migration 列表。原则是：新增“历史解释字段”，不重复保存已经不可变且语义清楚的“计费事实字段”。

第一批只加对账必需字段：

```sql
ALTER TABLE usage_logs
  ADD COLUMN IF NOT EXISTS api_key_name_snapshot VARCHAR(100),
  ADD COLUMN IF NOT EXISTS user_email_snapshot VARCHAR(255),
  ADD COLUMN IF NOT EXISTS group_name_snapshot VARCHAR(100),
  ADD COLUMN IF NOT EXISTS account_name_snapshot VARCHAR(100),
  ADD COLUMN IF NOT EXISTS account_platform_snapshot VARCHAR(50);
```

可选字段：

```sql
ALTER TABLE usage_logs
  ADD COLUMN IF NOT EXISTS api_key_status_snapshot VARCHAR(20),
  ADD COLUMN IF NOT EXISTS subscription_name_snapshot VARCHAR(100),
  ADD COLUMN IF NOT EXISTS account_rate_multiplier_snapshot NUMERIC,
  ADD COLUMN IF NOT EXISTS user_rate_multiplier_snapshot NUMERIC;
```

是否保存倍率快照要谨慎。倍率与费用口径相关，如果当前 `usage_logs` 已保存最终费用和倍率字段，新增快照应避免重复或造成歧义。

### 写入路径

usage log 写入时，从当次请求上下文直接填充快照：

- `api_key_name_snapshot`：认证出的 Key 名称。
- `user_email_snapshot`：认证出的用户邮箱。
- `group_name_snapshot`：最终归因分组名称。
- `account_name_snapshot`：实际使用的上游账号名称。
- `account_platform_snapshot`：实际使用的上游平台。

快照字段必须和 `usage_logs` 同事务或同一次持久化逻辑写入，不能由异步任务事后补齐，否则会留下空窗。

### 历史回填

历史数据回填按可信度分层：

1. `api_keys` 仍存在：从 `api_keys.name` 回填 Key 名称。
2. `api_keys` 已软删除：仍可从 `api_keys.name` 回填，并标记删除状态来自当前行。
3. `deleted_api_key_audits` 有记录：用 `key_name` 补充缺失名称。
4. `users`、`groups`、`accounts` 仍存在：从当前维度表回填。
5. 无法回填：保留空值，不伪造历史名称。

回填必须记录限制：

- 回填的是“当前可推断名称”，不一定是请求发生时的名称。
- 真正的写入时快照只能对上线后的新记录完全准确。

### 展示优先级

用量明细展示名称时按以下优先级：

1. `usage_logs.*_snapshot`
2. 当前关联对象名称
3. 删除审计表名称
4. `#id`

这样新数据优先使用不可变账本快照，旧数据继续尽量补全。

### 阶段 2 验收

必须覆盖以下用例：

1. Key 写入 usage log 后改名，历史明细仍显示写入时名称。
2. 用户改邮箱后，历史明细仍显示写入时邮箱。
3. 分组改名后，历史分组名称仍可按快照展示。
4. 上游账号改名后，历史账号名称仍可按快照展示。
5. 删除 Key 后，历史明细仍显示快照 Key 名称和删除状态。
6. 导出使用快照名称，不因当前维度对象改名而变化。
7. 阶段 2 前置列审计完成，并明确说明每个 snapshot 字段与现有 `usage_logs` 列不是重复计费事实。

## 阶段 3：账本完整性约束

### 目标

防止未来维护脚本、迁移或硬删除路径误删账本记录。

`001_init.sql` 中 `usage_logs.api_key_id`、`usage_logs.user_id`、`usage_logs.account_id` 最初带有 `ON DELETE CASCADE`；当前 Ent 生成 schema 则把 user / key / account 的删除动作声明为 `NO ACTION`。这两个来源不一致，所以阶段 3 的第一步不是直接迁移，而是确认实际数据库约束。

建议用以下查询核实每个环境：

```sql
SELECT
  tbl.relname AS table_name,
  attr.attname AS column_name,
  ref_tbl.relname AS ref_table,
  CASE c.confdeltype
    WHEN 'a' THEN 'NO ACTION'
    WHEN 'r' THEN 'RESTRICT'
    WHEN 'c' THEN 'CASCADE'
    WHEN 'n' THEN 'SET NULL'
    WHEN 'd' THEN 'SET DEFAULT'
  END AS on_delete
FROM pg_constraint c
JOIN pg_class tbl ON tbl.oid = c.conrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
JOIN pg_class ref_tbl ON ref_tbl.oid = c.confrelid
JOIN pg_attribute attr ON attr.attrelid = tbl.oid AND attr.attnum = ANY(c.conkey)
WHERE ns.nspname = 'public'
  AND c.contype = 'f'
  AND tbl.relname = 'usage_logs'
ORDER BY attr.attname;
```

如果实际库仍是 `CASCADE`，物理删除维度对象会删除消费记录，这对账本不安全。即使实际库已经是 `NO ACTION` / `RESTRICT`，也应把该行为写入 schema 集成测试，防止未来迁移回退。

### 推荐方向

长期建议把账本表对维度表的删除策略固定为：

- `usage_logs.api_key_id -> api_keys.id`：`ON DELETE RESTRICT` 或 `NO ACTION`
- `usage_logs.user_id -> users.id`：`ON DELETE RESTRICT` 或 `NO ACTION`
- `usage_logs.account_id -> accounts.id`：`ON DELETE RESTRICT` 或 `NO ACTION`

如果必须允许维度对象清理，则应保留 tombstone 行，而不是级联删除 usage log。

### 迁移注意

这类迁移风险较高，需要先确认：

- 实际库中 `usage_logs` 外键当前到底是 `CASCADE`、`NO ACTION`、`RESTRICT` 还是 `SET NULL`。
- 是否存在真实物理删除用户、Key、账号的运维任务。
- 是否有测试依赖 cascade 删除清理 usage log。
- 是否有外部脚本绕过 service 层直接删表。
- 历史数据是否存在孤儿 usage log。

建议先加测试和保护，再改约束。

### 阶段 3 验收

必须覆盖以下用例：

1. 数据库层物理删除被 usage log 引用的 Key 时失败或被阻止。
2. 业务删除 Key 仍是软删除，不影响历史 usage log。
3. 清理任务不能删除带有 usage log 的维度行，除非先明确归档账本。
4. 迁移前后 `/admin/usage` 统计数值一致。
5. schema 集成测试断言 `usage_logs.user_id`、`usage_logs.api_key_id`、`usage_logs.account_id` 不允许 `CASCADE`。

## 口径规范

### 管理员 `/admin/usage`

默认语义：历史真实消费。

- 包含已删除 Key 的历史用量。
- 支持按已删除 Key 筛选。
- 明确标识删除状态。
- 导出必须保留历史名称和 ID。

### 用户侧 `/keys`

默认语义：当前可管理 Key。

- 不显示已删除 Key。
- 不允许恢复或编辑已删除 Key。
- 已删除 Key 的历史消费通过用量页面或账单导出解释。

### 用户侧 `/usage` 分析

MVP 立场：第一阶段不在用户侧开放已删除 Key 选择器，先只在管理员证据查询中支持已删除 Key。用户侧 `/usage` 继续以当前可见 Key 为主，明细列表也不穿透已删除 Key 去恢复名称或删除时间；但必须解释总用量和 Key 维度分析的口径差异。

后续如果开放用户侧历史 Key 查询，再区分两种口径：

- 当前 Key 分析：只包含 `deleted_at IS NULL` 的 Key。
- 历史消费分析：包含已删除 Key。

第一版必须在 UI 文案中明确“Key 维度分析仅包含当前 Key；历史总消费可能包含已删除 Key”。这样用户不会把两种口径的差异误认为金额错误。

### 导出和对账

默认语义：历史证据。

必须包含：

- `usage_log_id`
- `request_id`
- `user_id`
- `user_email_snapshot`
- `api_key_id`
- `api_key_name_snapshot`
- `api_key_deleted`
- `model`
- `requested_model`
- `input_tokens`
- `output_tokens`
- `cache_creation_tokens`
- `cache_read_tokens`
- `total_cost`
- `actual_cost`
- `account_stats_cost`
- `created_at`

导出字段的命名要避免把不同费用口径混在一起：

- `actual_cost`：用户实际扣费。
- `total_cost`：标准计费。
- `account_stats_cost` 或倍率后账号成本：平台运营成本。

## 安全和权限边界

1. 普通用户不能通过“包含已删除 Key”枚举其它用户的 Key。
2. 用户侧如果允许查看已删除 Key 的历史用量，必须校验该 Key 曾属于当前用户。
3. 管理员导出可以包含运营成本字段，用户侧导出不能包含 `account_cost`、上游账号密钥、渠道内部成本等管理员字段。
4. `deleted_api_key_audits.key` 是敏感字段，不应进入用量明细 DTO 或导出。
5. 搜索已删除 Key 时只返回 `id`、`name`、`user_id`、`deleted`、`deleted_at`，不返回原始 key。

## 测试计划

### 后端

- `api_key_repo`：
  - 删除 Key 后 `api_keys` 行仍存在，`deleted_at` 不为空。
  - `deleted_api_key_audits` 保存原 Key 名称。
- `usage_log_repo`：
  - 删除 Key 后按 `api_key_id` 查询 usage logs 仍返回记录。
  - 管理员 hydrate 已删除 Key 时能返回名称和删除状态。
  - 默认 / 用户侧 hydrate 不解析已删除 Key，避免阶段 1 顺带改变用户侧展示语义。
  - 普通 active Key 搜索仍排除已删除 Key。
  - 管理员历史 Key 搜索在显式包含 deleted 时返回已删除 Key。
- owner analytics：
  - 默认继续排除已删除 Key。
  - 无论是否新增“包含历史 Key”开关，都必须新增口径回归测试：构造同一用户下 1 把活跃 Key 和 1 把已删除 Key 且两者都有消费，断言用户总额包含两者之和，当前 Key 画像只包含活跃 Key。
  - 如果未来新增包含历史 Key 开关，再额外断言两种口径的数值差异和 UI 标注。
- 导出：
  - 删除 Key 后导出仍包含 Key ID、Key 名称和删除状态。

### 前端

- `/admin/usage` 表格：
  - 已删除 Key 显示原名和 `已删除` badge。
  - 点击已删除 Key 仍能进入 Key 历史用量分析。
- 对象选择器：
  - 默认不显示已删除 Key。
  - 打开“包含已删除 Key”后可以搜索并选择。
  - 已删除 Key 不能跳到编辑页。
- 导出：
  - 已删除 Key 的名称不为空。
  - 导出列包含删除状态。
- 用户侧：
  - `/keys` 不显示已删除 Key。
  - `/usage` 第一阶段不提供已删除 Key 选择器。
  - `/usage` 明细第一阶段不恢复已删除 Key 名称或 `deleted_at`；用户侧只通过口径说明解释总额和当前 Key 分析差异。
  - `/usage` 的口径说明存在且不遮挡主流程。

### 数据迁移

- 新增 snapshot 字段后，旧数据回填可重复运行。
- 回填脚本不覆盖已有非空 snapshot。
- 回填失败时不会中断业务写入。
- 迁移前后总请求数、总 token、总费用一致。

## 开放问题

1. 用户侧是否要允许查看“已删除 Key 的历史用量”？
   - 如果允许，需要一个只读历史 Key 选择器。
   - 如果不允许，用户侧必须在总用量和 Key 分析之间加口径说明。

2. `deleted_api_key_audits` 是否应该只服务认证失败反查，还是可以作为 usage 历史名称兜底？
   - 如果复用，必须确保不泄露原始 key。

3. 快照字段是否应该保存用户邮箱？
   - 保存邮箱利于对账，但涉及个人信息留存。
   - 倾向：可以保存 `user_email_snapshot` 作为对账解释字段，但隐私删除流程不应删除 usage log 本体；如需响应个人信息擦除，应把邮箱快照替换为 tombstone 值，例如 `deleted-user-<user_id>` 或不可逆摘要，同时保留 `user_id`、费用、token、时间等非展示账本事实。
   - 这不是法律结论；实现前应把该策略写入数据保留和隐私处理规则，避免“不可变账本”和“个人信息擦除”互相冲突。

4. 是否需要专门的账本归档表？
   - 当前目标是增强 `usage_logs`。
   - 如果未来数据保留策略更复杂，可考虑 `usage_ledger_entries` 作为更严格的不可变账本。

5. 物理删除维度对象的运维需求是否真实存在？
   - 如果没有，优先改为 RESTRICT。
   - 如果有，需要先设计 tombstone 和归档策略。

## 推荐实施顺序

1. 阶段 1：先补管理员证据展示和导出。
2. 阶段 1 后让实际管理员人工验证删除 Key 后的明细、统计、画像和导出。
3. 阶段 2：新增 snapshot 字段并接入新写入路径。
4. 阶段 2 后再做历史回填，明确回填可信度。
5. 阶段 3：最后处理外键约束和物理删除防护。

不建议第一步就改外键或大规模迁移。当前最影响对账的是“查得到数值但解释不完整”，应先让管理员证据视图完整可靠。

## 成功标准

这个优化完成后，应满足：

- 删除 Key 不影响历史 usage log 数值。
- 删除 Key 后，管理员仍能通过名称、ID、用户和时间范围查到历史消费。
- 历史明细和导出能明确显示 Key 已删除，但保留原名称。
- Key 改名、用户改邮箱、分组改名、账号改名不影响新写入日志的历史解释。
- 普通 Key 管理视图和历史证据查询视图口径分离，不互相污染。
- 账本数据不会因为维度对象物理删除而被级联删除。
