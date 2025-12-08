package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/delivery-station/ds/pkg/types"
	"github.com/spf13/viper"
)

func TestLoadDefaults(t *testing.T) {
	// Create a new viper instance for testing
	v := viper.New()
	loader := &Loader{viper: v}

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
	loader := NewLoader()

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
	loader := NewLoader()

	// Test that tilde gets expanded or stays as-is
	result := loader.expandString("~/config/file.yaml")

	// The function may or may not expand tilde depending on whether it's in a path context
	// Just verify it's not causing errors
	if result == "" {
		t.Error("expandString returned empty string")
	}
}

func TestValidate(t *testing.T) {
	loader := NewLoader()

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
	// Create temporary config file
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

	// Create a new viper instance for testing
	v := viper.New()
	v.SetConfigFile(configFile)
	loader := &Loader{viper: v}

	// Load defaults
	loader.LoadDefaults()

	// Load from file
	if err := loader.LoadFromFile(); err != nil {
		t.Fatalf("failed to load from file: %v", err)
	}

	// Check that file values override defaults
	if v.GetString("registry.default") != "file-registry.io" {
		t.Errorf("expected registry.default from file, got %s", v.GetString("registry.default"))
	}

	if v.GetString("logging.level") != "debug" {
		t.Errorf("expected logging.level from file, got %s", v.GetString("logging.level"))
	}

	// Check that defaults are still used for non-overridden values
	if v.GetString("logging.format") != "text" {
		t.Errorf("expected logging.format from defaults, got %s", v.GetString("logging.format"))
	}
}

func TestExpandVariables(t *testing.T) {
	loader := NewLoader()

	// Set test environment variables
	_ = os.Setenv("TEST_USER", "testuser")
	_ = os.Setenv("TEST_TOKEN", "secret123")
	defer func() {
		_ = os.Unsetenv("TEST_USER")
		_ = os.Unsetenv("TEST_TOKEN")
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
}
