package cmd

import (
	"fmt"
	"os"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/spf13/cobra"
)

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
		if !dryRun {
			if _, err := initializeRepoFunc(root); err != nil {
				if cfg.JSON {
					return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
				}
				return err
			}
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

		summaries := make([]installer.Summary, 0, len(scopes))
		scopeWarnings := []string{}
		for _, targetScope := range scopes {
			if targetScope == "global" && string(pkg.InstallScope) == "local-only" {
				scopeWarnings = append(scopeWarnings, fmt.Sprintf("package %s cannot be installed globally; skipped global scope", pkg.ID))
				continue
			}

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
				if cfg.JSON {
					return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
				}
				return err
			}
			summaries = append(summaries, summary)
		}
		if len(summaries) == 0 {
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
		allWarnings := append(scopeWarnings, catalogWarnings...)
		if cfg.JSON {
			if len(summaries) == 1 {
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
			return formatter.WriteJSON(map[string]any{
				"ok":       true,
				"plan":     dryRun,
				"scope":    scope,
				"installs": summaries,
				"warnings": allWarnings,
			})
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
		for _, summary := range summaries {
			for _, warning := range summary.DependencyWarnings {
				formatter.Success("warning: " + warning)
			}
			for _, warning := range summary.TemplateValidationWarnings {
				formatter.Success("template warning: " + warning)
			}
		}
		return nil
	})
}
