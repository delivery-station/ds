package registry

import (
	"context"
	"io"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	authProvider := NewAuthProvider()

	client, err := NewClient("ghcr.io", authProvider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.registry != "ghcr.io" {
		t.Errorf("expected registry 'ghcr.io', got '%s'", client.registry)
	}

	if client.authProvider != authProvider {
		t.Error("auth provider not set correctly")
	}

	if client.httpClient == nil {
		t.Error("http client not initialized")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	authProvider := NewAuthProvider()

	client, err := NewClient("ghcr.io", authProvider,
		WithInsecure(true),
		WithProxy("http://proxy.example.com:8080"),
	)

	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if !client.insecure {
		t.Error("insecure option not applied")
	}

	if client.httpClient == nil {
		t.Error("http client not set")
	}
}

func TestWithInsecure(t *testing.T) {
	authProvider := NewAuthProvider()

	client, _ := NewClient("localhost:5000", authProvider, WithInsecure(true))

	if !client.insecure {
		t.Error("WithInsecure option not applied")
	}
}

func TestWithProxy(t *testing.T) {
	authProvider := NewAuthProvider()

	client, _ := NewClient("ghcr.io", authProvider, WithProxy("http://proxy.test:8080"))

	if client.httpClient == nil {
		t.Error("WithProxy did not set http client")
	}

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestWithProxy_InvalidURL(t *testing.T) {
	authProvider := NewAuthProvider()

	// Should not error on invalid proxy URL, just log warning
	client, err := NewClient("ghcr.io", authProvider, WithProxy("://invalid"))

	if err != nil {
		t.Errorf("NewClient should not fail on invalid proxy URL: %v", err)
	}

	if client == nil {
		t.Error("client should still be created")
	}
}

func TestLastIndex(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   int
	}{
		{
			s:      "hello:world:test",
			substr: ":",
			want:   11,
		},
		{
			s:      "nosubstring",
			substr: ":",
			want:   -1,
		},
		{
			s:      "test:latest",
			substr: ":",
			want:   4,
		},
		{
			s:      "",
			substr: ":",
			want:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := lastIndex(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("lastIndex(%q, %q) = %d, want %d", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestPull_InvalidReference(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	authProvider := NewAuthProvider()
	client, err := NewClient("ghcr.io", authProvider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	// Try to pull non-existent artifact
	err = client.Pull(ctx, "nonexistent/artifact:v1.0.0", io.Discard)

	// Should get an error
	if err == nil {
		t.Error("expected error for non-existent artifact")
	}
}

func TestList_EmptyRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	authProvider := NewAuthProvider()
	client, err := NewClient("ghcr.io", authProvider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	// Try to list tags from non-existent repository
	_, err = client.List(ctx, "nonexistent/repo")

	// Should get an error or empty list
	// Don't fail test as this depends on registry behavior
	if err != nil {
		t.Logf("List returned error (expected): %v", err)
	}
}

func TestCreateRepository(t *testing.T) {
	authProvider := NewAuthProvider()
	authProvider.AddCredential("ghcr.io", "testuser", "testpass")

	client, err := NewClient("ghcr.io", authProvider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	repo, err := client.createRepository("test/repo:v1.0.0")
	if err != nil {
		t.Fatalf("createRepository failed: %v", err)
	}

	if repo == nil {
		t.Fatal("expected repository, got nil")
	}

	// Check that authentication is configured
	if repo.Client == nil {
		t.Error("repository client not configured")
	}
}

func TestCreateRepository_InsecureRegistry(t *testing.T) {
	authProvider := NewAuthProvider()

	client, err := NewClient("localhost:5000", authProvider, WithInsecure(true))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	repo, err := client.createRepository("test/repo")
	if err != nil {
		t.Fatalf("createRepository failed: %v", err)
	}

	if !repo.PlainHTTP {
		t.Error("expected PlainHTTP to be true for insecure registry")
	}
}
