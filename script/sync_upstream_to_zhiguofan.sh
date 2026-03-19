#!/usr/bin/env bash
set -euo pipefail

UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-upstream}"
UPSTREAM_BRANCH="${UPSTREAM_BRANCH:-main}"
GITHUB_REMOTE="${GITHUB_REMOTE:-origin}"
MAIN_BRANCH="${MAIN_BRANCH:-main}"
WORK_BRANCH="${WORK_BRANCH:-zhiguofan}"
UPSTREAM_MERGE_MSG="${UPSTREAM_MERGE_MSG:-chore: sync from upstream Wei-Shaw/sub2api:main}"
WORK_MERGE_MSG="${WORK_MERGE_MSG:-chore: sync from origin/main}"

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

original_branch="$(git branch --show-current)"
cleanup() {
  if [[ -n "${original_branch:-}" ]] && [[ "$(git branch --show-current)" != "$original_branch" ]]; then
    git switch "$original_branch" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

git remote get-url "$UPSTREAM_REMOTE" >/dev/null 2>&1 || die "Missing remote: $UPSTREAM_REMOTE"
git remote get-url "$GITHUB_REMOTE" >/dev/null 2>&1 || die "Missing remote: $GITHUB_REMOTE"

echo "Fetching ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}..."
git fetch "$UPSTREAM_REMOTE" "$UPSTREAM_BRANCH"

echo "Switching to ${MAIN_BRANCH}..."
git switch "$MAIN_BRANCH"

main_behind="$(git rev-list --count "HEAD..${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}")"
if [[ "$main_behind" != "0" ]]; then
  echo "Merging ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH} into ${MAIN_BRANCH} (${main_behind} commit(s) behind)..."
  git merge "${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}" --no-edit -m "$UPSTREAM_MERGE_MSG"
else
  echo "${MAIN_BRANCH} is already up to date with ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}."
fi

echo "Pushing ${MAIN_BRANCH} to ${GITHUB_REMOTE}..."
git push "$GITHUB_REMOTE" "$MAIN_BRANCH"

echo "Fetching ${GITHUB_REMOTE}/${MAIN_BRANCH}..."
git fetch "$GITHUB_REMOTE" "$MAIN_BRANCH"

echo "Switching to ${WORK_BRANCH}..."
git switch "$WORK_BRANCH"

work_behind="$(git rev-list --count "HEAD..${GITHUB_REMOTE}/${MAIN_BRANCH}")"
if [[ "$work_behind" != "0" ]]; then
  echo "Merging ${GITHUB_REMOTE}/${MAIN_BRANCH} into ${WORK_BRANCH} (${work_behind} commit(s) behind)..."
  git merge "${GITHUB_REMOTE}/${MAIN_BRANCH}" --no-edit -m "$WORK_MERGE_MSG"
else
  echo "${WORK_BRANCH} is already up to date with ${GITHUB_REMOTE}/${MAIN_BRANCH}."
fi

echo "Pushing ${WORK_BRANCH} to ${GITHUB_REMOTE}..."
git push "$GITHUB_REMOTE" "$WORK_BRANCH"

echo "Sync complete."
