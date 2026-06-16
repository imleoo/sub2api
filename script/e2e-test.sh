#!/usr/bin/env bash
#
# 自包含全功能 E2E runner
# ============================================================================
# 覆盖：网关多平台转发(Claude/OpenAI/Gemini) + 流式/count_tokens + 计费成本扣减
#       + 配额(quota)拦截 + 余额不足拒绝 + 限流(rate_limit_5h) + API Key 生命周期
#       + admin 账号/分组 CRUD。
# 详见 backend/internal/integration/e2e_full_test.go / e2e_full_provision_test.go。
#
# 用法：
#   # 1) 上游凭证（必需 anthropic；openai/gemini 可选，配了才跑）——务必走 env，勿写进仓库
#   export E2E_ANTHROPIC_UPSTREAM_KEY=sk-xxx
#   export E2E_ANTHROPIC_UPSTREAM_BASE_URL=https://your-upstream        # 默认 https://api.anthropic.com
#   export E2E_ANTHROPIC_MODEL=claude-sonnet-4-6
#   export E2E_OPENAI_UPSTREAM_KEY=sk-yyy                               # 可选
#   export E2E_OPENAI_UPSTREAM_BASE_URL=https://your-upstream/v1
#   export E2E_OPENAI_MODEL=gpt-4o-mini
#   # 2) 跑（默认自动用 script/dev_local.sh 起本地服务，跑完自动拆）
#   ./script/e2e-test.sh                # 或 cd backend && make test-e2e
#
# 已有运行中的服务时，跳过起服务：
#   export E2E_BASE_URL=http://127.0.0.1:8091
#   export E2E_ADMIN_EMAIL=...  E2E_ADMIN_PASSWORD=...
#   ./script/e2e-test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
DEV_LOCAL="$SCRIPT_DIR/dev_local.sh"

BACKEND_PORT="${E2E_BACKEND_PORT:-8091}"
TIMEOUT="${E2E_TIMEOUT:-600s}"
RUN_FILTER="${E2E_RUN:-TestE2EFull}"   # 默认只跑新全功能套件；设为空跑全部 e2e

BOOTED_SERVER=false

cleanup() {
  if [[ "$BOOTED_SERVER" == "true" ]]; then
    echo "🧹 拆除本地服务..."
    START_FRONTEND=false "$DEV_LOCAL" down >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# ── 前置检查 ────────────────────────────────────────────────────────────────
if [[ -z "${E2E_ANTHROPIC_UPSTREAM_KEY:-}" ]]; then
  echo "❌ 缺少 E2E_ANTHROPIC_UPSTREAM_KEY（至少需要 anthropic 上游凭证）。"
  echo "   见本脚本顶部用法说明。"
  exit 1
fi

# ── 确定 BASE_URL / 是否自起服务 ─────────────────────────────────────────────
if [[ -n "${E2E_BASE_URL:-}" ]]; then
  export BASE_URL="$E2E_BASE_URL"
  echo "▶ 使用已运行服务 ${BASE_URL} (不自动起停)"
else
  if [[ ! -x "$DEV_LOCAL" ]]; then
    echo "❌ 找不到 ${DEV_LOCAL} 无法自动起服务。请改用 E2E_BASE_URL 指向已运行实例。"
    exit 1
  fi
  echo "▶ 通过 dev_local 起本地服务 端口=${BACKEND_PORT} 跳过前端 ..."
  START_FRONTEND=false BACKEND_PORT="$BACKEND_PORT" "$DEV_LOCAL" up >/dev/null 2>&1 &
  BOOTED_SERVER=true
  export BASE_URL="http://127.0.0.1:${BACKEND_PORT}"
  # dev_local 默认管理员
  export E2E_ADMIN_EMAIL="${E2E_ADMIN_EMAIL:-admin@sub2api.local}"
  export E2E_ADMIN_PASSWORD="${E2E_ADMIN_PASSWORD:-admin123}"

  echo -n "▶ 等待健康检查"
  for _ in $(seq 1 60); do
    if curl -sS -m 3 "${BASE_URL}/health" >/dev/null 2>&1; then
      echo " ✅"
      break
    fi
    echo -n "."
    sleep 2
  done
  if ! curl -sS -m 3 "${BASE_URL}/health" >/dev/null 2>&1; then
    echo " ❌ 服务未就绪"
    exit 1
  fi
fi

# ── 跑测试 ───────────────────────────────────────────────────────────────────
echo "▶ 运行 E2E -run='${RUN_FILTER}' timeout=${TIMEOUT} ..."
cd "$BACKEND_DIR"
if [[ -n "$RUN_FILTER" ]]; then
  go test -tags=e2e -v -timeout="$TIMEOUT" -run "$RUN_FILTER" ./internal/integration/...
else
  go test -tags=e2e -v -timeout="$TIMEOUT" ./internal/integration/...
fi
