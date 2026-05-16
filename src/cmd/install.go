package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
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

		scopes, err := scopesFromFlag(scope)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, "invalid_args", err.Error())
			}
			return err
		}
		root, err := os.Getwd()
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, api.ErrorCodeInternal, err.Error())
			}
			return fmt.Errorf("getting current directory: %w", err)
		}
		pkg, err := client.GetPackage(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err, "get_package")
			}
			return err
		}
		if pkg == nil {
			err := fmt.Errorf("package %q not found", packageID)
			if cfg.JSON {
				return writeJSONError(formatter, "not_found", err.Error())
			}
			return err
		}
		if string(pkg.InstallScope) == "local-only" && (scope == "global" || scope == "both") {
			err := fmt.Errorf("package %s is local-only and cannot be installed globally", pkg.ID)
			if cfg.JSON {
				return writeJSONError(formatter, "invalid_args", err.Error())
			}
			return err
		}
		files, err := client.GetPackageFiles(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err, "get_package_files")
			}
			return err
		}
		deps, err := client.GetPackageDeps(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err, "get_package_deps")
			}
			return err
		}
		hooks, err := client.GetPackageHooks(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err, "get_package_hooks")
			}
			return err
		}
		questions, err := client.GetPackageQuestions(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err, "get_package_questions")
			}
			return err
		}

		if err := confirmExternalDeps(cmd, formatter, deps, yolo, dryRun); err != nil {
			if cfg.JSON {
				return writeStructuredJSONError(formatter, classifyRuntimeJSONError(cfg, err, "confirm_external_dependencies"))
			}
			return err
		}
		if !dryRun {
			if _, err := initializeRepoFunc(root); err != nil {
				if cfg.JSON {
					return writeJSONError(formatter, api.ErrorCodeInternal, err.Error())
				}
				return err
			}
		}

		summaries := make([]installer.Summary, 0, len(scopes))
		failures := make([]installScopeFailure, 0, len(scopes))
		rolledBack := make([]installer.Summary, 0, len(scopes))
		partial := false
		for _, targetScope := range scopes {
			summary, err := executeInstallService(cmd.Context(), installer.Request{
				Package:   pkg,
				Files:     files,
				Deps:      deps,
				Hooks:     hooks,
				Questions: questions,
				Branch:    cfg.EffectiveBranch(),
				Global:    targetScope == "global",
				DryRun:    dryRun,
				RepoRoot:  root,
			})
			if err != nil {
				if scope == "both" {
					failureErr := classifyRuntimeJSONError(cfg, err, "install_scope")
					failures = append(failures, api.NewInstallScopeFailure(pkg.ID, targetScope, failureErr))
					remaining := summaries[:0]
					for _, prior := range summaries {
						if rollbackErr := rollbackInstallSummary(root, prior); rollbackErr != nil {
							partial = true
							failures = append(failures, api.NewInstallScopeFailure(
								prior.PackageID,
								prior.Scope,
								api.NewError(api.ErrorCodeInternal, "rollback failed: "+rollbackErr.Error()),
							))
							remaining = append(remaining, prior)
							continue
						}
						rolledBack = append(rolledBack, prior)
					}
					summaries = remaining
					break
				}
				if cfg.JSON {
					return writeClassifiedJSONError(formatter, cfg, err, "install_scope")
				}
				return err
			}
			summaries = append(summaries, summary)
		}
		if len(summaries) == 0 {
			if len(failures) > 0 {
				if cfg.JSON {
					message := "install failed for all selected scopes"
					errorValue := aggregateInstallError(message, failures)
					if err := formatter.WriteJSON(api.InstallResponse{
						OK:         false,
						Error:      &errorValue,
						Plan:       dryRun,
						Scope:      scope,
						Partial:    partial,
						Installs:   apiInstallSummaries(summaries),
						RolledBack: apiInstallSummaries(rolledBack),
						Failures:   failures,
					}); err != nil {
						return err
					}
					return jsonCmdError{cause: fmt.Errorf("%s", message)}
				}
				return fmt.Errorf("install failed for all selected scopes")
			}
			err := fmt.Errorf("no install scopes were eligible for package %q", packageID)
			if cfg.JSON {
				return writeJSONError(formatter, "invalid_args", err.Error())
			}
			return err
		}
		catalogWarnings := []string{}
		if !dryRun {
			catalogWarnings = refreshCatalogNonFatal(cmd.Context(), formatter, root, cfg.EffectiveBranch(), client)
		}
		allWarnings := catalogWarnings
		partialFailed := len(failures) > 0
		if cfg.JSON {
			if len(summaries) == 1 && !partialFailed {
				summary := summaries[0]
				return formatter.WriteJSON(api.InstallResponse{
					OK:    true,
					Plan:  dryRun,
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
					Warnings:                   allWarnings,
					Files:                      apiInstallPlannedFiles(summary.Files),
					Answers:                    apiInstallAnswers(summary.Answers),
				})
			}
			var errorValue *api.Error
			if partialFailed {
				aggregate := aggregateInstallError("install failed for one or more scopes", failures)
				errorValue = &aggregate
			}
			if err := formatter.WriteJSON(api.InstallResponse{
				OK:         !partialFailed,
				Error:      errorValue,
				Plan:       dryRun,
				Scope:      scope,
				Partial:    partial,
				Installs:   apiInstallSummaries(summaries),
				RolledBack: apiInstallSummaries(rolledBack),
				Failures:   failures,
				Warnings:   allWarnings,
			}); err != nil {
				return err
			}
			if partialFailed {
				return jsonCmdError{cause: fmt.Errorf("install failed for one or more scopes")}
			}
			return nil
		}

		rows := make([][]string, 0, len(summaries))
		for _, summary := range summaries {
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
		for _, warning := range allWarnings {
			writeWarning(formatter, warning)
		}
		for _, failure := range failures {
			writeWarning(formatter, fmt.Sprintf("%s [%s] failed: %s", failure.Package, failure.Scope, failure.Error))
		}
		for _, summary := range summaries {
			for _, warning := range summary.DependencyWarnings {
				formatter.Success("warning: " + warning)
			}
			for _, warning := range summary.TemplateValidationWarnings {
				formatter.Success("template warning: " + warning)
			}
		}
		if partialFailed {
			return fmt.Errorf("install failed for one or more scopes")
		}
		return nil
	})
}

func aggregateInstallError(message string, failures []installScopeFailure) api.Error {
	dominant := dominantInstallFailure(failures)
	return api.NewError(dominant.Code, message, api.ErrorOptions{
		Retryable:       dominant.Retryable,
		Details:         dominant.Details,
		SuggestedAction: dominant.SuggestedAction,
	})
}

func dominantInstallFailure(failures []installScopeFailure) installScopeFailure {
	for _, failure := range failures {
		if failure.Code != "" && failure.Code != api.ErrorCodeInternal {
			return failure
		}
	}
	if len(failures) > 0 {
		return failures[0]
	}
	return installScopeFailure{
		Code:      api.ErrorCodeInternal,
		Error:     "install failed",
		Retryable: false,
	}
}
