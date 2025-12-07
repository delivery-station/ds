# Plugin Development Guide

## Overview

DS plugins are standalone executable binaries that extend DS functionality. Plugins can be written in any language and are distributed as OCI artifacts. This guide walks you through creating, testing, and publishing a DS plugin.

## Plugin Fundamentals

### What is a Plugin?

A DS plugin is:
- A standalone executable binary
- Named with `ds-` prefix (e.g., `ds-porter`, `ds-s3-uploader`)
- Accompanied by a `plugin.yaml` manifest
- Distributed as an OCI artifact
- Invoked as a subprocess by DS core

### Plugin Contract

Plugins must adhere to the following contract:

1. **Binary Naming**: `ds-<plugin-name>` (lowercase, hyphen-separated)
2. **Manifest**: `plugin.yaml` or `<plugin-name>.yaml` in the same directory
3. **Distribution Manifest**: `ds.manifest.yaml` for OCI distribution (see [Plugin Manifest Specification](plugin-manifest.md))
4. **Version Flag**: Support `--version` flag
5. **Exit Codes**: Return 0 for success, non-zero for errors
6. **Configuration**: Read from `DS_*` environment variables
7. **Output**: Write to stdout/stderr appropriately

## Quick Start

### 1. Create Plugin Directory

```bash
mkdir my-plugin
cd my-plugin
```

### 2. Create Plugin Binary

Example in Go:

```go
// main.go
package main

import (
    "fmt"
    "os"
)

const version = "1.0.0"

func main() {
    // Handle --version flag
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Println(version)
        os.Exit(0)
    }

    // Read DS configuration from environment
    registry := os.Getenv("DS_REGISTRY_DEFAULT")
    cacheDir := os.Getenv("DS_CACHE_DIR")
    
    fmt.Printf("Using registry: %s\n", registry)
    fmt.Printf("Using cache: %s\n", cacheDir)
    
    // Your plugin logic here
    fmt.Println("Hello from my-plugin!")
}
```

Build:
```bash
go build -o ds-my-plugin main.go
chmod +x ds-my-plugin
```

Example in Python:

```python
#!/usr/bin/env python3
import os
import sys

VERSION = "1.0.0"

def main():
    # Handle --version flag
    if len(sys.argv) > 1 and sys.argv[1] == "--version":
        print(VERSION)
        sys.exit(0)
    
    # Read DS configuration from environment
    registry = os.getenv("DS_REGISTRY_DEFAULT", "")
    cache_dir = os.getenv("DS_CACHE_DIR", "")
    
    print(f"Using registry: {registry}")
    print(f"Using cache: {cache_dir}")
    
    # Your plugin logic here
    print("Hello from my-plugin!")

if __name__ == "__main__":
    main()
```

Make executable:
```bash
chmod +x ds-my-plugin
```

### 3. Create Plugin Manifest

```yaml
# plugin.yaml
name: my-plugin
version: 1.0.0
description: My awesome DS plugin
author: Your Name <your.email@example.com>

# Supported platforms
platforms:
  - os: linux
    arch: amd64
  - os: linux
    arch: arm64
  - os: darwin
    arch: amd64
  - os: darwin
    arch: arm64
  - os: windows
    arch: amd64

# Commands provided by this plugin
commands:
  - name: hello
    description: Say hello
    usage: ds my-plugin hello [name]
  
  - name: fetch
    description: Fetch an artifact
    usage: ds my-plugin fetch <artifact>

# Dependencies on other plugins (optional)
dependencies:
  - name: s3-uploader
    version: ">=1.0.0"

# Configuration schema (optional)
config:
  - name: MY_PLUGIN_API_KEY
    description: API key for my-plugin service
    required: false
    default: ""
```

### 4. Test Locally

```bash
# Test version flag
./ds-my-plugin --version

# Test with DS environment
export DS_REGISTRY_DEFAULT=ghcr.io
export DS_CACHE_DIR=~/.cache/ds
./ds-my-plugin

# Install locally
mkdir -p ~/.config/ds/plugins
cp ds-my-plugin ~/.config/ds/plugins/
cp plugin.yaml ~/.config/ds/plugins/my-plugin.yaml

# Test via DS
ds my-plugin hello
```

## Using the DS Client Library

For Go plugins, you can use the DS client library to interact with DS core:

### Installation

```bash
go get github.com/delivery-station/ds/pkg/client
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/delivery-station/ds/pkg/client"
)

func main() {
    // Create DS client
    dsClient, err := client.NewClient()
    if err != nil {
        log.Fatalf("Failed to create DS client: %v", err)
    }
    defer dsClient.Close()

    ctx := context.Background()

    // Pull an artifact using DS cache
    artifactRef := "ghcr.io/myorg/myapp:v1.0.0"
    err = dsClient.Pull(ctx, artifactRef, os.Stdout)
    if err != nil {
        log.Fatalf("Failed to pull artifact: %v", err)
    }

    fmt.Println("Artifact pulled successfully")
}
```

### Advanced Features

#### Event Publishing

```go
// Publish an event that other plugins can subscribe to
err = dsClient.PublishEvent(ctx, client.Event{
    Type: "artifact.downloaded",
    Source: "my-plugin",
    Data: map[string]interface{}{
        "artifact": artifactRef,
        "size": 1024000,
    },
})
```

#### State Management

```go
// Store state that other plugins can access
err = dsClient.SetState(ctx, "my-plugin.last-artifact", artifactRef)

// Retrieve state
lastArtifact, err := dsClient.GetState(ctx, "my-plugin.last-artifact")
```

#### Calling Other Plugins

```go
// Call another plugin
output, err := dsClient.CallPlugin(ctx, "s3-uploader", []string{
    "upload",
    "--bucket", "my-bucket",
    "--key", "artifact.tar",
    "--file", "/tmp/artifact.tar",
})
if err != nil {
    log.Fatalf("Failed to call s3-uploader: %v", err)
}
fmt.Println(output)
```

## Configuration Access

### Environment Variables

DS automatically sets these environment variables for plugins:

```bash
# Registry configuration
DS_REGISTRY_DEFAULT          # Default registry (e.g., ghcr.io)
DS_REGISTRY_MIRRORS          # JSON array of mirror registries
DS_REGISTRY_INSECURE         # JSON array of insecure registries

# Cache configuration
DS_CACHE_DIR                 # Cache directory path
DS_CACHE_MAX_SIZE            # Maximum cache size
DS_CACHE_TTL                 # Cache TTL

# Authentication
DS_AUTH_DOCKER_CONFIG        # Path to Docker config.json
DS_AUTH_CREDENTIALS          # JSON array of explicit credentials

# Plugin configuration
DS_PLUGINS_DIR               # Plugin directory
DS_PLUGINS_TIMEOUT           # Plugin execution timeout

# Logging
DS_LOGGING_LEVEL             # Log level (debug, info, warn, error)
DS_LOGGING_FORMAT            # Log format (text, json)

# Context (when called by another plugin)
DS_PLUGIN_CONTEXT            # JSON context from calling plugin
DS_TRACE_ID                  # Trace ID for request tracing
```

### Reading Configuration in Go

```go
package main

import (
    "encoding/json"
    "os"
    "time"
)

type Config struct {
    Registry     string
    CacheDir     string
    CacheTTL     time.Duration
    LogLevel     string
}

func loadConfig() (*Config, error) {
    cfg := &Config{
        Registry: os.Getenv("DS_REGISTRY_DEFAULT"),
        CacheDir: os.Getenv("DS_CACHE_DIR"),
        LogLevel: os.Getenv("DS_LOGGING_LEVEL"),
    }
    
    // Parse TTL
    ttlStr := os.Getenv("DS_CACHE_TTL")
    if ttlStr != "" {
        ttl, err := time.ParseDuration(ttlStr)
        if err != nil {
            return nil, err
        }
        cfg.CacheTTL = ttl
    }
    
    return cfg, nil
}
```

### Reading Configuration in Python

```python
import os
import json

class Config:
    def __init__(self):
        self.registry = os.getenv("DS_REGISTRY_DEFAULT", "ghcr.io")
        self.cache_dir = os.getenv("DS_CACHE_DIR", "~/.cache/ds")
        self.cache_ttl = os.getenv("DS_CACHE_TTL", "7d")
        self.log_level = os.getenv("DS_LOGGING_LEVEL", "info")
        
        # Parse JSON arrays
        mirrors = os.getenv("DS_REGISTRY_MIRRORS", "[]")
        self.mirrors = json.loads(mirrors)

config = Config()
print(f"Using registry: {config.registry}")
```

## Testing Plugins

### Unit Testing

Test your plugin logic independently:

```go
// my_plugin_test.go
package main

import (
    "testing"
)

func TestPluginLogic(t *testing.T) {
    // Your tests here
}
```

### Integration Testing with DS

Create a test script:

```bash
#!/bin/bash
# test-plugin.sh

set -e

echo "Building plugin..."
go build -o ds-my-plugin main.go

echo "Installing plugin..."
mkdir -p ~/.config/ds/plugins
cp ds-my-plugin ~/.config/ds/plugins/
cp plugin.yaml ~/.config/ds/plugins/my-plugin.yaml

echo "Testing plugin via DS..."
ds my-plugin --version
ds my-plugin hello world

echo "All tests passed!"
```

### Mock Testing

For plugins that interact with external services, use mocks:

```go
// Use interfaces for testability
type ArtifactStore interface {
    Pull(ref string) error
    Push(ref string, data []byte) error
}

// Real implementation
type OCIStore struct {}

// Mock implementation for testing
type MockStore struct {
    PullFunc func(ref string) error
    PushFunc func(ref string, data []byte) error
}
```

## Building for Multiple Platforms

### Using Go

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o ds-my-plugin-linux-amd64 main.go

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o ds-my-plugin-linux-arm64 main.go

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o ds-my-plugin-darwin-amd64 main.go

# macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o ds-my-plugin-darwin-arm64 main.go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o ds-my-plugin-windows-amd64.exe main.go
```

### Using Make

```makefile
# Makefile
PLUGIN_NAME := my-plugin
VERSION := 1.0.0

.PHONY: build
build:
	go build -o ds-$(PLUGIN_NAME) main.go

.PHONY: build-all
build-all:
	GOOS=linux GOARCH=amd64 go build -o dist/ds-$(PLUGIN_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 go build -o dist/ds-$(PLUGIN_NAME)-linux-arm64 main.go
	GOOS=darwin GOARCH=amd64 go build -o dist/ds-$(PLUGIN_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build -o dist/ds-$(PLUGIN_NAME)-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 go build -o dist/ds-$(PLUGIN_NAME)-windows-amd64.exe main.go

.PHONY: test
test:
	go test -v ./...

.PHONY: clean
clean:
	rm -rf dist ds-$(PLUGIN_NAME)
```

## Publishing to OCI Registry

### Prepare Artifact

```bash
# Create artifact directory
mkdir -p artifact/my-plugin/1.0.0

# Copy binary and manifest
cp ds-my-plugin artifact/my-plugin/1.0.0/
cp plugin.yaml artifact/my-plugin/1.0.0/

# Optional: Add README
cp README.md artifact/my-plugin/1.0.0/
```

### Push to Registry

Using ORAS:

```bash
# Login to registry
docker login ghcr.io

# Push artifact
oras push ghcr.io/myorg/ds-my-plugin:1.0.0 \
    --artifact-type application/vnd.ds.plugin.v1 \
    artifact/my-plugin/1.0.0/
```

Using DS:

```bash
# Package and push
ds plugin package ./artifact/my-plugin/1.0.0
ds plugin push ghcr.io/myorg/ds-my-plugin:1.0.0 ./artifact/my-plugin/1.0.0
```

### Multi-Platform Artifacts

Create manifest list for multiple platforms:

```bash
# Push platform-specific images
oras push ghcr.io/myorg/ds-my-plugin:1.0.0-linux-amd64 \
    --artifact-type application/vnd.ds.plugin.v1 \
    ./dist/ds-my-plugin-linux-amd64

oras push ghcr.io/myorg/ds-my-plugin:1.0.0-darwin-arm64 \
    --artifact-type application/vnd.ds.plugin.v1 \
    ./dist/ds-my-plugin-darwin-arm64

# Create manifest list
docker manifest create ghcr.io/myorg/ds-my-plugin:1.0.0 \
    --amend ghcr.io/myorg/ds-my-plugin:1.0.0-linux-amd64 \
    --amend ghcr.io/myorg/ds-my-plugin:1.0.0-darwin-arm64

# Push manifest list
docker manifest push ghcr.io/myorg/ds-my-plugin:1.0.0

> **Important**: DS installs plugins exclusively from multi-architecture indexes. Always push an OCI index (`application/vnd.oci.image.index.v1+json`) that references each platform-specific binary. DS does not attempt to resolve architecture-specific tags such as `:latest-darwin-arm64`; if the index is missing an entry for the requesting platform, installation will fail.
```

## Plugin Examples

### Example 1: Simple Greeter Plugin

```go
// ds-greeter/main.go
package main

import (
    "fmt"
    "os"
)

const version = "1.0.0"

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Println(version)
        return
    }

    name := "World"
    if len(os.Args) > 1 {
        name = os.Args[1]
    }

    fmt.Printf("Hello, %s!\n", name)
    fmt.Printf("Running from DS in %s\n", os.Getenv("DS_PLUGINS_DIR"))
}
```

```yaml
# ds-greeter/plugin.yaml
name: greeter
version: 1.0.0
description: Simple greeting plugin
author: DS Team

platforms:
  - os: linux
    arch: amd64
  - os: darwin
    arch: arm64

commands:
  - name: greet
    description: Greet someone
    usage: ds greeter [name]
```

### Example 2: Artifact Processor

```go
// ds-processor/main.go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/delivery-station/ds/pkg/client"
)

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Println("1.0.0")
        return
    }

    if len(os.Args) < 2 {
        log.Fatal("Usage: ds processor <artifact-ref>")
    }

    artifactRef := os.Args[1]

    // Create DS client
    dsClient, err := client.NewClient()
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer dsClient.Close()

    ctx := context.Background()

    // Pull artifact
    fmt.Printf("Pulling %s...\n", artifactRef)
    err = dsClient.Pull(ctx, artifactRef, os.Stdout)
    if err != nil {
        log.Fatalf("Failed to pull: %v", err)
    }

    // Process artifact (example)
    fmt.Println("Processing artifact...")
    
    // Publish event
    dsClient.PublishEvent(ctx, client.Event{
        Type: "artifact.processed",
        Source: "processor",
        Data: map[string]interface{}{
            "artifact": artifactRef,
            "status": "success",
        },
    })

    fmt.Println("Done!")
}
```

## Best Practices

### 1. Error Handling

- Return appropriate exit codes
- Write errors to stderr
- Provide helpful error messages

```go
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
}
```

### 2. Logging

- Respect `DS_LOGGING_LEVEL`
- Use structured logging
- Don't log sensitive data

```go
logLevel := os.Getenv("DS_LOGGING_LEVEL")
if logLevel == "debug" {
    fmt.Fprintf(os.Stderr, "DEBUG: Connecting to %s\n", registry)
}
```

### 3. Configuration

- Always read from DS environment variables
- Provide sensible defaults
- Validate configuration

```go
cacheDir := os.Getenv("DS_CACHE_DIR")
if cacheDir == "" {
    cacheDir = filepath.Join(os.Getenv("HOME"), ".cache", "ds")
}
```

### 4. Performance

- Cache expensive operations
- Use DS cache for artifacts
- Provide progress feedback

```go
fmt.Printf("Downloading... 0%%\r")
// Download progress
fmt.Printf("Downloading... 100%%\n")
```

### 5. Security

- Validate all inputs
- Don't log credentials
- Use secure defaults
- Respect file permissions

### 6. Compatibility

- Follow semantic versioning
- Maintain backward compatibility
- Document breaking changes
- Test on all supported platforms

## Troubleshooting

### Plugin Not Found

**Problem**: DS can't find your plugin

**Solution**:
- Verify binary name: Must be `ds-<plugin-name>`
- Check location: Should be in `DS_PLUGINS_DIR`
- Verify permissions: Must be executable (`chmod +x`)
- Check manifest: Should be in same directory

### Configuration Not Available

**Problem**: Environment variables are empty

**Solution**:
- Test outside DS first: Set variables manually
- Check DS config: `ds config show`
- Verify DS version: `ds --version`
- Enable debug logging: `ds --log-level debug plugin list`

### Plugin Crashes

**Problem**: Plugin exits with error

**Solution**:
- Test standalone: Run binary directly
- Check dependencies: Verify required files exist
- Review logs: Look for error messages
- Add debug output: Print environment variables

## Resources

- [DS Client Library API](../pkg/client/)
- [Type Definitions](../pkg/types/)
- [Example Plugins](../examples/plugin-template/)
- [Porter Plugin Source](https://github.com/delivery-station/porter)

## Contributing

Found a bug or have a feature request? Please open an issue on GitHub!

Want to share your plugin? Add it to the [Plugin Registry](https://github.com/delivery-station/plugins).
