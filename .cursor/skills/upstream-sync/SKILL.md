---
name: upstream-sync
description: Sync the spicedb-operator fork from upstream authzed/spicedb-operator using a merge-based workflow. Resolves merge conflicts via the drift tracking table in README-redhat.md, updates SYNC.md, validates with redhat-diff.sh, and creates a PR. Use when the user mentions "sync upstream", "upstream sync", "sync from authzed", "merge upstream", or "update fork".
---

# Upstream Sync for SpiceDB Operator

Syncs this fork (project-kessel/spicedb-operator) from upstream authzed/spicedb-operator.

## Prerequisites

Before starting, read these files from the repo root:
- `README-redhat.md` — drift tracking table with merge actions per file
- `SYNC.md` — current upstream tag and commit SHA
- `CLAUDE.md` — conflict resolution rules summary

## Workflow

### Step 1: Determine Target Tag

If the user provides a tag, use it. Otherwise, determine the latest:

```bash
# ensure authzed remote exists
git remote get-url authzed 2>/dev/null || git remote add authzed https://github.com/authzed/spicedb-operator.git
git fetch authzed --tags
git tag -l 'v*' --sort=-v:refname | head -5
```

**Go version gate**: The upstream tag's `go.mod` must not require a Go version newer than what go-toolset supports. Compare:
```bash
git show tags/<TAG>:go.mod | grep -E '^(go |toolchain )'
```
against the current `go.mod`. If upstream requires a newer Go, pick an older tag.

### Step 2: Create Branches

```bash
git checkout -b sync-upstream-<TAG> tags/<TAG>
git checkout -b merge-upstream-<TAG> main
```

### Step 3: Merge

```bash
git merge sync-upstream-<TAG> --no-edit
```

### Step 4: Resolve Conflicts

Use the **Merge Action** column from the drift tracking table in `README-redhat.md`:

| Action | How to resolve |
|--------|---------------|
| **Keep ours** | `git checkout HEAD -- <file>` |
| **Delete** | `git rm <file>` |
| **Red Hat only** | No conflict expected (file only exists in our fork) |
| **Re-apply** | Accept upstream, then re-apply our modifications (see below) |

**Files not in the table**: accept the upstream version.

#### Re-apply modifications

For workflow files marked **Re-apply**, verify after auto-merge:
- All `runs-on:` values are `ubuntu-latest` (not `depot-*`, `buildjet-*`)
- `build-test.yaml`: image-build uses `Dockerfile.openshift`, E2E has `if: false`
- `lint.yaml`: extra-lint has `if: false`

#### go.mod / go.sum conflicts

Never resolve these line-by-line. Reset to upstream entirely:
```bash
git diff --name-only --diff-filter=U --relative | grep -E "mod|sum|work"
git checkout sync-upstream-<TAG> -- <each conflicting file>
```

After all conflicts are resolved:
```bash
git add -A
git merge --continue
```

### Step 5: Update SYNC.md

```bash
# get the commit SHA for the tag
git rev-parse tags/<TAG>
```

Update `SYNC.md` with the new TAG and COMMIT_SHA, then:
```bash
git add SYNC.md
git commit -m "Update SYNC.md for <TAG>"
```

### Step 6: Validate with redhat-diff.sh

```bash
./scripts/redhat-diff.sh --stat
```

The script reports three categories:
- **Red Hat changes** — expected; these are what reviewers focus on
- **Stale files** (warning) — upstream deleted/renamed but survived merge; fix with `git rm`
- **Diverged files** (warning) — merge artifacts; fix with `git checkout tags/<TAG> -- <file>`

If stale or diverged files exist (and they are NOT files Red Hat intentionally modified per the drift table), clean them up:
```bash
git rm <stale files>
git checkout tags/<TAG> -- <diverged files>
git add -A
git commit -m "Clean up merge artifacts from <TAG> sync"
./scripts/redhat-diff.sh --stat
```

Repeat until the output is clean.

### Step 7: Push and Create PR

```bash
git push -u origin merge-upstream-<TAG>
```

Create PR targeting `main` on `project-kessel/spicedb-operator`. Include:
- Summary of upstream changes (check upstream release notes or commit log)
- Conflict resolution actions taken
- `redhat-diff.sh` validation results
- Reminder: **do not squash commits** when merging

```bash
gh repo set-default project-kessel/spicedb-operator
gh pr create --title "Sync upstream authzed/spicedb-operator <TAG>" \
  --base main --body "..."
```

### Step 8: Post Red Hat Diff on PR

```bash
./scripts/redhat-diff.sh --pr <PR_NUMBER>
```

## Post-Sync Reminders

After the sync PR is created, remind the user about manual follow-ups:
1. **Review deploy/deploy.yml** against the upstream bundle.yaml from the release assets
2. **Check for new workflows** that may need `if: false` or runner changes
3. **Validate CI** passes on the PR
4. **SpiceDB version**: run `make build-validate-upgrade-path && ./bin/validate-upgrade-path --list-versions` to determine if a SpiceDB repo sync is also needed
5. **Deployment changes** must be coordinated per the SpiceDB Upgrade Process before merging
