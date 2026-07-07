# 模型自检 Token 消耗统计（管理员）

> 状态：已实现（2026-07-08）。用于给模型状态页补充「自检探针消耗了多少 token」的管理员视图。
>
> 关联：本页是 [模型自检监控（定价驱动）](./pricing-driven-self-check-monitoring-design.md) 的补充。该文已明确自检走 `max_tokens=1` 探针、**不写 `usage_logs`、不计用户账单**，且「表结构与 status 口径预留」了后续更细的探测口径——本页正是把探针**真实产生的 token** 记录下来并做管理员侧聚合展示。

## 背景

模型状态页以 `(分组, 模型)` 维度展示自检健康状态。自检探针会周期性向上游模型发送 `max_tokens=1` 的最小请求，这些请求**真实消耗上游 token 额度**，但目前没有任何地方记录消耗量：

- `model_self_check_histories`（`migrations/161_model_self_check.sql`）只存 `latency_ms` / `http_status` / `error_code`，不含 token。
- 探针未从上游响应读取 `usage`。
- 自检刻意不调用 `RecordUsage`，所以也不落 `usage_logs`。

管理员因此无法判断「自检功能到底吃掉了多少 token 额度」。

## 目标

- 模型状态页（**仅管理员视图**）每个模型行展示该模型自检累计消耗的 token：input / output / 合计。
- 时间窗可切换：今日 / 近 7 天 / 近 30 天；今日按管理员浏览器时区计算日历日。
- 数据仅管理员可见，沿用现有脱敏口径。

## 非目标（本期）

- 不折算费用/金额（明确不做，只统计 token）。
- 不向用户侧暴露；不进用户 DTO（account/error/cost 维度保持内部）。
- 不做告警、预算上限、趋势图（仅数字）。
- 不改变探针的健康判定逻辑（token 采集是旁路）。

## 事实基础（已核对代码）

- 探针实现：`backend/internal/service/model_self_check_probe.go`，请求体 `max_tokens: 1`。
- 明细表：`model_self_check_histories`，已含 `model`、`checked_at`，天然支持按模型 + 时间窗聚合；已有索引 `(model, account_id, checked_at DESC)` 与 `(checked_at DESC)`。
- 自检 repo 采用手写 SQL 风格：`backend/internal/repository/model_self_check_repo.go`。
- 设置项：`SystemSettings`（`settings_view.go`）已有 `ModelSelfCheck*` 一组配置。
- 前端入口：`frontend/src/api/modelStatus.ts` + 模型状态页。

## 方案设计

### 1. 数据模型变更（复用现有表，不新建表）

```sql
-- migration: 170_self_check_token_usage.sql
ALTER TABLE model_self_check_histories
    ADD COLUMN IF NOT EXISTS input_tokens  INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output_tokens INT NOT NULL DEFAULT 0;
```

- 已新增 `backend/migrations/170_self_check_token_usage.sql`。
- 本仓库迁移 runner 通过 `go:embed *.sql` 读取新迁移并自动记录 checksum；本次未修改历史迁移，不需要补 checksum 兼容规则。
- 若聚合走 `(model, checked_at)` 全表扫描且数据量偏大，评估补 `(model, checked_at)` 复合索引。
- ent/schema 是否同步以现有 self-check repo 是否走 ent 为准（该 repo 目前手写 SQL）。

### 2. 探针采集 usage

探针当前用**合成 `gin.Context` + `httptest` recorder**走真实 Forward，捕获 `ForwardResult`（status/latency）后**丢弃响应体、不调 `RecordUsage`**（`model_self_check_probe.go`：Anthropic `gatewayService.Forward:260`、OpenAI `ForwardAsChatCompletions:276`、Gemini compat `:292`、Antigravity `antigravityGatewayService.Forward:308`；各探测函数签名 `(int, time.Duration, error)` 仅返回 status/latency/error，无 token）。因此 usage 采集要点：

- **从 recorder 捕获的响应体（或 `ForwardResult`）提取 `usage`**，而不是新发请求；`max_tokens=1` 时 output ≈ 1，input = prompt 规模。
- **4 条转发路径字段名不一**，需分别归一化为 `input_tokens` / `output_tokens`：
  - OpenAI / Gemini-compat（Chat Completions）：`usage.prompt_tokens` / `usage.completion_tokens`
  - Anthropic / Antigravity（Messages）：`usage.input_tokens` / `usage.output_tokens`
- 无 usage / 解析失败：记 `0`（不阻断自检主流程）。探测失败但上游错误响应仍返回 `usage` 时，记录响应中的真实 token；绝大多数错误响应无 `usage`，因此自然归 `0`。
- 探针**结果结构体新增两个 token 字段**，透传到写 `model_self_check_histories` 的 runner/repo 插入处。
- **不新增计费副作用**：仍不调 `RecordUsage`、不写 `usage_logs`。

健康状态判定逻辑不变。

### 3. 聚合查询（repository）

```sql
SELECT model,
       COALESCE(SUM(input_tokens), 0)  AS input_tokens,
       COALESCE(SUM(output_tokens), 0) AS output_tokens
FROM model_self_check_histories
WHERE checked_at >= $1        -- 窗口起点，由后端按 today|7d|30d 计算
GROUP BY model;
```

失败探测通常记 0；若上游错误响应显式返回 `usage`，SUM 会计入这些真实消耗。

### 4. 管理员接口

```
GET /admin/model-self-check/token-usage?window=today|7d|30d
→ 200 {
    "window": "7d",
    "items": [
      { "model": "claude-opus-4-6", "input_tokens": 1234, "output_tokens": 56, "total_tokens": 1290 }
    ]
  }
```

- 挂在现有 admin 鉴权路由（`backend/internal/server/routes/admin.go`）。
- `total_tokens` 后端计算。`window` 非法值回退默认 `today`。
- `today` 使用前端传入的浏览器 `timezone` 计算起点；未传时回退服务器默认时区。
- 已新增 `backend/internal/handler/admin/model_self_check_handler.go`，路由为 `GET /api/v1/admin/model-self-check/token-usage`。

### 5. 前端

- `frontend/src/api/modelStatus.ts` 增加 `fetchSelfCheckTokenUsage(window)`。
- 模型状态页管理员视图：复用当前 24h / 7 天 / 30 天切换器，其中 24h 映射为 `today`；点击模型卡片后的详情弹窗展示 input/output/合计。
- 统计口径是模型全局聚合，不按分组拆分；弹窗文案标注为「全局自检 Token」以避免误读成单个分组消耗。
- i18n 已补 `zh.ts` / `en.ts`。

## 实现落点

- 数据库：`backend/migrations/170_self_check_token_usage.sql`
- 采集与写入：`backend/internal/service/model_self_check_probe.go`、`backend/internal/service/model_self_check_status.go`、`backend/internal/repository/model_self_check_repo.go`
- 管理员接口：`backend/internal/handler/admin/model_self_check_handler.go`、`backend/internal/server/routes/admin.go`
- 前端展示：`frontend/src/api/modelStatus.ts`、`frontend/src/views/user/ChannelStatusView.vue`
- 文案：`frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`

## 数据流

```
探针请求上游 → 解析响应 usage(input/output)
            → 写 model_self_check_histories(含 token 列)
管理员打开模型状态页 → 选时间窗
            → GET /admin/.../token-usage?window=...
            → repository 按 (model, 时间窗) SUM
            → 详情弹窗展示 input/output/合计
```

## 错误处理与边界

- 无 usage / 上游不返回 usage：记 0，不报错。
- 失败响应若带 usage：按真实 usage 记录；这不会改变健康判定，也不会写入用户账单。
- 聚合无数据：返回空数组或全 0。
- 历史清理：沿用现有 self-check retention；统计口径为「保留窗口内」。
- 权限：接口仅管理员。

## 测试计划

- 已覆盖：探针 usage 解析（OpenAI / Anthropic / Gemini / Antigravity；无 usage → 0）。
- 已覆盖：RunProbe 将 token 透传到历史行。
- 已覆盖：手写 repository 插入 token 字段、按 since 聚合 SUM。
- 已验证：后端 targeted tests、`cmd/server` 编译、前端 typecheck/lint。
- 未覆盖：浏览器视觉回归；当前变更仅增加管理员可见的紧凑指标块。

## 影响面

- 1 个迁移（2 列）+ 1 个聚合查询 + 1 个 admin 接口 + 前端一处展示。
- 探针路径新增旁路采集，不改判定逻辑。
- 与其它两个需求无耦合。
