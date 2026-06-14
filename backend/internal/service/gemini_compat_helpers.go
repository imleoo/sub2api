package service

import (
	"encoding/json"
	"strings"
)

// isGoogleProjectConfigError 检测 Google 间歇性 Bug：Project ID 有效但被临时识别失败。
func isGoogleProjectConfigError(lowerMsg string) bool {
	return strings.Contains(lowerMsg, "invalid project resource name")
}

// filterEmptyPartsFromGeminiRequest 过滤掉 Gemini 请求体中 parts 为空的消息
// （Gemini API 不接受空 parts）。
func filterEmptyPartsFromGeminiRequest(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	contents, ok := payload["contents"].([]any)
	if !ok || len(contents) == 0 {
		return body, nil
	}

	filtered := make([]any, 0, len(contents))
	modified := false

	for _, c := range contents {
		contentMap, ok := c.(map[string]any)
		if !ok {
			filtered = append(filtered, c)
			continue
		}

		parts, hasParts := contentMap["parts"]
		if !hasParts {
			filtered = append(filtered, c)
			continue
		}

		partsSlice, ok := parts.([]any)
		if !ok {
			filtered = append(filtered, c)
			continue
		}

		// 跳过 parts 为空数组的消息
		if len(partsSlice) == 0 {
			modified = true
			continue
		}

		filtered = append(filtered, c)
	}

	if !modified {
		return body, nil
	}

	payload["contents"] = filtered
	return json.Marshal(payload)
}
