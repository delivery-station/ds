package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/delivery-station/ds/internal/homedir"
	"github.com/delivery-station/ds/pkg/log"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/spf13/viper"
)

// Loader handles configuration loading from multiple sources
type Loader struct {
	viper *viper.Viper
	home  homedir.Provider
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return NewLoaderWithHome(homedir.OSProvider{})
}

// NewLoaderWithHome allows overriding how the home directory is resolved (useful in tests).
func NewLoaderWithHome(home homedir.Provider) *Loader {
	return &Loader{
		viper: viper.GetViper(),
		home:  home,
	}
}

// Load loads configuration from all sources with proper precedence
// Precedence: CLI Flags > Environment Variables > Config File > Defaults
func (l *Loader) Load() (*types.Config, error) {
	l.ensureEnvBindings()

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
	if rawMaxSize := l.viper.Get("cache.max_size"); rawMaxSize != nil {
		size, err := parseSizeValue(rawMaxSize)
		if err != nil {
			return nil, fmt.Errorf("invalid cache.max_size: %w", err)
		}
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

	// Normalize logging configuration after all sources merge
	config.Logging.Level = strings.ToLower(strings.TrimSpace(config.Logging.Level))
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}

	config.Logging.Format = strings.ToLower(strings.TrimSpace(config.Logging.Format))
	if config.Logging.Format == "" {
		config.Logging.Format = "text"
	}

	config.Logging.Output = strings.TrimSpace(config.Logging.Output)
	if config.Logging.Output == "" {
		config.Logging.Output = "stdout"
	}

	// Merge Docker credentials (best-effort)
	if err := l.mergeDockerCredentials(&config); err != nil {
		log.Warn("Failed to merge Docker credentials", "error", err)
	}

	// Validate configuration
	if err := l.Validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// LoadDefaults sets sensible default values
func (l *Loader) LoadDefaults() {
	home := homedir.Resolve(l.home)

	// Registry defaults
	l.viper.SetDefault("registry.default", "ghcr.io/delivery-station")
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
	l.viper.SetDefault("plugins.settings", map[string]map[string]interface{}{})

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
	fileViper := viper.New()
	fileViper.SetConfigType("yaml")
	fileViper.SetConfigName("config")

	home := homedir.Resolve(l.home)
	if home != "" {
		fileViper.AddConfigPath(filepath.Join(home, ".config", "ds"))
	}
	fileViper.AddConfigPath(".")

	loadedAny := false

	if err := fileViper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	} else {
		loadedAny = true
		log.Debug("Loaded base config file", "file", fileViper.ConfigFileUsed())
	}

	if override := getExplicitConfigFile(); override != "" {
		fileViper.SetConfigFile(override)
		if err := fileViper.MergeInConfig(); err != nil {
			return fmt.Errorf("failed to load config file %s: %w", override, err)
		}
		loadedAny = true
		log.Debug("Merged override config file", "file", override)
	}

	if !loadedAny {
		log.Debug("No config file found, using defaults and environment variables")
		return nil
	}

	if err := l.viper.MergeConfigMap(fileViper.AllSettings()); err != nil {
		return fmt.Errorf("failed to apply configuration settings: %w", err)
	}

	return nil
}

func (l *Loader) ensureEnvBindings() {
	replacer := strings.NewReplacer(".", "_")
	l.viper.SetEnvPrefix("DS")
	l.viper.SetEnvKeyReplacer(replacer)
	l.viper.AutomaticEnv()
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

	// Expand plugin settings
	l.expandPluginSettings(config.Plugins.Settings)

	return nil
}

func (l *Loader) expandPluginSettings(settings map[string]map[string]interface{}) {
	for plugin, values := range settings {
		settings[plugin] = l.expandMap(values)
	}
}

func (l *Loader) expandMap(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return nil
	}
	for key, value := range values {
		values[key] = l.expandValue(value)
	}
	return values
}

func (l *Loader) expandSlice(items []interface{}) []interface{} {
	for idx, item := range items {
		items[idx] = l.expandValue(item)
	}
	return items
}

func (l *Loader) expandValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return l.expandString(v)
	case map[string]interface{}:
		return l.expandMap(v)
	case []interface{}:
		return l.expandSlice(v)
	default:
		return value
	}
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
		if home := homedir.Resolve(l.home); home != "" {
			s = filepath.Join(home, s[2:])
		}
	}

	return s
}

func (l *Loader) mergeDockerCredentials(cfg *types.Config) error {
	if cfg == nil {
		return nil
	}

	path := strings.TrimSpace(cfg.Auth.DockerConfig)
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Debug("Docker config file not found", "path", path)
			return nil
		}
		return fmt.Errorf("failed to read docker config %s: %w", path, err)
	}

	var dockerCfg dockerConfig
	if err := json.Unmarshal(data, &dockerCfg); err != nil {
		return fmt.Errorf("failed to parse docker config %s: %w", path, err)
	}

	if len(dockerCfg.Auths) == 0 {
		return nil
	}

	existing := make(map[string]int)
	for idx, cred := range cfg.Auth.Credentials {
		normalized := normalizeRegistryHost(cred.Registry)
		if normalized != "" {
			existing[normalized] = idx
		}
	}

	for registryHost, entry := range dockerCfg.Auths {
		normalized := normalizeRegistryHost(registryHost)
		if normalized == "" {
			continue
		}

		if _, found := existing[normalized]; found {
			continue
		}

		username := strings.TrimSpace(entry.Username)
		password := entry.Password

		if username == "" && password == "" && entry.Auth != "" {
			user, pass, decodeErr := decodeDockerAuth(entry.Auth)
			if decodeErr != nil {
				log.Warn("Failed to decode docker auth entry", "registry", registryHost, "error", decodeErr)
			} else {
				username = user
				password = pass
			}
		}

		if username == "" && entry.IdentityToken != "" {
			username = "token"
			password = entry.IdentityToken
		}

		if username == "" && password == "" {
			continue
		}

		cfg.Auth.Credentials = append(cfg.Auth.Credentials, types.Credential{
			Registry: registryHost,
			Username: username,
			Password: password,
		})
		existing[normalized] = len(cfg.Auth.Credentials) - 1
		log.Debug("Added Docker credential from config", "registry", registryHost)
	}

	return nil
}

type dockerConfig struct {
	Auths map[string]dockerAuth `json:"auths"`
}

type dockerAuth struct {
	Auth          string `json:"auth,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	IdentityToken string `json:"identitytoken,omitempty"`
}

func normalizeRegistryHost(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimSuffix(trimmed, "/")
	if trimmed == "docker.io" || trimmed == "index.docker.io" {
		return "https://index.docker.io/v1/"
	}
	return trimmed
}

func decodeDockerAuth(encoded string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) == 0 {
		return "", "", fmt.Errorf("docker auth payload empty")
	}

	username := parts[0]
	password := ""
	if len(parts) == 2 {
		password = parts[1]
	}

	return username, password, nil
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
func parseSizeValue(raw interface{}) (int64, error) {
	switch v := raw.(type) {
	case nil:
		return 0, fmt.Errorf("missing value")
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		const maxInt64 = 1<<63 - 1
		if v > maxInt64 {
			return 0, fmt.Errorf("size exceeds int64 range")
		}
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return parseSizeString(v)
	default:
		// Attempt string conversion for unexpected numeric representations
		return parseSizeString(fmt.Sprintf("%v", v))
	}
}

func parseSizeString(s string) (int64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("size string is empty")
	}

	upper := strings.ToUpper(trimmed)

	// Separate numeric portion from unit by scanning runes
	idx := 0
	for idx < len(upper) {
		ch := upper[idx]
		if (ch >= '0' && ch <= '9') || ch == '.' || ch == '_' {
			idx++
			continue
		}
		break
	}

	numeric := strings.ReplaceAll(strings.TrimSpace(upper[:idx]), "_", "")
	unit := strings.TrimSpace(upper[idx:])

	if numeric == "" {
		return 0, fmt.Errorf("invalid size format: %s", s)
	}

	value, err := strconv.ParseFloat(numeric, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", s)
	}

	multiplier, err := sizeUnitMultiplier(unit)
	if err != nil {
		return 0, err
	}

	return int64(value * float64(multiplier)), nil
}

func sizeUnitMultiplier(unit string) (int64, error) {
	if unit == "" || unit == "B" {
		return 1, nil
	}

	switch unit {
	case "K", "KB", "KI", "KIB":
		return 1024, nil
	case "M", "MB", "MI", "MIB":
		return 1024 * 1024, nil
	case "G", "GB", "GI", "GIB":
		return 1024 * 1024 * 1024, nil
	case "T", "TB", "TI", "TIB":
		return 1024 * 1024 * 1024 * 1024, nil
	case "P", "PB", "PI", "PIB":
		return 1024 * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("invalid size unit: %s", unit)
	}
}

func parseSize(s string) (int64, error) {
	return parseSizeString(s)
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
	home := homedir.Resolve(l.home)

	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\ds\plugins
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "ds", "plugins")
		}
		if home != "" {
			return filepath.Join(home, "AppData", "Roaming", "ds", "plugins")
		}
		return ""
	default:
		// Linux/macOS: ~/.config/ds/plugins
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".config", "ds", "plugins")
	}
}

// GetConfigPath returns the platform-specific default config file path
func GetConfigPath() string {
	home := homedir.Resolve(homedir.OSProvider{})

	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\ds\config.yaml
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "ds", "config.yaml")
		}
		if home != "" {
			return filepath.Join(home, "AppData", "Roaming", "ds", "config.yaml")
		}
		return ""
	default:
		// Linux/macOS: ~/.config/ds/config.yaml
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".config", "ds", "config.yaml")
	}
}
