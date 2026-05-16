package operations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/repo"
)

func ValidateScope(scope string) error {
	switch scope {
	case "project", "global", "both":
		return nil
	default:
		return fmt.Errorf("invalid --scope %q; expected project, global, or both", scope)
	}
}

func ScopesFromFlag(scope string) ([]string, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}
	if scope == "both" {
		return []string{"project", "global"}, nil
	}
	return []string{scope}, nil
}

func StateRootForScope(repoRoot, scope string) (string, error) {
	if scope == "global" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home dir: %w", err)
		}
		return home, nil
	}
	return repoRoot, nil
}

func SelectInstalls(installs []TrackedInstall, packageID, scope string) []TrackedInstall {
	var matches []TrackedInstall
	for _, install := range installs {
		if install.Record.Package == packageID && (scope == "both" || install.Record.InstallScope == scope) {
			matches = append(matches, install)
		}
	}
	return matches
}

func HasLocalModifications(validation api.ValidatedInstall) bool {
	for _, file := range validation.Items {
		if file.Kind == api.ValidationKindFile && (file.State == api.ValidationStateModified || file.State == api.ValidationStateUnreadable) {
			return true
		}
	}
	return false
}

func BuildUpgradeWarnings(install installer.InstallRecord, validation api.ValidatedInstall, deps []models.PackageDep, questions []models.PackageQuestion, profile map[string]any) []string {
	warnings := []string{}
	if HasLocalModifications(validation) {
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

func DependencyBlockers(deps []models.PackageDep) []string {
	blockers := []string{}
	for _, dep := range deps {
		name := strings.ToLower(dep.DepName + " " + dep.DepSpec)
		if strings.Contains(name, "incompatible") || strings.Contains(name, "missing") || strings.Contains(name, "blocked") {
			blockers = append(blockers, fmt.Sprintf("incompatible dependency: %s%s", dep.DepName, dep.DepSpec))
		}
	}
	return blockers
}

func CurrentProfileSnapshot(repoRoot string, now time.Time) map[string]any {
	profile := map[string]any{}
	detected, err := repo.DetectProfile(repoRoot, now)
	if err != nil {
		return profile
	}
	profile["name"] = detected.Repo.Name
	profile["primary_language"] = detected.Repo.PrimaryLanguage
	return profile
}

func RemoveOwnedFiles(root string, record installer.InstallRecord) ([]string, []string) {
	removed := make([]string, 0, len(record.Files))
	failed := make([]string, 0)
	paths := make([]string, 0, len(record.Files))
	for rel := range record.Files {
		paths = append(paths, rel)
	}
	slices.Sort(paths)
	for _, rel := range paths {
		path := filepath.Join(record.InstallRoot, filepath.FromSlash(rel))
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			failed = append(failed, rel)
			continue
		}
		removed = append(removed, rel)
		PruneEmptyParents(filepath.Dir(path), record.InstallRoot)
	}
	if len(failed) == 0 {
		PruneEmptyParents(record.InstallRoot, root)
	}
	return removed, failed
}

func RemoveInstallRecord(lock *installer.ManifestLock, record installer.InstallRecord) bool {
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

func RollbackInstallSummary(repoRoot string, summary installer.Summary) error {
	stateRoot, err := StateRootForScope(repoRoot, summary.Scope)
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
		PruneEmptyParents(filepath.Dir(path), summary.InstallRoot)
	}
	PruneEmptyParents(filepath.FromSlash(summary.InstallRoot), stateRoot)
	if err := installer.WithManifestLock(stateRoot, func(lock *installer.ManifestLock) error {
		lock.RemoveInstall(fmt.Sprintf(installer.InstallIDFormat, summary.PackageID, summary.Scope))
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

func PruneEmptyParents(path, stop string) {
	current := path
	for strings.HasPrefix(current, stop) && current != stop {
		err := os.Remove(current)
		if err != nil {
			return
		}
		current = filepath.Dir(current)
	}
}

type UpgradePackageReader interface {
	GetPackage(context.Context, string) (*models.Package, error)
	GetPackageFiles(context.Context, string) ([]models.PackageFile, error)
	GetPackageDeps(context.Context, string) ([]models.PackageDep, error)
	GetPackageHooks(context.Context, string) ([]models.PackageHook, error)
	GetPackageQuestions(context.Context, string) ([]models.PackageQuestion, error)
	ResolveVariant(context.Context, string, string) (string, error)
}

func FetchUpgradePackage(ctx context.Context, client UpgradePackageReader, branch string, install installer.InstallRecord) (*models.Package, []models.PackageFile, []models.PackageDep, []models.PackageHook, []models.PackageQuestion, error) {
	targetID := install.Package
	if variantID, err := client.ResolveVariant(ctx, install.Package, install.Variant); err != nil {
		return nil, nil, nil, nil, nil, WrapOperation("resolve_variant", err)
	} else if variantID != "" {
		targetID = variantID
	}

	pkg, err := client.GetPackage(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, WrapOperation("get_package", err)
	}
	if pkg == nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("package %q not found on branch %s", targetID, branch)
	}
	files, err := client.GetPackageFiles(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, WrapOperation("get_package_files", err)
	}
	deps, err := client.GetPackageDeps(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, WrapOperation("get_package_deps", err)
	}
	hooks, err := client.GetPackageHooks(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, WrapOperation("get_package_hooks", err)
	}
	questions, err := client.GetPackageQuestions(ctx, targetID)
	if err != nil {
		return nil, nil, nil, nil, nil, WrapOperation("get_package_questions", err)
	}
	return pkg, files, deps, hooks, questions, nil
}
