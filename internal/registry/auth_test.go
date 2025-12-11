package registry

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewAuthProvider(t *testing.T) {
	provider := NewAuthProvider()
	if provider == nil {
		t.Fatal("NewAuthProvider returned nil")
	}

	if provider.credentials == nil {
		t.Error("credentials map not initialized")
	}
}

func TestLoadDockerConfig(t *testing.T) {
	// Create temporary docker config
	tmpDir := t.TempDir()
	dockerDir := filepath.Join(tmpDir, ".docker")
	if err := os.MkdirAll(dockerDir, 0755); err != nil {
		t.Fatalf("failed to create docker dir: %v", err)
	}

	config := DockerConfig{
		Auths: map[string]DockerAuth{
			"ghcr.io": {
				Username: "testuser",
				Password: "testpass",
			},
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	configPath := filepath.Join(dockerDir, "config.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	provider := NewAuthProvider()
	err = provider.LoadDockerConfigFrom(configPath)
	if err != nil {
		t.Fatalf("LoadDockerConfig failed: %v", err)
	}

	if provider.dockerConfig == nil {
		t.Error("docker config not loaded")
	}

	if len(provider.dockerConfig.Auths) != 1 {
		t.Errorf("expected 1 auth, got %d", len(provider.dockerConfig.Auths))
	}
}

func TestLoadDockerConfig_NotFound(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing", "config.json")

	provider := NewAuthProvider()
	err := provider.LoadDockerConfigFrom(missingPath)

	// Should not error on missing file
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadCredentials(t *testing.T) {
	provider := NewAuthProvider()

	creds := []Credentials{
		{
			Registry: "ghcr.io",
			Username: "user1",
			Password: "pass1",
		},
		{
			Registry: "docker.io",
			Username: "user2",
			Password: "pass2",
		},
	}

	provider.LoadCredentials(creds)

	if len(provider.credentials) != 2 {
		t.Errorf("expected 2 credentials, got %d", len(provider.credentials))
	}
}

func TestGetCredentials_Explicit(t *testing.T) {
	provider := NewAuthProvider()

	provider.AddCredential("ghcr.io", "testuser", "testpass")

	cred, err := provider.GetCredentials("ghcr.io")
	if err != nil {
		t.Fatalf("GetCredentials failed: %v", err)
	}

	if cred == nil {
		t.Fatal("expected credentials, got nil")
	}

	if cred.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", cred.Username)
	}

	if cred.Password != "testpass" {
		t.Errorf("expected password 'testpass', got '%s'", cred.Password)
	}
}

func TestGetCredentials_NotFound(t *testing.T) {
	provider := NewAuthProvider()

	cred, err := provider.GetCredentials("nonexistent.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cred != nil {
		t.Error("expected nil credentials for non-existent registry")
	}
}

func TestNormalizeRegistry(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "ghcr.io",
			expected: "ghcr.io",
		},
		{
			input:    "https://ghcr.io",
			expected: "ghcr.io",
		},
		{
			input:    "http://ghcr.io",
			expected: "ghcr.io",
		},
		{
			input:    "ghcr.io/",
			expected: "ghcr.io",
		},
		{
			input:    "docker.io",
			expected: "https://index.docker.io/v1/",
		},
		{
			input:    "index.docker.io",
			expected: "https://index.docker.io/v1/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeRegistry(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRegistry(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAddCredential(t *testing.T) {
	provider := NewAuthProvider()

	provider.AddCredential("test.io", "user", "pass")

	cred, err := provider.GetCredentials("test.io")
	if err != nil {
		t.Fatalf("GetCredentials failed: %v", err)
	}

	if cred == nil {
		t.Fatal("expected credentials")
	}

	if cred.Username != "user" || cred.Password != "pass" {
		t.Error("credentials not stored correctly")
	}
}

func TestGetCredentials_FromDockerConfig(t *testing.T) {
	provider := NewAuthProvider()

	// Set up docker config
	provider.dockerConfig = &DockerConfig{
		Auths: map[string]DockerAuth{
			"test.io": {
				Username: "dockeruser",
				Password: "dockerpass",
			},
		},
	}

	cred, err := provider.GetCredentials("test.io")
	if err != nil {
		t.Fatalf("GetCredentials failed: %v", err)
	}

	if cred == nil {
		t.Fatal("expected credentials from docker config")
	}

	if cred.Username != "dockeruser" {
		t.Errorf("expected username 'dockeruser', got '%s'", cred.Username)
	}
}

func TestGetCredentials_FromDockerConfigAuthField(t *testing.T) {
	provider := NewAuthProvider()

	encoded := base64.StdEncoding.EncodeToString([]byte("runner:gh_token"))
	provider.dockerConfig = &DockerConfig{
		Auths: map[string]DockerAuth{
			"ghcr.io": {
				Auth: encoded,
			},
		},
	}

	cred, err := provider.GetCredentials("ghcr.io")
	if err != nil {
		t.Fatalf("GetCredentials failed: %v", err)
	}

	if cred == nil {
		t.Fatal("expected credentials from docker config auth field")
	}

	if cred.Username != "runner" {
		t.Errorf("expected username runner, got %s", cred.Username)
	}
	if cred.Password != "gh_token" {
		t.Errorf("expected password gh_token, got %s", cred.Password)
	}
}

func TestGetCredentials_PrecedenceOrder(t *testing.T) {
	provider := NewAuthProvider()

	// Add docker config
	provider.dockerConfig = &DockerConfig{
		Auths: map[string]DockerAuth{
			"test.io": {
				Username: "dockeruser",
				Password: "dockerpass",
			},
		},
	}

	// Add explicit credential (should take precedence)
	provider.AddCredential("test.io", "explicituser", "explicitpass")

	cred, err := provider.GetCredentials("test.io")
	if err != nil {
		t.Fatalf("GetCredentials failed: %v", err)
	}

	if cred == nil {
		t.Fatal("expected credentials")
	}

	// Should use explicit credential
	if cred.Username != "explicituser" {
		t.Errorf("expected explicit credential to take precedence, got username '%s'", cred.Username)
	}
}
