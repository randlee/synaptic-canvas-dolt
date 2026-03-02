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
	DoltDir string
	Remote  string
	JSON    bool
	Quiet   bool
	Verbose bool
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

	return &Config{
		DoltDir: doltDir,
		Remote:  remote,
		JSON:    jsonMode,
		Quiet:   quiet,
		Verbose: verbose,
	}, nil
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
	if c.DoltDir == "" {
		return ""
	}
	if strings.HasPrefix(c.DoltDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return c.DoltDir
		}
		return filepath.Join(home, c.DoltDir[2:])
	}
	return c.DoltDir
}
