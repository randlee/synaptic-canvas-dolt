package cmd

import (
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/cmd/admin"
	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/logging"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/spf13/cobra"
)

// Execute creates the root command, configures it with version info, and runs it.
func Execute(version, commit, date string) error {
	defer logging.Close()
	rootCmd := NewRootCmd(version, commit, date)
	return executeCommand(rootCmd)
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if skipRootBootstrap(cmd, args) {
				return nil
			}
			cfg, err := loadConfig(cmd)
			if err != nil {
				return renderRootJSONError(cmd, classifyJSONErr(err), err)
			}
			if err := cfg.Validate(); err != nil {
				return renderRootJSONError(cmd, api.ErrorCodeInvalidArgs, fmt.Errorf("invalid configuration: %w", err))
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
				"branch", cfg.EffectiveBranch(),
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
	pf.String("client", "", "Dolt client to use: http, sql, or cli")
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
	_ = pf.MarkHidden("dolt-client")

	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewInstallCmd())
	rootCmd.AddCommand(NewListCmd())
	rootCmd.AddCommand(NewInfoCmd())
	rootCmd.AddCommand(NewValidateCmd())
	rootCmd.AddCommand(NewStatusCmd())
	rootCmd.AddCommand(NewSnapshotCmd())
	rootCmd.AddCommand(NewUpgradeCmd())
	rootCmd.AddCommand(NewUninstallCmd())
	rootCmd.AddCommand(NewConfigCmd())
	rootCmd.AddCommand(NewCatalogCmd())
	rootCmd.AddCommand(NewScanCmd())
	rootCmd.AddCommand(admin.NewAdminCmd())

	return rootCmd
}

func executeCommand(rootCmd *cobra.Command) error {
	err := rootCmd.Execute()
	if err == nil || isJSONCmdError(err) || !jsonRequested(rootCmd) {
		return err
	}
	return renderRootJSONError(rootCmd, classifyJSONErr(err), err)
}

func renderRootJSONError(cmd *cobra.Command, code api.ErrorCode, err error) error {
	if !jsonRequested(cmd) {
		return err
	}
	formatter := output.NewFormatter(true, quietRequested(cmd))
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()
	var cfg *config.Config
	if loadedCfg, cfgErr := loadConfig(cmd); cfgErr == nil {
		cfg = loadedCfg
	}
	metadata := jsonErrorMetadata(cfg, code, err, cmd.Name())
	payload := api.NewError(code, err.Error(), api.ErrorOptions{
		Retryable:       metadata.Retryable,
		Details:         metadata.Details,
		SuggestedAction: metadata.SuggestedAction,
	})
	if writeErr := writeStructuredJSONError(formatter, payload); writeErr != nil {
		return writeErr
	}
	return jsonCmdError{cause: err}
}

func jsonRequested(cmd *cobra.Command) bool {
	return persistentBoolFlag(cmd, "json")
}

func quietRequested(cmd *cobra.Command) bool {
	return persistentBoolFlag(cmd, "quiet")
}

func persistentBoolFlag(cmd *cobra.Command, name string) bool {
	value, err := cmd.Root().PersistentFlags().GetBool(name)
	return err == nil && value
}

func skipRootBootstrap(cmd *cobra.Command, args []string) bool {
	if cmd == cmd.Root() && len(args) == 0 && cmd.Flags().NFlag() == 0 {
		return true
	}
	if flag := cmd.Flags().Lookup("help"); flag != nil && flag.Changed {
		return true
	}
	if flag := cmd.Flags().Lookup("version"); flag != nil && flag.Changed {
		return true
	}
	return false
}

// formatVersion returns a human-readable version string.
func formatVersion(version, commit, date string) string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
}
