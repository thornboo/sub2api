# 企业成员模型感知路由实施测试规格

> 状态：实施门（2026-08-05 审核通过；阶段 0-4 的本地实现和最终本地验证已完成；v1.7.29 故障后部署默认回到 legacy，shadow / enforce 改为显式 opt-in；阶段 5 的默认切换和旧模式退役继续由真实发布窗口 gate 阻断）
> 上位方案：[企业成员模型感知路由与跨分组故障转移治理](../features/enterprise-member-model-aware-routing-governance.md)
> 适用范围：企业成员多分组 Key。普通 Key、单分组 Key、账号内 failover、计费与预算合同默认保持现状。

## 1. 目的与执行顺序

本规格把治理方案第 11、14、15 节转成可执行测试。实施必须遵守：

1. 先增加能在旧实现上稳定暴露问题的回归测试。
2. 再实现共享 routing revision、稳定资格规划、shadow/LKG 和 typed attempt。
3. 最后在 readiness、rollout 和 auto-stop 都满足时启用 `enforce_published`；Ops 路由链、alias review 和管理端证据必须先兼容落地。
4. 任一阶段不得以放宽成员授权、恢复全分组扫描或改变普通 Key 语义换取测试通过。

测试中的分组名、模型名和账号 ID 均使用匿名 fixture，不依赖生产数据。

## 2. 固定合同

### 2.1 admission mode

| 值 | 测试语义 |
| --- | --- |
| legacy_order_only | v1.7.29 故障后的当前安全默认；仍是临时恢复状态，不是新安装最终目标 |
| shadow_published | 新旧计划双算、实际仍执行旧计划；必须留下差异证据 |
| enforce_published | 只激活新规划器返回的 eligible 候选；服务端 readiness、rollout 和 auto-stop 未满足时必须拒绝写入或运行时降级为 shadow |

### 2.2 稳定原因码

测试只断言闭集原因码，不断言内部错误字符串：

- model_unpublished
- model_not_authorized
- model_unsupported
- endpoint_capability
- no_persistent_pool
- evaluation_failed
- routing_eligibility_unavailable

### 2.3 计划来源

- live
- last_known_good

只有完整成功的 live 计划可以写入或刷新 LKG；降级计划不能续期另一个降级计划。

### 2.4 当前 enforce readiness 安全门

在阶段 3 退出条件和生产灰度证据尚未同时满足时，以下合同必须持续成立：

- 管理 API 写入 `enforce_published` 返回 409 和稳定原因码 `ENFORCE_NOT_READY`，不得持久化；
- 数据库或部署配置中的历史 enforce 值不得绕过 API，运行时必须降级到 `shadow_published`，来源标记为 `enforce_blocked`；
- 管理端 enforce 选项禁用并显示服务端返回的阻断原因；前端确认框不是授权边界；
- readiness 必须由服务端提供，并包含 routing revision、evaluator coverage、alias audit、evidence pipeline、expansion evidence 和 auto-stop；解除阻断不得只删除前端 disabled 状态。

## 3. 阶段 0：四个生产形态 fixture

在 backend/internal/server/middleware/enterprise_member_group_test.go 增加统一 fixture builder，生成绑定顺序：

~~~text
deepseek → minimax → glm → mimo
~~~

每个组必须配置独立 active channel、定价/mapping 和稳定账号池，避免跨组共享假数据造成伪通过。

| 测试名 | 请求与配置 | 必须断言 |
| --- | --- | --- |
| TestEnterpriseMemberRoutePlanRejectsUnpublishedImageModelBeforeGroupActivation | /v1/responses，gpt-image-1.5，显式生图；所有授权组均未精确发布 | 404 model_not_found；0 次分组激活；响应不含 deepseek 或 “this group” |
| TestEnterpriseMemberRoutePlanKeepsOnlyPublishedGLMGroup | /v1/responses，glm-5.2；仅 glm 组精确发布且有稳定路由 | 只有 glm 候选；不得激活 mimo |
| TestEnterpriseMemberRoutePlanKeepsOnlyPublishedMiniMaxGroup | /v1/responses，minimax-m3；仅 minimax 组精确发布且有稳定路由 | 只有 minimax 候选；不得激活 mimo |
| TestEnterpriseMemberRoutePlanKeepsOnlyPublishedOpenAICompatibleGroup | /v1/chat/completions，gpt-5.6-terra；仅相关 OpenAI/Composite 路由发布 | 无关 deepseek/minimax/glm/mimo 不执行 |

阶段 0 还必须：

- 用“规划后的首个合格组胜出”替换 TestActivateEnterpriseMemberGroupForModelUsesFirstAuthorizedSnapshot 的旧目标。
- 保留 models_list_config 仅供展示的测试，防止把展示字段改成路由白名单。
- 保留普通 Key model miss 不触发企业跨组重试。
- 保留普通 Key 空 model_mapping 等于允许所有的现有语义。
- 锁定响应提交、预算结果不明、外部任务创建和 WebSocket 首 turn 后禁止重放。

## 4. 规划器单元测试

新建 backend/internal/service/enterprise_member_route_planner_test.go。

### 4.1 授权、排序与不可变性

- TestEnterpriseMemberRoutePlannerPreservesAuthorizedMemberOrder
- TestEnterpriseMemberRoutePlannerNeverAddsUnauthorizedGroup
- TestEnterpriseMemberRoutePlannerRejectsInactiveOrInvalidGroup
- TestEnterpriseMemberRoutePlannerDoesNotMutateAuthorizedGroupInput
- TestEnterpriseMemberRoutePlannerDecisionIsRequestScoped

规划器输入已经是当前成员授权集合；规划器只裁剪、绝不扩权；输出按成员绑定顺序稳定排序。

### 4.2 published-only

- TestEnterpriseMemberRoutePlannerExactMappingPublishesAlias
- TestEnterpriseMemberRoutePlannerExactPricingPublishesModel
- TestEnterpriseMemberRoutePlannerWildcardMappingRequiresExactPricedExpansion
- TestEnterpriseMemberRoutePlannerWildcardPricingDoesNotPublishArbitraryModel
- TestEnterpriseMemberRoutePlannerEmptyAccountMappingDoesNotPublishModel
- TestEnterpriseMemberRoutePlannerPassthroughAccountDoesNotPublishModel
- TestEnterpriseMemberRoutePlannerIgnoresModelsListConfig

公开身份只能来自精确 mapping 源或精确定价。展示配置、空账号 mapping、透传能力和 review 状态均不能独立发布任意模型。

### 4.3 稳定资格与动态容量隔离

- TestEnterpriseMemberRoutePlannerRequiresPersistentSchedulableAccount
- TestEnterpriseMemberRoutePlannerFiltersUnsupportedAccountMapping
- TestEnterpriseMemberRoutePlannerUsesChannelMappedModel
- TestEnterpriseMemberRoutePlannerUsesPerModelProtocolCapability
- TestEnterpriseMemberRoutePlannerDoesNotReadConcurrencyCooldownOrStickyState
- TestEnterpriseMemberRoutePlannerBatchesGroupsAccountsChannelsAndCapabilities

批量测试使用计数 repository stub，断言查询次数不随“分组 × 账号”乘积增长。

### 4.4 端点与 intent

- TestEnterpriseMemberRoutePlannerResponsesTextDoesNotRequireImagePermission
- TestEnterpriseMemberRoutePlannerPassiveImageNamespaceDoesNotRequireImagePermission
- TestEnterpriseMemberRoutePlannerExplicitImageIntentRequiresGroupPermission
- TestEnterpriseMemberRoutePlannerOpenAIImageIntentRequiresResponsesImageCapability
- TestEnterpriseMemberRoutePlannerMessagesRequiresDispatchCapability
- TestEnterpriseMemberRoutePlannerEmbeddingsRequiresCompatiblePlatform
- TestEnterpriseMemberRoutePlannerLiveRequiresCompatiblePlatformAndGroupFlag
- TestEnterpriseMemberRoutePlannerBatchImageRequiresBatchAndImageFlags
- TestEnterpriseMemberRoutePlannerVideoRequiresVideoCapableRoute

意图识别必须复用 IsExplicitImageGenerationIntent 等权威函数，不得按模型名前缀猜厂商或功能。

### 4.5 Composite

- TestCompositeRouteResolverPreviewIsSideEffectFree
- TestEnterpriseMemberModelDeliveryCompositePreviewUsesPerCandidateTarget
- TestEnterpriseMemberModelDeliveryCompositePreviewHonorsEndpointAndDependencyFailures
- TestEnterpriseMemberRoutePlannerUsesSideEffectFreeCompositePreview

上述测试锁定四个合同：预览不改写调用方上下文；同一公开别名在不同 Composite 候选中按各自配置重新解析；端点不匹配只裁剪当前候选；渠道/账号判断使用预览后的具体目标平台和上游模型。

同时保留依赖缺失测试：

- TestEnterpriseMemberModelDeliveryFailsClosedWhenCompositeEvaluatorIsUnavailable
- TestEnterpriseMemberRoutePlannerTreatsMissingCompositeEvaluatorAsDependencyFailure

这两个测试证明 Wire/测试装配缺少预览 evaluator 时会把它分类为资格依赖不可用，而不是错误的客户端 model_not_found 或任意兼容放行。仓储读取错误也必须走相同的 503 语义；显式未命中或端点不匹配则是当前候选的 `endpoint_capability`，不得污染下一候选。

### 4.6 企业严格投影与兼容合同

- TestEnterpriseMemberModelDeliveryUsesAuthorizedSnapshotsAndLegacyCompatibility
- TestEnterpriseMemberModelDeliverySupportsGrokCompatibility
- TestEnterpriseMemberModelDeliveryFailsOnCapabilityDependencyError
- TestEnterpriseMemberRoutePlannerUsesRealCompatibilityProjectionWhenNativeRoutingIsDisabled
- TestEnterpriseMemberRoutePlannerUsesRealGrokCompatibilityProjection

这些用例锁定三个边界：请求热路径只读取当前授权快照；全局 native routing 关闭不能误裁剪既有 OpenAI/Grok 网关兼容路径；资格依赖读取错误必须向上返回 503 语义。

## 5. middleware 与 admission mode 测试

继续使用 backend/internal/server/middleware/enterprise_member_group_test.go：

- shadow_published 下执行顺序仍等于 legacy，同时新旧差异进入请求级 shadow trace。
- enforce_published 下只有 eligible 候选写入 plan。
- 规划阶段 404/403/503 时 ActiveGroup、API Key GroupID 和 Ops fallback group 均未设置。
- 普通 Key 绕过企业规划器；fake planner 不得被调用。
- WebSocket 首帧得到模型后重新运行同一规划合同，不能恢复“首个授权组永远胜出”。
- WebSocket 首帧必须区分 no_eligible_group 与 eligibility_unavailable；后者使用临时不可用/可重连关闭语义，不能伪装成客户端 policy violation。
- planner 投影读取失败且无合格 LKG 时返回 503，不进入 legacy 全组扫描。

## 6. routing revision 与 LKG

新建：

- backend/internal/service/routing_eligibility_revision_test.go
- backend/internal/service/routing_eligibility_runtime_test.go
- backend/internal/service/enterprise_member_route_snapshot_test.go
- backend/internal/repository/routing_eligibility_revision_integration_test.go
- backend/migrations/routing_eligibility_revision_migration_test.go

### 6.1 revision 三层合同

配置版本必须区分：

1. 持久 revision：配置事实的权威版本。
2. 集群事件：跨实例快速传播；可复用现有 outbox/Pub/Sub 基础设施，但使用独立事件类型。
3. 本地 atomic mirror：请求热路径高可用读取。

Pub/Sub 不是唯一真相，TTL 是最后风险上限。不得直接把 per-key auth cache invalidation 消息兼作 eligibility revision。

### 6.2 LKG 单元测试

- TestRouteSnapshotOnlyStoresCompleteLivePlan
- TestRouteSnapshotRejectsCrossGroupOrIncompleteCandidates
- TestRouteSnapshotFallbackCannotRefreshSnapshot
- TestRouteSnapshotKeySeparatesModelEndpointIntentRevisionAndAlgorithm
- TestRouteSnapshotIntersectsCurrentAuthorizedGroups
- TestRouteSnapshotNeverRestoresRevokedGroup
- TestRouteSnapshotExpiresAtConfiguredTTL
- TestRouteSnapshotZeroTTLDisablesFallback
- TestRouteSnapshotRevisionMismatchFailsClosed
- TestRouteSnapshotExplicitInvalidationRemovesAffectedScopes
- TestRouteSnapshotExcludesKeyBodyCredentialsCapacityAndStickyState
- TestRoutingEligibilityRuntimeRefusesLKGAfterFullReconciliationBecomesStale
- TestRoutingEligibilityRuntimeThrottlesPublishedOutboxCleanup

### 6.3 多实例与故障注入

当前真实 PostgreSQL/Redis 集成测试位于 backend/internal/repository/routing_eligibility_revision_integration_test.go，已覆盖迁移执行、各稳定 writer scope、不依赖 API Key 枚举、事务回滚不遗留 revision/outbox、账号 mapping 与 privacy 触发 revision、瞬态 last-used/无关 extra 不触发 revision、repository 权威读取和 Redis Pub/Sub round-trip。以下多实例、重启与 outbox 故障场景已在本地 PostgreSQL 16/Redis Testcontainers 门中实现并通过；生产灰度仍须收集传播 SLO、重连和对账告警证据，不能用本地 PASS 代替：

- TestRoutingEligibilityRevisionPropagatesAcrossServiceInstances
- TestRoutingEligibilityRevisionPublishesForGroupWithoutAPIKeys
- TestRoutingEligibilityRevisionDoesNotDependOnAPIKeyEnumeration
- TestRoutingEligibilityRevisionSubscriberRecoversAfterRedisRestart
- TestRoutingEligibilityRevisionReconcilesMissedEvents
- TestRoutingEligibilityRevisionIgnoresDuplicateAndOlderEvents
- TestRoutingEligibilityRevisionDoesNotPublishRolledBackChange
- TestRoutingEligibilityRevisionOutboxRetriesAfterPublishFailure
- TestRoutingEligibilityRevisionOutboxStatusReportsPendingLag
- TestRoutingEligibilityRevisionRestartCannotMatchOldSharedSnapshot

本地故障注入必须持续证明，生产灰度还要以相同口径观测：

- 实例 A 提交渠道、分组、账号 mapping 或协议能力变更后，实例 B 在传播 SLO 内拒绝旧 revision。
- Pub/Sub 中断时，旧计划最多只在有界 TTL 内存在。
- Pub/Sub 事件不能续期 mirror 权威窗口；连续全量对账失败超过 TTL 后，即使仍有 live 流量也不能继续生成或恢复 LKG。
- 已观察到新 revision 但新投影失败时，不得使用旧 LKG。
- 当前授权读取失败时，即使存在匹配 LKG 也返回 503。
- 分组没有 API Key、或 Key 枚举失败时，group revision 仍正常发布。

### 6.4 writer coverage

以下入口必须各有“revision 恰当递增、作用域正确、失败语义明确”的测试：

- Channel Create/Update/Delete：mapping、pricing、group binding、status。
- Group Update/Delete：status、platform、图片/Live/Batch/Messages 等能力。
- Account Create/Update/Delete/Status：status、schedulable、group binding、credentials.model_mapping、协议/透传 extra 与 privacy_mode；last-used、cooldown 和无关 extra 不递增。
- Group/account 稳定投影：`require_oauth_only` 与 `require_privacy_set` 必须在规划阶段过滤不合格账号。
- ModelProtocolCapability UpdateOverrides、SyncCatalog。
- Composite 路由配置写入入口。

数据库触发器是当前 writer coverage 的权威汇聚点；静态 migration 测试守触发器清单，真实 PostgreSQL 测试守事务/字段语义。后续每新增资格字段或表，必须同时扩展触发器、静态清单和至少一个真实数据库行为测试。

当前可执行命令：

```bash
cd backend
go test ./internal/service ./migrations -run 'Test(RoutingEligibilityRuntime|RoutingEligibilityRevision|RouteSnapshot|EnterpriseMemberRoutePlannerRestores|EnterpriseMemberRoutePlannerRejectsLKG|EnterpriseMemberModelDeliveryAppliesStableGroupAccountPolicies|RoutingEligibilityRevisionMigration)' -count=1
DOCKER_HOST=unix:///Users/thornboo/.colima/default/docker.sock \
TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
go test -tags integration ./internal/repository \
  -run '^TestRoutingEligibility' \
  -count=1 -v
```

迁移验证补充：

```bash
cd backend
go test ./migrations -run 'Test(RoutingEligibilityRevisionMigration|EnterpriseMemberAliasReviewLedgerMigration|OpsRoutingAttemptsMigration|ModelProtocolCapabilitiesNonTextMigrationExtendsProtocolCheck)' -count=1
```

## 7. typed attempt 与安全重放

继续使用 backend/internal/server/middleware/enterprise_member_orchestrator_test.go。

### 7.1 允许切组的闭集

- capability_mismatch
- capacity_exhausted
- transient_upstream

每个原因还必须满足 SafeToReplay=true、响应未提交、预算和上游结果确定、未创建外部任务、客户端未取消。

### 7.2 永不自动切组

- 参数/schema 400。
- 身份、成员授权、余额、预算、quota、IP 和内容安全拒绝。
- 无 typed marker 的通用 500。
- Header flush、首 SSE data、首业务字节。
- WebSocket 首 turn 已提交。
- 图片、视频、Batch 外部任务已创建。
- 上游写入结果不明、预算结果不明、客户端取消。

### 7.3 状态恢复

- 请求 body、Content-Length、context 和 Gin keys 恢复。
- 保留历史 routing attempts 和 upstream errors。
- 清除上一候选的 retry marker、Composite decision、目标平台、映射模型和 sticky 预取。
- 每个组最多尝试一次，不产生 fallback 环。
- 渠道定价限制、利润门、隐私、代理和 sticky 语义在首次与重试路径一致。
- 普通 Key 遇到相同图片能力门仍保留原有 403，不进入企业跨组 orchestrator。

## 8. 聚合错误、Ops 与隐私

| 内部事实 | HTTP | 对客类型 | Ops reason |
| --- | --- | --- | --- |
| 全站未发布模型 | 404 | model_not_found | model_not_found |
| 站点存在但授权范围未发布 | 404 | model_not_found | model_not_authorized |
| 已发布但端点/图片能力不允许 | 403 | permission_error | endpoint_not_allowed |
| 已发布但无持久账号池 | 503 | api_error | no_available_accounts |
| 资格投影失败且无合格 LKG | 503 | api_error | routing_eligibility_unavailable |

对客 body 不得包含其他分组名、账号、上游模型、channel ID 或候选数量。

同时覆盖：

- usage 最终 group_id 是实际执行分组。
- routing attempts 包含规划裁剪和实际 attempt，但不含 Key、请求正文或凭据。
- shadow/LKG schedule_meta 只进入管理员 DTO。
- enterprise owner、成员 usage、Key 自助查询和 CSV 不暴露内部拓扑。
- 旧 Ops 行 routing_attempts 为空时继续兼容展示。
- `ops_error_logs.routing_attempts` 必须是 JSON array；`routing_plan_source`、`routing_snapshot_age_ms`、`usage_logs.route_plan_source` 和 `route_plan_snapshot_age_ms` 只记录低基数来源与非负年龄。

当前可执行命令：

```bash
cd backend
go test ./internal/service ./internal/handler ./internal/repository ./migrations -run 'Test(OpsRouting|GroupAttempt|ApplyOpsRouting|OpsRoutingAttemptsMigration|UsageLog)' -count=1
```

## 9. alias 迁移与污染防护

- shadow 观察不能自动写入 channel mapping、pricing 或白名单。
- review ledger 状态不能改变规划器结果。
- registered 只有真实配置和稳定交付投影通过后才可写入。
- 7d/30d 聚合限制高基数；Prometheus label 不包含原始模型名。
- 控制字符、超长模型名、扫描流量和大小写漂移均受边界保护。
- enforce 门要求近期 legacy_success_new_pruned 没有未审核活跃项。

当前可执行命令：

```bash
cd backend
go test ./internal/service ./internal/handler/admin ./internal/repository ./migrations -run 'Test(EnterpriseMemberAliasReview|AliasReview|EnterpriseMemberAliasReviewLedgerMigration)' -count=1
```

## 10. 阶段验证命令

### 阶段 0-2

~~~bash
cd backend
go test ./internal/server/middleware -run 'Test(EnterpriseMemberRoute|ResolveEnterpriseMemberGroup|OrchestrateEnterpriseMemberGroups)' -count=1
go test ./internal/service -run 'Test(EnterpriseMemberRoute|RouteSnapshot|RoutingEligibilityRevision|ModelDelivery)' -count=1
go test ./internal/handler ./internal/server/routes -run 'EnterpriseMember|ImageGeneration|Composite' -count=1
go test ./internal/handler/admin ./internal/service -run 'Test(SettingHandler_UpdateSettings_RejectsEnterpriseMemberEnforceBeforeReadiness|BuildSystemSettingsUpdates.*EnterpriseMember|EnterpriseMemberModelAdmission)' -count=1
~~~

### 数据、前端与文档

~~~bash
cd backend
go test ./migrations ./internal/repository ./internal/handler -run 'RoutingAttempt|EnterpriseMemberModelAdmission|AliasReview|ModelProtocolCapabilitiesNonText' -count=1
cd ..
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check
pnpm --dir frontend test:run
pnpm --dir docs-site docs:build
git diff --check
~~~

需要 Docker/Testcontainers 时沿用 verification-matrix.md 的 Colima 环境变量。未实际运行的集成测试必须标为未验证，不能用单元测试代替。

### 最终本地验证证据（2026-08-06）

- 后端：`go generate`、`make test-unit`、`go test ./... -count=1`、`go vet ./...`、`golangci-lint run ./...`（0 issues）通过。
- 集成：18 个 Colima/Testcontainers PostgreSQL 16/Redis 顶层测试通过：14 个 `TestRoutingEligibility*` 场景覆盖 revision/outbox 事务边界、跨实例传播、Redis 重启恢复、发布重试与 pending lag，4 个 `TestMigrationsRunner*` 场景覆盖迁移串行化、幂等执行和 schema 对齐。
- 并发：focused race 验证通过。
- 前端：typecheck、lint、完整测试（259 个测试文件、1758 条测试）、build 通过。
- 文档与仓库卫生：docs build、`git diff --check`、隐私/脱敏专项测试及新增差异的高置信度密钥模式扫描通过；未把环境中未安装的 gitleaks 等专用扫描器计作已执行证据。
- 仍未执行：commit、release、deploy、生产 7d/30d alias/canary release-window 证据收集、默认 enforce flip、shadow 双算删除。
- 生产迁移剩余门：`201` 的 CHECK 约束使用 `NOT VALID` 避免启动期历史全表扫描；发布后必须先审计历史不合规行，再在低峰窗口执行 `VALIDATE CONSTRAINT`。`201a` 的并发索引还必须在生产同量级副本上演练耗时、磁盘峰值和失败重试。

## 11. 阶段退出门

| 阶段 | 必须满足 |
| --- | --- |
| 0 | 四个 fixture 暴露旧缺陷；普通 Key、展示配置和不可重放不变量继续通过 |
| 1 | shadow 不改变实际路由；新计划解释四个 fixture；严格投影不扫描全站分组；revision/LKG 故障测试通过；无 N+1 |
| 2 | typed attempt 闭集和所有不可重放路径通过；图片能力竞态可安全切到下一合格候选 |
| 3 | 服务端 readiness 全部前置条件通过；rollout policy 只覆盖目标企业 / 成员 / 稳定哈希范围；auto-stop clear；未发布模型 0 次分组尝试；alias 迁移门满足；合法模型成功率无回归 |
| 4 | 单条 Ops 详情可还原计划、裁剪、attempt 和终态；alias review 管理入口可审计 `legacy_success_new_pruned`；旧行兼容 |
| 5 | 生产全量 enforce 稳定至少一个真实发布窗口；新安装默认 enforce；legacy 有明确退役期限；shadow 双算和旧候选分支在独立发布计划中删除 |

## 12. 完成报告

交付报告必须包含：

- 实际改动文件与数据迁移。
- admission mode 最终默认值和回滚方式。
- 四个生产形态 fixture 的执行证据。
- 普通 Key、自定义精确 alias、透传账号和协议兼容回归证据。
- 多实例 revision/Pub/Sub/LKG 故障注入证据。
- 目标测试、相关全量测试、前端检查和文档构建结果。
- 未运行项、残余风险、灰度指标和自动停止条件。
- 明确声明未执行生产部署，除非用户另行授权。
