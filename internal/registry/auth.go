package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/delivery-station/ds/pkg/log"
)

// DockerConfig represents the Docker config.json structure
type DockerConfig struct {
	Auths map[string]DockerAuth `json:"auths"`
}

// DockerAuth represents authentication for a registry
type DockerAuth struct {
	Auth          string `json:"auth,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	IdentityToken string `json:"identitytoken,omitempty"`
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

// LoadDockerConfig loads credentials from the default Docker config path.
func (a *AuthProvider) LoadDockerConfig() error {
	return a.LoadDockerConfigFrom("")
}

// LoadDockerConfigFrom loads Docker credentials from the provided config path.
// When path is empty, the default ~/.docker/config.json is used.
func (a *AuthProvider) LoadDockerConfigFrom(path string) error {
	configPath := resolveDockerConfigPath(path)
	if configPath == "" {
		configPath = filepath.Join(os.Getenv("HOME"), ".docker", "config.json")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Debug("Docker config.json not found, skipping", "path", configPath)
			return nil
		}
		return fmt.Errorf("failed to read docker config %s: %w", configPath, err)
	}

	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse docker config %s: %w", configPath, err)
	}

	a.dockerConfig = &config
	log.Debug("Loaded Docker config", "path", configPath, "registries", len(config.Auths))

	return nil
}

// LoadCredentials loads credentials from DS configuration
func (a *AuthProvider) LoadCredentials(creds []Credentials) {
	for _, cred := range creds {
		a.credentials[normalizeRegistry(cred.Registry)] = &cred
	}
	log.Debug("Loaded credentials from config", "count", len(creds))
}

// GetCredentials returns credentials for a specific registry
func (a *AuthProvider) GetCredentials(registry string) (*Credentials, error) {
	normalized := normalizeRegistry(registry)

	// Check explicit credentials first
	if cred, ok := a.credentials[normalized]; ok {
		log.Debug("Using explicit credentials", "registry", registry)
		return cred, nil
	}

	// Check Docker config
	if a.dockerConfig != nil {
		if auth, ok := a.dockerConfig.Auths[normalized]; ok {
			cred := &Credentials{Registry: registry}

			if auth.Auth != "" {
				username, password, err := decodeDockerAuth(auth.Auth)
				if err != nil {
					log.Warn("Failed to decode docker auth entry", "registry", registry, "error", err)
				} else {
					cred.Username = username
					cred.Password = password
				}
			}

			if cred.Username == "" && auth.Username != "" && auth.Password != "" {
				cred.Username = auth.Username
				cred.Password = auth.Password
			}

			if cred.Username == "" && auth.IdentityToken != "" {
				cred.Username = "token"
				cred.Password = auth.IdentityToken
			}

			if cred.Username != "" || cred.Password != "" {
				log.Debug("Using Docker config credentials", "registry", registry)
				return cred, nil
			}
		}
	}

	// No credentials found
	log.Debug("No credentials found", "registry", registry)
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

func resolveDockerConfigPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, trimmed[2:])
		}
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

// RefreshToken refreshes an expired token (placeholder for future implementation)
func (a *AuthProvider) RefreshToken(ctx context.Context, registry string) error {
	// TODO: Implement OAuth2 token refresh
	log.Debug("Token refresh not yet implemented", "registry", registry)
	return nil
}
