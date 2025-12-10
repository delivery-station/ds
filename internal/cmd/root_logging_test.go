package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/delivery-station/ds/pkg/types"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/viper"
)

func TestResolveLogLevel(t *testing.T) {
	cfg := &types.Config{}
	cfg.Logging.Level = "debug"

	lvl, err := resolveLogLevel(cfg)
	if err != nil {
		t.Fatalf("resolveLogLevel returned error: %v", err)
	}

	if lvl != hclog.Debug {
		t.Fatalf("expected debug level, got %s", lvl)
	}
}

func TestResolveLogLevelInvalid(t *testing.T) {
	cfg := &types.Config{}
	cfg.Logging.Level = "invalid"

	if _, err := resolveLogLevel(cfg); err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestResolveLogOutputStdout(t *testing.T) {
	writer, err := resolveLogOutput("stdout")
	if err != nil {
		t.Fatalf("resolveLogOutput returned error: %v", err)
	}

	if writer != nil && writer != os.Stdout {
		t.Fatalf("expected stdout writer, got %T", writer)
	}
}

func TestResolveLogOutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs", "ds.log")

	writer, err := resolveLogOutput(logPath)
	if err != nil {
		t.Fatalf("resolveLogOutput returned error: %v", err)
	}

	file, ok := writer.(*os.File)
	if !ok {
		t.Fatalf("expected *os.File writer, got %T", writer)
	}
	t.Cleanup(func() {
		_ = file.Close()
	})

	if file.Name() != logPath {
		t.Fatalf("expected file path %s, got %s", logPath, file.Name())
	}
}

func TestApplyLogLevelOverrideUsesFlag(t *testing.T) {
	originalLogLevel := logLevel
	originalViperLevel := viper.GetString("logging.level")
	originalOverride := viper.GetString("logging.level_override")

	t.Cleanup(func() {
		logLevel = originalLogLevel
		viper.Set("logging.level", originalViperLevel)
		viper.Set("logging.level_override", originalOverride)
	})

	logLevel = "warn"
	viper.Set("logging.level_override", "")

	cfg := &types.Config{}
	cfg.Logging.Level = "debug"

	applyLogLevelOverride(cfg)

	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected override to set warn level, got %s", cfg.Logging.Level)
	}

	if viper.GetString("logging.level") != "warn" {
		t.Fatalf("expected viper logging.level to be warn, got %s", viper.GetString("logging.level"))
	}
}

func TestApplyLogLevelOverrideDefaults(t *testing.T) {
	originalLogLevel := logLevel
	originalViperLevel := viper.GetString("logging.level")
	originalOverride := viper.GetString("logging.level_override")

	t.Cleanup(func() {
		logLevel = originalLogLevel
		viper.Set("logging.level", originalViperLevel)
		viper.Set("logging.level_override", originalOverride)
	})

	logLevel = ""
	viper.Set("logging.level_override", "")

	cfg := &types.Config{}
	cfg.Logging.Level = ""

	applyLogLevelOverride(cfg)

	if cfg.Logging.Level != "info" {
		t.Fatalf("expected default log level info, got %s", cfg.Logging.Level)
	}
}
