#!/usr/bin/env bash
# pre_push_check.sh —— 推送前门禁：CHANGELOG 更新 + 自定义功能列表一致性
#
# 目的：每次 push 前检查「本次推送范围」（默认 origin/<当前分支>..HEAD）内：
#   1. CHANGELOG.md 是否已随实质性代码改动更新；
#   2. 是否新增了源码文件但未在 自定义开发功能列表.md 记录（漏记新功能）。
#
# 用法：
#   手动：  ./script/pre_push_check.sh
#   钩子：  由 .git/hooks/pre-push 调用（自动传入远端 sha 精确界定推送范围）
#   绕过：  git push --no-verify  或  PREPUSH_SKIP=1 git push
#
# 退出码：0=通过；1=有阻塞项（缺 CHANGELOG / 有未记录新文件）。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

DOC="自定义开发功能列表.md"
CHANGELOG="CHANGELOG.md"
BRANCH="$(git rev-parse --abbrev-ref HEAD)"

# 允许显式绕过
if [ "${PREPUSH_SKIP:-0}" = "1" ]; then
  echo "⏭  PREPUSH_SKIP=1，跳过推送前检查"; exit 0
fi

# ── 确定推送范围 BASE..HEAD ───────────────────────────────────────────────
# 钩子模式：pre-push 把每条待推 ref 以 "<localref> <localsha> <remoteref> <remotesha>"
# 写到 stdin；取第一条的 remotesha 作为 BASE（全 0 表示远端无此分支=首推）。
BASE=""
if [ ! -t 0 ]; then
  while read -r _localref _localsha _remoteref remotesha || [ -n "${_localref:-}" ]; do
    if [ -n "${remotesha:-}" ] && ! echo "$remotesha" | grep -qE '^0+$'; then
      BASE="$remotesha"; break
    fi
  done || true
fi
# 手动模式（或首推）：回退到远端跟踪分支
if [ -z "$BASE" ]; then
  if git rev-parse --verify --quiet "origin/$BRANCH" >/dev/null; then
    BASE="origin/$BRANCH"
  else
    echo "ℹ️  远端无 origin/$BRANCH（首次推送），跳过范围检查（仅提示）"
    BASE=""
  fi
fi

if [ -z "$BASE" ]; then
  echo "✅ 首推/无基线，pre-push 检查跳过"; exit 0
fi

RANGE="$BASE..HEAD"
COMMITS=$(git rev-list --count "${RANGE}" 2>/dev/null || echo 0)
if [ "$COMMITS" = "0" ]; then
  echo "✅ 无新提交待推（${RANGE}），跳过"; exit 0
fi
echo "▶ 推送前检查范围：${RANGE}（$COMMITS 个提交）"

# 范围内改动文件
CHANGED=$(git diff --name-only "${RANGE}")
# 是否有实质性源码改动（后端 go / 前端 ts,vue，排除测试）
SRC_CHANGED=$(echo "$CHANGED" | grep -E '^(backend|frontend)/.*\.(go|ts|vue)$' \
  | grep -vE '_test\.go|\.spec\.ts|/__tests__/' || true)

FAIL=0

# ── 检查 1：CHANGELOG 是否随实质源码改动更新 ──────────────────────────────
if [ -n "$SRC_CHANGED" ]; then
  if echo "$CHANGED" | grep -qx "$CHANGELOG"; then
    echo "✅ CHANGELOG.md 已在本次范围内更新"
  else
    echo "❌ 有实质源码改动但 CHANGELOG.md 未更新（范围 ${RANGE}）"
    echo "   请在 $CHANGELOG 顶部补一段本次改动，或 git push --no-verify 绕过"
    FAIL=1
  fi
  # VERSION 变了但 CHANGELOG 没提及新版本号 → 提示（非阻塞）
  if echo "$CHANGED" | grep -q 'backend/cmd/server/VERSION'; then
    VER=$(cat backend/cmd/server/VERSION 2>/dev/null || echo "")
    if [ -n "${VER}" ] && ! grep -qF "${VER}" "$CHANGELOG" 2>/dev/null; then
      echo "⚠️  VERSION=${VER} 有改动但 CHANGELOG 未提及该版本号（建议补版本段）"
    fi
  fi
else
  echo "✅ 本次范围无实质源码改动，CHANGELOG 检查跳过"
fi

# ── 检查 2：新增源码文件是否已在功能列表记录 ──────────────────────────────
# 仅看「本次范围新增（A）」的源码文件，排除测试/生成码/迁移——低噪声。
NEW_SRC=$(git diff --diff-filter=A --name-only "${RANGE}" \
  | grep -E '\.(go|ts|vue)$' \
  | grep -vE '_test\.go|\.spec\.ts|/__tests__/|/ent/|zz_generated|wire_gen|\.d\.ts' || true)

# 若本地有 upstream/main，排除「上游本来就有」的文件（merge 带来的上游文件不算 fork 新功能）
HAS_UPSTREAM=0
git rev-parse --verify --quiet upstream/main >/dev/null && HAS_UPSTREAM=1

UNDOC=""
if [ -n "$NEW_SRC" ]; then
  while read -r f; do
    [ -z "$f" ] && continue
    # 上游已有此文件 → 非 fork 独有，跳过
    if [ "$HAS_UPSTREAM" = "1" ] && git cat-file -e "upstream/main:$f" 2>/dev/null; then
      continue
    fi
    base=$(basename "$f")
    if ! grep -qF "$base" "$DOC" 2>/dev/null; then
      UNDOC="$UNDOC$f\n"
    fi
  done <<< "$NEW_SRC"
fi

if [ -n "$UNDOC" ]; then
  echo "❌ 本次新增源码文件未在 $DOC 记录（可能漏记 fork 功能）："
  echo -e "$UNDOC" | sed '/^$/d' | sed 's/^/     /'
  echo "   请补录到功能列表（新功能加编号段 + 风险表行），或 git push --no-verify 绕过"
  FAIL=1
elif [ -n "$NEW_SRC" ]; then
  echo "✅ 本次新增源码文件均已在功能列表出现"
else
  echo "✅ 本次范围无新增源码文件"
fi

# ── 新增迁移文件（提示，不阻塞）──────────────────────────────────────────
NEW_MIG=$(git diff --diff-filter=A --name-only "${RANGE}" | grep -E 'backend/migrations/.*\.sql$' || true)
if [ -n "$NEW_MIG" ]; then
  echo "ℹ️  本次新增迁移（确认对应功能已在列表记录）："
  echo "$NEW_MIG" | sed 's/^/     /'
fi

echo ""
if [ "$FAIL" = "1" ]; then
  echo "🛑 pre-push 检查未通过（见上）。修复后重推，或 git push --no-verify 强制。"
  exit 1
fi
echo "✅ pre-push 检查通过"
exit 0
