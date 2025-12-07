package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// buildTestPlugin creates a real executable plugin for testing
func buildTestPlugin(t *testing.T, dir, name, sourceCode string) string {
	t.Helper()

	pluginName := "ds-" + name
	if runtime.GOOS == "windows" {
		pluginName += ".exe"
	}
	pluginPath := filepath.Join(dir, pluginName)

	// Create a temporary Go source file
	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte(sourceCode), 0644); err != nil {
		t.Fatalf("failed to write plugin source: %v", err)
	}

	// Initialize module
	cmdInit := exec.Command("go", "mod", "init", "testplugin")
	cmdInit.Dir = dir
	if out, err := cmdInit.CombinedOutput(); err != nil {
		t.Fatalf("failed to init module: %v\nOutput: %s", err, out)
	}

	// Get absolute path to ds module
	// Assuming we are running tests from ds/internal/plugin
	dsPath, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("failed to get ds path: %v", err)
	}

	// Replace ds module
	cmdReplace := exec.Command("go", "mod", "edit", "-replace", fmt.Sprintf("github.com/delivery-station/ds=%s", dsPath))
	cmdReplace.Dir = dir
	if out, err := cmdReplace.CombinedOutput(); err != nil {
		t.Fatalf("failed to replace module: %v\nOutput: %s", err, out)
	}

	// Tidy to resolve dependencies
	cmdTidy := exec.Command("go", "mod", "tidy")
	cmdTidy.Dir = dir
	if out, err := cmdTidy.CombinedOutput(); err != nil {
		t.Fatalf("failed to tidy module: %v\nOutput: %s", err, out)
	}

	// Build the plugin
	cmd := exec.Command("go", "build", "-o", pluginPath, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GOOS=%s", runtime.GOOS),
		fmt.Sprintf("GOARCH=%s", runtime.GOARCH),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test plugin: %v\nOutput: %s", err, output)
	}

	return pluginPath
}

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
	sourceCode := `package main

import (
	"context"
	"github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	goplugin "github.com/hashicorp/go-plugin"
)

type TestPlugin struct{}

func (p *TestPlugin) GetMetadata(ctx context.Context) (*types.PluginMetadata, error) {
	return &types.PluginMetadata{
		Name:    "testexec",
		Version: "1.0.0",
	}, nil
}

func (p *TestPlugin) Execute(ctx context.Context, operation string, args []string, env map[string]string) (*types.ExecutionResult, error) {
	return &types.ExecutionResult{
		Stdout:   "Hello from plugin\n",
		ExitCode: 0,
	}, nil
}

func (p *TestPlugin) ValidateConfig(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (p *TestPlugin) GetSchema(ctx context.Context) (*types.PluginSchema, error) {
	return &types.PluginSchema{}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ds-plugin": &plugin.DSPlugin{Impl: &TestPlugin{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
`
	buildTestPlugin(t, tmpDir, "testexec", sourceCode)

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-testexec.json")
	manifest := `{"name":"testexec","version":"1.0.0","platform":{"os":["linux","darwin","windows"],"arch":["amd64","arm64"]},"commands":[{"name":"run","description":"Run command"}]}`
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
	exitCode, err := executor.ExecutePlugin("testexec", []string{"run"})

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
	sourceCode := `package main

import (
	"context"
	"github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	goplugin "github.com/hashicorp/go-plugin"
)

type TestPlugin struct{}

func (p *TestPlugin) GetMetadata(ctx context.Context) (*types.PluginMetadata, error) {
	return &types.PluginMetadata{
		Name:    "failplugin",
		Version: "1.0.0",
	}, nil
}

func (p *TestPlugin) Execute(ctx context.Context, operation string, args []string, env map[string]string) (*types.ExecutionResult, error) {
	return &types.ExecutionResult{
		ExitCode: 42,
	}, nil
}

func (p *TestPlugin) ValidateConfig(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (p *TestPlugin) GetSchema(ctx context.Context) (*types.PluginSchema, error) {
	return &types.PluginSchema{}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ds-plugin": &plugin.DSPlugin{Impl: &TestPlugin{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
`
	buildTestPlugin(t, tmpDir, "failplugin", sourceCode)

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-failplugin.json")
	manifest := `{"name":"failplugin","version":"1.0.0","platform":{"os":["linux","darwin","windows"],"arch":["amd64","arm64"]},"commands":[{"name":"run"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	exitCode, err := executor.ExecutePlugin("failplugin", []string{"run"})

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
	sourceCode := `package main

import (
	"context"
	"github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	goplugin "github.com/hashicorp/go-plugin"
)

type TestPlugin struct{}

func (p *TestPlugin) GetMetadata(ctx context.Context) (*types.PluginMetadata, error) {
	return &types.PluginMetadata{
		Name:    "outputplugin",
		Version: "1.0.0",
	}, nil
}

func (p *TestPlugin) Execute(ctx context.Context, operation string, args []string, env map[string]string) (*types.ExecutionResult, error) {
	return &types.ExecutionResult{
		Stdout:   "stdout message",
		Stderr:   "stderr message",
		ExitCode: 0,
	}, nil
}

func (p *TestPlugin) ValidateConfig(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (p *TestPlugin) GetSchema(ctx context.Context) (*types.PluginSchema, error) {
	return &types.PluginSchema{}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ds-plugin": &plugin.DSPlugin{Impl: &TestPlugin{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
`
	buildTestPlugin(t, tmpDir, "outputplugin", sourceCode)

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-outputplugin.json")
	manifest := `{"name":"outputplugin","version":"1.0.0","platform":{"os":["linux","darwin","windows"],"arch":["amd64","arm64"]},"commands":[{"name":"run"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	stdout, stderr, exitCode, err := executor.ExecutePluginWithOutput("outputplugin", []string{"run"})

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

func TestExecutePlugin_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a plugin that sleeps
	sourceCode := `package main

import (
	"context"
	"time"
	"github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	goplugin "github.com/hashicorp/go-plugin"
)

type TestPlugin struct{}

func (p *TestPlugin) GetMetadata(ctx context.Context) (*types.PluginMetadata, error) {
	return &types.PluginMetadata{
		Name:    "slowplugin",
		Version: "1.0.0",
	}, nil
}

func (p *TestPlugin) Execute(ctx context.Context, operation string, args []string, env map[string]string) (*types.ExecutionResult, error) {
	time.Sleep(10 * time.Second)
	return &types.ExecutionResult{
		ExitCode: 0,
	}, nil
}

func (p *TestPlugin) ValidateConfig(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (p *TestPlugin) GetSchema(ctx context.Context) (*types.PluginSchema, error) {
	return &types.PluginSchema{}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ds-plugin": &plugin.DSPlugin{Impl: &TestPlugin{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
`
	buildTestPlugin(t, tmpDir, "slowplugin", sourceCode)

	// Create manifest
	manifestPath := filepath.Join(tmpDir, "ds-slowplugin.json")
	manifest := `{"name":"slowplugin","version":"1.0.0","platform":{"os":["linux","darwin","windows"],"arch":["amd64","arm64"]},"commands":[{"name":"run"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr)
	executor.SetTimeout(1 * time.Second) // Short timeout

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	exitCode, err := executor.ExecutePlugin("slowplugin", []string{"run"})

	// We expect a timeout error
	if err == nil {
		t.Error("expected timeout error, got nil")
	}

	// Should return timeout exit code
	if exitCode != 124 {
		t.Errorf("expected timeout exit code 124, got %d", exitCode)
	}
}
