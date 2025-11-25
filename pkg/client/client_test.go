package client

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/delivery-station/ds/internal/cache"
	"github.com/delivery-station/ds/internal/communication"
	"github.com/delivery-station/ds/internal/plugin"
	"github.com/delivery-station/ds/internal/registry"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/sirupsen/logrus"
)

func TestNewClient(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}
	
	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if client.config == nil {
		t.Error("Config should not be nil")
	}
	if client.cache == nil {
		t.Error("Cache should not be nil")
	}
	if client.registry == nil {
		t.Error("Registry should not be nil")
	}
	if client.pluginMgr == nil {
		t.Error("PluginManager should not be nil")
	}
	if client.eventBus == nil {
		t.Error("EventBus should not be nil")
	}
	if client.stateStore == nil {
		t.Error("StateStore should not be nil")
	}
	if client.pluginRegistry == nil {
		t.Error("PluginRegistry should not be nil")
	}
}

func TestNewClient_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir: filepath.Join(tmpDir, "cache"),
			TTL: time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
	}

	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if client.config != cfg {
		t.Error("Config should match provided config")
	}
}

func TestNewClient_WithCache(t *testing.T) {
	tmpDir := t.TempDir()
	logger := logrus.New()
	customCache, err := cache.NewCache(
		filepath.Join(tmpDir, "cache"),
		1024*1024*100, // 100MB
		time.Hour,
		logger,
	)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}

	client, err := NewClient(WithConfig(cfg), WithCache(customCache))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if client.cache != customCache {
		t.Error("Cache should match provided cache")
	}
}

func TestNewClient_WithRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir: tmpDir,
			TTL: time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: tmpDir,
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}
	
	customRegistry, err := registry.NewClient("ghcr.io", nil)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	client, err := NewClient(WithConfig(cfg), WithRegistry(customRegistry))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if client.registry != customRegistry {
		t.Error("Registry should match provided registry")
	}
}

func TestNewClient_WithPluginManager(t *testing.T) {
	tmpDir := t.TempDir()
	customPluginMgr := plugin.NewManager(filepath.Join(tmpDir, "plugins"))
	
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}

	client, err := NewClient(WithConfig(cfg), WithPluginManager(customPluginMgr))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if client.pluginMgr != customPluginMgr {
		t.Error("PluginManager should match provided plugin manager")
	}
}

func TestClient_PullAndPush(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}

	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Note: This test requires network access and will likely fail
	// It's here to demonstrate the API
	t.Run("Pull_NonExistent", func(t *testing.T) {
		t.Skip("Skipping network test - requires actual registry access")
		err := client.Pull(ctx, "nonexistent/artifact:latest", os.Stdout)
		if err == nil {
			t.Error("Expected error for nonexistent artifact")
		}
	})
}

func TestClient_List(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}
	
	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Note: This test requires network access
	t.Run("List_NonExistent", func(t *testing.T) {
		t.Skip("Skipping network test - requires actual registry access")
		_, err := client.List(ctx, "nonexistent/repository")
		if err == nil {
			t.Error("Expected error for nonexistent repository")
		}
	})
}

func TestClient_ListPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir: filepath.Join(tmpDir, "cache"),
			TTL: time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
	}

	// Create plugins directory
	if err := os.MkdirAll(cfg.Plugins.Dir, 0755); err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}

	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	plugins, err := client.ListPlugins()
	if err != nil {
		t.Fatalf("Failed to list plugins: %v", err)
	}

	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(plugins))
	}
}

func TestClient_EventBus(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}
	
	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	var mu sync.Mutex
	received := false
	handler := func(ctx context.Context, event *communication.Event) error {
		mu.Lock()
		received = true
		mu.Unlock()
		return nil
	}

	client.Subscribe(communication.EventCustom, handler)

	ctx := context.Background()
	data := map[string]interface{}{"message": "test"}
	if err := client.Publish(ctx, communication.EventCustom, "test-plugin", data); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	wasReceived := received
	mu.Unlock()
	
	if !wasReceived {
		t.Error("Event handler was not called")
	}
}

func TestClient_StateManagement(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}
	
	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test setting and getting state
	testState := map[string]interface{}{
		"key1": "value1",
		"key2": float64(42), // JSON numbers are float64
	}
	
	if err := client.SetState(ctx, "test.key", "test-plugin", testState, nil); err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	value, err := client.GetState(ctx, "test.key")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if value["key1"] != "value1" {
		t.Errorf("Expected 'value1', got %v", value["key1"])
	}
}

func TestClient_PluginRegistration(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}
	
	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Simplified test - just check the method doesn't panic
	err = client.RegisterPlugin(ctx, "test-plugin", "1.0.0", "/path/to/plugin")
	if err == nil {
		t.Skip("RegisterPlugin not yet implemented")
	}

	err = client.DiscoverPlugin(ctx, "test-plugin")
	if err == nil {
		t.Skip("DiscoverPlugin not yet implemented")
	}
}

func TestClient_Accessors(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}
	
	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if client.Config() == nil {
		t.Error("Config() should not return nil")
	}
	if client.Cache() == nil {
		t.Error("Cache() should not return nil")
	}
	if client.Registry() == nil {
		t.Error("Registry() should not return nil")
	}
	if client.PluginManager() == nil {
		t.Error("PluginManager() should not return nil")
	}
	if client.EventBus() == nil {
		t.Error("EventBus() should not return nil")
	}
	if client.StateStore() == nil {
		t.Error("StateStore() should not return nil")
	}
	if client.PluginRegistry() == nil {
		t.Error("PluginRegistry() should not return nil")
	}
}

func TestClient_Close(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 100,
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
		Registry: types.RegistryConfig{
			Default: "ghcr.io",
		},
	}
	
	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}
