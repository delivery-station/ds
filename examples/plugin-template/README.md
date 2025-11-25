# DS Plugin Template

This is a template for creating DS plugins in Go.

## Quick Start

1. **Copy this template**:
   ```bash
   cp -r plugin-template my-plugin
   cd my-plugin
   ```

2. **Update metadata**:
   - Edit `plugin.yaml` with your plugin details
   - Update `go.mod` module path
   - Update `main.go` constants (name, version)

3. **Implement your logic**:
   - Add commands in `main.go`
   - Use the DS client library for DS integration

4. **Build**:
   ```bash
   go build -o ds-my-plugin main.go
   ```

5. **Test locally**:
   ```bash
   # Install to plugin directory
   mkdir -p ~/.config/ds/plugins
   cp ds-my-plugin ~/.config/ds/plugins/
   cp plugin.yaml ~/.config/ds/plugins/my-plugin.yaml

   # Test via DS
   ds my-plugin hello
   ds my-plugin process ghcr.io/example/app:v1
   ```

## Structure

```
plugin-template/
├── main.go           # Plugin entry point
├── plugin.yaml       # Plugin manifest
├── go.mod           # Go module definition
├── Makefile         # Build automation
└── README.md        # This file
```

## Development

### Build

```bash
# Build for current platform
go build -o ds-my-plugin main.go

# Build for all platforms
make build-all
```

### Test

```bash
# Run tests
go test -v ./...

# Test plugin directly
./ds-my-plugin --version
./ds-my-plugin hello World

# Test via DS
export DS_REGISTRY_DEFAULT=ghcr.io
export DS_CACHE_DIR=~/.cache/ds
ds my-plugin hello World
```

### Publish

```bash
# Login to registry
docker login ghcr.io

# Push plugin
oras push ghcr.io/yourusername/ds-my-plugin:1.0.0 \
    --artifact-type application/vnd.ds.plugin.v1 \
    ds-my-plugin:application/octet-stream \
    plugin.yaml:application/yaml
```

## Features

- ✅ Version flag (`--version`)
- ✅ Help flag (`--help`)
- ✅ DS client integration
- ✅ Configuration from environment
- ✅ Event publishing
- ✅ State management
- ✅ Error handling
- ✅ Logging support

## Customization

### Add a New Command

1. Add command handler function:
   ```go
   func handleMyCommand(ctx context.Context, dsClient *client.Client, args []string) {
       // Your logic here
   }
   ```

2. Add case in switch statement:
   ```go
   case "mycommand":
       handleMyCommand(ctx, dsClient, args)
   ```

3. Update `plugin.yaml`:
   ```yaml
   commands:
     - name: mycommand
       description: My new command
       usage: ds my-plugin mycommand <args>
   ```

### Use External Dependencies

```bash
# Add dependency
go get github.com/some/package

# Update go.mod
go mod tidy
```

### Add Configuration

1. Update `plugin.yaml`:
   ```yaml
   config:
     - name: MY_SETTING
       env: MY_PLUGIN_SETTING
       description: My setting
       required: true
   ```

2. Read in code:
   ```go
   setting := os.Getenv("MY_PLUGIN_SETTING")
   ```

## Best Practices

1. **Always validate inputs**:
   ```go
   if len(args) < 1 {
       fmt.Fprintf(os.Stderr, "Error: Missing required argument\n")
       os.Exit(1)
   }
   ```

2. **Handle errors gracefully**:
   ```go
   if err != nil {
       log.Fatalf("Error: %v\n", err)
   }
   ```

3. **Respect log levels**:
   ```go
   logLevel := os.Getenv("DS_LOGGING_LEVEL")
   if logLevel == "debug" {
       fmt.Println("Debug info...")
   }
   ```

4. **Use DS cache**:
   ```go
   err := dsClient.Pull(ctx, artifactRef, os.Stdout)
   ```

5. **Publish events**:
   ```go
   dsClient.PublishEvent(ctx, client.Event{
       Type: "my-plugin.event",
       Source: "my-plugin",
       Data: map[string]interface{}{"status": "success"},
   })
   ```

## Troubleshooting

**Plugin not found**:
- Check binary name: `ds-my-plugin`
- Check location: `~/.config/ds/plugins/`
- Check permissions: `chmod +x ds-my-plugin`

**Configuration not available**:
- Test outside DS: `export DS_REGISTRY_DEFAULT=ghcr.io && ./ds-my-plugin`
- Check DS config: `ds config show`

**Build errors**:
- Update dependencies: `go mod tidy`
- Check Go version: `go version` (need 1.21+)

## Resources

- [DS Documentation](../../docs/)
- [Plugin Development Guide](../../docs/plugin-development.md)
- [DS Client API](../../pkg/client/)
- [Example Plugins](https://github.com/delivery-station/plugins)

## License

Apache License 2.0 - see LICENSE file
