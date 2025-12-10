package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/delivery-station/ds/internal/registry"
)

func TestNewInstaller(t *testing.T) {
	installer := NewInstaller("/test/plugins", "ghcr.io/delivery-station", nil)

	if installer == nil {
		t.Fatal("NewInstaller returned nil")
	}

	if installer.pluginDir != "/test/plugins" {
		t.Errorf("expected plugin dir '/test/plugins', got '%s'", installer.pluginDir)
	}

	if installer.registry != "ghcr.io/delivery-station" {
		t.Errorf("expected registry 'ghcr.io/delivery-station', got '%s'", installer.registry)
	}
}

func TestResolvePlatform(t *testing.T) {
	platform := ResolvePlatform()

	expectedOS := runtime.GOOS
	expectedArch := runtime.GOARCH
	expected := expectedOS + "/" + expectedArch

	if platform != expected {
		t.Errorf("expected platform '%s', got '%s'", expected, platform)
	}
}

func TestParsePluginReference(t *testing.T) {
	// This is testing the cmd function, but we'll create a local version for testing
	parseRef := func(ref string) (string, string) {
		// Same logic as parsePluginReference in plugin_install.go
		if idx := len(ref) - 1; idx >= 0 {
			for i := len(ref) - 1; i >= 0; i-- {
				if ref[i] == '@' {
					return ref[:i], ref[i+1:]
				}
			}
			for i := len(ref) - 1; i >= 0; i-- {
				if ref[i] == ':' {
					return ref[:i], ref[i+1:]
				}
			}
		}
		return ref, "latest"
	}

	tests := []struct {
		ref             string
		expectedName    string
		expectedVersion string
	}{
		{
			ref:             "porter",
			expectedName:    "porter",
			expectedVersion: "latest",
		},
		{
			ref:             "porter@1.0.0",
			expectedName:    "porter",
			expectedVersion: "1.0.0",
		},
		{
			ref:             "porter:v2.0.0",
			expectedName:    "porter",
			expectedVersion: "v2.0.0",
		},
		{
			ref:             "myorg/plugin@1.2.3",
			expectedName:    "myorg/plugin",
			expectedVersion: "1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			name, version := parseRef(tt.ref)
			if name != tt.expectedName {
				t.Errorf("expected name '%s', got '%s'", tt.expectedName, name)
			}
			if version != tt.expectedVersion {
				t.Errorf("expected version '%s', got '%s'", tt.expectedVersion, version)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	content := []byte("test content")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dest.txt")
	err := copyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify destination exists and has same content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("content mismatch: expected '%s', got '%s'", content, dstContent)
	}
}

func TestRemovePlugin(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock plugin files
	pluginName := "testplugin"
	binaryName := pluginName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	binaryPath := filepath.Join(tmpDir, binaryName)

	// Create files
	_ = os.WriteFile(binaryPath, []byte("binary"), 0755)

	// Create installer
	authProvider := registry.NewAuthProvider()
	client, _ := registry.NewClient("ghcr.io/delivery-station", authProvider)
	installer := NewInstaller(tmpDir, "ghcr.io/delivery-station", client)

	// Remove plugin
	ctx := context.Background()
	err := installer.RemovePlugin(ctx, pluginName)
	if err != nil {
		t.Fatalf("RemovePlugin failed: %v", err)
	}

	// Verify files are removed
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		t.Error("binary still exists after removal")
	}
}

func TestRemovePlugin_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	authProvider := registry.NewAuthProvider()
	client, _ := registry.NewClient("ghcr.io/delivery-station", authProvider)
	installer := NewInstaller(tmpDir, "ghcr.io/delivery-station", client)

	// Try to remove non-existent plugin (should not error)
	ctx := context.Background()
	err := installer.RemovePlugin(ctx, "nonexistent")
	if err != nil {
		t.Errorf("RemovePlugin should not error on non-existent plugin: %v", err)
	}
}

func TestBackupPlugin(t *testing.T) {
	tmpDir := t.TempDir()

	pluginName := "testplugin"
	binaryName := pluginName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	binaryPath := filepath.Join(tmpDir, binaryName)
	content := []byte("binary content")
	if err := os.WriteFile(binaryPath, content, 0755); err != nil {
		t.Fatalf("failed to create binary: %v", err)
	}

	authProvider := registry.NewAuthProvider()
	client, _ := registry.NewClient("ghcr.io/delivery-station", authProvider)
	installer := NewInstaller(tmpDir, "ghcr.io/delivery-station", client)

	// Backup plugin
	err := installer.backupPlugin(pluginName)
	if err != nil {
		t.Fatalf("backupPlugin failed: %v", err)
	}

	// Verify backup exists
	backupPath := filepath.Join(tmpDir, binaryName+".bak")
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}

	if string(backupContent) != string(content) {
		t.Error("backup content doesn't match original")
	}
}
