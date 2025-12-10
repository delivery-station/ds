package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/delivery-station/ds/pkg/log"
	pkgplugin "github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Manager handles plugin discovery and management
type Manager struct {
	pluginDir string
	plugins   map[string]*types.PluginInfo
	lastScan  time.Time
	cacheTTL  time.Duration
	verifier  *SignatureVerifier
	mu        sync.RWMutex
}

var (
	errManifestRPCUnsupported = errors.New("plugin manifest RPC not supported")
)

const manifestRPCDeadline = 5 * time.Second

// NewManager creates a new plugin manager
func NewManager(pluginDir string) *Manager {
	return &Manager{
		pluginDir: pluginDir,
		plugins:   make(map[string]*types.PluginInfo),
		cacheTTL:  5 * time.Minute, // Cache plugin info for 5 minutes
		verifier:  nil,             // Will be set via SetSignatureVerifier
	}
}

// SetSignatureVerifier sets the signature verifier for the manager
func (m *Manager) SetSignatureVerifier(verifier *SignatureVerifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifier = verifier
}

// DiscoverPlugins scans the plugin directory for installed plugins
func (m *Manager) DiscoverPlugins() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we need to rescan (cache expired)
	if time.Since(m.lastScan) < m.cacheTTL && len(m.plugins) > 0 {
		log.Debug("Using cached plugin list")
		return nil
	}

	// Check if plugin directory exists
	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		log.Debug("Plugin directory does not exist", "dir", m.pluginDir)
		m.plugins = make(map[string]*types.PluginInfo)
		m.lastScan = time.Now()
		return nil
	}

	// Clear existing plugins
	m.plugins = make(map[string]*types.PluginInfo)

	// Read directory
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	// Scan for ds-* binaries
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Check if it's a plugin binary (ds-*)
		if !strings.HasPrefix(name, "ds-") {
			continue
		}

		// Skip manifest files
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			continue
		}

		// Remove ds- prefix and any extension
		pluginName := strings.TrimPrefix(name, "ds-")
		// Remove common extensions
		pluginName = strings.TrimSuffix(pluginName, ".exe")
		pluginName = strings.TrimSuffix(pluginName, ".sh")

		// Full path to plugin
		pluginPath := filepath.Join(m.pluginDir, name)

		// Check if executable
		info, err := os.Stat(pluginPath)
		if err != nil {
			log.Warn("Failed to stat plugin", "name", name, "error", err)
			continue
		}

		// On Unix-like systems, check executable bit
		if runtime.GOOS != "windows" {
			if info.Mode()&0111 == 0 {
				log.Debug("Skipping non-executable file", "name", name)
				continue
			}
		}

		// Create plugin info
		pluginInfo := &types.PluginInfo{
			Name: pluginName,
			Path: pluginPath,
		}

		// Try to get version from plugin
		version, err := m.getPluginVersion(pluginPath)
		if err != nil {
			log.Debug("Failed to get version", "plugin", pluginName, "error", err)
			pluginInfo.Version = "unknown"
		} else {
			pluginInfo.Version = version
		}

		// Try to load manifest
		manifest, err := m.loadPluginManifest(pluginPath)
		if err != nil {
			log.Debug("No manifest found", "plugin", pluginName, "error", err)
		} else {
			pluginInfo.Manifest = manifest
			pluginInfo.Description = manifest.Description

			// Override version from manifest if available and plugin version failed
			if pluginInfo.Version == "unknown" && manifest.Version != "" {
				pluginInfo.Version = manifest.Version
			}
		}

		// Validate platform compatibility
		if !m.isCompatiblePlatform(pluginInfo) {
			log.Warn("Plugin is not compatible with current platform",
				"plugin", pluginName, "os", runtime.GOOS, "arch", runtime.GOARCH)
			continue
		}

		// Verify signature if verifier is configured
		if m.verifier != nil {
			if err := m.verifier.VerifyPlugin(pluginPath); err != nil {
				log.Warn("Plugin failed signature verification", "plugin", pluginName, "error", err)
				// In strict mode, skip the plugin
				if m.verifier.config.Mode == SignatureModeStrict {
					continue
				}
			}
		}

		m.plugins[pluginName] = pluginInfo
		log.Debug("Discovered plugin", "plugin", pluginName, "version", pluginInfo.Version)
	}

	m.lastScan = time.Now()
	log.Info("Discovered plugin(s)", "count", len(m.plugins))

	return nil
}

// ListPlugins returns all discovered plugins
func (m *Manager) ListPlugins() ([]*types.PluginInfo, error) {
	// Ensure plugins are discovered
	if err := m.DiscoverPlugins(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*types.PluginInfo, 0, len(m.plugins))
	for _, plugin := range m.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// GetPlugin returns information about a specific plugin
func (m *Manager) GetPlugin(name string) (*types.PluginInfo, error) {
	// Ensure plugins are discovered
	if err := m.DiscoverPlugins(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin '%s' not found", name)
	}

	return plugin, nil
}

// ValidatePlugin checks if a plugin is valid and executable
func (m *Manager) ValidatePlugin(name string) error {
	plugin, err := m.GetPlugin(name)
	if err != nil {
		return err
	}

	// Check if binary exists
	if _, err := os.Stat(plugin.Path); err != nil {
		return fmt.Errorf("plugin binary not found: %w", err)
	}

	// Check if executable on Unix-like systems
	if runtime.GOOS != "windows" {
		info, err := os.Stat(plugin.Path)
		if err != nil {
			return fmt.Errorf("failed to stat plugin: %w", err)
		}

		if info.Mode()&0111 == 0 {
			return fmt.Errorf("plugin is not executable")
		}
	}

	// Check platform compatibility
	if !m.isCompatiblePlatform(plugin) {
		return fmt.Errorf("plugin is not compatible with %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return nil
}

// getPluginVersion attempts to get the plugin version by calling it with --version
func (m *Manager) getPluginVersion(pluginPath string) (string, error) {
	output, err := exec.Command(pluginPath, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	// Parse version from output (first line)
	version := strings.TrimSpace(string(output))
	if idx := strings.Index(version, "\n"); idx > 0 {
		version = version[:idx]
	}

	return version, nil
}

// loadPluginManifest loads the plugin manifest file

func (m *Manager) loadPluginManifest(pluginPath string) (*types.PluginManifest, error) {
	manifest, err := m.fetchPluginManifest(pluginPath)
	if err != nil {
		if errors.Is(err, errManifestRPCUnsupported) {
			log.Debug("Plugin does not expose manifest RPC", "path", pluginPath)
			return nil, errManifestRPCUnsupported
		}
		log.Debug("Failed to fetch manifest via RPC", "path", pluginPath, "error", err)
		return nil, err
	}

	return manifest, nil
}

func (m *Manager) fetchPluginManifest(pluginPath string) (*types.PluginManifest, error) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: pkgplugin.Handshake,
		Plugins:         pkgplugin.PluginMap,
		Cmd:             exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
		Logger: hclog.New(&hclog.LoggerOptions{
			Name:   "ds-plugin-manifest",
			Output: os.Stderr,
			Level:  hclog.Error,
		}),
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to plugin: %w", err)
	}

	raw, err := rpcClient.Dispense("ds-plugin")
	if err != nil {
		return nil, fmt.Errorf("failed to dispense plugin: %w", err)
	}

	manifestClient, ok := raw.(interface {
		GetManifest(context.Context) (*types.PluginManifest, error)
	})
	if !ok {
		return nil, errManifestRPCUnsupported
	}

	ctx, cancel := context.WithTimeout(context.Background(), manifestRPCDeadline)
	defer cancel()

	manifest, err := manifestClient.GetManifest(ctx)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return nil, errManifestRPCUnsupported
		}
		return nil, fmt.Errorf("manifest RPC call failed: %w", err)
	}

	if manifest == nil {
		return nil, fmt.Errorf("plugin returned empty manifest")
	}

	clone := &types.PluginManifest{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
	}

	if len(manifest.Commands) > 0 {
		clone.Commands = make([]types.PluginCommand, len(manifest.Commands))
		copy(clone.Commands, manifest.Commands)
	}

	if len(manifest.Platform.OS) > 0 {
		clone.Platform.OS = append([]string{}, manifest.Platform.OS...)
	}
	if len(manifest.Platform.Arch) > 0 {
		clone.Platform.Arch = append([]string{}, manifest.Platform.Arch...)
	}

	if len(manifest.Annotations) > 0 {
		clone.Annotations = make(map[string]string, len(manifest.Annotations))
		for k, v := range manifest.Annotations {
			clone.Annotations[k] = v
		}
	}

	return clone, nil
}

// isCompatiblePlatform checks if the plugin is compatible with current platform
func (m *Manager) isCompatiblePlatform(plugin *types.PluginInfo) bool {
	// If no manifest or no platform info, assume compatible
	if plugin.Manifest == nil || len(plugin.Manifest.Platform.OS) == 0 {
		return true
	}

	// Check OS compatibility
	osCompatible := false
	for _, os := range plugin.Manifest.Platform.OS {
		if os == runtime.GOOS || os == "all" {
			osCompatible = true
			break
		}
	}

	if !osCompatible {
		return false
	}

	// Check architecture compatibility
	if len(plugin.Manifest.Platform.Arch) == 0 {
		return true
	}

	for _, arch := range plugin.Manifest.Platform.Arch {
		if arch == runtime.GOARCH || arch == "all" {
			return true
		}
	}

	return false
}

// InvalidateCache forces a rescan on next operation
func (m *Manager) InvalidateCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastScan = time.Time{}
}

// SetPluginDir updates the plugin directory
func (m *Manager) SetPluginDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginDir = dir
	m.lastScan = time.Time{} // Invalidate cache
}
