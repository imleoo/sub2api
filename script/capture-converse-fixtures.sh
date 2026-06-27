#!/usr/bin/env bash
# 从 openclaw 标准上游抓取真实 Anthropic native 响应，落成 conversecompat 的测试 fixture。
#
# 用途：为 BedrockFixAdapter（Anthropic native → Bedrock Converse）的转换单测提供「输入侧」真实金样本。
#   - 非流式 JSON：验证 AnthropicToConverseResponse 的字段映射（表 A）
#   - 流式 SSE：验证逐事件转换 + usage 位置（message_start.input / message_delta.output / message_stop.usage）
#
# 凭证来自 script/e2e.env（gitignored）：E2E_BEDROCK_CONVERSE_UPSTREAM_{KEY,BASE_URL} + E2E_BEDROCK_CONVERSE_MODEL
# openclaw 不暴露 Converse 出站（不吐二进制帧），故本脚本只取 native（标准协议侧）。AWS 二进制帧需另测（见设计 §5.4 第 2 层）。
#
# 用法：  ./script/capture-converse-fixtures.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT/script/e2e.env"
# 设计阶段：fixture 落在 claudedocs 下作参考证据，不进生产代码树。
# 待实现 conversecompat 包后，把 OUT_DIR 改到 backend/internal/pkg/claude/conversecompat/testdata 即可。
OUT_DIR="$ROOT/claudedocs/fixtures/converse"

[ -f "$ENV_FILE" ] || { echo "ERROR: 缺少 $ENV_FILE（从 e2e.env.example 复制并填入 openclaw key）" >&2; exit 1; }
# shellcheck disable=SC1090
source "$ENV_FILE"

KEY="${E2E_BEDROCK_CONVERSE_UPSTREAM_KEY:-}"
BASE="${E2E_BEDROCK_CONVERSE_UPSTREAM_BASE_URL:-https://openclaw.zhiguo.fan}"
MODEL="${E2E_BEDROCK_CONVERSE_MODEL:-claude-sonnet-4-6}"
[ -n "$KEY" ] || { echo "ERROR: E2E_BEDROCK_CONVERSE_UPSTREAM_KEY 未设置" >&2; exit 1; }

mkdir -p "$OUT_DIR"
echo "上游: $BASE  模型: $MODEL  → 输出: ${OUT_DIR#"$ROOT/"}"

req() { # req <json-body> <outfile> ; 非流式
  local body="$1" out="$2"
  curl -sS -m 60 "$BASE/v1/messages" \
    -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" -H "anthropic-version: 2023-06-01" \
    -d "$body" -o "$OUT_DIR/$out"
  echo "  ✓ $out ($(wc -c <"$OUT_DIR/$out" | tr -d ' ')B)"
}
reqs() { # reqs <json-body> <outfile> ; 流式（原始 SSE）
  local body="$1" out="$2"
  curl -sS -m 60 -N "$BASE/v1/messages" \
    -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" -H "anthropic-version: 2023-06-01" \
    -d "$body" -o "$OUT_DIR/$out"
  echo "  ✓ $out ($(wc -c <"$OUT_DIR/$out" | tr -d ' ')B; events: $(grep -ac '^event:' "$OUT_DIR/$out" 2>/dev/null || echo 0))"
}

# 1) 非流式 · 纯文本（text 块 + stop_reason + usage 结构）
req "{\"model\":\"$MODEL\",\"max_tokens\":64,\"messages\":[{\"role\":\"user\",\"content\":\"Reply with one short sentence.\"}]}" \
  native_nonstream_text.json

# 2) 流式 · 纯文本（完整事件序列 + usage 位置）
reqs "{\"model\":\"$MODEL\",\"max_tokens\":64,\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"Reply with one short sentence.\"}]}" \
  native_stream_text.sse

# 3) 非流式 · 工具调用（tool_use 块 → 验证 id→toolUseId 映射）
req "{\"model\":\"$MODEL\",\"max_tokens\":256,\"tool_choice\":{\"type\":\"any\"},\"tools\":[{\"name\":\"get_weather\",\"description\":\"Get the weather for a city\",\"input_schema\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"messages\":[{\"role\":\"user\",\"content\":\"What is the weather in Paris?\"}]}" \
  native_nonstream_tooluse.json

# 4) 流式 · 工具调用（input_json_delta → 验证 toolUse.input 片段拼接）
reqs "{\"model\":\"$MODEL\",\"max_tokens\":256,\"stream\":true,\"tool_choice\":{\"type\":\"any\"},\"tools\":[{\"name\":\"get_weather\",\"description\":\"Get the weather for a city\",\"input_schema\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"messages\":[{\"role\":\"user\",\"content\":\"What is the weather in Paris?\"}]}" \
  native_stream_tooluse.sse

echo "完成。fixture 均为模型输出，无凭证，可提交。"
