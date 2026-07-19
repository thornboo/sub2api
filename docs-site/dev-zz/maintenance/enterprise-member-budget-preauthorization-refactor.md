# 企业成员预算预占语义纠偏

> 状态：实现完成，后端、客户前端、文档与回归验证均已收口，待提交发布；当前线上行为尚未改变。
>
> 记录日期：2026-07-20

## 问题摘要

企业成员的同步模型请求当前采用严格金额预占：请求进入上游前，系统按照可达模型、模型映射、分组倍率、最大输出 Token 和安全余量计算理论消费上界，并要求：

```text
已结算用量 + 在途预占 + 本次理论预占 <= 成员额度
```

这会导致成员仍有明显剩余预算时，请求仍因理论上界过高而返回 `ENTERPRISE_MEMBER_BUDGET_EXCEEDED`。客户看到“预算尚未用完”，却无法继续请求，无法从实际消费角度理解失败原因。降低分组倍率可能让同一请求通过，进一步证明失败由理论预占而非真实余额耗尽触发。

## 已确认根因

企业成员预算被独立设计成了并发情况下也绝不超额的严格硬限额，因此引入了金额 reservation、最坏情况费用估算、持久化预留、结算、释放、模糊结果恢复和对账。

这与项目现有计费语义不一致：

- 普通余额只要未耗尽就允许同步请求，完成后按实际费用扣减，最后一笔可以形成有限透支。
- API Key quota 在请求前只检查当前已用，完成后累计实际费用，最后一笔可以跨过额度。
- 订阅日、周、月额度和用户平台额度同样采用当前用量预检、实际费用后结算。
- 企业成员同步请求却按照理论最大金额预占并提前拒绝，形成了更严格且客户不可理解的特殊语义。

这不是单纯的前端文案问题，而是产品计费语义与现有系统不一致。

## 目标语义

企业成员的普通同步请求应与项目现有计费保持一致：

1. 请求前只判断成员当前实际已用是否已经达到 5h、1d、7d 或自然月限额。
2. 当前额度未耗尽时允许请求，不计算理论最大费用，不进行金额预占。
3. 请求完成后按 `ActualCost` 原子结算成员用量、企业余额、Key quota、usage 和预算账本。
4. 允许最后一笔或少量并发在途请求造成有限超额；结算后立即阻止后续请求。
5. 同步请求继续保留零金额请求 receipt，用于成员归属、幂等结算、崩溃恢复、防重放和对账，但 receipt 不占用客户预算。
6. 只有 HTTP 响应结束后仍会继续产生费用的异步任务，例如 Batch image、异步图片或异步视频，才保留基于明确预计任务费用的资金冻结。

## 客户侧展示目标

成员列表、预算详情、Key 自助查询和成员用量分析只以实际费用展示：

```text
预算
已用
剩余
使用率
```

普通同步请求不再向客户展示“预占”“处理中预占”或将 `reserved_usd` 计入已用金额。

如果最后一笔请求产生超额，页面应展示：

```text
已用金额
预算金额
超出金额
后续请求已停止
```

异步任务如确有资金冻结，只在对应任务详情中使用“任务冻结金额”表达，不与普通成员预算混合展示。

## 必须保留的可靠性能力

本次纠偏不能删除以下能力：

- 成员身份与多 Key 聚合归属。
- 请求级 idempotency/receipt。
- usage、企业余额、Key quota 和成员账本的原子结算。
- 重复请求和重复 worker 不重复扣费。
- 已提交上游但结果未知时的 `ambiguous` 状态与人工核对能力。
- 服务崩溃后的恢复和预算账本对账。
- Batch image、异步图片和异步视频等后台任务的真实余额冻结与捕获/释放。

需要删除的是同步请求的理论金额预授权，不是计费证据链。

## 实现边界

### 后端

- 将同步入口从“保守上界 Reserve”改为“检查当前实际用量 + 创建零金额 receipt”。
- Responses WebSocket 每个 turn 保留独立 receipt，但不再创建金额 hold。
- 5h、1d、7d 和自然月限额只比较当前实际用量，不加入 `reserved_usd` 或本次预计金额。
- 请求成功后继续按 `ActualCost` 结算；达到或超过限额后阻止下一次请求。
- 同步请求不再因缺少 `max_tokens`、模型映射候选昂贵或无法计算可靠上界而失败。
- 已有非零 reservation 不再参与新同步请求授权，但仍由现有恢复流程结算、释放或转为待核对状态；禁止上线时无条件清空历史记录。
- 保留现有数据库字段和已应用 migration，第一阶段不做破坏性 schema 删除。

主要代码入口：

- `backend/internal/server/middleware/enterprise_member_group.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/service/enterprise_member_budget.go`
- `backend/internal/repository/enterprise_member_spending_limits.go`
- `backend/internal/repository/enterprise_member_budget_repo.go`
- `backend/internal/repository/usage_billing_repo.go`
- `backend/internal/service/batch_image_billing_hold.go`

### 前端与 API

- 成员列表和预算详情不再使用 `used_usd + reserved_usd` 计算已占用金额。
- Key 自助查询的成员预算 `used` 只映射 `UsedUSD`。
- 成员用量分析、图表、tooltip 和导出不再把 `reserved_usd` 当作实际消费。
- 客户接口可以暂时保留 `reserved_usd` 字段以兼容旧客户端，但客户 UI 不再消费；后续再评估废弃。
- Ops 和对账面仍可查看历史 reservation 与 `ambiguous` receipt。

主要代码入口：

- `frontend/src/views/user/EnterpriseMembersView.vue`
- `frontend/src/components/user/UsageAnalyticsPanel.vue`
- `frontend/src/views/KeyUsageView.vue`
- `frontend/src/api/enterpriseMembers.ts`
- `frontend/src/api/publicKeyUsage.ts`
- `backend/internal/handler/public_key_usage_handler.go`

## 验收场景

1. 成员月预算为 US$300、实际已用 US$39.64 时，即使旧算法会预占 US$253.38，同步请求仍应允许。
2. 调整分组倍率不再改变请求前是否有资格执行；倍率只影响请求后的实际结算。
3. 未传 `max_tokens`、存在昂贵模型映射或多分组候选，不再触发预算上界错误。
4. 月预算 US$300、已用 US$299.90、本次实际消费 US$0.30 时，请求成功并结算为 US$300.20；下一次请求因额度耗尽被拒绝。
5. 并发在途请求可以形成有限超额，结算后所有后续请求均被阻止。
6. 请求明确未到达上游时不增加实际用量；已到达上游但结果未知时保留 receipt，不自动重放。
7. 同一 request ID 重复结算只产生一次实际扣费和成员账本记录。
8. Batch image 和其他后台异步任务仍按明确预计费用冻结，并在完成或失败后捕获/释放。
9. 客户页面不再出现普通同步请求的“预占”金额，预算使用率只按实际已用计算。
10. 线上已有 reservation 不再阻断新同步请求，并继续保持可恢复、可审计。

## 明确取舍

对于最终费用在执行前不可知的模型请求，无法同时保证“只要还有余额就允许请求”和“任何并发情况下绝不超过限额一分钱”。本次纠偏选择与项目现有计费一致：优先允许客户使用剩余额度，接受最后一笔或少量并发请求造成的有限超额，并在实际结算达到额度后阻止后续请求。

如未来确有客户需要绝对硬上限，应作为显式、可选且有清晰产品说明的严格预算模式单独设计，不能再次作为企业成员的默认行为。

## 实施与验证顺序

1. 更新 ADR 与企业成员功能合同，删除同步请求必须金额预占的既有结论。
2. 先补充上述客户场景的回归测试。
3. 重构同步授权、receipt 和实际结算边界。
4. 隔离异步任务 hold，验证 Batch image、异步图片和视频不回归。
5. 调整成员列表、预算详情、Key 自助查询、分析和导出。
6. 完成后端、前端、文档和历史 reservation 兼容验证，再进入发布流程。

## 发布注意与后续增强

### 新旧后端切换

新后端能够兼容复用旧版本创建的正金额同步 receipt；旧后端不能反向理解新版本创建的零金额同步 receipt。若同一个 request ID 在滚动发布期间先进入新实例、重试又进入旧实例，旧实例可能因金额不一致而拒绝该请求。

因此生产发布必须避免新旧后端长时间混跑：先停止旧实例接收新流量或排空旧实例，再完成全部后端实例切换；确认旧实例退出后再发布前端。发布期间不清空历史 reservation，也不回滚现有预算表数据。

### 异步图片跨进程恢复

异步图片 Redis `ImageTaskRecord` 现在持久化预算 request ID、成员、实际分组、任务冻结金额、预算状态、执行阶段与恢复期限，并用 Redis 有序索引记录需要恢复的任务。PostgreSQL receipt 通过 migration `195_enterprise_member_budget_receipt_task_link.sql` 增加显式 `receipt_kind`、`async_task_id` 与 `async_task_phase`；handler 必须先把 PG 阶段写为 `executing`，才允许调用上游。任务状态更新使用 WATCH 事务，正常完成与恢复 claim 不能互相覆盖，WATCH 重试不得复用前一次已丢弃快照的排队证据。

恢复规则按“是否可能到达上游”保守裁决：

- Redis 与 PG 栅栏都为 `queued` 时，超时表示任务尚未开始执行，恢复任务释放 receipt，并把客户可见预算状态设为 `released`；只有 Redis 为 queued 但 PG 已为 executing 时必须按未知结果处理。
- `executing`、`finalizing` 或恢复重试表示上游结果可能已经产生，禁止自动重放，receipt 转 `ambiguous`，任务预算状态设为 `needs_review`。
- receipt 已被统一计费结算时，即使图片结果因进程崩溃丢失，任务仍明确显示 `settled`，不会错误宣称冻结已释放。
- 已终止但预算仍为 `needs_review` 的任务继续低频核对 receipt；后续自动/人工结算或释放会同步刷新为 `settled/released`，不会把过时文案保留到任务过期。
- 所有异步图片释放与转 `ambiguous` 都同时校验 request ID 和 task ID；重复 request ID 创建的新 task 不能释放或改写已绑定原 task 的 receipt。通用 request-ID-only release / ambiguous 接口拒绝已绑定 task 的异步图片 receipt；任务创建后，HTTP 中间件也把 receipt 生命周期交给 task-fenced handler，形成纵深防护。公开 `budget.status` 使用 repository 返回的最终 receipt 状态，不根据“尝试了哪种操作”猜测。
- 服务启动后立即恢复一次，之后周期扫描；关闭服务时取消并等待恢复 goroutine 退出后才关闭 Redis/数据库依赖。
- 对象存储临时失效只停止新任务提交，不停止既有任务查询和恢复。
- Redis task key 丢失时，轮询按 PG task ID 查到 receipt，返回客户安全的失败 tombstone 和当前冻结状态；只要 receipt 仍为 `reserved/ambiguous` 并占用预算，就不能因普通 Redis TTL 到期退化为 `404`。
- 管理员 ambiguous receipt 列表保留 task ID、receipt kind、task phase 和冻结金额，隐藏客户 request ID 与成员名称，但仍足以关联任务、核实上游结果并执行人工释放。

提交和轮询响应返回客户安全的 `budget.task_hold_usd/status/message`，不暴露内部 receipt ID。实际已用未耗尽但异步任务冻结不足时使用独立错误 `ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE`，错误正文与 `X-Sub2API-Budget-*` 响应头明确列出限额窗口、实际已用、其他任务冻结和本次预计冻结，与真正的月额度/滚动窗口耗尽错误分开。

### 发布顺序与旧任务排空

1. 先应用 migration 195，确认所有实例都能读写 receipt kind、task ID 和 task phase；迁移完成前不得让新代码接收异步图片请求。
2. 停止旧实例接收新的异步图片任务，并等待旧实例中已经返回 `202` 的后台任务完成或进入可核对终态；普通 HTTP 连接排空并不代表这些 goroutine 已结束。无法逐项确认时，至少等待旧版最大执行窗口并核对 Redis `processing` task 后再退出最后一个旧实例；不能让不了解 PG 执行栅栏的旧实例继续创建任务。
3. 再切换全部后端实例，避免新旧实现长时间混跑；新实例启动后应立即执行一次 Redis/PG 恢复扫描。
4. 验证既有 task ID 仍可轮询、queued hold 能自动释放、executing hold 会进入 `needs_review`，再发布前端。
5. 不清空历史 receipt、预算投影或 Redis 任务索引；若必须回滚应用，只能回滚到能够容忍 migration 195 新列的版本，不能回滚数据库 migration。
