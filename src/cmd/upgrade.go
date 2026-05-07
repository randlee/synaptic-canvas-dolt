package cmd

import (
	"fmt"

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
	cmd.Flags().String("scope", "both", "upgrade scope: project, global, or both")
	cmd.Flags().String("version", "", "required target version on the selected branch")
	cmd.Flags().Bool("force", false, "force a blocked single-package upgrade")
	cmd.Flags().Bool("yolo", false, "skip interactive confirmations")
	return cmd
}

func runUpgradeCmd(cmd *cobra.Command, args []string) error {
	upgradeAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return fmt.Errorf("reading --all: %w", err)
	}
	scope, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("reading --scope: %w", err)
	}
	targetVersion, err := cmd.Flags().GetString("version")
	if err != nil {
		return fmt.Errorf("reading --version: %w", err)
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("reading --force: %w", err)
	}
	packageID := ""
	if len(args) == 1 {
		packageID = args[0]
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return fmt.Errorf("reading config flags: %w", err)
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()
	if err := validateScope(scope); err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", err.Error())
		}
		return err
	}
	if upgradeAll && force {
		message := "--force cannot be used with --all; target a specific package"
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", message)
		}
		return fmt.Errorf("%s", message)
	}

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

	var targets []trackedInstall
	if upgradeAll {
		targets = filterInstallsByScope(installs, scope)
	} else {
		if packageID == "" {
			err := fmt.Errorf("upgrade requires <package> or --all")
			if cfg.JSON {
				return writeJSONError(formatter, "query_failed", err.Error())
			}
			return err
		}
		targets = selectInstalls(installs, packageID, scope)
		if len(targets) == 0 {
			err := fmt.Errorf("package %q is not installed", packageID)
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
	}

	client, err := readClientOpener(cfg)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	defer func() { _ = client.Close() }()

	results := make([]upgradeResult, 0, len(targets))
	successes := 0
	for _, target := range targets {
		validation, err := validateTrackedInstall(target.Record)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, "query_failed", err.Error())
			}
			return err
		}
		pkg, files, deps, hooks, questions, err := fetchUpgradePackage(cmd.Context(), client, cfg.EffectiveBranch(), target.Record)
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
				Skipped:     true,
			})
			continue
		}

		warnings := buildUpgradeWarnings(target.Record, validation, deps, questions, currentProfileSnapshot(repoRoot))
		if blockers := dependencyBlockers(deps); len(blockers) > 0 && (upgradeAll || !force) {
			results = append(results, upgradeResult{
				Package:     target.Record.Package,
				Scope:       target.Record.InstallScope,
				FromVersion: target.Record.Version,
				ToVersion:   target.Record.Version,
				FromBranch:  target.Record.Branch,
				ToBranch:    cfg.EffectiveBranch(),
				InstallRoot: target.Record.InstallRoot,
				Warnings:    append(warnings, append([]string{"skipped upgrade"}, blockers...)...),
				Skipped:     true,
			})
			continue
		}
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
				Skipped:     true,
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
		successes++
	}

	if cfg.JSON {
		return formatter.WriteJSON(upgradeResponse{OK: successes > 0 || len(results) == 0, Upgrades: results})
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
	if upgradeAll && successes == 0 && len(results) > 0 {
		return fmt.Errorf("all upgrades failed or were skipped")
	}
	return nil
}
