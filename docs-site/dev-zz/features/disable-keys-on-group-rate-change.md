# 修改分组倍率时停用受影响的 API Key（可配置）

> 状态：已实现（2026-07-08）。管理员调整计费倍率后，可按全局开关自动停用受影响的 API Key，要求用户确认新倍率后重新启用。

## 背景

管理员可以修改某个分组的计费倍率（分组默认倍率），也可以为特定用户配置专属倍率（覆盖分组默认）。倍率一改，用户的实际计费口径就变了，但**用户可能毫不知情**，继续按新倍率被扣费。

诉求：管理员修改倍率后，**受影响用户的 key 自动停用**，用户必须手动重新启用——重开过程让用户必然感知到「倍率变了」。是否启用此行为由全局开关控制。

## 目标

- 管理员修改倍率（涨 / 降**任意方向**）后，精准命中的用户 key 自动停用。
- 用户在自己的 key 管理页手动重新启用（重开时清除停用原因）。
- 停用提示能区分出「因分组倍率调整被停用」这一原因。
- 全局开关控制是否启用该行为（默认关闭，保持现状）。

## 非目标（本期）

- 不新增独立通知渠道（复用现有请求期停用提示，见下）。
- 不做倍率变更的审计历史。
- 不停用「有专属倍率、实际倍率未变」的用户 key（精准命中，避免误伤）。

## 关键前提：停用提示链路已存在

无需新造通知。key 一旦 `status=disabled`，用户下次调用即被现有鉴权中间件拦截并提示：

`backend/internal/server/middleware/api_key_auth.go:90`

```go
if !apiKey.IsActive() &&
   apiKey.Status != service.StatusAPIKeyExpired &&
   apiKey.Status != service.StatusAPIKeyQuotaExhausted {
    AbortWithError(c, 401, "API_KEY_DISABLED", "API key is disabled")
    return
}
```

同链路已有区分：`API_KEY_EXPIRED`（已过期）、`API_KEY_QUOTA_EXHAUSTED`（额度已用完）、`GROUP_DISABLED`（"API Key 所属分组已停用"）。体验与「余额不足 / 已过期」一致。

用户侧**已能自助重开**：`backend/internal/handler/api_key_handler.go:159` 的更新接口接受 `status: active|disabled|inactive`，`api_key_service.go` 的 `Update` 会把状态置回 `active`。故本需求不需要新增「自助启用」动作，只需在重开时清空停用原因。

## 数据模型：实际倍率的两个来源

```
effective = user_group_rate_multipliers[user_id, group_id].rate_multiplier
            ?? groups[group_id].rate_multiplier
```

| 来源 | 存储 | 管理员修改入口 |
|------|------|----------------|
| 分组默认倍率 | `groups.rate_multiplier` | `UpdateGroup`（`admin/group_handler.go`） |
| 用户专属倍率 | `user_group_rate_multipliers(user_id, group_id, rate_multiplier)`（migration `047` / `127`）；NULL = 沿用分组默认 | `BatchSetGroupRateMultipliers` / `ClearGroupRateMultipliers`（`admin_service.go`） |

API Key 归属：`api_key` 具有 `user_id` 与 `group_id`。

## 方案设计

### 1. 设置项

`SystemSettings` 增加：

```go
DisableKeysOnRateChange bool `json:"disable_keys_on_rate_change"` // 默认 false
```

默认 `false` → 完全维持现状。前端设置页加开关 + 说明。

### 2. 数据模型变更

`api_key` 增加轻量「停用原因」字段：

```sql
-- migration: 171_api_key_disabled_reason.sql
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS disabled_reason VARCHAR(40) DEFAULT '';

UPDATE api_keys
SET disabled_reason = ''
WHERE disabled_reason IS NULL;

ALTER TABLE api_keys
    ALTER COLUMN disabled_reason SET DEFAULT '',
    ALTER COLUMN disabled_reason SET NOT NULL;
```

- 取值：`''`（无 / 管理员手动）| `'rate_changed'`（因倍率调整）。
- ent schema `backend/ent/schema/api_key.go` 同步一个 `field.String("disabled_reason")`。
- 新迁移由 `backend/migrations/migrations.go` 的 `embed *.sql` 自动纳入，运行时按文件内容写入 `schema_migrations.checksum`；`migrations_runner.go` 的兼容表只用于历史误改迁移，本迁移无需新增兼容规则。

### 3. 触发点与精准命中（方向：涨 / 降任意变化）

仅当 `DisableKeysOnRateChange == true` 时生效。在倍率更新的**同一事务**内执行停用：

**A. 修改分组默认倍率**（`UpdateGroup`，`rate_multiplier` 发生变化时）

命中该分组下、**没有专属倍率覆盖**的用户的 key：

```sql
UPDATE api_keys
SET status = 'disabled', disabled_reason = 'rate_changed'
WHERE group_id = $groupID
  AND status = 'active'
  AND deleted_at IS NULL          -- api_keys 为软删除，勿动已删除记录
  AND user_id NOT IN (
    SELECT user_id FROM user_group_rate_multipliers
    WHERE group_id = $groupID AND rate_multiplier IS NOT NULL
  );
```

> `rate_multiplier IS NOT NULL` 精确识别「rate 被专属覆盖」的用户：migration `127` 起该列可空，一行记录可只覆盖 rpm 而 rate 仍沿用分组默认——这类用户实际 rate 随分组默认变化，**应当**被停用，故不在排除集内。

有专属倍率的用户其实际倍率未随分组默认变化 → **不停用**。

**B. 修改某用户专属倍率**（`BatchSetGroupRateMultipliers` / `ClearGroupRateMultipliers`）

只停用该用户在该分组的 key（清除专属倍率＝改回沿用分组默认，也算变化）：

```sql
UPDATE api_keys
SET status = 'disabled', disabled_reason = 'rate_changed'
WHERE group_id = $groupID AND user_id = $userID
  AND status = 'active' AND deleted_at IS NULL;
```

**「是否变化」判定**：只有新值与旧值不同才触发（改了但值没变、或改的是其它字段，不误停）。分组更新是部分更新（指针字段），需对比 `rate_multiplier` 旧 / 新值。

### 4. 停用提示文案

- 中间件返回 `API_KEY_DISABLED` 时，若 `disabled_reason == 'rate_changed'`，文案改为：
  「因分组倍率调整，该 API Key 已被停用，请在控制台确认后重新启用。」
- 其它情况沿用通用 `"API key is disabled"`。
- i18n 同步 `zh.ts` / `en.ts`。

### 5. 用户重新启用

- 用户在自己的 key 管理页把该 key 置回 `active`（现有 `Update` 已支持）。
- 服务端在 `disabled → active` 时**清空** `disabled_reason`（置 `''`）。
- 前端 key 列表：`disabled_reason == 'rate_changed'` 时标注原因徽标 + 「重新启用」按钮。

## 数据流

```
管理员改分组默认倍率 / 用户专属倍率
  → 开关关闭：仅更新倍率(现状)
  → 开关开启：同事务内
       ├─ A: 停用该分组内无专属倍率用户的 active key
       └─ B: 停用该用户在该分组的 active key
         (status=disabled, disabled_reason='rate_changed')
用户下次调用 → 401 API_KEY_DISABLED
             → reason=rate_changed 时提示「因倍率调整被停用」
用户控制台重新启用 → status=active, 清空 disabled_reason
```

## 错误处理与边界

- **精准命中**：有专属倍率的用户不受「分组默认倍率变更」影响，避免误伤。
- **事务一致性**：倍率更新与批量停用同事务；停用失败整体回滚（避免「倍率变了但 key 没停」的错配）。
- **不误停**：仅在倍率值真正变化时停用。
- **只停 active**：不覆盖已 disabled/expired/quota_exhausted 的 key。
- **开关关闭**：全流程等价现状。
- **批量规模**：一个分组可能有大量 key，批量 UPDATE 注意锁范围与超时（参考现有迁移 `lock_timeout` 约定，必要时分批）。

## 测试计划

- 单元：倍率「是否变化」判定（值变 / 值没变 / 改其它字段 / 清除专属倍率）。
- 集成 A：分组默认倍率变更 → 无专属倍率用户 key 被停、有专属倍率用户不动；reason=rate_changed。
- 集成 B：某用户专属倍率变更 → 只停该用户该分组 key。
- 事务：停用失败 → 倍率回滚。
- 中间件：reason=rate_changed 文案分支；通用文案不受影响。
- 重开：disabled→active 清空 reason；非管理员用户能重开自己的 key。
- 开关关闭：倍率变更不停任何 key（回归保护）。
- 前端：reason 徽标；重新启用按钮；设置开关读写。

## 影响面

- `SystemSettings` 加 1 字段 + `api_key` 加 1 列 + 2 处倍率更新入口加停用逻辑 + 中间件文案分支 + 前端标注 / 开关。
- 复用现有停用提示链路与用户自助更新接口，不新增通知渠道。
- 与其它两个需求无耦合。

## 实现记录

- 设置项：`disable_keys_on_rate_change`，默认 `false`，已接入后端 `SystemSettings`、管理员设置接口与前端设置页「功能开关」。
- 数据列：`api_keys.disabled_reason`，迁移文件 `backend/migrations/171_api_key_disabled_reason.sql`，ent schema 已生成。
- 后端触发点：
  - `AdminService.UpdateGroup`：仅当默认 `rate_multiplier` 实际变化时，停用该分组内 active 且没有非空专属 rate 的 Key。
  - `AdminService.UpdateUser`：管理员在「用户分组配置」弹窗修改单个用户的 `group_rates` 时，只停用该用户在实际变化分组下的 active Key；同值保存不误停。
  - `AdminService.BatchSetGroupRateMultipliers`：只停用专属 rate 新旧值变化的用户在该分组下的 active Key。
  - `AdminService.ClearGroupRateMultipliers`：只停用清除前存在非空专属 rate 的用户在该分组下的 active Key。
- 一致性：倍率更新与 Key 停用通过仓储事务上下文执行；停用失败时返回错误并回滚该次倍率写入。
- 重开：用户或管理员将 Key 状态改回 `active` 时，服务端清空 `disabled_reason`。
- 提示：
  - 请求期鉴权：`disabled_reason='rate_changed'` 返回专用 `API_KEY_DISABLED` 文案。
  - 用户 Key 列表：显示「倍率变更停用」原因徽标，启用按钮显示「确认启用」。

## 已验证

- `go test ./internal/service`
- `go test ./internal/service ./internal/repository ./internal/handler/dto ./internal/handler/admin ./internal/server/middleware`
- `go test -tags=unit ./internal/service -run 'TestAdminService_(UpdateGroup_DisablesKeysWhenDefaultRateChanges|BatchSetGroupRateMultipliers|ClearGroupRateMultipliers)|TestAPIKeyServiceUpdate_ClearsRateChangedDisabledReasonWhenReenabled'`
- `pnpm -C frontend typecheck`
- `git diff --check`
