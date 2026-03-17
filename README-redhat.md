# Information about Red Hat's Fork of SpiceDB Operator

This document captures all changes made by Red Hat that diverge from the upstream fork. It outlines why these changes were made for historical reference and to help ensure they are kept when syncing with upstream.

&nbsp;

### Drift Tracking

The table below captures high level changes to our fork from upstream and the reason for these changes.

|Change|Reason|
|------|------|
|Dependabot intervals changed to daily |This better aligns with other Kessel Services|
|All active workflows defined by Authzed updated use the `ubuntu-latest` image for the runner|Authzed uses a custom self-hosted runner in their workflows which we don't have access to|
|Build Test - Build Container Image workflow uses `Dockerfile.openshift` vs `Dockerfile`|This ensures the image build test uses our custom Dockerfile vs upstreams|
|Non-critical workflows disabled or removed|Workflows that do not impact code functionality or Red Hat builds are disabled.<br><br> This includes:<br> * E2E test<br> * Yaml & Markdown Linting<br> * CLA workflow<br> * Release workflows|
|Added the security scanning workflow|Required ConsoleDot platform security workflow to check for CVE's in code and images|
|Added push/pull tekton pipelines|Used for Konflux PR and merge builds|
|Added Dockerfile.openshift|Dockerfile used by Konflux for building images, to comply with requirements of using UBI as the base image, and to ensure FIPS-compliant builds using Go Toolset|
|`build_deploy.sh` script added|Legacy script used by App Interface for container image builds pre-Konflux. Still used for building images locally for testing operator updates|
| `deploy/deploy.yml` added|This is the main deployment file for deploying SpiceDB Operator to OpenShift clusters. It contains all required CRD's, ClusterRoles/Bindings, and various Kubernetes objects required to run the operator that ship with a release from Authzed, but with key updates made to support running on OpenShift clusters (see below)|


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

The below table captures changes that diverge from the upstream bundle release. Each release of SpiceDB Operator will require a review of the bundle to pull in any changes required, which may require updating the below table.

|Change|Reason|
|------|------|
|Added the `renovate.json` file|This configures Mintmaker (part of Konflux) to prevent Go pkg update PRs and move to weekly updates for Dockerfile base image updates|
|Removed the `update-graph` ConfigMap|The `update-graph` is useful for defining the SpiceDB version to use for a cluster, and controlling automatic upgrades. Since we build and use our own SpiceDB image, this feature is disabled in our `SpiceDbClusters` CR, so the ConfigMap is not needed|
|`spec.containers.image` has been updated to use the Red Hat built SpiceDB|This ensures the SpiceDB image running in clusters complies with Red Hat policies, and security standards|
|CPU and Memory adjustments|The CPU and Memory requests/limits have been increased for our deployment needs|
|`runAsUser` and `runAsGroup` directives from both container and pod `securityContexts` have been removed|These violate OpenShift security policies and prevent the image from running due to using the `nobody` user|
|The `config` volume/volume mount has been removed|This volume is used to facilitate the `update-graph` ConfigMap which was also removed|


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

An easy way to capture differences is to use `git diff` and compare your synced branch to the upstream tag branch to see all differences between upstream and the current state.

```bash
# assumes you have an `upstream` remote for the upstream source code
git checkout -b upstream-<tag> tags/<tag>
git diff upstream-<tag>..<your-synced-branch>
```
