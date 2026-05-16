package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestJSONInitCommand(t *testing.T) {
	root := t.TempDir()
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp initResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}
	for _, rel := range []string{".synaptic/manifest.lock", ".synaptic/repo-profile.toml", ".synaptic/env.toml", ".synaptic/hooks/registry.toml"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("%s missing: %v", rel, err)
		}
	}
}

func TestJSONInitCommandError(t *testing.T) {
	root := t.TempDir()
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	prev := initializeRepoFunc
	initializeRepoFunc = func(string) (initResponse, error) {
		return initResponse{}, errors.New("boom")
	}
	defer func() { initializeRepoFunc = prev }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init", "--json"})

	requireJSONCmdError(t, cmd.Execute())
	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != "internal_error" || resp.Error.Message != "boom" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestJSONInstallCommandDryRun(t *testing.T) {
	root := t.TempDir()
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath:   "SKILL.md.j2",
		Content:    "Hello {{ repo.name }}",
		SHA256:     "ignored",
		FileType:   models.FileTypeSkill,
		IsTemplate: true,
	}})

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--dry-run", "--json"})

	restore := installReadTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".synaptic", "manifest.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run should not create manifest.lock, got err=%v", err)
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("unexpected json output: %s", out.String())
	}
}

func TestInstallDependencyAcknowledgementRequiresYoloInNonTTY(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill,
	}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "missing-cli", DepSpec: ">=1"}})
	restore := installReadTestHooks(mock)
	defer restore()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--scope", "project", "--json"})
	requireJSONCmdError(t, cmd.Execute())
	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Message != "interactive confirmation required; use --yolo to proceed non-interactively" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestInstallYoloSkipsDependencyAcknowledgement(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill,
	}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "missing-cli", DepSpec: ">=1"}})
	restore := installReadTestHooks(mock)
	defer restore()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--scope", "project", "--json", "--yolo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestInstallYoloDryRunNoMutationNoPrompt(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill,
	}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "missing-cli", DepSpec: ">=1"}})
	restore := installReadTestHooks(mock)
	defer restore()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--scope", "project", "--dry-run", "--json", "--yolo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
	for _, path := range []string{
		filepath.Join(root, ".synaptic", "manifest.lock"),
		filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "team-lead", "SKILL.md"),
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("dry-run should not create %s, got err=%v", path, err)
		}
	}
}

func TestInstallCommandWritesFiles(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()
	resolvedRoot := mustEvalSymlinks(t, root)
	resolvedHome := mustEvalSymlinks(t, home)

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	static := "# title\n"
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: static, SHA256: testSHA(static), FileType: models.FileTypeSkill,
	}})

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetArgs([]string{"install", "team-lead"})

	restore := installReadTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(resolvedRoot, ".synaptic", "manifest.lock")); err != nil {
		t.Fatalf("manifest.lock missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resolvedRoot, ".claude", "skills", "team-lead", "SKILL.md")); err != nil {
		t.Fatalf("installed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resolvedHome, ".claude", "skills", "team-lead", "SKILL.md")); err != nil {
		t.Fatalf("global installed file missing: %v", err)
	}
}

func TestInstallScopeProjectWritesOnlyProjectArtifacts(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()
	resolvedRoot := mustEvalSymlinks(t, root)
	resolvedHome := mustEvalSymlinks(t, home)

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "# title\n", SHA256: testSHA("# title\n"), FileType: models.FileTypeSkill,
	}})
	restore := installReadTestHooks(mock)
	defer restore()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetArgs([]string{"install", "team-lead", "--scope", "project", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(resolvedRoot, ".claude", "skills", "team-lead", "SKILL.md")); err != nil {
		t.Fatalf("project installed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resolvedRoot, ".synaptic", "manifest.lock")); err != nil {
		t.Fatalf("project manifest.lock missing: %v", err)
	}
	for _, path := range []string{
		filepath.Join(resolvedHome, ".claude", "skills", "team-lead", "SKILL.md"),
		filepath.Join(resolvedHome, ".synaptic", "manifest.lock"),
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("project scope should not write global artifact %s, got err=%v", path, err)
		}
	}
}

func TestInstallLocalOnlyGlobalScopeHardFailsBeforeWrite(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("local-only", "local-only", "1.2.0", nil)
	pkg.AgentVariant = "claude"
	pkg.InstallScope = models.InstallScopeLocalOnly
	mock.AddPackage(pkg)
	mock.AddFiles("local-only", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill,
	}})
	restore := installReadTestHooks(mock)
	defer restore()

	const want = "package local-only is local-only and cannot be installed globally"
	for _, scope := range []string{"global", "both"} {
		cmd := NewRootCmd("test", "abc", "2025-01-01")
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"install", "local-only", "--scope", scope, "--json"})
		requireJSONCmdError(t, cmd.Execute())
		var resp jsonErrorEnvelope
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v\noutput=%s", scope, err, out.String())
		}
		if resp.OK || resp.Error.Code != "invalid_args" || resp.Error.Message != want {
			t.Fatalf("unexpected response for scope %s: %+v", scope, resp)
		}
		for _, path := range []string{
			filepath.Join(root, ".synaptic", "manifest.lock"),
			filepath.Join(root, ".claude", "skills", "local-only", "SKILL.md"),
			filepath.Join(home, ".synaptic", "manifest.lock"),
			filepath.Join(home, ".claude", "skills", "local-only", "SKILL.md"),
		} {
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("local-only failure should not write %s, got err=%v", path, err)
			}
		}
	}
}

func TestInstallScopeBothMixedWriteAndFailureReturnsExitOne(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", nil)
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill,
	}})
	restore := installReadTestHooks(mock)
	defer restore()

	globalDest := filepath.Join(home, ".claude", "skills", "team-lead", "SKILL.md")
	if err := os.MkdirAll(globalDest, 0o750); err != nil {
		t.Fatalf("MkdirAll(globalDest) error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--scope", "both", "--json"})
	if err := cmd.Execute(); err == nil || err.Error() != "install failed for all selected scopes" {
		t.Fatalf("Execute() error = %v, want rolled-back install failure", err)
	}

	var resp struct {
		OK         bool                  `json:"ok"`
		Error      jsonErrorPayload      `json:"error"`
		Partial    bool                  `json:"partial"`
		Installs   []map[string]any      `json:"installs"`
		RolledBack []map[string]any      `json:"rolled_back"`
		Failures   []installScopeFailure `json:"failures"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Partial || len(resp.Installs) != 0 || len(resp.RolledBack) != 1 || len(resp.Failures) != 1 {
		t.Fatalf("expected project rollback after global failure, got %+v", resp)
	}
	if resp.Error.Code != "internal_error" || resp.Error.Message != "install failed for all selected scopes" {
		t.Fatalf("unexpected error envelope: %+v", resp.Error)
	}
	if resp.Failures[0].Scope != "global" {
		t.Fatalf("failure scope = %q, want global", resp.Failures[0].Scope)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project install should have been rolled back, got err=%v", err)
	}
	localLock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock(local) error = %v", err)
	}
	if len(localLock.Installs) != 0 {
		t.Fatalf("project lock should have been rolled back, got %+v", localLock.Installs)
	}
	if info, err := os.Stat(globalDest); err != nil || !info.IsDir() {
		t.Fatalf("global failure fixture should remain a directory, info=%+v err=%v", info, err)
	}
}

func TestJSONInstallCommandError(t *testing.T) {
	root := t.TempDir()
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	mock.GetErr = errors.New("boom")

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--json"})

	restore := installReadTestHooks(mock)
	defer restore()

	requireJSONCmdError(t, cmd.Execute())
	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != "internal_error" || resp.Error.Message != "boom" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func installReadTestHooks(mock *dolt.MockClient) func() {
	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) { return mock, nil }
	return func() {
		readClientOpener = prevOpener
	}
}

func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	return func() { _ = os.Chdir(prev) }
}

func writeCmdFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func testSHA(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", path, err)
	}
	return resolved
}
