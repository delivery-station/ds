// +build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/delivery-station/ds/internal/config"
	"github.com/delivery-station/ds/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientIntegration tests the complete client workflow
func TestClientIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create test configuration
	cfg := &config.Config{
		Registry: config.RegistryConfig{
			Default:            "ghcr.io",
			InsecureRegistries: []string{},
		},
		Cache: config.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: "1GB",
			TTL:     "1h",
		},
		Plugins: config.PluginsConfig{
			Dir:         filepath.Join(tmpDir, "plugins"),
			AutoInstall: true,
		},
		Auth: config.AuthConfig{
			DockerConfig: filepath.Join(os.Getenv("HOME"), ".docker", "config.json"),
		},
	}

	// Create client
	dsClient, err := client.NewClient(
		client.WithConfig(cfg),
	)
	require.NoError(t, err)
	defer dsClient.Close()

	// Test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("list plugins", func(t *testing.T) {
		plugins := dsClient.ListPlugins()
		// Should have at least discovered any installed plugins
		assert.NotNil(t, plugins)
	})

	t.Run("event bus", func(t *testing.T) {
		eventBus := dsClient.EventBus()
		assert.NotNil(t, eventBus)

		// Test subscribing to events
		received := make(chan bool, 1)
		eventBus.Subscribe("test.event", func(data interface{}) {
			received <- true
		})

		eventBus.Publish("test.event", "test data")

		select {
		case <-received:
			// Success
		case <-time.After(time.Second):
			t.Error("Did not receive event")
		}
	})

	t.Run("state management", func(t *testing.T) {
		state := dsClient.GetState()
		assert.NotNil(t, state)

		// Set and get state
		state.Set("test.key", "test.value")
		
		value, exists := state.Get("test.key")
		assert.True(t, exists)
		assert.Equal(t, "test.value", value)
	})

	// Note: These tests require actual registry access
	// They can be enabled when testing with a real registry

	t.Run("pull artifact - requires registry", func(t *testing.T) {
		if os.Getenv("INTEGRATION_REGISTRY_TEST") != "true" {
			t.Skip("Skipping registry test - set INTEGRATION_REGISTRY_TEST=true to enable")
		}

		// This would pull an actual artifact
		_, err := dsClient.Pull(ctx, "ghcr.io/delivery-station/test:latest")
		// We expect this to fail without proper authentication or if artifact doesn't exist
		// The test is here to verify the integration flow works
		t.Logf("Pull result: %v", err)
	})

	t.Run("list artifacts - requires registry", func(t *testing.T) {
		if os.Getenv("INTEGRATION_REGISTRY_TEST") != "true" {
			t.Skip("Skipping registry test - set INTEGRATION_REGISTRY_TEST=true to enable")
		}

		// This would list artifacts from a real repository
		_, err := dsClient.List(ctx, "ghcr.io/delivery-station/test")
		t.Logf("List result: %v", err)
	})
}

// TestPluginIntegration tests plugin discovery and execution
func TestPluginIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Registry: config.RegistryConfig{
			Default: "ghcr.io",
		},
		Cache: config.CacheConfig{
			Dir: filepath.Join(tmpDir, "cache"),
		},
		Plugins: config.PluginsConfig{
			Dir:         filepath.Join(tmpDir, "plugins"),
			AutoInstall: false, // Don't auto-install for this test
		},
	}

	dsClient, err := client.NewClient(
		client.WithConfig(cfg),
	)
	require.NoError(t, err)
	defer dsClient.Close()

	t.Run("plugin discovery", func(t *testing.T) {
		plugins := dsClient.ListPlugins()
		// Should be empty since we don't have any plugins installed
		assert.Equal(t, 0, len(plugins))
	})

	t.Run("register plugin callback", func(t *testing.T) {
		called := false
		dsClient.RegisterPlugin("test-plugin", func(args []string) (int, error) {
			called = true
			return 0, nil
		})

		// Verify callback was registered by executing it
		exitCode, err := dsClient.ExecutePlugin("test-plugin", []string{})
		assert.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.True(t, called)
	})
}

// TestCacheIntegration tests cache operations
func TestCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: "100MB",
			TTL:     "1h",
		},
		Plugins: config.PluginsConfig{
			Dir: filepath.Join(tmpDir, "plugins"),
		},
	}

	dsClient, err := client.NewClient(
		client.WithConfig(cfg),
	)
	require.NoError(t, err)
	defer dsClient.Close()

	cache := dsClient.Cache()
	require.NotNil(t, cache)

	t.Run("store and retrieve", func(t *testing.T) {
		key := "test-key"
		value := []byte("test-value")

		// Store
		err := cache.Store(key, value)
		require.NoError(t, err)

		// Retrieve
		retrieved, err := cache.Retrieve(key)
		require.NoError(t, err)
		assert.Equal(t, value, retrieved)

		// Check exists
		exists := cache.Exists(key)
		assert.True(t, exists)
	})

	t.Run("delete", func(t *testing.T) {
		key := "test-delete"
		value := []byte("to-be-deleted")

		err := cache.Store(key, value)
		require.NoError(t, err)

		err = cache.Delete(key)
		require.NoError(t, err)

		exists := cache.Exists(key)
		assert.False(t, exists)
	})
}
