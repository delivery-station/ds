package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/delivery-station/ds/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configOutputFormat string
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage DS configuration",
	Long: `Manage DS configuration including viewing effective configuration,
validating config files, and displaying configuration sources.`,
}

// configShowCmd shows the effective configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective configuration",
	Long: `Display the effective configuration after merging all sources
(defaults, config file, environment variables, and CLI flags).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		loader := config.NewLoader()
		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		var output []byte
		switch configOutputFormat {
		case "json":
			output, err = json.MarshalIndent(cfg, "", "  ")
		case "yaml":
			output, err = yaml.Marshal(cfg)
		default:
			return fmt.Errorf("unsupported output format: %s (use json or yaml)", configOutputFormat)
		}

		if err != nil {
			return fmt.Errorf("failed to format configuration: %w", err)
		}

		fmt.Println(string(output))
		return nil
	},
}

// configValidateCmd validates the configuration
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long: `Validate the configuration file and report any errors.
Checks for syntax errors, missing required fields, and invalid values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		loader := config.NewLoader()
		cfg, err := loader.Load()
		if err != nil {
			fmt.Println("❌ Configuration validation failed:")
			fmt.Printf("   %v\n", err)
			return err
		}

		fmt.Println("✅ Configuration is valid")
		fmt.Printf("   Registry: %s\n", cfg.Registry.Default)
		fmt.Printf("   Cache dir: %s\n", cfg.Cache.Dir)
		fmt.Printf("   Plugin dir: %s\n", cfg.Plugins.Dir)
		fmt.Printf("   Log level: %s\n", cfg.Logging.Level)

		return nil
	},
}

// configPathCmd shows the configuration file path
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	Long:  `Display the path to the configuration file being used.`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfgFile != "" {
			fmt.Printf("Configuration file: %s (explicitly set)\n", cfgFile)
		} else {
			defaultPath := config.GetConfigPath()
			fmt.Printf("Default configuration path: %s\n", defaultPath)
			fmt.Println("\nConfiguration search paths:")
			fmt.Println("  1. ./config.yaml (current directory)")
			fmt.Println("  2. " + defaultPath)
		}
	},
}

func init() {
	// Add config command to root
	rootCmd.AddCommand(configCmd)

	// Add subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configPathCmd)

	// Flags for show command
	configShowCmd.Flags().StringVarP(&configOutputFormat, "format", "f", "yaml", "Output format (yaml or json)")
}
