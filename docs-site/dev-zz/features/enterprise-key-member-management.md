# 企业客户 Key 成员管理

> **⚠️ 已废弃（2026-07-12）。** 本文档是“用 Key 代替成员、不引入成员实体”的历史实施记录，不再作为后续企业架构依据。现已被 [企业用户成员管理](./enterprise-member-management.md) 取代——新方案使用 `account_type=enterprise`、不可登录成员实体、请求级 `ActiveGroup`、成员预算预留/账本和成员 N:N 有序分组。对应决策见 [ADR-0003](../decisions/adr-0003-enterprise-member-entity.md)。
>
> 本文档中已落地的能力（批量创建 Key、标签、按 ID / 筛选批量管理、公共 Key 状态查询、owner 用量分析）对**普通用户 Key 和历史普通 Key 保留有效**。以下尚未落地的建议均视为历史方案，不得绕过 ADR-0003 重新实施。
>
> 原状态：部分落地。批量创建、标签、筛选批量操作和公共 Key 状态查询已经实现；多分组访问范围仍是后续设计。

## 已落地情况

- 阶段一批量创建、公共 Key 状态查询已落地。
- 阶段二标签、按 ID 批量维护、按筛选条件批量维护已落地。
- owner 用量分析第一版已落地，详见 [企业用量分析中心](./enterprise-usage-analytics.md)。
- 多供应商单员工一把 Key 的 Key Access Profile / 多分组访问范围仍未落地，需另写设计取舍文档。

## 背景

sub2api 面向部分企业客户。一个企业客户在站点上注册的是**一个普通账号**（`users` 表中的一行）。当这个企业有 100 个员工时，企业管理员需要为每个员工分发独立的 API Key，以便分别控制和观察每个员工的用量、配额和限制。

原来的 sub2api 只能单把创建 Key，缺少批量创建、批量管理、结构化标签，以及面向企业管理员的聚合监控视图。本计划的目标是把 Key 管理升级成“以 Key 承载成员管理”的形态：**一把 Key 等于一个员工席位**，让企业管理员能批量发放、分组分类、限额管控，并统一观察旗下所有 Key。

本计划参考了 `claude-code-hub`（`ding113/claude-code-hub`）的用户/成员管理交互，但**不照搬其数据模型与代码**。两者技术栈不同，产品视角也不同；这里只借鉴功能形态和管理体验。

## 关键取舍：用 Key 代替成员，不引入子账号实体

两边的架构视角不同：

- **claude-code-hub 是平台管理员视角**：管理员在后台管理多个 `user`（成员），每个 user 名下挂多把 key。
- **sub2api 企业客户是自助视角**：企业客户本身就是一个 `users` 行，要管理的"员工"映射为**该账号名下的多把 Key**。

因此本计划**不引入 sub-account / org-member 新实体**，而是直接增强 `api_keys`。理由是：

1. sub2api 数据模型已是 `User 1:N APIKey`，Key 天然适合承载"员工席位"。
2. 企业管理员需求是"统一控制和限制员工用量"，不是"员工各自独立登录"。Key 方案足以覆盖。
3. 不新增登录主体，避免认证、计费、权限体系的大规模改动，符合 dev-zz "改动可隔离、与上游冲突面小"的纪律。

> 若未来企业客户要求员工各自登录查看本人用量，再评估引入子账号实体的重型方案。该场景不在本计划范围内。相关取舍见 [设计取舍 0002](../decisions/adr-0002-key-as-enterprise-member.md)。

## 现有基础

### 数据模型

- Key 实体：`backend/ent/schema/api_key.go`，表名 `api_keys`，软删除 + 时间 mixin。
- 已有字段：`user_id`、`key`、`name`(100 字符)、`group_id`(可空，关联 Group)、`status`、`last_used_at`、`ip_whitelist`/`ip_blacklist`(CIDR)、`quota`/`quota_used`(USD decimal20,8)、`expires_at`、`rate_limit_5h/1d/7d` + 对应 `usage_*` 滑窗用量与 `window_*_start`。
- User 实体：`backend/ent/schema/user.go`，`User` edge `api_keys`(1:N)、`balance`、`concurrency`、`rpm_limit`、`role`。

### 后端分层

- 用户侧 Key handler：`backend/internal/handler/api_key_handler.go`
  - 实际路由注册在 `backend/internal/server/routes/user.go` 的 `/api/v1/keys` 分组下：`GET /keys`、`POST /keys`、`PUT /keys/:id`、`DELETE /keys/:id`。
  - 备注：`api_key_handler.go` 内部注释仍写作 `/api/v1/api-keys`，这是历史注释与实际路由不一致；新增用户侧 Key 端点应延续实际路由命名 `/api/v1/keys/*`。
  - 方法：`List`、`GetByID`、`Create`、`Update`、`Delete`、`GetAvailableGroups`、`GetUserGroupRates`。
  - `Create`(:143) 单把创建，已支持 Name / GroupID / CustomKey / IPWhitelist / IPBlacklist / ExpiresInDays / Quota / RateLimit5h/1d/7d。
  - `Update`(:188) 单把更新，支持改名 / 分组 / 状态 / 配额（含重置）/ 限流（含重置用量）/ 过期（含清除）。
  - 写操作已接入 idempotency：`executeUserIdempotentJSON`。
- Key service：`backend/internal/service/api_key_service.go`、`api_key.go`。
- Key repository：`backend/internal/repository/api_key_repo.go`、`api_key_cache.go`。
- admin 侧 Key handler：`backend/internal/handler/admin/apikey_handler.go`，仅 `UpdateGroup`。
- admin 侧 User handler：`backend/internal/handler/admin/user_handler.go`，含 `GetUserAPIKeys`、`BatchUpdateConcurrency`、`ReplaceGroup`、`GetUserUsage` 等。

### 已存在的批量范式（可复用）

- `backend/internal/handler/admin/account_handler.go`：`BatchCreate`(POST `/api/v1/admin/accounts/batch`)、`BatchClearError`、`BatchRefresh`，配合 `executeAdminIdempotentJSON` 与 `BulkUpdateAccountsRequest` + `Filters` 过滤器范式。
- admin user 侧 `BatchUpdateConcurrency`(:515)。
- group 侧 `BatchSetGroupRateMultipliers`、`BatchSetGroupRPMOverrides`。

### 用量统计聚合（可复用）

- `GetBatchAPIKeyUsageStats`、`GetAPIKeyUsageTrend`、`GetAPIKeyStatsAggregated`、`GetAPIKeyDashboardStats`（均在 usage 仓储层），目前主要服务 admin 视角。

### 前端

- 用户侧 Key 页：`frontend/src/views/user/KeysView.vue`，接口 `frontend/src/api/keys.ts`。
- admin 侧 Key 弹窗：`frontend/src/components/admin/user/UserApiKeysModal.vue`，接口 `frontend/src/api/admin/apiKeys.ts`。
- 创建组件测试：`frontend/src/components/__tests__/ApiKeyCreate.spec.ts`。

## 还缺什么

| 维度 | sub2api 现状 | 需要补充 |
| --- | --- | --- |
| 单把创建 | 已支持完整参数 | 无需改 |
| 批量创建 | 缺失 | 一次创建 N 把（命名模板 / 数量 / 统一配额限流分组） |
| 批量管理 | 缺失 | 批量启用/禁用、批量改配额/限流/过期/分组/标签、批量删除 |
| 结构化分类 | 仅 `name` 自由文本 | 新增 `tags`（jsonb + GIN 索引），对标 cch |
| 企业聚合视图 | 偏 admin 视角 | owner 视角的旗下 Key 用量排行 / 成本分摊 / 按标签聚合 |
| 自助管理 UI | `KeysView.vue` 偏薄 | 增强为带批量、筛选、标签、用量的企业管理台 |

## 非目标

- 不引入子账号 / 组织成员登录实体（见 [设计取舍 0002](../decisions/adr-0002-key-as-enterprise-member.md)）。
- 不改真实计费与网关转发链路。
- 不改 admin 全局视角的现有用户/分组管理方式。
- 第一阶段不做员工各自独立登录、员工自助查看个人用量。
- 不照搬 claude-code-hub 的表结构或代码。

## 推荐方案

按风险与价值分三个阶段推进，每个阶段独立可交付、独立可记录为 dev-zz 补丁。

### 阶段一：批量创建 Key（最高优先级）

让企业管理员一次性发放一批 Key，对应批量入职员工。

阶段一原始范围只解决"发放"问题，不引入 `tags` schema，不支持员工登录，也不支持批量自定义 Key。2026-06-13 的实现补充了基于已选 ID 的批量更新 / 删除，但仍不引入标签字段，也不支持按筛选条件批量操作。`quota` 在本阶段继续沿用单把 Key 的语义：它是该 Key 的最大可用额度，不在创建时冻结或预占 owner 余额；真实扣费仍发生在请求计费链路中。

**落地情况（2026-06-13）**

- 已落地用户侧 `POST /api/v1/keys/batch`。批量创建为全有或全无的单事务写入，不存在部分成功状态。
- 已落地按 ID 勾选的用户侧 `POST /api/v1/keys/batch-update` 和 `POST /api/v1/keys/batch-delete`。2026-06-14 阶段二 B 在同一路由上补充了 `apply_to=filtered`，可对当前筛选条件匹配的 Key 执行批量改/删。
- 已落地只读公共查询 `POST /api/v1/key/status`，用于只有 Key、没有站点账号的员工查询本人 Key 的状态、额度、过期时间、最近使用时间和限流配置。该能力来自"员工只有 Key 也要自助查询"的阶段一补充需求，可与批量创建独立审查。
- 阶段一没有修改 `api_keys` schema，也没有迁移老数据；老 Key 与个人用户单把 Key 的创建、认证、扣费、限流、过期和 IP ACL 使用链路保持兼容。
- 前端已在用户侧 `KeysView.vue` 增加"批量创建"入口、结果弹窗、一次性明文提示、复制全部、单 Key 复制、CSV 导出、批量操作栏、批量更新弹窗和批量删除确认。
- API Key 可写状态现在以 `active` / `disabled` 为准。旧输入 `inactive` 只作为兼容别名归一到 `disabled`，不得重新作为持久化状态使用。
- `quota_exhausted` 与 `expired` 是系统派生状态。普通编辑表单不应在用户没有显式禁用时把这两个状态覆盖为 `disabled`。

**后端**

- 新增用户侧端点 `POST /api/v1/keys/batch`，延续当前用户侧 Key 路由命名。实现可参考 `account_handler.go` 的批量入口与 `executeUserIdempotentJSON` 风格，但不能复用其"逐条尽力创建、允许部分成功"语义；企业 Key 批量创建必须是整批成功或整批回滚。
- 批量数量上限做成系统设置项，建议 key 为 `api_key_batch_create_max_count`，默认 `200`。后端读取失败或未配置时回退到 `200`，后台设置页可暴露该配置；服务端仍应设置硬上限（建议 `500`）避免误配置导致单次响应过大。
- 请求体建议字段：
  - `count`：数量，范围 `1..max_count`。
  - `name_template`：命名模板，如 `员工-{seq}`。
  - `names`：显式名称数组，与 `name_template` 二选一。
  - `group_id`：统一分组，可空，沿用单把 Key 的分组权限校验。
  - `quota`：统一 Key quota，`0` 表示不限制；不做 owner 余额预占。
  - `rate_limit_5h` / `rate_limit_1d` / `rate_limit_7d`：统一限流。
  - `expires_in_days`：统一有效期。
  - `ip_whitelist` / `ip_blacklist`：统一 IP ACL。
- 阶段一请求体不包含：
  - `tags`：等阶段二新增 `api_keys.tags` 后再支持。
  - `custom_key` / `custom_keys`：批量自定义 Key 的冲突、幂等和安全校验复杂，收益低，不进入阶段一。
- 命名规则：
  - `name_template` 与 `names` 必须恰好提供一个。两个都传或两个都不传均返回 400，避免前后端各自猜测优先级。
  - `name_template` 必须包含 `{seq}`。
  - `{seq}` 从 `1` 开始，按 `max(3, count 的十进制位数)` 补零，例如 `count=12` 得到 `001..012`，`count=1200` 得到 `0001..1200`。
  - 使用 `names` 时，`len(names)` 必须等于 `count`；每个名称 trim 后不能为空、不得重复，并遵守单把 Key 的名称长度限制。
- service 层新增独立 `BatchCreate(ctx, userID, input)`，不要在 handler 中循环调用单把 `Create`：
  - 批量请求先统一校验数量、命名、分组归属、IP ACL、quota / rate limit / expires 字段。
  - 在单个数据库事务内生成并创建全部 Key；任意一把创建失败则整批回滚，不产生部分成功状态。
  - repository 层新增事务感知的批量创建能力，或抽出可复用的 tx-aware create helper；不要让 handler 直接掌握事务细节。
  - Key 生成沿用系统生成策略，并对唯一索引冲突做有界重试。
  - 事务提交后再做 auth cache / key cache 失效，避免回滚后产生无效缓存操作。
- 返回创建结果列表（含明文 Key，仅此次返回），同时返回批量摘要：`created_count`、`max_count`、`quota_total_declared` 等便于前端展示。

**幂等与明文 Key 安全约束**

- 批量创建必须要求 `Idempotency-Key`。幂等 scope 建议使用 `user.api_keys.batch_create`。
- 幂等 fingerprint 必须绑定 HTTP method、route、actor scope 和完整归一化请求体。当前 `IdempotencyCoordinator` 的 `Payload` fingerprint 已具备该能力；批量创建实现必须把完整 batch request 作为 payload 传入，确保同一 `Idempotency-Key` 搭配不同请求体时返回 fingerprint conflict，而不是误判为重放。
- 普通 `executeUserIdempotentJSON` 会把成功响应写入 `idempotency_records.response_body`。批量创建响应含明文 Key，不能直接用普通成功响应缓存落库，否则会违背"完整 Key 只显示一次"的安全语义。
- 批量创建需要专用幂等处理：成功后 idempotency 记录最多保存不含明文 Key 的摘要（例如 `created_count`、创建的 Key ID / 名称、`keys_available:false`），不得保存完整 Key。首次请求仍向客户端返回完整 Key；同一请求重放时只能返回不含明文 Key 的摘要，或返回明确的已完成但明文不可重放错误，不得再次返回完整 Key。
- 日志、审计、错误响应、idempotency 存储与前端持久化状态均不得记录完整 Key。完整 Key 只允许出现在首次创建响应和用户主动复制 / 导出的本地文件中。

**员工 Key 状态查询（阶段一补充）**

- 新增公共只读端点 `POST /api/v1/key/status`，请求体为 `{ "key": "sk-..." }`。
- 返回范围只包含该 Key 自身信息：名称、状态、是否可用、分组、quota / quota_used、rate_limit_*、expires_at、last_used_at、created_at、updated_at。不得返回 owner 账号余额、邮箱、角色、其它 Key 或全局企业数据。
- 该端点不走网关认证缓存，不更新 `last_used_at`，不消耗 quota，不改限流窗口，只做读查询和状态推导。
- 限流策略：
  - 同一 Key 10 秒内只允许查询一次，用 Key 哈希做限流标识，不记录明文 Key。
  - Redis 冷却写入失败时 fail-close 返回不可用，不静默降级为多实例不一致的进程内限流。
  - 路由层再叠加 IP 级 `30/min` 限流，避免暴力枚举和高频撞库。
- 该查询页面后续放在 docs-site 或公开文档项目中实现；阶段一后端已经提供可用接口，前端主站只新增接口封装，不强制员工拥有站点账号。

**前端**

- 在 `KeysView.vue` 增加"批量创建"入口，与单把创建并列。
- 批量创建表单包含：数量、命名方式（模板 / 名称列表）、分组、额度、有效期、5h/1d/7d 限流、IP 白名单、IP 黑名单。
- 模板命名模式实时预览前几条生成名称，避免管理员创建后才发现命名错误。
- 名称列表模式支持一行一个名称；前端即时校验数量、空行、重复名称和超长名称。
- 阶段一不展示标签输入。
- 创建成功后显示结果表格：名称、完整 Key、分组、quota、有效期、限流、IP ACL。
- 完整 Key 只在创建结果中展示一次。关闭结果弹窗前应提示管理员先复制或导出。
- 提供"复制全部"与"导出 CSV"。CSV 字段建议为：`name`、`key`、`group`、`quota`、`expires_at`、`rate_limit_5h`、`rate_limit_1d`、`rate_limit_7d`、`ip_whitelist`、`ip_blacklist`。

### 阶段二：结构化标签 + 批量管理

**数据模型**

- 2026-06-13 阶段二 A 已在 `api_keys` 新增 `tags` 字段：`field.JSON("tags", []string{}).Default([]string{})`，对应 `jsonb NOT NULL DEFAULT '[]'::jsonb`。
- 迁移文件使用 `151_add_api_key_tags.sql` 新增列和数组约束，并使用 `152_add_api_key_tags_index_notx.sql` 通过 `_notx` 约定创建并发 GIN 索引。
- 后端统一将标签 trim、小写化、去重，并限制最多 20 个、单个最多 40 个字符。

**后端批量管理端点**（用户侧，均限定 owner 只能操作自己的 Key）

- `POST /api/v1/keys/batch-update`：当前已支持按 `ids` 或 `apply_to=filtered` 批量改 `status` / `quota` / `rate_limit_*` / `expires_at` / `group_id` / IP ACL / `tags`，并支持重置限流窗口用量。标签更新支持 `set` / `add` / `remove` / `clear`。
- `POST /api/v1/keys/batch-delete`：当前已支持按 `ids` 或 `apply_to=filtered` 批量软删除。
- 用户侧 Key 列表当前支持按 `tags` 查询参数筛选，多个标签语义为同时包含。
- `GET /api/v1/keys/tags`：返回当前 owner 未删除 Key 的去重标签候选，供前端标签筛选下拉使用。
- `apply_to=filtered` 支持 `search` / `status` / `group_id` / `tags` 筛选，要求至少一个筛选条件，并限制单次最多匹配 500 个 Key。后端会先把筛选结果解析为当前 owner 名下的 ID 集合，再复用现有按 ID 批量事务、越权检查和缓存失效链路。
- 仓储层在 `Create` / `Update` 写入前把 nil tags 转为 `[]`，保证 `api_keys.tags` 始终是 jsonb array，不把 JSON `null` 留到数据库。
- 单把 Key 编辑时，如果 `group_id` 未变化，后端不会重新执行分组绑定授权。这样历史上已经绑定、但当前 owner 不再可绑定的分组，不会阻止用户更新标签、额度、限流、过期或 IP ACL。
- 批量操作接入 idempotency，单批设上限。

**前端**

- `KeysView.vue` 已增加标签筛选、标签展示、单把创建 / 编辑标签、批量创建统一标签、批量标签 set / add / remove / clear；当前前端批量修改 / 删除入口仍以列表勾选为准。
- 批量创建结果表格和 CSV 导出已包含 `tags` 字段。
- 标签输入和展示已抽成 `TagEditor` / `TagPills` / `TagFilterSelect`，前端校验与后端 trim、小写化、去重、数量和长度限制保持一致。

### 阶段三：企业聚合监控视图（owner 第一版已落地）

让企业管理员统一观察旗下所有 Key 的用量与成本分摊。

阶段三的详细产品、权限和接口设计见 [企业用量分析中心](./enterprise-usage-analytics.md)。当前 owner 第一版已在用户侧 Usage 页面落地；平台管理员全站增强、异常治理和多分组 Key 仍是后续阶段。该设计把企业 owner 自助分析、平台管理员全站分析和单 Key 下钻分开，避免把管理员成本字段或全站数据误暴露给普通用户。

**后端**

- 已新增 owner 视角统计端点 `/api/v1/usage/analytics/*`，按当前登录 user 的全部 Key 聚合。
- 已支持 summary、leaderboard、models、groups、tags、trend。
- 已支持按 `tags` 分组聚合；标签聚合是归因观察，不是严格财务分摊。

**前端**

- 已在用户侧 `UsageView.vue` 增加 `分析` Tab 和 `UsageAnalyticsPanel`。
- 复用现有单 Key 用量弹窗作为下钻详情。
- 多供应商员工 Key 不建议长期通过“同一员工多把物理 Key”解决。短期可用标签归并，长期应单独设计 Key Access Profile / 多分组访问范围，让一把员工 Key 绑定多个可用分组，同时保留 `api_keys.group_id` 兼容旧逻辑；多分组绑定必须复用现有用户可绑定分组规则，不能绕过 `AllowedGroups`、订阅型分组或 fallback group 语义。

## 实施纪律（dev-zz）

- 新增能力集中在 `backend/internal/{handler,service,repository}` 与 `frontend/src/views/user`、`frontend/src/components/keys`，与上游冲突面小。
- 每阶段完成后更新 `docs-site/dev-zz/patches.md`（实现）与 `changelog.md`（用户可见变化）。
- `tags` 字段属于 schema 变更，需在 patches.md 标注 migration 编号，便于后续合并上游时核对。
- 关键取舍（Key 代替成员、不引入子账号）记录于 [设计取舍 0002](../decisions/adr-0002-key-as-enterprise-member.md)。
- 状态字段只写 `active` 或 `disabled`；`inactive` 只能作为输入兼容。
- 修改分组时才检查目标分组可绑定；未修改分组时不能因为历史分组当前不可绑定而阻断其它字段更新。

## 验证计划

- 后端：`mise x -C backend -- go test ./internal/handler ./internal/service ./internal/repository`，针对批量创建/批量更新新增表驱动测试与 `-race`。
- 阶段一批量创建需覆盖：数量上限、设置项默认值与硬上限、名称模板展开、名称列表校验、重复名称拒绝、分组越权拒绝、IP ACL 校验、事务回滚、幂等重放、完整 Key 只在创建响应中返回。
- 前端：`pnpm --dir frontend typecheck`、`lint:check`、批量创建表单 / 结果表 / CSV 导出的 `test:run`。
- 阶段二再补充筛选 / 标签 / 批量更新组件测试。
- schema 变更：迁移前后 `ent` 代码生成一致性、GIN 索引生效。

## 未决问题

- 是否需要"Key 模板/预设"以便重复发放同规格 Key（可作为阶段一的延伸，非必需）。
- 是否需要从 CSV / Excel 导入员工名称列表（可作为阶段一之后的增强，第一版先支持文本粘贴）。
- 标签是否需要预定义字典（受控词表）还是完全自由文本，影响前端编辑器形态。
