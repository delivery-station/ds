package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/delivery-station/ds/internal/config"
	"github.com/delivery-station/ds/internal/plugin"
	"github.com/delivery-station/ds/pkg/log"
	"github.com/delivery-station/ds/pkg/types"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

		if strings.Contains(errStr, "unknown command") || strings.Contains(errStr, "unknown flag") {
			pluginName, operation, pluginArgs, parseErr := parsePluginInvocation(os.Args[1:])
			if parseErr != nil {
				return parseErr
			}

			if pluginName != "" {
				// Try to execute as plugin
				exitCode, pluginErr := executePlugin(pluginName, operation, pluginArgs)
				if pluginErr != nil {
					// If plugin was not found, return the original "unknown command" error
					// We check for the specific error message returned by the executor
					if strings.Contains(pluginErr.Error(), fmt.Sprintf("plugin '%s' not found", pluginName)) {
						return err
					}
					// Otherwise, return the plugin execution error
					return pluginErr
				}

				// Exit with plugin's exit code
				os.Exit(exitCode)
			}
		}
	}

	return err
}

// parsePluginInvocation extracts the plugin name and arguments while ensuring
// that persistent flags are parsed consistently with Cobra/Viper handling.
func parsePluginInvocation(args []string) (string, string, []string, error) {
	if len(args) == 0 {
		return "", "", nil, nil
	}

	flagSet := pflag.NewFlagSet("ds", pflag.ContinueOnError)
	flagSet.ParseErrorsAllowlist.UnknownFlags = true
	// Using the same flag definitions keeps CLI/Viper behaviour consistent.
	flagSet.AddFlagSet(rootCmd.PersistentFlags())
	// Route parse diagnostics through the same channel Cobra uses for flag errors.
	flagSet.SetOutput(os.Stderr)

	// Parse to populate persistent flags (e.g. --config) but preserve plugin flags.
	// pflag drops unknown flags when UnknownFlags is allowed, so we rebuild the
	// plugin argument list manually.
	_ = flagSet.Parse(args)

	knownFlags := map[string]*pflag.Flag{}
	flagSet.VisitAll(func(f *pflag.Flag) {
		knownFlags[f.Name] = f
		if f.Shorthand != "" {
			knownFlags[f.Shorthand] = f
		}
	})

	var remaining []string
	skipNext := false
	for i := 0; i < len(args); i++ {
		if skipNext {
			skipNext = false
			continue
		}

		token := strings.TrimSpace(args[i])
		if token == "" {
			continue
		}
		if token == "--" {
			remaining = append(remaining, args[i:]...)
			break
		}

		if !looksLikeFlag(token) {
			remaining = append(remaining, token)
			continue
		}

		name, hasValue := parseFlagToken(token)
		flag, isKnown := knownFlags[name]
		if isKnown {
			if !hasValue && flag.Value.Type() != "bool" && i+1 < len(args) && !looksLikeFlag(args[i+1]) {
				skipNext = true
			}
			continue
		}

		remaining = append(remaining, token)
	}

	if len(remaining) == 0 {
		return "", "", nil, nil
	}

	pluginName := remaining[0]
	if len(remaining) < 2 {
		return pluginName, "", nil, fmt.Errorf("plugin '%s' requires a command", pluginName)
	}

	normalizedArgs := normalizePluginArgs(remaining[2:])
	return pluginName, remaining[1], normalizedArgs, nil
}

func normalizePluginArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(args))
	positionalIndex := 0

	for i := 0; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		if token == "" {
			continue
		}

		if token == "--" {
			for j := i + 1; j < len(args); j++ {
				value := strings.TrimSpace(args[j])
				if value == "" {
					continue
				}
				normalized = append(normalized, fmt.Sprintf("arg%d=%s", positionalIndex, value))
				positionalIndex++
			}
			break
		}

		if strings.HasPrefix(token, "--") {
			key, value, advance := parseLongFlag(token, args, i)
			if key != "" {
				normalized = append(normalized, fmt.Sprintf("%s=%s", key, value))
			}
			i += advance
			continue
		}

		if strings.HasPrefix(token, "-") && token != "-" {
			keyValues, advance := parseShortFlag(token, args, i)
			normalized = append(normalized, keyValues...)
			i += advance
			continue
		}

		normalized = append(normalized, fmt.Sprintf("arg%d=%s", positionalIndex, token))
		positionalIndex++
	}

	return normalized
}

func parseLongFlag(token string, args []string, index int) (string, string, int) {
	trimmed := strings.TrimPrefix(token, "--")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", "", 0
	}

	if eq := strings.Index(trimmed, "="); eq >= 0 {
		key := sanitizeArgKey(trimmed[:eq])
		value := strings.TrimSpace(trimmed[eq+1:])
		return key, value, 0
	}

	key := sanitizeArgKey(trimmed)
	if key == "" {
		return "", "", 0
	}

	if next := index + 1; next < len(args) {
		candidate := strings.TrimSpace(args[next])
		if candidate != "" && !looksLikeFlag(candidate) {
			return key, candidate, 1
		}
		if looksLikeNegativeNumber(candidate) {
			return key, candidate, 1
		}
	}

	return key, "true", 0
}

func parseShortFlag(token string, args []string, index int) ([]string, int) {
	trimmed := strings.TrimPrefix(token, "-")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil, 0
	}

	// Handle inline assignment (-o=value)
	if strings.Contains(trimmed, "=") {
		parts := strings.SplitN(trimmed, "=", 2)
		key := sanitizeArgKey(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, 0
		}
		return []string{fmt.Sprintf("%s=%s", key, value)}, 0
	}

	if len(trimmed) > 1 {
		// Treat clustered short flags (-abc) as booleans
		values := make([]string, 0, len(trimmed))
		for _, r := range trimmed {
			key := sanitizeArgKey(string(r))
			if key == "" {
				continue
			}
			values = append(values, fmt.Sprintf("%s=true", key))
		}
		return values, 0
	}

	key := sanitizeArgKey(trimmed)
	if key == "" {
		return nil, 0
	}

	if next := index + 1; next < len(args) {
		candidate := strings.TrimSpace(args[next])
		if candidate != "" && !looksLikeFlag(candidate) {
			return []string{fmt.Sprintf("%s=%s", key, candidate)}, 1
		}
		if looksLikeNegativeNumber(candidate) {
			return []string{fmt.Sprintf("%s=%s", key, candidate)}, 1
		}
	}

	return []string{fmt.Sprintf("%s=true", key)}, 0
}

func parseFlagToken(token string) (name string, hasValue bool) {
	if !strings.HasPrefix(token, "-") {
		return "", false
	}

	trimmed := strings.TrimLeft(token, "-")
	if trimmed == "" {
		return "", false
	}

	if eq := strings.Index(trimmed, "="); eq >= 0 {
		return sanitizeArgKey(trimmed[:eq]), true
	}

	return sanitizeArgKey(trimmed), false
}

func sanitizeArgKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "-")
	key = strings.TrimPrefix(key, "-")
	key = strings.TrimSpace(key)
	key = strings.ToLower(key)
	return key
}

func looksLikeFlag(token string) bool {
	if token == "" || token == "-" {
		return false
	}
	if !strings.HasPrefix(token, "-") {
		return false
	}
	if token == "--" {
		return true
	}
	if looksLikeNegativeNumber(token) {
		return false
	}
	return true
}

func looksLikeNegativeNumber(token string) bool {
	if token == "" {
		return false
	}
	if token == "-" {
		return false
	}
	if !strings.HasPrefix(token, "-") {
		return false
	}
	if _, err := strconv.ParseFloat(token, 64); err == nil {
		return true
	}
	return false
}

// executePlugin attempts to execute a plugin
func executePlugin(pluginName, operation string, args []string) (int, error) {
	// Initialize config manually (since PersistentPreRunE won't run for unknown commands)
	if err := initConfig(); err != nil {
		// Continue anyway with defaults - config errors are non-fatal for plugin execution
		log.Debug("Failed to initialize config, using defaults", "error", err)
	}

	pluginDir := viper.GetString("plugins.dir")

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
	if err := mgr.DiscoverPlugins(); err != nil {
		return 1, fmt.Errorf("failed to discover plugins: %w", err)
	}

	// Create executor
	executor := plugin.NewExecutor(mgr, cfg)

	// Execute plugin
	exitCode, err := executor.ExecutePlugin(pluginName, operation, args)
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
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level override (debug, info, warn, error)")
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
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("logging.level_override", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("plugins.dir", rootCmd.PersistentFlags().Lookup("plugin-dir"))
	_ = viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() error {
	config.SetExplicitConfigFile(cfgFile)

	viper.SetEnvPrefix("DS")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	applyLogLevelOverride(cfg)

	lvl, err := resolveLogLevel(cfg)
	if err != nil {
		return err
	}

	outputWriter, err := resolveLogOutput(cfg.Logging.Output)
	if err != nil {
		return fmt.Errorf("failed to configure logging output: %w", err)
	}

	opts := &hclog.LoggerOptions{
		Name:       "ds",
		Output:     outputWriter,
		Level:      lvl,
		Color:      hclog.AutoColor,
		JSONFormat: strings.EqualFold(cfg.Logging.Format, "json"),
	}

	if noColor {
		opts.Color = hclog.ColorOff
	}

	log.SetLogger(hclog.New(opts))

	if cfgFile != "" {
		log.Debug("Configured explicit config override", "file", cfgFile)
	}

	return nil
}

func applyLogLevelOverride(cfg *types.Config) {
	if cfg == nil {
		return
	}

	override := strings.TrimSpace(logLevel)
	if override == "" {
		override = strings.TrimSpace(viper.GetString("logging.level_override"))
	}

	if override != "" {
		override = strings.ToLower(override)
		cfg.Logging.Level = override
	}

	if strings.TrimSpace(cfg.Logging.Level) == "" {
		cfg.Logging.Level = "info"
	}

	viper.Set("logging.level", cfg.Logging.Level)
}

func resolveLogLevel(cfg *types.Config) (hclog.Level, error) {
	if cfg == nil {
		return hclog.NoLevel, fmt.Errorf("missing configuration")
	}

	level := strings.TrimSpace(cfg.Logging.Level)
	if level == "" {
		level = "info"
	}

	lvl := hclog.LevelFromString(strings.ToLower(level))
	if lvl == hclog.NoLevel {
		return hclog.NoLevel, fmt.Errorf("invalid log level: %s", level)
	}

	return lvl, nil
}

func resolveLogOutput(output string) (io.Writer, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" || strings.EqualFold(trimmed, "stdout") {
		return os.Stdout, nil
	}
	if strings.EqualFold(trimmed, "stderr") {
		return os.Stderr, nil
	}

	dir := filepath.Dir(trimmed)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	file, err := os.OpenFile(trimmed, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return file, nil
}
