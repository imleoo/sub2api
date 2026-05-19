# API 中转商架构设计方案

> 状态：草稿 v2 | 日期：2026-05-14 | 修订：结合 zhiguofan 分支当前源码事实修订路线图与桥接矩阵判断；新增 §11 流式 SSE 测试集；Phase 拆分调整为 6 阶段（Phase 0 计费先行 + Phase 1–5）。

---

## 1. 核心问题：入站协议与出站协议尚未解耦

一个 AI API 中转商的目标形态是：

```text
用户使用任意一种协议 -> 系统调度 -> 向可用上游协议转发 -> 按用户入站协议返回
```

用户侧有多种入站协议，上游侧也有多种出站协议。当前代码的主要限制是：**分组、路由、账号筛选仍大量依赖 `platform`，入站协议和出站协议没有形成独立调度维度**。

```text
                          上游账号实际协议
                 Anthropic      OpenAI         Gemini
入站协议  --------------------------------------------------
Anthropic Msg  |  已有(同协议)  已有 [B1]      Gemini 特例
OpenAI Chat    |  apicompat 可拼装 [B2]  已有(同协议)  待实现
OpenAI Resp    |  已有 [B3]    已有(同协议)   待实现
Gemini v1beta  |  待实现       待实现         已有(同协议)

[B1] `OpenAIGatewayService.ForwardAsAnthropic`
     (backend/internal/service/openai_gateway_messages.go:28)
     Anthropic Messages 入站 -> OpenAI Responses 上游 -> Anthropic Messages 响应。
[B3] `GatewayService.ForwardAsResponses`
     (backend/internal/service/gateway_forward_as_responses.go:31)
     OpenAI Responses 入站 -> Anthropic Messages 上游 -> OpenAI Responses 响应。
[B2] 尚未实现，但 `backend/internal/pkg/apicompat/chatcompletions_to_responses.go`
     + `responses_to_anthropic.go` 已提供零件，只需拼装。
```

这个矩阵中：

- **对角线**：同协议透传或同平台处理，当前系统已有。
- **Anthropic -> OpenAI Responses**：已存在 `ForwardAsAnthropic`（B1），嵌在 OpenAI handler/service 中的兼容链路；Phase 3 抽到 Bridge 注册体系。
- **OpenAI Responses -> Anthropic**：**已存在 `ForwardAsResponses`（B3），先前文档判断为"待实现"是陈旧的**；同样在 Phase 3 由 Bridge 注册接管。
- **OpenAI ChatCompletions -> Anthropic**：尚未实现，但 `backend/internal/pkg/apicompat/` 已具备双向零件，Phase 3 完成拼装即可。
- **Gemini 跨协议组合**：当前有 Gemini 自身和部分 Antigravity/Gemini 特例，不是通用 N×M 矩阵；Phase 4 补齐 Gemini 入站桥。

---

## 2. 当前代码事实

### 2.1 `platform` 仍同时承担多种职责

当前 `Group.Platform` 仍直接影响入口 handler 分流：

```text
Group.Platform = "openai"
  /v1/messages          -> OpenAIGateway.Messages
  /v1/chat/completions  -> OpenAIGateway.ChatCompletions
  /v1/responses         -> OpenAIGateway.Responses

Group.Platform != "openai"
  /v1/messages          -> Gateway.Messages
  /v1/chat/completions  -> Gateway.ChatCompletions
  /v1/responses         -> Gateway.Responses

# fork 第 12 项 lingjing 异步任务分流（routes/gateway.go:168, 193-203）
Group.Platform = "openai" && Account.IsLingjing()
  /v1/images/generations -> LingjingGateway.ForwardSeedreamImage（同步轮询 60s）

Group.Platform = "lingjing"
  /lingjing/v1/video/submit -> LingjingHandler.SubmitVideoTask（异步 HTTP 202 + taskId）
  /lingjing/v1/video/:taskId -> LingjingHandler.GetVideoTaskStatus（poll 查询）
```

`/v1beta/*` 入口也仍以 Gemini handler 为主，并在 handler 内校验 `Group.Platform == gemini` 或显式 force platform。也就是说，入口层尚未按 `Group.InboundProtocol` 解耦。

> **fork 第 12 项 lingjing 注意**：lingjing 平台是 5 个 platform 中的**第 5 个**（详见 `glossary.md §1.2`），**有自己的 ForcePlatform middleware**（`routes/gateway.go:198`，第 4 处 ForcePlatform）。Phase 2 P2-3 改造时**不能并入"7 处 platform 分流"批量改造**——lingjing 的 `/lingjing/v1/video/*` 路由组和 `/v1/images/generations` 的 lingjing 分支需单独维护，且 Phase 5 多 endpoint 重构**不纳入 lingjing**（详见 §6 与 `generic-channel-design.md` §10）。

### 2.2 现有协议转换点

| 文件/函数 | 当前真实方向 | 现状边界 |
|---|---|---|
| `openai_gateway_messages.go:28 ForwardAsAnthropic` | Anthropic Messages 入站 → OpenAI Responses 上游 → Anthropic Messages 响应 | 已实现，但绑定在 OpenAI 账号链路和 OpenAI handler 调度中 |
| `gateway_forward_as_responses.go:31 ForwardAsResponses` | OpenAI Responses 入站 → Anthropic Messages 上游 → OpenAI Responses 响应 | 已实现，反向桥；同样未抽到 Bridge 注册体系 |
| `openai_gateway_chat_completions_raw.go` | OpenAI Chat Completions 入站 → OpenAI 兼容出站 | 平台内透传/兼容，不是跨到 Anthropic |
| `gemini_messages_compat_service.go` | Anthropic Messages 入站 → Gemini 出站兼容 | Gemini/Antigravity 相关特例，不是通用 Bridge |
| `antigravity_gateway_service.go:4207 ForwardUpstream` | Anthropic Messages 入站 → Anthropic Messages 出站 | Antigravity upstream 类型透传 |
| `pkg/apicompat/` 9 个文件 | 协议层双向转换工具集 | Anthropic↔Responses（含流式 SSE）、ChatCompletions↔Responses、Responses↔Anthropic 已成型，是 N×M 矩阵的翻译层 |

### 2.3 现状结论

- 协议双向转换的"翻译层"在 `pkg/apicompat/` 已经基本成型；缺的是把 `ForwardAsAnthropic` 与 `ForwardAsResponses` 抽到 `ProtocolBridge` 注册表，并拼装 ChatCompletions↔Anthropic 一对桥（apicompat 已有零件）。
- `ForwardAsAnthropic` / `ForwardAsResponses` 的命名容易误导：它们的含义是"让某协议账号接受另一种入站协议"，不是单向转换，请按方向标注（"入站→上游"）阅读。
- 当前路由仍由入口路径和 `Group.Platform` 选 handler，再由 handler/service 做账号选择、sticky、并发、计费、failover。Bridge 改造必须嵌入现有链路，不能取代 handler/service。
- 平台常量扩散面：`Platform{Anthropic,OpenAI,Gemini,Antigravity}` 已在大量文件中固化（精确数字见 `glossary.md` §6）。`Account.platform`、`Group.platform` 已固化在 ent schema 和索引中（代码位置见 `glossary.md` §5）。这决定了**只能渐进迁移**，一次性替换不现实。

---

## 3. 目标架构：三层分离，但渐进落地

### 3.1 三层职责

```text
Layer 1: 入站协议层
  Group.InboundProtocol 决定用户可使用的请求协议。
  必须同步改造 /v1/messages、/v1/chat/completions、/v1/responses、
  /responses、/backend-api/codex/responses、/v1beta/* 的分流逻辑。

Layer 2: 协议桥层
  当 inbound_protocol != outbound_protocol 时执行请求/响应转换。
  Bridge 不绕过现有并发、计费、sticky、failover、usage 记录链路。

Layer 3: 出站账号层
  Account.OutboundProtocol 描述实际打上游的协议。
  候选账号筛选不能只看 account.Platform，还要看 outbound_protocol、
  endpoint 能力、模型支持和请求特性。
```

### 3.2 数据模型变化

**Group（分组）**新增 `inbound_protocol`（取值见 `glossary.md` §1.1）：

```go
// 兼容迁移：inbound_protocol 为空时按 platform 推断
group.Platform        = "anthropic"           // 现有分组类型/兼容字段
group.InboundProtocol = "anthropic_messages"  // 用户请求协议,glossary §1.1 取值
```

入口层改造要求（取值统一使用 glossary §1.1 长名）：

- `/v1/messages`：不能再只用 `Group.Platform == openai` 决定是否进 OpenAI handler；应读取 `Group.InboundProtocol == anthropic_messages`，再由后续调度决定出站协议。
- `/v1/chat/completions` 与 `/chat/completions`：应以 `Group.InboundProtocol == openai_chat` 为入口判断。
- `/v1/responses`、`/responses`、`/backend-api/codex/responses`：应明确属于 `openai_responses` 入站协议，不能继续混用 group platform。
- `/v1beta/*`：应以 `Group.InboundProtocol == gemini_v1beta` 为入口判断，并保留 Google 格式错误响应。

**Account（账号）**新增 `outbound_protocol`（取值同 glossary §1.1）：

```go
// 兼容迁移：outbound_protocol 为空时按 platform/type 推断
account.Platform         = "openai"        // 厂商/账号类型
account.OutboundProtocol = "openai_chat"   // 实际上游协议,glossary §1.1 取值
```

账号筛选要求：

- native 账号：`platform=anthropic`、`outbound_protocol=anthropic_messages`。
- OpenAI 兼容账号：`platform=openai`、`outbound_protocol=openai_chat` 或 `openai_responses`，按 `/v1/responses` 能力探测结果确定。
- Antigravity 账号：不能只写成一个协议；它现有同时涉及 Anthropic 和 Gemini 兼容，需要按账号类型和 endpoint 能力归档。
- 通用渠道账号：Phase 1 先复用真实协议平台并补充 provider 元信息；只有进入多 endpoint Generic 阶段时，才新增 `platform=generic` 常量、schema/service、前端表单和统计维度。

---

## 4. Protocol Bridge 设计

### 4.1 Bridge 不是新网关入口

Bridge 的职责是“转换执行器”，不是独立替换现有 Gateway/OpenAI/Gemini handler。落地方式应是：

```text
现有 handler 读取请求体
  -> 解析模型、stream、metadata、请求特性
  -> 获取用户/分组/并发槽位
  -> 调度器选择 account + bridge
  -> bridge 构造上游请求与响应转换器
  -> 现有 service 执行转发、failover、usage 记录、账单记录
```

这样可以避免一次性重写并发限制、用户队列、sticky session、错误透传、usage 统计和后台告警。

### 4.2 Bridge 接口草案

原先的 `TranslateResponse([]byte)` 不适合流式响应。Bridge 必须同时支持非流式和流式转换，避免把大 SSE 响应整体 buffer 到内存。

```go
type ProtocolBridge interface {
    ID() string // "anthropic->openai_responses"

    Capabilities() BridgeCapabilities

    Prepare(ctx context.Context, in BridgeInput) (*BridgePlan, error)
}

type BridgeInput struct {
    InboundProtocol  string
    OutboundProtocol string
    Body             []byte
    Model            string
    Stream           bool
    Features         RequestFeatures
    Account          *Account
}

type BridgePlan struct {
    UpstreamEndpoint string
    UpstreamBody     []byte
    ContentType      string
    Headers          http.Header

    // 非流式：读取上游完整响应，转换成入站协议响应。
    ConvertResponse func(ctx context.Context, status int, header http.Header, body []byte) (*BridgeResponse, error)

    // 流式：边读上游边写客户端，不整体 buffer。
    StreamResponse func(ctx context.Context, upstream io.Reader, client http.ResponseWriter) (*BridgeUsage, error)

    UsageExtractor func(status int, header http.Header, body []byte) (*BridgeUsage, error)
    LossyFeatures  []FeatureID
    DroppedFeatures []FeatureID
}
```

Bridge 必须输出：

- 上游请求体。
- 目标 endpoint，例如 `/v1/responses` 或 `/v1/chat/completions`。
- 响应转换器，且 streaming 路径必须是 reader/writer 式。
- usage 提取结果，供现有 usage/billing 记录复用。
- lossy/dropped feature 标记，必要时写回 `X-Bridge-Lossy`、`X-Bridge-Dropped`。

### 4.3 第一批 Bridge

Bridge ID 命名规则与 protocol 取值见 `glossary.md` §1.1；代码定位（`ForwardAsAnthropic` / `ForwardAsResponses` 等）见 `glossary.md` §5。

| Bridge ID | 来源 | 优先级 | 说明 |
|---|---|---:|---|
| `anthropic_messages->openai_responses` | 现有 `ForwardAsAnthropic` | P0 | Phase 3 注册到 Registry，不是重新实现 |
| `openai_responses->anthropic_messages` | 现有 `ForwardAsResponses` | P0 | Phase 3 注册到 Registry；先前版本误标为"待实现"已修正 |
| `anthropic_messages->openai_chat` | 新增（apicompat 已有零件拼装） | P1 | 面向只支持 Chat Completions 的 DeepSeek/Kimi/Qwen 等兼容上游 |
| `openai_chat->anthropic_messages` | 新增（apicompat 已有零件拼装） | P1 | 复用 `chatcompletions_to_responses` + `responses_to_anthropic` |
| `openai_responses->openai_chat` | 新增（apicompat 已直通） | P2 | 同源协议互转，黏合工作量最低 |
| `gemini_v1beta->openai_chat` / `gemini_v1beta->anthropic_messages` | 新增 | P3 | Phase 4 Gemini 入站桥 |
| `openai_chat->gemini_v1beta` / `anthropic_messages->gemini_v1beta` | 新增 | P4 | 按真实客户场景排期，可推迟 |

---

## 5. 请求特性与调度规则

### 5.1 RequestFeatures

Bridge 前必须先嗅探请求实际使用的协议特性：

```go
type RequestFeatures struct {
    Required map[FeatureID]bool
    Optional map[FeatureID]bool
}

type FeatureID string

const (
    FeatureText             FeatureID = "text"
    FeatureVision           FeatureID = "vision"
    FeatureDocument         FeatureID = "document"
    FeatureToolUse          FeatureID = "tool_use"
    FeatureStreaming        FeatureID = "streaming"
    FeatureCacheControl     FeatureID = "cache_control"
    FeatureExtendedThinking FeatureID = "extended_thinking"
    FeatureComputerUse      FeatureID = "computer_use"
    FeatureCitations        FeatureID = "citations"
    FeaturePromptCache      FeatureID = "prompt_cache"
)
```

Anthropic 嗅探规则示例：

| 触发条件 | 特性 |
|---|---|
| `messages[].content[].type == "image"` | `vision` Required |
| `messages[].content[].type == "document"` | `document` Required |
| 任意位置出现 `cache_control` | `cache_control` Required |
| 顶层 `thinking` 字段 | `extended_thinking` Required |
| `tools[]` 含 `computer_*` | `computer_use` Required |
| `tools[]` 非空，非 computer use | `tool_use` Required |
| `stream: true` | `streaming` Required |

嗅探必须只解析一次，并把结果放进 request context，供 sticky、模型路由、候选账号、failover 复用。

### 5.2 BridgeCapabilities

实现层复用现有 `backend/internal/pkg/openai_compat/upstream_capability.go` 的三态枚举与 `accounts.extra` JSON 模式，无需另起一套：

```go
// 模板来自 openai_compat.ResolveResponsesSupport：
//   Unknown / Yes / No 三态 + accounts.extra 持久化 + 探测任务异步填充
type FeatureFit int

const (
    FitUnknown FeatureFit = iota // 等价于 ResponsesSupportUnknown，默认乐观（让调度尝试）
    FitNative                    // 完整兼容
    FitLossy                     // 转换后语义降级但可用
    FitDropped                   // 字段被丢弃但请求仍可走通
    FitRejected                  // 不可走通
)

type BridgeCapabilities struct {
    Levels map[FeatureID]FeatureFit
}
```

实现要点：

- 能力探测沿用 `accounts.extra.openai_responses_supported` 同款模式：新增 `accounts.extra.bridge_capabilities` JSON，由异步探测任务回填，调度路径只读。
- 默认值策略沿用 `ShouldUseResponsesAPI()`：`FitUnknown` 视为乐观（让调度尝试）；只有显式 `FitRejected` 才硬剔除。
- 不再为 BridgeCapabilities 单独建表，避免与 `extra` JSON 双源。

能力矩阵基线（Bridge ID 命名遵循 `glossary.md` §1.1：`<inbound>-><outbound>` 使用 protocol 长名）：

| 特性 | `anthropic_messages->anthropic_messages` | `anthropic_messages->openai_responses` | `anthropic_messages->openai_chat` | `openai_responses->anthropic_messages` |
|---|---|---|---|---|
| text | Native | Native | Native | 待验证 |
| vision | Native | Native/需按模型能力判断 | Native/需按模型能力判断 | 待验证 |
| tool_use | Native | Native/Lossy 需按 Responses 能力判断 | Lossy | 待验证 |
| streaming | Native | Native | Native | 待验证 |
| document | Native | Rejected | Rejected | 待验证 |
| cache_control | Native | Lossy 或 Dropped，取决于 OpenAI Responses 兼容能力 | Dropped | 待验证 |
| extended_thinking | Native | Lossy | Lossy | 待验证 |
| computer_use | Native | Rejected | Rejected | 待验证 |
| citations | Native | Rejected | Rejected | 待验证 |

`cache_control` 不能简单写成“OpenAI 一定无缓存”。当前 OpenAI Responses 兼容链路已有 prompt cache key、digest、continuation 等兼容处理，但它不等价于 Anthropic 原生 `cache_control`。能力判定应按具体上游能力标为 `Native`、`Lossy` 或 `Dropped`。

### 5.3 调度规则

特性感知过滤必须出现在每一个可能选账号的入口：

1. sticky session 命中。
2. 模型路由或 prefix/digest session 命中。
3. 普通候选账号枚举。
4. failover 排除账号后的重选。

通用流程：

```text
Step 1  嗅探请求 -> RequestFeatures
Step 2  根据 inbound_protocol、模型、分组拿候选账号
Step 3  对每个账号推导 outbound_protocol 和 endpoint 能力
Step 4  Resolve bridge(inbound, outbound)
Step 5  用 RequestFeatures + BridgeCapabilities + 模型能力计算 fit
Step 6  过滤 Rejected；按策略处理 Lossy/Dropped
Step 7  在剩余候选中按 sticky、负载、成本、可用性排序
```

sticky session 约束：

- sticky 命中后不能直接使用旧账号。
- 必须重新校验 `inbound_protocol`、`outbound_protocol`、模型支持、endpoint 支持和 `RequestFeatures`。
- 如果不兼容，应清理 sticky 或跳过该账号，进入普通重选。

failover 约束：

- failover 只新增 excluded account，不重新嗅探 body。
- 第二次及后续重选必须使用同一份 `RequestFeatures`。
- 否则第一次因 `document` 排除 OpenAI 兼容账号，第二次 failover 可能误选回有损桥。

默认策略：

```go
type GroupFeaturePolicy struct {
    // strict: 只允许全 Native
    // lossy_ok: 允许 Lossy，不允许 Dropped（默认）
    // best_effort: 允许 Dropped
    Strictness string

    // 默认保护关键特性
    // [document, computer_use, citations]
    // cache_control 按上游 Responses 兼容能力判定，默认不接受 Dropped。
    NativeOnlyFeatures []FeatureID

    // reject: 无兼容路径时直接拒绝（默认）
    // fallback: 仅 best_effort 可用
    FallbackPolicy string
}
```

验收要求：

- Anthropic `document` 请求不能 sticky 到 OpenAI 兼容账号，也不能 failover 到 OpenAI 兼容账号。
- Anthropic `computer_use`、`citations` 默认只走原生或明确声明兼容的路径。
- `cache_control` 默认优先 native/兼容缓存路径；如果只能 Dropped，应被排除，除非用户显式配置 `best_effort`。

---

## 6. Generic Channel 定位

> **fork 第 12 项 lingjing 不属于 Generic Channel**：Phase 5 引入 `platform=generic` 多 endpoint 账号时，lingjing 不参与（详见 `generic-channel-design.md` §10）。lingjing 的异步任务制（Seedance 视频走 HTTP 202 + poll_runner）与 generic 设计的同步 endpoint 选择 + Bridge 转换模型不兼容，应作为独立 platform 持续维护。多 endpoint Generic 不是"万能容器"。计费侧 lingjing **仍接入** Phase 0 P0-7 上游成本快照（见 `upstream-cost-snapshot.md` §4.1），与调度层正交。

Generic Channel 是 Layer 3 的一种账号类型，用于承载当前 `anthropic`、`openai`、`gemini` 三类内置平台之外的通用渠道。它覆盖两类上游：

- **原厂兼容接口**：DeepSeek、豆包等有独立厂商身份，但主要以 OpenAI-compatible 或厂商自定义兼容协议对外提供 API。
- **非原厂聚合平台**：硅基流动、万界方舟等，一个 API Key 后面可能暴露多个厂商、多个协议或多个 endpoint。

通用渠道的细化设计见 `docs/generic-channel-design.md`。Generic Channel 的边界是：它不是“第四种入站协议”，而是当前内置平台之外的厂商/渠道抽象；真正使用什么协议由账号或 endpoint 的 `protocol` 决定。

当前源码没有 `generic` 平台常量。Phase 1 不应直接新增 `platform=generic` 并让 DeepSeek、豆包、硅基流动等 OpenAI-compatible 渠道改走它；否则会绕开现有 OpenAI handler、调度器和 `/v1/responses` 能力探测。Phase 1 应先复用现有协议平台，并补齐以下内容：

- 后端 provider 元信息：区分 `deepseek`、`doubao`、`siliconflow`、`wanjie` 等厂商或聚合渠道。
- 前端表单：在 OpenAI APIKey 账号创建中增加渠道预设和自定义 base_url。
- 模型同步：按 provider 决定远程拉取、手动填写或静态预设，不能假设所有渠道都支持同一 `/models`。
- 渠道统计：usage、error、availability、cost 维度能区分具体厂商、账号和实际协议。
- 多协议聚合平台：Phase 1 先拆成多个协议账号；真正 `platform=generic` 多 endpoint 账号推迟到调度器按 endpoint protocol 筛选之后。

### 6.1 Phase 1 Credentials 结构

OpenAI-compatible 的原厂或聚合渠道先按 OpenAI APIKey 账号保存：

```jsonc
{
  "platform": "openai",
  "type": "apikey",
  "credentials": {
    "api_key": "sk-xxx",
    "base_url": "https://api.deepseek.com",
    "provider": "deepseek"
  },
  "extra": {
    "provider": "deepseek",
    "provider_type": "official_compatible",
    "protocol": "openai_chat",                // 取值见 glossary §1.1
    "models_source": "remote"
  }
}
```

> `extra.provider` 的规范化规则与别名表见 `glossary.md` §1.3；UsageLog 快照字段清单见 `glossary.md` §3.1。本文档不重复定义，避免口径漂移。

聚合平台如果同时提供多个协议 endpoint，Phase 1 先拆成多个账号，每个账号仍归属真实协议平台（`protocol` 取值见 `glossary.md` §1.1）：

| 账号 | platform | provider | protocol |
|---|---|---|---|
| 硅基流动 OpenAI | `openai` | `siliconflow` | `openai_chat` |
| 某聚合 Anthropic | `anthropic` | `vendor_x` | `anthropic_messages` |
| 某聚合 Gemini | `gemini` | `vendor_x` | `gemini_v1beta` |

### 6.2 Phase 5 多 endpoint 结构

> 真正的 `platform=generic` 多 endpoint 账号在 Phase 5 引入（依赖 Phase 2 协议字段、Phase 3 Bridge Registry、scheduler 双桶并存 ≥99% 一致率验证）。Phase 1 不要使用此结构，也不要在前端类型/表单中放出 `generic` 选项。

**数据结构（唯一权威）见 `glossary.md` §4**（采用数组结构，便于 ent 1→N 与稳定 `endpoint_id`）。

每个 endpoint 必须显式声明（字段释义）：

- 协议标识：`protocol` 取值见 `glossary.md` §1.1。
- 厂商/渠道标识：`provider`（取值见 `glossary.md` §1.3 别名表），用于统计、模型同步策略和错误诊断。
- `base_url` 拼接规则，避免 `/v1/v1/...`。
- 鉴权头策略：`Authorization: Bearer`、`x-api-key` 或厂商自定义。
- 模型列表来源：远程拉取、手动配置或继承账号级白名单。
- endpoint 类型：原厂兼容接口（通常单协议）或聚合平台接口（可挂多个 endpoint）。

### 6.3 Phase 1 范围限制

Phase 1 只支持直接 endpoint 匹配：

```text
inbound_protocol == endpoint.protocol -> 直接转发
inbound_protocol != endpoint.protocol -> 不在 Phase 1 自动 Bridge
```

Generic endpoint 可为后续 Bridge 提供候选，但 Phase 1 不应把 N×M 全矩阵一次性做进 Generic Channel。Phase 1 的成功标准拆为两条独立要求：

**Phase 1 调度与测试的成功标准**：DeepSeek/豆包这类原厂兼容渠道和硅基流动/万界方舟这类聚合渠道能用现有协议平台表达；调度链路、测试连接、错误日志能定位到具体厂商（`provider_key`）、账号（`account_id`）、协议（`protocol`），不再以 `platform` 单一字段为唯一定位锚。

**Phase 1 统计的成功标准**：按账号、按厂商的用量与计费汇总不被分组切分。账号同时归属多个分组（`account_groups` m2m，常态而非异常）时，账号维度与厂商维度的总额等于其所有分组下 UsageLog 的逐行求和。具体口径与字段承诺以 `docs/generic-channel-design.md` §5.4 修订版为准。

---

## 7. 分阶段实施路线图（v2，**6 个 Phase（Phase 0–5）**，~14–22 周）

> 工期取决于是否含 Phase 5 完整观察期：不含 ~14 周（Phase 0–4 完成），含 ~22 周（含 4 周 scheduler 双桶并存验证）。详见 `docs/sprint-plan.md` 顶部"总工期估算"与"全局 Sprint 编排"——三处口径必须一致；以 `sprint-plan.md` 顶部为唯一权威。

> 设计原则：每阶段独立可交付、可回滚、可单独发布；避免在同一 schema 迁移窗口里塞多个目标。
> 工程量估算基于 1–2 名熟手全栈开发者。

> 工期、PR 数、Phase 简表见 `glossary.md` §2；UsageLog 字段清单见 `glossary.md` §3；关键代码定位见 `glossary.md` §5。本节仅描述各 Phase **设计意图**，不再重复事实数据。

### Phase 0：计费基础设施先行

目标：让"上游真实成本"和"下游售价"在 UsageLog 一行内可见，毛利率立即可查。**这是 MAAS 商业模式的根问题，与协议层完全解耦，应当最先落地**。

- `usage_log.go` 新增 7 个可空字段（见 `glossary.md` §3.1）。
- 新增 `provider_pricing` ent 实体（上游 Provider 单价表）。
- 新增独立的 `resolveUpstreamCost()`（详见 `docs/upstream-cost-snapshot.md` §3.2），严格两态：命中 `provider_pricing` 返回 `(cost, "provider_table")`，未命中返回 `(nil, "")`。
- **不修改** `account_stats_pricing.go` 现有四级链——它仍负责"客户售价/账号统计估算"，与上游成本快照解耦；上游成本不复用 LiteLLM/自定义规则/默认公式等估算回退。
- 双轨写入落在 UsageLog 装配三处（代码位置见 `glossary.md` §5），**不在 `writeUsageLogBestEffort` 封装内改**。
- 历史数据：`provider` 字段 Phase 0 保持 NULL，Phase 1 联动按 `account.extra.provider` 细颗粒回填；`upstream_total_cost` 与 `pricing_source` 全程 NULL。

### Phase 1：轻量 Generic + provider_key 规范化

目标：按 `docs/generic-channel-design.md` Phase 1（不引 `platform=generic`，只在 OpenAI 平台加 `extra.provider` 元数据 + provider_key 规范化 + 跨 group 聚合视图）。

- `Account.extra.provider` 写入规范 + `normalize_provider` 别名表（见 `glossary.md` §1.3）。
- 新增 `account_repository` 跨 group 聚合查询：禁止以 `group_id` 为强制过滤条件，确保账号/Provider 维度汇总跨 group 一致。
- 前端 `AccountPlatform` 类型补齐 `antigravity`，新增 `ProviderDistributionChart` 与 `AccountDistributionChart`，与 `GroupDistributionChart.vue` 同级。
- 账号创建表单增加 OpenAI 平台的 Provider 预设 + 自定义 base_url。
- **前后端必须同 sprint 推进**，避免类型滞后债务继续累积。

### Phase 2：InboundProtocol / OutboundProtocol 双写兼容层

目标：用"渐进迁移"替换"大爆炸"。新增字段，旧字段保留并由派生函数同步。

- `Group` 新增 `inbound_protocol`、`Account` 新增 `outbound_protocol`（取值见 `glossary.md` §1.1，字段位置见 §3.3，代码 schema 位置见 §5）。
- 新增 `domain/protocol.go` 集中常量与互转函数；现有 `service.PlatformOpenAI` 等保留为 alias + deprecation 注释（依靠 staticcheck/go vet 自定义检查给出编译期警告，CI 报告但不阻断）。
- `backend/internal/server/routes/gateway.go` 7 处分流判断（行号见 `glossary.md` §5）同时检查 `inbound_protocol`，缺失时回退到 `platform`。
- **不动** 现有 `Platform*` 引用（数量见 `glossary.md` §6），让后续阶段按子系统逐步切换。
- 数据库迁移脚本一次性反向回填：从 `platform` 推导 `inbound_protocol` 默认值。
- 一致性约束：写入时若两者都给必须互恰，缺失由另一方推导；启动健康检查扫描所有 group/account。

### Phase 3：Bridge Registry + N×M 矩阵收口

目标：从现有 `ForwardAsAnthropic` / `ForwardAsResponses`（代码位置见 `glossary.md` §5）提炼抽象，补齐缺失桥。

实施顺序：

1. **先做 Registry，不做 interface**：在 `backend/internal/service/protocol_bridge_registry.go` 注册当前两条 Forward 为 `(inbound, outbound) → func` 的 map。
2. **再做 interface**：当桥数量 ≥3 时（即下面新增 `anthropic_messages->openai_chat` 桥后），抽出 `ProtocolBridge.Forward(...)` 接口。**桥数=2 时不要抽**（YAGNI）。
3. **能力矩阵**：复用 `pkg/openai_compat/upstream_capability.go` 模板（见 `glossary.md` §5），在 `accounts.extra.bridge_capabilities` JSON 持久化，由探测任务异步填充。

新增桥（按 apicompat 复用度排序，Bridge ID 取值见 `glossary.md` §1.1）：

- `anthropic_messages->openai_chat`（复用 `anthropic_to_responses` + `responses_to_chatcompletions`，~3 天）
- `openai_chat->anthropic_messages`（复用 `chatcompletions_to_responses` + `responses_to_anthropic`，~3 天）
- `openai_responses->openai_chat`（apicompat 已直通，~1 天黏合）

特性感知调度（来自原 Phase 2）：

- 在 sticky、模型路由、普通候选、failover 重选中统一调用特性过滤。
- 增加调度诊断日志：选中账号、bridge id、lossy/dropped features、被排除账号原因。
- `RequestFeatures` 嗅探仅一次，放入 context。
- 默认策略沿用 `FitUnknown=乐观`（让调度尝试），仅 `FitRejected` 硬剔除。

### Phase 4：Gemini 入站桥

目标：让 `gemini_v1beta` 入站协议接入桥矩阵，下游可走任意账号。

- 新增 `pkg/apicompat/gemini_to_openai.go` + `gemini_to_anthropic.go`（含 `streamGenerateContent` → SSE 流式）。
- `backend/internal/handler/gemini_v1beta_handler.go` 改走 Bridge Registry。
- 单独的 fuzz 测试集覆盖 Gemini ↔ Anthropic / OpenAI 的 function calling schema 映射。

### Phase 5：Generic 多 Endpoint + Scheduler 重构

目标：单个 Generic Account 可挂多个 endpoint（不同 base_url / 不同协议），scheduler 按 endpoint 而非平台分桶。

- 新增 `endpoint` ent 实体；`Account` 1→N `Endpoint`，每个 endpoint 持有 `base_url` / `outbound_protocol` / `capabilities` / `priority` / `health`。数据结构见 `glossary.md` §4，字段见 §3.4。
- `bucketFor`（代码位置见 `glossary.md` §5）**双桶并存**：旧调用方继续走 `(groupID, platform, mode)`，新调用方走 `(groupID, inbound_protocol, mode)` 或 `(groupID, endpoint_id)`。保留 4 周双写埋点对比，确认 ≥99% 一致率后切单桶。
- `sticky_session` key 从 `(api_key, account_id)` 扩到 `(api_key, account_id, endpoint_id)`。
- 引入 `platform=generic` 常量，前端/后端类型同步扩充。

### 阶段并行性与依赖

```text
Phase 0 (计费)        ────────────────────────────────────▶ (基础)
                       │
Phase 1 (Generic 轻量) │  依赖 Phase 0 的 provider 列存在
                       ▼
                  Phase 2 (双写字段) ──▶ Phase 3 (Bridge 收口) ──▶ Phase 4 (Gemini 桥) ──▶ Phase 5 (多 endpoint)
```

- Phase 0 与 Phase 1 在 schema 上正交，可在同一 sprint 推进。
- Phase 2 起串行，每阶段独立 PR、独立验收、独立回滚。

---

## 8. 后续文档关系

Generic Channel 的细化设计见 `docs/generic-channel-design.md`。本文只定义它在入站/出站协议解耦架构中的位置和边界；细化文档覆盖：

- 数据库字段和迁移。
- credentials schema。
- 前端表单。
- 模型同步。
- 账号测试与可用性探测。
- usage/error/availability 统计维度。

---

## 9. 风险与取舍

### 9.1 有损翻译

Bridge 转换不可能 100% 保留语义。调度器必须把损失显式化，而不是默默转发。

| 损失 | 影响 | 缓解 |
|---|---|---|
| Anthropic `document` -> OpenAI | PDF/文件语义无法表达 | `Rejected`，调度器硬剔除 |
| Anthropic `computer_use` -> OpenAI | beta 能力缺失 | `Rejected`，默认拒绝或走 native |
| Anthropic `citations` -> OpenAI | 引用结构缺失 | `Rejected` 或明确 Lossy，默认保护 |
| Anthropic `cache_control` -> OpenAI Responses | 不完全等价 | 按具体 Responses 兼容能力判定 Native/Lossy/Dropped；默认不接受 Dropped |
| Anthropic `thinking` -> OpenAI reasoning | 语义不等价 | `Lossy`，仅 `lossy_ok` 或更宽策略可选 |
| OpenAI SSE -> Anthropic SSE | 事件语义不同 | writer/reader 流式桥，单测覆盖事件序列 |
| 异步任务 poll 期间 UsageLog 行级成本 = NULL（fork 第 12 项 lingjing） | 实时毛利视图覆盖率波动 | poll 期间用 `lingjing_task.status` join 显示"待结算"；poll_runner 完成回填 `upstream_total_cost` + `cost_finalized_at` 后自然消失；BI 按 `cost_finalized_at` 而非 `created_at` 聚合实时毛利 |

### 9.2 调度复杂度

复杂度来源：账号协议、endpoint 能力、Bridge 能力、模型能力、请求特性、sticky、负载、成本、failover。

控制手段：

- `RequestFeatures` 只嗅探一次，放入 context。
- `ScoreFit(capabilities, features, policy)` 做成纯函数。
- sticky 和 failover 只复用同一份特性结果，不重新猜测。
- 诊断日志必须记录排除原因，避免线上“为什么没选便宜账号”不可解释。

### 9.3 不做的事

- 不在 Phase 1 做完整 N×M 自动桥接。
- 不做 UI 配置化自定义 Bridge 规则。
- 不承诺完全无损翻译。
- 不把 Generic Channel 写成"万能平台"；它只是多 endpoint 出站账号类型。
- 不在单个 PR 内一次性替换 49 个 `Platform*` 引用。

---

## 10. 实施验收用例

后续代码实现至少覆盖以下用例：

1. Anthropic 纯文本请求可路由到 OpenAI Responses 兼容账号，并以 Anthropic Messages 格式返回。
2. Anthropic `document` 请求不会 sticky 到 OpenAI 兼容账号，也不会 failover 到 OpenAI 兼容账号。
3. 带 `cache_control` 的 Anthropic 请求在默认策略下优先 native 或等价缓存兼容路径；只能 Dropped 时不选有损桥。
4. sticky session 命中不兼容账号时会跳过或清理 sticky，并重新选路。
5. failover 重选保留同一份 `RequestFeatures`，不会第二次误选有损桥。
6. OpenAI Responses streaming 回包能持续转换为 Anthropic SSE，不整体 buffer。
7. `/v1/messages`、`/v1/chat/completions`、`/v1/responses`、`/responses`、`/backend-api/codex/responses`、`/v1beta/*` 的入口分流均由 `Group.InboundProtocol` 驱动（Phase 2 起；旧 `Group.Platform` 仍作为回退）。
8. 对**命中 `provider_pricing`**（`pricing_source = "provider_table"`）的请求，UsageLog 行内 `upstream_total_cost` 与 `actual_cost` 同时非空；毛利 `actual_cost − upstream_total_cost` 可正可负（负值代表亏损或折扣/免费额度，是有效数据，须进入运营审视而不是验收失败）。未命中行 `upstream_total_cost` 与 `pricing_source` 同时 NULL 是合法状态。（Phase 0 起）
9. 同一 provider 账号被两个 group 引用时，按 Provider 维度聚合显示一行而非两行（Phase 1 起）。

---

## 11. 流式 SSE 回归测试集

桥层最易出 bug 的是 streaming SSE 时序：typing 抖动、token 重复、事件顺序错乱、buffer 不刷新。Phase 3 起强制：

### 11.1 Fixture 形态

- 每个入站协议（Anthropic / ChatCompletions / Responses / Gemini）准备 **≥5 个固定 prompt fixture**，覆盖：
  - 纯文本（短/长）
  - Tool call（含并行调用）
  - 多轮上下文
  - vision 输入
  - cache_control / prompt_cache_key 行为
- 每个 fixture 录制上游真实 SSE 字节流到 `backend/testdata/sse/<protocol>/<fixture>.sse`。
- 录制工具：`script/record_upstream_sse.sh`（新建），通过现有账号调用真实上游一次。

### 11.2 Record-Replay 测试形态

- 测试不走真实网络，从 fixture 重放 SSE 字节流到桥层 reader。
- 断言：
  - 写到 client 的 SSE 事件序列与同协议 native 录制完全等价（事件顺序、event 名、data 字段、空行间隔）。
  - 中间不出现整体 buffer（验证 reader/writer 流式，按 chunk 推进）。
  - usage 提取结果与 fixture 记录的 token 数对齐。

### 11.3 CI 集成

- `make test-bridges` 跑全部 fixture × 全部桥组合（含同协议透传作为基线）。
- CI 在 PR 阶段必跑（`backend-ci.yml` 中新增 job）。
- 录制更新需要单独审批：fixture 更改的 PR 自动打 `requires-bridge-review` 标签。
