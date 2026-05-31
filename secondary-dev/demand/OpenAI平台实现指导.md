# OpenAI 平台密钥池实现指导文档

**版本**: 1.0
**日期**: 2026-05-07
**目标读者**: Codex（代码实现 AI）
**参考文档**: `开发路线图.md` v1.1

---

## 一、实现概述

### 1.1 目标

将 OpenAI 平台的 API-key passthrough 转发方法改造为支持密钥池调度的三层架构，参考 Anthropic 平台的成功实现模式。

### 1.2 三层方法结构

```
forwardOpenAIPassthrough()                    // 第一层：主入口
  ↓
forwardOpenAIPassthroughWithInput()          // 第二层：密钥池调度
  ↓
forwardOpenAIPassthroughWithToken()          // 第三层：单密钥转发
```

**职责划分**：
- **第一层**：参数准备、请求体解析、调用第二层
- **第二层**：密钥池迭代、故障转移、冷却标记、使用统计
- **第三层**：单个密钥的实际 HTTP 转发、响应处理

### 1.3 核心改造点

1. 新增 `forwardOpenAIPassthroughWithInput()` 方法（密钥池调度层）
2. 重构 `forwardOpenAIPassthrough()` 为主入口层
3. 提取 `forwardOpenAIPassthroughWithToken()` 为单密钥转发层
4. 实现 OpenAI 特定的错误分类逻辑
5. 集成冷却机制和使用统计

---

## 二、详细任务清单

### 任务 1：代码重构（预计 2-3 小时）

#### 1.1 创建 `forwardOpenAIPassthroughWithInput()` 方法

**文件**: `backend/internal/service/openai_gateway_service.go`

**方法签名**：
```go
func (s *OpenAIGatewayService) forwardOpenAIPassthroughWithInput(
    ctx context.Context,
    c *gin.Context,
    account *Account,
    input openaiPassthroughForwardInput,
) (*OpenAIForwardResult, error)
```

**输入结构体定义**：
```go
type openaiPassthroughForwardInput struct {
    Body           []byte
    OriginalModel  string
    RequestModel   string
    ReasoningEffort *string
    Stream         bool
    StartTime      time.Time
}
```

**实现要点**：

1. **密钥池路径**（当 `account.HasAccountAPIKeyPool()` 为 true）：
   ```go
   now := time.Now()
   keySelections := account.EffectiveAPIKeySelectionsForRequest(
       input.OriginalModel,
       input.RequestModel,
       now,
   )

   if len(keySelections) == 0 {
       return nil, &UpstreamFailoverError{
           StatusCode: http.StatusServiceUnavailable,
           ResponseBody: []byte(`{"error":{"message":"no available upstream key for model","type":"service_unavailable"}}`),
       }
   }
   ```

2. **密钥迭代与故障转移**：
   ```go
   var lastFailoverErr *UpstreamFailoverError

   for i := range keySelections {
       selection := keySelections[i]
       key := selection.Key

       // 准备密钥级输入
       keyInput := input
       keyInput.RequestModel = selection.UpstreamModel

       // 如果模型映射发生变化，替换请求体中的模型名
       if selection.UpstreamModel != input.RequestModel {
           keyInput.Body = s.replaceModelInBody(input.Body, selection.UpstreamModel)
       }

       // 调用单密钥转发
       result, err := s.forwardOpenAIPassthroughWithToken(
           ctx, c, account, keyInput, key.APIKey, &key,
       )

       // 成功：标记使用并返回
       if err == nil {
           s.markAccountAPIKeyUsed(ctx, key.ID, false)
           return result, nil
       }

       // 可重试错误：冷却并继续下一个密钥
       var failoverErr *UpstreamFailoverError
       if errors.As(err, &failoverErr) {
           lastFailoverErr = failoverErr
           s.cooldownAccountAPIKeyModel(
               ctx,
               key.ID,
               keyInput.RequestModel,
               failoverErr.StatusCode,
               failoverErr.ResponseBody,
           )
           s.markAccountAPIKeyUsed(ctx, key.ID, true)
           continue
       }

       // 不可重试错误：直接返回
       return nil, err
   }

   // 所有密钥都失败
   if lastFailoverErr != nil {
       return nil, lastFailoverErr
   }
   return nil, &UpstreamFailoverError{
       StatusCode: http.StatusServiceUnavailable,
       ResponseBody: []byte(`{"error":{"message":"all upstream keys failed","type":"service_unavailable"}}`),
   }
   ```

3. **传统单密钥路径**（向后兼容）：
   ```go
   // Legacy single-key path
   token, tokenType, err := s.GetAccessToken(ctx, account)
   if err != nil {
       return nil, err
   }

   if tokenType != "apikey" {
       return nil, fmt.Errorf("openai api key passthrough requires apikey token, got: %s", tokenType)
   }

   return s.forwardOpenAIPassthroughWithToken(ctx, c, account, input, token, nil)
   ```

#### 1.2 重构 `forwardOpenAIPassthrough()` 为主入口

**当前位置**: `openai_gateway_service.go` 第 2816-3065 行

**改造步骤**：

1. 保留参数解析逻辑（`reqModel`, `reasoningEffort`, `reqStream`）
2. 构建 `openaiPassthroughForwardInput` 结构体
3. 调用 `forwardOpenAIPassthroughWithInput()`
4. 删除原有的直接转发逻辑

**改造后的方法骨架**：
```go
func (s *OpenAIGatewayService) forwardOpenAIPassthrough(
    ctx context.Context,
    c *gin.Context,
    account *Account,
    body []byte,
    reqModel string,
    reasoningEffort *string,
    reqStream bool,
    startTime time.Time,
) (*OpenAIForwardResult, error) {
    // 构建输入
    input := openaiPassthroughForwardInput{
        Body:            body,
        OriginalModel:   reqModel,
        RequestModel:    reqModel,
        ReasoningEffort: reasoningEffort,
        Stream:          reqStream,
        StartTime:       startTime,
    }

    // 调用密钥池调度层
    return s.forwardOpenAIPassthroughWithInput(ctx, c, account, input)
}
```

#### 1.3 提取 `forwardOpenAIPassthroughWithToken()` 方法

**方法签名**：
```go
func (s *OpenAIGatewayService) forwardOpenAIPassthroughWithToken(
    ctx context.Context,
    c *gin.Context,
    account *Account,
    input openaiPassthroughForwardInput,
    apiKey string,
    key *AccountAPIKey, // nil for legacy single-key accounts
) (*OpenAIForwardResult, error)
```

**实现要点**：

1. 从当前 `forwardOpenAIPassthrough()` 的第 2954 行开始提取
2. 将 `token` 参数替换为 `apiKey`
3. 保留所有 HTTP 请求构建逻辑
4. 保留流式/非流式响应处理逻辑
5. **关键**：在错误处理部分调用 `shouldFailoverOpenAIPassthroughResponse()`

**错误分类逻辑**（第 3001-3008 行附近）：
```go
// 检查是否应该故障转移
if shouldFailover, failoverStatusCode := s.shouldFailoverOpenAIPassthroughResponse(
    upstreamResp.StatusCode,
    respBody,
); shouldFailover {
    return nil, &UpstreamFailoverError{
        StatusCode:   failoverStatusCode,
        ResponseBody: respBody,
    }
}
```

### 任务 2：错误分类实现（预计 1 小时）

#### 2.1 实现 `shouldFailoverOpenAIPassthroughResponse()` 方法

**当前状态**: 方法已存在但可能需要增强

**需要支持的错误码**：

| HTTP 状态码 | 错误类型 | 是否故障转移 | 冷却时长（默认） |
|------------|---------|------------|----------------|
| 401 | 认证失败 | 是 | 1 小时 |
| 429 | 速率限制 | 是 | 5 分钟 |
| 5xx | 服务器错误 | 是 | 1 分钟 |
| 400 | 客户端错误 | 否 | - |
| 其他 4xx | 客户端错误 | 否 | - |

**实现参考**：
```go
func (s *OpenAIGatewayService) shouldFailoverOpenAIPassthroughResponse(
    statusCode int,
    body []byte,
) (bool, int) {
    // 401: 认证失败 → 故障转移
    if statusCode == http.StatusUnauthorized {
        return true, statusCode
    }

    // 429: 速率限制 → 故障转移
    if statusCode == http.StatusTooManyRequests {
        return true, statusCode
    }

    // 5xx: 服务器错误 → 故障转移
    if statusCode >= 500 && statusCode < 600 {
        return true, statusCode
    }

    // 其他错误 → 不故障转移
    return false, statusCode
}
```

#### 2.2 流式响应的错误处理

**重要决策**（来自 `开发路线图.md` 决策 2）：
- 流式响应开始后发生的错误**不进行故障转移**
- 直接返回错误给客户端
- 原因：流式响应已经开始发送，无法回滚

**实现位置**: 在流式响应处理的错误捕获部分，确保不返回 `UpstreamFailoverError`

### 任务 3：冷却机制集成（预计 30 分钟）

#### 3.1 使用现有的冷却方法

**方法**: `s.cooldownAccountAPIKeyModel()`

**调用位置**: `forwardOpenAIPassthroughWithInput()` 的故障转移循环中

**参数**：
- `ctx`: 上下文
- `key.ID`: 密钥 ID
- `keyInput.RequestModel`: 请求的模型名
- `failoverErr.StatusCode`: HTTP 状态码
- `failoverErr.ResponseBody`: 响应体（用于日志）

#### 3.2 冷却时长配置

**配置位置**: 账号级配置或全局配置

**配置格式**（JSON）：
```json
{
  "cooldown_durations": {
    "401": 3600,  // 1 小时
    "429": 300,   // 5 分钟
    "5xx": 60     // 1 分钟
  }
}
```

**实现说明**:
- 冷却逻辑已在 `account_api_key_pool.go` 中实现
- 只需确保正确调用 `cooldownAccountAPIKeyModel()` 方法
- 冷却时长由配置决定，代码无需硬编码

### 任务 4：使用统计集成（预计 30 分钟）

#### 4.1 使用现有的统计方法

**方法**: `s.markAccountAPIKeyUsed()`

**调用位置**:
1. 成功转发后：`s.markAccountAPIKeyUsed(ctx, key.ID, false)`
2. 故障转移后：`s.markAccountAPIKeyUsed(ctx, key.ID, true)`

**参数**：
- `ctx`: 上下文
- `key.ID`: 密钥 ID
- `isError`: 是否为错误请求（true = 失败，false = 成功）

**用途**：
- 更新 `last_used_at` 时间戳（用于 LRU 排序）
- 统计成功/失败次数（用于成本分析）

### 任务 5：模型映射支持（预计 1 小时）

#### 5.1 实现 `replaceModelInBody()` 方法

**用途**: 当密钥的上游模型映射与请求模型不同时，替换请求体中的模型名

**方法签名**：
```go
func (s *OpenAIGatewayService) replaceModelInBody(
    body []byte,
    newModel string,
) []byte
```

**实现步骤**：

1. 解析 JSON 请求体
2. 替换 `model` 字段
3. 重新序列化为 JSON

**实现示例**：
```go
func (s *OpenAIGatewayService) replaceModelInBody(body []byte, newModel string) []byte {
    var reqMap map[string]interface{}
    if err := json.Unmarshal(body, &reqMap); err != nil {
        // 解析失败，返回原始 body
        return body
    }

    reqMap["model"] = newModel

    newBody, err := json.Marshal(reqMap)
    if err != nil {
        // 序列化失败，返回原始 body
        return body
    }

    return newBody
}
```

#### 5.2 模型映射逻辑

**调用位置**: `forwardOpenAIPassthroughWithInput()` 的密钥迭代循环中

**逻辑**：
```go
if selection.UpstreamModel != input.RequestModel {
    keyInput.Body = s.replaceModelInBody(input.Body, selection.UpstreamModel)
}
```

**说明**：
- `selection.UpstreamModel` 来自 `AccountAPIKey.ResolveUpstreamModelForRequest()`
- 如果密钥配置了模型映射（如 `gpt-4` → `gpt-4-turbo`），则替换请求体中的模型名
- 如果没有映射，`UpstreamModel` 等于 `RequestModel`，不需要替换

---

## 三、测试策略

### 3.1 单元测试

**测试文件**: `backend/internal/service/openai_gateway_service_test.go`

**测试用例**：

1. **密钥池调度测试**
   - 测试多个可用密钥的 LRU 排序
   - 测试优先级排序（高优先级优先）
   - 测试同优先级的 LRU 排序

2. **故障转移测试**
   - 测试 401 错误触发故障转移
   - 测试 429 错误触发故障转移
   - 测试 5xx 错误触发故障转移
   - 测试 400 错误不触发故障转移

3. **冷却机制测试**
   - 测试 401 错误触发 1 小时冷却
   - 测试 429 错误触发 5 分钟冷却
   - 测试 5xx 错误触发 1 分钟冷却
   - 测试冷却期间密钥不可用

4. **模型映射测试**
   - 测试请求体中的模型名替换
   - 测试无映射时不修改请求体

5. **向后兼容测试**
   - 测试传统单密钥账号仍然正常工作
   - 测试 OAuth 账号不受影响

### 3.2 集成测试

**测试场景**：

1. **正常流程**
   - 创建带密钥池的账号
   - 发送 OpenAI API 请求
   - 验证请求成功转发
   - 验证使用统计更新

2. **故障转移流程**
   - 模拟第一个密钥返回 401
   - 验证自动切换到第二个密钥
   - 验证第一个密钥被冷却
   - 验证第二个密钥成功转发

3. **全部密钥失败**
   - 模拟所有密钥都返回错误
   - 验证返回 503 错误
   - 验证所有密钥都被冷却

4. **流式响应**
   - 测试流式请求的正常转发
   - 测试流式响应开始后的错误不故障转移

### 3.3 手动测试清单

- [ ] 使用 Postman/curl 测试 OpenAI chat completions API
- [ ] 测试流式响应（`stream: true`）
- [ ] 测试非流式响应（`stream: false`）
- [ ] 测试 reasoning effort 参数（o1 系列模型）
- [ ] 测试密钥池故障转移（手动禁用第一个密钥）
- [ ] 测试冷却机制（观察数据库 `account_api_key_model_cooldowns` 表）
- [ ] 测试使用统计（观察 `account_api_keys` 表的 `last_used_at` 字段）
- [ ] 测试管理后台显示完整错误消息
- [ ] 测试用户 API 返回净化后的错误消息

---

## 四、OpenAI 特定注意事项

### 4.1 OpenAI API 错误响应格式

**标准格式**：
```json
{
  "error": {
    "message": "Incorrect API key provided",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

**错误类型**：
- `invalid_request_error`: 客户端错误（400, 401, 403, 404）
- `rate_limit_error`: 速率限制（429）
- `server_error`: 服务器错误（5xx）

### 4.2 OpenAI 特定的错误码

| 错误码 | HTTP 状态 | 含义 | 是否故障转移 |
|-------|----------|------|------------|
| `invalid_api_key` | 401 | API 密钥无效 | 是 |
| `insufficient_quota` | 429 | 配额不足 | 是 |
| `rate_limit_exceeded` | 429 | 速率限制 | 是 |
| `model_not_found` | 404 | 模型不存在 | 否 |
| `context_length_exceeded` | 400 | 上下文长度超限 | 否 |

### 4.3 Reasoning Effort 参数

**适用模型**: o1-preview, o1-mini

**参数位置**: 请求体中的 `reasoning_effort` 字段

**注意事项**：
- 只有 o1 系列模型支持此参数
- 其他模型会忽略此参数
- 转发时需要保留此参数

### 4.4 流式响应格式

**SSE 格式**：
```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: [DONE]
```

**注意事项**：
- 流式响应开始后不能故障转移
- 需要正确处理 `[DONE]` 标记
- 需要保持 SSE 连接的稳定性

---

## 五、数据库 Schema 参考

### 5.1 `account_api_keys` 表

```sql
CREATE TABLE account_api_keys (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    api_key TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**字段说明**：
- `priority`: 优先级（数字越大优先级越高）
- `enabled`: 是否启用
- `last_used_at`: 最后使用时间（用于 LRU 排序）

### 5.2 `account_api_key_model_cooldowns` 表

```sql
CREATE TABLE account_api_key_model_cooldowns (
    id BIGSERIAL PRIMARY KEY,
    account_api_key_id BIGINT NOT NULL REFERENCES account_api_keys(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    cooldown_until TIMESTAMP NOT NULL,
    reason TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(account_api_key_id, model)
);
```

**字段说明**：
- `model`: 模型名（如 `gpt-4`, `gpt-3.5-turbo`）
- `cooldown_until`: 冷却截止时间
- `reason`: 冷却原因（HTTP 状态码 + 错误消息）

---

## 六、验证清单

### 6.1 代码质量

- [ ] 所有新方法都有完整的注释
- [ ] 错误处理覆盖所有边界情况
- [ ] 日志记录关键操作（密钥选择、故障转移、冷却）
- [ ] 代码风格符合项目规范（gofmt, golint）

### 6.2 功能完整性

- [ ] 密钥池调度逻辑正确实现
- [ ] 故障转移逻辑正确实现
- [ ] 冷却机制正确集成
- [ ] 使用统计正确更新
- [ ] 模型映射正确实现
- [ ] 向后兼容传统单密钥账号

### 6.3 性能

- [ ] 密钥选择算法时间复杂度为 O(n log n)（排序）
- [ ] 数据库查询使用索引（`account_id`, `model`）
- [ ] 无不必要的数据库往返
- [ ] 流式响应无额外延迟

### 6.4 安全性

- [ ] API 密钥不记录到日志
- [ ] 错误消息不泄露敏感信息（用户 API）
- [ ] 管理后台需要权限验证
- [ ] 冷却清除操作需要二次确认

---

## 七、参考资料

### 7.1 相关文件

- `backend/internal/service/gateway_service.go` (第 5043-5241 行)
  - Anthropic 平台的参考实现
  - 三层方法结构的完整示例

- `backend/internal/service/account_api_key_pool.go`
  - 密钥池调度逻辑
  - `EffectiveAPIKeySelectionsForRequest()` 方法

- `backend/migrations/135_account_api_key_pool.sql`
  - 数据库 schema 定义

- `secondary-dev/demand/开发路线图.md` v1.1
  - 最终实现决策（8 个决策点）
  - 配置示例

### 7.2 关键方法

| 方法名 | 文件 | 用途 |
|-------|------|------|
| `EffectiveAPIKeySelectionsForRequest()` | `account_api_key_pool.go` | 获取排序后的可用密钥列表 |
| `cooldownAccountAPIKeyModel()` | `gateway_service.go` | 冷却指定密钥的指定模型 |
| `markAccountAPIKeyUsed()` | `gateway_service.go` | 标记密钥使用并更新统计 |
| `HasAccountAPIKeyPool()` | `account.go` | 判断账号是否使用密钥池 |
| `ResolveUpstreamModelForRequest()` | `account_api_key_pool.go` | 解析模型映射 |

### 7.3 错误类型

```go
type UpstreamFailoverError struct {
    StatusCode   int
    ResponseBody []byte
}

func (e *UpstreamFailoverError) Error() string {
    return fmt.Sprintf("upstream failover error: %d", e.StatusCode)
}
```

**用途**: 标识可重试的上游错误，触发故障转移逻辑

---

## 八、实现时间估算

| 任务 | 预计时间 | 优先级 |
|-----|---------|-------|
| 任务 1: 代码重构 | 2-3 小时 | 高 |
| 任务 2: 错误分类 | 1 小时 | 高 |
| 任务 3: 冷却机制 | 30 分钟 | 高 |
| 任务 4: 使用统计 | 30 分钟 | 中 |
| 任务 5: 模型映射 | 1 小时 | 中 |
| 单元测试 | 2 小时 | 高 |
| 集成测试 | 1 小时 | 中 |
| 手动测试 | 1 小时 | 中 |
| **总计** | **9-10 小时** | - |

---

## 九、实施建议

### 9.1 实施顺序

1. **第一阶段**（核心功能）
   - 任务 1.1: 创建 `forwardOpenAIPassthroughWithInput()`
   - 任务 1.2: 重构 `forwardOpenAIPassthrough()`
   - 任务 1.3: 提取 `forwardOpenAIPassthroughWithToken()`
   - 任务 2.1: 实现错误分类逻辑

2. **第二阶段**（集成功能）
   - 任务 3: 冷却机制集成
   - 任务 4: 使用统计集成
   - 任务 5: 模型映射支持

3. **第三阶段**（测试验证）
   - 单元测试
   - 集成测试
   - 手动测试

### 9.2 风险点

1. **流式响应处理**
   - 风险：流式响应开始后无法故障转移
   - 缓解：确保在发送第一个数据块前完成密钥验证

2. **向后兼容性**
   - 风险：破坏现有单密钥账号的功能
   - 缓解：保留传统单密钥路径，充分测试

3. **错误分类准确性**
   - 风险：错误分类不准确导致不必要的故障转移或遗漏故障转移
   - 缓解：参考 OpenAI 官方文档，充分测试各种错误场景

### 9.3 调试建议

1. **启用详细日志**
   ```go
   log.Printf("[OpenAI] Selected key: %s, priority: %d, model: %s",
       key.ID, key.Priority, keyInput.RequestModel)
   log.Printf("[OpenAI] Failover triggered: status=%d, key=%s",
       failoverErr.StatusCode, key.ID)
   ```

2. **监控数据库状态**
   ```sql
   -- 查看冷却状态
   SELECT * FROM account_api_key_model_cooldowns
   WHERE cooldown_until > NOW();

   -- 查看密钥使用情况
   SELECT id, priority, last_used_at, enabled
   FROM account_api_keys
   WHERE account_id = ?
   ORDER BY priority DESC, last_used_at ASC;
   ```

3. **使用 Postman 测试**
   - 创建测试集合，包含各种错误场景
   - 使用环境变量切换不同的密钥
   - 观察响应时间和故障转移行为

---

**文档结束**

如有疑问或需要澄清，请参考 `开发路线图.md` v1.1 或联系文档作者。
