package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/delivery-station/ds/internal/registry"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Installer manages plugin installation from OCI registries
type Installer struct {
	pluginDir string
	registry  string
	client    *registry.Client
	verifier  *SignatureVerifier
}

// NewInstaller creates a new plugin installer
func NewInstaller(pluginDir, registryURL string, client *registry.Client) *Installer {
	return &Installer{
		pluginDir: pluginDir,
		registry:  registryURL,
		client:    client,
		verifier:  nil, // Will be set via SetSignatureVerifier
	}
}

// SetSignatureVerifier sets the signature verifier for the installer
func (i *Installer) SetSignatureVerifier(verifier *SignatureVerifier) {
	i.verifier = verifier
}

// InstallPlugin downloads and installs a plugin from the registry
func (i *Installer) InstallPlugin(ctx context.Context, name, version string) error {
	logrus.Infof("Installing plugin %s@%s", name, version)

	// Resolve platform
	platform := ResolvePlatform()
	logrus.Debugf("Resolved platform: %s", platform)

	// Construct reference
	ref := fmt.Sprintf("%s:%s", name, version)
	if version == "" || version == "latest" {
		ref = fmt.Sprintf("%s:latest", name)
	}

	// Create temp directory for download
	tmpDir, err := os.MkdirTemp("", "ds-plugin-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download plugin manifest
	manifestPath := filepath.Join(tmpDir, "plugin.yaml")
	if err := i.downloadManifest(ctx, ref, manifestPath); err != nil {
		return fmt.Errorf("failed to download manifest: %w", err)
	}

	// Parse manifest
	manifest, err := loadPluginManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Check platform compatibility
	if !isManifestCompatible(manifest, runtime.GOOS, runtime.GOARCH) {
		return fmt.Errorf("plugin not compatible with %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Download plugin binary
	binaryName := name
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	tmpBinary := filepath.Join(tmpDir, binaryName)
	if err := i.downloadBinary(ctx, ref, tmpBinary, platform); err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Verify checksum if provided
	if manifest.Checksum != "" {
		if err := verifyChecksum(tmpBinary, manifest.Checksum); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		logrus.Debug("Checksum verified")
	}

	// Ensure plugin directory exists
	if err := os.MkdirAll(i.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Install binary
	destBinary := filepath.Join(i.pluginDir, binaryName)
	if err := copyFile(tmpBinary, destBinary); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	// Set executable permissions
	if err := os.Chmod(destBinary, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	// Download and install signature if available
	sigPath := tmpBinary + ".sig"
	if err := i.downloadSignature(ctx, ref, sigPath, platform); err != nil {
		logrus.Debugf("No signature found for plugin: %v", err)
	} else {
		destSig := destBinary + ".sig"
		if err := copyFile(sigPath, destSig); err != nil {
			logrus.Warnf("Failed to install signature: %v", err)
		}
	}

	// Verify signature
	if i.verifier != nil {
		if err := i.verifier.VerifyPlugin(destBinary); err != nil {
			// Clean up on verification failure
			os.Remove(destBinary)
			os.Remove(destBinary + ".sig")
			return fmt.Errorf("signature verification failed: %w", err)
		}
	}

	// Install manifest
	destManifest := filepath.Join(i.pluginDir, name+".yaml")
	if err := copyFile(manifestPath, destManifest); err != nil {
		return fmt.Errorf("failed to install manifest: %w", err)
	}

	logrus.Infof("Successfully installed %s@%s", name, manifest.Version)
	return nil
}

// UpdatePlugin updates a plugin to the latest version
func (i *Installer) UpdatePlugin(ctx context.Context, name string) error {
	logrus.Infof("Updating plugin %s", name)

	// Check current version
	currentManifest := filepath.Join(i.pluginDir, name+".yaml")
	var currentVersion string
	if manifest, err := loadPluginManifest(currentManifest); err == nil {
		currentVersion = manifest.Version
		logrus.Debugf("Current version: %s", currentVersion)
	}

	// Get available versions
	versions, err := i.GetAvailableVersions(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get available versions: %w", err)
	}

	if len(versions) == 0 {
		return fmt.Errorf("no versions available for %s", name)
	}

	// Use latest version
	latestVersion := "latest"
	if len(versions) > 0 {
		latestVersion = versions[0] // Assuming first is latest
	}

	// Check if already at latest
	if currentVersion == latestVersion {
		logrus.Infof("Plugin %s is already at latest version (%s)", name, currentVersion)
		return nil
	}

	// Backup current version (optional)
	if currentVersion != "" {
		if err := i.backupPlugin(name); err != nil {
			logrus.Warnf("Failed to backup plugin: %v", err)
		}
	}

	// Install latest version
	return i.InstallPlugin(ctx, name, latestVersion)
}

// RemovePlugin removes a plugin from the plugin directory
func (i *Installer) RemovePlugin(ctx context.Context, name string) error {
	logrus.Infof("Removing plugin %s", name)

	// Remove binary
	binaryName := name
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(i.pluginDir, binaryName)

	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove binary: %w", err)
	}

	// Remove manifest
	manifestPath := filepath.Join(i.pluginDir, name+".yaml")
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove manifest: %w", err)
	}

	logrus.Infof("Successfully removed plugin %s", name)
	return nil
}

// GetAvailableVersions lists available versions for a plugin in the registry
func (i *Installer) GetAvailableVersions(ctx context.Context, name string) ([]string, error) {
	logrus.Debugf("Fetching available versions for %s", name)

	versions, err := i.client.List(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	return versions, nil
}

// ResolvePlatform returns the current platform string (os/arch)
func ResolvePlatform() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

// downloadManifest downloads the plugin manifest from the registry
func (i *Installer) downloadManifest(ctx context.Context, ref, destPath string) error {
	logrus.Debugf("Downloading manifest for %s", ref)

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Download manifest artifact
	manifestRef := fmt.Sprintf("%s-manifest", ref)
	if err := i.client.Pull(ctx, manifestRef, file); err != nil {
		// If manifest artifact doesn't exist, try fetching from main artifact
		file.Seek(0, 0)
		file.Truncate(0)

		if err := i.client.Pull(ctx, ref, file); err != nil {
			return err
		}
	}

	return nil
}

// downloadBinary downloads the plugin binary from the registry
func (i *Installer) downloadBinary(ctx context.Context, ref, destPath, platform string) error {
	logrus.Debugf("Downloading binary for %s (%s)", ref, platform)

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Construct platform-specific reference
	binaryRef := fmt.Sprintf("%s-%s", ref, strings.ReplaceAll(platform, "/", "-"))

	// Download binary
	if err := i.client.Pull(ctx, binaryRef, file); err != nil {
		return err
	}

	return nil
}

// downloadSignature downloads the plugin signature from the registry
func (i *Installer) downloadSignature(ctx context.Context, ref, destPath, platform string) error {
	logrus.Debugf("Downloading signature for %s (%s)", ref, platform)

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Construct signature reference (same as binary but with .sig suffix)
	sigRef := fmt.Sprintf("%s-%s.sig", ref, strings.ReplaceAll(platform, "/", "-"))

	// Download signature
	if err := i.client.Pull(ctx, sigRef, file); err != nil {
		return err
	}

	return nil
}

// verifyChecksum verifies the SHA256 checksum of a file
func verifyChecksum(path, expectedChecksum string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// backupPlugin creates a backup of the current plugin version
func (i *Installer) backupPlugin(name string) error {
	binaryName := name
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	srcPath := filepath.Join(i.pluginDir, binaryName)
	backupPath := filepath.Join(i.pluginDir, binaryName+".bak")

	return copyFile(srcPath, backupPath)
}

// loadPluginManifest loads and parses a plugin manifest file
func loadPluginManifest(path string) (*types.PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest types.PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// isManifestCompatible checks if a manifest is compatible with the given platform
func isManifestCompatible(manifest *types.PluginManifest, goos, goarch string) bool {
	// If no platform info, assume compatible
	if len(manifest.Platform.OS) == 0 {
		return true
	}

	// Check OS compatibility
	osCompatible := false
	for _, os := range manifest.Platform.OS {
		if os == goos || os == "all" {
			osCompatible = true
			break
		}
	}

	if !osCompatible {
		return false
	}

	// Check architecture compatibility
	if len(manifest.Platform.Arch) == 0 {
		return true // No arch restriction
	}

	for _, arch := range manifest.Platform.Arch {
		if arch == goarch || arch == "all" {
			return true
		}
	}

	return false
}
