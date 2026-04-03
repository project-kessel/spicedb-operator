#!/usr/bin/env bash
#
# redhat-diff.sh - Show only Red Hat-specific changes introduced or modified during
#                  an upstream sync, filtering out pure upstream changes and unchanged
#                  Red Hat modifications.
#
# How it works:
#   For each file that has Red Hat modifications on the merge branch, the script
#   compares the "Red Hat delta" (diff from upstream tag) before and after the sync.
#   Only files where this delta changed are shown — meaning the Red Hat-specific
#   modifications were added, updated, or removed during this sync.
#
# Usage:
#   ./scripts/redhat-diff.sh                    # print diff to stdout
#   ./scripts/redhat-diff.sh --stat             # print diffstat summary only
#   ./scripts/redhat-diff.sh --pr <number>      # post diff as a comment on a GitHub PR
#   ./scripts/redhat-diff.sh --tag <tag>        # override the new tag from SYNC.md
#   ./scripts/redhat-diff.sh --branch <branch>  # compare against a branch other than HEAD
#   ./scripts/redhat-diff.sh --base <branch>    # base branch to compare from (default: main)
#   ./scripts/redhat-diff.sh --all              # show cumulative Red Hat delta from upstream

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
    echo "Error: not inside a git repository" >&2
    exit 1
}
SYNC_FILE="$REPO_ROOT/SYNC.md"

# Defaults
TAG=""
BRANCH="HEAD"
BASE="main"
PR_NUMBER=""
STAT_ONLY=false
SHOW_ALL=false

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Show Red Hat-specific changes made during an upstream sync.

By default, compares the Red Hat delta (diff from upstream) before and after
the sync. Only files where the Red Hat modifications changed are shown.
Use --all to show the full cumulative Red Hat delta from upstream.

Options:
  --tag <tag>        Override the upstream tag (default: read from SYNC.md on branch)
  --branch <branch>  Branch to compare (default: HEAD)
  --base <branch>    Base branch to compare from (default: main)
  --stat             Show diffstat summary only
  --all              Show all Red Hat changes vs upstream (cumulative)
  --pr <number>      Post the diff as a comment on the given GitHub PR
  -h, --help         Show this help message
EOF
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --tag)    TAG="$2"; shift 2 ;;
        --branch) BRANCH="$2"; shift 2 ;;
        --base)   BASE="$2"; shift 2 ;;
        --stat)   STAT_ONLY=true; shift ;;
        --all)    SHOW_ALL=true; shift ;;
        --pr)
            if ! command -v gh >/dev/null 2>&1; then
                echo "Error: the --pr flag requires the GitHub CLI (gh) to be installed." >&2
                echo "Install it from https://cli.github.com/" >&2
                exit 1
            fi
            PR_NUMBER="$2"; shift 2 ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1" >&2; usage ;;
    esac
done

# Read new tag from SYNC.md on the branch being compared
read_tag_from_ref() {
    local ref="$1"
    git -C "$REPO_ROOT" show "$ref:SYNC.md" 2>/dev/null \
        | sed -n 's/^TAG:[[:space:]]*//p' \
        | tr -d '[:space:]'
}

if [[ -z "$TAG" ]]; then
    TAG=$(read_tag_from_ref "$BRANCH")
    if [[ -z "$TAG" ]]; then
        echo "Error: could not parse TAG from SYNC.md on $BRANCH" >&2
        exit 1
    fi
fi

# Verify the new tag exists locally
if ! git -C "$REPO_ROOT" rev-parse "tags/$TAG" >/dev/null 2>&1; then
    echo "Error: tag '$TAG' not found. Run 'git fetch upstream --tags' first." >&2
    exit 1
fi

# --all mode: show cumulative Red Hat delta
if [[ "$SHOW_ALL" == true ]]; then
    echo "Comparing $BRANCH against upstream tag: $TAG (all Red Hat changes)" >&2
    if [[ "$STAT_ONLY" == true ]]; then
        git -C "$REPO_ROOT" diff --stat "tags/$TAG..$BRANCH"
    else
        git -C "$REPO_ROOT" diff "tags/$TAG..$BRANCH"
    fi
    exit 0
fi

# Read old tag from SYNC.md on the base branch
OLD_TAG=$(read_tag_from_ref "$BASE")
if [[ -z "$OLD_TAG" ]]; then
    echo "Error: could not parse TAG from SYNC.md on $BASE" >&2
    exit 1
fi

if ! git -C "$REPO_ROOT" rev-parse "tags/$OLD_TAG" >/dev/null 2>&1; then
    echo "Error: old tag '$OLD_TAG' not found. Run 'git fetch upstream --tags' first." >&2
    exit 1
fi

echo "Comparing Red Hat delta: $BASE ($OLD_TAG) -> $BRANCH ($TAG)" >&2

# Get all files that have Red Hat modifications on either side
ALL_RH_FILES=$(
    {
        git -C "$REPO_ROOT" diff --name-only "tags/$TAG..$BRANCH"
        git -C "$REPO_ROOT" diff --name-only "tags/$OLD_TAG..$BASE"
    } | sort -u
)

if [[ -z "$ALL_RH_FILES" ]]; then
    echo "No Red Hat-specific changes found." >&2
    exit 0
fi

# Extract only the added/removed lines from a diff, stripping headers,
# hunk markers, and context lines so that comparisons are not thrown off
# by shifted line numbers or changed surrounding context.
diff_essence() {
    { grep -E '^\+[^+]|^-[^-]' || true; } | sort
}

# Check if a file exists at a given git ref
file_exists_at_ref() {
    git -C "$REPO_ROOT" cat-file -t "$1:$2" >/dev/null 2>&1
}

# Compare the Red Hat delta for each file before and after the sync.
# Only include files where the substantive delta changed.
# Also detect merge artifacts: files that upstream deleted/renamed but
# survived the merge.
CHANGED_FILES=()
STALE_FILES=()
DIVERGED_FILES=()
while IFS= read -r file; do
    # Detect stale files: file exists on merge branch but was removed/renamed
    # upstream (not in new tag) and was an upstream file (existed in old tag).
    # These are not Red Hat changes — they should have been removed by the merge.
    if ! file_exists_at_ref "tags/$TAG" "$file" && file_exists_at_ref "tags/$OLD_TAG" "$file"; then
        STALE_FILES+=("$file")
        continue
    fi

    old_delta=$(git -C "$REPO_ROOT" diff "tags/$OLD_TAG..$BASE" -- "$file" | diff_essence)
    new_delta=$(git -C "$REPO_ROOT" diff "tags/$TAG..$BRANCH" -- "$file" | diff_essence)

    if [[ "$old_delta" != "$new_delta" ]]; then
        # If the file had no Red Hat delta before but now differs from upstream,
        # and the file exists on the upstream tag, it's a merge artifact — the
        # merge produced content that doesn't match upstream exactly.
        if [[ -z "$old_delta" ]] && [[ -n "$new_delta" ]] && file_exists_at_ref "tags/$TAG" "$file"; then
            DIVERGED_FILES+=("$file")
            continue
        fi
        CHANGED_FILES+=("$file")
    fi
done <<< "$ALL_RH_FILES"

# Warn about stale files (upstream deleted/renamed but still on merge branch)
if [[ ${#STALE_FILES[@]} -gt 0 ]]; then
    echo "" >&2
    echo "WARNING: The following files were deleted or renamed upstream but still" >&2
    echo "exist on the merge branch. These may need to be removed:" >&2
    for f in "${STALE_FILES[@]}"; do
        echo "  - $f" >&2
    done
    echo "" >&2
fi

# Warn about diverged files (merge produced content that differs from upstream)
if [[ ${#DIVERGED_FILES[@]} -gt 0 ]]; then
    echo "WARNING: The following files diverged from upstream during the merge." >&2
    echo "These are not Red Hat changes but the merge produced content that" >&2
    echo "doesn't match upstream. Consider resetting them to the upstream version:" >&2
    for f in "${DIVERGED_FILES[@]}"; do
        echo "  - $f" >&2
    done
    echo "  To fix: git checkout tags/$TAG -- <file>" >&2
    echo "" >&2
fi

if [[ ${#CHANGED_FILES[@]} -eq 0 ]]; then
    echo "No Red Hat-specific changes for this sync." >&2
    exit 0
fi

# Generate the output: show the NEW Red Hat delta for changed files
if [[ "$STAT_ONLY" == true ]]; then
    git -C "$REPO_ROOT" diff --stat "tags/$TAG..$BRANCH" -- "${CHANGED_FILES[@]}"
    exit 0
fi

DIFF_OUTPUT=$(git -C "$REPO_ROOT" diff "tags/$TAG..$BRANCH" -- "${CHANGED_FILES[@]}")

if [[ -z "$DIFF_OUTPUT" ]]; then
    echo "No Red Hat-specific changes for this sync." >&2
    exit 0
fi

# Post to PR or print to stdout
if [[ -n "$PR_NUMBER" ]]; then
    STAT_OUTPUT=$(git -C "$REPO_ROOT" diff --stat "tags/$TAG..$BRANCH" -- "${CHANGED_FILES[@]}")

    WARNINGS_SECTION=""
    if [[ ${#STALE_FILES[@]} -gt 0 ]]; then
        STALE_LIST=""
        for f in "${STALE_FILES[@]}"; do
            STALE_LIST+=$'\n'"- \`$f\`"
        done
        WARNINGS_SECTION+=$(cat <<STALE

### :warning: Stale files (upstream deleted/renamed)

The following files were deleted or renamed upstream between \`$OLD_TAG\` and \`$TAG\`
but still exist on this branch. They may need to be removed:
$STALE_LIST

STALE
        )
    fi

    if [[ ${#DIVERGED_FILES[@]} -gt 0 ]]; then
        DIVERGED_LIST=""
        for f in "${DIVERGED_FILES[@]}"; do
            DIVERGED_LIST+=$'\n'"- \`$f\`"
        done
        WARNINGS_SECTION+=$(cat <<DIVERGED

### :warning: Diverged files (merge artifacts)

The following files have no intentional Red Hat modifications but diverged from
upstream during the merge. Consider resetting them: \`git checkout tags/$TAG -- <file>\`
$DIVERGED_LIST

DIVERGED
        )
    fi

    COMMENT_BODY=$(cat <<EOF
## Red Hat-specific changes (vs upstream \`$TAG\`)

These are the Red Hat-specific changes that were added or modified in this sync.
Reviewers: focus your review on these changes -- everything else is from upstream
or unchanged Red Hat modifications.

Previously synced to: \`$OLD_TAG\`
$WARNINGS_SECTION
### Summary
\`\`\`
$STAT_OUTPUT
\`\`\`

<details>
<summary>Full diff (click to expand)</summary>

\`\`\`diff
$DIFF_OUTPUT
\`\`\`

</details>
EOF
    )

    echo "Posting Red Hat diff to PR #$PR_NUMBER..." >&2
    gh pr comment "$PR_NUMBER" --body "$COMMENT_BODY" --repo "$(gh repo view --json nameWithOwner -q .nameWithOwner)"
    echo "Done. Comment posted to PR #$PR_NUMBER." >&2
else
    echo "$DIFF_OUTPUT"
fi
