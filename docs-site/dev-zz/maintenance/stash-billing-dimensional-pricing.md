# 维度化计费 stash 设计清单

## 这份记录怎么看

- 这是只读 stash 审计记录；审计时没有应用 stash，也没有删除 stash。
- 审计日期：2026-06-21。
- 审计对象：`stash@{1}`。
- Stash 名称：`On dev-zz: stash billing dimensional pricing design`。
- 这份记录说明这个 stash 保存了什么，以及它和当前供应商成本讨论有什么关系。

这份文档只记录维度化计费设计。它和 `dev-zz-apipool` 分支、DEV_SEED stash 都不是同一个需求。

## Stash 状态

| 项 | 值 |
| --- | --- |
| Stash | `stash@{1}` |
| 原路径 | `secondary-dev/designs/billing-dimensional-pricing.md` |
| 类型 | 未跟踪文件被 stash 保存 |
| 规模 | 578 行 |
| 原始 base | `437e2df5ec5a8882886a809ea1296b9d685101e5` |

因为它保存的是未跟踪文件，内容在 stash 的第三个父提交里。只读查看时使用：

```bash
git show stash@{1}^3:secondary-dev/designs/billing-dimensional-pricing.md
```

## 需求归属

这个 stash 对应的需求是“维度化计费”。

它不是：

- 上游账号子 API Key 池。
- 模型级冷却。
- 开发假数据种子。
- 上游余额查询。

它和供应商成本感知有关，但更偏底层，也更像一项长期账务架构改造。

## 文档内容

它设计的是“维度化计费”改造，主要解决模型计费维度越来越多之后，旧计费结构难以继续扩展的问题：

- Gemini audio / video token。
- `gpt-image-1` / `gpt-image-2` 的文本 token 与图片 token 拆分。
- reasoning token。
- web search。
- batch discount。
- 未来更多 provider-specific 维度。

它指出当时的代码问题：

- 计费字段是固定列，新增维度需要改 DB、结构体、解析器、计算函数和 UI。
- usage 解析分散在多个路径。
- 主计费路径和 `account_stats` 的 interval 匹配存在不一致风险。
- 使用 `float64` 聚合金额存在精度风险。

目标设计包括：

- `NormalizedUsage = map[Dimension]int64`。
- `Pricing = []RateComponent`。
- 通用成本引擎。
- 维度字典作为单一事实来源。
- 固定列 + JSONB `components` 的混合存储。
- 固定列拥有的维度和 JSON components 维度互斥，避免重复计费。
- `usage_log` 保存自包含计费快照，包括标准化 usage、应用的 rate component、tier、multiplier、breakdown、pricing source、unknown/unpriced dimensions 和 engine version。

阶段计划：

1. 第 0 阶段：先做 usage normalization，零行为变更，用 characterization tests 锁住旧行为。
2. 第 1 阶段：增加 JSON columns。
3. 第 2 阶段：引入统一 billing engine。
4. 第 2.5 阶段：引入精确 Money 类型，单独解决金额精度。
5. 第 3 阶段：前端通用 pricing editor 和可用渠道价格导出。
6. 第 4 阶段：可选地收敛旧固定列。

关键取舍：

- 未知维度第一版不计费，但要报警和保存快照，不自动追溯扣费。
- 不急着用 micro-USD 锁死金额精度方案，因为 token 单价可能非常小。
- 上下文 tier 匹配倾向使用输入侧 context，但要先用 characterization tests 固化现状。
- 不建议为了新模型维度做一次性 quick fix patch。

## 为什么还值得留

这份设计和“上游供应商成本感知”互补：

- 成本感知调度解决“选哪个供应商更划算”。
- 维度化计费解决“某个请求到底由哪些维度构成、按什么价格计算”。
- 供应商倍率、充值比例和模型族折扣最终也需要落到更通用的 pricing component 上，才能覆盖 audio、image、reasoning、search 等复杂维度。

这项工作范围明显大于上游调度，不应该和 API-key pool 或成本感知调度混在一个提交里做。

## 如果要恢复

- 先把它作为 docs-site 的计费设计草案迁移，而不是直接 apply 回 `secondary-dev/designs/`。
- 在实现前重新审计当前 `ChannelModelPricing`、`PricingInterval`、`UsageTokens`、usage log、account stats 和 dashboard 成本字段。
- 第一阶段仍应按原设计里的第 0 阶段做 characterization tests，避免重构计费时改变现有扣费。
- 等供应商成本感知调度设计稳定后，再决定这份维度化计费是否独立立项。
