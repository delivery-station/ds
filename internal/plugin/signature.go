package plugin

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"encoding/base64"

	"github.com/delivery-station/ds/pkg/keys"
	"github.com/delivery-station/ds/pkg/log"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/hashicorp/go-hclog"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	// SignatureModeStrict only allows plugins with valid signatures
	SignatureModeStrict = "strict"
	// SignatureModePermissive validates signatures but allows unsigned plugins with warning
	SignatureModePermissive = "permissive"
	// SignatureModeDisabled disables signature verification
	SignatureModeDisabled = "disabled"
)

// SignatureVerifier handles digital signature verification for plugins
type SignatureVerifier struct {
	config     *types.SignatureConfig
	publicKeys []*rsa.PublicKey
	logger     hclog.Logger
}

// NewSignatureVerifier creates a new signature verifier
func NewSignatureVerifier(config *types.SignatureConfig, logger hclog.Logger) (*SignatureVerifier, error) {
	if logger == nil {
		logger = log.Named("signature-verifier")
	}

	if config == nil || config.Mode == SignatureModeDisabled || config.Mode == "" {
		return &SignatureVerifier{
			config:     &types.SignatureConfig{Mode: SignatureModeDisabled},
			publicKeys: nil,
			logger:     logger,
		}, nil
	}

	v := &SignatureVerifier{
		config:     config,
		publicKeys: make([]*rsa.PublicKey, 0),
		logger:     logger,
	}

	// 1. Load embedded official keys (Root of Trust)
	embedded, err := keys.LoadEmbeddedKeys()
	if err != nil {
		// This is critical - warning only if strict mode is not enforced?
		// Actually, if we can't load embedded keys, something is wrong with the binary.
		// Use logger.Error but proceed as we might have other keys.
		logger.Error("Failed to load embedded official keys", "error", err)
	} else {
		v.publicKeys = append(v.publicKeys, embedded...)
		logger.Debug("Loaded embedded official keys", "count", len(embedded))
	}

	// 2. Load public keys from specified paths (Config)
	for _, keyPath := range config.PublicKeys {
		key, err := v.loadPublicKey(keyPath)
		if err != nil {
			logger.Warn("Failed to load public key", "path", keyPath, "error", err)
			continue
		}
		v.publicKeys = append(v.publicKeys, key)
		logger.Debug("Loaded public key from config", "path", keyPath)
	}

	// 3. Load public keys from trust store directory (Filesystem)
	if config.TrustStore != "" {
		keys, err := v.loadTrustStore(config.TrustStore)
		if err != nil {
			// If default trust store doesn't exist, it's fine (warn if it was explicitly set?)
			// Typically we just warn if it fails.
			logger.Debug("Failed to load trust store (might be empty or missing)", "path", config.TrustStore, "error", err)
		} else {
			v.publicKeys = append(v.publicKeys, keys...)
			logger.Debug("Loaded public keys from trust store", "path", config.TrustStore, "count", len(keys))
		}
	}

	// Validate configuration
	if config.Mode == SignatureModeStrict && len(v.publicKeys) == 0 {
		return nil, fmt.Errorf("strict mode requires at least one public key")
	}

	return v, nil
}

// VerifyLayer checks the signature annotations on a layer
func (v *SignatureVerifier) VerifyLayer(layer ocispec.Descriptor) error {
	if v.config.Mode == SignatureModeDisabled {
		return nil
	}

	sigEnc := layer.Annotations["delivery-station.io/signature"]
	keyID := layer.Annotations["delivery-station.io/key.id"]

	if sigEnc == "" {
		if v.config.Mode == SignatureModeStrict {
			return fmt.Errorf("missing signature annotations on layer (strict mode)")
		}
		// Permissive mode with missing signature
		if v.config.AllowUnsigned {
			v.logger.Warn("⚠️  Layer has no signature (running in permissive mode)")
			return nil
		}
		return fmt.Errorf("layer is not signed (mode=%s)", v.config.Mode)
	}

	sig, err := base64.StdEncoding.DecodeString(sigEnc)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Verify signature against trusted keys
	// The signature signs the Layer Digest (which is the digest of the content + metadata in OCI,
	// but here we signed the *content* hash.
	// Wait, in ds-porter we signed the *binary content*. The digest of the layer IS the digest of the content.
	// So we verify that the signature corresponds to the digest string.
	// Actually, usually we sign the hex string or the raw bytes.
	// In ds-porter step 472: signature, keyID, _, err := p.signContent(contentBytes)
	// So we signed the CONTENT.
	// To verify, we need the content or the hash of the content.
	// We only have the digest here (which is the hash of the content).
	// We can't verify PKCS1v15 signature of *content* if we only have the *hash* unless we blindly trust the hash.
	// BUT, RSA SignPKCS1v15 takes the HASH of the message.
	// rsa.SignPKCS1v15(rand, priv, crypto.SHA256, hashed[:])
	// So we need the HASH of the content.
	// The layer.Digest IS the SHA256 hash of the content (usually).
	// So we can parse layer.Digest to get the hash bytes.

	// Parse Digest
	// digest is "sha256:<hex>"
	parts := strings.Split(layer.Digest.String(), ":")
	if len(parts) != 2 || parts[0] != "sha256" {
		return fmt.Errorf("unsupported digest algorithm: %s", layer.Digest.Algorithm())
	}
	hashBytes, err := hex.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("failed to decode digest hex: %w", err)
	}

	// Verify
	verified := false
	var lastErr error

	// If Key ID is provided, try to find matching key
	if keyID != "" {
		v.logger.Debug("Verifying with specific Key ID", "key_id", keyID)

		// Find matching key in trusted keys
		var matchedKey *rsa.PublicKey
		for _, pubKey := range v.publicKeys {
			// Calculate ID for this key
			pkix, err := x509.MarshalPKIXPublicKey(pubKey)
			if err != nil {
				continue
			}
			id := fmt.Sprintf("%x", sha256.Sum256(pkix))

			if id == keyID {
				matchedKey = pubKey
				break
			}
		}

		if matchedKey != nil {
			err := rsa.VerifyPKCS1v15(matchedKey, crypto.SHA256, hashBytes, sig)
			if err == nil {
				verified = true
				v.logger.Info("Layer signature verified with trusted Key ID", "key_id", keyID)
			} else {
				lastErr = err
				v.logger.Warn("Signature verification failed for matched Key ID", "key_id", keyID, "error", err)
			}
		} else {
			lastErr = fmt.Errorf("key ID %s not found in trusted keys", keyID)
			v.logger.Warn("Key ID not found in trusted keys", "key_id", keyID)
		}
	} else {
		// Fallback to trying all keys if no ID provided (backward compatibility / resilience)
		v.logger.Debug("No Key ID in layer annotations, trying all trusted keys")
		for _, pubKey := range v.publicKeys {
			err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashBytes, sig)
			if err == nil {
				verified = true
				break
			}
			lastErr = err
		}
	}

	if !verified {
		return fmt.Errorf("signature verification failed: %w", lastErr)
	}

	v.logger.Info("Layer signature verified successfully")
	return nil
}

// VerifyArtifact checks if the artifact's hash matches the expected hash from SHA256SUMS
func (v *SignatureVerifier) VerifyArtifact(path string, expectedHash string) error {
	if v.config.Mode == SignatureModeDisabled {
		return nil
	}

	if expectedHash == "" {
		// If verification was skipped (permissive/unsigned), we won't have an expected hash
		// In strict mode, VerifyMetadata would have failed if signatures were missing.
		// So here we just return if no hash provided.
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read artifact: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if hash != expectedHash {
		return fmt.Errorf("artifact hash mismatch: expected %s, got %s", expectedHash, hash)
	}

	v.logger.Debug("Artifact hash verified", "hash", hash)
	return nil
}

// loadPublicKey loads a public key from a PEM file
func (v *SignatureVerifier) loadPublicKey(path string) (*rsa.PublicKey, error) {
	// Expand path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Decode PEM
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Parse public key
	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Convert to RSA public key
	pubKey, ok := pubKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA public key")
	}

	return pubKey, nil
}

// loadTrustStore loads all public keys from a directory
func (v *SignatureVerifier) loadTrustStore(dir string) ([]*rsa.PublicKey, error) {
	// Expand path
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dir = filepath.Join(home, dir[2:])
	}

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("trust store directory does not exist: %s", dir)
	}

	keys := make([]*rsa.PublicKey, 0)

	// Walk directory
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .pem and .pub files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".pem" && ext != ".pub" {
			return nil
		}

		// Try to load key
		key, err := v.loadPublicKey(path)
		if err != nil {
			v.logger.Debug("Skipping key file", "path", path, "error", err)
			return nil
		}

		keys = append(keys, key)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk trust store: %w", err)
	}

	return keys, nil
}
