# 管理员用量分析下钻

> 状态：已落地。管理员侧用户 / API Key 下钻入口、日期回写和月粒度趋势已经实现。

## 已落地情况

- 对应提交：`d4192194 Add admin usage drilldown for users and keys`。
- 涉及页面：管理员侧 `/admin/usage`、用户管理页、用户 API Key 弹窗、管理端 Key 用量弹窗、日期选择组件、趋势图月粒度。
- 后端变更：仅扩展管理员 Dashboard 趋势粒度到 `hour | day | month`，没有新增数据库表或聚合接口。
- 前端变更：新增用户 / API Key 用量下钻入口，把原本需要手动拼 URL 的筛选流程收敛成可视化选择器。
- 不包含：计费、扣费、限流、网关转发、usage log 写入语义。

这页记录的是当前已经落地的实现，不再作为早期设计稿使用。早期设计中的 Key 任意时间范围排行、失败率诊断、复制分析链接、单接口画像聚合等内容没有进入本次实现，已移到后续项。

后续提交 `9e99d62a Make dense usage charts inspectable` 为 `/admin/usage` 的模型、分组、端点和 Token 趋势图增加了展开查看与排名序号。这属于用量分析页的独立展示增强，不并入本页的“用户 / API Key 下钻”主体范围。

## 背景

管理员原来可以看到用户累计消耗、今日 / 近 30 天消耗，也可以在 `/admin/usage` 里查看全站请求明细、趋势、模型分布和错误请求。但这些能力分散在不同页面中：

- 用户列表更适合扫描余额和近 30 天消耗，缺少直接进入用量分析的入口。
- `/admin/usage` 虽然有用户和 Key 筛选能力，但入口偏查询表单，管理员需要自己知道要筛选哪个用户 / Key。
- 用户 API Key 弹窗只能看到 Key 列表和今日 / 近 30 天消耗，不能直接打开某把 Key 的趋势、模型和明细。
- Usage 明细表中用户邮箱点击保留为余额历史，API Key 名称和用量列缺少清晰的分析入口。

本次改动的目标，是把现有 usage 数据组织成管理员能直接操作的“用量分析下钻”流程：

- 从用户列表进入某个用户的用量分析。
- 从用户 API Key 弹窗进入用户全部 Key 或单把 Key 的用量分析。
- 从 Usage 明细表直接下钻到用户或 Key。
- 在 `/admin/usage` 顶部用一个对象选择器完成用户 / Key 搜索和选择，不再要求管理员手写 URL。

## 现在能做什么

### `/admin/usage` 顶部分析上下文

`frontend/src/views/admin/UsageView.vue` 新增了顶部分析上下文区，核心组件是：

- `frontend/src/components/admin/usage/UsageProfileHeader.vue`
- `frontend/src/components/admin/usage/UsageObjectFilterPicker.vue`

顶部区域现在承担三类职责：

- 显示当前分析对象，例如全站、某用户、某用户的全部 Key、某用户的某把 Key。
- 提供对象选择入口，支持搜索用户和选择该用户下的 Key。
- 承载时间范围、趋势粒度、余额历史、用户 Keys、清除筛选等高频动作。

下方通用筛选区仍保留模型、账号、分组、请求类型、计费类型、计费模式等横向诊断条件，但 `/admin/usage` 里不再重复展示用户 / API Key 搜索框：

```vue
<UsageFilters
  ...
  :show-object-filters="false"
/>
```

这样页面信息结构更清晰：

- 顶部：当前在分析谁。
- 中部：当前时间范围和粒度。
- 下方：摘要卡、趋势、分布、明细、错误。

### 分析对象选择器

`UsageObjectFilterPicker` 是一个单下拉面板，分成左右两栏：

- 左侧：用户搜索和用户列表。
- 右侧：选中用户后展示该用户的 Key，并提供“所有 Key”入口。

用户选择逻辑：

- 默认打开时通过 `adminAPI.users.list(...)` 分页加载用户。
- 输入关键词后通过 `adminAPI.usage.searchUsers(...)` 搜索用户。
- 用户列表支持滚动加载更多。

Key 选择逻辑：

- 选中用户后加载该用户的 Key。
- Key 搜索使用 `adminAPI.usage.searchApiKeys(userId, keyword)`。
- 右侧第一个选项是“所有 Key”，用于查看该用户全部 Key 的聚合用量。
- 选择单把 Key 后，页面会进入用户 + Key 双重筛选。

面板交互边界：

- `document click`、`window resize`、`window scroll` 监听只在面板打开期间挂载。
- 关闭面板或组件卸载时会移除监听。
- `panelListenersActive` 防止重复挂载 / 重复移除。

### 路由 Query 与筛选收敛

下钻状态通过 `/admin/usage` 的 route query 表达，相关纯函数位于：

- `frontend/src/utils/adminUsageProfile.ts`

当前支持并校验的 query：

| 参数 | 规则 |
| --- | --- |
| `user_id` | 正整数 |
| `api_key_id` | 正整数 |
| `account_id` | 正整数 |
| `group_id` | 正整数 |
| `start_date` | 有效 `YYYY-MM-DD` |
| `end_date` | 有效 `YYYY-MM-DD` |
| `request_type` | `ws_v2`、`stream`、`sync` |
| `billing_type` | `0`、`1` |
| `billing_mode` | `token`、`per_request`、`image` |
| `model` | trim 后非空字符串 |

非法 query 会被丢弃，并在需要时通过 `router.replace` 静默规范化 URL。

切换分析对象时保留的横向诊断条件：

- 时间范围
- `request_type`
- `billing_type`
- `billing_mode`
- `group_id`

切换分析对象时不会保留：

- 旧的对象筛选，例如上一个 `user_id` 或 `api_key_id`。
- 容易造成误判的 `account_id` 和 `model` 筛选。

这个规则是为了避免“换了用户但仍带着上一个对象的模型 / 账号条件”，导致管理员误以为新对象没有用量。

### Usage 明细表入口

`frontend/src/components/admin/usage/UsageTable.vue` 增加了面向分析的事件：

- `userUsageClick`
- `apiKeyUsageClick`

`UsageView` 中对应处理函数：

- `handleUserUsageClick`
- `handleApiKeyUsageClick`

行为约定：

- 点击用户分析入口：进入该用户全部 Key 的用量分析。
- 点击 Key 分析入口：进入该 Key 的用量分析。
- 用户邮箱的既有点击语义保留为余额 / 充值历史，不直接改成用量分析。

### 用户管理页入口

`frontend/src/views/admin/UsersView.vue` 增加了用量分析入口：

- 用户操作菜单中可以进入该用户的 `/admin/usage` 分析上下文。
- 原有余额历史、API Key、充值、退款、平台限额、充值记录等入口不改变语义。

这样管理员在用户列表看到某个用户消耗异常后，可以直接进入该用户的趋势、模型、端点、明细和错误请求视图。

### 用户 API Key 弹窗入口

`frontend/src/components/admin/user/UserApiKeysModal.vue` 增加了两类入口：

- 顶部用户级用量按钮：打开该用户全部 Key 的管理端用量弹窗。
- 每个 Key 行的图表按钮：打开该 Key 的管理端用量弹窗。

弹窗中的 Key 列表仍使用：

- `adminAPI.users.getUserApiKeys(userId)`

今日 / 近 30 天消耗摘要使用：

- `adminAPI.dashboard.getBatchApiKeysUsage(ids)`

这里展示的是快速定位数据，不是任意自定义时间范围内的精确 Key 贡献排行。

### 管理端 API Key 用量弹窗

新增组件：

- `frontend/src/components/admin/user/AdminApiKeyUsageModal.vue`

它支持两种模式：

- 用户级：只传 `user_id`，查看该用户全部 Key 的用量。
- Key 级：同时传 `user_id` 和 `api_key_id`，查看单把 Key 的用量。

弹窗内包含：

- 摘要统计。
- 趋势 Tab。
- 模型分布 Tab。
- 最近请求 Tab。
- 日期范围选择。

数据来源：

```ts
adminAPI.usage.getStats(baseParams)
adminAPI.dashboard.getUsageTrend({ ...baseParams, granularity: 'day' })
adminAPI.dashboard.getModelStats(baseParams)
adminAPI.usage.list({
  ...baseParams,
  page: 1,
  page_size: 10,
  sort_by: 'created_at',
  sort_order: 'desc',
})
```

弹窗默认时间范围是近 30 天。它目前不会自动继承 `/admin/usage` 页面上的时间范围，这是后续可优化项。

弹窗内部负责维护开始日期和结束日期的区间约束：

- 开始日期不能超过结束日期。
- 结束日期不能早于开始日期。
- `AdminApiKeyUsageModal` 内部使用 `@internationalized/date` 的 `parseDate(...).compare(...)` 比较日期。
- 如果解析失败，才回退到 `YYYY-MM-DD` 字符串比较。

### 日期选择组件

本次引入了 shadcn-vue / reka-ui 风格的日期选择基础组件：

- `frontend/components.json`
- `frontend/src/components/common/AppDatePicker.vue`
- `frontend/src/components/ui/button/*`
- `frontend/src/components/ui/calendar/*`
- `frontend/src/components/ui/popover/*`
- `frontend/src/lib/utils.ts`

`AdminApiKeyUsageModal` 使用 `AppDatePicker` 选择开始日期和结束日期。

`AppDatePicker` 本身只负责单日期选择，不内置开始 / 结束日期联动规则；区间保护由调用它的弹窗或页面组件实现。

### 趋势图月粒度

前端类型 `DashboardTrendGranularity` 扩展为：

```ts
type DashboardTrendGranularity = 'hour' | 'day' | 'month'
```

`/admin/usage` 的自动粒度规则：

| 时间范围 | 自动粒度 |
| --- | --- |
| `<= 1` 天 | `hour` |
| `2 - 60` 天 | `day` |
| `> 60` 天 | `month` |

用户手动选择粒度后：

- 切换分析对象不会重置粒度。
- 修改时间范围会重新推导粒度。
- 重置筛选会重新推导粒度。

后端新增统一归一化函数：

- `backend/internal/handler/admin/dashboard_granularity.go`

支持 `month` 的接口：

- `GET /api/v1/admin/dashboard/trend`
- `GET /api/v1/admin/dashboard/api-keys-trend`
- `GET /api/v1/admin/dashboard/users-trend`
- `GET /api/v1/admin/dashboard/snapshot-v2`

后端默认粒度仍为 `day`，非法值会归一化为 `day`。

### 平台用量悬浮层样式

`frontend/src/components/user/PlatformUsageBreakdown.vue` 调整了平台用量悬浮层的暗色主题样式，使它和项目现有管理后台视觉更一致。

## 数据与权限边界

本次实现没有新增 usage 数据源，仍基于现有管理员接口：

- `/api/v1/admin/usage`
- `/api/v1/admin/usage/stats`
- `/api/v1/admin/dashboard/trend`
- `/api/v1/admin/dashboard/models`
- `/api/v1/admin/dashboard/api-keys-usage`

管理员侧可以看到运营排查字段，例如账号成本、上游账号、模型映射、端点、IP 等。用户侧接口不会复用这些管理员字段。

三个成本口径必须区分：

| 字段 | 管理后台命名建议 | 含义 |
| --- | --- | --- |
| `actual_cost` | 用户扣费 | 用户余额实际扣减 |
| `total_cost` | 标准计费 | 按标准价格计算的费用 |
| `account_stats_cost` / `account_cost` | 账号成本 | 上游账号侧成本 |

旧接口 `GET /api/v1/admin/users/:id/usage` 仍不作为本功能的数据来源。该接口历史上存在 mock / 零值风险，后续如果继续保留，应单独修正或标记废弃。

## 人工验收建议

### `/admin/usage`

1. 打开 `/admin/usage`，确认顶部默认显示全站用量分析。
2. 打开“分析对象”下拉，搜索用户，选择用户后页面应切换为该用户全部 Key 的用量分析。
3. 在右侧选择某把 Key，页面应切换为该用户 / 该 Key 的用量分析。
4. 点击清除 Key，应回到该用户全部 Key。
5. 点击清除用户，应回到全站。
6. 检查 URL query，确认 `user_id`、`api_key_id`、日期和横向筛选能正确同步。
7. 手动修改非法 query，例如 `user_id=abc`，页面应丢弃非法值而不是带脏筛选请求后端。
8. 选择超过 60 天的范围，趋势粒度应自动切到按月。
9. 手动选择按月后切换用户 / Key，粒度应保持不被重置。

### 用户管理页

1. 在用户列表打开某用户操作菜单。
2. 点击用量分析入口，应进入 `/admin/usage` 并带上该用户上下文。
3. 原有邮箱 / 余额历史入口语义不应被改变。

### 用户 API Key 弹窗

1. 从用户操作菜单打开用户 Keys。
2. 顶部用户级用量按钮应打开管理端用量弹窗。
3. 每个 Key 行的图表按钮应打开该 Key 的用量弹窗。
4. 日期选择器中开始日期不能超过结束日期，结束日期不能早于开始日期。
5. Tab 切换时趋势、模型、最近请求应按当前用户 / Key 范围刷新。

### 样式与交互

1. 分析对象下拉不应被下方卡片遮挡。
2. 左侧用户列表和右侧 Key 列表在数据较多时应能滚动。
3. 下拉面板关闭后滚动页面不应触发持续定位开销。
4. 平台用量悬浮层应使用暗色主题，不应出现浅色浮层突兀样式。

## 已验证

本次实现提交前已运行：

```bash
pnpm --dir frontend test:run \
  src/components/admin/usage/__tests__/UsageObjectFilterPicker.spec.ts \
  src/views/admin/__tests__/UsageView.spec.ts \
  src/utils/__tests__/adminUsageProfile.spec.ts \
  src/components/admin/user/__tests__/AdminApiKeyUsageModal.spec.ts \
  src/components/admin/user/__tests__/UserApiKeysModal.spec.ts
```

结果：上述测试文件全部通过。

```bash
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check
```

结果：通过。

```bash
go test -count=1 ./internal/handler/admin \
  -run 'TestDashboardTrendAcceptsMonthlyGranularity|TestDashboardSnapshotV2AcceptsMonthlyGranularity|TestDashboardTrendRequestTypePriority'
```

结果：通过。

```bash
git diff --cached --check
```

结果：通过。

## 当前限制与后续项

### 独立 Key URL 的名称解析

如果 URL 只有 `api_key_id`，没有 `user_id`，前端目前没有管理员侧按 Key ID 直查元数据的接口。当前只能尝试从当前 usage 日志命中行里补 Key 名称。

影响：

- 如果该 Key 在当前时间范围内没有日志，页面可能只能显示 `#<id>`。
- 这不是数据错误，但体验不完整。

建议后续新增轻量后端接口，例如：

```text
GET /api/v1/admin/usage/api-key/:id
```

返回 Key 名称、所属用户、状态等最小展示字段。

### 弹窗默认时间范围

`AdminApiKeyUsageModal` 目前固定默认近 30 天，不继承 `/admin/usage` 当前时间范围。

后续可增加：

```ts
initialRange?: {
  startDate: string
  endDate: string
}
```

让从用量页打开弹窗时保持同一时间口径。

### 后端粒度跨度保护

后端目前只做 `hour | day | month` 白名单归一化，没有限制“超长时间范围 + 小粒度”的组合。

后续建议：

- `hour` 粒度超过一定跨度时强制降级为 `day`。
- `day` 粒度超过一定跨度时强制降级为 `month`。
- 响应中返回实际使用粒度，前端据此回显。

### 任意时间范围 Key 贡献排行

当前用户 API Key 弹窗里的 Key 消耗摘要来自 `getBatchApiKeysUsage(ids)`，只覆盖 today / 近 30 天快速定位。

本次没有实现“当前自定义时间范围内，该用户各 Key 消耗排行”。如果要做精确排行，应新增后端聚合接口，而不是在前端对当前页 usage 明细做不完整聚合。

### 诊断信号

早期设计中提过的失败率、Top 模型占比、环比增长等诊断信号未在本次落地。

如果后续实现，失败率必须保证分子分母同口径：

- 同一时间范围。
- 同一用户 / Key / 模型 / 分组 / 账号 / 请求类型 / 计费模式筛选。
- 若错误请求来源和总请求数来源无法证明一致，宁可不展示失败率。

### 复制分析链接

当前 route query 已经可以表达下钻状态，但没有新增“复制分析链接”按钮。

如果后续增加该按钮，需要提示该链接包含管理员侧对象 ID，例如 `user_id`、`api_key_id`，不应随意发给非管理员。

## 主要文件索引

### 前端页面与组件

- `frontend/src/views/admin/UsageView.vue`
- `frontend/src/views/admin/UsersView.vue`
- `frontend/src/components/admin/usage/UsageProfileHeader.vue`
- `frontend/src/components/admin/usage/UsageObjectFilterPicker.vue`
- `frontend/src/components/admin/usage/UsageTable.vue`
- `frontend/src/components/admin/usage/UsageFilters.vue`
- `frontend/src/components/admin/user/UserApiKeysModal.vue`
- `frontend/src/components/admin/user/AdminApiKeyUsageModal.vue`
- `frontend/src/components/common/AppDatePicker.vue`
- `frontend/src/components/user/PlatformUsageBreakdown.vue`

### 前端工具与测试

- `frontend/src/utils/adminUsageProfile.ts`
- `frontend/src/utils/__tests__/adminUsageProfile.spec.ts`
- `frontend/src/components/admin/usage/__tests__/UsageObjectFilterPicker.spec.ts`
- `frontend/src/views/admin/__tests__/UsageView.spec.ts`
- `frontend/src/components/admin/user/__tests__/AdminApiKeyUsageModal.spec.ts`
- `frontend/src/components/admin/user/__tests__/UserApiKeysModal.spec.ts`

### UI 原语

- `frontend/components.json`
- `frontend/src/components/ui/button/*`
- `frontend/src/components/ui/calendar/*`
- `frontend/src/components/ui/popover/*`
- `frontend/src/lib/utils.ts`

### 后端

- `backend/internal/handler/admin/dashboard_granularity.go`
- `backend/internal/handler/admin/dashboard_handler.go`
- `backend/internal/handler/admin/dashboard_snapshot_v2_handler.go`
- `backend/internal/handler/admin/dashboard_handler_request_type_test.go`
