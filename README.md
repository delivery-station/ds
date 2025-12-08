# DS (Delivery Station)

[![Go Version](https://img.shields.io/github/go-mod/go-version/delivery-station/ds)](https://go.dev/)
[![License](https://img.shields.io/github/license/delivery-station/ds)](LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/delivery-station/ds)](https://github.com/delivery-station/ds/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/delivery-station/ds)](https://goreportcard.com/report/github.com/delivery-station/ds)

A plugin-based CLI meta-application that serves as a **wrapper and infrastructure manager** for OCI artifact tools.

## Overview

DS is a **plugin manager and infrastructure provider** - it does not directly handle artifacts. Instead, it:

1. **Manages plugins**: Discovers, installs, updates, and executes plugin binaries
2. **Provides shared infrastructure**: Configuration, cache, registry client, event bus
3. **Delegates operations**: All artifact operations (pull, push, list, cache management) are handled by plugins


## Features

- 🔌 **Plugin Management**: Install, update, sign, and manage plugins from OCI registries
- ⚙️ **Unified Configuration**: Share configuration across all plugins via YAML, environment variables, or CLI flags
- 🔐 **Authentication**: Support for Docker config.json and explicit credentials (available to plugins)
- 💾 **Shared Cache**: Artifact caching infrastructure accessible by all plugins
- 🔗 **Inter-Plugin Communication**: Event bus and state store for plugin coordination
- 🌐 **Cross-Platform**: Support for Linux, macOS, and Windows
- 📦 **OCI Registry Client**: Shared registry client library for plugins

**Note**: DS itself does not perform artifact operations (pull, push, list, etc.). These are delegated to plugins.

## Installation

### Download Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/delivery-station/ds/releases):


### Build from Source

Requirements:
- Go 1.25 or later
- Make

```bash
# Clone the repository
git clone https://github.com/delivery-station/ds.git
cd ds

# Build
make build

# Install to $GOPATH/bin
make install
```

## Quick Start

### Basic Usage

```bash
# Check version
ds version

# View help
ds --help

# Show configuration
ds config show
ds config validate

# Plugin management
ds plugin list
ds plugin info porter
ds plugin install porter
ds plugin update porter
ds plugin sign <binary>

> DS installs plugins from multi-architecture OCI indexes. Ensure the published reference includes manifests for each supported OS/architecture.

# Use a plugin (artifact operations delegated to plugins)
ds porter fetch ghcr.io/org/app:v1.0.0
ds porter deliver ghcr.io/org/app:v1.0.0
```

## Configuration

DS supports multiple configuration sources with the following precedence:
**CLI Flags > Environment Variables > Config File > Defaults**

### Config File Locations

- **Linux/macOS**: `~/.config/ds/config.yaml`
- **Windows**: `%APPDATA%\ds\config.yaml`
- **Project-local**: `./config.yaml`

### Example Configuration

```yaml
# Registry configuration
registry:
  # The default entry can include a namespace (e.g. ghcr.io/delivery-station)
  default: "ghcr.io/delivery-station"
  mirrors:
    - "registry.example.com"
  insecure_registries: []

# Authentication
auth:
  docker_config: "~/.docker/config.json"

# Cache settings
cache:
  dir: "~/.cache/ds"
  max_size: "10GB"
  ttl: "7d"

# Logging
logging:
  level: "info"
  format: "text"

# Plugin management
plugins:
  dir: "~/.config/ds/plugins"
  sources:
    - registry: "ghcr.io/delivery-station"
```

### Environment Variables

All configuration values can be set via environment variables with the `DS_` prefix:

```bash
export DS_REGISTRY_DEFAULT=ghcr.io/delivery-station
export DS_AUTH_DOCKER_CONFIG=~/.docker/config.json
export DS_CACHE_DIR=~/.cache/ds
export DS_LOGGING_LEVEL=debug
```

### Global Flags

```bash
--config string       Config file path (default: ~/.config/ds/config.yaml)
--log-level string    Log level: debug, info, warn, error (default: info)
--plugin-dir string   Plugin directory (default: ~/.config/ds/plugins)
--no-color            Disable colored output
```

## Development

### Prerequisites

- Go 1.21 or later
- Make

### Building

```bash
# Download dependencies
make deps

# Build the binary
make build

# Run tests
make test

# Run linters
make lint

# Build with coverage
make test-coverage

# Clean build artifacts
make clean
```

### Project Structure

```
ds/
├── cmd/
│   └── ds/              # Main entry point
├── internal/
│   ├── cmd/             # Command implementations
│   ├── config/          # Configuration management (Agent 2)
│   ├── plugin/          # Plugin management (Agent 3-4, 6)
│   ├── registry/        # OCI registry client (Agent 5)
│   └── cache/           # Artifact caching (Agent 7)
├── pkg/
│   ├── types/           # Shared types
│   └── client/          # DS client library for plugins (Agent 8)
├── docs/                # Documentation (Agent 10)
├── Makefile
├── go.mod
└── README.md
```

## Architecture

DS is a **wrapper/manager** that provides infrastructure for plugins:

```
┌─────────────────────────────────────────┐
│              User CLI                   │
│     $ ds porter fetch <artifact>        │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│         DS Core (Wrapper/Manager)       │
│  ✓ Plugin Discovery & Execution         │
│  ✓ Configuration Management             │
│  ✓ Shared Cache Infrastructure          │
│  ✓ OCI Registry Client (for plugins)    │
│  ✓ Inter-Plugin Communication           │
│  ✗ No built-in artifact operations      │
└──────────────────┬──────────────────────┘
                   │ (delegates to plugin)
┌──────────────────▼──────────────────────┐
│         Plugin: Porter (Go)             │
│  Uses DS Client Library                 │
│  Fetch → Analyze → Deliver              │
└─────────────────────────────────────────┘
```

**Key Point**: DS provides the infrastructure (config, cache, registry client) but delegates all artifact operations to plugins.

## Plugin Development

Plugins are independent binaries that follow a simple contract:

1. **Naming**: Binary must be named `ds-<plugin-name>` (e.g., `ds-porter`)
2. **Configuration**: Read from `DS_*` environment variables
3. **Inter-plugin calls**: Use `ds <other-plugin> <command>` to call other plugins

See the [Plugin Development Guide](docs/plugin-development.md) and [plugin template](examples/plugin-template/) for detailed instructions.

## Documentation

- [Architecture Documentation](docs/architecture.md) - Detailed system design and components
- [Configuration Guide](docs/configuration.md) - Complete configuration reference
- [Plugin Development Guide](docs/plugin-development.md) - How to create plugins
- [Plugin Signing Guide](docs/PLUGIN_SIGNING.md) - Security and plugin verification
- [Contributing Guidelines](CONTRIBUTING.md) - How to contribute to the project
- [Code of Conduct](CODE_OF_CONDUCT.md) - Community guidelines
- [Security Policy](SECURITY.md) - Reporting security vulnerabilities

## Contributing

We welcome contributions! Here's how you can help:

1. **Report Bugs**: Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.yml)
2. **Request Features**: Use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.yml)
3. **Submit Pull Requests**: See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines
4. **Improve Documentation**: Documentation improvements are always appreciated
5. **Create Plugins**: Extend DS functionality by creating new plugins

Please read our [Code of Conduct](CODE_OF_CONDUCT.md) before contributing.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: Report bugs at [GitHub Issues](https://github.com/delivery-station/ds/issues)
- **Discussions**: Ask questions at [GitHub Discussions](https://github.com/delivery-station/ds/discussions)
- **Security**: Report vulnerabilities via [Security Advisories](https://github.com/delivery-station/ds/security/advisories)

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [ORAS Go](https://oras.land/oras-go/) - OCI registry client library

