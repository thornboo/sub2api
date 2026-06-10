# 可用渠道模型广场与报价导出改造计划

## 背景

当前 `/available-channels` 是用户侧的可用渠道聚合页，按“渠道 -> 平台 -> 我可访问的分组 -> 支持模型”展示数据。模型价格目前主要藏在 `SupportedModelChip` 的悬浮价格卡片里，用户需要逐个 hover 才能查看，不适合横向比较，也不适合直接整理成客户报价单。

本次改造目标是把这个页面升级成更接近“模型广场 / 模型目录”的用户体验，同时支持导出 Excel 报价表，减少手工整理价格表的成本。

## 当前事实

- 用户侧入口：`frontend/src/views/user/AvailableChannelsView.vue`
- 当前渠道表格组件：`frontend/src/components/channels/AvailableChannelsTable.vue`
- 当前模型价格悬浮展示：`frontend/src/components/channels/SupportedModelChip.vue`
- 用户侧接口定义：`frontend/src/api/channels.ts`
- 用户侧后端接口：`GET /api/v1/channels/available`
- 后端用户侧过滤逻辑：`backend/internal/handler/available_channel_handler.go`
- 可用渠道聚合服务：`backend/internal/service/channel_available.go`
- 前端已有 Excel 依赖：`xlsx`、`file-saver`
- 现有 Excel 导出参考：`frontend/src/views/admin/UsageView.vue`

## 产品目标

1. 在 `/available-channels` 页面保留当前“渠道视图”，避免破坏原有理解路径。
2. 新增“模型列表”表格视图，把模型和价格直接摊开展示，方便模型之间比较。
3. 新增 Excel 导出功能，导出系统当前价格数据，减少手工整理报价单。
4. 导出内容尽量与页面展示价格一致，避免手填造成价格不一致。
5. 明确区分“当前登录用户可见报价”和“销售对外报价”的口径，避免误把专属/内部价格发给未注册客户。

## 非目标

- 不在第一阶段做公开免登录报价页面。
- 不在第一阶段做报价模板保存、历史报价归档、分享链接。
- 不改真实计费链路。
- 不新增数据库表。
- 不改变现有渠道、分组、模型定价的管理方式。

## 推荐方案

### 阶段一：用户侧模型列表 + 当前可见数据导出

在 `/available-channels` 页面增加视图切换：

- `渠道视图`：保留现有 `AvailableChannelsTable`。
- `模型列表`：新增表格，按模型维度展示价格。

模型列表建议列：

- 模型名
- 平台
- 渠道
- 渠道描述
- 可访问分组
- 分组类型：公开 / 专属 / 订阅
- 默认倍率
- 用户专属倍率
- 计费模式
- 上下文区间 / 阶梯标签
- 输入价格 / 1M token
- 输出价格 / 1M token
- 缓存写入 / 1M token
- 缓存读取 / 1M token
- 图片输出价格
- 按次价格
- 价格状态：渠道价 / 展示回落价 / 未配置

说明：

- 当前接口已经返回 `platforms -> groups -> supported_models -> pricing`，前端可以先把数据扁平化，不必新增后端接口。
- 如果同一模型存在多个渠道、多个分组、多个阶梯定价，则导出多行，避免把不同价格口径合并成模糊单元格。
- 价格单位保持与当前 UI 一致：token 价格按 `1M token` 展示，按次/图片价格按实际模式展示。

### 阶段二：管理员报价导出

为销售场景增加更明确的管理员导出口径，避免把“当前用户可见”误当成“对外报价”。

建议导出选项：

- `仅公开分组`：默认推荐，适合发未注册客户。
- `公开 + 订阅分组`：适合发套餐介绍。
- `包含专属分组`：适合定制客户报价，默认关闭。
- `仅启用渠道`：默认开启。
- `包含未配置价格模型`：默认关闭或以 `未配置定价` 标注。

实现上可以先复用管理端已有渠道列表数据；如果前端复用成本高或需要服务端统一口径，再新增管理端只读导出接口。

### 阶段三：公开报价页（可选）

如果后续希望客户不登录也能看报价，可以再做公开报价页或分享链接。

这一步需要单独设计权限边界：

- 是否只展示公开分组？
- 是否展示渠道名还是只展示模型服务商？
- 是否允许搜索全部模型？
- 是否要分享 token 或报价模板？
- 是否需要隐藏内部倍率、专属分组、未上架渠道？

公开报价页不建议和本次第一阶段混在一起做。

## 技术设计

### 前端数据扁平化

新增一个工具函数，把用户侧渠道数据转换成模型报价行：

输入：

- `UserAvailableChannel[]`
- `Record<number, number>` 用户专属倍率

输出：

- `AvailableModelPriceRow[]`

建议字段：

```ts
interface AvailableModelPriceRow {
  channelName: string
  channelDescription: string
  platform: string
  modelName: string
  groupNames: string
  groupTypes: string
  defaultRateMultiplier: number | null
  userRateMultiplier: number | null
  billingMode: string
  intervalLabel: string
  inputPricePerMillion: string
  outputPricePerMillion: string
  cacheWritePricePerMillion: string
  cacheReadPricePerMillion: string
  imageOutputPrice: string
  perRequestPrice: string
  pricingStatus: string
}
```

注意：

- `pricing.intervals` 不为空时，每个 interval 输出一行。
- `pricing.intervals` 为空时，用 flat pricing 输出一行。
- 没有价格时保留模型行，价格列显示 `-` 或 `未配置定价`。
- 同一模型跨多个渠道/分组时不强行合并，避免价格口径丢失。

### 表格视图

新增组件建议：

- `frontend/src/components/channels/AvailableChannelModelTable.vue`

职责：

- 接收扁平后的模型报价行。
- 支持加载态、空态。
- 支持横向滚动。
- 支持按模型、平台、渠道、分组、计费模式扫描。

页面改动：

- `frontend/src/views/user/AvailableChannelsView.vue`
  - 增加视图切换状态。
  - 复用当前搜索结果。
  - 新增模型列表 rows 的 computed。
  - 新增导出按钮。

### Excel 导出

新增工具建议：

- `frontend/src/utils/availableChannelsExport.ts`

职责：

- 动态 import `xlsx`。
- 使用 `file-saver` 下载 `.xlsx`。
- 生成 workbook。
- 主 sheet：`模型报价`
- 辅助 sheet：`渠道分组`

文件名建议：

```text
model_quote_YYYY-MM-DD.xlsx
```

Excel 主表建议列与模型列表保持一致，避免页面和导出不一致。

### 国际化

需要补充：

- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

新增文案包括：

- `渠道视图`
- `模型列表`
- `导出 Excel`
- `导出成功`
- `导出失败`
- `模型报价`
- `上下文区间`
- `价格状态`
- `渠道价`
- `展示回落价`
- `未配置定价`

## 后端影响

第一阶段预计不需要后端改动。

当前用户侧接口已经返回：

- 渠道名
- 渠道描述
- 平台
- 用户可访问分组
- 分组倍率
- 模型名
- 模型平台
- 价格字段
- 阶梯定价 intervals

潜在后端增强：

- 如果要区分“渠道自定义价格”和“全局展示回落价格”，建议后端增加 `pricing_source` 字段。
- 如果要做管理员报价导出，建议新增 admin 只读 DTO 或导出接口，避免复用用户侧权限过滤。
- 如果要做公开报价页，需要新增公开接口和权限边界。

## 数据库影响

第一阶段不需要数据库迁移。

只有以下扩展才可能需要数据库：

- 保存报价模板。
- 保存公开分享链接。
- 保存客户专属报价历史。
- 保存报价单版本快照。

## 预计改动文件

第一阶段预计改动 6 到 9 个文件：

- `frontend/src/views/user/AvailableChannelsView.vue`
- `frontend/src/components/channels/AvailableChannelModelTable.vue`（新增）
- `frontend/src/utils/availableChannelsRows.ts`（新增）
- `frontend/src/utils/availableChannelsExport.ts`（新增）
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/components/channels/__tests__/...` 或 `frontend/src/utils/__tests__/...`（新增测试，具体位置按现有测试习惯确定）
- 可能调整 `frontend/src/components/channels/SupportedModelChip.vue`，把价格格式化逻辑抽到共享工具

第二阶段如果做管理员报价导出，可能额外涉及：

- `frontend/src/views/admin/ChannelsView.vue`
- `frontend/src/api/admin/channels.ts`
- `backend/internal/handler/admin/channel_handler.go`
- `backend/internal/server/routes/admin.go`
- 对应后端测试文件

## 实施步骤

1. 抽出价格格式化与扁平化工具。
   - 建立 `AvailableModelPriceRow` 数据结构。
   - 覆盖 token、image、per_request、intervals、无价格等情况。

2. 新增模型列表表格组件。
   - 支持横向滚动。
   - 保持暗色模式可读。
   - 重要价格直接显示，不再依赖 hover。

3. 改造 `/available-channels` 页面。
   - 加入 `渠道视图 / 模型列表` 切换。
   - 保留现有搜索逻辑。
   - 模型列表复用筛选后的渠道数据。

4. 新增 Excel 导出。
   - 导出当前筛选后的模型报价数据。
   - 生成 `模型报价` 和 `渠道分组` 两个 sheet。
   - 空数据时给出提示，不生成空文件。

5. 补充 i18n 文案。
   - 中文和英文同时补齐。

6. 补充测试。
   - 扁平化工具单测。
   - 价格格式化单测。
   - Excel rows 生成逻辑单测。

7. 运行验证。
   - `pnpm --dir frontend run typecheck`
   - `pnpm --dir frontend run lint:check`
   - 针对新增测试运行 `pnpm --dir frontend exec vitest run ...`
   - 手动打开 `/available-channels` 验证渠道视图、模型列表和导出文件。

## 验收标准

- 用户仍可使用原有渠道视图。
- 用户可以切换到模型列表视图。
- 模型列表无需 hover 即可查看主要价格。
- 同一模型跨渠道、跨分组、跨阶梯价格时不会被错误合并。
- Excel 导出的价格与页面展示价格一致。
- Excel 至少包含 `模型报价` sheet。
- 导出文件能被 Excel / Numbers / WPS 打开。
- 暗色模式下模型列表可读。
- 无需数据库迁移。
- 前端 typecheck、lint、相关测试通过。

## 风险与缓解

### 风险：对外报价口径不清

如果直接导出当前用户可见数据，可能包含专属分组或不适合给未注册客户的价格。

缓解：

- 第一阶段按钮文案使用 `导出当前可见报价`。
- 第二阶段再做 `管理员报价导出`，默认仅公开分组。

### 风险：展示回落价格与真实计费价格混淆

当前服务层可能用全局 LiteLLM 数据合成展示价格，这不一定等于渠道真实计费配置。

缓解：

- 第一阶段在文档和 UI 上避免声称“真实扣费价”。
- 后续可让后端返回 `pricing_source`，Excel 增加价格来源列。

### 风险：表格列太多，移动端不好看

价格比较天然是宽表。

缓解：

- 桌面端横向滚动。
- 移动端允许横向滚动，不强行卡片化。
- 关键列固定在左侧可作为后续优化，不放入第一阶段硬要求。

### 风险：intervals 展示复杂

不同渠道可能用不同阶梯标签和价格字段。

缓解：

- 一条 interval 一行。
- 优先使用 `tier_label`。
- 没有 label 时使用 `min_tokens-max_tokens` 生成区间文本。

## 推荐优先级

优先做第一阶段：

1. 模型列表视图。
2. 导出当前可见报价。
3. 明确导出口径文案。

第二阶段再做管理员报价导出，因为那需要更明确的销售口径和权限设计。

