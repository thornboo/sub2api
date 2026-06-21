# dev-zz-apipool 分支清单

## 这份记录怎么看

- 这是只读分支审计记录；审计时没有切换分支，也没有改写分支历史。
- 审计日期：2026-06-21。
- 当前工作分支：`dev-zz-develop`，当前提交 `738a15be`。
- 审计对象是本地与远端分支 `dev-zz-apipool` / `origin/dev-zz-apipool`。
- 这份记录只回答三件事：当时做了什么、为什么不能直接合并、如果要恢复应该怎么拆。

这份文档只记录 `dev-zz-apipool` 分支。它和本地 stash 里的 DEV_SEED、维度化计费设计不是同一个需求。

## 分支状态

| 项 | 值 |
| --- | --- |
| 分支提交 | `20852bc13b72d05330a37a9d7173fa3245fb5fc4` |
| 提交标题 | `Route provider traffic through account key pools` |
| 与当前 `dev-zz-develop` 的 merge-base | `e6e7e5b9b0c4ed2c125d864f71b286b9adc96a4d` |
| 当前 `dev-zz-develop` 相对它领先 | 321 个提交 |
| 它相对当前 `dev-zz-develop` 领先 | 1 个提交 |
| 分支/远端状态 | 本地 `dev-zz-apipool` 与 `origin/dev-zz-apipool` 均指向 `20852bc1` |

这个分支只比当时的 fork 点多一个功能提交，但已经落后当前开发线 321 个提交。直接看 `dev-zz-develop..dev-zz-apipool` 会出现大量删除和回退，因为它没有后续的 docs-site、迁移、调度、前端和上游合并工作。判断功能范围时，应看 merge-base 到 `dev-zz-apipool` 的差异，不要把它直接 merge 回当前分支。

## 需求归属

`dev-zz-apipool` 实现的是“账号内上游子 API Key 池”和“子 key + 模型级冷却隔离”。

它不是：

- 开发环境假数据自动种子。
- 维度化计费。
- 上游余额查询。
- 完整成本感知调度。

它可以作为上游调度优化的一块基础能力，但不是完整方案。

## 它解决的问题

核心目标是：一个上游账号下面可以维护多把真实上游 API Key，失败时只影响尽可能小的范围。

- 请求先按现有账号调度选择父账号，再在父账号内部选择一把子 key。
- 子 key 可以配置自己的状态、优先级、模型白名单和模型映射。
- 某把子 key 对某个上游模型失败时，只冷却这把子 key 的这个模型。
- 如果父账号内其它子 key 仍可用，系统可以继续使用同一个供应商账号。
- 如果这个父账号所有子 key 都不可用，才把请求交给下一账号。

这和“上游成本感知与模型级调度”有关：它提供了更细的健康隔离，避免因为一个模型或一把上游 key 故障，就把整个低成本供应商冷却掉。

## 主要后端设计

新增的核心表和结构：

- `account_api_keys`：父账号下的子 API Key 池。
- `account_api_key_model_cooldowns`：子 key 维度的模型冷却状态。
- `AccountAPIKey`：包含名称、密钥、优先级、状态、模型限制、模型映射、全局冷却、最近请求/错误计数、模型冷却列表。
- `AccountAPIKeySelection`：转发时选择出的父账号、子 key、最终上游模型和鉴权密钥。

关键行为：

- `EffectiveAPIKeySelectionsForRequest` 会按优先级、最近使用时间和 ID 排序子 key。
- 子 key 支持 `whitelist` 和 `mapping` 两类模型限制。
- `ResolveUpstreamModelForRequest` 允许子 key 的模型映射覆盖父账号映射后的上游模型。
- 当账号没有配置子 key 池时，保留旧的单 `credentials.api_key` 路径。
- 当账号配置了子 key 池但没有可调度子 key 时，不再回退到旧密钥，而是让上层调度尝试其它账号。
- 子 key 失败会写入 `account_api_key_model_cooldowns`，并更新最近错误计数。
- 子 key 使用会更新 `last_used_at` 和最近请求计数。

## 覆盖的转发路径

这个提交把 API-key 池扩展到了当时的多个 API-key 转发路径：

- Claude / Anthropic API-key 转发。
- OpenAI API-key Responses passthrough。
- OpenAI Images。
- Gemini messages compatibility。
- Antigravity API-key / upstream forwarding。
- Bedrock `auth_mode=apikey`。

流式请求有一条重要约束：如果已经向客户端写出字节，就不做透明子 key failover，避免同一个响应流被多个上游拼接。

## 前端与接口

分支新增了账号编辑中的 `AccountAPIKeyPoolEditor.vue`：

- 在创建/编辑上游账号时维护子 API Key。
- 支持子 key 名称、密钥、优先级、状态。
- 密钥为空表示保留旧值，避免编辑时回显真实 secret。
- 每把子 key 可配置模型白名单或模型映射。
- 可对单把子 key 做模型探测。
- 可展示最近请求数、错误数、最后使用时间和模型冷却 badge。

接口和类型上，账号响应、创建请求、更新请求都新增了 `api_keys` 字段。

如果要恢复这部分前端，需要重新适配现在的视觉和交互。它来自较早的界面时期，还保留了旧圆角、旧布局和手写控件痕迹，不适合原样并入当前 dev-zz 管理后台。

## 当时的验证记录

提交信息记录的验证包括：

- `mise x -C backend -- go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/server`
- `pnpm typecheck`
- `pnpm lint:check`
- `git diff --check`
- `git diff --cached --check`

分支中的测试覆盖了关键故障语义，例如：

- Claude 子 key 失败只冷却被选中的 key 和模型。
- OpenAI API-key passthrough 子 key 失败只冷却被选中的 key 和模型。
- Gemini compatibility、Antigravity、Bedrock API-key 路径也有类似测试。
- 子 key pool 存在但不可用时，应交给下一账号，而不是把父账号旧密钥拿来兜底。

## 当前恢复风险

这条分支现在不适合直接合并，主要风险有：

1. **迁移编号冲突**
   - 分支新增了 `135_account_api_key_pool.sql`、`136_account_api_key_pool_defaults.sql`、`137_account_api_key_pool_scheduler_indexes_notx.sql`。
   - 当前开发线已经有多个 `135`、`136`、`137` 迁移，并且迁移编号已经推进到 `157`。
   - 恢复时必须把这些迁移顺延到当前最新编号之后，并同步更新迁移测试和文档。

2. **调度器已经继续演进**
   - 当前代码已有 `model_rate_limits`、scheduler outbox 修复、pending dedup index、账号过期自动暂停、图片限流故障转移和更多模型级语义。
   - 恢复子 key pool 时，不能覆盖这些后续修复。

3. **错误分类和冷却规则需要重审**
   - 旧分支的目标是子 key/model 冷却。
   - 当前需求还包含供应商成本、模型族健康、余额、调度解释和稳定性评分。
   - 不能简单把旧分支视为完整方案，它只是细粒度健康隔离的一块基础。

4. **文档位置过时**
   - 分支中新增了 `secondary-dev/PLAN.md` 和 `secondary-dev/demand/...`。
   - 当前 dev-zz 文档中心已经迁移到 `docs-site/dev-zz/`，恢复时应改写到 docs-site，不再扩展旧 `secondary-dev` 目录。

5. **前端样式与交互需要更新**
   - 当前管理端表格、多选、深色主题和组件风格已经持续调整。
   - 旧 editor 可以作为交互草稿，但不应原样并入。

## 如果要恢复

如果要重新启用这块功能，建议按下面的顺序做：

1. 从当前 `dev-zz-develop` 新建恢复分支，不在 `dev-zz-apipool` 上继续开发。
2. 用 `20852bc1` 的 merge-base diff 作为参考，按模块抽取，而不是直接 merge。
3. 先恢复数据模型和仓储层，并把迁移编号顺延到当前最新迁移之后。
4. 再恢复服务层选择器，先只覆盖一个关键平台路径，锁定“子 key/model 冷却不影响父账号其它模型”的行为。
5. 逐步接回 Claude、OpenAI、Gemini、Antigravity、Bedrock 等路径。
6. 前端重新设计账号编辑中的“上游子 key 池”区域，复用当前组件和样式。
7. 把旧 `secondary-dev` 说明改写为 docs-site 的功能文档和迁移记录。
8. 再结合“上游成本感知与模型级调度”设计，明确父账号调度和子 key 调度各自负责什么。
