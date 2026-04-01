BIN_DIR := bin
VALIDATE_UPGRADE_PATH := $(BIN_DIR)/validate-upgrade-path

.PHONY: build-validate-upgrade-path
build-validate-upgrade-path: $(VALIDATE_UPGRADE_PATH) ## Build the validate-upgrade-path tool

$(VALIDATE_UPGRADE_PATH):
	go build -C tools -o ../$(VALIDATE_UPGRADE_PATH) ./validate-upgrade-path/

.PHONY: clean
clean: ## Remove built binaries
	rm -rf $(BIN_DIR)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}'
