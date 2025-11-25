package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/delivery-station/ds/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Loader handles configuration loading from multiple sources
type Loader struct {
	viper *viper.Viper
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		viper: viper.GetViper(),
	}
}

// Load loads configuration from all sources with proper precedence
// Precedence: CLI Flags > Environment Variables > Config File > Defaults
func (l *Loader) Load() (*types.Config, error) {
	// Set defaults first
	l.LoadDefaults()

	// Load from config file
	if err := l.LoadFromFile(); err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// Environment variables and flags are already loaded by viper
	// (handled in root.go)

	// Parse cache settings BEFORE unmarshal to handle string formats
	var config types.Config

	// Pre-parse cache.max_size and cache.ttl before unmarshal
	if maxSizeStr := l.viper.GetString("cache.max_size"); maxSizeStr != "" {
		size, err := parseSize(maxSizeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid cache.max_size: %w", err)
		}
		// Set as int64 in viper for unmarshal
		l.viper.Set("cache.max_size", size)
	}

	if ttlStr := l.viper.GetString("cache.ttl"); ttlStr != "" {
		duration, err := parseDuration(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid cache.ttl: %w", err)
		}
		// Set as duration in viper for unmarshal
		l.viper.Set("cache.ttl", duration)
	}

	// Unmarshal into config struct
	if err := l.viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand variables
	if err := l.expandVariables(&config); err != nil {
		return nil, fmt.Errorf("failed to expand variables: %w", err)
	}

	// Validate configuration
	if err := l.Validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// LoadDefaults sets sensible default values
func (l *Loader) LoadDefaults() {
	home, _ := os.UserHomeDir()

	// Registry defaults
	l.viper.SetDefault("registry.default", "ghcr.io")
	l.viper.SetDefault("registry.mirrors", []string{})
	l.viper.SetDefault("registry.insecure_registries", []string{})

	// Cache defaults
	l.viper.SetDefault("cache.dir", filepath.Join(home, ".cache", "ds"))
	l.viper.SetDefault("cache.max_size", "10GB")
	l.viper.SetDefault("cache.ttl", "7d")

	// Logging defaults
	l.viper.SetDefault("logging.level", "info")
	l.viper.SetDefault("logging.format", "text")
	l.viper.SetDefault("logging.output", "stdout")

	// Plugin defaults
	l.viper.SetDefault("plugins.dir", l.getDefaultPluginDir())
	l.viper.SetDefault("plugins.auto_install", true)
	l.viper.SetDefault("plugins.sources", []types.PluginSource{})
	l.viper.SetDefault("plugins.signature.mode", "permissive")
	l.viper.SetDefault("plugins.signature.allow_unsigned", true)
	l.viper.SetDefault("plugins.signature.public_keys", []string{})
	l.viper.SetDefault("plugins.signature.trust_store", filepath.Join(home, ".config", "ds", "trust"))

	// Auth defaults
	l.viper.SetDefault("auth.docker_config", filepath.Join(home, ".docker", "config.json"))
	l.viper.SetDefault("auth.credentials", []types.Credential{})

	// Proxy defaults (read from environment if not set)
	l.viper.SetDefault("proxy.http_proxy", os.Getenv("HTTP_PROXY"))
	l.viper.SetDefault("proxy.https_proxy", os.Getenv("HTTPS_PROXY"))
	l.viper.SetDefault("proxy.no_proxy", os.Getenv("NO_PROXY"))
}

// LoadFromFile loads configuration from YAML file
func (l *Loader) LoadFromFile() error {
	// Config file is already set up in root.go
	// This method handles the actual reading

	if err := l.viper.ReadInConfig(); err != nil {
		// Only return error if config file was explicitly specified
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logrus.Debug("No config file found, using defaults and environment variables")
			return nil
		}
		return err
	}

	logrus.Debugf("Loaded config from: %s", l.viper.ConfigFileUsed())
	return nil
}

// LoadFromEnv loads configuration from environment variables
// This is automatically handled by viper with AutomaticEnv() in root.go
func (l *Loader) LoadFromEnv() {
	// Viper automatically reads DS_* environment variables
	// This method is kept for documentation/clarity
	l.viper.SetEnvPrefix("DS")
	l.viper.AutomaticEnv()
	l.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

// LoadFromFlags loads configuration from CLI flags
// This is automatically handled by viper flag binding in root.go
func (l *Loader) LoadFromFlags() {
	// Flags are bound to viper in root.go
	// This method is kept for documentation/clarity
}

// expandVariables expands ${VAR} style variables in configuration
func (l *Loader) expandVariables(config *types.Config) error {
	// Expand auth config
	config.Auth.DockerConfig = l.expandString(config.Auth.DockerConfig)
	for i := range config.Auth.Credentials {
		config.Auth.Credentials[i].Username = l.expandString(config.Auth.Credentials[i].Username)
		config.Auth.Credentials[i].Token = l.expandString(config.Auth.Credentials[i].Token)
		config.Auth.Credentials[i].Password = l.expandString(config.Auth.Credentials[i].Password)
	}

	// Expand cache dir
	config.Cache.Dir = l.expandString(config.Cache.Dir)

	// Expand plugin dir
	config.Plugins.Dir = l.expandString(config.Plugins.Dir)

	// Expand proxy settings
	config.Proxy.HTTPProxy = l.expandString(config.Proxy.HTTPProxy)
	config.Proxy.HTTPSProxy = l.expandString(config.Proxy.HTTPSProxy)
	config.Proxy.NoProxy = l.expandString(config.Proxy.NoProxy)

	return nil
}

// expandString expands ${VAR} or $VAR style variables
func (l *Loader) expandString(s string) string {
	// Replace ${VAR} style
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})

	// Replace $VAR style (but not ${...})
	re = regexp.MustCompile(`\$([A-Z_][A-Z0-9_]*)`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.TrimPrefix(match, "$")
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})

	// Expand tilde for home directory
	if strings.HasPrefix(s, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			s = filepath.Join(home, s[2:])
		}
	}

	return s
}

// Validate validates the configuration
func (l *Loader) Validate(config *types.Config) error {
	// Validate registry
	if config.Registry.Default == "" {
		return fmt.Errorf("registry.default cannot be empty")
	}

	// Validate cache
	if config.Cache.Dir == "" {
		return fmt.Errorf("cache.dir cannot be empty")
	}
	if config.Cache.MaxSize < 0 {
		return fmt.Errorf("cache.max_size cannot be negative")
	}
	if config.Cache.TTL < 0 {
		return fmt.Errorf("cache.ttl cannot be negative")
	}

	// Validate logging
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[config.Logging.Level] {
		return fmt.Errorf("invalid logging.level: %s (must be debug, info, warn, or error)", config.Logging.Level)
	}
	validFormats := map[string]bool{"text": true, "json": true}
	if !validFormats[config.Logging.Format] {
		return fmt.Errorf("invalid logging.format: %s (must be text or json)", config.Logging.Format)
	}

	// Validate plugins
	if config.Plugins.Dir == "" {
		return fmt.Errorf("plugins.dir cannot be empty")
	}

	// Validate signature mode
	if config.Plugins.Signature.Mode != "" {
		validModes := map[string]bool{"strict": true, "permissive": true, "disabled": true}
		if !validModes[config.Plugins.Signature.Mode] {
			return fmt.Errorf("invalid plugins.signature.mode: %s (must be strict, permissive, or disabled)",
				config.Plugins.Signature.Mode)
		}
	}

	return nil
}

// parseSize parses a size string (e.g., "10GB", "500MB") into bytes
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	// Extract number and unit
	var value float64
	var unit string
	_, err := fmt.Sscanf(s, "%f%s", &value, &unit)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", s)
	}

	// Convert to bytes
	multiplier := int64(1)
	switch unit {
	case "B", "":
		multiplier = 1
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("invalid size unit: %s", unit)
	}

	return int64(value * float64(multiplier)), nil
}

// parseDuration parses a duration string (e.g., "7d", "24h", "30m") into time.Duration
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	// Handle day units (not supported by time.ParseDuration)
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		var daysFloat float64
		if _, err := fmt.Sscanf(daysStr, "%f", &daysFloat); err != nil {
			return 0, fmt.Errorf("invalid duration format: %s", s)
		}
		return time.Duration(daysFloat * 24 * float64(time.Hour)), nil
	}

	// Use standard time.ParseDuration for other units
	return time.ParseDuration(s)
}

// getDefaultPluginDir returns the platform-specific default plugin directory
func (l *Loader) getDefaultPluginDir() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\ds\plugins
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "ds", "plugins")
		}
		return filepath.Join(home, "AppData", "Roaming", "ds", "plugins")
	default:
		// Linux/macOS: ~/.config/ds/plugins
		return filepath.Join(home, ".config", "ds", "plugins")
	}
}

// GetConfigPath returns the platform-specific default config file path
func GetConfigPath() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\ds\config.yaml
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "ds", "config.yaml")
		}
		return filepath.Join(home, "AppData", "Roaming", "ds", "config.yaml")
	default:
		// Linux/macOS: ~/.config/ds/config.yaml
		return filepath.Join(home, ".config", "ds", "config.yaml")
	}
}
