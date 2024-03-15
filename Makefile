NAME := network-observability-cli
DIST_DIR ?= build
FILES_OUTPUT_DIR ?= output
OUTPUT := $(DIST_DIR)/$(NAME)

# Image building tool (docker / podman) - docker is preferred in CI
OCI_BIN_PATH = $(shell which docker 2>/dev/null || which podman)
OCI_BIN ?= $(shell basename ${OCI_BIN_PATH})

GOLANGCI_LINT_VERSION = v1.53.3

.PHONY: all
all: build

.PHONY: prepare
prepare:
	@mkdir -p $(DIST_DIR)

.PHONY: prereqs
prereqs: ## Test if prerequisites are met, and installing missing dependencies
	@echo "### Test if prerequisites are met, and installing missing dependencies"
	GOFLAGS="" go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}

.PHONY: build
build: prepare lint ## Build the binary and run Lint
	@go build -o $(OUTPUT)
	cp -a ./oc/. ./$(DIST_DIR)
	cp -a ./res/. ./$(DIST_DIR)/network-observability-cli-resources

.PHONY: image
image: ## Build the docker image
	$(OCI_BIN) build -t network-observability-cli .

.PHONY: lint
lint: prereqs ## Lint code
	@echo "### Linting code"
	golangci-lint run ./...
ifeq (, $(shell which shellcheck))
	@echo "### shellcheck could not be found, skipping shell lint"
else
	@echo "### Run shellcheck against bash scripts"
	find . -name '*.sh' | xargs shellcheck
endif

.PHONY: clean
clean: ## Clean up build directory
	@rm -rf $(DIST_DIR)
	@rm -rf $(FILES_OUTPUT_DIR)

.PHONY: oc-commands
oc-commands: build
	sudo cp -a ./build/. /usr/bin/

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
