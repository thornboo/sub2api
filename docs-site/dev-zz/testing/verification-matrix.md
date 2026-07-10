# 验证矩阵

这页给 dev-zz 后续修改提供最小验证清单。每次变更按影响范围选择，不要求无差别跑全量，但不能在没有证据时声明完成。

## 文档

| 场景 | 命令 |
| --- | --- |
| 只改 docs-site Markdown / VitePress 配置 | `pnpm --dir docs-site docs:build` |
| 改部署脚本说明或 shell 脚本 | `bash -n deploy/backup-dev-zz.sh deploy/build-image-dev-zz.sh deploy/docker-deploy.sh` |
| 改 deploy 安装脚本 | `bash -n deploy/docker-deploy.sh deploy/install.sh` |
| 改大量 Markdown | `git diff --check` |

## API Key 管理

| 场景 | 推荐命令 |
| --- | --- |
| 批量创建/更新/删除 service | `mise x -C backend -- go test ./internal/service -run 'APIKeyService.*Batch|BuildBatchAPIKeyNames|NormalizeAPIKeyTags' -count=1` |
| 单把 Key 更新、状态、分组语义 | `mise x -C backend -- go test ./internal/service -run 'APIKeyServiceUpdate|APIKeyServiceBatchUpdate' -count=1` |
| handler 请求绑定 | `mise x -C backend -- go test ./internal/handler -run 'TestAPIKeyHandler' -count=1` |
| 路由顺序 | `mise x -C backend -- go test ./internal/server/routes -run 'TestUserRoutesAPIKey' -count=1` |
| repository 标签/筛选 | `mise x -C backend -- go test ./internal/repository -run 'APIKey' -count=1` |
| 前端 API Key 页面 | `pnpm --dir frontend typecheck` 和 `pnpm --dir frontend lint:check` |

必要人工核对：

- `disabled` 是持久化禁用状态，`inactive` 只作为旧别名。
- 编辑标签或额度时，如果 `group_id` 没变，不应重新检查当前用户是否仍可绑定该历史分组。
- `quota_exhausted` / `expired` 是系统状态，普通保存不应把它们覆盖成 `disabled`，除非用户显式切换禁用。

## 用量分析

| 场景 | 推荐命令 |
| --- | --- |
| 单 Key 趋势 | `mise x -C backend -- go test ./internal/repository -run 'TestUsageLogRepositoryGetAPIKeyUsageTrendForUser' -count=1` |
| 用户 usage handler | `mise x -C backend -- go test ./internal/handler -run 'Usage' -count=1` |
| owner analytics | `mise x -C backend -- go test ./internal/repository -run 'OwnerAPIKeyAnalytics|APIKeyUsageTrend' -count=1` |
| 前端 Usage 分析页 | `pnpm --dir frontend typecheck` 和 `pnpm --dir frontend lint:check` |

必要人工核对：

- owner analytics 响应不包含 `account_cost`、`account_id`、账号名、渠道或 `upstream_model`。
- `tags` 聚合不展示总和必须为 100% 的占比。
- `summary.current_key_snapshot` 在 UI 上与历史时间范围聚合分开展示。

## 管理员设置与 OpenAI Fast / Flex

| 场景 | 推荐命令 |
| --- | --- |
| 设置原子写入、策略校验与审计 | `cd backend && go test -tags=unit ./internal/handler/admin -run 'OpenAIFastPolicy|SettingsAuditChanges' -count=1` |
| Fast / Flex 用户匹配和 fallback 语义 | `cd backend && go test -tags=unit ./internal/service -run 'OpenAIFastPolicy' -count=1` |
| Codex identity 大小写规范化 | `cd backend && go test -tags=unit ./internal/pkg/openai -run '^TestPairCodexClientIdentity$' -count=1` |
| 管理端保存与 i18n 契约 | `pnpm --dir frontend exec vitest run src/views/admin/__tests__/SettingsView.spec.ts src/i18n/__tests__/localesNoKeyCollision.spec.ts` |

必要人工核对：

- Fast / Flex 用户 ID 只能是正整数，且同一条规则内不能重复；失败响应不得留下普通设置或认证来源默认值的部分写入。
- 用户专属规则命中 scope / tier 后，其模型白名单 fallback 是终止结果，不会继续落到全局规则。
- WebSocket 会话使用建连时的策略快照；设置变更只影响新连接，已有连接重连后生效。
- 策略变更的审计只记录设置键，不记录完整用户 ID 列表或规则内容。

## 可用渠道和账号模型

| 场景 | 推荐命令 |
| --- | --- |
| 可用渠道模型表格/导出 | `pnpm --dir frontend test:run src/utils/__tests__/availableChannelsCatalog.spec.ts` |
| 模型目录/推荐工具 | `pnpm --dir frontend test:run src/components/account/__tests__/modelCatalog.spec.ts src/components/account/__tests__/channelModelRecommendations.spec.ts` |
| 账号模型映射弹窗 | `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts` |
| 后端模型探测 | `mise x -C backend -- go test ./internal/handler/admin ./internal/server -run 'ProbeModels|Admin' -count=1` |

## 上游供应商与成本边界

| 场景 | 推荐命令 |
| --- | --- |
| 供应商、账号绑定和默认成本迁移 | `cd backend && go test ./migrations -run 'TestMigration(166|172|173|174)'` |
| 默认池历史数据回填 | `cd backend && go test -tags=integration ./internal/repository -run TestMigration174BackfillsOnlyUnambiguousRealSupplierDefaults -count=1` |
| 供应商严格创建、删除审计边界、默认结算 service 语义 | `cd backend && go test ./internal/service -run 'Test(ApplyUpstreamSupplierUpdate|CreateUpstreamSupplier|EnsureUpstreamSupplierDeletable|UpdateDefaultUpstreamCostPoolConfig|NormalizeUpstreamCostPoolDefault)'` |
| 系统供应商 / 无快照成本隔离 | `cd backend && go test ./internal/repository -run 'TestAccount(ListOrder|Repository_LoadUpstream)'`；真实库另跑 `go test -tags=integration ./internal/repository -run 'TestAccountRepoSuite/TestListWithFilters_UpstreamDiscountRequiresRealNonSystemSnapshot'` |
| 管理端上游成本池后端包 | `cd backend && go test ./internal/server ./internal/service` |
| 资金池真实数据库与供应商绑定 | `cd backend && go test -tags=integration ./internal/service -run 'TestUpstream' -count=1` |
| 供应商标签页 UI | `pnpm --dir frontend exec vitest run src/components/admin/account/__tests__/UpstreamCostComparison.spec.ts` |
| 供应商新增 / 编辑 Modal | `pnpm --dir frontend exec vitest run src/components/admin/account/__tests__/UpstreamSupplierModal.spec.ts` |
| 充值默认计算与单笔覆盖 | `pnpm --dir frontend exec vitest run src/components/admin/account/__tests__/UpstreamRechargeRecordsModal.spec.ts` |
| 账号编辑供应商归属边界 | `pnpm --dir frontend exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts` |
| 充值后强制刷新和晚到响应隔离 | `pnpm --dir frontend exec vitest run src/views/admin/__tests__/AccountsView.schedulerScore.spec.ts` |
| 前端类型和目标 lint | `pnpm --dir frontend run typecheck`，再对改动文件跑 `pnpm --dir frontend exec eslint ...` |

必要人工核对：

- 供应商硬删除必须同时没有 active 绑定和任何历史绑定；曾被账号使用时应归档保留审计链。
- `is_system=true` 的旧迁移系统供应商不得出现在供应商列表、资金池列表、账号编辑供应商候选或账号列表成本上下文中。
- 有充值记录、成本快照或非默认资金池时，应归档而不是硬删除。
- 通过历史 ID 直接请求 `is_system=true` 供应商时，不能编辑、归档或删除。
- 归档仍有 active 绑定的供应商时，应提示已有绑定继续生效、新绑定候选隐藏；账号编辑以禁用历史项显示当前归档供应商，无关字段保存不得解绑，只有明确清空才解绑。
- 供应商标签页只使用页面顶部的刷新 / 添加供应商入口；不能同时出现添加账号或第二个新增按钮，账号筛选、自动刷新和账号工具在供应商视图隐藏。
- 供应商新增 / 编辑 Modal 保存默认充值换算和默认参考汇率；修改默认值不得改变当前真实成本、历史记录或已有成本快照。重名创建必须失败且不得覆盖已有默认配置。
- 无真实快照时账号成本折扣必须为 `nil`；`is_system=true` 供应商即使保留历史成本也不得影响排序和 `cost_first`。
- 充值新增 / 修改 / 删除后必须刷新绑定账号的调度快照；前端有旧请求在飞行时仍需强制发起新请求并忽略晚到旧响应。
- `bonus` 只增加额度，不定义独立单位成本，也不单独生成当前成本快照。
- 普通充值默认只输入支付金额并自动显示到账额度；“本次与默认不同”展开后才允许覆盖实际到账和本次参考汇率。
- 账号编辑弹窗只展示供应商归属、上游分组名和上游分组倍率；不得展示真实充值比例、参考汇率、资金池基础成本或模型族倍率编辑。
- 没有真实供应商绑定的账号不得自动落到“未归类供应商”；按账号新增充值记录应提示先绑定真实供应商。
- 普通用户侧接口和页面仍不暴露供应商、资金池、真实成本或利润字段。

迁移 `174` 上线前先确认资金池实际规模；它是事务迁移，小配置表可以保持原子执行，大表则需要单独评估锁窗口：

```sql
SELECT COUNT(*) AS cost_pool_count
FROM upstream_cost_pools;
```

迁移完成后检查每个非系统、未归档供应商是否恰好存在一个未归档默认池；查询应返回零行：

```sql
SELECT
    supplier.id,
    supplier.name,
    COUNT(*) FILTER (
        WHERE pool.is_default
          AND pool.archived_at IS NULL
    ) AS active_default_pool_count
FROM upstream_suppliers supplier
LEFT JOIN upstream_cost_pools pool
    ON pool.supplier_id = supplier.id
WHERE supplier.is_system = FALSE
  AND supplier.archived_at IS NULL
GROUP BY supplier.id, supplier.name
HAVING COUNT(*) FILTER (
    WHERE pool.is_default
      AND pool.archived_at IS NULL
) <> 1;
```

## 运维弹窗和控制台 UI

| 场景 | 推荐命令 |
| --- | --- |
| 通用弹窗栈 | `pnpm --dir frontend test:run src/components/common/__tests__/BaseDialog.spec.ts` |
| 通用 Select 外部点击 | `pnpm --dir frontend test:run src/components/common/__tests__/Select.spec.ts` |
| 运维请求/错误详情弹窗 | `pnpm --dir frontend test:run src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts` |
| 运维弹窗 composable | `pnpm --dir frontend test:run src/views/admin/ops/composables/__tests__/useOpsModalStack.spec.ts` |

## 分支级验证

| 变更规模 | 推荐组合 |
| --- | --- |
| 小型 docs-only | docs build + `git diff --check` |
| 小型后端 | targeted Go tests + `git diff --check` |
| 小型前端 | targeted Vitest + typecheck + lint |
| API/DTO/schema | targeted backend tests + frontend typecheck + docs build |
| 上游 merge | `git diff --check`、冲突标记扫描、前端 typecheck/lint、相关后端包测试、更新 merge-log |
| 发布/部署 | docs build、shell `bash -n`、compose 配置解析、镜像名核对 |

冲突标记扫描：

```bash
rg -n "^(<<<<<<<|=======|>>>>>>>)$"
```

常用全局静态检查：

```bash
git diff --check
pnpm --dir docs-site docs:build
```
