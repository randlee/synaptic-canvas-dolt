package operations

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
)

type UninstallRequest struct {
	PackageID string
	Scope     string
	RepoRoot  string
	Force     bool
	Yolo      bool
	Verbose   bool
}

type UninstallDependencies struct {
	LoadInstalls            func(string) ([]TrackedInstall, error)
	ResolveRepoRoot         func() (string, error)
	ValidateTrackedInstall  func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error)
	ConfirmProceed          func(string, string, bool, bool) error
	ConfirmRemoveDependency func(string) (bool, error)
	RemoveSCDependency      func(string) error
	CommandInputTTY         func() bool
}

func RunUninstall(ctx context.Context, req UninstallRequest, deps UninstallDependencies) (api.UninstallResponse, error) {
	response := api.UninstallResponse{OK: true}
	if err := ValidateScope(req.Scope); err != nil {
		return response, err
	}
	repoRoot := req.RepoRoot
	var err error
	if repoRoot == "" {
		if deps.ResolveRepoRoot == nil {
			return response, fmt.Errorf("repo root resolver is required")
		}
		repoRoot, err = deps.ResolveRepoRoot()
		if err != nil {
			return response, err
		}
	}
	loadInstalls := deps.LoadInstalls
	if loadInstalls == nil {
		loadInstalls = LoadTrackedInstalls
	}
	installs, err := loadInstalls(repoRoot)
	if err != nil {
		return response, err
	}
	targets := SelectInstalls(installs, req.PackageID, req.Scope)
	if len(targets) == 0 {
		return response, fmt.Errorf("package %q is not installed", req.PackageID)
	}
	if deps.ValidateTrackedInstall == nil {
		return response, fmt.Errorf("tracked install validator is required")
	}
	results := make([]api.UninstallResult, 0, len(targets))
	for _, target := range targets {
		validation, err := deps.ValidateTrackedInstall(ctx, target.Record)
		if err != nil {
			return response, err
		}
		if HasLocalModifications(validation) && !req.Force && !req.Yolo {
			nonInteractive := "locally modified files detected; use --force to proceed or --yolo in non-interactive mode"
			if deps.ConfirmProceed != nil {
				if err := deps.ConfirmProceed("Package has locally modified files. Proceed anyway?", nonInteractive, req.Yolo, req.Force); err != nil {
					return response, err
				}
			} else {
				return response, fmt.Errorf("%s", nonInteractive)
			}
		}
		stateRoot, err := StateRootForScope(repoRoot, target.Record.InstallScope)
		if err != nil {
			return response, err
		}
		removedFiles, failedFiles := RemoveOwnedFiles(stateRoot, target.Record)
		if len(failedFiles) > 0 {
			return response, fmt.Errorf("conflict removing tracked files: %s; manifest record preserved", strings.Join(failedFiles, ", "))
		}
		hooksRemoved := 0
		if err := installer.WithHookRegistry(stateRoot, func(registry *installer.HookRegistry) error {
			hooksRemoved = registry.RemovePackageHooks(target.Record.Package, target.Record.InstallScope)
			return nil
		}); err != nil {
			return response, err
		}
		if err := installer.WithManifestLock(stateRoot, func(lock *installer.ManifestLock) error {
			RemoveInstallRecord(lock, target.Record)
			return nil
		}); err != nil {
			return response, err
		}
		PruneEmptyParents(filepath.Join(stateRoot, ".synaptic", "hooks"), filepath.Join(stateRoot, ".synaptic"))

		result := api.UninstallResult{
			Package:      target.Record.Package,
			Scope:        target.Record.InstallScope,
			RemovedFiles: removedFiles,
			HooksRemoved: hooksRemoved,
		}
		for _, dep := range target.Record.Requirements.CLIInstalled {
			if !target.Record.Requirements.IsInstalledBySC(dep) {
				if req.Verbose {
					result.Warnings = append(result.Warnings, "leaving pre-existing dependency untouched: "+dep)
				}
				continue
			}
			removeDep := req.Yolo
			if !removeDep && deps.ConfirmRemoveDependency != nil {
				confirmed, err := deps.ConfirmRemoveDependency(dep)
				if err != nil {
					return response, err
				}
				removeDep = confirmed
				if !confirmed && deps.CommandInputTTY != nil && !deps.CommandInputTTY() {
					result.Warnings = append(result.Warnings, "skipped SC-installed dependency removal in non-interactive mode: "+dep)
				}
			}
			if !removeDep {
				continue
			}
			if deps.RemoveSCDependency != nil {
				if err := deps.RemoveSCDependency(dep); err != nil {
					return response, err
				}
			}
			result.RemovedDependencies = append(result.RemovedDependencies, dep)
		}
		results = append(results, result)
	}
	if len(results) > 0 {
		response.Removed = results[0]
	}
	response.RemovedAll = results
	return response, nil
}
