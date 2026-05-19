#!/usr/bin/env bash
# Phase 0 P0-6 上游成本对账脚本
#
# 用途：每日对比 UsageLog.upstream_total_cost（命中 provider_pricing 表的真实成本快照）
# 与 actual_cost（客户售价）之间的差异，按 provider / account 维度输出 CSV 报告。
#
# 不做的事：
#   1. 不做 provider 历史回填（按 docs/upstream-cost-snapshot.md §2.3，留 Phase 1 P1-1 细颗粒回填）
#   2. 不参与 lingjing 异步任务对账（async_task_id IS NOT NULL 的行单独由 P0-7 lingjing_task vs UsageLog 对账，每日 ≤0.1% 差异）
#   3. 不参与 masking 短路对账（fork 8 短路行 upstream_total_cost=NULL，与无快照同语义，单独归类）
#
# 调用方式：
#   1. crontab 每日运行（推荐 04:00 低峰期）：0 4 * * * /path/to/reconcile_upstream_cost.sh
#   2. 手动验证：DATABASE_URL=... ./reconcile_upstream_cost.sh
#
# 环境变量：
#   DATABASE_URL  - PostgreSQL 连接串，例如 postgres://user:pass@host:port/db?sslmode=disable
#                   （必填；缺省时尝试从 .dev/local-debug/dev.env 读取，方便本地验证）
#   REPORT_DIR    - 输出 CSV 报告目录（默认 ./reports）
#   TIME_RANGE    - 对账时间窗口（默认 "1 day"，可改 "7 days" / "1 hour"）
#   DIFF_THRESHOLD - 标注阈值（默认 0.10，即 10%）

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

REPORT_DIR="${REPORT_DIR:-$ROOT_DIR/reports}"
TIME_RANGE="${TIME_RANGE:-1 day}"
DIFF_THRESHOLD="${DIFF_THRESHOLD:-0.10}"
TIMESTAMP="$(date -u +'%Y%m%dT%H%M%SZ')"

mkdir -p "$REPORT_DIR"

# 自动加载本地 dev.env（方便手动验证；CI/生产应直接传 DATABASE_URL）
DEV_ENV_FILE="$ROOT_DIR/.dev/local-debug/dev.env"
if [[ -z "${DATABASE_URL:-}" && -f "$DEV_ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$DEV_ENV_FILE"
  if [[ -n "${DATABASE_HOST:-}" && -n "${DATABASE_PORT:-}" && -n "${DATABASE_USER:-}" && -n "${DATABASE_DBNAME:-}" ]]; then
    DATABASE_URL="postgres://${DATABASE_USER}:${DATABASE_PASSWORD:-}@${DATABASE_HOST}:${DATABASE_PORT}/${DATABASE_DBNAME}?sslmode=${DATABASE_SSLMODE:-disable}"
  fi
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "ERROR: DATABASE_URL must be set" >&2
  exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "ERROR: psql is required" >&2
  exit 1
fi

# 维度 1：按 provider 聚合差异
PROVIDER_REPORT="$REPORT_DIR/upstream_cost_provider_${TIMESTAMP}.csv"

# 维度 2：按 account 聚合差异
ACCOUNT_REPORT="$REPORT_DIR/upstream_cost_account_${TIMESTAMP}.csv"

# 维度 3：异常摘要（同步路径无快照 / pricing_source 与 cost 不同步 / 单价为零）
SUMMARY_REPORT="$REPORT_DIR/upstream_cost_summary_${TIMESTAMP}.csv"

echo "[reconcile_upstream_cost] window=${TIME_RANGE}  threshold=${DIFF_THRESHOLD}"
echo "  provider report → $PROVIDER_REPORT"
echo "  account  report → $ACCOUNT_REPORT"
echo "  summary  report → $SUMMARY_REPORT"

# --- 维度 1：provider 聚合 ---
# 排除异步任务（async_task_id IS NOT NULL，由 P0-7 独立对账）
# 仅扫 upstream_total_cost IS NOT NULL（命中行）；NULL 行另行归类
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -A -F"," --pset=footer=off -c "
COPY (
  SELECT
    provider,
    COUNT(*) AS sample_count,
    ROUND(SUM(upstream_total_cost)::numeric, 6) AS snapshot_sum,
    ROUND(SUM(actual_cost)::numeric, 6) AS actual_sum,
    ROUND((SUM(actual_cost) - SUM(upstream_total_cost))::numeric, 6) AS margin,
    CASE
      WHEN SUM(upstream_total_cost) = 0 THEN NULL
      ELSE ROUND(((SUM(actual_cost) - SUM(upstream_total_cost)) / SUM(upstream_total_cost))::numeric, 4)
    END AS margin_ratio,
    CASE
      WHEN SUM(upstream_total_cost) = 0 THEN 'ZERO_COST'
      WHEN ABS((SUM(actual_cost) - SUM(upstream_total_cost)) / NULLIF(SUM(upstream_total_cost), 0)) >= ${DIFF_THRESHOLD}
        THEN 'EXCEEDS_THRESHOLD'
      ELSE 'OK'
    END AS flag
  FROM usage_logs
  WHERE created_at >= NOW() - INTERVAL '${TIME_RANGE}'
    AND upstream_total_cost IS NOT NULL
    AND pricing_source = 'provider_table'
    AND async_task_id IS NULL  -- 排除 lingjing 异步任务（P0-7 独立对账）
  GROUP BY provider
  ORDER BY snapshot_sum DESC NULLS LAST
) TO STDOUT WITH CSV HEADER
" > "$PROVIDER_REPORT"

# --- 维度 2：account 聚合 ---
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -A -F"," --pset=footer=off -c "
COPY (
  SELECT
    account_id,
    provider,
    COUNT(*) AS sample_count,
    ROUND(SUM(upstream_total_cost)::numeric, 6) AS snapshot_sum,
    ROUND(SUM(actual_cost)::numeric, 6) AS actual_sum,
    ROUND((SUM(actual_cost) - SUM(upstream_total_cost))::numeric, 6) AS margin
  FROM usage_logs
  WHERE created_at >= NOW() - INTERVAL '${TIME_RANGE}'
    AND upstream_total_cost IS NOT NULL
    AND pricing_source = 'provider_table'
    AND async_task_id IS NULL
  GROUP BY account_id, provider
  ORDER BY snapshot_sum DESC NULLS LAST
) TO STDOUT WITH CSV HEADER
" > "$ACCOUNT_REPORT"

# --- 维度 3：异常摘要 ---
# 包括：
#   - has_snapshot_sync：同步路径命中行（基线）
#   - no_snapshot_sync：同步路径未命中（async_task_id IS NULL + upstream_total_cost IS NULL，含 masking 短路 fork 8）
#   - pending_async：异步路径待回填（async_task_id IS NOT NULL + upstream_total_cost IS NULL，fork 12 lingjing）
#   - finalized_async：异步路径已回填（async_task_id IS NOT NULL + upstream_total_cost IS NOT NULL）
#   - inconsistent：违反一致性约束（upstream_total_cost 与 pricing_source 不同步，应为 0）
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -A -F"," --pset=footer=off -c "
COPY (
  SELECT
    'has_snapshot_sync' AS bucket,
    COUNT(*) AS sample_count
  FROM usage_logs
  WHERE created_at >= NOW() - INTERVAL '${TIME_RANGE}'
    AND async_task_id IS NULL
    AND upstream_total_cost IS NOT NULL
    AND pricing_source = 'provider_table'
  UNION ALL
  SELECT 'no_snapshot_sync', COUNT(*)
  FROM usage_logs
  WHERE created_at >= NOW() - INTERVAL '${TIME_RANGE}'
    AND async_task_id IS NULL
    AND upstream_total_cost IS NULL
    AND pricing_source IS NULL
  UNION ALL
  SELECT 'pending_async', COUNT(*)
  FROM usage_logs
  WHERE created_at >= NOW() - INTERVAL '${TIME_RANGE}'
    AND async_task_id IS NOT NULL
    AND upstream_total_cost IS NULL
  UNION ALL
  SELECT 'finalized_async', COUNT(*)
  FROM usage_logs
  WHERE created_at >= NOW() - INTERVAL '${TIME_RANGE}'
    AND async_task_id IS NOT NULL
    AND upstream_total_cost IS NOT NULL
  UNION ALL
  SELECT 'inconsistent_constraint_violation', COUNT(*)
  FROM usage_logs
  WHERE created_at >= NOW() - INTERVAL '${TIME_RANGE}'
    AND ((upstream_total_cost IS NULL) <> (pricing_source IS NULL))
) TO STDOUT WITH CSV HEADER
" > "$SUMMARY_REPORT"

# 一致性约束告警：若发现违反（不应存在）
INCONSISTENT=$(awk -F, '$1 == "inconsistent_constraint_violation" { print $2 }' "$SUMMARY_REPORT" | tr -d '[:space:]')
if [[ -n "$INCONSISTENT" && "$INCONSISTENT" != "0" ]]; then
  echo "ALERT: $INCONSISTENT rows violate constraint (upstream_total_cost IS NULL XOR pricing_source IS NULL)" >&2
  echo "       see docs/upstream-cost-snapshot.md §3.1 + 验收用例 7" >&2
  exit 2
fi

# 阈值告警：列出 EXCEEDS_THRESHOLD 的 provider
EXCEEDED=$(awk -F, 'NR > 1 && $7 == "EXCEEDS_THRESHOLD" { print $1 }' "$PROVIDER_REPORT" || true)
if [[ -n "$EXCEEDED" ]]; then
  echo "WARN: providers with margin ratio diff >= ${DIFF_THRESHOLD}:" >&2
  echo "$EXCEEDED" >&2
fi

echo "[reconcile_upstream_cost] done"
