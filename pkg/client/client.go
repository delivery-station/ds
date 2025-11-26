package client

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/delivery-station/ds/internal/cache"
	"github.com/delivery-station/ds/internal/communication"
	"github.com/delivery-station/ds/internal/config"
	"github.com/delivery-station/ds/internal/plugin"
	"github.com/delivery-station/ds/internal/registry"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/hashicorp/go-hclog"
)

// Client is the main public API for interacting with the delivery station
type Client struct {
	config         *types.Config
	cache          *cache.Cache
	registry       *registry.Client
	pluginMgr      *plugin.Manager
	eventBus       *communication.EventBus
	stateStore     *communication.StateStore
	pluginRegistry *communication.PluginRegistry
	logger         hclog.Logger
}

// Option is a functional option for configuring the Client
type Option func(*Client) error

// NewClient creates a new delivery station client
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:  "ds-client",
			Level: hclog.Info,
		}),
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Apply defaults if not set
	if c.config == nil {
		loader := config.NewLoader()
		cfg, err := loader.Load()
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		c.config = cfg
	}

	if c.cache == nil {
		cacheInstance, err := cache.NewCache(
			c.config.Cache.Dir,
			c.config.Cache.MaxSize,
			c.config.Cache.TTL,
			c.logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create cache: %w", err)
		}
		c.cache = cacheInstance
	}

	if c.registry == nil {
		regClient, err := registry.NewClient(c.config.Registry.Default, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create registry client: %w", err)
		}
		c.registry = regClient
	}

	if c.eventBus == nil {
		c.eventBus = communication.NewEventBus(c.logger, 100)
	}

	if c.stateStore == nil {
		stateDir := filepath.Join(c.config.Cache.Dir, "state")
		store, err := communication.NewStateStore(stateDir, c.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create state store: %w", err)
		}
		c.stateStore = store
	}

	if c.pluginRegistry == nil {
		regDir := filepath.Join(c.config.Cache.Dir, "registry")
		reg, err := communication.NewPluginRegistry(regDir, c.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create plugin registry: %w", err)
		}
		c.pluginRegistry = reg
	}

	if c.pluginMgr == nil {
		c.pluginMgr = plugin.NewManager(c.config.Plugins.Dir)
	}

	return c, nil
}

// WithConfig sets the configuration
func WithConfig(cfg *types.Config) Option {
	return func(c *Client) error {
		c.config = cfg
		return nil
	}
}

// WithCache sets the cache
func WithCache(cache *cache.Cache) Option {
	return func(c *Client) error {
		c.cache = cache
		return nil
	}
}

// WithRegistry sets the registry client
func WithRegistry(reg *registry.Client) Option {
	return func(c *Client) error {
		c.registry = reg
		return nil
	}
}

// WithPluginManager sets the plugin manager
func WithPluginManager(mgr *plugin.Manager) Option {
	return func(c *Client) error {
		c.pluginMgr = mgr
		return nil
	}
}

// Pull downloads an artifact from a registry
func (c *Client) Pull(ctx context.Context, ref string, writer io.Writer) error {
	return c.registry.Pull(ctx, ref, writer)
}

// Push uploads an artifact to a registry
func (c *Client) Push(ctx context.Context, ref string, reader io.Reader, mediaType string) error {
	return c.registry.Push(ctx, ref, reader, mediaType)
}

// List returns available tags for a repository
func (c *Client) List(ctx context.Context, repository string) ([]string, error) {
	tags, err := c.registry.List(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	return tags, nil
}

// InstallPlugin installs a plugin from a reference
func (c *Client) InstallPlugin(ctx context.Context, ref, version string) error {
	installer := plugin.NewInstaller(c.config.Plugins.Dir, c.config.Registry.Default, c.registry)
	return installer.InstallPlugin(ctx, ref, version)
}

// ListPlugins returns all installed plugins
func (c *Client) ListPlugins() ([]*types.PluginInfo, error) {
	return c.pluginMgr.ListPlugins()
}

// ExecutePlugin runs a plugin command
func (c *Client) ExecutePlugin(name string, args []string) (int, error) {
	executor := plugin.NewExecutor(c.pluginMgr)
	return executor.ExecutePlugin(name, args)
}

// Subscribe subscribes to events from the event bus
func (c *Client) Subscribe(topic communication.EventType, handler communication.EventHandler) {
	c.eventBus.Subscribe(topic, handler)
}

// Publish publishes an event to the event bus
func (c *Client) Publish(ctx context.Context, eventType communication.EventType, pluginID string, data map[string]interface{}) error {
	event := &communication.Event{
		Type:     eventType,
		PluginID: pluginID,
		Data:     data,
	}
	return c.eventBus.Publish(ctx, event)
}

// GetState retrieves state by key
func (c *Client) GetState(ctx context.Context, key string) (map[string]interface{}, error) {
	entry, err := c.stateStore.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return entry.Value, nil
}

// SetState sets state by key
func (c *Client) SetState(ctx context.Context, key string, pluginID string, value map[string]interface{}, ttl *time.Duration) error {
	return c.stateStore.Set(ctx, key, value, pluginID, ttl)
}

// RegisterPlugin registers a plugin in the plugin registry
// Note: This uses communication.PluginInfo internally
func (c *Client) RegisterPlugin(ctx context.Context, name, version, path string) error {
	// Convert to internal type - simplified for now
	return fmt.Errorf("not yet implemented")
}

// DiscoverPlugin discovers a plugin by name
// Note: This returns communication.PluginInfo internally
func (c *Client) DiscoverPlugin(ctx context.Context, name string) error {
	// Convert from internal type - simplified for now
	return fmt.Errorf("not yet implemented")
}

// Close cleans up resources
func (c *Client) Close() error {
	// Cleanup if needed
	return nil
}

// Config returns the current configuration
func (c *Client) Config() *types.Config {
	return c.config
}

// Cache returns the cache instance
func (c *Client) Cache() *cache.Cache {
	return c.cache
}

// Registry returns the registry client
func (c *Client) Registry() *registry.Client {
	return c.registry
}

// PluginManager returns the plugin manager
func (c *Client) PluginManager() *plugin.Manager {
	return c.pluginMgr
}

// EventBus returns the event bus
func (c *Client) EventBus() *communication.EventBus {
	return c.eventBus
}

// StateStore returns the state store
func (c *Client) StateStore() *communication.StateStore {
	return c.stateStore
}

// PluginRegistry returns the plugin registry
func (c *Client) PluginRegistry() *communication.PluginRegistry {
	return c.pluginRegistry
}
