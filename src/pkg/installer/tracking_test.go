package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestLockRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	lock := ManifestLock{
		Version: 1,
		Installs: []InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.0.0",
			Branch:       "main",
			InstallScope: "project",
			Files:        map[string]string{"a.txt": "abc"},
		}},
	}
	if err := SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	got, err := LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(got.Installs) != 1 || got.Installs[0].Package != "team-lead" {
		t.Fatalf("unexpected roundtrip: %+v", got)
	}
	if _, err := filepath.Abs(filepath.Join(root, ManifestLockPath)); err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
}

func TestLoadManifestLockNormalizesMissingTrackingOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, ManifestLockPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	data := []byte(`version = 1

[[installs]]
install_id = "pkg_team-lead_project"
package = "team-lead"
version = "1.0.0"
branch = "main"
install_scope = "project"
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := LoadManifestLock(root)
	if err != nil {
		t.Fatalf("LoadManifestLock() error = %v", err)
	}
	if len(got.Installs) != 1 {
		t.Fatalf("expected one install, got %+v", got)
	}
	if got.Installs[0].TrackingOrigin != "local-install" {
		t.Fatalf("TrackingOrigin = %q, want local-install", got.Installs[0].TrackingOrigin)
	}
}
