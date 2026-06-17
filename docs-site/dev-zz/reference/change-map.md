# dev-zz 变更地图

本文按 `origin/main...dev-zz` 的真实差异整理二开范围。最近一次盘点基于：

| 项 | 值 |
| --- | --- |
| dev-zz HEAD | `3a7d0474` |
| origin/main | `e34ad2b1` |
| merge-base | `e34ad2b1` |
| 差异规模 | 243 个文件，约 25923 行新增、3623 行删除 |

## 变更分布

| 区域 | 文件数 | 说明 |
| --- | ---: | --- |
| `frontend/` | 133 | 用户/API Key、owner 用量分析、可用渠道模型、运维弹窗栈、主题与控制台 UI |
| `backend/` | 63 | API Key 批量/标签/状态、owner analytics、usage 聚合、配置默认值、测试与迁移 |
| `docs-site/` | 23 | dev-zz 文档中心、功能文档、部署/开发/维护记录 |
| `deploy/` | 13 | fork 镜像默认值、源码构建脚本、Compose/安装脚本与部署样例 |
| `.github/` | 3 | CI、release、security scan 的 Node 24 actions runtime 验证 |
| 根目录 / README / Dockerfile | 8 | release 镜像、版本号、项目说明、分布式 Dockerfile 与设计索引 |

## 已落地能力

### 企业 API Key 管理

- API Key 被确认为企业成员席位，不引入员工登录实体。
- 用户侧 Key 支持结构化 `tags`，新增 `jsonb` 字段和 GIN 索引。
- 创建、编辑、列表筛选、批量创建、批量更新、批量删除均支持标签。
- 批量创建支持模板名或名称列表；响应首次展示明文 Key，幂等重放会脱敏。
- 批量更新和删除支持 `apply_to=selected` 与 `apply_to=filtered` 两种目标。
- 筛选批量目标支持 `search`、`status`、`group_id`、`tags`，且要求至少一个筛选条件，单次最多 500 把 Key。
- Key 状态以 `active` / `disabled` 为可写状态，`inactive` 仅作为旧别名归一化到 `disabled`；`quota_exhausted`、`expired` 是系统状态，不作为普通保存状态写入。
- 编辑 Key 时，未改变 `group_id` 不会重新触发分组绑定授权，避免“只改标签”被历史不可绑定分组阻断。

### 用量分析

- 用户侧单 Key 下钻已落地：趋势、模型分布、请求记录三块能力。
- 企业 owner 级用量分析已落地在用户 Usage 页的分析 Tab，包括 summary、leaderboard、models、groups、tags、trend。
- owner analytics 接口在 `/api/v1/usage/analytics/*`，所有查询绑定当前登录用户，不接收外部 `user_id`。
- owner DTO 不返回 `account_cost`、上游账号、渠道、`upstream_model` 等管理员字段。
- 标签聚合采用“多标签重复计入”的归因语义，不作为严格财务分摊。

### 可用渠道模型与账号模型维护

- 用户侧可用渠道增加模型级表格视图和导出能力。
- 账号模型配置支持从上游 `/v1/models` 探测、将探测结果写入白名单或同名映射行。
- 自定义模型输入可以查询 models.dev 目录。
- 映射模式支持清空全部模型，并保留映射模式语义。

### UI 与运维体验

- 首页、认证页、控制台布局、通用表单/表格/弹窗、管理端/用户端页面统一到当前 stone / neutral / emerald 视觉方向。
- 前端隐藏 LinuxDo / 微信登录、注册、资料绑定和管理端认证显示入口；后端 OAuth 能力保留。
- 运维明细弹窗支持父子层叠，Escape、遮罩和滚动锁只作用于最上层弹窗。
- 运维错误详情和上游响应预览改为阅读型自动换行，降低长 JSON 横向滚动负担。

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
