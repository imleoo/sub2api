#!/usr/bin/env bash
set -euo pipefail

INTERNAL_REMOTE_NAME="${INTERNAL_REMOTE_NAME:-internal}"
INTERNAL_REMOTE_URL="${INTERNAL_REMOTE_URL:-ssh://git_prod_backend@192.168.1.10/home/git_prod_backend/wmtoken_platform.git}"
BRANCH="${BRANCH:-zhiguofan}"

die() {
  echo "error: $*" >&2
  exit 1
}

require_clean_worktree() {
  if [[ -n "$(git status --porcelain)" ]]; then
    echo "Working tree is not clean. Please commit, stash, or discard local changes first." >&2
    git status --short >&2
    exit 1
  fi
}

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
[[ -n "$repo_root" ]] || die "Run this script inside a git repository."
cd "$repo_root"

require_clean_worktree

current_branch="$(git branch --show-current)"
[[ "$current_branch" == "$BRANCH" ]] || die "Please switch to ${BRANCH} before pushing. Current branch: ${current_branch:-detached}"

if git remote get-url "$INTERNAL_REMOTE_NAME" >/dev/null 2>&1; then
  git remote set-url "$INTERNAL_REMOTE_NAME" "$INTERNAL_REMOTE_URL"
else
  git remote add "$INTERNAL_REMOTE_NAME" "$INTERNAL_REMOTE_URL"
fi

echo "Pushing ${BRANCH} to ${INTERNAL_REMOTE_NAME} (${INTERNAL_REMOTE_URL})..."
git push -u "$INTERNAL_REMOTE_NAME" "$BRANCH"

echo "Internal push complete."
