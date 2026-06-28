package service

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude/conversecompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude/kirocompat"
)

// account_adapter.go —— 账号级协议适配器（Account Protocol Adapter）
//
// 选到账号后，按账号 flag 取一个 adapter；adapter 对请求做能力检查、对响应做针对性改写。
// 与现有 response_masking 同源（账号 flag + 选号后判断 + 响应侧处理），是其一般化。
// adapter=nil 的账号（绝大多数）链路零行为变化。
//
// 接口含「请求能力门 + 非流式响应改写 + 流式接管」三组钩子。流式 Converse（二进制
// EventStream）已实现并挂在通用 handleStreamingResponse（接管范围见 streamAdapterFromCtx）。

// ActionKind 是请求侧能力检查的结果类型。
type ActionKind int

const (
	// ActionPass 表示上游支持该请求，正常转发。
	ActionPass ActionKind = iota
	// ActionReject 表示上游不支持请求用到的能力，构造错误体直接回客户端（不 failover）。
	ActionReject
	// ActionReroute 表示当前账号上游处理不了该能力，请求应换到兜底组（若配置了
	// FallbackGroupIDOnInvalidRequest）；未配兜底组时退化为放行。
	ActionReroute
)

// RequestAction 是 InspectRequest 的返回：Pass 放行 / Reject 报错。
type RequestAction struct {
	Kind ActionKind
	Err  error // Reject 时携带，用于构造标准 Anthropic 错误体
}

// AccountProtocolAdapter 暴露请求侧 / 响应侧钩子。
//
// 流式：当 StreamTakesOver()==true 时，adapter 接管出站写帧（自定 Content-Type，
// 逐事件 EmitStreamEvent，末尾 FinishStream）；为 false 时流式走原生逻辑（如 Kiro
// 仍走 needMask 路径），此时 Stream* 方法不会被调用。
// 每个请求一个 adapter 实例（pickAdapter 每次新建），故可持有 per-request 流式状态。
type AccountProtocolAdapter interface {
	// InspectRequest 检查上游能力。返回 Pass（放行）或 Reject（报错）。
	InspectRequest(parsed *ParsedRequest) RequestAction
	// CorrectNonStreamResponse 把非流式响应体改写为目标形态。
	CorrectNonStreamResponse(body []byte) []byte
	// StreamTakesOver 是否由 adapter 接管流式出站写帧。
	StreamTakesOver() bool
	// StreamContentType 出站 Content-Type（接管时覆盖默认 text/event-stream）。
	StreamContentType() string
	// EmitStreamEvent 把一条 native SSE 事件转目标格式并写真实 w。
	EmitStreamEvent(w io.Writer, eventType string, data []byte) error
	// FinishStream 流末尾收尾（Converse：合成并写 metadata{usage} 帧）。
	FinishStream(w io.Writer) error
}

// accountAdapterCtxKey 是 adapter 在 gin.Context 中的存取键。
const accountAdapterCtxKey = "account_adapter"

// pickAdapter 按账号上的两个独立开关选 adapter（互斥取其一；都关 → nil）。
// 两个开关同时打开属配置异常，按此 switch 取先匹配项（bedrock 优先）。
func pickAdapter(a *Account) AccountProtocolAdapter {
	if a == nil {
		return nil
	}
	switch {
	case a.IsBedrockCompatEnabled(): // accounts.extra.bedrock_compat（新增）
		return newBedrockFixAdapter()
	case a.IsResponseMaskingEnabled(): // accounts.extra.response_masking（现有「Kiro 兼容」开关）
		return newKiroCompatAdapter()
	default:
		return nil // 不适配，链路完全不变
	}
}

// applyAdapterNonStream 取 ctx 中的 adapter，对非流式响应体做改写。
// 无 adapter 时原样返回——供三条非流式响应路径在写出前统一调用。
func applyAdapterNonStream(c *gin.Context, body []byte) []byte {
	if c == nil {
		return body
	}
	if v, ok := c.Get(accountAdapterCtxKey); ok {
		if ad, ok := v.(AccountProtocolAdapter); ok && ad != nil {
			return ad.CorrectNonStreamResponse(body)
		}
	}
	return body
}

// streamAdapterFromCtx 返回接管流式的 adapter（仅当 StreamTakesOver），否则 nil。
//
// 接管点目前仅挂在**通用** handleStreamingResponse（bedrock_compat 的主路径：标准
// anthropic 上游账号走此路）。两条特殊路径**不接管**流式，按 native 输出：
//   - APIKey 直通 *AnthropicAPIKeyPassthrough：逐行透传模型与按事件接口不匹配；且
//     passthrough(原样直通) 与 bedrock_compat(改写响应) 语义冲突，属非典型组合。
//   - AWS Bedrock handleBedrockStreamingResponse：上游本就是 Bedrock，叠加 Converse 改写无意义。
//
// 即：这两类账号若打了 bedrock_compat，非流式仍转 Converse（三路都接 applyAdapterNonStream），
// 流式则保持 native。该限制是有意的范围控制（避免高风险重构边缘路径）。
func streamAdapterFromCtx(c *gin.Context) AccountProtocolAdapter {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(accountAdapterCtxKey); ok {
		if ad, ok := v.(AccountProtocolAdapter); ok && ad != nil && ad.StreamTakesOver() {
			return ad
		}
	}
	return nil
}

// 注：Reject 早返回复用现有的 writeAnthropicError（openai_gateway_messages.go），
// 签名 (c *gin.Context, statusCode int, errType, message string)，标准 Anthropic 错误体。

// KiroCompatAdapter —— Kiro 兼容适配器。
//
// 非流式响应归一为标准 Anthropic（P5，基于真实 Kiro 样本）：移除 usage.inference_geo、
// 补缺失的 stop_reason。归一与现有 response_masking 的身份遮蔽（needMask 逐块
// maskResponseBody）**正交**——归一改 usage/stop_reason，遮蔽改文本身份，互不冲突，
// 故不移除 needMask，也不会双重处理。
//
// 流式不接管（StreamTakesOver=false）：实测 Kiro 流式无 stop_reason 偏差，仅 usage 偶带
// inference_geo（多余字段，不破坏客户端解析），归一价值低、改造成本高，保持现状走 needMask。
type KiroCompatAdapter struct{}

func newKiroCompatAdapter() *KiroCompatAdapter { return &KiroCompatAdapter{} }

func (k *KiroCompatAdapter) InspectRequest(parsed *ParsedRequest) RequestAction {
	// 实测 Kiro 对 image(vision)/document 块**静默忽略**（200 但模型看不到内容）。
	// 若请求用到这两类能力 → ActionReroute：由 Forward 检查当前组是否配了
	// FallbackGroupIDOnInvalidRequest（如 Claude 官方组），配了则换组兜底，未配则放行
	// （退化为 Kiro 静默忽略的现状，零回归）。
	if parsed != nil && hasContentBlockType(parsed.MessagesRaw(), "image", "document") {
		return RequestAction{
			Kind: ActionReroute,
			Err:  errors.New("image/document blocks are silently ignored by Kiro; rerouting to fallback group"),
		}
	}
	return RequestAction{Kind: ActionPass}
}

func (k *KiroCompatAdapter) CorrectNonStreamResponse(body []byte) []byte {
	return kirocompat.NormalizeNonStream(body) // 归一为标准 Anthropic（与 needMask 正交）
}

// 流式不接管：保持现状走原生 needMask 路径（理由见类型注释）。
func (k *KiroCompatAdapter) StreamTakesOver() bool                           { return false }
func (k *KiroCompatAdapter) StreamContentType() string                       { return "text/event-stream" }
func (k *KiroCompatAdapter) EmitStreamEvent(io.Writer, string, []byte) error { return nil }
func (k *KiroCompatAdapter) FinishStream(io.Writer) error                    { return nil }

// BedrockFixAdapter —— Bedrock Converse 兼容适配器。
//
// 把标准 Anthropic 响应改写为 AWS Bedrock Converse 形态返回客户端。
//   - InspectRequest：上游不支持的能力（如 document 块）→ Reject。
//   - CorrectNonStreamResponse：native JSON → Converse JSON。
//   - Stream*：接管流式，输出 Converse 二进制 EventStream（usage 末尾 metadata 帧）。
type BedrockFixAdapter struct {
	streamConv *conversecompat.StreamConverter // per-request 流式 usage 缓存（懒初始化）
}

func newBedrockFixAdapter() *BedrockFixAdapter { return &BedrockFixAdapter{} }

func (b *BedrockFixAdapter) InspectRequest(parsed *ParsedRequest) RequestAction {
	if parsed == nil {
		return RequestAction{Kind: ActionPass}
	}
	// 上游不支持的能力（最小集：document 块）→ Reject。后续可在此集中扩充清单。
	if unsupported := bedrockUnsupportedCapability(parsed); unsupported != "" {
		return RequestAction{
			Kind: ActionReject,
			Err:  fmt.Errorf("'%s' content blocks are not supported by the Bedrock Converse upstream", unsupported),
		}
	}
	return RequestAction{Kind: ActionPass}
}

// bedrockUnsupportedCapability 返回请求命中的、Bedrock Converse 上游不支持的能力名；无则空串。
func bedrockUnsupportedCapability(parsed *ParsedRequest) string {
	if hasDocumentBlock(parsed.MessagesRaw()) {
		return "document"
	}
	return ""
}

// hasDocumentBlock 检测 messages 中是否存在 Anthropic document 内容块。
func hasDocumentBlock(messagesRaw []byte) bool {
	return hasContentBlockType(messagesRaw, "document")
}

// hasContentBlockType 检测 messages 中是否存在指定 type 的任一 content 块。
func hasContentBlockType(messagesRaw []byte, types ...string) bool {
	if len(messagesRaw) == 0 {
		return false
	}
	found := false
	gjson.ParseBytes(messagesRaw).ForEach(func(_, msg gjson.Result) bool {
		content := msg.Get("content")
		if content.IsArray() {
			content.ForEach(func(_, block gjson.Result) bool {
				if slices.Contains(types, block.Get("type").String()) {
					found = true
					return false // 停止内层遍历
				}
				return true
			})
		}
		return !found // found 后停止外层遍历
	})
	return found
}

func (b *BedrockFixAdapter) CorrectNonStreamResponse(body []byte) []byte {
	// native Anthropic JSON → Bedrock Converse JSON（§5.4②/§11.2 表 A）。
	return conversecompat.AnthropicToConverseJSON(body)
}

// 流式：接管出站写帧，输出 Converse 二进制 EventStream。
func (b *BedrockFixAdapter) StreamTakesOver() bool     { return true }
func (b *BedrockFixAdapter) StreamContentType() string { return "application/vnd.amazon.eventstream" }

func (b *BedrockFixAdapter) EmitStreamEvent(w io.Writer, eventType string, data []byte) error {
	if b.streamConv == nil {
		b.streamConv = conversecompat.NewStreamConverter()
	}
	frame := b.streamConv.Convert(eventType, data)
	if frame == nil {
		return nil // ping / message_stop / 未知 → 无对应 Converse 帧
	}
	_, err := w.Write(frame)
	return err
}

func (b *BedrockFixAdapter) FinishStream(w io.Writer) error {
	if b.streamConv == nil {
		b.streamConv = conversecompat.NewStreamConverter()
	}
	_, err := w.Write(b.streamConv.Finish()) // 末尾 metadata{usage} 帧
	return err
}
