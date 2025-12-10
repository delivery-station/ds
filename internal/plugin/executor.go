package plugin

import (
	"context"
	"encoding/json"
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
func (e *Executor) ExecutePlugin(pluginName, operation string, args []string) (int, error) {
	// Get plugin info
	p, err := e.manager.GetPlugin(pluginName)
	if err != nil {
		return 1, fmt.Errorf("plugin '%s' not found. Use 'ds plugin list' to see available plugins", pluginName)
	}

	// Validate plugin
	if err := e.manager.ValidatePlugin(pluginName); err != nil {
		return 1, fmt.Errorf("plugin '%s' is invalid: %w", pluginName, err)
	}

	log.Debug("Executing plugin", "name", pluginName, "operation", operation, "args", args)

	if strings.TrimSpace(operation) == "" {
		return 1, fmt.Errorf("no operation specified")
	}

	result, exitCode, err := e.runPlugin(pluginName, p, operation, args)
	if err != nil {
		return exitCode, err
	}

	if result.Stdout != "" {
		fmt.Println(result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	if result.Error != "" {
		return result.ExitCode, fmt.Errorf("%s", result.Error)
	}

	if exitCode == 0 {
		finalizers := e.collectFinalizers(pluginName, p, result)
		e.invokeFinalizers(finalizers)
	}

	return exitCode, nil
}

func (e *Executor) runPlugin(pluginName string, info *types.PluginInfo, operation string, args []string) (*types.ExecutionResult, int, error) {
	if strings.TrimSpace(operation) == "" {
		return nil, 1, fmt.Errorf("no operation specified")
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: pkgplugin.Handshake,
		Plugins:         pkgplugin.PluginMap,
		Cmd:             exec.Command(info.Path),
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

	rpcClient, err := client.Client()
	if err != nil {
		return nil, 1, fmt.Errorf("failed to connect to plugin: %w", err)
	}

	raw, err := rpcClient.Dispense("ds-plugin")
	if err != nil {
		return nil, 1, fmt.Errorf("failed to dispense plugin: %w", err)
	}

	dsPlugin, ok := raw.(types.PluginProtocol)
	if !ok {
		return nil, 1, fmt.Errorf("plugin does not implement PluginProtocol")
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	if e.config != nil {
		ctx = types.WithHostConfigPayload(ctx, e.config)
	}
	defer cancel()

	result, err := dsPlugin.Execute(ctx, operation, args)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, 124, fmt.Errorf("plugin execution timed out")
		}
		return nil, 1, fmt.Errorf("plugin execution failed: %w", err)
	}

	return result, result.ExitCode, nil
}

func (e *Executor) executeFinalizer(req types.FinalizerRequest) (int, error) {
	finalizerName := strings.TrimSpace(req.Name)
	if finalizerName == "" {
		return 1, fmt.Errorf("finalizer name cannot be empty")
	}

	operation := strings.TrimSpace(req.Operation)
	if operation == "" {
		operation = "upload"
	}

	log.Info("Executing finalizer plugin", "plugin", finalizerName, "operation", operation)

	finalizerInfo, err := e.manager.GetPlugin(finalizerName)
	if err != nil {
		return 1, fmt.Errorf("finalizer plugin '%s' not found: %w", finalizerName, err)
	}

	if err := e.manager.ValidatePlugin(finalizerName); err != nil {
		return 1, fmt.Errorf("finalizer plugin '%s' is invalid: %w", finalizerName, err)
	}

	normalizedArgs := normalizeFinalizerArgs(req.Args)
	result, exitCode, err := e.runPlugin(finalizerName, finalizerInfo, operation, normalizedArgs)
	if err != nil {
		return exitCode, fmt.Errorf("finalizer plugin '%s' failed: %w", finalizerName, err)
	}

	if result.Stdout != "" {
		fmt.Println(result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}
	if result.Error != "" {
		return result.ExitCode, fmt.Errorf("finalizer plugin '%s' reported error: %s", finalizerName, result.Error)
	}
	if result.ExitCode != 0 {
		return result.ExitCode, fmt.Errorf("finalizer plugin '%s' exited with status %d", finalizerName, result.ExitCode)
	}

	log.Info("Finalizer plugin completed", "plugin", finalizerName, "operation", operation)
	return 0, nil
}

func (e *Executor) invokeFinalizers(finalizers []types.FinalizerRequest) {
	for _, finalizer := range finalizers {
		exitCode, err := e.executeFinalizer(finalizer)
		if err != nil {
			log.Warn("Finalizer execution failed", "plugin", finalizer.Name, "operation", finalizer.Operation, "exit_code", exitCode, "error", err)
		}
	}
}

func normalizeFinalizerArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(args))
	positional := 0

	for _, raw := range args {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}

		if strings.Contains(token, "=") && !strings.HasPrefix(token, "=") {
			normalized = append(normalized, token)
			continue
		}

		normalized = append(normalized, fmt.Sprintf("arg%d=%s", positional, token))
		positional++
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

func (e *Executor) collectFinalizers(pluginName string, pluginInfo *types.PluginInfo, result *types.ExecutionResult) []types.FinalizerRequest {
	type key struct {
		name      string
		operation string
		args      string
	}

	seen := make(map[key]struct{})
	appendFinalizer := func(f types.FinalizerRequest, output *[]types.FinalizerRequest) {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			return
		}
		if strings.EqualFold(name, strings.TrimSpace(pluginName)) {
			return
		}

		operation := strings.TrimSpace(f.Operation)
		if operation == "" {
			operation = "upload"
		}

		argsCopy := append([]string{}, f.Args...)
		for i := range argsCopy {
			argsCopy[i] = strings.TrimSpace(argsCopy[i])
		}

		dedupKey := key{
			name:      strings.ToLower(name),
			operation: strings.ToLower(operation),
			args:      strings.Join(argsCopy, "\u0000"),
		}
		if _, ok := seen[dedupKey]; ok {
			return
		}
		seen[dedupKey] = struct{}{}

		*output = append(*output, types.FinalizerRequest{
			Name:      name,
			Operation: operation,
			Args:      argsCopy,
		})
	}

	finalizers := make([]types.FinalizerRequest, 0)

	if result != nil {
		for _, f := range result.Finalizers {
			appendFinalizer(f, &finalizers)
		}

		for _, f := range finalizersFromStdout(result.Stdout) {
			appendFinalizer(f, &finalizers)
		}
	}

	return finalizers
}

func finalizersFromStdout(stdout string) []types.FinalizerRequest {
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		return nil
	}

	var payload struct {
		Metadata map[string]string `json:"metadata"`
	}

	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}

	return finalizersFromMetadata(payload.Metadata)
}

func finalizersFromMetadata(metadata map[string]string) []types.FinalizerRequest {
	if len(metadata) == 0 {
		return nil
	}

	name := firstNonEmpty(metadata, "ds.finalizer", "finalizer")
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	operation := strings.TrimSpace(firstNonEmpty(metadata, "ds.finalizer.operation", "finalizer.operation"))
	args := parseFinalizerArgs(firstNonEmpty(metadata, "ds.finalizer.args", "finalizer.args"))

	return []types.FinalizerRequest{{
		Name:      name,
		Operation: operation,
		Args:      args,
	}}
}

func firstNonEmpty(metadata map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := metadata[k]; ok {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func parseFinalizerArgs(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if val := strings.TrimSpace(item); val != "" {
					result = append(result, val)
				}
			}
			if len(result) > 0 {
				return result
			}
			return nil
		}
	}

	parts := strings.Split(trimmed, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if val := strings.TrimSpace(part); val != "" {
			result = append(result, val)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
