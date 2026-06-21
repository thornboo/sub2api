# 项目文档

`project/` 放的是 Sub2API 的稳定使用文档，主要给普通用户、管理员、部署者和外部集成方阅读。

## 内容

| 文档 | 说明 |
| --- | --- |
| [项目说明](./overview.md) | Sub2API 的定位、主要能力和仓库结构 |
| [支付系统配置指南](./payment/payment-cn.md) | 支付方式、服务商实例、Webhook、迁移说明 |
| [Admin 支付集成 API](./payment/admin-payment-integration-api.md) | 外部后台创建兑换码、查询用户和余额调整接口 |

## 与 dev-zz 的关系

`project/` 尽量写成通用说明，不放 dev-zz 的补丁流水和分支决策。

dev-zz 专属内容放在：

- [dev-zz 文档入口](../dev-zz/index.md)
- [dev-zz 变更地图](../dev-zz/reference/change-map.md)
- [dev-zz 接口索引](../dev-zz/reference/api-surface.md)
- [dev-zz 配置与迁移索引](../dev-zz/reference/configuration-and-migrations.md)

## 维护规则

- 稳定使用说明写在 `docs-site/project/`。
- 需要同时保留 GitHub 根目录可读版本的内容，继续放在仓库根目录 `docs/`。
- 二开分支策略、部署差异、接口差异、验证命令和上游合并记录不要写进 `project/`，避免通用说明和分支档案互相串味。
