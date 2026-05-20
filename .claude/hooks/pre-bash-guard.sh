#!/usr/bin/env bash
# Pre-bash guard：拦截对本项目有破坏性的 git 操作
# 从 stdin 读取 Claude Code 传入的 JSON，提取 command 字段进行检查
# 退出码 2 = 拦截（输出到 stderr 的内容会作为提示显示给 Claude）

INPUT=$(cat)
CMD=$(echo "$INPUT" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    cmd = d.get('tool_input', {}).get('command', '')
    print(cmd)
except Exception:
    pass
" 2>/dev/null || true)

[[ -z "$CMD" ]] && exit 0

# git commit 命令不做危险模式检测（commit message 中可能包含示例代码）
if echo "$CMD" | grep -qE '^\s*git\s+commit\b'; then
  exit 0
fi

# ── 规则 1：禁止 force push 到 main / master / zhiguofan ──────────────────
if echo "$CMD" | grep -qE 'git\s+push.*(--force|-f)'; then
  TARGET=$(echo "$CMD" | grep -oE '(main|master|zhiguofan)' | head -1)
  if [[ -n "$TARGET" ]]; then
    echo "⛔ [guards] 拦截：禁止 force push 到受保护分支 '$TARGET'。" >&2
    echo "   如确需操作，请在终端直接执行并自行确认风险。" >&2
    exit 2
  fi
fi

# ── 规则 2：禁止直接 push origin main（非 force 也拦截）───────────────────
if echo "$CMD" | grep -qE 'git\s+push\s+(origin\s+)?main\b'; then
  echo "⛔ [guards] 拦截：不应直接推送到 main 分支。" >&2
  echo "   main 分支仅通过 /sync-upstream 流程更新。" >&2
  exit 2
fi

# ── 规则 3：禁止 git reset --hard（非 HEAD 相对引用）────────────────────────
# 允许：git reset --hard HEAD（撤销工作区改动）
# 拦截：git reset --hard <sha> / git reset --hard HEAD~N
if echo "$CMD" | grep -qE 'git\s+reset\s+--hard'; then
  if ! echo "$CMD" | grep -qE 'git\s+reset\s+--hard\s+HEAD$'; then
    echo "⛔ [guards] 拦截：git reset --hard 到非 HEAD 目标是破坏性操作。" >&2
    echo "   如确认，请在终端直接执行并明确 commit 可追回方式（reflog）。" >&2
    exit 2
  fi
fi

# ── 规则 4：禁止删除受保护分支 ────────────────────────────────────────────
if echo "$CMD" | grep -qE 'git\s+branch\s+-[Dd]\s+(main|master|zhiguofan)\b'; then
  BRANCH=$(echo "$CMD" | grep -oE '(main|master|zhiguofan)' | head -1)
  echo "⛔ [guards] 拦截：禁止删除受保护分支 '$BRANCH'。" >&2
  exit 2
fi

# ── 规则 5：禁止 git checkout main/zhiguofan 后直接提交 ──────────────────
# （只检测切换到受保护分支的动作，配合 Stop hook 的状态提示）
if echo "$CMD" | grep -qE 'git\s+checkout\s+(main|zhiguofan)\b'; then
  TARGET=$(echo "$CMD" | grep -oE '(main|zhiguofan)' | head -1)
  # 只警告，不拦截（切换分支本身无害）
  echo "⚠️  [guards] 注意：即将切换到受保护分支 '$TARGET'，请勿直接在此分支提交开发代码。" >&2
fi

exit 0
