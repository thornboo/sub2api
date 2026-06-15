# dev-zz 分支策略

`dev-zz` 是在上游 `main` 基础上维护的二开分支。它需要持续吸收上游正确性、安全性和兼容性修复，同时保留本分支已经形成的产品、视觉、部署、运维和企业管理策略。

## 基本原则

- 上游正确性修复、兼容性修复、安全修复和后端运行时修复优先吸收。
- dev-zz 已记录的行为，不因上游合并自动丢弃。
- 处理冲突前，先阅读本文件、[补丁记录](./patches.md)、[变更地图](./reference/change-map.md) 和 [上游合并记录](./maintenance/merge-log.md)。
- 每次合并完成后，必须记录到 [上游合并记录](./maintenance/merge-log.md)。
- 影响用户可见行为、部署方式、接口语义或数据库结构时，同时更新 [变更记录](./changelog.md)、[补丁记录](./patches.md) 和必要的 reference 文档。

## 默认保留的 dev-zz 策略

| 策略 | 当前口径 |
| --- | --- |
| 文档中心 | `docs-site/` 是完整文档中心，`secondary-dev/` 不再恢复 |
| 前端视觉 | 保留 stone / neutral / emerald 控制台方向，除非明确决定跟随上游重做 |
| 认证展示 | 前端隐藏 LinuxDo / 微信入口；后端能力不因此删除 |
| 数据保留 | usage 和 ops 自动清理默认关闭，由管理员显式清理 |
| 企业 Key | API Key 作为企业员工席位，不引入子账号登录实体 |
| Key 状态 | `disabled` 是禁用持久化值，`inactive` 仅作为旧别名 |
| 标签与批量 | `api_keys.tags` 是 jsonb 数组，批量操作必须保持所有权、事务和上限 |
| 用量分析 | owner 只能看自己的 Key 和 `actual_cost`；admin-only 成本字段不外泄 |
| 模型维护 | 保留账号模型探测、模型映射、models.dev 查询和可用渠道模型表格 |
| 部署 | 生产默认使用 fork 镜像 `thornboo/sub2api:latest`，本地源码构建是开发/应急路径 |

## 合并判断

| 上游变更类型 | 处理方式 |
| --- | --- |
| 后端正确性 / 安全修复 | 优先接受，除非破坏已记录的 dev-zz 策略 |
| API / DTO 变更 | 检查 owner/admin 字段边界、前端类型和接口索引 |
| 前端 UI / 交互 | 对照 dev-zz 视觉方向和已记录行为后再合并 |
| 认证入口 | 后端能力可保留；前端展示遵循 dev-zz 隐藏策略 |
| 数据清理 / 保留 | 默认保留 dev-zz 的显式清理策略 |
| 部署 / release | 保留 fork 镜像和 `dev-zz` 分支默认来源，吸收通用修复 |
| 数据库迁移 | 检查迁移编号、事务要求、索引锁表风险和 ent schema |

## 冲突处理纪律

1. 先用只读方式预判冲突：

   ```bash
   git merge-tree --write-tree "$(git merge-base HEAD origin/main)" HEAD origin/main
   ```

2. 真正合并时优先保留可验证的 dev-zz 策略，不凭记忆判断。
3. 冲突文件只解决当前冲突，不顺手重构无关区域。
4. 解决后按 [验证矩阵](./testing/verification-matrix.md) 选择最小有效验证。
5. 合并日志写清楚 base、目标、上游 head、冲突文件、取舍、验证命令和未验证范围。

## 记录要求

- 合并 main：更新 [上游合并记录](./maintenance/merge-log.md)。
- 用户可见变化：更新 [变更记录](./changelog.md)。
- 补丁实现：更新 [补丁记录](./patches.md)。
- 新接口或接口语义：更新 [接口索引](./reference/api-surface.md)。
- 配置、迁移、镜像、CI：更新 [配置与迁移索引](./reference/configuration-and-migrations.md)。
- 长期设计决策：放入 `docs-site/dev-zz/decisions/`。
