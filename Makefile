.PHONY: build test lint clean install run help release-prepare release-build-all release-checksums

# Build variables
BINARY_NAME=ds
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build directories
BUILD_DIR=bin
CMD_DIR=cmd/ds

help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

test: ## Run unit tests
	@echo "Running unit tests..."
	$(GOTEST) -v -race -short -coverprofile=coverage.out ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -v -race -tags=integration ./test/integration/...

test-all: ## Run all tests (unit + integration)
	@echo "Running all tests..."
	$(GOTEST) -v -race -tags=integration -coverprofile=coverage.out ./...

test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linters
	@echo "Running go fmt..."
	$(GOFMT) ./...
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Checking go mod tidy..."
	$(GOMOD) tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod or go.sum needs updating" && exit 1)

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

install: build ## Install binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

run: build ## Build and run the binary
	@$(BUILD_DIR)/$(BINARY_NAME)

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

all: deps lint test build ## Run all checks and build

release-prepare: ## Prepare for release
	@echo "Preparing release $(VERSION)"
	@mkdir -p $(BUILD_DIR)

release-build-all: release-prepare ## Build multi-platform binaries for release
	@echo "Building release artifacts for $(VERSION)"
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	COPYFILE_DISABLE=1 tar --exclude='._*' -czf $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)
	rm $(BUILD_DIR)/$(BINARY_NAME)
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	COPYFILE_DISABLE=1 tar --exclude='._*' -czf $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)
	rm $(BUILD_DIR)/$(BINARY_NAME)
	# Linux ARM (32-bit)
	GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	COPYFILE_DISABLE=1 tar --exclude='._*' -czf $(BUILD_DIR)/$(BINARY_NAME)-linux-arm.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)
	rm $(BUILD_DIR)/$(BINARY_NAME)
	# macOS AMD64 (Intel)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	COPYFILE_DISABLE=1 tar --exclude='._*' -czf $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)
	rm $(BUILD_DIR)/$(BINARY_NAME)
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	COPYFILE_DISABLE=1 tar --exclude='._*' -czf $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)
	rm $(BUILD_DIR)/$(BINARY_NAME)
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME).exe ./$(CMD_DIR)
	cd $(BUILD_DIR) && zip $(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME).exe && cd ..
	rm $(BUILD_DIR)/$(BINARY_NAME).exe
	# Windows ARM64
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME).exe ./$(CMD_DIR)
	cd $(BUILD_DIR) && zip $(BINARY_NAME)-windows-arm64.zip $(BINARY_NAME).exe && cd ..
	rm $(BUILD_DIR)/$(BINARY_NAME).exe

release-checksums: ## Create checksums for release artifacts
	@echo "Creating checksums..."
	cd $(BUILD_DIR) && sha256sum $(BINARY_NAME)-* > checksums.txt && cd ..
	@echo "Checksums created: $(BUILD_DIR)/checksums.txt"

.DEFAULT_GOAL := help
