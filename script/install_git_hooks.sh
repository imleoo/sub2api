#!/usr/bin/env bash
# install_git_hooks.sh —— 安装 fork 的 git 钩子（版本化，可复现）
#
# .git/hooks/ 不进版本库，克隆后运行本脚本即可装上 pre-push 检查。
# 幂等：重复运行覆盖为最新。
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
HOOK="$ROOT/.git/hooks/pre-push"

cat > "$HOOK" <<'EOF'
#!/usr/bin/env bash
# 自动生成（script/install_git_hooks.sh）——请勿手改；改逻辑改 script/pre_push_check.sh
# git 通过 stdin 传入待推 ref 信息，exec 透传给检查脚本。
ROOT="$(git rev-parse --show-toplevel)"
exec "$ROOT/script/pre_push_check.sh"
EOF

chmod +x "$HOOK"
echo "✅ 已安装 pre-push 钩子 → $HOOK"
echo "   逻辑在 script/pre_push_check.sh；绕过用 git push --no-verify"
