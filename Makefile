NAME := network-observability-cli
DIST_DIR ?= build
OUTPUT := $(DIST_DIR)/$(NAME)
DOCKER := $(shell which podman)
ifeq ($(DOCKER),)
	DOCKER := $(shell which docker)
endif

.PHONY: all
all: build

.PHONY: prepare
prepare:
	@mkdir -p $(DIST_DIR)

.PHONY: build
build: prepare
	@go build -o $(OUTPUT)
	cp -a ./oc/. ./$(DIST_DIR)
	cp -a ./res/. ./$(DIST_DIR)/network-observability-cli-resources

.PHONY: image
image:
	$(DOCKER) build -t network-observability-cli .

.PHONY: clean
clean:
	@rm -rf $(DIST_DIR)