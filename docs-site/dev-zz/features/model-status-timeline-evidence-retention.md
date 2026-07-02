# 模型状态时间线与无可用账号证据保留 — 方案文档

> 状态：分阶段落地中。设计 2026-07-02，阶段 1 已落地；列表状态切快照和时间桶化展示仍是后续阶段。
>
> 本文是《定价驱动的站点自检模型监控》的后续补强方案。现有用户侧 `/monitor` 已按 (分组, 模型) 展示站点自检状态，但「近 60 次记录」仍直接读取账号级自检明细，无法表达「某个分组/模型在一段时间内没有任何可用账号」这种状态窗口。本文定义修正口径、数据模型和验收标准。

## 背景

用户侧模型状态页当前展示：

- 当前状态：按 (分组, 模型) 聚合出来的健康状态。
- 近 60 次记录：详情弹窗中的时间线条。

当前实现存在一个误导性场景：

1. 某个模型在某个分组下所有账号进入冷却、禁用或不可调度状态。
2. 当前状态会被聚合为失败或不可用。
3. 由于没有可自检账号，自检 runner 不再生成该模型的探针任务。
4. 失败窗口内没有新的 `model_self_check_histories` 行。
5. 账号恢复后重新开始探针，新的记录又是正常。
6. 「近 60 次记录」会把失败前后的正常记录接起来，看起来像中间从未失败。

这不是单纯前端渲染问题，而是当前数据模型没有持久化 (分组, 模型) 维度的状态快照。

## 当前代码事实

### 1. 时间线是「最近 60 条结果」，不是「过去 60 个时间桶」

前端 `MonitorTimeline` 只拿 `detail.timeline` 中最近若干个点渲染，并补足到固定长度。它不会按真实时间间隔生成缺失桶。

相关位置：

- `frontend/src/views/user/ChannelStatusView.vue`
- `frontend/src/components/user/monitor/MonitorTimeline.vue`
- `frontend/src/api/modelStatus.ts`

因此，如果失败期间没有写入历史记录，时间线上不会出现这段失败时间。

### 2. 自检任务依赖当前可用账号集合

`ModelSelfCheckService.ListProbeTasks` 遍历 `accountIDsForTarget(...)` 生成探针任务。当前可用账号集合为空时，不会生成任务，也不会写探针历史。

账号来源 `ListTargetAccounts` 当前只返回：

- `accounts.status = 'active'`
- `accounts.schedulable = TRUE`
- `accounts.deleted_at IS NULL`
- `temp_unschedulable_until IS NULL OR temp_unschedulable_until <= NOW()`

这意味着账号级冷却、手动关闭调度、停用、归档等状态都会让账号退出自检候选池。

### 3. 当前状态可以失败，但失败不一定有历史点

`aggregateSelfCheckStatus` 在 `expectedAccounts == 0` 时会返回失败状态。这个失败是请求时即时聚合出来的，不会自动写入 `model_self_check_histories`。

所以会出现：

```text
当前状态：failed / unavailable
历史时间线：空，或只显示失败前后的正常记录
```

### 4. 现有历史表按账号存储，不能表达「无账号」

当前 `model_self_check_histories` 字段包括：

- `model`
- `account_id`
- `platform`
- `status`
- `latency_ms`
- `http_status`
- `error_code`
- `checked_at`

其中 `account_id` 是 `NOT NULL` 外键，且表中没有 `group_id`。这张表适合记录「某账号探测某模型的结果」，但不适合记录「某分组下某模型没有任何可用账号」。

## 问题定义

当前实现把两个概念耦合在一起：

| 概念 | 当前实现 | 问题 |
| --- | --- | --- |
| 当前可服务账号集合 | 决定当前状态 | 合理 |
| 历史时间线查询范围 | 也由当前可服务账号集合决定 | 不合理 |
| 无可用账号状态 | 只在请求时聚合出来 | 不落库，恢复后无法回看 |
| 用户侧时间线 | 账号级历史明细投影 | 缺少 (分组, 模型) 快照口径 |

根本问题：用户看到的是 (分组, 模型) 状态页，但底层时间线仍是账号级探针明细，且查询还依赖当前账号可用性。

## 目标

- 为用户侧 `/monitor` 提供稳定的 (分组, 模型) 历史状态时间线。
- 即使某个分组/模型没有可用账号，也要持久化一条可展示的状态证据。
- 模型失败两小时后恢复，时间线必须保留这两小时内的失败/不可用窗口，而不是恢复后看起来全绿。
- 用户侧继续不暴露账号 ID、渠道 ID、provider、endpoint、上游模型、原始错误、成本。
- 保留现有账号级 `model_self_check_histories`，作为管理员内部排障明细。
- 修正「近 60 次记录」的产品口径，避免用户误以为它是完整连续时间轴。

## 非目标

- 不在用户侧展示具体账号、渠道或上游错误。
- 不替代管理员侧渠道监控。
- 不改变 `usage_logs`、计费或余额逻辑。
- 不让自检请求触发生产调度副作用、封禁、failover 或用量计费。
- 不在本方案中设计公开免登录状态页。

## 术语

| 术语 | 含义 |
| --- | --- |
| 账号级探针明细 | `model_self_check_histories` 中按 `account_id` 写入的真实探测结果。 |
| 状态快照 | 按固定刷新节奏为 (分组, 模型) 写入的脱敏聚合状态。它可以使用最近探针结果采样，不要求所有账号探针在同一批次完成。 |
| 无可用账号 | 某 (分组, 模型) 当前没有任何账号可用于服务或自检。 |
| 时间线点 | 用户侧「近 60 次记录」中的一个可展示状态点。 |

## 方案概览

新增一层 (分组, 模型) 状态快照：

```text
model_self_check_histories
  账号级真实探针明细，保留内部排障用途
        │
        │ 快照刷新器采样最近探针结果，或无账号时生成脱敏状态
        ▼
model_self_check_status_snapshots
  用户侧 (group_id, model) 状态快照
        │
        ▼
/api/v1/model-status/detail
  timeline 改读快照，不再直接读当前可用账号下的账号级历史
```

核心原则：

- 当前状态和时间线都以 (分组, 模型) 为用户侧口径。
- 账号级探针失败可以聚合成快照。
- 没有账号可探时，也必须写快照。
- 快照只存脱敏聚合状态，不存账号、渠道、上游 endpoint 或原始错误。

## 数据模型

新增表：`model_self_check_status_snapshots`。

建议字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | BIGSERIAL PK | |
| `group_id` | BIGINT NOT NULL | 用户可见分组维度。建议外键 `groups(id)`；删除分组时可级联或保留需另行决定。 |
| `model` | VARCHAR(255) NOT NULL | 公开模型名。 |
| `status` | VARCHAR(20) NOT NULL | `operational` / `degraded` / `failed` / `unknown`。若后续要表达刷新器自身异常，应另行补充 `error` 产生规则，避免实现时出现不可解释状态。 |
| `reason_code` | VARCHAR(80) NOT NULL | 脱敏原因码，见下节。 |
| `eligible_account_count` | INT NOT NULL DEFAULT 0 | 本次快照采样时可服务/可自检账号数。 |
| `checked_account_count` | INT NOT NULL DEFAULT 0 | 本次快照采样到的新鲜探针结果账号数。 |
| `operational_account_count` | INT NOT NULL DEFAULT 0 | 成功账号数。 |
| `degraded_account_count` | INT NOT NULL DEFAULT 0 | 降级账号数。 |
| `failed_account_count` | INT NOT NULL DEFAULT 0 | 失败账号数。 |
| `latency_ms` | INT NULL | 用户侧展示延迟，可取本次采样窗口内最佳成功延迟或聚合延迟。 |
| `checked_at` | TIMESTAMPTZ NOT NULL DEFAULT NOW() | 快照时间。 |
| `created_at` | TIMESTAMPTZ NOT NULL DEFAULT NOW() | 写入时间。 |

建议索引：

```sql
CREATE INDEX idx_model_self_check_status_snapshots_group_model_checked
    ON model_self_check_status_snapshots (group_id, model, checked_at DESC);

CREATE INDEX idx_model_self_check_status_snapshots_checked
    ON model_self_check_status_snapshots (checked_at DESC);
```

不建议在用户侧快照表中存：

- `account_id`
- `channel_id`
- `provider`
- `endpoint`
- 上游模型名
- 原始错误消息
- 上游 request id
- 成本或 token

## reason_code 口径

建议先定义一组稳定的机器码：

| reason_code | status | 说明 |
| --- | --- | --- |
| `ok` | `operational` | 至少一个账号成功，且无失败/降级账号。 |
| `partial_degraded` | `degraded` | 至少一个账号成功，但存在失败/降级账号，或覆盖账号不完整。 |
| `all_probe_failed` | `failed` | 有账号参与探针，但没有任何账号成功。 |
| `all_degraded` | `degraded` | 本次采样窗口内全部新鲜账号结果都是降级；不在快照层推断具体降级原因。 |
| `no_available_account` | `failed` | 没有任何当前可服务/可自检账号。 |
| `no_fresh_probe` | `unknown` | 有候选账号，但新鲜窗口内没有探针结果。 |
| `self_check_disabled` | `unknown` | 自检全局开关关闭；通常不写入用户时间线，若写入需明确口径。 |

用户侧文案可以只映射到通用状态，不展示内部原因细节；管理员排障页或日志可展示 `reason_code`。

## 写入流程

### 当前流程

```text
ListProbeTasks
  -> 只为当前可用账号生成 task
RunProbe
  -> 写 model_self_check_histories
GetUserModelStatus
  -> 即时聚合当前账号
  -> detail timeline 读账号级 history
```

### 目标流程

现有 runner 是「每个账号一个定时任务」的持续调度模型，不是严格的批处理轮次。因此快照写入不应依赖「所有账号探针完成」这个不存在的统一批次。

建议新增一个状态快照刷新器，按固定间隔执行。刷新目标必须复用现有 `ListStatusTargets` / `ModelSelfCheckTarget` 口径，即从 `model_self_check_config -> channels -> channel_groups -> groups` 推导 enabled 的 (group, model)，并沿用 active channel / active group 过滤；不要另写一套 channel 到 group 的推导逻辑。

```text
刷新状态快照
  -> 复用现有 target 枚举，得到 enabled 的 (group, model)
  -> 计算该 (group, model) 的当前候选账号

对有候选账号的目标
  -> 读取这些账号最近的新鲜 model_self_check_histories
  -> 按现有 OR 口径聚合
  -> 写 model_self_check_status_snapshots

对没有候选账号的目标
  -> 直接写一条 status=failed, reason_code=no_available_account 的快照

用户侧详情
  -> timeline 从 model_self_check_status_snapshots 读取最近 60 条
```

账号级 probe runner 继续独立运行：

```text
ListProbeTasks
  -> 为当前可自检账号生成 task
RunProbe
  -> 写账号级 model_self_check_histories
```

状态快照刷新器只消费这些账号级结果；当没有账号级结果可消费时，也能写 `no_available_account` 或 `no_fresh_probe` 快照。这样不用重写现有 probe runner，也不用向 `model_self_check_histories` 写没有 `account_id` 的伪行。

## 聚合规则

建议沿用现有 OR 聚合思想：

| 条件 | 快照状态 |
| --- | --- |
| `eligible_account_count == 0` | `failed` + `no_available_account` |
| 有候选账号，但新鲜窗口内无结果 | `unknown` + `no_fresh_probe` |
| 至少一个账号 `operational`，且无失败/降级 | `operational` + `ok` |
| 至少一个账号 `operational` 或 `degraded`，但存在失败/降级/覆盖不足 | `degraded` + `partial_degraded` |
| 所有账号结果都是 429 限流 | 先按现有口径记 `degraded`；如产品要求「全限流不可用」，再改为 `failed` |
| 有账号参与，但没有任何可用结果 | `failed` + `all_probe_failed` |

注意：`model_self_check_histories` 中 `degraded` 当前计入可用率；快照表应保持同一口径，避免用户侧 24h / 7d / 30d 指标与时间线冲突。

## 读取流程

### 列表接口

`GET /api/v1/model-status` 可以继续按现有逻辑即时聚合，也可以逐步切到最新快照。

建议分阶段：

1. 阶段 1：列表继续使用现有逻辑；详情页的 `timeline`、24h / 7d / 30d 可用率、降级比例和详情平均延迟改读快照。
2. 阶段 2：列表当前状态和列表卡片指标也改读每个 (group, model) 的最新快照。
3. 阶段 3：补充更长窗口、清理策略和可选桶化展示，保证用户侧完全脱离账号级明细。

### 详情接口

`GET /api/v1/model-status/detail?group_id=...&model=...`：

- 当前状态：阶段 1 可沿用现有即时聚合。
- `timeline`：改读 `model_self_check_status_snapshots`。
- `availability_24h` / `availability_7d` / `availability_30d` / `degraded_ratio_24h` / 详情平均延迟：阶段 1 同步改读快照，避免同一详情页出现「时间线有失败快照，但可用率仍按账号级历史显示 100%」的矛盾。
- 最近 60 条按 `(group_id, model, checked_at DESC)` 查询。
- 不再因为当前没有可用账号而返回空 timeline。

## 前端展示调整

### 文案

当前「近 60 次记录」容易被理解成连续时间轴。建议改为更精确的文案：

- `近 60 次状态快照`
- 或 `最近 60 次检测`

如果仍沿用「近 60 次记录」，应在 tooltip 或空状态中明确它是记录序列，不是连续时间桶。

### 空状态

当快照表仍为空时，时间线不应只显示 60 个无 tooltip 的灰条。建议显示一个明确空状态：

- `暂无状态快照`
- `等待下一次状态快照`

当最新快照是 `no_available_account` 时：

- 时间线显示红色或失败高度条。
- tooltip 文案使用脱敏描述：`当前无可用账号`。
- 不展示账号数以外的内部细节；是否展示 `eligible_account_count=0` 由产品决定。

## 数据保留

建议：

- `model_self_check_histories` 保留现有策略，内部排障用。
- `model_self_check_status_snapshots` 至少保留 30 天，满足 24h / 7d / 30d 可用率。
- 如果数据量可控，默认保留 90 天，方便回溯稳定性。

阶段 1 实现默认采用 90 天保留，并由现有模型自检 runner 每 24 小时触发一次清理。保留期由后台设置 `model_self_check_status_snapshot_retention_days` 控制：

- 默认值：`90`。
- `0`：关闭状态快照自动清理，适合希望长期保留模型状态证据的部署。
- 正数：小于 `30` 会按 `30` 天执行，大于 `3650` 会按 `3650` 天执行。

默认 90 天时等价清理 SQL 示例：

```sql
DELETE FROM model_self_check_status_snapshots
WHERE checked_at < NOW() - INTERVAL '90 days';
```

## 幂等与多实例

快照表是时序证据表，允许 append-only 写入，但刷新器仍需要明确多实例口径：

- 首选：复用现有 runner 的单例运行边界或部署侧单实例约束，保证同一环境只有一个快照刷新器写入。
- 如果未来支持多副本同时运行，必须增加轻量去重或租约机制，例如按 `(group_id, model, checked_at_bucket)` 做唯一约束，或用数据库 advisory lock / 分布式租约保护刷新周期。
- 阶段 1 沿用现有 runner 的部署侧单实例约束；若短期多副本重复写入，读取端已按 `checked_at DESC, id DESC` 稳定排序，但数据量会被实例数放大，不建议把重复写入作为长期口径。

## 兼容性与迁移

- 新表为空时，详情接口可以回退到现有 `model_self_check_histories` 时间线，避免上线瞬间无数据。
- 快照写入上线后，等待至少一个快照刷新周期即可开始产生新时间线。
- 不需要回填历史中的无可用账号窗口，因为现有数据没有保存这些窗口；只能从上线后开始保真。
- 如果要回填已有账号级历史，只能回填有探针结果的点，不能推断没有记录的失败窗口。

## 阶段计划

### 阶段 1：快照表与无账号快照

- 新增迁移 `model_self_check_status_snapshots`。
- 新增 repository 方法：
  - `CreateStatusSnapshot`
  - `ListRecentStatusSnapshots`
  - `ListStatusSnapshotsSince`
- 新增快照刷新器，对所有 enabled 的 (group, model) 写状态快照。
- 无候选账号时写 `failed/no_available_account`。
- 详情 timeline、详情窗口可用率、降级比例和详情平均延迟优先读取快照；无快照时回退旧账号级历史。
- Runner 每 24 小时按 `model_self_check_status_snapshot_retention_days` 清理状态快照；默认 90 天，配置为 0 时不自动清理。

### 阶段 2：列表状态切到快照

- `/api/v1/model-status` 当前状态读取最新快照。
- 列表卡片中的 24h / 7d / 30d 指标从快照聚合。
- 保留账号级历史作为管理员内部 drilldown，不进入用户 DTO。

### 阶段 3：时间桶化展示（可选）

如果产品希望表达连续时间窗口，而不是最近 N 次检测，可在快照基础上增加桶化：

- 后端按固定粒度聚合，如 5 分钟一个桶。
- 前端显示「近 5 小时」或「近 24 小时」。
- 缺失桶明确显示为 `unknown/no_data`，不再默默跳过。

这一阶段不是修复当前误导的必要条件；只要阶段 1 写快照，失败窗口就不会在恢复后消失。

## 验收用例

1. 某 (group, model) 有 2 个账号，均正常，自检后详情时间线出现绿色 `operational/ok` 快照。
2. 某 (group, model) 有 2 个账号，其中 1 个失败、1 个成功，自检后时间线出现 `degraded/partial_degraded` 快照。
3. 某 (group, model) 所有账号被设置为 `schedulable=false` 后，下一次快照刷新写入 `failed/no_available_account` 快照。
4. 某 (group, model) 所有账号进入 `temp_unschedulable_until > NOW()` 后，下一次快照刷新写入 `failed/no_available_account` 快照。
5. 模型连续 2 小时无可用账号，恢复后再次正常，详情时间线仍能看到失败窗口内的失败快照，不会变成全绿。
6. 用户侧 timeline DTO 不包含 `account_id`、`channel_id`、provider、endpoint、上游模型、原始错误、成本字段。
7. 新表为空时，详情接口仍可回退旧账号级历史，不造成上线后时间线全空。
8. 当快照存在时，详情接口不再因当前 `accountIDsForTarget` 为空而返回空 `timeline`。
9. 429 探针结果仍按既有 degraded 口径计入可用率，除非产品明确调整「全限流」状态定义。
10. 出现 `no_available_account` 快照后，详情页 24h / 7d / 30d 可用率会反映该失败窗口，不再出现「时间线有红条但可用率仍为 100%」的同页矛盾。
11. 清理任务不会删除 30 天窗口内用于 24h / 7d / 30d 可用率计算的数据。

## 风险与注意事项

- 快照写入频率等于 (分组, 模型) 数量乘以快照刷新频率；需要评估数据量。
- 如果一个账号属于多个分组，同一次账号探针结果会映射到多个分组快照，这是用户侧口径需要的，不是重复探针。
- 现有 `model_self_check_histories` 没有 `group_id`，不能单靠它准确恢复历史分组状态。
- 无可用账号快照是脱敏状态，不应被解释为某个具体上游失败。
- 如果列表状态阶段 1 仍走即时聚合，而 timeline 已走快照，短时间内可能出现当前状态与最新快照相差一个快照/探针刷新周期；需要前端文案或刷新策略接受这个延迟。

## 开放问题

1. `no_available_account` 在用户侧文案是否显示为「不可用」还是「暂无可用线路」？
2. 全部账号都是模型级限流时，状态应为 `degraded` 还是 `failed`？
3. 是否要在管理员侧提供快照 drilldown，显示聚合原因和账号数量？
4. 列表页是否要与详情页同阶段切到快照口径，还是接受一个阶段的列表/详情口径差异？

## 建议默认决策

- 阶段 1 先落地快照表、无账号快照；详情时间线和详情窗口指标同步读取快照。
- `no_available_account` 用户侧显示为失败状态，tooltip 用「当前无可用账号」。
- 429 继续沿用现有 degraded 口径，避免改变可用率定义。
- 快照默认保留 90 天；管理员可通过 `model_self_check_status_snapshot_retention_days` 调整，`0` 表示关闭自动清理。
- 列表当前状态暂不改，等详情时间线稳定后再统一切到快照。
