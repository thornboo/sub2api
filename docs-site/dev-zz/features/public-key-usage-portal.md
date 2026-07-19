# Key 持有者自助查询

> 状态：已实现，待发布验收
>
> 路由：`/key-usage`
> 日期：2026-07-19

## 1. 目标

将现有公开 `/key-usage` 页面升级为正式产品模块，让只有 API Key、没有站点账号的普通用户或企业成员能够查询这把 Key 自身的信息，而不会获得 owner 控制台权限。

页面必须直接回答四个问题：

1. 这把 Key 当前能否使用，何时过期，受哪些限制。
2. 这把 Key 能访问哪些分组和模型。
3. 当前 Key 已用了多少；若属于企业成员，成员共享预算又用了多少。
4. 这把 Key 在所选日期范围内产生了哪些请求、Token 和实际扣除。

首页导航在“登录”之前提供“Key 查询”入口。保留现有 `/key-usage` URL，不创建平行页面。根目录 HTML 仅作为已确认的视觉与信息架构参考；正式页面使用 Vue、项目主题、i18n 和现有组件体系重写，验收完成后删除原型。

## 2. 用户与权限

### 2.1 支持对象

- 普通用户 Key：展示当前 Key 状态、Key 级额度、有效分组/模型、当前 Key 用量和请求记录。
- 企业成员 Key：在普通 Key 信息之外，展示当前成员的安全身份字段和成员共享预算/窗口限制。
- 已禁用、额度耗尽或已过期 Key：仍允许持有者查询状态和历史证据，但不能因此恢复调用权限。

自助查询以“持有完整 Key”为证明，不继承网关调用的 IP 白名单/黑名单。这样部署在服务器上的 Key 仍可由成员从浏览器查询；IP 规则只决定 API 调用是否被允许。该取舍意味着泄露的完整 Key 也能读取自身脱敏记录，因此接口采用短会话、严格限流、字段白名单和不缓存响应降低暴露面。

### 2.2 允许展示

- Key 名称、脱敏前缀、状态、创建/最后使用/过期时间。
- IP 访问规则的模式与数量，不返回完整规则值。
- 当前 Key 的 quota、quota used、5h/1d/7d 窗口与重置时间。
- 当前成员的名称、成员编号、月预算、已结算、处理中预占、剩余和共享窗口限制。
- 当前 Key 已委派分组的名称、平台、顺序、RPM 和当前可见模型目录。实际调用仍取决于 endpoint、余额/订阅、实时账号可用性和路由规则。
- 当前 Key 的请求数、输入/输出/缓存 Token、实际扣除、平均耗时、趋势和模型分布。
- 当前 Key 的成功使用记录与脱敏失败记录。
- 当前筛选范围内的脱敏 CSV。

### 2.3 禁止展示

- 完整 API Key、其他 Key 的名称/前缀/记录。
- owner 邮箱、用户名、余额、订阅或其他成员。
- 上游账号、渠道、代理、账号池、调度候选、真实上游端点。
- 账号成本、渠道成本、利润、上游倍率、管理员备注。
- 内部错误堆栈、未经脱敏的错误体、数据库标识或供应商凭证。

## 3. 查询会话

### 3.1 建立

```text
POST /api/v1/key/usage-session
Authorization: Bearer <API Key>
```

后端验证 Key 后生成至少 256 bit 的随机会话令牌，只在 `HttpOnly` Cookie 中返回。Redis 仅保存令牌哈希和以下最小快照：

- `api_key_id`
- `user_id`
- `member_id`（可空）
- `created_at`
- `absolute_expires_at`

前端收到成功响应后必须立即清空输入框和响应式 Key 状态。完整 Key 不进入 URL、localStorage、sessionStorage、埋点或应用日志。

### 3.2 生命周期

- 无操作 15 分钟过期。
- 最长 1 小时绝对过期。
- 每次有效读取只刷新空闲 TTL，不突破绝对过期时间。
- `DELETE /api/v1/key/usage-session` 删除服务端会话并清除 Cookie。
- Key 被删除，或 `user_id/member_id` 归属发生变化后，会话在下一次读取时失效；Key 状态变化会在每次查询时重新读取，不使用会话创建时的状态作为权威值。

Cookie 属性：

- `HttpOnly`
- HTTPS 环境 `Secure`
- `SameSite=Strict`
- Path 限定到 `/api/v1/key`

当前 Cookie 合同要求前端与 API 属于浏览器意义上的 same-site。若未来部署到不同 registrable domain，必须先补充严格 Origin 白名单与 CSRF 方案，再评估 `SameSite=None; Secure`，不能只放宽 Cookie 属性。

## 4. 接口

### 4.1 会话状态

```text
GET    /api/v1/key/usage-session
DELETE /api/v1/key/usage-session
```

只返回会话是否有效及过期时间，不返回 Key 或内部 ID。

### 4.2 摘要

```text
GET /api/v1/key/usage/summary
  ?start_date=YYYY-MM-DD
  &end_date=YYYY-MM-DD
  &timezone=Asia/Shanghai
```

返回：

- `identity`
- `access_groups`
- `key_budget`
- `member_budget`（普通 Key 为空）
- `stats`
- `trend`
- `models`

对外的 `start_date`、`end_date` 都是用户时区下的包含型日历日期；后端内部转换为半开区间。日期输入最大跨度 90 天，默认最近 30 天。

`identity.active` 表示 Key、owner、企业成员和固定/成员分组的静态可用性。具体请求仍可能因模型、端点、订阅/余额、额度或 IP 规则被拒绝，因此页面不把它表述为对任意请求都可用的绝对保证。

### 4.3 请求记录

```text
GET /api/v1/key/usage/records
  ?kind=success|error
  &page=1&page_size=50
  &start_date=YYYY-MM-DD&end_date=YYYY-MM-DD
  &model=...
  &status_code=...
```

成功记录来自 `usage_logs`，强制 `user_id + api_key_id` 双重过滤。失败记录复用用户侧 ops 脱敏视图，并再次强制 `api_key_id`。两类记录分别分页，避免在数据库层用不稳定的跨表 offset 合并。

### 4.4 详情

```text
GET /api/v1/key/usage/records/:id?kind=success|error
```

成功详情只返回用户计费与请求字段；失败详情除 owner 校验外还必须校验 `api_key_id`，猜中同一 owner 其他 Key 的错误 ID 也只能得到 404。

### 4.5 导出

```text
GET /api/v1/key/usage/export?kind=success|error&...
```

- 复用记录接口的范围与过滤合同。
- 最大日期范围 90 天。
- 最大 5,000 行。
- 只导出白名单字段。
- 文件名不包含 Key、成员名称或用户信息。

## 5. 限流与审计

| 操作 | 初始限制 |
| --- | ---: |
| 建立会话 | 单 IP 5 次/分钟且 30 次/小时 |
| 同一 Key 建立会话 | 10 秒冷却 |
| 已建立会话读取 | 单 IP 60 次/分钟 |
| 详情 | 单 IP 30 次/分钟 |
| 导出 | 单 IP 3 次/10 分钟 |

会话本身与限流都依赖 Redis，因此 Redis 异常时所有会话操作 fail closed。错误响应不返回 Key 或会话令牌。

审计允许记录：事件类型、结果、Key ID 或 HMAC/哈希指纹、脱敏前缀、IP、User-Agent、时间。禁止记录完整 Key 和会话令牌。

## 6. 前端结构

- `HomeView.vue`：增加“Key 查询”次级入口。
- `frontend/src/api/publicKeyUsage.ts`：独立公共客户端，不继承站点 JWT。
- `KeyUsageView.vue`：页面状态、信息面板、记录筛选与短会话编排。
- `backend/internal/handler/public_key_usage_handler.go`：Cookie、字段白名单、日期范围、记录和 CSV 边界。
- `backend/internal/service/public_key_usage_session.go`：会话令牌、身份快照、空闲/绝对过期。
- `backend/internal/repository/api_key_cache.go`：只保存令牌哈希和最小会话快照。

视觉继续使用项目现有 stone/neutral/emerald 主题。保持扁平、紧凑、少层级，不使用大圆环、营销式 hero、装饰性渐变或解释产品实现的可见文案。

## 7. 验收

### 7.1 安全

- 原始 Key 只在建立会话请求中出现一次，成功后前端状态为空。
- Cookie 无法被 JavaScript 读取，退出后旧会话不可继续访问。
- 任意接口都不能查询会话 Key 之外的数据。
- DTO 测试证明不存在 owner、上游账号、渠道、管理员成本和完整 Key 字段。
- 同 owner 其他 Key 的记录 ID 访问返回 404。
- 日期、分页、导出行数和限流均由后端强制。

### 7.2 产品

- 首页桌面和移动端都能找到“Key 查询”。
- 普通 Key 与成员 Key 都可以查询。
- 成员共享预算与当前 Key 用量具有不同标题，不混算。
- 分组直接展示名称、平台和模型，不只展示数量。
- 页面有浅色/深色模式、中文/英文、键盘焦点、加载/空/错误状态。
- 退出查询在顶部导航中。

### 7.3 验证

- 后端 service/handler/repository 单元测试。
- Redis 会话创建、刷新、绝对过期和撤销测试。
- 跨 Key 越权与 DTO 字段白名单测试。
- 前端 Vitest：输入清除、会话恢复、退出、筛选、分页、主题和首页入口。
- 前端 typecheck、lint、定向测试与构建。
