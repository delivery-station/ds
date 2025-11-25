package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestNewExecutor(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	if executor == nil {
		t.Fatal("NewExecutor returned nil")
	}

	if executor.manager != mgr {
		t.Error("executor manager not set correctly")
	}

	if executor.timeout != 5*time.Minute {
		t.Errorf("expected default timeout 5m, got %v", executor.timeout)
	}
}

func TestSetTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	newTimeout := 10 * time.Second
	executor.SetTimeout(newTimeout)

	if executor.timeout != newTimeout {
		t.Errorf("expected timeout %v, got %v", newTimeout, executor.timeout)
	}
}

func TestPrepareEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	// Set some viper values
	viper.Set("registry.default", "test.registry.io")
	viper.Set("cache.dir", "/tmp/test-cache")
	viper.Set("plugins.dir", tmpDir)
	defer viper.Reset()

	env := executor.PrepareEnvironment()

	// Check that environment contains DS_* variables
	found := make(map[string]bool)
	for _, e := range env {
		if strings.HasPrefix(e, "DS_") {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				found[parts[0]] = true
			}
		}
	}

	expectedVars := []string{"DS_REGISTRY_DEFAULT", "DS_CACHE_DIR", "DS_PLUGIN_DIR"}
	for _, expectedVar := range expectedVars {
		if !found[expectedVar] {
			t.Errorf("expected environment variable %s not found", expectedVar)
		}
	}

	// Check that we have more than just DS_ vars (should include system env)
	if len(env) < 10 {
		t.Errorf("expected at least 10 env vars, got %d", len(env))
	}
}

func TestExecutePlugin_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	exitCode, err := executor.ExecutePlugin("nonexistent", []string{"test"})

	if err == nil {
		t.Error("expected error for non-existent plugin")
	}

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestExecutePlugin_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple test plugin
	pluginPath := filepath.Join(tmpDir, "ds-testexec")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	// Create a script that exits with 0
	script := "#!/bin/sh\necho 'Hello from plugin'\nexit 0\n"
	if err := os.WriteFile(pluginPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test plugin: %v", err)
	}

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-testexec.yaml")
	manifest := `name: testexec
version: 1.0.0
platform:
  os: [linux, darwin, windows]
  arch: [amd64, arm64]
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	// Discover plugins
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	// Execute plugin
	exitCode, err := executor.ExecutePlugin("testexec", []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestExecutePlugin_NonZeroExit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin that exits with error code
	pluginPath := filepath.Join(tmpDir, "ds-failplugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	script := "#!/bin/sh\nexit 42\n"
	if err := os.WriteFile(pluginPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test plugin: %v", err)
	}

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-failplugin.yaml")
	manifest := `name: failplugin
version: 1.0.0
platform:
  os: [linux, darwin, windows]
  arch: [amd64, arm64]
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	exitCode, err := executor.ExecutePlugin("failplugin", []string{})

	if err != nil {
		t.Errorf("unexpected error (should be nil): %v", err)
	}

	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
}

func TestExecutePluginWithOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin that outputs to stdout and stderr
	pluginPath := filepath.Join(tmpDir, "ds-outputplugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	script := "#!/bin/sh\necho 'stdout message'\necho 'stderr message' >&2\nexit 0\n"
	if err := os.WriteFile(pluginPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test plugin: %v", err)
	}

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-outputplugin.yaml")
	manifest := `name: outputplugin
version: 1.0.0
platform:
  os: [linux, darwin, windows]
  arch: [amd64, arm64]
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	stdout, stderr, exitCode, err := executor.ExecutePluginWithOutput("outputplugin", []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout, "stdout message") {
		t.Errorf("expected 'stdout message' in stdout, got: %s", stdout)
	}

	if !strings.Contains(stderr, "stderr message") {
		t.Errorf("expected 'stderr message' in stderr, got: %s", stderr)
	}
}

func TestStreamPlugin(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin
	pluginPath := filepath.Join(tmpDir, "ds-streamplugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	script := "#!/bin/sh\necho 'streaming output'\n"
	if err := os.WriteFile(pluginPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test plugin: %v", err)
	}

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-streamplugin.yaml")
	manifest := `name: streamplugin
version: 1.0.0
platform:
  os: [linux, darwin, windows]
  arch: [amd64, arm64]
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	// Capture output
	var stdout, stderr bytes.Buffer

	exitCode, err := executor.StreamPlugin("streamplugin", []string{}, &stdout, &stderr)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "streaming output") {
		t.Errorf("expected 'streaming output' in stdout, got: %s", stdout.String())
	}
}

func TestExecutePlugin_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a plugin that sleeps
	pluginPath := filepath.Join(tmpDir, "ds-slowplugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	script := "#!/bin/sh\nsleep 10\n"
	if err := os.WriteFile(pluginPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test plugin: %v", err)
	}

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-slowplugin.yaml")
	manifest := `name: slowplugin
version: 1.0.0
platform:
  os: [linux, darwin, windows]
  arch: [amd64, arm64]
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)
	executor.SetTimeout(1 * time.Second) // Short timeout

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	exitCode, err := executor.ExecutePlugin("slowplugin", []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should return timeout exit code
	if exitCode != 124 {
		t.Errorf("expected timeout exit code 124, got %d", exitCode)
	}
}
