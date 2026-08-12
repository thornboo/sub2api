# 设计取舍 0005：以企业访问策略和产品权益取代成员有序分组授权

## 结论

**已采纳。**

日期：2026-08-12。

本 ADR 提议保留 [ADR-0003](./adr-0003-enterprise-member-entity.md) 的不可登录成员实体、多 Key 聚合、成员预算、幂等结算和历史审计边界，但取代其中以下领域结论：

- 成员通过 `enterprise_member_group_bindings` 继承有序分组集合；
- 成员权限以分组为最小授权单位；
- 成员通过分组顺序表达模型访问、路由优先级和跨组故障转移；
- 同一公开模型的对客计费可以由最终实际执行分组决定。

本 ADR 已于 2026-08-12 获得产品负责人采纳。ADR-0003 中与成员有序分组授权、按实际分组决定对客价格相关的目标态结论由本 ADR 取代；成员实体、多 Key 聚合、成员预算、幂等结算和历史审计合同继续有效。

采纳本 ADR 不等于要求一次性堆叠全部概念表或抽象。实施优先级只有一个：企业成员现有合法请求必须稳定成功，并且不能因首个不相关分组、规划依赖、错误分类或重放 guard 而提前失败。任何 Catalog、Offering、Entitlement 或 Route Pool 结构只有在直接降低该风险、保持计费与预算正确且有回归证据时才进入实现。

## 背景

ADR-0003 解决了第一版“一把 Key 等于一名员工”的核心缺陷：成员成为稳定、不可登录的领域主体，一名成员可以持有多把 Key，成员预算和用量能够跨 Key 聚合，历史事实不再依赖 Key 名称或标签。

为复用既有单分组鉴权、订阅、调度和计费链路，ADR-0003 同时选择 `Group` 作为成员最小授权单位：企业 owner 为成员绑定多个有序分组，成员 Key 在请求时从这些分组中选择实际 `ActiveGroup`。这一选择降低了第一阶段迁移成本，但把产品权限与后台基础设施拓扑绑定在了一起。

当前 `Group` 同时承载：

- 平台与协议能力；
- 对客倍率、高峰倍率、图片/视频价格；
- 订阅类型和日/周/月限制；
- 模型路由、默认映射和账号池；
- Messages、Live、Image、Batch、Video 等入口开关；
- OAuth、隐私、RPM、reasoning effort 和利润控制；
- fallback group 和账号调度策略。

因此“给成员绑定一个分组”并非单纯授予某项产品能力，而是同时继承授权、目录、价格、订阅、路由、故障转移和运维策略。分组、渠道、账号或模型能力任一变化，都可能隐式改变成员权限、客户价格和请求行为。

2026-08-12 的生产回归进一步暴露了这个耦合：成员的第一个绑定分组为 Anthropic 组，请求 `gpt-5.6-sol` 时旧候选顺序先激活该组；本地调度确认该组不支持模型后，新的 admission/retry guard 又阻止 legacy/shadow 请求进入下一授权组，导致多个合法模型在版本切换后确定性返回本地 404。该请求没有选择账号，也没有访问上游。回滚到完整保留旧跨组 fallback 的版本后，企业请求重新在正确的 OpenAI 分组成功。

这次事故的直接代码根因可以修复，但继续以“成员有序分组”为正式领域边界，仍会保留以下结构性问题：

1. 成员权限随着内部资源池拓扑漂移。
2. 路由器需要通过候选分组试错发现模型是否可用。
3. 同一个公开模型可能因为实际选择不同分组而产生不同对客价格。
4. 企业 owner 被迫理解和排列平台内部运维对象。
5. 分组授权、模型准入、订阅资格、动态健康和跨组重试难以分别验证。

## 决策

### 1. 保留不可登录成员实体，撤销分组作为成员权限边界

`enterprise_members` 继续是：

- 成员生命周期主体；
- 多把成员 Key 的稳定归属；
- 5h、1d、7d 和自然月预算主体；
- 用量、迁移开账、人工调整和审计主体。

成员 Key 继续满足 `member_id IS NOT NULL` 且 `group_id IS NULL`。普通单分组 Key 保持兼容。

目标态中，成员不再保存或继承有序 `group_ids`；`enterprise_member_group_bindings` 不再是在线授权事实。成员改为绑定一个企业访问策略 `AccessProfile`。

### 2. 采用五层正交领域模型

企业成员访问与内部执行拆成五层：

| 层 | 权威问题 | 主要管理方 |
| --- | --- | --- |
| Public Model Catalog | 平台正式发布哪些公开模型和能力 | 平台产品/管理员 |
| Service Offering | 这些模型以什么价格、服务等级和能力组合提供 | 平台产品/商务 |
| Enterprise Entitlement | 某企业当前购买或获准使用哪些 Offering | 平台与企业合同 |
| Member Access Profile | 某成员可以使用企业权益中的哪一部分 | 企业 owner |
| Route Pool | 请求实际由哪些平台、账号和协议路线执行 | 平台运维 |

授权链固定为：

```text
member
  -> access profile
  -> enterprise entitlement
  -> service offering
  -> public model catalog
```

执行链固定为：

```text
public model + protocol + endpoint intent + route constraints
  -> compiled route table
  -> route pool
  -> account
```

任何 route pool、group、channel 或 account 的存在都不能自行授予企业或成员模型权限。

### 3. Public Model Catalog 是公开模型身份的唯一来源

公开模型必须先进入版本化 catalog，才能被 Offering 或 Access Profile 引用。Catalog 至少描述：

- 稳定公开模型 ID；
- 文本、代码、图片、视频、embedding、tools、Web Search 等能力；
- 支持的公共协议和端点类别；
- 模型状态和 catalog version；
- 必要的产品级限制，而不是账号级瞬态状态。

账号空 mapping、通配符透传、历史偶然成功或 UI 展示字符串都不能自动把任意模型加入 Catalog。

### 4. Service Offering 是对客产品与计费边界

Offering 表达对客户销售的稳定产品，例如：

- Standard Text Models；
- Advanced Coding Models；
- Image Generation；
- Priority AI；
- Privacy-Required AI；
- Dedicated Capacity。

Offering 引用一个或多个版本化 model set，并绑定客户计费策略、服务等级、允许的能力和 route class。新模型只有经过 catalog 与 Offering 审核后才进入企业可分配范围。

### 5. Enterprise Entitlement 限定企业最大权限

企业 entitlement 表达企业当前购买、订阅或被授予的 Offering，并保存状态、有效期和版本。企业 owner 只能创建企业 entitlement 子集内的 Access Profile。

必须始终成立：

```text
member profile offerings
  subset-of enterprise active entitlements
```

管理员撤销企业 entitlement 后，旧 Access Profile、成员 Key 缓存和 route plan 都不能继续提供访问。

### 6. 每名成员绑定一个可复用 Access Profile

Access Profile 是企业 owner 可理解和复用的成员访问策略，例如“开发人员”“数据分析师”“设计师”“低预算只读成员”。它至少表达：

- 可使用的 Offering 或版本化 model set；
- 允许的 endpoint/capability；
- tools、Web Search、Image、Video、Batch 等权限；
- 最大 reasoning effort；
- 客户计费策略或 Offering 价格层；
- 高层 route class，例如 standard、priority、dedicated、privacy_required；
- profile version。

一名成员在一个时刻只绑定一个主 Access Profile，避免多个策略 union/deny 冲突。需要不同权限时，由 owner 创建新的复用 profile；成员预算和 Key 限额仍可在 profile 默认值之上保留成员级覆盖。

Access Profile 不等于为每个成员复制一份巨大模型白名单。模型集合必须通过版本化 Catalog Set 或 Offering 复用；新模型是否自动进入某个集合必须由显式产品策略决定，不能依赖通配符或内部账号变化。

### 7. Group 降级为内部 Route Pool，不再面向企业成员授权

目标态中的 Route Pool 只表达内部执行事实：

- platform、region、privacy class、capacity tier；
- account membership；
- channel mapping 和最终 upstream model；
- protocol capability；
- 动态健康、容量、熔断和内部成本；
- dedicated/shared ownership。

现有 `groups` 表可以在迁移期继续作为 Route Pool 的物理载体，但新领域代码不得继续把它同时当成企业成员产品权限。是否最终拆表或重命名，留给实施设计在不破坏普通 Key 的条件下决定。

企业确有 provider、地区、隐私、OAuth 或专属容量要求时，使用企业级或 Offering 级 `RouteConstraint` 表达高层约束，而不是重新暴露 raw group IDs 或成员 group sort order。

### 8. 对客价格与内部 Route Pool 选择解耦

目标态对客价格由发生时快照化的 Catalog/Offering/BillingPolicy 决定：

```text
customer charge
  = catalog public price
  x enterprise/offering billing policy
  x request capability price
```

实际 Route Pool 和 Account 只决定内部成本、利润与运维归因：

```text
platform cost
  = actual route pool
  x actual account upstream cost
```

同一个公开模型、同一个 Offering 和同一种请求能力，不得因为后台选择不同 Route Pool 而产生不可预测的客户价格。

如果业务需要不同价格或服务等级，必须显式创建不同 Offering 或 BillingPolicy，而不是让内部 route choice 隐式改变价格。

迁移期间若当前多个成员分组对同一公开模型存在不同对客倍率，不得自动任选一个值；必须生成价格冲突清单，选择兼容价格策略或取得明确产品决策后才能切换。

### 9. 正常请求使用预编译路由表，不逐组发现能力

控制面在配置变化后异步编译全局 immutable route table：

```text
RouteKey {
  public_model,
  inbound_protocol,
  endpoint_class,
  request_intent
}
  -> RouteSet {
       eligible route pools,
       mapped models,
       stable account routes,
       route constraints,
       generation
     }
```

Route table 由 routing eligibility revision/outbox 驱动增量失效或重建；完整新快照校验通过后以 atomic swap 发布。请求路径不得为了正常模型准入逐个查询分组、账号和协议能力。

请求时：

1. 从成员 Access Profile 和企业 Entitlement 得到产品授权。
2. 从 Catalog/Offering 得到规范化 RouteKey 和对客 BillingPolicy。
3. 从内存 route table 取得全局可交付 RouteSet。
4. 应用企业 route constraints 和当前动态健康。
5. 一次性选择一个 primary 和有界 backups。
6. 按同一 immutable plan 做预算授权、执行、结算和 Ops 记录。

正常请求只执行 primary。Backups 只用于 primary 在选择后出现的安全、瞬态运行时故障；它们不承担发现模型归属的职责。

不得通过对所有 Route Pool 并发 fan-out 或 hedging 来降低延迟。模型生成、工具调用、流式请求、图片/视频和异步任务可能产生费用和外部副作用，默认并发抢跑会导致重复执行、重复计费和不可判定结果。

### 10. Capability mismatch 是一致性故障，不是常态路由信号

如果 compiled route table 已证明某个 target 支持模型，但运行时 scheduler 在访问上游前再次报告该 pool 没有任何账号支持该模型，则视为 route snapshot、mapping 或 evaluator 一致性故障：

- 当前请求在没有提交响应、没有上游副作用且仍有已规划 backup 时，可以使用下一个 backup；
- 对应 RouteKey/Pool 必须被标记为 suspect 并触发重编译或临时 eject；
- Ops 记录计划 generation、target、能力证据与 mismatch 原因；
- 持续 mismatch 必须触发高优先级告警；
- 稳态 mismatch 率应接近零，不能用逐组 fallback 掩盖。

### 11. `/v1/models` 使用产品授权与稳定交付交集

成员模型目录定义为：

```text
published catalog
  intersect enterprise entitlements
  intersect member access profile
  intersect stable route capability
```

瞬态账号限流或短时容量不足不应让目录频繁抖动；请求可以在短时全池容量耗尽时返回 503。内部 route pool 名称、账号、上游模型和成本不得通过成员目录暴露。

### 12. Usage、预算和 Ops 同时保存产品与实际执行证据

成功或终态失败记录至少保留：

- member ID、Access Profile ID/version；
- Offering、Catalog Model 和 BillingPolicy snapshot/version；
- route plan generation/source；
- 实际 Route Pool/group、account、platform 和 upstream model；
- planned candidates、实际 attempts 和终态分类。

产品字段决定成员权限和对客计费；实际执行字段用于平台成本、运维、SLA 和历史审计。后续内部 pool 调整不得重写历史产品或计费快照。

## 概念数据模型

以下表名是目标语义建议，实施可以根据现有 Ent/schema 和迁移冲突面调整物理名称，但不能重新合并领域职责：

```text
catalog_models
model_catalog_sets
model_catalog_set_entries

service_offerings
service_offering_model_sets
billing_policies

enterprise_entitlements
enterprise_access_profiles
enterprise_access_profile_offerings
enterprise_route_constraints

enterprise_members.access_profile_id

route_pools                         # 迁移期可由 groups 承载
route_pool_accounts
route_table_snapshots / process-local immutable snapshot
```

建议的关键版本：

```text
enterprise authorization version
enterprise entitlement version
access profile version
catalog/model-set version
offering/billing-policy version
routing generation
```

鉴权缓存和热点 member route plan cache 必须把这些版本纳入 key 或在读取时拒绝过期快照。

## 请求结果语义

新领域边界下，终态不能再由“最后尝试的分组错误”决定：

| 事实 | 建议响应 | 责任边界 |
| --- | --- | --- |
| 模型未进入平台 Catalog | 404 `model_not_found` | client/capability |
| 企业未拥有对应 Offering | 404 或安全隐藏的 `model_not_authorized` | enterprise entitlement |
| 企业拥有但成员 Profile 不允许 | 403 `permission_error` | enterprise policy |
| 成员有权，但没有符合约束的稳定 Route Target | 503 `routing_unavailable` | platform routing |
| 有稳定 Target，但当前全部瞬态容量耗尽 | 503 + 合理 `Retry-After` | platform capacity 或对应 pool ownership |
| route table 依赖失败且无有效 LKG | 503 `routing_eligibility_unavailable` | platform dependency |

客户有产品权限但平台暂时无法交付时，不得误报为模型不存在或成员权限不足。具体 SLA 归因继续遵守 [ADR-0004](./adr-0004-ops-failure-taxonomy-and-sla.md)。

## 必须成立的系统不变量

1. 不可登录成员、多 Key 聚合、成员预算和历史审计继续以 `enterprise_members` 为主体。
2. 企业成员或 Access Profile 不直接引用内部 Route Pool/group/account 作为产品授权。
3. 成员有效 Offering 始终是“Profile 分配 ∩ 企业当前有效 Entitlement”。
4. Route Pool、account 或 channel 的新增不能自动扩大企业或成员产品权限。
5. 企业 Entitlement、Profile 或 Catalog 撤权必须通过版本拒绝旧鉴权缓存和旧 plan cache。
6. 同一个公开模型、Offering 和请求能力的对客价格不因实际 Route Pool 改变。
7. 请求执行 target 必须属于发生时 RoutePlan，且满足 Offering route class 和企业 route constraints。
8. 正常请求只执行一个 primary；每个 backup 最多执行一次，且重放必须满足响应未提交、结果已知、无外部副作用和预算状态明确。
9. 同一逻辑请求无论经历多少安全 backup，只能产生一次客户结算和一次成员实际用量入账。
10. `/v1/models`、请求准入和真实 route table 使用同一 Catalog/Entitlement/Profile/稳定能力合同。
11. Capability mismatch 不得作为正常逐池发现机制；发生时必须产生 snapshot inconsistency 证据。
12. Usage/Ops 必须同时保存产品授权快照和实际执行 Route Pool，配置变化不得回写历史。
13. 普通单分组 Key 的现有合同在独立迁移计划明确修改前保持兼容。

## 迁移策略

### 阶段 0：采纳决策与生产事实审计

在任何 schema 或运行时修改前：

- 明确采纳或拒绝本 ADR；
- 用脱敏生产事实审计现有企业、成员、group binding、模型发布、历史成功模型、endpoint 使用、订阅和价格；
- 建立 2026-08-12 事故固定 fixture；
- 找出同模型跨分组价格冲突、仅靠透传成功的 alias、永远不服务成员常用模型的绑定和 provider/隐私特殊约束。

### 阶段 1：添加 Catalog、Offering、Entitlement 和 Profile

- 只做 additive schema；
- 从现有正式 channel mapping/pricing 和能力事实生成待审核 Catalog；
- 为每个企业生成待审核 Entitlement；
- 为每个企业创建默认 Access Profile，但不改变在线请求；
- 当前 group 暂时继续作为内部 Route Pool。

### 阶段 2：建立编译路由表

- 将稳定模型交付投影从每请求 DB/账号枚举迁移到后台 compiler；
- 使用 revision/outbox 驱动重建；
- 初始 snapshot 未成功编译时实例不得声明企业路由 ready；
- 重建失败时只允许使用仍在权威新鲜度窗口内的上一完整 snapshot；过期后返回 503，不恢复逐组扫描。

### 阶段 3：生成迁移清单，不静默猜测

每个成员至少输出：

- 当前 group union 对应的正式 Catalog/Offering；
- 历史成功但未正式发布的模型；
- 同模型价格冲突；
- subscription 与 shared-balance 语义；
- provider、privacy、OAuth、region 和 dedicated 约束；
- 新 Profile 与旧行为的允许/拒绝差异。

任何价格、alias 或订阅语义不唯一的成员都进入 `needs_review`，不得自动切换。

### 阶段 4：纯观察 shadow

线上请求继续完整使用旧引擎；新引擎只计算：

- 产品授权结果；
- RoutePlan primary/backups；
- 对客价格；
- 预期终态。

Shadow 不得改变候选、预算、retry marker、handler 或响应。差异必须可按企业、Profile、Offering、模型和原因聚合。

### 阶段 5：完整请求级 canary

每个逻辑请求在入口处选择完整旧引擎或完整新引擎，禁止混合旧 group candidates、新 pricing、部分新 retry guard 或两套预算语义。Canary 从受控测试成员/Key 开始，再按稳定 member hash 扩大。

自动停止至少观察：

- 合法模型成功率；
- `model_not_found`、`routing_unavailable` 和预算错误率；
- 对客金额差异；
- plan latency 和 route table cache hit；
- actual target 是否属于 planned candidates；
- capability mismatch 和 snapshot inconsistency；
- usage/预算幂等一致性。

回滚通过把新引擎 cohort 降为零或回到已验证版本完成，不在同一请求中回退为 raw group 扫描。

### 阶段 6：切换权威与退役旧绑定

- Access Profile/Entitlement 成为成员在线授权权威；
- `group_ids` 成员 API 和 UI 标记 deprecated 并停止写入；
- `enterprise_member_group_bindings` 在审计期只读保留；
- 从鉴权快照和请求路径删除成员 group binding；
- 普通 Key 和历史 usage 中的实际 group ID 继续保留；
- 旧表最终删除必须另有生产数据审计、回滚窗口和明确授权。

## 采纳与发布门

本 ADR 被采纳不等于实现可以直接发布。进入正式流量前必须至少证明：

1. 生产历史成功模型全部进入 Catalog/Offering，或有明确、可审计的拒绝决定。
2. 迁移样本的客户价格差异为零，或每个差异都有明确批准和通知策略。
3. 企业 Profile 权限不超过企业 Entitlement。
4. RoutePlan targets 不超出 Offering route class 和企业约束。
5. 2026-08-12 事故 fixture 中 Anthropic pool 不进入 GPT 请求的 actual attempt，正确 OpenAI-compatible target 被一次选择。
6. PostgreSQL、Redis、revision、snapshot compiler、LKG 过期和进程重启故障注入通过。
7. 响应提交、上游结果不明、WebSocket turn、异步任务和预算 ambiguous 后都禁止 backup replay。
8. `/v1/models` 与真实请求准入使用同一合同。
9. Usage、成员预算、企业余额和 Key quota 的幂等结算保持原子一致。
10. Shadow 和 canary 具备可自动停止的生产指标，且 rollback 不依赖数据库回滚或旧表恢复。

## 被拒绝的方案

### 保留成员有序分组，只修正模型感知 planner

拒绝作为目标态。它可以修复当前直接回归，但仍让成员权限、客户价格和路由拓扑绑定在一起，也继续要求企业 owner 理解内部 pool。

### 为每个成员保存完整模型白名单

拒绝。它会复制大量模型 ID，新增模型和策略调整需要逐成员更新，授权漂移和审计成本过高。使用可复用、版本化的 Access Profile、Offering 和 Catalog Set。

### 为每个成员创建一个虚拟或聚合 Group

拒绝作为领域真相。它只把多组复杂性藏到 group-of-groups 中，继续继承 Group 的计费、订阅、路由和 fallback 混合职责，并导致配置对象数量膨胀。迁移期可以作为兼容适配器，但不能成为正式授权模型。

### 为每个成员和模型预计算完整路由矩阵

拒绝全量物化。`members x models x protocols x intents` 会造成组合膨胀和复杂失效。全局预编译 RouteKey/RouteSet，请求时与单成员 Profile/Entitlement 求交；热点计划可以使用版本化小型 cache。

### 并行请求所有可用 Route Pool，采用最先成功结果

拒绝。模型生成、工具调用、图片、视频、Batch 和流式请求存在费用、外部副作用和结果不明边界，并行 fan-out 会重复执行、放大上游负载并破坏预算幂等。

### 继续让最终 Route Pool 决定客户价格

拒绝。内部容量和成本变化不能让同一公开产品的客户账单不可预测。不同价格必须由显式 Offering/BillingPolicy 表达。

## 后果

- 企业 owner 的成员管理从“选择并排序分组”改为“选择 Access Profile、能力、服务等级和预算”。
- 平台需要新增 Catalog、Offering、Entitlement、Profile、BillingPolicy 和 RouteConstraint 领域能力。
- 当前 Group 的产品、授权、计费和路由职责需要分阶段拆分；普通 Key 兼容会延长迁移期。
- 路由资格从每请求分组试错迁移到控制面预编译和数据面内存查找，正常请求只执行一个 primary。
- 对客计费与内部成本正式拆分，需要迁移现有 group/user rate multiplier 语义并建立价格冲突审计。
- 企业模型目录、请求准入、计费和 route planning 获得共同的产品身份与版本合同。
- 成员权限不再因增加账号、替换 provider、调整 pool 健康或迁移内部拓扑而隐式扩大或缩小。
- 本次改造范围为中大型领域迁移，不能以单个 handler guard 或一组新增表宣称完成。

## 非目标

- 本 ADR 不引入可登录员工子账号；是否允许成员登录仍遵守 ADR-0003。
- 本 ADR 不改变普通单分组 Key，除非后续独立 ADR 明确统一产品授权模型。
- 本 ADR 不允许企业 owner 查看真实上游账号、凭据、内部成本或管理员运维拓扑。
- 本 ADR 不规定具体负载均衡算法；实施设计可以在满足产品授权和有界 replay 不变量后选择 least-request、power-of-two choices、EWMA 或粘性策略。
- 本 ADR 不授权删除历史 `group_id`、usage、预算账本、Ops 或审计事实。

## 后续实施合同

本 ADR 经采纳后，必须先更新 [企业用户成员管理](../features/enterprise-member-management.md) 的完整目标设计，并新增独立的 Catalog/Offering/Entitlement/Access Profile/Route Table 实施与测试规格。ADR-0003 顶部应增加“部分被 ADR-0005 取代”的状态说明，但其成员实体、预算、Key、历史证据和不可登录边界继续有效。
