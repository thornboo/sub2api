# API Key 用量下钻

## 已落地情况

- 已按第一版范围落地。
- 后端新增 `GET /api/v1/user/api-keys/:id/usage/trend`，并通过专用 repository 方法按用户时区分桶。
- 后端新增 `GET /api/v1/user/api-keys/:id/usage/models`，用于查看当前用户本人 Key 的模型调用分布。
- 前端在用户侧 `API 密钥` 列表的用量列新增详情入口，弹窗内包含用量趋势、模型分布和请求记录三个 Tab。
- 趋势与模型分布复用项目已有 `chart.js` / `vue-chartjs`，未新增图表依赖。
- 用户侧模型统计响应已脱敏，只返回本人 Key 的请求数、Token 与实际扣费；`cost` / `account_cost` 这类运营成本口径不从用户模型统计接口返回。
- 本轮没有做列表按用量排序，也没有新增 API Key 维度预聚合表。
- owner 视角的多 Key 聚合分析已在 [企业用量分析中心](./enterprise-usage-analytics.md) 中另行落地；本页只记录单 Key 下钻能力。

## 背景

用户侧 `API 密钥` 页面当前已经能看到每把 Key 的简要用量摘要：今日实际扣除和近 30 天实际扣除。这个摘要适合列表扫描，但不适合定位某把 Key 的具体消耗变化，例如：

- 某个员工 Key 今天哪个小时突然消耗升高。
- 某把 Key 最近 7 天、30 天、12 周或 12 个月的费用趋势。
- 某把 Key 的请求次数、输入 token、输出 token、缓存 token 与实际扣费构成。
- 从 Key 列表直接下钻到该 Key 的逐请求使用记录。

这项功能的目标，是在用户侧 `API 密钥` 模块里补上单 Key 用量详情。企业管理员或普通用户可以从某把 Key 直接进入更细的用量视图，按小时、天、周、月查看趋势，也可以继续查看这把 Key 的请求明细。

## 现有基础

### API 密钥列表现状

- 用户侧 Key 页：`frontend/src/views/user/KeysView.vue`。
- 列表中的 `usage` 列只展示：
  - `keys.today`: `usageStats[row.id]?.today_actual_cost`
  - `keys.total`: `usageStats[row.id]?.total_actual_cost`
- 该数据来自 `usageAPI.getDashboardApiKeysUsage(keyIds)`，即当前页 Key 批量汇总接口。
- 前端接口：`frontend/src/api/usage.ts` 的 `getDashboardApiKeysUsage`。
- 后端端点：`POST /api/v1/usage/dashboard/api-keys-usage`。
- handler：`backend/internal/handler/usage_handler.go` 的 `DashboardAPIKeysUsage`。
- repository：`backend/internal/repository/usage_log_repo.go` 的 `GetBatchAPIKeyUsageStats`。
- `GetBatchAPIKeyUsageStats` 默认取近 30 天范围，并单独计算今日实际扣除，输出形态正好对应列表摘要。

### 已有单 Key 日粒度能力

- 用户侧已有单 Key 日粒度端点：
  - `GET /api/v1/user/api-keys/:id/usage/daily?days=30`
  - handler：`GetMyAPIKeyDailyUsage`
  - service：`GetAPIKeyDailyUsage`
  - 前端 wrapper：`usageAPI.getMyApiKeyDailyUsage`
- 该端点会校验 API Key 属于当前登录用户，避免横向越权。
- 当前只支持 `days=1..90`，默认 30 天。
- 当前 service 层固定用 `"day"` 调用 `GetUsageTrendWithFilters`，因此不能直接切换小时、周、月粒度。

### 已有独立 Key 查询页

- 公开路由：`/key-usage`。
- 前端页面：`frontend/src/views/KeyUsageView.vue`。
- 该页面通过用户输入完整 Key 调用网关侧 `GET /v1/usage`。
- 页面已经能展示 `daily_usage` 表格，但它不属于登录后的 `API 密钥` 管理列表，也不支持通用粒度切换。
- 该页面面向“只有 Key、没有站点账号”的查询场景；登录后管理自己 Key 的场景仍应在 `KeysView.vue` 内提供更直接的下钻入口。

### 使用记录页现状

- 用户侧 `使用记录` 页面：`frontend/src/views/user/UsageView.vue`。
- 已支持按 API Key 和日期范围过滤逐请求日志。
- 后端 `GET /api/v1/usage` 会在传入 `api_key_id` 时校验该 Key 归属当前用户。
- 该能力适合展示“每次请求的明细”，但不是按小时、天、周、月聚合的趋势视图。

### 数据与聚合基础

- 原始数据表：`usage_logs`，包含 `user_id`、`api_key_id`、token 字段、`total_cost`、`actual_cost`、`created_at`。
- 已有索引：
  - `idx_usage_logs_api_key_id`
  - `idx_usage_logs_created_at`
  - `idx_usage_logs_api_key_created_at`
- 仓储层 `safeDateFormat` 已允许 `hour`、`day`、`week`、`month`。
- `GetUsageTrendWithFilters` 已支持传入 `userID` 与 `apiKeyID` 过滤后聚合趋势。
- 全局 dashboard 有 `usage_dashboard_hourly` / `usage_dashboard_daily` 预聚合表，但 `GetUsageTrendWithFilters` 只有在没有 user/api_key/account/group/model 等过滤条件时才使用预聚合。单 Key 趋势会查原始 `usage_logs`。

## 产品目标

1. 在用户侧 `API 密钥` 页面为每把 Key 增加用量详情入口。
2. 详情视图支持按 `小时 / 天 / 周 / 月` 粒度查看单 Key 趋势。
3. 趋势视图至少展示请求数、输入 token、输出 token、缓存读写 token、总 token、标准费用、实际扣除。
4. 保留列表摘要的轻量性，不在列表加载时为所有 Key 拉取时间序列。
5. 支持从单 Key 详情继续查看该 Key 的逐请求使用记录。
6. 复用现有 usage 仓储、现有鉴权和现有前端视觉风格，避免引入新数据模型。

## 权限与隐私边界

用户侧下钻接口只服务“当前登录用户查看自己名下 Key”的场景，不能承载管理员分析需求。

用户侧允许展示：

- 本人 Key 的请求数。
- 本人 Key 的输入、输出、缓存读写和总 Token。
- 本人 Key 的实际扣费 `actual_cost`。
- 本人 Key 的请求模型分布和逐请求日志中已经面向用户展示的字段。

用户侧禁止展示：

- 其他用户、其他 owner 或全站聚合数据。
- 上游账号、渠道、账号池、真实上游模型调度细节等运营数据。
- `account_cost`、上游账号成本、账号倍率、渠道成本和利润分析口径。
- 管理员才能使用的跨用户榜单、分组榜单和全站排行榜。

实现要求：

- 所有单 Key 用户接口必须先校验 Key 属于当前登录用户，再查询 usage 数据。
- 用户模型统计接口使用独立响应 DTO，不直接返回 `usagestats.ModelStat`，避免把管理员成本字段透出。
- 管理员后续要做员工榜单、分组榜单、模型成本分析时，应新增管理员端 API 和组件，不能复用用户接口绕过权限边界。

## 非目标

- 不改变真实计费、扣费、限流和网关转发逻辑。
- 不新增子账号、成员实体或企业组织模型。
- 不在第一版新增 API-key 维度预聚合表。
- 不把所有 Key 的完整趋势直接塞进列表页首屏。
- 不改变公开 `/key-usage` 页面现有能力，除非为了复用文案或通用组件做小范围调整。
- 不在第一版做导出 Excel、异常检测、预算告警或预测分析。

## 实现方案

### 单 Key 趋势 API

新增用户侧端点：

```text
GET /api/v1/user/api-keys/:id/usage/trend
```

查询参数：

```text
granularity=hour|day|week|month
start_date=YYYY-MM-DD
end_date=YYYY-MM-DD
timezone=Asia/Shanghai
```

默认策略：

| 粒度 | 默认范围 | 建议最大范围 | 说明 |
| --- | --- | --- | --- |
| hour | 今天 | 31 天 | 当前接口只使用 `start_date/end_date`，小时粒度按自然日范围分桶，不表达滚动 24 小时 |
| day | 最近 30 天 | 180 天 | 覆盖当前 daily 能力并向后兼容 |
| week | 最近 12 周 | 104 周 | 使用 ISO 周显示，如 `2026-24` |
| month | 最近 12 个月 | 60 个月 | 用于长期成本变化 |

实现要点：

- handler 解析 `apiKeyID` 并读取当前登录用户。
- 复用 `apiKeyService.GetByID` 校验 Key 归属。
- 校验 `granularity` 只能为 `hour/day/week/month`，不要依赖 repository 默认回落掩盖错误输入。
- 解析 `start_date` / `end_date`，使用半开区间 `[start, end)`。
- 未传范围时根据粒度生成默认范围。
- 对范围跨度设上限，避免单 Key 查询拖垮大表。该限制必须在后端 handler 层硬校验并返回 400，前端快捷范围按钮只是便利入口，不能作为性能保护边界。
- service 新增 `GetAPIKeyUsageTrend(ctx, userID, apiKeyID, startTime, endTime, granularity, timezone)`，内部调用单 Key 专用 repository 方法，不直接改造公共 `GetUsageTrendWithFilters`。
- 返回字段可复用 `TrendDataPoint`，但 response 应包含 `granularity`、`start_date`、`end_date`、`timezone`。
- 第一版仅支持 date-only 范围。如果未来需要“最近 24 小时”这种滚动窗口，应另行增加 `start_time/end_time` 或 `range=24h`，不要把 date-only 参数解释成滚动小时。
- `week` 粒度 response 或前端展示应在 ISO 周编号旁补自然日期区间，例如 `2026-24 (06-08 ~ 06-14)`，避免用户只看到 `IYYY-IW` 后无法判断实际范围。

响应示例：

```json
{
  "items": [
    {
      "date": "2026-06-14 10:00",
      "requests": 18,
      "input_tokens": 12000,
      "output_tokens": 3200,
      "cache_creation_tokens": 0,
      "cache_read_tokens": 6400,
      "total_tokens": 21600,
      "cost": 0.42,
      "actual_cost": 0.31
    }
  ],
  "granularity": "hour",
  "start_date": "2026-06-14",
  "end_date": "2026-06-14",
  "timezone": "Asia/Shanghai"
}
```

### 保留 daily 端点并迁移前端到 trend 端点

已有端点 `GET /api/v1/user/api-keys/:id/usage/daily?days=30` 不建议立即删除。

处理策略：

- 保留 daily 端点，避免破坏已经接入的前端 wrapper、测试或外部调用。
- 新增 trend 端点作为通用能力。
- `getMyApiKeyDailyUsage` 可以继续保留；新的 UI 使用 `getMyApiKeyUsageTrend`。
- 后续若要减少重复逻辑，可让 daily handler 内部转调 trend service，并保持返回结构不变。

### API 密钥列表增加详情弹窗

在 `frontend/src/views/user/KeysView.vue` 增加单 Key 用量详情入口。

推荐交互：

- 在 `usage` 列的今日/近 30 天摘要下方增加一个小按钮或图标按钮，文案为“详情”。
- 点击后打开项目统一 `BaseDialog` 弹窗，标题显示 Key 名称、标签、分组、状态。
- 弹窗顶部保留摘要：
  - 今日实际扣除
  - 近 30 天实际扣除
  - quota / quota_used
  - rate limit 5h/1d/7d 当前窗口用量
- 主区域使用 Tab：
  - `趋势`
  - `请求明细`

趋势 Tab：

- 粒度切换：小时、天、周、月。
- 日期范围选择：
  - 小时：今天、昨天、最近 7 天、自定义日期范围。
  - 天：最近 7 天、30 天、90 天、自定义。
  - 周：最近 12 周、26 周、52 周、自定义。
  - 月：最近 6 个月、12 个月、自定义。
- 图表优先展示实际扣除和请求数；表格展示完整字段。
- 加载态、空态、错误态要和现有页面风格一致。

请求明细 Tab：

- 复用 `usageAPI.query`，固定传入 `api_key_id`。
- 支持分页和日期范围。
- 第一版优先内嵌请求明细 Panel，而不是跳转到 `/usage`。原因是 `UsageView.vue` 目前没有读取 URL query 初始化 `api_key_id` 的逻辑；为了跳转方案去改复杂存量页面，回归面不一定比新建 `ApiKeyUsageLogsPanel` 更小。
- 如果因时间原因暂缓内嵌请求明细，也可以先放一个“查看使用记录”按钮跳转到 `/usage`，但不能只追加 `?api_key_id=...` 后假设页面会生效；若要自动带上当前 Key，必须同步补齐 `UsageView.vue` 的 query 初始化。

### 组件拆分与复用整理

`KeysView.vue` 已经承担批量创建、筛选、标签、批量操作、复制导出等大量职责。用量下钻第一版不应继续把弹窗、趋势表格和请求明细分页直接塞进该文件，而应从开始就拆出组件：

- `frontend/src/components/keys/ApiKeyUsageModal.vue`
- `frontend/src/components/keys/ApiKeyUsageTrendPanel.vue`
- `frontend/src/components/keys/ApiKeyUsageLogsPanel.vue`

推荐边界：

- `KeysView.vue` 只负责打开弹窗、传入 selected key 和当前列表摘要。
- `ApiKeyUsageModal` 负责 Tab、日期范围和加载编排。
- `ApiKeyUsageTrendPanel` 负责趋势 API、图表和趋势表格。
- `ApiKeyUsageLogsPanel` 负责逐请求记录分页。
- 若第一版先不内嵌请求明细，也仍应保留 `ApiKeyUsageModal` / `ApiKeyUsageTrendPanel` 的边界，避免后续再从 `KeysView.vue` 大文件中拆分。

## 时区与分桶决策

当前代码中存在一个需要显式处理的边界：

- handler 的日期范围可以按用户 timezone 解析。
- repository 当前使用 `TO_CHAR(created_at, format)` 聚合，没有显式 `AT TIME ZONE`。

第一版必须采用“用户时区分桶”，不接受临时按数据库会话时区或系统时区分桶。原因是该功能的核心价值是定位单 Key 在小时/自然日边界上的消耗变化；如果 handler 按用户 timezone 计算 `[start, end)`，SQL 却用裸 `TO_CHAR(created_at, ...)` 按数据库会话时区分桶，会造成桶边界与查询边界错位，小时和日粒度结果都不可信。

- handler 继续接收 `timezone`。
- repository 层新增单 Key 专用查询方法，不改公共 `GetUsageTrendWithFilters`，避免影响 admin/user dashboard 等现有调用点。
- SQL 形态应类似：

```sql
TO_CHAR(created_at AT TIME ZONE $tz, $format)
```

注意：

- `$format` 仍必须来自白名单，不能拼接用户输入。
- `$tz` 必须来自 IANA timezone 校验后的值。当前 `timezone.ParseInUserLocation` / `NowInUserLocation` / `StartOfDayInUserLocation` 的既有语义是用户 timezone 无效时回退服务端配置 timezone；新趋势端点应沿用这一语义，并在 response 中返回最终采用的 timezone。
- 范围边界仍使用 `ParseInUserLocation` 得到的 timestamptz，保持半开区间。

不要在第一版选择“暂不改 repository 时区分桶”。如果实现中发现时区分桶无法安全落地，应先停下来重新评估，而不是交付一个按错误边界聚合的小时/日趋势。

## 后端设计细节

### Handler

建议新增：

```go
func (h *UsageHandler) GetMyAPIKeyUsageTrend(c *gin.Context)
```

注册位置：

```go
user.GET("/api-keys/:id/usage/trend", h.Usage.GetMyAPIKeyUsageTrend)
```

校验顺序：

1. 读取 auth subject。
2. 解析 `:id`。
3. 校验 granularity。
4. 解析 timezone。
5. 解析或生成时间范围。
6. 校验范围跨度上限。
7. 校验 API Key 归属。
8. 调用 service。
9. 返回统一 response。

### Service

建议新增：

```go
func (s *UsageService) GetAPIKeyUsageTrend(
  ctx context.Context,
  userID int64,
  apiKeyID int64,
  startTime time.Time,
  endTime time.Time,
  granularity string,
  timezone string,
) ([]usagestats.TrendDataPoint, error)
```

若 repository 不在第一版支持 timezone 参数，service 签名可先不带 timezone，但 handler response 仍返回 timezone，便于后续无破坏升级。

### Repository

第一版只新增单 Key 专用 repository 方法：

```go
GetAPIKeyUsageTrendForUser(ctx, userID, apiKeyID, startTime, endTime, granularity, timezone)
```

实现要求：

- 传入 `userID` 和 `apiKeyID`，双重过滤保证归属边界。
- 使用现有 `hour/day/week/month` 格式白名单，但不能依赖 `safeDateFormat` 对非法 granularity 的默认回落；handler 已校验，repository 也应尽量 fail-fast。
- SQL 使用 `TO_CHAR(created_at AT TIME ZONE $tz, $format)` 做用户时区分桶。
- 不改公共 `GetUsageTrendWithFilters`，后续如要统一趋势查询接口，再单独做小心的公共接口整理。

## 前端设计细节

### API wrapper

在 `frontend/src/api/usage.ts` 新增：

```ts
export type UsageTrendGranularity = 'hour' | 'day' | 'week' | 'month'

export interface ApiKeyUsageTrendParams {
  granularity?: UsageTrendGranularity
  start_date?: string
  end_date?: string
  timezone?: string
}

export interface ApiKeyUsageTrendResponse {
  items: TrendDataPoint[]
  granularity: UsageTrendGranularity
  start_date: string
  end_date: string
  timezone: string
}

export async function getMyApiKeyUsageTrend(
  apiKeyId: number,
  params: ApiKeyUsageTrendParams,
  options?: { signal?: AbortSignal }
): Promise<ApiKeyUsageTrendResponse>
```

### UI 状态

弹窗状态建议：

- `selectedUsageKey`
- `showUsageModal`
- `usageGranularity`
- `usageStartDate`
- `usageEndDate`
- `usageTrendLoading`
- `usageTrendError`
- `usageTrendRows`
- `usageLogsPagination`

加载策略：

- 打开弹窗时默认加载 `day + 最近 30 天`。
- 切换粒度或日期范围时取消上一请求，避免快速切换造成旧响应覆盖新状态。
- 弹窗关闭时清理请求和错误态，但保留列表页 `usageStats` 不变。

### 图表

如果项目已有图表组件，优先复用。若没有，第一版可以只做表格加轻量汇总，避免为单个功能引入新依赖。

第一版可接受的最小 UI：

- 顶部粒度/日期范围控制。
- 一行汇总：请求数、总 token、实际扣除。
- 趋势表格。
- 请求明细分页 Panel。

后续再增强折线图或柱状图。

## 安全与权限

- 所有单 Key 趋势和明细接口必须校验 Key 归属当前用户。
- 不允许通过 `api_key_id` 查询其他用户 Key 的趋势或请求日志。
- 响应中不返回完整 API Key 明文。
- 可返回 Key 名称、标签、分组、状态，因为这些在当前用户的 `KeysView.vue` 已可见。
- 请求明细 Tab 复用 `/usage` 时，也必须继续依赖后端归属校验，不能只靠前端过滤。

## 性能边界

第一版使用原始 `usage_logs` 聚合是可接受的，因为：

- 查询只针对单个 `api_key_id`。
- 已有 `idx_usage_logs_api_key_created_at`。
- 弹窗懒加载，避免列表页一次性拉取全部 Key 趋势。

必须加范围限制：

- `hour` 粒度不允许无上限查询，超出后端最大范围时直接返回 400。
- `day/week/month` 也应有最大跨度，超出后端最大范围时直接返回 400。
- 请求明细分页必须保留。

如果后续出现以下情况，再评估新增 API-key 维度预聚合表：

- 企业用户有数百把 Key，经常批量查看长周期趋势。
- `usage_logs` 数据量进入千万级后，单 Key 长周期查询仍明显慢。
- 需要在 Key 总览页同时展示 Top N Key 的多周期趋势。

候选预聚合表：

```text
usage_api_key_hourly(api_key_id, bucket_start, requests, tokens, total_cost, actual_cost, ...)
usage_api_key_daily(api_key_id, bucket_date, requests, tokens, total_cost, actual_cost, ...)
```

但这不是第一版目标。

## 测试计划

### 后端

新增或扩展：

- `backend/internal/handler/usage_handler_daily_test.go`
- `backend/internal/service/usage_service.go` 对应单元测试或 stub 测试。
- `backend/internal/repository/usage_log_repo_integration_test.go` 视改动范围补充。

覆盖场景：

- 未登录返回 401。
- 非法 Key ID 返回 400。
- 查询其他用户 Key 返回 403。
- 非法 granularity 返回 400。
- 非法日期格式返回 400。
- 范围超过上限返回 400。
- `hour/day/week/month` 都能传到 repository 并返回对应数据。
- 空数据返回空数组而不是错误。
- timezone 参数合法时用于范围解析；非法 timezone 行为明确。
- timezone 分桶测试：构造接近 UTC/Asia-Shanghai 跨日边界的数据，确认 day/hour 桶按用户 timezone 归属，而不是按数据库会话时区归属。
- 范围上限测试必须直接打后端 handler/API，确认绕过前端传超长范围会返回 400。

建议命令：

```bash
mise x -C backend -- go test ./internal/handler -run 'TestGetMyAPIKey.*Usage' -count=1
mise x -C backend -- go test ./internal/service -run 'Test.*APIKey.*Usage' -count=1
```

### 前端

新增或扩展：

- `frontend/src/views/user/__tests__/KeysView.spec.ts`
- 或新增 `frontend/src/components/keys/__tests__/ApiKeyUsageModal.spec.ts`
- `frontend/src/api` 类型编译由 typecheck 覆盖。

覆盖场景：

- 列表中出现用量详情入口。
- 点击某把 Key 后打开弹窗并默认请求 `day + 最近 30 天`。
- 切换粒度后调用新 API 并渲染返回行。
- 空数据展示空态。
- 请求失败展示错误态。
- 关闭弹窗时不会影响列表摘要。
- 请求明细 Panel 固定带 `api_key_id` 调用 `usageAPI.query`，分页切换不丢失该过滤条件。
- 周粒度展示包含 ISO 周编号和自然日期区间。

建议命令：

```bash
pnpm --dir frontend test:run src/views/user/__tests__/KeysView.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check
```

## 实施顺序

1. 后端新增通用 trend endpoint，保留 daily endpoint。
2. 补后端 handler/service 测试，确认鉴权和粒度边界。
3. 前端新增 `getMyApiKeyUsageTrend` wrapper 和类型。
4. 新建 `ApiKeyUsageModal` / `ApiKeyUsageTrendPanel`，在 `KeysView.vue` 只接入打开入口和 selected key 状态。
5. 新建 `ApiKeyUsageLogsPanel`，内嵌请求明细分页。
6. 补前端测试。
7. 运行 targeted backend/frontend 验证。
8. 实现确认后更新 `docs-site/dev-zz/patches.md` 和 `docs-site/dev-zz/changelog.md`。

## 与现有企业 Key 计划的关系

`enterprise-key-member-management.md` 阶段三已经提到：

- owner 视角统计端点。
- 旗下 Key 用量排行、成本分摊、按标签聚合图表。
- 复用 `KeyUsageView.vue` 的单 Key 用量展示作为下钻详情。

本计划是该阶段三中“单 Key 下钻详情”的独立拆分版本。它先解决每把 Key 的趋势和明细，不直接实现全企业排行、标签聚合和成本分摊大盘。这样改动面更小，也能先复用到普通用户的多 Key 管理场景。

## 未决问题

1. 第一版趋势是否必须上图表，还是表格加汇总即可接受？结论：第一版表格 + 汇总即可，不为单功能新增图表依赖。
2. 请求明细 Tab 第一版是否内嵌表格，还是先跳转到现有 `使用记录` 页面？结论：优先内嵌 `ApiKeyUsageLogsPanel`，避免为跳转方案改复杂存量 `UsageView.vue`。
3. `week` 粒度显示 ISO 周编号是否足够，是否需要显示自然日期区间，如 `2026-06-08 至 2026-06-14`？结论：ISO 周编号旁补自然日期区间。
4. timezone 分桶是否本次一次性做正确。如果不做，是否接受按系统时区分桶的临时行为？结论：第一版必须做正确，不接受错误分桶的临时方案。
5. 是否需要在 API 密钥列表支持按“今日/近 30 天用量”排序。当前列表只展示摘要，不支持按该列排序。结论：第一版不做排序，避免把列表查询和排序口径改造混入本需求。

## Claude 审阅记录

### 2026-06-14 方案审阅结论

Claude 对本方案的关键补充已经合入正文：

- 时区分桶是本需求最大风险。handler 按用户 timezone 计算范围、repository 却用裸 `TO_CHAR(created_at, ...)` 分桶会造成边界错位；第一版必须用 `created_at AT TIME ZONE $tz` 做用户时区分桶。
- 不改公共 `GetUsageTrendWithFilters`，新增单 Key 专用 repository 方法 `GetAPIKeyUsageTrendForUser`，把回归面限制在本需求内。
- `granularity` 必须在 handler 层白名单校验，不能依赖 `safeDateFormat` 的默认 day 回落。
- 时间范围上限必须在后端硬校验，不能只靠前端快捷按钮限制。
- 请求明细第一版优先内嵌 `ApiKeyUsageLogsPanel`，不要为了跳转方案去改复杂的 `UsageView.vue` query 初始化。
- 周粒度应同时展示 ISO 周编号和自然日期区间。
- 第一版不做图表依赖、不做列表用量排序、不混入 gateway/settings 大文件拆分或无关标签清理。
