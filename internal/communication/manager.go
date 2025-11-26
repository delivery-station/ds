package communication

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
)

// Manager manages all inter-plugin communication
type Manager struct {
	stateStore     *StateStore
	eventBus       *EventBus
	pluginRegistry *PluginRegistry
	logger         hclog.Logger
}

// ManagerConfig holds configuration for the communication manager
type ManagerConfig struct {
	StateDir        string
	EventBufferSize int
	Logger          hclog.Logger
}

// NewManager creates a new communication manager
func NewManager(config *ManagerConfig) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	logger := config.Logger
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	// Create state store
	stateStore, err := NewStateStore(filepath.Join(config.StateDir, "state"), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}

	// Create event bus
	eventBus := NewEventBus(logger, config.EventBufferSize)

	// Create plugin registry
	pluginRegistry, err := NewPluginRegistry(filepath.Join(config.StateDir, "registry"), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin registry: %w", err)
	}

	manager := &Manager{
		stateStore:     stateStore,
		eventBus:       eventBus,
		pluginRegistry: pluginRegistry,
		logger:         logger,
	}

	// Subscribe to plugin lifecycle events to update registry
	eventBus.Subscribe(EventPluginStarted, manager.handlePluginStarted)
	eventBus.Subscribe(EventPluginFinished, manager.handlePluginFinished)
	eventBus.Subscribe(EventPluginFailed, manager.handlePluginFailed)

	return manager, nil
}

// StateStore returns the state store
func (m *Manager) StateStore() *StateStore {
	return m.stateStore
}

// EventBus returns the event bus
func (m *Manager) EventBus() *EventBus {
	return m.eventBus
}

// PluginRegistry returns the plugin registry
func (m *Manager) PluginRegistry() *PluginRegistry {
	return m.pluginRegistry
}

// handlePluginStarted updates registry when plugin starts
func (m *Manager) handlePluginStarted(ctx context.Context, event *Event) error {
	pluginID := event.PluginID
	if pluginID == "" {
		return fmt.Errorf("plugin ID is required")
	}

	err := m.pluginRegistry.UpdateStatus(ctx, pluginID, PluginStatusRunning)
	if err != nil {
		m.logger.Warn("Failed to update plugin status", "error", err)
	}

	return nil
}

// handlePluginFinished updates registry when plugin finishes
func (m *Manager) handlePluginFinished(ctx context.Context, event *Event) error {
	pluginID := event.PluginID
	if pluginID == "" {
		return fmt.Errorf("plugin ID is required")
	}

	err := m.pluginRegistry.UpdateStatus(ctx, pluginID, PluginStatusFinished)
	if err != nil {
		m.logger.Warn("Failed to update plugin status", "error", err)
	}

	return nil
}

// handlePluginFailed updates registry when plugin fails
func (m *Manager) handlePluginFailed(ctx context.Context, event *Event) error {
	pluginID := event.PluginID
	if pluginID == "" {
		return fmt.Errorf("plugin ID is required")
	}

	err := m.pluginRegistry.UpdateStatus(ctx, pluginID, PluginStatusFailed)
	if err != nil {
		m.logger.Warn("Failed to update plugin status", "error", err)
	}

	return nil
}

// Close shuts down the communication manager
func (m *Manager) Close() {
	m.eventBus.Close()
	m.logger.Info("Communication manager closed")
}
