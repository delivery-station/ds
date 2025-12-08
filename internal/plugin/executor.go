package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/delivery-station/ds/pkg/log"

	pkgplugin "github.com/delivery-station/ds/pkg/plugin"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// Executor handles plugin execution
type Executor struct {
	manager *Manager
	config  *types.Config
	timeout time.Duration
}

// NewExecutor creates a new plugin executor
func NewExecutor(manager *Manager, cfg *types.Config) *Executor {
	return &Executor{
		manager: manager,
		config:  cfg,
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
	p, err := e.manager.GetPlugin(pluginName)
	if err != nil {
		return 1, fmt.Errorf("plugin '%s' not found. Use 'ds plugin list' to see available plugins", pluginName)
	}

	// Validate plugin
	if err := e.manager.ValidatePlugin(pluginName); err != nil {
		return 1, fmt.Errorf("plugin '%s' is invalid: %w", pluginName, err)
	}

	log.Debug("Executing plugin", "name", pluginName, "args", args)

	// Create client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: pkgplugin.Handshake,
		Plugins:         pkgplugin.PluginMap,
		Cmd:             exec.Command(p.Path),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
		Logger: hclog.New(&hclog.LoggerOptions{
			Name:   "ds-plugin",
			Output: os.Stderr,
			Level:  hclog.Error,
		}),
	})
	defer client.Kill()

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		return 1, fmt.Errorf("failed to connect to plugin: %w", err)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("ds-plugin")
	if err != nil {
		return 1, fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Cast to our interface
	dsPlugin, ok := raw.(types.PluginProtocol)
	if !ok {
		return 1, fmt.Errorf("plugin does not implement PluginProtocol")
	}

	// Prepare environment
	envMap := make(map[string]string)
	env := e.PrepareEnvironment(pluginName)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Execute
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	if e.config != nil {
		ctx = types.WithHostConfigPayload(ctx, e.config)
	}
	defer cancel()

	// The first arg is the operation (e.g. "fetch"), the rest are args
	if len(args) == 0 {
		return 1, fmt.Errorf("no operation specified")
	}
	operation := args[0]
	opArgs := args[1:]

	result, err := dsPlugin.Execute(ctx, operation, opArgs, envMap)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 124, fmt.Errorf("plugin execution timed out")
		}
		return 1, fmt.Errorf("plugin execution failed: %w", err)
	}

	// Print output
	if result.Stdout != "" {
		fmt.Println(result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	if result.Error != "" {
		return result.ExitCode, fmt.Errorf("%s", result.Error)
	}

	return result.ExitCode, nil
}

// PrepareEnvironment prepares environment variables for plugin execution
func (e *Executor) PrepareEnvironment(pluginName string) []string {
	// Start with current environment
	env := append([]string{}, os.Environ()...)

	log.Debug("Prepared environment variables for plugin", "plugin", pluginName, "count", len(env))

	return env
}

// HandleError interprets command execution errors and returns appropriate exit code
func (e *Executor) HandleError(err error, ctx context.Context, pluginName string) int {
	if err == nil {
		return 0
	}

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		log.Error("Plugin timed out", "plugin", pluginName, "timeout", e.timeout)
		return 124 // Standard timeout exit code
	}

	// Check for exit error
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		log.Debug("Plugin exited", "plugin", pluginName, "code", exitCode)
		return exitCode
	}

	// Other errors
	log.Error("Failed to execute plugin", "plugin", pluginName, "error", err)
	return 1
}

// ExecutePluginWithOutput executes a plugin and captures its output
func (e *Executor) ExecutePluginWithOutput(pluginName string, args []string) (string, string, int, error) {
	// Get plugin info
	p, err := e.manager.GetPlugin(pluginName)
	if err != nil {
		return "", "", 1, fmt.Errorf("plugin '%s' not found", pluginName)
	}

	// Validate plugin
	if err := e.manager.ValidatePlugin(pluginName); err != nil {
		return "", "", 1, fmt.Errorf("plugin '%s' is invalid: %w", pluginName, err)
	}

	log.Debug("Executing plugin with output capture", "plugin", pluginName, "args", args)

	// Create client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: pkgplugin.Handshake,
		Plugins:         pkgplugin.PluginMap,
		Cmd:             exec.Command(p.Path),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
		Logger: hclog.New(&hclog.LoggerOptions{
			Name:   "ds-plugin",
			Output: os.Stderr,
			Level:  hclog.Error,
		}),
	})
	defer client.Kill()

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to connect to plugin: %w", err)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("ds-plugin")
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Cast to our interface
	dsPlugin, ok := raw.(types.PluginProtocol)
	if !ok {
		return "", "", 1, fmt.Errorf("plugin does not implement PluginProtocol")
	}

	// Prepare environment
	envMap := make(map[string]string)
	env := e.PrepareEnvironment(pluginName)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Execute
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	if e.config != nil {
		ctx = types.WithHostConfigPayload(ctx, e.config)
	}
	defer cancel()

	// The first arg is the operation (e.g. "fetch"), the rest are args
	if len(args) == 0 {
		return "", "", 1, fmt.Errorf("no operation specified")
	}
	operation := args[0]
	opArgs := args[1:]

	result, err := dsPlugin.Execute(ctx, operation, opArgs, envMap)
	if err != nil {
		return "", "", 1, fmt.Errorf("plugin execution failed: %w", err)
	}

	return result.Stdout, result.Stderr, result.ExitCode, nil
}
