package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
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
	if resp.OK || resp.Error.Code != api.ErrorCodeConfirmationNeeded || resp.Error.Message != "interactive confirmation required; use --yolo to proceed non-interactively" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Error.Retryable {
		t.Fatalf("confirmation_required should not be retryable: %+v", resp.Error)
	}
	if resp.Error.SuggestedAction != "rerun with --yolo to proceed non-interactively" {
		t.Fatalf("unexpected suggested_action: %+v", resp.Error)
	}
}

func TestInstallScopeBothAllFailuresPreserveTypedCode(t *testing.T) {
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

	prevExecute := executeInstallService
	executeInstallService = func(context.Context, installer.Request) (installer.Summary, error) {
		return installer.Summary{}, errors.New("incompatible dependency: missing tool")
	}
	defer func() { executeInstallService = prevExecute }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--scope", "both", "--json", "--yolo"})

	requireJSONCmdError(t, cmd.Execute())

	var resp api.InstallResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error == nil {
		t.Fatalf("expected aggregate install failure, got %+v", resp)
	}
	if resp.Error.Code != api.ErrorCodeBlocked {
		t.Fatalf("aggregate code = %q, want blocked", resp.Error.Code)
	}
	if resp.Error.Retryable {
		t.Fatalf("blocked aggregate should not be retryable: %+v", resp.Error)
	}
	if len(resp.Failures) != 1 || resp.Failures[0].Code != api.ErrorCodeBlocked {
		t.Fatalf("expected typed per-scope failure, got %+v", resp.Failures)
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

func TestInstallScopeBothPartialFailurePreservesTypedSuberrors(t *testing.T) {
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

	callCount := 0
	prevExecute := executeInstallService
	executeInstallService = func(ctx context.Context, req installer.Request) (installer.Summary, error) {
		callCount++
		if callCount == 1 {
			return installer.Summary{
				PackageID:    req.Package.ID,
				Version:      req.Package.Version,
				Branch:       req.Branch,
				Scope:        "project",
				InstallRoot:  filepath.Join(root, ".claude", "skills", "team-lead"),
				FilesWritten: 1,
				Files: []installer.PlannedFile{{
					Path: filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md"),
				}},
			}, nil
		}
		return installer.Summary{}, errors.New("interactive confirmation required; use --yolo to proceed non-interactively")
	}
	defer func() { executeInstallService = prevExecute }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--scope", "both", "--json", "--yolo"})

	requireJSONCmdError(t, cmd.Execute())

	var resp api.InstallResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error == nil || resp.Error.Code != api.ErrorCodeConfirmationNeeded {
		t.Fatalf("expected confirmation_required aggregate failure, got %+v", resp)
	}
	if resp.Error.Retryable {
		t.Fatalf("confirmation aggregate should not be retryable: %+v", resp.Error)
	}
	if resp.Error.SuggestedAction != "rerun with --yolo to proceed non-interactively" {
		t.Fatalf("unexpected aggregate suggested_action: %+v", resp.Error)
	}
	if len(resp.Failures) != 1 || resp.Failures[0].Code != api.ErrorCodeConfirmationNeeded {
		t.Fatalf("expected typed failure entry, got %+v", resp.Failures)
	}
	if resp.Failures[0].SuggestedAction != "rerun with --yolo to proceed non-interactively" {
		t.Fatalf("unexpected failure suggested_action: %+v", resp.Failures[0])
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

func TestJSONInstallBackendFailureIncludesRetryableMetadata(t *testing.T) {
	t.Run("get package path", func(t *testing.T) {
		root := t.TempDir()
		writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
		restoreDir := chdirForTest(t, root)
		defer restoreDir()

		mock := dolt.NewMockClient()
		mock.GetErr = fmt.Errorf("%w: upstream busy", dolt.ErrRateLimited)

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
		if resp.OK || resp.Error.Code != api.ErrorCodeBackendUnavailable {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if !resp.Error.Retryable {
			t.Fatalf("expected backend failure to be retryable: %+v", resp.Error)
		}
		if resp.Error.SuggestedAction != "retry or switch to a reachable backend" {
			t.Fatalf("unexpected suggested_action: %+v", resp.Error)
		}
		if resp.Error.Details["cause_code"] != "rate_limited" || resp.Error.Details["operation"] != "get_package" {
			t.Fatalf("unexpected backend details: %+v", resp.Error.Details)
		}
	})

	t.Run("scope loop path", func(t *testing.T) {
		root := t.TempDir()
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

		prevExecute := executeInstallService
		executeInstallService = func(context.Context, installer.Request) (installer.Summary, error) {
			return installer.Summary{}, fmt.Errorf("%w: upstream busy", dolt.ErrRateLimited)
		}
		defer func() { executeInstallService = prevExecute }()

		cmd := NewRootCmd("test", "abc", "2025-01-01")
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"install", "team-lead", "--scope", "both", "--json", "--yolo"})

		requireJSONCmdError(t, cmd.Execute())

		var resp api.InstallResponse
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
		}
		if resp.OK || resp.Error == nil || resp.Error.Code != api.ErrorCodeBackendUnavailable {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if !resp.Error.Retryable {
			t.Fatalf("expected aggregate backend failure to be retryable: %+v", resp.Error)
		}
		if resp.Error.Details["cause_code"] != "rate_limited" || resp.Error.Details["operation"] != "install_scope" {
			t.Fatalf("unexpected aggregate backend details: %+v", resp.Error.Details)
		}
		if len(resp.Failures) != 1 {
			t.Fatalf("len(resp.Failures) = %d, want 1 (%+v)", len(resp.Failures), resp.Failures)
		}
		failure := resp.Failures[0]
		if !failure.Retryable {
			t.Fatalf("expected scoped failure to be retryable: %+v", failure)
		}
		if failure.Details["cause_code"] != "rate_limited" || failure.Details["operation"] != "install_scope" {
			t.Fatalf("unexpected scoped backend details: %+v", failure.Details)
		}
	})
}

func TestInstallReadbackUsesStatusAndValidateJSON(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setTestHome(t, home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{
		{DestPath: "SKILL.md", Content: "skill", SHA256: testSHA("skill"), FileType: models.FileTypeSkill},
		{DestPath: "hooks/pre-commit.sh", Content: "#!/bin/sh\n", SHA256: testSHA("#!/bin/sh\n"), FileType: models.FileTypeHook},
	})
	mock.AddHooks("team-lead", []models.PackageHook{{
		PackageID:  "team-lead",
		Event:      models.HookPreToolUse,
		Matcher:    "git commit",
		ScriptPath: "hooks/pre-commit.sh",
		Priority:   10,
		Blocking:   true,
	}})
	restore := installReadTestHooks(mock)
	defer restore()

	installCmd := NewRootCmd("test", "abc", "2025-01-01")
	var installOut bytes.Buffer
	installCmd.SetOut(&installOut)
	installCmd.SetErr(&installOut)
	installCmd.SetArgs([]string{"install", "team-lead", "--scope", "project", "--json", "--yolo"})
	if err := installCmd.Execute(); err != nil {
		t.Fatalf("install Execute() error = %v\noutput=%s", err, installOut.String())
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
	if local.Version != "1.2.0" || local.Branch != "main" || local.Validation != "PASS" {
		t.Fatalf("unexpected status local readback: %+v", local)
	}
	if local.HookSummary.Tracked != 1 || local.HookSummary.Registered != 1 || len(local.HookSummary.Hooks) != 1 {
		t.Fatalf("unexpected hook summary: %+v", local.HookSummary)
	}
	if hook := local.HookSummary.Hooks[0]; hook.Event != "PreToolUse" || hook.Script != "hooks/pre-commit.sh" || !hook.Registered {
		t.Fatalf("unexpected hook readback: %+v", hook)
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
		t.Fatalf("unexpected validate response: %+v", validateResp)
	}
	if hook := validateResp.Packages[0].HookSummary.Hooks[0]; hook.Event != "PreToolUse" || hook.Script != "hooks/pre-commit.sh" || !hook.Registered {
		t.Fatalf("unexpected validate hook readback: %+v", hook)
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
