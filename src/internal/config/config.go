package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Config holds the global configuration derived from CLI flags.
type Config struct {
	DoltDir       string
	Remote        string
	Branch        string
	JSON          bool
	Quiet         bool
	Verbose       bool
	fileValues    map[string]string
	explicitFlags map[string]string
}

// NewConfigFromFlags extracts global flag values from the given cobra command.
func NewConfigFromFlags(cmd *cobra.Command) (*Config, error) {
	flags := cmd.Root().PersistentFlags()

	doltDir, err := flags.GetString("dolt-dir")
	if err != nil {
		return nil, fmt.Errorf("reading --dolt-dir: %w", err)
	}

	remote, err := flags.GetString("remote")
	if err != nil {
		return nil, fmt.Errorf("reading --remote: %w", err)
	}
	branch, err := flags.GetString("branch")
	if err != nil {
		return nil, fmt.Errorf("reading --branch: %w", err)
	}

	jsonMode, err := flags.GetBool("json")
	if err != nil {
		return nil, fmt.Errorf("reading --json: %w", err)
	}

	quiet, err := flags.GetBool("quiet")
	if err != nil {
		return nil, fmt.Errorf("reading --quiet: %w", err)
	}

	verbose, err := flags.GetBool("verbose")
	if err != nil {
		return nil, fmt.Errorf("reading --verbose: %w", err)
	}

	cfg := &Config{
		DoltDir:       doltDir,
		Remote:        remote,
		Branch:        branch,
		JSON:          jsonMode,
		Quiet:         quiet,
		Verbose:       verbose,
		fileValues:    map[string]string{},
		explicitFlags: map[string]string{},
	}
	if flags.Lookup("dolt-dir").Changed {
		cfg.explicitFlags[KeyDoltDir] = doltDir
	}
	return cfg, nil
}

// Validate checks the configuration for conflicting or invalid settings.
func (c *Config) Validate() error {
	if c.Verbose && c.Quiet {
		return fmt.Errorf("--verbose and --quiet cannot be used together")
	}
	return nil
}

// DoltDirExpanded returns the DoltDir path with the leading ~ expanded to the
// user's home directory. An empty string means auto-detect and is returned as-is.
func (c *Config) DoltDirExpanded() string {
	return ExpandPath(c.DoltDir)
}

// EffectiveBranch resolves the branch flag using flag, then environment, then
// the default main branch.
func (c *Config) EffectiveBranch() string {
	if c.Branch != "" {
		return c.Branch
	}
	if env := os.Getenv("SC_DOLT_BRANCH"); env != "" {
		return env
	}
	return "main"
}

// ExpandPath expands a leading ~/ in a filesystem path. Empty paths are
// returned unchanged.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
