package cmd

import (
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
