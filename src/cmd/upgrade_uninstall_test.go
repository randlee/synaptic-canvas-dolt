package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestJSONUpgradeCommand(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "old")
	lock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.0.0",
			Branch:       "main",
			Variant:      "claude",
			InstallScope: "project",
			InstallRoot:  installRoot,
			InstallSite:  root,
			Files: map[string]string{
				"SKILL.md": integrity.ComputeContentSHA256([]byte("old")),
			},
			QuestionSnapshot: installer.QuestionSnapshot{QuestionIDs: []string{"style"}},
			Requirements:     installer.RequirementSnapshot{Tools: []string{"gh>=2"}},
			RepoProfile:      map[string]any{"name": filepath.Base(root), "primary_language": "go"},
		}},
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "2.0.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "new", SHA256: integrity.ComputeContentSHA256([]byte("new")), FileType: models.FileTypeSkill,
	}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "gh", DepSpec: ">=2"}})
	mock.AddQuestions("team-lead", []models.PackageQuestion{{QuestionID: "style"}, {QuestionID: "new-q"}})

	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) { return mock, nil }
	defer func() {
		readClientOpener = prevOpener
	}()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "team-lead", "--branch", "beta", "--json", "--yolo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp upgradeResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if len(resp.Upgrades) != 1 {
		t.Fatalf("expected 1 upgrade result, got %+v", resp)
	}
	got := resp.Upgrades[0]
	if got.FromVersion != "1.0.0" || got.ToVersion != "2.0.0" || got.ToBranch != "beta" {
		t.Fatalf("unexpected upgrade result: %+v", got)
	}
	if len(got.Warnings) == 0 {
		t.Fatalf("expected warnings for new questions/profile drift, got %+v", got)
	}
	if data, err := os.ReadFile(filepath.Join(installRoot, "SKILL.md")); err != nil || string(data) != "new" { //nolint:gosec // test reads a temp file under a controlled install root.
		t.Fatalf("expected upgraded file content, got %q err=%v", string(data), err)
	}
}

func TestUpgradeWithoutBranchUsesTrackedBranch(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "old")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "beta",
		Variant:      "claude",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("old"))},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "2.0.0", nil)
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "new", SHA256: integrity.ComputeContentSHA256([]byte("new")), FileType: models.FileTypeSkill,
	}})
	openedBranches := []string{}
	prevOpener := readClientOpener
	readClientOpener = func(cfg *config.Config) (readClient, error) {
		openedBranches = append(openedBranches, cfg.EffectiveBranch())
		return mock, nil
	}
	defer func() { readClientOpener = prevOpener }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "team-lead", "--json", "--yolo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp upgradeResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if len(resp.Upgrades) != 1 || resp.Upgrades[0].ToBranch != "beta" {
		t.Fatalf("expected upgrade to stay on beta branch, got %+v", resp.Upgrades)
	}
	if len(openedBranches) == 0 {
		t.Fatal("read client was not opened")
	}
	for _, branch := range openedBranches {
		if branch != "beta" {
			t.Fatalf("read client opened branches = %+v, want only beta", openedBranches)
		}
	}
}

func TestUpgradeCommandWarnsOnLocalModification(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "locally modified")
	lock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.0.0",
			Branch:       "main",
			Variant:      "claude",
			InstallScope: "project",
			InstallRoot:  installRoot,
			InstallSite:  root,
			Files: map[string]string{
				"SKILL.md": integrity.ComputeContentSHA256([]byte("original")),
			},
			QuestionSnapshot: installer.QuestionSnapshot{QuestionIDs: []string{"style"}},
			Requirements:     installer.RequirementSnapshot{Tools: []string{"gh>=2"}},
			RepoProfile:      map[string]any{"name": filepath.Base(root), "primary_language": "go"},
		}},
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "2.0.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "new", SHA256: integrity.ComputeContentSHA256([]byte("new")), FileType: models.FileTypeSkill,
	}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "gh", DepSpec: ">=2"}})
	mock.AddQuestions("team-lead", []models.PackageQuestion{{QuestionID: "style"}})

	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) { return mock, nil }
	defer func() {
		readClientOpener = prevOpener
	}()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "team-lead", "--json", "--yolo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp upgradeResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if len(resp.Upgrades) != 1 {
		t.Fatalf("expected 1 upgrade result, got %+v", resp)
	}
	if !containsCI(resp.Upgrades[0].Warnings, "local modification") {
		t.Fatalf("expected local modification warning, got %+v", resp.Upgrades[0].Warnings)
	}
}

func TestJSONUpgradeAllForceRejected(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--all", "--force", "--json"})
	requireJSONCmdError(t, cmd.Execute())
	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != "invalid_args" || resp.Error.Message != "--force cannot be used with --all; target a specific package" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestUpgradeScopeProjectWithOnlyGlobalInstallReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	globalRoot := filepath.Join(home, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(globalRoot, "SKILL.md"), "global")
	if err := installer.SaveManifestLock(home, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_global",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "global",
		InstallRoot:  globalRoot,
		InstallSite:  home,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("global"))},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock(global) error = %v", err)
	}

	mock := dolt.NewMockClient()
	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) { return mock, nil }
	defer func() { readClientOpener = prevOpener }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--all", "--scope", "project", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp upgradeResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.OK || len(resp.Upgrades) != 0 {
		t.Fatalf("expected empty successful upgrade result, got %+v", resp)
	}
}

func TestUpgradeAllWarnsAndSkipsBlockedDependency(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	goodRoot := filepath.Join(root, ".claude", "skills", "good")
	blockedRoot := filepath.Join(root, ".claude", "skills", "blocked")
	writeCmdFile(t, filepath.Join(goodRoot, "SKILL.md"), "old-good")
	writeCmdFile(t, filepath.Join(blockedRoot, "SKILL.md"), "old-blocked")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_good_project",
		Package:      "good",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  goodRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("old-good"))},
	}, {
		InstallID:    "pkg_blocked_project",
		Package:      "blocked",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  blockedRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("old-blocked"))},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	mock := dolt.NewMockClient()
	goodPkg := dolt.NewTestPackage("good", "good", "2.0.0", nil)
	goodPkg.AgentVariant = "claude"
	mock.AddPackage(goodPkg)
	mock.AddFiles("good", []models.PackageFile{{DestPath: "SKILL.md", Content: "new-good", SHA256: testSHA("new-good"), FileType: models.FileTypeSkill}})
	blockedPkg := dolt.NewTestPackage("blocked", "blocked", "2.0.0", nil)
	blockedPkg.AgentVariant = "claude"
	mock.AddPackage(blockedPkg)
	mock.AddFiles("blocked", []models.PackageFile{{DestPath: "SKILL.md", Content: "new-blocked", SHA256: testSHA("new-blocked"), FileType: models.FileTypeSkill}})
	mock.AddDeps("blocked", []models.PackageDep{{DepType: models.DepTypeTool, DepName: "missing-tool", DepSpec: ">=1"}})
	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) { return mock, nil }
	defer func() { readClientOpener = prevOpener }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--all", "--scope", "project", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp upgradeResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.OK || len(resp.Upgrades) != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	skipped := 0
	for _, result := range resp.Upgrades {
		if result.Package == "blocked" && result.Skipped && containsCI(result.Warnings, "incompatible dependency") {
			skipped++
		}
	}
	if skipped != 1 {
		t.Fatalf("expected blocked package skipped with warning, got %+v", resp.Upgrades)
	}
	if data, err := os.ReadFile(filepath.Join(goodRoot, "SKILL.md")); err != nil || string(data) != "new-good" { //nolint:gosec // test reads temp install root.
		t.Fatalf("good package was not upgraded: %q err=%v", string(data), err)
	}
	if data, err := os.ReadFile(filepath.Join(blockedRoot, "SKILL.md")); err != nil || string(data) != "old-blocked" { //nolint:gosec // test reads temp install root.
		t.Fatalf("blocked package should be unchanged: %q err=%v", string(data), err)
	}
}

func TestUninstallDefaultsToBothScopes(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	localRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	globalRoot := filepath.Join(home, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(localRoot, "SKILL.md"), "local")
	writeCmdFile(t, filepath.Join(globalRoot, "SKILL.md"), "global")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.0.0",
			Branch:       "main",
			InstallScope: "project",
			InstallRoot:  localRoot,
			InstallSite:  root,
			Files: map[string]string{
				"SKILL.md": integrity.ComputeContentSHA256([]byte("local")),
			},
		}},
	}); err != nil {
		t.Fatalf("SaveManifestLock(local) error = %v", err)
	}
	if err := installer.SaveManifestLock(home, installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_global",
			Package:      "team-lead",
			Version:      "1.1.0",
			Branch:       "main",
			InstallScope: "global",
			InstallRoot:  globalRoot,
			InstallSite:  home,
			Files: map[string]string{
				"SKILL.md": integrity.ComputeContentSHA256([]byte("global")),
			},
		}},
	}); err != nil {
		t.Fatalf("SaveManifestLock(global) error = %v", err)
	}
	if err := installer.SaveHookRegistry(root, installer.HookRegistry{Hooks: []installer.HookEntry{{Skill: "team-lead", Script: "x"}}}); err != nil {
		t.Fatalf("SaveHookRegistry() error = %v", err)
	}

	mock := dolt.NewMockClient()
	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) { return mock, nil }
	defer func() {
		readClientOpener = prevOpener
	}()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "team-lead", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp uninstallResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if len(resp.RemovedAll) != 2 {
		t.Fatalf("expected both scopes removed, got %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(localRoot, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected local file removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(globalRoot, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected global file removed, got err=%v", err)
	}
	localLock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock(local) error = %v", err)
	}
	if len(localLock.Installs) != 0 {
		t.Fatalf("expected local install removed, got %+v", localLock)
	}
	globalLock, err := installer.LoadManifestLock(home)
	if err != nil {
		t.Fatalf("LoadManifestLock(global) error = %v", err)
	}
	if len(globalLock.Installs) != 0 {
		t.Fatalf("expected global install removed, got %+v", globalLock)
	}
}

func TestUninstallRemovesHooksWithoutSiblingInstalls(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	localRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(localRoot, "SKILL.md"), "local")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.0.0",
			Branch:       "main",
			InstallScope: "project",
			InstallRoot:  localRoot,
			InstallSite:  root,
			Files: map[string]string{
				"SKILL.md": integrity.ComputeContentSHA256([]byte("local")),
			},
		}},
	}); err != nil {
		t.Fatalf("SaveManifestLock(local) error = %v", err)
	}
	if err := installer.SaveHookRegistry(root, installer.HookRegistry{Hooks: []installer.HookEntry{{Skill: "team-lead", Script: "x"}}}); err != nil {
		t.Fatalf("SaveHookRegistry() error = %v", err)
	}

	mock := dolt.NewMockClient()
	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) { return mock, nil }
	defer func() {
		readClientOpener = prevOpener
	}()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "team-lead", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp uninstallResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.Removed.HooksRemoved <= 0 {
		t.Fatalf("expected hooks removed > 0, got %+v", resp.Removed)
	}
}

func TestUninstallRemovesOnlyPackageOwnedHooks(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	alphaRoot := filepath.Join(root, ".claude", "skills", "alpha")
	writeCmdFile(t, filepath.Join(alphaRoot, "SKILL.md"), "alpha")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_alpha_project",
			Package:      "alpha",
			Version:      "1.0.0",
			Branch:       "main",
			InstallScope: "project",
			InstallRoot:  alphaRoot,
			InstallSite:  root,
			Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("alpha"))},
		}},
	}); err != nil {
		t.Fatalf("SaveManifestLock(local) error = %v", err)
	}
	if err := installer.SaveHookRegistry(root, installer.HookRegistry{Hooks: []installer.HookEntry{
		{Skill: "alpha", Scope: "project", Script: "alpha-hook"},
		{Skill: "beta", Scope: "project", Script: "beta-hook"},
	}}); err != nil {
		t.Fatalf("SaveHookRegistry() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "alpha", "--scope", "project", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	registry, err := installer.LoadHookRegistry(root)
	if err != nil {
		t.Fatalf("LoadHookRegistry() error = %v", err)
	}
	if len(registry.Hooks) != 1 || registry.Hooks[0].Skill != "beta" {
		t.Fatalf("expected beta hook preserved, got %+v", registry.Hooks)
	}
}

func TestUninstallLocalModificationRequiresForceOrYolo(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "changed")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files: map[string]string{
			"SKILL.md": integrity.ComputeContentSHA256([]byte("original")),
		},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "team-lead", "--scope", "project", "--json"})
	requireJSONCmdError(t, cmd.Execute())
	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Message != "locally modified files detected; use --force to proceed or --yolo in non-interactive mode" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	cmd = NewRootCmd("test", "abc", "2025-01-01")
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "team-lead", "--scope", "project", "--json", "--yolo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() yolo error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected modified file removed by --yolo, got err=%v", err)
	}

	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "changed")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files: map[string]string{
			"SKILL.md": integrity.ComputeContentSHA256([]byte("original")),
		},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock() recreate error = %v", err)
	}

	cmd = NewRootCmd("test", "abc", "2025-01-01")
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "team-lead", "--scope", "project", "--json", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() force error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected modified file removed by --force, got err=%v", err)
	}
}

func TestUninstallYoloRemovesOnlySCInstalledDependencies(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "skill")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("skill"))},
		Requirements: installer.RequirementSnapshot{
			CLIInstalled: []string{"owned", "preexisting", "missing-provenance", "empty-provenance"},
			CLIProvenance: map[string]string{
				"owned":            "installed-by-synaptic",
				"preexisting":      "already-present",
				"empty-provenance": "",
			},
		},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	removedDeps := []string{}
	prevRemove := removeSCDependency
	removeSCDependency = func(dep string) error {
		removedDeps = append(removedDeps, dep)
		return nil
	}
	defer func() { removeSCDependency = prevRemove }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "team-lead", "--scope", "project", "--json", "--yolo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp uninstallResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if strings.Join(removedDeps, ",") != "owned" {
		t.Fatalf("expected only owned dependency removed, got %+v", removedDeps)
	}
	if strings.Join(resp.Removed.RemovedDependencies, ",") != "owned" {
		t.Fatalf("expected JSON to report only owned dependency, got %+v", resp.Removed.RemovedDependencies)
	}
}

func TestUpgradeReadbackUsesStatusAndValidateJSON(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setTestHome(t, home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "old-skill")
	writeCmdFile(t, filepath.Join(installRoot, "hooks", "pre-commit.sh"), "#!/bin/sh\nexit 0\n")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		Variant:      "claude",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files: map[string]string{
			"SKILL.md":            integrity.ComputeContentSHA256([]byte("old-skill")),
			"hooks/pre-commit.sh": integrity.ComputeContentSHA256([]byte("#!/bin/sh\nexit 0\n")),
		},
		Hooks: []installer.HookEntry{{
			Event:    "PreToolUse",
			Matcher:  "git commit",
			Skill:    "team-lead",
			Scope:    "project",
			Script:   "hooks/pre-commit.sh",
			Priority: 10,
			Blocking: true,
		}},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	if err := installer.SaveHookRegistry(root, installer.HookRegistry{Hooks: []installer.HookEntry{{
		Event:    "PreToolUse",
		Matcher:  "git commit",
		Skill:    "team-lead",
		Scope:    "project",
		Script:   filepath.Join(installRoot, "hooks", "pre-commit.sh"),
		Priority: 10,
		Blocking: true,
	}}}); err != nil {
		t.Fatalf("SaveHookRegistry() error = %v", err)
	}

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "2.0.0", nil)
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{
		{DestPath: "SKILL.md", Content: "new-skill", SHA256: testSHA("new-skill"), FileType: models.FileTypeSkill},
		{DestPath: "hooks/pre-commit.sh", Content: "#!/bin/sh\necho upgraded\n", SHA256: testSHA("#!/bin/sh\necho upgraded\n"), FileType: models.FileTypeHook},
	})
	mock.AddHooks("team-lead", []models.PackageHook{{
		PackageID:  "team-lead",
		Event:      models.HookPreToolUse,
		Matcher:    "git commit",
		ScriptPath: "hooks/pre-commit.sh",
		Priority:   10,
		Blocking:   true,
	}})
	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) { return mock, nil }
	defer func() { readClientOpener = prevOpener }()

	upgradeCmd := NewRootCmd("test", "abc", "2025-01-01")
	var upgradeOut bytes.Buffer
	upgradeCmd.SetOut(&upgradeOut)
	upgradeCmd.SetErr(&upgradeOut)
	upgradeCmd.SetArgs([]string{"upgrade", "team-lead", "--scope", "project", "--branch", "beta", "--json", "--yolo"})
	if err := upgradeCmd.Execute(); err != nil {
		t.Fatalf("upgrade Execute() error = %v\noutput=%s", err, upgradeOut.String())
	}

	statusCmd := NewRootCmd("test", "abc", "2025-01-01")
	var statusOut bytes.Buffer
	statusCmd.SetOut(&statusOut)
	statusCmd.SetErr(&statusOut)
	statusCmd.SetArgs([]string{"status", "--json"})
	if err := statusCmd.Execute(); err != nil {
		t.Fatalf("status Execute() error = %v\noutput=%s", err, statusOut.String())
	}
	var statusResp statusResponse
	if err := json.Unmarshal(statusOut.Bytes(), &statusResp); err != nil {
		t.Fatalf("json.Unmarshal(status) error = %v\noutput=%s", err, statusOut.String())
	}
	if len(statusResp.Packages) != 1 || statusResp.Packages[0].Local == nil {
		t.Fatalf("expected one local package, got %+v", statusResp)
	}
	local := statusResp.Packages[0].Local
	if local.Version != "2.0.0" || local.Branch != "beta" || local.Validation != "PASS" {
		t.Fatalf("unexpected status after upgrade: %+v", local)
	}
	if hook := local.HookSummary.Hooks[0]; hook.Event != "PreToolUse" || hook.Script != "hooks/pre-commit.sh" || !hook.Registered {
		t.Fatalf("unexpected upgraded hook readback: %+v", hook)
	}

	validateCmd := NewRootCmd("test", "abc", "2025-01-01")
	var validateOut bytes.Buffer
	validateCmd.SetOut(&validateOut)
	validateCmd.SetErr(&validateOut)
	validateCmd.SetArgs([]string{"validate", "team-lead", "--scope", "project", "--json"})
	if err := validateCmd.Execute(); err != nil {
		t.Fatalf("validate Execute() error = %v\noutput=%s", err, validateOut.String())
	}
	var validateResp validateResponse
	if err := json.Unmarshal(validateOut.Bytes(), &validateResp); err != nil {
		t.Fatalf("json.Unmarshal(validate) error = %v\noutput=%s", err, validateOut.String())
	}
	if !validateResp.Pass || len(validateResp.Packages) != 1 {
		t.Fatalf("unexpected validate response after upgrade: %+v", validateResp)
	}
	if validateResp.Packages[0].Version != "2.0.0" || validateResp.Packages[0].Branch != "beta" {
		t.Fatalf("unexpected validate package after upgrade: %+v", validateResp.Packages[0])
	}
}

func TestUninstallReadbackUsesStatusAndValidateJSON(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setTestHome(t, home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "skill")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("skill"))},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	uninstallCmd := NewRootCmd("test", "abc", "2025-01-01")
	var uninstallOut bytes.Buffer
	uninstallCmd.SetOut(&uninstallOut)
	uninstallCmd.SetErr(&uninstallOut)
	uninstallCmd.SetArgs([]string{"uninstall", "team-lead", "--scope", "project", "--json", "--force"})
	if err := uninstallCmd.Execute(); err != nil {
		t.Fatalf("uninstall Execute() error = %v\noutput=%s", err, uninstallOut.String())
	}

	statusCmd := NewRootCmd("test", "abc", "2025-01-01")
	var statusOut bytes.Buffer
	statusCmd.SetOut(&statusOut)
	statusCmd.SetErr(&statusOut)
	statusCmd.SetArgs([]string{"status", "--json"})
	if err := statusCmd.Execute(); err != nil {
		t.Fatalf("status Execute() error = %v\noutput=%s", err, statusOut.String())
	}
	var statusResp statusResponse
	if err := json.Unmarshal(statusOut.Bytes(), &statusResp); err != nil {
		t.Fatalf("json.Unmarshal(status) error = %v\noutput=%s", err, statusOut.String())
	}
	if len(statusResp.Packages) != 0 {
		t.Fatalf("expected no packages after uninstall, got %+v", statusResp)
	}

	validateCmd := NewRootCmd("test", "abc", "2025-01-01")
	var validateOut bytes.Buffer
	validateCmd.SetOut(&validateOut)
	validateCmd.SetErr(&validateOut)
	validateCmd.SetArgs([]string{"validate", "team-lead", "--scope", "project", "--json"})
	requireJSONCmdError(t, validateCmd.Execute())
	var envelope jsonErrorEnvelope
	if err := json.Unmarshal(validateOut.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(validate) error = %v\noutput=%s", err, validateOut.String())
	}
	if envelope.OK || envelope.Error.Code != "not_found" {
		t.Fatalf("expected not_found after uninstall, got %+v", envelope)
	}
}

func containsCI(values []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}
