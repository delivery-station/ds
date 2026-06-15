package types

import "time"

// Config represents the complete DS configuration
type Config struct {
	Registry RegistryConfig                    `mapstructure:"registry" yaml:"registry"`
	Auth     AuthConfig                        `mapstructure:"auth" yaml:"auth"`
	Cache    CacheConfig                       `mapstructure:"cache" yaml:"cache"`
	Logging  LoggingConfig                     `mapstructure:"logging" yaml:"logging"`
	Proxy    ProxyConfig                       `mapstructure:"proxy" yaml:"proxy"`
	Plugins  PluginsConfig                     `mapstructure:"plugins" yaml:"plugins"`
	Settings map[string]map[string]interface{} `mapstructure:"settings" yaml:"settings,omitempty"`
}

// RegistryConfig contains OCI registry settings
type RegistryConfig struct {
	Default            string   `mapstructure:"default" yaml:"default"`
	Mirrors            []string `mapstructure:"mirrors" yaml:"mirrors,omitempty"`
	InsecureRegistries []string `mapstructure:"insecure_registries" yaml:"insecure_registries,omitempty"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	DockerConfig string       `mapstructure:"docker_config" yaml:"docker_config,omitempty"`
	Credentials  []Credential `mapstructure:"credentials" yaml:"credentials,omitempty"`
}

// Credential represents a single registry credential
type Credential struct {
	Registry string `mapstructure:"registry" yaml:"registry"`
	Username string `mapstructure:"username" yaml:"username"`
	Token    string `mapstructure:"token" yaml:"token,omitempty"`
	Password string `mapstructure:"password" yaml:"password,omitempty"`
}

// CacheConfig contains cache settings
type CacheConfig struct {
	Dir     string        `mapstructure:"dir" yaml:"dir"`
	MaxSize int64         `mapstructure:"max_size" yaml:"max_size"` // in bytes
	TTL     time.Duration `mapstructure:"ttl" yaml:"ttl"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level" yaml:"level"`
	Format string `mapstructure:"format" yaml:"format"`
	Output string `mapstructure:"output" yaml:"output"`
}

// ProxyConfig contains proxy settings
type ProxyConfig struct {
	HTTPProxy  string `mapstructure:"http_proxy" yaml:"http_proxy,omitempty"`
	HTTPSProxy string `mapstructure:"https_proxy" yaml:"https_proxy,omitempty"`
	NoProxy    string `mapstructure:"no_proxy" yaml:"no_proxy,omitempty"`
}

// PluginsConfig contains plugin management settings
type PluginsConfig struct {
	Dir         string          `mapstructure:"dir" yaml:"dir"`
	AutoInstall bool            `mapstructure:"auto_install" yaml:"auto_install"`
	Sources     []PluginSource  `mapstructure:"sources" yaml:"sources,omitempty"`
	Signature   SignatureConfig `mapstructure:"signature" yaml:"signature,omitempty"`
}

// SignatureConfig contains digital signature verification settings
type SignatureConfig struct {
	// Mode: "strict" (only valid signatures) or "permissive" (warn on unsigned)
	Mode string `mapstructure:"mode" yaml:"mode"`
	// PublicKeys contains paths to trusted public key files
	PublicKeys []string `mapstructure:"public_keys" yaml:"public_keys,omitempty"`
	// TrustStore is a directory containing trusted public keys
	TrustStore string `mapstructure:"trust_store" yaml:"trust_store,omitempty"`
	// AllowUnsigned allows unsigned plugins in permissive mode
	AllowUnsigned bool `mapstructure:"allow_unsigned" yaml:"allow_unsigned"`
}

// PluginSource represents a plugin registry source
type PluginSource struct {
	Registry          string `mapstructure:"registry" yaml:"registry"`
	VersionConstraint string `mapstructure:"version_constraint" yaml:"version_constraint,omitempty"`
}
