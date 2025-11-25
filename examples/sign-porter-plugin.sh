#!/bin/bash
# Example: Sign a Porter Plugin
# This script demonstrates how to sign a porter plugin for distribution

set -e

echo "=== Porter Plugin Signing Example ==="
echo

# Step 1: Build the Porter plugin
echo "Step 1: Building porter plugin..."
cd "$(dirname "$0")/../../../porter"
go build -o ds-porter ./cmd/porter
echo "✓ Built: ds-porter"
echo

# Step 2: Generate signing keys (first time only)
echo "Step 2: Generating RSA key pair..."
if [ ! -f plugin-private.pem ]; then
    # Using ds CLI to generate keys
    ../ds/bin/ds plugin sign --generate --key-size 4096 ./ds-porter
    echo "✓ Generated keys:"
    echo "  - plugin-private.pem (KEEP SECRET!)"
    echo "  - plugin-public.pem (distribute to users)"
else
    echo "✓ Keys already exist, skipping generation"
fi
echo

# Step 3: Sign the plugin
echo "Step 3: Signing plugin..."
../ds/bin/ds plugin sign --key ./plugin-private.pem ./ds-porter
echo "✓ Signature created: ds-porter.sig"
echo

# Step 4: Verify the signature
echo "Step 4: Verifying signature..."

# Create a test DS config with the public key
mkdir -p ~/.config/ds/trust-test
cp plugin-public.pem ~/.config/ds/trust-test/

# Create test config
cat > /tmp/ds-test-config.yaml <<EOF
plugins:
  signature:
    mode: strict
    trust_store: ~/.config/ds/trust-test
EOF

# Verify using the public key
DS_CONFIG=/tmp/ds-test-config.yaml ../ds/bin/ds plugin verify ./ds-porter
echo "✓ Signature verified successfully!"
echo

# Step 5: Show distribution files
echo "Step 5: Files to distribute:"
echo "  Binary:    $(pwd)/ds-porter"
echo "  Signature: $(pwd)/ds-porter.sig"
echo "  Public Key: $(pwd)/plugin-public.pem"
echo
echo "File sizes:"
ls -lh ds-porter ds-porter.sig plugin-public.pem 2>/dev/null || true
echo

# Step 6: Calculate checksums
echo "Step 6: Checksums (for verification):"
echo "Binary SHA256:"
shasum -a 256 ds-porter | awk '{print "  " $1}'
echo "Public Key SHA256:"
shasum -a 256 plugin-public.pem | awk '{print "  " $1}'
echo

# Step 7: Example OCI distribution
echo "Step 7: Publishing to OCI registry (example):"
echo
cat <<'EOF'
# Push the signed plugin to an OCI registry
oras push ghcr.io/myorg/porter:1.0.0-linux-amd64 \
  ./ds-porter:application/vnd.ds.plugin.binary.v1 \
  ./ds-porter.sig:application/vnd.ds.plugin.signature.v1 \
  ./plugin.yaml:application/vnd.ds.plugin.manifest.v1

# Push the public key separately
oras push ghcr.io/myorg/porter:pubkey \
  ./plugin-public.pem:application/vnd.ds.plugin.pubkey.v1

# Users can then install with automatic verification:
# ds plugin install ghcr.io/myorg/porter:1.0.0
EOF
echo

# Step 8: Cleanup test files
rm -f /tmp/ds-test-config.yaml

echo "=== Example Complete ==="
echo
echo "⚠️  Security Reminders:"
echo "  - Keep plugin-private.pem secure and never commit it!"
echo "  - Distribute only plugin-public.pem to users"
echo "  - Include public key SHA256 in your documentation"
echo "  - Consider using a hardware security module (HSM) for production"
echo
echo "Next steps:"
echo "  1. Distribute ds-porter, ds-porter.sig, and plugin-public.pem"
echo "  2. Document the public key fingerprint"
echo "  3. Publish to your OCI registry"
echo "  4. Update your documentation with installation instructions"
