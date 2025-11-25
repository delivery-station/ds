package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Client represents an OCI registry client
type Client struct {
	authProvider *AuthProvider
	httpClient   *http.Client
	registry     string
	insecure     bool
}

// ClientOption is a functional option for configuring the client
type ClientOption func(*Client)

// WithInsecure enables insecure (HTTP) registry connections
func WithInsecure(insecure bool) ClientOption {
	return func(c *Client) {
		c.insecure = insecure
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithProxy configures proxy settings
func WithProxy(proxyURL string) ClientOption {
	return func(c *Client) {
		if proxyURL != "" {
			proxy, err := url.Parse(proxyURL)
			if err != nil {
				logrus.Warnf("Invalid proxy URL: %v", err)
				return
			}

			transport := &http.Transport{
				Proxy: http.ProxyURL(proxy),
			}

			c.httpClient = &http.Client{
				Transport: transport,
				Timeout:   30 * time.Second,
			}
		}
	}
}

// NewClient creates a new OCI registry client
func NewClient(registry string, authProvider *AuthProvider, opts ...ClientOption) (*Client, error) {
	client := &Client{
		authProvider: authProvider,
		registry:     registry,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Pull downloads an OCI artifact from the registry
func (c *Client) Pull(ctx context.Context, reference string, dest io.Writer) error {
	logrus.Infof("Pulling %s from %s", reference, c.registry)

	// Create repository
	repo, err := c.createRepository(reference)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	// Parse reference to get tag
	tag := "latest"
	if idx := lastIndex(reference, ":"); idx != -1 {
		tag = reference[idx+1:]
		reference = reference[:idx]
	}

	// Pull manifest
	manifestDesc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to resolve reference: %w", err)
	}

	logrus.Debugf("Resolved manifest: %s (size: %d bytes)", manifestDesc.Digest, manifestDesc.Size)

	// Fetch manifest
	manifestReader, err := repo.Fetch(ctx, manifestDesc)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer manifestReader.Close()

	// Copy to destination
	_, err = io.Copy(dest, manifestReader)
	if err != nil {
		return fmt.Errorf("failed to copy artifact: %w", err)
	}

	logrus.Info("Pull completed successfully")
	return nil
}

// Push uploads an OCI artifact to the registry
func (c *Client) Push(ctx context.Context, reference string, content io.Reader, contentType string) error {
	logrus.Infof("Pushing %s to %s", reference, c.registry)

	// Create repository
	repo, err := c.createRepository(reference)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	// Parse reference to get tag
	tag := "latest"
	if idx := lastIndex(reference, ":"); idx != -1 {
		tag = reference[idx+1:]
		reference = reference[:idx]
	}

	// Read content into bytes
	data, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}

	// Push bytes using ORAS
	desc, err := oras.PushBytes(ctx, repo, contentType, data)
	if err != nil {
		return fmt.Errorf("failed to push artifact: %w", err)
	}

	// Tag the artifact
	err = repo.Tag(ctx, desc, tag)
	if err != nil {
		return fmt.Errorf("failed to tag artifact: %w", err)
	}

	logrus.Info("Push completed successfully")
	return nil
}

// List returns available versions/tags for a repository
func (c *Client) List(ctx context.Context, repository string) ([]string, error) {
	logrus.Debugf("Listing tags for %s", repository)

	// Create repository
	repo, err := c.createRepository(repository)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	// List tags
	var tags []string
	err = repo.Tags(ctx, "", func(tagsPage []string) error {
		tags = append(tags, tagsPage...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	logrus.Debugf("Found %d tags", len(tags))
	return tags, nil
}

// GetManifest fetches the manifest for an artifact
func (c *Client) GetManifest(ctx context.Context, reference string) ([]byte, error) {
	logrus.Debugf("Fetching manifest for %s", reference)

	// Create repository
	repo, err := c.createRepository(reference)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	// Parse reference to get tag
	tag := "latest"
	if idx := lastIndex(reference, ":"); idx != -1 {
		tag = reference[idx+1:]
		reference = reference[:idx]
	}

	// Resolve manifest descriptor
	manifestDesc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve reference: %w", err)
	}

	// Fetch manifest
	manifestReader, err := repo.Fetch(ctx, manifestDesc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer manifestReader.Close()

	// Read manifest
	manifest, err := io.ReadAll(manifestReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	return manifest, nil
}

// createRepository creates an ORAS repository with authentication
func (c *Client) createRepository(reference string) (*remote.Repository, error) {
	// Parse reference to extract repository name
	repoName := reference
	if idx := lastIndex(reference, ":"); idx != -1 {
		repoName = reference[:idx]
	}

	// Create repository
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", c.registry, repoName))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	// Configure client
	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
	}

	// Set up authentication
	var creds *Credentials
	if c.authProvider != nil {
		var authErr error
		creds, authErr = c.authProvider.GetCredentials(c.registry)
		if authErr != nil {
			logrus.Warnf("Failed to get credentials: %v", authErr)
		}
	}

	if creds != nil {
		credential := auth.Credential{
			Username: creds.Username,
			Password: creds.Password,
		}

		if creds.Token != "" {
			credential.RefreshToken = creds.Token
		}

		repo.Client.(*auth.Client).Credential = func(ctx context.Context, registry string) (auth.Credential, error) {
			return credential, nil
		}
	}

	// Handle insecure registries
	if c.insecure {
		repo.PlainHTTP = true
	}

	return repo, nil
}

// lastIndex returns the last index of substr in s, or -1 if not found
func lastIndex(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
