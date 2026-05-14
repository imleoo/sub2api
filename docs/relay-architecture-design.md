# API 中转商架构设计方案

> 状态：草稿 | 日期：2026-05-14

---

## 1. 核心问题：N×M 协议矩阵

一个 AI API 中转商的本质是：

```
用户使用任意一种协议 →【系统】→ 向任意一家上游厂商转发
```

用户侧（下游）有 N 种协议格式，上游厂商侧（上游）有 M 种接入方式。当前系统的根本局限是：**把"入站协议"和"出站协议"绑定在同一个 `platform` 字段上**，导致只能走对角线（同进同出），无法跨格。

```
                    上游厂商支持的协议
                 OpenAI  Anthropic  Gemini  自定义
用          ──────────────────────────────────────
站  OpenAI  │   ✅已有    ✅已有*    ❌       ❌
协  Anthropic│   ❌缺失    ✅已有     ❌       ❌
议  Gemini  │   ❌缺失    ❌缺失    ✅已有    ❌

* ForwardAsAnthropic 已在 openai_gateway_messages.go 实现，但未系统化暴露
```

这个矩阵中：
- **✅ 对角线**：同协议透传，当前系统已支持
- **✅ 已有但隐式**：OpenAI 入站 → Anthropic 出站，存在于 antigravity 逻辑中
- **❌ 缺失**：Anthropic 入站 → OpenAI 出站（商业价值最高，用户买 Claude 账号，系统用便宜的 OpenAI 兼容厂商兜底）
- **❌ 缺失**：跨格的 Gemini 组合

---

## 2. 当前架构的隐式设计（现状）

当前 `platform` 字段同时承担了两个职责：

```
platform = "anthropic"
  含义1: 用户可以向这个分组发送 Anthropic 格式请求（/v1/messages）
  含义2: 系统向上游发送 Anthropic 格式请求
  → 两个含义强绑定，无法解耦
```

唯一的例外是 `antigravity`：它通过特殊逻辑同时接受 OpenAI 和 Anthropic 格式入站，但这是硬编码的特例，无法泛化。

**现有协议转换点**（分散在代码各处，未系统化）：

| 文件 | 转换方向 | 备注 |
|------|---------|------|
| `openai_gateway_messages.go:ForwardAsAnthropic` | OpenAI入 → Anthropic出 | 用于 antigravity 平台 |
| `openai_gateway_chat_completions_raw.go` | OpenAI入 → OpenAI出（透传） | 用于三方 compat 厂商 |
| `gemini_messages_compat_service.go` | Anthropic入 → Gemini出 | Gemini 分组特有 |
| `antigravity_gateway_service.go:ForwardUpstream` | Anthropic入 → Anthropic出（透传） | upstream 类型 |

---

## 3. 提议的新架构：三层分离

### 核心思想

将现在隐式绑定的 `platform` 拆分为两个独立维度：

```
分组（Group）→ 定义"入站协议"（用户用什么格式请求）
账号（Account）→ 定义"出站协议"（系统用什么格式打上游）
协议桥（Protocol Bridge）→ 当两者不同时，负责转换
```

### 3.1 三层架构图

```
┌─────────────────────────────────────────────────────────────┐
│                  Layer 1：入站协议层（用户侧）                │
│                                                             │
│  分组 A              分组 B              分组 C             │
│  inbound=openai      inbound=anthropic   inbound=gemini     │
│  （用户发OpenAI格式）  （用户发Claude格式）  （用户发Gemini格式）│
└─────────────────────────┬───────────────────────────────────┘
                          │ 用户请求（含协议标记）
                          ▼
┌─────────────────────────────────────────────────────────────┐
│              Layer 2：协议桥层（Protocol Bridge）            │
│                                                             │
│  入站协议 = 出站协议 → 透传，无转换开销                       │
│  入站协议 ≠ 出站协议 → 查找并调用注册的 Bridge 函数           │
│                                                             │
│  已注册桥：                                                  │
│  openai→anthropic  ✅（已有代码，需正式注册）                 │
│  anthropic→openai  ⭐（最高商业价值，新增）                   │
│  openai→openai     ✅（透传）                                │
│  anthropic→anthropic ✅（透传）                              │
│  gemini→gemini     ✅（透传）                                │
└─────────────────────────┬───────────────────────────────────┘
                          │ 标准化/转换后的请求
                          ▼
┌─────────────────────────────────────────────────────────────┐
│              Layer 3：出站账号层（上游厂商侧）                │
│                                                             │
│  账号类型 A          账号类型 B          账号类型 C           │
│  outbound=openai     outbound=anthropic  outbound=generic    │
│  （OpenAI/DeepSeek   （Anthropic/        （多协议聚合商       │
│   /Kimi/Qwen等）      万界方舟Anthropic   端点映射）          │
│                       端点等）                               │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 数据模型变化

**Group（分组）** - 新增字段 `inbound_protocol`：

```go
// 现在：platform 同时代表入站协议和出站协议
group.Platform = "anthropic"

// 新设计：明确区分（向后兼容：inbound_protocol 默认等于 platform）
group.Platform         = "anthropic"        // 保留，决定分组"类型"
group.InboundProtocol  = "anthropic"        // 用户入站格式（新增，初始等于 platform）
```

**Account（账号）** - 新增字段 `outbound_protocol`：

```go
// 现在：platform 隐含了出站协议
account.Platform = "openai"   // 意味着出站是 OpenAI 格式

// 新设计：出站协议显式标注
account.Platform         = "openai"         // 厂商类型/身份（保留）
account.OutboundProtocol = "openai"         // 实际出站格式（新增，初始等于对应格式）

// generic 账号的特殊表达：
account.Platform         = "generic"
account.OutboundProtocol = ""               // 由 endpoints 映射动态决定
// credentials.endpoints = { "openai": "...", "anthropic": "..." }
```

**迁移兼容性**：现有账号 `outbound_protocol` 为空时，按原 platform 推断，完全向后兼容。

---

## 4. 协议桥（Protocol Bridge）设计

### 4.1 Bridge 接口

```go
// ProtocolBridge 负责在两种协议格式之间转换请求和响应
type ProtocolBridge interface {
    // ID 返回桥的唯一标识，格式："inbound→outbound"，如 "anthropic→openai"
    ID() string

    // Capabilities 声明该桥对各特性的支持等级，供调度器做特性感知路由
    Capabilities() BridgeCapabilities

    // TranslateRequest 将入站格式的请求体转换为出站格式
    // 返回转换后的请求体和内容类型
    TranslateRequest(ctx context.Context, inboundBody []byte, meta BridgeMeta) (outboundBody []byte, err error)

    // TranslateResponse 将出站格式的响应体转换回入站格式（返回给用户）
    // stream=true 时按行处理 SSE 事件
    TranslateResponse(ctx context.Context, outboundBody []byte, meta BridgeMeta, stream bool) (inboundBody []byte, err error)
}

type BridgeMeta struct {
    Model         string
    Stream        bool
    InboundProto  string  // "openai" | "anthropic" | "gemini"
    OutboundProto string
    Features      RequestFeatures  // 由请求嗅探器填充，桥可据此决定转换策略
}

// FeatureID 标识一种入站协议级别的能力点
type FeatureID string

const (
    FeatureText             FeatureID = "text"
    FeatureVision           FeatureID = "vision"            // 图片输入
    FeatureDocument         FeatureID = "document"          // Anthropic document content block（PDF/文件）
    FeatureToolUse          FeatureID = "tool_use"
    FeatureStreaming        FeatureID = "streaming"
    FeatureCacheControl     FeatureID = "cache_control"     // Anthropic prompt caching
    FeatureExtendedThinking FeatureID = "extended_thinking" // Anthropic thinking / OpenAI reasoning
    FeatureComputerUse      FeatureID = "computer_use"      // Anthropic beta
    FeatureCitations        FeatureID = "citations"
    FeaturePromptCache      FeatureID = "prompt_cache"      // OpenAI 隐式缓存
)

// FeatureFit 表示桥/账号对单个特性的支持等级
type FeatureFit int

const (
    FitNative    FeatureFit = iota // 完全等价，零损
    FitLossy                       // 语义近似（thinking→reasoning_effort 之类）
    FitDropped                     // 静默丢弃（cache_control→无）
    FitRejected                    // 无法表达，必须拒绝（computer_use→OpenAI）
)

// BridgeCapabilities 是 Bridge 对特性的等级声明
// 未列出的特性按 FitNative 处理（默认透传到 RawBody）
type BridgeCapabilities struct {
    Levels map[FeatureID]FeatureFit
}
```

### 4.2 Bridge 注册表

```go
// BridgeRegistry 全局注册表，启动时注册所有已实现的桥
type BridgeRegistry struct {
    bridges map[string]ProtocolBridge  // key = "inbound→outbound"
}

func (r *BridgeRegistry) Register(b ProtocolBridge) { ... }

// Resolve 返回对应的桥，inbound==outbound 时返回 nil（透传）
func (r *BridgeRegistry) Resolve(inbound, outbound string) (ProtocolBridge, bool) {
    if inbound == outbound {
        return nil, true  // nil bridge = 透传，true = 支持
    }
    b, ok := r.bridges[inbound+"→"+outbound]
    return b, ok
}
```

### 4.3 实现优先级

按商业价值排序：

| 桥标识 | 商业价值 | 实现难度 | 说明 |
|--------|---------|---------|------|
| `openai→anthropic` | ★★★★ | 低 | 已有代码，正式注册即可 |
| `anthropic→openai` | ★★★★★ | 中 | **最高价值**：用户购买 Claude API 配额，用 DeepSeek/Qwen 兜底，成本大幅降低 |
| `openai→openai` | — | — | 透传，无需 Bridge |
| `anthropic→anthropic` | — | — | 透传，无需 Bridge |
| `gemini→openai` | ★★★ | 中 | Gemini 用户透明切换到 OpenAI 兼容后端 |
| `openai→gemini` | ★★ | 中 | 较少见 |
| `gemini→anthropic` | ★ | 高 | 场景罕见 |
| `anthropic→gemini` | ★ | 高 | 场景罕见 |

### 4.4 anthropic→openai 桥（最高优先级实现）

这是商业价值最高的桥。用户用 Claude 客户端，系统透明地用 DeepSeek/Qwen/万界方舟 OpenAI 端点完成请求。

**请求转换（Anthropic Messages → OpenAI Chat Completions）：**

```
Anthropic 请求                    OpenAI 请求
─────────────────────────────────────────────────────
POST /v1/messages              → POST /v1/chat/completions
{ model, messages,             → { model, messages,
  system, max_tokens,               max_tokens,
  stream, tools }                   stream, tools,
                                    system (→ messages[0]) }

messages[].role:
  user/assistant              → user/assistant（不变）
  system（独立字段）           → messages 数组第一条 system role

content[].type:
  text                        → content string 或 text part
  image（base64/url）         → image_url part
  tool_use                    → tool_call
  tool_result                 → tool 角色消息

cache_control               → 丢弃（OpenAI 无缓存控制）
thinking                    → reasoning_effort（尽力映射）
```

**响应转换（OpenAI Chat Completions → Anthropic Messages）：**

```
OpenAI 响应                       Anthropic 响应
─────────────────────────────────────────────────────
choices[0].message.content    → content[0].text
choices[0].finish_reason:
  stop                        → stop_reason: "end_turn"
  tool_calls                  → stop_reason: "tool_use"
  length                      → stop_reason: "max_tokens"

usage.prompt_tokens           → usage.input_tokens
usage.completion_tokens       → usage.output_tokens

流式（SSE）：
data: {...}                   → event: content_block_delta / message_delta
[DONE]                        → event: message_stop
```

### 4.5 Bridge 能力矩阵（特性级支持声明）

调度器据此做"特性感知路由"——只有协议匹配是不够的，还要确保**请求里实际用到的特性**在出口桥上不会被静默丢失或拒绝。

| 特性 \ Bridge | `anthropic→anthropic` | `anthropic→openai` | `openai→anthropic` | `openai→openai` |
|---|---|---|---|---|
| text                | Native | Native | Native | Native |
| vision (image)      | Native | Native | Native | Native |
| tool_use            | Native | Native | Native | Native |
| streaming           | Native | Native | Native | Native |
| **document**        | Native | **Rejected** | — | — |
| **cache_control**   | Native | **Dropped** | — | — |
| extended_thinking   | Native | Lossy (→ reasoning_effort) | Lossy | Native |
| **computer_use**    | Native | **Rejected** | — | — |
| citations           | Native | Rejected | — | — |
| prompt_cache (隐式) | — | — | Native | Native |

**Rejected 是硬性拒绝信号**：调度器看到候选桥对请求里的某个特性为 Rejected 时，必须把这个账号排除——否则就会让 `document` 请求落到一个根本无法表达 PDF 输入的后端上。

**Dropped 是软性降级信号**：用户允许"为了成本牺牲缓存"时可以接受；不允许时同样要排除。由分组策略 `feature_strictness` 决定（见 5.2）。

### 4.6 请求特性嗅探（Request Feature Sniffing）

Bridge 之前必须先扫描请求体，得出请求实际依赖的特性集合：

```go
type RequestFeatures struct {
    Required map[FeatureID]bool  // 用户显式使用了，丢失会破坏语义
    Optional map[FeatureID]bool  // 检测到但用户可能不强依赖（如隐式缓存）
}

// SniffAnthropic 解析 Anthropic Messages 请求，提取它依赖了哪些特性
func SniffAnthropic(body []byte) (RequestFeatures, error)
// SniffOpenAI 解析 OpenAI Chat Completions 请求
func SniffOpenAI(body []byte) (RequestFeatures, error)
```

**嗅探规则示例（Anthropic）：**

| 触发条件 | 标记特性 |
|---|---|
| `messages[].content[].type == "image"` | vision (Required) |
| `messages[].content[].type == "document"` | document (Required) |
| 任意位置出现 `cache_control: {...}` | cache_control (Required) |
| 顶层 `thinking` 字段 | extended_thinking (Required) |
| `tools[]` 非空，含 `type: "computer_*"` | computer_use (Required) |
| `stream: true` | streaming (Required) |
| `tools[]` 非空（非 computer_*） | tool_use (Required) |

嗅探必须在不消耗 body（不破坏后续转发）的前提下完成——做一次 `json.Unmarshal` 到轻量结构，或用流式 decoder 提取关键字段。

---

## 5. Generic Channel（通用渠道）在新架构中的定位

Generic channel 是 Layer 3（出站账号层）的一种账号类型，专门用于**多协议聚合商**（如万界方舟、硅基流动聚合版）。

```
一个 generic 账号 = 一个 API Key + 多个协议端点映射
```

### 5.1 Generic 账号与协议桥的协作

```
用户（anthropic 分组）
  ↓ 发 Anthropic 格式请求
Layer 2 Bridge 解析：
  inbound=anthropic, account.endpoints={"openai": "...", "anthropic": "..."}
  → 账号支持 anthropic 协议 → 直接透传到 anthropic 端点，无需 Bridge
  → 账号只支持 openai 协议  → 调用 anthropic→openai Bridge，再转发
  → 账号无任何协议           → 返回 501
```

### 5.2 调度器增强（特性感知路由）

旧设计只在**协议层**做选择，会导致带 `document`/`cache_control`/`computer_use` 的请求被静默路由到无法表达这些特性的桥上。新调度器做四件事：

```
Step 1  嗅探请求 → RequestFeatures
Step 2  枚举分组内候选账号
Step 3  对每个账号计算：
          bridge = Resolve(inbound, account.outbound)
          fitMap = bridge.Capabilities() 在 RequestFeatures 上的最严等级
          score  = ScoreFit(fitMap, group.policy)
Step 4  按 score 降序，同分再按 (Sticky Session > 负载 > 成本) 兜底
```

**评分规则（默认）：**

| 最严 Fit                              | Score                | 说明 |
|--------------------------------------|----------------------|------|
| 全 Native                             | 100                  | 同协议透传，或所有用到的特性桥都完整支持 |
| 含 Lossy                              | 70                   | 语义近似（如 thinking→reasoning_effort）|
| 含 Dropped 且 `feature_strictness` 允许 | 40                   | 用户接受"为成本牺牲缓存" |
| 含 Dropped 且 `feature_strictness=strict` | 0（剔除）          | 关键特性会丢，硬拒 |
| 含 Rejected                           | 0（剔除）            | 任何策略下都不可用 |

**分组级策略字段（新增到 Group 表）：**

```go
type GroupFeaturePolicy struct {
    // strict       - 只允许 Score=100（全 Native）
    // lossy_ok     - 允许 Lossy，不允许 Dropped（默认）
    // best_effort  - 允许 Dropped（极致降本，用户自担风险）
    Strictness string

    // 当请求出现这些"关键特性"时，强制走 Native 账号；
    // 即使其它账号成本更低，也必须降级或排队等待 Native 账号。
    // 默认 = [document, cache_control, computer_use, citations]
    NativeOnlyFeatures []FeatureID

    // 全部 Score=0 时的兜底行为：
    // reject (默认) - 直接 400/501 给客户端
    // fallback     - 即便丢失关键特性也尝试转发（仅 best_effort 模式）
    FallbackPolicy string
}
```

**核心保证**：只要分组内存在 `outbound=anthropic` 的账号（如 Anthropic 官方、antigravity、generic 的 anthropic 端点），带 `document`/`cache_control`/`computer_use` 的请求一定优先落到这些账号上——不会因为有更便宜的 OpenAI 兼容账号在场就被误路由。

### 5.3 Generic 账号的 Credentials 结构（更新）

```jsonc
{
  "api_key": "wjark-xxx",

  // 协议→base_url 映射，key 对应出站协议标识
  "endpoints": {
    "openai":    "https://maas-openapi.wanjiedata.com/api",
    "anthropic": "https://maas-openapi.wanjiedata.com/api/anthropic"
    // 未填的协议：先看有无 Bridge 可用，再看能否透传
  }
}
```

### 5.4 特性感知路由典型场景

设分组为 anthropic 入站，组内候选账号：
- `A1`：Anthropic 官方（outbound=anthropic，贵）
- `A2`：万界方舟 generic.anthropic（outbound=anthropic，中）
- `A3`：DeepSeek OpenAI 兼容（outbound=openai，便宜）

| 用户请求特点 | Score 计算 | 选中账号 | 说明 |
|---|---|---|---|
| 纯文本对话 | A1=100, A2=100, A3=100 | A3 | 同分按成本兜底，命中最便宜 |
| 带 `cache_control` 长 prompt | A1=100, A2=100, A3=40 (Dropped) | A2 | 默认 `lossy_ok` 排除 A3；A1/A2 同分按成本选 A2 |
| 带 `document` (PDF) | A1=100, A2=100, A3=0 (Rejected) | A1 或 A2 | A3 硬拒，**自动回到 anthropic 原生** |
| 带 `computer_use` beta | A1=100, A2=0 (假设 generic 端点不支持 beta), A3=0 | A1 | 只能走官方 |
| 带 `thinking` | A1=100, A2=100, A3=70 (Lossy) | A1 或 A2 | 默认策略下 A3 不被选中 |
| 纯文本 + `strict` 策略 + 入站 anthropic | A1=100, A2=100, A3=0 (因桥不是 Native) | A1 或 A2 | 严格模式禁用一切跨协议桥 |

**这就回答了"document 模式能否自动切到 anthropic 官方"的问题**：能，且不仅 document——所有 Rejected/Dropped 级特性都按同一套机制自动避开有损桥。

---

## 6. 账号类型矩阵（完整）

| platform | outbound_protocol | 典型厂商 | 备注 |
|---------|------------------|---------|------|
| `anthropic` | `anthropic` | Anthropic 官方 | 现有，不变 |
| `openai` | `openai` | OpenAI 官方、DeepSeek、Kimi、Qwen | 现有，不变 |
| `gemini` | `gemini` | Google Gemini | 现有，不变 |
| `antigravity` | `anthropic` | Antigravity | 现有，不变 |
| `bedrock` | `bedrock` | AWS Bedrock | 现有，不变 |
| `generic` | _由 endpoints 动态决定_ | 万界方舟、硅基流动聚合版 | **新增** |

---

## 7. 分阶段实施路线图

### Phase 1：Generic Channel（当前在做，约 5 天）

目标：支持多协议聚合商接入，渠道商维度统计

- 新增 `generic` platform
- `credentials.endpoints` 映射表
- `generic_gateway_service.go` 按入站端点路由到对应 base_url
- 模型列表同步（`/v1/models`，作为白名单）
- 前端表单：动态协议端点配置

**范围限制**：Phase 1 的 generic 账号只处理**入站协议 == endpoints 中已配置协议**的情况，不涉及 Bridge。

### Phase 2：协议桥 + 特性感知路由（约 5 天）

目标：让现有隐式转换代码系统化，并把"特性级路由"作为第一公民

- 抽象 `ProtocolBridge` 接口，含 `Capabilities()`
- 实现请求嗅探器 `SniffAnthropic` / `SniffOpenAI`
- 正式注册 `openai→anthropic` 桥（整理现有 `ForwardAsAnthropic` 代码）
- 调度器升级为四步流程（嗅探 → 候选 → 评分 → 选号）
- Group 表新增 `feature_policy`（strictness / native_only_features / fallback_policy）
- 监控指标：每个特性命中 Native / Lossy / Dropped 的请求量与 P95 时延

**验收基线**：带 `document` 的 anthropic 请求一定不会被路由到 openai 兼容账号；带 `cache_control` 的请求在默认策略下也不会，除非用户显式切到 `best_effort`。

### Phase 3：anthropic→openai 桥（约 5 天）

目标：用户购买 Anthropic 格式配额，系统用 OpenAI-compat 厂商兜底

- 实现 `AnthropicToOpenAIBridge`（请求转换 + 响应转换 + SSE 流转换）
- 声明该桥的 `Capabilities`：document/cache_control/computer_use = Rejected/Dropped
- 有损转换说明文档（Header `X-Bridge-Lossy: cache_control,thinking` 回写给客户端）
- 分组设置：允许 anthropic 分组调度到 openai 出站账号
- 成本优化：同等能力优先选最低成本账号（在 Score 相等时生效）

### Phase 4：完善矩阵 + 智能路由（按需，约 1 周）

目标：接近完整的 N×M 矩阵，成本感知的智能调度

- `gemini→openai`、`openai→gemini` 等剩余桥
- 账号级成本配置（token 单价）
- 调度器：在满足 SLA 的前提下，选择综合成本最低路径（含 Bridge 开销估算）
- 监控：桥转换成功率、转换耗时

---

## 8. 与现有 Generic Channel 设计文档的关系

`docs/generic-channel-design.md` 描述的是 Phase 1 的具体实现。

本文档是更高层次的架构演进规划，Phase 1 的 generic channel 设计与本架构完全兼容：

- generic 账号的 `endpoints` 映射 = Layer 3 的多出站协议声明
- Phase 1 不引入 Bridge，调度器只匹配"入站协议在 endpoints 中有直接配置"的账号
- Phase 2 起，generic 账号可以在 Bridge 的帮助下服务更多入站协议组合

---

## 9. 风险与取舍

### 9.1 有损翻译

Bridge 转换是有损的，不可能 100% 保留语义。下表的"缓解"列已落到 Capabilities + 调度器评分上：

| 损失 | 影响 | 形式化缓解 |
|------|------|------|
| Anthropic `cache_control` → OpenAI 无缓存 | prompt caching 失效，成本上升 | `anthropic→openai` 声明 `cache_control=Dropped`；默认 `lossy_ok` 策略下排除该桥，强制走 anthropic 出站账号 |
| Anthropic `document` (PDF/文件) | 无法在 OpenAI 协议表达 | 声明 `document=Rejected`，调度器硬剔除；分组无 native 账号时返回 422 而非静默失败 |
| Anthropic `computer_use` beta | OpenAI 无对应能力 | 声明 `computer_use=Rejected`，行为同上 |
| Anthropic `thinking` → `reasoning_effort` | 语义不等价 | 声明为 Lossy=70 分，仅在 lossy_ok 策略下作为次选 |
| OpenAI tool_calls 格式差异 | 工具调用可能失败 | 桥单测覆盖主流工具场景；失败时上报 `bridge_translate_error{from,to,feature}` |
| 流式 SSE 事件格式不同 | 客户端可能解析失败 | streaming 标记为 Native（两侧都必须实现），桥单测覆盖 |
| 响应头丢失 lossy 信号 | 客户端不知道发生了降级 | 出口响应注入 `X-Bridge-Lossy: <features>` 头 |

### 9.2 调度复杂度

引入 Bridge + 特性感知后，调度器需要考虑：账号出站协议 × Bridge 能力 × 请求特性 × 模型支持性 × 负载 × 成本。

控制复杂度的两个手段：
1. **特性嗅探只解一次**：进入调度器前完成 `RequestFeatures` 计算并缓存到 ctx
2. **评分函数纯函数**：`(BridgeCapabilities, RequestFeatures, Policy) → int`，便于单测和回归

### 9.3 不做的事

- **自定义 Bridge 规则（UI 配置化）**：过于复杂，维护负担高，且大多数协议差异需要代码级处理
- **完全无损翻译**：物理上不可能，AI API 各有私有特性
- **GraphQL / 完全私有协议**：需要专门的平台包，case by case 开发，不在通用框架内
