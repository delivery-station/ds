package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/delivery-station/ds/internal/config"
	"github.com/delivery-station/ds/internal/plugin"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	pluginOutputFormat string
)

// pluginCmd represents the plugin command
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage DS plugins",
	Long: `Manage DS plugins including listing installed plugins,
installing new plugins, and updating existing ones.`,
}

// pluginListCmd lists installed plugins
var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	Long: `List all installed plugins with their versions and descriptions.
Plugins are discovered from the plugin directory configured in DS settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		loader := config.NewLoader()
		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create plugin manager
		mgr := plugin.NewManager(cfg.Plugins.Dir)

		// Discover plugins
		plugins, err := mgr.ListPlugins()
		if err != nil {
			return fmt.Errorf("failed to list plugins: %w", err)
		}

		if len(plugins) == 0 {
			fmt.Println("No plugins installed.")
			fmt.Printf("\nPlugin directory: %s\n", cfg.Plugins.Dir)
			fmt.Println("\nTo install a plugin, place the plugin binary (ds-<name>) in the plugin directory.")
			return nil
		}

		// Display plugins
		switch pluginOutputFormat {
		case "table":
			displayPluginsTable(plugins)
		case "simple":
			displayPluginsSimple(plugins)
		default:
			return fmt.Errorf("unsupported format: %s (use 'table' or 'simple')", pluginOutputFormat)
		}

		fmt.Printf("\nTotal: %d plugin(s)\n", len(plugins))
		fmt.Printf("Plugin directory: %s\n", cfg.Plugins.Dir)

		return nil
	},
}

// pluginInfoCmd shows detailed information about a specific plugin
var pluginInfoCmd = &cobra.Command{
	Use:   "info <plugin-name>",
	Short: "Show detailed plugin information",
	Long:  `Display detailed information about a specific plugin including version, commands, and platform support.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pluginName := args[0]

		// Load configuration
		loader := config.NewLoader()
		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create plugin manager
		mgr := plugin.NewManager(cfg.Plugins.Dir)

		// Get plugin info
		pluginInfo, err := mgr.GetPlugin(pluginName)
		if err != nil {
			return fmt.Errorf("failed to get plugin info: %w", err)
		}

		// Display plugin information
		fmt.Printf("Plugin: %s\n", pluginInfo.Name)
		fmt.Printf("Version: %s\n", pluginInfo.Version)
		fmt.Printf("Path: %s\n", pluginInfo.Path)

		if pluginInfo.Description != "" {
			fmt.Printf("Description: %s\n", pluginInfo.Description)
		}

		if pluginInfo.Manifest != nil {
			manifest := pluginInfo.Manifest

			if len(manifest.Commands) > 0 {
				fmt.Println("\nCommands:")
				for _, cmd := range manifest.Commands {
					fmt.Printf("  - %s", cmd.Name)
					if cmd.Description != "" {
						fmt.Printf(": %s", cmd.Description)
					}
					fmt.Println()
				}
			}

			if len(manifest.Platform.OS) > 0 || len(manifest.Platform.Arch) > 0 {
				fmt.Println("\nPlatform Support:")
				if len(manifest.Platform.OS) > 0 {
					fmt.Printf("  OS: %v\n", manifest.Platform.OS)
				}
				if len(manifest.Platform.Arch) > 0 {
					fmt.Printf("  Arch: %v\n", manifest.Platform.Arch)
				}
			}
		}

		// Validate plugin
		if err := mgr.ValidatePlugin(pluginName); err != nil {
			fmt.Printf("\n⚠️  Warning: %v\n", err)
		} else {
			fmt.Println("\n✅ Plugin is valid and ready to use")
		}

		return nil
	},
}

func displayPluginsTable(plugins []*types.PluginInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer func() {
		if err := w.Flush(); err != nil {
			logrus.WithError(err).Warn("Failed to flush output")
		}
	}()

	_, _ = fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION")
	_, _ = fmt.Fprintln(w, "----\t-------\t-----------")

	for _, p := range plugins {
		description := p.Description
		if description == "" {
			description = "-"
		}
		// Truncate long descriptions
		if len(description) > 50 {
			description = description[:47] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Version, description)
	}
}

func displayPluginsSimple(plugins []*types.PluginInfo) {
	for _, p := range plugins {
		fmt.Printf("- %s (%s)", p.Name, p.Version)
		if p.Description != "" {
			fmt.Printf(": %s", p.Description)
		}
		fmt.Println()
	}
}

func init() {
	// Add plugin command to root
	rootCmd.AddCommand(pluginCmd)

	// Add subcommands
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInfoCmd)

	// Flags for list command
	pluginListCmd.Flags().StringVarP(&pluginOutputFormat, "format", "f", "table", "Output format (table or simple)")
}
