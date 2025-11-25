package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
)

// Store provides persistent storage for cache metadata
type Store struct {
	path    string
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	logger  *logrus.Logger
}

// NewStore creates a new cache store
func NewStore(path string, logger *logrus.Logger) (*Store, error) {
	if logger == nil {
		logger = logrus.New()
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	store := &Store{
		path:    path,
		entries: make(map[string]*CacheEntry),
		logger:  logger,
	}

	// Load existing entries
	if err := store.load(); err != nil {
		logger.WithError(err).Warn("Failed to load cache store, starting fresh")
	}

	return store, nil
}

// Get retrieves a cache entry by key
func (s *Store) Get(key string) (*CacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[key]
	if !exists {
		return nil, fmt.Errorf("cache entry not found")
	}

	// Return a copy to prevent external modification
	entryCopy := *entry
	return &entryCopy, nil
}

// Put stores a cache entry
func (s *Store) Put(entry *CacheEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy to prevent external modification
	entryCopy := *entry
	s.entries[entry.Key] = &entryCopy

	return s.save()
}

// Update updates an existing cache entry
func (s *Store) Update(entry *CacheEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[entry.Key]; !exists {
		return fmt.Errorf("cache entry not found")
	}

	// Store a copy to prevent external modification
	entryCopy := *entry
	s.entries[entry.Key] = &entryCopy

	return s.save()
}

// Delete removes a cache entry by key
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, key)
	return s.save()
}

// List returns all cache entries
func (s *Store) List() ([]*CacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]*CacheEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		// Return copies to prevent external modification
		entryCopy := *entry
		entries = append(entries, &entryCopy)
	}

	return entries, nil
}

// Close closes the store and saves any pending changes
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.save()
}

// load loads cache entries from disk
func (s *Store) load() error {
	// Check if file exists
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		s.logger.WithField("path", s.path).Debug("Cache store file not found, starting fresh")
		return nil
	}

	// Read file
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("failed to read store file: %w", err)
	}

	// Parse JSON
	var entries []*CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to parse store file: %w", err)
	}

	// Load into map
	s.entries = make(map[string]*CacheEntry)
	for _, entry := range entries {
		s.entries[entry.Key] = entry
	}

	s.logger.WithFields(logrus.Fields{
		"path":  s.path,
		"count": len(s.entries),
	}).Debug("Loaded cache store")

	return nil
}

// save persists cache entries to disk
func (s *Store) save() error {
	// Convert map to slice
	entries := make([]*CacheEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		entries = append(entries, entry)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store data: %w", err)
	}

	// Write to temporary file first
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write store file: %w", err)
	}

	// Rename to actual path (atomic on most systems)
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("failed to rename store file: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"path":  s.path,
		"count": len(s.entries),
	}).Debug("Saved cache store")

	return nil
}
