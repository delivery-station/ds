# DS Configuration Guide

## Configuration Overview

DS uses a layered configuration system that allows you to specify settings through multiple sources. Configuration is loaded with the following precedence (highest to lowest):

1. **CLI Flags** - Command-line arguments
2. **Environment Variables** - Shell environment with `DS_` prefix
3. **Configuration File** - YAML or TOML file
4. **Defaults** - Built-in sensible defaults

## Configuration File

### File Locations

DS searches for configuration files in the following locations (in order):

1. Path specified by `--config` flag
2. `./config.yaml` or `./config.toml` (current directory)
3. Platform-specific config directory:
   - **Linux**: `~/.config/ds/config.yaml`
   - **macOS**: `~/.config/ds/config.yaml`
   - **Windows**: `%APPDATA%\ds\config.yaml`

### File Formats

DS supports both YAML and TOML formats. Examples below use YAML.

### Complete Configuration Reference

```yaml
# Registry Configuration
registry:
  # Default registry for plugin and artifact operations
  # Registry can include an optional namespace prefix (e.g. ghcr.io/delivery-station)
  default: "ghcr.io"
  
  # Alternative registries to try if default fails
  mirrors:
    - "registry.example.com"
    - "mirror.company.net"
  
  # Registries that use HTTP instead of HTTPS (development only!)
  insecure_registries:
    - "localhost:5000"
    - "192.168.1.100:5000"
    # Entries can be provided with or without repository prefixes (e.g. localhost/delivery-station)

# Authentication Configuration
auth:
  # Path to Docker config.json (contains registry credentials)
  docker_config: "~/.docker/config.json"
  
  # Explicit credentials (alternative to docker_config)
  credentials:
    - registry: "ghcr.io"
      username: "myuser"
      password: "${GITHUB_TOKEN}"  # Variable expansion
    - registry: "registry.example.com"
      username: "admin"
      token: "${REGISTRY_TOKEN}"

# Cache Configuration
cache:
  # Cache directory path
  dir: "~/.cache/ds"
  
  # Maximum cache size (supports: B, KB, MB, GB, TB)
  max_size: "10GB"
  
  # Time-to-live for cached artifacts (supports: h, d, w)
  ttl: "7d"
  
  # Clean cache on startup if size exceeds threshold
  auto_clean: true
  
  # Percentage of max_size to trigger cleanup (0.9 = 90%)
  clean_threshold: 0.9

# Logging Configuration
logging:
  # Log level: trace, debug, info, warn, error
  level: "info"
  
  # Log format: text, json
  format: "text"
  
  # Log output: stdout, stderr, or file path
  output: "stdout"
  
  # Enable colored output (auto-detected if terminal supports)
  color: true
  
  # Include caller information in logs
  show_caller: false

# Plugin Configuration
plugins:
  # Plugin directory path
  dir: "~/.config/ds/plugins"
  
  # Plugin sources (registries to search for plugins during manual installation)
  sources:
    - registry: "ghcr.io/delivery-station"
    - registry: "registry.example.com/plugins"
      version_constraint: ">=1.0.0"
  
  # Plugin execution timeout (0 = no timeout)
  timeout: "5m"
  
  # Enable plugin context passing (for inter-plugin communication)
  context_enabled: true

# Proxy Configuration
proxy:
  # HTTP proxy for registry operations
  http_proxy: "${HTTP_PROXY}"
  
  # HTTPS proxy for registry operations
  https_proxy: "${HTTPS_PROXY}"
  
  # Hosts to bypass proxy
  no_proxy:
    - "localhost"
    - "127.0.0.1"
    - "*.internal"

# Communication Configuration (Advanced)
communication:
  # Enable event bus
  event_bus_enabled: true
  
  # Event bus buffer size
  event_buffer_size: 100
  
  # Enable state store
  state_store_enabled: true
  
  # State store TTL
  state_ttl: "1h"
```

## Environment Variables

All configuration values can be set via environment variables using the `DS_` prefix and underscore-separated paths:

### Registry Settings

```bash
export DS_REGISTRY_DEFAULT=ghcr.io
export DS_REGISTRY_MIRRORS_0=registry.example.com
export DS_REGISTRY_MIRRORS_1=mirror.company.net
export DS_REGISTRY_INSECURE_REGISTRIES_0=localhost:5000
```

### Authentication Settings

```bash
export DS_AUTH_DOCKER_CONFIG=~/.docker/config.json
export DS_AUTH_CREDENTIALS_0_REGISTRY=ghcr.io
export DS_AUTH_CREDENTIALS_0_USERNAME=myuser
export DS_AUTH_CREDENTIALS_0_PASSWORD=ghp_token123
```

### Cache Settings

```bash
export DS_CACHE_DIR=~/.cache/ds
export DS_CACHE_MAX_SIZE=10GB
export DS_CACHE_TTL=7d
export DS_CACHE_AUTO_CLEAN=true
export DS_CACHE_CLEAN_THRESHOLD=0.9
```

### Logging Settings

```bash
export DS_LOGGING_LEVEL=debug
export DS_LOGGING_FORMAT=json
export DS_LOGGING_OUTPUT=stdout
export DS_LOGGING_COLOR=true
export DS_LOGGING_SHOW_CALLER=true
```

### Plugin Settings

```bash
export DS_PLUGINS_DIR=~/.config/ds/plugins
export DS_PLUGINS_SOURCES_0_REGISTRY=ghcr.io/delivery-station
export DS_PLUGINS_SOURCES_1_REGISTRY=registry.example.com/plugins
export DS_PLUGINS_SOURCES_1_VERSION_CONSTRAINT=">=1.0.0"
export DS_PLUGINS_TIMEOUT=5m
export DS_PLUGINS_CONTEXT_ENABLED=true
```

### Proxy Settings

```bash
export DS_PROXY_HTTP_PROXY=http://proxy.example.com:8080
export DS_PROXY_HTTPS_PROXY=http://proxy.example.com:8080
export DS_PROXY_NO_PROXY_0=localhost
export DS_PROXY_NO_PROXY_1=*.internal
```

## CLI Flags

Global flags that override all other configuration sources:

```bash
ds --config /path/to/config.yaml    # Custom config file path
ds --log-level debug                # Set log level
ds --plugin-dir /custom/plugins     # Custom plugin directory
ds --no-color                       # Disable colored output
ds --cache-dir /custom/cache        # Custom cache directory
ds --registry ghcr.io               # Default registry
```

## Configuration Examples

### Minimal Configuration

Suitable for getting started quickly:

```yaml
registry:
  default: "ghcr.io"

auth:
  docker_config: "~/.docker/config.json"

cache:
  dir: "~/.cache/ds"
  max_size: "5GB"
  ttl: "7d"

logging:
  level: "info"

plugins:
  dir: "~/.config/ds/plugins"
```

### Production Configuration

Suitable for production deployments:

```yaml
registry:
  default: "registry.company.com"
  mirrors:
    - "registry-mirror-1.company.com"
    - "registry-mirror-2.company.com"

auth:
  docker_config: "~/.docker/config.json"

cache:
  dir: "/var/cache/ds"
  max_size: "50GB"
  ttl: "30d"
  auto_clean: true
  clean_threshold: 0.8

logging:
  level: "warn"
  format: "json"
  output: "/var/log/ds/ds.log"
  show_caller: true

plugins:
  dir: "/usr/local/lib/ds/plugins"
  sources:
    - "registry.company.com/plugins"
  timeout: "10m"

proxy:
  https_proxy: "http://proxy.company.com:3128"
  no_proxy:
    - "localhost"
    - "*.company.com"
```

### Development Configuration

Suitable for local development:

```yaml
registry:
  default: "localhost:5000"
  insecure_registries:
    - "localhost:5000"

auth:
  docker_config: "~/.docker/config.json"

cache:
  dir: "./cache"
  max_size: "1GB"
  ttl: "1d"
  auto_clean: true

logging:
  level: "debug"
  format: "text"
  color: true
  show_caller: true

plugins:
  dir: "./plugins"
  sources:
    - "localhost:5000/plugins"
  timeout: "30s"
```

## Variable Expansion

DS supports environment variable expansion in configuration values using `${VAR}` syntax:

```yaml
auth:
  credentials:
    - registry: "ghcr.io"
      username: "myuser"
      password: "${GITHUB_TOKEN}"

cache:
  dir: "${HOME}/.cache/ds"

proxy:
  http_proxy: "${HTTP_PROXY}"
  https_proxy: "${HTTPS_PROXY}"
```

Default values are supported with `${VAR:-default}` syntax:

```yaml
registry:
  default: "${DS_REGISTRY:-ghcr.io}"

cache:
  max_size: "${DS_CACHE_SIZE:-10GB}"
```

## Configuration Validation

Validate your configuration file before using it:

```bash
ds config validate
```

View the effective configuration (merged from all sources):

```bash
ds config show
```

View configuration in JSON format:

```bash
ds config show --format json
```

## Size Format

Size values support the following units:
- `B` - Bytes
- `KB` - Kilobytes (1000 bytes)
- `MB` - Megabytes (1000 KB)
- `GB` - Gigabytes (1000 MB)
- `TB` - Terabytes (1000 GB)
- `KiB` - Kibibytes (1024 bytes)
- `MiB` - Mebibytes (1024 KiB)
- `GiB` - Gibibytes (1024 MiB)
- `TiB` - Tebibytes (1024 GiB)

Examples:
```yaml
cache:
  max_size: "10GB"      # 10 gigabytes
  max_size: "5GiB"      # 5 gibibytes
  max_size: "512MB"     # 512 megabytes
```

## Duration Format

Duration values support the following units:
- `s` - Seconds
- `m` - Minutes
- `h` - Hours
- `d` - Days
- `w` - Weeks

Examples:
```yaml
cache:
  ttl: "7d"          # 7 days
  ttl: "24h"         # 24 hours
  ttl: "30m"         # 30 minutes

plugins:
  timeout: "5m"      # 5 minutes
  timeout: "300s"    # 300 seconds
```

## Authentication Methods

### Docker Config (Recommended)

Use your existing Docker credentials:

```yaml
auth:
  docker_config: "~/.docker/config.json"
```

Login to a registry using Docker:
```bash
docker login ghcr.io
```

### Explicit Credentials

Specify credentials directly:

```yaml
auth:
  credentials:
    - registry: "ghcr.io"
      username: "myuser"
      password: "${GITHUB_TOKEN}"
    - registry: "registry.example.com"
      username: "admin"
      token: "${REGISTRY_TOKEN}"
```

### Token Authentication

For registries that use bearer tokens:

```yaml
auth:
  credentials:
    - registry: "registry.example.com"
      token: "${BEARER_TOKEN}"
```

## Troubleshooting

### Configuration Not Loaded

**Problem**: DS doesn't seem to use your configuration

**Solutions**:
1. Check file location: `ds config show` displays the loaded config
2. Verify file syntax: `ds config validate`
3. Check file permissions: Config file must be readable
4. Use explicit path: `ds --config /path/to/config.yaml`

### Authentication Failures

**Problem**: Cannot authenticate with registry

**Solutions**:
1. Verify Docker config path: `cat ~/.docker/config.json`
2. Test Docker login: `docker login <registry>`
3. Use explicit credentials in config file
4. Check credential environment variables
5. Enable debug logging: `ds --log-level debug porter fetch <artifact>`

### Cache Issues

**Problem**: Cache not working or filling up

**Solutions**:
1. Check cache directory: `ls -la ~/.cache/ds`
2. Check cache configuration: `ds config show | grep cache`
3. Adjust cache settings in config file or via environment:
   - `DS_CACHE_MAX_SIZE=20GB`
   - `DS_CACHE_TTL=1d`
4. Cache management is handled by plugins (e.g., porter includes cache utilities)
5. Verify cache permissions: `ls -ld ~/.cache/ds`

### Plugin Not Found

**Problem**: DS cannot find installed plugin

**Solutions**:
1. List plugins: `ds plugin list`
2. Check plugin directory: `ls -la ~/.config/ds/plugins`
3. Verify binary name: Must be `ds-<plugin-name>`
4. Check executable permissions: `chmod +x ~/.config/ds/plugins/ds-*`
5. Use explicit plugin dir: `ds --plugin-dir /path/to/plugins plugin list`

### Proxy Issues

**Problem**: Cannot connect to registry through proxy

**Solutions**:
1. Set proxy environment variables:
   ```bash
   export HTTPS_PROXY=http://proxy.example.com:3128
   export NO_PROXY=localhost,127.0.0.1
   ```
2. Configure in config file:
   ```yaml
   proxy:
     https_proxy: "http://proxy.example.com:3128"
     no_proxy: ["localhost", "*.internal"]
   ```
3. Test proxy: `curl -x $HTTPS_PROXY https://ghcr.io`

### Environment Variable Precedence

**Problem**: Environment variables not overriding config file

**Solution**: Environment variables always override config file. Check:
1. Variable name matches format: `DS_SECTION_KEY`
2. Variable is exported: `export DS_REGISTRY_DEFAULT=ghcr.io`
3. View effective config: `ds config show`

## Security Best Practices

1. **Never Commit Credentials**: Use environment variables for sensitive data
2. **Use Docker Config**: Leverage existing credential management
3. **Restrict File Permissions**: `chmod 600 ~/.config/ds/config.yaml`
4. **Avoid Insecure Registries**: Only use HTTP for local development
5. **Rotate Tokens**: Regularly update registry tokens
6. **Use Least Privilege**: Grant minimum required permissions

## Performance Tuning

### Cache Optimization

```yaml
cache:
  max_size: "50GB"           # Large cache for better hit rate
  ttl: "30d"                 # Longer TTL for stable artifacts
  auto_clean: true           # Automatic cleanup
  clean_threshold: 0.8       # Clean when 80% full
```

### Registry Mirrors

```yaml
registry:
  mirrors:
    - "registry-mirror-1.example.com"  # Closer mirror
    - "registry-mirror-2.example.com"  # Fallback mirror
```

### Plugin Timeout

```yaml
plugins:
  timeout: "10m"  # Increase for slow operations
```

## See Also

- [Architecture](architecture.md) - System design and components
- [Plugin Development](plugin-development.md) - Creating custom plugins
- [Examples](../examples/) - Sample configurations
