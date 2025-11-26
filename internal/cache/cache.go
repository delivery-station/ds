package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// Cache provides artifact caching functionality
type Cache struct {
	dir     string
	store   *Store
	maxSize int64 // Maximum cache size in bytes
	ttl     time.Duration
	mu      sync.RWMutex
	logger  hclog.Logger
}

// CacheEntry represents a cached artifact
type CacheEntry struct {
	Key         string    `json:"key"`
	Reference   string    `json:"reference"`
	ContentPath string    `json:"content_path"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	AccessedAt  time.Time `json:"accessed_at"`
	AccessCount int       `json:"access_count"`
}

// NewCache creates a new cache instance
func NewCache(dir string, maxSize int64, ttl time.Duration, logger hclog.Logger) (*Cache, error) {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Name:  "cache",
			Level: hclog.Info,
		})
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Initialize store
	store, err := NewStore(filepath.Join(dir, "cache.db"), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache store: %w", err)
	}

	return &Cache{
		dir:     dir,
		store:   store,
		maxSize: maxSize,
		ttl:     ttl,
		logger:  logger,
	}, nil
}

// GenerateKey generates a cache key from an artifact reference
func GenerateKey(reference string) string {
	hash := sha256.Sum256([]byte(reference))
	return hex.EncodeToString(hash[:])
}

// Get retrieves an artifact from the cache
func (c *Cache) Get(ctx context.Context, reference string) (*CacheEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := GenerateKey(reference)
	c.logger.Debug("Getting artifact from cache", "reference", reference, "key", key)

	// Get entry from store
	entry, err := c.store.Get(key)
	if err != nil {
		return nil, err
	}

	// Check if expired
	if c.ttl > 0 && time.Since(entry.CreatedAt) > c.ttl {
		c.logger.Debug("Cache entry expired", "key", key)
		// Remove expired entry (unlock first to avoid deadlock)
		c.mu.RUnlock()
		_ = c.Remove(key) // Ignore error as entry is already considered expired
		c.mu.RLock()
		return nil, fmt.Errorf("cache entry expired")
	}

	// Check if content file exists
	if _, err := os.Stat(entry.ContentPath); os.IsNotExist(err) {
		c.logger.Warn("Cache content file missing", "key", key)
		return nil, fmt.Errorf("cache content file not found")
	}

	// Update access time and count
	entry.AccessedAt = time.Now()
	entry.AccessCount++
	if err := c.store.Update(entry); err != nil {
		c.logger.Warn("Failed to update cache entry metadata", "error", err)
	}

	c.logger.Info("Cache hit", "reference", reference, "key", key, "size", entry.Size)

	return entry, nil
}

// Put stores an artifact in the cache
func (c *Cache) Put(ctx context.Context, reference string, reader io.Reader) (*CacheEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := GenerateKey(reference)
	c.logger.Debug("Putting artifact in cache", "reference", reference, "key", key)

	// Create entry directory
	entryDir := filepath.Join(c.dir, key)
	if err := os.MkdirAll(entryDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache entry directory: %w", err)
	}

	// Write content to file
	contentPath := filepath.Join(entryDir, "content")
	file, err := os.Create(contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache content file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			c.logger.Warn("Failed to close cache content file", "error", err)
		}
	}()

	// Copy content and track size
	size, err := io.Copy(file, reader)
	if err != nil {
		_ = os.RemoveAll(entryDir) // Best effort cleanup
		return nil, fmt.Errorf("failed to write cache content: %w", err)
	}

	// Check if we need to evict entries to make space
	if c.maxSize > 0 {
		currentSize, err := c.sizeNoLock()
		if err != nil {
			c.logger.Warn("Failed to calculate cache size", "error", err)
		} else if currentSize+size > c.maxSize {
			c.logger.Info("Cache size limit exceeded, evicting entries",
				"current_size", currentSize,
				"new_size", size,
				"max_size", c.maxSize)
			if err := c.evictLRUNoLock(currentSize + size - c.maxSize); err != nil {
				c.logger.Warn("Failed to evict cache entries", "error", err)
			}
		}
	}

	// Create cache entry
	now := time.Now()
	entry := &CacheEntry{
		Key:         key,
		Reference:   reference,
		ContentPath: contentPath,
		Size:        size,
		CreatedAt:   now,
		AccessedAt:  now,
		AccessCount: 0,
	}

	// Store entry metadata
	if err := c.store.Put(entry); err != nil {
		_ = os.RemoveAll(entryDir) // Best effort cleanup
		return nil, fmt.Errorf("failed to store cache entry metadata: %w", err)
	}

	c.logger.Info("Artifact cached", "reference", reference, "key", key, "size", size)

	return entry, nil
}

// List returns all cached artifacts
func (c *Cache) List() ([]*CacheEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.store.List()
}

// Remove removes an artifact from the cache
func (c *Cache) Remove(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Debug("Removing cache entry", "key", key)

	// Get entry to find content path
	entry, err := c.store.Get(key)
	if err != nil {
		return err
	}

	// Remove content directory
	entryDir := filepath.Dir(entry.ContentPath)
	if err := os.RemoveAll(entryDir); err != nil {
		c.logger.Warn("Failed to remove cache content directory", "error", err)
	}

	// Remove from store
	if err := c.store.Delete(key); err != nil {
		return err
	}

	c.logger.Info("Cache entry removed", "key", key)
	return nil
}

// Clean removes expired cache entries
func (c *Cache) Clean() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("Cleaning expired cache entries")

	entries, err := c.store.List()
	if err != nil {
		return 0, err
	}

	removed := 0
	now := time.Now()

	for _, entry := range entries {
		// Check if expired
		if c.ttl > 0 && now.Sub(entry.CreatedAt) > c.ttl {
			c.logger.Debug("Removing expired entry",
				"key", entry.Key,
				"age", now.Sub(entry.CreatedAt),
				"ttl", c.ttl)

			// Remove content directory
			entryDir := filepath.Dir(entry.ContentPath)
			if err := os.RemoveAll(entryDir); err != nil {
				c.logger.Warn("Failed to remove expired content", "error", err)
			}

			// Remove from store
			if err := c.store.Delete(entry.Key); err != nil {
				c.logger.Warn("Failed to remove expired entry from store", "error", err)
			} else {
				removed++
			}
		}
	}

	c.logger.Info("Cache cleanup complete", "removed", removed)
	return removed, nil
}

// Size calculates the total size of cached artifacts
func (c *Cache) Size() (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.sizeNoLock()
}

// sizeNoLock calculates size without locking (internal use)
func (c *Cache) sizeNoLock() (int64, error) {
	entries, err := c.store.List()
	if err != nil {
		return 0, err
	}

	var total int64
	for _, entry := range entries {
		total += entry.Size
	}

	return total, nil
}

// evictLRUNoLock evicts least recently used entries until target bytes are freed (internal use)
func (c *Cache) evictLRUNoLock(targetBytes int64) error {
	entries, err := c.store.List()
	if err != nil {
		return err
	}

	// Sort by access time (least recent first)
	// Simple bubble sort for small lists
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].AccessedAt.After(entries[j].AccessedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	var freedBytes int64
	for _, entry := range entries {
		if freedBytes >= targetBytes {
			break
		}

		c.logger.Debug("Evicting cache entry",
			"key", entry.Key,
			"reference", entry.Reference,
			"size", entry.Size,
			"last_access", entry.AccessedAt)

		// Remove content directory
		entryDir := filepath.Dir(entry.ContentPath)
		if err := os.RemoveAll(entryDir); err != nil {
			c.logger.Warn("Failed to remove evicted content", "error", err)
		}

		// Remove from store
		if err := c.store.Delete(entry.Key); err != nil {
			c.logger.Warn("Failed to remove evicted entry from store", "error", err)
		} else {
			freedBytes += entry.Size
		}
	}

	c.logger.Info("Cache eviction complete",
		"target_bytes", targetBytes,
		"freed_bytes", freedBytes)

	return nil
}

// Close closes the cache and releases resources
func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.store.Close()
}
