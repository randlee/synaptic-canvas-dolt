package cmd

import (
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/spf13/cobra"
)

type upgradeResponse struct {
	OK       bool            `json:"ok"`
	Upgrades []upgradeResult `json:"upgrades"`
}

// NewUpgradeCmd creates the sc upgrade command.
func NewUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade <package>",
		Short: "Upgrade installed packages",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runUpgradeCmd,
	}
	cmd.Flags().Bool("all", false, "upgrade all tracked installs")
	cmd.Flags().Bool("global", false, "target global install")
	cmd.Flags().String("version", "", "required target version on the selected branch")
	return cmd
}

func runUpgradeCmd(cmd *cobra.Command, args []string) error {
	upgradeAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return fmt.Errorf("reading --all: %w", err)
	}
	global, err := cmd.Flags().GetBool("global")
	if err != nil {
		return fmt.Errorf("reading --global: %w", err)
	}
	targetVersion, err := cmd.Flags().GetString("version")
	if err != nil {
		return fmt.Errorf("reading --version: %w", err)
	}
	packageID := ""
	if len(args) == 1 {
		packageID = args[0]
	}

	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("reading config flags: %w", err)
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()

	repoRoot, err := currentRepoRoot()
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	installs, err := loadTrackedInstalls(repoRoot)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}

	targets := []trackedInstall{}
	if upgradeAll {
		targets = installs
		if global {
			targets = filterInstallsByScope(targets, "global")
		}
	} else {
		if packageID == "" {
			err := fmt.Errorf("upgrade requires <package> or --all")
			if cfg.JSON {
				return writeJSONError(formatter, "query_failed", err.Error())
			}
			return err
		}
		install, err := selectInstall(installs, packageID, global)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
		targets = append(targets, *install)
	}

	doltDir, err := detectReadDoltDir(cfg.DoltDirExpanded())
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}

	results := make([]upgradeResult, 0, len(targets))
	for _, target := range targets {
		validation, err := validateTrackedInstall(target.Record)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, "query_failed", err.Error())
			}
			return err
		}
		pkg, files, deps, hooks, questions, err := fetchUpgradePackage(cmd.Context(), readClientOpener, doltDir, cfg.EffectiveBranch(), target.Record)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
		if targetVersion != "" && pkg.Version != targetVersion {
			warning := fmt.Sprintf("requested version %s not available on branch %s", targetVersion, cfg.EffectiveBranch())
			results = append(results, upgradeResult{
				Package:     target.Record.Package,
				Scope:       target.Record.InstallScope,
				FromVersion: target.Record.Version,
				ToVersion:   target.Record.Version,
				FromBranch:  target.Record.Branch,
				ToBranch:    cfg.EffectiveBranch(),
				InstallRoot: target.Record.InstallRoot,
				Warnings:    []string{warning},
			})
			continue
		}

		warnings := buildUpgradeWarnings(target.Record, validation, deps, questions, currentProfileSnapshot(repoRoot))
		if pkg.Version == target.Record.Version && cfg.EffectiveBranch() == target.Record.Branch && targetVersion == "" {
			results = append(results, upgradeResult{
				Package:     target.Record.Package,
				Scope:       target.Record.InstallScope,
				FromVersion: target.Record.Version,
				ToVersion:   target.Record.Version,
				FromBranch:  target.Record.Branch,
				ToBranch:    cfg.EffectiveBranch(),
				InstallRoot: target.Record.InstallRoot,
				Warnings:    append(warnings, "already on latest version for selected branch"),
			})
			continue
		}

		summary, err := (installer.Service{}).Execute(cmd.Context(), installer.Request{
			Package:   pkg,
			Files:     files,
			Deps:      deps,
			Hooks:     hooks,
			Questions: questions,
			Branch:    cfg.EffectiveBranch(),
			Global:    target.Record.InstallScope == "global",
			RepoRoot:  repoRoot,
		})
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
		results = append(results, upgradeResult{
			Package:            target.Record.Package,
			Scope:              target.Record.InstallScope,
			FromVersion:        target.Record.Version,
			ToVersion:          pkg.Version,
			FromBranch:         target.Record.Branch,
			ToBranch:           cfg.EffectiveBranch(),
			InstallRoot:        summary.InstallRoot,
			Warnings:           warnings,
			FilesWritten:       summary.FilesWritten,
			TemplateWarnings:   summary.TemplateValidationWarnings,
			DependencyWarnings: summary.DependencyWarnings,
		})
	}

	if cfg.JSON {
		return formatter.WriteJSON(upgradeResponse{OK: true, Upgrades: results})
	}
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			result.Package,
			result.Scope,
			result.FromVersion,
			result.ToVersion,
			result.ToBranch,
		})
	}
	if err := formatter.Table([]string{"PACKAGE", "SCOPE", "FROM", "TO", "BRANCH"}, rows); err != nil {
		return err
	}
	for _, result := range results {
		for _, warning := range result.Warnings {
			formatter.Success("warning: " + warning)
		}
	}
	return nil
}
