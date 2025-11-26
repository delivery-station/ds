package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/delivery-station/ds/internal/config"
	"github.com/delivery-station/ds/internal/plugin"
	"github.com/delivery-station/ds/internal/registry"
	"github.com/delivery-station/ds/pkg/log"
	"github.com/spf13/cobra"
)

var pluginInstallCmd = &cobra.Command{
	Use:   "install <name>[@version]",
	Short: "Install a plugin from the registry",
	Long: `Install a plugin from the OCI registry.

Examples:
  # Install latest version
  ds plugin install porter

  # Install specific version
  ds plugin install porter@1.0.0

  # Install from specific registry
  ds plugin install ghcr.io/myorg/myplugin:latest`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInstall,
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a plugin to the latest version",
	Long: `Update an installed plugin to the latest version available in the registry.

Examples:
  # Update porter to latest
  ds plugin update porter

  # Update specific plugin
  ds plugin update courier`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginUpdate,
}

var pluginRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"uninstall", "delete"},
	Short:   "Remove an installed plugin",
	Long: `Remove a plugin from the plugin directory.

Examples:
  # Remove porter plugin
  ds plugin remove porter

  # Remove with confirmation
  ds plugin remove --force courier`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginRemove,
}

var (
	removeForce bool
)

func init() {
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginUpdateCmd)
	pluginCmd.AddCommand(pluginRemoveCmd)

	pluginRemoveCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Skip confirmation prompt")
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	ref := args[0]

	// Parse reference (name[@version])
	name, version := parsePluginReference(ref)

	log.Info("Installing plugin", "name", name)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set up authentication
	authProvider := registry.NewAuthProvider()
	if err := authProvider.LoadDockerConfig(); err != nil {
		log.Warn("Failed to load Docker config", "error", err)
	}

	// Create registry client
	registryURL := cfg.Registry.Default
	if registryURL == "" {
		registryURL = "ghcr.io"
	}

	client, err := registry.NewClient(registryURL, authProvider)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	// Create installer
	installer := plugin.NewInstaller(cfg.Plugins.Dir, registryURL, client)

	// Set up signature verification
	verifier, err := plugin.NewSignatureVerifier(&cfg.Plugins.Signature, log.L())
	if err != nil {
		return fmt.Errorf("failed to create signature verifier: %w", err)
	}
	installer.SetSignatureVerifier(verifier)

	// Install plugin
	ctx := context.Background()
	if err := installer.InstallPlugin(ctx, name, version); err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	fmt.Printf("✓ Successfully installed %s", name)
	if version != "" && version != "latest" {
		fmt.Printf("@%s", version)
	}
	fmt.Println()

	return nil
}

func runPluginUpdate(cmd *cobra.Command, args []string) error {
	name := args[0]

	log.Info("Updating plugin", "name", name)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set up authentication
	authProvider := registry.NewAuthProvider()
	if err := authProvider.LoadDockerConfig(); err != nil {
		log.Warn("Failed to load Docker config", "error", err)
	}

	// Create registry client
	registryURL := cfg.Registry.Default
	if registryURL == "" {
		registryURL = "ghcr.io"
	}

	client, err := registry.NewClient(registryURL, authProvider)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	// Create installer
	installer := plugin.NewInstaller(cfg.Plugins.Dir, registryURL, client)

	// Set up signature verification
	verifier, err := plugin.NewSignatureVerifier(&cfg.Plugins.Signature, log.L())
	if err != nil {
		return fmt.Errorf("failed to create signature verifier: %w", err)
	}
	installer.SetSignatureVerifier(verifier)

	// Update plugin
	ctx := context.Background()
	if err := installer.UpdatePlugin(ctx, name); err != nil {
		return fmt.Errorf("failed to update plugin: %w", err)
	}

	fmt.Printf("✓ Successfully updated %s\n", name)

	return nil
}

func runPluginRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Confirmation prompt
	if !removeForce {
		fmt.Printf("Remove plugin '%s'? [y/N] ", name)
		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	log.Info("Removing plugin", "name", name)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set up authentication (not really needed for remove, but for consistency)
	authProvider := registry.NewAuthProvider()

	// Create registry client (dummy for remove operation)
	registryURL := cfg.Registry.Default
	if registryURL == "" {
		registryURL = "ghcr.io"
	}

	client, err := registry.NewClient(registryURL, authProvider)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	// Create installer
	installer := plugin.NewInstaller(cfg.Plugins.Dir, registryURL, client)

	// Remove plugin
	ctx := context.Background()
	if err := installer.RemovePlugin(ctx, name); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	fmt.Printf("✓ Successfully removed %s\n", name)

	return nil
}

// parsePluginReference parses a plugin reference into name and version
func parsePluginReference(ref string) (name, version string) {
	// Check for @version syntax
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		return ref[:idx], ref[idx+1:]
	}

	// Check for :version syntax
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		return ref[:idx], ref[idx+1:]
	}

	// No version specified
	return ref, "latest"
}
