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
	pf.String("dolt-client", "", "Dolt client to use: http, sql, or cli")
	pf.String("dolt-host", "", "DoltHub HTTP API host")
	pf.String("dolt-database", "", "DoltHub database slug in owner/database format")
	pf.String("dolt-token", "", "DoltHub API token")
	pf.String("dolt-dsn", "", "Dolt SQL server DSN")
	pf.String("dolt-dir", "", "Dolt database directory (default: auto-detect)")
	pf.String("dolt-timeout", "", "Dolt HTTP timeout in seconds")
	pf.String("remote", "", "DoltHub remote name")
	pf.String("branch", "", "Branch override (default: SC_DOLT_BRANCH or main)")
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
		"--branch", "develop",
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
	if cfg.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", cfg.Branch, "develop")
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

func TestEffectiveBranch(t *testing.T) {
	t.Setenv("SC_DOLT_BRANCH", "")
	if got := (&Config{Branch: "beta"}).EffectiveBranch(); got != "beta" {
		t.Fatalf("EffectiveBranch(flag) = %q", got)
	}

	t.Setenv("SC_DOLT_BRANCH", "develop")
	if got := (&Config{}).EffectiveBranch(); got != "develop" {
		t.Fatalf("EffectiveBranch(env) = %q", got)
	}

	t.Setenv("SC_DOLT_BRANCH", "")
	if got := (&Config{}).EffectiveBranch(); got != "main" {
		t.Fatalf("EffectiveBranch(default) = %q", got)
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

func TestFileConfigGetPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("SC_DOLT_DIR", "env-dir")

	if _, err := SetFileValue(KeyDoltDir, "file-dir"); err != nil {
		t.Fatalf("SetFileValue() error = %v", err)
	}

	cmd := newTestCmd()
	cmd.SetArgs([]string{"--dolt-dir", "flag-dir"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}
	cfg, err := NewConfigFromFlags(cmd)
	if err != nil {
		t.Fatalf("NewConfigFromFlags() error = %v", err)
	}
	if err := cfg.LoadFileConfig(); err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	if got := cfg.Get(KeyDoltDir, "default-dir"); got != "flag-dir" {
		t.Fatalf("Get(flag precedence) = %q, want flag-dir", got)
	}

	cmd = newTestCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}
	cfg, err = NewConfigFromFlags(cmd)
	if err != nil {
		t.Fatalf("NewConfigFromFlags() error = %v", err)
	}
	if err := cfg.LoadFileConfig(); err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	if got := cfg.Get(KeyDoltDir, "default-dir"); got != "env-dir" {
		t.Fatalf("Get(env precedence) = %q, want env-dir", got)
	}

	t.Setenv("SC_DOLT_DIR", "")
	if got := cfg.Get(KeyDoltDir, "default-dir"); got != "file-dir" {
		t.Fatalf("Get(file precedence) = %q, want file-dir", got)
	}
	if got := cfg.Get(KeyDoltDatabase, "default-db"); got != "default-db" {
		t.Fatalf("Get(default precedence) = %q, want default-db", got)
	}
}

func TestAllDoltConfigFlagsOverrideEnvAndFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	tests := []struct {
		key   string
		flag  string
		value string
	}{
		{key: KeyDoltClient, flag: "--dolt-client", value: "sql"},
		{key: KeyDoltHost, flag: "--dolt-host", value: "example.invalid"},
		{key: KeyDoltDatabase, flag: "--dolt-database", value: "flag/db"},
		{key: KeyDoltToken, flag: "--dolt-token", value: "flag-token"},
		{key: KeyDoltDSN, flag: "--dolt-dsn", value: "root@tcp(127.0.0.1:3306)/db"},
		{key: KeyDoltDir, flag: "--dolt-dir", value: "flag-dir"},
		{key: KeyDoltTimeout, flag: "--dolt-timeout", value: "7"},
	}
	args := []string{}
	for _, tt := range tests {
		if _, err := SetFileValue(tt.key, "file-value"); err != nil {
			t.Fatalf("SetFileValue(%s) error = %v", tt.key, err)
		}
		t.Setenv(EnvNameForKey(tt.key), "env-value")
		args = append(args, tt.flag, tt.value)
	}

	cmd := newTestCmd()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}
	cfg, err := NewConfigFromFlags(cmd)
	if err != nil {
		t.Fatalf("NewConfigFromFlags() error = %v", err)
	}
	if err := cfg.LoadFileConfig(); err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	for _, tt := range tests {
		if got := cfg.Get(tt.key, "default"); got != tt.value {
			t.Fatalf("Get(%s) = %q, want %q", tt.key, got, tt.value)
		}
	}
	if got := cfg.GetInt(KeyDoltTimeout, 30); got != 7 {
		t.Fatalf("GetInt(timeout) = %d, want 7", got)
	}
}

func TestSetFileValueWritesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	path, err := SetFileValue(KeyDoltDatabase, "randlee/synaptic-canvas")
	if err != nil {
		t.Fatalf("SetFileValue() error = %v", err)
	}
	if path != filepath.Join(home, ".sc", "config.toml") {
		t.Fatalf("path = %q, want ~/.sc/config.toml", path)
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: test reads from t.TempDir()-based path, not user input.
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `database = 'randlee/synaptic-canvas'`) &&
		!strings.Contains(string(data), `database = "randlee/synaptic-canvas"`) {
		t.Fatalf("config file missing database value:\n%s", string(data))
	}
}
