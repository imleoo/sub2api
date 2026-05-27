#!/usr/bin/env bash
# test_generic_runtime.sh — 功能 25 集成测试脚本（在线，需要真实 API Key）。
#
# 用法：
#   export BACKEND_URL=http://localhost:8082       # 默认 http://localhost:8082
#   export USER_API_KEY=cr_...                     # 你的 generic 账号所在分组的用户 API Key
#   export OPENAI_MODEL=qwen3.7-max                # generic 转发的 openai 模型名
#   export ANTHROPIC_MODEL=claude-3-5-sonnet       # generic 转发的 anthropic 模型名
#   export GEMINI_MODEL=gemini-2.0-flash           # generic 转发的 gemini 模型名
#   ./script/test_generic_runtime.sh
#
# 前置条件：
#   1. 后台创建 generic 账号（万界方舟）：账号级 API Key + 三个端点：
#        - openai_chat:        https://maas-openapi.wanjiedata.com/api
#        - anthropic_messages: https://maas-openapi.wanjiedata.com/api/anthropic
#        - gemini_v1beta:      https://maas-openapi.wanjiedata.com/api
#   2. 该账号加入 openai/anthropic/gemini 入站分组，分组下生成 USER_API_KEY。
#
# 注：generic 运行时默认开启。如需回滚旧行为，设 GATEWAY_SCHEDULING_GENERIC_RUNTIME_ENABLED=false。
#
# 输出：每条协议命中/失败的 HTTP 状态码，便于快速判断哪条不通。
set -u

BACKEND_URL="${BACKEND_URL:-http://localhost:8082}"
USER_API_KEY="${USER_API_KEY:-}"
OPENAI_MODEL="${OPENAI_MODEL:-qwen3.7-max}"
ANTHROPIC_MODEL="${ANTHROPIC_MODEL:-claude-3-5-sonnet}"
GEMINI_MODEL="${GEMINI_MODEL:-gemini-2.0-flash}"

if [[ -z "$USER_API_KEY" ]]; then
  echo "ERROR: 请先 export USER_API_KEY=..."
  exit 1
fi

red()   { printf "\033[31m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
hdr()   { printf "\n\033[1;34m== %s ==\033[0m\n" "$*"; }

fail=0
check_status() {
  local label="$1" status="$2"
  if [[ "$status" =~ ^2 ]]; then
    green "[$label] HTTP $status OK"
  else
    red   "[$label] HTTP $status FAIL"
    fail=1
  fi
}

# ---------- 1) OpenAI Chat Completions（chat/completions 原生直通）----------
hdr "1) OpenAI /v1/chat/completions  → 万界方舟 openai_chat"
status=$(curl -sS -o /tmp/wanjie_openai.json -w "%{http_code}" \
  -X POST "$BACKEND_URL/v1/chat/completions" \
  -H "Authorization: Bearer $USER_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$OPENAI_MODEL\",\"stream\":false,\"messages\":[{\"role\":\"user\",\"content\":\"ping\"}]}")
check_status "openai_chat" "$status"
[[ "$status" =~ ^2 ]] && head -c 200 /tmp/wanjie_openai.json && echo

# ---------- 2) Anthropic Messages（API Key 直通）----------
hdr "2) Anthropic /v1/messages  → 万界方舟 anthropic_messages"
status=$(curl -sS -o /tmp/wanjie_anthropic.json -w "%{http_code}" \
  -X POST "$BACKEND_URL/v1/messages" \
  -H "x-api-key: $USER_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$ANTHROPIC_MODEL\",\"max_tokens\":32,\"stream\":false,\"messages\":[{\"role\":\"user\",\"content\":\"ping\"}]}")
check_status "anthropic_messages" "$status"
[[ "$status" =~ ^2 ]] && head -c 200 /tmp/wanjie_anthropic.json && echo

# ---------- 3) Gemini :generateContent（v1beta 直通）----------
hdr "3) Gemini /v1beta/models/${GEMINI_MODEL}:generateContent  → 万界方舟 gemini_v1beta"
status=$(curl -sS -o /tmp/wanjie_gemini.json -w "%{http_code}" \
  -X POST "$BACKEND_URL/v1beta/models/${GEMINI_MODEL}:generateContent" \
  -H "x-goog-api-key: $USER_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"contents\":[{\"role\":\"user\",\"parts\":[{\"text\":\"ping\"}]}]}")
check_status "gemini_v1beta" "$status"
[[ "$status" =~ ^2 ]] && head -c 200 /tmp/wanjie_gemini.json && echo

# ---------- 4) UsageLog 端点归因检查（如有 Postgres 访问可手动验证）----------
hdr "4) UsageLog endpoint_id 归因验证（可选）"
cat <<'EOF'
若有 PostgreSQL 访问，执行：
  psql -c "SELECT account_id, model, endpoint_id, created_at FROM usage_logs
           WHERE endpoint_id IS NOT NULL ORDER BY id DESC LIMIT 3"
预期：刚才的请求会产生三行（endpoint_id 分别为 wanjie-openai / wanjie-anthropic / wanjie-gemini，与你建账号时填的 stable_id 一致）。
EOF

echo
if [[ "$fail" -eq 0 ]]; then
  green "三协议直通全部通过 ✓"
else
  red   "存在失败项，请检查后端日志（grep account_id/endpoint_id/forward）"
  exit 1
fi
