package repo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectProfile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module test\n")
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"react":"18.0.0"}}`)
	writeFile(t, filepath.Join(root, ".github", "workflows", "test.yml"), "name: test")

	profile, err := DetectProfile(root, time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DetectProfile() error = %v", err)
	}
	if profile.Repo.Name != filepath.Base(root) {
		t.Fatalf("unexpected repo name: %s", profile.Repo.Name)
	}
	if profile.Repo.PrimaryLanguage != "go" {
		t.Fatalf("unexpected primary language: %s", profile.Repo.PrimaryLanguage)
	}
	if profile.Repo.CISystem != "github-actions" {
		t.Fatalf("unexpected ci system: %s", profile.Repo.CISystem)
	}
	if len(profile.Repo.Frameworks) == 0 || profile.Repo.Frameworks[0] != "react" {
		t.Fatalf("expected react framework, got %+v", profile.Repo.Frameworks)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
