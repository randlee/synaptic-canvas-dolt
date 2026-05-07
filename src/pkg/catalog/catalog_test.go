package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSanitizeBranchName(t *testing.T) {
	t.Parallel()

	got := SanitizeBranchName("../../feature/http:beta")
	if got != ".._.._feature_http_beta" {
		t.Fatalf("SanitizeBranchName() = %q", got)
	}
	if filename := CatalogFilename("feature/http"); filename != "catalog-feature_http.toml" {
		t.Fatalf("CatalogFilename() = %q", filename)
	}
}

func TestLoadCorruptTOML(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog-main.toml")
	if err := os.WriteFile(path, []byte("[meta\nbad"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, _, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "parsing catalog") {
		t.Fatalf("Load() error = %v, want parse error", err)
	}
}

func TestLoadFutureSchemaWarns(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog-main.toml")
	data := `[meta]
branch = "main"
fetched_at = 2026-05-07T00:00:00Z
schema_version = 99
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, warnings, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Meta.SchemaVersion != 99 {
		t.Fatalf("SchemaVersion = %d, want 99", got.Meta.SchemaVersion)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "newer than supported") {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestRefreshPreservesOlderVersions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog-main.toml")
	old := []CatalogEntry{{PackageID: "team-lead", Version: "1.0.0", DocPath: "SKILL.md", SHA256: "old"}}
	if _, err := Refresh(path, "main", old, time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Refresh(old) error = %v", err)
	}
	next := []CatalogEntry{{PackageID: "team-lead", Version: "1.1.0", DocPath: "SKILL.md", SHA256: "new"}}
	got, err := Refresh(path, "main", next, time.Date(2026, 5, 7, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Refresh(new) error = %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("entries = %+v, want old and new versions", got.Entries)
	}
}

func TestRefreshConcurrentWritesDoNotCorrupt(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog-main.toml")
	var wg sync.WaitGroup
	for _, version := range []string{"1.0.0", "1.1.0"} {
		version := version
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = Refresh(path, "main", []CatalogEntry{{
				PackageID: "pkg",
				Version:   version,
				DocPath:   "SKILL.md",
				SHA256:    version,
			}}, time.Now().UTC())
		}()
	}
	wg.Wait()

	got, _, err := Load(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Fatal("catalog file was not written")
	}
	if err != nil {
		t.Fatalf("Load() after concurrent writes error = %v", err)
	}
	if got.Meta.Branch != "main" || len(got.Entries) == 0 {
		t.Fatalf("unexpected catalog after concurrent writes: %+v", got)
	}
}
