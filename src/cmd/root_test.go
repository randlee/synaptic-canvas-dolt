package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandExecutes(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test", "abc123", "2025-01-01")
	cmd.SetArgs([]string{})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root command should execute without error: %v", err)
	}
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("1.0.0", "abc123", "2025-01-01")
	cmd.SetArgs([]string{"--version"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("--version should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "sc version") {
		t.Errorf("version output should contain 'sc version', got: %s", output)
	}
	if !strings.Contains(output, "1.0.0") {
		t.Errorf("version output should contain version '1.0.0', got: %s", output)
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("version output should contain commit 'abc123', got: %s", output)
	}
	if !strings.Contains(output, "2025-01-01") {
		t.Errorf("version output should contain date '2025-01-01', got: %s", output)
	}
}

func TestHelpContainsExpectedText(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test", "abc123", "2025-01-01")

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Use Help() to get the full help text including description and flags.
	if err := cmd.Help(); err != nil {
		t.Fatalf("Help() should not error: %v", err)
	}

	output := buf.String()
	expectedPhrases := []string{
		"Synaptic Canvas",
		"package management system",
		"--dolt-dir",
		"--remote",
		"--json",
		"--quiet",
		"--verbose",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("help output should contain %q, got:\n%s", phrase, output)
		}
	}
}

func TestConflictingFlagsError(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test", "abc123", "2025-01-01")
	// Add a no-op run to trigger PersistentPreRunE.
	cmd.RunE = func(_ *cobra.Command, _ []string) error { return nil }
	cmd.SetArgs([]string{"--verbose", "--quiet"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --verbose and --quiet are both set")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("error should mention flag conflict, got: %v", err)
	}
}
