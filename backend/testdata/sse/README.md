# SSE Fixtures (Phase 3 P3-2)

录制的上游 SSE 字节流，用于 `apicompat.ParseSSEStream` / `ReplaySSEInChunks` 重放回归测试。

## 目录结构

```
testdata/sse/
├── anthropic/         # /v1/messages 流式响应
├── openai_chat/       # /v1/chat/completions 流式响应
├── openai_responses/  # /v1/responses 流式响应
└── gemini/            # /v1beta/models/:model:streamGenerateContent 流式响应
```

## 录制方法

使用 `script/record_upstream_sse.sh`：

```bash
GATEWAY_URL=http://localhost:8082 \
API_KEY=sk-xxx \
./script/record_upstream_sse.sh anthropic short_text < prompt.json
```

按 `docs/relay-architecture-design.md §11.1` 每个 protocol 需要 ≥5 fixture：
- short_text：短文本输出
- multiturn：多轮对话上下文
- tool_call：工具调用（含并行调用）
- vision：图片输入
- cache_control：缓存控制（仅 Anthropic）

## 命名约定

- 文件名：`<scenario>.sse`（小写，下划线分隔）
- 内容：原始 SSE 字节流（不做格式化）
- 一行 = 一个 SSE 字段；空行触发事件分发

## 测试用法

```go
fixture, _ := os.ReadFile("testdata/sse/anthropic/short_text.sse")
events, _ := apicompat.ParseSSEStream(bytes.NewReader(fixture))
require.Equal(t, "message_start", events[0].Event)
```

或验证 chunk 抖动鲁棒性：

```go
var dst bytes.Buffer
apicompat.ReplaySSEInChunks(&dst, fixture, 13)  // 13 字节 chunk 模拟网络抖动
parsed, _ := apicompat.ParseSSEStream(&dst)
```

## 维护规约

- fixture 更新 → PR 标签 `requires-bridge-review`（按 §11.3）
- 删除 fixture 需先把对应测试改成可选（避免 CI 立即变红）
- 录制完成后必须检视：移除 PII / API Key 等敏感信息（fixture 会进入 git history）
