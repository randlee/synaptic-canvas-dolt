package cmd

import (
	"fmt"

	"github.com/randlee/synaptic-canvas/internal/config"
	"github.com/randlee/synaptic-canvas/internal/logging"
	"github.com/spf13/cobra"
)

// Execute creates the root command, configures it with version info, and runs it.
func Execute(version, commit, date string) error {
	rootCmd := NewRootCmd(version, commit, date)
	return rootCmd.Execute()
}

// NewRootCmd creates and returns the root cobra.Command for the sc CLI.
func NewRootCmd(version, commit, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sc",
		Short: "Synaptic Canvas — Dolt-backed package manager for Claude Code skills",
		Long: `Synaptic Canvas is a Dolt-backed package management system for Claude Code skills.
The sc CLI provides commands to search, install, export, and manage skill packages
stored in a Dolt database.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       formatVersion(version, commit, date),
		// Show help when invoked with no subcommand.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.NewConfigFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("reading config flags: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}
			logger := logging.Setup(cfg.Verbose, cfg.Quiet)
			logger = logging.WithContext(logger, "cli", "init")

			doltDirDisplay := cfg.DoltDirExpanded()
			if doltDirDisplay == "" {
				doltDirDisplay = "(auto-detect)"
			}
			logger.Debug("configuration loaded",
				"dolt_dir", doltDirDisplay,
				"remote", cfg.Remote,
				"json", cfg.JSON,
				"verbose", cfg.Verbose,
				"quiet", cfg.Quiet,
			)
			return nil
		},
	}

	// Override the default version template to match the required format.
	rootCmd.SetVersionTemplate("sc version {{.Version}}\n")

	// Register persistent (global) flags.
	pf := rootCmd.PersistentFlags()
	pf.String("dolt-dir", "", "Dolt database directory (default: auto-detect)")
	pf.String("remote", "", "DoltHub remote name")
	pf.Bool("json", false, "output as JSON")
	pf.Bool("quiet", false, "suppress non-essential output")
	pf.Bool("verbose", false, "enable debug logging")

	return rootCmd
}

// formatVersion returns a human-readable version string.
func formatVersion(version, commit, date string) string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
}
