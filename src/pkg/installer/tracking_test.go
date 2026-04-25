package installer

import (
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
