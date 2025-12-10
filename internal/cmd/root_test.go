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
