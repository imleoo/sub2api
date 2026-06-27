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
| 账号开关 | **新增** `accounts.extra.bedrock_compat`（新 UI 开关「bedrock 兼容」） | **复用现有** `accounts.extra.response_masking`（现有 UI 开关「响应遮蔽（Kiro 兼容）」，行为扩展） |

> **开关形态 = 添加/编辑账号表单里的两个独立布尔开关**，与现有「自动透传（仅替换认证）」「响应遮蔽（Kiro 兼容）」完全同款，互不相干。
> - **Kiro**：复用现有「响应遮蔽（Kiro 兼容）」开关（存 `accounts.extra.response_masking`，方法 `Account.IsResponseMaskingEnabled()` `account.go:1443`）。本方案把它背后的行为从"仅身份遮蔽"**扩展**为完整 KiroCompatAdapter（遮蔽 + 能力判定兜底 + 响应归一）。
> - **Bedrock**：**新增**一个平行开关「bedrock 兼容」，存 `accounts.extra.bedrock_compat`，新增 `Account.IsBedrockCompatEnabled()`（严格仿 `:1443`，默认 false）。
> - 落点（与现有开关同处）：后端 `account.go`；前端 `frontend/src/components/account/CreateAccountModal.vue` + `EditAccountModal.vue` + i18n `frontend/src/i18n/locales/zh.ts`（批量编辑 `BulkEditAccountModal` 同步）。
>
> （撤回上一版讨论里"统一 `protocol_adapter` 枚举选择器"的想法——与现有"每个兼容能力一个独立开关"的 UI/存储约定不符，不采用。）

---

## 2. 核心机制：账号级协议适配器（Account Protocol Adapter）

选到账号后，按账号 flag 取一个 adapter；adapter 暴露两个钩子，分别挂在「请求侧（选号后/转发前）」和「响应侧（写出前）」。

```go
// 概念抽象（落点与命名在 §5 细化；此处先定语义）
type AccountProtocolAdapter interface {
    // 请求侧：检查上游能力。返回三选一：Pass / Reject(err) / Reroute(target)
    InspectRequest(parsed *ParsedRequest) RequestAction

    // 非流式：完整响应体 native → 目标形态（BedrockFix: → Converse JSON；Kiro: → 标准 Anthropic）
    CorrectNonStreamResponse(body []byte) []byte

    // 流式：adapter 自己控制出站「线格式 + 写帧」，因为不同目标线格式不同：
    //   BedrockFix(Converse) → application/vnd.amazon.eventstream 二进制帧
    //   Kiro                → 标准 Anthropic 文本 SSE
    // 故不再用「返回 (et,data) 交给固定的 fmt.Fprintf」那种写法（那只能发文本 SSE）。
    StreamContentType() string                         // 出站 Content-Type（覆盖 handler 默认 text/event-stream）
    EmitStreamEvent(w io.Writer, anthEventType string, anthData []byte) error // 把一条 native 事件转目标格式并写真实 w
    FinishStream(w io.Writer) error                    // 流末尾收尾（Converse: 合成并写 metadata{usage} 帧）
}

type RequestAction struct {
    Kind   ActionKind     // Pass | Reject | Reroute
    Err    error          // Reject 时（构造标准 Anthropic 错误体）
    Target FallbackTarget // Reroute 时（账号或分组，§4）
}
```

> 流式接口的关键修正（因 §11.0 确认 Converse 是二进制 EventStream）：adapter **自己写帧**、自己定 Content-Type，并维护跨事件状态（usage 汇总）。写的仍是**真实 `w`(=c.Writer)** → `Size()` 正常增长、failover「已写字节禁止换号」守卫不破（§6）。Kiro adapter 的 `EmitStreamEvent` 就是输出标准 Anthropic SSE 文本。

按账号上的两个独立开关选 adapter（互斥取其一；都关 → nil）：
```go
func pickAdapter(a *Account) AccountProtocolAdapter {
    switch {
    case a.IsBedrockCompatEnabled():   // accounts.extra.bedrock_compat（新增）
        return BedrockFixAdapter
    case a.IsResponseMaskingEnabled(): // accounts.extra.response_masking（现有「Kiro 兼容」开关）
        return KiroCompatAdapter
    default:
        return nil // 不适配，链路完全不变
    }
}
```

> 关键：adapter 为 nil 时（绝大多数账号）整条链路**零行为变化**——与现有 `response_masking` 默认关闭一致。
> 两个开关同时打开属配置异常（一个账号不会既是 kiro 又是 bedrock 上游）；按上面的 `switch` 取先匹配项，或在保存账号时校验互斥。

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
adapter 存在时**接管出站写帧**（因目标线格式可能是二进制，§2）。在每个流式响应处理器：
- 流开始：若有 adapter，用 `adapter.StreamContentType()` 覆盖 Content-Type（Bedrock→`application/vnd.amazon.eventstream`；Kiro→`text/event-stream`）。
- 每事件：`parseSSEUsagePassthrough(native)`（计费，不变）→ `adapter.EmitStreamEvent(w, eventType, sseData)`（adapter 内部转目标格式 + 编码 + 写真实 `w`）；无 adapter 时走原 `fmt.Fprintf(w, …)`。
- 流结束：`adapter.FinishStream(w)`（Converse 在此合成并写 `metadata{usage}` 帧）。

接入点同 §5.1（通用 `:6714` / 直通 `:4738` / Bedrock `bedrock_stream.go`）。
```go
// handler 流式循环内（伪代码）
if a := adapterFromCtx(c); a != nil {
    s.parseSSEUsagePassthrough(string(sseData), usage)   // 计费：喂 native，不变
    if err := a.EmitStreamEvent(w, eventType, sseData); err != nil { /* 客户端断开 */ break }
} else {
    /* 原有 fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, sseData) */
}
// 循环结束后： if a != nil { a.FinishStream(w) }
```
铁律：**先解析 usage（native）→ 再交 adapter 写**；adapter 写的是**真实 `w`**，不缓冲整流（保 `Size()` 语义与 failover「已写字节禁止换号」守卫）。adapter 内部可为「合成 metadata」缓存少量 usage 标量，但不缓存整条流。

### 5.3 修正内容（待确认，§9.1）
- **需求 A（按 bedrock 协议修正）**：调研后明确「bedrock 协议」分两种（§11.0），**工作量天差地别**，需先定目标：
  - **目标 = InvokeModel 形态**：响应≈标准 Anthropic，修正极小——`id` 改 `msg_bdrk_*` 前缀、流式 `message_stop` 注入 `amazon-bedrock-invocationMetrics`、保留 `model` 字段即可（§11.1、表 B）。
  - **目标 = Converse 形态**：整体重映射——`output.message.content`、`tool_use.id→toolUse.toolUseId`、`thinking→reasoningContent.reasoningText.text`、`stop_reason→stopReason`（9 值，§11.2 表 A）、`usage` 改名（`cache_creation→cacheWrite`）、流式逐事件 → Converse 事件 + usage 移到末尾 `metadata` 帧 + **二进制 EventStream 编码**（§11.3）。
  - 实测真实样本（§8，经标准 `/v1/messages` 从 bedrock 上游取得）可作非流式/流式对拍 fixture。
- **需求 B（修正为 Anthropic 标准）**：把 Kiro 响应里偏离标准 Anthropic（§11.1 标准列）的部分补齐/改写。具体偏差项待你给（Kiro 现状已实现一部分，需列出还差什么）。

### 5.4 BedrockFixAdapter 详设（Converse 形态 · 本方案重点）

已确认需求 A 目标 = **Converse**。BedrockFixAdapter 把标准 Anthropic 请求/响应桥到 Bedrock Converse 形态。字段映射全部依据 §11.2 表 A / §11.3 陷阱。纯转换函数放 `internal/pkg/claude/conversecompat/`（无 service 依赖），adapter 在 handler/service 层组合。

**① 请求侧 `InspectRequest`（能力门）**
- 解析 native 请求，命中「Converse/上游不支持的能力」→ `Reject(标准 Anthropic 错误体)`（需求 A 报错，不 failover）。
- 能力清单待定（§9.2），候选：`documents` 块、Converse 不支持的 `tool_choice` 形态、`top_k`（Converse 需走 `additionalModelRequestFields`，非报错而是搬移）等。
- 注意：请求侧**不改请求体**（"请求不变"）——能力检查只决定放行/报错；真正发往上游的仍是 native（上游是标准协议）。

**② 非流式响应 `CorrectNonStreamResponse`：native JSON → Converse JSON**（出站 `application/json`）
- 顶层：丢 `id/type/role/model/stop_sequence` → 包成 `{output:{message:{role:"assistant",content:[…]}}, stopReason, usage, metrics?}`。
- content 块（表 A）：text 去判别符；`tool_use{id,name,input}`→`{toolUse:{toolUseId,name,input}}`；`thinking{thinking,signature}`→`{reasoningContent:{reasoningText:{text,signature}}}`；`redacted_thinking{data}`→`{reasoningContent:{redactedContent}}`；image `source.data`→`{image:{format,source:{bytes}}}`。
- `stop_reason→stopReason`：4 值同名；`pause_turn/refusal` 无 Converse 对应——按 §9.x 决定（建议 `refusal→content_filtered`、`pause_turn→end_turn`）。
- `usage`：`input_tokens→inputTokens`、`output_tokens→outputTokens`、`cache_read_input_tokens→cacheReadInputTokens`、**`cache_creation_input_tokens→cacheWriteInputTokens`**、补 `totalTokens`。

**③ 流式 `StreamContentType/EmitStreamEvent/FinishStream`：native SSE 事件 → Converse 二进制帧**
- `StreamContentType()` = `application/vnd.amazon.eventstream`。
- `EmitStreamEvent`：把 native 事件转为对应 Converse 事件并**封二进制帧**写 `w`：
  - `message_start`→`messageStart{role}`（缓存 `message.usage.input`）
  - `content_block_start`→`contentBlockStart`；`content_block_delta`→`contentBlockDelta`（`text_delta`→`delta.text`；`input_json_delta.partial_json`→`delta.toolUse.input`(String)；`thinking_delta/signature_delta`→`delta.reasoningContent.{text|signature}`）；`content_block_stop`→`contentBlockStop`
  - `message_delta`→`messageStop{stopReason}`（缓存 `usage.output`）
  - `ping`/未知 → 跳过
- `FinishStream`：用缓存的 input/output 合成并写 **`metadata{usage{inputTokens,outputTokens,totalTokens,…},metrics{latencyMs}}`** 帧（Converse usage 只在末尾 metadata，§11.3）。
- **二进制帧编码** `EncodeEventStreamFrame(eventType, payloadJSON)`（新增 `conversecompat/eventstream.go`）：帧 = `total_len + headers_len + prelude_crc(CRC32) + headers(:event-type / :content-type=application/json / :message-type=event) + payload + message_crc(CRC32)`，大端。**实现金标准 = 仓库现有 `bedrockEventStreamDecoder`（bedrock_stream.go:251）的逆过程**（它每天在解真实 AWS Bedrock 二进制帧），以 `decode(encode(x))==x` 往返测试把关。错误中途 → 封 `:message-type=exception` 帧（`internalServerException` 等）。

> **验证策略（分两层）**：
> - **第 1 层 · openclaw 标准上游**（key 已存 `script/e2e.env` 的 `E2E_BEDROCK_CONVERSE_UPSTREAM_KEY` / `_BASE_URL=https://openclaw.zhiguo.fan` / `_MODEL`，gitignored）：取**真实 native 请求/响应金样本**，验证 ①非流式 Anthropic→Converse JSON 映射 ②编码器自洽 `decode(encode(x))==x`（用仓库 `bedrockEventStreamDecoder` 反解自己编的帧）。openclaw **不吐 Converse 二进制帧**，故只覆盖标准侧 + 编码器自洽。
> - **第 2 层 · 真实 AWS Bedrock `ConverseStream`**（需另找 AWS 账号）：抓一段真实二进制响应,对拍帧布局/CRC/header 大小写、`:message-type` 取值——这是与 AWS 线格式一致性的**最终门禁**,编码器实现前完成。

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

## 8. 附：真实上游样本 + 测试 fixture（2026-06-27 实测）

来源：真实 Bedrock 上游（响应 `id: msg_bdrk_*` 实锤），经标准 `/v1/messages` 从 openclaw 取得。无密钥、可提交、可作单测 fixture。

**已落盘 fixture**（设计阶段放 `claudedocs/`，不进代码树；捕获脚本 `script/capture-converse-fixtures.sh`，读 `E2E_BEDROCK_CONVERSE_*` 重跑即更新。**实现 `conversecompat` 包时再把它们移到 `<包>/testdata/` 并把脚本 OUT_DIR 改回去**）：
```
claudedocs/fixtures/converse/
  native_nonstream_text.json      非流式·纯文本（text 块 + stop_reason + usage 结构）
  native_stream_text.sse          流式·纯文本（8 事件，usage 位置）
  native_nonstream_tooluse.json   非流式·工具调用（tool_use 块）
  native_stream_tooluse.sse       流式·工具调用（9 事件，input_json_delta 分片）
```

实测要点（直接影响 §5.4 映射）：
- `stop_reason`：`end_turn`、`tool_use`（→ Converse 同名）。
- **`tool_use.id` / 流式 `content_block_start.content_block.id` 带 `toolu_bdrk_` 前缀** → 映射 `id→toolUseId` **原样保留**，勿剥前缀。
- `usage`：`{input_tokens,output_tokens,cache_creation_input_tokens,cache_read_input_tokens,cache_creation{ephemeral_5m,1h}}`（`stop_details` 为 null）。
- 流式：`message_start → content_block_start → content_block_delta×N → content_block_stop → message_delta → message_stop`；`input_json_delta.partial_json` 分片需拼接（实测 `{"ci`+`ty": "Pa`+`ris"}`）；`message_delta.usage` 含最终 input+output，`message_stop.usage`={input,output} 干净副本 → `StreamAggregator` 直接取之合成 metadata。

---

## 9. 待确认事项（拍板后即可细化到可实现）

✅ **已定·需求 A 目标 = Converse 形态**（重点工作）。BedrockFixAdapter 详设见 §5.4：完整 Anthropic→Converse 映射（§11.2 表 A）+ 流式二进制 EventStream 编码 + usage 末尾 metadata 帧。剩余子项：`pause_turn/refusal` 的 stopReason 落子（建议 `content_filtered`/`end_turn`）。
2. **能力检查清单**（§4.3）——除 `documents` 外还要判哪些（tools/图片/PDF/thinking/cache_control…）？各项对 A（报错）/ B（兜底）分别如何生效？
3. **兜底配置形态**（§4.2）——兜底指向账号还是分组？配置放哪（账号 `extra.fallback_group_id`？分组级？）？是否已有现成兜底机制可复用？

✅ **已定·开关形态（§1.4）**：两个独立账号开关。Kiro 复用现有「响应遮蔽（Kiro 兼容）」=`accounts.extra.response_masking`（行为扩展）；Bedrock 新增「bedrock 兼容」=`accounts.extra.bedrock_compat` + `Account.IsBedrockCompatEnabled()`（仿 `:1443`，默认 false）+ 前端 Create/Edit AccountModal 开关 + i18n。`bedrock_compat` 键名可再敲定（备选 `bedrock_protocol_fix`）。

---

## 10. 实现阶段（设计确认后展开；本文档暂不含代码改动）

> 先把 §9 四点定下来，再细化每阶段的具体代码改动点。当前仅列骨架，**待设计确认后再展开"修改代码的内容"**。

- **P0 — 开关 + adapter 抽象 + gating**：新增 `Account.IsBedrockCompatEnabled()`（`accounts.extra.bedrock_compat`，仿 `:1443`）；前端新增「bedrock 兼容」开关（`CreateAccountModal.vue`/`EditAccountModal.vue`/`BulkEditAccountModal` + `zh.ts`，与「Kiro 兼容」同款，含 vitest）；定义 `AccountProtocolAdapter`、`pickAdapter(account)`（读两个开关）、`Forward` 内 `c.Set("account_adapter", …)`。
- **P1 — 响应侧修正接入**：在 response_masking 的同组点位（通用 `handleNonStreamingResponse`/`handleStreamingResponse`、APIKey 直通、bedrock 各一处）接同一钩子（建议抽共享 helper）；先空实现（pass-through）跑通挂载，再填 §5.3 规则。
- **P2 — 请求侧能力检查**：`InspectRequest`；Reject（需求 A 报错）；同组排除换号（复用 failover）。
- **P3 — 跨分组兜底**（需求 B，若需要）：兜底配置 + handler 循环换组层。
- **P4 — KiroCompatAdapter 规则**：按 §9.2 填实（响应归一标准 Anthropic + 能力判定兜底）；真实样本对拍单测。
- **P5 — BedrockFixAdapter（Converse 形态，重点·最大工作量）**：
  - `conversecompat/` 非流式映射（表 A）+ 单测（用 §8 native 样本对拍 Converse JSON）
  - `eventstream.go` 二进制帧编码（金标准 = `bedrockEventStreamDecoder` 逆过程）+ `decode(encode(x))==x` 往返测试
  - 流式 `EmitStreamEvent`/`FinishStream`（usage 合成 metadata 帧）
  - 请求侧 `InspectRequest` 能力门（§9.2）+ Reject
  - ⚠️ **前置门禁**：先抓真实 AWS Bedrock `ConverseStream` 二进制金样本对拍帧细节（§5.4）

---

## 11. 附：协议差异参考（联网调研结论，2026-06-27）

> 标注 **[C]** = 有官方文档佐证，**[I]** = 跨 API 等价关系/社区抓包推断。文档源：`platform.claude.com/docs/en/api/*`（原 docs.anthropic.com 301 跳转）、`docs.aws.amazon.com/bedrock/latest/APIReference/*`。

### 11.0 关键结论：「bedrock 协议」有两种，差异天壤之别

| | Bedrock **InvokeModel**（native-body） | Bedrock **Converse** |
|---|---|---|
| 响应形状 | **≈ 标准 Anthropic**（几乎零差异） | **完全不同**（camelCase、`output.message`、union 块） |
| 非流式需修正 | 仅 `id` 前缀 `msg_bdrk_`、可选注入 `amazon-bedrock-invocationMetrics` | 整体重映射（见表 A） |
| 流式线格式 | 二进制 EventStream，帧内是 **native Anthropic 事件** | 二进制 EventStream，帧内是 **Converse 事件**；usage 只在末尾 `metadata` 帧 |
| 「协议修正」工作量 | 极小 | 大 |

→ **需求 A 到底要哪种，决定工作量与实现（§9.1 待你最终确认）。** 标题「Bedrock Converse」+ InvokeModel 几乎不用修正 ⇒ 大概率指 **Converse**。

### 11.1 三协议请求/响应要点
- **标准 Anthropic `/v1/messages`** [C]：头 `anthropic-version: 2023-06-01`；响应 `content[]`（text/tool_use/thinking/redacted_thinking{**data**}/server_tool_use）、`stop_reason`（7 值：`end_turn|max_tokens|stop_sequence|tool_use|pause_turn|refusal|model_context_window_exceeded`）、`usage{input_tokens,output_tokens,cache_creation_input_tokens,cache_read_input_tokens,cache_creation{ephemeral_5m,1h},output_tokens_details,service_tier}`。流式 SSE：input usage 在 `message_start`、output（**累计值**）在 `message_delta`。
- **Bedrock InvokeModel** [C]：请求体**去 model**（在 URL）、加 `anthropic_version:"bedrock-2023-05-31"`、`anthropic_beta:[...]`；响应**与 native 同形**且**保留 model 字段**；`stop_reason` 加 `refusal`+`stop_details{type,category,explanation}`。流式 `application/vnd.amazon.eventstream` 二进制，`chunk.bytes` base64 解出就是 native Anthropic 事件，**无 `[DONE]`**，`amazon-bedrock-invocationMetrics{inputTokenCount,outputTokenCount,invocationLatency,firstByteLatency}` 挂在 `message_stop`[I]。
- **Bedrock Converse** [C]：camelCase。请求 `messages[{role,content[ContentBlock union]}]`、`system[{text}]`、`inferenceConfig{maxTokens,temperature,topP,stopSequences}`（**无 topK**，走 `additionalModelRequestFields`）、`toolConfig{tools[{toolSpec{name,description,inputSchema{json}}}],toolChoice}`。响应 `output.message.content[]`、`stopReason`（**9 值**：`end_turn|tool_use|max_tokens|stop_sequence|guardrail_intervened|content_filtered|malformed_model_output|malformed_tool_use|model_context_window_exceeded`）、`usage{inputTokens,outputTokens,totalTokens,cacheReadInputTokens,cacheWriteInputTokens,cacheDetails}`、`metrics{latencyMs}`。流式事件 `messageStart/contentBlockStart/contentBlockDelta/contentBlockStop/messageStop/metadata`，**usage 只在末尾 `metadata` 帧**；帧按 `:event-type` header 分发。

### 11.2 表 A — 标准 Anthropic 响应 → Converse 响应（核心映射）
| Anthropic | Converse | 备注 |
|---|---|---|
| `{type:"text",text}` | `{text}` | 去 type 判别符 |
| `{type:"tool_use",id,name,input}` | `{toolUse:{toolUseId,name,input}}` | **`id`→`toolUseId`** [C] |
| `{type:"thinking",thinking,signature}` | `{reasoningContent:{reasoningText:{text,signature}}}` | **`thinking`→`reasoningText.text`** [I]；signature 同名 [C] |
| `{type:"redacted_thinking",data}` | `{reasoningContent:{redactedContent:<bytes>}}` | 扁平 bytes 成员 [I] |
| image `source:{type:base64,media_type,data}` | `{image:{format,source:{bytes}}}` | `data`→`bytes`；`image/png`→`png` |
| `id/type/role`、`model`、`stop_sequence` | 无对应 | Converse 输出只有 `message{role,content}` |
| `stop_reason` | `stopReason` | 4 值同名(`end_turn/max_tokens/stop_sequence/tool_use`)；`pause_turn/refusal` 无 Converse 对应，需显式决定 |
| `usage.input_tokens` | `usage.inputTokens` | snake→camel |
| `usage.output_tokens` | `usage.outputTokens` | |
| `usage.cache_read_input_tokens` | `usage.cacheReadInputTokens` | [C] |
| `usage.cache_creation_input_tokens` | `usage.cacheWriteInputTokens` | **改名 creation→write（词+大小写都变）** [C] |
| — | `usage.totalTokens` / `metrics.latencyMs` | Converse 专有，需自算/补 |

请求侧缓存：Anthropic 块内 `cache_control:{type:ephemeral}` → Converse 独立 `{cachePoint:{type:default}}` 块。

### 11.3 关键陷阱（写转换代码必看）
1. `cache_creation_input_tokens → cacheWriteInputTokens`（唯一非纯 snake→camel 的改名）。
2. `tool_use.id → toolUse.toolUseId`；`tool_result.tool_use_id → toolResult.toolUseId`。
3. `thinking → reasoningContent.reasoningText.text`；redacted 是扁平 `redactedContent` bytes。
4. Converse `stopReason` 9 值（4 个与 Anthropic 同名）；`pause_turn/refusal` 无对应须显式映射。
5. 流式：Anthropic 文本 SSE vs Bedrock 二进制 EventStream；Converse 把**全部 usage 挪到末尾单个 `metadata` 帧**（Anthropic 是 input@message_start + output@message_delta 分散）。
6. `topK` 不在 `inferenceConfig`，走 `additionalModelRequestFields` 的 snake_case `top_k`。
7. 置信度：AWS 字段名 [C]（API Reference 逐字、双 agent 交叉确认）；跨 API 等价关系与 `msg_bdrk_`/`invocationMetrics` 流式细节 [I]（社区抓包，无官方 side-by-side crosswalk）。
