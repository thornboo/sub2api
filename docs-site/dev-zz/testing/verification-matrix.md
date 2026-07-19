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

## 企业成员

| 场景 | 推荐命令 |
| --- | --- |
| 成员、预算、导入、审计和 Grok 身份 schema | `cd backend && DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/repository -run '^TestMigrationsRunner_EnterpriseMemberSchemaStaysAligned$' -count=1 -v` |
| 导入多 worker 唯一领取、续租、接管和 fencing | `cd backend && DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/repository -run '^TestEnterpriseMemberImport(ClaimIsUniqueAcrossWorkers\|LeaseTakeoverFencesStaleWorker\|ClaimRecoversProcessingJobWithoutLeaseTimestamp\|LeaseRenewalPreventsPrematureTakeover)$' -count=1 -v` |
| 导入心跳状态机和 5000 行解析边界 | `cd backend && go test -tags=unit ./internal/service -run 'TestEnterpriseMemberImportWorker\|TestParseEnterpriseMemberImportCSVEnforces5000RowCapacityBoundary' -count=1` |
| 5000 行解析 benchmark | `cd backend && go test -tags=unit ./internal/service -run '^$' -bench '^BenchmarkParseEnterpriseMemberImportCSV5000Rows$' -benchmem -count=1` |
| 5000 成员真实事务与逐成员审计 | `cd backend && DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/repository -run '^TestEnterpriseMemberImportCommitHandlesMaximum5000Rows$' -count=1 -v` |
| 软删除历史 Key 防复用 | `cd backend && DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/repository -run '^TestEnterpriseMemberImportReferenceValidationRejectsSoftDeletedKeyReuse$' -count=1 -v` |
| Redis 跨实例认证 L1 失效 | `cd backend && DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/service -run '^TestAPIKeyAuthCacheInvalidationPropagatesAcrossServiceInstances$' -count=1 -v` |
| Redis 重启后的 Pub/Sub 订阅恢复 | `cd backend && DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/service -run '^TestAPIKeyAuthCacheInvalidationSubscriberRecoversAfterRedisRestart$' -count=1 -v` |
| PostgreSQL 事务连接强杀、整体回滚和接管 | `cd backend && DOCKER_HOST="unix://$HOME/.colima/default/docker.sock" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/repository -run '^TestEnterpriseMemberImportCommitConnectionLossRollsBackAndAllowsTakeover$' -count=1 -v` |
| worker Stop、处理 timeout 与 goroutine 生命周期 | `cd backend && go test -tags=unit ./internal/service -run '^TestEnterpriseMemberImportWorker(StopCancelsActiveProcessingAndWaitsForExit\|ProcessingTimeoutUsesFreshFailureContext)$' -count=1 -v` |
| 普通 Key 显式迁移事务 | `cd backend && go test ./internal/repository -run 'TestEnterpriseMemberRepositoryAdoptKey' -count=1` |
| 成员请求记录字段隔离 | `cd backend && go test ./internal/repository -run 'TestEnterpriseMemberRepositoryListUsageRecords' -count=1` |
| 永久删除成员退出 owner 成员统计、趋势、请求与错误记录（含按 ID 详情），归档成员继续保留 | `cd backend && go test ./internal/repository ./internal/service -run 'Test(AppendUsageLogMemberWhereCondition\|OwnerAnalyticsAssignedMembersExcludeRemovedTombstones\|OwnerMemberAssignedUsageConditionsExcludeRemovedTombstones\|BuildOpsErrorLogsWhere_AssignedMembersExcludeRemovedTombstones\|EnterpriseMemberOwnerUsageTrendExcludesRemovedMembers\|OwnerVisibleEnterpriseMemberFactOrUnassignedCondition\|UsageLogRepositoryGetByIDForOwnerRequiresVisibleOrUnassignedMember\|UsageLogRepositoryGetByIDRetainsUnfilteredAuditLookup\|OpsRepositoryGetErrorLogByIDForOwnerRequiresVisibleOrUnassignedMember\|OpsRepositoryGetErrorLogByIDRetainsUnfilteredAuditLookup\|UsageServiceGetByIDForOwnerUsesOwnerVisibleRepositoryQuery\|GetUserErrorRequestDetail_OwnershipEnforced)$' -count=1` |
| 成员多窗口限额预留与无预留结算拒绝 | `cd backend && go test ./internal/repository -run 'TestReserveEnterpriseMemberSpendingLimits\|TestSettleEnterpriseMemberBudgetRejectsRateLimitedMemberWithoutReservation' -count=1` |
| 成员 usage/计费原子写入、outbox 重放和载荷脱敏 | `cd backend && go test -tags=unit ./internal/repository -run 'TestUsageBillingRepositoryApply_\|TestUsageBillingRepositoryReplayPendingSettlement\|TestEnterpriseMemberSettlementPayloadExcludesHydratedSecrets' -count=1` |
| 请求回执、核对元数据与 settlement outbox 迁移合同 | `cd backend && go test ./migrations -run 'TestEnterpriseMember(RequestReceipt\|ReceiptReconciliationMetadata\|UsageSettlementOutbox)Migration' -count=1` |
| 导入 Token 两位小数解析、JSON/SQL 精度与 migration 191 合同 | `cd backend && go test -tags=unit ./internal/service ./migrations -run 'Test(ParseImportTokenCount\|EnterpriseMemberTokenCount\|EnterpriseMemberImportPreviewPreservesDecimalTokenFormats\|EnterpriseMemberImportXLSXPreservesDecimalNumericCells\|EnterpriseMemberFractionalTokenBaselinesMigration)' -count=1` |
| WebSocket 上游结果不明时禁止重放并保留成员预算 | `cd backend && go test -tags=unit ./internal/service -run 'TestOpenAIGatewayService_(ProxyResponsesWebSocketFromClient_(WriteOutcomeUnknownDoesNotRetry\|PreviousResponseNotFoundRecoversByDroppingPrevID\|PassthroughUnknownOutcomeMarksBudgetAmbiguous)\|Forward_WSv2(StreamEarlyCloseMarksOutcomeUnknown\|CloseAfterDispatchDoesNotReplay))' -count=1` |
| Batch image 提交结果不明时保留 hold、禁止重提与退款 | `cd backend && go test -tags=unit ./internal/service -run 'Test(GeminiProvider_CreateBatch\|VertexProvider_CreateBatch\|BatchImagePublicService_Submit\|BatchImageBillingRecoveryService_\|CanTransitionBatchImageJob)' -count=1` |
| 企业成员 zh/en 文案键和页面引用 | `pnpm --dir frontend exec vitest run src/i18n/__tests__/enterpriseMembersLocales.spec.ts` |
| 企业成员控制台布局和交互入口 | `pnpm --dir frontend exec vitest run src/views/user/__tests__/EnterpriseMembersView.layout.spec.ts` |

必要人工核对：

- 普通 Key 迁移前必须展示原分组如何进入成员路由；提交后 Key 固定 `group_id` 清空，但成员绑定不得静默丢失。
- owner 请求记录不得包含上游 account、channel、provider endpoint、account cost、供应商成本或利润字段。
- 企业成员页面必须继续位于 `AppLayout` 和现有侧边栏内，不得恢复为孤立页面。
- 企业成员主体在桌面必须是一行一个成员的紧凑数据表，不得恢复为卡片网格；成员名与成员编号、Key 数与分组数必须拆为独立列，不得纵向堆叠抬高行高；桌面数据单元格使用紧凑垂直内边距，月预算和本月已用金额不得以省略号截断，操作按钮不得折到第二行；表头、桌面行和窄屏行必须复用共享表格选择样式并支持勾选、半选、Space 键与 `aria-checked`，不得使用浏览器原生白色 checkbox；窄屏使用同一容器内的连续紧凑行。
- 编辑成员时成员编号必须只读；5h/1d/7d/月限额和已用值必须成组展示，不展示额外的调整原因字段，已用值变化由后端写入带稳定系统来源的不可变审计。“成员可访问的分组”候选必须来自 owner 当前可访问分组，勾选表达授权，排序表达调用优先级。
- 企业成员状态、预算风险和排序筛选必须使用共享 `Select.vue`，不得回退为浏览器原生 `<select>`；打开态、下拉浮层、选中勾号、方向键、Enter、Escape 和 Tab 语义沿用共享控件。
- 归档可见性必须是共享 Select 的成员范围筛选，不使用眼睛按钮；仅“包含已归档”时状态筛选才出现“已归档”，切回“仅当前成员”必须清除归档状态并重新加载，不能留下无结果的互斥组合。
- 旧 worker 租约被接管后，其迟到 commit 和失败回写都不得改变新 worker 的 job；跨实例缓存测试必须先证明远端 L1 确实持有旧快照，再证明 Pub/Sub 后读到新状态。
- 单次续租错误不得立即中断仍在有效租约内的任务；确认失租或持续错误超过租约期限后必须取消处理，并由 commit fencing 防止迟到写入。
- 上游已经成功但本地 usage/统一计费事务失败时，必须留下可重放 outbox 并保持预算回执不释放；outbox payload 不得包含 API Key 明文或水合上游账号对象。
- Batch image 已完成外部工作后，结算重试耗尽只能进入保留 hold 的低频恢复状态，不得标记普通失败或退款。
- Redis 重启测试必须先观察真实 outage，再等待 `PUBSUB NUMSUB` 证明订阅恢复，最后只发布一次失效消息；PostgreSQL 故障测试必须在 `pg_stat_activity` 证明事务正在执行成员 INSERT 时终止连接。

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

## 提示词审计与安全开关

| 场景 | 推荐命令 |
| --- | --- |
| 配置 CAS、脱敏、节点探测、队列/阻断和删除确认 | `cd backend && go test ./internal/securityaudit -count=1` |
| 网关审计顺序、HTTP/WS 错误和媒体提交边界 | `cd backend && go test ./internal/handler -run 'SecurityAudit\|PromptAudit' -count=1` |
| 管理路由、step-up/session-binding 开关 | `cd backend && go test ./internal/server/routes ./internal/server/middleware ./internal/handler/admin -run 'PromptAudit\|StepUp\|SessionBinding' -count=1` |
| 迁移与真实 PostgreSQL 证据/删除合同 | `cd backend && go test -tags=integration ./internal/securityaudit -run '^TestPromptAudit' -count=1` |
| 管理端提示词审计页面 | `pnpm --dir frontend test:run src/features/prompt-audit/__tests__` |
| Stripe 按需加载与默认 chunk graph | `pnpm --dir frontend test:run src/views/user/__tests__/stripeLazyLoading.spec.ts src/views/user/__tests__/StripePaymentView.spec.ts` |

必要人工核对：

- `prompt_audit_config` 缺失时审计和阻断都关闭；`blocking_enabled` 不得在总审计关闭时独立生效。
- Guard token 只允许写入/清除，公开配置和日志不得回显；任务表不得保存完整提示词，完整内容只允许进入最终事件证据。
- 筛选删除必须先预览并冻结 filter hash、最高事件 ID、管理员和过期时间；确认不得删除预览后新增的事件。
- WebSocket 首 turn 只审计一次，后续 turn 独立审计；企业成员预算仍按 turn 预留并在阻断/断连路径释放或标记结果不明。
- 前端不得为 Stripe 恢复全局 `manualChunks`；三个支付入口必须继续通过 `@stripe/stripe-js/pure` 动态导入。

## OpenAI Responses → Chat fallback 工具桥

| 场景 | 推荐命令 |
| --- | --- |
| Tool Search / deferred / 动态 identity | `cd backend && go test ./internal/pkg/apicompat -run 'ToolSearch|ResponsesToolRegistry|LoadedTopLevel|Deferred|Hosted' -count=1` |
| capability extra 与 scheduler cache | `cd backend && go test -tags=unit ./internal/pkg/openai_compat ./internal/repository -run 'ChatFallbackCapabilities|SchedulerMetadataAccount' -count=1` |
| service capability mismatch | `cd backend && go test -tags=unit ./internal/service ./internal/handler -run 'ChatFallback|AccountCapabilityMismatch' -count=1` |

必要人工核对：

- type-only `tool_search` 是 hosted，不得在无显式账号兼容开关时改成 client proxy。
- 顶层与 namespace 中 `defer_loading: true` 的 function 在 `additional_tools` / client `tool_search_output` 加载前不得出现在 Chat tools。
- 动态顶层 function 的 Responses 回程必须带 `namespace=name`，且历史、非流式和流式名称一致。
- 重复 `tool_search_output.call_id` 只生成一个 Chat tool result，替换前的历史调用仍使用当时的 Chat 名；added/done/completed 的 item ID 一致。
- hosted/server-only 工具与跨来源 identity/flatten 冲突触发 capability mismatch；未知 `execution` 在账号调度前直接返回客户端错误，不遍历账号。
- capability mismatch 只排除当前账号，不写提前响应、不降低账号健康评分；所有账号均不支持时才返回 `unsupported_feature`。若任一账号已访问上游并失败，最终优先返回 upstream 错误。
- `allowed_tools` 与有损 custom grammar 只在账号 extra 明确启用时发送，不根据第三方 base URL 猜测。
- 原始载荷预检必须拒绝关键对象的重复 JSON key，把 `tool_choice.allowed_tools.tools` 与声明/动态工具计入同一资源预算，并拒绝超过 64 个字段的关键/part/嵌套 image URL 对象，或超过 16384 项的 input/content/summary part 数组；历史 identity 必须来自 replay cache，不能在消息转换阶段按 item 回扫全部工具，part 和上游 custom arguments 转换也不得把未知字段解码为通用 map。流式工具 arguments 必须线性累积并执行单调用 16 MiB / 单响应 32 MiB 上限；Responses 超限发 `response.failed`，Messages 超限发 Anthropic `event: error`，两者都停止读取且不生成不完整 done/completed/message_stop。fallback 内其他客户端 400 不得上报账号调度失败。

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
| 失败分类器、生产 fixture 与当前状态阈值 | `cd backend && go test ./internal/handler ./internal/service -run 'OpsFailure|ClassifyOpsCurrentFailureState|ClassificationUnknown' -count=1` |
| 分类迁移与共享 SQL 合同 | `cd backend && go test ./migrations ./internal/opssql -run 'FailureClassification|OpsFailure' -count=1` |
| 分类 v2 真实 PostgreSQL 回填、滚动兼容与索引计划 | `cd backend && go test -tags=integration ./internal/repository -run '^TestOpsFailureClassificationV2' -count=1` |
| Ops Repository、API 和聚合兼容 | `cd backend && go test ./internal/repository ./internal/handler/admin ./internal/service -count=1` |
| 通用弹窗栈 | `pnpm --dir frontend test:run src/components/common/__tests__/BaseDialog.spec.ts` |
| 通用 Select 外部点击 | `pnpm --dir frontend test:run src/components/common/__tests__/Select.spec.ts` |
| 运维请求/错误详情弹窗 | `pnpm --dir frontend test:run src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts` |
| 运维弹窗 composable | `pnpm --dir frontend test:run src/views/admin/ops/composables/__tests__/useOpsModalStack.spec.ts` |
| 前端类型、完整回归和生产构建 | `pnpm --dir frontend typecheck`、`pnpm --dir frontend test:run`、`pnpm --dir frontend build` |

发布前需要在同一绝对时间窗口人工对账：

- 全 v2 窗口满足 `customer_visible_failure_count = platform_sla_failure_count + sla_excluded_failure_count + classification_unknown_count`；混入 v1 时，unknown 作为数据质量覆盖层可与旧 SLA headline 重叠，此时改为核对终态 `failure_breakdown`（排除 `recovered_attempt`）之和等于客户可见失败；
- `error_count_total / business_limited_count / error_count_sla` 分别等于新的客户可见、SLA 排除和平台 SLA 兼容别名；
- `query_mode=raw` 与 `query_mode=preagg` 的 headline 计数一致，跨小时 head/preagg/tail 不重复；
- 每个归因数字打开的明细使用 overview 响应的 `start_time/end_time` 和相同结构化过滤条件；
- 最近 15 分钟当前状态按 `request_error_rate_percent_max` 判断，自定义历史窗口不显示“当前已恢复”；
- recovered upstream attempt 只进入 provider-health 明细，不进入客户可见失败或平台 SLA 终态计数。
- 模拟旧实例在迁移后晚写普通 v1 错误、非 recovered HTTP 200 流式终态和严格 recovered 尝试：前两者进入客户可见与未分类，recovered 不进入；hourly/daily 桶保持 v1，强制 preagg 返回未就绪而不是伪装成完整 v2。
- migration 192 对两种 cyber-policy、provider 403 余额、provider 4xx/5xx 的真实回填结果与 writer 分类一致；migration 193 的 customer-visible、SLA、v1 探针和 reason 查询在 PostgreSQL `EXPLAIN` 中命中目标索引。

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
