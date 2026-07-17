#!/usr/bin/env bash
# pre_push_check.sh —— 推送前门禁：CHANGELOG 更新 + 自定义功能列表一致性 + 高风险复核 + e2e
#
# 目的：每次 push 前检查「本次推送范围」（默认 origin/<当前分支>..HEAD）内：
#   1. CHANGELOG.md 是否已随实质性代码改动更新；
#   2. 是否新增了源码文件但未在 自定义开发功能列表.md 记录（漏记新功能）；
#   3. 是否命中 自定义开发功能列表.md 风险表里登记的 fork 文件（🔴 高 / 🟡 中 / 🟢 低
#      三档全部，非仅 🔴）——命中则强制逐个打印 diff，并要求「交互终端 y/N 确认」+
#      「CHANGELOG/功能列表书面留痕（一行以『高风险复核：』开头的结论）」双重留痕，
#      防止上游合并静默覆盖 fork 逻辑；
#   4. 若范围内有实质源码改动，钩子内联跑一次 ./script/e2e-test.sh（全量 e2e，
#      需要本地 script/e2e.env 真实上游凭证），未通过则阻塞推送。
#
# 注意：命中检查 3/4 时本钩子可能耗时数分钟，且检查 3 需要交互终端（读 /dev/tty），
# 非交互环境（如 CI、无 tty 的自动化脚本）会直接失败，只能 git push --no-verify 绕过。
#
# 用法：
#   手动：  ./script/pre_push_check.sh
#   钩子：  由 .git/hooks/pre-push 调用（自动传入远端 sha 精确界定推送范围）
#   绕过：  git push --no-verify  或  PREPUSH_SKIP=1 git push（四项检查全部跳过）
#
# 退出码：0=通过；1=有阻塞项。
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

# 文档习惯用 shell brace-expansion 记法压缩同类文件（如 {a,b,c}.go、{zh,en}/fork.ts），
# 直接对原始文本做子串匹配会漏判（如 {team_handler,enterprise_handler}.go 里
# "enterprise_handler.go" 因为中间隔着 "}" 而不是连续子串）。这里先抽取文档里所有
# 形如 path/to/{a,b,c}suffix.ext 的 token，用 bash 原生 brace expansion 展开成完整
# 路径，再取 basename 建立「已记录文件名」集合，比对时用这份展开集合。
# 注意：抽取用的字符类只含 [A-Za-z0-9_./{},-]，不含 $ ` ; 等 shell 特殊字符，
# 后面对其 eval 不会执行任意命令。
DOC_BASENAMES=$(grep -oE '[A-Za-z0-9_./{},-]+\.(go|ts|vue)' "$DOC" 2>/dev/null | sort -u | while read -r tok; do
  eval "printf '%s\n' $tok" 2>/dev/null
done | xargs -n1 basename 2>/dev/null | sort -u)

UNDOC=""
if [ -n "$NEW_SRC" ]; then
  while read -r f; do
    [ -z "$f" ] && continue
    # 上游已有此文件 → 非 fork 独有，跳过
    if [ "$HAS_UPSTREAM" = "1" ] && git cat-file -e "upstream/main:$f" 2>/dev/null; then
      continue
    fi
    base=$(basename "$f")
    if ! echo "$DOC_BASENAMES" | grep -qxF "$base" 2>/dev/null; then
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

# ── 检查 3：命中功能列表登记的 fork 文件 → 强制逐个 diff + 交互确认 + 书面留痕 ──
# 背景：上游合并会「静默吞掉 fork 代码块」（已出现 ≥3 次：workflow 触发器、批量改账号
# 丢模型映射、setting_update.go fork 字段块）。编译过 + 测试绿 无法发现，只能人工逐行
# 核对文件 diff。本检查从 自定义开发功能列表.md 风险表提取**全部登记的 fork 文件**
# （🔴 高 / 🟡 中 / 🟢 低 三档，而非仅 🔴），若本次推送范围改动了其中任意文件，则：
#   (a) 逐个打印该文件在本推送范围内的 diff；
#   (b) 要求 CHANGELOG/功能列表 在本范围内新增一行以「高风险复核：」开头的书面结论；
#   (c) 交互终端 y/N 二次确认（读 /dev/tty；非交互环境无法确认 → 阻塞）。
# 从风险表（含 🔴/🟡/🟢 标记的行）第 3 列（文件列，awk -F'|' 的 $3）提取反引号包裹的文件 token，
# 过滤到安全字符集（含 { } , * 供 brace/glob，排除 shell 元字符防 eval 注入）。
REG_RAW=$(awk -F'|' '/🔴|🟡|🟢/ {print $3}' "$DOC" 2>/dev/null \
  | grep -oE '`[^`]+`' | tr -d '`' \
  | grep -E '\.(go|ts|vue|yml|sql)$' \
  | grep -vE '_test\.go|\.spec\.ts' \
  | grep -xE '[A-Za-z0-9_./{}*,-]+' \
  | sort -u || true)
# bash 展开 brace（path/{a,b,c}.go → 三行完整路径）；set -f 关闭 glob，让 * 保持字面量交给下方正则。
REG_PATTERNS=$( set -f; while IFS= read -r tok; do
    [ -n "$tok" ] && eval "printf '%s\n' $tok"
  done <<< "$REG_RAW" 2>/dev/null | sort -u )

# 分类：含 / 的转整行正则（. 转义、* → [^/]*）；裸文件名转 (^|/)name$ 做 basename 精确匹配。
# 全部并成一个正则集，对改动文件列表一次 grep -Ef（O(files+patterns)，避免逐文件×逐pattern 的慢循环）。
ALL_RX=""
while IFS= read -r p; do
  [ -z "$p" ] && continue
  case "$p" in
    */*) ALL_RX+="^$(printf '%s' "$p" | sed 's|[.]|\\.|g; s|[*]|[^/]*|g')\$"$'\n' ;;
    *)   ALL_RX+="(^|/)$(printf '%s' "$p" | sed 's|[.]|\\.|g; s|[*]|[^/]*|g')\$"$'\n' ;;
  esac
done <<< "$REG_PATTERNS"

HIT_FILES=""
if [ -n "$CHANGED" ] && [ -n "$ALL_RX" ]; then
  HIT_FILES=$(printf '%s\n' "$CHANGED" \
    | grep -Ef <(printf '%s\n' "$ALL_RX" | sed '/^$/d') 2>/dev/null \
    | sed '/^$/d' | sort -u || true)
fi

if [ -n "$HIT_FILES" ]; then
  HIT_COUNT=$(printf '%s\n' "$HIT_FILES" | sed '/^$/d' | wc -l | tr -d ' ')
  echo ""
  echo "🔎 本次推送范围命中 ${HIT_COUNT} 个功能列表登记的 fork 文件（🔴/🟡/🟢）："
  printf '%s\n' "$HIT_FILES" | sed 's/^/     /'
  echo "   ── 以下逐个打印 diff，请逐行核对 fork 逻辑是否被上游静默覆盖 ──"
  while read -r f; do
    [ -z "$f" ] && continue
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "📄 $f"
    echo "────────────────────────────────────────────────────────────────"
    git diff "${RANGE}" -- "$f" || true
  done <<< "$HIT_FILES"

  # (b) 书面留痕：本范围内 CHANGELOG/功能列表 须新增一行以「高风险复核：」开头
  REVIEW_NOTE=$(git diff "${RANGE}" -- "$CHANGELOG" "$DOC" 2>/dev/null \
    | grep -E '^\+' | grep -F '高风险复核：' || true)
  echo ""
  if [ -n "$REVIEW_NOTE" ]; then
    echo "✅ 已找到高风险复核书面留痕："
    printf '%s\n' "$REVIEW_NOTE" | sed 's/^+/     /'
  else
    echo "❌ 缺少高风险复核书面留痕：请在 $CHANGELOG 或 $DOC 内新增一行，以"
    echo "   「高风险复核：」开头，写明已核对上述文件 diff 的结论（如：高风险复核：已逐个核对"
    echo "   config.go/wire_gen.go，fork 字段与注入链完整，未被上游覆盖）。"
    FAIL=1
  fi

  # (c) 交互终端二次确认
  echo ""
  if [ -r /dev/tty ] && [ -w /dev/tty ]; then
    printf '❓ 已逐个核对以上登记 fork 文件 diff，确认无 fork 逻辑被上游静默覆盖？[y/N] ' > /dev/tty
    read -r REG_ANS < /dev/tty || REG_ANS=""
    case "$REG_ANS" in
      y|Y|yes|YES|Yes)
        echo "✅ 登记 fork 文件复核已确认" ;;
      *)
        echo "❌ 未确认登记 fork 文件复核（回答非 y），阻塞推送"
        FAIL=1 ;;
    esac
  else
    echo "❌ 命中登记 fork 文件但当前无交互终端（/dev/tty 不可用，如 GUI 客户端/CI）。"
    echo "   请改用命令行 git push 完成人工确认，或 git push --no-verify 绕过全部门禁。"
    FAIL=1
  fi
fi

# ── 检查 4：实质源码改动 → 内联跑全量 e2e（最慢，放最后；前置已失败则跳过）──────
if [ -n "$SRC_CHANGED" ]; then
  if [ "$FAIL" = "1" ]; then
    echo ""
    echo "⏭  前置检查已失败，跳过 e2e 全量测试（请先修复上面的阻塞项再重推）"
  else
    echo ""
    echo "▶ 检测到实质源码改动，内联运行全量 e2e（./script/e2e-test.sh，需 script/e2e.env）..."
    if [ ! -f "script/e2e.env" ] && [ -z "${E2E_ANTHROPIC_UPSTREAM_KEY:-}" ]; then
      echo "❌ 缺少 script/e2e.env（且环境未设 E2E_ANTHROPIC_UPSTREAM_KEY），无法运行 e2e。"
      echo "   请 cp script/e2e.env.example script/e2e.env 并填入真实上游凭证，或 git push --no-verify 绕过。"
      FAIL=1
    elif ./script/e2e-test.sh; then
      echo "✅ e2e 全量测试通过"
    else
      echo "❌ e2e 全量测试未通过，阻塞推送"
      FAIL=1
    fi
  fi
fi

echo ""
if [ "$FAIL" = "1" ]; then
  echo "🛑 pre-push 检查未通过（见上）。修复后重推，或 git push --no-verify 强制。"
  exit 1
fi
echo "✅ pre-push 检查通过"
exit 0
