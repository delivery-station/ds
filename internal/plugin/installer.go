package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/delivery-station/ds/internal/registry"
	"github.com/delivery-station/ds/pkg/log"
	"github.com/delivery-station/ds/pkg/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
)

// Installer manages plugin installation from OCI registries
type Installer struct {
	pluginDir string
	registry  string
	client    *registry.Client
	verifier  *SignatureVerifier
}

const (
	mediaTypePluginArchive   = "application/vnd.delivery-station.plugin.v1+archive.tar+gzip"
	mediaTypeArtifactArchive = "application/vnd.delivery-station.artifact.v1+archive.tar+gzip"
)

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
	log.Info("Installing plugin", "name", name, "version", version)

	// Resolve platform
	platform := ResolvePlatform()
	log.Debug("Resolved platform", "platform", platform)

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
	defer func() {
		_ = os.RemoveAll(tmpDir) // Best effort cleanup
	}()

	// Download plugin manifest
	manifestPath := filepath.Join(tmpDir, "plugin.json")
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

	// Determine binary name and manifest identifier
	binaryBase := filepath.Base(name)
	binaryName := binaryBase
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
		log.Debug("Checksum verified")
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
		log.Debug("No signature found for plugin", "error", err)
	} else {
		destSig := destBinary + ".sig"
		if err := copyFile(sigPath, destSig); err != nil {
			log.Warn("Failed to install signature", "error", err)
		}
	}

	// Verify signature
	if i.verifier != nil {
		if err := i.verifier.VerifyPlugin(destBinary); err != nil {
			// Clean up on verification failure
			_ = os.Remove(destBinary)          // Best effort cleanup
			_ = os.Remove(destBinary + ".sig") // Best effort cleanup
			return fmt.Errorf("signature verification failed: %w", err)
		}
	}

	// Install manifest with a .json extension to match the OCI index content
	destManifest := filepath.Join(i.pluginDir, binaryBase+".json")
	if err := copyFile(manifestPath, destManifest); err != nil {
		return fmt.Errorf("failed to install manifest: %w", err)
	}

	log.Info("Successfully installed plugin", "name", name, "version", manifest.Version)
	return nil
}

// UpdatePlugin updates a plugin to the latest version
func (i *Installer) UpdatePlugin(ctx context.Context, name string) error {
	log.Info("Updating plugin", "name", name)

	// Check current version
	manifestName := filepath.Base(name) + ".json"
	jsonManifest := filepath.Join(i.pluginDir, manifestName)
	var currentVersion string
	if manifest, err := loadPluginManifest(jsonManifest); err == nil {
		currentVersion = manifest.Version
		log.Debug("Current version", "version", currentVersion)
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
		log.Info("Plugin is already at latest version", "name", name, "version", currentVersion)
		return nil
	}

	// Backup current version (optional)
	if currentVersion != "" {
		if err := i.backupPlugin(name); err != nil {
			log.Warn("Failed to backup plugin", "error", err)
		}
	}

	// Install latest version
	return i.InstallPlugin(ctx, name, latestVersion)
}

// RemovePlugin removes a plugin from the plugin directory
func (i *Installer) RemovePlugin(ctx context.Context, name string) error {
	log.Info("Removing plugin", "name", name)

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
	manifestPath := filepath.Join(i.pluginDir, filepath.Base(name)+".json")
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove manifest: %w", err)
	}

	log.Info("Successfully removed plugin", "name", name)
	return nil
}

// GetAvailableVersions lists available versions for a plugin in the registry
func (i *Installer) GetAvailableVersions(ctx context.Context, name string) ([]string, error) {
	log.Debug("Fetching available versions", "name", name)

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
	log.Debug("Downloading manifest", "reference", ref)

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Warn("Failed to close manifest file", "error", err)
		}
	}()

	// Download manifest artifact
	manifestRef := fmt.Sprintf("%s-manifest", ref)
	if err := i.client.Pull(ctx, manifestRef, file); err != nil {
		// If manifest artifact doesn't exist, try fetching from main artifact
		_, _ = file.Seek(0, 0) // Reset file position
		_ = file.Truncate(0)   // Truncate file

		if err := i.client.Pull(ctx, ref, file); err != nil {
			return err
		}
	}

	return nil
}

// downloadBinary downloads the plugin binary from the registry
func (i *Installer) downloadBinary(ctx context.Context, ref, destPath, platform string) error {
	log.Debug("Downloading binary", "reference", ref, "platform", platform)

	if err := i.downloadBinaryFromManifest(ctx, ref, destPath, platform); err != nil {
		return fmt.Errorf("failed to download binary for platform %s: %w", platform, err)
	}

	return nil
}

func (i *Installer) downloadBinaryFromManifest(ctx context.Context, ref, destPath, platform string) error {
	manifestBytes, manifestDesc, err := i.client.GetManifest(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to retrieve manifest: %w", err)
	}

	switch manifestDesc.MediaType {
	case ocispec.MediaTypeImageIndex:
		return i.downloadBinaryFromIndex(ctx, ref, destPath, platform, manifestBytes)
	case ocispec.MediaTypeImageManifest, "application/vnd.oci.artifact.manifest.v1+json":
		return i.downloadBinaryFromSingleManifest(ctx, ref, destPath, manifestBytes)
	case "":
		// Media type may be empty; attempt to decode as index first, then manifest
		if err := i.downloadBinaryFromIndex(ctx, ref, destPath, platform, manifestBytes); err == nil {
			return nil
		}
		return i.downloadBinaryFromSingleManifest(ctx, ref, destPath, manifestBytes)
	default:
		if strings.Contains(manifestDesc.MediaType, "index") {
			return i.downloadBinaryFromIndex(ctx, ref, destPath, platform, manifestBytes)
		}
		return i.downloadBinaryFromSingleManifest(ctx, ref, destPath, manifestBytes)
	}
}

func (i *Installer) downloadBinaryFromIndex(ctx context.Context, ref, destPath, platform string, data []byte) error {
	var index ocispec.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("failed to parse index manifest: %w", err)
	}

	osName, archName := splitPlatform(platform)
	var selected *ocispec.Descriptor

	for idx := range index.Manifests {
		desc := &index.Manifests[idx]
		if matchesPlatform(desc, osName, archName) {
			selected = desc
			break
		}
	}

	if selected == nil {
		return fmt.Errorf("no artifact found for platform %s", platform)
	}

	manifestBytes, err := i.client.FetchDescriptor(ctx, ref, *selected)
	if err != nil {
		return fmt.Errorf("failed to fetch platform manifest: %w", err)
	}

	return i.downloadBinaryFromSingleManifest(ctx, ref, destPath, manifestBytes)
}

func (i *Installer) downloadBinaryFromSingleManifest(ctx context.Context, ref, destPath string, data []byte) (err error) {
	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	if len(manifest.Layers) == 0 {
		return fmt.Errorf("manifest contains no layers")
	}

	layer := manifest.Layers[0]
	if strings.EqualFold(layer.MediaType, mediaTypePluginArchive) || strings.EqualFold(layer.MediaType, mediaTypeArtifactArchive) {
		if err := i.downloadArchiveLayer(ctx, ref, layer, destPath); err != nil {
			return err
		}
		return nil
	}

	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Warn("Failed to close binary file", "error", closeErr)
		}
		if err != nil {
			_ = os.Remove(destPath)
		}
	}()

	if err = i.client.CopyDescriptor(ctx, ref, layer, file); err != nil {
		return fmt.Errorf("failed to download binary layer: %w", err)
	}

	return nil
}

func (i *Installer) downloadArchiveLayer(ctx context.Context, ref string, layer ocispec.Descriptor, destPath string) error {
	data, err := i.client.FetchDescriptor(ctx, ref, layer)
	if err != nil {
		return fmt.Errorf("failed to fetch archive layer: %w", err)
	}

	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure destination directory: %w", err)
	}

	if err := extractTarGz(bytes.NewReader(data), destDir); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	if _, err := os.Stat(destPath); err != nil {
		return fmt.Errorf("archive missing expected binary %s: %w", destPath, err)
	}

	return nil
}

func extractTarGz(reader io.Reader, destDir string) error {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			log.Warn("Failed to close gzip reader", "error", closeErr)
		}
	}()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read archive: %w", err)
		}

		targetPath := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(targetPath, filepath.Clean(destDir)+string(os.PathSeparator)) && filepath.Clean(targetPath) != filepath.Clean(destDir) {
			return fmt.Errorf("archive entry %s escapes destination", header.Name)
		}

		if header.Typeflag == 0 {
			header.Typeflag = tar.TypeReg
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				if !os.IsExist(err) {
					return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
				}
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directories for %s: %w", targetPath, err)
			}
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(file, tarReader); err != nil {
				_ = file.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("failed to close file %s: %w", targetPath, err)
			}
		default:
			// Skip unsupported entry types for now
			continue
		}
	}

	return nil
}

func splitPlatform(platform string) (string, string) {
	parts := strings.Split(platform, "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return runtime.GOOS, runtime.GOARCH
}

func matchesPlatform(desc *ocispec.Descriptor, osName, archName string) bool {
	if desc.Platform != nil {
		if strings.EqualFold(desc.Platform.OS, osName) && strings.EqualFold(desc.Platform.Architecture, archName) {
			return true
		}
	}

	if desc.Annotations != nil {
		osAnn := desc.Annotations["os"]
		archAnn := desc.Annotations["architecture"]
		if strings.EqualFold(osAnn, osName) && strings.EqualFold(archAnn, archName) {
			return true
		}
	}

	return false
}

// downloadSignature downloads the plugin signature from the registry
func (i *Installer) downloadSignature(ctx context.Context, ref, destPath, platform string) error {
	log.Debug("Downloading signature", "reference", ref, "platform", platform)

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Warn("Failed to close signature file", "error", err)
		}
	}()

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
	defer func() {
		if err := file.Close(); err != nil {
			log.Warn("Failed to close file for checksum verification", "error", err)
		}
	}()

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
	defer func() {
		if err := sourceFile.Close(); err != nil {
			log.Warn("Failed to close source file", "error", err)
		}
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			log.Warn("Failed to close destination file", "error", err)
		}
	}()

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
