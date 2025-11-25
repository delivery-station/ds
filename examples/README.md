# DS Configuration Examples

This directory contains example configuration files and templates for DS (Delivery Station).

## Configuration Files

### [minimal-config.yaml](minimal-config.yaml)

The simplest working configuration with sensible defaults. Perfect for getting started quickly.

**Use case**: Development, personal projects, trying out DS

```bash
ds --config examples/minimal-config.yaml version
```

### [config.yaml](config.yaml)

A comprehensive example showing all available configuration options with explanations. Use this as a reference for understanding what can be configured.

**Use case**: Understanding all available options, customizing your setup

### [production-config.yaml](production-config.yaml)

Optimized configuration for production deployments with:
- Larger cache sizes
- JSON logging for aggregation
- Stricter plugin verification
- Enterprise registry mirrors
- Proxy configuration

**Use case**: Production deployments, enterprise environments

## Plugin Template

### [plugin-template/](plugin-template/)

A complete template for creating DS plugins in Go. Includes:
- Basic plugin structure
- Configuration handling via environment variables
- Example commands
- Build configuration
- Documentation

**Getting started with plugin development**:

```bash
# Copy the template
cp -r examples/plugin-template my-plugin
cd my-plugin

# Customize for your use case
# Edit plugin.yaml with your plugin details
# Implement your functionality in main.go

# Build
make build

# Test locally
./ds-my-plugin --help
```

See [plugin-template/README.md](plugin-template/README.md) for detailed instructions.

## Helper Scripts

### [sign-porter-plugin.sh](sign-porter-plugin.sh)

Example script demonstrating how to sign a plugin binary for verification. This is useful when distributing plugins to ensure authenticity and integrity.

```bash
# Generate a key pair (first time only)
openssl genrsa -out private-key.pem 2048
openssl rsa -in private-key.pem -pubout -out public-key.pem

# Sign a plugin
./examples/sign-porter-plugin.sh path/to/plugin-binary private-key.pem

# Configure DS to verify signatures
# Add to your config.yaml:
# plugins:
#   signature:
#     mode: strict
#     public_keys:
#       - path/to/public-key.pem
```

## Using Example Configs

### Option 1: Use directly with --config flag

```bash
ds --config examples/minimal-config.yaml plugin list
```

### Option 2: Copy to default location

**Linux/macOS**:
```bash
mkdir -p ~/.config/ds
cp examples/config.yaml ~/.config/ds/config.yaml
```

**Windows**:
```powershell
mkdir $env:APPDATA\ds
copy examples\config.yaml $env:APPDATA\ds\config.yaml
```

### Option 3: Copy to project directory

```bash
cp examples/minimal-config.yaml ./config.yaml
# DS will automatically detect config.yaml in the current directory
```

## Configuration Precedence

DS loads configuration in this order (later sources override earlier ones):

1. Built-in defaults
2. Config file (`~/.config/ds/config.yaml` or `./config.yaml`)
3. Environment variables (prefixed with `DS_`)
4. Command-line flags

Example:
```bash
# Override registry via environment variable
export DS_REGISTRY_DEFAULT=registry.example.com
ds plugin list

# Override via command-line flag
ds --log-level=debug plugin list
```

## Environment Variables

All config values can be set via environment variables:

```bash
export DS_REGISTRY_DEFAULT=ghcr.io
export DS_CACHE_DIR=~/.cache/ds
export DS_CACHE_MAX_SIZE=10GB
export DS_LOGGING_LEVEL=debug
export DS_PLUGINS_DIR=~/.config/ds/plugins
```

## Common Configuration Patterns

### Development Setup
- Use `minimal-config.yaml`
- Set log level to `debug`
- Smaller cache size
- Allow unsigned plugins

### CI/CD Environment
- Explicit credentials (no docker config)
- JSON logging
- Disable colored output
- Fixed plugin versions

### Production Setup
- Use `production-config.yaml`
- JSON logging with file output
- Strict plugin signature verification
- Large cache with auto-cleanup
- Registry mirrors for reliability
- Proxy configuration if needed

## Getting Help

- Full documentation: [../docs/configuration.md](../docs/configuration.md)
- Plugin development: [../docs/plugin-development.md](../docs/plugin-development.md)
- Issues: https://github.com/delivery-station/ds/issues
