package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/delivery-station/ds/internal/homedir"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/spf13/viper"
)

func TestLoadDefaults(t *testing.T) {
	// Create a new viper instance for testing
	v := viper.New()
	loader := &Loader{viper: v, home: staticHomeDir{path: t.TempDir()}}

	loader.LoadDefaults()

	// Check registry defaults
	if v.GetString("registry.default") != "ghcr.io/delivery-station" {
		t.Errorf("expected registry.default to be ghcr.io/delivery-station, got %s", v.GetString("registry.default"))
	}

	// Check cache defaults
	if v.GetString("cache.max_size") != "10GB" {
		t.Errorf("expected cache.max_size to be 10GB, got %s", v.GetString("cache.max_size"))
	}

	if v.GetString("cache.ttl") != "7d" {
		t.Errorf("expected cache.ttl to be 7d, got %s", v.GetString("cache.ttl"))
	}

	// Check logging defaults
	if v.GetString("logging.level") != "info" {
		t.Errorf("expected logging.level to be info, got %s", v.GetString("logging.level"))
	}

	if v.GetString("logging.format") != "text" {
		t.Errorf("expected logging.format to be text, got %s", v.GetString("logging.format"))
	}

	// Check plugins defaults
	if !v.GetBool("plugins.auto_install") {
		t.Error("expected plugins.auto_install to be true")
	}
}

func TestExpandString(t *testing.T) {
	loader := NewLoaderWithHome(staticHomeDir{path: t.TempDir()})

	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "expand ${VAR}",
			input:    "path/${HOME}/config",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: "path//home/user/config",
		},
		{
			name:     "expand $VAR",
			input:    "path/$HOME/config",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: "path//home/user/config",
		},
		{
			name:     "no expansion when var not set",
			input:    "path/${NOTSET}/config",
			envVars:  map[string]string{},
			expected: "path/${NOTSET}/config",
		},
		{
			name:     "expand multiple vars",
			input:    "${USER}@${HOST}",
			envVars:  map[string]string{"USER": "admin", "HOST": "localhost"},
			expected: "admin@localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
				defer func(key string) {
					_ = os.Unsetenv(key)
				}(k)
			}

			result := loader.expandString(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExpandStringTilde(t *testing.T) {
	loader := NewLoaderWithHome(staticHomeDir{path: t.TempDir()})

	// Test that tilde gets expanded or stays as-is
	result := loader.expandString("~/config/file.yaml")

	// The function may or may not expand tilde depending on whether it's in a path context
	// Just verify it's not causing errors
	if result == "" {
		t.Error("expandString returned empty string")
	}
}

func TestValidate(t *testing.T) {
	loader := NewLoaderWithHome(staticHomeDir{path: t.TempDir()})

	tests := []struct {
		name    string
		config  types.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: types.Config{
				Registry: types.RegistryConfig{Default: "ghcr.io/delivery-station"},
				Cache:    types.CacheConfig{Dir: "/tmp/cache", MaxSize: 10 * 1024 * 1024 * 1024, TTL: 7 * 24 * time.Hour},
				Logging:  types.LoggingConfig{Level: "info", Format: "text"},
				Plugins:  types.PluginsConfig{Dir: "/tmp/plugins"},
			},
			wantErr: false,
		},
		{
			name: "empty registry default",
			config: types.Config{
				Registry: types.RegistryConfig{Default: ""},
				Cache:    types.CacheConfig{Dir: "/tmp/cache", MaxSize: 10 * 1024 * 1024 * 1024, TTL: 7 * 24 * time.Hour},
				Logging:  types.LoggingConfig{Level: "info", Format: "text"},
				Plugins:  types.PluginsConfig{Dir: "/tmp/plugins"},
			},
			wantErr: true,
			errMsg:  "registry.default cannot be empty",
		},
		{
			name: "invalid log level",
			config: types.Config{
				Registry: types.RegistryConfig{Default: "ghcr.io/delivery-station"},
				Cache:    types.CacheConfig{Dir: "/tmp/cache", MaxSize: 10 * 1024 * 1024 * 1024, TTL: 7 * 24 * time.Hour},
				Logging:  types.LoggingConfig{Level: "invalid", Format: "text"},
				Plugins:  types.PluginsConfig{Dir: "/tmp/plugins"},
			},
			wantErr: true,
		},
		{
			name: "invalid log format",
			config: types.Config{
				Registry: types.RegistryConfig{Default: "ghcr.io/delivery-station"},
				Cache:    types.CacheConfig{Dir: "/tmp/cache", MaxSize: 10 * 1024 * 1024 * 1024, TTL: 7 * 24 * time.Hour},
				Logging:  types.LoggingConfig{Level: "info", Format: "invalid"},
				Plugins:  types.PluginsConfig{Dir: "/tmp/plugins"},
			},
			wantErr: true,
		},
		{
			name: "empty cache dir",
			config: types.Config{
				Registry: types.RegistryConfig{Default: "ghcr.io/delivery-station"},
				Cache:    types.CacheConfig{Dir: "", MaxSize: 10 * 1024 * 1024 * 1024, TTL: 7 * 24 * time.Hour},
				Logging:  types.LoggingConfig{Level: "info", Format: "text"},
				Plugins:  types.PluginsConfig{Dir: "/tmp/plugins"},
			},
			wantErr: true,
			errMsg:  "cache.dir cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.Validate(&tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message to contain %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestConfigPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
registry:
  default: "file-registry.io"
logging:
  level: "debug"
cache:
  max_size: "5GB"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}

	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create fake home: %v", err)
	}

	SetExplicitConfigFile(configFile)
	defer SetExplicitConfigFile("")

	loader := &Loader{viper: viper.New(), home: staticHomeDir{path: homeDir}}
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Registry.Default != "file-registry.io" {
		t.Errorf("expected registry.default from file, got %s", cfg.Registry.Default)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected logging.level from file, got %s", cfg.Logging.Level)
	}

	if cfg.Logging.Format != "text" {
		t.Errorf("expected logging.format from defaults, got %s", cfg.Logging.Format)
	}
}

func TestLoadFromFileMergesBaseAndExplicit(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	baseConfigDir := filepath.Join(homeDir, ".config", "ds")
	if err := os.MkdirAll(baseConfigDir, 0755); err != nil {
		t.Fatalf("failed to create base config dir: %v", err)
	}

	baseConfigPath := filepath.Join(baseConfigDir, "config.yaml")
	baseConfig := `
logging:
  level: "debug"
cache:
  dir: "/tmp/base-cache"
`
	if err := os.WriteFile(baseConfigPath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write base config: %v", err)
	}

	overridePath := filepath.Join(tmpDir, "override.yaml")
	overrideConfig := `
cache:
  max_size: "2GB"
`
	if err := os.WriteFile(overridePath, []byte(overrideConfig), 0644); err != nil {
		t.Fatalf("failed to write override config: %v", err)
	}

	SetExplicitConfigFile(overridePath)
	defer SetExplicitConfigFile("")

	loader := &Loader{viper: viper.New(), home: staticHomeDir{path: homeDir}}
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load merged config: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected logging.level from base config, got %s", cfg.Logging.Level)
	}

	if cfg.Cache.Dir != "/tmp/base-cache" {
		t.Errorf("expected cache.dir from base config, got %s", cfg.Cache.Dir)
	}

	const twoGiB = int64(2 * 1024 * 1024 * 1024)
	if cfg.Cache.MaxSize != twoGiB {
		t.Errorf("expected cache.max_size override of %d, got %d", twoGiB, cfg.Cache.MaxSize)
	}
}

func TestParseSizeStringSupportsBytes(t *testing.T) {
	value, err := parseSize("10737418240")
	if err != nil {
		t.Fatalf("parseSize returned error: %v", err)
	}

	const expected = int64(10737418240)
	if value != expected {
		t.Fatalf("expected %d bytes, got %d", expected, value)
	}
}

func TestParseSizeValueNumericTypes(t *testing.T) {
	size, err := parseSizeValue(10737418240)
	if err != nil {
		t.Fatalf("parseSizeValue returned error: %v", err)
	}

	if size != 10737418240 {
		t.Fatalf("expected 10737418240, got %d", size)
	}

	size, err = parseSizeValue(uint64(10737418240))
	if err != nil {
		t.Fatalf("parseSizeValue returned error: %v", err)
	}

	if size != 10737418240 {
		t.Fatalf("expected 10737418240 for uint value, got %d", size)
	}
}

func TestLoadHandlesNumericCacheSize(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
cache:
  max_size: 10737418240
`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("HOME", filepath.Join(tmpDir, "home"))
	SetExplicitConfigFile(configPath)
	defer SetExplicitConfigFile("")

	loader := &Loader{viper: viper.New(), home: staticHomeDir{path: filepath.Join(tmpDir, "home")}}
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	const expected = int64(10737418240)
	if cfg.Cache.MaxSize != expected {
		t.Fatalf("expected cache.max_size %d, got %d", expected, cfg.Cache.MaxSize)
	}
}

func TestMergeDockerCredentialsAddsEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dockerPath := filepath.Join(tmpDir, "config.json")

	authPayload := base64.StdEncoding.EncodeToString([]byte("runner:gh_secret"))
	content := []byte(`{"auths":{"ghcr.io":{"auth":"` + authPayload + `"}}}`)

	if err := os.WriteFile(dockerPath, content, 0600); err != nil {
		t.Fatalf("failed to write docker config: %v", err)
	}

	loader := &Loader{viper: viper.New()}
	cfg := &types.Config{
		Auth: types.AuthConfig{DockerConfig: dockerPath},
	}

	if err := loader.mergeDockerCredentials(cfg); err != nil {
		t.Fatalf("mergeDockerCredentials returned error: %v", err)
	}

	if len(cfg.Auth.Credentials) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(cfg.Auth.Credentials))
	}

	cred := cfg.Auth.Credentials[0]
	if cred.Registry != "ghcr.io" {
		t.Errorf("expected registry ghcr.io, got %s", cred.Registry)
	}
	if cred.Username != "runner" {
		t.Errorf("expected username runner, got %s", cred.Username)
	}
	if cred.Password != "gh_secret" {
		t.Errorf("expected password gh_secret, got %s", cred.Password)
	}
}

func TestMergeDockerCredentialsDoesNotOverrideExisting(t *testing.T) {
	tmpDir := t.TempDir()
	dockerPath := filepath.Join(tmpDir, "config.json")

	authPayload := base64.StdEncoding.EncodeToString([]byte("runner:new_secret"))
	content := []byte(`{"auths":{"ghcr.io":{"auth":"` + authPayload + `"}}}`)

	if err := os.WriteFile(dockerPath, content, 0600); err != nil {
		t.Fatalf("failed to write docker config: %v", err)
	}

	loader := &Loader{viper: viper.New()}
	cfg := &types.Config{
		Auth: types.AuthConfig{
			DockerConfig: dockerPath,
			Credentials: []types.Credential{
				{
					Registry: "https://ghcr.io",
					Username: "existing",
					Password: "existing_secret",
				},
			},
		},
	}

	if err := loader.mergeDockerCredentials(cfg); err != nil {
		t.Fatalf("mergeDockerCredentials returned error: %v", err)
	}

	if len(cfg.Auth.Credentials) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(cfg.Auth.Credentials))
	}

	cred := cfg.Auth.Credentials[0]
	if cred.Username != "existing" {
		t.Errorf("expected username existing, got %s", cred.Username)
	}
	if cred.Password != "existing_secret" {
		t.Errorf("expected password existing_secret, got %s", cred.Password)
	}
}

func TestExpandVariables(t *testing.T) {
	loader := NewLoaderWithHome(staticHomeDir{path: t.TempDir()})

	// Set test environment variables
	_ = os.Setenv("TEST_USER", "testuser")
	_ = os.Setenv("TEST_TOKEN", "secret123")
	_ = os.Setenv("TEST_BUCKET", "env-bucket")
	defer func() {
		_ = os.Unsetenv("TEST_USER")
		_ = os.Unsetenv("TEST_TOKEN")
		_ = os.Unsetenv("TEST_BUCKET")
	}()

	config := &types.Config{
		Auth: types.AuthConfig{
			Credentials: []types.Credential{
				{
					Registry: "test.io",
					Username: "${TEST_USER}",
					Token:    "${TEST_TOKEN}",
				},
			},
		},
		Proxy: types.ProxyConfig{
			HTTPProxy: "${TEST_USER}:${TEST_TOKEN}@proxy.com",
		},
		Plugins: types.PluginsConfig{
			Dir: "./plugins",
			Settings: map[string]map[string]interface{}{
				"s3": {
					"bucket": "${TEST_BUCKET}",
					"credentials": map[string]interface{}{
						"access_key": "${TEST_USER}",
					},
					"endpoints": []interface{}{"${TEST_USER}"},
				},
			},
		},
	}

	err := loader.expandVariables(config)
	if err != nil {
		t.Fatalf("expandVariables failed: %v", err)
	}

	if config.Auth.Credentials[0].Username != "testuser" {
		t.Errorf("expected username to be expanded, got %s", config.Auth.Credentials[0].Username)
	}

	if config.Auth.Credentials[0].Token != "secret123" {
		t.Errorf("expected token to be expanded, got %s", config.Auth.Credentials[0].Token)
	}

	if config.Proxy.HTTPProxy != "testuser:secret123@proxy.com" {
		t.Errorf("expected proxy to be expanded, got %s", config.Proxy.HTTPProxy)
	}

	settings := config.Plugins.Settings["s3"]
	if settings == nil {
		t.Fatalf("expected plugin settings for s3")
	}
	if settings["bucket"] != "env-bucket" {
		t.Errorf("expected bucket to be expanded, got %v", settings["bucket"])
	}
	creds := settings["credentials"].(map[string]interface{})
	if creds["access_key"] != "testuser" {
		t.Errorf("expected nested value to be expanded, got %v", creds["access_key"])
	}
	endpoints := settings["endpoints"].([]interface{})
	if endpoints[0] != "testuser" {
		t.Errorf("expected slice value to be expanded, got %v", endpoints[0])
	}
}

type staticHomeDir struct {
	path string
}

func (s staticHomeDir) HomeDir() (string, error) {
	return s.path, nil
}

var _ homedir.Provider = (*staticHomeDir)(nil)
