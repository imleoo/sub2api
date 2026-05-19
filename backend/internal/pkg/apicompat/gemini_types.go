// Phase 4 P4-2：Gemini API 类型定义。
//
// 覆盖 Google Gemini v1beta generateContent / streamGenerateContent API 的最小类型集，
// 足以支撑 gemini_v1beta→openai_chat / gemini_v1beta→anthropic_messages 两座桥的
// 请求/响应/流式签名定义。
//
// 参考规范：https://ai.google.dev/api/generate-content（v1beta）
// 字段子集原则：只定义桥层转换实际需要的字段；前端/协议校验不在此层做。
package apicompat

import "encoding/json"

// ---------------------------------------------------------------------------
// Gemini generateContent request types
// ---------------------------------------------------------------------------

// GeminiGenerateContentRequest 对应 POST /v1beta/models/{model}:generateContent。
type GeminiGenerateContentRequest struct {
	// Contents 多轮对话历史，每条含 role + parts。
	Contents []GeminiContent `json:"contents"`

	// Tools 可选工具声明（Gemini function calling）。
	Tools []GeminiTool `json:"tools,omitempty"`

	// SystemInstruction 系统提示（单条 Content，role 通常省略）。
	SystemInstruction *GeminiContent `json:"systemInstruction,omitempty"`

	// GenerationConfig 生成参数（温度、最大 token、停止序列等）。
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`

	// Stream 由调用方在 URL 段选择（generateContent vs streamGenerateContent）；
	// 此字段仅在桥内部标记，不序列化到上游请求。
	Stream bool `json:"-"`
}

// GeminiContent 一轮对话的内容（role + parts）。
type GeminiContent struct {
	// Role 取值："user" | "model"（Gemini 侧）或省略（systemInstruction）。
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart 消息体的最小单元；三种形态互斥（text / inlineData / functionCall / functionResponse）。
type GeminiPart struct {
	// Text 纯文本。
	Text string `json:"text,omitempty"`

	// InlineData 内联二进制数据（图像/文档）。
	InlineData *GeminiBlob `json:"inlineData,omitempty"`

	// FunctionCall 模型输出的函数调用（tool_use 方向）。
	FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`

	// FunctionResponse 用户侧工具结果回传（tool_result 方向）。
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
}

// GeminiBlob 内联数据（图像、视频帧等）。
type GeminiBlob struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64-encoded
}

// GeminiFunctionCall 模型发起的函数调用。
type GeminiFunctionCall struct {
	// Name 函数名，与 GeminiFunctionDeclaration.Name 对应。
	Name string `json:"name"`
	// Args JSON 对象，key = 参数名，value = 参数值。
	Args json.RawMessage `json:"args,omitempty"`
}

// GeminiFunctionResponse 用户侧工具结果。
type GeminiFunctionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

// GeminiTool 工具声明容器；Gemini API 按 Tool 对象分组 functionDeclarations。
type GeminiTool struct {
	// FunctionDeclarations OpenAI 侧 tools[].function 的 Gemini 等价物。
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

// GeminiFunctionDeclaration 单个函数声明（对应 OpenAI ChatFunction / Anthropic AnthropicTool）。
//
// 字段含义严格遵循 Gemini API 规范；桥层转换映射见下方注释。
//
// Gemini → OpenAI ChatFunction 字段映射：
//   - Name        → function.name
//   - Description → function.description
//   - Parameters  → function.parameters（JSON Schema，格式相同）
//
// Gemini → Anthropic AnthropicTool 字段映射：
//   - Name        → name
//   - Description → description
//   - Parameters  → input_schema（JSON Schema，格式相同）
type GeminiFunctionDeclaration struct {
	// Name 函数名（必填，规范：[a-zA-Z][a-zA-Z0-9_]*，最长 64 字符）。
	Name string `json:"name"`
	// Description 函数描述（可选；模型根据描述判断何时调用）。
	Description string `json:"description,omitempty"`
	// Parameters JSON Schema 对象（type=object，properties/required 字段）。
	// Gemini 与 OpenAI / Anthropic 使用相同的 JSON Schema Draft-07 子集，
	// 因此参数层可直接透传，零损失。
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

// GeminiGenerationConfig 生成参数。
type GeminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
	CandidateCount  *int     `json:"candidateCount,omitempty"`
}

// ---------------------------------------------------------------------------
// Gemini generateContent response types（非流式 + 流式 chunk 共用）
// ---------------------------------------------------------------------------

// GeminiGenerateContentResponse 对应 generateContent 完整响应或 streamGenerateContent 单个 SSE chunk。
//
// streamGenerateContent 的每条 SSE 事件均为独立的此结构体序列化（`data: {...}`）；
// 最后一条 chunk 包含 finishReason 和 usageMetadata。
type GeminiGenerateContentResponse struct {
	// Candidates 候选回答列表（通常 candidateCount=1，故只有 1 条）。
	Candidates []GeminiCandidate `json:"candidates,omitempty"`

	// UsageMetadata token 用量（流式时仅最后一条 chunk 携带）。
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`

	// Error 非零时表示上游返回了错误（Gemini 错误格式）。
	Error *GeminiAPIError `json:"error,omitempty"`
}

// GeminiCandidate 单个候选回答。
type GeminiCandidate struct {
	// Content 本 turn 的模型输出内容。
	Content GeminiContent `json:"content"`

	// FinishReason 停止原因：STOP | MAX_TOKENS | SAFETY | RECITATION | OTHER（流式：仅最后 chunk）。
	FinishReason string `json:"finishReason,omitempty"`

	// Index 候选序号（通常 0）。
	Index int `json:"index"`

	// SafetyRatings 安全评级（可选，透传不处理）。
	SafetyRatings []json.RawMessage `json:"safetyRatings,omitempty"`
}

// GeminiUsageMetadata token 用量统计。
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GeminiAPIError Gemini API 错误结构（桥层需要将其映射到 HTTP 5xx / 4xx）。
type GeminiAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}
