//go:build unit

package service

import "testing"

func TestIsModelVisibleToUser(t *testing.T) {
	cases := []struct {
		name         string
		m            ModelInfo
		showOverseas bool
		want         bool
	}{
		{"claude below floor hidden", ModelInfo{ID: "claude-3-opus-20240229", LiteLLMProvider: "anthropic"}, true, false},
		// claude-3-5-sonnet 经正则解析为 5.0（(?:\w+-)? 把 "3-" 当家族名），与前端同一行为 → 显示。
		{"claude-3-5 parses as 5.0 (frontend parity)", ModelInfo{ID: "claude-3-5-sonnet-20241022", LiteLLMProvider: "anthropic"}, true, true},
		{"claude at floor shown", ModelInfo{ID: "claude-sonnet-4-5-20250929", LiteLLMProvider: "anthropic"}, true, true},
		{"gpt below floor hidden", ModelInfo{ID: "gpt-4o", LiteLLMProvider: "openai"}, true, false},
		{"gpt at floor shown", ModelInfo{ID: "gpt-5.2", LiteLLMProvider: "openai"}, true, true},
		{"o1 reasoning hidden", ModelInfo{ID: "o1-mini", LiteLLMProvider: "openai"}, true, false},
		{"gemini below floor hidden", ModelInfo{ID: "gemini-2.5-flash", LiteLLMProvider: "google"}, true, false},
		{"gemini at floor shown", ModelInfo{ID: "gemini-3-pro-preview", LiteLLMProvider: "google"}, true, true},
		{"domestic always shown", ModelInfo{ID: "deepseek-v3-2", LiteLLMProvider: "deepseek"}, true, true},
		{"domestic image shown", ModelInfo{ID: "doubao-seedream-4-0", Mode: "image_generation"}, true, true},
		{"overseas image shown when toggle on", ModelInfo{ID: "gpt-image-1", Mode: "image_generation", LiteLLMProvider: "openai"}, true, true},
		{"overseas hidden when toggle off", ModelInfo{ID: "gpt-5.2", LiteLLMProvider: "openai"}, false, false},
		{"overseas image hidden when toggle off", ModelInfo{ID: "gpt-image-1", Mode: "image_generation"}, false, false},
		{"other overseas hidden", ModelInfo{ID: "mistral-large", LiteLLMProvider: "mistral"}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isModelVisibleToUser(c.m, c.showOverseas); got != c.want {
				t.Errorf("isModelVisibleToUser(%q, overseas=%v)=%v, want %v", c.m.ID, c.showOverseas, got, c.want)
			}
		})
	}
}
