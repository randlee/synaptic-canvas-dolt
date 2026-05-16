package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/repo"
)

type upgradeResult = api.UpgradeResult
type uninstallResult = api.UninstallResult

func stateRootForScope(repoRoot, scope string) (string, error) {
	if scope == "global" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home dir: %w", err)
		}
		return home, nil
	}
	return repoRoot, nil
}

func selectInstalls(installs []trackedInstall, packageID, scope string) []trackedInstall {
	var matches []trackedInstall
	for _, install := range installs {
		if install.Record.Package == packageID && (scope == "both" || install.Record.InstallScope == scope) {
			matches = append(matches, install)
		}
	}
	return matches
}

func fetchUpgradePackage(ctx context.Context, client readClient, branch string, install installer.InstallRecord) (*models.Package, []models.PackageFile, []models.PackageDep, []models.PackageHook, []models.PackageQuestion, error) {
	targetID := install.Package
	if variantID, err := client.ResolveVariant(ctx, install.Package, install.Variant); err != nil {
		return nil, nil, nil, nil, nil, err
	} else if variantID != "" {
		targetID = variantID
	}

	pkg, err := client.GetPackage(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if pkg == nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("package %q not found on branch %s", targetID, branch)
	}
	files, err := client.GetPackageFiles(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	deps, err := client.GetPackageDeps(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	hooks, err := client.GetPackageHooks(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	questions, err := client.GetPackageQuestions(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return pkg, files, deps, hooks, questions, nil
}

func buildUpgradeWarnings(install installer.InstallRecord, validation validatedInstall, deps []models.PackageDep, questions []models.PackageQuestion, profile map[string]any) []string {
	warnings := []string{}
	if hasLocalModifications(validation) {
		warnings = append(warnings, "local modifications detected; overwriting tracked files")
	}
	currentQuestions := install.QuestionSnapshot.QuestionIDs
	nextQuestions := make([]string, 0, len(questions))
	for _, q := range questions {
		nextQuestions = append(nextQuestions, q.QuestionID)
	}
	for _, questionID := range nextQuestions {
		if !slices.Contains(currentQuestions, questionID) {
			warnings = append(warnings, "new question detected: "+questionID)
		}
	}
	nextDeps := make([]string, 0, len(deps))
	for _, dep := range deps {
		nextDeps = append(nextDeps, dep.DepName+dep.DepSpec)
	}
	for _, dep := range nextDeps {
		if !slices.Contains(install.Requirements.Tools, dep) {
			warnings = append(warnings, "new dependency requirement: "+dep)
		}
	}
	if install.RepoProfile["primary_language"] != profile["primary_language"] || install.RepoProfile["name"] != profile["name"] {
		warnings = append(warnings, "repo profile changed; templates will be re-rendered")
	}
	return warnings
}

func dependencyBlockers(deps []models.PackageDep) []string {
	blockers := []string{}
	for _, dep := range deps {
		name := strings.ToLower(dep.DepName + " " + dep.DepSpec)
		if strings.Contains(name, "incompatible") || strings.Contains(name, "missing") || strings.Contains(name, "blocked") {
			blockers = append(blockers, fmt.Sprintf("incompatible dependency: %s%s", dep.DepName, dep.DepSpec))
		}
	}
	return blockers
}

func hasLocalModifications(validation validatedInstall) bool {
	for _, file := range validation.Items {
		if file.Kind == ValidationKindFile && (file.State == ValidationStateModified || file.State == ValidationStateUnreadable) {
			return true
		}
	}
	return false
}

func removeOwnedFiles(root string, record installer.InstallRecord) ([]string, error) {
	removed := make([]string, 0, len(record.Files))
	for rel := range record.Files {
		path := filepath.Join(record.InstallRoot, filepath.FromSlash(rel))
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, rel)
		pruneEmptyParents(filepath.Dir(path), record.InstallRoot)
	}
	pruneEmptyParents(record.InstallRoot, root)
	return removed, nil
}

func removeInstallRecord(lock *installer.ManifestLock, record installer.InstallRecord) bool {
	if lock.RemoveInstall(record.InstallID) {
		return true
	}
	for i := range lock.Installs {
		current := lock.Installs[i]
		if current.Package == record.Package && current.InstallScope == record.InstallScope {
			lock.Installs = append(lock.Installs[:i], lock.Installs[i+1:]...)
			return true
		}
	}
	return false
}

func rollbackInstallSummary(repoRoot string, summary installer.Summary) error {
	stateRoot, err := stateRootForScope(repoRoot, summary.Scope)
	if err != nil {
		return err
	}
	errs := make([]error, 0, 3)
	for _, file := range summary.Files {
		path := filepath.FromSlash(file.Path)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("removing %s: %w", file.Path, err))
			continue
		}
		pruneEmptyParents(filepath.Dir(path), summary.InstallRoot)
	}
	pruneEmptyParents(filepath.FromSlash(summary.InstallRoot), stateRoot)
	if err := installer.WithManifestLock(stateRoot, func(lock *installer.ManifestLock) error {
		lock.RemoveInstall(fmt.Sprintf("pkg_%s_%s", summary.PackageID, summary.Scope))
		return nil
	}); err != nil {
		errs = append(errs, err)
	}
	if err := installer.WithHookRegistry(stateRoot, func(registry *installer.HookRegistry) error {
		registry.RemovePackageHooks(summary.PackageID, summary.Scope)
		return nil
	}); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func pruneEmptyParents(path, stop string) {
	current := path
	for strings.HasPrefix(current, stop) && current != stop {
		err := os.Remove(current)
		if err != nil {
			return
		}
		current = filepath.Dir(current)
	}
}

func currentProfileSnapshot(repoRoot string) map[string]any {
	profile := map[string]any{}
	detected, err := repo.DetectProfile(repoRoot, snapshotNow())
	if err != nil {
		return profile
	}
	profile["name"] = detected.Repo.Name
	profile["primary_language"] = detected.Repo.PrimaryLanguage
	return profile
}
