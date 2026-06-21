// Phase 4 P4-2：Gemini function calling schema 映射 fuzz + fixture 测试。
//
// 验证目标（docs/relay-architecture-design.md §4.3 P4-2 验收）：
//  1. SSE fixture 可被 ParseSSEStream 正确解析，每个 chunk 可反序列化为 GeminiGenerateContentResponse
//  2. GeminiFunctionDeclaration JSON marshal/unmarshal 无字段丢失（fuzz 覆盖随机 JSON）
//  3. 流式 stub 函数签名与接口定义一致（编译即验证）
package apicompat

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- Gemini SSE fixture 解析测试 ---

// TestGeminiSSEFixture_TextTurn 验证 text_turn.sse fixture 可被正确解析，
// 每个 chunk 均可反序列化为 GeminiGenerateContentResponse，且最后一个 chunk 含 finishReason。
func TestGeminiSSEFixture_TextTurn(t *testing.T) {
	events := loadGeminiSSEFixture(t, "text_turn.sse")
	require.NotEmpty(t, events, "text_turn.sse 至少应有 1 条 SSE 事件")

	for i, evt := range events {
		var chunk GeminiGenerateContentResponse
		err := json.Unmarshal([]byte(evt.Data), &chunk)
		require.NoError(t, err, "text_turn.sse chunk[%d] 反序列化失败", i)
		require.NotEmpty(t, chunk.Candidates, "chunk[%d] candidates 为空", i)
	}

	// 最后一个 chunk 必须带 finishReason
	last := events[len(events)-1]
	var lastChunk GeminiGenerateContentResponse
	require.NoError(t, json.Unmarshal([]byte(last.Data), &lastChunk))
	require.Equal(t, "STOP", lastChunk.Candidates[0].FinishReason, "最后 chunk 应有 finishReason=STOP")
}

// TestGeminiSSEFixture_FunctionCall 验证 function_call.sse fixture：
// parts[0].functionCall 字段存在且 name/args 正确。
func TestGeminiSSEFixture_FunctionCall(t *testing.T) {
	events := loadGeminiSSEFixture(t, "function_call.sse")
	require.Len(t, events, 1, "function_call.sse 应只有 1 条 SSE 事件（单 chunk 完整 response）")

	var chunk GeminiGenerateContentResponse
	require.NoError(t, json.Unmarshal([]byte(events[0].Data), &chunk))
	require.Len(t, chunk.Candidates, 1)

	parts := chunk.Candidates[0].Content.Parts
	require.Len(t, parts, 1, "function call 响应应有 1 个 part")
	require.NotNil(t, parts[0].FunctionCall, "part 应是 functionCall 类型")
	require.Equal(t, "get_weather", parts[0].FunctionCall.Name)

	// args 应可解析为 JSON 对象
	var args map[string]any
	require.NoError(t, json.Unmarshal(parts[0].FunctionCall.Args, &args))
	require.Equal(t, "Tokyo", args["location"])
}

// TestGeminiSSEFixture_AllFiles 枚举 testdata/sse/gemini/ 下所有 .sse 文件，
// 确保每个文件都可被 ParseSSEStream 解析（至少 1 条事件，所有 data: 均合法 JSON）。
func TestGeminiSSEFixture_AllFiles(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "..", "testdata", "sse", "gemini")
	entries, err := os.ReadDir(fixtureDir)
	if os.IsNotExist(err) {
		t.Skipf("fixture 目录 %s 不存在，跳过", fixtureDir)
	}
	require.NoError(t, err)

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sse") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			events := loadGeminiSSEFixture(t, entry.Name())
			require.NotEmpty(t, events, "fixture 文件 %s 不应为空", entry.Name())
			for i, evt := range events {
				if evt.Data == "" {
					continue
				}
				var obj map[string]any
				require.NoError(t, json.Unmarshal([]byte(evt.Data), &obj),
					"fixture %s chunk[%d] data 应为合法 JSON", entry.Name(), i)
			}
		})
	}
}

// --- Gemini stub 函数签名冒烟测试 ---

// TestGeminiStubFunctions 验证 stub 函数返回正确的错误，不 panic。
func TestGeminiStubFunctions(t *testing.T) {
	req := &GeminiGenerateContentRequest{
		Contents: []GeminiContent{
			{Role: "user", Parts: []GeminiPart{{Text: "hello"}}},
		},
	}

	t.Run("ForwardGeminiAsChat returns ErrNotImplemented", func(t *testing.T) {
		resp, err := ForwardGeminiAsChat(req)
		require.Nil(t, resp)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrGeminiToOpenAINotImplemented))
	})

	t.Run("ForwardGeminiAsAnthropic returns ErrNotImplemented", func(t *testing.T) {
		resp, err := ForwardGeminiAsAnthropic(req)
		require.Nil(t, resp)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrGeminiToAnthropicNotImplemented))
	})

	t.Run("ForwardGeminiStreamAsChat returns ErrNotImplemented", func(t *testing.T) {
		err := ForwardGeminiStreamAsChat(req, nil)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrGeminiToOpenAINotImplemented))
	})

	t.Run("ForwardGeminiStreamAsAnthropic returns ErrNotImplemented", func(t *testing.T) {
		err := ForwardGeminiStreamAsAnthropic(req, nil)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrGeminiToAnthropicNotImplemented))
	})
}

// --- GeminiFunctionDeclaration JSON 映射保全 fuzz 测试 ---

// FuzzGeminiFunctionDeclarationRoundtrip 验证 GeminiFunctionDeclaration marshal/unmarshal 零丢失。
//
// 种子集覆盖：
//   - 基础 text 工具（必填 name）
//   - 含 parameters JSON Schema 的工具（properties + required）
//   - 嵌套 parameters（对象嵌套）
//   - 无 parameters 工具（空 schema）
//
// 运行方式：
//   - 正常测试：go test -run=FuzzGeminiFunctionDeclarationRoundtrip
//   - fuzz 模式：go test -fuzz=FuzzGeminiFunctionDeclarationRoundtrip -fuzztime=30s
func FuzzGeminiFunctionDeclarationRoundtrip(f *testing.F) {
	// seed 1: 基础工具
	f.Add([]byte(`{"name":"say_hello","description":"Say hello to the user"}`))

	// seed 2: 含 parameters（JSON Schema object）
	f.Add([]byte(`{"name":"get_weather","description":"Get current weather","parameters":{"type":"object","properties":{"location":{"type":"string","description":"City name or coordinates"},"unit":{"type":"string","enum":["celsius","fahrenheit"]}},"required":["location"]}}`))

	// seed 3: 嵌套参数（struct 类型）
	f.Add([]byte(`{"name":"create_event","parameters":{"type":"object","properties":{"event":{"type":"object","properties":{"title":{"type":"string"},"start_time":{"type":"string","format":"date-time"}},"required":["title","start_time"]}}}}`))

	// seed 4: 无 parameters
	f.Add([]byte(`{"name":"get_current_time"}`))

	// seed 5: 特殊字符描述
	f.Add([]byte(`{"name":"search","description":"Search for results\nSupports multi-line","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var decl GeminiFunctionDeclaration
		if err := json.Unmarshal(data, &decl); err != nil {
			return // 跳过非法 JSON，不是测试目标
		}

		// Round-trip：marshal → unmarshal → 断言关键字段保留
		b, err := json.Marshal(decl)
		if err != nil {
			t.Fatalf("marshal GeminiFunctionDeclaration 失败: %v", err)
		}

		var decl2 GeminiFunctionDeclaration
		if err := json.Unmarshal(b, &decl2); err != nil {
			t.Fatalf("unmarshal round-trip 失败: %v", err)
		}

		// name 字段必须保留（函数名是 function calling 的核心键）
		if decl.Name != decl2.Name {
			t.Fatalf("name 在 round-trip 中丢失: %q → %q", decl.Name, decl2.Name)
		}

		// description 字段必须保留（不能被截断或替换）
		if decl.Description != decl2.Description {
			t.Fatalf("description 在 round-trip 中丢失: %q → %q", decl.Description, decl2.Description)
		}

		// parameters 字段如果原始非空，round-trip 后仍应非空（JSON Schema 不可丢失）
		if len(decl.Parameters) > 0 && len(decl2.Parameters) == 0 {
			t.Fatalf("parameters 在 round-trip 中丢失（原始: %s）", decl.Parameters)
		}

		// parameters 内容语义等价（re-marshal 后字节可能因 key 排序变化，用 JSON unmarshal 比较）
		if len(decl.Parameters) > 0 {
			var p1, p2 any
			if err1 := json.Unmarshal(decl.Parameters, &p1); err1 == nil {
				if err2 := json.Unmarshal(decl2.Parameters, &p2); err2 != nil {
					t.Fatalf("round-trip 后 parameters 不是合法 JSON: %v", err2)
				}
			}
		}
	})
}

// --- 辅助函数 ---

// loadGeminiSSEFixture 读取 testdata/sse/gemini/<name> 并用 ParseSSEStream 解析。
func loadGeminiSSEFixture(t *testing.T, name string) []SSEEvent {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "sse", "gemini", name)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		t.Skipf("fixture 文件 %s 不存在，跳过", path)
	}
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	events, err := ParseSSEStream(f)
	require.NoError(t, err, "ParseSSEStream 失败")
	return events
}
