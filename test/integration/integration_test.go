//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/delivery-station/ds/internal/communication"
	"github.com/delivery-station/ds/pkg/client"
	"github.com/delivery-station/ds/pkg/types"
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
	cfg := &types.Config{
		Registry: types.RegistryConfig{
			Default:            "ghcr.io",
			InsecureRegistries: []string{},
		},
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 1024 * 1024 * 1024, // 1GB in bytes
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
			Dir:         filepath.Join(tmpDir, "plugins"),
			AutoInstall: true,
		},
		Auth: types.AuthConfig{
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
		plugins, err := dsClient.ListPlugins()
		require.NoError(t, err)
		// Should have at least discovered any installed plugins
		assert.NotNil(t, plugins)
	})

	t.Run("event bus", func(t *testing.T) {
		eventBus := dsClient.EventBus()
		assert.NotNil(t, eventBus)

		// Test subscribing to events
		received := make(chan bool, 1)
		eventBus.Subscribe(communication.EventCustom, func(ctx context.Context, event *communication.Event) error {
			received <- true
			return nil
		})

		err := eventBus.Publish(ctx, &communication.Event{
			Type: communication.EventCustom,
			Data: map[string]interface{}{"test": "test data"},
		})
		require.NoError(t, err)

		select {
		case <-received:
			// Success
		case <-time.After(time.Second):
			t.Error("Did not receive event")
		}
	})

	t.Run("state management", func(t *testing.T) {
		stateStore := dsClient.StateStore()
		assert.NotNil(t, stateStore)

		// Set state
		testValue := map[string]interface{}{"key": "test.value"}
		err := dsClient.SetState(ctx, "test.key", "test-plugin", testValue, nil)
		require.NoError(t, err)

		// Get state
		value, err := dsClient.GetState(ctx, "test.key")
		require.NoError(t, err)
		assert.Equal(t, testValue, value)
	})

	// Note: These tests require actual registry access
	// They can be enabled when testing with a real registry

	t.Run("pull artifact - requires registry", func(t *testing.T) {
		if os.Getenv("INTEGRATION_REGISTRY_TEST") != "true" {
			t.Skip("Skipping registry test - set INTEGRATION_REGISTRY_TEST=true to enable")
		}

		// This would pull an actual artifact
		err := dsClient.Pull(ctx, "ghcr.io/delivery-station/test:latest", os.Stdout)
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

	cfg := &types.Config{
		Registry: types.RegistryConfig{
			Default: "ghcr.io/delivery-station",
		},
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 100 * 1024 * 1024, // 100MB in bytes
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
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
		plugins, err := dsClient.ListPlugins()
		require.NoError(t, err)
		// Should be empty since we don't have any plugins installed
		assert.Equal(t, 0, len(plugins))
	})

	t.Run("plugin execution", func(t *testing.T) {
		// Note: This would require an actual plugin to be installed
		// For now, we just test that the API is accessible
		_, err := dsClient.ExecutePlugin("non-existent-plugin", "run", nil)
		// We expect this to fail since plugin doesn't exist
		assert.Error(t, err)
	})
}

// TestCacheIntegration tests cache operations
func TestCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	cfg := &types.Config{
		Cache: types.CacheConfig{
			Dir:     filepath.Join(tmpDir, "cache"),
			MaxSize: 100 * 1024 * 1024, // 100MB in bytes
			TTL:     time.Hour,
		},
		Plugins: types.PluginsConfig{
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
		ctx := context.Background()
		reference := "test-reference"
		value := []byte("test-value")

		// Store (Put)
		entry, err := cache.Put(ctx, reference, bytes.NewReader(value))
		require.NoError(t, err)
		assert.NotNil(t, entry)

		// Retrieve (Get)
		retrievedEntry, err := cache.Get(ctx, reference)
		require.NoError(t, err)
		assert.Equal(t, reference, retrievedEntry.Reference)
		assert.Greater(t, retrievedEntry.Size, int64(0))
	})

	t.Run("delete", func(t *testing.T) {
		ctx := context.Background()
		reference := "test-delete"
		value := []byte("to-be-deleted")

		// Put entry first
		entry, err := cache.Put(ctx, reference, bytes.NewReader(value))
		require.NoError(t, err)

		// Remove using the key from the entry
		err = cache.Remove(entry.Key)
		require.NoError(t, err)

		// Verify it's gone
		_, err = cache.Get(ctx, reference)
		assert.Error(t, err)
	})
}
