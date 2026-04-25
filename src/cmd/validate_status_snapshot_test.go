package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

func TestValidateCommandJSONAllFileStates(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "good.txt"), "good")
	writeCmdFile(t, filepath.Join(installRoot, "modified.txt"), "changed")
	writeCmdFile(t, filepath.Join(installRoot, "extra.txt"), "extra")
	if err := os.MkdirAll(filepath.Join(installRoot, "unreadable.txt"), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	lock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.2.0",
			Branch:       "main",
			InstallScope: "project",
			InstallRoot:  installRoot,
			InstallSite:  root,
			Files: map[string]string{
				"good.txt":       integrity.ComputeContentSHA256([]byte("good")),
				"modified.txt":   integrity.ComputeContentSHA256([]byte("original")),
				"missing.txt":    integrity.ComputeContentSHA256([]byte("missing")),
				"unreadable.txt": integrity.ComputeContentSHA256([]byte("anything")),
			},
		}},
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
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
	if resp.Pass {
		t.Fatalf("expected validate to fail, got %+v", resp)
	}
	if len(resp.Packages) != 1 {
		t.Fatalf("expected 1 package, got %+v", resp)
	}
	got := map[string]string{}
	for _, file := range resp.Packages[0].Files {
		got[file.Path] = file.Status
	}
	for path, want := range map[string]string{
		"good.txt":       "OK",
		"modified.txt":   "MODIFIED",
		"missing.txt":    "MISSING",
		"unreadable.txt": "UNREADABLE",
		"extra.txt":      "EXTRA",
	} {
		if got[path] != want {
			t.Fatalf("status[%s] = %q, want %q (all=%+v)", path, got[path], want, got)
		}
	}
}

func TestStatusCommandJSONMergedScopes(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	localRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	globalRoot := filepath.Join(home, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(localRoot, "SKILL.md"), "local")
	writeCmdFile(t, filepath.Join(globalRoot, "SKILL.md"), "global")

	localLock := installer.ManifestLock{
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
	}
	globalLock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_global",
			Package:      "team-lead",
			Version:      "1.1.0",
			Branch:       "beta",
			InstallScope: "global",
			InstallRoot:  globalRoot,
			InstallSite:  home,
			Files: map[string]string{
				"SKILL.md": integrity.ComputeContentSHA256([]byte("global")),
			},
		}},
	}
	if err := installer.SaveManifestLock(root, localLock); err != nil {
		t.Fatalf("SaveManifestLock(local) error = %v", err)
	}
	if err := installer.SaveManifestLock(home, globalLock); err != nil {
		t.Fatalf("SaveManifestLock(global) error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"status", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp statusResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if len(resp.Packages) != 1 {
		t.Fatalf("expected 1 package, got %+v", resp)
	}
	row := resp.Packages[0]
	if row.Global == nil || row.Local == nil {
		t.Fatalf("expected both scopes, got %+v", row)
	}
	if row.Global.Branch != "beta" || row.Global.Version != "1.1.0" || row.Global.Validation != "PASS" {
		t.Fatalf("unexpected global row: %+v", row.Global)
	}
	if row.Local.Branch != "main" || row.Local.Version != "1.0.0" || row.Local.Validation != "PASS" {
		t.Fatalf("unexpected local row: %+v", row.Local)
	}
}

func TestSnapshotCommandModifiedOnly(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "good.txt"), "good")
	writeCmdFile(t, filepath.Join(installRoot, "modified.txt"), "changed")
	writeCmdFile(t, filepath.Join(installRoot, "extra.txt"), "extra")

	lock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.2.0",
			Branch:       "advanced",
			InstallScope: "project",
			InstallRoot:  installRoot,
			InstallSite:  root,
			Files: map[string]string{
				"good.txt":     integrity.ComputeContentSHA256([]byte("good")),
				"modified.txt": integrity.ComputeContentSHA256([]byte("original")),
			},
		}},
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	snapshotNow = func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { snapshotNow = func() time.Time { return time.Now().UTC() } })
	snapshotGitRemoteURL = func(string) string { return "https://example.com/repo.git" }
	t.Cleanup(func() { snapshotGitRemoteURL = gitRemoteURL })

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "team-lead", "--scope", "project", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp snapshotResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if len(resp.Files) != 2 {
		t.Fatalf("expected 2 copied files, got %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(resp.OutputDir, "modified.txt")); err != nil {
		t.Fatalf("modified snapshot missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resp.OutputDir, "extra.txt")); err != nil {
		t.Fatalf("extra snapshot missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resp.OutputDir, "good.txt")); !os.IsNotExist(err) {
		t.Fatalf("good file should not be copied, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(resp.OutputDir, "snapshot.toml")); err != nil {
		t.Fatalf("snapshot.toml missing: %v", err)
	}
}
