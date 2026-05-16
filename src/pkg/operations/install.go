package operations

import (
	"context"
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

type InstallReader interface {
	GetPackage(context.Context, string) (*models.Package, error)
	GetPackageFiles(context.Context, string) ([]models.PackageFile, error)
	GetPackageDeps(context.Context, string) ([]models.PackageDep, error)
	GetPackageHooks(context.Context, string) ([]models.PackageHook, error)
	GetPackageQuestions(context.Context, string) ([]models.PackageQuestion, error)
}

type InstallRequest struct {
	PackageID string
	Scope     string
	Branch    string
	RepoRoot  string
	DryRun    bool
	Yolo      bool
}

type InstallDependencies struct {
	Reader              InstallReader
	ResolveRepoRoot     func() (string, error)
	ConfirmExternalDeps func([]models.PackageDep, bool, bool) error
	InitializeRepo      func(string) error
	ExecuteInstall      func(context.Context, installer.Request) (installer.Summary, error)
	RollbackInstall     func(string, installer.Summary) error
	RefreshCatalog      func(context.Context, string, string) []string
	ClassifyError       func(error, string) api.Error
}

type InstallResult struct {
	Scope      string
	Plan       bool
	Summaries  []installer.Summary
	RolledBack []installer.Summary
	Failures   []api.InstallScopeFailure
	Partial    bool
	Warnings   []string
}

func (r InstallResult) OK() bool {
	return len(r.Failures) == 0
}

func (r InstallResult) ErrorMessage() string {
	if len(r.Failures) == 0 {
		return ""
	}
	if len(r.Summaries) == 0 {
		return "install failed for all selected scopes"
	}
	return "install failed for one or more scopes"
}

func (r InstallResult) AggregateError() *api.Error {
	if len(r.Failures) == 0 {
		return nil
	}
	aggregate := AggregateInstallError(r.ErrorMessage(), r.Failures)
	return &aggregate
}

func AggregateInstallError(message string, failures []api.InstallScopeFailure) api.Error {
	dominant := DominantInstallFailure(failures)
	return api.NewError(dominant.Code, message, api.ErrorOptions{
		Retryable:       dominant.Retryable,
		Details:         dominant.Details,
		SuggestedAction: dominant.SuggestedAction,
	})
}

func DominantInstallFailure(failures []api.InstallScopeFailure) api.InstallScopeFailure {
	for _, failure := range failures {
		if failure.Code != "" && failure.Code != api.ErrorCodeInternal {
			return failure
		}
	}
	if len(failures) > 0 {
		return failures[0]
	}
	return api.InstallScopeFailure{
		Code:      api.ErrorCodeInternal,
		Error:     "install failed",
		Retryable: false,
	}
}

func RunInstall(ctx context.Context, req InstallRequest, deps InstallDependencies) (InstallResult, error) {
	result := InstallResult{Scope: req.Scope, Plan: req.DryRun}
	scopes, err := ScopesFromFlag(req.Scope)
	if err != nil {
		return result, err
	}
	root := req.RepoRoot
	if root == "" {
		if deps.ResolveRepoRoot == nil {
			return result, fmt.Errorf("repo root resolver is required")
		}
		root, err = deps.ResolveRepoRoot()
		if err != nil {
			return result, err
		}
	}
	if deps.Reader == nil {
		return result, fmt.Errorf("install reader is required")
	}
	if deps.ExecuteInstall == nil {
		return result, fmt.Errorf("install executor is required")
	}
	rollbackInstall := deps.RollbackInstall
	if rollbackInstall == nil {
		rollbackInstall = RollbackInstallSummary
	}
	pkg, err := deps.Reader.GetPackage(ctx, req.PackageID)
	if err != nil {
		return result, WrapOperation("get_package", err)
	}
	if pkg == nil {
		return result, fmt.Errorf("package %q not found", req.PackageID)
	}
	if string(pkg.InstallScope) == "local-only" && (req.Scope == "global" || req.Scope == "both") {
		return result, fmt.Errorf("package %s is local-only and cannot be installed globally", pkg.ID)
	}
	files, err := deps.Reader.GetPackageFiles(ctx, req.PackageID)
	if err != nil {
		return result, WrapOperation("get_package_files", err)
	}
	depsList, err := deps.Reader.GetPackageDeps(ctx, req.PackageID)
	if err != nil {
		return result, WrapOperation("get_package_deps", err)
	}
	hooks, err := deps.Reader.GetPackageHooks(ctx, req.PackageID)
	if err != nil {
		return result, WrapOperation("get_package_hooks", err)
	}
	questions, err := deps.Reader.GetPackageQuestions(ctx, req.PackageID)
	if err != nil {
		return result, WrapOperation("get_package_questions", err)
	}
	if deps.ConfirmExternalDeps != nil {
		if err := deps.ConfirmExternalDeps(depsList, req.Yolo, req.DryRun); err != nil {
			return result, WrapOperation("confirm_external_dependencies", err)
		}
	}
	if !req.DryRun && deps.InitializeRepo != nil {
		if err := deps.InitializeRepo(root); err != nil {
			return result, err
		}
	}
	result.Summaries = make([]installer.Summary, 0, len(scopes))
	result.Failures = make([]api.InstallScopeFailure, 0, len(scopes))
	result.RolledBack = make([]installer.Summary, 0, len(scopes))
	for _, targetScope := range scopes {
		summary, err := deps.ExecuteInstall(ctx, installer.Request{
			Package:   pkg,
			Files:     files,
			Deps:      depsList,
			Hooks:     hooks,
			Questions: questions,
			Branch:    req.Branch,
			Global:    targetScope == "global",
			DryRun:    req.DryRun,
			RepoRoot:  root,
		})
		if err != nil {
			if req.Scope == "both" {
				failureErr := api.NewError(api.ErrorCodeInternal, err.Error())
				if deps.ClassifyError != nil {
					failureErr = deps.ClassifyError(err, "install_scope")
				}
				result.Failures = append(result.Failures, api.NewInstallScopeFailure(pkg.ID, targetScope, failureErr))
				remaining := result.Summaries[:0]
				for _, prior := range result.Summaries {
					if rollbackErr := rollbackInstall(root, prior); rollbackErr != nil {
						result.Partial = true
						result.Failures = append(result.Failures, api.NewInstallScopeFailure(
							prior.PackageID,
							prior.Scope,
							api.NewError(api.ErrorCodeInternal, "rollback failed: "+rollbackErr.Error()),
						))
						remaining = append(remaining, prior)
						continue
					}
					result.RolledBack = append(result.RolledBack, prior)
				}
				result.Summaries = remaining
				break
			}
			return result, WrapOperation("install_scope", err)
		}
		result.Summaries = append(result.Summaries, summary)
	}
	if len(result.Summaries) == 0 && len(result.Failures) == 0 {
		return result, fmt.Errorf("no install scopes were eligible for package %q", req.PackageID)
	}
	if !req.DryRun && deps.RefreshCatalog != nil {
		result.Warnings = deps.RefreshCatalog(ctx, root, req.Branch)
	}
	return result, nil
}
