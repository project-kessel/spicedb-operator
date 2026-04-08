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

## Process

### Step 1: Determine the Target Tag

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
git show tags/<tag>:go.mod | head -5
```
If the Go version exceeds our go-toolset version, stop and report the issue.
The current go-toolset version constraint is documented in `README-redhat.md`.

### Step 2: Sync

1. `git checkout -b sync-upstream-<tag> tags/<tag>`
2. `git checkout -b merge-upstream-<tag> main`
3. `git merge sync-upstream-<tag>`
4. Resolve any merge conflicts using the **Merge Action** column in the drift
   tracking table in `README-redhat.md`
5. For `go.mod` and `go.sum` conflicts, reset to upstream entirely:
   ```bash
   # Find all conflicting go.mod/go.sum/go.work files
   git diff --name-only --diff-filter=U --relative | grep -E "mod|sum|work"

   # Reset them to the upstream version
   git checkout sync-upstream-<tag> -- <file1> <file2> ...
   ```
6. For any file not listed in the table, accept the upstream (incoming) version

### Step 3: Update SYNC.md

Update `SYNC.md` with the new tag and the commit SHA of the upstream tag:
```bash
git rev-parse tags/<tag>
```
Commit the update.

### Step 4: Post-Merge Cleanup

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

### Step 5: Build and Install validate-upgrade-path

Build and install the tool so it is available system-wide:
```bash
make install-validate-upgrade-path
```
Verify it is in PATH:
```bash
which validate-upgrade-path
```

### Step 6: Push and Create PR

1. Push the branch:
   ```bash
   git push origin merge-upstream-<tag>
   ```
2. Create a PR to main (**do not squash commits**)
3. Post the Red Hat diff on the PR:
   ```bash
   ./scripts/redhat-diff.sh --pr <pr-number>
   ```

### Step 7: Summary

Report:
- Old tag → new tag
- PR link
- Number of Red Hat changes, stale files cleaned, diverged files reset
- Whether validate-upgrade-path was successfully installed
- Remind the user to review the PR and check CI before merging
