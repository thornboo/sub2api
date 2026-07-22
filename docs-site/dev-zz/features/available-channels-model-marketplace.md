# 可用渠道模型广场与报价导出

> 状态：已落地。用户侧模型广场、价格表格、当前可见报价导出和管理员全量目录导出已经实现。

## 已落地情况

- 用户侧模型广场、价格表格、当前可见报价导出和管理员全量目录导出已经落地。
- 用户侧入口仍是 `/available-channels`；默认“模型广场”按分组分区、以模型为主实体，“价格表格”用于按渠道、分组和价格区间横向比较。
- 模型广场组件为 `frontend/src/components/channels/AvailableModelMarketplace.vue`，聚合逻辑集中在 `frontend/src/utils/availableModelMarketplace.ts`。
- 一张卡片只代表一个“分组 + 模型”组合；同一分组内的同名模型跨渠道聚合，不同分组分别展示。卡片只展示该分组下存在稳定交付路由的渠道、计费摘要及用户可调用的 API 端点，不把其他分组的能力混入。
- 前端表格组件为 `frontend/src/components/channels/AvailableChannelModelsTable.vue`。
- 前端扁平化、排序、分组范围过滤、价格格式化和 Excel 导出集中在 `frontend/src/utils/availableChannelsCatalog.ts`。
- 用户侧数据源仍是 `GET /api/v1/channels/available`。
- 管理员导出可额外使用 `GET /api/v1/admin/channels/available-catalog` 读取完整可见目录。
- 已有单测覆盖 `availableChannelsCatalog` 的价格格式化、分组范围过滤、阶梯价格展开、启用/禁用渠道状态过滤和导出行生成。

## 目标

`/available-channels` 原先按“渠道 -> 平台 -> 可访问分组 -> 支持模型”展示。该结构适合理解渠道配置，但不适合回答这些问题：

- 某个模型在哪些渠道可用。
- 同一模型在不同分组或阶梯上的价格差异。
- 当前用户可见报价是否能直接导出给内部整理。
- 管理员是否能导出完整渠道目录，包含启用和禁用渠道状态。

这项功能把同一份渠道数据整理成“模型报价目录”，同时保留价格表格用于精确横向比较。

## 页面行为

### 视图切换

页面顶部提供两个视图：

| 视图 | 说明 |
| --- | --- |
| 模型广场 | 使用 `AvailableModelMarketplace`，先按用户可访问分组分区，再按模型展示卡片；同组多渠道聚合，跨分组不混合 |
| 价格表格 | 使用 `AvailableChannelModelsTable`，按模型报价行展示平台、渠道、计费模式、阶梯、价格和分组 |

表格视图支持：

- 平台筛选。
- 计费模式筛选。
- 分组范围筛选：全部、公开 + 专属、仅公开、仅专属。
- 价格状态筛选：全部、有价格、未配置。
- 按模型、平台、渠道、计费模式、阶梯和价格列排序。
- 阶梯定价展开为多行，避免把不同区间价格合并成模糊单元格。
- 宽表横向滚动，保留价格比较所需的列密度。

### 导出

导出入口使用同一套 `buildAvailableChannelCatalogRows` 生成行，确保页面表格和 Excel 口径一致。

普通用户导出：

- 数据源固定为当前登录用户可见的 `/channels/available`。
- 默认只导出启用渠道。
- 默认排除订阅分组，仅导出公开 + 专属分组。

管理员导出：

- 可选择“管理员全量目录”，来自 `/admin/channels/available-catalog`。
- 可选择“当前可见可用渠道”，与普通用户口径一致。
- 管理员全量目录支持导出全部、仅启用、仅禁用渠道。
- 若管理员全量目录加载失败，导出源降级为当前可见渠道，并在导出弹窗显示提示。

导出文件包含模型报价行，列包括：

- 渠道、渠道状态、描述。
- 平台、模型。
- 分组。
- 计费模式。
- 阶梯区间。
- 输入、输出、缓存写入、缓存读取价格。
- 图片输出价格。
- 按次价格。

## 数据口径

### 用户侧目录

用户侧接口：

```text
GET /api/v1/channels/available
```

该接口只返回当前登录用户可访问的渠道和分组。普通用户看到的模型目录不是全站报价表，而是“当前账号可见报价”。

“配置了报价”和“当前可交付”是两件事：渠道映射/定价决定商品是否发布，分组关联的 active、schedulable、平台匹配且支持该模型的账号决定是否存在稳定交付路由。普通用户的“可用模型”只保留至少一条稳定路由的商品；管理员全量目录保留无路由商品用于排障，并明确标记状态。

用户侧表格和导出不应声明为公开对外报价，因为其中可能包含：

- 管理员授予该用户的专属分组。
- 用户专属倍率。
- 只对当前账号可见的模型组合。

### 管理员目录

管理员接口：

```text
GET /api/v1/admin/channels/available-catalog
```

该接口返回管理员可见的完整渠道目录，用于管理侧报价整理。前端仍通过导出弹窗要求选择分组范围和渠道状态，避免默认把订阅或禁用内容混入普通报价。

### 价格含义

价格展示来自渠道模型定价和展示回落逻辑。文档和 UI 不应把它描述为“最终真实扣费价”，因为真实扣费还会受到倍率、分组授权、账号侧计价和调用链路影响。

如果未来需要严格区分价格来源，应让后端返回 `pricing_source`，再在表格和导出中显示“渠道价 / 展示回落价 / 未配置”等来源字段。

## 实现边界

已落地文件：

- `frontend/src/views/user/AvailableChannelsView.vue`
- `frontend/src/components/channels/AvailableModelMarketplace.vue`
- `frontend/src/components/channels/__tests__/AvailableModelMarketplace.spec.ts`
- `frontend/src/components/channels/AvailableChannelModelsTable.vue`
- `frontend/src/utils/availableModelMarketplace.ts`
- `frontend/src/utils/__tests__/availableModelMarketplace.spec.ts`
- `frontend/src/utils/availableChannelsCatalog.ts`
- `frontend/src/utils/__tests__/availableChannelsCatalog.spec.ts`
- `frontend/src/api/admin/channels.ts`
- `backend/internal/handler/admin/channel_handler.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

第一版没有新增数据库表，也没有改真实计费链路。

## 后续保留项

下面这些还只是后续设计，不要把当前表格导出误认为已经覆盖：

- 公开免登录报价页。
- 报价模板保存、历史报价归档、分享链接。
- 面向销售的正式报价单版本快照。
- 服务端统一生成 Excel。
- `pricing_source` 精确来源字段。
- 按客户、区域或套餐维度的报价模板。

公开报价页需要单独处理权限边界，至少明确：

- 是否只展示公开分组。
- 是否隐藏渠道名，只展示模型服务商。
- 是否允许搜索全部模型。
- 是否需要分享 token 或报价模板。
- 是否隐藏内部倍率、专属分组、订阅分组和禁用渠道。

## 验收标准

- 模型广场使用响应式卡片展示；每个区块只代表一个分组，每张卡片只代表该分组中的一个模型，窄屏保持单列。
- 同名模型在不同分组中分别出现；同一分组跨渠道时只出现一次，卡片内仅聚合该分组真实可用的渠道、报价和 API 端点。
- 某分组不在模型的 `route_group_ids` 中时，不生成该“分组 + 模型”卡片，避免借用其他分组的交付能力；仍可沿用旧兼容路由但能力证据未知时保留卡片，并明确显示未发布端点信息。
- 未配置账号原生 Messages 能力但存在可证明的稳定兼容路由时仍展示 `/v1/messages`；其它 API 端点只在统一交付判定确认其实际 Chat / Responses 上游传输可用时追加。
- 只有定价、没有稳定账号路由的模型不在用户侧伪装成可用；管理员侧必须能看到“无可用路由”诊断。
- 卡片显示模型价格摘要；发现多个不同报价时不擅自选取最低价，而是明确提示切换价格表格精确比较。
- 用户可以切换到表格视图并直接比较模型价格。
- 价格表格中的同一模型跨渠道、跨分组、跨阶梯报价仍保持独立行，不会被错误合并。
- 导出行与当前筛选、分组范围和价格状态一致。
- 管理员全量目录失败时，导出不会静默使用错误数据源。
- Excel 文件可被 Excel、Numbers 或 WPS 打开。
- 暗色模式下表格可读。
- 无需数据库迁移。

## 推荐验证

```bash
pnpm --dir frontend test:run src/utils/__tests__/availableChannelsCatalog.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check
```

手动验证时重点检查：

- `/available-channels` 模型广场和价格表格切换。
- 同一分组中的同一模型同时由多个渠道提供时，模型广场只显示一张模型卡片；切换到其他分组时独立展示。
- 窄屏下长模型名不会把卡片横向撑开，协议标签仍保持可见。
- 平台、计费模式、分组范围、价格状态筛选。
- 表头排序。
- 普通用户导出当前可见报价。
- 管理员导出全量目录和降级提示。
