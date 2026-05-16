package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/operations"
	"github.com/spf13/cobra"
)

type installScopeFailure = api.InstallScopeFailure

var executeInstallService = func(ctx context.Context, req installer.Request) (installer.Summary, error) {
	return (installer.Service{}).Execute(ctx, req)
}

// NewInstallCmd creates the sc install command.
func NewInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <package>",
		Short: "Install a package from Dolt",
		Args:  cobra.ExactArgs(1),
		RunE:  runInstallCmd,
	}
	cmd.Flags().String("scope", "both", "install scope: project, global, or both")
	cmd.Flags().Bool("dry-run", false, "show plan without side effects")
	cmd.Flags().Bool("yolo", false, "skip interactive confirmations")
	return cmd
}

func runInstallCmd(cmd *cobra.Command, args []string) error {
	scope, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("reading --scope: %w", err)
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("reading --dry-run: %w", err)
	}
	yolo, err := cmd.Flags().GetBool("yolo")
	if err != nil {
		return fmt.Errorf("reading --yolo: %w", err)
	}
	packageID := args[0]

	return withReadClient(cmd, func(cfg *config.Config, client readClient) error {
		formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
		formatter.Writer = cmd.OutOrStdout()
		formatter.ErrW = cmd.ErrOrStderr()

		result, err := operations.RunInstall(cmd.Context(), operations.InstallRequest{
			PackageID: packageID,
			Scope:     scope,
			Branch:    cfg.EffectiveBranch(),
			DryRun:    dryRun,
			Yolo:      yolo,
		}, operations.InstallDependencies{
			Reader:          client,
			ResolveRepoRoot: os.Getwd,
			ConfirmExternalDeps: func(deps []models.PackageDep, yolo, dryRun bool) error {
				return confirmExternalDeps(cmd, formatter, deps, yolo, dryRun)
			},
			InitializeRepo: func(root string) error {
				_, err := initializeRepoFunc(root)
				return err
			},
			ExecuteInstall: executeInstallService,
			RefreshCatalog: func(ctx context.Context, repoRoot, branch string) []string {
				return refreshCatalogNonFatal(ctx, formatter, repoRoot, branch, client)
			},
			ClassifyError: func(err error, operation string) api.Error {
				return classifyRuntimeJSONError(cfg, err, operation)
			},
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
		return renderInstallResult(formatter, cfg.JSON, result)
	})
}

func renderInstallResult(formatter *output.Formatter, jsonMode bool, result operations.InstallResult) error {
	if jsonMode {
		if len(result.Summaries) == 1 && result.OK() {
			summary := result.Summaries[0]
			if err := formatter.WriteJSON(api.InstallResponse{
				OK:    true,
				Plan:  result.Plan,
				Scope: summary.Scope,
				Package: &api.InstallPackageRef{
					ID:      summary.PackageID,
					Version: summary.Version,
					Branch:  summary.Branch,
				},
				InstallRoot:                summary.InstallRoot,
				FilesWritten:               summary.FilesWritten,
				Dependencies:               summary.Dependencies,
				DependencyWarnings:         summary.DependencyWarnings,
				HooksRegistered:            apiInstallHookEntries(summary.HooksRegistered),
				TemplateValidationWarnings: summary.TemplateValidationWarnings,
				Warnings:                   result.Warnings,
				Files:                      apiInstallPlannedFiles(summary.Files),
				Answers:                    apiInstallAnswers(summary.Answers),
			}); err != nil {
				return err
			}
			return nil
		}
		if err := formatter.WriteJSON(api.InstallResponse{
			OK:         result.OK(),
			Error:      result.AggregateError(),
			Plan:       result.Plan,
			Scope:      result.Scope,
			Partial:    result.Partial,
			Installs:   apiInstallSummaries(result.Summaries),
			RolledBack: apiInstallSummaries(result.RolledBack),
			Failures:   result.Failures,
			Warnings:   result.Warnings,
		}); err != nil {
			return err
		}
		if !result.OK() {
			return jsonCmdError{cause: fmt.Errorf("%s", result.ErrorMessage())}
		}
		return nil
	}

	rows := make([][]string, 0, len(result.Summaries))
	for _, summary := range result.Summaries {
		rows = append(rows, []string{
			summary.PackageID,
			summary.Version,
			summary.Branch,
			summary.Scope,
			summary.InstallRoot,
			fmt.Sprintf("%d", summary.FilesWritten),
		})
	}
	if err := formatter.Table([]string{"PACKAGE", "VERSION", "BRANCH", "SCOPE", "INSTALL_ROOT", "FILES"}, rows); err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		writeWarning(formatter, warning)
	}
	for _, failure := range result.Failures {
		writeWarning(formatter, fmt.Sprintf("%s [%s] failed: %s", failure.Package, failure.Scope, failure.Error))
	}
	for _, summary := range result.Summaries {
		for _, warning := range summary.DependencyWarnings {
			formatter.Success("warning: " + warning)
		}
		for _, warning := range summary.TemplateValidationWarnings {
			formatter.Success("template warning: " + warning)
		}
	}
	if !result.OK() {
		return fmt.Errorf("%s", result.ErrorMessage())
	}
	return nil
}
