package communication

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPluginRegistry(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)
	assert.NotNil(t, registry)
}

func TestPluginRegistryRegister(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	info := &PluginInfo{
		ID:           "test-plugin",
		Name:         "Test Plugin",
		Version:      "1.0.0",
		Capabilities: []string{"build", "test"},
		Metadata:     map[string]interface{}{"key": "value"},
	}

	err = registry.Register(ctx, info)
	require.NoError(t, err)

	// Verify registered
	retrieved, err := registry.Get(ctx, "test-plugin")
	require.NoError(t, err)
	assert.Equal(t, "Test Plugin", retrieved.Name)
	assert.Equal(t, "1.0.0", retrieved.Version)
	assert.Equal(t, PluginStatusRegistered, retrieved.Status)
}

func TestPluginRegistryGet(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	info := &PluginInfo{
		ID:      "test-plugin",
		Name:    "Test Plugin",
		Version: "1.0.0",
	}

	err = registry.Register(ctx, info)
	require.NoError(t, err)

	// Get plugin
	retrieved, err := registry.Get(ctx, "test-plugin")
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", retrieved.ID)
	assert.Equal(t, "Test Plugin", retrieved.Name)
}

func TestPluginRegistryGetNotFound(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = registry.Get(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")
}

func TestPluginRegistryUnregister(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	info := &PluginInfo{
		ID:      "test-plugin",
		Name:    "Test Plugin",
		Version: "1.0.0",
	}

	err = registry.Register(ctx, info)
	require.NoError(t, err)

	// Unregister
	err = registry.Unregister(ctx, "test-plugin")
	require.NoError(t, err)

	// Verify unregistered
	_, err = registry.Get(ctx, "test-plugin")
	assert.Error(t, err)
}

func TestPluginRegistryUpdateStatus(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	info := &PluginInfo{
		ID:      "test-plugin",
		Name:    "Test Plugin",
		Version: "1.0.0",
	}

	err = registry.Register(ctx, info)
	require.NoError(t, err)

	// Update status
	err = registry.UpdateStatus(ctx, "test-plugin", PluginStatusRunning)
	require.NoError(t, err)

	// Verify status updated
	retrieved, err := registry.Get(ctx, "test-plugin")
	require.NoError(t, err)
	assert.Equal(t, PluginStatusRunning, retrieved.Status)
}

func TestPluginRegistryList(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Register multiple plugins
	for i := 1; i <= 3; i++ {
		info := &PluginInfo{
			ID:      "test-plugin-" + string(rune('0'+i)),
			Name:    "Test Plugin",
			Version: "1.0.0",
		}
		err = registry.Register(ctx, info)
		require.NoError(t, err)
	}

	// List all plugins
	plugins := registry.List(ctx)
	assert.Equal(t, 3, len(plugins))
}

func TestPluginRegistryFindByCapability(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Register plugins with different capabilities
	err = registry.Register(ctx, &PluginInfo{
		ID:           "plugin1",
		Name:         "Plugin 1",
		Version:      "1.0.0",
		Capabilities: []string{"build", "test"},
	})
	require.NoError(t, err)

	err = registry.Register(ctx, &PluginInfo{
		ID:           "plugin2",
		Name:         "Plugin 2",
		Version:      "1.0.0",
		Capabilities: []string{"deploy"},
	})
	require.NoError(t, err)

	err = registry.Register(ctx, &PluginInfo{
		ID:           "plugin3",
		Name:         "Plugin 3",
		Version:      "1.0.0",
		Capabilities: []string{"build", "deploy"},
	})
	require.NoError(t, err)

	// Find plugins with "build" capability
	buildPlugins := registry.FindByCapability(ctx, "build")
	assert.Equal(t, 2, len(buildPlugins))

	// Find plugins with "deploy" capability
	deployPlugins := registry.FindByCapability(ctx, "deploy")
	assert.Equal(t, 2, len(deployPlugins))

	// Find plugins with "test" capability
	testPlugins := registry.FindByCapability(ctx, "test")
	assert.Equal(t, 1, len(testPlugins))
}

func TestPluginRegistryPersistence(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	ctx := context.Background()

	// Create registry and register plugin
	registry1, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	info := &PluginInfo{
		ID:      "test-plugin",
		Name:    "Test Plugin",
		Version: "1.0.0",
	}
	err = registry1.Register(ctx, info)
	require.NoError(t, err)

	// Create new registry from same directory
	registry2, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	// Verify plugin persisted
	retrieved, err := registry2.Get(ctx, "test-plugin")
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", retrieved.ID)
	assert.Equal(t, "Test Plugin", retrieved.Name)
}

func TestPluginRegistryFileStructure(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	info := &PluginInfo{
		ID:      "test-plugin",
		Name:    "Test Plugin",
		Version: "1.0.0",
	}

	err = registry.Register(ctx, info)
	require.NoError(t, err)

	// Verify registry.json exists
	registryFile := filepath.Join(tempDir, "registry.json")
	_, err = os.Stat(registryFile)
	assert.NoError(t, err)
}

func TestPluginRegistryEmptyList(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	plugins := registry.List(ctx)
	assert.Equal(t, 0, len(plugins))
}

func TestPluginRegistryFindByCapabilityNoMatches(t *testing.T) {
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	registry, err := NewPluginRegistry(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Register plugin without matching capability
	err = registry.Register(ctx, &PluginInfo{
		ID:           "plugin1",
		Name:         "Plugin 1",
		Version:      "1.0.0",
		Capabilities: []string{"build"},
	})
	require.NoError(t, err)

	// Find plugins with non-existent capability
	plugins := registry.FindByCapability(ctx, "deploy")
	assert.Equal(t, 0, len(plugins))
}
