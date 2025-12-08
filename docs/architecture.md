# DS Architecture

## Overview

Delivery Station (DS) is a meta-application framework that provides plugin management, configuration, caching, and inter-plugin communication for OCI artifact delivery tools. It follows a hub-and-spoke model where DS acts as the central coordinator for specialized plugins.

## Design Principles

1. **Plugin-First Architecture**: Core DS provides infrastructure; plugins provide functionality
2. **Configuration Inheritance**: Plugins automatically inherit DS configuration
3. **Loose Coupling**: Plugins are independent binaries, not Go packages
4. **OCI-Native**: Everything is an OCI artifact (plugins, bundles, manifests)
5. **Zero Trust**: No automatic plugin installation from untrusted sources

## System Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      User Interface                      │
│                    $ ds <command>                        │
└───────────────────────────┬──────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────┐
│                     DS Core Process                      │
├──────────────────────────────────────────────────────────┤
│  ┌────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   CLI      │  │  Plugin     │  │  Configuration  │  │
│  │  Commands  │  │  Manager    │  │   Loader        │  │
│  └────────────┘  └─────────────┘  └─────────────────┘  │
│                                                          │
│  ┌────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   OCI      │  │   Cache     │  │ Communication   │  │
│  │  Registry  │  │   System    │  │   Layer         │  │
│  └────────────┘  └─────────────┘  └─────────────────┘  │
└───────────────────────────┬──────────────────────────────┘
                            │ (subprocess spawn)
            ┌───────────────┼───────────────┐
            │               │               │
    ┌───────▼─────┐ ┌──────▼──────┐ ┌─────▼──────┐
    │   Plugin 1  │ │   Plugin 2  │ │  Plugin N  │
    │  (porter)   │ │ (s3-upload) │ │   (...)    │
    └─────────────┘ └─────────────┘ └────────────┘
```

## Core Components

### 1. CLI Layer (`internal/cmd/`)

**Purpose**: Provides the user-facing command interface

**Components**:
- Root command with global flags
- Built-in commands (plugin, config, version)
- Command delegation to plugins for all artifact operations
- Output formatting and error handling

**Key Design Decisions**:
- Uses Cobra for CLI framework (industry standard)
- Global flags available to all commands
- Unknown commands automatically delegated to plugins
- Colored output with `--no-color` override
- All artifact operations (pull, push, list, cache) are handled by plugins

### 2. Configuration System (`internal/config/`)

**Purpose**: Manages configuration from multiple sources with proper precedence

**Components**:
- Config loader (YAML/TOML support)
- Environment variable parser (DS_* prefix)
- CLI flag override handler
- Validation and defaults engine

**Configuration Precedence** (highest to lowest):
1. CLI flags (`--flag=value`)
2. Environment variables (`DS_*`)
3. Config file (`~/.config/ds/config.yaml`)
4. Built-in defaults

**Key Design Decisions**:
- Uses Viper for configuration management
- Cross-platform config paths (XDG on Linux, AppData on Windows)
- Support for both YAML and TOML formats
- Variable expansion (`${VAR}` syntax)

### 3. Plugin System (`internal/plugin/`)

**Purpose**: Discovers, manages, and executes plugin binaries

**Components**:
- **Manager**: Plugin discovery and listing
- **Executor**: Subprocess execution with config passing
- **Installer**: Downloads and installs plugins from OCI registries
- **Registry**: Tracks installed plugins and versions

**Plugin Contract**:
```
Binary naming: ds-<plugin-name>
Manifest: plugin.yaml in same directory
Configuration: Via Host Config gRPC service
Invocation: ds <plugin-name> <args>
```

**Key Design Decisions**:
- Plugins are separate binaries, not in-process
- Binary naming convention (`ds-*`) enables auto-discovery
- Configuration delivered through Host Config service (no plugin-side config parsing)
- Plugins distributed as OCI artifacts
- No automatic installation (security)

### 4. OCI Registry Client (`internal/registry/`)

**Purpose**: Handles all OCI registry operations

**Components**:
- Registry client (pull/push/list)
- Authentication handler (Docker config, explicit credentials)
- Manifest parser
- Multi-registry support

**Key Design Decisions**:
- Uses ORAS library (OCI standard implementation)
- Docker config.json compatibility
- Support for insecure registries (development)
- Automatic retry with exponential backoff

### 5. Cache System (`internal/cache/`)

**Purpose**: Provides shared artifact caching for plugins

**Components**:
- **Cache Manager**: High-level cache operations
- **Store**: Persistent metadata storage
- **Eviction Engine**: TTL and size-based cleanup

**Cache Structure**:
```
~/.cache/ds/
├── artifacts/
│   ├── <hash1>/
│   │   ├── blob
│   │   └── metadata.json
│   └── <hash2>/
│       ├── blob
│       └── metadata.json
└── index.json
```

**Eviction Policies**:
- TTL-based: Remove after configured time
- Size-based: LRU eviction when cache exceeds max size
- Manual: Cache management delegated to plugins

**Key Design Decisions**:
- Content-addressable storage (hash-based keys)
- Metadata separate from blobs
- Concurrent access protection (file locking)
- Lazy cleanup (on-demand and periodic)

### 6. Communication Layer (`internal/communication/`)

**Purpose**: Enables inter-plugin communication and state sharing

**Components**:
- **Event Bus**: Pub/sub messaging between plugins
- **State Store**: Key-value store with TTL support
- **Plugin Registry**: Runtime plugin discovery

**Event Flow**:
```
Plugin A → Publish Event → Event Bus → Subscribe → Plugin B
```

**Key Design Decisions**:
- In-memory event bus (no external dependencies)
- Buffered channels (100 events) to prevent blocking
- State store supports TTL for temporary data
- Events are fire-and-forget (no ACK required)

### 7. Public API (`pkg/client/`, `pkg/types/`)

**Purpose**: Provides Go library for plugin developers

**Components**:
- DS Client: High-level API for plugins
- Type definitions: Shared structs and interfaces
- Functional options pattern for flexibility

**Example Usage**:
```go
client, err := client.NewClient(
    client.WithConfig(cfg),
    client.WithLogger(logger),
)
defer client.Close()

err = client.Pull(ctx, "ghcr.io/org/app:v1.0.0", os.Stdout)
```

## Data Flow

### Plugin Installation Flow

```
User: ds plugin install porter
  │
  ├─> Parse plugin reference (name, version, registry)
  ├─> Resolve platform (os/arch)
  ├─> Authenticate with registry
  ├─> Pull plugin artifact
  │   └─> Contains: binary, plugin.yaml manifest
  ├─> Verify checksums
  ├─> Extract to plugin directory
  ├─> Set executable permissions
  └─> Register in plugin index
```

### Plugin Execution Flow

```
User: ds porter fetch ghcr.io/org/app:v1
  │
  ├─> Discover plugins (scan ~/.config/ds/plugins/)
  ├─> Find "porter" plugin (ds-porter binary)
  ├─> Prepare execution context
  │   ├─> Load DS configuration
  │   ├─> Expose Host Config gRPC service via go-plugin broker
  │   └─> Set plugin context (JSON via stdin)
  ├─> Execute: exec.Command("ds-porter", "fetch", "ghcr.io/org/app:v1")
  ├─> Stream stdout/stderr to terminal
  └─> Return exit code
```

### Artifact Pull Flow (via Plugin)

```
User: ds porter fetch ghcr.io/org/app:v1
  │
  ├─> DS delegates to porter plugin
  ├─> Porter uses DS client library
  │   ├─> Check cache
  │   │   └─> Hit? Return cached artifact
  │   ├─> Authenticate with registry
  │   ├─> Pull manifest
  │   ├─> Pull layers
  │   └─> Store in cache
  │       ├─> Generate hash key
  │       ├─> Write blob to ~/.cache/ds/artifacts/<hash>/blob
  │       ├─> Write metadata
  │       └─> Update cache index
  └─> Return artifact
```

### Inter-Plugin Communication Flow

```
Porter Plugin: Need to upload to S3
  │
  ├─> Use DS client library
  ├─> Call: client.CallPlugin("s3-uploader", args)
  ├─> DS spawns s3-uploader subprocess
  │   └─> Shares Host Config provider via broker
  ├─> Pass data via stdin (JSON)
  ├─> Capture s3-uploader output
  └─> Return result to porter
```

## Design Decisions & Rationale

### Why Plugins as Binaries?

**Alternatives Considered**:
- In-process plugins (Go packages)
- Shared libraries (.so/.dll)
- Remote plugins (gRPC)

**Decision**: Separate binary processes

**Rationale**:
- **Language Independence**: Plugins can be written in any language
- **Isolation**: Plugin crashes don't crash DS core
- **Distribution**: OCI artifacts naturally contain binaries
- **Versioning**: Independent plugin versions
- **Security**: Process isolation

**Trade-offs**:
- Slower than in-process (subprocess overhead)
- More memory (separate processes)
- Inter-process communication overhead

### How Plugins Receive Configuration

**Approach**: Host Config gRPC service provided through the go-plugin broker for every invocation.

**Rationale**:
- **Typed data**: Host Config returns the full structured configuration after precedence resolution.
- **Future proof**: gRPC service can evolve without forcing plugins to parse new formats.
- **Single source of truth**: Eliminates divergent environment-derived overrides.
- **Secure handling**: Credentials never touch environment variables.

**Trade-offs**:
- Plugins must obtain the broker reference and invoke the Host Config service.
- Non-Go plugins need a lightweight gRPC client to consume the service.

### Why Content-Addressable Cache?

**Alternatives Considered**:
- Simple tag-based cache (registry/repo:tag)
- Digest-only cache
- No cache (always fetch)

**Decision**: Content-addressable with hash keys

**Rationale**:
- **Deduplication**: Same artifact, different tags = one cache entry
- **Integrity**: Hash verification built-in
- **Immutability**: Cache entries never change
- **Garbage Collection**: Can safely delete unused entries

**Trade-offs**:
- More complex key generation
- Requires manifest inspection

### Why In-Memory Event Bus?

**Alternatives Considered**:
- External message broker (Redis, RabbitMQ)
- File-based IPC
- Unix sockets

**Decision**: In-memory pub/sub

**Rationale**:
- **Zero Dependencies**: No external services needed
- **Simplicity**: Easy to understand and debug
- **Performance**: Minimal latency
- **Scope**: Events only needed during single DS invocation

**Trade-offs**:
- Not persistent (events lost on exit)
- Limited to single DS process
- No guaranteed delivery

## Security Considerations

### Plugin Security

1. **No Auto-Install**: Users must explicitly install plugins
2. **Verification**: Checksums verified on install
3. **Process Isolation**: Plugins run in separate processes
4. **Limited Privileges**: Plugins inherit user permissions

### Authentication Security

1. **Docker Config**: Reuse existing credentials
2. **No Plain Text**: Credentials not logged or printed
3. **Token Refresh**: Automatic token rotation
4. **Secure Storage**: User-scoped file permissions

### Cache Security

1. **User Scoped**: Cache directory owned by user
2. **No Shared Cache**: Each user has separate cache
3. **Integrity Checks**: Hash verification on read

## Performance Characteristics

### Time Complexity

- Plugin Discovery: O(n) where n = number of plugin binaries
- Cache Lookup: O(1) hash table
- LRU Eviction: O(log n) priority queue
- Event Publishing: O(m) where m = number of subscribers

### Space Complexity

- Cache: O(artifacts × average_size)
- Event Bus: O(buffered_events) = O(100)
- Plugin Registry: O(plugins)

### Bottlenecks

1. **Network I/O**: Registry operations are slow
   - Mitigation: Aggressive caching, concurrent pulls
2. **Subprocess Spawn**: ~10ms overhead per plugin call
   - Mitigation: Acceptable for typical use cases
3. **File I/O**: Cache reads/writes
   - Mitigation: Lazy loading, async writes

## Future Architecture Improvements

### Planned Enhancements

1. **Persistent Event Log**: For debugging and audit
2. **Plugin Sandboxing**: Limit plugin capabilities
3. **Distributed Cache**: Shared team cache
4. **Plugin Marketplace**: Curated plugin registry
5. **Streaming APIs**: For large artifacts

### Non-Goals

- **In-Process Plugins**: Intentionally separate
- **Plugin API Versioning**: Binary interface is stable
- **Hot Reload**: Restart required for plugin updates

## Testing Strategy

### Unit Tests

- Each component tested in isolation
- Mock external dependencies (registry, filesystem)
- Coverage target: >80%

### Integration Tests

- End-to-end workflows
- Real plugin execution
- Mock registry server

### Performance Tests

- Cache performance under load
- Plugin execution overhead
- Memory usage profiling

## Observability

### Logging

- Structured logging (JSON option)
- Log levels: debug, info, warn, error
- Context propagation to plugins

### Metrics

- Cache hit rate
- Plugin execution time
- Registry operation latency

### Tracing

- Request tracing across plugin calls
- Trace ID propagation via DS_TRACE_ID

## Migration Path

### From Manual Plugin Management

1. Install DS: `make install`
2. Install plugins: `ds plugin install <name>`
3. Update scripts: `porter` → `ds porter`
4. Plugins automatically use DS infrastructure (cache, config, registry client)

### Adoption Strategy

1. Configure DS with registry credentials and cache settings
2. Install required plugins: `ds plugin install porter`
3. Plugins automatically benefit from shared cache and configuration
4. No changes needed to plugin invocation - DS delegates transparently

## Conclusion

DS provides a solid plugin management and infrastructure foundation for OCI artifact tools. As a pure wrapper/manager, it focuses on doing one thing well: discovering, managing, and executing plugins while providing shared infrastructure (cache, config, registry client, communication). The architecture balances simplicity and flexibility while maintaining clear separation of concerns - DS manages infrastructure, plugins handle functionality. This plugin model enables community contributions without compromising core stability.
