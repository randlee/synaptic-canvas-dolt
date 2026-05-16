package cmd

import (
	"bytes"
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
	if len(lock.Installs) != 0 {
		t.Fatalf("manifest record should be removed before file deletion failure, got %+v", lock.Installs)
	}
	if _, err := os.Stat(filepath.Join(blockingDir, "child.txt")); err != nil {
		t.Fatalf("file removal failure fixture should remain for recovery, got %v", err)
	}
}
