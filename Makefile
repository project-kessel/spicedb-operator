BIN_DIR := bin
VALIDATE_UPGRADE_PATH := $(BIN_DIR)/validate-upgrade-path

GIT_COMMIT     := $(shell git rev-parse --short HEAD)
IMAGE_TAG      := $(shell git rev-parse --short=7 HEAD)
DOCKER         ?= $(shell command -v podman || command -v docker)
PLATFORM_FLAGS := $(shell if [ "$$(uname -s)" = "Darwin" ]; then echo "--platform linux/amd64 --build-arg TARGETARCH=amd64"; fi)

.PHONY: build-validate-upgrade-path
build-validate-upgrade-path: $(VALIDATE_UPGRADE_PATH) ## Build the validate-upgrade-path tool

$(VALIDATE_UPGRADE_PATH):
	go build -C tools -o ../$(VALIDATE_UPGRADE_PATH) ./validate-upgrade-path/

.PHONY: install-validate-upgrade-path
install-validate-upgrade-path: ## Install validate-upgrade-path to $GOPATH/bin
	go install -C tools ./validate-upgrade-path/

.PHONY: clean
clean: ## Remove built binaries
	rm -rf $(BIN_DIR)

.PHONY: docker-build-push
docker-build-push: ## Build and push the container image (requires IMAGE=quay.io/youruser/spicedb-operator)
ifndef IMAGE
	$(error IMAGE is required. Example: make docker-build-push IMAGE=quay.io/youruser/spicedb-operator)
endif
	@[ -n "$(DOCKER)" ] || { echo "Error: neither podman nor docker found. Please install one to continue."; exit 1; }
	@printf '%s\n' "$(IMAGE)" | grep -qE '^[a-zA-Z0-9][a-zA-Z0-9._/:@-]*$$' || { echo "IMAGE contains invalid characters. Use format: quay.io/your-org/image-name"; exit 1; }
	@"$(DOCKER)" build $(PLATFORM_FLAGS) --build-arg GIT_COMMIT="$(GIT_COMMIT)" -t "$(IMAGE):$(IMAGE_TAG)" -f ./Dockerfile . || \
		(echo "Build failed. If due to authentication, check your registry credentials and try again." && exit 1)
	@"$(DOCKER)" push "$(IMAGE):$(IMAGE_TAG)" || \
		(echo "Push failed. If due to authentication, run: $(DOCKER) login quay.io" && exit 1)
	@"$(DOCKER)" tag "$(IMAGE):$(IMAGE_TAG)" "$(IMAGE):latest"
	@"$(DOCKER)" push "$(IMAGE):latest" || \
		(echo "Push failed. If due to authentication, run: $(DOCKER) login quay.io" && exit 1)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}'
