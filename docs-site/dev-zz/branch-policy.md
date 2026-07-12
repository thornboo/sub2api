# dev-zz 分支策略

`dev-zz` 是在上游 `main` 基础上维护的二开分支。它要持续吸收上游的正确性、安全性和兼容性修复，也要守住本分支已经形成的产品、视觉、部署、运维和企业管理策略。

## 基本原则

- 上游正确性修复、兼容性修复、安全修复和后端运行时修复优先吸收。
- dev-zz 已经记录清楚的行为，不因为合并上游就自动丢掉。
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
| 企业成员 | 保留普通 Key 批量/标签/分析能力；企业长期模型采用独立但不可登录的成员实体、`account_type=enterprise`、成员聚合 Key 与请求级多分组路由，见 ADR-0003 |
| Key 状态 | `disabled` 是禁用持久化值，`inactive` 仅作为旧别名 |
| 标签与批量 | `api_keys.tags` 是 jsonb 数组，批量操作必须保持所有权、事务和上限 |
| 用量分析 | owner 只能看自己的 Key 和 `actual_cost`；admin-only 成本字段不外泄 |
| 模型维护 | 保留账号模型探测、模型映射、models.dev 查询和可用渠道模型表格 |
| 部署 | `dev-zz-develop` 用于测试环境分支镜像；`dev-zz` 保持二开正式线；`latest` 只代表正式发布 |

## 分支与镜像策略

dev-zz 采用“开发集成分支 + 正式稳定分支 + 版本 tag 发布”的轻量流程：

| 分支 / 事件 | 用途 | 镜像口径 |
| --- | --- | --- |
| `dev-zz-develop` | 二开开发、集成和测试环境验证分支 | `ghcr.io/thornboo/sub2api:dev-zz-develop`、`dev-zz-develop-<shortsha>`、`sha-<shortsha>` |
| `dev-zz` | 二开正式稳定分支，只接收已验证改动 | `ghcr.io/thornboo/sub2api:dev-zz`、`dev-zz-<shortsha>`、`sha-<shortsha>` |
| `v*` tag / `Release` workflow | 正式版本发布 | `vX.Y.Z`、`latest` 和对应 release 产物 |

分支镜像的默认 tag 是多架构 manifest，当前覆盖 `linux/amd64` 和 `linux/arm64`。需要排查架构特定问题时，可以使用 `-amd64` 或 `-arm64` 后缀的架构专用 tag。

推荐流转：

```text
dev-zz-fix-* / dev-zz-feature-*
        -> dev-zz-develop
        -> 测试环境验证分支镜像
        -> dev-zz
        -> v* tag 正式发布
```

`dev-zz-develop` 不是长期试错分支。它应始终代表下一批准备进入 `dev-zz` 的候选代码，并定期从 `dev-zz` 同步，避免测试环境和正式稳定线越走越远。

测试环境不要使用 `latest`。正式环境不要使用 `dev-zz-develop` 镜像。需要追溯具体构建时，优先使用带 `<shortsha>` 的镜像 tag 或 `sha-<shortsha>`。

## 合并判断

| 上游变更类型 | 处理方式 |
| --- | --- |
| 后端正确性 / 安全修复 | 优先接受，除非破坏已记录的 dev-zz 策略 |
| API / DTO 变更 | 检查 owner/admin 字段边界、前端类型和接口索引 |
| 前端 UI / 交互 | 对照 dev-zz 视觉方向和已记录行为后再合并 |
| 认证入口 | 后端能力可保留；前端展示遵循 dev-zz 隐藏策略 |
| 数据清理 / 保留 | 默认保留 dev-zz 的显式清理策略 |
| 部署 / release | 保留 fork 镜像、`dev-zz-develop` 测试镜像和 `dev-zz` 正式线边界，吸收通用修复 |
| 数据库迁移 | 检查迁移编号、事务要求、索引锁表风险和 ent schema |

## 冲突处理纪律

1. 先用只读方式预判冲突：

   ```bash
   git merge-tree --write-tree "$(git merge-base HEAD origin/main)" HEAD origin/main
   ```

2. 真正合并时优先保留可验证的 dev-zz 策略，不凭记忆判断。
3. 冲突文件只解决当前冲突，不借机重构无关区域。
4. 解决后按 [验证矩阵](./testing/verification-matrix.md) 选择最小有效验证。
5. 合并日志写清楚 base、目标、上游 head、冲突文件、取舍、验证命令和未验证范围。

## 记录要求

- 合并 main：更新 [上游合并记录](./maintenance/merge-log.md)。
- 用户可见变化：更新 [变更记录](./changelog.md)。
- 补丁实现：更新 [补丁记录](./patches.md)。
- 新接口或接口语义：更新 [接口索引](./reference/api-surface.md)。
- 配置、迁移、镜像、CI：更新 [配置与迁移索引](./reference/configuration-and-migrations.md)。
- 长期设计取舍：放入 `docs-site/dev-zz/decisions/`。
