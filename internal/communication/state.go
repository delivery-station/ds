package communication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/delivery-station/ds/pkg/log"
	"github.com/hashicorp/go-hclog"
)

var (
	ErrStateNotFound = errors.New("state not found")
	ErrStateExpired  = errors.New("state has expired")
)

// StateEntry represents a shared state entry
type StateEntry struct {
	Key         string                 `json:"key"`
	Value       map[string]interface{} `json:"value"`
	PluginID    string                 `json:"plugin_id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	AccessedAt  time.Time              `json:"accessed_at"`
	AccessCount int                    `json:"access_count"`
}

// StateStore manages shared state between plugins
type StateStore struct {
	mu       sync.RWMutex
	states   map[string]*StateEntry
	storeDir string
	logger   hclog.Logger
}

// NewStateStore creates a new state store
func NewStateStore(storeDir string, logger hclog.Logger) (*StateStore, error) {
	if logger == nil {
		logger = log.Named("state-store")
	}

	// Create store directory
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state store directory: %w", err)
	}

	store := &StateStore{
		states:   make(map[string]*StateEntry),
		storeDir: storeDir,
		logger:   logger,
	}

	// Load existing state
	if err := store.load(); err != nil {
		logger.Warn("Failed to load existing state", "error", err)
	}

	return store, nil
}

// Set stores or updates a state entry
func (s *StateStore) Set(ctx context.Context, key string, value map[string]interface{}, pluginID string, ttl *time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var expiresAt *time.Time
	if ttl != nil {
		exp := now.Add(*ttl)
		expiresAt = &exp
	}

	entry, exists := s.states[key]
	if exists {
		entry.Value = value
		entry.UpdatedAt = now
		entry.ExpiresAt = expiresAt
		s.logger.Debug("Updated state", "key", key, "plugin_id", pluginID)
	} else {
		entry = &StateEntry{
			Key:        key,
			Value:      value,
			PluginID:   pluginID,
			CreatedAt:  now,
			UpdatedAt:  now,
			ExpiresAt:  expiresAt,
			AccessedAt: now,
		}
		s.states[key] = entry
		s.logger.Debug("Created state", "key", key, "plugin_id", pluginID)
	}

	// Persist to disk
	if err := s.save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// Get retrieves a state entry
func (s *StateStore) Get(ctx context.Context, key string) (*StateEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.states[key]
	if !exists {
		return nil, ErrStateNotFound
	}

	// Check expiration
	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
		delete(s.states, key)
		_ = s.save() // Best effort save
		return nil, ErrStateExpired
	}

	// Update access metadata
	entry.AccessedAt = time.Now()
	entry.AccessCount++

	return entry, nil
}

// Delete removes a state entry
func (s *StateStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.states[key]; !exists {
		return ErrStateNotFound
	}

	delete(s.states, key)
	s.logger.Debug("Deleted state", "key", key)

	// Persist to disk
	if err := s.save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// List returns all state entries
func (s *StateStore) List(ctx context.Context) []*StateEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]*StateEntry, 0, len(s.states))
	now := time.Now()

	for _, entry := range s.states {
		// Skip expired entries
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			continue
		}
		entries = append(entries, entry)
	}

	return entries
}

// ListByPlugin returns all state entries for a specific plugin
func (s *StateStore) ListByPlugin(ctx context.Context, pluginID string) []*StateEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]*StateEntry, 0)
	now := time.Now()

	for _, entry := range s.states {
		// Skip expired entries
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			continue
		}
		if entry.PluginID == pluginID {
			entries = append(entries, entry)
		}
	}

	return entries
}

// Clean removes expired state entries
func (s *StateStore) Clean(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, entry := range s.states {
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			delete(s.states, key)
			removed++
			s.logger.Debug("Cleaned expired state", "key", key)
		}
	}

	if removed > 0 {
		if err := s.save(); err != nil {
			return removed, fmt.Errorf("failed to save state: %w", err)
		}
	}

	return removed, nil
}

// load reads state from disk
func (s *StateStore) load() error {
	storePath := filepath.Join(s.storeDir, "state.json")

	data, err := os.ReadFile(storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state file yet
		}
		return err
	}

	var entries []*StateEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	for _, entry := range entries {
		s.states[entry.Key] = entry
	}

	s.logger.Debug("Loaded state entries from disk", "count", len(entries))
	return nil
}

// save writes state to disk
func (s *StateStore) save() error {
	storePath := filepath.Join(s.storeDir, "state.json")
	tempPath := storePath + ".tmp"

	entries := make([]*StateEntry, 0, len(s.states))
	for _, entry := range s.states {
		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tempPath, storePath); err != nil {
		return err
	}

	return nil
}
