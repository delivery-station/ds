package cmd

import (
	"strings"
	"testing"
)

func TestRootCommand(t *testing.T) {
	// Test that root command initializes
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil")
	}

	if rootCmd.Use != "ds" {
		t.Errorf("expected Use to be 'ds', got '%s'", rootCmd.Use)
	}
}

func TestSetVersionInfo(t *testing.T) {
	testVersion := "1.0.0"
	testCommit := "abc123"
	testDate := "2025-01-01"

	SetVersionInfo(testVersion, testCommit, testDate)

	if version != testVersion {
		t.Errorf("expected version %s, got %s", testVersion, version)
	}
	if commit != testCommit {
		t.Errorf("expected commit %s, got %s", testCommit, commit)
	}
	if date != testDate {
		t.Errorf("expected date %s, got %s", testDate, date)
	}
}

func TestNormalizePluginArgs(t *testing.T) {
	args := []string{"example/ref:1.0.0", "--insecure", "--output", "./out"}
	got := normalizePluginArgs(args)

	if len(got) != 3 {
		t.Fatalf("expected 3 normalized args, got %d: %#v", len(got), got)
	}

	expects := map[string]string{
		"arg0":     "example/ref:1.0.0",
		"insecure": "true",
		"output":   "./out",
	}

	for _, pair := range got {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("expected key=value format, got %q", pair)
		}
		key, val := parts[0], parts[1]
		expected, ok := expects[key]
		if !ok {
			t.Fatalf("unexpected key %q in normalized args", key)
		}
		if expected != val {
			t.Fatalf("expected %q=%q, got %q=%q", key, expected, key, val)
		}
		delete(expects, key)
	}

	if len(expects) != 0 {
		t.Fatalf("missing keys after normalization: %#v", expects)
	}
}

func TestParsePluginInvocationPreservesPluginFlags(t *testing.T) {
	originalCfg := cfgFile
	defer func() { cfgFile = originalCfg }()

	args := []string{
		"--config", "/tmp/custom-config.yaml",
		"porter", "pull",
		"example/ref:1.0.0",
		"--output", "./out/dir",
		"--insecure",
	}

	pluginName, operation, pluginArgs, err := parsePluginInvocation(args)
	if err != nil {
		t.Fatalf("parsePluginInvocation returned error: %v", err)
	}

	if pluginName != "porter" {
		t.Fatalf("expected plugin name porter, got %s", pluginName)
	}
	if operation != "pull" {
		t.Fatalf("expected operation pull, got %s", operation)
	}

	expects := map[string]string{
		"arg0":     "example/ref:1.0.0",
		"output":   "./out/dir",
		"insecure": "true",
	}

	if len(pluginArgs) != len(expects) {
		t.Fatalf("expected %d plugin args, got %d: %#v", len(expects), len(pluginArgs), pluginArgs)
	}

	for _, pair := range pluginArgs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("expected key=value format, got %q", pair)
		}
		key, val := parts[0], parts[1]

		expected, ok := expects[key]
		if !ok {
			t.Fatalf("unexpected plugin arg %q", key)
		}
		if val != expected {
			t.Fatalf("expected %s=%s, got %s=%s", key, expected, key, val)
		}
		delete(expects, key)
	}

	if len(expects) != 0 {
		t.Fatalf("missing plugin args: %#v", expects)
	}

	if cfgFile != "/tmp/custom-config.yaml" {
		t.Fatalf("expected cfgFile to be set from persistent flag, got %s", cfgFile)
	}
}
