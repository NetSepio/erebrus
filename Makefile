# Binaries
NODE_BINARY=erebrus-node
LEGACY_BINARY=erebrus
SENTINEL_BINARY=erebrus-sentinel
NODE_PKG=./cmd/erebrus-node
LEGACY_PKG=./cmd/erebrus
SENTINEL_PKG=./cmd/erebrus-sentinel

GOCMD=go
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOCLEAN=$(GOCMD) clean

BUILD_TAGS=with_reality_server
LDFLAGS=-ldflags "-X github.com/NetSepio/erebrus/internal/config.Version=2.0.0-dev"

.PHONY: build build-node build-legacy build-sentinel install vet test clean all

build: build-node build-legacy

build-node:
	$(GOBUILD) -tags "$(BUILD_TAGS)" $(LDFLAGS) -o $(NODE_BINARY) -v $(NODE_PKG)

build-legacy:
	$(GOBUILD) -tags "$(BUILD_TAGS)" $(LDFLAGS) -o $(LEGACY_BINARY) -v $(LEGACY_PKG)

build-sentinel:
	$(GOBUILD) -o $(SENTINEL_BINARY) -v $(SENTINEL_PKG)

build-all: build build-sentinel

install:
	$(GOINSTALL) -tags "$(BUILD_TAGS)" $(NODE_PKG)
	$(GOINSTALL) -tags "$(BUILD_TAGS)" $(LEGACY_PKG)

vet:
	$(GOVET) -tags "$(BUILD_TAGS)" ./...

test:
	$(GOTEST) -tags "$(BUILD_TAGS)" ./...

clean:
	$(GOCLEAN)
	rm -f $(NODE_BINARY) $(LEGACY_BINARY) $(SENTINEL_BINARY)

all: build install