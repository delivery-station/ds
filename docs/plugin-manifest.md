# Plugin Manifest Specification

This document describes the manifest format for Delivery Station (DS) plugins. The manifest is used to define plugin metadata and support multi-architecture distribution via OCI registries.

## Overview

DS plugins are distributed as OCI artifacts. To support multiple platforms (OS/Architecture) and provide rich metadata, DS uses a manifest format compatible with OCI Image Index (Manifest List), extended with DS-specific fields.

> **Requirement**: The DS CLI expects plugin references to resolve to an OCI index. Each platform that should be installable must appear as a child manifest in the index.

## Manifest Structure

The manifest file (typically `ds.manifest.yaml`) uses YAML format and follows the structure below:

```yaml
artifact-type: application/vnd.delivery-station.plugin.index.v1+json
annotations:
  name: <plugin-name>
  version: <semver>
  description: <description>
  url: <project-url>
  vendor: <vendor-name>
  license: <license>
manifests:
  - platform: <os>[/<arch>][/<variant>][:<os_version>]
    mediaType: <media-type>
    digest: <digest> # Optional in source, required in registry
    size: <size>     # Optional in source, required in registry
```

### Fields

| Field | Description |
|-------|-------------|
| `artifact-type` | Must be `application/vnd.delivery-station.plugin.index.v1+json` |
| `annotations` | Metadata about the plugin (name, version, etc.) |
| `manifests` | List of platform-specific artifacts |

### Platform Format

The `platform` field uses the format: `os[/arch][/variant][:os_version]`

Examples:
- `linux/amd64`
- `linux/arm64`
- `linux/arm/v7`
- `darwin/arm64`
- `windows/amd64:10.0.17763.4974`

This format aligns with [ORAS platform selection](https://oras.land/docs/commands/oras_attach) and allows precise targeting of plugin binaries.

## Multi-Architecture Support

To publish a multi-arch plugin:

1. Build binaries for all target platforms.
2. Create a `ds.manifest.yaml` listing all supported platforms.
3. Use `oras` or the DS CLI to push the artifacts and the index to an OCI registry.

### Example `ds.manifest.yaml`

```yaml
artifact-type: application/vnd.delivery-station.plugin.index.v1+json
annotations:
  name: porter
  version: 0.1.0
  description: Delivery Station Plugin for OCI artifact management
  url: https://github.com/delivery-station/porter
  vendor: Delivery Station Team
  license: MIT
manifests:
  - platform: linux/amd64
    mediaType: application/vnd.delivery-station.plugin.layer.v1+tar+gzip
  - platform: linux/arm64
    mediaType: application/vnd.delivery-station.plugin.layer.v1+tar+gzip
  - platform: darwin/amd64
    mediaType: application/vnd.delivery-station.plugin.layer.v1+tar+gzip
  - platform: darwin/arm64
    mediaType: application/vnd.delivery-station.plugin.layer.v1+tar+gzip
  - platform: windows/amd64
    mediaType: application/vnd.delivery-station.plugin.layer.v1+tar+gzip
```

## References

- [OCI Image Manifest Specification](https://specs.opencontainers.org/image-spec/manifest/)
- [ORAS Documentation](https://oras.land/docs/)
