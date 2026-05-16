package cmd

import (
	"context"
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/operations"
	"github.com/spf13/cobra"
)

type upgradeResponse = api.UpgradeResponse

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
	yolo, err := cmd.Flags().GetBool("yolo")
	if err != nil {
		return fmt.Errorf("reading --yolo: %w", err)
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

	result, err := operations.RunUpgrade(cmd.Context(), operations.UpgradeRequest{
		PackageID:       packageID,
		UpgradeAll:      upgradeAll,
		Scope:           scope,
		TargetVersion:   targetVersion,
		Force:           force,
		Yolo:            yolo,
		EffectiveBranch: cfg.EffectiveBranch(),
		BranchExplicit:  branchFlagChanged(cmd),
	}, operations.UpgradeDependencies{
		ResolveRepoRoot: currentRepoRoot,
		ValidateTrackedInstall: func(ctx context.Context, record installer.InstallRecord) (api.ValidatedInstall, error) {
			return validateTrackedInstall(ctx, record)
		},
		OpenClient: func(branch string) (operations.UpgradeClient, error) {
			branchCfg := *cfg
			branchCfg.Branch = branch
			return readClientOpener(&branchCfg)
		},
		ConfirmExternalDeps: func(deps []models.PackageDep, yolo bool) error {
			return confirmExternalDeps(cmd, formatter, deps, yolo, false)
		},
		ExecuteInstall: func(ctx context.Context, req installer.Request) (installer.Summary, error) {
			return executeInstallService(ctx, req)
		},
		ProfileSnapshot: currentProfileSnapshot,
	})
	if err != nil {
		if cfg.JSON {
			op := operations.OperationName(err)
			if op != "" {
				return writeClassifiedJSONError(formatter, cfg, err, op)
			}
			return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
		}
		return err
	}

	if cfg.JSON {
		return formatter.WriteJSON(result.Response)
	}
	rows := make([][]string, 0, len(result.Response.Upgrades))
	for _, item := range result.Response.Upgrades {
		rows = append(rows, []string{
			item.Package,
			item.Scope,
			item.FromVersion,
			item.ToVersion,
			item.ToBranch,
		})
	}
	if err := formatter.Table([]string{"PACKAGE", "SCOPE", "FROM", "TO", "BRANCH"}, rows); err != nil {
		return err
	}
	for _, item := range result.Response.Upgrades {
		for _, warning := range item.Warnings {
			formatter.Success("warning: " + warning)
		}
	}
	if !result.Response.OK && len(result.Response.Upgrades) > 0 {
		return fmt.Errorf("all upgrades failed or were skipped")
	}
	return nil
}

func branchFlagChanged(cmd *cobra.Command) bool {
	flag := cmd.Flag("branch")
	return flag != nil && flag.Changed
}
