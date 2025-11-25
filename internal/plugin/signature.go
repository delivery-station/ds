package plugin

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/delivery-station/ds/pkg/types"
	"github.com/sirupsen/logrus"
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
}

// NewSignatureVerifier creates a new signature verifier
func NewSignatureVerifier(config *types.SignatureConfig) (*SignatureVerifier, error) {
	if config == nil || config.Mode == SignatureModeDisabled || config.Mode == "" {
		return &SignatureVerifier{
			config:     &types.SignatureConfig{Mode: SignatureModeDisabled},
			publicKeys: nil,
		}, nil
	}

	v := &SignatureVerifier{
		config:     config,
		publicKeys: make([]*rsa.PublicKey, 0),
	}

	// Load public keys from specified paths
	for _, keyPath := range config.PublicKeys {
		key, err := v.loadPublicKey(keyPath)
		if err != nil {
			logrus.Warnf("Failed to load public key from %s: %v", keyPath, err)
			continue
		}
		v.publicKeys = append(v.publicKeys, key)
		logrus.Debugf("Loaded public key from %s", keyPath)
	}

	// Load public keys from trust store directory
	if config.TrustStore != "" {
		keys, err := v.loadTrustStore(config.TrustStore)
		if err != nil {
			logrus.Warnf("Failed to load trust store from %s: %v", config.TrustStore, err)
		} else {
			v.publicKeys = append(v.publicKeys, keys...)
			logrus.Debugf("Loaded %d keys from trust store %s", len(keys), config.TrustStore)
		}
	}

	// Validate configuration
	if config.Mode == SignatureModeStrict && len(v.publicKeys) == 0 {
		return nil, fmt.Errorf("strict mode requires at least one public key")
	}

	return v, nil
}

// VerifyPlugin verifies the digital signature of a plugin binary
func (v *SignatureVerifier) VerifyPlugin(binaryPath string) error {
	if v.config.Mode == SignatureModeDisabled {
		return nil
	}

	// Look for signature file
	sigPath := binaryPath + ".sig"
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		// No signature file found
		if os.IsNotExist(err) {
			return v.handleUnsignedPlugin(binaryPath)
		}
		return fmt.Errorf("failed to read signature file: %w", err)
	}

	// Read plugin binary
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin binary: %w", err)
	}

	// Compute hash
	hash := sha256.Sum256(binaryData)

	// Try to verify with each public key
	verified := false
	var lastErr error
	for i, pubKey := range v.publicKeys {
		err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sigData)
		if err == nil {
			verified = true
			logrus.Debugf("Plugin %s verified with public key #%d", filepath.Base(binaryPath), i+1)
			break
		}
		lastErr = err
	}

	if !verified {
		if len(v.publicKeys) == 0 {
			return v.handleUnsignedPlugin(binaryPath)
		}
		if v.config.Mode == SignatureModeStrict {
			return fmt.Errorf("signature verification failed: %w", lastErr)
		}
		// Permissive mode - warn but allow
		logrus.Warnf("Plugin %s has invalid signature, but permissive mode allows it", filepath.Base(binaryPath))
		return nil
	}

	logrus.Infof("Plugin %s signature verified successfully", filepath.Base(binaryPath))
	return nil
}

// handleUnsignedPlugin handles plugins without signatures based on mode
func (v *SignatureVerifier) handleUnsignedPlugin(binaryPath string) error {
	pluginName := filepath.Base(binaryPath)

	switch v.config.Mode {
	case SignatureModeStrict:
		return fmt.Errorf("plugin %s is not signed (strict mode requires signatures)", pluginName)
	case SignatureModePermissive:
		if v.config.AllowUnsigned {
			logrus.Warnf("⚠️  Plugin %s is not signed (running in permissive mode)", pluginName)
			return nil
		}
		return fmt.Errorf("plugin %s is not signed (allow_unsigned is false)", pluginName)
	default:
		return nil
	}
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
			logrus.Debugf("Skipping %s: %v", path, err)
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

// SignPlugin signs a plugin binary with a private key
// This is a helper function for plugin developers
func SignPlugin(binaryPath, privateKeyPath string) error {
	// Read private key
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	// Decode PEM
	block, _ := pem.Decode(keyData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	// Parse private key
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Read plugin binary
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin binary: %w", err)
	}

	// Compute hash
	hash := sha256.Sum256(binaryData)

	// Sign
	signature, err := rsa.SignPKCS1v15(nil, privKey, crypto.SHA256, hash[:])
	if err != nil {
		return fmt.Errorf("failed to sign: %w", err)
	}

	// Write signature file
	sigPath := binaryPath + ".sig"
	if err := os.WriteFile(sigPath, signature, 0644); err != nil {
		return fmt.Errorf("failed to write signature: %w", err)
	}

	logrus.Infof("Plugin signed successfully: %s", sigPath)
	return nil
}
