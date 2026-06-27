# 账号级协议适配（Bedrock 协议修正 / Kiro 兼容）— 设计方案

> 文档状态：草稿 · 2026-06-27
> **两端都是标准 `/v1/messages`，不存在协议转换、不新增任何 URL/路径**。
>
> **一句话**：客户端始终走标准 `/v1/messages`、上游也走标准协议；网关在**选到账号之后**，按账号 flag 对**请求做能力检查**（放行/报错/兜底路由）、对**响应做针对性修正**。这是与 Kiro 兼容模式（`response_masking`，`account.go:1443`）同一类的**账号级**机制，**不碰协议、不加路由、不重构热路径**。

---

## 1. 背景与两个核心需求

### 1.1 共同前提
- 入站：客户端只用标准 `/v1/messages`（Anthropic 协议），**不新增任何 URL / 路径**。
- 出站：走**现有转发路径**，按账号类型分流——**通用 Anthropic 上游**（`handleStreamingResponse:6714` / `handleNonStreamingResponse:7339`）、APIKey 直通（`*AnthropicAPIKeyPassthrough` :4738/:5120），AWS Bedrock 账号才走 `forwardBedrock:5208`。本方案**不假定**这些账号是 Bedrock 类型；响应钩子按账号实际所走路径接入（见 §3、§5）。
- 触发：**账号级 flag**（`accounts.extra.*`），默认关闭、opt-in，与 `response_masking` 同款承载。
- 选号、failover、计费、并发等链路**完全复用**现有 `/v1/messages → Messages → Forward` 流程，不改动。

### 1.2 需求 A — Bedrock 协议修正账号
- 客户端 `/v1/messages` → Anthropic 分组 → 选到打了「bedrock 协议修正」flag 的账号。
- **请求**：原样转发；但若请求用到了**上游 bedrock 不支持的 Anthropic 能力**，需**返回错误给客户端**。
- **响应**：按 **bedrock 协议**对响应做修正后返回客户端。

### 1.3 需求 B — Kiro 账号（已部分实现，需补全）
- 客户端 `/v1/messages` → 选到上游是 Kiro 的账号。
- **响应**：修正为**标准 Anthropic**（Kiro 不完全具备 Anthropic 标准协议能力）。
- **请求**：判断能力；若用到了 Kiro 不支持的能力（如 Anthropic `documents`），把请求**路由到兜底账号 / 分组**处理。

### 1.4 两个需求 = 同一机制，按 flag 走不同修改
「报错给客户端」与「兜底路由」本就是**请求侧能力检查**的两种结果分支；「按 bedrock 修正」与「修正为 Anthropic 标准」是**响应侧修正**的两种实现。因此抽象为**一个机制、两类钩子、按账号 flag 取不同适配器**。

| 维度 | 需求 A（bedrock 协议修正） | 需求 B（kiro 兼容） |
|------|--------------------------|--------------------|
| 请求侧·遇上游不支持的能力 | **报错返回客户端** | **改路由到兜底账号/分组** |
| 响应侧·修正方向 | 按 **bedrock 协议** 修正 | 修正为 **Anthropic 标准** |
| 账号 flag | `accounts.extra.<待定>`（§9.4） | 复用/扩展 `response_masking`（§9.4） |

---

## 2. 核心机制：账号级协议适配器（Account Protocol Adapter）

选到账号后，按账号 flag 取一个 adapter；adapter 暴露两个钩子，分别挂在「请求侧（选号后/转发前）」和「响应侧（写出前）」。

```go
// 概念抽象（落点与命名在 §5 细化；此处先定语义）
type AccountProtocolAdapter interface {
    // 请求侧：检查上游能力。返回三选一：
    //   Pass            放行，正常转发
    //   Reject(err)     直接给客户端返回错误（需求 A）
    //   Reroute(target) 改路由到兜底账号/分组（需求 B）
    InspectRequest(parsed *ParsedRequest) RequestAction

    // 响应侧：把上游响应修正为目标形态（覆盖非流式 + 流式）。
    // 非流式：拿到完整响应体 → 修正 → 写出
    // 流式：逐事件 → 修正 → 写出（不缓冲、不破坏 Size() 语义，见 §6）
    CorrectNonStreamResponse(body []byte) []byte
    CorrectStreamEvent(eventType string, data []byte) (outEventType string, outData []byte, drop bool)
}

type RequestAction struct {
    Kind   ActionKind // Pass | Reject | Reroute
    Err    error      // Reject 时
    Target FallbackTarget // Reroute 时（账号或分组，§4）
}
```

按 flag 选 adapter：
```
account.flag == bedrock 协议修正  → BedrockFixAdapter
account.flag == kiro              → KiroCompatAdapter
否则                              → nil（不适配，链路完全不变）
```

> 关键：adapter 为 nil 时（绝大多数账号）整条链路**零行为变化**——与现有 `response_masking` 默认关闭一致。

---

## 3. 数据流与挂载点（基于已校验的真实代码）

```
客户端 ──POST /v1/messages（标准 Anthropic，无新路由）──▶
  internal/handler/gateway_handler.go  Messages(:116)
    解析 native → 选号 failover 循环（:530-885，单组内换号）
      SelectAccountWithLoadAwareness(:1171)
        │
        ▼ service.Forward(ctx, c, account, parsed)  (gateway_service.go:3702)
        ├─ ★选 adapter：adapter := pickAdapter(account)；adapter!=nil 时 c.Set("account_adapter", adapter)
        ├─ ★请求侧钩子：action := adapter.InspectRequest(parsed)
        │     Pass    → 继续
        │     Reject  → 返回错误（同 masking 早返回写法，:3714 风格）→ 客户端拿到错误
        │     Reroute → 触发兜底路由（§4）
        ├─ webSearch 模拟(:3709) / response_masking(:3714) / bedrock 分支(:3747) … 维持
        ▼
      上游转发（按账号类型分流，与 response_masking 判断点完全重合）：
        ├─ 非流式响应处理器（择一，取决于账号类型）：
        │     通用 handleNonStreamingResponse(:7339)  ｜ APIKey 直通 *Passthrough(:5120) ｜ Bedrock handleBedrockNonStreamingResponse(:5541)
        │     usage 先从 native 解析 → ★ if adapter: body = adapter.CorrectNonStreamResponse(body) → c.Data
        └─ 流式响应处理器（择一）：
              通用 handleStreamingResponse(:6714) ｜ APIKey 直通 *Passthrough(:4738) ｜ Bedrock handleBedrockStreamingResponse(bedrock_stream.go:25)
              usage 先解析 → ★ if adapter: (et,d,drop)=adapter.CorrectStreamEvent(...) → 写真实 w
  ◀── 修正后的标准/目标响应 ──
```

挂载点全部已实读校验：
- `Forward(ctx, c, account, parsed)` 签名含 `c/account/parsed`；`response_masking` gate 在 `:3714`；`forwardBedrock` 分支仅在 `account.IsBedrock()` 时（`:3748`），**非本方案账号的默认路径**。
- **响应钩子 = response_masking 的同一组判断点**，覆盖全部上游类型（type-agnostic）：
  - 通用：`handleStreamingResponse:6972` / `handleNonStreamingResponse:7407`
  - APIKey 直通：`handleStreamingResponseAnthropicAPIKeyPassthrough:4847` / `handleNonStreamingResponseAnthropicAPIKeyPassthrough:5150`
  - Bedrock：`handleBedrockStreamingResponse` / `handleBedrockNonStreamingResponse:5541`
- 各处 **usage 都在写出前从 native 解析**，修正不影响计费；流式写真实 `w`（=`c.Writer`），`Size()` 正常增长，failover「已写字节禁止换号」守卫不破。
- 账号 flag 模式对齐 `IsResponseMaskingEnabled()`（`account.go:1443`，读 `accounts.extra`，默认 false）。

---

## 4. 请求侧：能力检查与兜底路由（唯一的"新机制"）

`InspectRequest` 三种结果：

### 4.1 Pass / Reject
- **Pass**：上游支持，正常转发。
- **Reject**（需求 A 主用）：上游不支持请求用到的能力 → 构造错误，按 `response_masking` 早返回的同款写法直接回客户端（标准 Anthropic 错误体）。不触发 failover（这是确定性的能力缺失，不是上游抖动）。

### 4.2 Reroute（需求 B 主用）— 兜底路由
请求用到 Kiro 不支持的能力 → 不让 Kiro 账号处理，改投兜底。两种粒度：

- **同组排除换号**：把当前账号加入 failover 的 `excludedIDs`、`continue` 循环选同组另一账号。**这条复用现有 failover 循环，零新增**。前提：同组内有能处理该能力的非 Kiro 账号。
- **跨分组兜底**（兜底是"别的分组"时）：现有 failover 循环**只在单组内换号**（`gateway_handler.go:530-885`），跳到另一个分组是**新机制**——需要：
  1. 账号/分组上配置兜底目标（§9.3 待定形态，如 `accounts.extra.fallback_group_id`）；
  2. 在 handler 循环层支持"换组重选"（在 `Messages` 选号循环外再套一层，或在 Reroute 时用兜底 groupID 重新进入选号）。

> 这是本方案唯一触及"选号/路由"的地方，需谨慎设计与回归。其余都只在响应侧做无害修正。

### 4.3 能力如何判定（待确认清单，§9.2）
`InspectRequest` 解析 `parsed` 的 native 字段，判断是否命中"上游不支持的能力"。已点名：Anthropic `documents`（Kiro 不支持）。其余候选（tools / 图片 / PDF document / thinking / cache_control 等）分别对 A、B 是否生效，见 §9.2 待确认。

---

## 5. 响应侧：按 flag 修正（CorrectResponse）

### 5.1 非流式
在每个非流式响应处理器的 `c.Data(...)` 写出之前、usage 解析之后插入（**与 response_masking 同点位**）：通用 `handleNonStreamingResponse:7339`（masking@:7407）、APIKey 直通 `*Passthrough:5120`（@:5150）、Bedrock `handleBedrockNonStreamingResponse:5541`（usage@:5556、写出@:5562）。
```go
if a, ok := c.Get("account_adapter"); ok {
    body = a.(AccountProtocolAdapter).CorrectNonStreamResponse(body)
}
c.Data(resp.StatusCode, "application/json", body)
```
> 账号实际只走其中一条（取决于账号类型）；在三处都接入同一钩子即可 type-agnostic 覆盖。建议抽一个共享 helper（`applyAdapterNonStream(c, body)`）避免三处重复。

### 5.2 流式
在每个流式响应处理器写出前、usage 解析之后（喂 **native**）插入：通用 `handleStreamingResponse:6714`（masking@:6972）、APIKey 直通 `*Passthrough:4738`（@:4847）、Bedrock `handleBedrockStreamingResponse`（usage@`bedrock_stream.go:142`、写出@:147-154）。
```go
et, d := eventType, sseData
if a, ok := c.Get("account_adapter"); ok {
    var drop bool
    if et, d, drop = a.(...).CorrectStreamEvent(eventType, sseData); drop { continue }
}
// 用 et/d 走原有 fmt.Fprintf(w, ...) 写真实 w
```
铁律：**先解析 usage（native）→ 再修正 → 再写**；写真实 `w`，不缓冲、不包裹（保 `Size()` 语义与 failover 守卫）。

### 5.3 修正内容（待确认，§9.1）
- **需求 A（按 bedrock 协议修正）**：具体改哪些字段，待你给 before→after。已知真实 bedrock 响应特征（实测样本，§8）：`id: msg_bdrk_*`、`usage` 含 `cache_creation_input_tokens`/`cache_read_input_tokens`/`cache_creation{ephemeral_5m,1h}`、`stop_details`，流式含 `amazon-bedrock-invocationMetrics`。待定：是**保留/恢复**这些 bedrock 特有字段（给 bedrock 客户端），还是别的修正。
- **需求 B（修正为 Anthropic 标准）**：把 Kiro 响应里偏离标准的部分补齐/改写为标准 Anthropic。具体偏差项待你给（Kiro 现状已实现一部分，需列出还差什么）。

---

## 6. 不踩的坑（已校验保证）

- **不包裹 `c.Writer`**：修正后直接写真实 `c.Writer`，`Size()`/`Written()` 由 gin 维护，failover「已写字节禁止换号」守卫不破。
- **计费不受影响**：各响应处理器都在修正**之前**先从 native 解析 usage（通用/直通/bedrock 三类路径一致）。
- **不碰热路径循环**：选号/failover 循环（`gateway_handler.go:530-885`）原样复用；唯一例外是 §4.2 跨分组兜底需在循环外加一层（单列设计）。
- **adapter=nil 零影响**：未打 flag 的账号完全走原逻辑。

---

## 7. 与现有机制的关系

- **同源于 `response_masking`**：账号 flag + 选号后按账号判断 + 响应侧处理。本方案把它一般化为「adapter」，`response_masking` 可视作 KiroCompatAdapter 的一个子能力（身份遮蔽）。
- **不是协议桥**：不在 Converse 与 Anthropic 之间转换；两端都标准。
- **不新增路由/中间件/Handlers 字段**：纯在 service 层选号后挂 adapter。

---

## 8. 附：真实上游样本（2026-06-27 实测，作修正规则参考）

来源：真实 Bedrock 上游（响应 `id: msg_bdrk_*` 实锤），经标准 `/v1/messages` 取得。**仅作 §5.3 修正规则的事实参考**，无密钥、可作单测 fixture。

- 非流式响应（节选）：`{"id":"msg_bdrk_…","stop_reason":"end_turn","stop_sequence":null,"stop_details":null,"usage":{"input_tokens":12,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":0},"output_tokens":4}}`
- 流式事件序列：`message_start → content_block_start → content_block_delta → content_block_stop → message_delta → message_stop`
- 流式 usage 位置：`message_delta.usage` 含最终 input+output；`message_stop.usage` = `{input,output}` 干净副本。

---

## 9. 待确认事项（拍板后即可细化到可实现）

1. **需求 A 响应"按 bedrock 协议修正"具体改什么**（§5.3）——给一个字段级 before→after。是否保留/恢复 `amazon-bedrock-invocationMetrics`、`usage.cache_creation_*`、`msg_bdrk_` id 等 bedrock 特有字段？
2. **能力检查清单**（§4.3）——除 `documents` 外还要判哪些（tools/图片/PDF/thinking/cache_control…）？各项对 A（报错）/ B（兜底）分别如何生效？
3. **兜底配置形态**（§4.2）——兜底指向账号还是分组？配置放哪（账号 `extra.fallback_group_id`？分组级？）？是否已有现成兜底机制可复用？
4. **flag 命名**——需求 A 用哪个 `accounts.extra` key？需求 B 复用 `response_masking` 还是新增（如 `kiro_compat`）？

---

## 10. 实现阶段（设计确认后展开；本文档暂不含代码改动）

> 先把 §9 四点定下来，再细化每阶段的具体代码改动点。当前仅列骨架，**待设计确认后再展开"修改代码的内容"**。

- **P0 — adapter 抽象 + flag 选择 + gating**：定义 `AccountProtocolAdapter`、`pickAdapter(account)`、`Forward` 内 `c.Set("account_adapter", …)`；account flag 方法（仿 `:1443`）。
- **P1 — 响应侧修正接入**：在 response_masking 的同组点位（通用 `handleNonStreamingResponse`/`handleStreamingResponse`、APIKey 直通、bedrock 各一处）接同一钩子（建议抽共享 helper）；先空实现（pass-through）跑通挂载，再填 §5.3 规则。
- **P2 — 请求侧能力检查**：`InspectRequest`；Reject（需求 A 报错）；同组排除换号（复用 failover）。
- **P3 — 跨分组兜底**（需求 B，若需要）：兜底配置 + handler 循环换组层。
- **P4 — 各 adapter 具体规则**：BedrockFixAdapter / KiroCompatAdapter 按 §9.1/§9.2 填实；真实样本对拍单测。
