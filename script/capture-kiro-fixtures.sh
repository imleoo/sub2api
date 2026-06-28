#!/usr/bin/env bash
# 从 openclaw 的 kiro 分组抓取真实 Kiro 响应，作 P5 kirocompat 归一的偏差分析样本。
#
# 两类采集：
#   1) 基础样本（text/tool_use × 非流式/流式）→ 对比标准 Anthropic 结构，找响应侧偏差
#   2) 能力探测（thinking/vision/document/cache_control/count_tokens）→ 看哪些被 Kiro
#      拒绝/降级/字段缺失，决定请求侧能力门（哪些 → Reject/Reroute）
#
# 每个请求打印 HTTP 状态码（判断支持性）。凭证来自 script/e2e.env（gitignored）。
# 用法：  ./script/capture-kiro-fixtures.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT/script/e2e.env"
OUT_DIR="$ROOT/claudedocs/fixtures/kiro"

[ -f "$ENV_FILE" ] || { echo "ERROR: 缺少 $ENV_FILE" >&2; exit 1; }
# shellcheck disable=SC1090
source "$ENV_FILE"

KEY="${E2E_KIRO_UPSTREAM_KEY:-}"
BASE="${E2E_KIRO_UPSTREAM_BASE_URL:-https://openclaw.zhiguo.fan}"
MODEL="${E2E_KIRO_MODEL:-claude-sonnet-4-6}"
[ -n "$KEY" ] || { echo "ERROR: E2E_KIRO_UPSTREAM_KEY 未设置" >&2; exit 1; }

mkdir -p "$OUT_DIR"
echo "上游: $BASE  模型: $MODEL  → 输出: ${OUT_DIR#"$ROOT/"}"

# 1x1 透明 PNG（vision 探测用）
PNG1X1="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

req() { # req <json-body> <outfile> ; 非流式，打印 http_code
  local body="$1" out="$2" code
  code=$(curl -sS -m 90 -w '%{http_code}' "$BASE/v1/messages" \
    -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" -H "anthropic-version: 2023-06-01" \
    -d "$body" -o "$OUT_DIR/$out" || echo "ERR")
  echo "  [$code] $out ($(wc -c <"$OUT_DIR/$out" 2>/dev/null | tr -d ' ')B)"
}
reqs() { # reqs <json-body> <outfile> ; 流式
  local body="$1" out="$2" code
  code=$(curl -sS -m 90 -N -w '%{http_code}' "$BASE/v1/messages" \
    -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" -H "anthropic-version: 2023-06-01" \
    -d "$body" -o "$OUT_DIR/$out" || echo "ERR")
  echo "  [$code] $out ($(wc -c <"$OUT_DIR/$out" 2>/dev/null | tr -d ' ')B; events: $(grep -ac '^event:' "$OUT_DIR/$out" 2>/dev/null || echo 0))"
}
probe() { # probe <json-body> <outfile> ; 非流式能力探测（同 req，语义区分）
  req "$1" "$2"
}

echo "── 基础样本 ──"
req  "{\"model\":\"$MODEL\",\"max_tokens\":64,\"messages\":[{\"role\":\"user\",\"content\":\"Reply with one short sentence.\"}]}" \
  kiro_nonstream_text.json
reqs "{\"model\":\"$MODEL\",\"max_tokens\":64,\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"Reply with one short sentence.\"}]}" \
  kiro_stream_text.sse
req  "{\"model\":\"$MODEL\",\"max_tokens\":256,\"tool_choice\":{\"type\":\"any\"},\"tools\":[{\"name\":\"get_weather\",\"description\":\"Get the weather for a city\",\"input_schema\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"messages\":[{\"role\":\"user\",\"content\":\"What is the weather in Paris?\"}]}" \
  kiro_nonstream_tooluse.json
reqs "{\"model\":\"$MODEL\",\"max_tokens\":256,\"stream\":true,\"tool_choice\":{\"type\":\"any\"},\"tools\":[{\"name\":\"get_weather\",\"description\":\"Get the weather for a city\",\"input_schema\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"messages\":[{\"role\":\"user\",\"content\":\"What is the weather in Paris?\"}]}" \
  kiro_stream_tooluse.sse

echo "── 能力探测（看支持性 / 偏差）──"
# extended thinking
probe "{\"model\":\"$MODEL\",\"max_tokens\":2048,\"thinking\":{\"type\":\"enabled\",\"budget_tokens\":1024},\"messages\":[{\"role\":\"user\",\"content\":\"What is 17*23? Think step by step first.\"}]}" \
  probe_thinking.json
# vision（image 块）
probe "{\"model\":\"$MODEL\",\"max_tokens\":64,\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"image\",\"source\":{\"type\":\"base64\",\"media_type\":\"image/png\",\"data\":\"$PNG1X1\"}},{\"type\":\"text\",\"text\":\"What color is this?\"}]}]}" \
  probe_vision.json
# cache_control
probe "{\"model\":\"$MODEL\",\"max_tokens\":64,\"system\":[{\"type\":\"text\",\"text\":\"You are a helpful assistant. This is a long cached system prompt.\",\"cache_control\":{\"type\":\"ephemeral\"}}],\"messages\":[{\"role\":\"user\",\"content\":\"Hi\"}]}" \
  probe_cache_control.json
# document（PDF 块；Kiro 大概率不支持 → 看错误形态）
probe "{\"model\":\"$MODEL\",\"max_tokens\":64,\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"document\",\"source\":{\"type\":\"base64\",\"media_type\":\"application/pdf\",\"data\":\"JVBERi0xLjQK\"}},{\"type\":\"text\",\"text\":\"Summarize\"}]}]}" \
  probe_document.json
# count_tokens 端点
echo "  ── count_tokens ──"
CT=$(curl -sS -m 60 -w '%{http_code}' "$BASE/v1/messages/count_tokens" \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" -H "anthropic-version: 2023-06-01" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"count these tokens\"}]}" \
  -o "$OUT_DIR/probe_count_tokens.json" || echo "ERR")
echo "  [$CT] probe_count_tokens.json ($(wc -c <"$OUT_DIR/probe_count_tokens.json" 2>/dev/null | tr -d ' ')B)"

echo "完成。样本落 ${OUT_DIR#"$ROOT/"}（含真实响应，分析后决定是否脱敏提交）。"
