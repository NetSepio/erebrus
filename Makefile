# Binary name and entrypoint
BINARY_NAME=erebrus
PKG=./cmd/erebrus

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOCLEAN=$(GOCMD) clean

# Build tags. with_reality_server enables the sing-box REALITY *server* used by
# the VLESS stealth carrier; without it REALITY inbounds fail to start at
# runtime. Keep this in sync with the Dockerfile.
BUILD_TAGS=with_reality_server

# Build the node binary
build:
	$(GOBUILD) -tags "$(BUILD_TAGS)" -o $(BINARY_NAME) -v $(PKG)

# Install the binary
install:
	$(GOINSTALL) -tags "$(BUILD_TAGS)" $(PKG)

# Run vet across all packages (stealth needs the tag to type-check fully)
vet:
	$(GOVET) -tags "$(BUILD_TAGS)" ./...

# Run the test suite
test:
	$(GOTEST) -tags "$(BUILD_TAGS)" ./...

# Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Build and install
all: build install

.PHONY: build install vet test clean all
