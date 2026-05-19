#!/usr/bin/env bash
# Phase 2 P2-6 fork 12 项功能合并守护检查
#
# 在 CI / merge 前运行，防止上游同步或重构误删 fork 自定义功能。
#
# 守护规则（按 docs/sprint-plan.md P2-6 + glossary.md §5 fork 锚点）：
#   1. routes/gateway.go 必须保留 promptAnalytics 中间件挂载（fork 4 引入）
#   2. routes/gateway.go 必须保留 /lingjing/v1/video 路由组（fork 12 引入）
#   3. routes/gateway.go 必须保留 ForcePlatform middleware 至少 3 处（lingjing + antigravity v1/v1beta）
#   4. service 层必须保留 LingjingPollRunner / lingjing_gateway_service / lingjing_task_port
#   5. service 层必须保留 applyDiscount + loadDiscounts（fork 5 折扣链）
#   6. service 层必须保留 IsResponseMaskingEnabled + isIdentityQuestion（fork 8 masking）
#   7. domain 必须保留 PlatformLingjing 常量
#
# 退出码：
#   0 - 全部守护规则通过
#   非 0 - 至少一条违规（具体见 stderr）

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"

FAILED=0

# 辅助函数：grep 文件必须含 pattern，否则 FAIL
check_required() {
  local label="$1"
  local file="$2"
  local pattern="$3"
  if [[ ! -f "$file" ]]; then
    echo "  [FAIL] $label: file missing: $file" >&2
    FAILED=$((FAILED + 1))
    return
  fi
  if ! grep -qE "$pattern" "$file"; then
    echo "  [FAIL] $label: pattern not found in $file" >&2
    echo "         pattern: $pattern" >&2
    FAILED=$((FAILED + 1))
    return
  fi
  echo "  [PASS] $label"
}

# 辅助函数：grep 文件至少 N 次命中 pattern
check_min_count() {
  local label="$1"
  local file="$2"
  local pattern="$3"
  local min_count="$4"
  if [[ ! -f "$file" ]]; then
    echo "  [FAIL] $label: file missing: $file" >&2
    FAILED=$((FAILED + 1))
    return
  fi
  local actual
  actual=$(grep -cE "$pattern" "$file" || true)
  if (( actual < min_count )); then
    echo "  [FAIL] $label: found $actual occurrences, expected >= $min_count" >&2
    echo "         pattern: $pattern" >&2
    FAILED=$((FAILED + 1))
    return
  fi
  echo "  [PASS] $label (found $actual occurrences)"
}

echo "== Phase 2 P2-6: fork 12 项功能合并守护 =="

# 规则 1: routes/gateway.go 必须保留 promptAnalytics 挂载
# glossary §5 fork 锚点：7 处挂载点，目前实际更多（含 fork 12 后的扩展）
check_min_count \
  "fork 4 promptAnalytics middleware mount" \
  "$BACKEND_DIR/internal/server/routes/gateway.go" \
  "promptAnalytics" \
  7

# 规则 2: lingjing 路由组完整
check_required \
  "fork 12 /lingjing/v1/video 路由组" \
  "$BACKEND_DIR/internal/server/routes/gateway.go" \
  "/lingjing/v1"

check_required \
  "fork 12 /lingjing/v1/video/submit endpoint" \
  "$BACKEND_DIR/internal/server/routes/gateway.go" \
  "video/submit"

# 规则 3: ForcePlatform middleware（lingjing + antigravity v1 + antigravity v1beta = 至少 3 处）
check_min_count \
  "ForcePlatform middleware（lingjing + antigravity x2）" \
  "$BACKEND_DIR/internal/server/routes/gateway.go" \
  "ForcePlatform\(" \
  3

# 规则 4: lingjing service 文件
check_required \
  "fork 12 lingjing_poll_runner.go 存在" \
  "$BACKEND_DIR/internal/service/lingjing_poll_runner.go" \
  "LingjingPollRunner"

check_required \
  "fork 12 lingjing_gateway_service.go 存在" \
  "$BACKEND_DIR/internal/service/lingjing_gateway_service.go" \
  "LingjingGatewayService"

check_required \
  "fork 12 lingjing_task_port.go interface" \
  "$BACKEND_DIR/internal/service/lingjing_task_port.go" \
  "LingjingTaskRepository"

# 规则 5: fork 5 折扣链
check_required \
  "fork 5 applyDiscount 函数" \
  "$BACKEND_DIR/internal/service/billing_service.go" \
  "applyDiscount"

check_required \
  "fork 5 pricing_service.go loadDiscounts" \
  "$BACKEND_DIR/internal/service/pricing_service.go" \
  "loadDiscounts"

# 规则 6: fork 8 masking
check_required \
  "fork 8 IsResponseMaskingEnabled" \
  "$BACKEND_DIR/internal/service/account.go" \
  "IsResponseMaskingEnabled"

check_required \
  "fork 8 isIdentityQuestion (gateway_response_masking)" \
  "$BACKEND_DIR/internal/service/gateway_response_masking.go" \
  "isIdentityQuestion"

# 规则 7: PlatformLingjing 常量
check_required \
  "fork 12 PlatformLingjing 常量" \
  "$BACKEND_DIR/internal/domain/constants.go" \
  "PlatformLingjing"

# 规则 8（Phase 0 联动）：UsageLog 9 列字段保留（P0-2 引入）
check_required \
  "Phase 0 P0-2 UsageLog.upstream_total_cost 字段" \
  "$BACKEND_DIR/ent/schema/usage_log.go" \
  "upstream_total_cost"

check_required \
  "Phase 0 P0-2 UsageLog.async_task_id 字段（lingjing 异步计费）" \
  "$BACKEND_DIR/ent/schema/usage_log.go" \
  "async_task_id"

# 规则 9（Phase 0 P0-4 联动）：UpstreamCostResolver 0 diff 守护
check_required \
  "Phase 0 P0-4 UpstreamCostResolver" \
  "$BACKEND_DIR/internal/service/upstream_cost.go" \
  "UpstreamCostResolver"

# 规则 10（Phase 1 P1-1 联动）：NormalizeProvider
check_required \
  "Phase 1 P1-1 NormalizeProvider" \
  "$BACKEND_DIR/internal/service/provider_normalize.go" \
  "NormalizeProvider"

# 规则 11（Phase 2 P2-2 联动）：protocol 长名常量
check_required \
  "Phase 2 P2-2 ProtocolAnthropicMessages 常量" \
  "$BACKEND_DIR/internal/domain/protocol.go" \
  "ProtocolAnthropicMessages"

echo ""
if (( FAILED == 0 )); then
  echo "== ALL FORK 12 GUARDS PASSED =="
  exit 0
fi
echo "== $FAILED CHECK(S) FAILED =="
exit 1
