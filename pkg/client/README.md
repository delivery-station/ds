# Delivery Station Public API Client

The `client` package provides the high-level public API for interacting with the Delivery Station system.

## Overview

The Client provides a unified interface for **plugins** to:
- Pull and push OCI artifacts from registries (for plugin use)
- Access shared cache and configuration
- Manage shared state between plugins
- Publish and subscribe to events
- Access the underlying DS subsystems

**Note**: This is a library for plugin developers. End users interact with DS through plugins, not directly with these APIs.

## Usage

### Creating a Client

```go
import "github.com/delivery-station/ds/pkg/client"

// Create client with default configuration
client, err := client.NewClient()
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Or with custom configuration
cfg := &types.Config{
    Cache: types.CacheConfig{
        Dir:     "/path/to/cache",
        MaxSize: 1024 * 1024 * 100, // 100MB
        TTL:     time.Hour,
    },
    Plugins: types.PluginsConfig{
        Dir: "/path/to/plugins",
    },
    Registry: types.RegistryConfig{
        Default: "ghcr.io/delivery-station",
    },
}

client, err := client.NewClient(client.WithConfig(cfg))
```

### Artifact Operations

```go
ctx := context.Background()

// Pull an artifact
err := client.Pull(ctx, "ghcr.io/owner/repo:tag", outputWriter)

// Push an artifact
err := client.Push(ctx, "ghcr.io/owner/repo:tag", inputReader, "application/octet-stream")

// List tags
tags, err := client.List(ctx, "ghcr.io/owner/repo")
```

### Plugin Management

```go
// Install a plugin
err := client.InstallPlugin(ctx, "plugin-name", "1.0.0")

// List installed plugins
plugins, err := client.ListPlugins()

// Execute a plugin
exitCode, err := client.ExecutePlugin("plugin-name", []string{"arg1", "arg2"})
```

### State Management

```go
ctx := context.Background()

// Set state
state := map[string]interface{}{
    "key": "value",
}
err := client.SetState(ctx, "state-key", "plugin-id", state, nil)

// Get state
value, err := client.GetState(ctx, "state-key")
```

### Event Bus

```go
// Subscribe to events
handler := func(ctx context.Context, event *communication.Event) error {
    fmt.Printf("Received event: %s\n", event.Type)
    return nil
}
client.Subscribe(communication.EventCustom, handler)

// Publish events
data := map[string]interface{}{"message": "hello"}
err := client.Publish(ctx, communication.EventCustom, "plugin-id", data)
```

### Accessing Subsystems

```go
// Access underlying components
config := client.Config()
cache := client.Cache()
registry := client.Registry()
pluginMgr := client.PluginManager()
eventBus := client.EventBus()
stateStore := client.StateStore()
pluginRegistry := client.PluginRegistry()
```

## Features

- **Simple API**: High-level, easy-to-use interface
- **Flexible Configuration**: Support for custom configurations and dependency injection
- **Integrated Subsystems**: Unified access to cache, registry, plugins, and communication
- **Extensible**: Access to underlying components for advanced use cases
- **Well-Tested**: Comprehensive test coverage

## Architecture

The Client orchestrates several internal subsystems:

1. **Config**: Configuration management and loading
2. **Cache**: Artifact caching layer
3. **Registry**: OCI registry client
4. **Plugin Manager**: Plugin lifecycle management
5. **Event Bus**: Event-driven communication
6. **State Store**: Shared state management
7. **Plugin Registry**: Plugin discovery and registration

## Best Practices

1. Always use `defer client.Close()` to ensure proper cleanup
2. Pass context for cancellation and timeouts
3. Use the functional options pattern for custom configuration
4. Check errors from all operations
5. Use the high-level Client API rather than directly accessing subsystems

## Testing

Run the client tests:

```bash
go test ./pkg/client/... -v
```

## Examples

See the test file `client_test.go` for comprehensive usage examples.
