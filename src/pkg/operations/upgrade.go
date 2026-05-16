package operations

import (
	"context"
	"fmt"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

type UpgradeClient interface {
	UpgradePackageReader
	Close() error
}

type UpgradeRequest struct {
	PackageID       string
	UpgradeAll      bool
	Scope           string
	TargetVersion   string
	Force           bool
	Yolo            bool
	EffectiveBranch string
	BranchExplicit  bool
	RepoRoot        string
}

type UpgradeDependencies struct {
	LoadInstalls           func(string) ([]TrackedInstall, error)
	ResolveRepoRoot        func() (string, error)
	ValidateTrackedInstall func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error)
	OpenClient             func(string) (UpgradeClient, error)
	ConfirmExternalDeps    func([]models.PackageDep, bool) error
	ExecuteInstall         func(context.Context, installer.Request) (installer.Summary, error)
	ProfileSnapshot        func(string) map[string]any
	Now                    func() time.Time
}

type UpgradeRunResult struct {
	Response  api.UpgradeResponse
	Successes int
}

func RunUpgrade(ctx context.Context, req UpgradeRequest, deps UpgradeDependencies) (UpgradeRunResult, error) {
	result := UpgradeRunResult{Response: api.UpgradeResponse{OK: true, Upgrades: []api.UpgradeResult{}}}
	if err := ValidateScope(req.Scope); err != nil {
		return result, err
	}
	if req.UpgradeAll && req.Force {
		return result, fmt.Errorf("--force cannot be used with --all; target a specific package")
	}
	repoRoot := req.RepoRoot
	var err error
	if repoRoot == "" {
		if deps.ResolveRepoRoot == nil {
			return result, fmt.Errorf("repo root resolver is required")
		}
		repoRoot, err = deps.ResolveRepoRoot()
		if err != nil {
			return result, err
		}
	}
	loadInstalls := deps.LoadInstalls
	if loadInstalls == nil {
		loadInstalls = LoadTrackedInstalls
	}
	installs, err := loadInstalls(repoRoot)
	if err != nil {
		return result, err
	}
	var targets []TrackedInstall
	if req.UpgradeAll {
		targets = FilterInstallsByScope(installs, req.Scope)
	} else {
		if req.PackageID == "" {
			return result, fmt.Errorf("upgrade requires <package> or --all")
		}
		targets = SelectInstalls(installs, req.PackageID, req.Scope)
		if len(targets) == 0 {
			return result, fmt.Errorf("package %q is not installed", req.PackageID)
		}
	}
	if deps.ValidateTrackedInstall == nil {
		return result, fmt.Errorf("tracked install validator is required")
	}
	if deps.OpenClient == nil {
		return result, fmt.Errorf("upgrade client opener is required")
	}
	if deps.ExecuteInstall == nil {
		return result, fmt.Errorf("upgrade install executor is required")
	}
	profileSnapshot := deps.ProfileSnapshot
	if profileSnapshot == nil {
		now := time.Now().UTC()
		if deps.Now != nil {
			now = deps.Now().UTC()
		}
		profileSnapshot = func(repoRoot string) map[string]any {
			return CurrentProfileSnapshot(repoRoot, now)
		}
	}
	clients := map[string]UpgradeClient{}
	defer func() {
		for _, client := range clients {
			_ = client.Close()
		}
	}()
	clientForBranch := func(branch string) (UpgradeClient, error) {
		if client := clients[branch]; client != nil {
			return client, nil
		}
		client, err := deps.OpenClient(branch)
		if err != nil {
			return nil, err
		}
		clients[branch] = client
		return client, nil
	}
	results := make([]api.UpgradeResult, 0, len(targets))
	successes := 0
	for _, target := range targets {
		validation, err := deps.ValidateTrackedInstall(ctx, target.Record)
		if err != nil {
			return result, err
		}
		targetBranch := UpgradeBranchForTarget(req.EffectiveBranch, req.BranchExplicit, target.Record)
		client, err := clientForBranch(targetBranch)
		if err != nil {
			return result, err
		}
		pkg, files, depsList, hooks, questions, err := FetchUpgradePackage(ctx, client, targetBranch, target.Record)
		if err != nil {
			return result, err
		}
		if req.TargetVersion != "" && pkg.Version != req.TargetVersion {
			results = append(results, api.UpgradeResult{
				Package:     target.Record.Package,
				Scope:       target.Record.InstallScope,
				FromVersion: target.Record.Version,
				ToVersion:   target.Record.Version,
				FromBranch:  target.Record.Branch,
				ToBranch:    targetBranch,
				InstallRoot: target.Record.InstallRoot,
				Warnings:    []string{fmt.Sprintf("requested version %s not available on branch %s", req.TargetVersion, targetBranch)},
				Skipped:     true,
			})
			continue
		}
		warnings := BuildUpgradeWarnings(target.Record, validation, depsList, questions, profileSnapshot(repoRoot))
		if blockers := DependencyBlockers(depsList); len(blockers) > 0 && (req.UpgradeAll || !req.Force) {
			results = append(results, api.UpgradeResult{
				Package:     target.Record.Package,
				Scope:       target.Record.InstallScope,
				FromVersion: target.Record.Version,
				ToVersion:   target.Record.Version,
				FromBranch:  target.Record.Branch,
				ToBranch:    targetBranch,
				InstallRoot: target.Record.InstallRoot,
				Warnings:    append(warnings, append([]string{"skipped upgrade"}, blockers...)...),
				Skipped:     true,
			})
			continue
		}
		if deps.ConfirmExternalDeps != nil {
			if err := deps.ConfirmExternalDeps(depsList, req.Yolo); err != nil {
				return result, WrapOperation("confirm_external_dependencies", err)
			}
		}
		if pkg.Version == target.Record.Version && targetBranch == target.Record.Branch && req.TargetVersion == "" {
			results = append(results, api.UpgradeResult{
				Package:     target.Record.Package,
				Scope:       target.Record.InstallScope,
				FromVersion: target.Record.Version,
				ToVersion:   target.Record.Version,
				FromBranch:  target.Record.Branch,
				ToBranch:    targetBranch,
				InstallRoot: target.Record.InstallRoot,
				Warnings:    append(warnings, "already on latest version for selected branch"),
				Skipped:     true,
			})
			continue
		}
		summary, err := deps.ExecuteInstall(ctx, installer.Request{
			Package:   pkg,
			Files:     files,
			Deps:      depsList,
			Hooks:     hooks,
			Questions: questions,
			Branch:    targetBranch,
			Global:    target.Record.InstallScope == "global",
			RepoRoot:  repoRoot,
		})
		if err != nil {
			return result, WrapOperation("upgrade_install", err)
		}
		results = append(results, api.UpgradeResult{
			Package:            target.Record.Package,
			Scope:              target.Record.InstallScope,
			FromVersion:        target.Record.Version,
			ToVersion:          pkg.Version,
			FromBranch:         target.Record.Branch,
			ToBranch:           targetBranch,
			InstallRoot:        summary.InstallRoot,
			Warnings:           warnings,
			FilesWritten:       summary.FilesWritten,
			TemplateWarnings:   summary.TemplateValidationWarnings,
			DependencyWarnings: summary.DependencyWarnings,
		})
		successes++
	}
	result.Successes = successes
	result.Response = api.UpgradeResponse{OK: successes > 0 || len(results) == 0, Upgrades: results}
	return result, nil
}

func UpgradeBranchForTarget(defaultBranch string, branchExplicit bool, record installer.InstallRecord) string {
	if branchExplicit {
		return defaultBranch
	}
	if record.Branch != "" {
		return record.Branch
	}
	if defaultBranch != "" {
		return defaultBranch
	}
	return "main"
}
