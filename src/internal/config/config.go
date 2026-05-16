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
	changedFlags  map[string]bool
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
		changedFlags:  map[string]bool{},
	}
	for _, spec := range doltFlagSpecs {
		var changed []string
		var explicitValue string
		for _, flagName := range spec.Flags {
			flag := flags.Lookup(flagName)
			if flag == nil {
				continue
			}
			if flag.Changed {
				cfg.changedFlags[flagName] = true
				value, err := flags.GetString(flagName)
				if err != nil {
					return nil, fmt.Errorf("reading --%s: %w", flagName, err)
				}
				if len(changed) > 0 && explicitValue != value {
					return nil, fmt.Errorf("conflicting values supplied for %s via %s", spec.Key, strings.Join(spec.Flags, " and "))
				}
				explicitValue = value
				changed = append(changed, flagName)
			}
		}
		if len(changed) > 0 {
			cfg.explicitFlags[spec.Key] = explicitValue
		}
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

type doltFlagSpec struct {
	Flags []string
	Key   string
}

var doltFlagSpecs = []doltFlagSpec{
	{Flags: []string{"client", "dolt-client"}, Key: KeyDoltClient},
	{Flags: []string{"dolt-host"}, Key: KeyDoltHost},
	{Flags: []string{"dolt-database"}, Key: KeyDoltDatabase},
	{Flags: []string{"dolt-token"}, Key: KeyDoltToken},
	{Flags: []string{"dolt-dsn"}, Key: KeyDoltDSN},
	{Flags: []string{"dolt-dir"}, Key: KeyDoltDir},
	{Flags: []string{"dolt-timeout"}, Key: KeyDoltTimeout},
}

type ValueSource string

const (
	ValueSourceExplicit      ValueSource = "explicit"
	ValueSourceEnvironment   ValueSource = "environment"
	ValueSourceFile          ValueSource = "file"
	ValueSourceCompatibility ValueSource = "compatibility"
	ValueSourceDefault       ValueSource = "default"
)

type DoltClientSelection struct {
	Client        string
	ClientSource  ValueSource
	DoltDir       string
	DoltDirSource ValueSource
}

func (c *Config) FlagChanged(name string) bool {
	return c.changedFlags[name]
}

func (c *Config) ResolveValue(key, defaultVal string) (string, ValueSource) {
	if value, ok := c.explicitFlags[key]; ok && value != "" {
		return value, ValueSourceExplicit
	}
	if envName := EnvNameForKey(key); envName != "" {
		if value := os.Getenv(envName); value != "" {
			return value, ValueSourceEnvironment
		}
	}
	if c.fileValues != nil {
		if value := c.fileValues[key]; value != "" {
			return value, ValueSourceFile
		}
	}
	return defaultVal, ValueSourceDefault
}

func (c *Config) ResolveDoltClient() (DoltClientSelection, error) {
	client, clientSource := c.ResolveValue(KeyDoltClient, "")
	doltDir, dirSource := c.ResolveValue(KeyDoltDir, "")

	if client == "" && doltDir != "" {
		client = "cli"
		clientSource = ValueSourceCompatibility
	}
	if client == "" {
		client = "http"
		clientSource = ValueSourceDefault
	}
	if doltDir != "" && client != "cli" {
		return DoltClientSelection{}, fmt.Errorf("--dolt-dir may only be used with client=cli; effective client is %s", client)
	}
	switch client {
	case "http", "sql", "cli":
	default:
		return DoltClientSelection{}, fmt.Errorf("unsupported dolt.client %q", client)
	}

	return DoltClientSelection{
		Client:        client,
		ClientSource:  clientSource,
		DoltDir:       doltDir,
		DoltDirSource: dirSource,
	}, nil
}
