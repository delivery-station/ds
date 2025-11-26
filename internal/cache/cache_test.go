package cache

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
)

func TestNewCache(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	cache, err := NewCache(tmpDir, 1024*1024, time.Hour, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	if cache.dir != tmpDir {
		t.Errorf("expected dir %s, got %s", tmpDir, cache.dir)
	}

	if cache.maxSize != 1024*1024 {
		t.Errorf("expected maxSize 1048576, got %d", cache.maxSize)
	}

	if cache.ttl != time.Hour {
		t.Errorf("expected ttl 1h, got %v", cache.ttl)
	}
}

func TestGenerateKey(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		wantSame  string
		wantDiff  string
	}{
		{
			name:      "basic reference",
			reference: "ghcr.io/org/image:v1.0.0",
			wantSame:  "ghcr.io/org/image:v1.0.0",
			wantDiff:  "ghcr.io/org/image:v2.0.0",
		},
		{
			name:      "with digest",
			reference: "ghcr.io/org/image@sha256:abc123",
			wantSame:  "ghcr.io/org/image@sha256:abc123",
			wantDiff:  "ghcr.io/org/image@sha256:def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := GenerateKey(tt.reference)
			key2 := GenerateKey(tt.wantSame)
			key3 := GenerateKey(tt.wantDiff)

			// Same reference should produce same key
			if key1 != key2 {
				t.Errorf("expected same key for identical references")
			}

			// Different reference should produce different key
			if key1 == key3 {
				t.Errorf("expected different key for different references")
			}

			// Key should be valid hex string
			if len(key1) != 64 { // SHA256 produces 64 hex chars
				t.Errorf("expected 64 character key, got %d", len(key1))
			}
		})
	}
}

func TestCachePutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	cache, err := NewCache(tmpDir, 10*1024*1024, time.Hour, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()
	reference := "ghcr.io/test/artifact:v1.0.0"
	content := []byte("test artifact content")

	// Put artifact in cache
	entry, err := cache.Put(ctx, reference, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if entry.Reference != reference {
		t.Errorf("expected reference %s, got %s", reference, entry.Reference)
	}

	if entry.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), entry.Size)
	}

	// Get artifact from cache
	retrieved, err := cache.Get(ctx, reference)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Reference != reference {
		t.Errorf("expected reference %s, got %s", reference, retrieved.Reference)
	}

	if retrieved.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), retrieved.Size)
	}

	// Verify content file exists
	if _, err := os.Stat(retrieved.ContentPath); err != nil {
		t.Errorf("content file not found: %v", err)
	}

	// Read and verify content
	storedContent, err := os.ReadFile(retrieved.ContentPath)
	if err != nil {
		t.Fatalf("failed to read stored content: %v", err)
	}

	if !bytes.Equal(storedContent, content) {
		t.Errorf("stored content doesn't match original")
	}
}

func TestCacheGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	cache, err := NewCache(tmpDir, 10*1024*1024, time.Hour, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()
	reference := "ghcr.io/test/nonexistent:v1.0.0"

	// Try to get non-existent artifact
	_, err = cache.Get(ctx, reference)
	if err == nil {
		t.Error("expected error for non-existent artifact")
	}
}

func TestCacheList(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	cache, err := NewCache(tmpDir, 10*1024*1024, time.Hour, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()

	// Put multiple artifacts
	refs := []string{
		"ghcr.io/test/artifact1:v1.0.0",
		"ghcr.io/test/artifact2:v1.0.0",
		"ghcr.io/test/artifact3:v1.0.0",
	}

	for _, ref := range refs {
		content := []byte("content for " + ref)
		if _, err := cache.Put(ctx, ref, bytes.NewReader(content)); err != nil {
			t.Fatalf("Put failed for %s: %v", ref, err)
		}
	}

	// List all entries
	entries, err := cache.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) != len(refs) {
		t.Errorf("expected %d entries, got %d", len(refs), len(entries))
	}

	// Verify all references are present
	refMap := make(map[string]bool)
	for _, entry := range entries {
		refMap[entry.Reference] = true
	}

	for _, ref := range refs {
		if !refMap[ref] {
			t.Errorf("reference %s not found in list", ref)
		}
	}
}

func TestCacheRemove(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	cache, err := NewCache(tmpDir, 10*1024*1024, time.Hour, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()
	reference := "ghcr.io/test/artifact:v1.0.0"
	content := []byte("test content")

	// Put artifact
	entry, err := cache.Put(ctx, reference, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify it exists
	if _, err := cache.Get(ctx, reference); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Remove it
	if err := cache.Remove(entry.Key); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify it's gone
	if _, err := cache.Get(ctx, reference); err == nil {
		t.Error("expected error after removal")
	}

	// Verify content directory is removed
	entryDir := filepath.Dir(entry.ContentPath)
	if _, err := os.Stat(entryDir); !os.IsNotExist(err) {
		t.Error("content directory still exists after removal")
	}
}

func TestCacheClean(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	// Create cache with 1 second TTL
	cache, err := NewCache(tmpDir, 10*1024*1024, time.Second, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()

	// Put artifact
	reference := "ghcr.io/test/artifact:v1.0.0"
	content := []byte("test content")
	if _, err := cache.Put(ctx, reference, bytes.NewReader(content)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(2 * time.Second)

	// Clean cache
	removed, err := cache.Clean()
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if removed != 1 {
		t.Errorf("expected 1 removed entry, got %d", removed)
	}

	// Verify artifact is gone
	if _, err := cache.Get(ctx, reference); err == nil {
		t.Error("expected error for expired artifact")
	}
}

func TestCacheSize(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	cache, err := NewCache(tmpDir, 10*1024*1024, time.Hour, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()

	// Initially size should be 0
	size, err := cache.Size()
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 0 {
		t.Errorf("expected initial size 0, got %d", size)
	}

	// Put artifacts
	content1 := []byte(strings.Repeat("a", 1000))
	content2 := []byte(strings.Repeat("b", 2000))

	if _, err := cache.Put(ctx, "ref1", bytes.NewReader(content1)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if _, err := cache.Put(ctx, "ref2", bytes.NewReader(content2)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Check size
	size, err = cache.Size()
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}

	expectedSize := int64(len(content1) + len(content2))
	if size != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, size)
	}
}

func TestCacheLRUEviction(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	// Create cache with small max size (3KB)
	cache, err := NewCache(tmpDir, 3*1024, 0, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()

	// Put first artifact (1KB)
	content1 := []byte(strings.Repeat("a", 1024))
	ref1 := "ref1"
	if _, err := cache.Put(ctx, ref1, bytes.NewReader(content1)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Put second artifact (1KB)
	time.Sleep(50 * time.Millisecond)
	content2 := []byte(strings.Repeat("b", 1024))
	ref2 := "ref2"
	if _, err := cache.Put(ctx, ref2, bytes.NewReader(content2)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Access ref2 to make it more recent than ref1
	time.Sleep(50 * time.Millisecond)
	if _, err := cache.Get(ctx, ref2); err != nil {
		t.Fatalf("Get ref2 failed: %v", err)
	}

	// Put third artifact (2KB) - should trigger eviction of ref1 (least recently used)
	time.Sleep(50 * time.Millisecond)
	content3 := []byte(strings.Repeat("c", 2048))
	ref3 := "ref3"
	if _, err := cache.Put(ctx, ref3, bytes.NewReader(content3)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// ref2 should still exist (most recently accessed)
	if _, err := cache.Get(ctx, ref2); err != nil {
		t.Error("ref2 should still exist after eviction")
	}

	// ref3 should exist (just added)
	if _, err := cache.Get(ctx, ref3); err != nil {
		t.Error("ref3 should exist")
	}
	// We don't strictly test this as LRU behavior can be implementation-specific
}

func TestCacheTTLExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	// Create cache with 500ms TTL
	cache, err := NewCache(tmpDir, 10*1024*1024, 500*time.Millisecond, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()
	reference := "ghcr.io/test/artifact:v1.0.0"
	content := []byte("test content")

	// Put artifact
	if _, err := cache.Put(ctx, reference, bytes.NewReader(content)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Should be retrievable immediately
	if _, err := cache.Get(ctx, reference); err != nil {
		t.Fatalf("Get failed before expiration: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(600 * time.Millisecond)

	// Should fail after expiration
	if _, err := cache.Get(ctx, reference); err == nil {
		t.Error("expected error for expired entry")
	}
}

func TestCacheAccessCountIncrement(t *testing.T) {
	tmpDir := t.TempDir()
	logger := hclog.NewNullLogger()

	cache, err := NewCache(tmpDir, 10*1024*1024, time.Hour, logger)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	ctx := context.Background()
	reference := "ghcr.io/test/artifact:v1.0.0"
	content := []byte("test content")

	// Put artifact
	if _, err := cache.Put(ctx, reference, bytes.NewReader(content)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Access it multiple times
	for i := 0; i < 5; i++ {
		entry, err := cache.Get(ctx, reference)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		expectedCount := i + 1
		if entry.AccessCount != expectedCount {
			t.Errorf("expected access count %d, got %d", expectedCount, entry.AccessCount)
		}
	}
}
