# 项目说明

Sub2API 是面向 AI API 转发、订阅额度分发、多账号管理和用量计费的网关平台。后端统一处理认证、账号调度、额度、用量记录和支付回调，前端提供用户侧控制台和管理侧控制台。

## 核心能力

| 能力 | 说明 |
| --- | --- |
| 多上游账号管理 | 管理 OpenAI 兼容账号、渠道、分组、代理、模型白名单和映射 |
| 用户 API Key 分发 | 用户创建 Key，绑定分组、额度、限流、过期时间和 IP ACL |
| 用量统计与计费 | 记录请求、Token、图片、缓存、实际扣费和标准计费 |
| 分组与订阅 | 支持分组倍率、订阅权益、用户可用分组和渠道可见性 |
| 管理端控制台 | 用户、账号、渠道、用量、支付、公告和运维配置 |
| 用户端控制台 | API Key、用量记录、订阅、可用渠道、资料和安全设置 |
| 支付系统 | 支持多支付方式、服务商实例、Webhook 和外部 Admin 集成 |
| PostgreSQL / Redis | PostgreSQL 持久化，Redis 用于缓存、限流和部分运行时状态 |

## 仓库结构

| 路径 | 说明 |
| --- | --- |
| `backend/` | Go 后端、ent schema、迁移、handler、service、repository |
| `frontend/` | Vue/Vite 前端控制台 |
| `deploy/` | Docker、Compose、安装脚本、部署样例和运行配置 |
| `docs/` | 上游兼容或 GitHub 直接阅读的项目文档 |
| `docs-site/` | VitePress 文档站，当前 dev-zz 的正式文档中心 |
| `docs-site/project/` | 项目稳定使用文档 |
| `docs-site/dev-zz/` | dev-zz 二开功能、策略、部署、维护、接口和验证文档 |

## dev-zz 分支说明

当前 checkout 的 `dev-zz` 是二次开发分支。它在上游 `main` 基础上持续吸收修复，同时保留已经形成的企业 Key 管理、owner 用量分析、模型/渠道展示、部署镜像、运维体验和控制台视觉策略。

二开内容请从 [dev-zz 文档](../dev-zz/index.md) 开始阅读。需要快速了解 dev-zz 相对上游改动范围时，优先看 [变更地图](../dev-zz/reference/change-map.md)。

## 上游兼容文档

仓库根目录 `docs/` 仍保留若干可直接在 GitHub 阅读的文档，便于和上游合并时减少冲突：

- `docs/PAYMENT.md`
- `docs/PAYMENT_CN.md`
- `docs/ADMIN_PAYMENT_INTEGRATION_API.md`

这些文档不替代 `docs-site/dev-zz/` 的二开记录。
