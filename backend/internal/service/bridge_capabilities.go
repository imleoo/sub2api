package service

// Phase 3 P3-6 能力探测 + RequestFeatures + ScoreFit（骨架）。
//
// 完整工作量（plan 标 5 人天）：嗅探 → 调度集成 → sticky / failover / 候选三处 → 诊断日志。
// 当前 PR 落地骨架（类型 + 嗅探函数 + ScoreFit 纯函数）；调度路径接入留独立 PR。
//
// 参考：docs/relay-architecture-design.md §5.1 / §5.2 / §5.3。

import (
	"bytes"
	"encoding/json"
)

// FeatureID 是请求特性的稳定标识符。
type FeatureID string

const (
	FeatureText             FeatureID = "text"
	FeatureVision           FeatureID = "vision"
	FeatureDocument         FeatureID = "document"
	FeatureToolUse          FeatureID = "tool_use"
	FeatureStreaming        FeatureID = "streaming"
	FeatureCacheControl     FeatureID = "cache_control"
	FeatureExtendedThinking FeatureID = "extended_thinking"
	FeatureComputerUse      FeatureID = "computer_use"
	FeatureCitations        FeatureID = "citations"
	FeaturePromptCache      FeatureID = "prompt_cache"
)

// RequestFeatures 是请求经过嗅探后得到的特性集合。
//
// 设计意图（§5.1）：
//   - Required：缺这些特性请求无法走通（如 vision、document）
//   - Optional：有更好但缺也能走通（如 streaming 客户端可降级到非流式）
//   - 嗅探只在请求入口做一次，放进 context；sticky / 候选 / failover 三处复用
type RequestFeatures struct {
	Required map[FeatureID]bool
	Optional map[FeatureID]bool
}

// HasRequired 判断指定特性是否在 Required 集合。
func (f *RequestFeatures) HasRequired(id FeatureID) bool {
	if f == nil || f.Required == nil {
		return false
	}
	return f.Required[id]
}

// FeatureFit 是单个特性在某条桥上的兼容性等级（§5.2）。
type FeatureFit int

const (
	// FitUnknown 等价于 ResponsesSupportUnknown：默认乐观，让调度尝试
	FitUnknown FeatureFit = iota
	// FitNative 完整兼容，桥层无降级
	FitNative
	// FitLossy 转换后语义降级但仍可走通（如 extended_thinking → OpenAI reasoning）
	FitLossy
	// FitDropped 字段被丢弃但请求仍可走通（如 cache_control 元信息）
	FitDropped
	// FitRejected 不可走通，调度层应硬剔除
	FitRejected
)

// BridgeCapabilities 描述一条桥的所有特性兼容性矩阵（§5.2）。
//
// 实现路线：
//   - accounts.extra.bridge_capabilities JSON 持久化（与 openai_responses_supported 同模式）
//   - 异步探测任务回填（如 OpenAIResponsesProbe 类似的 BridgeCapabilitiesProbe，留独立 PR）
//   - 调度路径只读，FitUnknown 时按乐观策略尝试
type BridgeCapabilities struct {
	Levels map[FeatureID]FeatureFit
}

// LevelOf 返回特性的兼容性等级；未定义返回 FitUnknown（乐观默认）。
func (c *BridgeCapabilities) LevelOf(id FeatureID) FeatureFit {
	if c == nil || c.Levels == nil {
		return FitUnknown
	}
	return c.Levels[id]
}

// GroupFeaturePolicy 描述分组对桥层兼容性的策略偏好（§5.3）。
type GroupFeaturePolicy struct {
	// Strictness 全局策略：strict / lossy_ok / best_effort
	Strictness string
	// NativeOnlyFeatures 这些特性即便策略允许 Lossy 也必须 Native
	NativeOnlyFeatures []FeatureID
	// FallbackPolicy reject（无兼容路径直接拒绝）/ fallback（仅 best_effort 可用）
	FallbackPolicy string
}

// ScoreFit 是纯函数：给定 RequestFeatures + BridgeCapabilities + 策略，
// 返回是否应当让该桥进入候选集（true）或硬剔除（false）。
//
// 算法（§5.3）：
//  1. 对每个 Required 特性 → 查 BridgeCapabilities.LevelOf
//  2. FitRejected → 立即返回 false
//  3. FitDropped 且未在策略允许中 → false
//  4. FitLossy 且 strict / 在 NativeOnlyFeatures → false
//  5. FitNative / FitUnknown（乐观）/ 其他通过策略允许的 → 累加
//  6. 所有 Required 通过 → 返回 true
//
// 当前实现按 plan 验收最小化（不解析复杂策略组合）；后续 PR 引入 Strictness/FallbackPolicy 时再扩展。
func ScoreFit(features *RequestFeatures, caps *BridgeCapabilities, policy *GroupFeaturePolicy) bool {
	if features == nil {
		return true // 无特性约束，桥总是可选
	}
	nativeOnly := map[FeatureID]bool{}
	if policy != nil {
		for _, id := range policy.NativeOnlyFeatures {
			nativeOnly[id] = true
		}
	}

	for id, required := range features.Required {
		if !required {
			continue
		}
		fit := caps.LevelOf(id)
		switch fit {
		case FitRejected:
			return false
		case FitDropped:
			// 当前默认策略不允许 Dropped（除 best_effort）
			if policy == nil || policy.Strictness != "best_effort" {
				return false
			}
		case FitLossy:
			if nativeOnly[id] {
				return false
			}
			if policy != nil && policy.Strictness == "strict" {
				return false
			}
		case FitNative, FitUnknown:
			// 通过
		}
	}
	return true
}

// SniffAnthropic 从 Anthropic Messages 请求体嗅探请求特性（§5.1）。
//
// 关键约束：嗅探只在入口做一次，结果放进 context 由 sticky / 候选 / failover 复用。
// 嗅探失败返回空 RequestFeatures（调用方应按"无约束"处理）。
//
// 当前实现覆盖：
//   - vision（messages[].content[].type == "image"）
//   - document（== "document"）
//   - cache_control（任意位置出现）
//   - extended_thinking（顶层 thinking 字段）
//   - tool_use（tools 非空且非 computer use）
//   - computer_use（tools 含 computer_*）
//   - streaming（顶层 stream: true）
//   - citations（messages 中出现 citations 字段）
func SniffAnthropic(body []byte) *RequestFeatures {
	features := &RequestFeatures{
		Required: map[FeatureID]bool{},
		Optional: map[FeatureID]bool{},
	}
	if len(body) == 0 {
		return features
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return features
	}

	// Streaming
	if v, ok := parsed["stream"].(bool); ok && v {
		features.Required[FeatureStreaming] = true
	}

	// Extended thinking
	if _, ok := parsed["thinking"]; ok {
		features.Required[FeatureExtendedThinking] = true
	}

	// Tools
	if toolsRaw, ok := parsed["tools"].([]any); ok && len(toolsRaw) > 0 {
		hasComputerUse := false
		for _, t := range toolsRaw {
			if tm, ok := t.(map[string]any); ok {
				if name, ok := tm["name"].(string); ok {
					if bytes.HasPrefix([]byte(name), []byte("computer_")) {
						hasComputerUse = true
					}
				}
				if typ, ok := tm["type"].(string); ok {
					if bytes.HasPrefix([]byte(typ), []byte("computer_")) {
						hasComputerUse = true
					}
				}
			}
		}
		if hasComputerUse {
			features.Required[FeatureComputerUse] = true
		} else {
			features.Required[FeatureToolUse] = true
		}
	}

	// cache_control（深度扫描；命中即标记）
	if containsCacheControl(parsed) {
		features.Required[FeatureCacheControl] = true
	}

	// Messages 扫描 vision / document / citations
	if msgs, ok := parsed["messages"].([]any); ok {
		for _, msg := range msgs {
			scanAnthropicMessage(msg, features)
		}
	}

	return features
}

// scanAnthropicMessage 扫描单条消息的 content 数组，更新 features。
func scanAnthropicMessage(msg any, features *RequestFeatures) {
	mm, ok := msg.(map[string]any)
	if !ok {
		return
	}
	content, ok := mm["content"].([]any)
	if !ok {
		return
	}
	for _, c := range content {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := cm["type"].(string)
		switch typ {
		case "image":
			features.Required[FeatureVision] = true
		case "document":
			features.Required[FeatureDocument] = true
		}
		if _, ok := cm["citations"]; ok {
			features.Required[FeatureCitations] = true
		}
	}
}

// containsCacheControl 深度扫描 JSON 是否包含 cache_control 键。
func containsCacheControl(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if k == "cache_control" {
				return true
			}
			if containsCacheControl(val) {
				return true
			}
		}
	case []any:
		for _, item := range x {
			if containsCacheControl(item) {
				return true
			}
		}
	}
	return false
}
