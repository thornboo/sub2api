# dev-zz 文档

`dev-zz` 是基于上游 `main` 维护的二次开发分支。`docs-site/dev-zz/` 是本分支的正式文档中心，替代旧的 `secondary-dev/` 目录。

## 当前分支画像

- 对比口径：`origin/main...dev-zz`。
- 当前 HEAD：`3a7d0474`。
- 差异规模：243 个文件，约 25923 行新增、3623 行删除。
- 主要方向：企业 API Key 管理、owner 用量分析、可用渠道模型展示、账号模型探测、fork 镜像部署、控制台 UI/运维体验和 CI/发布链路。

完整归纳见 [dev-zz 变更地图](./reference/change-map.md)。

## 推荐阅读顺序

| 目的 | 文档 |
| --- | --- |
| 先了解 dev-zz 改了什么 | [变更地图](./reference/change-map.md) |
| 查用户可见变化 | [变更记录](./changelog.md) |
| 查实现与验证 | [补丁记录](./patches.md) |
| 查新增接口和字段边界 | [接口索引](./reference/api-surface.md) |
| 查配置、迁移、镜像和 CI | [配置与迁移索引](./reference/configuration-and-migrations.md) |
| 本地启动开发 | [完全本地开发指南](./development/local-development.md) |
| 部署或更新服务器 | [dev-zz 部署](./deployment/deploy-dev-zz.md) |
| 从上游 main 同步 | [合并 main 到 dev-zz](./maintenance/merge-main.md) |
| 确认该跑哪些测试 | [验证矩阵](./testing/verification-matrix.md) |

## 功能文档

- [企业客户 Key 成员管理](./features/enterprise-key-member-management.md)
- [API Key 用量下钻](./features/api-key-usage-drilldown.md)
- [企业用量分析中心](./features/enterprise-usage-analytics.md)
- [用量账本与已删除 Key 证据完整性](./features/usage-ledger-evidence-integrity.md)
- [可用渠道模型广场与报价导出](./features/available-channels-model-marketplace.md)

## 决策记录

- [ADR 0001: docs-site 作为 dev-zz 文档中心](./decisions/adr-0001-docs-site-as-dev-zz-doc-hub.md)
- [ADR 0002: 用 Key 承载企业成员管理，不引入子账号实体](./decisions/adr-0002-key-as-enterprise-member.md)

## 记录规则

- 用户可见的行为、样式、模块、路由、构建或运行方式变化，更新 [变更记录](./changelog.md) 和 [补丁记录](./patches.md)。
- 新增或变更 dev-zz 接口，更新 [接口索引](./reference/api-surface.md)。
- 新增迁移、配置默认值、镜像、CI 或发布策略，更新 [配置与迁移索引](./reference/configuration-and-migrations.md)。
- 上游 `main` 合并进 `dev-zz`，更新 [上游合并记录](./maintenance/merge-log.md)。
- 临时需求资料不写进总览页；需要保留时，放入对应的功能、补丁、接口或决策文档。
- 文档中不存储密钥、访问令牌、私有凭据或环境敏感值。
