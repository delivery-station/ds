package types

// PluginInfo describes an installed plugin along with the metadata it exposes.
type PluginInfo struct {
	Name        string          `json:"name" yaml:"name"`
	Version     string          `json:"version" yaml:"version"`
	Description string          `json:"description" yaml:"description"`
	Path        string          `json:"path,omitempty" yaml:"path,omitempty"`
	Commands    []PluginCommand `json:"commands,omitempty" yaml:"commands,omitempty"`
	Platform    PluginPlatform  `json:"platform,omitempty" yaml:"platform,omitempty"`
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
