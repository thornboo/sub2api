# 可用渠道模型广场与报价导出

> 状态：已落地。用户侧模型表格、当前可见报价导出和管理员全量目录导出已经实现。

## 已落地情况

- 用户侧模型表格、当前可见报价导出和管理员全量目录导出已经落地。
- 用户侧入口仍是 `/available-channels`，保留原有“渠道视图”，新增“表格视图”用于按模型维度横向比较价格。
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

这项功能把同一份渠道数据再整理成“模型报价目录”，同时保留原有渠道视图，避免破坏用户已经熟悉的路径。

## 页面行为

### 视图切换

页面顶部提供两个视图：

| 视图 | 说明 |
| --- | --- |
| 渠道视图 | 复用 `AvailableChannelsTable`，按渠道和平台展示支持模型 |
| 表格视图 | 使用 `AvailableChannelModelsTable`，按模型报价行展示平台、渠道、计费模式、阶梯、价格和分组 |

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
- `frontend/src/components/channels/AvailableChannelModelsTable.vue`
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

- 原有渠道视图仍可使用。
- 用户可以切换到表格视图并直接比较模型价格。
- 同一模型跨渠道、跨分组、跨阶梯价格时不会被错误合并。
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

- `/available-channels` 渠道视图和表格视图切换。
- 平台、计费模式、分组范围、价格状态筛选。
- 表头排序。
- 普通用户导出当前可见报价。
- 管理员导出全量目录和降级提示。
