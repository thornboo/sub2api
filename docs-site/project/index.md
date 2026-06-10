# 项目文档

Sub2API 面向使用者、管理员和部署者的稳定说明文档。

## 内容

- [项目说明](./overview.md)：Sub2API 是什么、能做什么、整体结构。
- [支付系统配置指南](./payment/payment-cn.md)：支付功能的配置说明。
- [Admin 支付集成 API](./payment/admin-payment-integration-api.md)：管理端支付集成的接口参考。

> dev-zz 分支专属的二开策略、变更、补丁与部署差异，见 [dev-zz 文档](../dev-zz/index.md)。

## 维护说明

- 稳定的使用类说明放在 `project/`；`docs/` 保留为上游兼容入口，不随意删除。
- 需要结构化重写时，在 `docs-site/project/` 维护更完整的版本。
