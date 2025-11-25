PLUGIN_NAME := my-plugin
VERSION := 1.0.0
BINARY := ds-$(PLUGIN_NAME)

.PHONY: build
build:
	go build -o $(BINARY) main.go

.PHONY: build-all
build-all:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -o dist/$(BINARY)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 go build -o dist/$(BINARY)-linux-arm64 main.go
	GOOS=darwin GOARCH=amd64 go build -o dist/$(BINARY)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build -o dist/$(BINARY)-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY)-windows-amd64.exe main.go

.PHONY: test
test:
	go test -v ./...

.PHONY: install
install: build
	mkdir -p ~/.config/ds/plugins
	cp $(BINARY) ~/.config/ds/plugins/
	cp plugin.yaml ~/.config/ds/plugins/$(PLUGIN_NAME).yaml

.PHONY: clean
clean:
	rm -rf $(BINARY) dist

.PHONY: deps
deps:
	go mod download
	go mod tidy

.PHONY: lint
lint:
	golangci-lint run

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       - Build plugin for current platform"
	@echo "  build-all   - Build plugin for all platforms"
	@echo "  test        - Run tests"
	@echo "  install     - Install plugin locally"
	@echo "  clean       - Clean build artifacts"
	@echo "  deps        - Download dependencies"
	@echo "  lint        - Run linter"
	@echo "  help        - Show this help"
