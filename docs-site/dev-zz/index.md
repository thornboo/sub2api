# dev-zz 文档

`dev-zz` 是在上游 `main` 基础上长期维护的二次开发分支。`docs-site/dev-zz/` 是这条分支的正式文档中心，旧的 `secondary-dev/` 不再作为资料入口。

## 当前分支画像

- 对比口径：`origin/main...dev-zz-develop`。`dev-zz-develop` 是当前持续开发分支，验证后再推进到正式线 `dev-zz`。
- 当前开发 HEAD：`48f62862`（上游 `origin/main` 为 `945b9b20`）。
- 差异规模：339 个文件，约 34828 行新增、4174 行删除。
- 主要方向：企业 API Key 管理、owner 用量分析、管理员用量下钻、已删除 Key 证据保全、可用渠道模型展示、账号模型探测、fork 镜像部署、控制台体验、运维体验和 CI/发布流程。

完整归纳见 [dev-zz 变更地图](./reference/change-map.md)。

## 推荐阅读顺序

| 目的 | 文档 |
| --- | --- |
| 先了解 dev-zz 改了什么 | [变更地图](./reference/change-map.md) |
| 查分支、镜像和发布边界 | [分支策略](./branch-policy.md) |
| 查用户可见变化 | [变更记录](./changelog.md) |
| 查具体改动和验证记录 | [补丁记录](./patches.md) |
| 查新增接口和字段边界 | [接口索引](./reference/api-surface.md) |
| 查配置、迁移、镜像和 CI | [配置与迁移索引](./reference/configuration-and-migrations.md) |
| 本地启动开发 | [完全本地开发指南](./development/local-development.md) |
| 部署或更新服务器 | [dev-zz 部署](./deployment/deploy-dev-zz.md) |
| 从上游 main 同步 | [同步上游 main](./maintenance/merge-main.md) |
| 确认该跑哪些测试 | [验证矩阵](./testing/verification-matrix.md) |
| 排查删除 Key 后用量是否丢失 | [删除 Key 后用量显示归零排查](./maintenance/deleted-key-usage-ledger-triage-2026-06-22.md) |

## 功能状态

| 状态 | 文档 | 当前口径 |
| --- | --- | --- |
| 已落地 | [可用渠道模型广场与报价导出](./features/available-channels-model-marketplace.md) | 用户侧模型表格、当前可见报价导出和管理员全量目录导出已实现。 |
| 已落地 | [API Key 用量下钻](./features/api-key-usage-drilldown.md) | 用户侧单 Key 趋势、模型分布和请求记录下钻已实现。 |
| 已落地 | [管理员用量分析下钻](./features/admin-usage-profile-drilldown.md) | 管理员侧用户 / API Key 下钻入口、日期回写和月粒度趋势已实现。 |
| 已落地 + 目标设计 | [企业用量分析中心](./features/enterprise-usage-analytics.md) | 现有 owner Key analytics 已落地；成员 → Key → 实际分组的完整分析合同已确认、待实现。 |
| 目标设计 | [企业用户成员管理](./features/enterprise-member-management.md) | 不可登录成员、聚合 Key、请求级 ActiveGroup、强成员预算、导入账本和权限撤销传播的完整目标架构。 |
| 部分落地 | [用量账本与已删除 Key 证据完整性](./features/usage-ledger-evidence-integrity.md) | 阶段 1 管理员证据视图已实现；快照字段和外键约束仍是方案。 |
| 方案稿 | [上游账号生命周期与数据保留策略](./features/account-lifecycle-data-retention.md) | 定义账号停用、归档和永久删除/Purge 的数据保留、统计变化、权限和验收口径。 |
| 部分落地 | [上游供应商资金池与成本账本](./features/upstream-cost-pools-and-ledger.md) | 阶段 1 后端兼容层、供应商生命周期、严格创建 / 审计保留、默认结算 Modal、真实快照隔离、简化充值录入和账号供应商归属已落地；本期仍仅支持正向账本子集，资金池高级管理、合池、余额迁移和 usage 成本证据仍是后续阶段。 |
| 部分落地 | [上游供应商成本感知与模型级调度](./features/upstream-provider-cost-aware-scheduling.md) | 模型级错误转移、真实快照综合折扣和 `strict_priority` / `cost_first` 已落地；余额查询及 balanced / canary 仍是后续设计。 |
| 部分落地 | [运维监控客户可见错误排障](./features/ops-customer-visible-error-triage.md) | 第一版已拆分客户可见失败、SLA 错误、客户侧限制和上游 429/529 明细入口；流式 200 后失败仍是后续阶段。 |
| 核心已落地 | [运维失败分类与 SLA 重构](./features/ops-failure-classification-redesign.md) | v2 分类双写、31 天回填、raw/preagg 统一统计、结构化钻取、15 分钟当前状态及健康/告警/报表口径已实现；主要故障事件聚合和 HTTP 200 后流式终态去重仍待后续阶段。 |
| 已落地 | [定价驱动的站点自检模型监控](./features/pricing-driven-self-check-monitoring-design.md) | 用户侧 `/monitor` 按分组/模型展示健康状态，由渠道定价的「自检」开关驱动站点自检探针（走真实网关链路但不计费、不影响生产调度），并撤下旧用户侧探针接口；上游渠道监控保持现状、仅管理员排障。 |
| 部分落地 | [模型状态时间线与无可用账号证据保留](./features/model-status-timeline-evidence-retention.md) | 阶段 1 已实现：为 (分组, 模型) 持久化脱敏状态快照，详情时间线和详情指标优先读快照；列表状态切快照和时间桶化仍是后续阶段。 |

## 设计取舍

- [设计取舍 0001：docs-site 作为 dev-zz 文档中心](./decisions/adr-0001-docs-site-as-dev-zz-doc-hub.md)
- [设计取舍 0002：用 Key 承载企业成员管理，不引入子账号实体（已被 0003 取代）](./decisions/adr-0002-key-as-enterprise-member.md)
- [设计取舍 0003：不可登录成员实体、聚合 Key 与成员预算](./decisions/adr-0003-enterprise-member-entity.md)
- [设计取舍 0004：运维失败分类、责任归属与 SLA 口径](./decisions/adr-0004-ops-failure-taxonomy-and-sla.md)

## 记录规则

- 用户可见的行为、样式、模块、路由、构建或运行方式变化，更新 [变更记录](./changelog.md) 和 [补丁记录](./patches.md)。
- 新增或变更 dev-zz 接口，更新 [接口索引](./reference/api-surface.md)。
- 新增迁移、配置默认值、镜像、CI 或发布策略，更新 [配置与迁移索引](./reference/configuration-and-migrations.md)。
- 上游 `main` 同步进 `dev-zz-develop` 或 `dev-zz`，更新 [上游合并记录](./maintenance/merge-log.md)。
- 临时需求资料不要塞进总览页；确实需要保留时，放到对应的功能、补丁、接口或设计取舍文档里。
- 文档中不存储密钥、访问令牌、私有凭据或环境敏感值。
