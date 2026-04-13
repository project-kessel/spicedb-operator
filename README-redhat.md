# Information about Red Hat's Fork of SpiceDB Operator

This document captures all changes made by Red Hat that diverge from the upstream fork. It outlines why these changes were made for historical reference and to help ensure they are kept when syncing with upstream.

&nbsp;

### Drift Tracking

The table below captures all changes to our fork from upstream. Each entry includes the affected files, what changed, why, and how to handle conflicts during upstream syncs.

**Merge actions:**
- **Keep ours**: always preserve the Red Hat version of this file
- **Re-apply**: accept upstream changes, then re-apply our specific modifications
- **Delete**: file should not exist in our fork; remove if upstream re-adds it
- **Red Hat only**: file exists only in our fork; no upstream equivalent

| File(s) | Change | Reason | Merge Action |
|---------|--------|--------|-------------|
| `.github/dependabot.yml` | Removed | Aligns with Red Hat mandates to leverage Konflux | Delete |
| `.github/renovate.json` | Replaced with our own config | Configures Mintmaker (part of Konflux) to prevent Go pkg update PRs and move to weekly updates for Dockerfile base image updates | Keep ours |
| Active workflows in `.github/workflows/` | Runner changed to `ubuntu-latest` | Authzed uses custom self-hosted runners (`depot-*`, `buildjet-*`) which we don't have access to | Re-apply |
| `.github/workflows/build-test.yaml` | Build Container Image job uses `Dockerfile.openshift` instead of `Dockerfile`; disabled E2E tests (`if: false`) | Ensures image build test uses our custom Dockerfile; E2E tests not critical for our builds | Re-apply |
| `.github/workflows/lint.yaml` | Disabled YAML & Markdown linting (`if: false`) | Not critical to Red Hat builds | Re-apply |
| `.github/workflows/cla.yaml` | Removed | Not applicable to our fork | Delete |
| `.github/workflows/release.yaml` | Removed | Not applicable to our fork | Delete |
| `.github/workflows/security-scanning.yml` | Added | Required ConsoleDot platform security workflow for CVE scanning | Red Hat only |
| `.tekton/spicedb-operator-pull-request.yaml`, `.tekton/spicedb-operator-push.yaml` | Added | Konflux PR and merge build pipelines | Red Hat only |
| `Dockerfile.openshift` | Added | FIPS-compliant builds using UBI base image and Go Toolset for Konflux | Red Hat only |
| `build_deploy.sh` | Added | Legacy App Interface build script, still used for local image builds when testing operator updates | Red Hat only |
| `deploy/deploy.yml` | Added | Main deployment file for SpiceDB Operator on OpenShift clusters with CRDs, RBAC, and customizations for OpenShift (see deployment table below) | Red Hat only |
| `config/operator_openshift.yaml` | Added | OpenShift-specific operator configuration | Red Hat only |
| `scripts/redhat-diff.sh` | Added | Script to isolate Red Hat-specific changes from upstream sync PRs for easier code review | Red Hat only |
| `Makefile` | Added | Build tooling for Red Hat-specific tools (e.g., `validate-upgrade-path`) | Red Hat only |
| `tools/validate-upgrade-path/` | Added | CLI tool to validate SpiceDB upgrade paths against the operator's update graph. Useful for verifying upgrades when managing SpiceDB images outside the operator's built-in update mechanism | Red Hat only |
| `.gitignore` | Updated to ignore `bin/` directory | Prevents built binaries from being committed | Re-apply |
| `CLAUDE.md` | Replaced with our own | Contains Red Hat-specific merge conflict resolution rules for upstream syncs | Keep ours |
| `.claude/skills/sync-upstream/SKILL.md` | Added | Claude skill to handle the upstream syncing process | Red Hat only |
| `README-redhat.md` | Added | Documents all Red Hat fork changes and rationale | Red Hat only |
| `SYNC.md` | Added | Tracks the current upstream version synced to this fork | Red Hat only |
| `.yamllint` | Added | YAML linting configuration | Red Hat only |


&nbsp;

### More on our Dockerfile and Builds

**UBI Base Images**

To comply with [Red Hat Software Certification policies](https://docs.redhat.com/en/documentation/red_hat_software_certification/2025/html-single/red_hat_openshift_software_certification_policy_guide/index#con-image-content-requirements_openshift-sw-cert-policy-container-images), UBI images are used as the base image for both building and running the container image. This ensures images are built to Red Hat security standards, comply with our build processes, and ensures that application runtime dependencies, such as operating system components and libraries, are covered under the customer’s subscription (where applicable). Our current UBI base image specifically targets RHEL 9.x to ensure it contains [FIPS-validated cryptographic modules](https://access.redhat.com/compliance/fips) for running in FedRAMP environments.


**Go Toolset**

To ensure FIPS compliant binaries for running in FedRAMP environments, we leverage Go Toolset vs upstream Go for all builds. While upstream Go is currently [working on FIPS 140-3 certification](https://go.dev/blog/fips140), these modules are still in process and under review and therefore are not validated nor shipped with any RHEL products including UBI based images.

Go Toolset leverages a fork of Go which is based on the upstream work to enable Go to link against the C library Boring Crypto. This fork uses OpenSSL instead of BoringSSL which is already FIPS validated on RHEL systems (hence the reliance on specific versions of UBI such as 9.x). When upstream Go has finished FIPS validation, it is expected that Go Toolset will converge to using upstream Go and will remain our install target long term on UBI images.

**Go and Base Image Updates**

With our reliance on Go Toolset and FIPS-validated OpenSSL modules, our ability to sync upstream code changes can be limited due to Go versions used upstream. The version of Go listed in our `go.mod` file must not be greater than the [latest version](https://catalog.redhat.com/en/software/containers/ubi9/go-toolset/61e5c00b4ec9945c18787690) of Go Toolset available for the specific UBI version in use. This is required to remain FIPS compliant and have FIPS-validated OpenSSL modules. Base images must also be updated with care to ensure only validated versions of RHEL are used.

For more info on Go Toolset and FIPS certifications at Red Hat:
* [Golang-FIPS](https://github.com/golang-fips/go/blob/main/README.md)
* [FIPS Mode for Red Hat Go Toolset](https://developers.redhat.com/articles/2025/01/23/fips-mode-red-hat-go-toolset)
* [Red Hat FIPS Compliance](https://access.redhat.com/compliance/fips)

**Deployment File**

Each release of the SpiceDB Operator includes a `bundle.yaml` that contains all the necessary resources and CRD’s to deploy the operator. The deployment file under `deploy/deploy.yml` incorporates the contents of the `bundle.yaml` with customizations that remove unwanted configurations and allow us to deploy into OpenShift clusters.

The below table captures changes in `deploy/deploy.yml` that diverge from the upstream bundle release. Each release of SpiceDB Operator will require a review of the bundle to pull in any changes required, which may require updating the below table.

| Change | Reason |
|--------|--------|
| Removed the `update-graph` ConfigMap | The `update-graph` is useful for defining the SpiceDB version to use for a cluster, and controlling automatic upgrades. Since we build and use our own SpiceDB image, this feature is disabled in our `SpiceDbClusters` CR, so the ConfigMap is not needed |
| `spec.containers.image` updated to use Red Hat built SpiceDB | Ensures the SpiceDB image running in clusters complies with Red Hat policies and security standards |
| CPU and Memory adjustments | Requests/limits have been increased for our deployment needs |
| `runAsUser` and `runAsGroup` removed from container and pod `securityContexts` | These violate OpenShift security policies and prevent the image from running due to using the `nobody` user |
| The `config` volume/volume mount removed | This volume facilitates the `update-graph` ConfigMap which was also removed |


&nbsp;

### Keeping SYNC.md Up to Date

The [SYNC.md](SYNC.md) file tracks the current upstream [authzed/spicedb-operator](https://github.com/authzed/spicedb-operator/) version that has been merged into this fork. This file must be updated whenever a new upstream sync is performed.

**When to Update**

Update `SYNC.md` as part of any PR that merges changes from the upstream [authzed/spicedb-operator](https://github.com/authzed/spicedb-operator/) repository.

**How to Update**

1. Set `TAG` to the upstream release tag being synced (eg. `v1.21.0`)
2. Set `COMMIT_SHA` to the full commit SHA of the upstream commit being merged

### Updating this File

If changes are made to our fork that diverge from upstream that are not captured in this README, make sure to update this file with any relevant changes. Be sure to capture the change and reason in the table above.

An easy way to capture differences is to use `scripts/redhat-diff.sh` which compares the merge branch against the upstream tag and shows only Red Hat-specific changes:

```bash
# Show summary of Red Hat changes for this sync
./scripts/redhat-diff.sh --stat

# Show all cumulative Red Hat changes vs upstream
./scripts/redhat-diff.sh --all --stat
```
