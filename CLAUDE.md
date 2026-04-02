# CLAUDE.md — SpiceDB Operator Fork (Red Hat)

This is Red Hat's fork of [authzed/spicedb-operator](https://github.com/authzed/spicedb-operator).
See `README-redhat.md` for a full list of Red Hat-specific changes and their rationale.

## Upstream Sync Process

This fork is periodically synced from upstream using a merge-based workflow.
See `SYNC.md` for the currently synced upstream tag.

To run a full sync, use the `/sync-upstream <TAG>` command (Claude Code) or
trigger the `upstream-sync` skill (Cursor). Both reference the same process
documented below and in `.cursor/skills/upstream-sync/SKILL.md`.

### Merge Conflict Resolution Rules

When merging upstream changes, resolve conflicts using the **Merge Action** column
in the drift tracking table in `README-redhat.md`. The four actions are:

- **Keep ours**: always preserve the Red Hat version of this file
- **Re-apply**: accept upstream changes, then re-apply our specific modifications
  (e.g., change runners to `ubuntu-latest`, add `if: false`, use `Dockerfile.openshift`)
- **Delete**: file should not exist in our fork; remove if upstream re-adds it
- **Red Hat only**: file exists only in our fork; no upstream equivalent — keep as-is

**For all other files not listed in the table**: accept the upstream (incoming) version.
This includes all Go source code, `go.mod`, `go.sum`, protobuf definitions, and any
new files introduced by upstream.

**For `go.mod` and `go.sum` conflicts**: do not resolve these line-by-line. Instead,
reset all `go.mod` and `go.sum` files to the upstream version entirely:
```
git checkout sync-upstream-<tag> -- <all go.mod and go.sum files with conflicts>
```
This ensures Go dependencies stay aligned with upstream, since git auto-merges
non-conflicting lines and may preserve newer dependency versions from our fork
that we no longer maintain independently.

### Workflow Runner Mappings

When upstream introduces or modifies workflows we keep, replace their runner with ours:
- `depot-*`, `buildjet-*`, or any custom runner → `ubuntu-latest`

### Post-Merge Cleanup

After the merge completes, run `./scripts/redhat-diff.sh --stat` to check for:
- **Stale files**: upstream deleted/renamed files that survived the merge — remove them
- **Diverged files**: upstream files where the merge produced different content — reset
  them with `git checkout tags/<tag> -- <file>`
- **Red Hat changes**: the actual changes to review

### Deployment File (deploy/deploy.yml)

This file is NOT synced from upstream via merge. It is manually maintained based on
the upstream `bundle.yaml` from each release. See the deployment changes table in
`README-redhat.md` for the specific modifications made for OpenShift compatibility.
