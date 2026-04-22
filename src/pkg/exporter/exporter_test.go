package exporter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/importer"
	"gopkg.in/yaml.v3"
)

func TestExportRoundTripFixture(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("..", "importer", "testdata", "basic-package")
	scanned, warnings, err := importer.ScanForTest(fixtureDir)
	if err != nil {
		t.Fatalf("ScanForTest() error = %v", err)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected template warnings to be preserved for fixture")
	}

	mock := dolt.NewMockClient()
	mock.AddPackage(&scanned.Package)
	mock.AddFiles(scanned.Package.ID, scanned.Files)
	mock.AddDeps(scanned.Package.ID, scanned.Deps)
	mock.AddHooks(scanned.Package.ID, scanned.Hooks)
	mock.AddQuestions(scanned.Package.ID, scanned.Questions)

	outDir := t.TempDir()
	svc := Service{Reader: mock}
	summary, err := svc.Export(context.Background(), ExportRequest{
		PackageID: scanned.Package.ID,
		OutputDir: outDir,
		Branch:    "main",
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if summary.FilesWritten != 5 {
		t.Fatalf("FilesWritten = %d, want 5", summary.FilesWritten)
	}
	if summary.FileSHAVerified != 4 {
		t.Fatalf("FileSHAVerified = %d, want 4", summary.FileSHAVerified)
	}

	exportedPackageDir := filepath.Join(outDir, scanned.Package.ID)
	compareTextFile(t, filepath.Join(fixtureDir, "agents", "sample-agent.md"), filepath.Join(exportedPackageDir, "agents", "sample-agent.md"))
	compareTextFile(t, filepath.Join(fixtureDir, "commands", "sample-command.md"), filepath.Join(exportedPackageDir, "commands", "sample-command.md"))
	compareTextFile(t, filepath.Join(fixtureDir, "skills", "sample-skill", "SKILL.md.j2"), filepath.Join(exportedPackageDir, "skills", "sample-skill", "SKILL.md.j2"))
	compareTextFile(t, filepath.Join(fixtureDir, ".claude-plugin", "plugin.json"), filepath.Join(exportedPackageDir, ".claude-plugin", "plugin.json"))

	sourceManifest := loadYAMLMap(t, filepath.Join(fixtureDir, "manifest.yaml"))
	exportedManifest := loadYAMLMap(t, filepath.Join(exportedPackageDir, "manifest.yaml"))
	if !yamlEqual(sourceManifest, exportedManifest) {
		t.Fatalf("manifest mismatch\nsource=%#v\nexported=%#v", sourceManifest, exportedManifest)
	}
}

func TestExportFailsOnAggregateMismatch(t *testing.T) {
	t.Parallel()

	scanned, _, err := importer.ScanForTest(filepath.Join("..", "importer", "testdata", "basic-package"))
	if err != nil {
		t.Fatalf("ScanForTest() error = %v", err)
	}
	bad := scanned.Package
	bad.SHA256 = stringPtr("bad-sha")

	mock := dolt.NewMockClient()
	mock.AddPackage(&bad)
	mock.AddFiles(bad.ID, scanned.Files)
	mock.AddDeps(bad.ID, scanned.Deps)
	mock.AddHooks(bad.ID, scanned.Hooks)
	mock.AddQuestions(bad.ID, scanned.Questions)

	_, err = Service{Reader: mock}.Export(context.Background(), ExportRequest{
		PackageID: bad.ID,
		OutputDir: t.TempDir(),
		Branch:    "main",
	})
	if err == nil || !strings.Contains(err.Error(), "aggregate sha mismatch") {
		t.Fatalf("expected aggregate mismatch, got %v", err)
	}
}

func compareTextFile(t *testing.T, src, dst string) {
	t.Helper()
	srcData, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", src, err)
	}
	dstData, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", dst, err)
	}
	if string(srcData) != string(dstData) {
		t.Fatalf("file mismatch for %s", dst)
	}
}

func loadYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	var out map[string]any
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("yaml.Unmarshal(%s): %v", path, err)
	}
	return out
}

func yamlEqual(a, b map[string]any) bool {
	return normalizeYAML(a) == normalizeYAML(b)
}

func normalizeYAML(v any) string {
	data, _ := yaml.Marshal(v)
	return string(data)
}

func stringPtr(v string) *string {
	return &v
}
