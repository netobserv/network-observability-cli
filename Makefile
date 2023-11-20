NAME := network-observability-cli
DIST_DIR ?= build
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
build: prepare lint
	@go build -o $(OUTPUT)
	cp -a ./oc/. ./$(DIST_DIR)
	cp -a ./res/. ./$(DIST_DIR)/network-observability-cli-resources

.PHONY: image
image:
	$(OCI_BIN) build -t network-observability-cli .

.PHONY: lint
lint: prereqs ## Lint code
	@echo "### Linting code"
	golangci-lint run ./...

.PHONY: clean
clean:
	@rm -rf $(DIST_DIR)
