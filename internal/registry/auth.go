package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// DockerConfig represents the Docker config.json structure
type DockerConfig struct {
	Auths map[string]DockerAuth `json:"auths"`
}

// DockerAuth represents authentication for a registry
type DockerAuth struct {
	Auth     string `json:"auth,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Credentials represents registry credentials
type Credentials struct {
	Registry string
	Username string
	Password string
	Token    string
}

// AuthProvider manages authentication for registries
type AuthProvider struct {
	dockerConfig *DockerConfig
	credentials  map[string]*Credentials
}

// NewAuthProvider creates a new authentication provider
func NewAuthProvider() *AuthProvider {
	return &AuthProvider{
		credentials: make(map[string]*Credentials),
	}
}

// LoadDockerConfig loads credentials from ~/.docker/config.json
func (a *AuthProvider) LoadDockerConfig() error {
	configPath := filepath.Join(os.Getenv("HOME"), ".docker", "config.json")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logrus.Debug("Docker config.json not found, skipping")
		return nil
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read docker config: %w", err)
	}

	// Parse JSON
	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse docker config: %w", err)
	}

	a.dockerConfig = &config
	logrus.Debugf("Loaded Docker config with %d registries", len(config.Auths))

	return nil
}

// LoadCredentials loads credentials from DS configuration
func (a *AuthProvider) LoadCredentials(creds []Credentials) {
	for _, cred := range creds {
		a.credentials[normalizeRegistry(cred.Registry)] = &cred
	}
	logrus.Debugf("Loaded %d credentials from config", len(creds))
}

// GetCredentials returns credentials for a specific registry
func (a *AuthProvider) GetCredentials(registry string) (*Credentials, error) {
	normalized := normalizeRegistry(registry)

	// Check explicit credentials first
	if cred, ok := a.credentials[normalized]; ok {
		logrus.Debugf("Using explicit credentials for %s", registry)
		return cred, nil
	}

	// Check Docker config
	if a.dockerConfig != nil {
		if auth, ok := a.dockerConfig.Auths[normalized]; ok {
			cred := &Credentials{
				Registry: registry,
			}

			// Try different auth formats
			if auth.Auth != "" {
				// Base64 encoded username:password
				cred.Token = auth.Auth
			} else if auth.Username != "" && auth.Password != "" {
				cred.Username = auth.Username
				cred.Password = auth.Password
			}

			if cred.Token != "" || cred.Username != "" {
				logrus.Debugf("Using Docker config credentials for %s", registry)
				return cred, nil
			}
		}
	}

	// No credentials found
	logrus.Debugf("No credentials found for %s", registry)
	return nil, nil
}

// AddCredential adds a credential for a registry
func (a *AuthProvider) AddCredential(registry, username, password string) {
	a.credentials[normalizeRegistry(registry)] = &Credentials{
		Registry: registry,
		Username: username,
		Password: password,
	}
}

// normalizeRegistry normalizes registry URLs for consistent lookup
func normalizeRegistry(registry string) string {
	// Remove protocol
	registry = strings.TrimPrefix(registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")

	// Remove trailing slash
	registry = strings.TrimSuffix(registry, "/")

	// Docker Hub special case
	if registry == "docker.io" || registry == "index.docker.io" {
		return "https://index.docker.io/v1/"
	}

	return registry
}

// RefreshToken refreshes an expired token (placeholder for future implementation)
func (a *AuthProvider) RefreshToken(ctx context.Context, registry string) error {
	// TODO: Implement OAuth2 token refresh
	logrus.Debugf("Token refresh not yet implemented for %s", registry)
	return nil
}
