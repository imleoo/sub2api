#!/usr/bin/env bash
# Phase 3 P3-2 上游 SSE 录制工具
#
# 用途：通过本项目网关代理调用真实上游账号，把 SSE 字节流录制到
# backend/testdata/sse/<protocol>/<name>.sse 供 P3-2 replay 框架使用。
#
# 关键设计：录制走本项目网关（而不是直连上游），这样保证：
#   1. 鉴权/路由/usage 记录链路与生产一致
#   2. fixture 反映"客户端实际能收到的字节"
#
# 调用方式（按 docs/relay-architecture-design.md §11.1 ≥5 fixture per protocol）：
#   GATEWAY_URL=http://localhost:8082 \
#   API_KEY=sk-xxx \
#   ./script/record_upstream_sse.sh anthropic short_text < fixtures/prompts/short_text.json
#
# 参数：
#   $1 - protocol（anthropic / openai_chat / openai_responses / gemini）
#   $2 - fixture name（如 short_text / tool_call / vision / cache_control / multiturn）
#   stdin - 请求体（JSON）

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURE_DIR="$ROOT_DIR/backend/testdata/sse"

PROTOCOL="${1:-}"
FIXTURE_NAME="${2:-}"
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8082}"
API_KEY="${API_KEY:-}"

if [[ -z "$PROTOCOL" || -z "$FIXTURE_NAME" ]]; then
  echo "usage: $0 <protocol> <fixture_name>" >&2
  echo "  protocol  ∈ anthropic | openai_chat | openai_responses | gemini" >&2
  echo "  fixture   ∈ short_text | tool_call | vision | cache_control | multiturn" >&2
  echo "" >&2
  echo "env:" >&2
  echo "  GATEWAY_URL  本项目网关地址（默认 http://localhost:8082）" >&2
  echo "  API_KEY      本项目签发的 API Key（绑定对应 protocol 的 group）" >&2
  exit 2
fi

if [[ -z "$API_KEY" ]]; then
  echo "ERROR: API_KEY must be set" >&2
  exit 2
fi

# 路径映射 → 请求方法 + endpoint
declare -A ENDPOINT_MAP=(
  [anthropic]="/v1/messages"
  [openai_chat]="/v1/chat/completions"
  [openai_responses]="/v1/responses"
  [gemini]="/v1beta/models/gemini-2.5-pro:streamGenerateContent"
)
ENDPOINT="${ENDPOINT_MAP[$PROTOCOL]:-}"
if [[ -z "$ENDPOINT" ]]; then
  echo "ERROR: unknown protocol $PROTOCOL" >&2
  exit 2
fi

# 输出路径
OUT_DIR="$FIXTURE_DIR/$PROTOCOL"
mkdir -p "$OUT_DIR"
OUT_FILE="$OUT_DIR/${FIXTURE_NAME}.sse"

# 鉴权头：Anthropic 用 x-api-key，OpenAI 用 Authorization
AUTH_HEADER=""
case "$PROTOCOL" in
  anthropic)
    AUTH_HEADER="x-api-key: $API_KEY"
    ;;
  *)
    AUTH_HEADER="Authorization: Bearer $API_KEY"
    ;;
esac

echo "[record_upstream_sse] protocol=$PROTOCOL fixture=$FIXTURE_NAME"
echo "  gateway: $GATEWAY_URL$ENDPOINT"
echo "  output:  $OUT_FILE"

# curl 持续读取 stream 到文件
# -N: 禁用缓冲（保证字节序与真实 chunk 一致）
# --raw: 不解 transfer-encoding（保留 chunk 边界）
# -s: 静默（错误用 --show-error 单独）
curl -sN --show-error \
  -X POST \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d @- \
  "$GATEWAY_URL$ENDPOINT" > "$OUT_FILE"

LINES=$(wc -l < "$OUT_FILE")
EVENTS=$(grep -c "^event:" "$OUT_FILE" || true)
echo "  recorded $LINES lines, $EVENTS events"

if (( EVENTS == 0 )); then
  echo "WARN: no event: lines detected — fixture may be invalid" >&2
  exit 1
fi
