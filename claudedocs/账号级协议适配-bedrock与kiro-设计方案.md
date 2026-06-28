# 账号级协议适配（Bedrock 协议修正 / Kiro 兼容）— 设计方案

> 文档状态：草稿 · 2026-06-27 · 已过一轮 codex 真实代码 review（挂载点行号已核验，缺口已回填，见各节 **[review]** 标注与 §12）· **2026-06-28 全部 P0–P5 已实现落地，见 §14**
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
> - **Kiro**：复用现有「响应遮蔽（Kiro 兼容）」开关（存 `accounts.extra.response_masking`，方法 `Account.IsResponseMaskingEnabled()` `backend/internal/service/account.go:1443`）。本方案把它背后的行为从"仅身份遮蔽"**扩展**为完整 KiroCompatAdapter（遮蔽 + 能力判定兜底 + 响应归一）。
> - **Bedrock**：**新增**一个平行开关「bedrock 兼容」，存 `accounts.extra.bedrock_compat`，新增 `Account.IsBedrockCompatEnabled()`（严格仿 `service/account.go:1443`，默认 false）。
> - 落点（与现有开关同处）：后端 `backend/internal/service/account.go`；前端 `frontend/src/components/account/CreateAccountModal.vue` + `EditAccountModal.vue` + i18n `frontend/src/i18n/locales/zh.ts`。**[review] 批量编辑修正**：`BulkEditAccountModal` 当前**不含** `response_masking`（实测），故 `bedrock_compat` **第一版也不进 BulkEdit**（详见 §13.0②、§13 明确改法）。
> - **[review]** 路径更正：`IsResponseMaskingEnabled` 实际在 `backend/internal/service/account.go`（**不是** `model/account.go`）；符号与行号 `:1443` 经 codex 实读核验对齐。
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
- 账号 flag 模式对齐 `IsResponseMaskingEnabled()`（`backend/internal/service/account.go:1443`，读 `accounts.extra`，默认 false）。
- **[review] 通用流式 usage 顺序更正**：通用 `handleStreamingResponse` 实际是 `processSSEEvent` 先从 native 提取 `usagePatch`，**写出当前 block 之后**再 `mergeSSEUsagePatch` 合并——并非"先解析完 usage 再写"。接入 adapter 时必须显式固定为「**提取 usagePatch → 交 adapter 写帧 → 合并 usagePatch**」，确保 adapter 收尾（`FinishStream` 合成 metadata 帧）前 usage 标量已就绪；否则 Converse 末尾 metadata 帧可能拿到未合并的 usage。计费总账仍以 native 提取为准，不受帧改写影响。

---

## 4. 请求侧：能力检查与兜底路由（唯一的"新机制"）

`InspectRequest` 三种结果：

### 4.1 Pass / Reject
- **Pass**：上游支持，正常转发。
- **Reject**（需求 A 主用）：上游不支持请求用到的能力 → 构造错误，按 `response_masking` 早返回的同款写法直接回客户端（标准 Anthropic 错误体）。不触发 failover（这是确定性的能力缺失，不是上游抖动）。

### 4.2 Reroute（需求 B 主用）— 兜底路由
请求用到 Kiro 不支持的能力 → 不让 Kiro 账号处理，改投兜底。两种粒度：

- **同组排除换号**：把当前账号加入 failover 的排除集、`continue` 循环选同组另一账号。**复用现有 failover 循环**，但**不是零新增**（见下 review 补强）。前提：同组内有能处理该能力的非 Kiro 账号。
- **跨分组兜底**（兜底是"别的分组"时）：现有 failover 循环**只在单组内换号**（`gateway_handler.go:530-885`），跳到另一个分组是**新机制**——需要：
  1. 账号/分组上配置兜底目标（§9.3 待定形态，如 `accounts.extra.fallback_group_id`）；
  2. 在 handler 循环层支持"换组重选"（在 `Messages` 选号循环外再套一层，或在 Reroute 时用兜底 groupID 重新进入选号）。

> **[review] 同组 reroute 需要 handler/service 交互协议（不能只"循环外套一层"）**：
> `InspectRequest` 在 **service 层**（`Forward`）执行，而排除集 `FailedAccountIDs` 由 **handler 层** 的 `FailoverState` 管理；service 没有直接改它的通道。因此"复用 failover"必须新增一条**可识别的 reroute 信号**：
> 1. `InspectRequest` 判 `Reroute` 时，`Forward` 返回一个**专用 error 类型**（如 `RerouteError{AccountID, Reason}`，与上游抖动用的 `UpstreamFailoverError` **区分**——前者是确定性能力缺失、必须换号，后者是可重试抖动）。
> 2. handler 选号循环识别 `RerouteError` → 把该 `AccountID` 加入排除集 → `continue` 重选。
> 3. **必须在换号前完成的清理**（与上游抖动 failover 同款，不能漏）：释放并发槽 / 等待队列位、解除 sticky session 绑定、释放用户组串行锁——否则被 reroute 的 Kiro 账号会泄漏占用。这些清理点在现有 failover 路径已有，reroute 分支要走同一套，不可另起炉灶。
> 4. **死循环防护**：reroute 与 failover 共用同一排除集与重试上限，确保"能力缺失账号被逐个排除后"能正常收敛到 Reject 或耗尽报错，不会无限换号。
>
> **[review] 跨分组兜底必须单独成方案（最高风险，本文档不展开实现）**：
> 当前 `Messages` 循环用 `currentAPIKey.GroupID` 选号，成功后用同一 `currentAPIKey / currentSubscription / quotaPlatform / channelMapping` 记账。一旦跨组，下列每一项都要重新定义口径，缺一即数据错乱：
> - **分组模型映射 / channelMapping**：兜底组的模型白名单/透传映射可能与原组不同（参见已知陷阱"批量改账号丢模型映射"）。
> - **订阅与配额归属**：`currentSubscription` / `quotaPlatform` 是按原组/原 key 绑定的，跨组后计费归属与配额扣减归谁？
> - **sticky session key**：sticky 绑定含分组维度，跨组会破坏对话连续性，需明确是否重建。
> - **并发等待队列**：跨组要重新进目标组的等待队列，原组的占用要先释放。
> - **usage 日志的 group/platform 字段**：日志里的分组/平台口径以哪个组为准。
> 结论：跨分组兜底**不在 P0–P5 范围内**，需作为独立设计文档（含上述 6 项口径表 + 回归用例）评审通过后再动；§10 的 P0–P5 不含跨组逻辑。

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
  - **[review] Kiro 侧目前仍是占位、不是可实现设计**：实读代码确认 Kiro 现状**只有** `response_masking`（`Forward` 内按身份问题早返回 + `maskResponseBody` 关键词替换），**没有**任何响应归一 / schema 映射 / 请求能力判定兜底的实现；`SniffAnthropic` 仅是能力嗅探骨架，**未接入** `Messages` 选号循环。因此 KiroCompatAdapter 不能凭"待你给"进实现，必须先用**真实 Kiro 样本**产出下列可测规格（与 §8 Bedrock 同款 fixture 驱动），列入 §9.2：
    1. **非流式偏差表**：顶层 `id/type/model/role/content/stop_reason/usage/error` 各字段 Kiro 实际形态 vs 标准 Anthropic，逐项补齐/改写规则。
    2. **流式偏差表**：事件名集合、各事件 `data` JSON 结构、终止事件、usage 出现位置、错误事件格式。
    3. **content 块规范化**：`text/tool_use/tool_result/image/document/thinking` 哪些 Kiro 支持；不支持的在**请求侧** reject 还是 reroute（如 Anthropic `documents`）。
    4. **usage 归一**：input/output/cache_read/cache_creation 各自来源与缺失兜底（保证计费口径不变）。
    5. **请求能力判定**：复用并补全 `SniffAnthropic`，明确接入 `Messages` 循环的位置与 reroute 触发条件。
  - 在拿到真实 Kiro 样本前，§10 的 P4 不展开；可先按 §12「更简落地路径」让 KiroCompatAdapter 退化为现有 `response_masking` 行为（零回归），把归一/兜底留到样本到位。

### 5.4 BedrockFixAdapter 详设（Converse 形态 · 本方案重点）

已确认需求 A 目标 = **Converse**。BedrockFixAdapter 把标准 Anthropic 请求/响应桥到 Bedrock Converse 形态。字段映射全部依据 §11.2 表 A / §11.3 陷阱。纯转换函数放 `internal/pkg/claude/conversecompat/`（无 service 依赖），adapter 在 handler/service 层组合。

**① 请求侧 `InspectRequest`（能力门）**
- 解析 native 请求，命中「Converse/上游不支持的能力」→ `Reject(标准 Anthropic 错误体)`（需求 A 报错，不 failover）。
- 能力清单待定（§9.2），候选：`documents` 块、Converse 不支持的 `tool_choice` 形态、`top_k`（Converse 需走 `additionalModelRequestFields`，非报错而是搬移）等。
- 注意：请求侧**不改请求体**（"请求不变"）——能力检查只决定放行/报错；真正发往上游的仍是 native（上游是标准协议）。

**② 非流式响应 `CorrectNonStreamResponse`：native JSON → Converse JSON**（出站 `application/json`）
- 顶层：丢 `id/type/role/model/stop_sequence` → 包成 `{output:{message:{role:"assistant",content:[…]}}, stopReason, usage, metrics?}`。
- content 块（表 A）：text 去判别符；`tool_use{id,name,input}`→`{toolUse:{toolUseId,name,input}}`；`thinking{thinking,signature}`→`{reasoningContent:{reasoningText:{text,signature}}}`；`redacted_thinking{data}`→`{reasoningContent:{redactedContent}}`；image `source.data`→`{image:{format,source:{bytes}}}`。
- **[review] content 块枚举补全**：
  - `server_tool_use`（§11.1 列出但 §5.4 漏映射）：Converse 无原生对应。落子 = **当作 `tool_use` 同款映射到 `{toolUse:{toolUseId,name,input}}`**（server_tool_use 与 tool_use 结构同构），并在 §9.2 能力门标记"若上游会回 `web_search` 等 server tool，需确认 Converse 侧是否拒绝"。
  - `tool_result`（§11.3 提到 `tool_use_id→toolResult.toolUseId`）：**仅出现在请求侧**（assistant 响应不含 tool_result）。响应侧 `CorrectNonStreamResponse` **断言不应出现**；若意外出现 → 走"未知块兜底"（见下）。
  - **未知 content 块兜底**：遇到上面未枚举的 `type` → 不静默丢弃，记一条 warn 日志并**降级为 `{text:"[unsupported block: <type>]"}`**（保证 Converse 结构合法、可观测），不中断整条响应。
- `stop_reason→stopReason`：5 值同名（`end_turn/max_tokens/stop_sequence/tool_use/model_context_window_exceeded`）；`pause_turn/refusal` 无 Converse 对应——按 §9.x 决定（建议 `refusal→content_filtered`、`pause_turn→end_turn`）。**[review] 补 `model_context_window_exceeded`**：§11.1 已列、Converse 侧同名存在，原 §5.4 漏映射，按"同名直传"处理。未知 `stop_reason` → 兜底 `end_turn` 并记 warn。
- `usage`：`input_tokens→inputTokens`、`output_tokens→outputTokens`、`cache_read_input_tokens→cacheReadInputTokens`、**`cache_creation_input_tokens→cacheWriteInputTokens`**、补 `totalTokens`。**[review] `cache_creation` 5m/1h 细分落子**：native 当前还解析 `cache_creation.{ephemeral_5m_input_tokens,ephemeral_1h_input_tokens}` 到独立字段。Converse `usage` 无逐 TTL 细分字段——落子 = **汇总进 `cacheWriteInputTokens`、细分丢弃**（计费仍以 native 提取为准，不依赖 Converse 字段）。若后续确认 Converse `cacheDetails` 可承载，再迁移；当前不映射 `cacheDetails`。

**③ 流式 `StreamContentType/EmitStreamEvent/FinishStream`：native SSE 事件 → Converse 二进制帧**
- `StreamContentType()` = `application/vnd.amazon.eventstream`。
- `EmitStreamEvent`：把 native 事件转为对应 Converse 事件并**封二进制帧**写 `w`：
  - `message_start`→`messageStart{role}`（缓存 `message.usage.input`）
  - `content_block_start`→`contentBlockStart`；`content_block_delta`→`contentBlockDelta`（`text_delta`→`delta.text`；`input_json_delta.partial_json`→`delta.toolUse.input`(String)；`thinking_delta/signature_delta`→`delta.reasoningContent.{text|signature}`）；`content_block_stop`→`contentBlockStop`
  - `message_delta`→`messageStop{stopReason}`（缓存 `usage.output`）
  - `ping`/未知 → 跳过
- `FinishStream`：用缓存的 input/output 合成并写 **`metadata{usage{inputTokens,outputTokens,totalTokens,…},metrics{latencyMs}}`** 帧（Converse usage 只在末尾 metadata，§11.3）。
- **二进制帧编码** `EncodeEventStreamFrame(eventType, payloadJSON)`（新增 `conversecompat/eventstream.go`）：帧 = `total_len + headers_len + prelude_crc(CRC32) + headers(:event-type / :content-type=application/json / :message-type=event) + payload + message_crc(CRC32)`，大端。**实现金标准 = 仓库现有 `bedrockEventStreamDecoder`（bedrock_stream.go:240 类型 / `Decode()` :251）的逆过程**（它每天在解真实 AWS Bedrock 二进制帧）。错误中途 → 封 `:message-type=exception` 帧（`internalServerException` 等）。

> **[review] 验证策略修正（关键）**：原方案打算用 `decode(encode(x))==x` 让现有 `bedrockEventStreamDecoder` 反解自己编的帧来自洽把关——**这条不成立**。实读 `bedrock_stream.go`：现有 decoder 只接受 `:event-type == "chunk"` 的 payload（InvokeModel 流的唯一帧类型），而 Converse 真实事件是 `messageStart / contentBlockStart / contentBlockDelta / contentBlockStop / messageStop / metadata` 等**多种 `:event-type`**，现有 decoder 会直接拒掉，无法做往返校验。
>
> 修正为以下三选一（按工作量/可靠性排序，建议①+③）：
> 1. **抽一个纯帧层 decoder**（只解 `total_len/headers_len/prelude_crc/headers/payload/message_crc`，**不校验 `:event-type` 取值**）放 `conversecompat/eventstream.go`，与 encoder 成对，`decodeFrame(encodeFrame(x))==x` 验证**帧布局 + CRC + header 编解码**自洽。这是编码器自洽测试的正确形态。
> 2. **扩展现有 `bedrockEventStreamDecoder`** 放开 `:event-type` 白名单到 Converse 事件集——改动现有热路径文件，风险更高，不推荐仅为测试而改。
> 3. **真实 ConverseStream 二进制金样本对拍**（见下第 2 层）——校验"我们编的帧"与"AWS 实际吐的帧"在布局/CRC/header 大小写/`:message-type` 上逐字节一致。这是与 AWS 线格式一致性的**唯一可信门禁**，不可被 ①替代（①只证明自洽，不证明与 AWS 一致）。
>
> **分两层落地**：
> - **第 1 层 · openclaw 标准上游**（key 已存 `script/e2e.env` 的 `E2E_BEDROCK_CONVERSE_UPSTREAM_KEY` / `_BASE_URL=https://openclaw.zhiguo.fan` / `_MODEL`，gitignored）：取**真实 native 请求/响应金样本**，验证 ①非流式 Anthropic→Converse JSON 映射 ②编码器自洽（用上面**新抽的纯帧 decoder**，**不是**现有 `bedrockEventStreamDecoder`）。openclaw **不吐 Converse 二进制帧**，故只覆盖标准侧 + 编码器自洽。
> - **第 2 层 · 真实 AWS Bedrock `ConverseStream`**（需另找 AWS 账号）：抓一段真实二进制响应，对拍帧布局/CRC/header 大小写、`:message-type` 取值、各 `:event-type` 的 payload schema——**编码器实现前必须先完成此层取样**，否则字段名/嵌套靠推断，返工成本高。

---

## 6. 不踩的坑（已校验保证）

- **不包裹 `c.Writer`**：修正后直接写真实 `c.Writer`，`Size()`/`Written()` 由 gin 维护，failover「已写字节禁止换号」守卫不破。**[review] 约束补强**：adapter **禁止**①包裹/替换 `c.Writer`；②用内部缓冲替代真实 writer；③先写半帧后又返回"可 failover"的错误（守卫只看 `Size()` 是否变化，半帧已让 `Size()>0`，此时换号会产生坏响应）。adapter 一旦开始写帧，后续只能写完或写 `:message-type=exception` 帧，不得回退到 failover。
- **计费不受影响**：各响应处理器都在修正**之前**先从 native 解析 usage（通用/直通/bedrock 三类路径一致）。**[review] 例外**：通用流式是"写出 block 后再 merge usage"（详见 §3、§5.2），接入 adapter 时须把顺序固定为「提取→写帧→合并」，保证 `FinishStream` 合成 metadata 前 usage 已就绪。
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

## 10. 实现阶段（按 §12 分阶段路径展开 · 风险最小先做）

> 顺序已按 §12 重排：**先 P0 地基 → P1 Bedrock 非流式（先出价值、零热路径风险）→ P2 请求能力门 → P3 Bedrock 流式 EventStream（前置门禁后做）→ P4 Kiro 零回归接壳 → P5 Kiro 归一（样本到位后）**。跨分组兜底**不在 P0–P5**，作为独立方案另评（§4.2）。
> 每阶段独立可提交、可回滚；adapter=nil 全程零行为变化（§6）。行号均为 codex 已核验的当前值，实现时以实际为准。

### P0 — 地基：开关 + adapter 抽象 + gating + 共享 helper（无业务规则）
**后端**
- `backend/internal/service/account.go`：新增 `Account.IsBedrockCompatEnabled()`（读 `accounts.extra.bedrock_compat`，**严格仿 `IsResponseMaskingEnabled():1443`**，默认 false）。键名 `bedrock_compat`（备选 `bedrock_protocol_fix`，§9 待敲）。
- 新增 `AccountProtocolAdapter` 接口（§2 签名）+ `pickAdapter(a *Account)`（§2，先匹配 bedrock、再 masking、否则 nil）。落点建议新文件 `backend/internal/service/account_adapter.go`。
- `backend/internal/service/gateway_service.go` `Forward:3702`：在 `response_masking` gate（`:3714`）**同一段**加 `if ad := pickAdapter(account); ad != nil { c.Set("account_adapter", ad) }`。
- 抽两个共享 helper（供 P1/P3 三处响应路径复用，避免重复）：
  - `applyAdapterNonStream(c, body) []byte`（取 ctx adapter，非 nil 则 `CorrectNonStreamResponse`）。
  - `adapterFromCtx(c) AccountProtocolAdapter`（流式用）。
- **P0 阶段 adapter 行为 = 纯 pass-through**（`CorrectNonStreamResponse` 原样返回、流式接口先不接管），只验证挂载不破链路。

**前端**（与「Kiro 兼容」开关同款，§1.4 落点）
- `frontend/src/components/account/CreateAccountModal.vue` + `EditAccountModal.vue`：新增「bedrock 兼容」布尔开关，写 `extra.bedrock_compat`。
- `BulkEditAccountModal`：同步该字段。
- `frontend/src/i18n/locales/zh.ts`（及其它 locale）：新增开关文案。
- vitest：仿现有 `response_masking` 开关测试补 `*.spec.ts`。

**门禁**：`go test -tags=unit ./internal/service/...`；前端 `pnpm test:run` 相关 spec + `pnpm run lint:check`。挂载后跑一次 e2e（`./script/e2e-test.sh`）确认 adapter=nil 路径零变化。

### P1 — BedrockFixAdapter 非流式（阶段一核心；流式先 Reject）
**新增纯转换包**（无 service 依赖）`backend/internal/pkg/claude/conversecompat/`
- `mapping.go`：`AnthropicToConverseJSON(native []byte) []byte` —— 实现 §5.4 ② 全量映射：顶层重包 `{output:{message:{role,content}},stopReason,usage,metrics?}`；content 块按表 A（含 §5.4 review 补的 `server_tool_use`/`tool_result`/未知块兜底）；`stop_reason→stopReason`（5 值同名 + `pause_turn→end_turn`/`refusal→content_filtered` + 未知兜底 `end_turn`）；usage 改名（`cache_creation_input_tokens→cacheWriteInputTokens`，5m/1h 细分汇总丢弃）。
- `mapping_test.go`：用 §8 fixture `native_nonstream_text.json` / `native_nonstream_tooluse.json` 对拍 Converse JSON（golden file）。

**adapter 落地**：`account_adapter.go` 的 `BedrockFixAdapter`
- `CorrectNonStreamResponse(body)` = `conversecompat.AnthropicToConverseJSON(body)`。
- 流式三接口（`StreamContentType/EmitStreamEvent/FinishStream`）**本阶段不实现**——由 P2 的 `InspectRequest` 对 `stream:true` 直接 `Reject`（见下），故 P1 不会进流式分支。

**响应侧接入**（§5.1，三条非流式路径，用 P0 的 `applyAdapterNonStream`）
- 通用 `handleNonStreamingResponse:7339`：在 masking 点 `:7407`、`c.Data` 写出前插入。
- APIKey 直通 `*AnthropicAPIKeyPassthrough:5120`：masking 点 `:5150` 前。
- Bedrock `handleBedrockNonStreamingResponse:5541`：usage 解析（`:5556`）后、写出（`:5562`）前。
- **顺序铁律**：usage 已在各路径写出前从 native 解析（§6），adapter 只改 body 不碰 usage。

**门禁**：`conversecompat` 单测 golden 通过；`go test -tags=unit ./internal/service ./internal/pkg/claude/...`；打 flag 账号走非流式请求实测返回 Converse JSON。

### P2 — Bedrock 请求侧能力门 `InspectRequest` + Reject
- `account_adapter.go` `BedrockFixAdapter.InspectRequest(parsed)`：
  - **`stream:true` → `Reject`**（标准 Anthropic 错误体「暂不支持流式 Converse」）——这是 P1 流式不实现的兜底闸，P3 落地后移除此条。
  - §9.2 能力清单命中（候选：`documents` 块、Converse 不支持的 `tool_choice` 形态等）→ `Reject`；**不改请求体**（§5.4 ①）。
  - `top_k` 等"可搬移"项不 Reject，标记留待 P3 搬 `additionalModelRequestFields`。
- `gateway_service.go` `Forward`：在 `c.Set("account_adapter")` 之后、转发前插 `action := ad.InspectRequest(parsed)`；`Reject` 按 `response_masking` 早返回同款写法（`:3714` 风格）直接回客户端，**不触发 failover**（确定性能力缺失）。
- 单测：构造命中/不命中能力的 `parsed`，断言 Pass/Reject 与错误体格式。

**门禁**：`go test -tags=unit ./internal/service/...`；e2e 发流式请求到 bedrock-compat 账号断言收到 400 而非半截流。

### P3 — BedrockFixAdapter 流式 EventStream（阶段二；最大工作量）
> ⚠️ **前置硬门禁**：先抓**真实 AWS Bedrock `ConverseStream`** 二进制金样本（§5.4 第 2 层），对拍帧布局/CRC/header 大小写/`:event-type`/`:message-type`。**未取到金样本不动编码器**。
- `conversecompat/eventstream.go`：
  - `EncodeEventStreamFrame(eventType, payloadJSON) []byte`（§5.4 ③ 帧结构，大端，prelude_crc + message_crc）。
  - `decodeFrameRaw(frame)`（**新抽纯帧 decoder**，只解布局+CRC、**不校验 `:event-type` 白名单**，§5.4 ③ review）——专供往返测试，**不复用** `bedrockEventStreamDecoder`（它只认 `chunk`）。
  - `eventstream_test.go`：`decodeFrameRaw(EncodeEventStreamFrame(x))==x` 自洽 + 与 AWS 金样本逐字节对拍。
- `conversecompat/stream.go`：native 事件 → Converse 事件映射（§5.4 ③：`message_start→messageStart`、`content_block_*`、`input_json_delta→delta.toolUse.input`、`thinking_delta/signature_delta`、`message_delta→messageStop`、`ping`/未知跳过），含 usage 标量缓存（input@message_start、output@message_delta）。
- `BedrockFixAdapter` 流式三接口落地：
  - `StreamContentType()` = `application/vnd.amazon.eventstream`。
  - `EmitStreamEvent(w, et, data)` = 映射 + `EncodeEventStreamFrame` + 写真实 `w`。
  - `FinishStream(w)` = 合成并写 `metadata{usage,metrics{latencyMs}}` 帧。
- **流式接入**（§5.2，三条流式路径）：通用 `handleStreamingResponse:6714`（masking@`:6972`）、APIKey 直通 `*Passthrough:4738`（@`:4847`）、Bedrock `bedrock_stream.go:25`。每路径：开流用 `StreamContentType()` 覆盖 Content-Type；每事件**先 `parseSSEUsagePassthrough`（计费）→ 再 `EmitStreamEvent`**（§6 review 修正的「提取→写帧→合并」顺序）；流末 `FinishStream`。
- **移除 P2 的 `stream:true → Reject` 闸**。
- **守卫约束**（§6 review）：只写真实 `c.Writer`、不包裹/不缓冲整流、开始写帧后只能写完或写 `:message-type=exception` 帧、不回退 failover。

**门禁**：`eventstream_test` 往返 + AWS 金样本对拍通过；`go test -tags=unit ./internal/pkg/claude/conversecompat/...`；真实流式请求对拍。

### P4 — KiroCompatAdapter 零回归接壳（不改现状行为）
- `account_adapter.go` `KiroCompatAdapter`：
  - `InspectRequest` = 恒 `Pass`（暂不做能力判定）。
  - `CorrectNonStreamResponse` = **现有 `maskResponseBody` 行为**（仅身份遮蔽），等价于今天的 `response_masking`，**零回归**。
  - 流式 `EmitStreamEvent` = 输出标准 Anthropic SSE 文本（与无 adapter 路径一致）。
- 目的：把 Kiro 纳入统一 adapter 框架但**不改任何可观测行为**，为 P5 留接口。

**门禁**：现有 `response_masking` 相关测试全绿（行为不变即通过）。

### P5 — KiroCompatAdapter 归一 + 同组剔除（**前置：先采 Kiro 真实样本**）
> ⚠️ **前置硬门禁**：先按 §5.3 review 的 5 张规格表采集 Kiro 真实非流式/流式样本（仿 §8 fixture）。**样本未到位不实现归一**。
- `backend/internal/pkg/claude/kirocompat/`（仿 conversecompat）：Kiro→标准 Anthropic 归一（非流式 + 流式事件），含 usage/error/content/stop_reason 映射表（§5.3 ①②④）。
- `KiroCompatAdapter.CorrectNonStreamResponse/EmitStreamEvent` 改为「身份遮蔽 + 归一」。
- 请求能力判定：补全 `SniffAnthropic`，接入 `Forward` 的 `InspectRequest`；命中 Kiro 不支持能力（如 `documents`）→ **`Reroute`（同组排除换号）**，按 §4.2 review 的 `RerouteError` + 换号前清理协议落地（handler `gateway_handler.go` 选号循环 `:530-885` 识别 `RerouteError`）。
- **不碰跨分组兜底**（独立方案）。
- 单测：Kiro 样本对拍归一结果；reroute 路径覆盖并发槽/sticky 释放。

**门禁**：`kirocompat` 对拍 + reroute 单测；e2e 验证 Kiro 账号归一响应与同组剔除。

### 阶段依赖图

```mermaid
flowchart TD
    P0["P0 地基<br/>开关 + adapter 抽象 + pickAdapter<br/>+ 共享 helper + 前端开关"]

    subgraph Bedrock 轨（需求 A）
        P2a["P2a · stream:true → Reject 闸<br/>(P1 安全上线前提)"]
        P1["P1 · Bedrock 非流式映射<br/>conversecompat/mapping.go"]
        P2b["P2b · 能力清单 Reject<br/>documents / tool_choice…"]
        P3["P3 · Bedrock 流式 EventStream<br/>encoder + 纯帧 decoder + stream.go"]
        G3{{"外部门禁<br/>AWS ConverseStream 真实金样本"}}
    end

    subgraph Kiro 轨（需求 B）
        P4["P4 · Kiro 零回归接壳<br/>退化为现有 response_masking"]
        P5["P5 · Kiro 归一 + 同组剔除<br/>kirocompat + SniffAnthropic + RerouteError"]
        G5{{"外部门禁<br/>Kiro 上游真实样本"}}
    end

    X["跨分组兜底<br/>独立方案 · 另评（不在 P0–P5）"]

    P0 --> P2a
    P0 --> P2b
    P0 --> P1
    P2a --> P1
    P1 --> P3
    P2b --> P3
    G3 --> P3
    P3 -.->|上线后移除| P2a
    P0 --> P4
    P4 --> P5
    G5 --> P5
    P5 --> X
```

**读图要点**：
- **两条独立轨在 P0 后并行**：Bedrock 轨（P2a→P1→P3）与 Kiro 轨（P4→P5）互不依赖，可分人/分时推进。
- **P0 是唯一全局前置**：开关 + 抽象 + helper 没落地，两轨都无法挂。
- **P1 的"流式不实现"靠 P2a 兜底**：故 **P2a 必须先于 P1 上线**（否则打 flag 账号发流式请求会撞到未实现接口）；P1 与 P2b（完整能力清单）可并行。
- **两个外部硬门禁阻塞各自轨的深水区**：`G3`（AWS 真实二进制样本）不到位 → P3 不动；`G5`（Kiro 真实样本）不到位 → P5 不动。两者都需提前安排取样，是关键路径上的长周期项。
- **P3 上线后回收 P2a 的临时闸**（虚线），流式 Converse 正式可用。
- **跨分组兜底**依赖 P5 落地的 `RerouteError` 机制，但本身是独立评审项，不计入 P0–P5 工期。

**最短可交付路径（先出价值）**：`P0 → P2a → P1` 即可让「bedrock 兼容」账号在**非流式**场景返回 Converse JSON，无 EventStream、无 Kiro、无跨组，风险面最小。

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
| `stop_reason` | `stopReason` | **5 值同名**(`end_turn/max_tokens/stop_sequence/tool_use/model_context_window_exceeded`)；`pause_turn/refusal` 无 Converse 对应，需显式决定（建议 `content_filtered`/`end_turn`）|
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

---

## 12. codex review 小结（2026-06-27）

一轮 codex 真实代码 review 结论：**方案有条件成立**——响应侧挂载点行号全部真实（±5 行内核验），Bedrock EventStream 编码方向与现有 decoder 结构相容，框架可落地；但**进实现前**需先回填以下缺口（已就地写入对应章节，此处汇总）。

**已就地修正/补全的 7 项**：
1. **Bedrock 映射枚举补全**（§5.4 ②）：`model_context_window_exceeded` 同名直传、`server_tool_use` 按 tool_use 映射、`tool_result` 仅请求侧、未知 content/stop_reason 兜底降级。
2. **EventStream 验证方案修正**（§5.4 ③）：现有 `bedrockEventStreamDecoder` 只认 `:event-type=="chunk"`，**不能**做 Converse 多事件的 `decode(encode(x))==x`——改为「新抽纯帧 decoder 验证自洽 + 真实 ConverseStream 金样本验证 AWS 一致性」。
3. **Kiro 侧 = 占位**（§5.3）：现状只有 `response_masking`，归一/能力判定/兜底全无；必须真实样本驱动产出可测规格后再实现。
4. **同组 reroute 交互协议**（§4.2）：需新增 `RerouteError`（区别于 `UpstreamFailoverError`）+ 换号前清理（并发槽/sticky/串行锁）+ 死循环防护。
5. **跨分组兜底单独成方案**（§4.2）：牵涉 channelMapping/订阅/配额/sticky/队列/日志 6 项口径，**不在 P0–P5 范围内**。
6. **通用流式 usage 顺序**（§3、§5.2、§6）：实为"写出 block 后 merge"，须固定为「提取→写帧→合并」。
7. **路径更正**（§1.4、§3）：`IsResponseMaskingEnabled` 在 `backend/internal/service/account.go:1443`，非 `model/account.go`。

**建议的更简落地路径（分阶段降风险）**：
- **阶段一**：只做 BedrockFixAdapter **非流式** 响应 helper，挂现有三条响应路径；流式 Converse **先明确 Reject**（`InspectRequest` 对 `stream:true` 报错"暂不支持流式 Converse"）。无 EventStream、无跨组兜底，风险边界最小。
- **阶段二**：补 EventStream 二进制流（前置门禁：先抓真实 AWS ConverseStream 金样本，§5.4）。
- **Kiro**：先让 KiroCompatAdapter **退化为现有 `response_masking`**（零回归）；真实样本到位后再补归一表 + `SniffAnthropic` 同组硬剔除；**不碰跨分组兜底**。

> 对照原始目标：**Anthropic→Bedrock 响应**方向设计成立、可落地（补全映射 + 换验证方案即可推进）；**Kiro→Anthropic** 方向当前仍是占位，**必须先采 Kiro 真实样本列偏差清单**，否则架构无支撑。

---

## 13. 前端「bedrock 兼容」开关 — 后台加账号页面的明确修改建议（P0 前端部分）

> 基于实读真实代码（CreateAccountModal.vue / EditAccountModal.vue / BulkEditAccountModal.vue / zh.ts，2026-06-27）。新开关**严格仿照现有「响应遮蔽（Kiro 兼容）」开关**，存 `extra.bedrock_compat`。下列行号为当前值，实现时以实际为准。

### 13.0 两个必须先澄清的事实（避免踩坑）
1. **`accountCategory==='bedrock'` 已存在**（CreateAccountModal.vue:174，amber 色 + cloud 图标，文案 `admin.accounts.bedrockLabel/bedrockDesc`）——那是「**AWS Bedrock 上游账号**」的账号类型分类。本方案的「bedrock 兼容」是**出站响应改写开关**（把响应转成 Converse 形态返回客户端），**与账号是不是 AWS Bedrock 类型无关**。两者概念不同，**i18n key 与 UI 文案必须明显区分**，否则管理员会混淆。
2. **BulkEditAccountModal 当前不含 `response_masking`**（实测 grep 为空）——所以 §1.4 写的"BulkEditAccountModal 同步"**不准确**：现状批量编辑根本没有这类 extra-flag 开关。建议 `bedrock_compat` **第一版同样不进 BulkEdit**（与 masking 现状保持一致，避免范围蔓延）；若将来要批量设，masking 与 bedrock_compat 一起补一套"批量 extra flag"机制。**[修正 §1.4]**

### 13.1 显示条件（与 masking 同款）
开关在 `form.platform === 'anthropic'` 时显示（不限 `accountCategory`），与现有 masking 开关并列。语义：任何 Anthropic 平台账号都可开启"把返回给客户端的响应改写成 Converse 形态"。

### 13.2 CreateAccountModal.vue（4 处）
**① template** — 在现有 masking 开关 `div` 之后（约 `:1778` 结束的 `</div>` 后）紧接插入并列块：
```vue
<!-- Anthropic: Bedrock Converse 兼容（出站响应改写，与 AWS Bedrock 账号类型无关） -->
<div
  v-if="form.platform === 'anthropic'"
  class="border-t border-gray-200 pt-4 dark:border-dark-600"
>
  <div class="flex items-center justify-between">
    <div>
      <label class="input-label mb-0">{{ t('admin.accounts.anthropic.bedrockCompat') }}</label>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.anthropic.bedrockCompatDesc') }}
      </p>
    </div>
    <button
      type="button"
      @click="toggleBedrockCompat()"
      :class="[
        'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
        bedrockCompatEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
      ]"
    >
      <span
        :class="[
          'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
          bedrockCompatEnabled ? 'translate-x-5' : 'translate-x-0'
        ]"
      />
    </button>
  </div>
</div>
```
**② ref** — 在 `const responseMaskingEnabled = ref(false)`（`:2356`）后加：
```ts
const bedrockCompatEnabled = ref(false)
// 互斥：与 Kiro 兼容（response_masking）不能同时开（§2）
function toggleBedrockCompat() {
  bedrockCompatEnabled.value = !bedrockCompatEnabled.value
  if (bedrockCompatEnabled.value) responseMaskingEnabled.value = false
}
```
（对称地，建议把 masking 的 `@click` 也改为开启时关掉 `bedrockCompatEnabled`，实现双向互斥。）

**③ save** — 在 `if (form.platform === 'anthropic') { … response_masking … }`（`:3146-3151`）块内、masking 处理之后追加：
```ts
    if (bedrockCompatEnabled.value) {
      extra.bedrock_compat = true
    } else {
      delete extra.bedrock_compat
    }
```

### 13.3 EditAccountModal.vue（4 处，多一处 load）
- **① template**：同 13.2 ①，插在 masking 块（约 `:1440` 结束）之后。
- **② ref**：`const responseMaskingEnabled = ref(false)`（`:2057`）后加 `bedrockCompatEnabled` + `toggleBedrockCompat()`（同 13.2 ②）。
- **③ load**：在 `if (newAccount.platform === 'anthropic') { responseMaskingEnabled.value = … }`（`:2461-2463`）块内追加：
```ts
    bedrockCompatEnabled.value = extra?.bedrock_compat === true
```
- **④ save**：在 `props.account.platform === 'anthropic'` 的 `newExtra` 段（`:3179-3186`）内、masking 之后追加：
```ts
      if (bedrockCompatEnabled.value) {
        newExtra.bedrock_compat = true
      } else {
        delete newExtra.bedrock_compat
      }
```

### 13.4 i18n（zh.ts / en.ts 等所有 locale）
在 `admin.accounts.anthropic` 下、`responseMasking/responseMaskingDesc`（zh.ts `:3761`）旁新增（**注意与顶层 `admin.accounts.bedrockLabel` 区分**）：
```ts
// zh.ts
bedrockCompat: 'Bedrock Converse 兼容',
bedrockCompatDesc:
  '把返回给客户端的响应改写为 AWS Bedrock Converse 协议形态（字段重映射、流式二进制 EventStream）。用于客户端按 Converse 协议消费的场景，与账号是否为 AWS Bedrock 类型无关。与「响应遮蔽（Kiro 兼容）」互斥。默认关闭。',
```
```ts
// en.ts
bedrockCompat: 'Bedrock Converse Compatibility',
bedrockCompatDesc:
  'Rewrites the response returned to the client into AWS Bedrock Converse shape (field remapping, binary EventStream for streaming). For clients that consume the Converse protocol; independent of whether the account is an AWS Bedrock account. Mutually exclusive with "Response Masking (Kiro Compat)". Disabled by default.',
```

### 13.5 vitest
仿现有 masking 开关的组件测试补：开关默认关、toggle 写入/删除 `extra.bedrock_compat`、与 `response_masking` 互斥（开一个关另一个）、Edit 的 load 回填。

### 13.6 前端改动小结
| 文件 | 改动 |
|---|---|
| `CreateAccountModal.vue` | template 开关块 + `bedrockCompatEnabled` ref + `toggleBedrockCompat` 互斥 + save 写 `extra.bedrock_compat` |
| `EditAccountModal.vue` | 同上 + load 回填 |
| `BulkEditAccountModal.vue` | **不改**（与 masking 现状一致，§13.0②） |
| `zh.ts` / `en.ts`（全 locale） | 新增 `bedrockCompat` / `bedrockCompatDesc`，文案强调"与 AWS Bedrock 账号类型无关 + 与 Kiro 互斥" |
| `*.spec.ts` | 开关默认关 / toggle / 互斥 / load 回填 |

---

## 14. 实现落地状态（2026-06-28）

全部 P0–P5 已实现并经真实上游 e2e 验证。提交：阶段一 `47bf3ed5` · P3 `c510bd8c` · P5 `34284e21`（分支 `feature/bedrock-converse-compat`）。

### 14.1 已落地

| 批次 | 内容 | 验证 |
|------|------|------|
| **P0** | `IsBedrockCompatEnabled()` + `account_adapter.go`（接口/pickAdapter/helper）+ Forward gating + 前端开关（与 Kiro 互斥） | 单测 + e2e PASS=14 |
| **P1** | `conversecompat/mapping.go` 非流式 Anthropic→Converse（表 A 全量）+ 三路接入 | golden 对拍 + e2e Converse JSON |
| **P2** | `InspectRequest`：document 块 → Reject | 单测 |
| **P3** | `conversecompat/eventstream.go`（编码器 + 纯帧 decoder）+ `stream.go`（事件映射）+ 通用流式接管 | 往返自洽 + **现有 bedrockEventStreamDecoder 交叉验证** + e2e 流式二进制帧 |
| **P4→P5** | `KiroCompatAdapter` 归一（见 14.2） | 单测 + 真实 kiro e2e |

### 14.2 Kiro 真实偏差表（基于 openclaw kiro 分组实测，2026-06-28）

实测结论与设计假设不同：**Kiro 响应高度符合标准 Anthropic**，偏差仅两处，已由 `kirocompat.NormalizeNonStream` 修正：

| 偏差 | 位置 | 标准 Anthropic | Kiro 实测 | 归一动作 |
|------|------|---------------|----------|---------|
| 缺 `stop_reason` | 非流式 tool_use 响应 | `"tool_use"` | **字段缺失** | 补齐（有 tool_use→`tool_use`，否则 `end_turn`）|
| 多余 `usage.inference_geo` | text/probe 响应 | 无 | `"global"`/`"not_available"` | 移除 |

归一与现有 `response_masking` 身份遮蔽**正交**（改 usage/stop_reason vs 改文本），不移除 `needMask`、不双重处理。流式实测无 stop_reason 偏差（`message_delta` 正常带），故 KiroCompat 流式不接管。

### 14.3 能力探测结果（请求侧）

| 能力 | Kiro 行为 | 处理 |
|------|----------|------|
| `cache_control` | 支持（cache_creation_input_tokens>0） | 透传 |
| `tool_use` / `streaming` / `count_tokens` | 支持 | 透传 |
| `vision`(image) / `document` | **静默忽略**（200，模型看不到，不报错） | **reroute 兜底**（见 14.5） |
| `thinking` | 接受但不输出 thinking 块（降级） | 透传 |

### 14.4 与原设计的偏离（均有实测依据）

1. **P3 流式接管仅通用上游路径**：passthrough/AWS-Bedrock 两条流式路径不接管（逐行透传模型不适配按事件接口 + 非典型组合），非流式仍转 Converse。见 `streamAdapterFromCtx` 注释。
2. **reroute 改为跨分组兜底（非同组剔除）**：原设计假设「Kiro 报错→同组换号」，实测 Kiro 不硬拒绝能力（image/document 静默忽略）。按需求改为：Kiro 遇 image/document → **跨组 reroute 到兜底组**（如 Claude 官方组），见 14.5。

### 14.5 Kiro 能力兜底（跨组 reroute，task 12，commit `086be436`）

复用现有预留字段 `groups.fallback_group_id_on_invalid_request`（DB+CRUD+前端 UI 已有，网关此前未消费）。

- **触发**：`KiroCompatAdapter.InspectRequest` 检测请求含 image/document 块 → `ActionReroute`。
- **决策**：`Forward` 查当前组 `FallbackGroupIDOnInvalidRequest`——配了则返回 `RequestRerouteError` 让 handler 换组；**未配则放行**（退化为 Kiro 静默忽略现状，零回归）。
- **换组**：handler 选号循环捕获 `RequestRerouteError` → 换 `currentGroupID` + 重置 failover 状态 + 重进循环。防循环计数 max=1。
- **口径**：计费/quota 仍按**原 key**（用户同一 key），仅**选号组**变到兜底组。
- **安全性**（比 §4.2 review 担心的低风险）：reroute 在转发前（未写客户端字节，`writerSize` 守卫天然满足）；并发槽/串行锁已由 Forward 返回后的释放逻辑自动回收；换组后兜底组的官方账号 `adapter=nil`，不会再次 reroute（天然防无限循环）。
- **验证**：单测（image/document/text 检测）+ e2e 对比强证明——文本走 Kiro(200) / vision reroute 到官方组(官方真处理 image) / vision reroute 到空组(503，证明请求确实离开 Kiro)。

> **真正的跨分组兜底已落地**（推翻 §4.2 review「不在 P0–P5 范围」的保守结论）——得益于现有 `FallbackGroupIDOnInvalidRequest` 预留字段 + 槽/锁自动释放，实际风险远低于 review 预估。
