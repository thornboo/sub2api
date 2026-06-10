# 项目说明

Sub2API 是面向 AI API 转发、订阅额度分发和多账号管理的网关平台。它通过后端统一处理认证、账号调度、额度和用量记录，前端提供用户侧和管理侧控制台。

## 主要能力

- 多上游账号管理
- 用户 API Key 分发
- token 级用量统计和费用计算
- 分组、渠道、模型定价管理
- 管理端和用户端控制台
- 内置支付系统
- PostgreSQL 持久化和 Redis 缓存

## 文档范围

本目录是项目级文档，涵盖支付、集成、配置、部署和接口说明。

dev-zz 分支专属的二开策略、维护记录、合并日志和部署方式，见 [dev-zz 文档](../dev-zz/index.md)。

## 上游兼容文档

仓库根目录的 `docs/` 保留了几份可在 GitHub 直接阅读、便于和上游合并的文档：

- `docs/PAYMENT.md`
- `docs/PAYMENT_CN.md`
- `docs/ADMIN_PAYMENT_INTEGRATION_API.md`
