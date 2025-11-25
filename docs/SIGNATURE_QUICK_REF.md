# Plugin Signature Quick Reference

## For Plugin Developers

### Generate Keys (One Time)
```bash
# Generate 4096-bit RSA keys
ds plugin sign --generate --key-size 4096 ./ds-myplugin
```

Creates:
- `plugin-private.pem` - **KEEP SECRET!**
- `plugin-public.pem` - Share with users

### Sign Plugin
```bash
ds plugin sign --key ./plugin-private.pem ./ds-myplugin
```

Creates:
- `ds-myplugin.sig` - Signature file

### Distribute
Include in your release:
1. `ds-myplugin` (binary)
2. `ds-myplugin.sig` (signature)
3. `plugin-public.pem` (public key)

### Publish to OCI
```bash
# Push plugin with signature
oras push ghcr.io/org/plugin:1.0.0-linux-amd64 \
  ./ds-myplugin:application/vnd.ds.plugin.binary.v1 \
  ./ds-myplugin.sig:application/vnd.ds.plugin.signature.v1 \
  ./plugin.yaml:application/vnd.ds.plugin.manifest.v1
```

## For Users

### Install Public Key
```bash
# Create trust directory
mkdir -p ~/.config/ds/trust

# Copy public key
cp plugin-public.pem ~/.config/ds/trust/myplugin.pem
```

### Configure Verification

**Strict Mode** (Recommended for Production):
```yaml
# ~/.config/ds/config.yaml
plugins:
  signature:
    mode: strict
    trust_store: ~/.config/ds/trust
```

**Permissive Mode** (Development):
```yaml
plugins:
  signature:
    mode: permissive
    allow_unsigned: true
    trust_store: ~/.config/ds/trust
```

### Install Plugin
```bash
# Automatic verification during install
ds plugin install myplugin@1.0.0
```

### Manual Verification
```bash
# Verify installed plugin
ds plugin verify ~/.config/ds/plugins/ds-myplugin

# Check plugin info
ds plugin info myplugin
```

## Configuration Modes

| Mode | Signed | Unsigned | Invalid Signature |
|------|--------|----------|-------------------|
| **strict** | ✅ Allow | ❌ Reject | ❌ Reject |
| **permissive** | ✅ Allow | ⚠️ Warn | ⚠️ Warn |
| **disabled** | ✅ Allow | ✅ Allow | ✅ Allow |

## Environment Variables

Override configuration via environment:
```bash
# Set mode
export DS_PLUGINS_SIGNATURE_MODE=strict

# Allow unsigned in permissive mode
export DS_PLUGINS_SIGNATURE_ALLOW_UNSIGNED=false

# Set trust store
export DS_PLUGINS_SIGNATURE_TRUST_STORE=~/.config/ds/trust
```

## Common Commands

```bash
# Sign plugin with existing key
ds plugin sign --key private.pem ./ds-plugin

# Sign and generate keys
ds plugin sign --generate ./ds-plugin

# Verify signature
ds plugin verify ./ds-plugin

# Install with strict verification
DS_PLUGINS_SIGNATURE_MODE=strict ds plugin install myplugin

# List plugins with signature status
ds plugin list

# Show detailed plugin info
ds plugin info myplugin
```

## Troubleshooting

### "Plugin is not signed (strict mode requires signatures)"
- Switch to permissive mode, or
- Obtain signed version of plugin

### "Signature verification failed"
- Redownload plugin and signature
- Verify correct public key
- Check if binary was modified

### "Strict mode requires at least one public key"
- Add public keys to `trust_store` directory
- Or specify in `public_keys` list

## Security Tips

**For Developers:**
- Never commit private keys
- Use 4096-bit keys minimum
- Rotate keys periodically
- Document public key fingerprints

**For Users:**
- Use strict mode in production
- Verify public key fingerprints
- Download keys from official sources
- Monitor signature warnings

## Learn More

- [Full Documentation](./PLUGIN_SIGNING.md)
- [Example Script](../examples/sign-porter-plugin.sh)
- [Configuration Example](../examples/config.yaml)
