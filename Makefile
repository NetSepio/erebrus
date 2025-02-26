swagger:
	GO111MODULE=off swagger generate spec -o ./docs/swagger.yml  --scan-models

markdown:
	swagger generate markdown -f ./docs/swagger.yml --output=./docs/docs.md

# Binary name
BINARY_NAME=erebrus

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GOCLEAN=$(GOCMD) clean

# Build the project
build:
	$(GOBUILD) -o $(BINARY_NAME) -v

# Install the binary
install:
	$(GOINSTALL)

# Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Build and install
all: build install

.PHONY: build install clean all