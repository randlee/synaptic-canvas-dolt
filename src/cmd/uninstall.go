package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/spf13/cobra"
)

type uninstallResponse struct {
	OK      bool            `json:"ok"`
	Removed uninstallResult `json:"removed"`
}

// NewUninstallCmd creates the sc uninstall command.
func NewUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <package>",
		Short: "Uninstall a tracked package",
		Args:  cobra.ExactArgs(1),
		RunE:  runUninstallCmd,
	}
	cmd.Flags().Bool("global", false, "target global install")
	return cmd
}

func runUninstallCmd(cmd *cobra.Command, args []string) error {
	global, err := cmd.Flags().GetBool("global")
	if err != nil {
		return fmt.Errorf("reading --global: %w", err)
	}
	packageID := args[0]

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
	target, err := selectInstall(installs, packageID, global)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
		}
		return err
	}

	validation, err := validateTrackedInstall(target.Record)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	if hasLocalModifications(validation) {
		err := fmt.Errorf("tracked files contain local modifications; refusing uninstall")
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}

	stateRoot, err := stateRootForScope(repoRoot, target.Record.InstallScope)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	lock, err := installer.LoadManifestLock(stateRoot)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	registry, err := installer.LoadHookRegistry(stateRoot)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}

	removedFiles, err := removeOwnedFiles(stateRoot, target.Record)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	lock.RemoveInstall(target.Record.InstallID)
	if err := installer.SaveManifestLock(stateRoot, lock); err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	registry, hooksRemoved := removeHookEntries(registry, target.Record.Package, hasOtherInstall(installs, target.Record))
	if err := installer.SaveHookRegistry(stateRoot, registry); err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
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
	if cfg.JSON {
		return formatter.WriteJSON(uninstallResponse{OK: true, Removed: result})
	}
	rows := [][]string{
		{"package", result.Package},
		{"scope", result.Scope},
		{"files_removed", fmt.Sprintf("%d", len(result.RemovedFiles))},
		{"hooks_removed", fmt.Sprintf("%d", result.HooksRemoved)},
	}
	return formatter.Table([]string{"FIELD", "VALUE"}, rows)
}
