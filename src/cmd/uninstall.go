package cmd

import (
	"context"
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/operations"
	"github.com/spf13/cobra"
)

type uninstallResponse = api.UninstallResponse

// NewUninstallCmd creates the sc uninstall command.
func NewUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <package>",
		Short: "Uninstall a tracked package",
		Args:  cobra.ExactArgs(1),
		RunE:  runUninstallCmd,
	}
	cmd.Flags().String("scope", "both", "uninstall scope: project, global, or both")
	cmd.Flags().Bool("force", false, "remove package files even when local modifications are present")
	cmd.Flags().Bool("yolo", false, "skip interactive confirmations")
	return cmd
}

func runUninstallCmd(cmd *cobra.Command, args []string) error {
	scope, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("reading --scope: %w", err)
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("reading --force: %w", err)
	}
	yolo, err := cmd.Flags().GetBool("yolo")
	if err != nil {
		return fmt.Errorf("reading --yolo: %w", err)
	}
	packageID := args[0]

	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("reading config flags: %w", err)
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()

	response, err := operations.RunUninstall(cmd.Context(), operations.UninstallRequest{
		PackageID: packageID,
		Scope:     scope,
		Force:     force,
		Yolo:      yolo,
		Verbose:   cfg.Verbose,
	}, operations.UninstallDependencies{
		ResolveRepoRoot: currentRepoRoot,
		ValidateTrackedInstall: func(ctx context.Context, record installer.InstallRecord) (api.ValidatedInstall, error) {
			return validateTrackedInstall(ctx, record)
		},
		ConfirmProceed: func(prompt, nonInteractive string, yolo, force bool) error {
			if cfg.JSON {
				return fmt.Errorf("%s", nonInteractive)
			}
			return confirmProceed(cmd, prompt, nonInteractive, yolo, force)
		},
		ConfirmRemoveDependency: func(dep string) (bool, error) { return confirmRemoveDependency(cmd, dep) },
		RemoveSCDependency:      removeSCDependency,
		CommandInputTTY:         func() bool { return isCommandInputTTY(cmd) },
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
		return formatter.WriteJSON(response)
	}
	rows := make([][]string, 0, len(response.RemovedAll))
	for _, item := range response.RemovedAll {
		rows = append(rows, []string{
			item.Package,
			item.Scope,
			fmt.Sprintf("%d", len(item.RemovedFiles)),
			fmt.Sprintf("%d", item.HooksRemoved),
		})
	}
	if err := formatter.Table([]string{"PACKAGE", "SCOPE", "FILES_REMOVED", "HOOKS_REMOVED"}, rows); err != nil {
		return err
	}
	for _, item := range response.RemovedAll {
		for _, warning := range item.Warnings {
			writeWarning(formatter, warning)
		}
	}
	return nil
}
