package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

var codexModelMap = map[string]string{
	"gpt-5.6-sol":          "gpt-5.6-sol",
	"gpt-5.6-terra":        "gpt-5.6-terra",
	"gpt-5.6-luna":         "gpt-5.6-luna",
	"gpt-5.5":              "gpt-5.5",
	"gpt-5.5-pro":          "gpt-5.5-pro",
	"codex-auto-review":    "codex-auto-review",
	"gpt-5.4":              "gpt-5.4",
	"gpt-5.4-mini":         "gpt-5.4-mini",
	"gpt-5.4-none":         "gpt-5.4",
	"gpt-5.4-low":          "gpt-5.4",
	"gpt-5.4-medium":       "gpt-5.4",
	"gpt-5.4-high":         "gpt-5.4",
	"gpt-5.4-xhigh":        "gpt-5.4",
	"gpt-5.4-chat-latest":  "gpt-5.4",
	"gpt-5.3":              "gpt-5.3-codex",
	"gpt-5.3-none":         "gpt-5.3-codex",
	"gpt-5.3-low":          "gpt-5.3-codex",
	"gpt-5.3-medium":       "gpt-5.3-codex",
	"gpt-5.3-high":         "gpt-5.3-codex",
	"gpt-5.3-xhigh":        "gpt-5.3-codex",
	"gpt-5.3-codex":        "gpt-5.3-codex",
	"gpt-5.3-codex-spark":  "gpt-5.3-codex-spark",
	"gpt-5.3-codex-low":    "gpt-5.3-codex",
	"gpt-5.3-codex-medium": "gpt-5.3-codex",
	"gpt-5.3-codex-high":   "gpt-5.3-codex",
	"gpt-5.3-codex-xhigh":  "gpt-5.3-codex",
	"gpt-5.2":              "gpt-5.2",
	"gpt-5.2-none":         "gpt-5.2",
	"gpt-5.2-low":          "gpt-5.2",
	"gpt-5.2-medium":       "gpt-5.2",
	"gpt-5.2-high":         "gpt-5.2",
	"gpt-5.2-xhigh":        "gpt-5.2",
	"gpt-5":                "gpt-5.4",
	"gpt-5-mini":           "gpt-5.4",
	"gpt-5-nano":           "gpt-5.4",
	"gpt-5.1":              "gpt-5.4",
	"gpt-5.1-codex":        "gpt-5.3-codex",
	"gpt-5.1-codex-max":    "gpt-5.3-codex",
	"gpt-5.1-codex-mini":   "gpt-5.3-codex",
	"gpt-5.2-codex":        "gpt-5.2",
	"codex-mini-latest":    "gpt-5.3-codex",
	"gpt-5-codex":          "gpt-5.3-codex",
}

// CodexAliasPairs 返回 (variant, canonical) 对的列表，供 SSOT PR-4 aliasIdx 构建使用。
// 把 OpenAI codex/gpt-5.x 的各种变体映射到 catalog 中的标准 model_id。
// 列表顺序：原 codexModelMap 迭代顺序 + codexVersionModelPrefixes（前缀匹配规则）。
// 调用方应去重并按需归一化 variant key。
func CodexAliasPairs() [][2]string {
	pairs := make([][2]string, 0, len(codexModelMap)+len(codexVersionModelPrefixes))
	for variant, target := range codexModelMap {
		pairs = append(pairs, [2]string{variant, target})
	}
	for _, p := range codexVersionModelPrefixes {
		pairs = append(pairs, [2]string{p.prefix, p.target})
	}
	return pairs
}

var codexVersionModelPrefixes = []struct {
	prefix string
	target string
}{
	{prefix: "gpt-5.6-sol", target: "gpt-5.6-sol"},
	{prefix: "gpt-5.6-terra", target: "gpt-5.6-terra"},
	{prefix: "gpt-5.6-luna", target: "gpt-5.6-luna"},
	{prefix: "gpt-5.3-codex-spark", target: "gpt-5.3-codex-spark"},
	{prefix: "gpt-5.3-codex", target: "gpt-5.3-codex"},
	{prefix: "gpt-5.4-mini", target: "gpt-5.4-mini"},
	{prefix: "gpt-5.4-nano", target: "gpt-5.4-nano"},
	{prefix: "gpt-5.5-pro", target: "gpt-5.5-pro"},
	{prefix: "gpt-5.5", target: "gpt-5.5"},
	{prefix: "gpt-5.4", target: "gpt-5.4"},
	{prefix: "gpt-5.2", target: "gpt-5.2"},
}

const (
	codexImageGenerationBridgeMarker = "<tokenpanel-codex-image-generation>"
	codexImageGenerationBridgeText   = codexImageGenerationBridgeMarker + "\nWhen the user asks for raster image generation or editing, use the OpenAI Responses native `image_generation` tool attached to this request. The local Codex client may not expose an `image_gen` namespace, but that does not mean image generation is unavailable. Do not ask the user to switch to CLI fallback solely because `image_gen` is absent.\n</tokenpanel-codex-image-generation>"
	codexSparkImageUnsupportedMarker = "<tokenpanel-codex-spark-image-unsupported>"
	codexSparkImageUnsupportedText   = codexSparkImageUnsupportedMarker + "\nThe current model is gpt-5.3-codex-spark, which does not support image generation, image editing, image input, the `image_generation` tool, or Codex `image_gen`/`$imagegen` workflows. If the user asks for image generation or image editing, clearly explain this model limitation and ask them to switch to a non-Spark Codex model such as gpt-5.3-codex or gpt-5.4. Do not claim that the local environment merely lacks image_gen tooling, and do not suggest CLI fallback as the primary fix while the model remains Spark.\n</tokenpanel-codex-spark-image-unsupported>"
)

func normalizeCodexModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "gpt-5.4"
	}
	if mapped, ok := normalizeKnownCodexModel(model); ok {
		return mapped
	}
	return model
}

func normalizeKnownCodexModel(model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", false
	}
	if isOpenAIImageGenerationModel(model) {
		return model, true
	}

	modelID := lastOpenAIModelSegment(model)

	if normalized := canonicalizeOpenAIModelAliasSpelling(modelID); normalized != "" {
		modelID = normalized
	}
	if mapped := normalizeKnownOpenAICodexModel(modelID); mapped != "" {
		return mapped, true
	}
	key := codexModelLookupKey(modelID)
	if key == "" {
		return "", false
	}
	if mapped := getNormalizedCodexModel(key); mapped != "" {
		return mapped, true
	}
	for _, item := range codexVersionModelPrefixes {
		if key == item.prefix {
			return item.target, true
		}
		suffix, ok := strings.CutPrefix(key, item.prefix+"-")
		if ok && isKnownCodexModelSuffix(suffix) {
			return item.target, true
		}
	}
	return "", false
}

func codexModelLookupKey(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ""
	}
	if strings.Contains(modelID, "/") {
		parts := strings.Split(modelID, "/")
		modelID = parts[len(parts)-1]
	}
	return strings.ToLower(strings.Join(strings.Fields(modelID), "-"))
}

func isKnownCodexModelSuffix(suffix string) bool {
	switch suffix {
	case "none", "minimal", "low", "medium", "high", "xhigh":
		return true
	}
	return isCodexDateSuffix(suffix)
}

func isCodexDateSuffix(suffix string) bool {
	parts := strings.Split(suffix, "-")
	if len(parts) != 3 || len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 2 {
		return false
	}
	for _, part := range parts {
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func isCodexSparkModel(model string) bool {
	return normalizeCodexModel(model) == "gpt-5.3-codex-spark"
}

func hasOpenAIImageGenerationTool(reqBody map[string]any) bool {
	if toolsContainImageGeneration(reqBody["tools"]) {
		return true
	}
	return inputContainsImageGenerationTool(reqBody["input"])
}

func toolsContainImageGeneration(rawTools any) bool {
	if rawTools == nil {
		return false
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false
	}
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if isOpenAIImageGenerationToolMap(toolMap) {
			return true
		}
	}
	return false
}

func isOpenAIImageGenerationToolMap(tool map[string]any) bool {
	return isOpenAIImageGenerationType(firstNonEmptyString(tool["type"])) ||
		isImageGenNamespaceToolMap(tool)
}

func isImageGenNamespaceToolMap(tool map[string]any) bool {
	return strings.TrimSpace(firstNonEmptyString(tool["type"])) == "namespace" &&
		isOpenAIImageGenNamespaceName(firstNonEmptyString(tool["name"]))
}

func inputContainsImageGenerationTool(rawInput any) bool {
	input, ok := rawInput.([]any)
	if !ok {
		return false
	}
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(item["type"])) != "additional_tools" {
			continue
		}
		if toolsContainImageGeneration(item["tools"]) {
			return true
		}
	}
	return false
}

// stripOpenAIImageGenerationTools keeps account-level strip policy symmetric
// across standard Responses tools, Responses Lite additional_tools, and tool_choice.
func stripOpenAIImageGenerationTools(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	modified := stripOpenAIImageGenerationToolList(reqBody, "tools")
	if stripOpenAIImageGenerationToolsFromInput(reqBody) {
		modified = true
	}
	if openAIAnyToolChoiceSelectsImageGeneration(reqBody["tool_choice"]) {
		delete(reqBody, "tool_choice")
		modified = true
	}
	return modified
}

func stripOpenAIImageGenerationToolList(container map[string]any, key string) bool {
	rawTools, ok := container[key]
	if !ok || rawTools == nil {
		return false
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false
	}
	filtered := make([]any, 0, len(tools))
	removed := false
	for _, rawTool := range tools {
		if toolMap, ok := rawTool.(map[string]any); ok && isOpenAIImageGenerationToolMap(toolMap) {
			removed = true
			continue
		}
		filtered = append(filtered, rawTool)
	}
	if !removed {
		return false
	}
	if len(filtered) == 0 {
		delete(container, key)
	} else {
		container[key] = filtered
	}
	return true
}

func stripOpenAIImageGenerationToolsFromInput(reqBody map[string]any) bool {
	input, ok := reqBody["input"].([]any)
	if !ok {
		return false
	}

	filteredInput := make([]any, 0, len(input))
	modified := false
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok || strings.TrimSpace(firstNonEmptyString(item["type"])) != "additional_tools" {
			filteredInput = append(filteredInput, rawItem)
			continue
		}
		if !stripOpenAIImageGenerationToolList(item, "tools") {
			filteredInput = append(filteredInput, rawItem)
			continue
		}
		modified = true
		if _, hasTools := item["tools"]; hasTools {
			filteredInput = append(filteredInput, rawItem)
		}
		// An empty additional_tools carrier is not useful upstream; drop the item
		// after its only declared capability has been removed.
	}
	if modified {
		reqBody["input"] = filteredInput
	}
	return modified
}

// stripOpenAIImageGenerationToolsFromRawPayload is the shared adapter for paths
// that forward raw HTTP or WebSocket payloads without the normal request map.
func stripOpenAIImageGenerationToolsFromRawPayload(payload []byte) ([]byte, bool, error) {
	if !openAIRequestBodyHasImageGenerationDeclaration(payload) {
		if json.Valid(payload) {
			return payload, false, nil
		}
		var invalidPayload map[string]any
		return payload, false, json.Unmarshal(payload, &invalidPayload)
	}
	payloadMap := make(map[string]any)
	if err := json.Unmarshal(payload, &payloadMap); err != nil {
		return payload, false, err
	}
	if !stripOpenAIImageGenerationTools(payloadMap) {
		return payload, false, nil
	}
	rebuilt, err := json.Marshal(payloadMap)
	if err != nil {
		return payload, false, err
	}
	return rebuilt, true, nil
}

// stripCodexSparkImageGenerationTools removes image tool declarations and choices.
// gpt-5.3-codex-spark rejects those capabilities upstream, while Codex clients may
// advertise them by default.
func stripCodexSparkImageGenerationTools(reqBody map[string]any) bool {
	return stripOpenAIImageGenerationTools(reqBody)
}

func hasOpenAIInputImage(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	return hasOpenAIInputImageValue(reqBody["input"]) || hasOpenAIInputImageValue(reqBody["messages"])
}

func hasOpenAIInputImageValue(value any) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if hasOpenAIInputImageValue(item) {
				return true
			}
		}
	case map[string]any:
		if strings.TrimSpace(firstNonEmptyString(v["type"])) == "input_image" {
			return true
		}
		if _, ok := v["image_url"]; ok {
			return true
		}
		return hasOpenAIInputImageValue(v["content"])
	}
	return false
}

func validateCodexSparkInput(reqBody map[string]any, model string) error {
	if !isCodexSparkModel(model) || !hasOpenAIInputImage(reqBody) {
		return nil
	}
	return fmt.Errorf("model %q does not support image input", strings.TrimSpace(model))
}

func normalizeOpenAIResponsesImageGenerationTools(reqBody map[string]any) bool {
	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		return false
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false
	}

	modified := false
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok || strings.TrimSpace(firstNonEmptyString(toolMap["type"])) != "image_generation" {
			continue
		}
		if _, ok := toolMap["output_format"]; !ok {
			if value := strings.TrimSpace(firstNonEmptyString(toolMap["format"])); value != "" {
				toolMap["output_format"] = value
				modified = true
			}
		}
		if _, ok := toolMap["output_compression"]; !ok {
			if value, exists := toolMap["compression"]; exists && value != nil {
				toolMap["output_compression"] = value
				modified = true
			}
		}
		if _, ok := toolMap["format"]; ok {
			delete(toolMap, "format")
			modified = true
		}
		if _, ok := toolMap["compression"]; ok {
			delete(toolMap, "compression")
			modified = true
		}
	}
	return modified
}

func ensureOpenAIResponsesImageGenerationTool(reqBody map[string]any) bool {
	if len(reqBody) == 0 {
		return false
	}
	if isCodexSparkModel(firstNonEmptyString(reqBody["model"])) {
		return false
	}

	tool := map[string]any{
		"type":          "image_generation",
		"output_format": "png",
	}

	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		reqBody["tools"] = []any{tool}
		return true
	}

	tools, ok := rawTools.([]any)
	if !ok {
		reqBody["tools"] = []any{tool}
		return true
	}
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(toolMap["type"])) == "image_generation" {
			return false
		}
	}

	reqBody["tools"] = append(tools, tool)
	return true
}

func applyCodexImageGenerationBridgeInstructions(reqBody map[string]any) bool {
	if len(reqBody) == 0 || !hasOpenAIImageGenerationTool(reqBody) {
		return false
	}
	if isCodexSparkModel(firstNonEmptyString(reqBody["model"])) {
		return false
	}

	existing, _ := reqBody["instructions"].(string)
	if strings.Contains(existing, codexImageGenerationBridgeMarker) {
		return false
	}

	existing = strings.TrimRight(existing, " \t\r\n")
	if strings.TrimSpace(existing) == "" {
		reqBody["instructions"] = codexImageGenerationBridgeText
		return true
	}

	reqBody["instructions"] = existing + "\n\n" + codexImageGenerationBridgeText
	return true
}

func validateOpenAIResponsesImageModel(reqBody map[string]any, model string) error {
	if !hasOpenAIImageGenerationTool(reqBody) {
		return nil
	}
	model = strings.TrimSpace(model)
	if !isOpenAIImageGenerationModel(model) {
		return nil
	}
	return fmt.Errorf("/v1/responses image_generation requests require a Responses-capable text model; image-only model %q is not allowed", model)
}

func normalizeOpenAIResponsesImageOnlyModel(reqBody map[string]any) bool {
	if len(reqBody) == 0 {
		return false
	}
	imageModel := strings.TrimSpace(firstNonEmptyString(reqBody["model"]))
	if !isOpenAIImageGenerationModel(imageModel) {
		return false
	}

	modified := false
	tools, _ := reqBody["tools"].([]any)
	imageToolIndex := -1
	for i, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(toolMap["type"])) == "image_generation" {
			imageToolIndex = i
			break
		}
	}
	if imageToolIndex < 0 {
		tools = append(tools, map[string]any{
			"type":  "image_generation",
			"model": imageModel,
		})
		imageToolIndex = len(tools) - 1
		reqBody["tools"] = tools
		modified = true
	}

	if toolMap, ok := tools[imageToolIndex].(map[string]any); ok {
		if strings.TrimSpace(firstNonEmptyString(toolMap["model"])) == "" {
			toolMap["model"] = imageModel
			modified = true
		}
		for _, key := range []string{
			"size",
			"quality",
			"background",
			"output_format",
			"output_compression",
			"moderation",
			"style",
			"partial_images",
		} {
			if value, exists := reqBody[key]; exists && value != nil {
				if _, toolHas := toolMap[key]; !toolHas {
					toolMap[key] = value
				}
				delete(reqBody, key)
				modified = true
			}
		}
	}

	if prompt := strings.TrimSpace(firstNonEmptyString(reqBody["prompt"])); prompt != "" {
		if _, hasInput := reqBody["input"]; !hasInput {
			reqBody["input"] = prompt
		}
		delete(reqBody, "prompt")
		modified = true
	}

	if _, ok := reqBody["tool_choice"]; !ok {
		reqBody["tool_choice"] = map[string]any{"type": "image_generation"}
		modified = true
	}
	if imageModel != openAIImagesResponsesMainModel {
		modified = true
	}
	reqBody["model"] = openAIImagesResponsesMainModel
	return modified
}

func normalizeOpenAIModelForUpstream(account *Account, model string) string {
	if account == nil {
		return normalizeCodexModel(model)
	}
	return strings.TrimSpace(model)
}

func SupportsVerbosity(model string) bool {
	if !strings.HasPrefix(model, "gpt-") {
		return true
	}

	var major, minor int
	n, _ := fmt.Sscanf(model, "gpt-%d.%d", &major, &minor)

	if major > 5 {
		return true
	}
	if major < 5 {
		return false
	}

	// gpt-5
	if n == 1 {
		return true
	}

	return minor >= 3
}

func getNormalizedCodexModel(modelID string) string {
	key := codexModelLookupKey(modelID)
	if key == "" {
		return ""
	}
	if mapped, ok := codexModelMap[key]; ok {
		return mapped
	}
	return ""
}

func defaultCodexSynthInstructions(model string) string {
	if instructions := strings.TrimSpace(openai.CodexBaseInstructionsForModel(model)); instructions != "" {
		return instructions
	}
	return "You are a helpful coding assistant."
}

func isCodexToolCallItemType(typ string) bool {
	switch typ {
	case "function_call",
		"tool_call",
		"local_shell_call",
		"tool_search_call",
		"custom_tool_call",
		"mcp_tool_call",
		"function_call_output",
		"mcp_tool_call_output",
		"custom_tool_call_output",
		"tool_search_output":
		return true
	default:
		return false
	}
}

// openAIChatGPTInternalUnsupportedFields 上游 Responses 端点不接受的请求字段（透传前剔除）。
var openAIChatGPTInternalUnsupportedFields = []string{
	"user",
	"metadata",
	"prompt_cache_retention",
	"safety_identifier",
	"stream_options",
}

// ensureOpenAIResponsesImageGenerationToolChoiceAuto 携带 image_generation 工具且未显式指定
// tool_choice 时补 "auto"，与 WS 图像桥接路径保持一致。
func ensureOpenAIResponsesImageGenerationToolChoiceAuto(reqBody map[string]any) bool {
	if len(reqBody) == 0 || !hasOpenAIImageGenerationTool(reqBody) {
		return false
	}
	if isCodexSparkModel(firstNonEmptyString(reqBody["model"])) {
		return false
	}
	if _, ok := reqBody["tool_choice"]; ok {
		return false
	}
	reqBody["tool_choice"] = "auto"
	return true
}
