package cmd

import (
	"fmt"
	"os"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/spf13/cobra"
)

type installScopeFailure struct {
	Package string `json:"package"`
	Scope   string `json:"scope"`
	Error   string `json:"error"`
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
				return writeJSONError(formatter, "query_failed", err.Error())
			}
			return fmt.Errorf("getting current directory: %w", err)
		}
		pkg, err := client.GetPackage(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
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
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
		deps, err := client.GetPackageDeps(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
		hooks, err := client.GetPackageHooks(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
		questions, err := client.GetPackageQuestions(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}

		if err := confirmExternalDeps(cmd, formatter, deps, yolo, dryRun); err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, "interactive_confirmation_required", err.Error())
			}
			return err
		}
		if !dryRun {
			if _, err := initializeRepoFunc(root); err != nil {
				if cfg.JSON {
					return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
				}
				return err
			}
		}

		summaries := make([]installer.Summary, 0, len(scopes))
		failures := make([]installScopeFailure, 0, len(scopes))
		rolledBack := make([]installer.Summary, 0, len(scopes))
		partial := false
		for _, targetScope := range scopes {
			summary, err := (installer.Service{}).Execute(cmd.Context(), installer.Request{
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
					failures = append(failures, installScopeFailure{
						Package: pkg.ID,
						Scope:   targetScope,
						Error:   err.Error(),
					})
					remaining := summaries[:0]
					for _, prior := range summaries {
						if rollbackErr := rollbackInstallSummary(root, prior); rollbackErr != nil {
							partial = true
							failures = append(failures, installScopeFailure{
								Package: prior.PackageID,
								Scope:   prior.Scope,
								Error:   "rollback failed: " + rollbackErr.Error(),
							})
							remaining = append(remaining, prior)
							continue
						}
						rolledBack = append(rolledBack, prior)
					}
					summaries = remaining
					break
				}
				if cfg.JSON {
					return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
				}
				return err
			}
			summaries = append(summaries, summary)
		}
		if len(summaries) == 0 {
			if len(failures) > 0 {
				if cfg.JSON {
					message := "install failed for all selected scopes"
					if err := formatter.WriteJSON(map[string]any{
						"ok":          false,
						"error":       jsonErrorPayload{Code: "install_failed", Message: message},
						"plan":        dryRun,
						"scope":       scope,
						"partial":     partial,
						"installs":    summaries,
						"rolled_back": rolledBack,
						"failures":    failures,
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
				return formatter.WriteJSON(map[string]any{
					"ok":    true,
					"plan":  dryRun,
					"scope": summary.Scope,
					"package": map[string]any{
						"id":      summary.PackageID,
						"version": summary.Version,
						"branch":  summary.Branch,
					},
					"install_root":                 summary.InstallRoot,
					"files_written":                summary.FilesWritten,
					"dependencies":                 summary.Dependencies,
					"dependency_warnings":          summary.DependencyWarnings,
					"hooks_registered":             summary.HooksRegistered,
					"template_validation_warnings": summary.TemplateValidationWarnings,
					"warnings":                     allWarnings,
					"files":                        summary.Files,
					"answers":                      summary.Answers,
				})
			}
			errorValue := any(nil)
			if partialFailed {
				errorValue = jsonErrorPayload{Code: "install_failed", Message: "install failed for one or more scopes"}
			}
			if err := formatter.WriteJSON(map[string]any{
				"ok":          !partialFailed,
				"error":       errorValue,
				"plan":        dryRun,
				"scope":       scope,
				"partial":     partial,
				"installs":    summaries,
				"rolled_back": rolledBack,
				"failures":    failures,
				"warnings":    allWarnings,
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
