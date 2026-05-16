package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

func TestUninstallPersistsManifestBeforeRemoveOwnedFilesFailure(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	blockingDir := filepath.Join(installRoot, "SKILL.md")
	writeCmdFile(t, filepath.Join(blockingDir, "child.txt"), "keeps directory non-empty")
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

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "team-lead", "--scope", "project", "--json", "--force"})
	requireJSONCmdError(t, cmd.Execute())

	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(lock.Installs) != 1 {
		t.Fatalf("manifest record should be preserved when file deletion fails, got %+v", lock.Installs)
	}
	if _, err := os.Stat(filepath.Join(blockingDir, "child.txt")); err != nil {
		t.Fatalf("file removal failure fixture should remain for recovery, got %v", err)
	}
	var envelope jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if envelope.Error.Code != "conflict" {
		t.Fatalf("expected conflict JSON error, got %+v", envelope)
	}
	if envelope.Error.Message != "conflict removing tracked files: SKILL.md; manifest record preserved" {
		t.Fatalf("unexpected conflict message: %+v", envelope)
	}
}

func TestUninstallRemovesManifestOnlyAfterSuccessfulDeletion(t *testing.T) {
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
	}}}); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"uninstall", "team-lead", "--scope", "project", "--json", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\noutput=%s", err, out.String())
	}

	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(lock.Installs) != 0 {
		t.Fatalf("manifest record should be removed after successful deletion, got %+v", lock.Installs)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected tracked file removed, got err=%v", err)
	}
}
