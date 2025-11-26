package communication

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateStore(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)
	assert.NotNil(t, store)
	assert.Equal(t, tempDir, store.storeDir)
}

func TestStateStoreSetAndGet(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	key := "test-key"
	value := map[string]interface{}{
		"foo": "bar",
		"num": 42,
	}

	// Set state
	err = store.Set(ctx, key, value, "plugin1", nil)
	require.NoError(t, err)

	// Get state
	entry, err := store.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, key, entry.Key)
	assert.Equal(t, "plugin1", entry.PluginID)
	assert.Equal(t, "bar", entry.Value["foo"])
	assert.Equal(t, 42, entry.Value["num"])
	assert.Equal(t, 1, entry.AccessCount)
}

func TestStateStoreGetNotFound(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = store.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrStateNotFound)
}

func TestStateStoreUpdate(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	key := "test-key"

	// Set initial state
	err = store.Set(ctx, key, map[string]interface{}{"v": 1}, "plugin1", nil)
	require.NoError(t, err)

	// Update state
	err = store.Set(ctx, key, map[string]interface{}{"v": 2}, "plugin1", nil)
	require.NoError(t, err)

	// Get state
	entry, err := store.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, 2, entry.Value["v"])
}

func TestStateStoreDelete(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	key := "test-key"

	// Set state
	err = store.Set(ctx, key, map[string]interface{}{"v": 1}, "plugin1", nil)
	require.NoError(t, err)

	// Delete state
	err = store.Delete(ctx, key)
	require.NoError(t, err)

	// Verify deleted
	_, err = store.Get(ctx, key)
	assert.ErrorIs(t, err, ErrStateNotFound)
}

func TestStateStoreList(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Set multiple states
	err = store.Set(ctx, "key1", map[string]interface{}{"v": 1}, "plugin1", nil)
	require.NoError(t, err)
	err = store.Set(ctx, "key2", map[string]interface{}{"v": 2}, "plugin2", nil)
	require.NoError(t, err)

	// List all states
	entries := store.List(ctx)
	assert.Equal(t, 2, len(entries))
}

func TestStateStoreListByPlugin(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Set states for different plugins
	err = store.Set(ctx, "key1", map[string]interface{}{"v": 1}, "plugin1", nil)
	require.NoError(t, err)
	err = store.Set(ctx, "key2", map[string]interface{}{"v": 2}, "plugin1", nil)
	require.NoError(t, err)
	err = store.Set(ctx, "key3", map[string]interface{}{"v": 3}, "plugin2", nil)
	require.NoError(t, err)

	// List states for plugin1
	entries := store.ListByPlugin(ctx, "plugin1")
	assert.Equal(t, 2, len(entries))

	// List states for plugin2
	entries = store.ListByPlugin(ctx, "plugin2")
	assert.Equal(t, 1, len(entries))
}

func TestStateStoreTTL(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	key := "test-key"
	ttl := 100 * time.Millisecond

	// Set state with TTL
	err = store.Set(ctx, key, map[string]interface{}{"v": 1}, "plugin1", &ttl)
	require.NoError(t, err)

	// Get immediately - should succeed
	_, err = store.Get(ctx, key)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Get after expiration - should fail
	_, err = store.Get(ctx, key)
	assert.ErrorIs(t, err, ErrStateExpired)
}

func TestStateStoreClean(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	ttl := 100 * time.Millisecond

	// Set states with TTL
	err = store.Set(ctx, "key1", map[string]interface{}{"v": 1}, "plugin1", &ttl)
	require.NoError(t, err)
	err = store.Set(ctx, "key2", map[string]interface{}{"v": 2}, "plugin1", nil)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Clean expired entries
	removed, err := store.Clean(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	// Verify only non-expired entry remains
	entries := store.List(ctx)
	assert.Equal(t, 1, len(entries))
}

func TestStateStorePersistence(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	ctx := context.Background()
	key := "test-key"
	value := map[string]interface{}{"v": 1}

	// Create store and set state
	store1, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)
	err = store1.Set(ctx, key, value, "plugin1", nil)
	require.NoError(t, err)

	// Create new store from same directory
	store2, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	// Verify state persisted
	entry, err := store2.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, key, entry.Key)
	assert.Equal(t, float64(1), entry.Value["v"])
}

func TestStateStoreAccessTracking(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()
	key := "test-key"

	// Set state
	err = store.Set(ctx, key, map[string]interface{}{"v": 1}, "plugin1", nil)
	require.NoError(t, err)

	// Get multiple times
	for i := 0; i < 5; i++ {
		entry, err := store.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, i+1, entry.AccessCount)
	}
}

func TestStateStoreFileStructure(t *testing.T) {
	tempDir := t.TempDir()
	logger := hclog.NewNullLogger()

	store, err := NewStateStore(tempDir, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Set state
	err = store.Set(ctx, "test-key", map[string]interface{}{"v": 1}, "plugin1", nil)
	require.NoError(t, err)

	// Verify state.json exists
	stateFile := filepath.Join(tempDir, "state.json")
	_, err = os.Stat(stateFile)
	assert.NoError(t, err)
}
