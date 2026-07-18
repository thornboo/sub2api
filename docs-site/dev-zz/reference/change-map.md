# dev-zz 变更地图

这页按 `origin/main...dev-zz-develop` 的真实差异整理二开范围。`dev-zz-develop` 是当前持续开发分支，正式线 `dev-zz` 只接收验证后的改动。最近一次盘点基于：

| 项 | 值 |
| --- | --- |
| dev-zz-develop HEAD | `48f62862` |
| origin/dev-zz | `738a15be` |
| origin/main | `945b9b20` |
| merge-base | `945b9b20` |
| 差异规模 | 339 个文件，约 34828 行新增、4174 行删除 |

## 变更分布

| 区域 | 文件数 | 说明 |
| --- | ---: | --- |
| `frontend/` | 199 | 用户/API Key、owner 用量分析、管理员用量下钻、可用渠道模型、运维弹窗栈、主题与控制台 UI |
| `backend/` | 80 | API Key 批量/标签/状态、owner analytics、usage 聚合、已删除 Key 证据 hydrate、配置默认值、测试与迁移 |
| `docs-site/` | 34 | dev-zz 文档中心、功能文档、部署/开发/维护记录 |
| `deploy/` | 14 | fork 镜像默认值、源码构建脚本、备份脚本、Compose/安装脚本与部署样例 |
| `.github/` | 4 | CI、release、security scan 的 Node 24 actions runtime 验证 |
| 根目录 / README / Dockerfile | 8 | release 镜像、版本号、项目说明、分布式 Dockerfile 与设计索引 |

## 已落地功能

### API Key 批量与标签管理

- 批量、标签和单 Key 分析是已落地的普通 Key 能力；“Key 即企业成员”的临时领域结论已由 ADR-0003 取代。
- 用户侧 Key 支持结构化 `tags`，新增 `jsonb` 字段和 GIN 索引。
- 创建、编辑、列表筛选、批量创建、批量更新、批量删除均支持标签。
- 批量创建支持模板名或名称列表；响应首次展示明文 Key，幂等重放会脱敏。
- 批量更新和删除支持 `apply_to=selected` 与 `apply_to=filtered` 两种目标。
- 筛选批量目标支持 `search`、`status`、`group_id`、`tags`，且要求至少一个筛选条件，单次最多 500 把 Key。
- Key 状态以 `active` / `disabled` 为可写状态，`inactive` 仅作为旧别名归一化到 `disabled`；`quota_exhausted`、`expired` 是系统状态，不作为普通保存状态写入。
- 编辑 Key 时，未改变 `group_id` 不会重新触发分组绑定授权，避免“只改标签”被历史不可绑定分组阻断。

### 企业成员完整目标架构（实现中）

- 企业能力使用独立 `account_type`，不把 `enterprise` 混入 admin/user 授权角色。
- 引入不可登录的稳定成员实体；成员聚合多把 Key，并继承有序分组集合。
- 成员 Key 使用请求级 `ActiveGroup` 完成跨协议分派、分组内账号 failover 和受控跨分组 fallback。
- 成员 5h、1d、7d 与自然月聚合限额由成员名下全部 Key 共享，采用同一持久化预留、预算账本、幂等结算、崩溃恢复和对账，不以调用后累加冒充严格硬限额。
- 成员编号作为不可复用的导入/审计身份，创建后普通编辑不可修改；成员编辑可以带原因调整各窗口已用投影，自然月差额保留不可变账本证据。
- 成员分组编辑候选来自企业 owner 当前可访问分组；勾选表达授权，排序表达调用优先级和受控 fallback 顺序。
- 迁移当月开账与真实 usage 分开保存；普通 Key 行为和现有 owner analytics 保持兼容。
- 当前已落地成员实体、多 Key、有序分组、普通 Key 显式事务迁移、ActiveGroup、预算 reservation/账本/恢复/对账、成员级请求记录、CSV/XLSX 持久化慢导入 job、一次性加密 Key 结果交付、append-only 审计、无高基数标签的 Ops 指标、正式 zh/en i18n 和企业成员控制台；真实 PostgreSQL/Redis 已验证多 worker 唯一领取、心跳续租、租约 fencing、异常 processing 恢复、5000 成员事务、Redis 重启订阅恢复、PostgreSQL 事务中断回滚和跨实例认证 L1 失效，浏览器 E2E、集群指标汇总、混合负载容量与持续性网络故障仍按设计合同继续收口。
- Grok 异步视频任务在返回上游任务 ID 前持久化 owner/member/Key/实际 group/account；状态查询只回到原任务账号，不重新选择成员候选或跨账号 failover。
- 权威设计见 [企业用户成员管理](../features/enterprise-member-management.md) 与 [ADR-0003](../decisions/adr-0003-enterprise-member-entity.md)。

### 用量分析

- 用户侧单 Key 下钻已落地：趋势、模型分布、请求记录三块功能。
- 企业 owner 级用量分析已落地在用户 Usage 页的分析 Tab，包括 summary、leaderboard、models、groups、tags、trend。
- owner analytics 接口在 `/api/v1/usage/analytics/*`，所有查询绑定当前登录用户，不接收外部 `user_id`。
- owner DTO 不返回 `account_cost`、上游账号、渠道、`upstream_model` 等管理员字段。
- 标签聚合采用“多标签重复计入”的归因语义，不作为严格财务分摊。
- 管理员用量下钻已落地：`/admin/usage` 顶部对象选择器、用户/Key 双栏下钻、路由 query 同步（含显式日期范围回写，刷新和分享链接保留时间口径）、趋势月粒度和密集图表展开。
- 已删除 Key 证据展示阶段 1 已落地：`/admin/usage` 管理员证据视图穿透软删除 hydrate Key 名称与删除状态，DTO 隐藏明文 key，导出保留已删除 Key 的 ID、名称和删除时间；用户侧 `/usage` 和普通 Key 列表仍只解析活跃 Key。

### 可用渠道模型与账号模型维护

- 用户侧可用渠道增加模型级表格视图和导出功能。
- 账号模型配置支持从上游 `/v1/models` 探测、将探测结果写入白名单或同名映射行。
- 自定义模型输入可以查询 models.dev 目录。
- 映射模式支持清空全部模型，并保留映射模式语义。

### UI 与运维体验

- 首页、认证页、控制台布局、通用表单/表格/弹窗、管理端/用户端页面统一到当前 stone / neutral / emerald 视觉方向。
- 前端隐藏 LinuxDo / 微信登录、注册、资料绑定和管理端认证显示入口；后端 OAuth 能力保留。
- 运维明细弹窗支持父子层叠，Escape、遮罩和滚动锁只作用于最上层弹窗。
- 运维错误详情和上游响应预览改为阅读型自动换行，降低长 JSON 横向滚动负担。
- 管理端新增独立提示词输入审计工作台，覆盖 Guard 节点配置、运行态、事件筛选/详情和确认删除；功能、阻断和通过事件保存默认均关闭，Guard token 不从管理 API 回显。
- `step_up_enabled` 与 `session_binding_enabled` 作为默认关闭的显式安全开关；启用后继续沿用 TOTP、会话绑定和操作审计合同。

### 数据保留与运维策略

- dashboard aggregation 自动清理默认关闭。
- ops cleanup 自动清理默认关闭。
- 管理员仍可手动创建用量清理任务、取消任务或清理运维日志。
- 默认保留运行数据，删除动作必须是显式管理操作。

### 部署、发布与 CI

- fork 镜像默认值为 `thornboo/sub2api:latest`，也支持 `ghcr.io/thornboo/sub2api:latest`。
- 上游镜像 `weishaw/sub2api:latest` 不包含 dev-zz 二开，不应用于本分支部署。
- `deploy/docker-deploy.sh` 默认从 `thornboo/sub2api` 的 `dev-zz` 分支拉取部署文件。
- `deploy/backup-dev-zz.sh` 作为发布镜像更新前的备份入口；`deploy/build-image-dev-zz.sh` 仅保留为本地源码构建、开发验证和远程镜像不可用时的镜像打包路径。
- CI 引入 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` 验证 GitHub JavaScript actions runtime；项目构建 Node 版本仍是 20。

## 文档归档规则

- 功能说明放在 `docs-site/dev-zz/features/`。
- 稳定接口和实现口径放在 [接口索引](./api-surface.md)。
- 配置、迁移、镜像和 CI 约束放在 [配置与迁移索引](./configuration-and-migrations.md)。
- 每次用户可见变化先写 [变更记录](../changelog.md)，再在 [补丁记录](../patches.md) 记录实现和验证。
