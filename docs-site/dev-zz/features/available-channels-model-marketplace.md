# 可用渠道模型广场与报价导出

> 状态：已落地。用户侧模型广场、价格表格、当前可见报价导出和管理员全量目录导出已经实现。

## 已落地情况

- 用户侧模型广场、价格表格、当前可见报价导出和管理员全量目录导出已经落地。
- 用户侧入口仍是 `/available-channels`；默认“模型广场”按分组分区、以模型为主实体，“价格表格”用于按渠道、计价分组和价格区间横向比较客户生效价。
- 模型广场组件为 `frontend/src/components/channels/AvailableModelMarketplace.vue`，聚合逻辑集中在 `frontend/src/utils/availableModelMarketplace.ts`。
- 模型广场与价格表格共享 `frontend/src/utils/availableChannelCallability.ts` 的可调用分组判断，避免新旧响应兼容口径漂移。
- 一张卡片只代表一个“分组 + 模型”组合；同一分组内的同名模型跨渠道聚合，不同分组分别展示。卡片只展示该分组下存在稳定交付路由的渠道、计费摘要及已确认可原生交付的 API 端点，不把其他分组的能力或网关兼容转换能力混入。
- 前端表格组件为 `frontend/src/components/channels/AvailableChannelModelsTable.vue`。
- 前端扁平化、排序、分组范围过滤、分组图片档位价、计费倍率、价格格式化和 Excel 导出集中在 `frontend/src/utils/availableChannelsCatalog.ts`。
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
| 价格表格 | 使用 `AvailableChannelModelsTable`，按“渠道 + 模型 + 计价分组 + 阶梯”展示平台、计费模式和客户生效价 |

表格视图支持：

- 平台筛选。
- 计费模式筛选。
- 分组范围筛选：全部、公开 + 专属、仅公开、仅专属。
- 价格状态筛选：全部、有价格、未配置。
- 按模型、平台、渠道、计费模式、阶梯和价格列排序。
- 阶梯定价展开为多行，避免把不同区间价格合并成模糊单元格。
- 宽表横向滚动，保留价格比较所需的列密度。

### 导出

导出入口使用同一套 `buildAvailableChannelCatalogRows` 生成行，确保页面表格和 Excel 都按单一计价分组应用生效倍率。

普通用户导出：

- 数据源固定为当前登录用户可见的 `/channels/available`。
- 默认只导出启用渠道。
- 默认排除订阅分组，仅导出公开 + 专属分组。

管理员导出：

- 可选择“管理员全量目录”，来自 `/admin/channels/available-catalog`。
- 可选择“当前可见可用渠道”，与普通用户口径一致。
- 管理员全量目录支持导出全部、仅启用、仅禁用渠道。
- 管理员接口仍保留无稳定路由的模型用于诊断，但其 `route_group_ids` 明确为 `[]`；生效报价导出只生成确有可调用分组的行，不为无路由模型伪造分组价格。
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

“配置了报价”和“当前可交付”是两件事：渠道映射/定价决定商品是否发布，分组关联的 active、schedulable、平台匹配且支持该模型的账号决定是否存在稳定交付路由。普通用户的“可用模型”只保留至少一条稳定路由的商品；管理员全量目录保留无路由商品用于排障，并用显式空数组 `route_group_ids: []` 表达没有可调用分组。

用户侧表格和导出不应声明为公开对外报价，因为其中可能包含：

- 管理员授予该用户的专属分组。
- 用户专属倍率。
- 只对当前账号可见的模型组合。

### 管理员目录

管理员接口：

```text
GET /api/v1/admin/channels/available-catalog
```

该接口返回管理员可见的完整渠道目录，用于管理侧报价整理。每个模型都会返回不省略的 `route_group_ids`：数组内只包含该平台区段中当前存在稳定可调用路由的分组；无路由模型保留在接口中但返回 `[]`。前端仍通过导出弹窗要求选择分组范围和渠道状态，避免默认把订阅或禁用内容混入普通报价。

### 价格含义

模型卡片的价格摘要来自渠道模型定价和展示回落逻辑。token 模型启用分时定价时，当前分时倍率直接替代当前分组默认倍率和用户专属倍率；未启用时才应用当前分组对当前用户生效的倍率，公开模型列表使用公开分组默认倍率。它表达当前目录口径下的客户价格，但真实扣费仍以请求最终命中的分组、计费模型和计费记录为准。

客户目录以最终价格为主，不在模型卡片、分组标题或价格表格的分组标签中另行展示“当前有效倍率”或分组倍率。已启用分时定价的模型仍可展开查看各时段名称、倍率和对应实际价格，便于理解未来价格变化；完整倍率配置只在管理端维护。

价格表格和导出使用与模型卡片相同的倍率优先级。Token 模型启用分时定价时使用当前时段倍率作为完整最终倍率；未启用的 Token 和普通按次模型使用“当前用户专属倍率优先，否则分组默认倍率”。图片模型在分组开启 `image_rate_independent` 时改用 `image_rate_multiplier`，不再叠加用户专属倍率或分组默认倍率。管理员全量目录导出没有目标用户上下文，因此未启用分时定价的普通计费使用各分组默认倍率，图片独立倍率仍按分组图片配置执行。

图片计费存在任一分组档位价时，模型广场、价格表格和导出共同生成 `1K / 2K / 4K` 展示档位。每档基础价按“分组图片档位价 > 渠道同名档位价 > 渠道默认按次价”回落；没有任何可用基础价的档位不展示。该转换只克隆展示数据，不修改共享渠道定价；`image_output_price` 是图片输出 Token 单价，不能冒充单张图片按次价。

可调用分组兼容顺序在模型广场、价格表格和导出中完全一致：响应包含 `route_group_ids` 时以它为权威（包括显式空数组）；旧响应缺少该字段时才按 `supported_endpoints[].group_ids` 回退；两个字段都缺失时保留全局回滚开关所需的旧目录行为。

接口仍分别返回渠道模型基础定价和分组倍率，前端把基础定价作为计算输入，不再把未乘倍率的原始值直接显示为用户价格。真实扣费仍以请求最终命中的分组、计费模型和计费记录为准。

如果未来需要严格区分价格来源，应让后端返回 `pricing_source`，再在表格和导出中显示“渠道价 / 展示回落价 / 未配置”等来源字段。

## 实现边界

已落地文件：

- `frontend/src/views/user/AvailableChannelsView.vue`
- `frontend/src/components/channels/AvailableModelMarketplace.vue`
- `frontend/src/components/channels/__tests__/AvailableModelMarketplace.spec.ts`
- `frontend/src/components/channels/AvailableChannelModelsTable.vue`
- `frontend/src/components/channels/__tests__/AvailableChannelModelsTable.spec.ts`
- `frontend/src/utils/availableModelMarketplace.ts`
- `frontend/src/utils/__tests__/availableModelMarketplace.spec.ts`
- `frontend/src/utils/availableChannelsCatalog.ts`
- `frontend/src/utils/availableChannelCallability.ts`
- `frontend/src/utils/__tests__/availableChannelCallability.spec.ts`
- `frontend/src/utils/__tests__/availableChannelsCatalog.spec.ts`
- `frontend/src/api/admin/channels.ts`
- `backend/internal/handler/admin/channel_handler.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

第一版没有新增数据库表，也没有改真实计费链路。

## 后续保留项

公开免登录模型与价格页已经落地，见 [公开模型列表与价格页](./public-model-catalog.md)。它复用当前模型广场的交付、端点和价格卡片口径，但只展示 active、standard、非专属分组；不要把它与登录后可追加专属、订阅分组和用户倍率的 `/available-channels` 混为一谈。

下面这些仍只是后续设计：

- 报价模板保存、历史报价归档、分享链接。
- 面向销售的正式报价单版本快照。
- 服务端统一生成 Excel。
- `pricing_source` 精确来源字段。
- 按客户、区域或套餐维度的报价模板。

公开模型页已经确认只展示普通新用户默认可见的 active、standard、非专属分组；登录状态不会让该页面追加专属分组、订阅分组或用户专属倍率。模型、价格和 API 端点复用 `/available-channels` 的客户安全投影，具体边界以独立设计文档为准。

## 验收标准

- 模型广场使用响应式卡片展示；每个区块只代表一个分组，每张卡片只代表该分组中的一个模型，窄屏保持单列。
- 同名模型在不同分组中分别出现；同一分组跨渠道时只出现一次，卡片内仅聚合该分组真实可用的渠道、报价和 API 端点。
- 某分组不在模型的 `route_group_ids` 中时，不生成该“分组 + 模型”卡片，避免借用其他分组的交付能力；严格路由开启时，只有 `unknown` 或兼容转换证据而没有任何已确认同协议路由的模型也不保留卡片。
- 用户目录只发布至少存在一条同协议上游路由的 API 端点；严格路由关闭后恢复的 Messages / Chat / Responses 兼容转换不冒充模型的上游原生能力出现在端点列表中。
- 只有定价、没有稳定账号路由的模型不在用户侧伪装成可用；管理员侧必须能看到“无可用路由”诊断。
- 卡片显示模型价格摘要；发现多个不同报价时不擅自选取最低价，而是明确提示切换价格表格精确比较。
- 用户可以切换到表格视图并直接比较模型价格。
- 价格表格中的同一模型跨渠道、跨计价分组、跨阶梯报价保持独立行，并按该行分组对当前用户的生效倍率计算价格；图片独立倍率与 `1K / 2K / 4K` 档位回落必须和真实计费口径一致；不在 `route_group_ids` 中的分组不会生成该模型报价行。
- 管理员全量目录对每个模型都返回数组形态的 `route_group_ids`；无路由模型仍保留在接口诊断数据中，但不会产生任何生效报价行。
- 缺少 `route_group_ids` 的旧响应在模型广场、表格和导出中统一按端点分组回退，避免三个视图对同一响应得出不同可调用分组。
- 导出行与当前筛选、分组范围和价格状态一致。
- 管理员全量目录失败时，导出不会静默使用错误数据源。
- Excel 文件可被 Excel、Numbers 或 WPS 打开。
- 暗色模式下表格可读。
- 无需数据库迁移。

## 推荐验证

```bash
pnpm --dir frontend test:run src/utils/__tests__/availableChannelsCatalog.spec.ts
pnpm --dir frontend exec vitest run src/utils/__tests__/availableModelMarketplace.spec.ts src/components/channels/__tests__/AvailableModelMarketplace.spec.ts
pnpm --dir frontend test:run src/utils/__tests__/availableChannelCallability.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check
go -C backend test -tags=unit ./internal/handler/admin
```

手动验证时重点检查：

- `/available-channels` 模型广场和价格表格切换。
- 同一分组中的同一模型同时由多个渠道提供时，模型广场只显示一张模型卡片；切换到其他分组时独立展示。
- 窄屏下长模型名不会把卡片横向撑开，协议标签仍保持可见。
- 平台、计费模式、分组范围、价格状态筛选。
- 图片模型的分组档位覆盖、渠道档位 / 默认按次价回落，以及图片独立倍率不会叠加用户专属倍率。
- 表头排序。
- 普通用户导出当前可见报价。
- 管理员导出全量目录和降级提示。
