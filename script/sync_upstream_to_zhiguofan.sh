#!/usr/bin/env bash
set -euo pipefail

UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-upstream}"
UPSTREAM_BRANCH="${UPSTREAM_BRANCH:-main}"
GITHUB_REMOTE="${GITHUB_REMOTE:-origin}"
MAIN_BRANCH="${MAIN_BRANCH:-main}"
WORK_BRANCH="${WORK_BRANCH:-zhiguofan}"
UPSTREAM_MERGE_MSG="${UPSTREAM_MERGE_MSG:-chore: sync from upstream Wei-Shaw/sub2api:main}"
WORK_MERGE_MSG="${WORK_MERGE_MSG:-chore: sync from origin/main}"

# Whether to attempt AI-assisted conflict resolution (default: yes)
AI_RESOLVE="${AI_RESOLVE:-true}"

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
    # Use --dangerously-skip-permissions so codebuddy doesn't pause for approvals
    if ! ai_output=$(echo "$prompt" | codebuddy --print --dangerously-skip-permissions 2>/dev/null); then
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
      if echo "$ai_output" | grep -qE '^(<<<<<<<|=======|>>>>>>>)'; then
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

echo "Switching to ${MAIN_BRANCH}..."
git switch "$MAIN_BRANCH"
switched_branch=true

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

echo "Syncing ${MAIN_BRANCH} with ${GITHUB_REMOTE} before push..."
git pull "$GITHUB_REMOTE" "$MAIN_BRANCH" --rebase

echo "Pushing ${MAIN_BRANCH} to ${GITHUB_REMOTE}..."
git push "$GITHUB_REMOTE" "$MAIN_BRANCH"

echo "Fetching ${GITHUB_REMOTE}/${MAIN_BRANCH}..."
git fetch "$GITHUB_REMOTE" "$MAIN_BRANCH"

echo "Switching to ${WORK_BRANCH}..."
git switch "$WORK_BRANCH"
switched_branch=true

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

echo "Pushing ${WORK_BRANCH} to ${GITHUB_REMOTE}..."
git push "$GITHUB_REMOTE" "$WORK_BRANCH"

echo "Sync complete."
if [[ -f "backend/cmd/server/VERSION" ]]; then
  echo ">>> Final version in ${WORK_BRANCH}: $(cat backend/cmd/server/VERSION)"
fi
completed=true
