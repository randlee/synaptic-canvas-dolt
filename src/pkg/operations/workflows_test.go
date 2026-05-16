package operations

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestRunInstallDryRunSuccess(t *testing.T) {
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", nil)
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "gh", DepSpec: ">=2"}})

	confirmCalls := 0
	initCalls := 0
	refreshCalls := 0
	executeCalls := 0
	result, err := RunInstall(context.Background(), InstallRequest{
		PackageID: "team-lead",
		Scope:     "project",
		Branch:    "main",
		RepoRoot:  t.TempDir(),
		DryRun:    true,
		Yolo:      true,
	}, InstallDependencies{
		Reader: mock,
		ConfirmExternalDeps: func(deps []models.PackageDep, yolo, dryRun bool) error {
			confirmCalls++
			if len(deps) != 1 || !yolo || !dryRun {
				t.Fatalf("unexpected confirm args: deps=%+v yolo=%v dryRun=%v", deps, yolo, dryRun)
			}
			return nil
		},
		InitializeRepo: func(string) error {
			initCalls++
			return nil
		},
		ExecuteInstall: func(_ context.Context, req installer.Request) (installer.Summary, error) {
			executeCalls++
			if !req.DryRun || req.Global {
				t.Fatalf("unexpected install request: %+v", req)
			}
			return installer.Summary{
				PackageID:    "team-lead",
				Version:      "1.2.0",
				Branch:       "main",
				Scope:        "project",
				InstallRoot:  filepath.Join(req.RepoRoot, ".claude", "skills", "team-lead"),
				FilesWritten: 1,
			}, nil
		},
		RefreshCatalog: func(context.Context, string, string) []string {
			refreshCalls++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}
	if !result.OK() || len(result.Summaries) != 1 || result.Summaries[0].Scope != "project" {
		t.Fatalf("unexpected install result: %+v", result)
	}
	if confirmCalls != 1 || initCalls != 0 || refreshCalls != 0 || executeCalls != 1 {
		t.Fatalf("unexpected side-effect counts confirm=%d init=%d refresh=%d execute=%d", confirmCalls, initCalls, refreshCalls, executeCalls)
	}
}

func TestRunInstallBothAggregateFailureAndRollback(t *testing.T) {
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", nil)
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill}})

	executeCalls := 0
	rolledBack := []string{}
	result, err := RunInstall(context.Background(), InstallRequest{
		PackageID: "team-lead",
		Scope:     "both",
		Branch:    "main",
		RepoRoot:  t.TempDir(),
		Yolo:      true,
	}, InstallDependencies{
		Reader:         mock,
		InitializeRepo: func(string) error { return nil },
		ExecuteInstall: func(_ context.Context, req installer.Request) (installer.Summary, error) {
			executeCalls++
			if executeCalls == 1 {
				return installer.Summary{PackageID: "team-lead", Version: "1.2.0", Branch: "main", Scope: "project", InstallRoot: req.RepoRoot}, nil
			}
			return installer.Summary{}, errors.New("incompatible dependency: missing tool")
		},
		RollbackInstall: func(_ string, summary installer.Summary) error {
			rolledBack = append(rolledBack, summary.Scope)
			return nil
		},
		ClassifyError: func(err error, operation string) api.Error {
			if operation != "install_scope" {
				t.Fatalf("unexpected operation = %q", operation)
			}
			return api.NewError(api.ErrorCodeBlocked, err.Error())
		},
	})
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}
	if result.OK() || result.ErrorMessage() != "install failed for all selected scopes" {
		t.Fatalf("unexpected aggregate result: %+v", result)
	}
	if len(result.Failures) != 1 || result.Failures[0].Code != api.ErrorCodeBlocked {
		t.Fatalf("expected blocked failure, got %+v", result.Failures)
	}
	if len(result.RolledBack) != 1 || strings.Join(rolledBack, ",") != "project" {
		t.Fatalf("expected project rollback, got rolledBack=%+v records=%+v", rolledBack, result.RolledBack)
	}
	if len(result.Summaries) != 0 {
		t.Fatalf("expected summaries cleared after rollback, got %+v", result.Summaries)
	}
}

func TestRunInstallLocalOnlyGlobalInvalidArgs(t *testing.T) {
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("local-only", "local-only", "1.0.0", nil)
	pkg.InstallScope = models.InstallScope("local-only")
	mock.AddPackage(pkg)
	_, err := RunInstall(context.Background(), InstallRequest{
		PackageID: "local-only",
		Scope:     "global",
		RepoRoot:  t.TempDir(),
	}, InstallDependencies{
		Reader:         mock,
		ExecuteInstall: func(context.Context, installer.Request) (installer.Summary, error) { return installer.Summary{}, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be installed globally") {
		t.Fatalf("expected local-only invalid args error, got %v", err)
	}
}

func TestRunInstallReaderOperationWrap(t *testing.T) {
	reader := fakeInstallReader{
		getPackage: func(context.Context, string) (*models.Package, error) {
			pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.0.0", nil)
			return pkg, nil
		},
		getPackageFiles: func(context.Context, string) ([]models.PackageFile, error) {
			return nil, errors.New("backend unavailable")
		},
	}
	_, err := RunInstall(context.Background(), InstallRequest{
		PackageID: "team-lead",
		Scope:     "project",
		RepoRoot:  t.TempDir(),
	}, InstallDependencies{
		Reader:         reader,
		ExecuteInstall: func(context.Context, installer.Request) (installer.Summary, error) { return installer.Summary{}, nil },
	})
	if err == nil || OperationName(err) != "get_package_files" {
		t.Fatalf("expected get_package_files operation error, got %v", err)
	}
}

func TestRunInstallResolvesRepoAndRefreshesCatalog(t *testing.T) {
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.0.0", nil)
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill}})

	root := t.TempDir()
	initCalls := 0
	refreshCalls := 0
	result, err := RunInstall(context.Background(), InstallRequest{
		PackageID: "team-lead",
		Scope:     "project",
		Branch:    "beta",
		Yolo:      true,
	}, InstallDependencies{
		Reader:          mock,
		ResolveRepoRoot: func() (string, error) { return root, nil },
		InitializeRepo: func(got string) error {
			initCalls++
			if got != root {
				t.Fatalf("InitializeRepo root = %q, want %q", got, root)
			}
			return nil
		},
		ExecuteInstall: func(_ context.Context, req installer.Request) (installer.Summary, error) {
			if req.RepoRoot != root || req.Branch != "beta" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return installer.Summary{PackageID: "team-lead", Version: "1.0.0", Branch: "beta", Scope: "project", InstallRoot: filepath.Join(root, ".claude", "skills", "team-lead")}, nil
		},
		RefreshCatalog: func(_ context.Context, gotRoot, branch string) []string {
			refreshCalls++
			if gotRoot != root || branch != "beta" {
				t.Fatalf("unexpected refresh args root=%q branch=%q", gotRoot, branch)
			}
			return []string{"catalog warning"}
		},
	})
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}
	if initCalls != 1 || refreshCalls != 1 || !containsWarning(result.Warnings, "catalog warning") {
		t.Fatalf("unexpected repo/init result: init=%d refresh=%d result=%+v", initCalls, refreshCalls, result)
	}
}

func TestRunInstallInvalidScopeAndConfirmationError(t *testing.T) {
	if _, err := RunInstall(context.Background(), InstallRequest{PackageID: "pkg", Scope: "invalid"}, InstallDependencies{}); err == nil || !strings.Contains(err.Error(), "invalid --scope") {
		t.Fatalf("expected invalid scope error, got %v", err)
	}

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.0.0", nil)
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "gh", DepSpec: ">=2"}})
	_, err := RunInstall(context.Background(), InstallRequest{
		PackageID: "team-lead",
		Scope:     "project",
		RepoRoot:  t.TempDir(),
	}, InstallDependencies{
		Reader: mock,
		ConfirmExternalDeps: func([]models.PackageDep, bool, bool) error {
			return errors.New("interactive confirmation required; use --yolo to proceed non-interactively")
		},
		ExecuteInstall: func(context.Context, installer.Request) (installer.Summary, error) { return installer.Summary{}, nil },
	})
	if err == nil || OperationName(err) != "confirm_external_dependencies" {
		t.Fatalf("expected confirm_external_dependencies operation error, got %v", err)
	}
}

func TestRunUpgradeSuccessAndWarnings(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	if err := os.MkdirAll(filepath.Dir(installRoot), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	record := installer.InstallRecord{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		Variant:      "claude",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		QuestionSnapshot: installer.QuestionSnapshot{
			QuestionIDs: []string{"style"},
		},
		Requirements: installer.RequirementSnapshot{Tools: []string{"gh>=2"}},
		RepoProfile:  map[string]any{"name": "before", "primary_language": "go"},
	}
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "2.0.0", nil)
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "SKILL.md", Content: "new", SHA256: testSHA("new"), FileType: models.FileTypeSkill}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "gh", DepSpec: ">=2"}})
	mock.AddQuestions("team-lead", []models.PackageQuestion{{QuestionID: "style"}, {QuestionID: "new-q"}})

	opened := []string{}
	result, err := RunUpgrade(context.Background(), UpgradeRequest{
		PackageID:       "team-lead",
		Scope:           "project",
		Yolo:            true,
		EffectiveBranch: "beta",
		BranchExplicit:  true,
		RepoRoot:        root,
	}, UpgradeDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) {
			return []TrackedInstall{{Record: record}}, nil
		},
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{Items: []api.ValidationItem{{Kind: api.ValidationKindFile, State: api.ValidationStateModified}}}, nil
		},
		OpenClient: func(branch string) (UpgradeClient, error) {
			opened = append(opened, branch)
			return mock, nil
		},
		ConfirmExternalDeps: func(deps []models.PackageDep, yolo bool) error {
			if !yolo || len(deps) != 1 {
				t.Fatalf("unexpected deps confirmation args: %+v yolo=%v", deps, yolo)
			}
			return nil
		},
		ExecuteInstall: func(_ context.Context, req installer.Request) (installer.Summary, error) {
			return installer.Summary{InstallRoot: req.RepoRoot, FilesWritten: 1, DependencyWarnings: []string{"dep"}, TemplateValidationWarnings: []string{"template"}}, nil
		},
		ProfileSnapshot: func(string) map[string]any {
			return map[string]any{"name": "after", "primary_language": "go"}
		},
	})
	if err != nil {
		t.Fatalf("RunUpgrade() error = %v", err)
	}
	if !result.Response.OK || len(result.Response.Upgrades) != 1 {
		t.Fatalf("unexpected upgrade result: %+v", result)
	}
	upgrade := result.Response.Upgrades[0]
	if upgrade.ToBranch != "beta" || upgrade.ToVersion != "2.0.0" {
		t.Fatalf("unexpected upgrade payload: %+v", upgrade)
	}
	if !containsWarning(upgrade.Warnings, "local modifications") || !containsWarning(upgrade.Warnings, "new question detected") || !containsWarning(upgrade.Warnings, "repo profile changed") {
		t.Fatalf("expected workflow warnings, got %+v", upgrade.Warnings)
	}
	if strings.Join(opened, ",") != "beta" {
		t.Fatalf("unexpected opened branches: %+v", opened)
	}
}

func TestRunUpgradeAllBlockedDependencySkipped(t *testing.T) {
	record := installer.InstallRecord{
		InstallID:    "pkg_blocked_project",
		Package:      "blocked",
		Version:      "1.0.0",
		Branch:       "main",
		Variant:      "claude",
		InstallScope: "project",
		InstallRoot:  t.TempDir(),
	}
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("blocked", "blocked", "2.0.0", nil)
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("blocked", []models.PackageFile{{DestPath: "SKILL.md", Content: "new", SHA256: testSHA("new"), FileType: models.FileTypeSkill}})
	mock.AddDeps("blocked", []models.PackageDep{{DepType: models.DepTypeTool, DepName: "missing-tool", DepSpec: ">=1"}})

	result, err := RunUpgrade(context.Background(), UpgradeRequest{
		UpgradeAll:      true,
		Scope:           "project",
		EffectiveBranch: "main",
		RepoRoot:        t.TempDir(),
	}, UpgradeDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return []TrackedInstall{{Record: record}}, nil },
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{}, nil
		},
		OpenClient: func(string) (UpgradeClient, error) { return mock, nil },
		ExecuteInstall: func(context.Context, installer.Request) (installer.Summary, error) {
			t.Fatal("ExecuteInstall should not be called")
			return installer.Summary{}, nil
		},
		ConfirmExternalDeps: func([]models.PackageDep, bool) error { return nil },
		ProfileSnapshot:     func(string) map[string]any { return map[string]any{} },
	})
	if err != nil {
		t.Fatalf("RunUpgrade() error = %v", err)
	}
	if result.Response.OK || len(result.Response.Upgrades) != 1 || !result.Response.Upgrades[0].Skipped {
		t.Fatalf("unexpected blocked upgrade result: %+v", result)
	}
	if !containsWarning(result.Response.Upgrades[0].Warnings, "incompatible dependency") {
		t.Fatalf("expected dependency blocker warning, got %+v", result.Response.Upgrades[0].Warnings)
	}
}

func TestRunUpgradeForceAllRejected(t *testing.T) {
	_, err := RunUpgrade(context.Background(), UpgradeRequest{
		UpgradeAll: true,
		Force:      true,
		Scope:      "project",
	}, UpgradeDependencies{})
	if err == nil || !strings.Contains(err.Error(), "cannot be used with --all") {
		t.Fatalf("expected invalid force/all error, got %v", err)
	}
}

func TestRunUpgradeRequestedVersionSkipped(t *testing.T) {
	record := installer.InstallRecord{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		Variant:      "claude",
		InstallScope: "project",
		InstallRoot:  t.TempDir(),
	}
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "2.0.0", nil)
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "SKILL.md", Content: "new", SHA256: testSHA("new"), FileType: models.FileTypeSkill}})

	result, err := RunUpgrade(context.Background(), UpgradeRequest{
		PackageID:       "team-lead",
		Scope:           "project",
		TargetVersion:   "9.9.9",
		EffectiveBranch: "main",
		RepoRoot:        t.TempDir(),
	}, UpgradeDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return []TrackedInstall{{Record: record}}, nil },
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{}, nil
		},
		OpenClient:          func(string) (UpgradeClient, error) { return mock, nil },
		ConfirmExternalDeps: func([]models.PackageDep, bool) error { return nil },
		ExecuteInstall: func(context.Context, installer.Request) (installer.Summary, error) {
			t.Fatal("ExecuteInstall should not be called")
			return installer.Summary{}, nil
		},
		ProfileSnapshot: func(string) map[string]any { return map[string]any{} },
	})
	if err != nil {
		t.Fatalf("RunUpgrade() error = %v", err)
	}
	if len(result.Response.Upgrades) != 1 || !result.Response.Upgrades[0].Skipped || !containsWarning(result.Response.Upgrades[0].Warnings, "requested version") {
		t.Fatalf("expected requested-version skip, got %+v", result)
	}
}

func TestRunUpgradeAlreadyLatestSkipped(t *testing.T) {
	record := installer.InstallRecord{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "2.0.0",
		Branch:       "beta",
		Variant:      "claude",
		InstallScope: "project",
		InstallRoot:  t.TempDir(),
	}
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "2.0.0", nil)
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "SKILL.md", Content: "new", SHA256: testSHA("new"), FileType: models.FileTypeSkill}})

	result, err := RunUpgrade(context.Background(), UpgradeRequest{
		PackageID:       "team-lead",
		Scope:           "project",
		EffectiveBranch: "main",
		RepoRoot:        t.TempDir(),
	}, UpgradeDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return []TrackedInstall{{Record: record}}, nil },
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{}, nil
		},
		OpenClient:          func(string) (UpgradeClient, error) { return mock, nil },
		ConfirmExternalDeps: func([]models.PackageDep, bool) error { return nil },
		ExecuteInstall: func(context.Context, installer.Request) (installer.Summary, error) {
			t.Fatal("ExecuteInstall should not be called")
			return installer.Summary{}, nil
		},
		ProfileSnapshot: func(string) map[string]any { return map[string]any{} },
	})
	if err != nil {
		t.Fatalf("RunUpgrade() error = %v", err)
	}
	if len(result.Response.Upgrades) != 1 || !result.Response.Upgrades[0].Skipped || !containsWarning(result.Response.Upgrades[0].Warnings, "already on latest version") {
		t.Fatalf("expected already-latest skip, got %+v", result)
	}
}

func TestRunUpgradePackageMissingAndOpenClientError(t *testing.T) {
	_, err := RunUpgrade(context.Background(), UpgradeRequest{
		PackageID: "missing",
		Scope:     "project",
		RepoRoot:  t.TempDir(),
	}, UpgradeDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return nil, nil },
	})
	if err == nil || !strings.Contains(err.Error(), `package "missing" is not installed`) {
		t.Fatalf("expected missing package error, got %v", err)
	}

	record := installer.InstallRecord{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  t.TempDir(),
	}
	_, err = RunUpgrade(context.Background(), UpgradeRequest{
		PackageID:       "team-lead",
		Scope:           "project",
		EffectiveBranch: "main",
		RepoRoot:        t.TempDir(),
	}, UpgradeDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return []TrackedInstall{{Record: record}}, nil },
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{}, nil
		},
		OpenClient: func(string) (UpgradeClient, error) {
			return nil, errors.New("connection refused")
		},
		ExecuteInstall:  func(context.Context, installer.Request) (installer.Summary, error) { return installer.Summary{}, nil },
		ProfileSnapshot: func(string) map[string]any { return map[string]any{} },
	})
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("expected open client error, got %v", err)
	}
}

func TestUpgradeBranchForTarget(t *testing.T) {
	if got := UpgradeBranchForTarget("develop", true, installer.InstallRecord{Branch: "beta"}); got != "develop" {
		t.Fatalf("branch explicit result = %q, want develop", got)
	}
	if got := UpgradeBranchForTarget("develop", false, installer.InstallRecord{Branch: "beta"}); got != "beta" {
		t.Fatalf("tracked branch result = %q, want beta", got)
	}
	if got := UpgradeBranchForTarget("develop", false, installer.InstallRecord{}); got != "develop" {
		t.Fatalf("default branch result = %q, want develop", got)
	}
	if got := UpgradeBranchForTarget("", false, installer.InstallRecord{}); got != "main" {
		t.Fatalf("fallback branch result = %q, want main", got)
	}
}

func TestRunUninstallSuccessAndDependencyProvenance(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeFile(t, filepath.Join(installRoot, "SKILL.md"), "skill")
	record := installer.InstallRecord{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("skill"))},
		Requirements: installer.RequirementSnapshot{
			CLIInstalled: []string{"owned", "preexisting"},
			CLIProvenance: map[string]string{
				"owned":       "installed-by-synaptic",
				"preexisting": "already-present",
			},
		},
	}
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{record}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	if err := installer.SaveHookRegistry(root, installer.HookRegistry{Hooks: []installer.HookEntry{{Skill: "team-lead", Scope: "project", Script: "hook.sh"}}}); err != nil {
		t.Fatalf("SaveHookRegistry() error = %v", err)
	}

	removedDeps := []string{}
	resp, err := RunUninstall(context.Background(), UninstallRequest{
		PackageID: "team-lead",
		Scope:     "project",
		RepoRoot:  root,
		Yolo:      true,
	}, UninstallDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return []TrackedInstall{{Record: record}}, nil },
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{}, nil
		},
		RemoveSCDependency: func(dep string) error {
			removedDeps = append(removedDeps, dep)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunUninstall() error = %v", err)
	}
	if !resp.OK || len(resp.RemovedAll) != 1 {
		t.Fatalf("unexpected uninstall response: %+v", resp)
	}
	if strings.Join(removedDeps, ",") != "owned" {
		t.Fatalf("expected only owned dependency removed, got %+v", removedDeps)
	}
	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(lock.Installs) != 0 {
		t.Fatalf("expected manifest cleared, got %+v", lock.Installs)
	}
}

func TestRunUninstallPreservesManifestOnRemoveFailure(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	blockingDir := filepath.Join(installRoot, "SKILL.md")
	writeFile(t, filepath.Join(blockingDir, "child.txt"), "block")
	record := installer.InstallRecord{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("skill"))},
	}
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{record}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	_, err := RunUninstall(context.Background(), UninstallRequest{
		PackageID: "team-lead",
		Scope:     "project",
		RepoRoot:  root,
		Force:     true,
	}, UninstallDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return []TrackedInstall{{Record: record}}, nil },
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "manifest record preserved") {
		t.Fatalf("expected manifest-preserved error, got %v", err)
	}
	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(lock.Installs) != 1 {
		t.Fatalf("expected manifest record preserved, got %+v", lock.Installs)
	}
}

func TestRunUninstallLocalModificationNeedsConfirmation(t *testing.T) {
	record := installer.InstallRecord{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  t.TempDir(),
		InstallSite:  t.TempDir(),
	}
	_, err := RunUninstall(context.Background(), UninstallRequest{
		PackageID: "team-lead",
		Scope:     "project",
		RepoRoot:  t.TempDir(),
	}, UninstallDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return []TrackedInstall{{Record: record}}, nil },
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{Items: []api.ValidationItem{{Kind: api.ValidationKindFile, State: api.ValidationStateModified}}}, nil
		},
		ConfirmProceed: func(prompt, nonInteractive string, yolo, force bool) error {
			if !strings.Contains(prompt, "Proceed anyway") || yolo || force {
				t.Fatalf("unexpected confirm args: prompt=%q yolo=%v force=%v", prompt, yolo, force)
			}
			return errors.New(nonInteractive)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "locally modified files detected") {
		t.Fatalf("expected local-modification error, got %v", err)
	}
}

func TestRunUninstallWarnsWhenDependencyRemovalSkippedNonInteractive(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeFile(t, filepath.Join(installRoot, "SKILL.md"), "skill")
	record := installer.InstallRecord{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("skill"))},
		Requirements: installer.RequirementSnapshot{
			CLIInstalled: []string{"owned"},
			CLIProvenance: map[string]string{
				"owned": "installed-by-synaptic",
			},
		},
	}
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{record}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	resp, err := RunUninstall(context.Background(), UninstallRequest{
		PackageID: "team-lead",
		Scope:     "project",
		RepoRoot:  root,
	}, UninstallDependencies{
		LoadInstalls: func(string) ([]TrackedInstall, error) { return []TrackedInstall{{Record: record}}, nil },
		ValidateTrackedInstall: func(context.Context, installer.InstallRecord) (api.ValidatedInstall, error) {
			return api.ValidatedInstall{}, nil
		},
		ConfirmRemoveDependency: func(string) (bool, error) { return false, nil },
		CommandInputTTY:         func() bool { return false },
	})
	if err != nil {
		t.Fatalf("RunUninstall() error = %v", err)
	}
	if len(resp.RemovedAll) != 1 || !containsWarning(resp.RemovedAll[0].Warnings, "skipped sc-installed dependency removal") {
		t.Fatalf("expected skipped dependency warning, got %+v", resp)
	}
}

func TestOperationNameRoundTrip(t *testing.T) {
	err := WrapOperation("get_package", errors.New("boom"))
	if got := OperationName(err); got != "get_package" {
		t.Fatalf("OperationName() = %q, want get_package", got)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func testSHA(input string) string {
	hash := integrity.ComputeContentSHA256([]byte(input))
	return hash
}

func containsWarning(values []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

type fakeInstallReader struct {
	getPackage          func(context.Context, string) (*models.Package, error)
	getPackageFiles     func(context.Context, string) ([]models.PackageFile, error)
	getPackageDeps      func(context.Context, string) ([]models.PackageDep, error)
	getPackageHooks     func(context.Context, string) ([]models.PackageHook, error)
	getPackageQuestions func(context.Context, string) ([]models.PackageQuestion, error)
}

func (f fakeInstallReader) GetPackage(ctx context.Context, id string) (*models.Package, error) {
	if f.getPackage == nil {
		return nil, nil
	}
	return f.getPackage(ctx, id)
}

func (f fakeInstallReader) GetPackageFiles(ctx context.Context, id string) ([]models.PackageFile, error) {
	if f.getPackageFiles == nil {
		return nil, nil
	}
	return f.getPackageFiles(ctx, id)
}

func (f fakeInstallReader) GetPackageDeps(ctx context.Context, id string) ([]models.PackageDep, error) {
	if f.getPackageDeps == nil {
		return nil, nil
	}
	return f.getPackageDeps(ctx, id)
}

func (f fakeInstallReader) GetPackageHooks(ctx context.Context, id string) ([]models.PackageHook, error) {
	if f.getPackageHooks == nil {
		return nil, nil
	}
	return f.getPackageHooks(ctx, id)
}

func (f fakeInstallReader) GetPackageQuestions(ctx context.Context, id string) ([]models.PackageQuestion, error) {
	if f.getPackageQuestions == nil {
		return nil, nil
	}
	return f.getPackageQuestions(ctx, id)
}

func TestInstallResultAggregateError(t *testing.T) {
	result := InstallResult{
		Failures: []api.InstallScopeFailure{api.NewInstallScopeFailure("team-lead", "project", api.NewError(api.ErrorCodeBlocked, "blocked", api.ErrorOptions{SuggestedAction: "fix"}))},
	}
	payload := result.AggregateError()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `"code":"blocked"`) {
		t.Fatalf("unexpected aggregate payload: %s", string(data))
	}
}

func TestCurrentProfileSnapshotFallback(t *testing.T) {
	result := CurrentProfileSnapshot(t.TempDir(), time.Now().UTC())
	if result == nil {
		t.Fatal("CurrentProfileSnapshot() returned nil map")
	}
}
