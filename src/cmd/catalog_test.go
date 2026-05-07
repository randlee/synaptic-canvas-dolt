package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestCatalogUpdateCommandWritesBothScopes(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	mock.Catalog = []catalog.CatalogEntry{{PackageID: "team-lead", Version: "1.0.0", DocPath: "SKILL.md", SHA256: "abc"}}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"catalog", "update", "--branch", "feature/http", "--json"})
	restore := installReadTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp catalogUpdateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.OK || resp.Branch != "feature/http" || resp.Entries != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	for _, path := range []string{
		filepath.Join(root, ".synaptic", "catalog-feature_http.toml"),
		filepath.Join(home, ".synaptic", "catalog-feature_http.toml"),
	} {
		got, _, err := catalog.Load(path)
		if err != nil {
			t.Fatalf("Load(%s) error = %v", path, err)
		}
		if len(got.Entries) != 1 || got.Entries[0].SHA256 != "abc" {
			t.Fatalf("unexpected catalog at %s: %+v", path, got)
		}
	}
}

func TestValidateUsesCatalogSHA(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "good")
	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files: map[string]string{
			"SKILL.md": integrity.ComputeContentSHA256([]byte("wrong-lock-sha")),
		},
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	if _, err := catalog.Refresh(catalog.ProjectPath(root, "main"), "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    integrity.ComputeContentSHA256([]byte("good")),
	}, {
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "README.md",
		SHA256:    integrity.ComputeContentSHA256([]byte("ignored")),
	}}, time.Now().UTC()); err != nil {
		t.Fatalf("catalog.Refresh() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "team-lead", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp validateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.Pass {
		t.Fatalf("expected catalog SHA to pass despite wrong lock SHA: %+v", resp)
	}
}

func TestValidateIgnoresCatalogEntryWithNoMatchingLockfileEntry(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "good")
	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files: map[string]string{
			"SKILL.md": integrity.ComputeContentSHA256([]byte("good")),
		},
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	if _, err := catalog.Refresh(catalog.ProjectPath(root, "main"), "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    integrity.ComputeContentSHA256([]byte("good")),
	}, {
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "README.md",
		SHA256:    integrity.ComputeContentSHA256([]byte("not installed")),
	}, {
		PackageID: "other-package",
		Version:   "9.9.9",
		DocPath:   "SKILL.md",
		SHA256:    integrity.ComputeContentSHA256([]byte("other")),
	}}, time.Now().UTC()); err != nil {
		t.Fatalf("catalog.Refresh() error = %v", err)
	}

	summary, err := validateTrackedInstall(lock.Installs[0])
	if err != nil {
		t.Fatalf("validateTrackedInstall() error = %v", err)
	}
	if !summary.Pass {
		t.Fatalf("expected validation pass with unmatched catalog entries ignored: %+v", summary)
	}
	if len(summary.Warnings) != 0 {
		t.Fatalf("unexpected warnings for benign unmatched catalog entries: %+v", summary.Warnings)
	}
	if len(summary.Files) != 1 || summary.Files[0].Path != "SKILL.md" {
		t.Fatalf("unexpected files: %+v", summary.Files)
	}
}

func TestValidateStaleCatalogEmitsWarning(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	prevNow := snapshotNow
	snapshotNow = func() time.Time { return now }
	defer func() { snapshotNow = prevNow }()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "good")
	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files: map[string]string{
			"SKILL.md": integrity.ComputeContentSHA256([]byte("good")),
		},
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	if _, err := catalog.Refresh(catalog.ProjectPath(root, "main"), "main", []catalog.CatalogEntry{{
		PackageID: "team-lead",
		Version:   "1.0.0",
		DocPath:   "SKILL.md",
		SHA256:    integrity.ComputeContentSHA256([]byte("good")),
	}}, now.Add(-25*time.Hour)); err != nil {
		t.Fatalf("catalog.Refresh() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "team-lead", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp validateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.Pass {
		t.Fatalf("expected stale catalog warning without failure: %+v", resp)
	}
	if len(resp.Packages) != 1 || !containsCI(resp.Packages[0].Warnings, "older than 24h") {
		t.Fatalf("expected stale catalog warning, got %+v", resp.Packages)
	}
}

func TestValidateAbsentCatalogDoltOfflineFallbackWarning(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "good")
	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  installRoot,
		InstallSite:  root,
		Files: map[string]string{
			"SKILL.md": integrity.ComputeContentSHA256([]byte("good")),
		},
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	prevFetch := validateCatalogFetch
	validateCatalogFetch = func(context.Context, string, string) ([]catalog.CatalogEntry, error) {
		return nil, errors.New("offline")
	}
	defer func() { validateCatalogFetch = prevFetch }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "team-lead", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp validateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.Pass || len(resp.Packages) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.Packages[0].Warnings) != 1 || !strings.Contains(resp.Packages[0].Warnings[0], "catalog unavailable and Dolt offline") {
		t.Fatalf("expected offline fallback warning, got %+v", resp.Packages[0].Warnings)
	}
}

func TestValidateEmptyCatalogSkipsSHACheck(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "changed")
	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
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
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	if _, err := catalog.Refresh(catalog.ProjectPath(root, "main"), "main", nil, time.Now().UTC()); err != nil {
		t.Fatalf("catalog.Refresh() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "team-lead", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp validateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.Pass || len(resp.Packages[0].Warnings) == 0 {
		t.Fatalf("expected pass with empty catalog warning, got %+v", resp)
	}
}

func TestInstallCatalogRefreshFailureIsNonFatal(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	mock.AddPackage(pkg)
	static := "# title\n"
	mock.AddFiles("team-lead", catalogPackageFiles("SKILL.md", static))
	mock.CatalogErr = errors.New("refresh failed")

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--json"})
	restore := installReadTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp["ok"] != true {
		t.Fatalf("install should succeed despite refresh failure: %+v", resp)
	}
	warnings, ok := resp["warnings"].([]any)
	if !ok || len(warnings) != 1 || !strings.Contains(warnings[0].(string), "catalog refresh failed") {
		t.Fatalf("expected catalog refresh warning, got %+v", resp["warnings"])
	}
}

func catalogPackageFiles(dest, content string) []models.PackageFile {
	return []models.PackageFile{{
		DestPath: dest,
		Content:  content,
		SHA256:   testSHA(content),
		FileType: models.FileTypeSkill,
	}}
}
