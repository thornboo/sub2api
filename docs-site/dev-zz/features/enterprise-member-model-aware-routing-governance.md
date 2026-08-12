# 企业成员模型感知路由与跨分组故障转移治理

- **日期**：2026-08-05
- **状态**：方案已审核；阶段 0-4 的本地实现和最终本地验证已完成；阶段 5 的本地退役准备已实现，但默认值切到 `enforce_published`、删除 shadow 双算和生产旧模式退役仍必须等真实生产发布窗口验证后执行
- **适用范围**：`api_keys.member_id IS NOT NULL` 的企业成员 Key；普通单分组 Key 保持现有行为
- **触发事件**：生产错误 `#291465`、`#291472`、`#291473`、`#291493`
- **第二轮审查修订**：补充精确最近有效资格快照降级、alias 7d/30d 审计清单、review ledger 和 enforce 迁移门；仍禁止规划故障时恢复全授权分组扫描
- **实施前补强**：明确 routing revision 的持久版本、集群事件和本地 atomic mirror 三层合同；可执行测试见 [企业成员模型感知路由实施测试规格](../testing/enterprise-member-model-aware-routing-test-spec.md)
- **上位合同**：[企业用户成员管理](./enterprise-member-management.md) 第 7 章、[模型级多协议能力与交付路由](./model-multi-protocol-capabilities-and-routing.md)、[运维失败归因、SLA 与事件视图重构](./ops-failure-classification-redesign.md)

本文记录 2026-08-05 企业成员多分组路由异常的事实、根因、风险与推荐修复方案。目标不是修复四个模型名，而是建立一套长期可维护的模型准入、候选筛选、跨组故障转移和运维归因合同。

### 当前本地实施快照（2026-08-06）

本快照是可审核的本地实现状态。它说明阶段 0-4 的代码与最终本地验证已经在当前工作区完成，但不构成 commit、release、deploy、生产灰度、生产默认值切换或 shadow 分支删除的证据。

已完成：

- 企业成员专用的精确发布模型规划器，按当前成员授权顺序只裁剪、不扩权；
- `legacy_order_only`、`shadow_published`、`enforce_published` 三态配置语法已经建立，当前有效默认仍为 `shadow_published`；服务端 readiness 是 enforce 的权威开关，未满足前置条件时，API 写入 enforce 返回 409，历史数据库值或配置值也会安全降级为 shadow 并标记 `enforce_blocked`；
- readiness 已从固定硬阻断扩展为服务端可计算条件集合：routing revision mirror、非文本 evaluator coverage、alias audit、shadow/canary/evidence pipeline 和 auto-stop 都进入同一权威 gate；前端只展示该 gate，不能自行放行；
- rollout policy 已接入企业 owner ID、成员 ID、稳定哈希百分比、salt 和手动 auto-stop；扩大 enforce 覆盖必须通过 readiness 与 auto-stop 检查，缩小范围和停止可以继续保存；raw/normalized 列表、总目标数、salt 和 JSON 大小均有服务端上限，管理设置更新请求在 JSON 解码前另受 4 MiB 请求体上限保护；
- `/v1/chat/completions`、`/v1/responses`、`/v1/messages` 及已注册同协议别名的请求前规划，WebSocket 在首个 `response.create` 帧按同一合同重规划；
- shadow 保留旧执行顺序并记录低基数差异指标；企业规划只读取当前请求已授权且精确发布的分组快照，不再为一次请求枚举全部 active groups；未来 enforce 只激活规划器返回的候选，资格依赖失败时返回平台 503，不恢复全授权分组扫描；
- OpenAI Chat/Responses 兼容路由不再错误依赖全局 native routing 开关，Grok 按现有网关协议合同参与稳定资格判断；协议能力仓库失败在企业严格投影中是规划依赖错误，不再被吞成候选 403/404；
- Composite 已接入与运行时同源的无副作用路由预览：对 `/v1/chat/completions`、`/v1/responses`、`/v1/messages` 按候选分组和精确端点独立解析目标平台/上游模型，再只在该分组、该目标平台的稳定账号池内判断资格；预览不会安装或覆盖请求上下文中的运行时 decision，缺少 evaluator 或读取失败仍按资格依赖错误处理；
- routing eligibility 三层合同已接入运行时代码：PostgreSQL `routing_eligibility_revisions` 是持久权威，独立 `routing_eligibility_outbox` 与 Redis Pub/Sub 负责快速传播，各实例启动先对账并维护 atomic mirror，之后每 30 秒补偿漏事件；Channel、Group、Account、ModelProtocolCapability、Composite 的资格相关表通过数据库触发器在配置事务内同时推进精确 scope 与全局安全 scope，避免某条写路径漏发 revision；
- routing eligibility outbox 的运维状态通过运行时 `OutboxStatus` 暴露 `pending_count`、`oldest_pending_at` 和 `oldest_pending_age`；发布失败只保留 pending 行等待重试，清理任务只删除超过保留期的已发布行，禁止为了降低 pending 数量而删除未发布事件。生产监控使用 `SELECT COUNT(*) AS pending_count, MIN(created_at) AS oldest_pending_at, NOW() - MIN(created_at) AS oldest_pending_age FROM routing_eligibility_outbox WHERE published_at IS NULL`；连续两个发布周期仍有 pending 或 oldest age 超过 30 秒应告警，超过 120 秒的 mirror/LKG 权威窗口必须按资格传播故障处置。回滚操作应先关闭/回退新写入路径或 Redis 发布通道变更，再观察 pending lag 是否停止增长；若 pending 持续增长，按 `routing_eligibility_revisions` 权威表重新对账实例，不对 `routing_eligibility_outbox WHERE published_at IS NULL` 执行手工 purge。
- 有界进程内 LKG 已接入三类文本协议规划器，默认 TTL 120 秒：只保存单分组、完整 live 合格候选，key 必须同时包含 channel/account/protocol/composite 全局 generation 与精确 group generation，并区分模型、端点、intent 和算法版本；资格投影失败时只读取启动已对账且仍在 120 秒权威新鲜度窗口内的 mirror，再与当前已确认授权求交集。新 revision 一经 Pub/Sub 或定期对账观察即主动清除旧快照；启动从未完成对账或最后一次全量对账已超出 TTL 时均 fail-closed，live 计划也不能用陈旧 mirror 刷新快照；
- 集群 revision 同时驱动跨实例渠道发布缓存与协议能力短缓存失效；稳定投影会应用分组 `require_oauth_only` / `require_privacy_set`，账号 `privacy_mode` 变化推进资格 revision，而 `last_used_at`、rate-limit/cooldown 和无关 extra 等瞬态写入不推进。本地真实 PostgreSQL/Redis 集成测试已验证迁移可执行、配置事实与 revision/outbox 同事务生成、稳定/瞬态账号字段边界，以及 Pub/Sub round-trip；
- typed group attempt、闭集重试原因，以及响应已提交、预算/上游结果不明、外部任务已创建、WebSocket 首 turn 已提交和客户端取消后的禁止重放门；
- WebSocket 首帧规划区分“无合格候选”和“资格依赖不可用”，后者使用可重连的临时不可用关闭码，不再伪装成客户端模型违规；
- Responses、OpenAI Images、异步图片任务、OpenAI Embeddings、OpenAI Live、Batch Images、Grok Video 和 Gemini Native 入口已接入受控 typed local gate 或 evaluator coverage；标记只有在当前请求已实际应用 `enforce_published` 计划且满足可重放条件时才可触发，shadow、legacy、普通 Key 和尚未纳入端点前规划的入口保持既有终止语义；
- `account_model_protocol_capabilities` 的协议约束已扩展到 `openai_embeddings`、`openai_images`、`openai_live`、`batch_images`、`grok_video`、`gemini_native`，使非文本端点能够进入同一稳定 evaluator coverage；
- Ops 路由证据已持久化：`ops_error_logs.routing_attempts` 保存 bounded JSONB attempts，`routing_plan_source` / `routing_snapshot_age_ms` 记录终态计划来源；`usage_logs.route_plan_source` / `route_plan_snapshot_age_ms` 保存成功请求的管理员用计划来源，不向企业 owner / Key 自助面暴露内部候选拓扑；
- alias review ledger 已落地：shadow 观察形成 `legacy_success_new_pruned` 7d/30d 清单，`registered` 必须复核真实精确 mapping/pricing 和稳定交付投影；review 状态本身仍不是运行时准入源；
- 管理端三态选择、配置来源提示、无效来源告警、readiness 条件、rollout policy、auto-stop 状态、alias review 清单和 legacy retirement target 已接入；readiness 未满足时 enforce 选项禁用并展示稳定阻断原因，未来 readiness 放行后仍保留危险确认。

最终本地验证证据（2026-08-06）：

- 后端：`go generate`、`make test-unit`、`go test ./... -count=1`、`go vet ./...`、`golangci-lint run ./...`（0 issues）通过；
- 集成：18 个 Colima/Testcontainers PostgreSQL 16/Redis 顶层测试通过，其中 14 个 `TestRoutingEligibility*` 场景覆盖 revision/outbox 事务边界、跨实例传播、Redis 重启恢复、发布重试与 pending lag，另有 4 个 `TestMigrationsRunner*` 场景覆盖迁移串行化、幂等执行和 schema 对齐；
- 并发：focused race 验证通过；
- 前端：typecheck、lint、完整测试（259 个测试文件、1758 条测试）、build 通过；
- 文档与仓库卫生：docs build、`git diff --check`、隐私/脱敏专项测试及新增差异的高置信度密钥模式扫描通过；未把环境中未安装的 gitleaks 等专用扫描器计作已执行证据。

尚未完成，不得据此宣称全量治理已经生产交付：

- commit、真实发布流水线、生产配置变更、生产灰度和生产部署尚未执行；本轮证据只覆盖本地工作区；
- 当前新安装默认仍是 `shadow_published`，`DefaultEnterpriseMemberModelAdmissionModeForNewInstall()` 仍由 `phase5_production_gate_pending` 阻止默认 enforce；不得把本地退役准备误读为已经默认 enforce；
- shadow 双算和旧候选分支仍必须保留到全量 enforce 至少稳定一个真实发布窗口后，才能在独立发布计划中删除；
- alias 迁移、auto-stop 和 evidence pipeline 已具备本地代码能力，但生产 7d/30d 数据、canary/release-window 对照、合法模型成功率、LKG 使用后成功率和完整 Ops/usage 观测仍需要生产窗口验证。
- `201` 新增 CHECK 约束以 `NOT VALID` 安装，避免启动迁移扫描历史大表；生产发布后必须先只读审计历史行，再在低峰维护窗口分别执行 `VALIDATE CONSTRAINT`。在验证完成前，不得把“新写入已受约束”误报为“历史数据已全量验证”。

本地迁移面：

| 迁移 | 当前作用 | 生产状态边界 |
| --- | --- | --- |
| `199_routing_eligibility_revision.sql` | 新增 `routing_eligibility_revisions`、`routing_eligibility_outbox`、revision sequence、writer coverage 触发器和 pending/published 索引 | 本地迁移与测试已存在；生产是否已应用只能由发布/数据库证据证明 |
| `200_enterprise_member_alias_review_ledger.sql` | 新增只作管理员处置证据的 alias review ledger，状态闭集为 `pending`、`registered`、`rejected_invalid`、`obsolete`、`needs_owner_action` | ledger 不参与运行时准入；生产清单必须等 shadow 数据生成 |
| `201_ops_routing_attempts.sql` | 给 Ops 错误与成功 usage 增加路由计划来源、LKG 年龄和 bounded routing attempts 证据字段；CHECK 约束以 `NOT VALID` 安装 | 字段为兼容新增；旧行为空数组 / null 时继续按旧终态字段展示；历史行约束需在生产低峰期单独验证 |
| `201a_ops_routing_attempts_indexes_notx.sql` | 以迁移运行器 `_notx` 模式并发建立 Ops/usage 路由证据索引；运行器在重试前清理同名 invalid index | 不进入普通事务，避免 `CREATE INDEX` 在启动迁移中长时间锁表；仍需在生产演练中确认实际耗时、磁盘余量和中断恢复 |
| `202_account_model_protocol_capabilities_non_text.sql` | 扩展协议能力约束到 Embeddings、Images、Live、Batch Images、Grok Video、Gemini Native | 只放开能力表协议枚举，不等于对应上游生产灰度完成 |

---

## 1. 结论

本次问题不是 `mimo` 或 `deepseek` 分组主动识别并接管了对应模型，而是企业成员路由当前采用了“**授权分组按绑定顺序逐个尝试**”的策略：

1. 候选计划只检查成员授权、分组状态、端点平台和少量功能开关。
2. 请求模型虽然被解析并写入上下文，但没有参与候选分组资格判断。
3. 每个候选进入具体 handler 后，才由分组内账号调度器判断模型、渠道定价和账号能力。
4. 当前分组返回显式可重试原因时，外层编排器继续激活下一绑定分组。
5. 全部候选耗尽时，运维错误记录只展示最后一个 ActiveGroup，因此看起来像“模型被路由到了 `mimo`”。

所以：

- `mimo` 是三个文本模型请求的**最后失败分组**，不是模型分类器选中的目标分组。
- `gpt-image-1.5` 先经过其它候选，最终进入名为 `deepseek` 的 OpenAI 兼容分组；该分组关闭了图片生成，handler 返回 403 后没有产生跨组重试标记，因此请求在那里终止。
- 四个样本都没有成功转发到错误厂商，但现有结构存在更严重的潜在风险：如果某个后续无关分组含有“空 `model_mapping`”或透传账号，并且渠道限制和功能门都放行，未知模型可能真的被转发给无关上游。

正式修复不能依赖调整分组顺序、维护模型名前缀规则、修改展示模型列表或把所有 403/404 都改成可重试。推荐方案是：

> 对企业成员多分组 Key 实施“明确发布模型准入”，使用调度层共享的稳定路由能力批量筛选候选，只在仍可能成功的候选之间保留成员配置顺序；所有分组尝试必须产生结构化结果和可审计链路。

---

## 2. 生产事实与复现证据

### 2.1 取证边界

本次仅执行只读检查，没有修改生产容器、数据库、配置或分组绑定。

取证时生产应用为：

| 项目 | 值 |
| --- | --- |
| 应用版本 | `1.7.28` |
| 生产 revision | `c165a733b5d528e67ab5c6a875992135f0b32ce3` |
| 容器状态 | healthy，重启次数 0 |
| 本地分析 HEAD | `6b92daf427bc` |

与根因直接相关的企业成员 middleware、orchestrator、无账号分类器、OpenAI Responses handler 和 Ops 分类代码，在本地 HEAD 与生产 revision 之间没有内容差异。因此下文代码分析可以解释当前生产行为。

文档刻意不记录用户邮箱、成员姓名、完整 Key、Key 前缀、上游凭据或临时服务器凭据。

### 2.2 四个终态错误

| 错误 | 模型 | 入站端点 | 运维页终态分组 | 终态响应 | 已核对的实际候选链 |
| --- | --- | --- | --- | --- | --- |
| `#291465` | `gpt-image-1.5` | `/v1/responses` | `deepseek` | 403 `Image generation is not enabled for this group` | `2 → 3 → 15(deepseek)` |
| `#291472` | `minimax-m3` | `/v1/responses` | `mimo` | 404 `model_not_found` | `2 → 3 → 15 → 16(minimax) → 17 → 18 → 19 → 20(mimo)` |
| `#291473` | `glm-5.2` | `/v1/responses` | `mimo` | 404 `model_not_found` | `2 → 3 → 15 → 16 → 17(glm) → 18 → 19 → 20(mimo)` |
| `#291493` | `gpt-5.6-terra` | `/v1/chat/completions` | `mimo` | 404 `model_not_found` | `2 → 3(openai) → 15 → 16 → 17 → 18 → 19 → 20(mimo)` |

四条记录来自同一企业成员 Key。成员绑定顺序固定为：

```text
anthropic → openai → deepseek → minimax → glm → kimi → qwen → mimo
```

这与三个 404 请求的实际候选顺序完全一致。

### 2.3 关键中间结果

- `gpt-image-1.5`
  - OpenAI 图片分组在账号选择阶段被渠道模型/定价限制拦截，没有可用账号。
  - 编排器继续到 `deepseek` 分组。
  - `deepseek` 分组属于 OpenAI 兼容 handler 路径，但 `allow_image_generation=false`。
  - 图片权限门直接返回 403，没有调用 `MarkOpsGroupRetry`，所以外层编排停止。
- `minimax-m3`
  - 只有 `minimax` 分组进入了非“渠道模型/定价限制”的无账号结果。
  - 其它大部分分组因模型未发布/定价限制退出，但仍被逐个激活。
  - 最后一个 `mimo` 分组产生对客 404，因此运维页只显示 `mimo`。
- `glm-5.2`
  - 与上条相同，真正相关的 `glm` 分组出现在中间；失败后仍继续到 `mimo`。
- `gpt-5.6-terra`
  - 真正相关的 OpenAI 分组在前部失败；其后无关分组继续被逐个尝试，最后由 `mimo` 返回 404。

### 2.4 本地回归基线

分析期间执行了现有企业成员路由定向测试：

```bash
cd backend
go test ./internal/server/middleware \
  -run 'TestResolveEnterpriseMemberGroup|TestEnterpriseMemberGroupEligible|TestActivateEnterpriseMemberGroupForModel|TestOrchestrateEnterpriseMemberGroups' \
  -count=1
```

结果：通过。这个结果说明当前行为受现有测试保护，不是偶发进程故障；修复必须先更新行为合同和回归用例，不能只改一处分支条件。

---

## 3. 当前数据流与根因

### 3.1 当前数据流

```text
API Key 鉴权
  → 读取企业成员的有序绑定分组
  → 解析 requested model
  → 只按授权/状态/端点粗粒度能力生成候选
  → 激活候选 1
      → Composite/平台 handler 分派
      → 渠道模型限制与映射
      → 分组内账号调度
      → 失败分类
  → 若写入了显式 OpsGroupRetryReason 且响应未提交
      → 激活候选 2
      → ……
  → 返回最后一次尝试的响应
  → Ops 终态记录读取最后一个 ActiveGroup
```

### 3.2 根因一：候选资格函数丢弃了模型

`backend/internal/server/middleware/enterprise_member_group.go` 中：

- `ResolveEnterpriseMemberGroup` 已经解析 `requestedModel`。
- `enterpriseMemberGroupEligible` 的模型参数却使用 `_ string`，完全不参与判断。
- `POST /v1/responses` 没有统一的文本/图片意图和目标平台资格规则。
- 候选数组按成员绑定顺序建立，并立即激活第一个候选。

现有测试还显式确认：即使请求模型不在 `models_list_config`，首个授权分组仍然胜出。这条测试对应当前实现，但不符合企业成员完整设计第 7.3 节已经定义的目标合同。

### 3.3 根因二：稳定能力判断发生得太晚

当前模型是否可调度，要到具体 handler 内部的账号选择阶段才判断。失败后虽然可以切组，但这形成了“先执行、再发现不相关”的 shotgun routing：

```text
所有授权分组
  ≈ 所有候选分组
  ≠ 对当前模型和端点可能成功的候选分组
```

系统已经有可复用的稳定事实：

- 渠道模型映射和定价发布；
- 渠道模型限制；
- 分组与账号关系；
- active + schedulable 账号；
- 账号 `model_mapping`；
- OpenAI 逐模型协议能力与 `DeliveryDecision`；
- Composite 公开模型到目标平台/模型的解析。

但这些事实尚未在企业成员候选计划阶段汇总为统一判断。

### 3.4 根因三：图片能力门是终态本地 403，不是候选能力结果

`backend/internal/handler/openai_gateway_handler.go` 的 Responses 路径会识别显式图片意图，然后检查 `GroupAllowsImageGeneration`。失败时直接写 403 并返回。

对普通单分组 Key，这个响应是合理的。对企业成员多分组 Key，当前 ActiveGroup 只是一个候选；如果其它授权分组可能支持图片，这应当被视为当前候选的 `capability_mismatch`。现有 Live 路由已经在类似本地能力门上显式调用 `MarkOpsGroupRetry`，图片路径没有采用同一合同。

### 3.5 根因四：站点模型目录和运行时准入没有统一

当前存在几个容易混淆但语义不同的配置：

| 配置/事实 | 当前语义 | 能否作为企业成员运行时准入 |
| --- | --- | --- |
| `models_list_config` | 可选的 `/v1/models` 展示列表 | **不能**，它是展示配置 |
| 渠道模型映射/定价 | 公开模型身份、映射和商品发布 | **可以作为公开模型准入事实** |
| `channel.restrict_models` | 当前分组调度时只允许定价列表模型 | 可以参与，但不能单独承担跨组规划 |
| 账号 `model_mapping` | 请求模型到上游模型的账号级资格与映射 | 可以参与稳定路由判断 |
| 空 `model_mapping` | 现有合同为允许所有模型 | 只能证明账号愿意透传，不能证明站点公开发布该模型 |
| OpenAI 透传账号 | 现有合同为允许所有模型 | 同上，不能替代站点公开模型准入 |
| `supported_model_scopes` | Antigravity 产品/订阅范围 | 不能推广成通用模型白名单 |

`Account.IsModelSupported` 明确规定：空 `model_mapping` 返回 true，OpenAI 透传也返回 true。这是普通 Key 和自定义上游兼容能力，不能直接删除；但对于会跨多个异构分组的企业成员 Key，如果没有先做“公开模型准入”，它会放大为跨厂商误投风险。

### 3.6 根因五：运维页把“最后分组”显示成“路由归属”

当前 `ops_error_logs.group_id` 来自终态 ActiveGroup。中间账号上游失败可以进入 `upstream_errors`，其中已经包含 `group_id` 和 `group_attempt`；但以下事实没有完整结构化落库：

- 初始授权候选数；
- 哪些分组在执行前被模型、端点或功能能力排除；
- 哪些分组因渠道模型未发布退出；
- 哪些分组执行过但没有发生上游调用；
- 为什么最后一个分组只是“候选耗尽位置”，而不是模型归属。

因此运维页的“分组”字段在多分组失败中容易被误读。

### 3.7 根因六：本地 4xx 的失败分类缺少明确 marker

图片能力门的 403 没有写入企业能力/本地能力 marker。Ops v2 分类器最终落入“剩余 4xx = client invalid_request”的兜底，因此截图显示：

- 失败域：客户端；
- 失败类别：请求协议；
- 原因：`invalid_request`；
- 处理方：客户端；
- 不计入 SLA。

这不是客户端 schema 错误，而是当前候选分组能力不足。分类结果掩盖了真正的路由问题。

---

## 4. 严重性与风险边界

### 4.1 已确认影响

- 用户收到与实际请求意图无关的终态分组名称和错误。
- 404 请求会按成员分组数放大本地调度、数据库诊断和日志量。
- 真正相关的中间分组失败被最后一个无关分组覆盖，排障方向错误。
- 图片能力失败被归为客户端协议错误，责任方和 SLA 口径不准确。
- 对客错误消息暴露了“某个候选分组关闭图片”这一内部实现细节，而没有表达整个授权范围的结果。

### 4.2 结构性风险

本次四个样本没有成功调用错误厂商，但不能据此判断系统安全。以下条件同时出现时可能产生真实误投：

1. 企业成员绑定了多个异构分组；
2. 请求模型未在站点公开模型目录中明确发布；
3. 某个后续无关分组关闭渠道模型限制，或关联空 `model_mapping` / 透传账号；
4. 端点 handler 能兼容该分组平台；
5. 上游接受未知模型名、内部别名或进一步代理请求。

风险包括：

- 请求被发送到错误供应商；
- 错误的数据地域、隐私或合规边界；
- 意外价格与计费模型；
- 工具、图片或结构化输出语义降级；
- sticky session 绑定到错误账号；
- 重试扩大外部副作用。

### 4.3 风险定级

建议定为 **高优先级路由正确性缺陷**：当前样本主要表现为错误终态归因和失败放大，但潜在后果涉及真实跨供应商投递，不能只当作错误文案问题。

---

## 5. 修复目标、不变量与非目标

### 5.1 修复目标

1. 未被站点明确发布的模型，不得通过企业成员多分组 Key 横扫所有授权分组。
2. 候选计划只包含对当前公开模型、端点、协议和功能意图存在稳定成功可能的分组。
3. 动态容量不足与稳定能力不支持必须分离：前者允许在安全边界内换组，后者执行前排除。
4. 成员配置顺序只决定**合格候选之间**的优先级，不再代表所有绑定分组都应尝试。
5. 本地能力门、账号调度失败和上游失败使用统一 typed attempt result。
6. Ops 能显示完整而脱敏的候选/尝试链，并正确区分模型不存在、成员未授权、平台容量和上游故障。
7. 普通单分组 Key、自定义透传账号、现有模型映射和协议转换保持兼容。

### 5.2 必须保持的不变量

- 企业成员绑定仍是权限边界；规划器不能增加成员未授权分组。
- 合格候选仍按 `sort_order ASC, group_id ASC` 排序。
- 同一逻辑请求中每个分组最多执行一次。
- 分组内调度器仍是实时账号选择、容量、冷却、并发、sticky 和 failover 的最终权威。
- 规划器只能缩小候选集，不能因为预判结果绕过渠道模型/定价限制、利润控制、隐私、OAuth、订阅或账号能力检查。
- 代理 fail-open 只能放宽代理隔离偏好，不能跳过渠道定价限制或 sticky 合同。
- 响应已提交、外部任务已创建、上游结果不明、预算结果不明或客户端取消后，不得跨分组重放。
- Composite 路由必须按每个候选重新解析；切组时清除上一候选的目标平台、上游模型和映射决策。
- `models_list_config` 继续只控制展示，不能升级为运行时安全策略。
- 对客 DTO 和日志不得暴露完整 Key、上游凭据、账号成本或不必要的内部拓扑。

### 5.3 非目标

- 不用模型名前缀推断厂商，例如 `glm-* → glm`、`minimax-* → minimax`。
- 不把企业成员改回“一把 Key 一个固定分组”。
- 不用本次修复重写所有账号调度器。
- 不改变普通 Key 的空 `model_mapping = 允许所有` 兼容语义。
- 不在第一阶段改变计费、预算、usage 原子结算或异步任务归属合同。
- 不把所有 4xx/5xx 状态码粗暴定义为跨组可重试。

---

## 6. 权威数据源

修复后每一层只回答一个问题，避免再次形成多份互相漂移的“模型真相”。

| 层 | 回答的问题 | 权威来源 |
| --- | --- | --- |
| 成员授权 | 这个成员能否访问该分组？ | `enterprise_member_group_bindings`、企业/成员/Key 状态、用户可绑定规则 |
| 公开模型准入 | 这个公共模型是否由站点在此分组明确发布？ | active channel 的精确模型映射源或模型定价条目 |
| 模型映射 | 此分组和账号最终会收到什么模型？ | 渠道映射 → Composite/协议映射 → 账号映射 → 平台规范化 |
| 稳定路由 | 忽略瞬时容量后，是否至少有一个持久可调度账号支持最终模型和协议？ | active + schedulable 账号、分组关系、`Account.IsModelSupported`、逐模型协议能力 |
| 端点/功能 | 当前端点、图片/视频/Live/Batch/Messages 能否在该分组保真执行？ | 分组功能开关、目标平台、`DeliveryDecision` 和专用能力判断 |
| 实时可用性 | 当前这一刻哪个账号能接单？ | 现有实时 scheduler、并发、冷却、限流、利润、代理与 sticky 逻辑 |
| 用户展示 | `/v1/models` 或模型广场展示什么？ | 复用公开模型 + 稳定交付投影；`models_list_config` 仅作为显式展示覆盖 |

关键原则：

```text
公开模型准入
  ∩ 稳定模型/协议路由
  ∩ 成员授权与端点能力
  → 企业成员候选计划

候选计划
  ∩ 实时调度条件
  → 实际 ActiveGroup + Account
```

规划器的“可用”不是实时成功保证，实时 scheduler 仍必须再次执行所有限制。规划器的价值是提前排除稳定不可能成功的分组。

---

## 7. 推荐目标架构

### 7.1 新数据流

```text
APIKeyAuth
  → MemberAuthorization
  → EndpointIntentExtractor
       public_model
       inbound_protocol
       image/video/live/batch intent
  → EnterpriseMemberRoutePlanner（一次批量解析）
       授权分组快照
       公开模型准入
       每组映射与 Composite 预览
       稳定账号/协议/功能能力
       输出有序 CandidateDecision[]
  → 如果无合格候选：聚合原因并在进入 handler 前终止
  → MemberRequestOrchestrator
       激活合格候选
       重新验证候选决策版本/必要动态门
       进入现有分组内 scheduler
       接收 typed GroupAttemptResult
       仅在安全且未提交时尝试下一个合格候选
  → 单次逻辑计费/usage 结算
  → Ops 写入终态 + routing_attempts
```

### 7.2 `EnterpriseMemberRoutePlanner`

建议在 service 层新增独立规划器，而不是继续把数据库和调度事实堆进 Gin middleware：

```go
type EnterpriseMemberRoutePlanInput struct {
    PublicModel      string
    InboundEndpoint  string
    InboundProtocol  ModelProtocol
    ExplicitIntent   EndpointIntent
    AuthorizedGroups []Group
}

type CandidateDecision struct {
    GroupID           int64
    MemberOrder       int
    Status            CandidateStatus // eligible | ineligible | unknown
    ReasonCodes       []RouteReasonCode
    PublicModel       string
    ChannelModel      string
    UpstreamModelHint string
    TargetPlatform    string
    RequiredProtocol  ModelProtocol
    StableRouteCount  int
}

type EnterpriseMemberRoutePlan struct {
    Source             RoutePlanSource // live | last_known_good
    EvaluatedAt        time.Time
    EligibilityVersion string
    SnapshotAge        time.Duration
    Candidates         []CandidateDecision
}
```

约束：

- 输入分组必须已经完成成员授权，规划器不能自行扩权。
- 输出按原绑定顺序稳定排序。
- 所有组、账号、能力和渠道数据必须批量加载，不能按“分组 × 模型 × 账号”产生 N+1 查询。
- `CandidateDecision` 是请求级快照，不写回 API Key 缓存对象。
- `UpstreamModelHint` 只用于诊断；实际候选激活时仍重新解析映射，避免配置变化后使用旧决策。
- 规划器内部错误返回 `unknown` 并进入第 7.4 节的降级决策，不得伪装为 `eligible` 后继续 shotgun routing。
- 每个计划必须标记 `live` 或 `last_known_good` 来源、资格版本和快照年龄；后续 Ops、usage 和指标不得把降级计划伪装成实时计划。

### 7.3 公开模型准入策略

推荐对企业成员多分组 Key 使用 `published_only`：

- 公共模型必须出现在成员至少一个授权分组关联的 active channel 中。
- “出现”指精确公开模型身份：精确 mapping 源或精确定价模型。
- 通配符 mapping 只能从明确的定价模型展开，不能让任意未知字符串自动成为已发布模型。
- 空账号 `model_mapping` 或透传账号只证明账号可以尝试这个模型，不证明站点向企业成员发布了它。
- 自定义别名需要在渠道 mapping 中显式登记源模型；这样可审计、可定价、可展示，也能进入稳定交付投影。

建议新增系统设置枚举：

```text
gateway.enterprise_member_model_admission_mode =
  legacy_order_only | shadow_published | enforce_published
```

语义：

| 值 | 行为 |
| --- | --- |
| `legacy_order_only` | 仅作为紧急回滚兼容，不作为长期默认 |
| `shadow_published` | 计算新旧候选差异并记录指标，但仍执行旧计划 |
| `enforce_published` | 只执行新规划器返回的合格候选 |

最终默认值应为 `enforce_published`，但该枚举值可被持久化不等于系统已经具备启用资格。服务端必须维护权威 readiness：routing revision/LKG、alias 审计、各平台 evaluator、端点覆盖和灰度停止条件任一未满足时，拒绝新的 enforce 写入，并把历史或部署配置中的 enforce 安全降级为 shadow；前端只能展示这个状态，不能自行决定放行。

如果业务确实需要任意未知模型透传，应使用普通单分组 Key，或者先把公开别名登记到渠道；不建议给企业成员多分组路由增加永久的“任意模型跨组透传”例外。

#### 7.3.1 alias 观察不是授权

shadow 阶段观察到一个模型名，只能证明客户端曾经提交过该字符串，不能证明它是合法公共模型或应当被登记。以下输入必须分开处理：

- 已经通过旧路径成功交付的企业内部精确 alias；
- 渠道映射中遗漏的合法公共模型名；
- 客户端直接使用了只应在内部出现的上游模型名；
- 拼写错误、大小写漂移或历史废弃模型；
- 模型扫描、探测或恶意高基数字符串；
- Composite 已有正式解析规则，但公开目录未同步的模型；
- 仅因空 `model_mapping` 或透传账号而偶然成功的未知模型。

任何 shadow 观察都不得自动写入渠道 mapping、模型定价或例外白名单。合法 alias 的唯一迁出路径仍是管理员审核后显式登记精确 mapping/定价，并通过稳定交付投影和定向网关测试。

#### 7.3.2 alias 迁移清单

`shadow_published` 必须产出可操作的管理员清单，而不是只增加一个总量指标。清单至少包含：

| 字段 | 说明 |
| --- | --- |
| `public_model` | 经过长度、控制字符和大小写规范化保护的客户端模型名 |
| `legacy_outcome` | 旧路径成功、404、403、503 或其它终态 |
| `planned_outcome` | 新规划器保留、裁剪或评估失败 |
| `reason_codes` | `model_unpublished`、`model_unsupported`、`endpoint_capability` 等闭集原因 |
| `final_group_id` / `channel_id` | 旧路径实际成功或终止的管理员级归属证据 |
| `request_count_7d` / `request_count_30d` | 影响规模，不作为 Prometheus label |
| `success_count_7d` / `success_count_30d` | 识别“旧成功、新裁剪”的真实兼容负担 |
| `last_seen_at` | 判断是否仍在使用 |
| `affected_enterprise_count` | 只返回企业 owner 聚合数量，不暴露成员、Key 或请求正文 |
| `stable_route_evidence` | 当前是否存在可验证的精确稳定路由；账号明细仅管理员可见 |
| `review_status` | `pending`、`registered`、`rejected_invalid`、`obsolete`、`needs_owner_action` |
| `reviewed_by` / `reviewed_at` / `review_note` | 审核责任与理由 |

观察证据优先复用：

- 成功请求的 `usage_logs.requested_model`、最终 `group_id` 和扩展后的 `schedule_meta`；
- 失败请求的 `ops_error_logs`、`routing_attempts`；
- 当前 `ModelDeliveryService` 稳定交付投影；
- 渠道 mapping/定价的当前配置。

审核状态可以使用独立、轻量的控制面 review ledger 保存，但该 ledger **不参与运行时授权**。`registered` 只有在真实渠道配置已经落地、稳定交付投影通过且定向请求验证成功后才能写入；单纯把 review 状态改为 accepted 不能让模型进入候选。

进入 enforce 前，每条“旧路径成功、新计划裁剪”的观察都必须有明确处置状态；不能因为 alias 数量较多就放宽为通配符准入。

### 7.4 稳定资格与动态容量必须分开

规划器只使用持久事实：

- 分组 active；
- 账号 active + schedulable；
- 分组/账号关系；
- 模型映射；
- 渠道发布与限制；
- 协议能力；
- 分组端点功能开关。

以下动态条件继续由 scheduler 判断，不应在规划器里复制：

- 当前并发和等待计划；
- rate limit / cooldown；
- 临时 unschedulable / runtime block；
- sticky account；
- 代理隔离和实时熔断；
- 当前利润阈值、动态倍率和账号 failover。

规划器的结果需要区分：

| 状态 | 含义 | 执行策略 |
| --- | --- | --- |
| `eligible` | 至少存在一条稳定路由 | 进入实际候选计划 |
| `ineligible:model_unpublished` | 模型未在该组明确发布 | 执行前排除 |
| `ineligible:model_unsupported` | 有账号池，但没有账号稳定支持映射后模型 | 执行前排除 |
| `ineligible:endpoint_capability` | 平台/功能/协议不能保真处理该入口 | 执行前排除 |
| `ineligible:no_persistent_pool` | 当前没有持久可调度账号 | 不执行，但终态聚合为容量/配置问题而非模型不存在 |
| `unknown:evaluation_failed` | 规划依赖读取失败 | 进入精确最近有效资格快照判定；没有合格快照才返回平台 503，永不进入旧式全组尝试 |

#### 7.4.1 故障必须按依赖类型分层

规划失败不能统一理解为“缓存坏了”。不同事实的降级权限不同：

| 失败层 | 能否使用旧资格快照 | 行为 |
| --- | --- | --- |
| API Key、企业 owner、成员状态或当前成员分组授权无法确认 | 否 | 503 fail-closed；旧快照不得恢复权限 |
| 请求模型或端点意图无法安全解析 | 否 | 400；不得进入规划和 fallback |
| 渠道发布、映射、账号稳定能力或协议能力的实时投影读取暂时失败 | 可以，但仅限第 7.4.2 节的精确快照 | 有快照时使用精确旧候选；无快照时 503 |
| 实时 scheduler、并发、冷却、利润、代理或 sticky 依赖失败 | 否 | 交给现有 typed runtime failure；资格快照不能替代实时调度 |
| 配置版本已知发生变化，但新资格尚未算出 | 否 | 快照立即失效并返回 503，不能用旧路线跨过明确变更 |

这里的降级不是传统 fail-open。它只复用此前已经成功计算过的精确资格集合，不能扩大授权、增加新分组、放宽模型准入或跳过实时 scheduler。

#### 7.4.2 最近有效资格快照

建议增加 bounded last-known-good eligibility snapshot：

```text
实时规划成功
  → 保存精确资格快照

实时资格投影失败
  → 先完成当前成员授权交集
  → 查找同一 model + endpoint profile + intent + config generation 的快照
  → 存在未过期快照：只恢复快照内仍被当前成员授权的候选
  → 无快照或快照过期：503 routing_eligibility_unavailable

任何分支
  → 实时 scheduler 重新执行全部账号、渠道定价、利润、容量、隐私、代理与 sticky 检查
  → 永不恢复 legacy 全授权分组扫描
```

快照 key 至少包含：

- 规范化 `public_model`；
- 入站端点、协议和图片/视频/Live/Batch 等 intent profile；
- `group_id`；
- 渠道映射、分组能力、账号稳定资格和逐模型协议能力的配置 generation；
- 规划算法版本。

快照 value 只保存候选资格、reason code、映射摘要、生成时间和配置版本；不保存成员授权、Key、请求正文、账号凭据、动态容量或 sticky 账号。使用时必须与**当前已确认的成员授权集合**求交集。

可用性要求：

- 至少保留一个有界的进程内 LKG 层，使 Redis 短暂不可用时仍有降级价值；共享缓存可以作为附加层，但不能是唯一快照来源。
- 快照只能由完整成功的实时规划生成，不能由另一个降级计划续期。
- 任何成员授权变化立即使请求级授权交集变化；快照自身不能恢复被删除的分组。
- 渠道、分组、账号模型映射、协议能力或 Composite 配置变更必须发布失效 generation，并主动清理本地/共享快照。
- 如果已经明确观察到 generation 变化但新投影不可用，旧快照不得继续使用。
- generation 必须从比资格投影更高可用的路径读取，例如进程内维护的计数器或独立轻量键，不能与投影本身共用同一次可能失败的读取。否则投影依赖故障期间观察不到 generation 变化，上一条约束在故障窗口内静默失效，唯一保护退化为 stale TTL。该窗口必须在 shadow 阶段用故障注入实测，再据此确定最终 TTL；无法实测时应取更短 TTL 或直接设为 0。
- 多实例部署必须把 generation 实现为三层合同：数据库中的持久 revision 是配置版本权威，独立的 routing eligibility 事件负责跨实例快速传播，各实例的 atomic mirror 负责热路径高可用读取。进程内计数器只能作为 mirror 或单元测试实现，不能独立充当集群版本权威。
- routing eligibility 事件可以复用现有 outbox/Pub/Sub 基础设施和配置写入汇聚点，但必须使用独立事件类型与 `scope_type`、`scope_id`、`revision` 载荷；不能直接把逐 Key 的 auth cache invalidation 消息兼作资格版本。即使分组没有 API Key、Key 枚举失败或企业成员认证快照不进入 auth cache，也必须发布分组 revision。
- 配置事实与持久 revision/outbox 必须在同一数据库事务提交。提交后即时发布失败由 outbox 重试和 revision 对账补偿；不得出现配置已生效但资格版本静默未变的成功响应。消费者按 revision 幂等应用，忽略重复或倒序事件，并在启动、重连和订阅故障恢复后主动对账。
- 初始建议 `stale_ttl=120s`，硬上限 `300s`；shadow 阶段必须根据依赖故障率、配置变更传播时间和误路由风险确认最终默认值。不得配置为无限期。
- 快照命中只减少 503，不承诺成功；实时 scheduler 或 handler 防御门仍可拒绝过时候选并产生 typed attempt。

需要接受的残余风险是：在未收到配置失效事件且投影依赖同时故障的短窗口内，某个精确旧候选可能被尝试。该风险通过短 TTL、generation、主动失效、当前授权交集和实时门复核共同限制；若业务不能接受任何短时陈旧路线，应把 stale TTL 设为 0，退化为原方案的严格 503。

### 7.5 图片、视频和工具意图

`/v1/responses` 不能只按 URL 判断，它可能是普通文本、显式图片生成或带工具的混合请求。规划器必须复用现有意图识别函数：

- 被动声明 `image_gen` namespace 不等于显式生图意图。
- 显式图片意图要求分组 `GroupAllowsImageGeneration=true`。
- OpenAI 图片意图还要求账号支持 Responses 图片所需能力，不能静默降级到 Chat Completions。
- Grok、Gemini、Composite 按各自已有实际 handler 和映射合同判断，不能只靠分组名称或模型前缀。
- 图片/视频外部任务创建后严禁切组；Batch job 必须继续固化原 ActiveGroup。

handler 内的能力门仍保留为防御性复核。如果规划和执行之间发生配置变化，它应产生 typed `capability_mismatch`，而不是无标记 403。

### 7.6 Composite 候选

Composite 路由决策属于单个候选 attempt：

1. 规划阶段对每个 Composite 分组执行无副作用预览，得到目标平台和映射后模型。
2. 激活候选时重新解析并绑定到当前 attempt。
3. 候选失败后，切组前清除 Composite decision、目标平台、映射模型和 sticky 预取。
4. 不能把上一 Composite 候选的映射结果带到下一普通分组。

当前本地实现已完成第 1 项在三类文本协议（Chat Completions、Responses、Messages）上的安全子集：规划器通过只读 `Preview` 使用与运行时 `Resolve` 完全相同的精确/前缀/端点匹配规则；每个候选先清除继承的 Composite 上下文，再以预览结果选择具体平台、应用目标平台对应的渠道映射，并限制到该候选绑定的账号。预览结果只进入资格投影，不写回原请求；第 2-4 项仍由运行时 attempt 隔离与候选重解析负责。其他端点必须先增加各自协议映射和稳定能力 evaluator，不能复用文本结论直接放行。

### 7.7 Typed `GroupAttemptResult`

现有 `OpsGroupRetryReason` 已经比“按 HTTP 状态猜是否重试”安全，应保留并扩展为完整 attempt 结果：

```go
type GroupAttemptResult struct {
    GroupID        int64
    AttemptNumber  int
    Outcome        GroupAttemptOutcome
    Reason         OpsGroupRetryReason
    SafeToReplay   bool
    ResponseCommit bool
    OutcomeUnknown bool
}
```

允许进入下一合格候选的原因仍是闭集：

- `capability_mismatch`；
- `capacity_exhausted`；
- `transient_upstream`。

并且必须同时满足：

- `SafeToReplay=true`；
- 响应尚未提交；
- 没有预算/上游结果不明；
- 没有外部任务或首个 WebSocket turn 已提交；
- 客户端未取消。

以下情况永不因状态码自动切组：

- schema/参数错误；
- 身份、余额、预算、quota、IP 或成员权限错误；
- 内容安全、本地策略拒绝；
- 通用 500；
- 已提交响应或结果不明。

### 7.8 无合格候选时的聚合响应

不能把“最后检查的分组错误”当作整个授权范围的结果。规划器应按结构化原因生成终态：

| 内部聚合事实 | 对客建议 | Ops 归因 |
| --- | --- | --- |
| 模型未在站点任一 active channel 明确发布 | 404 `model_not_found` | client / capability / `model_not_found`，不计 SLA |
| 模型在站点存在，但没有被发布到成员授权范围 | 404 `model_not_found`，避免泄露其它分组商品 | enterprise / capability / `model_not_authorized`，不计 SLA |
| 模型已在成员范围发布，但没有允许该端点/图片能力的授权分组 | 403 `permission_error`；文案不提具体候选分组 | enterprise / capability / `endpoint_not_allowed`，不计 SLA |
| 有发布模型，但所有相关组都没有持久账号池 | 503 `api_error` | platform 或 pool ownership 对应方 / routing / `no_available_accounts` |
| 有稳定路由，实时调度全部容量耗尽 | 503 `api_error` + 合理 `Retry-After` | platform / routing_capacity；按既有池归属决定 SLA |
| 资格投影读取失败，但命中未过期的精确 LKG 快照 | 继续使用快照内合格候选；不向客户暴露降级细节 | 内部记录 `plan_source=last_known_good`；最终 SLA 由真实终态决定 |
| 资格投影读取失败且没有合格 LKG 快照，或当前授权无法确认 | 503 `routing_eligibility_unavailable` | platform / dependency/internal，计入 SLA |

对客错误不得暴露其它授权分组、账号或上游模型拓扑。管理员 Ops 详情可以显示脱敏路由链。

---

## 8. 可观测性与数据合同

### 8.1 新增 `routing_attempts`

建议为 `ops_error_logs` 增加 nullable JSONB 字段 `routing_attempts`，保存终态失败的企业成员路由链：

```json
[
  {
    "group_id": 3,
    "member_order": 1,
    "phase": "planning",
    "outcome": "pruned",
    "reason": "model_unpublished"
  },
  {
    "group_id": 16,
    "member_order": 3,
    "attempt": 1,
    "phase": "runtime",
    "outcome": "capacity_exhausted",
    "safe_to_replay": true
  }
]
```

字段要求：

- 只使用闭集 reason code，不依赖自由文本做聚合。
- 不保存完整 Key、原始请求体、凭据或敏感上游响应。
- `ops_error_logs.group_id` 暂时保留为“最终实际执行分组”，兼容旧查询。
- 如果请求在规划阶段就失败、没有执行任何分组，`group_id` 应为空；前端显示“未进入分组执行”。
- `upstream_errors` 继续只表达真实上游尝试，不把本地候选裁剪伪装成上游错误。
- 不回填历史行；旧行前端按现有终态分组展示，并标注“无路由链证据”。

### 8.2 成功请求

成功请求不需要把完整裁剪链永久写入 Ops 错误表。建议：

- 在 `usage_logs.schedule_meta` 保存低体积摘要：`route_plan_mode`、`route_plan_source`、`route_snapshot_age_ms`、`authorized_count`、`eligible_count`、`selected_member_order`、`group_attempt_count`。
- shadow 期间额外保存可聚合差异：`shadow_diff_type`、`shadow_reason_codes`、`shadow_planned_group_ids`、`legacy_selected_group_id`。这些字段仅进入管理员诊断 DTO，不向企业 owner 或 Key 自助页面暴露。
- 详细候选原因只在结构化系统日志中保留有限周期。
- 通过低基数指标观测总体效果，禁止把模型名、Key ID、成员 ID 作为 Prometheus label。

### 8.3 alias 审计数据与管理流程

shadow alias 清单不能依赖 Prometheus，因为模型名是高基数且需要逐项审核。推荐的数据路径是：

```text
成功请求
  → usage_logs.requested_model + group_id + schedule_meta.shadow_*

失败请求
  → ops_error_logs + routing_attempts

当前配置
  → channel mapping/pricing + ModelDeliveryProjection

管理员聚合查询
  → 7d/30d alias 迁移清单
  → review ledger 保存处置状态
```

约束：

- `usage_logs` 和 `ops_error_logs` 是观察证据，不能成为运行时准入白名单。
- 管理员聚合接口按模型、最终分组、渠道和 shadow 差异聚合；默认不返回企业成员、Key 或请求正文。
- 模型字符串必须 trim、限制长度、过滤控制字符，并对极端高基数输入设置查询/展示上限。
- review ledger 只保存处置责任和状态，不保存“允许路由”布尔值；真正授权仍来自渠道精确 mapping/定价和稳定路由。
- 观察聚合和 review item 至少按 `(normalized_public_model, endpoint_profile, group_id)` 隔离；同名模型在不同协议或分组中的处置不能互相放行。
- review 状态迁移必须进入现有管理员审计链，记录 actor、前后状态、理由和时间；删除 review item 不能删除 usage/Ops 事实。
- 管理员不能从清单一键批量自动注册全部模型。登记操作必须展示目标渠道、目标模型、价格缺口和稳定路由验证结果。
- 被判定为拼写错误、扫描或废弃模型的条目可以标记为拒绝/过期，但不能删除原始 usage/Ops 证据。

进入 `enforce_published` 的 alias 迁移门：

1. 最近连续 7 天所有“旧路径成功、新计划裁剪”条目都有非 `pending` 处置状态。
2. 最近 30 天仍有调用的高影响 alias 已登记、要求 owner 修正，或明确拒绝并记录理由。
3. `registered` 条目已经在真实渠道配置中出现，并通过稳定交付投影和定向网关测试；review ledger 状态本身不能满足此门。
4. 不存在为了清零清单而新增的 `*`、宽前缀或任意模型例外。
5. 首批 enforce 后继续保留 shadow 对照至少一个代表性业务周期，用于发现低频长尾。

### 8.4 指标

建议新增：

```text
enterprise_member_route_plan_total{mode,result}
enterprise_member_route_candidate_total{decision,reason}
enterprise_member_route_attempts_total{outcome,reason}
enterprise_member_route_shadow_diff_total{diff_type}
enterprise_member_route_terminal_total{reason}
enterprise_member_route_plan_duration_seconds
enterprise_member_route_snapshot_total{result,reason}
```

重点看板：

- 每个逻辑请求平均授权分组数、合格候选数、实际尝试数；
- `model_unpublished`、`model_unsupported`、`endpoint_capability` 占比；
- shadow 新旧首选分组差异；
- `legacy_success_new_pruned` 未审核条目数及其 7d/30d 请求覆盖；该数据来自管理员聚合查询，不把模型名放入指标 label；
- LKG 快照 hit、miss、stale、generation mismatch 和使用后真实成功率；
- 404/403/503 的终态原因变化；
- 路由规划 p50/p95/p99 延迟；
- 规划器依赖失败率；
- 成功率、首 Token 延迟和总延迟是否回归。

### 8.5 运维页面

错误详情建议调整为：

- 原“分组”改为“终态执行分组”。
- 新增“企业成员路由链”：授权顺序、规划裁剪原因、实际 attempt、终态原因。
- 模型未进入任何分组时显示“规划阶段拒绝”。
- 404 不再把最后一个候选分组当作模型归属。
- 本地图片能力问题不能再分类为客户端 `invalid_request`。
- 降级计划显示“最近有效资格快照”、快照年龄和配置版本，不显示账号凭据或动态调度细节。
- 增加 alias 迁移清单入口，区分“观察到”“已登记”“已拒绝”“需要 owner 修正”；任何 review 状态都不能直接放行运行时路由。

---

## 9. 实施方案

### 阶段 0：锁定合同与回归样本

目标：在改代码前把当前缺陷和目标行为固化为测试。

工作项：

1. 在 `enterprise_member_group_test.go` 增加四个生产形态的匿名 fixture。
2. 增加“请求模型必须影响候选计划”的失败测试，替换“首个授权分组永远胜出”的旧目标。
3. 保留 `models_list_config` 为展示用途的测试，防止误用它修路由。
4. 为普通 Key、空 `model_mapping`、OpenAI 透传、自定义精确 alias 建立兼容测试。
5. 为响应提交、预算结果不明、图片任务创建、WebSocket 首 turn 提交建立不可重放测试。

退出条件：新测试在旧实现上能稳定暴露本次问题，现有安全边界测试继续通过。

### 阶段 1：共享稳定资格评估器与 shadow plan

目标：建立单一能力判断，不改变生产路由。

工作项：

1. 从 `ModelDeliveryService`、渠道限制、账号模型支持和协议能力中抽取共享的纯判断组件。
2. 新增 `EnterpriseMemberRoutePlanner`，一次批量加载授权组、账号、渠道和能力。
3. 引入 `shadow_published` 设置，记录新旧候选差异、规划耗时和成功请求的 `schedule_meta.shadow_*` 摘要。
4. 建立管理员 alias 聚合查询和 review ledger；先产出 7d/30d 基线，确认“旧成功、新裁剪”的真实数量和影响企业数。
5. 增加精确 LKG eligibility snapshot，区分鉴权/授权失败、资格投影失败和实时 scheduler 失败；只有资格投影暂时失败可以使用快照。
6. 对规划读取失败、LKG hit/miss/stale/generation mismatch 显式计数，不允许静默当作全模型支持。
7. 对 Composite 增加无副作用预览接口和每候选重解析测试。三类文本协议的预览和候选隔离已完成；非文本端点继续列为阶段退出项。

退出条件：

- 无 N+1 查询；
- shadow 结果可以解释四个生产样本；
- 可以列出全部 `legacy_success_new_pruned` 模型及其 7d/30d 证据，且没有自动修改渠道配置；
- 资格投影故障时只有精确、未过期、generation 匹配且仍在当前成员授权交集内的 LKG 候选可以恢复；授权读取失败始终不能恢复；
- p95 规划开销满足后述性能门；
- 不改变对客结果和实际分组。

### 阶段 2：Typed attempt 与本地能力门收口

目标：即使规划遗漏或配置竞态，本地能力失败也不会错误终止或误分类。

工作项：

1. 扩展 `OpsGroupRetryReason` 为完整 `GroupAttemptResult`。
2. 图片、Live、Embeddings、Messages、Batch、Video 和 Composite 本地能力门统一使用 typed result。
3. 图片权限门对企业成员候选标记 `capability_mismatch`；普通 Key 仍返回现有 403。
4. Orchestrator 只消费 typed result + transactional writer/结果不明状态，不按 HTTP 状态猜测。
5. 保存请求级 routing trace，切组时保留历史但清除当前候选临时状态。

退出条件：本地能力竞态可安全进入下一合格候选；已提交或结果不明路径零重放。

### 阶段 3：启用 `enforce_published`

目标：停止未知模型全组扫描和稳定不可能成功的候选执行。

当前本地状态：readiness provider、rollout policy、manual/metrics auto-stop、配置保存限制和前端展示均已实现，并已通过最终本地验证；真实生产 enforce 灰度尚未执行，不能把“readiness-capable”写成“已生产启用”。

工作项：

1. 将服务端 readiness 从硬阻断切换为可计算的前置条件集合；只有阶段 1-2 退出条件、端点/平台 evaluator 覆盖、revision/LKG、alias 审计和自动停止条件全部有验证证据时才允许返回 ready。
2. 先对测试企业账号/成员灰度。
3. 使用正式 alias 清单审计所有被裁剪但旧路径曾成功的请求；每条记录必须进入 `registered`、`rejected_invalid`、`obsolete` 或 `needs_owner_action`，不能遗留 `pending`。
4. 将合法自定义 alias 补到渠道精确 mapping/定价，并验证稳定交付投影和真实网关请求；不能增加硬编码前缀、宽通配符或 review-ledger 放行例外。
5. 对需要客户端修正的 alias 通知对应企业 owner，但不得向平台管理员展示成员或完整 Key。
6. 验证 LKG 快照的建议 TTL、generation 失效传播和依赖故障率，再决定生产默认 TTL；无法接受短时陈旧候选时将 TTL 设为 0。
7. 按企业账号或稳定哈希逐步扩大 enforce 比例。
8. 无合格候选时使用聚合错误，不激活“最后一个绑定分组”。

退出条件：

- 未发布模型的实际分组尝试数为 0；
- 合法模型成功率不下降；
- 最近连续 7 天 `legacy_success_new_pruned` 无未审核条目，30 天内仍活跃的条目全部有可审计处置；
- `registered` alias 均能在真实渠道配置和稳定交付投影中复核，不存在只改 review 状态就放行的路径；
- LKG 使用只降低规划依赖抖动造成的 503，没有恢复已撤销授权、绕过配置 generation 或触发旧式全组扫描；
- 错误终态分组不再被误读为模型归属；
- 无跨厂商误投证据。

### 阶段 4：Ops 数据和 UI

目标：让后续问题可直接从单条错误还原。

当前本地状态：`routing_attempts`、计划来源、LKG 年龄、alias review API/管理端入口和成功 usage 的管理员计划来源证据已实现；raw/preagg、导出、告警和健康评分仍只消费终态分类，避免把中间裁剪当客户失败。

工作项：

1. 增加 `ops_error_logs.routing_attempts` 迁移和 repository/DTO 映射。
2. 更新 v2 失败分类 marker 和 reason code。
3. 管理端错误详情增加路由链，旧行兼容。
4. 增加 alias 迁移清单、review 状态和 LKG 快照来源/年龄展示。
5. raw/preagg、导出、告警和健康评分继续只消费终态分类，不把中间裁剪当客户失败。

退出条件：四个匿名 fixture 在 Ops 详情中能显示正确的首选相关分组、裁剪原因和终态责任方。

### 阶段 5：默认值与旧模式退役

目标：避免 `legacy_order_only` 永久存在成为第二套行为。

当前本地状态：legacy retirement target、退役状态、风险提示和 phase5 gate 已实现；`phase5_production_gate_pending` 仍明确阻止新安装默认值切到 enforce。只有在全量 enforce 稳定一个真实发布窗口且有生产证据后，才能把 gate 改为已验证、切换默认值并删除 shadow/legacy 双轨代码。

工作项：

1. 全量 enforce 稳定一个发布窗口后，把新安装默认改为 `enforce_published`。
2. `legacy_order_only` 只保留一个明确的兼容期限和告警。
3. 删除 shadow 双算和已经无人使用的旧候选分支；保留 alias review ledger、历史 usage/Ops 证据和 enforce 后必要的准入审计查询。
4. 更新企业成员主设计、验证矩阵、配置迁移索引、补丁记录和变更记录。

退出条件：仓库只保留一套正式候选资格实现，回滚走版本/设置而不是长期双轨代码。

---

## 10. 预计代码与数据改动面

| 区域 | 建议改动 | 目的 |
| --- | --- | --- |
| `backend/internal/server/middleware/enterprise_member_group.go` | middleware 消费规划器结果；删除丢弃模型的资格签名 | 候选计划真正使用模型/意图 |
| `backend/internal/service/enterprise_member_route_planner.go` | 新增批量规划器和稳定 reason code | 集中维护企业成员路由合同 |
| `backend/internal/service/enterprise_member_route_snapshot.go` | 新增有界进程内 LKG 资格快照、快照 key 组装和失效消费逻辑 | 资格投影抖动时保持精确降级，不恢复全组扫描 |
| `channel_service.go` / `group_service.go` / `account_service.go` / `model_protocol_capability.go` 的配置写入路径 | 在渠道映射与定价、分组能力开关、账号 `model_mapping`、逐模型协议能力变更时递增并发布配置 generation | LKG 快照 key 的 generation 生产方；缺失则快照无法感知配置变更 |
| `backend/internal/service/model_delivery.go` | 抽取可复用的稳定候选判断 | 控制面和数据面共享真相 |
| `backend/internal/service/channel_service.go` | 提供批量公开模型/限制判断 | 避免每组重复读取与 N+1 |
| `backend/internal/service/*model_availability*.go` | 将单组诊断扩展为批量稳定能力输入 | 区分模型不支持与动态容量 |
| `backend/internal/server/middleware/enterprise_member_orchestrator.go` | 消费 typed attempt、保存 trace、严格清理候选状态 | 安全跨组切换 |
| `backend/internal/service/ops_upstream_context.go` | 增加 route attempt 类型和上下文收集 | 统一中间证据 |
| 各 gateway handler 的本地能力门 | 返回 typed capability mismatch | 防御规划/执行竞态 |
| `backend/internal/handler/ops_failure_classification.go` | 增加 enterprise/model/capacity 明确 marker | 修正责任方和 SLA |
| `backend/migrations/*_ops_routing_attempts.sql` | 新增 nullable JSONB | 持久化终态路由链 |
| `backend/migrations/*_enterprise_member_model_admission_reviews.sql` | 新增轻量 review ledger；不作为运行时准入源 | 保存 alias 处置责任、状态和理由 |
| `backend/internal/repository/ops_repo.go` | 写入/查询新字段 | Ops API 提供证据 |
| usage repository / `schedule_meta` 管线 | 保存 shadow 差异和 LKG 摘要；保持 owner DTO 隔离 | 支撑旧成功/新裁剪审计和降级观测 |
| admin enterprise route audit handler/service | 聚合 7d/30d alias 清单、稳定路由证据和 review 状态 | 在 enforce 前完成可审计迁移 |
| `frontend/src/views/admin/ops/*` | 展示终态执行分组与路由链 | 避免错误归属解读 |
| 管理端 alias 迁移视图 | 审核、定位配置入口和验证登记结果；不提供直接放行开关 | 防止 shadow 输入污染公共模型目录 |
| `docs-site/dev-zz/testing/verification-matrix.md` | 加入长期回归门 | 防止后续 merge 回退 |

实现时应先确认现有 `ModelDeliveryService.ResolveForGroups` 能否成为共享批量核心。若它对图片、Composite 或非三类文本协议覆盖不足，应抽出更底层的 `StableRouteEvaluator`，不要在企业成员规划器中复制一套账号/协议判断。

配置 generation 不需要新建一套失效体系，应扩展现有汇聚点，避免出现第二条与缓存失效不同步的版本通路：

- `ChannelService.invalidateCache()` 已是渠道 CRUD 的单一汇聚点（当前由 create/update/delete 三处写入调用），渠道映射与定价的 generation 递增应挂在这里，与缓存重建同一时刻发布。
- `APIKeyService.InvalidateAuthCacheByGroupID` 等既有按维度失效入口可作为分组维度 generation 的发布位置。
- 进程内 generation 计数器已有先例：账号运行时封禁使用 atomic sequence `Add(1)` 加 per-ID `Store` 的模式（见 `openai_account_runtime_block_fastpath.go`）。该模式作用域是运行时封禁而非配置版本，可复用写法但需独立计数器，不要与封禁 generation 共用。
- generation 递增必须与缓存失效在同一处发布。若两者分开，会出现缓存已重建但 generation 未变（快照继续使用陈旧资格），或 generation 已变但缓存未失效（快照被无谓丢弃）两种失配。

这里的“不新建一套失效体系”是指复用现有事务、outbox、Pub/Sub、订阅恢复和健康观测基础设施，而不是复用逐 API Key 的事件语义。实现必须维护一张 writer coverage matrix，逐项覆盖 Channel、Group、Account、ModelProtocolCapability 和 Composite 的所有资格相关写入口；新增入口如果没有 revision 测试，不得视为接入完成。

当前实现采用了更强的数据库汇聚点，而不是依赖 Go service 调用约定：migration `199_routing_eligibility_revision.sql` 为资格相关表安装行级触发器，使配置事实、持久 revision 与独立 outbox 在同一事务中提交。这样 Ent、自定义 SQL、批量导入或未来新增 service 只要仍写同一资格字段，就不会绕过 generation。scheduler outbox 不能直接承载该事件：它使用 Redis 共享单一 watermark，只保证共享调度快照更新一次，无法保证每个进程私有 mirror 都收到事件。为此 routing eligibility 使用独立 outbox + Redis Pub/Sub；Pub/Sub 允许重复，消费者按 revision 幂等忽略倒序事件，30 秒数据库全量对账补偿断线和漏消息。

当前 writer coverage：

| scope | 资格写入表/字段 | revision 行为 |
| --- | --- | --- |
| Channel | `channels.status/model_mapping`、`channel_groups`、`channel_model_pricing.channel_id/platform/models` | 推进精确 channel 与 `channel:0`；group binding 同时推进精确 group |
| Group | `groups` 的 status/platform、图片/Batch/Live/Messages、模型路由、默认映射、OAuth/privacy 筛选和软删除字段 | 推进精确 group 与 `group:0` |
| Account | `accounts` 的 platform/type/status/schedulable/软删除、`credentials.model_mapping/openai_capabilities`、Responses/透传/`privacy_mode` 相关 extra，以及 `account_groups` | 推进精确 account 与 `account:0`；group binding 同时推进精确 group；last-used、rate-limit、cooldown 和无关 extra 等瞬态字段被排除 |
| Protocol | `account_model_protocol_capabilities` 的模型、协议、override/observed state | 推进 account 对应的 protocol/account 精确 scope 与两个全局 scope；仅 observed_at/source 刷新不推进 |
| Composite | `composite_model_routes` 的匹配、端点、目标平台、上游模型、优先级、enabled/软删除 | 推进 group 对应的 composite/group 精确 scope 与两个全局 scope |

LKG 热路径使用 startup-reconciled atomic mirror，而不是在资格投影故障时再次依赖同一个 PostgreSQL；否则数据库故障会同时击穿投影与 revision 读取，使 LKG 失去意义。安全边界由事务内 outbox、Pub/Sub 主动失效、30 秒全量对账、120 秒快照 TTL、同为 120 秒的全量对账新鲜度上限、当前授权交集和实时 scheduler 复核共同构成。Pub/Sub 事件只负责提前失效，不能证明“没有漏事件”，因此不能延长 mirror 的权威窗口；只有成功的全量数据库对账可以续期该窗口。实例启动时数据库对账失败，或运行后全量对账连续失败直至超过窗口，都会把 LKG 置为 unready；shadow/legacy 主路径仍按原合同运行。outbox 保持 1 秒发布轮询，但 24 小时历史清理由独立的一小时节流执行，避免每实例每秒产生无效清理 SQL。

---

## 11. 验收与测试矩阵

### 11.1 四个事件的目标行为

| 场景 | 目标候选 | 目标终态 |
| --- | --- | --- |
| 未发布 `gpt-image-1.5` + `/v1/responses` 显式生图 | 0 个实际候选 | 规划阶段 404；不得进入 `deepseek`，不得出现“this group”文案 |
| `minimax-m3` 仅在 minimax 分组发布且有稳定路由 | 只保留 minimax | 只尝试 minimax；失败按真实能力/容量分类，不得落到 `mimo` |
| `glm-5.2` 仅在 glm 分组发布且有稳定路由 | 只保留 glm | 只尝试 glm；失败不显示 `mimo` |
| `gpt-5.6-terra` 仅在 OpenAI 分组发布 | 只保留相关 OpenAI/Composite 路由 | 无关 deepseek/minimax/glm/mimo 不执行 |

### 11.2 单元测试

- 绑定顺序只作用于 `eligible` 候选。
- 模型未发布、模型不支持、端点不允许、无持久账号池、评估失败分别输出稳定 reason code。
- 空 `model_mapping` 只在模型已发布后作为稳定账号支持事实。
- 精确 alias 能进入候选；未登记 alias 不能进入。
- `models_list_config` 变化不改变运行时候选。
- 渠道限制、渠道映射、账号映射和最终上游模型按固定顺序执行。
- Composite 每候选重解析并清理上一决策。
- 图片被动 namespace 不触发生图；显式意图触发完整能力门。
- 只有完整成功的实时计划能生成/刷新 LKG；LKG 命中不能续期另一个 LKG。
- LKG key 必须区分模型、端点、intent、配置 generation 和规划算法版本。
- LKG 使用前必须与当前授权集合求交集；被撤销分组永远不能被快照恢复。
- TTL 到期、generation 变化或显式失效后快照不可使用；`stale_ttl=0` 等价于关闭降级。
- alias review 状态不能改变规划器结果；只有真实渠道 mapping/定价和稳定路由可以使其变为已发布。
- 普通 Key 行为不变。

### 11.3 Orchestrator 测试

- `capability_mismatch`、`capacity_exhausted`、`transient_upstream` 在未提交时可进入下一合格候选。
- 客户端 4xx、本地安全策略、预算/余额/quota、通用 500 不切组。
- Header flush、首 SSE data、首业务字节、WebSocket 首 turn、外部任务创建后不切组。
- 上游写入结果不明、预算结果不明不切组。
- 请求 body、Content-Length、上下文、Gin keys 在安全重试时正确恢复。
- 每个分组最多一次，不产生 fallback 环。
- 渠道定价限制、利润门和 sticky 语义在所有重试路径保持不变。

### 11.4 集成测试

- PostgreSQL 批量加载候选时查询数有上限，不随“分组 × 账号”乘积增长。
- 当前授权读取失败时，即使存在 LKG 也返回平台 503，不恢复任何旧分组。
- 渠道/账号/协议资格投影读取失败且有未过期、generation 匹配的精确 LKG 时，只恢复 LKG 与当前授权集合的交集，并重新经过实时 scheduler。
- 同一资格投影故障在 LKG miss、stale、generation mismatch 或 TTL=0 时返回平台 503，不回退全组扫描。
- 依赖恢复后下一次成功实时规划替换旧快照；降级请求本身不能无限延长陈旧窗口。
- 渠道配置热更新后，新请求使用新 publication/映射；进行中的 attempt 保持请求级快照。
- 模型协议严格路由开启/关闭两种模式均使用同一候选事实，并遵守各自兼容合同。
- 企业成员 usage 最终仍原子记录实际执行分组、成员快照和预算回执。
- Ops 新旧行查询、分页、导出、raw/preagg 和详情兼容。
- `usage_logs.schedule_meta` 的 shadow/LKG 字段只进入管理员 DTO；企业 owner、成员 usage、自助 Key 查询和 CSV 不暴露内部候选拓扑。
- alias 管理查询能从成功 usage、失败 Ops 和当前交付投影得到一致的 7d/30d 聚合，review ledger 不改变路由结果。

### 11.5 alias 迁移与污染防护测试

- 旧路径成功、新计划裁剪的 alias 出现在审核清单，并带成功量、最后出现时间、最终分组和稳定路由证据。
- 同一模型大小写/空白按既定公共模型身份规则规范化，但不会把不同合法模型错误合并。
- 超长、控制字符、高基数扫描输入被安全截断/限制，不造成日志注入、指标 label 爆炸或 review ledger 无限增长。
- `registered` 状态只有在渠道配置和定向验证完成后可写入；直接修改 review 状态不能让请求通过。
- `rejected_invalid`、`obsolete` 和 `needs_owner_action` 保留历史证据，但不进入公共模型目录。
- enforce gate 能证明最近连续 7 天不存在 `pending` 的 `legacy_success_new_pruned`，并覆盖最近 30 天仍活跃条目。
- 清单不得返回企业成员、完整 Key、请求正文或非必要账号凭据。

### 11.6 性能门

- 每个企业成员请求最多增加一次批量规划读取；禁止逐候选仓储查询。
- 稳态缓存命中下，路由规划 p95 增量目标不超过 5 ms；数据库回源 p95 增量目标不超过 20 ms。上线前以生产同规格压测结果确认，目标不是硬编码超时值。
- LKG 查找必须是有界内存/O(1) key 访问；缓存有最大条目数、TTL 驱逐和高基数模型保护。
- alias 7d/30d 报表走聚合查询/索引或异步汇总，不扫描并阻塞网关热路径。
- 未发布模型的 handler/account scheduler 调用数必须为 0。
- 实际分组 attempt 数不得超过合格候选数，且通常显著小于授权分组数。

### 11.7 安全与隐私

- 对客 404/403 不暴露其它分组、账号、映射目标或能力证据。
- Ops routing trace 不含 Key、凭据、请求正文或敏感上游响应。
- 规划器不能把未授权分组加入候选。
- 自定义 alias 必须显式登记，不以用户可控正则或模型名前缀提升权限。
- LKG 不保存授权事实，不恢复被撤销权限，不跳过渠道定价、利润、隐私、代理或 sticky 复核。
- review ledger、shadow 观察和管理员处置状态都不是路由授权源。

---

## 12. 灰度、回滚与停止条件

### 12.1 上线顺序

1. 先发布测试、规划器、指标和 `shadow_published`，不改变路由。
2. shadow 启动即生成 7d/30d alias 基线和 review 队列，不能等到 enforce 前临时人工搜索日志。
3. 运行至少一个有代表性的业务周期，覆盖文本、图片、流式、WebSocket、自定义 alias、配置热更新和规划依赖故障注入。
4. 清理 shadow 中“旧路径成功、新计划裁剪”的合法模型配置；所有活跃项必须有明确 review 状态和验证证据。
5. 用故障注入验证 LKG hit/miss/stale/generation mismatch、授权撤销和 TTL=0 行为，再确认生产 stale TTL。
6. 对内部测试企业和少量成员 Key 启用 `enforce_published`。
7. 按稳定哈希扩大比例，观察成功率、候选数、延迟、LKG 使用后成功率和分类变化。
8. 全量后再发布 Ops UI 和默认值调整；数据字段和 alias 审核入口应提前兼容部署。

### 12.2 自动停止/回滚条件

任一条件触发时停止扩大灰度，并回到 `shadow_published`：

- 合法模型成功率相对对照组显著下降；
- 出现 `legacy_success_new_pruned` 的活跃未审核 alias；
- 规划器 `unknown:evaluation_failed` 持续升高；
- LKG generation mismatch、过期快照命中或使用 LKG 后错误分组显著增加；
- 规划延迟超过既定 p95/p99 预算；
- 出现成员未授权分组被加入候选；
- 出现成员授权撤销后被 LKG 恢复；
- 出现渠道定价、利润、隐私或 sticky 限制被绕过；
- 出现已提交/结果不明请求跨组重放；
- 出现合法自定义 alias 无法通过显式配置恢复。

### 12.3 回滚策略

- 行为回滚：把 admission mode 切回 `shadow_published`，保留规划和指标证据。
- 降级回滚：把 LKG stale TTL 设为 0，恢复严格资格投影失败即 503；不能把 LKG 故障回滚成全授权分组扫描。
- 代码回滚：新 JSONB 字段保持 nullable、旧版本忽略；不需要破坏性降级迁移。
- 数据回滚：不删除 `routing_attempts`、alias review ledger 或历史 usage/Ops shadow 证据。
- 禁止以“临时调整成员分组顺序”作为回滚，因为那会改变权限优先级且不能解决未知模型跨组扫描。

---

## 13. 不采用的方案

| 方案 | 不采用原因 |
| --- | --- |
| 把 `mimo`、`deepseek` 移到更前或更后 | 只改变最终错误位置，不改变模型无感知路由 |
| 用模型名前缀猜分组 | 厂商别名、渠道映射、Composite 和自定义模型会持续漂移 |
| 把 `models_list_config` 当白名单 | 它是展示配置；会制造控制面与数据面两套真相 |
| 给四个模型写硬编码 blacklist | 只修样本，新的未知模型仍会横扫分组 |
| 把所有 403/404 设为可重试 | 会重放真实客户端错误、权限拒绝或策略拒绝 |
| 只在最后改错误文案 | 不减少无关候选执行，也不消除真实误投风险 |
| 全局删除空 `model_mapping = 允许所有` | 会破坏普通 Key、自定义上游和透传兼容 |
| 只依赖实时 scheduler 自己逐组失败 | 仍然是 shotgun routing，且运维归因继续失真 |
| 规划依赖失败时恢复全部授权分组 | 可用性看似提高，但会重新打开未知模型跨厂商扫描和错误归因问题 |
| 无期限使用旧资格快照 | 会让已撤销的渠道/能力配置长期失效；LKG 必须短 TTL、版本化且主动失效 |
| 自动把 shadow 观察写成 alias | 用户输入、拼写错误和扫描流量会污染公共模型目录并隐式提权 |
| 永久保留两套候选算法 | 长期会再次漂移；shadow 只能是迁移工具 |

---

## 14. 审核决策点

以下决策需要在实现前确认。本文给出推荐值：

| 决策 | 推荐 |
| --- | --- |
| 企业成员未知模型准入 | `published_only`；未明确发布即不进入任何分组 |
| 自定义 alias | shadow 形成 7d/30d 审计清单；管理员审核后通过渠道精确 mapping/定价登记，不自动注册、不增加永久透传例外 |
| 普通 Key 行为 | 完全保持现状，本次只治理企业成员多分组 Key |
| 当前授权读取故障 | 始终 fail-closed 为平台 503；LKG 不保存也不恢复成员授权 |
| 资格投影读取故障 | 优先使用短 TTL、generation 匹配的精确 LKG 与当前授权交集；无合格快照才 503，绝不回退全组扫描 |
| LKG 陈旧窗口 | 初始建议 120 秒、硬上限 300 秒；shadow/故障注入后确认，业务不接受陈旧路线时设为 0 |
| alias enforce 门 | 连续 7 天无未审核 `legacy_success_new_pruned`，最近 30 天活跃项全部有明确处置和配置验证 |
| 运维存储 | 新增 `ops_error_logs.routing_attempts`，不复用 `upstream_errors` |
| alias 审核存储 | review ledger 只保存处置状态与责任，不作为运行时准入源 |
| 旧 `group_id` 语义 | 保持“最终实际执行分组”；UI 改名并增加完整路由链 |
| 上线方式 | shadow → 小比例 enforce → 全量；旧模式设退役期限 |

本方案已于 2026-08-05 通过实施审核；截至 2026-08-06，阶段 0-4 的本地代码已实现并通过最终本地验证，阶段 5 的本地退役准备已实现但仍受生产发布窗口 gate 阻断。可执行测试清单见 [企业成员模型感知路由实施测试规格](../testing/enterprise-member-model-aware-routing-test-spec.md)。后续修改必须继续按该规格先锁定回归合同，再修改运行时代码；未经单独授权不得修改生产配置或部署生产。

---

## 15. 完成定义

该问题只有在以下条件全部满足时才算关闭：

- 四个匿名生产形态 fixture 全部按第 11.1 节通过。
- 未发布模型不激活任何企业成员分组。
- 相关模型只在有稳定成功可能的授权分组间按原顺序尝试。
- 图片能力失败不再停在无关 `deepseek` 分组，也不再归为客户端 `invalid_request`。
- `mimo` 不再因为“绑定顺序最后”而成为无关模型的终态归属。
- 普通 Key、自定义精确 alias、透传账号和现有协议兼容测试无回归。
- 所有旧成功/新裁剪 alias 已形成 7d/30d 清单，活跃条目全部经审核；任何 shadow/review 状态都不能自动放行模型。
- 资格投影抖动只允许使用短 TTL、generation 匹配且仍在当前授权交集内的精确 LKG；授权读取失败、快照过期或版本不匹配均返回 503。
- LKG 从未恢复被撤销分组，也没有绕过实时渠道定价、利润、隐私、代理、容量或 sticky 检查。
- 无响应提交后、外部任务创建后或结果不明时的跨组重放。
- 渠道定价限制、利润控制、隐私和 sticky 合同在首次与重试路径一致。
- Ops 单条详情能够还原规划裁剪、实际 attempt 和终态原因。
- 成功 usage 能区分 live/LKG 计划来源，管理员可追踪快照年龄、命中原因及最终成功率；企业 owner 和 Key 自助接口不暴露这些内部信息。
- 灰度指标稳定，`legacy_order_only` 已有明确退役时间而非永久保留。
