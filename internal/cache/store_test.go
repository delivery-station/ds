package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cache.db")
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	store, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store.path != storePath {
		t.Errorf("expected path %s, got %s", storePath, store.path)
	}
}

func TestStorePutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cache.db")
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	store, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Create test entry
	now := time.Now()
	entry := &CacheEntry{
		Key:         "test-key",
		Reference:   "ghcr.io/test/artifact:v1.0.0",
		ContentPath: "/tmp/content",
		Size:        1024,
		CreatedAt:   now,
		AccessedAt:  now,
		AccessCount: 0,
	}

	// Put entry
	if err := store.Put(entry); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get entry
	retrieved, err := store.Get("test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Key != entry.Key {
		t.Errorf("expected key %s, got %s", entry.Key, retrieved.Key)
	}

	if retrieved.Reference != entry.Reference {
		t.Errorf("expected reference %s, got %s", entry.Reference, retrieved.Reference)
	}

	if retrieved.Size != entry.Size {
		t.Errorf("expected size %d, got %d", entry.Size, retrieved.Size)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cache.db")
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	store, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Try to get non-existent entry
	_, err = store.Get("nonexistent-key")
	if err == nil {
		t.Error("expected error for non-existent entry")
	}
}

func TestStoreUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cache.db")
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	store, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Create and put entry
	now := time.Now()
	entry := &CacheEntry{
		Key:         "test-key",
		Reference:   "ghcr.io/test/artifact:v1.0.0",
		ContentPath: "/tmp/content",
		Size:        1024,
		CreatedAt:   now,
		AccessedAt:  now,
		AccessCount: 0,
	}

	if err := store.Put(entry); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Update entry
	entry.AccessCount = 5
	entry.AccessedAt = time.Now()

	if err := store.Update(entry); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Get and verify update
	retrieved, err := store.Get("test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.AccessCount != 5 {
		t.Errorf("expected access count 5, got %d", retrieved.AccessCount)
	}
}

func TestStoreDelete(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cache.db")
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	store, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Create and put entry
	now := time.Now()
	entry := &CacheEntry{
		Key:         "test-key",
		Reference:   "ghcr.io/test/artifact:v1.0.0",
		ContentPath: "/tmp/content",
		Size:        1024,
		CreatedAt:   now,
		AccessedAt:  now,
		AccessCount: 0,
	}

	if err := store.Put(entry); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify it exists
	if _, err := store.Get("test-key"); err != nil {
		t.Fatalf("Get failed before delete: %v", err)
	}

	// Delete entry
	if err := store.Delete("test-key"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	if _, err := store.Get("test-key"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestStoreList(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cache.db")
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	store, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Put multiple entries
	now := time.Now()
	entries := []*CacheEntry{
		{
			Key:         "key1",
			Reference:   "ghcr.io/test/artifact1:v1.0.0",
			ContentPath: "/tmp/content1",
			Size:        1024,
			CreatedAt:   now,
			AccessedAt:  now,
			AccessCount: 0,
		},
		{
			Key:         "key2",
			Reference:   "ghcr.io/test/artifact2:v1.0.0",
			ContentPath: "/tmp/content2",
			Size:        2048,
			CreatedAt:   now,
			AccessedAt:  now,
			AccessCount: 0,
		},
		{
			Key:         "key3",
			Reference:   "ghcr.io/test/artifact3:v1.0.0",
			ContentPath: "/tmp/content3",
			Size:        4096,
			CreatedAt:   now,
			AccessedAt:  now,
			AccessCount: 0,
		},
	}

	for _, entry := range entries {
		if err := store.Put(entry); err != nil {
			t.Fatalf("Put failed for %s: %v", entry.Key, err)
		}
	}

	// List all entries
	listed, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != len(entries) {
		t.Errorf("expected %d entries, got %d", len(entries), len(listed))
	}

	// Verify all keys are present
	keyMap := make(map[string]bool)
	for _, entry := range listed {
		keyMap[entry.Key] = true
	}

	for _, entry := range entries {
		if !keyMap[entry.Key] {
			t.Errorf("key %s not found in list", entry.Key)
		}
	}
}

func TestStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cache.db")
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create store and add entry
	store1, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	now := time.Now()
	entry := &CacheEntry{
		Key:         "test-key",
		Reference:   "ghcr.io/test/artifact:v1.0.0",
		ContentPath: "/tmp/content",
		Size:        1024,
		CreatedAt:   now,
		AccessedAt:  now,
		AccessCount: 0,
	}

	if err := store1.Put(entry); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Close store
	if err := store1.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Open new store with same path
	store2, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed on reload: %v", err)
	}
	defer func() {
		if err := store2.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Verify entry was persisted
	retrieved, err := store2.Get("test-key")
	if err != nil {
		t.Fatalf("Get failed after reload: %v", err)
	}

	if retrieved.Reference != entry.Reference {
		t.Errorf("expected reference %s, got %s", entry.Reference, retrieved.Reference)
	}

	if retrieved.Size != entry.Size {
		t.Errorf("expected size %d, got %d", entry.Size, retrieved.Size)
	}
}

func TestStoreEmptyList(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cache.db")
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	store, err := NewStore(storePath, logger)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// List should return empty slice, not error
	entries, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
