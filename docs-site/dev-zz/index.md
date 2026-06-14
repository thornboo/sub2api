# dev-zz 文档

`dev-zz` 是基于上游 `main` 维护的二次开发分支。本目录是它的正式文档中心。

## 文档导航

- [分支策略](./branch-policy.md)：dev-zz 与上游 main 的关系、保留策略和合并原则。
- [变更记录](./changelog.md)：用户可见的行为、样式、模块、路由、构建或运行方式变化。
- [补丁记录](./patches.md)：每个补丁的范围、影响文件、实现说明和验证命令。
- [本地开发](./development/local-development.md)：前端、后端、数据库的全本地开发方式。
- [dev-zz 部署](./deployment/deploy-dev-zz.md)：从源码构建并部署 dev-zz 的方式。
- [企业客户 Key 成员管理](./features/enterprise-key-member-management.md)：把 API Key 作为企业员工席位的批量创建、标签、批量维护和聚合监控计划。
- [API Key 用量下钻](./features/api-key-usage-drilldown.md)：单把 API Key 的趋势、模型分布和请求记录下钻设计与实现记录。
- [企业用量分析中心](./features/enterprise-usage-analytics.md)：企业 owner 员工 Key 排行、分组/标签/模型分析、管理员全站分析和多供应商 Key 方案。
- [合并 main 到 dev-zz](./maintenance/merge-main.md)：上游同步流程和验证清单。
- [上游合并记录](./maintenance/merge-log.md)：每次合并 main 的具体结果。

## 记录规则

- 用户可见的行为、样式、模块、路由、构建或运行方式变化，更新 [变更记录](./changelog.md) 和 [补丁记录](./patches.md)。
- 上游 `main` 合并进 `dev-zz`，更新 [上游合并记录](./maintenance/merge-log.md)。
- 临时需求资料不写进总览页；需要保留时，放入对应的功能、补丁或决策文档。
- 文档中不存储密钥、访问令牌、私有凭据或环境敏感值。

> 本文档中心由 `secondary-dev/` 目录演进而来，迁移背景见 [ADR 0001](./decisions/adr-0001-docs-site-as-dev-zz-doc-hub.md)。
