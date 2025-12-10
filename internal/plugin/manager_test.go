package plugin

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestIsCompatiblePlatform(t *testing.T) {
	mgr := NewManager(t.TempDir())

	tests := []struct {
		name       string
		plugin     *types.PluginInfo
		compatible bool
	}{
		{
			name: "no manifest",
			plugin: &types.PluginInfo{
				Name: "test",
			},
			compatible: true,
		},
		{
			name: "no platform info",
			plugin: &types.PluginInfo{
				Name:     "test",
				Platform: types.PluginPlatform{},
			},
			compatible: true,
		},
		{
			name: "compatible platform",
			plugin: &types.PluginInfo{
				Name: "test",
				Platform: types.PluginPlatform{
					OS:   []string{runtime.GOOS},
					Arch: []string{runtime.GOARCH},
				},
			},
			compatible: true,
		},
		{
			name: "incompatible OS",
			plugin: &types.PluginInfo{
				Name: "test",
				Platform: types.PluginPlatform{
					OS: []string{"invalid-os"},
				},
			},
			compatible: false,
		},
		{
			name: "compatible OS, incompatible arch",
			plugin: &types.PluginInfo{
				Name: "test",
				Platform: types.PluginPlatform{
					OS:   []string{runtime.GOOS},
					Arch: []string{"invalid-arch"},
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

	if plugin.Version != "unknown" {
		t.Errorf("expected version to default to 'unknown', got '%s'", plugin.Version)
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
