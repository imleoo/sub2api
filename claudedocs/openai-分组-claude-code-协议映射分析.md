# OpenAI 分组下 Claude Code 协议映射深度分析

> 分析基准：commit `509773ac`（zhiguofan 1.1.124）  
> 分析时间：2026-05-07

---

## 一、总览

Claude Code 客户端发送的是 **Anthropic Messages API** 格式请求（`POST /v1/messages`）。当后台分配的分组（Group）平台类型为 `openai` 时，系统会将整个请求链路切换到 OpenAI Responses API，完成协议映射后再将响应翻译回 Anthropic 格式返回给客户端。

整个映射由五层机制协同完成：

| 层次 | 职责 | 核心文件 |
|------|------|---------|
| 路由层 | 按分组平台类型分发请求 | `routes/gateway.go` |
| 处理层 | 请求解析、权限校验、账号调度 | `handler/openai_gateway_handler.go` |
| 转换层 | 请求/响应格式互转 | `pkg/apicompat/*.go` |
| 服务层 | 上游转发、会话管理、流处理 | `service/openai_gateway_messages.go` |
| 映射层 | 模型名映射 | `service/openai_model_mapping.go` |

---

## 二、请求路由：如何识别 OpenAI 分组

### 2.1 路由分叉点

**文件**：`internal/server/routes/gateway.go:51-57`

```go
gateway.POST("/messages", func(c *gin.Context) {
    if getGroupPlatform(c) == service.PlatformOpenAI {
        h.OpenAIGateway.Messages(c)   // OpenAI 分组走这条路
        return
    }
    h.Gateway.Messages(c)             // Anthropic / Antigravity 分组
})
```

### 2.2 分组平台读取

**文件**：`internal/server/routes/gateway.go:229-235`

```go
func getGroupPlatform(c *gin.Context) string {
    apiKey, ok := middleware2.GetAPIKeyFromContext(c)
    if !ok || apiKey.Group == nil {
        return ""
    }
    return apiKey.Group.Platform   // 来自数据库 groups.platform 字段
}
```

**关键常量**：`service.PlatformOpenAI = "openai"`

### 2.3 账号类型体系

**文件**：`internal/service/account.go:964-978`

OpenAI 分组下有两种账号子类型，两者转发路径有差异：

| 子类型 | 常量 | 上游地址 | 凭证形式 |
|--------|------|---------|---------|
| OAuth（ChatGPT Plus/Pro）| `AccountTypeOAuth` | `chatgpt.com/backend-api/codex/responses` | Bearer access_token（定期刷新）|
| API Key | `AccountTypeAPIKey` | `api.openai.com/v1/responses` | Bearer sk-xxx |

```go
func (a *Account) IsOpenAIOAuth() bool  { return a.IsOpenAI() && a.Type == AccountTypeOAuth }
func (a *Account) IsOpenAIApiKey() bool { return a.IsOpenAI() && a.Type == AccountTypeAPIKey }
```

---

## 三、模型名映射

这是整个协议适配中最先执行的步骤，决定了上游实际调用哪个 GPT 模型。

### 3.1 三级映射优先级

**文件**：`internal/service/openai_model_mapping.go:8-20`

```
优先级 1（最高）: 账号级 model_mapping（管理员在账号配置中手动设置）
优先级 2        : 渠道级显式调度映射（仅对 claude-* 系列模型生效）
优先级 3（最低）: 原始模型名直通（兜底）
```

```go
func resolveOpenAIForwardModel(account *Account, requestedModel, defaultMappedModel string) string {
    mappedModel, matched := account.ResolveMappedModel(requestedModel)
    if !matched && defaultMappedModel != "" &&
        claudeMessagesDispatchFamily(requestedModel) != "" {
        return defaultMappedModel
    }
    return mappedModel
}
```

### 3.2 账号级模型映射（支持通配符）

**文件**：`internal/service/account.go:627-649`

账号的 `model_mapping` 字段（JSON）支持精确匹配和 `*` 前缀通配符，精确匹配优先级更高：

```
model_mapping 配置示例：
  "claude-3-5-sonnet-20241022"  → "gpt-4o"           (精确，优先级最高)
  "claude-3-opus*"              → "gpt-4-turbo"       (前缀通配)
  "claude-*"                    → "gpt-4o-mini"       (宽泛通配，兜底)
```

### 3.3 OAuth 账号的 Codex 模型标准化

**文件**：`internal/service/openai_codex_transform.go:835-840`

OAuth 账号走 ChatGPT Codex 路径，模型名需要额外标准化（映射为 Codex 内部可接受的模型名），API Key 账号则直通：

```go
func normalizeOpenAIModelForUpstream(account *Account, model string) string {
    if account == nil || account.Type == AccountTypeOAuth {
        return normalizeCodexModel(model)  // Codex 专属标准化
    }
    return strings.TrimSpace(model)        // API Key 直通
}
```

---

## 四、请求体转换：Anthropic Messages → OpenAI Responses API

### 4.1 入口与权限检查

**文件**：`internal/handler/openai_gateway_handler.go:540-604`

```go
// 1. 检查分组是否允许 /v1/messages 调度
if apiKey.Group != nil && !apiKey.Group.AllowMessagesDispatch {
    h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error", ...)
    return
}

// 2. 模型标准化
routingModel := service.NormalizeOpenAICompatRequestedModel(reqModel)

// 3. 渠道级显式映射解析
preferredMappedModel := resolveOpenAIMessagesDispatchMappedModel(apiKey, reqModel)
```

### 4.2 核心转换函数

**文件**：`internal/pkg/apicompat/anthropic_to_responses.go:13-73`

`AnthropicToResponses()` 是请求体转换的核心，完成以下映射：

#### 消息格式转换

| Anthropic 字段 | Responses API 字段 | 转换规则 |
|--------------|------------------|---------|
| `system: string` | `input[0]` | role=`"developer"`，放在 input 数组首位 |
| `messages[].role` | `input[].role` | `"user"` / `"assistant"` 直接映射 |
| `messages[].content` | `input[].content` | 文本/图片/工具结果块逐一转换 |

#### 参数映射

| Anthropic 参数 | Responses API 参数 | 说明 |
|--------------|------------------|------|
| `temperature` | `temperature` | 直通 |
| `top_p` | `top_p` | 直通 |
| `max_tokens` | `max_output_tokens` | 重命名 |
| `stream: true/false` | `stream: true`（强制）| 上游始终流式，客户端偏好在后处理层处理 |
| `thinking.type="enabled"` | `reasoning.effort` | 扩展思维模式映射 |
| `output_config.effort` | `reasoning.effort` | 推理深度 |

#### 思维强度（reasoning effort）映射

```
anthropic effort  →  Responses API effort
"low"            →  "low"
"medium"         →  "medium"
"high"           →  "high"
"xhigh" / 空     →  "high"（最高档）
```

额外注入：`include: ["reasoning.encrypted_content"]`，用于获取加密推理内容。

#### 工具调用转换

| Anthropic Tool | Responses Function Tool |
|---------------|------------------------|
| `tools[].name` | `functions[].name` |
| `tools[].description` | `functions[].description` |
| `tools[].input_schema` | `functions[].parameters` |
| `tool_choice.type="auto"` | `tool_choice: "auto"` |
| `tool_choice.type="any"` | `tool_choice: "required"` |
| `tool_choice.type="tool"` | `tool_choice: {type:"function", name:...}` |

### 4.3 OAuth 账号的额外 Codex 变换

**文件**：`internal/service/openai_gateway_messages.go:152-203`

OAuth 账号（ChatGPT Plus/Pro）不走标准 Responses API，而是走 ChatGPT Codex 专用路径，需要额外变换：

```go
if account.Type == AccountTypeOAuth {
    // 应用 Codex 特定转换（模型名正规化、instructions 字段注入）
    applyCodexOAuthTransformWithOptions(reqBody, ...)

    // 强制确保 instructions 字段存在（Codex API 必填）
    ensureCodexOAuthInstructionsField(reqBody)
}
```

### 4.4 Fast Mode 处理

**文件**：`internal/service/openai_gateway_messages.go:102-104`

Claude Code 客户端通过 `anthropic-beta: fast-mode-2026-02-01` 头请求快速模式，转换时映射为 OpenAI 优先级标记：

```go
if containsBetaToken(c.GetHeader("anthropic-beta"), claude.BetaFastMode) {
    responsesReq.ServiceTier = "priority"
}
```

---

## 五、完整转发流程（服务层）

**文件**：`internal/service/openai_gateway_messages.go`，核心方法 `ForwardAsAnthropic()`

### 步骤序列

```
步骤 1  解析 Anthropic 请求体
         └─ json.Unmarshal → AnthropicRequest{}
         └─ applyOpenAICompatModelNormalization()

步骤 2  确定三个模型名
         ├─ normalizedModel   = 标准化后的客户端请求模型（用于路由匹配）
         ├─ billingModel      = 映射后的计费模型（用于 Token 计费）
         └─ upstreamModel     = 上游实际使用的模型（发给 OpenAI 的）

步骤 3  会话键提取（三级回退）
         ├─ 优先：metadata.user_id 字段
         ├─ 次级：请求体中 cache_control 内容派生
         └─ 兜底：对整个请求体做摘要哈希

步骤 4  查询 previous_response_id（会话连续性）
         └─ s.getOpenAICompatSessionResponseID(account, promptCacheKey)
         └─ 存在则注入 responsesReq.PreviousResponseID

步骤 5  AnthropicToResponses() 核心转换
         └─ 参见第四节

步骤 6  （OAuth 账号）Codex 变换
         └─ applyCodexOAuthTransformWithOptions()

步骤 7  获取访问令牌
         ├─ OAuth: 从 token 池获取有效的 access_token
         └─ API Key: 从账号凭证读取 sk-xxx

步骤 8  构建上游 HTTP 请求
         ├─ 目标 URL：按账号类型选择（chatgpt.com 或 api.openai.com）
         ├─ 注入隔离 session_id（防止跨 API Key 会话混用）
         │   └─ isolatedSessionID = UUID(apiKeyID + promptCacheKey)
         └─ 设置 Authorization、Content-Type 等头

步骤 9  发送上游请求（带代理支持）
         └─ s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)

步骤 10 错误处理
         ├─ previous_response_id 失效 → 清除后重试（递归）
         ├─ Pool Mode 账号 → 返回 UpstreamFailoverError 触发换号重试
         └─ 其他错误 → 转换为 Anthropic 错误格式写回

步骤 11 响应处理
         ├─ 客户端要求流式 → handleAnthropicStreamingResponse()
         └─ 客户端不要流式 → handleAnthropicBufferedStreamingResponse()

步骤 12 保存 response_id（下次请求的会话连续性）
         └─ s.setOpenAICompatSessionResponseID(account, promptCacheKey, responseID)
```

---

## 六、响应转换：OpenAI Responses API → Anthropic SSE

### 6.1 流式路径（SSE → SSE 实时转换）

**文件**：`internal/service/openai_gateway_messages.go:622-800`  
**转换核心**：`internal/pkg/apicompat/responses_event_to_anthropic.go`

上游返回的是 OpenAI Responses API 的 SSE 事件流，逐行解析后翻译为 Anthropic SSE 事件实时写给客户端：

#### 事件类型映射表

| OpenAI Responses 事件 | 翻译为 Anthropic 事件 | 说明 |
|---------------------|---------------------|------|
| `response.created` | `message_start` | 携带 model、usage 初始值 |
| `response.output_item.added` | `content_block_start` | 输出块开始（text / tool_use / thinking）|
| `response.output_text.delta` | `content_block_delta` (text_delta) | 普通文本增量 |
| `response.reasoning_summary_text.delta` | `content_block_delta` (thinking_delta) | 扩展思维增量 |
| `response.function_call_arguments.delta` | `content_block_delta` (input_json_delta) | 工具调用参数增量 |
| `response.output_item.done` | `content_block_stop` | 输出块结束 |
| `response.completed` / `response.done` | `message_delta` + `message_stop` | 携带最终 usage 和停止原因 |

#### 处理逻辑

```go
state := apicompat.NewResponsesEventToAnthropicState()
state.Model = originalModel  // 确保返回客户端请求的原始模型名，而非 GPT 模型名

for scanner.Scan() {
    payload, _ := extractOpenAISSEDataLine(line)
    var event ResponsesStreamEvent
    json.Unmarshal(payload, &event)

    // 终止事件：提取最终 usage
    if isOpenAICompatResponsesTerminalEvent(event.Type) {
        usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
    }

    // 转换为 Anthropic 事件列表（一个 Responses 事件可能对应多个 Anthropic 事件）
    events := apicompat.ResponsesEventToAnthropicEvents(&event, state)

    // 逐事件序列化为 SSE 格式并写入
    for _, evt := range events {
        sse, _ := apicompat.ResponsesAnthropicEventToSSE(evt)
        fmt.Fprint(c.Writer, sse)
    }
    c.Writer.Flush()
}
```

### 6.2 非流式路径（SSE 缓冲 → JSON 组装）

**文件**：`internal/service/openai_gateway_messages.go:427-616`  
**转换核心**：`internal/pkg/apicompat/responses_to_anthropic.go`

上游始终流式，若客户端不需要流式，则先缓冲所有 SSE 事件，再通过 `ResponsesToAnthropic()` 组装为完整的 Anthropic JSON 响应：

#### 内容块类型映射

| Responses 输出类型 | Anthropic content block |
|------------------|------------------------|
| `"message"` / `output_text` | `{"type":"text","text":"..."}` |
| `"reasoning"` | `{"type":"thinking","thinking":"..."}` |
| `"function_call"` | `{"type":"tool_use","id":"...","name":"...","input":{...}}` |
| `"web_search_call"` | `{"type":"server_tool_use"}` + `{"type":"web_search_tool_result"}` |

**关键细节**：返回给客户端的 `model` 字段始终使用客户端请求的原始模型名（如 `claude-3-5-sonnet-20241022`），而不是上游实际调用的 GPT 模型名。

### 6.3 停止原因映射

| Responses API `status` | Anthropic `stop_reason` |
|----------------------|------------------------|
| `"completed"` | `"end_turn"` |
| `"max_output_tokens"` | `"max_tokens"` |
| `"incomplete"` | `"max_tokens"` |
| 工具调用存在 | `"tool_use"` |

---

## 七、Token 计费处理

**文件**：`internal/service/openai_gateway_messages.go:917-929`

Token 计数在 Responses API 和 Anthropic API 中语义对齐，可以直通，缓存 token 做细分处理：

```go
func copyOpenAIUsageFromResponsesUsage(usage *ResponsesUsage) OpenAIUsage {
    return OpenAIUsage{
        InputTokens:          usage.InputTokens,   // 直通
        OutputTokens:         usage.OutputTokens,  // 直通
        CacheReadInputTokens: usage.InputTokensDetails.CachedTokens,  // 缓存命中
    }
}
```

**Anthropic 格式 Usage**（返回给客户端）：

```json
{
  "input_tokens": 1024,
  "output_tokens": 256,
  "cache_creation_input_tokens": 0,
  "cache_read_input_tokens": 512
}
```

---

## 八、错误码映射

### 8.1 HTTP 状态码 → Anthropic 错误格式

**文件**：`internal/handler/openai_gateway_handler.go:847-876`

所有错误（包括上游返回的错误）都被包装为 Anthropic 标准错误格式：

```json
{
  "type": "error",
  "error": {
    "type": "api_error",
    "message": "具体错误信息"
  }
}
```

| 触发场景 | HTTP 状态 | `error.type` |
|---------|----------|-------------|
| 分组不允许 `/v1/messages` | 403 | `permission_error` |
| 请求体过大 | 413 | `invalid_request_error` |
| 并发限额已满 | 429 | `rate_limit_error` |
| 上游请求失败 | 502 | `api_error` |
| 账号不可用 | 503 | `api_error` |

### 8.2 流式错误处理

若响应流已开始，错误通过 SSE 事件下发（而非 HTTP 状态码）：

```
event: error
data: {"type":"api_error","message":"..."}
```

### 8.3 failover 决策

**文件**：`internal/service/openai_gateway_messages.go:330-350`

Pool Mode 账号（API Key 池）在以下状态码触发换号重试：

```go
// 401 / 403 / 429 可在同账号内 Pool 重试
func isPoolModeRetryableStatus(statusCode int) bool {
    switch statusCode {
    case 401, 403, 429:
        return true
    }
    return false
}
```

---

## 九、会话连续性与缓存键

**文件**：`internal/service/openai_gateway_messages.go:52-88`

OpenAI Responses API 通过 `previous_response_id` 实现多轮对话记忆。系统将 Anthropic 的无状态请求转换为有状态的 Responses API 会话：

### 会话键提取（三级回退）

```
回退级别 1：metadata.user_id 字段
              └─ Claude Code 通常在此传递会话标识

回退级别 2：请求体中 cache_control 内容哈希
              └─ 从系统提示的 cache_control 标记派生

回退级别 3：请求摘要哈希（兜底）
              └─ 对 system + messages 内容做 SHA 摘要
```

### 隔离机制

为防止不同 API Key 复用同一上游会话，系统对 session_id 做了隔离处理：

```go
isolatedSessionID = generateSessionUUID(
    isolateOpenAISessionID(apiKeyID, promptCacheKey)
)
upstreamReq.Header.Set("session_id", isolatedSessionID)
```

### previous_response_id 失效重试

当上游报告 `previous_response_id` 不存在时（会话已过期），系统自动清除缓存后以新会话重试：

```go
if previousResponseID != "" && isOpenAICompatPreviousResponseNotFound(...) {
    s.deleteOpenAICompatSessionResponseID(...)
    return s.ForwardAsAnthropic(ctx, c, account, body, promptCacheKey, ...)  // 清除后重试
}
```

---

## 十、完整调用链时序

```
Claude Code 客户端
│
│  POST /v1/messages
│  Headers: anthropic-beta, anthropic-version
│  Body: {model, system, messages, tools, stream, ...}
│
▼
[Middleware] API Key 认证 → 加载 Group（含 platform 字段）
│
▼
routes/gateway.go  getGroupPlatform() == "openai"?
│                         │ YES
│                         ▼
│              OpenAIGatewayHandler.Messages()
│              handler/openai_gateway_handler.go:540
│                         │
│                ┌─────────────────────────────────────┐
│                │ 1. 权限检查 AllowMessagesDispatch    │
│                │ 2. 解析请求体                        │
│                │ 3. 模型标准化 + 渠道映射             │
│                │ 4. 账号调度（SelectAccountWithScheduler）│
│                └─────────────────────────────────────┘
│                         │
│                         ▼
│              openai_gateway_messages.go:ForwardAsAnthropic()
│                         │
│              ┌──────────────────────────────────────────────────┐
│              │ 1. 解析 AnthropicRequest                         │
│              │ 2. 确定 normalizedModel / billingModel / upstreamModel│
│              │ 3. 提取 promptCacheKey（三级回退）               │
│              │ 4. 查询 previous_response_id                     │
│              │ 5. AnthropicToResponses() 核心格式转换           │
│              │ 6. （OAuth）Codex 变换 + instructions 注入       │
│              │ 7. Fast Mode → ServiceTier: "priority"           │
│              │ 8. 获取 access_token / API key                   │
│              │ 9. 注入隔离 session_id                           │
│              │ 10. HTTP POST → 上游                             │
│              └──────────────────────────────────────────────────┘
│                         │
│               ┌─────────┴──────────┐
│               │ OAuth 账号          │ API Key 账号
│               ▼                    ▼
│    chatgpt.com/backend-api    api.openai.com/v1/responses
│    /codex/responses           (标准 OpenAI Responses API)
│               └─────────┬──────────┘
│                         │ OpenAI SSE 流返回
│                         ▼
│              ┌──────────────────────────────────────┐
│              │ 流式路径：逐事件翻译                   │
│              │   ResponsesEvent → AnthropicEvent     │
│              │   → SSE 格式 → 实时写回               │
│              │                                       │
│              │ 非流式路径：缓冲 → 组装               │
│              │   全部 SSE → ResponsesResponse        │
│              │   → ResponsesToAnthropic()            │
│              │   → JSON 写回                         │
│              └──────────────────────────────────────┘
│                         │
▼                         ▼
Claude Code 客户端接收 Anthropic 格式响应
（model 字段 = 原始请求模型名，非 GPT 模型名）
```

---

## 十一、涉及的核心文件索引

| 文件 | 主要职责 |
|------|---------|
| `internal/server/routes/gateway.go:51-57` | 路由分叉：按 Group.Platform 分发 |
| `internal/handler/openai_gateway_handler.go:540-604` | 请求入口、权限校验、账号调度 |
| `internal/service/openai_gateway_messages.go` | 完整转发链路（ForwardAsAnthropic）|
| `internal/pkg/apicompat/anthropic_to_responses.go` | 请求格式转换核心 |
| `internal/pkg/apicompat/responses_to_anthropic.go` | 非流式响应组装 |
| `internal/pkg/apicompat/responses_event_to_anthropic.go` | 流式事件翻译 |
| `internal/service/openai_model_mapping.go` | 三级模型名映射逻辑 |
| `internal/service/openai_codex_transform.go` | OAuth Codex 专用变换 |
| `internal/service/account.go:627-649, 964-978` | 账号类型判断、模型映射解析 |
| `internal/service/openai_account_scheduler.go` | 账号调度（Pool Mode 等）|

---

*生成于 2026-05-07，基于 zhiguofan 分支 commit 509773ac*
