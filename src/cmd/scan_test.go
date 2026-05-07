package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

func TestScanNoClaudeDirReturnsEmptyCandidates(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !resp.OK || len(resp.Candidates) != 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestScanNonPackageFilesReturnZeroCandidates(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeCmdFile(t, filepath.Join(root, ".claude", "notes.txt"), "not a package")
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("skill"),
	}}, time.Now().UTC())
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json", "--scope", "project")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(resp.Candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", resp.Candidates)
	}
}

func TestScanPermissionErrorContinuesAndReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permission semantics do not reliably produce walk errors")
	}
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeCmdFile(t, filepath.Join(root, ".claude", "skills", "good", "SKILL.md"), "good")
	locked := filepath.Join(root, ".claude", "locked")
	writeCmdFile(t, filepath.Join(locked, "secret.md"), "secret")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o750) }() //nolint:gosec // test cleanup restores traversable temp directory permissions.
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "good",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("good"),
	}}, time.Now().UTC())

	result, err := scanInstalledPackages(context.Background(), scanOptions{
		RepoRoot: root,
		Branch:   "main",
		Scope:    "project",
	})
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Package != "good" {
		t.Fatalf("walk should continue after permission error, got %+v", result.Candidates)
	}
}

func TestScanCatalogMatchPreferenceInstalledThenLatest(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeCmdFile(t, filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md"), "same")
	entries := []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("same"),
	}, {
		PackageID: "team-lead",
		Version:   "2.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("same"),
	}}
	writeProjectCatalog(t, root, "main", entries, time.Now().UTC())

	result, err := scanInstalledPackages(context.Background(), scanOptions{RepoRoot: root, Branch: "main", Scope: "project"})
	if err != nil {
		t.Fatalf("scanInstalledPackages() error = %v", err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Version != "2.0.0" {
		t.Fatalf("expected latest version without lockfile, got %+v", result.Candidates)
	}

	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  filepath.Join(root, ".claude", "skills", "other-root"),
		Files:        map[string]string{},
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	result, err = scanInstalledPackages(context.Background(), scanOptions{RepoRoot: root, Branch: "main", Scope: "project"})
	if err != nil {
		t.Fatalf("scanInstalledPackages() error = %v", err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Version != "1.0.0" {
		t.Fatalf("expected installed version preference, got %+v", result.Candidates)
	}
}

func TestScanContextCancelAvoidsPartialLockfileWrites(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeCmdFile(t, filepath.Join(root, ".claude", "skills", "team-lead", "A.md"), "a")
	writeCmdFile(t, filepath.Join(root, ".claude", "skills", "team-lead", "B.md"), "b")
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "A.md",
		SHA256:    testSHA("a"),
	}, {
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "B.md",
		SHA256:    testSHA("b"),
	}}, time.Now().UTC())
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prev := scanComputeFileSHA
	calls := 0
	scanComputeFileSHA = func(path string) (string, error) {
		calls++
		if calls == 1 {
			cancel()
		}
		return integrity.ComputeFileSHA256(path)
	}
	defer func() { scanComputeFileSHA = prev }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"scan", "--accept-all"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(lock.Installs) != 0 {
		t.Fatalf("expected no partial lockfile writes, got %+v", lock.Installs)
	}
}

func TestScanAlreadyTrackedFileIsNotCandidate(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "skill")
	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		Files:        map[string]string{"SKILL.md": testSHA("skill")},
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("skill"),
	}}, time.Now().UTC())
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json", "--scope", "project")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(resp.Candidates) != 0 {
		t.Fatalf("tracked file should not be presented, got %+v", resp.Candidates)
	}
}

func TestScanSkipsSymlinkFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	target := filepath.Join(root, "target.md")
	writeCmdFile(t, target, "skill")
	link := filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("skill"),
	}}, time.Now().UTC())
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json", "--scope", "project")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(resp.Candidates) != 0 {
		t.Fatalf("symlink should be skipped, got %+v", resp.Candidates)
	}
}

func TestScanStaleCatalogWarning(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeCmdFile(t, filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md"), "skill")
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	prevNow := snapshotNow
	snapshotNow = func() time.Time { return now }
	defer func() { snapshotNow = prevNow }()
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("skill"),
	}}, now.Add(-25*time.Hour))
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json", "--scope", "project")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !containsCI(resp.Warnings, "older than 24h") {
		t.Fatalf("expected stale catalog warning, got %+v", resp.Warnings)
	}
}

func TestScanAcceptAllZeroCandidatesNoOp(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json", "--accept-all")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if resp.Accepted != 0 || resp.Mutated != true {
		t.Fatalf("unexpected response: %+v", resp)
	}
	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(lock.Installs) != 0 {
		t.Fatalf("expected no installs, got %+v", lock.Installs)
	}
}

func TestScanJSONAloneDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeCmdFile(t, filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md"), "skill")
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("skill"),
	}}, time.Now().UTC())
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json", "--scope", "project")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(resp.Candidates) != 1 || resp.Mutated {
		t.Fatalf("unexpected response: %+v", resp)
	}
	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(lock.Installs) != 0 {
		t.Fatalf("scan --json must not mutate, got %+v", lock.Installs)
	}
}

func TestScanJSONAcceptAllWritesLockfile(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeCmdFile(t, filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md"), "skill")
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("skill"),
	}}, time.Now().UTC())
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json", "--scope", "project", "--accept-all")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if resp.Accepted != 1 || !resp.Mutated {
		t.Fatalf("unexpected response: %+v", resp)
	}
	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(lock.Installs) != 1 {
		t.Fatalf("expected one install, got %+v", lock.Installs)
	}
	got := lock.Installs[0]
	if got.TrackingOrigin != trackingOriginScanReconciled || got.DoltCommit != "" || got.Files["SKILL.md"] != testSHA("skill") {
		t.Fatalf("unexpected scan install record: %+v", got)
	}
}

func TestScanUpgradeAllUpdatesTrackedVersion(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "new")
	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:      "pkg_team-lead_project",
		Package:        "team-lead",
		Version:        "1.0.0",
		Branch:         "main",
		DoltCommit:     "old-commit",
		InstallScope:   "project",
		InstallRoot:    installRoot,
		InstallSite:    root,
		TrackingOrigin: "local-install",
		Files:          map[string]string{"SKILL.md": testSHA("old")},
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	writeProjectCatalog(t, root, "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "2.0.0",
		DocPath:   "SKILL.md",
		SHA256:    testSHA("new"),
	}}, time.Now().UTC())
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	resp, err := executeScanJSON(t, "scan", "--json", "--scope", "project", "--upgrade-all")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if resp.Upgraded != 1 || len(resp.Candidates) != 1 || !resp.Candidates[0].NeedsUpgrade {
		t.Fatalf("unexpected response: %+v", resp)
	}
	gotLock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	got := gotLock.Installs[0]
	if got.Version != "2.0.0" || got.DoltCommit != "" || got.Files["SKILL.md"] != testSHA("new") {
		t.Fatalf("unexpected upgraded record: %+v", got)
	}
}

func TestScanAbsentCatalogErrors(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeCmdFile(t, filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md"), "skill")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetArgs([]string{"scan", "--scope", "project"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "catalog not found for branch main") {
		t.Fatalf("Execute() error = %v, want missing catalog", err)
	}
}

func TestScanActionFlagsAreMutuallyExclusive(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	_, err := executeScanJSON(t, "scan", "--json", "--accept-all", "--upgrade-all")
	if err != nil {
		t.Fatalf("JSON command writes error envelope without returning error, got %v", err)
	}
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetArgs([]string{"scan", "--accept-all", "--upgrade-all"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("Execute() error = %v, want flag conflict", err)
	}
}

func executeScanJSON(t *testing.T, args ...string) (scanResponse, error) {
	t.Helper()
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	var resp scanResponse
	if out.Len() > 0 {
		if decodeErr := json.Unmarshal(out.Bytes(), &resp); decodeErr != nil {
			t.Fatalf("json.Unmarshal() error = %v\noutput=%s", decodeErr, out.String())
		}
	}
	return resp, err
}

func writeProjectCatalog(t *testing.T, root, branch string, entries []catalog.CatalogEntry, fetchedAt time.Time) {
	t.Helper()
	if _, err := catalog.Refresh(catalog.ProjectPath(root, branch), branch, entries, fetchedAt); err != nil {
		t.Fatalf("catalog.Refresh() error = %v", err)
	}
}
