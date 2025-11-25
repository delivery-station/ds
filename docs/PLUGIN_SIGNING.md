# Plugin Signature Verification

This guide explains how to sign and verify plugins in Delivery Station.

## Overview

Delivery Station supports digital signature verification for plugins to ensure authenticity and integrity. There are two verification modes:

- **Strict Mode**: Only plugins with valid signatures are allowed
- **Permissive Mode**: Validates signatures when present, warns on unsigned plugins (default)

## Configuration

Configure signature verification in your `~/.config/ds/config.yaml`:

```yaml
plugins:
  signature:
    # Mode: "strict", "permissive", or "disabled"
    mode: permissive
    
    # Allow unsigned plugins in permissive mode
    allow_unsigned: true
    
    # List of trusted public key files
    public_keys:
      - ~/.config/ds/trust/my-org.pem
      - ~/.config/ds/trust/another-key.pub
    
    # Directory containing trusted public keys
    trust_store: ~/.config/ds/trust
```

### Modes

#### Strict Mode
```yaml
plugins:
  signature:
    mode: strict
    allow_unsigned: false
    trust_store: ~/.config/ds/trust
```

In strict mode:
- All plugins MUST have valid signatures
- Unsigned plugins are rejected
- Invalid signatures cause installation to fail
- At least one trusted public key is required

#### Permissive Mode (Default)
```yaml
plugins:
  signature:
    mode: permissive
    allow_unsigned: true
    trust_store: ~/.config/ds/trust
```

In permissive mode:
- Signatures are verified when present
- Unsigned plugins generate warnings but are allowed
- Invalid signatures generate warnings but installation proceeds
- Useful for development and testing

#### Disabled Mode
```yaml
plugins:
  signature:
    mode: disabled
```

Signature verification is completely disabled (not recommended for production).

## Signing Plugins (For Plugin Developers)

### Step 1: Generate a Key Pair

Generate an RSA key pair for signing your plugin:

```bash
cd /path/to/your/plugin

# Generate keys automatically during signing
ds plugin sign --generate --key-size 4096 ./ds-myplugin

# Or generate keys separately
ds plugin sign --generate --key ./my-private-key.pem ./ds-myplugin
```

This creates:
- `plugin-private.pem` - Private key (keep secret!)
- `plugin-public.pem` - Public key (distribute to users)

**Important**: Keep your private key secure! Never commit it to version control or share it publicly.

### Step 2: Sign Your Plugin Binary

Sign your plugin with your private key:

```bash
# Sign with existing private key
ds plugin sign --key ./plugin-private.pem ./ds-myplugin

# Or if you just generated keys
ds plugin sign --generate ./ds-myplugin
```

This creates:
- `ds-myplugin.sig` - Digital signature file

### Step 3: Distribute Your Plugin

When distributing your plugin, include:
1. The plugin binary: `ds-myplugin`
2. The signature file: `ds-myplugin.sig`
3. Your public key: `plugin-public.pem`

### Example: Signing a Porter Plugin

```bash
# Build your porter plugin
cd porter
go build -o ds-porter ./cmd/porter

# Generate keys (do this once)
ds plugin sign --generate --key-size 4096 ./ds-porter

# Sign the binary
ds plugin sign --key ./plugin-private.pem ./ds-porter

# Verify the signature
ds plugin verify ./ds-porter

# Distribute these files:
# - ds-porter (or ds-porter.exe on Windows)
# - ds-porter.sig
# - plugin-public.pem (for users to verify)
```

## Verifying Plugins (For Users)

### Step 1: Install Public Key

Save the plugin developer's public key to your trust store:

```bash
# Create trust store directory
mkdir -p ~/.config/ds/trust

# Copy the public key
cp plugin-public.pem ~/.config/ds/trust/myplugin.pem
```

### Step 2: Configure Verification Mode

Edit `~/.config/ds/config.yaml`:

```yaml
plugins:
  signature:
    mode: strict  # or permissive
    trust_store: ~/.config/ds/trust
```

### Step 3: Install Plugin

When you install a plugin, the signature is automatically verified:

```bash
# Install from registry (signature downloaded automatically)
ds plugin install myplugin@1.0.0

# Or manually install
cp ds-myplugin ~/.config/ds/plugins/
cp ds-myplugin.sig ~/.config/ds/plugins/
```

### Manual Verification

You can manually verify a plugin signature:

```bash
# Verify installed plugin
ds plugin verify ~/.config/ds/plugins/ds-myplugin

# Check verification status
ds plugin info myplugin
```

## Publishing Signed Plugins to OCI Registry

When publishing plugins to an OCI registry, include the signature:

```bash
# Build plugin
go build -o ds-myplugin ./cmd/myplugin

# Sign plugin
ds plugin sign --key ./my-private-key.pem ./ds-myplugin

# Push to registry (both binary and signature)
oras push ghcr.io/myorg/myplugin:1.0.0-linux-amd64 \
  ./ds-myplugin:application/vnd.ds.plugin.binary.v1 \
  ./ds-myplugin.sig:application/vnd.ds.plugin.signature.v1 \
  ./plugin.yaml:application/vnd.ds.plugin.manifest.v1

# Push public key for users
oras push ghcr.io/myorg/myplugin:pubkey \
  ./plugin-public.pem:application/vnd.ds.plugin.pubkey.v1
```

## Security Best Practices

### For Plugin Developers

1. **Protect Private Keys**
   - Never commit private keys to version control
   - Store private keys securely (encrypted filesystem, key vault)
   - Use strong passphrases if encrypting keys
   - Rotate keys periodically

2. **Use Strong Keys**
   - Minimum 2048-bit RSA (4096-bit recommended)
   - Consider using hardware security modules (HSM) for production

3. **Sign All Releases**
   - Sign every official release
   - Include signature in release artifacts
   - Document the public key location

4. **Distribute Public Keys Securely**
   - Host public keys on HTTPS
   - Include key fingerprints in documentation
   - Consider using keybase.io or similar services

### For Users

1. **Verify Public Keys**
   - Verify public key fingerprints before trusting
   - Download public keys from official sources
   - Check key authenticity through multiple channels

2. **Use Strict Mode for Production**
   ```yaml
   plugins:
     signature:
       mode: strict
       allow_unsigned: false
   ```

3. **Regular Key Updates**
   - Check for key revocations
   - Update trust store periodically
   - Remove untrusted keys

4. **Monitor Warnings**
   - Pay attention to signature warnings
   - Investigate unsigned plugins
   - Report suspicious plugins

## Troubleshooting

### "Plugin is not signed (strict mode requires signatures)"

**Solution**: Either obtain a signed version of the plugin or switch to permissive mode:
```yaml
plugins:
  signature:
    mode: permissive
    allow_unsigned: true
```

### "Signature verification failed"

**Causes**:
- Binary has been modified after signing
- Wrong public key is being used
- Corrupted signature file

**Solution**:
- Redownload the plugin and signature
- Verify you have the correct public key
- Contact plugin developer

### "Strict mode requires at least one public key"

**Solution**: Add trusted public keys to your configuration:
```yaml
plugins:
  signature:
    public_keys:
      - ~/.config/ds/trust/plugin-dev.pem
```

Or populate your trust store directory with public key files.

## Examples

### Example 1: Development Workflow

```bash
# Build plugin
go build -o ds-myplugin ./cmd/myplugin

# Generate keys (first time only)
ds plugin sign --generate --key-size 4096 ./ds-myplugin

# Sign
ds plugin sign --key ./plugin-private.pem ./ds-myplugin

# Test installation with verification disabled
DS_PLUGINS_SIGNATURE_MODE=disabled ds plugin install ./ds-myplugin
```

### Example 2: Production Deployment

```bash
# User configuration (~/.config/ds/config.yaml)
plugins:
  signature:
    mode: strict
    allow_unsigned: false
    trust_store: ~/.config/ds/trust

# Install trusted public keys
mkdir -p ~/.config/ds/trust
curl -o ~/.config/ds/trust/acme-corp.pem \
  https://acme.corp/plugins/public-key.pem

# Verify fingerprint
openssl pkey -pubin -in ~/.config/ds/trust/acme-corp.pem \
  -pubout -outform DER | sha256sum

# Install plugin (signature verified automatically)
ds plugin install acme-plugin@1.0.0
```

### Example 3: CI/CD Pipeline

```yaml
# .github/workflows/release.yml
name: Release Plugin

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Build plugin
        run: go build -o ds-myplugin ./cmd/myplugin
      
      - name: Sign plugin
        env:
          SIGNING_KEY: ${{ secrets.PLUGIN_SIGNING_KEY }}
        run: |
          echo "$SIGNING_KEY" > private.pem
          ds plugin sign --key ./private.pem ./ds-myplugin
          rm private.pem
      
      - name: Upload artifacts
        uses: actions/upload-artifact@v3
        with:
          name: plugin
          path: |
            ds-myplugin
            ds-myplugin.sig
```

## Related Commands

- `ds plugin sign` - Sign a plugin binary
- `ds plugin verify` - Verify a plugin signature
- `ds plugin install` - Install plugin (with automatic verification)
- `ds plugin list` - List installed plugins
- `ds plugin info` - Show plugin details including signature status

## FAQ

**Q: Do I need to sign plugins for development?**
A: No, use `mode: permissive` or `mode: disabled` for development.

**Q: Can I use different keys for different plugins?**
A: Yes, place all trusted public keys in your trust store directory.

**Q: What happens if a plugin is compromised?**
A: Revoke the signing key, remove it from trust stores, and publish a new signed version with a new key.

**Q: Can I use GPG keys instead of RSA?**
A: Currently, only RSA keys are supported. GPG support may be added in the future.

**Q: How do I rotate signing keys?**
A: Generate a new key pair, sign new releases with the new key, and distribute both old and new public keys during the transition period.
