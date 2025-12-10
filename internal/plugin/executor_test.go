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

func copyExecutable(t *testing.T, dst, src string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		t.Fatalf("failed to write file %s: %v", dst, err)
	}
	if err := os.Chmod(dst, mode); err != nil {
		t.Fatalf("failed to set permissions on %s: %v", dst, err)
	}
}

func TestNewExecutor(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr, nil)

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
	executor := NewExecutor(mgr, nil)

	newTimeout := 10 * time.Second
	executor.SetTimeout(newTimeout)

	if executor.timeout != newTimeout {
		t.Errorf("expected timeout %v, got %v", newTimeout, executor.timeout)
	}
}

func TestExecutePlugin_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	executor := NewExecutor(mgr, nil)

	exitCode, err := executor.ExecutePlugin("nonexistent", "test", nil)

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

func (p *TestPlugin) Execute(ctx context.Context, operation string, args []string) (*types.ExecutionResult, error) {
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

func (p *TestPlugin) GetManifest(ctx context.Context) (*types.PluginManifest, error) {
	return &types.PluginManifest{
		Name:    "testexec",
		Version: "1.0.0",
		Platform: types.PluginPlatform{
			OS:   []string{"linux", "darwin", "windows"},
			Arch: []string{"amd64", "arm64"},
		},
		Commands: []types.PluginCommand{
			{Name: "run", Description: "Run command"},
		},
	}, nil
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
	executor := NewExecutor(mgr, nil)

	// Discover plugins
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	// Execute plugin
	exitCode, err := executor.ExecutePlugin("testexec", "run", nil)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestExecutePlugin_FinalizerRunsOnSuccess(t *testing.T) {
	pluginDir := t.TempDir()

	finalizerDir := t.TempDir()
	finalizerSource := `package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	goplugin "github.com/hashicorp/go-plugin"
)

type FinalizerPlugin struct{}

func (p *FinalizerPlugin) Execute(ctx context.Context, operation string, args []string) (*types.ExecutionResult, error) {
	if operation != "upload" {
		return &types.ExecutionResult{ExitCode: 1, Error: fmt.Sprintf("unexpected operation: %s", operation)}, nil
	}
	if len(args) != 0 {
		return &types.ExecutionResult{ExitCode: 1, Error: "unexpected arguments"}, nil
	}
	dest := os.Getenv("FINALIZER_OUTPUT")
	if dest == "" {
		return &types.ExecutionResult{ExitCode: 1, Error: "FINALIZER_OUTPUT not set"}, nil
	}
	if err := os.WriteFile(dest, []byte("finalized"), 0600); err != nil {
		return &types.ExecutionResult{ExitCode: 1, Error: err.Error()}, nil
	}
	return &types.ExecutionResult{Stdout: "finalizer complete", ExitCode: 0}, nil
}

func (p *FinalizerPlugin) ValidateConfig(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (p *FinalizerPlugin) GetSchema(ctx context.Context) (*types.PluginSchema, error) {
	return &types.PluginSchema{}, nil
}

func (p *FinalizerPlugin) GetManifest(ctx context.Context) (*types.PluginManifest, error) {
	return &types.PluginManifest{
		Name:    "finalizer",
		Version: "1.0.0",
		Platform: types.PluginPlatform{
			OS:   []string{runtime.GOOS},
			Arch: []string{runtime.GOARCH},
		},
		Commands: []types.PluginCommand{
			{Name: "upload"},
		},
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ds-plugin": &plugin.DSPlugin{Impl: &FinalizerPlugin{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
`
	finalizerBinary := buildTestPlugin(t, finalizerDir, "finalizer", finalizerSource)
	copyExecutable(t, filepath.Join(pluginDir, filepath.Base(finalizerBinary)), finalizerBinary, 0755)
	finalizerManifest := fmt.Sprintf(`{"name":"finalizer","version":"1.0.0","platform":{"os":["%s"],"arch":["%s"]},"commands":[{"name":"upload"}]}`, runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(filepath.Join(pluginDir, "ds-finalizer.json"), []byte(finalizerManifest), 0644); err != nil {
		t.Fatalf("failed to write finalizer manifest: %v", err)
	}

	primaryDir := t.TempDir()
	primarySource := `package main

import (
	"context"
	"runtime"

	"github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	goplugin "github.com/hashicorp/go-plugin"
)

type PrimaryPlugin struct{}

func (p *PrimaryPlugin) Execute(ctx context.Context, operation string, args []string) (*types.ExecutionResult, error) {
	if operation != "run" {
		return &types.ExecutionResult{ExitCode: 1, Error: "unexpected operation"}, nil
	}
	return &types.ExecutionResult{Stdout: "primary complete", ExitCode: 0}, nil
}

func (p *PrimaryPlugin) ValidateConfig(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (p *PrimaryPlugin) GetSchema(ctx context.Context) (*types.PluginSchema, error) {
	return &types.PluginSchema{}, nil
}

func (p *PrimaryPlugin) GetManifest(ctx context.Context) (*types.PluginManifest, error) {
	return &types.PluginManifest{
		Name:    "primary",
		Version: "1.0.0",
		Platform: types.PluginPlatform{
			OS:   []string{runtime.GOOS},
			Arch: []string{runtime.GOARCH},
		},
		Commands: []types.PluginCommand{
			{Name: "run"},
		},
		Annotations: map[string]string{
			"finalizer": "finalizer",
		},
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ds-plugin": &plugin.DSPlugin{Impl: &PrimaryPlugin{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
`
	primaryBinary := buildTestPlugin(t, primaryDir, "primary", primarySource)
	copyExecutable(t, filepath.Join(pluginDir, filepath.Base(primaryBinary)), primaryBinary, 0755)
	primaryManifest := fmt.Sprintf(`{"name":"primary","version":"1.0.0","annotations":{"finalizer":"finalizer"},"platform":{"os":["%s"],"arch":["%s"]},"commands":[{"name":"run"}]}`, runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(filepath.Join(pluginDir, "ds-primary.json"), []byte(primaryManifest), 0644); err != nil {
		t.Fatalf("failed to write primary manifest: %v", err)
	}

	mgr := NewManager(pluginDir)
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	executor := NewExecutor(mgr, nil)
	outputFile := filepath.Join(t.TempDir(), "finalizer.txt")
	if err := os.Setenv("FINALIZER_OUTPUT", outputFile); err != nil {
		t.Fatalf("failed to set environment: %v", err)
	}
	defer func() {
		_ = os.Unsetenv("FINALIZER_OUTPUT")
	}()

	exitCode, err := executor.ExecutePlugin("primary", "run", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("expected finalizer to write output file: %v", err)
	}
	if !strings.Contains(string(data), "finalized") {
		t.Fatalf("unexpected finalizer output: %s", string(data))
	}
}

func TestExecutePlugin_FinalizerMissingIsOptional(t *testing.T) {
	pluginDir := t.TempDir()

	primaryDir := t.TempDir()
	primarySource := `package main

import (
	"context"
	"runtime"

	"github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	goplugin "github.com/hashicorp/go-plugin"
)

type PrimaryPlugin struct{}

func (p *PrimaryPlugin) Execute(ctx context.Context, operation string, args []string) (*types.ExecutionResult, error) {
	if operation != "run" {
		return &types.ExecutionResult{ExitCode: 1, Error: "unexpected operation"}, nil
	}
	return &types.ExecutionResult{Stdout: "primary complete", ExitCode: 0}, nil
}

func (p *PrimaryPlugin) ValidateConfig(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (p *PrimaryPlugin) GetSchema(ctx context.Context) (*types.PluginSchema, error) {
	return &types.PluginSchema{}, nil
}

func (p *PrimaryPlugin) GetManifest(ctx context.Context) (*types.PluginManifest, error) {
	return &types.PluginManifest{
		Name:    "primary",
		Version: "1.0.0",
		Platform: types.PluginPlatform{
			OS:   []string{runtime.GOOS},
			Arch: []string{runtime.GOARCH},
		},
		Commands: []types.PluginCommand{
			{Name: "run"},
		},
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ds-plugin": &plugin.DSPlugin{Impl: &PrimaryPlugin{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
`
	primaryBinary := buildTestPlugin(t, primaryDir, "primary", primarySource)
	copyExecutable(t, filepath.Join(pluginDir, filepath.Base(primaryBinary)), primaryBinary, 0755)
	primaryManifest := fmt.Sprintf(`{"name":"primary","version":"1.0.0","annotations":{"finalizer":"missing-finalizer"},"platform":{"os":["%s"],"arch":["%s"]},"commands":[{"name":"run"}]}`, runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(filepath.Join(pluginDir, "ds-primary.json"), []byte(primaryManifest), 0644); err != nil {
		t.Fatalf("failed to write primary manifest: %v", err)
	}

	mgr := NewManager(pluginDir)
	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	executor := NewExecutor(mgr, nil)
	exitCode, err := executor.ExecutePlugin("primary", "run", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
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

func (p *TestPlugin) Execute(ctx context.Context, operation string, args []string) (*types.ExecutionResult, error) {
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

func (p *TestPlugin) GetManifest(ctx context.Context) (*types.PluginManifest, error) {
	return &types.PluginManifest{
		Name:    "failplugin",
		Version: "1.0.0",
		Commands: []types.PluginCommand{{Name: "run"}},
		Platform: types.PluginPlatform{
			OS:   []string{"linux", "darwin", "windows"},
			Arch: []string{"amd64", "arm64"},
		},
	}, nil
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
	executor := NewExecutor(mgr, nil)

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	exitCode, err := executor.ExecutePlugin("failplugin", "run", nil)

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

func (p *TestPlugin) Execute(ctx context.Context, operation string, args []string) (*types.ExecutionResult, error) {
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

func (p *TestPlugin) GetManifest(ctx context.Context) (*types.PluginManifest, error) {
	return &types.PluginManifest{
		Name:    "outputplugin",
		Version: "1.0.0",
		Commands: []types.PluginCommand{{Name: "run"}},
		Platform: types.PluginPlatform{
			OS:   []string{"linux", "darwin", "windows"},
			Arch: []string{"amd64", "arm64"},
		},
	}, nil
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
	executor := NewExecutor(mgr, nil)

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	stdout, stderr, exitCode, err := executor.ExecutePluginWithOutput("outputplugin", "run", nil)

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

func (p *TestPlugin) Execute(ctx context.Context, operation string, args []string) (*types.ExecutionResult, error) {
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

func (p *TestPlugin) GetManifest(ctx context.Context) (*types.PluginManifest, error) {
	return &types.PluginManifest{
		Name:    "slowplugin",
		Version: "1.0.0",
		Commands: []types.PluginCommand{{Name: "run"}},
		Platform: types.PluginPlatform{
			OS:   []string{"linux", "darwin", "windows"},
			Arch: []string{"amd64", "arm64"},
		},
	}, nil
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
	executor := NewExecutor(mgr, nil)
	executor.SetTimeout(1 * time.Second) // Short timeout

	if err := mgr.DiscoverPlugins(); err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	exitCode, err := executor.ExecutePlugin("slowplugin", "run", nil)

	// We expect a timeout error
	if err == nil {
		t.Error("expected timeout error, got nil")
	}

	// Should return timeout exit code
	if exitCode != 124 {
		t.Errorf("expected timeout exit code 124, got %d", exitCode)
	}
}
