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
	cmd.Flags().Bool("global", false, "install to ~/.claude")
	cmd.Flags().Bool("dry-run", false, "show plan without side effects")
	return cmd
}

func runInstallCmd(cmd *cobra.Command, args []string) error {
	globalInstall, err := cmd.Flags().GetBool("global")
	if err != nil {
		return fmt.Errorf("reading --global: %w", err)
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("reading --dry-run: %w", err)
	}
	packageID := args[0]

	return withReadClient(cmd, func(cfg *config.Config, client readClient) error {
		formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
		formatter.Writer = cmd.OutOrStdout()
		formatter.ErrW = cmd.ErrOrStderr()

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

		summary, err := (installer.Service{}).Execute(cmd.Context(), installer.Request{
			Package:   pkg,
			Files:     files,
			Deps:      deps,
			Hooks:     hooks,
			Questions: questions,
			Branch:    cfg.EffectiveBranch(),
			Global:    globalInstall,
			DryRun:    dryRun,
			RepoRoot:  root,
		})
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
		if cfg.JSON {
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
				"files":                        summary.Files,
				"answers":                      summary.Answers,
			})
		}

		rows := [][]string{
			{"package", summary.PackageID},
			{"version", summary.Version},
			{"branch", summary.Branch},
			{"scope", summary.Scope},
			{"install_root", summary.InstallRoot},
			{"files_written", fmt.Sprintf("%d", summary.FilesWritten)},
		}
		if err := formatter.Table([]string{"FIELD", "VALUE"}, rows); err != nil {
			return err
		}
		if len(summary.DependencyWarnings) > 0 {
			for _, warning := range summary.DependencyWarnings {
				formatter.Success("warning: " + warning)
			}
		}
		for _, warning := range summary.TemplateValidationWarnings {
			formatter.Success("template warning: " + warning)
		}
		return nil
	})
}
