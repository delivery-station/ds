package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/delivery-station/ds/internal/config"
	"github.com/delivery-station/ds/internal/plugin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile   string
	logLevel  string
	pluginDir string
	noColor   bool

	version string
	commit  string
	date    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ds",
	Short: "Delivery Station - Plugin-based CLI for managing and delivering OCI artifacts",
	Long: `DS (Delivery Station) is a plugin-based CLI meta-application that manages 
and delivers OCI artifacts. It discovers, loads, and manages plugins similar 
to how Terraform manages providers, delegating work to plugins while providing 
common configuration.

Examples:
  ds plugin list                          # List installed plugins
  ds plugin install porter                # Install porter plugin
  ds porter fetch ghcr.io/org/app:v1.0.0 # Use porter to fetch an artifact`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
	// Handle unknown commands by delegating to plugins
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	err := rootCmd.Execute()

	// If command not found, try to delegate to plugin
	if err != nil {
		errStr := err.Error()

		if strings.Contains(errStr, "unknown command") {
			args := os.Args[1:]
			if len(args) > 0 {
				// Find the first non-flag argument (the command/plugin name)
				var pluginName string
				var pluginArgs []string

				skipNext := false
				for i, arg := range args {
					if skipNext {
						skipNext = false
						continue
					}

					if strings.HasPrefix(arg, "--") {
						// Long flag - may have value after = or as next arg
						if !strings.Contains(arg, "=") {
							skipNext = true // Skip next arg (the value)
						}
						continue
					}

					if strings.HasPrefix(arg, "-") && arg != "-" {
						// Short flag - skip next arg as value
						skipNext = true
						continue
					}

					// Found the command name
					pluginName = arg
					pluginArgs = args[i+1:]
					break
				}

				if pluginName != "" {
					// Try to execute as plugin
					exitCode, pluginErr := executePlugin(pluginName, pluginArgs)
					if pluginErr != nil {
						// Plugin not found or failed - return original error
						return err
					}

					// Exit with plugin's exit code
					os.Exit(exitCode)
				}
			}
		}
	}

	return err
}

// executePlugin attempts to execute a plugin
func executePlugin(pluginName string, args []string) (int, error) {
	// Initialize config manually (since PersistentPreRunE won't run for unknown commands)
	if err := initConfig(); err != nil {
		// Continue anyway with defaults
	}

	// Manually check for --plugin-dir flag in os.Args
	pluginDir := viper.GetString("plugins.dir")
	for i, arg := range os.Args {
		if arg == "--plugin-dir" && i+1 < len(os.Args) {
			pluginDir = os.Args[i+1]
			break
		}
		if strings.HasPrefix(arg, "--plugin-dir=") {
			pluginDir = strings.TrimPrefix(arg, "--plugin-dir=")
			break
		}
	}

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return 1, err
	}

	// Override plugin dir if specified
	if pluginDir != "" {
		cfg.Plugins.Dir = pluginDir
	}

	// Create plugin manager
	mgr := plugin.NewManager(cfg.Plugins.Dir)

	// Create executor
	executor := plugin.NewExecutor(mgr)

	// Execute plugin
	exitCode, err := executor.ExecutePlugin(pluginName, args)
	return exitCode, err
}

// SetVersionInfo sets version information from build time variables
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/ds/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&pluginDir, "plugin-dir", "", "plugin directory (default: ~/.config/ds/plugins)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ds version %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		},
	})

	// Bind flags to viper
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	viper.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("plugins.dir", rootCmd.PersistentFlags().Lookup("plugin-dir"))
	viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() error {
	// Setup logging
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	if noColor {
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableColors: true,
		})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			ForceColors:   true,
			FullTimestamp: true,
		})
	}

	// Set config file
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		// Search for config in standard locations
		viper.AddConfigPath(filepath.Join(home, ".config", "ds"))
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Environment variables
	viper.SetEnvPrefix("DS")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		// Only return error if config file was explicitly specified
		if cfgFile != "" {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		// Otherwise, just log debug message
		logrus.Debugf("No config file found: %v", err)
	} else {
		logrus.Debugf("Using config file: %s", viper.ConfigFileUsed())
	}

	// Set defaults
	setDefaults()

	return nil
}

func setDefaults() {
	home, _ := os.UserHomeDir()

	// Registry defaults
	viper.SetDefault("registry.default", "ghcr.io")
	viper.SetDefault("registry.insecure_registries", []string{})

	// Cache defaults
	viper.SetDefault("cache.dir", filepath.Join(home, ".cache", "ds"))
	viper.SetDefault("cache.max_size", "10GB")
	viper.SetDefault("cache.ttl", "7d")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.output", "stdout")

	// Plugin defaults
	viper.SetDefault("plugins.dir", filepath.Join(home, ".config", "ds", "plugins"))
	viper.SetDefault("plugins.auto_install", true)

	// Auth defaults
	viper.SetDefault("auth.docker_config", filepath.Join(home, ".docker", "config.json"))
}
