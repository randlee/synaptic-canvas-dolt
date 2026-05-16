package cmd

import (
	"fmt"
	"path/filepath"

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
	if err := validateScope(scope); err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", err.Error())
		}
		return err
	}

	repoRoot, err := currentRepoRoot()
	if err != nil {
		if cfg.JSON {
			return writeClassifiedJSONError(formatter, cfg, err)
		}
		return err
	}
	installs, err := operations.LoadTrackedInstalls(repoRoot)
	if err != nil {
		if cfg.JSON {
			return writeClassifiedJSONError(formatter, cfg, err)
		}
		return err
	}
	targets := selectInstalls(installs, packageID, scope)
	if len(targets) == 0 {
		err := fmt.Errorf("package %q is not installed", packageID)
		if cfg.JSON {
			return writeClassifiedJSONError(formatter, cfg, err)
		}
		return err
	}

	results := make([]uninstallResult, 0, len(targets))
	for _, target := range targets {
		validation, err := validateTrackedInstall(cmd.Context(), target.Record)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
		if hasLocalModifications(validation) && !force && !yolo {
			if cfg.JSON {
				err := fmt.Errorf("locally modified files detected; use --force to proceed or --yolo in non-interactive mode")
				return writeJSONError(formatter, api.ErrorCodeBlocked, err.Error())
			}
			err := confirmProceed(cmd, "Package has locally modified files. Proceed anyway?", "locally modified files detected; use --force to proceed or --yolo in non-interactive mode", yolo, force)
			if err != nil {
				return err
			}
		}

		stateRoot, err := stateRootForScope(repoRoot, target.Record.InstallScope)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
		if err := installer.WithManifestLock(stateRoot, func(lock *installer.ManifestLock) error {
			removeInstallRecord(lock, target.Record)
			return nil
		}); err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
		hooksRemoved := 0
		if err := installer.WithHookRegistry(stateRoot, func(registry *installer.HookRegistry) error {
			hooksRemoved = registry.RemovePackageHooks(target.Record.Package, target.Record.InstallScope)
			return nil
		}); err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
		removedFiles, err := removeOwnedFiles(stateRoot, target.Record)
		if err != nil {
			writeWarning(formatter, "manifest updated but file removal failed: "+err.Error())
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
		pruneEmptyParents(filepath.Join(stateRoot, ".synaptic", "hooks"), filepath.Join(stateRoot, ".synaptic"))

		result := uninstallResult{
			Package:      target.Record.Package,
			Scope:        target.Record.InstallScope,
			RemovedFiles: removedFiles,
			HooksRemoved: hooksRemoved,
		}
		for _, dep := range target.Record.Requirements.CLIInstalled {
			if !target.Record.Requirements.IsInstalledBySC(dep) {
				if cfg.Verbose {
					result.Warnings = append(result.Warnings, "leaving pre-existing dependency untouched: "+dep)
				}
				continue
			}

			removeDep := yolo
			if !removeDep {
				confirmed, err := confirmRemoveDependency(cmd, dep)
				if err != nil {
					if cfg.JSON {
						return writeJSONError(formatter, api.ErrorCodeConfirmationNeeded, err.Error())
					}
					return err
				}
				removeDep = confirmed
				if !confirmed && !isCommandInputTTY(cmd) {
					result.Warnings = append(result.Warnings, "skipped SC-installed dependency removal in non-interactive mode: "+dep)
				}
			}
			if !removeDep {
				continue
			}
			if err := removeSCDependency(dep); err != nil {
				if cfg.JSON {
					return writeClassifiedJSONError(formatter, cfg, err)
				}
				return err
			}
			result.RemovedDependencies = append(result.RemovedDependencies, dep)
		}
		results = append(results, result)
	}

	result := results[0]
	if cfg.JSON {
		return formatter.WriteJSON(uninstallResponse{OK: true, Removed: result, RemovedAll: results})
	}
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			result.Package,
			result.Scope,
			fmt.Sprintf("%d", len(result.RemovedFiles)),
			fmt.Sprintf("%d", result.HooksRemoved),
		})
	}
	if err := formatter.Table([]string{"PACKAGE", "SCOPE", "FILES_REMOVED", "HOOKS_REMOVED"}, rows); err != nil {
		return err
	}
	for _, result := range results {
		for _, warning := range result.Warnings {
			writeWarning(formatter, warning)
		}
	}
	return nil
}
