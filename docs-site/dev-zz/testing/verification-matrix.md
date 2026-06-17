# 验证矩阵

本文给 dev-zz 后续修改提供最小验证口径。每次变更按影响范围选取，不要求无差别跑全量，但不能在没有证据时声明完成。

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

## 可用渠道和账号模型

| 场景 | 推荐命令 |
| --- | --- |
| 可用渠道模型表格/导出 | `pnpm --dir frontend test:run src/utils/__tests__/availableChannelsCatalog.spec.ts` |
| 模型目录/推荐工具 | `pnpm --dir frontend test:run src/components/account/__tests__/modelCatalog.spec.ts src/components/account/__tests__/channelModelRecommendations.spec.ts` |
| 账号模型映射弹窗 | `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts` |
| 后端模型探测 | `mise x -C backend -- go test ./internal/handler/admin ./internal/server -run 'ProbeModels|Admin' -count=1` |

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
