package types

import (
	"context"
)

// PluginProtocol defines the RPC interface for DS plugins
// This interface will be implemented by all plugins and exposed via go-plugin
type PluginProtocol interface {
	// GetMetadata returns plugin metadata (name, version, description, etc.)
	GetMetadata(ctx context.Context) (*PluginMetadata, error)

	// Execute runs a plugin operation with given arguments
	// Returns output (stdout), error output (stderr), and exit code
	Execute(ctx context.Context, operation string, args []string, env map[string]string) (*ExecutionResult, error)

	// ValidateConfig validates plugin configuration
	ValidateConfig(ctx context.Context, config map[string]interface{}) error

	// GetSchema returns the configuration schema for this plugin
	GetSchema(ctx context.Context) (*PluginSchema, error)
}

// PluginManifestProvider can be implemented by plugins that expose a manifest over RPC.
type PluginManifestProvider interface {
	GetManifest(ctx context.Context) (*PluginManifest, error)
}

// PluginMetadata contains plugin information
type PluginMetadata struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Operations  []string          `json:"operations"`
	Platform    PluginPlatform    `json:"platform"`
	Config      map[string]string `json:"config,omitempty"`
}

// ExecutionResult contains the result of a plugin execution
type ExecutionResult struct {
	Stdout     string             `json:"stdout"`
	Stderr     string             `json:"stderr"`
	ExitCode   int                `json:"exit_code"`
	Error      string             `json:"error,omitempty"`
	Finalizers []FinalizerRequest `json:"finalizers,omitempty"`
}

// FinalizerRequest tells the DS host to invoke another plugin after a successful run.
type FinalizerRequest struct {
	Name      string   `json:"name"`
	Operation string   `json:"operation"`
	Args      []string `json:"args,omitempty"`
}

// PluginSchema describes the configuration schema
type PluginSchema struct {
	Version    string                    `json:"version"`
	Properties map[string]SchemaProperty `json:"properties"`
}

// SchemaProperty describes a configuration property
type SchemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}
