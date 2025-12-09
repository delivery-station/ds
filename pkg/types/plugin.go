package types

// PluginInfo contains information about an installed plugin
type PluginInfo struct {
	Name        string          `json:"name" yaml:"name"`
	Version     string          `json:"version" yaml:"version"`
	Description string          `json:"description" yaml:"description"`
	Path        string          `json:"path" yaml:"path"`
	Manifest    *PluginManifest `json:"manifest,omitempty" yaml:"manifest,omitempty"`
}

// PluginManifest represents the plugin.yaml manifest file
type PluginManifest struct {
	Name        string            `mapstructure:"name" yaml:"name"`
	Version     string            `mapstructure:"version" yaml:"version"`
	Description string            `mapstructure:"description" yaml:"description"`
	Checksum    string            `mapstructure:"checksum" yaml:"checksum,omitempty"`
	Commands    []PluginCommand   `mapstructure:"commands" yaml:"commands,omitempty"`
	Platform    PluginPlatform    `mapstructure:"platform" yaml:"platform,omitempty"`
	Annotations map[string]string `mapstructure:"annotations" yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// PluginCommand represents a command provided by the plugin
type PluginCommand struct {
	Name        string `mapstructure:"name" yaml:"name"`
	Description string `mapstructure:"description" yaml:"description"`
}

// PluginPlatform specifies platform compatibility
type PluginPlatform struct {
	OS   []string `mapstructure:"os" yaml:"os,omitempty"`
	Arch []string `mapstructure:"arch" yaml:"arch,omitempty"`
}

// Plugin interface defines the contract for plugin management
type Plugin interface {
	// Name returns the plugin name
	Name() string

	// Version returns the plugin version
	Version() string

	// Execute runs the plugin with given arguments
	Execute(args []string) error
}
