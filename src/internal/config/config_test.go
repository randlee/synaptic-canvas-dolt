package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestCmd returns a cobra.Command wired with the same persistent flags as
// the real root command.
func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	pf := cmd.PersistentFlags()
	pf.String("dolt-dir", "", "Dolt database directory (default: auto-detect)")
	pf.String("remote", "", "DoltHub remote name")
	pf.Bool("json", false, "output as JSON")
	pf.Bool("quiet", false, "suppress non-essential output")
	pf.Bool("verbose", false, "enable debug logging")
	return cmd
}

func TestNewConfigFromFlags(t *testing.T) {
	t.Parallel()

	cmd := newTestCmd()
	cmd.SetArgs([]string{
		"--dolt-dir", "/tmp/dolt",
		"--remote", "origin",
		"--json",
		"--verbose",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	cfg, err := NewConfigFromFlags(cmd)
	if err != nil {
		t.Fatalf("NewConfigFromFlags failed: %v", err)
	}

	if cfg.DoltDir != "/tmp/dolt" {
		t.Errorf("DoltDir = %q, want %q", cfg.DoltDir, "/tmp/dolt")
	}
	if cfg.Remote != "origin" {
		t.Errorf("Remote = %q, want %q", cfg.Remote, "origin")
	}
	if !cfg.JSON {
		t.Error("JSON should be true")
	}
	if !cfg.Verbose {
		t.Error("Verbose should be true")
	}
	if cfg.Quiet {
		t.Error("Quiet should be false")
	}
}

func TestValidateConflictingFlags(t *testing.T) {
	t.Parallel()

	cfg := &Config{Verbose: true, Quiet: true}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for conflicting --verbose and --quiet")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateNoConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		verbose bool
		quiet   bool
	}{
		{"default", false, false},
		{"verbose only", true, false},
		{"quiet only", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{Verbose: tt.verbose, Quiet: tt.quiet}
			if err := cfg.Validate(); err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestDoltDirExpanded(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home directory: %v", err)
	}

	tests := []struct {
		name    string
		doltDir string
		want    string
	}{
		{
			name:    "empty string auto-detect",
			doltDir: "",
			want:    "",
		},
		{
			name:    "tilde expansion",
			doltDir: "~/.sc/dolt",
			want:    filepath.Join(home, ".sc", "dolt"),
		},
		{
			name:    "absolute path unchanged",
			doltDir: "/var/data/dolt",
			want:    "/var/data/dolt",
		},
		{
			name:    "relative path unchanged",
			doltDir: "data/dolt",
			want:    "data/dolt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{DoltDir: tt.doltDir}
			got := cfg.DoltDirExpanded()
			if got != tt.want {
				t.Errorf("DoltDirExpanded() = %q, want %q", got, tt.want)
			}
		})
	}
}
