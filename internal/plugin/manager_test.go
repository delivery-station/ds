package plugin

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/delivery-station/ds/pkg/types"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager("/tmp/plugins")
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.pluginDir != "/tmp/plugins" {
		t.Errorf("expected plugin dir /tmp/plugins, got %s", mgr.pluginDir)
	}
}

func TestDiscoverPlugins_EmptyDirectory(t *testing.T) {
	// Create temporary plugin directory
	tmpDir := t.TempDir()

	mgr := NewManager(tmpDir)
	err := mgr.DiscoverPlugins()
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	plugins, err := mgr.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins failed: %v", err)
	}

	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestDiscoverPlugins_MissingDirectory(t *testing.T) {
	mgr := NewManager("/nonexistent/directory")
	err := mgr.DiscoverPlugins()
	if err != nil {
		t.Fatalf("DiscoverPlugins should not fail on missing directory: %v", err)
	}

	plugins, err := mgr.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins failed: %v", err)
	}

	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins in missing directory, got %d", len(plugins))
	}
}

func TestDiscoverPlugins_WithMockPlugin(t *testing.T) {
	// Create temporary plugin directory
	tmpDir := t.TempDir()

	// Create a mock plugin binary
	pluginPath := filepath.Join(tmpDir, "ds-testplugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	// Create a simple script/binary
	content := "#!/bin/sh\necho 'testplugin version 1.0.0'\n"
	if err := os.WriteFile(pluginPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to create mock plugin: %v", err)
	}

	// Create plugin manifest
	manifestPath := filepath.Join(tmpDir, "ds-testplugin.yaml")
	manifestContent := `name: testplugin
version: 1.0.0
description: Test plugin for unit tests
commands:
  - name: test
    description: Test command
platform:
  os: [linux, darwin, windows]
  arch: [amd64, arm64]
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	// Discover plugins
	mgr := NewManager(tmpDir)
	err := mgr.DiscoverPlugins()
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	plugins, err := mgr.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins failed: %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	plugin := plugins[0]
	if plugin.Name != "testplugin" {
		t.Errorf("expected plugin name 'testplugin', got '%s'", plugin.Name)
	}

	// Version comes from --version output which includes prefix
	if !strings.Contains(plugin.Version, "1.0.0") {
		t.Errorf("expected version to contain '1.0.0', got '%s'", plugin.Version)
	}

	if plugin.Description != "Test plugin for unit tests" {
		t.Errorf("expected description 'Test plugin for unit tests', got '%s'", plugin.Description)
	}
}

func TestGetPlugin(t *testing.T) {
	// Create temporary plugin directory
	tmpDir := t.TempDir()

	// Create a mock plugin binary
	pluginPath := filepath.Join(tmpDir, "ds-myplugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	content := "#!/bin/sh\necho 'test'\n"
	if err := os.WriteFile(pluginPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to create mock plugin: %v", err)
	}

	mgr := NewManager(tmpDir)

	// Discover plugins first
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	// Get existing plugin
	plugin, err := mgr.GetPlugin("myplugin")
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}

	if plugin.Name != "myplugin" {
		t.Errorf("expected plugin name 'myplugin', got '%s'", plugin.Name)
	}

	// Get non-existent plugin
	_, err = mgr.GetPlugin("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestValidatePlugin(t *testing.T) {
	// Create temporary plugin directory
	tmpDir := t.TempDir()

	// Create a valid plugin
	pluginPath := filepath.Join(tmpDir, "ds-validplugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	content := "#!/bin/sh\necho 'test'\n"
	if err := os.WriteFile(pluginPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to create mock plugin: %v", err)
	}

	mgr := NewManager(tmpDir)
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	// Validate existing plugin
	err := mgr.ValidatePlugin("validplugin")
	if err != nil {
		t.Errorf("ValidatePlugin failed for valid plugin: %v", err)
	}

	// Validate non-existent plugin
	err = mgr.ValidatePlugin("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestIsCompatiblePlatform(t *testing.T) {
	mgr := NewManager("/tmp")

	tests := []struct {
		name       string
		plugin     *types.PluginInfo
		compatible bool
	}{
		{
			name: "no manifest - always compatible",
			plugin: &types.PluginInfo{
				Name: "test",
			},
			compatible: true,
		},
		{
			name: "empty platform - always compatible",
			plugin: &types.PluginInfo{
				Name: "test",
				Manifest: &types.PluginManifest{
					Platform: types.PluginPlatform{},
				},
			},
			compatible: true,
		},
		{
			name: "matching OS and arch",
			plugin: &types.PluginInfo{
				Name: "test",
				Manifest: &types.PluginManifest{
					Platform: types.PluginPlatform{
						OS:   []string{runtime.GOOS},
						Arch: []string{runtime.GOARCH},
					},
				},
			},
			compatible: true,
		},
		{
			name: "all platforms",
			plugin: &types.PluginInfo{
				Name: "test",
				Manifest: &types.PluginManifest{
					Platform: types.PluginPlatform{
						OS:   []string{"all"},
						Arch: []string{"all"},
					},
				},
			},
			compatible: true,
		},
		{
			name: "incompatible OS",
			plugin: &types.PluginInfo{
				Name: "test",
				Manifest: &types.PluginManifest{
					Platform: types.PluginPlatform{
						OS: []string{"invalid-os"},
					},
				},
			},
			compatible: false,
		},
		{
			name: "compatible OS, incompatible arch",
			plugin: &types.PluginInfo{
				Name: "test",
				Manifest: &types.PluginManifest{
					Platform: types.PluginPlatform{
						OS:   []string{runtime.GOOS},
						Arch: []string{"invalid-arch"},
					},
				},
			},
			compatible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.isCompatiblePlatform(tt.plugin)
			if result != tt.compatible {
				t.Errorf("expected compatible=%v, got %v", tt.compatible, result)
			}
		})
	}
}

func TestPluginCaching(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create a plugin
	pluginPath := filepath.Join(tmpDir, "ds-cacheplugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}
	content := "#!/bin/sh\necho 'test'\n"
	if err := os.WriteFile(pluginPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to create mock plugin: %v", err)
	}

	// First discovery
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("first DiscoverPlugins failed: %v", err)
	}

	plugins1, _ := mgr.ListPlugins()
	if len(plugins1) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins1))
	}

	// Create another plugin
	pluginPath2 := filepath.Join(tmpDir, "ds-newplugin")
	if runtime.GOOS == "windows" {
		pluginPath2 += ".exe"
	}
	if err := os.WriteFile(pluginPath2, []byte(content), 0755); err != nil {
		t.Fatalf("failed to create second plugin: %v", err)
	}

	// Second discovery should use cache (won't see new plugin)
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("second DiscoverPlugins failed: %v", err)
	}

	plugins2, _ := mgr.ListPlugins()
	if len(plugins2) != 1 {
		t.Errorf("expected cache to return 1 plugin, got %d", len(plugins2))
	}

	// Invalidate cache and rediscover
	mgr.InvalidateCache()
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("third DiscoverPlugins failed: %v", err)
	}

	plugins3, _ := mgr.ListPlugins()
	if len(plugins3) != 2 {
		t.Errorf("expected 2 plugins after cache invalidation, got %d", len(plugins3))
	}
}

func TestLoadPluginManifest(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create plugin binary
	pluginPath := filepath.Join(tmpDir, "ds-manifesttest")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}
	content := "#!/bin/sh\necho 'test'\n"
	if err := os.WriteFile(pluginPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to create plugin: %v", err)
	}

	// Test: no manifest
	_, err := mgr.loadPluginManifest(pluginPath)
	if err == nil {
		t.Error("expected error when no manifest exists")
	}

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-manifesttest.yaml")
	manifestContent := `name: manifesttest
version: 2.0.0
description: Manifest test plugin
commands:
  - name: cmd1
    description: Command 1
  - name: cmd2
    description: Command 2
platform:
  os: [linux, darwin]
  arch: [amd64]
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	// Load manifest
	manifest, err := mgr.loadPluginManifest(pluginPath)
	if err != nil {
		t.Fatalf("loadPluginManifest failed: %v", err)
	}

	if manifest.Name != "manifesttest" {
		t.Errorf("expected name 'manifesttest', got '%s'", manifest.Name)
	}

	if manifest.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got '%s'", manifest.Version)
	}

	if len(manifest.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(manifest.Commands))
	}

	if len(manifest.Platform.OS) != 2 {
		t.Errorf("expected 2 OS entries, got %d", len(manifest.Platform.OS))
	}

	if len(manifest.Platform.Arch) != 1 {
		t.Errorf("expected 1 Arch entry, got %d", len(manifest.Platform.Arch))
	}
}
