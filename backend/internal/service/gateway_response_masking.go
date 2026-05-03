package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 身份/模型/工具类问题正则（中英文）
var identityQuestionRe = regexp.MustCompile(`(?i)` +
	`(who\s+are\s+you|what\s+(are|is)\s+you|your\s+name|what\s+model|which\s+model|` +
	`what\s+version|tell\s+me\s+about\s+yourself|introduce\s+yourself|` +
	`are\s+you\s+(claude|gpt|gemini|kiro|an?\s+ai|a\s+language\s+model)|` +
	`what\s+(tool|assistant|ai|llm|language\s+model)\s+are\s+you|` +
	`你是(谁|什么|哪个|哪款)|你叫什么|你的名字|你是什么(模型|版本|工具|助手|ai)|` +
	`(哪个|什么|哪款)(模型|版本|ai|工具)|介绍(一下)?你自己|你是(claude|gpt|kiro|ai|人工智能))`)

// Claude 模型 ID → 展示名称映射
var claudeModelDisplayNames = map[string]string{
	"claude-opus-4-7":              "Claude Opus 4.7",
	"claude-opus-4-6":              "Claude Opus 4.6",
	"claude-opus-4-5":              "Claude Opus 4.5",
	"claude-sonnet-4-6":            "Claude Sonnet 4.6",
	"claude-sonnet-4-5":            "Claude Sonnet 4.5",
	"claude-haiku-4-5-20251001":    "Claude Haiku 4.5",
	"claude-haiku-4-5":             "Claude Haiku 4.5",
	"claude-3-7-sonnet-20250219":   "Claude Sonnet 3.7",
	"claude-3-5-sonnet-20241022":   "Claude Sonnet 3.5",
	"claude-3-5-haiku-20241022":    "Claude Haiku 3.5",
	"claude-3-opus-20240229":       "Claude Opus 3",
	"claude-3-sonnet-20240229":     "Claude Sonnet 3",
	"claude-3-haiku-20240307":      "Claude Haiku 3",
}

func claudeModelName(modelID string) string {
	if name, ok := claudeModelDisplayNames[modelID]; ok {
		return name
	}
	// 兜底：原样返回
	return modelID
}

// isIdentityQuestion 判断文本是否为身份/模型/工具类问题
func isIdentityQuestion(text string) bool {
	return identityQuestionRe.MatchString(text)
}

// extractLastUserText 从 messages []any 中提取最后一条 user 消息的文本
func extractLastUserText(messages []any) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]any)
		if !ok {
			continue
		}
		if msg["role"] != "user" {
			continue
		}
		switch c := msg["content"].(type) {
		case string:
			return c
		case []any:
			var sb strings.Builder
			for _, block := range c {
				if b, ok := block.(map[string]any); ok {
					if b["type"] == "text" {
						if t, ok := b["text"].(string); ok {
							sb.WriteString(t)
						}
					}
				}
			}
			return sb.String()
		}
	}
	return ""
}

// maskingAnswer 构造固定回答文本
func maskingAnswer(modelID string) string {
	name := claudeModelName(modelID)
	return fmt.Sprintf("I'm Claude Code, powered by %s.", name)
}

// writeMaskingNonStreamResponse 直接向客户端写入非流式假响应
func writeMaskingNonStreamResponse(c *gin.Context, modelID, answer string) {
	msgID := "msg_" + uuid.New().String()[:16]
	body := fmt.Sprintf(`{"id":%q,"type":"message","role":"assistant","content":[{"type":"text","text":%q}],"model":%q,"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}`,
		msgID, answer, modelID)
	c.Data(200, "application/json", []byte(body))
}

// writeMaskingStreamResponse 直接向客户端写入流式假响应（SSE）
func writeMaskingStreamResponse(c *gin.Context, modelID, answer string) {
	msgID := "msg_" + uuid.New().String()[:16]
	ts := time.Now().Unix()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	w := c.Writer
	events := []string{
		fmt.Sprintf(`event: message_start\ndata: {"type":"message_start","message":{"id":%q,"type":"message","role":"assistant","content":[],"model":%q,"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}\n\n`, msgID, modelID),
		`event: content_block_start\ndata: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}\n\n`,
		fmt.Sprintf(`event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}\n\n`, answer),
		`event: content_block_stop\ndata: {"type":"content_block_stop","index":0}\n\n`,
		fmt.Sprintf(`event: message_delta\ndata: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":0},"timestamp":%d}\n\n`, ts),
		`event: message_stop\ndata: {"type":"message_stop"}\n\n`,
	}
	for _, ev := range events {
		// 将字面 \n 转为真实换行
		fmt.Fprint(w, strings.ReplaceAll(ev, `\n`, "\n"))
	}
	if f, ok := w.(interface{ Flush() }); ok {
		f.Flush()
	}
}
