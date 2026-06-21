# 删除 Key 后用量显示归零排查

> 状态：待核验。当前还不能定性为数据丢失，也不能简单归因于“老 Key”。这页记录 2026-06-22 本地测试中发现的疑似问题、已确认的代码口径和下一步排查顺序。

## 现象

本地恢复生产 dump 后，在普通用户视角测试 `/usage`：

1. 删除某个普通用户已有的几把 Key 后，页面看起来“使用记录变干净了”，顶部统计和明细疑似都归零。
2. 随后重新测试：新建一把 Key，调用几次，再删除该 Key。删除后 `/usage` 的顶部统计、明细和使用记录仍能看到刚产生的用量。

这两个现象不能直接合并成一个结论。第二次测试说明当前删除流程至少在“新建 Key -> 产生 usage_logs -> 删除 Key”的路径上没有把用量记录删掉。

## 先不要下的结论

### 不是“Key 创建得早”本身导致

Key 的创建时间不是关键条件。用量页统计看的是 `usage_logs.created_at`，不是 `api_keys.created_at`。

如果最早那几把 Key 是很久以前创建、也很久没有使用，那么 `/usage` 默认“近 7 天”筛选下显示 0 是正常结果。第二次新建 Key 的调用发生在当前时间范围内，所以能显示。

因此要比较两次测试，应该先比较这些字段：

- `usage_logs.created_at` 是否落在页面筛选区间内。
- `usage_logs.user_id` 是否等于当前登录用户。
- `usage_logs.api_key_id` 是否指向被删除的 Key。
- `api_keys.deleted_at` 是否只是软删除。

### 不是“管理员有效、用户必然无效”

当前确实存在管理员侧和用户侧的口径差异，但差异主要在“已删除 Key 的名称解析、选择器和 Key 维度分析”，不是所有用户总用量。

用户侧 `/usage/stats` 在没有指定 `api_key_id` 时，后端走的是 `usage_logs WHERE user_id = ? AND created_at BETWEEN ?` 这类聚合。按代码看，单纯删除 Key 不应该让用户总请求数、总 Token、总消费归零。

### 不排除旧删除路径留下历史缺口

迁移 `145_deleted_api_key_audit.sql` 明确说明：`deleted_api_key_audits` 只对该表上线后删除的 Key 生效；上线前已经删除的 Key，原始 key 可能已经被 tombstone 覆盖，无法补录。

这会影响“已删除 Key 的原始 key 反查”和部分证据解释能力，但它本身不等于 `usage_logs` 被删除。是否丢了用量，仍要查 `usage_logs`。

## 已确认的代码口径

### 当前删除流程是软删除

普通用户删除 API Key 时，`APIKeyService.Delete` 会先校验所有者，然后调用 `DeleteWithAudit`：

- 写入 `deleted_api_key_audits`。
- 把 `api_keys.key` 改成 tombstone。
- 设置 `api_keys.deleted_at`。
- 不直接删除 `usage_logs`。

这条路径下，删除 Key 不应物理清除历史用量。

### 管理员使用记录会穿透软删除

管理员 `/admin/usage` 列表会显式开启 `WithUsageLogDeletedAPIKeyResolution`，因此能解析已删除 Key 的名称和删除状态。

这是阶段 1 已落地的管理员证据视图。

### 用户明细不会穿透已删除 Key

用户 `/usage` 明细没有开启已删除 Key 解析。结果是：历史记录本身仍可返回，但 `api_key` 关联对象可能为空，前端只能显示 `#api_key_id` 或空名称。

这属于展示证据不完整，不等于数值丢失。

### 用户 API Key 分析默认排除已删除 Key

用户侧 owner API Key analytics 会 join `api_keys`，并强制 `ak.deleted_at IS NULL`。因此删除所有 Key 后，下面这些分析天然会变成 0：

- Key 排行。
- 消耗趋势。
- 模型分布。
- 分组分析。
- 标签归因。
- 当前活跃 Key、接近额度、接近速率限制等快照。

这是已确认的产品口径缺口：`/usage` 是历史账本页面，但其中一部分分析仍按“当前未删除 Key”来算。

## 最可能原因排序

### 1. 时间范围不一致

这是当前最优先排查项。

截图里页面处于“近 7 天”。如果最早删除的那几把 Key 只有更早以前的用量，页面显示 0 是正常的。第二次新建 Key 刚使用过，自然能显示。

判断方式：

```sql
SELECT
  user_id,
  api_key_id,
  COUNT(*) AS requests,
  SUM(actual_cost) AS actual_cost,
  MIN(created_at) AS first_seen,
  MAX(created_at) AS last_seen
FROM usage_logs
WHERE user_id = <当前登录用户ID>
GROUP BY user_id, api_key_id
ORDER BY last_seen DESC;
```

如果 `last_seen` 不在页面筛选时间范围内，前端显示 0 不是 bug。

### 2. 登录用户和 dump 里的消费用户不是同一个

本地从生产 dump 恢复后，如果当前登录的普通用户不是那几把旧 Key 的 `user_id`，用户侧 `/usage` 会正确显示 0。管理员侧如果按其他用户看，可能仍能看到。

判断方式：

```sql
SELECT
  id,
  email,
  username,
  deleted_at
FROM users
WHERE id IN (
  SELECT DISTINCT user_id
  FROM usage_logs
)
ORDER BY id DESC;
```

再对照当前登录用户的 ID。

### 3. 旧 Key 在审计迁移前已删除

如果 Key 是在 `deleted_api_key_audits` 上线前就已经删除，系统可能缺少 deleted audit 证据，原始 key 也可能已经 tombstone。

影响：

- 无法通过 deleted audit 反查原始 key。
- 已删除 Key 的名称、删除时间、历史解释可能不完整。

不应直接影响：

- `usage_logs.user_id` 聚合。
- `usage_logs.api_key_id` 明细查询。

所以它可以解释“名称证据缺失”，不能单独解释“顶部统计归零”。

### 4. 实际数据库外键仍是级联删除

文档中已经记录过来源差异：早期 `001_init.sql` 中 `usage_logs.user_id`、`usage_logs.api_key_id`、`usage_logs.account_id` 曾声明为 `ON DELETE CASCADE`，而当前 Ent schema 期望不是这个行为。

如果某个环境实际仍保留级联约束，并且发生过物理删除，就可能真的删掉 `usage_logs`。

判断方式：

```sql
SELECT
  conname,
  confdeltype
FROM pg_constraint
WHERE conrelid = 'usage_logs'::regclass
  AND contype = 'f';
```

`confdeltype` 常见含义：

- `c`：CASCADE，删除父表会级联删除子表。
- `r`：RESTRICT。
- `a`：NO ACTION。
- `n`：SET NULL。

如果 `api_key_id` 或 `user_id` 对应约束仍是 `c`，需要按外键约束问题处理。

## 建议的核验步骤

### 第一步：确认 usage_logs 是否还在

```sql
SELECT
  user_id,
  api_key_id,
  COUNT(*) AS requests,
  SUM(actual_cost) AS actual_cost,
  MIN(created_at) AS first_seen,
  MAX(created_at) AS last_seen
FROM usage_logs
WHERE api_key_id IN (<那几把旧 Key 的 ID>)
GROUP BY user_id, api_key_id
ORDER BY api_key_id;
```

判断：

- 有数据：记录没丢，继续查页面时间范围、登录用户和接口参数。
- 没数据：再查是否被清理任务、物理删除或外键级联影响。

### 第二步：确认 Key 删除状态

```sql
SELECT
  id,
  user_id,
  name,
  key,
  status,
  deleted_at,
  created_at,
  updated_at
FROM api_keys
WHERE id IN (<那几把旧 Key 的 ID>)
ORDER BY id;
```

判断：

- `deleted_at` 有值且行仍存在：软删除正常。
- 查不到行：可能发生过物理删除，需要重点查外键和删除路径。
- `key` 是 `__deleted__...`：说明原始 key 已 tombstone，不代表 usage_logs 消失。

### 第三步：确认页面请求参数

在浏览器 Network 里看：

- `/api/v1/usage/stats` 是否带了正确的 `start_date` / `end_date`。
- 是否误带了某个已删除的 `api_key_id`。
- `/api/v1/usage` 返回的是 0 条、报错，还是前端没有刷新。
- `/api/v1/usage/analytics/*` 归零是否只发生在 Key 分析区域。

### 第四步：确认外键删除行为

如果 `usage_logs` 真没了，再查 `pg_constraint.confdeltype`。只有确认实际约束和删除路径后，才能判断是不是历史环境遗留的级联删除。

## 修复候选

### 短期修复

1. 用户 `/usage` 明细支持解析当前用户自己的已删除 Key 元信息。
2. 用户 `/usage/stats?api_key_id=<deleted_key_id>` 和 `/usage?api_key_id=<deleted_key_id>` 改成允许查询本人已删除 Key 的历史用量。
3. 用户 API Key 分析增加“包含已删除 Key”的历史账本口径，默认用于 `/usage` 页面。
4. 前端把“当前活跃 Key”和“有历史用量的 Key”分开命名，避免用户把 0 个活跃 Key 理解为 0 用量。

### 中期修复

1. 为 `usage_logs` 增加 Key 名称、用户邮箱、分组、账号等写入时快照字段。
2. 为外键约束加迁移和测试，禁止 `usage_logs` 因用户、Key、账号删除而级联消失。
3. 为用户侧和管理员侧分别补充已删除 Key 的可见性测试，确保普通用户只能看到自己的历史 Key 元信息。

## 当前判断

这次现象更像是“历史账本页面混用了当前 Key 口径”加上“首次测试样本需要核验时间范围和用户归属”，不能直接定性为数据被删。

如果旧 Key 的 `usage_logs` 在数据库里仍存在，只是页面某些区域不显示，那么应修 `/usage` 的查询和展示口径。

如果旧 Key 的 `usage_logs` 已经不存在，才需要按数据完整性问题追查实际外键、清理任务或历史物理删除路径。
