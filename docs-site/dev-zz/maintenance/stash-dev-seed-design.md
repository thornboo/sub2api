# DEV_SEED stash 设计清单

## 这份记录怎么看

- 这是只读 stash 审计记录；审计时没有应用 stash，也没有删除 stash。
- 审计日期：2026-06-21。
- 审计对象：`stash@{0}`。
- Stash 名称：`On dev-zz: stash secondary-dev DEV_SEED_DESIGN design doc`。
- 这份记录只说明这个 stash 保存了什么，以及如果要恢复，应该迁移到哪里。

这份文档只记录 DEV_SEED 开发假数据设计。它和 `dev-zz-apipool` 分支、维度化计费 stash 都不是同一个需求。

## Stash 状态

| 项 | 值 |
| --- | --- |
| Stash | `stash@{0}` |
| 原路径 | `secondary-dev/DEV_SEED_DESIGN.md` |
| 类型 | 未跟踪文件被 stash 保存 |
| 规模 | 307 行 |
| 原始 base | `a3997b0721ab6d1069e578fc6b1a6cadd4d4b778` |

因为它保存的是未跟踪文件，内容在 stash 的第三个父提交里。只读查看时使用：

```bash
git show stash@{0}^3:secondary-dev/DEV_SEED_DESIGN.md
```

## 需求归属

这个 stash 对应的需求是“开发环境假数据自动种子”。

它不是：

- 上游账号子 API Key 池。
- 模型级冷却。
- 供应商成本感知调度。
- 维度化计费。

它属于本地开发体验改进，目标是让新环境启动后自动具备可操作的测试数据。

## 文档内容

它设计的是 `DEV_SEED` 机制：本地新环境启动后，自动准备一套能用的测试数据。

- dev 分组。
- 渠道价格。
- 禁用的假上游账号。
- 测试用户。
- 固定测试 API Key。
- 订阅计划。
- 兑换码。

主要决策：

- 只在精确设置 `DEV_SEED=local-fixtures` 时启用。
- 拒绝 `true`、`1`、`yes` 等宽松布尔值，避免生产误启。
- `GIN_MODE=release` 下硬阻断。
- 在应用初始化后、HTTP server 启动前同步执行。
- 失败只记录日志，不阻断服务启动。
- 使用 PostgreSQL advisory lock，避免多实例同时种子。
- 通过 service 层创建数据，不绕过业务规则直接写 repository。
- API Key 使用固定测试 key，并先查 `GetByKey` 保证幂等。
- 假上游账号默认 disabled，避免误发真实请求。

## 为什么还值得留

这份设计对本地开发仍有价值，尤其适合解决“刚起环境没有数据，还要手动造账号和 key”的问题。它也能配合 docs-site 的本地开发文档，让开发者一启动就能看到 dashboard、账号、套餐和 key。

它对调度和计费功能的价值是间接的：可以提供稳定演示数据和本地验收数据，但它本身不改变调度或计费逻辑。

## 如果要恢复

- 不建议直接 `git stash apply stash@{0}`，因为它会把文档放回旧 `secondary-dev/` 路径。
- 如果要继续推进，应把内容迁移为 `docs-site/dev-zz/development/` 下的设计文档。
- 实现前需要重新核对当前启动流程、wire 初始化、账号/分组/API Key/兑换码 service 的现状。
- 建议第一版仍保持“显式 opt-in + release 硬阻断 + 假账号 disabled”。
