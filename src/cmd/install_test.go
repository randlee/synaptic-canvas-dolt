package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

func TestRollbackInstallSummaryPartialFailureScenarios(t *testing.T) {
	tests := []struct {
		name       string
		createFile bool
	}{
		{name: "succeeded scope rolled back after later scope failure", createFile: true},
		{name: "missing file during rollback is ignored", createFile: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
			if tt.createFile {
				writeCmdFile(t, filepath.Join(installRoot, "SKILL.md"), "skill")
			} else if err := os.MkdirAll(installRoot, 0o750); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
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
			if err := installer.SaveHookRegistry(root, installer.HookRegistry{Hooks: []installer.HookEntry{{
				Skill: "team-lead",
				Scope: "project",
			}}}); err != nil {
				t.Fatalf("SaveHookRegistry() error = %v", err)
			}

			err := rollbackInstallSummary(root, installer.Summary{
				PackageID:   "team-lead",
				Scope:       "project",
				InstallRoot: installRoot,
				Files:       []installer.PlannedFile{{Path: filepath.Join(installRoot, "SKILL.md")}},
			})
			if err != nil {
				t.Fatalf("rollbackInstallSummary() error = %v", err)
			}
			lock, err := installer.LoadManifestLock(root)
			if err != nil {
				t.Fatalf("LoadManifestLock() error = %v", err)
			}
			if len(lock.Installs) != 0 {
				t.Fatalf("manifest record was not removed: %+v", lock.Installs)
			}
			registry, err := installer.LoadHookRegistry(root)
			if err != nil {
				t.Fatalf("LoadHookRegistry() error = %v", err)
			}
			if len(registry.Hooks) != 0 {
				t.Fatalf("hook record was not removed: %+v", registry.Hooks)
			}
		})
	}
}
