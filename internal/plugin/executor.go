package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Executor handles plugin execution
type Executor struct {
	manager *Manager
	timeout time.Duration
}

// NewExecutor creates a new plugin executor
func NewExecutor(manager *Manager) *Executor {
	return &Executor{
		manager: manager,
		timeout: 5 * time.Minute, // Default 5 minute timeout
	}
}

// SetTimeout sets the execution timeout
func (e *Executor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// ExecutePlugin executes a plugin with the given arguments
func (e *Executor) ExecutePlugin(pluginName string, args []string) (int, error) {
	// Get plugin info
	plugin, err := e.manager.GetPlugin(pluginName)
	if err != nil {
		return 1, fmt.Errorf("plugin '%s' not found. Use 'ds plugin list' to see available plugins", pluginName)
	}

	// Validate plugin
	if err := e.manager.ValidatePlugin(pluginName); err != nil {
		return 1, fmt.Errorf("plugin '%s' is invalid: %w", pluginName, err)
	}

	logrus.Debugf("Executing plugin: %s %v", pluginName, args)

	// Prepare environment
	env := e.PrepareEnvironment()

	// Create command with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, plugin.Path, args...)
	cmd.Env = env

	// Setup I/O streaming
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Execute plugin
	err = cmd.Run()

	// Handle errors
	exitCode := e.HandleError(err, ctx, pluginName)

	return exitCode, nil
}

// PrepareEnvironment prepares environment variables for plugin execution
func (e *Executor) PrepareEnvironment() []string {
	// Start with current environment
	env := os.Environ()

	// Add all DS_* variables from viper config
	allSettings := viper.AllSettings()
	for key, value := range allSettings {
		envKey := "DS_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		envValue := fmt.Sprintf("%v", value)
		env = append(env, fmt.Sprintf("%s=%s", envKey, envValue))
	}

	// Add specific DS variables
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		env = append(env, fmt.Sprintf("DS_CONFIG=%s", configFile))
	}

	// Add cache directory
	if cacheDir := viper.GetString("cache.dir"); cacheDir != "" {
		env = append(env, fmt.Sprintf("DS_CACHE_DIR=%s", cacheDir))
	}

	// Add plugin directory
	if pluginDir := viper.GetString("plugins.dir"); pluginDir != "" {
		env = append(env, fmt.Sprintf("DS_PLUGIN_DIR=%s", pluginDir))
	}

	// Add registry default
	if registry := viper.GetString("registry.default"); registry != "" {
		env = append(env, fmt.Sprintf("DS_REGISTRY_DEFAULT=%s", registry))
	}

	logrus.Debugf("Prepared %d environment variables for plugin", len(env))

	return env
}

// HandleError interprets command execution errors and returns appropriate exit code
func (e *Executor) HandleError(err error, ctx context.Context, pluginName string) int {
	if err == nil {
		return 0
	}

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		logrus.Errorf("Plugin '%s' timed out after %v", pluginName, e.timeout)
		return 124 // Standard timeout exit code
	}

	// Check for exit error
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		logrus.Debugf("Plugin '%s' exited with code %d", pluginName, exitCode)
		return exitCode
	}

	// Other errors
	logrus.Errorf("Failed to execute plugin '%s': %v", pluginName, err)
	return 1
}

// ExecutePluginWithOutput executes a plugin and captures its output
func (e *Executor) ExecutePluginWithOutput(pluginName string, args []string) (string, string, int, error) {
	// Get plugin info
	plugin, err := e.manager.GetPlugin(pluginName)
	if err != nil {
		return "", "", 1, fmt.Errorf("plugin '%s' not found", pluginName)
	}

	// Validate plugin
	if err := e.manager.ValidatePlugin(pluginName); err != nil {
		return "", "", 1, fmt.Errorf("plugin '%s' is invalid: %w", pluginName, err)
	}

	logrus.Debugf("Executing plugin with output capture: %s %v", pluginName, args)

	// Prepare environment
	env := e.PrepareEnvironment()

	// Create command with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, plugin.Path, args...)
	cmd.Env = env

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute plugin
	err = cmd.Run()

	// Handle errors
	exitCode := e.HandleError(err, ctx, pluginName)

	return stdout.String(), stderr.String(), exitCode, nil
}

// StreamPlugin executes a plugin with real-time output streaming
func (e *Executor) StreamPlugin(pluginName string, args []string, stdout, stderr io.Writer) (int, error) {
	// Get plugin info
	plugin, err := e.manager.GetPlugin(pluginName)
	if err != nil {
		return 1, fmt.Errorf("plugin '%s' not found", pluginName)
	}

	// Validate plugin
	if err := e.manager.ValidatePlugin(pluginName); err != nil {
		return 1, fmt.Errorf("plugin '%s' is invalid: %w", pluginName, err)
	}

	logrus.Debugf("Streaming plugin: %s %v", pluginName, args)

	// Prepare environment
	env := e.PrepareEnvironment()

	// Create command with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, plugin.Path, args...)
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

	// Execute plugin
	err = cmd.Run()

	// Handle errors
	exitCode := e.HandleError(err, ctx, pluginName)

	return exitCode, nil
}
