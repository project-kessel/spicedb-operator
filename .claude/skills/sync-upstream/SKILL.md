---
name: sync-upstream
description: Sync the SpiceDB Operator fork from the latest upstream release. Auto-detects the latest tag, merges, resolves conflicts, runs post-merge cleanup, and builds/installs the validate-upgrade-path tool.
disable-model-invocation: true
argument-hint: [--tag <tag>]
---

# Sync SpiceDB Operator from Upstream

Syncs this fork from the latest upstream release of
[authzed/spicedb-operator](https://github.com/authzed/spicedb-operator).

## Arguments

- `--tag <tag>` — (optional) override the upstream tag to sync to. If omitted,
  the skill auto-detects the latest release tag from upstream.

## Required Remotes

- `origin` — your personal fork (used for pushing branches)
- `kessel` — `project-kessel/spicedb-operator` (the repo PRs target)
- `upstream` — `authzed/spicedb-operator` (where upstream tags are fetched from)

## Process

### Step 1: Validate Prerequisites

Before starting the sync, validate that all required tools and configuration are
in place. Run all checks first, then report any issues together. For any issues
found, offer to fix them (with user confirmation). Do not ask the user to perform
manual steps unless they explicitly say they prefer to handle it themselves.

1. **Validate git remotes** by running `git remote -v` and checking:
   - `upstream` exists and points to `github.com/authzed/spicedb-operator` (HTTP or SSH)
   - `kessel` exists and points to `github.com/project-kessel/spicedb-operator` (HTTP or SSH)
   - `origin` exists (any URL is acceptable)

   If a remote is missing, offer to add it:
   ```bash
   git remote add upstream https://github.com/authzed/spicedb-operator.git
   git remote add kessel https://github.com/project-kessel/spicedb-operator.git
   ```
   If a remote exists but points to the wrong URL, offer to fix it:
   ```bash
   git remote set-url <name> <correct-url>
   ```

2. **Validate `gh` CLI is installed:**
   ```bash
   command -v gh
   ```
   If not installed, inform the user that `gh` is required for creating PRs and
   posting review diffs. Link to https://cli.github.com/ and ask how they want
   to proceed.

3. **Validate `gh` CLI is authenticated:**
   ```bash
   gh auth status
   ```
   If not authenticated, offer to run `gh auth login` for the user.

4. **Validate `gh` default repo is configured:**
   ```bash
   gh repo set-default --view
   ```
   The default repo should be `project-kessel/spicedb-operator`. If it is not
   set or points to a different repo, offer to fix it:
   ```bash
   gh repo set-default project-kessel/spicedb-operator
   ```

Only proceed to Step 2 once all prerequisites pass.

### Step 2: Determine the Target Tag

If `--tag` was provided, use that. Otherwise, find the latest release tag from
the upstream repository:

```bash
git fetch upstream --tags
git tag -l 'v*' --sort=-v:refname | head -1
```

Compare this to the current tag in `SYNC.md`. If they match, report that the
fork is already up to date and stop.

**Important**: Before proceeding, check the `go` directive in the upstream
`go.mod` at the target tag:
```bash
git show tags/<tag>:go.mod | grep -E '^go '
```
If the Go version exceeds our go-toolset version, stop and report the issue.
The current go-toolset version constraint is documented in `README-redhat.md`.

### Step 3: Sync

1. `git fetch upstream --tags` (if not already done)
2. `git fetch kessel`
3. `git checkout -b sync-upstream-<tag> tags/<tag>`
4. `git checkout -b merge-upstream-<tag> kessel/main`
5. `git merge sync-upstream-<tag>`
6. Resolve any merge conflicts using the **Merge Action** column in the drift
   tracking table in `README-redhat.md`
7. For `go.mod` and `go.sum` conflicts, reset to upstream entirely:
   ```bash
   # Find all conflicting go.mod/go.sum/go.work files
   git diff --name-only --diff-filter=U --relative | grep -E "mod|sum|work"

   # Reset them to the upstream version
   git checkout sync-upstream-<tag> -- <file1> <file2> ...
   ```
8. For any file not listed in the table, accept the upstream (incoming) version

### Step 4: Update SYNC.md

Update `SYNC.md` with the new tag and the commit SHA of the upstream tag:
```bash
git rev-parse tags/<tag>
```
Commit the update.

### Step 5: Post-Merge Cleanup

1. Run `./scripts/redhat-diff.sh --stat`
2. Remove any **stale files** listed in the warning:
   ```bash
   git rm <file1> <file2> ...
   ```
3. Reset any **diverged files** listed in the warning:
   ```bash
   git checkout tags/<tag> -- <file1> <file2> ...
   ```
4. Commit the cleanup
5. Re-run `./scripts/redhat-diff.sh --stat` to confirm clean output

### Step 6: Build and Install validate-upgrade-path

Build and install the tool so it is available system-wide:
```bash
make install-validate-upgrade-path
```
Verify it is in PATH:
```bash
which validate-upgrade-path
```

### Step 7: Push and Create PR

1. Push the branch to your fork:
   ```bash
   git push origin merge-upstream-<tag>
   ```
2. Create a PR against the kessel repo (**do not squash commits when merging**):
   ```bash
   gh pr create --repo project-kessel/spicedb-operator --base main --title "..." --body "..."
   ```
3. Post the Red Hat diff on the PR:
   ```bash
   ./scripts/redhat-diff.sh --pr <pr-number>
   ```

### Step 8: Summary

Report:
- Old tag → new tag
- PR link
- Number of Red Hat changes, stale files cleaned, diverged files reset
- Whether validate-upgrade-path was successfully installed
- Remind the user to review the PR and check CI before merging

**Important: Before merging this PR, you must also:**
1. Update the `deploy/deploy.yml` file to align with the new upstream bundle.
   See [Updating the SpiceDB Operator Deployment File](https://inscope.corp.redhat.com/docs/default/component/kessel-internal-docs/running-kessel/spicedb/syncing-spicedb-repos/#updating-the-spicedb-operator-deployment-file).
2. Follow all post-sync steps at
   https://inscope.corp.redhat.com/docs/default/component/kessel-internal-docs/running-kessel/spicedb/syncing-spicedb-repos/#after-syncing
