.DEFAULT_GOAL := build
GO ?= go
GOARCH ?= $(shell $(GO) env GOARCH)
BUILD_DIR := $(CURDIR)/dist
BIN := $(BUILD_DIR)/proxyctl
LINUX_BIN := $(BUILD_DIR)/proxyctl-linux-$(GOARCH)

.PHONY: build build-linux vet test smoke verify
build:
	mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BIN) .

build-linux:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=$(GOARCH) $(GO) build -o $(LINUX_BIN) .

vet:
	$(GO) vet ./...
	GOOS=linux $(GO) vet ./...

test:
	$(GO) test ./...

smoke: build
	sh scripts/smoke.sh $(BIN)

verify: vet test smoke build-linux

include make/orbstack.mk
