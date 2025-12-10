package communication

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/delivery-station/ds/pkg/log"
	"github.com/hashicorp/go-hclog"
)

// PluginInfo represents information about a registered plugin
type PluginInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`
	Status       PluginStatus           `json:"status"`
}

// PluginStatus represents the status of a plugin
type PluginStatus string

const (
	PluginStatusRegistered PluginStatus = "registered"
	PluginStatusRunning    PluginStatus = "running"
	PluginStatusFinished   PluginStatus = "finished"
	PluginStatusFailed     PluginStatus = "failed"
)

// PluginRegistry manages plugin registration and discovery
type PluginRegistry struct {
	mu       sync.RWMutex
	plugins  map[string]*PluginInfo
	storeDir string
	logger   hclog.Logger
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry(storeDir string, logger hclog.Logger) (*PluginRegistry, error) {
	if logger == nil {
		logger = log.Named("plugin-registry")
	}

	// Create store directory
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create registry directory: %w", err)
	}

	registry := &PluginRegistry{
		plugins:  make(map[string]*PluginInfo),
		storeDir: storeDir,
		logger:   logger,
	}

	// Load existing registry
	if err := registry.load(); err != nil {
		logger.Warn("Failed to load existing registry", "error", err)
	}

	return registry, nil
}

// Register registers a plugin in the registry
func (r *PluginRegistry) Register(ctx context.Context, info *PluginInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if info.Status == "" {
		info.Status = PluginStatusRegistered
	}

	r.plugins[info.ID] = info
	r.logger.Info("Registered plugin", "name", info.Name, "id", info.ID)

	// Persist to disk
	if err := r.save(); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

// Unregister removes a plugin from the registry
func (r *PluginRegistry) Unregister(ctx context.Context, pluginID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[pluginID]; !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	delete(r.plugins, pluginID)
	r.logger.Info("Unregistered plugin", "id", pluginID)

	// Persist to disk
	if err := r.save(); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

// UpdateStatus updates the status of a plugin
func (r *PluginRegistry) UpdateStatus(ctx context.Context, pluginID string, status PluginStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, exists := r.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	plugin.Status = status
	r.logger.Debug("Updated plugin status", "id", pluginID, "status", status)

	// Persist to disk
	if err := r.save(); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

// Get retrieves plugin information by ID
func (r *PluginRegistry) Get(ctx context.Context, pluginID string) (*PluginInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	return plugin, nil
}

// List returns all registered plugins
func (r *PluginRegistry) List(ctx context.Context) []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]*PluginInfo, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins
}

// FindByCapability returns all plugins with a specific capability
func (r *PluginRegistry) FindByCapability(ctx context.Context, capability string) []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]*PluginInfo, 0)
	for _, plugin := range r.plugins {
		for _, cap := range plugin.Capabilities {
			if cap == capability {
				plugins = append(plugins, plugin)
				break
			}
		}
	}

	return plugins
}

// load reads registry from disk
func (r *PluginRegistry) load() error {
	registryPath := filepath.Join(r.storeDir, "registry.json")

	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No registry file yet
		}
		return err
	}

	var plugins []*PluginInfo
	if err := json.Unmarshal(data, &plugins); err != nil {
		return err
	}

	for _, plugin := range plugins {
		r.plugins[plugin.ID] = plugin
	}

	r.logger.Debug("Loaded plugins from registry", "count", len(plugins))
	return nil
}

// save writes registry to disk
func (r *PluginRegistry) save() error {
	registryPath := filepath.Join(r.storeDir, "registry.json")
	tempPath := registryPath + ".tmp"

	plugins := make([]*PluginInfo, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}

	data, err := json.MarshalIndent(plugins, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tempPath, registryPath); err != nil {
		return err
	}

	return nil
}
