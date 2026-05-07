package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

func TestRollbackInstallSummaryPartialFailureScenarios(t *testing.T) {
	tests := []struct {
		name           string
		createFile     bool
		setup          func(t *testing.T, root string) func()
		wantErr        []string
		checkHooks     bool
		wantHooksAfter int
	}{
		{name: "succeeded scope rolled back after later scope failure", createFile: true},
		{name: "missing file during rollback is ignored", createFile: false},
		{
			name:       "manifest write failure still attempts hook rollback",
			createFile: true,
			setup: func(t *testing.T, root string) func() {
				t.Helper()
				if runtime.GOOS == "windows" {
					t.Skip("chmod-based write-denial semantics are platform-specific")
				}
				synapticDir := filepath.Join(root, ".synaptic")
				//nolint:gosec // Directory execute bit is required while denying writes in this fixture.
				if err := os.Chmod(synapticDir, 0o500); err != nil {
					t.Fatalf("Chmod(.synaptic) error = %v", err)
				}
				return func() {
					//nolint:gosec // Restore test fixture directory permissions.
					_ = os.Chmod(synapticDir, 0o750)
				}
			},
			wantErr: []string{
				"creating temp file",
			},
			checkHooks:     true,
			wantHooksAfter: 0,
		},
		{
			name:       "hook registry write failure is returned",
			createFile: true,
			setup: func(t *testing.T, root string) func() {
				t.Helper()
				if runtime.GOOS == "windows" {
					t.Skip("chmod-based write-denial semantics are platform-specific")
				}
				hooksDir := filepath.Join(root, ".synaptic", "hooks")
				//nolint:gosec // Directory execute bit is required while denying writes in this fixture.
				if err := os.Chmod(hooksDir, 0o500); err != nil {
					t.Fatalf("Chmod(hooks) error = %v", err)
				}
				return func() {
					//nolint:gosec // Restore test fixture directory permissions.
					_ = os.Chmod(hooksDir, 0o750)
				}
			},
			wantErr:        []string{"creating temp file"},
			checkHooks:     true,
			wantHooksAfter: 1,
		},
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
			restore := func() {}
			if tt.setup != nil {
				cleanup := tt.setup(t, root)
				restored := false
				restore = func() {
					if restored {
						return
					}
					cleanup()
					restored = true
				}
				t.Cleanup(restore)
			}

			err := rollbackInstallSummary(root, installer.Summary{
				PackageID:   "team-lead",
				Scope:       "project",
				InstallRoot: installRoot,
				Files:       []installer.PlannedFile{{Path: filepath.Join(installRoot, "SKILL.md")}},
			})
			if len(tt.wantErr) == 0 && err != nil {
				t.Fatalf("rollbackInstallSummary() error = %v", err)
			}
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatal("rollbackInstallSummary() error = nil, want failure")
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("rollbackInstallSummary() error = %v, want substring %q", err, want)
					}
				}
			}
			restore()
			lock, err := installer.LoadManifestLock(root)
			if err != nil && len(tt.wantErr) == 0 {
				t.Fatalf("LoadManifestLock() error = %v", err)
			}
			if len(tt.wantErr) == 0 && len(lock.Installs) != 0 {
				t.Fatalf("manifest record was not removed: %+v", lock.Installs)
			}
			registry, err := installer.LoadHookRegistry(root)
			if err != nil && len(tt.wantErr) == 0 {
				t.Fatalf("LoadHookRegistry() error = %v", err)
			}
			if len(tt.wantErr) == 0 && len(registry.Hooks) != 0 {
				t.Fatalf("hook record was not removed: %+v", registry.Hooks)
			}
			if tt.checkHooks && len(registry.Hooks) != tt.wantHooksAfter {
				t.Fatalf("registry hooks after rollback = %+v, want %d", registry.Hooks, tt.wantHooksAfter)
			}
		})
	}
}
