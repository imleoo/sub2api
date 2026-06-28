package service

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude/conversecompat"
)

// account_adapter.go —— 账号级协议适配器（Account Protocol Adapter）
//
// 选到账号后，按账号 flag 取一个 adapter；adapter 对请求做能力检查、对响应做针对性改写。
// 与现有 response_masking 同源（账号 flag + 选号后判断 + 响应侧处理），是其一般化。
// adapter=nil 的账号（绝大多数）链路零行为变化。
//
// 阶段一接口精简为「请求能力门 + 非流式响应改写」两个钩子；流式 Converse（二进制
// EventStream）留待后续批次（P3）再扩接口。

// ActionKind 是请求侧能力检查的结果类型。
type ActionKind int

const (
	// ActionPass 表示上游支持该请求，正常转发。
	ActionPass ActionKind = iota
	// ActionReject 表示上游不支持请求用到的能力，构造错误体直接回客户端（不 failover）。
	ActionReject
)

// RequestAction 是 InspectRequest 的返回：Pass 放行 / Reject 报错。
type RequestAction struct {
	Kind ActionKind
	Err  error // Reject 时携带，用于构造标准 Anthropic 错误体
}

// AccountProtocolAdapter 暴露请求侧 / 响应侧两个钩子。
type AccountProtocolAdapter interface {
	// InspectRequest 检查上游能力。返回 Pass（放行）或 Reject（报错）。
	InspectRequest(parsed *ParsedRequest) RequestAction
	// CorrectNonStreamResponse 把非流式响应体改写为目标形态。
	CorrectNonStreamResponse(body []byte) []byte
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

// 注：Reject 早返回复用现有的 writeAnthropicError（openai_gateway_messages.go），
// 签名 (c *gin.Context, statusCode int, errType, message string)，标准 Anthropic 错误体。

// KiroCompatAdapter —— Kiro 兼容适配器。
//
// 阶段一为「零回归接壳」：仅在 adapter 框架中在册，不接管任何响应改写。
// 现有 Kiro 行为仍由 response_masking 的 needMask 逻辑驱动（响应路径逐块
// maskResponseBody），故此处 CorrectNonStreamResponse 必须原样返回，避免双重 masking。
// 真正接管（身份遮蔽迁入 adapter + 响应归一标准 Anthropic）留待后续批次（P5）。
type KiroCompatAdapter struct{}

func newKiroCompatAdapter() *KiroCompatAdapter { return &KiroCompatAdapter{} }

func (k *KiroCompatAdapter) InspectRequest(_ *ParsedRequest) RequestAction {
	return RequestAction{Kind: ActionPass}
}

func (k *KiroCompatAdapter) CorrectNonStreamResponse(body []byte) []byte {
	return body // 不接管：现有 needMask 路径负责，避免双重 masking
}

// BedrockFixAdapter —— Bedrock Converse 兼容适配器。
//
// 把标准 Anthropic 响应改写为 AWS Bedrock Converse 形态返回客户端。
//   - InspectRequest（P2）：流式请求 + 上游不支持的能力 → Reject。
//   - CorrectNonStreamResponse（P1）：native JSON → Converse JSON。
//
// 流式 Converse（二进制 EventStream）留待后续批次（P3）；在此之前由 InspectRequest
// 对流式请求直接 Reject 兜底。
type BedrockFixAdapter struct{}

func newBedrockFixAdapter() *BedrockFixAdapter { return &BedrockFixAdapter{} }

func (b *BedrockFixAdapter) InspectRequest(parsed *ParsedRequest) RequestAction {
	if parsed == nil {
		return RequestAction{Kind: ActionPass}
	}
	// 流式 Converse（二进制 EventStream）尚未实现 → 先明确 Reject（后续批次 P3 落地后移除此条）。
	if parsed.Stream {
		return RequestAction{
			Kind: ActionReject,
			Err:  errors.New("streaming responses are not yet supported in Bedrock Converse compatibility mode"),
		}
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
	if len(messagesRaw) == 0 {
		return false
	}
	found := false
	gjson.ParseBytes(messagesRaw).ForEach(func(_, msg gjson.Result) bool {
		content := msg.Get("content")
		if content.IsArray() {
			content.ForEach(func(_, block gjson.Result) bool {
				if block.Get("type").String() == "document" {
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
