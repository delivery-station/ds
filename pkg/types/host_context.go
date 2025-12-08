package types

import "context"

type hostConfigProviderKey struct{}
type hostConfigBrokerKey struct{}
type hostConfigPayloadKey struct{}

// HostConfigProvider exposes host configuration access to plugins.
type HostConfigProvider interface {
	GetEffectiveConfig(ctx context.Context) (*Config, error)
}

// WithHostConfigProvider stores the host config provider in the context for plugin consumption.
func WithHostConfigProvider(ctx context.Context, provider HostConfigProvider) context.Context {
	if provider == nil {
		return ctx
	}
	return context.WithValue(ctx, hostConfigProviderKey{}, provider)
}

// HostConfigFromContext returns the host config provider if present.
func HostConfigFromContext(ctx context.Context) (HostConfigProvider, bool) {
	provider, ok := ctx.Value(hostConfigProviderKey{}).(HostConfigProvider)
	return provider, ok
}

// WithHostConfigBrokerID stores the broker identifier that plugins can dial for host services.
func WithHostConfigBrokerID(ctx context.Context, id uint32) context.Context {
	if id == 0 {
		return ctx
	}
	return context.WithValue(ctx, hostConfigBrokerKey{}, id)
}

// HostConfigBrokerIDFromContext extracts the broker id if present.
func HostConfigBrokerIDFromContext(ctx context.Context) (uint32, bool) {
	value := ctx.Value(hostConfigBrokerKey{})
	if value == nil {
		return 0, false
	}
	id, ok := value.(uint32)
	return id, ok
}

// WithHostConfigPayload attaches the effective configuration payload so the host can serve it via broker.
func WithHostConfigPayload(ctx context.Context, cfg *Config) context.Context {
	if cfg == nil {
		return ctx
	}
	return context.WithValue(ctx, hostConfigPayloadKey{}, cfg)
}

// HostConfigPayloadFromContext extracts the configuration payload if available.
func HostConfigPayloadFromContext(ctx context.Context) (*Config, bool) {
	value := ctx.Value(hostConfigPayloadKey{})
	if value == nil {
		return nil, false
	}
	cfg, ok := value.(*Config)
	return cfg, ok
}
