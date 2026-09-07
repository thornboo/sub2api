# 模型状态 V1 问题分析与调度资格修复方案

> 状态：问题 1、2 已在本地实现；随后增加按优先级探测链路，见[独立实施记录](./model-status-priority-probe-chain.md)。问题 3、4 继续跟踪。未发布或部署。
>
> 分析日期：2026-09-07。源码基线：`dev-zz` / `f1ae078be`。

## 范围与证据边界

用户侧 `/monitor` 由 `ChannelStatusView.vue` 根据 `channel_monitor_mode` 选择 V1/V2。本次重点检查 V1 的站点自检、按 `(group, model)` 聚合和历史快照链路。

2026-09-07 从线上恢复到本地的数据库快照中，`channel_monitor_mode=v1`、`model_self_check_enabled=true`、`self_check_default_interval_seconds=300`。这些是本地快照的配置证据，不代表之后的线上实时状态。

以下四项是分析基线的源码发现；具体影响由列出的配置和账号条件触发。后续经用户授权实施问题 2，保留原始问题和方案作为决策依据。本次没有启动前后端、没有对真实上游发起探测，也没有据此断言某个线上请求故障已被复现。

## 问题总表

编号沿用本次分析报告，便于后续修复和验收逐项追踪。

| 编号 | 优先级 | 问题 | 状态 |
| --- | --- | --- | --- |
| 1 | 高 | Kimi / 智谱 / DeepSeek 平台遗漏自检执行分支 | 已接入现有兼容网关协议路径，模拟协议及副作用测试通过，未部署 |
| 2 | 高，当前优先 | 自检账号资格与正常调度不一致 | 已在本地实现资格修复与列表／详情快照指标对齐，未部署 |
| 3 | 中 | 严格的平台相等判断排除合法混合调度账号 | 源码确认，未修复 |
| 4 | 中 | 固定 10 分钟有效期与可配置自检间隔冲突 | 源码确认，当前快照为 300 秒，长间隔触发条件未满足 |

## 1. 国产平台遗漏自检执行分支

正常 OpenAI 兼容文本入口允许 `kimi`、`zhipu`、`deepseek`，但 `gatewayModelSelfCheckProbeExecutor.Probe` 的平台分支未包含这三项。普通非 Bedrock 账号落入默认分支，直接得到 `failed/config_error`，没有真正探测上游。

- 触发条件：账号的 `platform` 为上述平台，对应分组模型启用自检且任务通过前置筛选。
- 影响：真实请求可用，但自检及用户侧状态误报失败。通过 `openai` 平台账号转发同名模型不等同于此条件。
- 依据：`backend/internal/service/model_self_check_probe.go` 的 `Probe`；`backend/internal/handler/openai_gateway_handler.go` 的 `openAICompatibleTextTargetAllowed`；`backend/internal/service/domain_constants.go` 的 `IsCNProvider`。
- 修复方向：接入已存在的平台和模型协议选择路径，覆盖三个平台的成功、真实失败及协议限制测试，避免另建协议猜测逻辑。

## 2. 自检账号资格与正常调度不一致

### 分析基线的代码事实

正常 `Account.IsSchedulable()` 检查 active、schedulable、到期自动暂停、过载、全局限流、临时不可调度，以及 API Key / Bedrock 配额耗尽。模型级判断另有 `IsSchedulableForModelWithContext`，含模型冷却及 Antigravity overages 等既有例外。

自检仓储 `ListTargetAccounts` 只预筛 active、schedulable、未删除、临时不可调度；`isAccountEligibleForSelfCheck` 的二次判断仍只有 active、schedulable、临时不可调度，没有补齐账号和模型资格条件。

同一宽松判断被用于：

1. `ListProbeTasks` / `accountIDsForTarget`：生成探测任务。
2. `buildStatusView`：计算列表和详情的当前状态。
3. `buildStatusSnapshot`：写入分组模型状态快照。
4. `RunProbe`：重新读取账号后的执行前检查。

依据：

- `backend/internal/service/account.go`：`IsSchedulable`、`IsQuotaExceeded`。
- `backend/internal/service/antigravity_quota_scope.go`：`IsSchedulableForModelWithContext`。
- `backend/internal/service/shadow_routing.go`：`parentHealthyForShadow`，用于保留 Spark 影子账号的母账号凭据健康边界。
- `backend/internal/repository/model_self_check_repo.go`：`ListTargetAccounts`。
- `backend/internal/service/model_self_check_probe.go`：`isAccountEligibleForSelfCheck`、`ListProbeTasks`、`RunProbe`。
- `backend/internal/service/model_self_check_status.go`：`accountCanSelfCheckTarget`、`accountIDsForTarget`、`buildStatusView`、`buildStatusSnapshot`。

### 明确的触发例子

一个分组模型只有一个 API Key 账号：`status=active`、`schedulable=true`，但本站设置的账号配额已耗尽。正常调度排除该账号；自检仍可能将它作为候选，使用 10 分钟内的成功记录显示健康，并继续发起探测。

由此可确认“没有可调度账号却显示健康”的代码路径存在。是否解释用户遇到的某次实际误报，还需要将当时的账号限制状态、模型和状态记录对齐；当前报告没有把源码推论冒充具体线上事件的复现。

### 修复目标

对于 V1 已支持的分组和平台，当前状态及探测任务使用同一套账号／模型资格规则：

- 不可调度账号不能因旧成功记录而使当前状态保持绿色。
- 没有合格账号时，当前状态为 `failed`，快照记录 `no_available_account`。
- 有合格账号但缺少新鲜探测时，为 `unknown`，不虚构成功。
- 有合格账号时，保持既有成功／降级／失败的聚合规则。

该状态是模型服务健康证据，不是对所有用户请求的成功保证；用户余额、Key 权限、实时并发竞争及请求专属路由条件仍可能影响单次请求。不能为追求“与真实请求一致”在状态查询中执行真实选号、占用并发槽、修改冷却或扣费。

### 已批准的实施顺序

#### A. 统一资格判断，修复当前状态与探测任务

1. 将现有自检资格函数调整为能接收 `ctx` 和模型名，优先复用正常调度现有的账号／模型判断，不在自检中复制一套日期、额度和模型冷却算法。
2. 以 `IsSchedulableForModelWithContext` 为基础，保留正常调度已有的平台例外；不能再无条件叠加“模型剩余限流时间大于零就拒绝”，否则可能覆盖 Antigravity overages 放行语义。Spark 影子账号还必须复用 `parentHealthyForShadow`：母账号缺失、类型不符或凭据不可用时排除影子；母账号自身的全局限流、过载或手动 schedulable 开关不应错误连坐影子。读取候选账号时须补齐引用的母账号，即使母账号不在候选集合；执行前也要重新读取所需的母账号状态。其他专用路由边界沿用相应调度合同，不能宣称一个通用函数覆盖所有路径。
3. `accountCanSelfCheckTarget` 与 `RunProbe` 使用同一资格判断；保留原有模型支持、分组权限和渠道限制检查。
4. 仓储 SQL 保持粗筛也可以，服务层必须作最终校验。无需仅为修复此问题扩大 SQL 中的业务逻辑副本；若后续为性能补充 SQL 预筛，需证明不提前排除正常调度允许的账号。
5. 必须有完整账号信息才能作资格判断。生产依赖已配置但批量读取结果为空／缺少目标账号时应保守排除或返回读取错误，不能将“没有账号详情”当成允许所有账号。相关测试替身应提供完整账号。

该步骤同时改变任务、当前状态和新快照的候选集，不改变正常业务请求调度规则，不扩展到问题 1、3、4 的实现。

#### B. 校验执行时状态，区分跳过与探测失败

任务生成与真正执行之间账号状态可能变化。`RunProbe` 已重新读取账号，应在这里复用完整资格判断：

- 账号已失去资格：跳过上游调用，不伪造一次上游失败历史；分组无可用账号的事实交由状态快照记录。
- 账号查询失败：返回可观测的内部读取错误，不把数据库故障写成上游故障。
- 真正调用上游后失败：保持真实探测失败记录。
- 冷却结束或资格恢复：后续调度刷新重新纳入账号。保留现有刷新节奏，不擅自增加高频恢复探测。

这样可以避免“修正了筛选，却把队列中的跳过任务全部记成失败”的第二次数据污染。

#### C. 明确历史与恢复语义

分析基线的列表可用率按“当前候选账号的历史”计算，详情优先读 `(group, model)` 快照。仅修资格函数不能同时解决全部历史统计问题：账号退出候选集时，其历史可能从列表统计中消失；修复前已写入的错误快照也不会自动变准确。

建议将列表历史指标对齐分组模型快照作为紧随其后的独立可审查步骤，复用详情的可用率口径，并通过批量查询／聚合避免逐模型加载 30 天快照。对应原有[时间线方案](./model-status-timeline-evidence-retention.md)的后续阶段，不应把它混称为一个条件判断的修复。

已有历史保留，不自动删除、重算或凭空补写。当前没有足够证据还原历史上每次账号资格变化的准确时点。

对于资格恢复后仍在有效期内的旧成功记录，需要明确有限承诺：基础修复按现有 10 分钟窗口允许复用，状态含义是“最近有效探活 + 当前资格过滤”，不代表恢复后已经重新探测成功；如果要求“每次恢复后必须重新探测成功才能变绿”，需要额外记录／传播资格恢复时间或状态代次，不能仅依据通用 `updated_at` 猜测。这是可独立验收的增强项。

### 回归验收矩阵

实现前补针对性失败用例，修复后运行同一组测试。探测用 stub，不触达真实上游或开发库。

| 场景 | 预期 |
| --- | --- |
| 唯一账号已达总／日／周配额，仍有旧成功探测 | 无探测任务；当前 failed；快照 no_available_account |
| 到期且自动暂停开启 | 排除，不调用上游 |
| 到期但未开启自动暂停 | 与正常调度规则保持一致，不自行扩大禁用条件 |
| 过载、全局限流、临时不可调度尚未结束 | 排除，不让旧成功记录维持健康 |
| 仅模型 A 处于模型级冷却，模型 B 正常 | A 排除，B 保持资格 |
| Antigravity 模型冷却，但 overages 开启且积分可用 | 保持正常调度既有的放行行为 |
| Spark 影子的母账号缺失、类型异常、到期暂停或凭据临时冷却 | 影子不进入任务和可用账号集合 |
| Spark 母账号只有自身全局限流／过载／手动暂停，凭据仍可用 | 不错误连坐合格的影子账号 |
| 两个账号中一个失去资格、另一个合格且探测成功 | 无资格账号不参与当前状态，剩余账号正常服务可显示健康 |
| 有合格账号但没有新鲜结果 | unknown，不能显示 operational |
| 任务入队后账号失效 | 执行前跳过；上游调用次数为 0；不新增伪失败探测记录 |
| 账号读取失败／详情缺失 | 不把读取失败转换为成功资格或上游失败 |
| 冷却结束 | 后续刷新恢复任务；真实新探测可更新状态 |
| 没有候选账号的一段时间 | 分组快照连续保留不可用证据，恢复后不抹除历史 |
| 列表历史指标迁移到快照后 | 同模型、同窗口的列表和详情采用相同统计口径 |

后续验证包括相关服务与仓储定向 unit、受影响包检查及必要构建；如果要启动本地应用或对真实上游进行端到端验证，需遵守用户明确要求前后端保持停止的边界，另行安排。

## 3. 合法混合调度账号被排除

正常调度允许开启 `mixed_scheduling` 的 Antigravity 账号进入对应平台的混合调度；模型状态通过 `samePlatform` 要求分组和账号平台严格相等。

- 触发条件：例如 Anthropic 分组中的 Claude 模型依赖开启混合调度的 Antigravity 账号。
- 影响：自检和状态忽略真实可用账号；没有同平台账号时，误报 `failed/no_available_account`。
- 依据：`gateway_scheduling.go` 的 `isAccountAllowedForPlatform`；`model_self_check_status.go` 的 `accountIDsForTarget` / `samePlatform`；`gateway_multiplatform_test.go` 中仅有混合 Antigravity 账号仍能调度的用例。
- 修复方向：对齐现有平台准入规则，不无条件放开所有跨平台账号。与问题 2 的账号／模型资格判断分别验收。

## 4. 自检间隔与结果有效期冲突

自检配置允许最长 86400 秒，但 `modelSelfCheckFreshWindow` 固定为 10 分钟。`filterFreshSelfCheckLatest` 会丢弃窗口之外的成功记录；`aggregateSnapshotAvailability` 将 unknown 计入分母而不计入可用数。

- 触发条件：例如将探测间隔设为 30 分钟；当前本地线上快照的 300 秒通常不触发该配置冲突。
- 影响：即使每次实际探测成功，两次探测之间仍可能长时间为 unknown，并压低详情可用率。
- 依据：`setting_service_devzz.go` 的间隔边界；`model_self_check_runner.go` 的实际调度间隔和抖动；`model_self_check_status.go` 的有效期及快照可用率聚合。
- 修复方向：明确有效期与调度间隔、抖动及排队延迟的关系，验证长间隔、临界间隔、runner 停止后的真实过期和 unknown 统计语义。

## 已有验证及未验证项

前一轮源码分析中，下列定向 unit 测试通过；它们未覆盖本文全部触发条件，不能作为四项问题已修复的证据：

```bash
mise x -C backend -- go test -tags=unit ./internal/service \
  -run '^(TestListUserModelStatus|TestGetUserModelStatus|TestRefreshStatusSnapshots|TestListProbeTasks)' -count=1

mise x -C backend -- go test -tags=unit ./internal/service \
  -run 'Test(ModelSelfCheck|GatewayModelSelfCheckProbeExecutor|ListProbeTasks|RunProbe)' -count=1
```

以上是修复前验证记录。尚未对业务历史查询的耗时和内存进行测量，不将相关规模风险列为已复现性能故障。

## 2026-09-07 本地实施结果

- `isAccountEligibleForSelfCheck` 复用 `IsSchedulableForModelWithContext` 和 `parentHealthyForShadow`，供状态候选筛选与执行前检查共同使用。批量加载额外的母账号凭据依赖，但不把母账号自动加入可用候选集；账号详情缺失时不再放行。
- `RunProbe` 对失去资格、账号不存在或不再支持模型的任务跳过调用及历史写入；读取错误向 runner 返回内部错误；真实探测失败仍保留。
- 列表、详情的可用率、降级比例和平均延迟共用 `ListStatusSnapshotMetrics` 的一次 SQL 批量聚合。按精确 `(group_id, model)` 匹配，窗口包含下界且排除未来记录；有历史但窗口内无样本时返回空指标，不改用账号历史。
- 只有确认该目标在查询时点及之前没有任何快照时才回退旧账号级指标；快照指标查询失败时指标保持未知，不用旧成功记录制造可用率。已由快照覆盖的模型不再加载 30 天账号原始历史。
- 列表与详情的当前状态、最近延迟、最后探测时间均来自当前合格账号的新鲜探测；旧快照不会覆盖详情的当前元数据。历史时间线仍独立保留快照及必要的早期历史补齐。
- 没有修改历史数据、迁移、配置默认值、前端界面或正常请求的调度逻辑。问题 1、3、4 和“恢复后必须重新成功探测”的增强项不在本次实现范围。

验证包括账号资格／父账号／执行前跳过和历史口径回归测试、服务／仓储／用户 DTO 定向 unit，以及使用 PostgreSQL 只读事务对实际仓储 SQL 运行合成 CTE 数据：覆盖 7 个目标、同名跨分组、所有窗口、精确边界、未来数据排除、空延迟、整数平均、仅旧快照和无快照场景；该检查没有读写业务表。

本次最终主验证通过：

```bash
mise x -C backend -- go test -tags=unit ./internal/service ./internal/repository ./internal/handler ./internal/handler/admin -count=1
mise x -C backend -- go vet ./internal/service ./internal/repository ./internal/handler ./internal/handler/admin
mise x -C backend -- go build -o /tmp/sub2api-model-status-check ./cmd/server
pnpm --dir docs-site run docs:build
git diff --check
```

补充：并行审查中的另一次 service 全包运行曾在未修改的 `TestRecordCyberPolicyEvent_RuntimeSnapshotRefreshFailureKeepsStaleScope` 失败；最终主验证全包通过，该用例隔离连续运行 10 次也通过。未据此修改内容审核模块或认定其失败根因。文档构建的包体积提示不影响构建成功。

没有由本次操作发起真实上游探测、应用启动／停止、线上操作、提交、推送或部署。末次检查发现本地 3000/8080 已有运行进程，保持原状；当前进程是否已加载本次代码不属于上述测试／构建证据。

## 相关文档

- [定价驱动的站点自检模型监控](./pricing-driven-self-check-monitoring-design.md)
- [模型状态时间线与无可用账号证据保留](./model-status-timeline-evidence-retention.md)
- [模型自检 Token 消耗统计](./self-check-token-usage-stats.md)

## 后续增强：优先级探测链路

问题 2 的资格修复完成后，用户进一步批准按 `(group, model)` 串行探测、首个成功停止，并为管理员提供实际轮次详情。新方案改变当前状态来源为完整轮次证据，取代上文第一阶段保留的账号结果聚合；账号级历史仍供历史兼容和 Token 用量使用。问题 1 同期接入现有国产平台协议路径，Chat / Responses / Messages 和 429 回归通过。完整规则、迁移和本次验证见[模型状态优先级探测链路](./model-status-priority-probe-chain.md)。
