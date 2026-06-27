# 定价驱动的站点自检模型监控 — 设计文档

> 状态：已落地。设计 2026-06-27，实现 2026-06-28。
>
> 本文取代了早期「按上游渠道探针（`channel_monitor_histories`）聚合模型状态」的方案（其设计文档与 `channel_monitor_model_status.go` 实现均已删除）。用户侧页面/DTO/路由外壳沿用，但底层数据源改为站点自检结果，展示维度改为 (分组, 模型)。
>
> 实现要点（与本设计的差异/补充）：用户 DTO 含 `group_id`/`group_name`/`degraded_ratio_24h`；探针标记 `ctxkey.ModelSelfCheckProbe` 让限流封禁 / runtime-block / **重试 / failover** 全部跳过（默认安全）；管理员设置新增 `model_self_check_enabled` 软开关与 `self_check_max_tasks_per_round` 单轮任务上限；4 个平台均有真实 Forward 集成测试。

## 背景与动机

旧方案（已落地 Stage 1）把用户侧「模型服务状态」建立在**上游渠道探针**（`channel_monitor_histories`）之上：管理员要手动配置 provider / endpoint / API Key / 模型，系统才会探测。这带来两个问题：

1. **配置重复**：对外可用的模型清单本来就存在于「渠道定价」里，再让管理员手动配置一份监控项是重复劳动。
2. **信号不对口**：上游探针测的是「上游 endpoint 是否可用」，不等于「站点把这个模型提供给用户时是否正常」。站点自身的路由、模型映射、鉴权、计费前置、网关兼容等问题，上游探针发现不了。

新方向：**把监控开关挂到渠道定价上，由站点自检探针驱动用户侧模型健康状态**，并以「分组/模型」为展示维度。

## 事实基础（已核对代码）

- **对外可用模型来源**：`/api/v1/channels/available` → `(*Channel).SupportedModels()` = **模型映射(ModelMapping) ∪ 渠道定价(ModelPricing)** 的并集（`backend/internal/service/channel.go:495`）。
- **渠道定价结构**：`channel_model_pricing.models` 是 JSONB 数组，一条定价绑定多个模型、价格一致（`migrations/081_create_channels.sql`）。渠道关联多个分组（`Channel.GroupIDs`）。
- **账号按分组绑定**：`account_groups (account_id, group_id, priority)` 多对多。账号选择按分组过滤候选池，分组决定 platform，并触发**按分组的渠道定价限制** `checkChannelPricingRestriction(groupID, model)`。因此「模型是否可用」本质上是**按 (分组, 模型)** 的；但一个账号可同时属于多个分组，账号常被多分组共享。
- **网关转发耦合 gin.Context**：`GatewayService.Forward(ctx, c *gin.Context, account, parsed)`（`gateway_service.go:4792`）。`Forward` 直接接收 `account` 参数，可绕过分组选择、对指定账号探测。编排逻辑在 handler（`gateway_handler.go`，2156 行），无 gin-free service 方法。
- **计费是显式调用**：`RecordUsage` 在 handler 中单独调用（`gateway_handler.go:515`）。自检路径**不调用它**即可不写 `usage_logs`、不计用户账单。
- **现有探测体可借鉴**：`channel_monitor_checker.go` 已有「极小 prompt + max_tokens」的最小请求体构造。
- **调度模式可复用**：`channel_monitor_runner.go` 的「每任务一个 goroutine + ticker + jitter + 并发防重」。

## 目标

- 用户配置**渠道定价**时，对每个模型有一个「启用自检」开关，无需再手动配置监控项。
- 系统对开启自检的模型，定时**调用本站网关链路**做健康探测，结果驱动用户侧模型状态页。
- 用户侧以 **`分组/模型`** 维度展示健康状态（分组名本就对用户可见），但**不暴露** provider、endpoint、账号、上游模型、原始错误、成本。
- 现有「渠道监控」子系统保持现状，作为**管理员排查上游**的能力，与自检完全分离。

## 非目标（本期）

- 不做对外免登录公开状态页。
- 不在用户侧展示 provider/endpoint/账号/上游模型/原始错误/成本。
- 不替代、不改动现有渠道监控子系统。
- 不做「不生成 token 的更轻探测」（表结构与 status 口径预留）。

## 关键决策（已与产品方对齐）

| 决策点 | 选择 |
| --- | --- |
| 探测目标 | **站点自检**（调本站网关，model = 对外模型名） |
| 开关粒度 | **渠道定价行里的每个模型** |
| 用户侧状态来源 | **只看自检**；上游探针仅管理员 |
| 执行方式 | **B1**：合成 `gin.Context`（httptest recorder）+ 走真实 `Forward`，丢弃响应，**不调 `RecordUsage`** |
| 探测单元 | **去重后的上游账号**：枚举能服务该模型的账号（跨分组解析后去重），每账号探一次 |
| 展示/状态单元 | **(分组, 模型)**：把账号探测结果映射回每个用到该账号的分组，按 **OR** 聚合 |
| 展示布局 | **按分组分区，组内列模型**（分组名对用户可见） |
| 结果存储 | **新建 `model_self_check_*`**，与 `channel_monitor_*` 物理分离 |
| 可用率口径 | `degraded` **计入**可用率；详情单独保留降级比例 |
| 间隔/上限 | **管理员可配置**（默认间隔 + 全局上限） |
| 成本/用量 | 不写 `usage_logs`；真实上游 token 成本靠**极小 prompt + 可配置频率 + 账号去重**压低 |

## 设计核心：分组/模型展示 + 账号去重探测

两层分离，是本设计的关键：

- **展示层（给用户）**：以 `分组/模型` 为单位。用户的 API Key 属于某个分组，他关心的就是「我这个分组里这个模型好不好」。布局按分组分区：

  ```
  ▌标准组
     claude-sonnet-4   ✅ 正常   99.2%(24h)
     gpt-4o            ⚠️ 降级   97.0%(24h)
  ▌订阅组
     claude-sonnet-4   ✅ 正常   99.5%(24h)
  ```

- **探测层（底层，省成本）**：按**去重后的账号**探。对一个开启自检的模型，枚举所有能服务它的账号（跨相关分组解析后去重），对每个不同账号通过真实 `Forward` 探一次。

- **回映与聚合**：账号探测结果映射回每个「用到该账号」的 (分组, 模型)；某 (分组, 模型) 的状态 = 其覆盖账号结果的 **OR 聚合**（任一账号成功 → 该分组下模型「正常」，符合网关故障转移的真实体验），详情保留降级比例。

**好处**：分组共享账号时账号只探一次、零浪费；分组隔离账号时各账号都探到、不漏；同时避免了「挑哪个代表分组」的伪命题。

## 整体架构

```
管理员配置层
  渠道定价页 (channel_model_pricing)
    每个模型 ──[ ☑ 启用自检 ]   ← 新增开关，写 model_self_check_config(channel_id, model)
  渠道监控页 (channel_monitors)  ← 保持现状，仅上游探针
        │ 启用的 (渠道,模型)                 │ 上游探针(不变)
        ▼                                   ▼
  站点自检子系统 (新)                   渠道监控子系统 (现有)
   • 解析 (模型→分组→账号) 去重           • 上游 endpoint 探针
   • runner: 每去重账号定时探测            • channel_monitor_*
   • B1: 合成 ctx → 真实 Forward
   • 不写 usage_logs
   • 写 model_self_check_histories
        │ 账号结果 → 回映 (分组,模型)        │ 上游结果
        ▼                                   ▼
  用户侧 /monitor 模型状态             管理员渠道监控/模型健康
   (复用 Stage-1 路由/DTO 外壳)         (排障，看上游细节)
   展示 分组/模型，数据源=自检           数据源 = 上游探针
```

**边界原则**：两套子系统不共享存储、不共享聚合逻辑；用户侧只读自检，管理员排障只读上游探针。

## 数据模型

### 1. `model_self_check_config`（自检开关）

挂在渠道定价上，按 **(渠道, 模型)** 记录开关。定价页勾选框是它的入口。某模型在某渠道开启即监控该渠道服务的分组下的这个模型。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | BIGSERIAL PK | |
| `channel_id` | BIGINT FK | 关联渠道 |
| `model` | VARCHAR | 公开模型名 |
| `enabled` | BOOL | 是否自检 |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

唯一索引 `(channel_id, model)`。**「被监控的 (分组, 模型) 集合」**在运行时从「enabled 行 × 渠道 GroupIDs」推导，无需单独存分组维度的开关。

间隔与全局上限放在**管理员设置**（复用现有 setting 机制），不入本表：`self_check_default_interval_seconds`、`self_check_max_concurrency` 等。

### 2. `model_self_check_histories`（自检结果明细，按账号）

探测单元是账号，故结果按 **(模型, 账号)** 记录；分组维度在读取时通过 `account_groups` 解析。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | BIGSERIAL PK | |
| `model` | VARCHAR | 公开模型名 |
| `account_id` | BIGINT | 被探测账号，**仅管理员/排障，绝不进用户 DTO** |
| `platform` | VARCHAR | 解析出的平台 |
| `status` | VARCHAR | operational / degraded / failed / error |
| `latency_ms` | INT NULL | 探测延迟 |
| `http_status` | INT NULL | 排障用 |
| `error_code` | VARCHAR NULL | 归一化错误码（见错误映射） |
| `checked_at` | TIMESTAMPTZ | |

索引：`(model, account_id, checked_at)`。保留期复用现有 30 天 + `OpsCleanupService` 物理删除模式。

> **(分组, 模型) → 账号集合** 的解析：`account_groups` 给出分组的账号池，与「能服务该模型的账号」取交集。读取状态时按此交集对账号的最近结果做 OR 聚合。

## 探测执行（B1）

### 解析与去重

对每个 `enabled` 的 (渠道, 模型)：渠道 GroupIDs → 各分组绑定账号（`account_groups`）→ 过滤出支持该模型/平台的账号 → **跨分组去重**得到「待探账号集合」。多个 (渠道,模型) 解析到同一账号也只探一次。

### Runner

复用 `channel_monitor_runner.go` 模式：为每个「待探 (模型, 账号)」建 goroutine + ticker（间隔取自管理员设置），带 jitter 与并发防重，受全局并发上限约束。配置变更（开关/间隔）触发 reschedule。全局功能开关 gate（复用 `featureEnabled`）。

### 单次探测流程

1. 构造合成 `gin.Context`（`httptest.NewRecorder()`，响应丢弃）；设置内部 context（platform 等）。
2. 构造最小请求体：model = 公开模型名，1 token 级 prompt，`max_tokens=1`。
3. **直接对指定账号** `Forward(ctx, synthCtx, account, parsed)`，捕获 `ForwardResult`（status、latency）或 error。**不调 `RecordUsage`**。
4. 归一化结果 → status + `error_code`。
5. 写入 `model_self_check_histories`。

### 成本控制

`max_tokens=1` + 极小 prompt + 账号去重 + 可配置间隔/并发上限 + 仅探 admin 显式开启项。

## 错误归一化（用户侧白名单）

原始错误只在结果表与管理员排障侧出现，**绝不进用户 DTO**：

| 内部情况 | error_code | 用户侧文案 |
| --- | --- | --- |
| 上游 401/403 | `config_error` | 服务配置异常，正在处理 |
| 上游 429 | `rate_limited` | 当前请求繁忙，可能出现限流 |
| 上游 5xx | `upstream_error` | 模型服务异常 |
| 连接失败 | `conn_error` | 服务连接异常 |
| 超时 | `timeout` | 响应超时 |
| 模型不存在 | `model_missing` | 模型暂不可用 |
| 无账号可探 | `no_account` | 模型暂不可用 |
| 解析失败 | `parse_error` | 服务响应异常 |
| 无历史 | `no_data` | 暂无检测数据 |

## 状态聚合与用户侧复用

### 数据源切换

新增 `model_self_check_status.go`（或重构现 `channel_monitor_model_status.go` 的数据访问层），从 `model_self_check_histories` + `account_groups` 读取，按 (分组, 模型) OR 聚合。**保留**：用户路由 `/api/v1/model-status`、`/detail`，脱敏 DTO，前端 `/monitor` 与 `frontend/src/api/modelStatus.ts`（DTO 增加 `group_name`、按分组分组结构）。

### 状态口径（每个 (分组, 模型)）

| 状态 | 规则 |
| --- | --- |
| `operational` | 该分组下任一覆盖账号最近探测成功，延迟/失败率未超阈值 |
| `degraded` | 仍有成功但失败率上升 / 延迟过高 / 部分账号失败 |
| `failed` | 该分组下所有覆盖账号最近探测失败 |
| `unknown` | 已启用但无足够历史 **或最近探测过期** |
| `unmonitored` | 在分组中可见但未启用自检 |

> **修正旧实现缺陷**：旧 Stage-1 只在「零数据」时返回 unknown，不处理「陈旧检测」。本设计聚合时加入 `checked_at` 新鲜度阈值：超阈值未更新 → unknown，避免展示陈旧成功。

窗口可用率 24h/7d/30d 直接扫 `model_self_check_histories` 聚合；`degraded` 计入可用，详情单独给出降级比例。

## 前端

- **渠道定价编辑界面**：每个模型行增加「启用自检」开关（读写 `model_self_check_config`）。
- **用户侧 `/monitor`**：沿用 Stage-1 页面框架，改为**按分组分区、组内列模型**，每行展示状态、24h/7d 可用率、最后检测、公开说明；不出现 provider/endpoint/账号/成本。

## 权限与安全边界

- 用户接口需登录；返回 `group_name`/`model`/`status`/可用率/延迟/时间线/降级比例；**绝不返回** `account_id`/`platform`/`provider`/`endpoint`/`upstream_model`/原始错误/`cost`/`channel_id`。
- 自检配置（开关）属管理员权限，挂在渠道定价管理路由下；间隔/上限属管理员设置。
- 自检不写 `usage_logs`、不计用户账单。

## 测试计划

后端（`go test`）：

- 解析去重：多分组共享账号 → 去重为一次探测；分组隔离 → 覆盖各账号。
- 探测成功 → operational；上游失败 → failed；部分降级 → degraded；无账号 → `no_account`。
- 错误归一化覆盖白名单各分支。
- (分组,模型) OR 聚合：单账号、多账号部分失败、全失败、无历史→unknown、**陈旧检测→unknown**、窗口可用率（degraded 计入）。
- **DTO 禁止字段断言**（旧实现缺失的安全回归护栏）：用户响应不含 `account_id`/`platform`/`provider`/`channel_id` 等。
- B1 探测路径：合成 context 不触发 `RecordUsage`（断言无 usage 写入）。
- Runner 调度：开关/间隔变更 reschedule、并发防重、全局并发上限。

前端：

- 定价页开关读写正确。
- `/monitor` 按分组分区展示，不出现上游字段。
- 搜索、刷新、空状态、移动端无溢出。

验证命令：

```bash
cd backend && go test ./internal/service ./internal/handler ./internal/server/routes
pnpm --dir frontend test:run
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check
```

## 实施策略（基于当前工作区，方案 A：保留骨架、重写核心）

> 本节供实现者（可能脱离本设计对话的上下文）直接照做。结论：**在当前工作区改动基础上继续，不全部撤销重做**。

### 当前工作区现状（均为未提交改动）

当前分支 `dev-zz-develop` 工作区已存在一版「用户侧模型状态页 Stage-1」实现，数据源是上游渠道探针（`channel_monitor_histories`）、按模型聚合、不含分组维度。本设计将其**数据源切到站点自检、展示维度切到 (分组, 模型)**。

涉及文件（`git status` 可见）：

- 后端：`backend/internal/service/channel_monitor_model_status.go`(+test)、`backend/internal/handler/channel_monitor_user_handler.go`、`backend/internal/server/routes/user.go`、`backend/internal/server/routes/user_routes_test.go`
- 前端：`frontend/src/views/user/ChannelStatusView.vue`、`frontend/src/api/modelStatus.ts`、`frontend/src/api/index.ts`、`frontend/src/composables/useChannelMonitorFormat.ts`、`frontend/src/i18n/locales/{en,zh}.ts`
- 文档：`docs-site/...`（api-surface / changelog / patches / index / vitepress config）、`docs-site/dev-zz/features/model-service-status-page.md`

### 文件级处置

| 文件 | 处置 | 说明 |
| --- | --- | --- |
| `routes/user.go`（`/model-status` 路由） | **保留** | 端点 `/api/v1/model-status` + `/detail` 不变 |
| `routes/user_routes_test.go`（旧路由 404 断言） | **保留** | 仍要撤下用户侧 `/channel-monitors` |
| `channel_monitor_user_handler.go` | **改造** | 路由→service→DTO 骨架保留；DTO 增加 `group_name` 与按分组结构；改为调用新 self-check service |
| `channel_monitor_model_status.go` | **删除并新建** | 新建 `model_self_check_status.go`：从 `model_self_check_histories` + `account_groups` 读取，按 (分组,模型) OR 聚合 |
| `channel_monitor_model_status_test.go` | **删除并新建** | 按新聚合逻辑重写（覆盖去重、OR、陈旧检测、禁止字段） |
| `ChannelStatusView.vue` | **改造** | 保留页面 chrome（搜索/刷新/空状态）；列表渲染改为「按分组分区、组内列模型」 |
| `modelStatus.ts` | **改造** | DTO 形状加分组维度 |
| `i18n {en,zh}.ts` | **保留+扩** | 状态/错误文案复用；新增分组相关 key |
| `docs-site/...`（api-surface 等） | **更新** | 同步新端点字段与展示 |
| `model-service-status-page.md` | **删除** | 旧 Stage-1 文档已删除，统一以本文为准（见「旧文档处理」） |

### 新增（当前工作区不存在）

- 迁移：`model_self_check_config`、`model_self_check_histories` 两张表。
- 后端：account 去重解析器、B1 自检 runner（合成 ctx + `Forward` + 不调 `RecordUsage`）、错误归一化、管理员设置项（默认间隔/全局上限）。
- 前端：渠道定价编辑界面的「启用自检」开关（读写 `model_self_check_config`）。

### 建议构建顺序

1. 迁移：两张新表。
2. 后端读写：config CRUD（绑定定价页开关）、histories 写入。
3. 探测链路：account 去重解析 → B1 runner → `Forward` 探测 → 归一化 → 写 histories（先可手动触发，验证不写 `usage_logs`）。
4. 聚合与接口：`model_self_check_status.go`（(分组,模型) OR 聚合）→ 改造 handler/DTO → 删旧 `channel_monitor_model_status.go`。
5. 前端：定价页开关；`/monitor` 改按分组分区展示。
6. 设置项：默认间隔/全局上限。
7. 测试：后端去重/聚合/陈旧检测/禁止字段/无 usage 写入；前端开关与展示。
8. 文档：删除旧 `model-service-status-page.md`，更新 `docs-site` 端点表与导航。

## 分阶段

- **阶段 A（本期）**：`model_self_check_config` + `model_self_check_histories` + 解析去重 + B1 自检 runner + (分组,模型) OR 聚合 + 数据源切换 + 定价页开关 + 用户页按分组展示 + 间隔/上限设置 + 陈旧检测修正 + 禁止字段测试。
- **阶段 B（后续）**：不生成 token 的更轻探测深度；自检结果的管理员排障视图；自检与上游探针的差异对比。

## 旧文档处理（已完成）

旧 `docs-site/dev-zz/features/model-service-status-page.md`（描述被取代的上游探针方案）已**删除**，避免误导后续开发者。所有引用（`index.md`、`reference/api-surface.md`、`.vitepress/config.ts`、`changelog.md`、`patches.md`）已统一指向本文。本文即模型自检监控的唯一权威设计来源。

## 实现落定（原待确认问题）

- 自检间隔/并发/单轮上限：改为**管理员可配置**——`self_check_default_interval_seconds`、`self_check_max_concurrency`、`self_check_max_tasks_per_round`，并有 `model_self_check_enabled` 软开关；运行时无配置时走代码内 fallback。
- 「能服务某模型的账号」枚举：**复用**现有账号-模型支持判定（platform + 模型支持 + 渠道定价限制 `checkChannelPricingRestriction`），并叠加账号资格（active / schedulable / 未临时下线）过滤后去重。
- 可用率口径：`degraded` **计入**可用率，用户 DTO 另返回 `degraded_ratio_24h` 单独反映降级占比。

## 后续可选增强

- 不生成 token 的更轻探测深度（当前 `max_tokens=1`）。
- 自检结果的管理员排障视图、与上游探针的差异对比。
- 非 OpenAI 平台「429 不封账号」的独立单测（当前 OpenAI 有专测，其余平台经共享限流守卫覆盖）。
