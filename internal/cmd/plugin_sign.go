package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/delivery-station/ds/internal/config"
	"github.com/delivery-station/ds/internal/plugin"
	"github.com/spf13/cobra"
)

var (
	signPrivateKeyPath string
	signGenerateKeys   bool
	signKeySize        int
)

// pluginSignCmd signs a plugin binary
var pluginSignCmd = &cobra.Command{
	Use:   "sign <plugin-binary>",
	Short: "Sign a plugin binary with a private key",
	Long: `Sign a plugin binary with a private key to create a digital signature.
The signature will be saved as <plugin-binary>.sig

You can generate a key pair with the --generate flag.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		binaryPath := args[0]

		// Check if binary exists
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			return fmt.Errorf("plugin binary not found: %s", binaryPath)
		}

		// Generate keys if requested
		if signGenerateKeys {
			dir := filepath.Dir(binaryPath)
			privKeyPath := filepath.Join(dir, "plugin-private.pem")
			pubKeyPath := filepath.Join(dir, "plugin-public.pem")

			fmt.Printf("Generating RSA key pair (%d bits)...\n", signKeySize)
			if err := generateKeyPair(privKeyPath, pubKeyPath, signKeySize); err != nil {
				return fmt.Errorf("failed to generate keys: %w", err)
			}

			fmt.Printf("✅ Private key: %s\n", privKeyPath)
			fmt.Printf("✅ Public key: %s\n", pubKeyPath)
			fmt.Println("\n⚠️  Keep the private key secure! Distribute only the public key.")

			// Use the generated private key
			signPrivateKeyPath = privKeyPath
		}

		// Check if private key is specified
		if signPrivateKeyPath == "" {
			return fmt.Errorf("private key path is required (use --key or --generate)")
		}

		// Check if private key exists
		if _, err := os.Stat(signPrivateKeyPath); os.IsNotExist(err) {
			return fmt.Errorf("private key not found: %s", signPrivateKeyPath)
		}

		// Sign the plugin
		fmt.Printf("Signing plugin: %s\n", binaryPath)
		if err := plugin.SignPlugin(binaryPath, signPrivateKeyPath); err != nil {
			return fmt.Errorf("failed to sign plugin: %w", err)
		}

		sigPath := binaryPath + ".sig"
		fmt.Printf("✅ Signature created: %s\n", sigPath)
		fmt.Println("\n📦 To distribute:")
		fmt.Printf("   - Plugin binary: %s\n", binaryPath)
		fmt.Printf("   - Signature: %s\n", sigPath)
		fmt.Println("   - Public key (for verification)")

		return nil
	},
}

// pluginVerifyCmd verifies a plugin signature
var pluginVerifyCmd = &cobra.Command{
	Use:   "verify <plugin-binary>",
	Short: "Verify a plugin signature",
	Long: `Verify the digital signature of a plugin binary using public keys
from the configuration.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		binaryPath := args[0]

		// Check if binary exists
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			return fmt.Errorf("plugin binary not found: %s", binaryPath)
		}

		// Load configuration
		loader := config.NewLoader()
		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create signature verifier
		verifier, err := plugin.NewSignatureVerifier(&cfg.Plugins.Signature)
		if err != nil {
			return fmt.Errorf("failed to create verifier: %w", err)
		}

		// Verify signature
		fmt.Printf("Verifying plugin: %s\n", binaryPath)
		if err := verifier.VerifyPlugin(binaryPath); err != nil {
			fmt.Printf("❌ Verification failed: %v\n", err)
			return err
		}

		fmt.Println("✅ Signature verified successfully!")
		return nil
	},
}

func generateKeyPair(privPath, pubPath string, keySize int) error {
	// Generate private key
	privKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Encode private key to PEM
	privKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}

	// Write private key
	privFile, err := os.OpenFile(privPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privFile.Close()

	if err := pem.Encode(privFile, privKeyPEM); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	// Encode public key to PEM
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}

	// Write public key
	pubFile, err := os.OpenFile(pubPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer pubFile.Close()

	if err := pem.Encode(pubFile, pubKeyPEM); err != nil {
		return fmt.Errorf("failed to encode public key: %w", err)
	}

	return nil
}

func init() {
	// Add sign command to plugin
	pluginCmd.AddCommand(pluginSignCmd)
	pluginCmd.AddCommand(pluginVerifyCmd)

	// Flags for sign command
	pluginSignCmd.Flags().StringVarP(&signPrivateKeyPath, "key", "k", "", "Path to private key file")
	pluginSignCmd.Flags().BoolVarP(&signGenerateKeys, "generate", "g", false, "Generate a new key pair")
	pluginSignCmd.Flags().IntVar(&signKeySize, "key-size", 2048, "RSA key size (2048 or 4096)")
}
