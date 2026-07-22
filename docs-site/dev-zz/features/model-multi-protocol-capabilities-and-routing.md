# 模型级多协议能力与交付路由

> 状态：第一阶段已实现并完成本地回归，当前工作区尚未提交；运行时原生路由默认关闭，可由管理员在“系统设置 → 网关 → 请求转发行为”中启用。
> 适用范围：解决“一个上游账号、一个模型、多个文本协议”的能力表达、公共端点发布和运行时路由；现有转换器继续复用，不重写协议转换实现。

## 结论

sub2api 应把“账号怎么接入”和“模型支持什么协议”拆成两个概念：

- `Account.Platform` / `Group.Platform` 继续是单值，表示账号接入适配器和分组主调度域。
- 新增“账号 + 最终上游模型 + 协议”的原生能力记录。
- 每个“公开模型 + 分组 + 账号 + 入站协议”只通过一个 `DeliveryDecision` 判定是否可交付、实际使用哪个上游协议，以及属于原生还是兼容交付。
- 用户模型目录把“渠道发布的公开模型”“稳定可交付路由”“用户可调用的 API 端点”和“上游原生能力”分开；只有存在稳定可交付路由时才发布端点，经过确认的原生能力负责选择原生路由，兼容路径继续表达已有公共合同。
- 渠道定价只发布商品，不再承担协议配置；渠道页展示的是只读交付结果和阻断原因。
- 同一个账号继续共享一份密钥、并发、健康、供应商、资金池、余额和综合折扣。

第一阶段完整交付后的目标效果：

~~~
上游账号：new-api-A
上游模型：MiniMax-M3

确认的原生协议：
- anthropic_messages       -> /v1/messages
- openai_chat_completions  -> /v1/chat/completions
~~~

管理员只创建一个账号。用户模型目录只展示一个 `MiniMax-M3`，下面列出两个公共端点。

## 当前实现快照

- 迁移 `197_account_model_protocol_capabilities.sql` 落地能力事实、观察结果与管理员覆盖。
- 管理端账号操作新增“模型与协议能力”，支持上游同步、警告查看、无法同步时手工添加精确模型，以及精确模型 / `*` 覆盖。
- new-api `supported_endpoint_types` 已按本文合同解析；字段缺失、空数组或未知枚举只生成/保留 `unknown` 证据，不覆盖既有支持/不支持观察；未知值和异常模型 ID 的管理员警告经过长度与控制字符限制。
- 多 API 端点路由开关默认继承 `gateway.native_model_protocol_routing_enabled`；管理员明确修改并保存后，数据库值成为显式覆盖且无需重启。关闭时只保留原有兼容合同；开启后，确认可交付的 Chat / Responses 端点参与目录发布和调度，OpenAI APIKey 的 `/v1/messages` 可选择确认支持 Messages 的原生路由。
- 原生路径按“显式渠道映射，或 Messages 分组调度回落 → 账号映射 → 平台规范化”后的最终上游模型查能力，并复用代理、Header 覆盖、并发占用、调度失败切换、usage 计费与使用记录。
- 原生 404/405/501 或明确的 endpoint-not-supported 响应只退出本次原生协议尝试，不降低账号整体健康；切换仍受统一账号切换预算约束，原生层耗尽后可回到兼容路径。
- `/v1/models` 使用分组、账号和能力的批量加载结果，只在开关开启且当前可见分组可形成原生路由时返回端点扩展字段。
- `/api/v1/channels/available` 只发布统一交付判定确认可调用的公共端点；OpenAI 分组继续服从 `AllowMessagesDispatch`。能力元数据读取失败时保留能够证明的旧 Messages 兼容合同，不中断原模型/渠道目录。
- 上述默认 Messages 合同必须建立在“当前渠道模型至少存在一个稳定可交付账号路由”之上；渠道定价本身只表示商品发布，不能单独证明模型可用。

## 为什么要这样设计

### 当前问题

sub2api 已经注册：

- `POST /v1/messages`
- `POST /v1/chat/completions`
- `POST /v1/responses`

也已经存在 Messages、Chat Completions、Responses 之间的部分转换。但当前能力表达仍以单值 `platform` 为主：

- `Account.Platform` 同时影响凭据、上游适配器和调度。
- `Group.Platform` 决定请求进入哪个主要网关服务。
- OpenAI Responses 能力使用专用的账号级标记。
- `/v1/models` 不返回模型端点能力。
- `/api/v1/channels/available` 的模型只有名称、平台和定价。

因此，一个 new-api 上游即使明确声明 `MiniMax-M3` 同时接受 Anthropic Messages 和 OpenAI Chat，sub2api 也无法准确表达、原生调度和展示这件事。

### 不采用的方案

| 方案 | 不采用的原因 |
| --- | --- |
| 把 `platform` 改成数组 | `platform` 还决定凭据、调度器、计费和错误处理；它不是协议集合。 |
| 同一密钥复制成两个账号 | 会重复并发、健康、冷却、供应商归属和密钥轮换，成本与余额也容易重复理解。 |
| 只配置账号级协议开关 | 同一上游的不同模型可能支持不同协议，账号级布尔值粒度不足。 |
| 根据模型名或 base URL 猜能力 | 厂商、二开版本和路由配置都会变化，静态白名单不可维护。 |
| 把所有请求统一转换成一种协议 | 工具、流式、thinking、结构化输出等语义并不完全等价。 |
| 第一版同时重构全部转换链路 | 会把核心需求与转换安全、流式合同、分组迁移和自动学习耦合在一起。 |

## 当前代码基础

本设计基于当前实现演进，不从零重写：

| 能力 | 现有位置 | 设计影响 |
| --- | --- | --- |
| 多协议公共入口 | `backend/internal/server/routes/gateway.go` | 公共路由不需要重新命名。 |
| Messages 到 OpenAI 兼容路径 | `backend/internal/service/openai_gateway_messages.go` | 没有可用原生 Messages 路由时继续使用。 |
| Chat / Responses 路径选择 | `backend/internal/service/openai_gateway_chat_completions.go` | 已有能力判定雏形，可以迁移到统一模型。 |
| Responses 手动覆盖与探测结果 | `backend/internal/pkg/openai_compat/upstream_capability.go` | 复用“手动覆盖与观察结果分离”的思想。 |
| 账号模型映射 | `backend/internal/service/account.go` | 能力必须在账号模型映射完成后判断。 |
| 上游模型同步 | `backend/internal/service/upstream_models.go` | 扩展为读取模型端点类型。 |
| OpenAI 能力筛选调度 | `backend/internal/service/openai_account_scheduler.go` | 原生协议能力应作为候选过滤条件，而不是选中账号后再检查。 |
| 端点使用记录 | `usage_logs.inbound_endpoint`、`usage_logs.upstream_endpoint` | 第一阶段不新增重复端点列。 |
| 管理员调度诊断 | `usage_logs.schedule_meta` | 可承载协议路由诊断，不向普通用户暴露。 |
| 用户模型目录 | `backend/internal/handler/available_channel_handler.go` | 增加用户安全的公共端点列表。 |

## 术语

| 概念 | 示例 | 定义 |
| --- | --- | --- |
| 接入平台 | `openai` | 决定账号凭据和主要网关适配器，不代表唯一协议。 |
| 入站协议 | `anthropic_messages` | 客户端发送给 sub2api 的请求/响应格式。 |
| 上游协议 | `openai_chat_completions` | sub2api 实际发送给上游的格式。 |
| 公共端点 | `/v1/messages` | 用户调用 sub2api 的路径。 |
| 上游端点 | `/v1/messages` | sub2api 调用具体账号的路径。 |
| 公开模型 | `MiniMax-M3` | 用户请求、渠道目录和定价使用的模型名。 |
| 上游模型 | 账号映射后的模型名 | 具体账号最终收到的模型 ID。 |
| 稳定可交付路由 | 分组 10 → 账号 7 → `MiniMax-M3` | 忽略瞬时并发/限流，但账号必须 active、schedulable、平台匹配且支持映射后的模型。 |
| 模型交付能力 | `/v1/messages` 可兼容交付 | 公开模型通过至少一条稳定路由完成某公共端点请求的聚合结果。 |
| 原生调用 | Messages → Messages | 不进行跨协议转换。 |
| 旧兼容调用 | Messages → Responses / Chat | 当前已有转换或回落路径。 |

第一阶段协议枚举：

| 内部协议 | 公共默认端点 | new-api 兼容值 |
| --- | --- | --- |
| `anthropic_messages` | `/v1/messages` | `anthropic` |
| `openai_chat_completions` | `/v1/chat/completions` | `openai` |
| `openai_responses` | `/v1/responses` | `openai-response` |

Embedding、媒体、Gemini 原生协议、WebSocket、compact、count_tokens 和 input_tokens 不进入第一阶段。

## 架构契约

配置链路只保留三类输入，不能互相替代：

| 输入 | 管理员负责什么 | 不负责什么 |
| --- | --- | --- |
| 渠道模型与定价 | 决定公开模型是否作为商品发布、如何计费和映射 | 不配置协议，不直接证明可调用 |
| 账号模型协议能力 | 记录最终上游模型原生支持哪些协议；上游声明优先自动同步，管理员只处理例外 | 不决定商品是否公开，不决定分组授权 |
| 全局多端点开关 | 控制新增公共端点是否发布并参与新调度 | 不删除能力证据，不改变渠道价格 |

`openai_responses_mode` 是账号内部 Chat / Responses 传输偏好，不是额外的多端点开关。管理员无需为了“开启多协议”统一改成 `auto`；`force_chat_completions` 和 `force_responses` 仍然有效，只会改变该账号实际选择的上游协议。原生 Messages 能力与这个偏好相互独立。

渠道页的“API 端点就绪度”是只读结果，不是第四处配置。它由统一判定函数计算：

~~~
DeliveryDecision =
  公开商品
  ∩ 稳定账号路由
  ∩ 分组协议许可
  ∩ 全局开关
  ∩ 实际上游传输可用
  ∩ 最终上游模型的协议能力
~~~

每个候选判定固定输出：

- `eligible`：这个公共端点是否可通过该账号稳定交付。
- `inbound_protocol` / `upstream_protocol`：用户调用格式和实际转发格式。
- `mode`：`native` 或 `compatibility`。
- `reason_codes`：不可交付时的稳定、可测试原因，例如无稳定路由、分组禁用、全局开关关闭、能力未知或明确不支持。

控制面与数据面共享同一个候选判定：管理员渠道页按 `eligible=true` 聚合已确认端点，Chat、Responses 和 Messages 运行时也用它筛选账号。模型广场额外区分“模型仍可调用”和“端点已证明”：能力为 `unknown` 时可以保留旧兼容模型卡片，但不发布该未知端点；明确 `unsupported`、模型不匹配或传输不可用属于权威拒绝，不能被旧选择器重新选回。能力存储不可用或全局开关未接管时只允许沿用既有合同，不会反向把未证明端点发布到目录。

## 已确定的设计原则

### 1. 平台保持单值

`Account.Platform` 和 `Group.Platform` 不修改为数组。

平台回答：

~~~
这个账号如何认证、如何加载配置、进入哪套主要调度和计费路径？
~~~

协议能力回答：

~~~
这个账号上的这个上游模型能否原生接受某种请求格式？
~~~

### 2. 能力判断使用最终上游模型

请求中的公开模型可能依次经过：

~~~
渠道模型映射
-> Messages 分组调度映射（仅在没有显式渠道映射时作为协议专属回落）
-> 账号模型映射
-> 平台规范化
-> 最终 upstream_model
~~~

只有得到最终 `upstream_model` 后，才能查询原生协议能力。

### 3. 第一阶段只支持精确模型和账号默认值

`upstream_model` 只允许：

- 精确上游模型 ID。
- `*`，表示该账号的默认能力。

第一阶段不支持 `claude-*` 之类的能力通配符。这样没有最长匹配、等长冲突和正则安全问题。

能力解析顺序固定为：

~~~
精确模型的手动覆盖
> * 默认的手动覆盖
> 精确模型的观察结果
> * 默认的观察结果
> 代码定义的账号固有能力
> unknown
~~~

其中 `unknown` 不会阻断继续查找下一层默认值。

### 4. 未知不是不支持

- 上游未返回能力字段：`unknown`。
- 同步失败：保留最后一次有效结果。
- 认证失败、余额不足、限流、超时：`unknown`，不是 `unsupported`。
- 只有可信能力声明或管理员明确设置，才得到确定状态。

未知账号不进入“确认原生”的候选层，也不进入原生端点目录；但仍可按当前旧兼容逻辑运行，避免破坏存量行为。

### 5. 第一阶段不让 runtime 自动修改能力

真实请求出现 404、405 或错误文本时：

- 写管理员诊断日志和指标。
- 可以显示“疑似能力失效”。
- 不直接更新持久化能力状态。

运行时错误可能来自代理、WAF、临时发布、路由错误或上游故障。第一阶段不引入错误阈值、防抖窗口和并发状态翻转。

### 6. 现有分组字段继续是真相源

第一阶段不新增 `group_protocol_policies`。

- OpenAI 分组是否允许 Messages，继续由 `AllowMessagesDispatch` 决定。
- 平台现有入口准入继续沿用当前代码。
- 新能力表只回答账号模型能否原生接收协议。

因此不存在新旧分组配置并存时的真相源冲突。

### 7. 公共目录不承诺流式和高级特性

第一阶段公共 DTO 不返回单个 `streaming: true/false`。

端点存在只表示基础协议入口可调用，不代表该模型所有工具、图片、thinking、JSON Schema 和流式组合都完全一致。

后续若要公开特性，使用明确的能力集合：

~~~json
{
  "features": [
    "streaming",
    "tools",
    "vision"
  ]
}
~~~

而不是继续增加含义模糊的布尔字段。

### 8. 能力事实和路由偏好分开

“上游支持 Responses”是能力事实；“即使用户调用 Chat，也强制转发到 Responses”是路由偏好。两者不能写进同一个状态。

现有 `openai_responses_mode` 在第一阶段继续作为显式路由偏好：

- `force_responses`：继续执行当前强制 Responses 路径。
- `force_chat_completions`：继续执行当前强制 Chat 路径。
- `auto`：继续根据账号探测结果选择 Chat 或 Responses。

这个偏好只决定该账号的 Chat / Responses 实际上游传输，不是协议能力事实，也不是多端点总开关。例如 `force_chat_completions` 下，入站 Responses 可以通过 Responses → Chat 兼容交付；如果同一模型确认支持 Messages，入站 Messages 仍可原生走 `/v1/messages`。能力表不会因为路由偏好而把另一个协议错误标成“不支持”。

## 数据模型

### 表：`account_model_protocol_capabilities`

~~~sql
CREATE TABLE account_model_protocol_capabilities (
    id               BIGSERIAL PRIMARY KEY,
    account_id       BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    upstream_model   VARCHAR(255) NOT NULL,
    protocol         VARCHAR(64) NOT NULL,
    override_state   VARCHAR(16) NOT NULL DEFAULT 'auto',
    observed_state   VARCHAR(16) NOT NULL DEFAULT 'unknown',
    observed_source  VARCHAR(32),
    observed_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT account_model_protocol_capability_unique
        UNIQUE (account_id, upstream_model, protocol),
    CONSTRAINT account_model_protocol_capability_model_check
        CHECK (BTRIM(upstream_model) <> ''),
    CONSTRAINT account_model_protocol_capability_protocol_check
        CHECK (protocol IN (
            'anthropic_messages',
            'openai_chat_completions',
            'openai_responses'
        )),
    CONSTRAINT account_model_protocol_capability_override_check
        CHECK (override_state IN ('auto', 'supported', 'unsupported')),
    CONSTRAINT account_model_protocol_capability_observed_check
        CHECK (observed_state IN ('unknown', 'supported', 'unsupported'))
);
~~~

设计说明：

- `upstream_model` 保留上游原始大小写，查询使用映射后的精确值。
- `*` 是唯一特殊值。
- 管理员意图存入 `override_state`。
- 系统发现结果存入 `observed_state`。
- 同步永远不能覆盖管理员手动选择。
- 表内不保存 API Key、完整响应体或上游错误正文。
- 不保存每模型端点路径；协议默认路径由代码注册表统一管理。

### 有效状态

单条记录：

~~~
override_state=supported   -> supported
override_state=unsupported -> unsupported
override_state=auto        -> observed_state
~~~

账号模型整体解析按前述六级优先级执行。

### 代码级固有能力

部分账号类型已经有稳定的原生协议合同，不要求数据库为每个模型重复写行：

| 账号类型 | 固有能力 |
| --- | --- |
| Anthropic 原生账号 | `anthropic_messages` |
| OpenAI OAuth / Codex 主路径 | `openai_responses` |
| 其它账号 | 只有当前代码能够确定时才注册；任意 OpenAI APIKey 不默认推断。 |

数据库中的手动覆盖仍可关闭固有能力，便于处理特殊代理和二开上游。

## 协议传输注册表

模型能力回答“能不能原生调用”；传输注册表回答“如何调用”。

它是代码，不是数据库配置：

~~~
ProtocolTransport {
  protocol
  default_path
  supported_account_kinds
  build_auth_headers(account)
  build_protocol_headers(account)
  rewrite_model(body, upstream_model)
  forward_streaming
  forward_non_streaming
}
~~~

第一阶段新增的关键传输是：

~~~
OpenAI APIKey 账号
+ anthropic_messages
+ 标准 /v1/messages
+ 共享账号 API Key
~~~

适用前提：

- 账号为 APIKey 类型。
- 使用已经校验过的账号 `base_url`。
- 上游接受该账号配置对应的共享 Bearer/APIKey 身份。
- 模型能力已确认支持 `anthropic_messages`。

Header 合成顺序：

~~~
协议必需 Header
-> 账号认证 Header
-> 账号 header_overrides
-> 安全层删除禁止覆盖的 Host / Content-Length 等字段
~~~

原生 Messages 路径必须复用现有：

- base URL 校验。
- 代理和 TLS/HTTP 客户端。
- 请求体大小限制。
- 模型映射。
- 并发占用和释放。
- 错误分类、失败重试和模型级冷却。
- usage 解析、计费和使用记录。

它不能进入 Responses/Codex 专用请求变换，也不能因为复用 OpenAI 账号而改变 Anthropic 请求语义。

非标准上游路径和自定义认证方式不进入第一阶段。确有需求时，应增加账号级传输配置，而不是给每个模型重复保存路径。

## 能力发现

### 上游模型描述对象

当前 `FetchUpstreamSupportedModels` 只返回 `[]string`。建议新增内部描述对象：

~~~go
type UpstreamModelDescriptor struct {
    ID                     string
    SupportedEndpointTypes []string
    EndpointTypesPresent   bool
}
~~~

兼容策略：

- 新增 `FetchUpstreamModelCatalog` 返回描述对象。
- 现有 `FetchUpstreamSupportedModels` 继续投影为 `[]string`。
- 现有调用方和管理员模型同步接口不被强制一次性重写。

### new-api 能力字段

识别：

~~~json
{
  "id": "MiniMax-M3",
  "supported_endpoint_types": [
    "anthropic",
    "openai"
  ]
}
~~~

映射：

~~~
anthropic       -> anthropic_messages
openai          -> openai_chat_completions
openai-response -> openai_responses
~~~

同步规则：

1. 字段缺失：只同步模型 ID，不更新协议状态。
2. 字段为空数组：按未声明处理，协议状态保持 `unknown`，不把空数组解释成“全部不支持”。
3. 字段存在、至少包含一个值且所有值都能识别：列出的协议写 `supported`；该合同内未列出的协议写 `unsupported`。
4. 字段存在但包含未知值：已识别值可写 `supported`，不根据“未列出”推断 `unsupported`，并返回管理员警告。
5. 同一个模型出现多次：先合并已声明协议；只有所有重复条目都带非空且合法的能力字段时，未列出的协议才可写 `unsupported`。
6. 本次列表中消失的模型：不自动删除、不自动写 `unsupported`。
7. 所有观察更新在账号级事务内完成。
8. `override_state` 永远保持不变。
9. 同步 upsert 只更新 observed 列；手动覆盖接口只更新 override 列，两条写路径禁止全行覆盖。
10. 同一账号的并发同步通过账号行锁或事务级 advisory lock 串行化。

第一阶段不做定时主动探测。管理员可以使用上游声明或手动覆盖；主动协议探测属于后续增强。

## 路由设计

### 入口协议解析

公共路径映射为内部协议：

~~~
/v1/messages          -> anthropic_messages
/v1/chat/completions  -> openai_chat_completions
/v1/responses         -> openai_responses
~~~

映射由代码常量维护，不从用户输入直接接受任意协议字符串。

### 交付候选必须在调度前过滤

不能先选出账号，再在转发阶段才判断协议。否则目录看到的交付结果、调度器选择的账号和实际转发协议可能互相矛盾。

当前流程：

~~~
1. 校验分组是否允许当前入站协议
2. 用公开模型执行渠道发布与定价限制检查
3. 用渠道映射后的模型执行账号模型资格筛选
4. 对候选应用账号映射并得到最终 upstream_model
5. 解析账号实际使用的上游协议
6. 查询该最终模型、该实际上游协议的有效能力
7. 生成 DeliveryDecision，排除 eligible=false 的候选
8. eligible 候选继续使用现有 priority / cost / load / sticky 规则
9. 执行决定中的原生或兼容转发，并把同一决定写入 usage 调度元数据；旧流量兜底只允许覆盖 `unknown`、能力存储不可用或全局开关未接管，且仍受原有重试和排除集合约束
~~~

公开模型和渠道映射模型必须作为两个参数传递。前者用于商品/定价合同，后者用于账号模型映射和能力解析，禁止复用一个字符串同时承担两种语义。

### 原生与兼容的选择

- Messages：确认原生 Messages 时走 Messages → Messages；否则按照该账号当前 Chat / Responses 传输策略判断现有 Messages 兼容桥是否可用。
- Chat：账号实际传输为 Chat 时是原生；实际传输为 Responses 时是兼容。
- Responses：账号实际传输为 Responses 时是原生；实际传输为 Chat 时是兼容。
- 能力判断始终针对“实际上游协议”，不是机械要求入站协议也必须被上游原生支持。
- 同一候选只生成一个确定的上游传输方案；多条账号路由聚合后才可能显示 `mixed`。
- 粘性账号不满足交付判定时不能绕过过滤，调度器应继续选择其它合格候选。
- 原生端点明确不支持时可切换其它候选或兼容路径，但必须遵守统一账号切换预算。

第一阶段不增加“成本优先于原生”的策略。协议语义正确性优先于小幅成本差异。

### 明确不改变的旧兼容路径

以下逻辑第一阶段保留：

- Messages 转 Responses。
- APIKey Responses 不可用时回落 Chat Completions。
- Chat Completions 转 Responses。
- OpenAI 请求转 Anthropic 的现有兼容服务。

它们继续维持存量行为，但不自动被写成“上游原生支持”，也不进入第一阶段公共原生端点目录。

### 确定性上游错误

原生转发得到明确 404、405、501 或 endpoint not supported 时：

1. 本次请求把该账号加入排除集合。
2. 继续尝试其它原生账号。
3. 原生层耗尽后按旧兼容规则决定是否兜底。
4. 记录管理员诊断。
5. 不自动更新能力表。

本地能力不匹配或协议传输不适用，不得标记账号不健康。

## API 设计

### 管理员能力接口

使用账号子资源，避免把表格能力塞进通用账号更新：

~~~
GET  /api/v1/admin/accounts/:id/model-protocol-capabilities
PUT  /api/v1/admin/accounts/:id/model-protocol-capabilities/overrides
POST /api/v1/admin/accounts/:id/model-protocol-capabilities/sync
~~~

读取示例：

~~~json
{
  "account_id": 123,
  "items": [
    {
      "upstream_model": "MiniMax-M3",
      "protocol": "anthropic_messages",
      "override_state": "auto",
      "observed_state": "supported",
      "effective_state": "supported",
      "observed_source": "upstream_model_list",
      "observed_at": "2026-07-21T12:00:00+08:00"
    }
  ],
  "warnings": []
}
~~~

覆盖请求：

~~~json
{
  "items": [
    {
      "upstream_model": "MiniMax-M3",
      "protocol": "anthropic_messages",
      "state": "supported"
    }
  ]
}
~~~

`state=auto` 表示删除手动意图，重新跟随观察结果。

客户端不能写：

- `observed_state`
- `observed_source`
- `observed_at`
- `effective_state`

### `/v1/models`

模型条目增加 new-api 兼容扩展：

~~~json
{
  "id": "MiniMax-M3",
  "object": "model",
  "supported_endpoint_types": [
    "anthropic",
    "openai"
  ]
}
~~~

规则：

- 只聚合当前 API Key 可访问分组中的确认原生能力。
- `/v1/models` 只聚合当前可形成的同协议原生路由，不把兼容转换伪装成上游原生能力；账号 Chat / Responses 传输偏好会影响对应协议是否属于原生，但不会错误移除独立的原生 Messages 能力。
- 没有确认能力时省略字段，不返回误导性的空数组。
- 不返回账号、供应商、上游地址、成本和交付拓扑。
- 老客户端可忽略未知字段。

### `/api/v1/channels/available`

在现有模型 DTO 增加：

~~~json
{
  "name": "MiniMax-M3",
  "platform": "openai",
  "route_group_ids": [10, 11],
  "supported_endpoints": [
    {
      "protocol": "anthropic_messages",
      "path": "/v1/messages",
      "group_ids": [10]
    },
    {
      "protocol": "openai_chat_completions",
      "path": "/v1/chat/completions",
      "group_ids": [10, 11]
    }
  ],
  "pricing": {}
}
~~~

`route_group_ids` 是当前用户可见范围内仍有可调用路由的分组；`supported_endpoints[].group_ids` 是其中已确认可发布该端点的分组。两者都只使用接口本来已经公开的可见分组 ID，解决同一渠道平台区段内不同分组协议权限不一致的问题。模型可能因 `unknown` 证据仍保留在 `route_group_ids`，但不会因此获得一个未经证明的 `supported_endpoints` 条目。

目录字段表达的是“用户可以调用的 API 端点”，不是纯粹的上游原生能力。计算必须先证明公开模型存在稳定可交付路由：

- 渠道映射或定价决定公开模型是否发布，但不能单独证明模型可交付。
- 原有 `/v1/messages` 不要求管理员先写 Messages 原生能力记录，但必须至少存在一个能够进入兼容路径或原生路径的稳定账号路由。
- OpenAI 分组只有在 `AllowMessagesDispatch=true` 时进入 `/v1/messages` 的 `group_ids`。
- Chat / Responses 公共端点按账号当前实际传输方案判定：同协议为原生，跨协议为兼容；两者都必须有实际上游协议的明确支持证据。
- `unknown` 不会删除仍可由存量 Messages 兼容合同证明的端点；如果当前账号由 Chat 承接兼容路径，则 Chat 明确 `unsupported` 会退出该路径；由 Responses 承接时则检查 Responses 能力。
- 能力查询失败或状态为 `unknown` 时，如果运行时仍允许走既有兼容合同，则保留模型及对应 `route_group_ids`；只保留能够证明的存量 Messages 兼容端点，并省略无法确认的新增端点。
- 没有任何稳定账号路由的公开模型不应在“可用模型”目录中伪装成可调用；管理员目录保留该商品配置并标记“无可用路由”。

公共模型仍只展示一次；协议不是模型身份的一部分。

### 目录计算性能

目录服务必须批量计算，禁止按“渠道 × 模型 × 账号”产生 N+1 查询：

1. 批量加载当前可见分组。
2. 批量加载分组关联的有效账号摘要。
3. 批量加载这些账号的协议能力。
4. 在内存中执行模型映射和能力聚合。

可使用短 TTL 缓存，并在以下事件后失效：

- 账号能力覆盖变化。
- 上游模型能力同步完成。
- 账号/分组关联变化。
- 账号状态变化。
- 渠道模型映射变化。

瞬时限流和并发占满不触发目录缓存失效，避免端点列表抖动。

## 管理端体验

### 渠道模型入口（主要入口）

渠道定价是公开模型的管理起点。每个模型行展示：

- 交付状态：可交付、部分可交付、已有稳定路由但无用户可调用端点、无可用路由。
- 可调用 API 端点摘要。
- 关联分组数量和稳定候选账号数量。
- “查看路由”入口，按“分组 → 账号 → 最终上游模型”展示交付方式与能力证据。

渠道模型页只编辑商品和价格，不保存协议能力副本。管理员从路由详情跳转到对应账号的上游能力覆盖，避免渠道和账号产生两份互相漂移的真相。

### 账号入口（高级入口）

账号列表增加独立操作：

~~~
上游模型协议能力
~~~

第一阶段该操作只在 OpenAI API Key 账号行直接展示；“供应商”视图只管理成本与余额归属，并引导管理员返回账号列表配置协议能力。

说明文案：

~~~
平台决定账号如何认证和调度；这里记录具体上游模型可原生接收的协议。用户可调用的 API 端点由渠道模型的实际交付路由聚合得出。
~~~

不要把完整模型 × 协议矩阵继续塞进已经很长的账号创建/编辑表单：

- 创建账号流程保持现状；账号保存成功后才能执行上游同步。
- 账号编辑表单只显示能力摘要和“管理能力”入口。
- 完整配置使用独立 Drawer 或页面，便于搜索、同步和批量查看模型。

页面分为：

- 账号默认能力（`*`）。
- 模型能力列表。
- 上游同步结果。

每一行展示：

- 上游模型。
- Anthropic Messages / OpenAI Chat / OpenAI Responses。
- 有效状态：支持、不支持、未知。
- 控制方式：自动、强制支持、强制不支持。
- 证据来源。
- 最近观察时间。
- 影响的渠道公开模型和分组。
- 未被任何渠道模型使用时的“孤儿能力”警告。

只允许管理员编辑覆盖状态；观察结果只读。

### 模型同步

同步结果示例：

~~~
MiniMax-M3    Anthropic Messages    OpenAI Chat
~~~

交互要求：

- “同步”只更新观察结果。
- 不把同步结果隐式变成手动配置。
- 未知枚举显示警告，但不阻止模型 ID 同步。
- 字段缺失时明确显示“上游未声明协议能力”，而不是“不支持”。

## 用户侧体验

一个模型只显示一次：

~~~
MiniMax-M3

API 端点
Anthropic Messages    /v1/messages
OpenAI Chat           /v1/chat/completions
~~~

用户侧不展示：

- 上游账号名。
- 上游供应商。
- 原生账号数量。
- 上游 base URL。
- 原生/旧兼容故障转移细节。
- 综合折扣和真实成本。

页面状态：

- 存在稳定兼容路由但没有额外原生证据：展示原有 `/v1/messages`，不把“未声明原生能力”误写成“没有可用端点”。
- 渠道发布了模型但没有稳定账号路由：普通用户目录不显示为可用；管理员渠道模型页显示“无可用路由”。
- 有确认能力：在默认端点后追加 `/v1/chat/completions`、`/v1/responses` 等端点。
- 卡片标题使用“API 端点”，按钮直接显示并复制真实路径；原生或兼容转发属于内部路由细节。
- 当前账号短时不健康：端点仍保留，由模型状态模块展示健康异常。
- 分组禁止某协议：对应 `group_id` 不进入该端点。

## 使用记录与可观测性

现有字段继续记录：

- `inbound_endpoint`
- `upstream_endpoint`
- `upstream_model`

第一阶段扩展管理员专用 `schedule_meta`：

~~~json
{
  "inbound_protocol": "anthropic_messages",
  "upstream_protocol": "anthropic_messages",
  "protocol_delivery_mode": "native",
  "capability_source": "upstream_model_list"
}
~~~

不新增重复数据库列。只有后续需要高频按协议聚合时，再评估规范化列和索引。

管理员至少能判断：

~~~
客户端：Anthropic Messages
上游：Anthropic Messages
交付：原生
证据：上游模型列表
~~~

需要的指标：

- 各入站协议请求量和成功率。
- 原生路径命中率。
- 无原生候选进入旧兼容路径的次数。
- 原生端点明确不支持的诊断次数。
- 能力同步成功、缺字段和未知枚举次数。

## 定价、供应商和余额

协议能力不创建新账号、供应商或资金池。

同一个账号的多个协议继续共享：

- 并发上限。
- 健康和模型级冷却。
- 供应商归属。
- 资金池和上游余额。
- 充值比例、汇率和综合折扣。
- 模型成本倍率。

第一阶段同一个模型在不同协议下使用同一渠道售价。只有未来存在可审计的协议级上游成本证据时，才考虑协议级定价。

## 权限与安全

- 只有管理员可查看和修改账号能力。
- 普通用户只看到公共端点和自己可见的分组 ID。
- 能力同步复用账号现有 base URL、认证、代理和 SSRF 校验。
- 不允许能力接口传入任意探测 URL。
- 上游未知枚举只保存脱敏值或管理员警告，不保存完整响应。
- 原生传输继续通过现有请求体限制、Header 安全和代理策略。
- 管理员覆盖变更进入现有审计体系。

## 迁移与兼容

### 能力迁移

存量字段映射：

| 存量证据 | 新模型 |
| --- | --- |
| `openai_responses_mode` | 不迁入能力表；继续作为独立的显式路由偏好。 |
| `openai_responses_supported=true/false` | `* + openai_responses + observed=supported/unsupported` |
| 明确配置的 `openai_capabilities` | 对应 `* + protocol + observed=legacy_migration` |
| 缺少明确证据 | 不写能力记录，保持 unknown |

迁移不能根据 `platform=openai` 推断三个协议都支持。

### 功能开关

增加全局开关：

~~~
native_model_protocol_routing_enabled
~~~

默认关闭。

管理员入口位于“系统设置 → 网关 → 请求转发行为 → 模型多 API 端点路由”。有效值按以下优先级解析：

1. 管理员后台已保存的数据库设置；
2. 数据库设置不存在时，继承 `gateway.native_model_protocol_routing_enabled` 配置文件默认值；
3. 两者都未配置时为 `false`。

保存后当前实例立即生效，多实例部署通过共享设置表在最多约 5 秒内收敛。账号“模型与协议能力”弹窗会同时显示全局路由状态；关闭开关只停止新增端点发布和新交付选择，不删除同步结果或管理员覆盖，存量兼容路由继续运行。

启用顺序：

1. 表、管理员能力页面和手动覆盖上线。
2. 上游模型同步写观察结果。
3. shadow 计算交付判定，只记录不改变路由。
4. 开启原生路由。
5. 最后开放公共模型目录字段。

回滚只需关闭开关；旧兼容路径继续运行，能力数据保留。

## 实施范围

### 里程碑 1：能力数据和发现

- 新增能力表、实体、仓储和迁移测试。
- 新增协议枚举和能力解析器。
- 新增上游模型描述对象。
- 解析 new-api `supported_endpoint_types`。
- 迁移 Responses 现有状态。
- 管理员能力接口、自动/手动覆盖和 UI。

完成标准：管理员能准确看到同一账号每个模型的原生协议，未知不会被误判。

### 里程碑 2：原生协议路由

- 调度请求增加统一交付判定筛选。
- OpenAI APIKey 账号增加原生 Messages 传输。
- Chat / Responses 按账号实际传输策略形成原生或兼容决定；存量选择器保留故障兜底。
- 接入并发、重试、错误分类、usage 和计费。
- `schedule_meta` 增加协议路由诊断。

完成标准：一个 new-api 账号的 `MiniMax-M3` 能从 Messages、Chat 和 Responses 公共入口按配置交付，且目录、管理员诊断和运行时使用同一判定。

### 里程碑 3：模型目录和用户展示

- `/v1/models` 增加 `supported_endpoint_types`。
- `/channels/available` 增加 `supported_endpoints`。
- 模型详情展示多个端点。
- 批量聚合与缓存失效。
- 普通用户隐私测试。

完成标准：页面只展示实际可调用的 API 端点；管理员能看到原生/兼容方式、实际上游协议和不可交付原因。

### 后续里程碑：转换能力合同

不属于本需求第一阶段。另行设计：

- 请求特征预检。
- 流式和非流式分别声明。
- 工具、图片、thinking 和结构化输出合同。
- 原生与安全转换的统一 RoutePlan。
- 转换可达能力进入公共模型目录。

### 后续里程碑：分组协议策略统一

等当前能力稳定后，再评估用统一协议策略替代 `AllowMessagesDispatch` 等布尔字段。迁移前现有字段始终是唯一真相源。

## 验收标准

### 核心场景

- 只创建一个 new-api 上游账号。
- 同一账号的 `MiniMax-M3` 可确认支持 Messages、Chat 和 Responses。
- 三个公共入口都按账号当前传输偏好形成可解释的原生或兼容交付决定。
- 两个入口共享同一账号并发、健康、供应商、余额池和综合折扣。
- 模型映射后按最终上游模型查能力。
- 交付判定不合格的高优先级账号不会遮住合格账号。
- 能力存储不可用或没有已证明的新路由时，当前旧兼容路径不受破坏。
- 上游模型列表缺少能力字段时保持 unknown。
- 管理员手动禁用优先于同步观察。
- 用户目录保留可证明的默认 Messages 兼容端点，并只追加统一交付判定确认可调用的其它端点。
- 普通用户接口不泄露上游账号和交付拓扑。

### 回归场景

- 未生成能力记录的存量账号行为不变。
- Responses 现有强制模式继续控制 Chat / Responses 的实际上游传输，但不再错误禁用独立的原生 Messages 能力。
- 关闭功能开关即可恢复旧路由。
- 老客户端可以忽略新增模型字段。
- 404/405/501 或 endpoint-not-supported 不会自动永久修改账号能力。
- 临时限流和并发占满不会让模型端点从目录抖动消失。

## 测试矩阵

### 数据与解析

- 表约束、唯一键和级联删除。
- 精确模型、`*` 和 unknown 回落。
- 手动覆盖优先于观察结果。
- new-api 字段存在、缺失、空数组、未知枚举和重复模型。
- 同步事务失败不留下半套状态。
- 并发同步串行化，手动覆盖与同步并发时互不覆盖字段。
- 模型从列表消失时不删除既有能力。
- Responses 存量字段迁移。

### 调度与交付判定

- 公开模型与渠道映射模型分别参与商品限制和账号模型资格检查。
- 每个候选的入站协议、实际上游协议、原生/兼容方式和原因码稳定可预测。
- 合格候选内部保留 priority、cost、load 和 sticky 规则。
- sticky 账号交付判定不匹配时重新选择。
- 最终上游模型能力匹配。
- 同一账号不重复进入候选。
- 原生 404/405/501 和 endpoint-not-supported 失败转移但不写能力状态。
- 新交付候选耗尽或能力存储故障后进入旧兼容路径。
- 功能开关关闭时行为完全回到旧路径。

### 转发与计费

- Messages 非流式原生透传。
- Messages 流式原生透传。
- 模型字段改写为最终上游模型。
- Header 合成和敏感 Header 防护。
- 代理、超时、请求体限制和取消传播。
- usage、停止原因、错误映射和计费。
- 并发槽位成功与失败都能释放。
- 使用记录的入站/上游端点和协议诊断正确。

### API 与前端

- 管理员读取、覆盖、重置自动和同步。
- 客户端不能写观察字段。
- `/v1/models` 按当前 API Key 权限聚合。
- `/channels/available` 的 `group_ids` 正确。
- 批量目录计算无 N+1 查询。
- 同一模型只展示一次。
- 用户 DTO 不包含账号、供应商、成本和交付方式。
- 暗色模式、长模型名、多端点换行和复制操作。

### 端到端

使用一个真实或合同测试 new-api 上游：

1. 同步 `MiniMax-M3` 的 `anthropic` 和 `openai` 能力。
2. 用同一 sub2api API Key 调用 `/v1/messages`。
3. 用同一 sub2api API Key 调用 `/v1/chat/completions`。
4. 确认两次请求命中同一个上游账号。
5. 确认上游分别收到原生 Messages 和原生 Chat。
6. 确认账号并发、usage、供应商成本和余额归属没有重复。
7. 手动禁用 Messages，确认目录和路由同时停止声明原生支持。
8. 关闭功能开关，确认旧兼容路径恢复。

## 本轮评审问题的最终处理

| 评审问题 | 最终决策 |
| --- | --- |
| 通配符等长冲突 | 第一阶段只允许精确模型和 `*`，不接受其它通配符。 |
| runtime 观察防抖和并发写入 | 第一阶段 runtime 完全不写持久化能力，只做诊断。 |
| 单个 `streaming` 布尔不够准确 | 第一阶段公共 DTO 不返回 streaming；未来使用 features 集合。 |
| `AllowMessagesDispatch` 与新表真相源冲突 | 第一阶段不新增分组协议表，现有字段保持唯一真相源。 |

这些问题已经成为正式设计决策，不再属于实施前开放问题。

## 实现时主要文件

预计涉及：

- `backend/migrations/`
- `backend/internal/service/account.go`
- `backend/internal/service/upstream_models.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/pkg/openai_compat/`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/available_channel_handler.go`
- `frontend/src/components/account/EditAccountModal.vue`
- 新增独立的账号模型协议能力 Drawer / 页面组件
- `frontend/src/views/user/AvailableChannelsView.vue`
- `frontend/src/api/channels.ts`
- `frontend/src/api/admin/accounts.ts`

实际实施时先补回归测试，再按里程碑拆分代码，不在一个提交里同时重构通用转换体系。

## 相关文档

- [可用渠道模型广场与报价导出](./available-channels-model-marketplace.md)
- [上游供应商成本感知与模型级调度](./upstream-provider-cost-aware-scheduling.md)
- [定价驱动的站点自检模型监控](./pricing-driven-self-check-monitoring-design.md)
- [接口索引](../reference/api-surface.md)
- [验证矩阵](../testing/verification-matrix.md)

## 外部参考

- new-api 端点类型定义：[`common/endpoint_type.go`](https://github.com/QuantumNous/new-api/blob/4aa08f917eedecf77cef387f2337af88277fbbd0/common/endpoint_type.go#L5-L44)
- new-api 按模型汇总渠道端点能力：[`model/pricing.go`](https://github.com/QuantumNous/new-api/blob/4aa08f917eedecf77cef387f2337af88277fbbd0/model/pricing.go#L272-L316)
- new-api `/v1/models` 输出 `supported_endpoint_types`：[`controller/model.go`](https://github.com/QuantumNous/new-api/blob/4aa08f917eedecf77cef387f2337af88277fbbd0/controller/model.go#L155-L169)
- Aether 格式透传与转换合同：[`format-passthrough-contract.md`](https://github.com/fawney19/Aether/blob/7756c0913f2e0089575d019f1ca90d9867f35c52/docs/api/format-passthrough-contract.md#L7-L55)

外部项目只用于验证领域表达和协议边界；实现必须基于 sub2api 现有代码独立完成，不复制许可证不兼容的代码。
