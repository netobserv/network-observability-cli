# VERSION defines the project version.
# Update this value when you upgrade the version of your project.
# To re-generate a tar.gz for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make tar-commands VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= main
BUILD_DATE := $(shell date +%Y-%m-%d\ %H:%M)
TAG_COMMIT := $(shell git rev-list --abbrev-commit --tags --max-count=1)
TAG := $(shell git describe --abbrev=0 --tags ${TAG_COMMIT} 2>/dev/null || true)
BUILD_SHA := $(shell git rev-parse --short HEAD)
BUILD_VERSION := $(TAG:v%=%)
ifneq ($(COMMIT), $(TAG_COMMIT))
	BUILD_VERSION := $(BUILD_VERSION)-$(BUILD_SHA)
endif
ifneq ($(shell git status --porcelain),)
	BUILD_VERSION := $(BUILD_VERSION)-dirty
endif

# Go architecture and targets images to build
GOARCH ?= amd64
MULTIARCH_TARGETS ?= amd64

# In CI, to be replaced by `netobserv`
IMAGE_ORG ?= $(USER)

# Build output 
NAME := network-observability-cli
DIST_DIR ?= build
FILES_OUTPUT_DIR ?= output
OUTPUT := $(DIST_DIR)/$(NAME)

# Available commands for development with args
COMMANDS = flows packets cleanup
COMMAND_ARGS ?= 

# Get either oc (favorite) or kubectl paths
K8S_CLI_BIN_PATH = $(shell which oc 2>/dev/null || which kubectl)
K8S_CLI_BIN ?= $(shell basename ${K8S_CLI_BIN_PATH})

# IMAGE_TAG_BASE defines the namespace and part of the image name for remote images.
IMAGE_TAG_BASE ?= quay.io/$(IMAGE_ORG)/$(NAME)

# Image URL to use all building/pushing image targets
IMAGE ?= $(IMAGE_TAG_BASE):$(VERSION)
PULL_POLICY ?=Always
OCI_BUILD_OPTS ?=

# Image building tool (docker / podman) - docker is preferred in CI
OCI_BIN_PATH := $(shell which docker 2>/dev/null || which podman)
OCI_BIN ?= $(shell basename ${OCI_BIN_PATH})
KREW_PLUGIN ?=false

GOLANGCI_LINT_VERSION = v1.54.2

# build a single arch target provided as argument
define build_target
	echo 'building image for arch $(1)'; \
	DOCKER_BUILDKIT=1 $(OCI_BIN) buildx build --load --build-arg TARGETPLATFORM=linux/$(1) --build-arg TARGETARCH=$(1) --build-arg BUILDPLATFORM=linux/amd64 ${OCI_BUILD_OPTS} -t ${IMAGE}-$(1) -f Dockerfile .;
endef

# push a single arch target image
define push_target
	echo 'pushing image ${IMAGE}-$(1)'; \
	DOCKER_BUILDKIT=1 $(OCI_BIN) push ${IMAGE}-$(1);
endef

# manifest create a single arch target provided as argument
define manifest_add_target
	echo 'manifest add target $(1)'; \
	DOCKER_BUILDKIT=1 $(OCI_BIN) manifest add ${IMAGE} ${IMAGE}-$(1);
endef

##@ General

.PHONY: prepare
prepare:
	@mkdir -p $(DIST_DIR)
	mkdir -p tmp

.PHONY: prereqs
prereqs: ## Test if prerequisites are met, and installing missing dependencies
	@echo "### Test if prerequisites are met, and installing missing dependencies"
ifeq (, $(shell which golangci-lint))
	GOFLAGS="" go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}
endif

.PHONY: vendors
vendors: ## Refresh vendors directory.
	@echo "### Checking vendors"
	go mod tidy && go mod vendor

##@ Develop

.PHONY: compile
compile: ## Build the binary
	@echo "### Compiling project"
	GOARCH=${GOARCH} go build -ldflags "-X main.version=${VERSION} -X 'main.buildVersion=${BUILD_VERSION}' -X 'main.buildDate=${BUILD_DATE}'" -mod vendor -a -o $(OUTPUT)

.PHONY: build
build: fmt lint compile ## Build project (fmt + lint + compile)

.PHONY: test
test: ## Test code using go test
	@echo "### Testing code"
	GOOS=$(GOOS) go test -mod vendor -a ./... -coverpkg=./... -coverprofile cover.out

.PHONY: coverage-report
coverage-report: ## Generate coverage report
	@echo "### Generating coverage report"
	go tool cover --func=./cover.out

.PHONY: coverage-report-html
coverage-report-html: ## Generate HTML coverage report
	@echo "### Generating HTML coverage report"
	go tool cover --html=./cover.out

.PHONY: fmt
fmt: ## Run go fmt against code.
	@echo "### Formatting code"
	go fmt ./...

.PHONY: lint
lint: prereqs ## Lint code
	@echo "### Linting code"
	golangci-lint run ./... --timeout=3m
ifeq (, $(shell which shellcheck))
	@echo "### shellcheck could not be found, skipping shell lint"
else
	@echo "### Run shellcheck against bash scripts"
	find . -name '*.sh' -not -path "./vendor/*" | xargs shellcheck
endif

.PHONY: clean
clean: ## Clean up build directory
	@rm -rf $(DIST_DIR)
	@rm -rf $(FILES_OUTPUT_DIR)

.PHONY: commands
commands: ## Generate either oc or kubectl plugins and add them to build folder
	@echo "### Generating $(K8S_CLI_BIN) commands"
	DIST_DIR=$(DIST_DIR) \
	K8S_CLI_BIN=$(K8S_CLI_BIN) \
	IMAGE=$(IMAGE) \
	PULL_POLICY=$(PULL_POLICY) \
	VERSION=$(VERSION) \
	./scripts/inject.sh

.PHONY: kubectl-commands
kubectl-commands: K8S_CLI_BIN=kubectl
kubectl-commands: commands ## Generate kubectl plugins and add them to build folder

.PHONY: oc-commands
oc-commands: K8S_CLI_BIN=oc
oc-commands: commands ## Generate oc plugins and add them to build folder

.PHONY: install-commands
install-commands: commands ## Generate plugins and add them to /usr/bin/
	sudo cp -a ./build/. /usr/bin/

.PHONY: tar-commands
tar-commands: clean ## Generate tar.gz containing krew plugin
	$(MAKE) KREW_PLUGIN=true kubectl-commands
	tar -czf netobserv-cli.tar.gz LICENSE ./build/netobserv
	sha256sum netobserv-cli.tar.gz

.PHONY: create-kind-cluster
create-kind-cluster: prereqs ## Create a kind cluster
	scripts/kind-cluster.sh

.PHONY: destroy-kind-cluster
destroy-kind-cluster: KUBECONFIG=./kubeconfig
destroy-kind-cluster: ## Destroy the kind cluster.
	test -s ./kubeconfig || { echo "kubeconfig does not exist! Exiting..."; exit 1; }
	$(K8S_CLI_BIN) delete -f ./res/namespace.yml --ignore-not-found
	kind delete cluster --name netobserv-cli-cluster
	rm ./kubeconfig

.PHONY: $(COMMANDS)
$(COMMANDS): commands ## Run command using custom image
	@echo "### Running ${K8S_CLI_BIN}-netobserv $@ using $(IMAGE)"
	./$(DIST_DIR)/${K8S_CLI_BIN}-netobserv $@ $(COMMAND_ARGS)

##@ Images

# note: to build and push custom image tag use: IMAGE_ORG=myuser VERSION=dev s
.PHONY: image-build
image-build: ## Build MULTIARCH_TARGETS images
	trap 'exit' INT; \
	$(foreach target,$(MULTIARCH_TARGETS),$(call build_target,$(target)))

.PHONY: image-push
image-push: ## Push MULTIARCH_TARGETS images
	trap 'exit' INT; \
	$(foreach target,$(MULTIARCH_TARGETS),$(call push_target,$(target)))

.PHONY: manifest-build
manifest-build: ## Build MULTIARCH_TARGETS manifest
	echo 'building manifest $(IMAGE)'
	DOCKER_BUILDKIT=1 $(OCI_BIN) rmi ${IMAGE} -f
	DOCKER_BUILDKIT=1 $(OCI_BIN) manifest create ${IMAGE} $(foreach target,$(MULTIARCH_TARGETS), --amend ${IMAGE}-$(target));

.PHONY: manifest-push
manifest-push: ## Push MULTIARCH_TARGETS manifest
	@echo 'publish manifest $(IMAGE)'
ifeq (${OCI_BIN}, docker)
	DOCKER_BUILDKIT=1 $(OCI_BIN) manifest push ${IMAGE};
else
	DOCKER_BUILDKIT=1 $(OCI_BIN) manifest push ${IMAGE} docker://${IMAGE};
endif

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

include .mk/shortcuts.mk
