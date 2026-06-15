package keys

import (
	"crypto/rsa"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"fmt"
	"path/filepath"
)

//go:embed *.pem
var embeddedKeys embed.FS

// LoadEmbeddedKeys loads all embedded official public keys
func LoadEmbeddedKeys() ([]*rsa.PublicKey, error) {
	var keys []*rsa.PublicKey

	entries, err := embeddedKeys.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded keys: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".pem" {
			continue
		}

		data, err := embeddedKeys.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded key %s: %w", entry.Name(), err)
		}

		block, _ := pem.Decode(data)
		if block == nil {
			continue
		}

		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key in %s: %w", entry.Name(), err)
		}

		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			continue
		}

		keys = append(keys, rsaPub)
	}

	return keys, nil
}
