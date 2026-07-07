#!/usr/bin/env bash
set -euo pipefail

UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-upstream}"
UPSTREAM_BRANCH="${UPSTREAM_BRANCH:-main}"
GITHUB_REMOTE="${GITHUB_REMOTE:-origin}"
MAIN_BRANCH="${MAIN_BRANCH:-main}"
WORK_BRANCH="${WORK_BRANCH:-zhiguofan}"
WORK_VERSION_MAJOR="${WORK_VERSION_MAJOR:-1}"
VERSION_FILE="${VERSION_FILE:-backend/cmd/server/VERSION}"
UPSTREAM_MERGE_MSG="${UPSTREAM_MERGE_MSG:-chore: sync from upstream Wei-Shaw/sub2api:main}"
WORK_MERGE_MSG="${WORK_MERGE_MSG:-chore: sync from origin/main}"
WORK_REMOTE_SYNC_MSG="${WORK_REMOTE_SYNC_MSG:-chore: sync with origin/zhiguofan}"
WORK_VERSION_MSG="${WORK_VERSION_MSG:-chore: sync zhiguofan version}"

# Whether to attempt Codebuddy-assisted conflict resolution.
#
# Default is intentionally off: merge conflicts in this fork often touch large
# service files and generated files, and an automatic rewrite can silently drop
# code. Enable only for a deliberate one-off run:
#
#   AI_RESOLVE=true ./script/sync_upstream_to_zhiguofan.sh
AI_RESOLVE="${AI_RESOLVE:-false}"

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

# ─── AI-assisted conflict resolution ─────────────────────────────────────────
# Returns 0 if all remaining conflicts were resolved, 1 if some need human help.
ai_resolve_conflicts() {
  local target_branch="$1"
  local source_ref="$2"

  # Collect files still in conflict (UU = both modified)
  local conflict_files
  conflict_files=$(git status --porcelain | awk '/^UU / {print $2}')

  if [[ -z "$conflict_files" ]]; then
    return 0
  fi

  echo
  echo "==> Invoking Codebuddy AI to evaluate $(echo "$conflict_files" | wc -l | tr -d ' ') conflicted file(s)..."

  local unresolved_files=()

  while IFS= read -r file; do
    echo
    echo "--- Evaluating: ${file} ---"

    local file_content
    file_content=$(cat "$file" 2>/dev/null || echo "")

    # Build the prompt for codebuddy
    local prompt
    prompt=$(cat <<PROMPT
You are resolving a git merge conflict.
- Base branch (keep our customisations): ${target_branch}
- Incoming branch (new upstream changes): ${source_ref}
- File path: ${file}

Below is the full file content with conflict markers (<<<<<<<, =======, >>>>>>>).
Your task:
1. Determine whether this conflict can be resolved automatically without ambiguity.
2. If YES: output ONLY the fully resolved file content (no conflict markers, no explanation, no markdown fences).
3. If NO (logic conflict, semantic ambiguity, or you are unsure): output exactly one line:
   CANNOT_RESOLVE: <short reason>

Rules:
- For generated/auto-formatted files (wire_gen.go, *.pb.go, *_gen.go, go.sum, package-lock.json):
  prefer the INCOMING (${source_ref}) version entirely.
- For VERSION files: use whichever version is higher.
- For code files with clear non-overlapping additions: merge both changes.
- If the conflict is a genuine logic/semantic conflict: output CANNOT_RESOLVE.

File content:
${file_content}
PROMPT
)

    local ai_output
    if ! ai_output=$(echo "$prompt" | codebuddy --print 2>/dev/null); then
      echo "  [warn] Codebuddy invocation failed for ${file}. Skipping AI resolution."
      unresolved_files+=("$file")
      continue
    fi

    # Trim leading/trailing whitespace from the first line for detection
    local first_line
    first_line=$(echo "$ai_output" | head -n1 | sed 's/^[[:space:]]*//')

    if [[ "$first_line" == CANNOT_RESOLVE* ]]; then
      local reason="${first_line#CANNOT_RESOLVE: }"
      echo "  [AI] Cannot auto-resolve: ${reason}"
      unresolved_files+=("$file")
    else
      # Validate: resolved output must not still contain conflict markers
      if [[ -z "$(echo "$ai_output" | tr -d '[:space:]')" ]]; then
        echo "  [warn] Codebuddy returned empty output. Flagging for human review."
        unresolved_files+=("$file")
      elif echo "$ai_output" | grep -qE '^(<<<<<<<|=======|>>>>>>>)'; then
        echo "  [warn] AI output still contains conflict markers. Flagging for human review."
        unresolved_files+=("$file")
      else
        echo "$ai_output" > "$file"
        git add "$file"
        echo "  [AI] Resolved and staged: ${file}"
      fi
    fi

  done <<< "$conflict_files"

  if [[ ${#unresolved_files[@]} -eq 0 ]]; then
    return 0
  fi

  echo
  echo "==> AI could not resolve the following file(s). Human intervention required:"
  for f in "${unresolved_files[@]}"; do
    echo "    - ${f}"
  done
  return 1
}

# ─── Core merge helper ────────────────────────────────────────────────────────
merge_with_conflict_help() {
  local target_branch="$1"
  local source_ref="$2"
  local merge_msg="$3"
  local conflict_policy="$4"

  echo "Merging ${source_ref} into ${target_branch}..."
  if git merge "$source_ref" --no-edit -m "$merge_msg"; then
    if [[ -f "backend/cmd/server/VERSION" ]]; then
      echo ">>> Successfully merged ${source_ref} into ${target_branch}. Current version: $(cat backend/cmd/server/VERSION)"
    else
      echo "Successfully merged ${source_ref} into ${target_branch}."
    fi
    return 0
  fi

  # ── Step 1: Auto-resolve VERSION file conflict ──────────────────────────────
  local version_file="backend/cmd/server/VERSION"
  if git status --porcelain | grep -q "^UU ${version_file}"; then
    echo "Conflict detected in ${version_file}. Auto-resolving..."
    local main_v
    main_v=$(git show "${MAIN_BRANCH}:${version_file}" 2>/dev/null || echo "0.0.0")
    local new_v
    new_v=$(echo "$main_v" | awk -F. 'BEGIN{OFS="."} {$1=$1+1; print $0}')
    echo "$new_v" > "$version_file"
    git add "$version_file"
    echo "Resolved ${version_file} to ${new_v} (incremented from ${MAIN_BRANCH} version)."
  fi

  # ── Step 2: AI-assisted resolution for remaining conflicts ──────────────────
  local ai_success=false
  if [[ "$AI_RESOLVE" == "true" ]] && command -v codebuddy >/dev/null 2>&1; then
    if ai_resolve_conflicts "$target_branch" "$source_ref"; then
      ai_success=true
    fi
  else
    if [[ "$AI_RESOLVE" != "true" ]]; then
      echo "(AI resolution disabled via AI_RESOLVE=false)"
    else
      echo "(codebuddy not found in PATH — skipping AI resolution)"
    fi
  fi

  # ── Step 3: Check if all conflicts are now resolved ─────────────────────────
  if ! git status --porcelain | grep -q "^UU "; then
    echo
    echo "All conflicts resolved. Completing merge..."
    if git commit --no-edit; then
      if [[ -f "backend/cmd/server/VERSION" ]]; then
        echo ">>> Merge completed. Current version: $(cat backend/cmd/server/VERSION)"
      fi
      return 0
    fi
  fi

  # ── Step 4: Still have unresolved conflicts — prompt human ─────────────────
  echo
  echo "╔══════════════════════════════════════════════════════════════════╗"
  echo "║           MANUAL INTERVENTION REQUIRED                          ║"
  echo "╚══════════════════════════════════════════════════════════════════╝"
  echo
  echo "Merge conflict detected while merging ${source_ref} into ${target_branch}."
  if [[ "$ai_success" == "false" ]] && [[ "$AI_RESOLVE" == "true" ]]; then
    echo "Codebuddy AI evaluated the conflicts but could not resolve all of them."
  fi
  echo
  echo "Policy hint: ${conflict_policy}"
  echo
  echo "Steps to resolve manually:"
  echo "  1. Open each conflicted file listed below and fix the conflict markers."
  echo "  2. git add <resolved-file>"
  echo "  3. git merge --continue"
  echo "  To abort entirely: git merge --abort"
  echo
  echo "Remaining conflicted files:"
  git status --short | grep "^UU"
  exit 1
}

sync_work_branch_version() {
  [[ -f "$VERSION_FILE" ]] || return 0

  local main_v
  main_v=$(git show "${MAIN_BRANCH}:${VERSION_FILE}" 2>/dev/null || cat "$VERSION_FILE")

  local work_v
  work_v=$(echo "$main_v" | awk -F. -v major="$WORK_VERSION_MAJOR" 'BEGIN{OFS="."} NF >= 1 {$1=major; print $0}')
  [[ -n "$work_v" ]] || die "Failed to derive ${WORK_BRANCH} version from ${MAIN_BRANCH}:${VERSION_FILE}"

  local current_v
  current_v=$(cat "$VERSION_FILE")
  if [[ "$current_v" == "$work_v" ]]; then
    echo "${VERSION_FILE} already uses ${WORK_BRANCH} version: ${work_v}"
    return 0
  fi

  echo "$work_v" > "$VERSION_FILE"
  git add "$VERSION_FILE"
  git commit -m "$WORK_VERSION_MSG"
  echo "Updated ${VERSION_FILE}: ${current_v} -> ${work_v}"
}

# ─── Main ─────────────────────────────────────────────────────────────────────
repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
[[ -n "$repo_root" ]] || die "Run this script inside a git repository."
cd "$repo_root"

require_clean_worktree

original_branch="$(git branch --show-current)"
switched_branch=false
completed=false
cleanup() {
  if [[ "$completed" == "true" && "$switched_branch" == "true" ]] && [[ -n "${original_branch:-}" ]] && [[ "$(git branch --show-current)" != "$original_branch" ]]; then
    git switch "$original_branch" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

git remote get-url "$UPSTREAM_REMOTE" >/dev/null 2>&1 || die "Missing remote: $UPSTREAM_REMOTE"
git remote get-url "$GITHUB_REMOTE" >/dev/null 2>&1 || die "Missing remote: $GITHUB_REMOTE"

echo "Fetching ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}..."
git fetch "$UPSTREAM_REMOTE" "$UPSTREAM_BRANCH"

echo "Fetching ${GITHUB_REMOTE}/${MAIN_BRANCH} and ${GITHUB_REMOTE}/${WORK_BRANCH}..."
git fetch "$GITHUB_REMOTE" "$MAIN_BRANCH" "$WORK_BRANCH"

echo "Switching to ${MAIN_BRANCH}..."
git switch "$MAIN_BRANCH"
switched_branch=true

echo "Fast-forwarding ${MAIN_BRANCH} to ${GITHUB_REMOTE}/${MAIN_BRANCH} (if possible)..."
git merge --ff-only "${GITHUB_REMOTE}/${MAIN_BRANCH}" >/dev/null 2>&1 || true

main_behind="$(git rev-list --count "HEAD..${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}")"
if [[ "$main_behind" != "0" ]]; then
  merge_with_conflict_help \
    "$MAIN_BRANCH" \
    "${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}" \
    "$UPSTREAM_MERGE_MSG" \
    "For upstream sync, prefer the upstream version when conflicts are only about keeping main aligned with upstream."
else
  echo "${MAIN_BRANCH} is already up to date with ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}."
fi

echo "Pushing ${MAIN_BRANCH} to ${GITHUB_REMOTE}..."
git push "$GITHUB_REMOTE" "$MAIN_BRANCH"

echo "Switching to ${WORK_BRANCH}..."
git switch "$WORK_BRANCH"
switched_branch=true

echo "Syncing ${WORK_BRANCH} with ${GITHUB_REMOTE}/${WORK_BRANCH} (to avoid non-fast-forward push)..."
work_remote_behind="$(git rev-list --count "HEAD..${GITHUB_REMOTE}/${WORK_BRANCH}")"
if [[ "$work_remote_behind" != "0" ]]; then
  merge_with_conflict_help \
    "$WORK_BRANCH" \
    "${GITHUB_REMOTE}/${WORK_BRANCH}" \
    "$WORK_REMOTE_SYNC_MSG" \
    "For remote sync, keep zhiguofan as the base and integrate remote zhiguofan changes."
else
  echo "${WORK_BRANCH} is already up to date with ${GITHUB_REMOTE}/${WORK_BRANCH}."
fi

echo "Merging ${GITHUB_REMOTE}/${MAIN_BRANCH} into ${WORK_BRANCH} (origin/main contains upstream/main after the main sync step)..."
work_behind="$(git rev-list --count "HEAD..${GITHUB_REMOTE}/${MAIN_BRANCH}")"
if [[ "$work_behind" != "0" ]]; then
  merge_with_conflict_help \
    "$WORK_BRANCH" \
    "${GITHUB_REMOTE}/${MAIN_BRANCH}" \
    "$WORK_MERGE_MSG" \
    "For zhiguofan sync, keep main as the shared baseline and re-apply zhiguofan-only changes where needed."
else
  echo "${WORK_BRANCH} is already up to date with ${GITHUB_REMOTE}/${MAIN_BRANCH}."
fi

sync_work_branch_version

echo "Pushing ${WORK_BRANCH} to ${GITHUB_REMOTE}..."
git push "$GITHUB_REMOTE" "$WORK_BRANCH"

echo "Sync complete."
if [[ -f "backend/cmd/server/VERSION" ]]; then
  echo ">>> Final version in ${WORK_BRANCH}: $(cat backend/cmd/server/VERSION)"
fi
completed=true
