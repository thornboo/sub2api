# 企业用户成员管理——完整目标设计

- **日期**：2026-07-12
- **状态**：核心代码已实现，生产级验收收口中
- **决策记录**：[ADR-0003](../decisions/adr-0003-enterprise-member-entity.md)
- **取代**：[企业客户 Key 成员管理](./enterprise-key-member-management.md) 与 [ADR-0002](../decisions/adr-0002-key-as-enterprise-member.md) 的临时领域结论

本文是企业成员、聚合 Key、多分组路由、成员预算、迁移导入和成员用量的完整实现合同。它描述最终稳定状态，不是 MVP。实现者可以按依赖顺序拆分提交和验证，但完成口径以本文全部不变量和验收项为准，不得把后续章节降级为“以后再补”的非约束说明。

---

## 1. 背景与目标

### 1.1 当前问题

现有 `api_keys.group_id` 是单值。一名员工需要访问多个分组或多个协议平台时，企业 owner 只能创建多把 Key，并依靠名称或标签人为表达“这些 Key 属于同一个人”。这会造成：

- 一名员工的预算被拆散到多把 Key，无法形成可靠的成员级控制。
- 成员换 Key、增加备用 Key 后，历史和当前统计难以稳定归集。
- 多分组访问顺序与 fallback 没有成员级权威配置。
- 迁移外部网关时，员工、Key、分组和当月已用额度之间没有可审计的导入主体。

### 1.2 完整目标

1. 企业账号管理不可登录的内部成员。
2. 一名成员可持有多把 Key；一把成员 Key 可通过成员访问多个有序分组。
3. 所有受支持入口使用统一的请求级路由编排，并按实际执行分组完成协议分发、调度、计费和用量记录。
4. 企业余额、成员月预算和单 Key quota 分层生效；成员同步请求按实际用量授权并使用零金额回执，异步任务使用真实金额占用，同时保留幂等结算和跨月恢复能力。
5. 成员用量、迁移期开账和人工调整可审计、可对账、可重建。
6. 企业权限收回、成员禁用、分组变化和账号类型变化能够立即传播到鉴权缓存。
7. 普通用户与普通 Key 保持行为兼容。
8. 企业 owner 永远看不到上游账号、渠道、真实上游成本或管理员运营字段。

### 1.3 非目标

- 不创建可登录员工子账号。
- 不允许成员拥有独立平台余额、充值、支付或订阅主体。
- 不把成员明细暴露给平台管理员。
- 不用成员级模型白名单重复分组已有的模型与渠道能力；成员权限以分组为最小授权单位。
- 不导入外部请求明细并伪装成本站 usage log。
- 不用当前配置回算或重写历史用量证据。

---

## 2. 术语与身份边界

| 术语 | 定义 |
| --- | --- |
| 平台管理员 | `users.role=admin`，管理用户、分组、账号、定价和运营数据 |
| 普通用户 | `users.role=user, account_type=individual` |
| 企业账号 / 企业 owner | `users.role=user, account_type=enterprise`，仍是唯一登录、余额和结算主体 |
| 企业成员 | `enterprise_members` 中的不可登录管理实体 |
| 普通 Key | `api_keys.member_id IS NULL`，继续使用现有单值 `group_id` |
| 成员 Key | `api_keys.member_id IS NOT NULL`，`group_id` 必须为空，通过成员继承有序分组 |
| 绑定分组 | 企业 owner 显式授予某成员的分组 |
| 有效分组 | 成员绑定、企业当前授权、分组当前状态、入口能力和模型可调度性的实时交集 |
| ActiveGroup | 一次路由尝试使用的不可变请求级实际分组 |
| 逻辑请求 | 客户端发起的一次请求，可包含多个账号尝试和多个分组尝试，但只产生一次最终计费 |

`role` 与 `account_type` 是正交字段。企业账号不是第三种权限角色；管理员创建或升级企业账号时只改变产品能力，不改变 admin/user 权限语义。

---

## 3. 现有架构约束

- 后端：Go、Gin、Ent ORM、PostgreSQL、Redis、Wire。
- 数据库迁移使用 `backend/migrations/NNN_*.sql`，不使用 Ent 自动迁移。
- API Key 鉴权缓存当前保存单个 `GroupID` 和 `Group` 快照。
- `backend/internal/server/routes/gateway.go` 当前在进入具体 handler 前读取 `apiKey.Group.Platform`。
- 调度、粘性会话、订阅校验、用户倍率、计费、usage 和 ops 多处依赖当前 `apiKey.GroupID`。
- `api_keys.key` 全局唯一，已软删除 Key 也禁止复用。
- `usage_logs` 是追加型请求事实，用户侧和管理员侧对字段可见性有严格边界。
- 前端使用 Vue 3、Vite、TypeScript；企业成员界面必须沿用现有布局、表格、Modal、表单、日期范围、i18n 和 stone/neutral/emerald 视觉约定。

因此，本功能不能只增加两张表或只改前端。真正的架构改动是把“缓存 Key 上的单个 Group”转换为“鉴权主体 + 请求级 ActiveGroup”，同时保证普通 Key 沿用现有快速路径。

---

## 4. 核心领域模型

### 4.1 `users.account_type`

在 `users` 增加：

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `account_type` | string | `individual` / `enterprise`，默认 `individual` |
| `enterprise_disabled_at` | nullable timestamp | 企业能力被停用的时间；不删除历史 |

规则：

- `role` 继续只有 `admin` / `user`。
- 只有 `role=user` 可以启用 `account_type=enterprise`。
- 普通账号可以升级为企业账号。
- 尚未创建任何成员事实时可以撤销误操作升级。
- 一旦存在成员、成员 Key、成员预算或成员用量证据，不允许直接改回 `individual`；只能设置 `enterprise_disabled_at`，保留数据并让成员 Key 全部失效。
- 管理员用户列表可以设置和筛选账号类型，但管理员 API 不返回成员明细。

### 4.2 `enterprise_members`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigint PK | 成员内部 ID |
| `enterprise_user_id` | bigint FK → users.id | 所属企业账号 |
| `member_code` | varchar(100) | 企业内稳定标识；人工创建时可生成，导入时可使用工号/外部 ID |
| `name` | varchar(100) | 展示名，可修改，允许同名 |
| `status` | varchar(20) | `active` / `disabled` |
| `monthly_limit_usd` | decimal(20,8) | 自然月预算；0 表示不限 |
| `rate_limit_5h` | decimal(20,8) | 成员全部 Key 共享的 5 小时消费限额；0 表示不限 |
| `rate_limit_1d` | decimal(20,8) | 成员全部 Key 共享的 24 小时消费限额；0 表示不限 |
| `rate_limit_7d` | decimal(20,8) | 成员全部 Key 共享的 7 天消费限额；0 表示不限 |
| `version` | bigint | 成员与绑定配置的单调版本，用于缓存一致性 |
| `created_at` / `updated_at` / `deleted_at` | | 沿用 TimeMixin + SoftDeleteMixin |

约束：

- `(enterprise_user_id, member_code)` 全局唯一，包含已归档成员；可恢复归档期间不得复用。永久删除进入墓碑路径时，服务端先把墓碑替换为内部保留 code，再释放原 code。
- `member_code` 创建后不可通过普通编辑接口修改；确需纠错必须另立带原因和前后值审计的专用操作。
- 展示名不作为业务唯一键，避免同名员工无法建档。
- 成员仅归档后 `member_code` 不可复用；同一员工重新入职时优先恢复原成员或分配新的 code。owner 明确永久删除后可以复用原 code，但新成员会获得新 `member_id`，不会继承旧成员历史事实。
- 成员状态或版本变化必须失效该成员全部 Key 的认证缓存。

### 4.3 `api_keys`

新增 `member_id`，并建立租户一致性约束：

- 普通 Key：`member_id IS NULL`，`group_id` 保持现有语义。
- 成员 Key：`member_id IS NOT NULL`，`group_id IS NULL`。
- 成员 Key 的 `user_id` 必须等于成员的 `enterprise_user_id`。
- 成员 Key 继续保留 `quota/quota_used`、过期时间、IP 规则、滑动窗口限额、状态和标签。
- Key 明文仍受全局唯一约束；已软删除 Key 不可复用。

数据库层优先使用复合约束保证租户一致性：

```text
enterprise_members UNIQUE (id, enterprise_user_id)
api_keys FOREIGN KEY (member_id, user_id)
  REFERENCES enterprise_members(id, enterprise_user_id)
CHECK (member_id IS NULL OR group_id IS NULL)
```

若 Ent 无法完整表达，必须在手写迁移中建立，不能只依赖前端或 handler 校验。

### 4.4 `enterprise_member_group_bindings`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `member_id` | bigint FK | 成员 |
| `group_id` | bigint FK | 分组 |
| `sort_order` | int | 越小越优先 |
| `created_at` / `updated_at` | timestamp | 审计时间 |

约束与索引：

- 复合主键 `(member_id, group_id)`。
- `(member_id, sort_order, group_id)` 索引用于稳定排序。
- 同一成员 `sort_order` 可以在临时更新过程中重复，但保存完成后 service 必须归一化为连续序列；最终稳定排序以 `sort_order ASC, group_id ASC` 为准。
- 绑定表只表达 owner 意图；请求时仍必须与企业账号当前可用分组求交集。

### 4.5 `usage_logs` 成员证据

新增：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `member_id` | nullable bigint | 请求发生时的成员 ID |
| `member_code_snapshot` | nullable string | 请求时稳定成员 code |
| `member_name_snapshot` | nullable string | 请求时展示名 |

`group_id` 必须记录实际执行的 `ActiveGroup`，不能记录第一个候选或成员默认分组。成员改名、调序、停用或归档不改写历史 usage。

对 `member_id, created_at` 建复合索引。保留 ID 用于聚合，保留快照用于成员归档或永久清理后的审计展示。

### 4.6 成员月度预算投影

新增 `enterprise_member_budget_periods`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `member_id` | bigint FK | 成员 |
| `period_start` | date | 按站点计费时区计算的自然月首日 |
| `used_usd` | decimal(20,8) | 已结算用户应付金额 |
| `reserved_usd` | decimal(20,8) | 异步任务在途金额占用；普通同步请求不增加该值 |
| `version` | bigint | CAS/并发更新 |
| `updated_at` | timestamp | |

唯一约束：`(member_id, period_start)`。

该表是控制面投影，可由预算账本重建，不是唯一事实来源。

### 4.7 成员请求回执与异步金额占用

新增 `enterprise_member_budget_reservations`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `request_id` | string unique | 逻辑请求幂等键 |
| `member_id` | bigint FK | 成员 |
| `group_id` | nullable bigint FK | 首次授权分组；最终成功分组以 usage 为准 |
| `request_payload_hash` | varchar(64) | 请求体语义哈希，防止 request ID 被不同请求复用 |
| `period_start` | date | 所属自然月 |
| `reserved_usd` | decimal(20,8) | 普通同步请求为 0；异步任务为明确预计占用金额 |
| `actual_usd` | decimal(20,8) | 最终结算金额 |
| `status` | string | `reserved` / `settled` / `released` / `expired` / `ambiguous` |
| `usage_log_id` | nullable bigint unique | 成功后关联 usage |
| `expires_at` | timestamp | 崩溃恢复截止时间 |
| `outcome_reason` | varchar(64) | 结算、释放或结果不明的机器可读原因 |
| `reconcile_attempts` | integer | 后台核对次数 |
| `last_reconcile_at` | nullable timestamp | 最近一次后台核对时间 |
| `created_at` / `updated_at` | timestamp | |

### 4.8 成员预算账本

新增 `enterprise_member_budget_entries`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigint PK | |
| `member_id` | bigint FK | |
| `period_start` | date | |
| `kind` | string | `usage` / `migration_opening` / `manual_adjustment` / `reconciliation` |
| `amount_usd` | decimal(20,8) | 可正可负；usage 必须为正 |
| `usage_log_id` | nullable bigint unique | usage 类型必填 |
| `idempotency_key` | string unique | 防重复入账 |
| `actor_user_id` | nullable bigint | 人工或导入操作人 |
| `note` | string | 原因和来源 |
| `created_at` | timestamp | 不更新、不软删除 |

预算投影必须等于该成员当月有效账本之和；迁移当月已用额度写 `migration_opening`，不写伪造 usage log。数据库层拒绝账本行的删除和事实字段更新；唯一允许的补链是 `usage` 行从空值一次性写入与同一成员、同一预算请求匹配的 `usage_log_id`，幂等的无变化更新仍可安全重放。

---

## 5. 生命周期与删除语义

### 5.1 企业账号

- 升级为 enterprise：保留原普通 Key 及其现有分组、额度和状态；“API 密钥”与“成员管理”同时保留，普通 Key 无需迁移并继续作为企业 owner 自己的 Key 使用。
- 停用企业能力：成员 Key 全部拒绝，普通 Key 按原状态处理；成员和历史证据保留。
- 删除企业用户：沿用用户删除策略；成员、Key 和 usage 的证据保留边界不得弱于现有用户/Key 证据策略。

### 5.2 成员

- `active`：成员 Key 可鉴权。
- `disabled`：成员 Key 立即拒绝，但成员、Key 和用量仍可查看。
- 软删除/归档：从默认列表隐藏，全部 Key 失效；成员资料、预算/用量、账本、审计和 Key 清单保持只读可查，不允许再编辑成员、创建或修改 Key、调整历史用量。归档可恢复，恢复后固定为 `disabled`，由 owner 核对分组后显式启用。
- 永久删除：只能对已归档成员执行，并且对 owner 始终可用。服务端在同一事务内持锁复检全部历史事实；没有 Key、预算账本、预留、usage/错误日志、迁移基线或异步任务事实时物理删除；存在任何事实时，注销现有成员 Key、删除当前分组绑定并把成员转为 owner 不可见的审计墓碑。两种路径对 owner 都表现为成员已永久删除。
- 墓碑会替换成员编号和名称，原成员编号可以再次使用；已有 usage 快照、预算账本、迁移基线和审计记录仍保留原始事实。普通产品删除不等同于隐私合规擦除，后者如需支持必须另行定义整组证据策略。
- 墓碑不出现在成员列表、归档列表或用量页的成员筛选目录；账号总用量仍包含其历史事实，历史请求继续使用请求发生时的成员快照展示。
- 改名不影响 `member_code` 和历史聚合。

### 5.3 Key

- Key 吊销、软删除、过期、quota 用尽和 IP 规则继续使用现有状态机。
- 归档不会删除 Key；永久删除进入墓碑路径时，当前成员 Key 会被停用并软删除，历史 Key 与请求关系继续保留。
- Key 不允许跨成员或跨企业静默移动。成员内轮换应创建新 Key、吊销旧 Key；确需迁移时使用显式审计操作。

---

## 6. 授权与租户隔离

### 6.1 管理面授权

所有企业成员 API 从登录主体取得 `enterprise_user_id`，不接受客户端传入 owner user ID。service/repository 查询必须同时带：

```text
member.id = :member_id
AND member.enterprise_user_id = :authenticated_user_id
AND member.deleted_at IS NULL
```

成员 Key 操作还必须同时验证 Key 的 `user_id` 与 `member_id`。

### 6.2 分组授权

成员有效分组为：

```text
member_bindings
∩ enterprise_user_current_allowed_groups
∩ currently_active_groups
∩ endpoint_capable_groups
∩ model_schedulable_groups
```

需要同时具备：

- 写入时校验：owner 不能保存超出当前权限的绑定。
- 请求时交集：防止管理员撤权后旧绑定继续生效。
- 撤权传播：管理员修改企业账号 allowed groups 后，增加用户授权版本并失效该用户全部成员 Key 缓存。

订阅型分组继续执行现有订阅资格、窗口与套餐校验；成员不会绕过企业账号的订阅资格。

### 6.3 管理员可见性

管理员可以看到：

- 企业账号类型与企业能力状态。
- 企业账号总余额、总用量和现有管理员用户分析字段。

管理员默认不可以看到：

- 成员列表、名称、code。
- 成员预算、成员 Key 归属。
- 成员级用量拆分。

管理员排障若未来需要成员信息，必须另立决策并建立审计权限，不能复用普通 admin DTO 顺手暴露。

---

## 7. 请求级路由架构

### 7.1 分层

```text
APIKeyAuth
  -> MemberAuthorization
  -> EndpointModelExtractor
  -> CandidateGroupResolver
  -> MemberRequestOrchestrator
       -> ActiveGroup attempt
       -> protocol dispatcher
       -> existing group/account scheduler
       -> upstream forward
       -> retry classification
  -> one logical billing/usage finalization
```

普通 Key 保留现有单 Group 快速路径；成员 Key 进入 orchestrator。二者最终都向下游提供统一的请求级 `ActiveGroupContext`。

### 7.2 模型与入口解析

模型提取器按入口定义，不假设模型总在 JSON body：

- Chat Completions、Messages、Responses：从规范化请求体提取。
- Gemini `/v1beta/models/*modelAction`：从 URL path 提取模型和 action。
- Embeddings、Images、Videos、Batch：按各自 schema/path 提取模型或能力类型。
- WebSocket：HTTP upgrade 前完成身份与成员候选鉴权；upgrade 后读取第一条 `response.create`，在打开任何上游连接前按其中模型选择初始分组。首个上游 turn 一旦提交后固定分组。
- `/models`：不选单个模型，返回成员有效分组能力的安全并集。

请求体只能读取一次时，middleware 必须使用现有可重放 body 机制；不得让前置解析消耗 handler body。

### 7.3 候选分组资格

候选判断不是静态字符串匹配。统一资格函数必须考虑：

- 成员绑定与企业当前授权。
- 分组 active 状态和入口平台能力。
- 请求模型经过 alias/mapping 后是否能在该分组调度到账号。
- 渠道定价/限制、模型路由、mixed scheduling、隐私要求。
- 订阅资格和分组级功能开关。
- 媒体、batch、messages dispatch 等入口专属能力。

资格探测应复用调度层的权威能力判断，避免 UI 展示列表、`supported_model_scopes` 和真实调度各自形成不同真相。动态容量不足不等于模型不支持：前者允许尝试下一个候选，后者应从候选中排除。

### 7.4 排序

- 主顺序：`member_group_bindings.sort_order ASC`。
- 稳定次序：`group_id ASC`。
- 同一逻辑请求中每个实际分组最多尝试一次。
- 管理员或调度器不能隐式重排成员配置；分组内部仍可按既有策略选择账号。

### 7.5 ActiveGroup

每次尝试创建不可变的 `ActiveGroupContext`，至少包含：

- logical request ID / attempt ID。
- member ID、member version。
- actual group ID、platform、rate multiplier 和订阅语义。
- endpoint、requested model、mapped model。
- 当前候选序号和已尝试分组集合。

下游调度、协议 handler、计费、粘性会话、usage、ops 和错误响应都读取 ActiveGroup，不修改缓存中的 API Key 对象。

### 7.6 failover 层级

顺序固定：

1. 当前分组内部按现有逻辑进行账号选择、账号 failover 和等待。
2. 当前分组被证明不可用且响应尚未提交时，orchestrator 判断是否切下一个成员候选。
3. 进入下一个分组时创建新的 ActiveGroup 和 attempt ID，保留同一 logical request ID。

允许跨分组的错误：

- 当前分组无可调度账号，但模型/入口本身有效。
- 上游 429、连接失败、超时。
- 明确可重试的 502/503/504 或平台临时 5xx。
- handler 返回的 typed capability mismatch，且另一个候选可以保真处理。

禁止跨分组的错误：

- API Key、企业账号或成员无效。
- 企业余额不足、成员预算不足、Key quota/限流不足。
- 客户端 schema/参数错误。
- 本地安全策略、内容策略或权限拒绝。
- 已经提交业务响应、首个 WebSocket 上游 turn、SSE data 或任何 body 字节；WebSocket 的 HTTP 101 本身不视为上游任务已提交。
- 请求取消。

通用 500 不能仅凭状态码自动 fallback；必须由 typed error 明确标记 retryable，避免业务 bug 被重复执行。

### 7.7 响应提交边界

跨分组 fallback 的硬条件是响应尚未提交：

- 非流式：在确定当前尝试成功前不向客户端写 body。
- 流式：允许缓冲上游 headers 和首个有效事件；向客户端写出首字节后锁定分组。
- WebSocket：HTTP 101 后仍可在“读取首帧但尚未打开/提交上游 turn”的窗口内选择或更换候选；首个上游 turn 提交后锁定。
- handler 必须返回结构化 attempt result，不得在失败后再由外层猜测 writer 状态。

### 7.8 现有分组 fallback 的交互

- 成员候选列表是权限边界和主顺序。
- `fallback_group_id`、`fallback_group_id_on_invalid_request` 或 Claude Code 限制解析出的目标，只有在目标也属于本请求有效分组集合时才可使用。
- 解析后的分组加入已尝试集合并去重。
- 任何 fallback 环、重复分组或超过候选上限都立即停止并记录 ops 错误。
- 客户端参数错误不因旧 `fallback_group_id_on_invalid_request` 绕过本文的“不 fallback”规则；确需兼容的入口必须有显式 typed exception 和测试。

### 7.9 入口能力矩阵

| 入口 | 成员多分组 | 跨分组 fallback | 关键边界 |
| --- | --- | --- | --- |
| `/v1/messages` | 是 | 提交首字节前 | 可在 Anthropic/OpenAI 兼容 handler 间分派 |
| `/v1/chat/completions` | 是 | 提交首字节前 | 保留协议转换 |
| `/v1/responses` 及别名 | 是 | 提交首字节前 | typed capability mismatch 可换组 |
| Responses WebSocket | 是 | 首个上游 turn 提交前 | 首帧按模型选组；每个 `response.create` 创建独立零金额回执；连接提交上游后固定分组 |
| Gemini `/v1beta/models/*` | 是 | 提交首字节前 | 模型从 URL path 提取 |
| `/embeddings` | 是 | 写响应前 | 只选择支持 embeddings 的平台/分组 |
| `/images/*` | 是 | 任务/响应提交前 | 创建外部任务后不得换组 |
| `/videos/*` | 是 | 创建任务前 | 状态查询按原任务分组，不重新选路 |
| Batch image | 是 | Job、余额冻结和外部任务创建前 | 仅选择启用 batch 的 Gemini 分组；Job 保存实际分组/成员快照；余额冻结与成员预算预留同事务，异步捕获/释放同事务 |
| `/models` | 返回并集 | 不适用 | 去重并只暴露用户安全能力 |
| `count_tokens` | 是 | 写响应前 | 只选择能够保真计数的候选 |

所有新入口接入网关时必须显式声明成员路由能力，禁止默认继承“支持”。

---

## 8. 计费、预算与用量一致性

### 8.1 金额口径

成员预算使用“企业用户最终实际应付金额”，与企业余额扣减口径一致。不得使用：

- 上游账号成本 `account_cost`。
- 管理员利润或成本差。
- 未应用用户/分组倍率的基础价格。

`usage_logs` 中用于用户侧展示的字段与成员预算必须保持同一口径；具体字段名由现有计费结果映射，但合同语义固定。

### 8.2 自然月

- 周期按站点统一的计费/报表时区计算，默认 `Asia/Shanghai`。
- 时区必须来自单一配置，不允许前端自行决定预算周期。
- `period_start` 保存该时区自然月首日。
- 时区配置变化不得重写已创建周期；历史周期保留创建时的时区语义。

### 8.3 请求前授权、同步回执与异步占用

调用前按以下顺序：

1. 校验企业账号、成员、Key 和有效分组。
2. 校验企业余额与现有 Key quota/限流。
3. 在事务中锁定成员及当前投影，并按当前实际已结算用量原子执行所有已启用的成员级约束：

```text
used_usd < monthly_limit_usd
usage_5h < rate_limit_5h
usage_1d < rate_limit_1d
usage_7d < rate_limit_7d
```

4. 为普通同步请求创建幂等、零金额 durable receipt；receipt 不占用客户预算。
5. 授权和 receipt 创建成功后才允许进入上游尝试。

任一限额为 0 只表示该窗口不限。当前实际用量达到或超过任一已启用限额时拒绝后续请求；尚未耗尽时不计算理论最大费用，也不因模型映射、候选分组倍率或缺少输出上限而提前拒绝。最后一笔或少量并发请求可以造成有限超额，随后由已超限投影阻止新请求。

模型查询、用量查询、`count_tokens` 与异步任务状态查询不产生对客费用，不创建成员预算 receipt。Batch image、异步图片和视频生成/编辑/扩展等在 HTTP 响应后仍继续产生费用的任务，必须根据明确预计任务费用创建正金额 hold，并继续使用实际用量加在途 hold 的严格授权。

### 8.4 异步任务 hold 计算

- 普通文本、embedding、同步图片和 Responses WebSocket turn 不计算金额上界，只创建零金额 receipt；成功后按实际费用结算。
- Batch image 使用提交时价格快照与 hold 金额作为成员预算占用；异步任务将预算 reservation ID、实际分组与成员快照持久化到 Job。
- 异步图片和视频按任务尺寸、数量、时长、模型价格和倍率计算明确预计费用；无法获得可靠有限金额时，不能在启用了成员限额的情况下创建异步任务。异步图片 Redis 快照必须持久化预算 receipt、成员、实际分组、冻结金额、执行阶段和恢复期限；不得只依赖进程内 goroutine。
- 异步图片任务接口只把 `task_hold_usd` 称为“任务冻结金额”，并返回 `held/settled/released/needs_review` 状态。实际用量未耗尽、但当前任务估算加既有冻结无法放入剩余额度时，返回独立的 `ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE`，不得伪装为“已用完”。
- 候选分组可能有不同倍率时，异步 hold 按所有可尝试候选中的最高应付金额，结算后释放差额。
- Responses WebSocket 不为整条长连接创建一个不可解释的占用；每个 `response.create` 使用独立请求 ID、请求体和模型创建零金额 receipt。成功 turn 按真实用量结算；明确未到达上游或上游返回结构化拒绝的失败才能释放。`response.create` 一旦进入上游写入边界，写入超时/断连、读取中断或缺失 terminal 事件都必须转为 `ambiguous` 待对账，不得换连或回退 HTTP 重放。
- 只有能证明尚未触发上游推理副作用的组内失败才允许切换到下一授权分组；裸 `408/502/503/504`、首包超时和请求写入后的连接中断不能自动跨组重放。

### 8.5 成功结算

最终成功后，usage 写入与成员预算结算进入同一幂等计费事务或同一可靠 outbox 消费：

1. 创建/确认 usage log，写实际 member/group 快照。
2. 以 `usage_log_id` 或现有 billing dedup identity 插入唯一 `usage` 预算账本。
3. 正金额异步 hold 从 `reserved_usd` 释放占用；零金额同步 receipt 不改变该投影。
4. `used_usd` 增加实际应付金额。
5. receipt/hold 标记 `settled` 并关联 usage log。
6. 企业余额和 Key quota 按现有可靠计费链路扣减。

重复 worker、重试或崩溃恢复不能重复扣费或重复增加成员用量。

实现上，成员用量在进入统一计费事务前先写入版本化 settlement outbox。outbox 保存重放所需的标量计费事实和 usage log 快照，但禁止序列化已水合的用户、Key、上游账号、分组或订阅对象，避免把 Key 明文和关系图写入恢复载荷；数据库复合外键同时证明 Key、成员与 enterprise owner 属于同一租户，repository 还会校验命令与 usage 快照的 owner、Key、成员和 request 身份完全一致。同一幂等键仅在 request fingerprint 与完整 command payload 都相同时允许重放，既有载荷不会被后续请求覆盖。只有企业余额、Key quota、usage log、成员预算账本和 reservation 状态全部在同一事务提交后，才删除对应 outbox 行。进程重启或临时数据库错误留下的行由恢复任务幂等重放。

同步请求的实际费用可以使成员从“仍有余额”跨越到有限超额；系统必须按真实费用原子写入 usage、账本和投影，后续请求由已经达到或超过限额的投影正常阻断。若异步任务实际费用超过明确 hold，必须额外记录运维告警，但同样不能回滚已经发生的请求事实。

### 8.6 失败与取消

- 所有分组均失败且不计费：释放 receipt/hold，状态为 `released`；零金额同步 receipt 的释放不改变客户预算。
- 客户端断开但上游已产生可计费用量：按现有计费事实结算，不因客户端断开丢账。
- worker 崩溃留下的 `reserved` 由恢复任务按 `expires_at` 检查实际 usage/billing 事实：证据完整则结算；能够证明请求未执行才释放；结果无法证明则转为 `ambiguous`。只有正金额异步 hold 会继续占用预算，零金额同步 receipt 只保留恢复和审计证据。
- 上游异步任务已经创建、但本地任务路由持久化失败时，结果立即转为 `ambiguous`；不得切换账号或分组重放，也不得按普通失败自动释放预留。
- Batch image 在调用 provider 前必须先持久化 `provider_submitting`，把外部副作用边界与纯上传准备阶段分开。进程若在远端创建结果落库前中断，stale recovery 只能把该状态转为 `submission_unknown`，并继续保留企业余额 hold 与成员预算占用，绝不能按“未提交”自动退款。`submission_unknown` 不自动重提、不允许取消退款；必须依据本地 batch ID/展示名、provider、账号和提交时间人工核对。
- Batch image 已经完成外部工作、但本地 capture/usage 结算连续失败时，Job 保持 `settling` 并保留余额冻结与成员预算占用；超过快速重试次数后改为低频重试和运维介入，不能把已完成的工作改成普通失败并退款。
- `ambiguous` 必须持久化原因、核对次数和最近核对时间，供管理员告警和后续人工审计；不得仅依赖进程内计数。
- `ambiguous` 的成功结算只能由可验证的 usage/billing 事实经统一计费事务完成，确保企业余额、Key quota、usage log 与成员预算原子一致；管理员只可在确认请求未形成可计费事实后人工释放，不能通过独立“手工结算”绕过统一计费链。
- 不允许定时任务无条件释放所有过期预留。

### 8.7 迁移期开账与人工调整

- 导入的当月已用额度写 `migration_opening` 账本。
- 手工创建成员也可以填写当前 5h/1d/7d/月初始已用额度，无需额外填写开账说明。成员、分组绑定、月度 `migration_opening` 账本、窗口投影和带稳定系统来源的 before/after 审计必须在同一事务内提交，失败时整体回滚。
- 管理员或企业 owner 的授权调整写 `manual_adjustment`，必须有 actor、reason 和幂等键。
- owner 可以在成员编辑流程中输入各窗口与自然月的目标已用金额；自然月差额写不可变 `manual_adjustment` 账本，窗口投影在同一事务内更新，并额外写入包含 before/after、稳定系统来源说明和幂等键的不可变审计事件。
- 客户预算页面分别展示：预算、实际已用、剩余、使用率；发生有限超额时展示超出金额和“后续请求已停止”。普通同步请求的 receipt 和历史 `reserved_usd` 不作为客户消费展示。
- usage 图表只统计真实请求；预算进度使用账本合计，两者差异可以解释和下钻。

### 8.8 对账

提供周期性对账：

- `usage` 账本金额与关联 usage log 用户应付金额一致。
- budget period 的 `used_usd` 等于当月预算账本之和。
- settled reservation 必须关联唯一 usage。
- released/expired reservation 不得残留 `reserved_usd`。

投影差异修复写 `reconciliation` 账本和审计日志，不直接静默覆盖；若发现已经存在真实 usage log、但请求预算账本因崩溃缺失，则补写与该 `usage_log_id`、请求 ID 唯一关联的 `usage` 账本，并使用稳定的 `reconciliation:usage:*` 幂等键和说明保留恢复来源。这样金额、请求数和 Token 汇总可以从同一请求事实重建，且不会把恢复事件重复算成第二次请求。

---

## 9. 鉴权缓存与一致性

### 9.1 快照结构

成员 Key 缓存至少保存：

- Key 状态、quota、过期、IP 与限流配置。
- 企业账号状态、account type、授权版本和当前 allowed groups。
- 成员 ID、code、name、status、monthly limit、version。
- 有序绑定分组及路由需要的安全快照。
- snapshot schema version。

`monthly used` 与异步任务 `reserved` 不作为长 TTL 静态快照；预算授权必须走原子预算存储或短期一致缓存。

### 9.2 失效触发

下列变化必须失效相关 Key：

- 企业账号状态、account type、enterprise disabled、allowed groups、订阅资格。
- 成员状态、软删除、预算、version。
- 成员分组增删或排序。
- Key 状态、quota、过期、IP、限流。
- 分组状态、平台、订阅类型、路由、模型能力或倍率。

当前按 `api_keys.group_id` 查 Key 的分组失效逻辑无法覆盖 `group_id=NULL` 的成员 Key。实现必须增加“group → member binding → member keys”的反向失效，或使用用户/成员/分组版本在读取时拒绝旧快照。

### 9.3 一致性原则

- 权限收回必须强一致或版本拒绝旧缓存。
- 名称和展示字段允许短暂最终一致，但不能影响授权。
- 缓存异常时 fail closed：回源数据库或拒绝，不使用过期成员权限继续调用。

---

## 10. API 合同

所有企业 API 位于用户认证域，并要求 `account_type=enterprise`：

### 10.1 成员

```text
GET    /api/v1/enterprise/members
POST   /api/v1/enterprise/members
GET    /api/v1/enterprise/members/:id
PATCH  /api/v1/enterprise/members/:id
POST   /api/v1/enterprise/members/:id/disable
POST   /api/v1/enterprise/members/:id/enable
DELETE /api/v1/enterprise/members/:id
```

创建请求可以同时携带当前周期初始用量：

```json
{
  "member_code": "finance.ops-01",
  "name": "财务运营",
  "rate_limit_5h": 25,
  "rate_limit_1d": 50,
  "rate_limit_7d": 75,
  "monthly_limit_usd": 100,
  "usage_5h": 5,
  "usage_1d": 10,
  "usage_7d": 20,
  "monthly_used_usd": 30,
  "group_ids": [8, 3]
}
```

任一初始已用值非零时，后端自动写入稳定的创建来源说明和 before/after 审计。自然月初始已用写 `migration_opening`，不生成伪造 usage log；5h/1d/7d 初始已用建立当前窗口起点。整个创建请求受同一 `Idempotency-Key` 和数据库事务保护。

### 10.2 分组绑定

```text
GET /api/v1/enterprise/members/:id/groups
PUT /api/v1/enterprise/members/:id/groups
```

PUT 使用完整替换和版本号，避免并发拖拽排序覆盖：

```json
{
  "expected_version": 12,
  "group_ids": [8, 3, 15]
}
```

版本冲突返回 409，前端重新加载后提示用户。

### 10.3 成员 Key

```text
GET    /api/v1/enterprise/members/:id/keys
POST   /api/v1/enterprise/members/:id/keys
POST   /api/v1/enterprise/members/:id/keys/:key_id/reveal
PATCH  /api/v1/enterprise/members/:id/keys/:key_id
DELETE /api/v1/enterprise/members/:id/keys/:key_id
```

成员 Key 列表只返回脱敏值，避免打开管理弹窗时批量暴露全部凭据；当前成员的 owner 主动点击复制时，前端通过成员专用 `reveal` 接口按需读取单把 Key。该接口同时限定认证 owner、成员和 Key，普通 Key 详情接口继续拒绝成员 Key；应用服务必须先写入不含凭据的 append-only `member_key.reveal_authorized` 审计，审计失败则不得返回明文。该动作只表示服务端已授权并准备返回，不声称客户端一定成功接收。成功响应只包含 `id`、`member_id` 和 `key`，并设置 `Cache-Control: no-store`。创建成功响应仍会立即展示一次明文，审计与用量日志永不返回完整 Key；已归档成员不提供复制入口，恢复后才能重新读取。

当前专用读取接口与普通 Key 明文详情保持同一认证基线：要求有效 owner 登录态并记录通用审计与企业成员读取审计，不额外强制 TOTP step-up。若未来把明文凭据读取提升为 step-up 操作，必须同时覆盖普通 Key 与成员 Key，避免两类凭据形成强弱不一致的旁路；专用读取限流也应作为统一凭据管理安全策略设计，而不是只给成员 Key 增加单侧限制。

早期版本的 `GET /:id/adoptable-keys` 与 `POST /:id/keys/:key_id/adopt` 暂时保留为传输层兼容接口，但企业成员控制台不再提供迁移入口，新客户端不得依赖它们。普通 Key 与成员 Key 是两类并行凭据：前者继续绑定自己的固定分组，后者通过成员的有序可访问分组路由。

### 10.4 用量与预算

```text
GET /api/v1/enterprise/members/usage/summary
GET /api/v1/enterprise/members/usage/trend
GET /api/v1/enterprise/members/:id/usage
GET /api/v1/enterprise/members/:id/usage/analytics
GET /api/v1/enterprise/members/:id/usage/records
GET /api/v1/enterprise/members/:id/budget
GET /api/v1/enterprise/members/:id/budget/entries
POST /api/v1/enterprise/members/:id/budget/adjustments
PUT /api/v1/enterprise/members/:id/usage
GET /api/v1/enterprise/members/audit
GET /api/v1/enterprise/members/:id/audit
```

`usage/summary` 为当前自然月的企业成员汇总与成员逐项投影；`usage/trend` 接受 `days=1..365`。`GET /:id/usage` 与 `/:id/usage/analytics` 返回只含对客费用、请求模型和公开分组的成员分析；`PUT /:id/usage` 以绝对目标值调整 5h/1d/7d/月已用金额，必须携带 `Idempotency-Key`，审计说明由后端自动补充，旧客户端传入的 `note` 仍可兼容。`/:id/usage/records` 使用 `(member_id, created_at)` 索引做 owner/member 双重限定的精确分页，只投影请求 ID、Key 名称、对客模型、公开分组、请求类型、token、耗时、对客费用和客户端入口；不复用包含 account/channel 字段的管理员 DTO。独立的人工调账仍必须携带 `Idempotency-Key`、非零 `amount_usd` 与审计 `note`，写入不可变 `manual_adjustment` 账本，且负向调账不能把周期已用金额扣成负数。

企业 owner DTO 只返回用户应付金额、token、请求数、成员、Key、请求模型和公开分组信息。禁止返回 account、channel、provider endpoint、account_cost、利润和真实上游路由细节。

### 10.5 导入

```text
GET  /api/v1/enterprise/members/import/template?format=csv|xlsx
POST /api/v1/enterprise/members/import/preview
POST /api/v1/enterprise/members/import/commit
GET  /api/v1/enterprise/members/import/jobs/:id
POST /api/v1/enterprise/members/import/jobs/:id/result-secrets
GET  /api/v1/enterprise/members/import/jobs/:id/error-report
```

preview 返回服务端生成的短期确认令牌和规范化预览；commit 只接受令牌、有效行选择和幂等键，不接受客户端重新提交任意解析结果。commit 将任务持久化为 `queued` 后立即返回，前端轮询 job；worker 使用 `FOR UPDATE SKIP LOCKED` 领取，并可在旧 processing 租约超时或历史异常记录缺少 `locked_at` 时跨实例重领。领取返回的 `lock_owner` 是写入 fencing token：只有当前租约持有者可以提交结果或标记失败，已被接管的旧 worker 即使迟到也不能创建成员、覆盖状态或清除新租约。领取和处理使用独立 timeout；处理窗口内按租约 TTL 的三分之一持续续租，单次短暂续租错误不会立刻放弃任务，确认所有权丢失或错误持续超过租约期限时才取消当前处理。默认处理上限 15 分钟，避免原固定 2 分钟窗口令合法大导入反复超时。

错误响应使用稳定 typed code，至少包括：

- `enterprise_account_required`
- `enterprise_account_disabled`
- `member_not_found`
- `member_disabled`
- `member_version_conflict`
- `member_group_not_allowed`
- `model_not_available_for_member`
- `member_budget_exceeded`
- `member_budget_unbounded_request`
- `all_member_groups_failed`
- `import_preview_expired`
- `import_conflict`

---

## 11. 前端产品与交互

### 11.1 信息架构

- 普通用户：保留“API 密钥”。
- 企业账号：主导航同时显示“API 密钥”和“成员管理”；普通 Key 继续在“API 密钥”中独立管理，成员 Key 在对应成员详情中管理，二者不要求相互迁移。
- 管理员：用户创建/编辑增加“账号类型”；不增加成员列表入口。账号类型与角色选择必须复用二开共享 `Select`，企业能力开关必须复用共享 `BaseCheckbox`，不得回退为浏览器原生 select/checkbox。

### 11.2 成员列表

展示：

- 成员名和不可变成员编号。
- active/disabled/archived 状态。
- 5h/1d/7d/月限额、对应实际已用金额、真实请求用量、迁移/调整和可用余额；同步 receipt 与异步 hold 不混入客户已用金额。
- Key 数、有效分组数、最后使用时间。

支持搜索、状态筛选、预算风险筛选、排序、成员范围、批量启用/禁用和导入。状态、预算风险、排序与成员范围必须复用二开共享 `Select` 控件及其暗色触发器、浮层、选中态与键盘交互，不使用浏览器原生 select。成员范围使用“仅当前成员 / 包含已归档”：只有选择包含已归档后，状态筛选才提供“已归档”，切回当前成员时必须自动退出已归档状态并重新加载，不能由互相矛盾的按钮与状态筛选共同控制。当前成员的删除操作执行归档；归档成员固定提供“恢复”和“删除成员”。列表返回服务端基于全部历史事实计算的删除策略提示，用于解释物理删除或证据保留语义，但删除权限不受该提示限制；永久删除事务仍须持锁复检，不能信任列表快照。

批量成员编辑分为两个明确入口：

1. **批量编辑成员策略**：可以选择性修改自然月预算、5h/1d/7d 成员限额、状态和有序可访问分组。每个限额字段必须先显式勾选才参与更新；未勾选字段保持原值，不能把表单默认 `0` 误解释为批量不限额。分组模式为保持、覆盖或追加；覆盖为空会停用成员，启用时最终分组集合必须非空。
2. **批量调整已用额度**：只接受作用于当前值的有符号差额，分别覆盖自然月、5h、1d、7d 投影。5h/1d/7d 窗口如果已经到期，必须先按当前有效值 `0` 计算差额，不能把过期投影带入新窗口。该操作与普通策略编辑分离，不改写 `usage_logs`，不得把任何成员的投影扣成负数，并在确认前显示成员数和合计差额。

两类批量写入都必须携带每个成员的 `expected_version`，服务端最多接受 500 个成员，按成员 ID 固定顺序持锁，在同一数据库事务内复检 owner、成员状态、版本和最终分组授权；“保持现有分组”同样必须复核现有分组仍在 owner 权限内。任何目标失败时整批回滚，不允许用前端循环单成员接口模拟批量成功。所有金额与窗口值必须处于数据库 `NUMERIC(20,8)` 的安全范围，当前上限为 `999999999999.99`。批量用量调整还必须使用非空的整批 `Idempotency-Key`，为每个成员写入独立的自然月账本差额和 before/after 审计证据。前端在响应结果不确定时必须冻结并复用同一组成员版本、请求内容和幂等键；只有取得权威成功结果、明确放弃或修改操作内容后才能创建新操作。批量接口成功响应只返回实际更新数量，避免把最多 500 行完整成员快照写入幂等响应存储。

导入的 API Key 会成为真实认证凭证，因此复用普通自定义 Key 的安全规则：长度至少 16 个字符、导入上限 128 个字符，只允许英文字母、数字、下划线和连字符。权威预览必须区分“长度不足”“长度超限”“包含非法字符”“文件内重复”和“系统中已存在”，并对长度错误返回限制值与当前值；不得把这些原因压缩成无法操作的统一“格式无效”。旧预览中的 `invalid_api_key` 仍保留兼容显示。

成员列表属于高密度管理界面，不使用成员卡片墙。桌面端必须使用一行一个成员的数据表，稳定呈现选择、成员名、成员编号、状态、预算/本月已用、Key 数、分组数、有序路由、更新时间和操作；成员名与成员编号、Key 数与分组数分别使用独立列，不在同一单元格纵向堆叠。桌面数据行采用紧凑垂直密度，金额不得因固定宽度以省略号截断，整组操作按钮必须保持在同一行。表头和数据行选择控件必须复用二开共享表格选择样式，不得回退为浏览器原生白色 checkbox；表头选择只作用于当前筛选结果，并具备全选、部分选择、取消全选、键盘 Space 和读屏 mixed 语义。窄屏不复制桌面横向表格，也不退化为相互分离的大卡片，而是在同一列表容器内使用连续紧凑行，保留完整金额、路由顺序和全部操作。

### 11.3 成员详情

固定分区：

1. 基本资料与状态。
2. 成员 5h/1d/7d/月限额、已用调整和账本说明。
3. “成员可访问的分组”授权与调用优先级；选择范围来自企业 owner 当前可访问分组。
4. 成员 Key。
5. 用量趋势、模型、分组和请求记录。
6. 审计记录。

分组拖拽必须同时提供键盘可操作的“上移/下移”按钮。保存使用 expected version，冲突时不静默覆盖。

### 11.4 关键交互状态

- Loading：使用骨架屏，操作按钮禁用。
- Empty：区分“尚无成员”“筛选无结果”“成员无 Key”“成员无授权分组”。
- Error：保留用户输入，展示 typed error 的可操作说明。
- Saving：防重复提交；创建与导入使用 idempotency key。
- Disabled：明确说明成员 Key 会立即失效，历史不会删除。
- Budget exhausted：展示周期、预算、实际已用、超出金额、后续请求已停止和下次重置时间。
- Partial import errors：预览逐行显示，commit 后提供可下载结果。
- Slow network：长导入改为 job 状态轮询，不让浏览器请求无限等待。

### 11.5 可访问性与响应式

- 目标为 WCAG 2.1 AA。
- 表格、Modal、拖拽、排序、进度条和错误摘要具备键盘与屏幕阅读器语义。
- 预算不能只靠颜色表达，必须有数值和状态文字。
- 桌面使用表格/详情双层；窄屏切换为同一列表容器内的紧凑行和分区详情，不依赖横向拖拽，也不使用成员卡片墙。
- 所有文案进入 zh/en i18n，不在组件中硬编码。

---

## 12. CSV/XLSX 导入

### 12.1 权威解析边界

- CSV 与 XLSX 都由服务器产生权威解析结果。
- 导入对话框必须提供带完整动作名称的“下载 CSV 模板”和“下载 XLSX 模板”入口，并显示下载中状态；不能只显示难以理解的 `CSV` / `XLSX` 缩写按钮。
- 新下载的模板使用中文文件名和中文列名；服务端同时接受中文列名和历史英文列名，已分发的旧模板必须保持可导入。
- CSV 模板带 UTF-8 BOM，保证常见桌面表格软件直接打开中文时不乱码。
- 客户端可以做本地预览优化，但后端不能信任客户端解析。
- XLSX 只接受无宏的 `.xlsx`，拒绝 `.xlsm`、`.xls`、外部链接、公式结果依赖和嵌入对象。
- 后端解析库必须在实现前完成维护状态、许可证、安全和资源消耗评审；现有前端 `xlsx` 的审计例外仅覆盖导出，不能自动扩展到解析用户上传文件。
- 解析在大小、行数、sheet 数、单元格长度、解压比例、CPU 和超时限制内执行。

### 12.2 稳定导入标识

导入以 `member_code` 关联成员，不以展示名关联。这样支持同名员工、重复导入检测和多 Key。

外部系统通常没有本站的成员编号。新模板允许单个成员省略 `member_code`，服务端在权威预览阶段生成不可变成员编号并回显；若同一规范化名称出现多行且都没有 `member_code`，服务端必须拒绝这些歧义行，因为仅凭展示名无法区分“同一成员多 Key”和“多个同名成员”。若外部系统有工号、邮箱或稳定用户 ID，应填入 `member_code`，以便同一成员多 Key 聚合和后续迁移对账。展示名不能作为长期更新键。

外部迁移事实与本站授权策略必须分离：新模板不包含本站 `group_id`。上传并完成数据预览后，owner 在导入向导中从当前可访问分组选择一套有序默认策略，或者选择“暂不授权”。暂不授权的成员以 `disabled` 创建，并在 owner 界面呈现为“待配置”；成员 Key 在批量设置分组并显式启用前不能调用。已分发的旧模板仍兼容 `groups` / `MemberGroups`，但其中分组必须继续按 owner 当前权限校验。

CSV 采用一行一把 Key 的扁平格式；同一 `member_code` 可以出现多行，成员级字段必须一致：

| 中文模板列名 | 兼容英文列名 | 必填 | 说明 |
| --- | --- | --- | --- |
| `成员编号` | `member_code` | 否 | 推荐使用工号/邮箱/外部 ID；空时由服务端生成并在预览回显 |
| `用户名称` | `member_name` | 是 | 展示名；历史列名 `成员名称` 继续兼容 |
| `5小时限额` | `rate_limit_5h` | 否 | 成员全部 Key 共享的 5 小时限额，0/空表示不限 |
| `1天限额` | `rate_limit_1d` | 否 | 成员全部 Key 共享的 24 小时限额，0/空表示不限 |
| `7天限额` | `rate_limit_7d` | 否 | 成员全部 Key 共享的 7 天限额，0/空表示不限 |
| `自然月预算（USD）` | `monthly_limit_usd` | 否 | 0/空表示不限 |
| `本月已消费金额（USD）` | `opening_used_usd` | 否 | 当前自然月的 Key 级迁移开账；同一成员多行时服务端汇总 |
| `密钥名称` | `key_name` | 否 | Key 名称 |
| `API密钥` | `api_key` | 否 | 外部明文 Key；空则生成 |
| `密钥额度（USD）` | `key_quota_usd` | 否 | 单 Key quota |
| `总消耗Token数` | `total_tokens` | 否 | 外部系统给出的汇总值，允许非负且最多两位小数；与分项不一致时预览警告但保留原值；未提供时统一以输入 + 输出作为有效总量 |
| `总输入Token数` | `input_tokens` | 否 | 当前统计周期的迁移期初输入 Token，允许非负且最多两位小数 |
| `总输出Token数` | `output_tokens` | 否 | 当前统计周期的迁移期初输出 Token，允许非负且最多两位小数 |
| `总缓存Token数` | `cache_tokens` | 否 | 外部系统给出的缓存汇总，允许非负且最多两位小数 |
| `总缓存Token写入数` | `cache_creation_tokens` | 否 | 迁移期初缓存写入 Token，允许非负且最多两位小数 |
| `总缓存Token读取数` | `cache_read_tokens` | 否 | 迁移期初缓存读取 Token，允许非负且最多两位小数 |
| `可访问分组ID（按顺序用\|分隔）` | `groups` | 否（历史兼容） | 新模板不再生成；旧模板仍接受并执行服务端授权校验 |

同时兼容客户常见列名：`用户名称`、`api key`、`消费金额`、`月限制金额` 以及上述 Token 中文列名。金额开账默认属于站点计费时区的当前自然月；预览和确认页必须显示具体月份与时区，不能把生命周期累计金额静默写入当前月。

新 XLSX 模板使用两个稳定的英文 sheet 名称，sheet 内列名使用中文；服务端同时兼容历史英文列名和旧三 sheet 模板：

- `Members`：`成员编号`、`用户名称`、各周期限额和自然月预算。
- `Keys`：`成员编号`、`密钥名称`、`API密钥`、`密钥额度（USD）`、Key 级本月已消费金额和 Token 迁移基线。
- `MemberGroups`：只作为旧模板兼容输入；新模板不再生成。

导入的聚合 Token 不写 `usage_logs`，因为没有可信的请求时间、模型、实际分组和请求 ID。实现使用 append-only 迁移基线表关联 import job、成员和 Key；owner 汇总可以展示“迁移期初 + 本站请求”，但请求明细永远只展示本站真实请求。

### 12.3 预览

preview 必须完成全部校验但不落业务数据：

- 文件内部重复和字段冲突。
- 现有成员 code 冲突。
- 旧模板携带分组时校验越权、停用或不存在；新模板的系统分组策略在预览后单独校验。
- Key 格式、长度、文件内部重复和全局重复。
- Key 重复检查必须包含已软删除记录。
- 金额为有限非负数，精度不超过 decimal(20,8)。
- opening used 不得为负；同一成员多 Key 的 Key 级金额汇总为成员本月期初已用，超过 monthly limit 时明确标记预算已耗尽，不能静默截断。
- Token 汇总必须为非负数且精度最多两位小数；CSV/XLSX、预览任务 JSON、不可变迁移基线、提交结果和后续预算摘要必须保持同一精确值，不得截断或四舍五入为整数。预览、结果和摘要 API 使用规范化十进制字符串传输这些迁移 Token，前端按字符串分组显示且不得先转换为 JavaScript `number` 或使用 compact notation。来源提供总量时保留该值并在与已知分项不一致时给出稳定警告；来源未提供总量时，导入结果、不可变基线和后续预算摘要统一以输入 + 输出作为有效总量，缓存分项独立展示且不重复加入总量；若派生总量超过单行数据库上限，必须在预览阶段标记 `invalid_total_tokens`，不能延迟到提交事务失败。
- 当前周期和站点计费时区。

服务端保存规范化 preview、文件哈希、企业 user ID、成员/授权版本和过期时间，返回 preview token。

### 12.4 提交

- commit 校验 preview token、owner、文件哈希、过期时间、系统侧默认分组策略、启用选择和幂等键。同一个 preview 在取得权威终态前必须复用同一幂等键；提交响应不确定时客户端使用 job ID 查询状态，不能直接宣称“数据未写入”。
- 策略版本 2 中，owner 在向导内选择的有序分组是本次导入的唯一系统授权策略，即使选择为空也表示明确的“暂不授权”，不得回退使用旧文件行内 `groups` / `MemberGroups`。只有迁移前已经存在、被固定为策略版本 1 的历史 job 继续使用逐行旧分组语义。
- 在事务内重新检查成员 code、Key 全局唯一和所有已选择分组授权，防止 preview/commit 之间的 TOCTOU。
- 创建成员、可选绑定、Key、opening budget entry、Token 迁移基线和审计记录。没有有效分组策略时成员必须以 disabled 创建。
- 选择“仅导入有效行”时，有效行集合是 preview 的不可变子集；客户端不能修改字段。
- 任何被选有效行落库失败则整批回滚。
- 返回导入 job、逐行结果和可下载错误报告。

导入结果返回本次创建的成员 ID，以及导入时冻结的 `period_start` 与 `timezone`；完成页必须继续显示该归属期，不能用浏览器当前月份重新解释迁移金额。owner 可以直接对本批成员执行“批量设置可访问分组”；导入完成页允许按该 job 返回的成员集合继续配置，普通成员列表工具栏仍受单次 500 人上限约束。批量操作支持按明确模式覆盖或追加，默认使用覆盖且必须展示影响范围。批量设置分组不会静默启用成员，启用必须是独立、明确的 owner 操作；覆盖为空会清空路由权限并同步停用成员，提交前必须使用危险操作确认框回显受影响成员数量。

重复提交相同 idempotency key 返回原结果，不重复创建成员或预算开账。服务端在排队事务提交后直接返回事务内确定的 job ID / 状态，不再通过一次可失败的事务外读取决定提交接口是否成功。

生成或导入的 Key 明文不会写入普通 job result：worker 只把一次性交付载荷加密写入 `result_secrets_ciphertext`。owner 使用原 preview token 调用 `result-secrets` 原子消费，数据库在同一语句中清空密文并写消费时间；未消费密文最长保留 24 小时。失败 job 在保存错误摘要时同时清除 preview 内的 Key 密文，`error-report` 只包含行号、成员 code 和稳定错误摘要。

已完成 import job 与迁移用量基线属于长期可审计证据。正常 owner 操作不提供 import job 或迁移基线的物理删除入口；成员永久删除存在历史事实时只移除 owner 可管理身份并保留这些事实。如未来引入独立的数据保留或合规清理流程，必须显式定义 job、成员、Key、预算账本和基线之间的整组证据处理语义，不能通过放宽外键或级联删除隐式实现。

---

## 13. 安全、审计与可观测性

### 13.1 安全

- 所有 owner 查询绑定认证主体，防 IDOR。
- Key 明文只在创建/导入成功或 owner 主动调用成员专用按需读取接口时返回；后者必须先落不含明文的读取审计并禁止缓存，日志、错误和审计始终脱敏。
- 导入文件是非可信输入；限制压缩炸弹、超大单元格、公式、外部引用和 MIME 欺骗。
- 缓存异常、成员版本不一致和权限交集计算失败时 fail closed。
- 企业 owner API DTO 使用独立类型，禁止复用含 admin 成本字段的 DTO。

### 13.2 审计

记录：

- 账号升级/停用企业能力。
- 成员创建、编辑、启停、归档、恢复、物理删除和墓碑删除。
- 分组授权和顺序前后值。
- Key 创建、按需读取、吊销和导入。
- 预算修改、迁移开账、人工调整和对账修复。
- 导入文件哈希、操作者、结果和错误摘要。

审计不保存完整 API Key 或原始上传文件；原文件按短期安全保留策略删除。

实现使用 `enterprise_member_audit_logs` append-only 表和业务表 `AFTER` trigger，使审计插入与原业务变更处于同一事务。审计表通过 `BEFORE UPDATE OR DELETE` trigger 拒绝改写；成员硬删除后仍保留无外键的 `member_id` 历史标识。Key 审计只复制名称、状态、额度、到期、限流、IP 规则和标签等白名单字段，导入审计只复制文件哈希、格式、状态和时间，不复制 `key`、`preview`、`result` 或原始上传文件。

企业 owner 可调用 `GET /api/v1/enterprise/members/audit` 查看本企业全量操作，或调用 `GET /api/v1/enterprise/members/:id/audit` 下钻单成员。两条查询都在 SQL 中绑定认证 owner；单成员查询还先验证成员归属，不能仅依赖前端过滤。

### 13.3 运维指标

目标指标至少包括：

- 成员 Key 鉴权成功/拒绝及原因。
- 候选分组数量、实际选择、跨分组尝试数和最终错误类型。
- 零金额 receipt 与正金额异步 hold 的创建、结算、释放、过期与恢复数量，二者必须可区分。
- settlement outbox 待处理数量、重放成功/失败和最老待处理时长；Batch image 长时间 `settling` 数量。
- 预算对账差异。
- cache version miss、撤权后旧缓存拒绝。
- 导入解析时长、行数、错误率、回滚、租约续期成功/错误和确认失租次数。

ops 明细可记录 member ID 的内部关联，但普通用户错误响应和管理员常规页面不显示成员名称。

当前管理员只读指标入口为 `GET /api/v1/admin/ops/enterprise-members/metrics`。现有快照使用固定原因集合和进程内原子累计，不使用 owner/member/key ID 作为指标 label；settlement outbox 队列深度、最老待处理时长等数据库状态指标尚未接入该快照。多实例部署的跨实例汇总由后续统一 metrics exporter/collector 验证收口，不能把单实例快照宣称为集群总量。

结果不明的预算 receipt 通过 `GET /api/v1/admin/ops/enterprise-members/ambiguous-receipts` 查询，并通过 `POST /api/v1/admin/ops/enterprise-members/ambiguous-receipts/:id/reconcile` 在具有外部证据时执行 `release`。列表和操作响应只返回 receipt ID、内部 member ID、固定原因、核对次数与时间字段，不返回成员名称、成员编号、请求 ID、分组、预留金额或企业 owner 身份；人工释放必须携带预期核对版本和审计原因，并写入不可变审计日志。

---

## 14. 兼容与迁移

### 14.1 数据迁移

- `users.account_type` 默认 `individual`。
- 新增列均允许老数据通过；不回填成员。
- 普通 Key `member_id=NULL`，继续走原路由。
- 成员 Key `group_id=NULL`，由新 orchestrator 处理。
- 新表和索引使用接续迁移编号；非事务并发索引按现有 `*_notx.sql` 约定。
- 成员 code 全局唯一、复合租户 FK、Key 互斥 CHECK 和预算幂等唯一约束必须由 migration 明确建立。

### 14.2 普通账号升级

升级不自动把现有 Key 转成成员 Key，也不要求用户迁移。企业 owner 可以并行使用两套能力：

- 原普通 Key 保持 `member_id=NULL`，继续按原固定分组、额度、限流和状态工作，并在“API 密钥”页面完整管理。
- 创建成员后为成员生成独立 Key；成员 Key 使用成员的有序可访问分组，并共享成员级预算与限额。
- 成员 Key 管理提供创建、按需复制、编辑、启停和删除；普通 Key 管理入口在成员 Key 弹窗中保持可达，但不会把普通 Key 展示成“待迁移资源”。

后端暂时保留早期迁移接口，仅用于兼容已经发布的客户端。该兼容接口不属于当前产品流程，不在控制台展示，也不得作为新功能或新客户端的依赖；后续移除应单独经过 API 兼容窗口与调用审计。

企业成员控制台的全部静态与动态文案已经进入独立 `enterpriseMembers` zh/en locale namespace；页面不保留按当前语言分支的双语 helper。动态数量、成员名称和导入结果使用参数插值，zh/en key 集合以及页面引用完整性由自动化测试锁定。现有 `common.enterpriseMembers.title/description` 通过显式 namespace 合并继续保留，不会被控制台文案模块覆盖。

### 14.3 回滚

- Schema 采用向前兼容扩展，旧普通 Key 路径可独立工作。
- 已产生成员用量后不能通过简单 down migration 删除成员字段；回滚只能关闭企业能力和成员路由，保留表与证据。
- 发布前必须验证混合版本期间旧实例不会误处理成员 Key；必要时先部署“识别并拒绝成员 Key”的兼容读版本，再启用创建入口。

---

## 15. 测试合同

### 15.1 数据与权限

- account type 与 role 正交。
- 跨企业 member/key 引用被 DB 和 service 双重拒绝。
- 成员 Key 同时设置 group_id 被拒绝。
- 同名成员允许；member code 与 active/archived 历史成员冲突都拒绝。
- 成员禁用/归档、企业能力停用立即阻止全部成员 Key。
- 管理员收回 allowed group 后，旧绑定和旧缓存不能继续访问。
- 分组自身 fallback 不能逃出成员有效集合。

### 15.2 路由

- 所有入口按矩阵提取模型并选择正确 platform handler。
- 候选顺序稳定，模型交叉按配置裁决。
- 分组内账号 failover 先于成员跨分组 fallback。
- retryable typed error 可换组；client/policy/budget 错误不换组。
- 非流式写出前可换组，写出后不可换。
- SSE 首事件后不可换组；Responses WebSocket 在首帧读取后、首个上游 turn 提交前可选组/换组，提交后不可换组。
- Gemini URL model、Responses alias、embeddings、media 和 batch 保存实际分组。
- 普通 Key 全入口回归不变。

### 15.3 计费与预算

- 同步请求按当前实际已用授权；最后一笔或少量并发请求允许形成有限超额，达到或超过限额后新请求被阻止。
- 多把成员 Key 并发调用时共享成员 5h/1d/7d/月限额，不能各自获得一份独立额度。
- 同步请求创建零金额 receipt，不受理论最大输出、模型映射或最高候选倍率影响。
- Batch image、异步图片和视频使用正金额 hold，最高候选倍率占用在完成后按实际金额释放差额。
- 一个逻辑请求多次尝试只产生一次扣费、usage 和预算 usage entry。
- 重复 worker 和幂等重放不重复入账。
- 统一计费写入失败时保留 settlement outbox，恢复后只生成一条 usage、一次余额/Key quota 扣减和一条成员预算 usage entry；持久化载荷不包含 Key 明文或水合上游账号。
- 失败、取消、客户端断开和 worker 崩溃分别按事实结算/释放。
- 月末并发、跨月首次请求、时区边界和时区配置变化。
- expired reservation 恢复不会误释放已产生 usage 的请求。
- Responses WebSocket 多 turn 各自只有一条零金额 receipt、usage 与账本记录；连接失败不会遗留整连接预算占用。
- Batch image 的余额冻结与成员预算预留原子提交，异步 capture/release 同步结算/释放成员预算，worker 重启后仍由 Job 快照恢复；外部工作已经完成时，即使结算重试耗尽也不得释放余额或成员预算。
- 异步图片进程重启后，仍在 `queued` 且未触发上游的任务释放冻结；已进入 `executing/finalizing` 的任务禁止重放并转 `ambiguous`；已结算 receipt 映射为 `settled`，任务查询必须给客户可解释状态。
- budget projection 可从 ledger 重建且对账一致。

### 15.4 导入

- 新 CSV 与两 sheet XLSX，以及历史三 sheet XLSX 兼容。
- 同一成员多 Key、多分组。
- 客户常见中文表头、缺省成员编号生成、Key 级金额汇总和 Token 迁移基线。
- 新模板无分组直接回传可以成功创建待配置成员；旧模板分组仍按 owner 权限校验。
- 导入向导选择统一分组策略，或导入后按本次 job 返回成员集合批量授权。
- 文件内重复、现有 active/archived Key 冲突、同 code 字段不一致。
- 越权/停用分组、非法金额、精度、超限文件、压缩炸弹和公式。
- preview/commit 之间发生冲突时事务重新校验并回滚。
- commit 幂等。
- opening used 进入预算账本但不生成 usage log。

### 15.5 前端

- individual/enterprise/admin 三视角。
- 普通 Key 与成员 Key 并行入口，企业账号升级后普通 Key 不失效、不迁移且仍可完整管理。
- 成员 Key 紧凑列表、按需复制、编辑、启停、删除和本地化状态展示。
- 分组拖拽与键盘排序，版本冲突。
- 客户预算只按 actual used 展示预算、已用、剩余、使用率和有限超额；内部 receipt/hold 保留在运维与对账面。
- loading、empty、error、disabled、slow job 和部分导入错误状态。
- zh/en i18n、键盘、焦点、读屏、窄屏。

### 15.6 迁移与性能

- 空库、现有生产规模数据、软删除数据上的 migration。
- 索引执行计划覆盖 member list、member usage、auth 反向失效和 budget period。
- 高并发同步授权/零金额 receipt、异步任务 hold、缓存失效风暴和大批量导入。
- 最大 5000 行 CSV 解析与真实 PostgreSQL 事务提交分别建立可重复基线；本机 Apple M4 Pro 参考值为解析约 2.56 ms、5000 成员连同逐成员审计提交约 7.9 s，该数值只作回归对比而不是跨机器 SLA。
- Redis 进程重启后，认证缓存 Pub/Sub 订阅必须重新出现，恢复订阅后的单次失效广播仍能清除其他实例重启前持有的旧 L1。
- PostgreSQL 在成员批量 INSERT 期间终止事务连接时，所有未提交成员和审计必须整体回滚；Job 保留 processing 租约事实，并可在租约过期后由新 worker 接管。
- worker Stop 必须取消活跃处理和心跳并等待 goroutine 退出；处理 timeout 后的失败状态使用独立有效 context 持久化。
- 混合版本部署期间成员 Key 被安全拒绝而不是误走未分组 Key。

---

## 16. 完成标准

只有以下条件全部满足，企业成员功能才算完成：

- ADR、schema、migration、repository、service、handler、routes、Wire、frontend、i18n 和 docs 同步。
- 所有系统不变量有 DB/service 约束和测试。
- 所有入口按能力矩阵接入请求级 ActiveGroup。
- 权限收回和成员禁用有可证明的缓存失效或版本拒绝。
- 同步请求零金额 receipt、异步任务金额 hold、幂等结算、崩溃恢复和对账全部工作。
- 导入支持多成员、多 Key、多分组和开账审计，并抵抗 TOCTOU。
- 导入 worker 的唯一领取、心跳续租、租约接管、失租取消和旧 worker fencing 由单元状态机与真实 PostgreSQL 并发测试共同证明。
- 最大 5000 行导入可以完成真实事务并生成逐成员 append-only 审计；第 5001 行在解析边界被拒绝。
- 用户、成员、分组或 Key 变更触发的认证缓存失效可以通过 Redis Pub/Sub 清除其他实例的 L1 快照。
- Redis 重启后的订阅恢复和 PostgreSQL 事务连接强杀后的零部分写入由真实进程级故障测试证明。
- 普通 Key 回归通过。
- 用户/admin 字段边界通过契约测试。
- 后端单元/集成测试、前端测试、lint、typecheck、migration 验证和 docs build 全部通过。

实现顺序可以是“领域与迁移 → 管理面 → 请求路由 → 预算与账本 → 导入与完整 UI → 全量验证”，但这是工程依赖顺序，不是 MVP 范围切割；最终交付必须达到本文完整目标状态。
