Sync our fork from upstream authzed/spicedb-operator tag $ARGUMENTS.

Follow these steps:

1. Ensure the authzed remote exists, or add it:
   `git remote get-url authzed 2>/dev/null || git remote add authzed https://github.com/authzed/spicedb-operator.git`

2. Fetch upstream tags: `git fetch authzed --tags`

3. If no tag was provided, determine the latest:
   `git tag -l 'v*' --sort=-v:refname | head -5`
   Compare the Go version in upstream go.mod against ours to ensure compatibility.

4. Read `README-redhat.md` for the drift tracking table and merge actions.
   Read `SYNC.md` for the current upstream tag.

5. Create a branch from the tag: `git checkout -b sync-upstream-<tag> tags/<tag>`

6. Create a merge branch from main: `git checkout -b merge-upstream-<tag> main`

7. Merge: `git merge sync-upstream-<tag>`

8. Resolve any merge conflicts using the **Merge Action** column in the drift
   tracking table in `README-redhat.md`:
   - **Keep ours**: preserve the Red Hat version
   - **Re-apply**: accept upstream, then re-apply our modifications
     (change runners to `ubuntu-latest`, add `if: false`, use `Dockerfile.openshift`)
   - **Delete**: remove the file with `git rm`
   - **Red Hat only**: keep as-is, no upstream equivalent
   - Files not in the table: accept the upstream version

9. For go.mod/go.sum conflicts, do NOT resolve line-by-line. Reset to upstream:
   ```
   git diff --name-only --diff-filter=U --relative | grep -E "mod|sum|work"
   git checkout sync-upstream-<tag> -- <each conflicting file>
   ```

10. After all conflicts resolved: `git add -A && git merge --continue`

11. Update `SYNC.md` with the new tag and commit SHA (`git rev-parse tags/<tag>`),
    then commit: `git commit -m "Update SYNC.md for <tag>"`

12. Run `./scripts/redhat-diff.sh --stat` to check for stale or diverged files

13. Clean up any stale files (`git rm`) and diverged files
    (`git checkout tags/<tag> -- <file>`), commit, and re-run the script

14. Push the branch: `git push -u origin merge-upstream-<tag>`

15. Create a PR to main (do not squash commits):
    `gh pr create --repo project-kessel/spicedb-operator --title "Sync upstream authzed/spicedb-operator <tag>" --base main`

16. Post the Red Hat diff on the PR:
    `gh repo set-default project-kessel/spicedb-operator`
    `./scripts/redhat-diff.sh --pr <PR_NUMBER>`

After completing, remind me about:
- Reviewing deploy/deploy.yml against upstream bundle.yaml
- Running validate-upgrade-path to check SpiceDB version
- Coordinating deployment changes per the SpiceDB Upgrade Process
