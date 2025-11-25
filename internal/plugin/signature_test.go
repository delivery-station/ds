package plugin

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/delivery-station/ds/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignatureVerifier_Disabled(t *testing.T) {
	config := &types.SignatureConfig{
		Mode: SignatureModeDisabled,
	}

	verifier, err := NewSignatureVerifier(config)
	require.NoError(t, err)
	assert.NotNil(t, verifier)

	// Create a test binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-plugin")
	err = os.WriteFile(binaryPath, []byte("test binary content"), 0644)
	require.NoError(t, err)

	// Should not verify (disabled mode)
	err = verifier.VerifyPlugin(binaryPath)
	assert.NoError(t, err)
}

func TestSignatureVerifier_Permissive_Unsigned(t *testing.T) {
	config := &types.SignatureConfig{
		Mode:          SignatureModePermissive,
		AllowUnsigned: true,
	}

	verifier, err := NewSignatureVerifier(config)
	require.NoError(t, err)

	// Create a test binary without signature
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-plugin")
	err = os.WriteFile(binaryPath, []byte("test binary content"), 0644)
	require.NoError(t, err)

	// Should allow unsigned in permissive mode
	err = verifier.VerifyPlugin(binaryPath)
	assert.NoError(t, err)
}

func TestSignatureVerifier_Strict_Unsigned(t *testing.T) {
	// Create a temporary key for strict mode requirement
	tmpDir := t.TempDir()
	_, pubKey := generateTestKeyPair(t)
	pubKeyPath := savePublicKey(t, tmpDir, pubKey)

	config := &types.SignatureConfig{
		Mode:       SignatureModeStrict,
		PublicKeys: []string{pubKeyPath},
	}

	verifier, err := NewSignatureVerifier(config)
	require.NoError(t, err)

	// Create a test binary without signature
	binaryPath := filepath.Join(tmpDir, "test-plugin")
	err = os.WriteFile(binaryPath, []byte("test binary content"), 0644)
	require.NoError(t, err)

	// Should reject unsigned in strict mode
	err = verifier.VerifyPlugin(binaryPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not signed")
}

func TestSignatureVerifier_ValidSignature(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate key pair
	privKey, pubKey := generateTestKeyPair(t)
	privKeyPath := savePrivateKey(t, tmpDir, privKey)
	pubKeyPath := savePublicKey(t, tmpDir, pubKey)

	// Create and sign a test binary
	binaryPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(binaryPath, []byte("test binary content"), 0644)
	require.NoError(t, err)

	err = SignPlugin(binaryPath, privKeyPath)
	require.NoError(t, err)

	// Verify signature exists
	sigPath := binaryPath + ".sig"
	assert.FileExists(t, sigPath)

	// Create verifier
	config := &types.SignatureConfig{
		Mode:       SignatureModeStrict,
		PublicKeys: []string{pubKeyPath},
	}

	verifier, err := NewSignatureVerifier(config)
	require.NoError(t, err)

	// Should verify successfully
	err = verifier.VerifyPlugin(binaryPath)
	assert.NoError(t, err)
}

func TestSignatureVerifier_InvalidSignature(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate two different key pairs
	privKey1, _ := generateTestKeyPair(t)
	_, pubKey2 := generateTestKeyPair(t)

	privKeyPath := savePrivateKey(t, tmpDir, privKey1)
	pubKeyPath := savePublicKey(t, tmpDir, pubKey2)

	// Create and sign with first key
	binaryPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(binaryPath, []byte("test binary content"), 0644)
	require.NoError(t, err)

	err = SignPlugin(binaryPath, privKeyPath)
	require.NoError(t, err)

	// Create verifier with second (different) public key
	config := &types.SignatureConfig{
		Mode:       SignatureModeStrict,
		PublicKeys: []string{pubKeyPath},
	}

	verifier, err := NewSignatureVerifier(config)
	require.NoError(t, err)

	// Should fail verification
	err = verifier.VerifyPlugin(binaryPath)
	assert.Error(t, err)
}

func TestSignatureVerifier_ModifiedBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate key pair
	privKey, pubKey := generateTestKeyPair(t)
	privKeyPath := savePrivateKey(t, tmpDir, privKey)
	pubKeyPath := savePublicKey(t, tmpDir, pubKey)

	// Create and sign a test binary
	binaryPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(binaryPath, []byte("original content"), 0644)
	require.NoError(t, err)

	err = SignPlugin(binaryPath, privKeyPath)
	require.NoError(t, err)

	// Modify the binary
	err = os.WriteFile(binaryPath, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Create verifier
	config := &types.SignatureConfig{
		Mode:       SignatureModeStrict,
		PublicKeys: []string{pubKeyPath},
	}

	verifier, err := NewSignatureVerifier(config)
	require.NoError(t, err)

	// Should fail verification (binary was modified after signing)
	err = verifier.VerifyPlugin(binaryPath)
	assert.Error(t, err)
}

func TestSignatureVerifier_TrustStore(t *testing.T) {
	tmpDir := t.TempDir()
	trustStore := filepath.Join(tmpDir, "trust")
	err := os.MkdirAll(trustStore, 0755)
	require.NoError(t, err)

	// Generate key pair and save public key to trust store
	privKey, pubKey := generateTestKeyPair(t)
	privKeyPath := savePrivateKey(t, tmpDir, privKey)

	pubKeyPath := filepath.Join(trustStore, "test-key.pem")
	savePublicKeyTo(t, pubKeyPath, pubKey)

	// Create and sign a test binary
	binaryPath := filepath.Join(tmpDir, "test-plugin")
	err = os.WriteFile(binaryPath, []byte("test binary content"), 0644)
	require.NoError(t, err)

	err = SignPlugin(binaryPath, privKeyPath)
	require.NoError(t, err)

	// Create verifier with trust store
	config := &types.SignatureConfig{
		Mode:       SignatureModeStrict,
		TrustStore: trustStore,
	}

	verifier, err := NewSignatureVerifier(config)
	require.NoError(t, err)
	assert.Len(t, verifier.publicKeys, 1)

	// Should verify successfully
	err = verifier.VerifyPlugin(binaryPath)
	assert.NoError(t, err)
}

func TestSignatureVerifier_MultipleKeys(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate two key pairs
	privKey1, pubKey1 := generateTestKeyPair(t)
	_, pubKey2 := generateTestKeyPair(t)

	privKeyPath := savePrivateKey(t, tmpDir, privKey1)
	pubKeyPath1 := filepath.Join(tmpDir, "key1.pem")
	pubKeyPath2 := filepath.Join(tmpDir, "key2.pem")
	savePublicKeyTo(t, pubKeyPath1, pubKey1)
	savePublicKeyTo(t, pubKeyPath2, pubKey2)

	// Create and sign with first key
	binaryPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(binaryPath, []byte("test binary content"), 0644)
	require.NoError(t, err)

	err = SignPlugin(binaryPath, privKeyPath)
	require.NoError(t, err)

	// Create verifier with both public keys
	config := &types.SignatureConfig{
		Mode:       SignatureModeStrict,
		PublicKeys: []string{pubKeyPath1, pubKeyPath2},
	}

	verifier, err := NewSignatureVerifier(config)
	require.NoError(t, err)
	assert.Len(t, verifier.publicKeys, 2)

	// Should verify successfully (matches first key)
	err = verifier.VerifyPlugin(binaryPath)
	assert.NoError(t, err)
}

// Helper functions

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privKey, &privKey.PublicKey
}

func savePrivateKey(t *testing.T, dir string, key *rsa.PrivateKey) string {
	path := filepath.Join(dir, "private.pem")
	savePrivateKeyTo(t, path, key)
	return path
}

func savePrivateKeyTo(t *testing.T, path string, key *rsa.PrivateKey) {
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	file, err := os.Create(path)
	require.NoError(t, err)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("Failed to close file: %v", err)
		}
	}()

	err = pem.Encode(file, pemBlock)
	require.NoError(t, err)
}

func savePublicKey(t *testing.T, dir string, key *rsa.PublicKey) string {
	path := filepath.Join(dir, "public.pem")
	savePublicKeyTo(t, path, key)
	return path
}

func savePublicKeyTo(t *testing.T, path string, key *rsa.PublicKey) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(key)
	require.NoError(t, err)

	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}

	file, err := os.Create(path)
	require.NoError(t, err)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("Failed to close file: %v", err)
		}
	}()

	err = pem.Encode(file, pemBlock)
	require.NoError(t, err)
}
