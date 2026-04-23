//go:build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/importer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/publisher"
)

func TestPublishPromotesBranch(t *testing.T) {
	if _, err := exec.LookPath("dolt"); err != nil {
		t.Skip("dolt not installed")
	}

	repoDir := initPublishRepo(t)
	importFixtureToBranch(t, repoDir, "develop", filepath.Join("..", "..", "pkg", "importer", "testdata", "basic-package"))

	svc := publisher.Service{
		Reader: dolt.NewCLIReader(repoDir, "develop"),
		Merger: dolt.NewCLIPublisher(repoDir),
	}
	summary, err := svc.Publish(context.Background(), publisher.PublishRequest{
		PackageID:  "sample-skill",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if summary.Merge == nil {
		t.Fatalf("expected merge result")
	}
	rows := runQuery(t, repoDir, "beta", "select id from packages where id = 'sample-skill';")
	if !strings.Contains(rows, "sample-skill") {
		t.Fatalf("package missing on beta:\n%s", rows)
	}
}

func TestPublishBlocksInvalidTemplate(t *testing.T) {
	if _, err := exec.LookPath("dolt"); err != nil {
		t.Skip("dolt not installed")
	}

	repoDir := initPublishRepo(t)
	fixtureDir := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "pkg", "importer", "testdata", "basic-package"), fixtureDir)
	badTemplate := filepath.Join(fixtureDir, "skills", "sample-skill", "SKILL.md.j2")
	if err := os.WriteFile(badTemplate, []byte("{{ answers.missing }}"), 0o644); err != nil { //nolint:gosec // G306: integration fixture output is intentional.
		t.Fatal(err)
	}
	importFixtureToBranch(t, repoDir, "develop", fixtureDir)

	svc := publisher.Service{
		Reader: dolt.NewCLIReader(repoDir, "develop"),
		Merger: dolt.NewCLIPublisher(repoDir),
	}
	summary, err := svc.Publish(context.Background(), publisher.PublishRequest{
		PackageID:  "sample-skill",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err == nil {
		t.Fatalf("expected blocking validation error")
	}
	if summary == nil || len(summary.TemplateValidationErrors) == 0 {
		t.Fatalf("expected template validation errors, got %#v", summary)
	}
}

func initPublishRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	runCmd(t, repoDir, "dolt", "init", "--name", "test", "--email", "test@example.com")
	runSQLFile(t, repoDir, filepath.Join("..", "..", "..", "sql", "001-create-tables.sql"))
	runQuery(t, repoDir, "main", "alter table packages add column sha256 varchar(64);")
	runCmd(t, repoDir, "dolt", "add", ".")
	runCmd(t, repoDir, "dolt", "commit", "-m", "init schema")
	runCmd(t, repoDir, "dolt", "branch", "develop")
	runCmd(t, repoDir, "dolt", "branch", "beta")
	return repoDir
}

func importFixtureToBranch(t *testing.T, repoDir, branch, fixtureDir string) {
	t.Helper()
	svc := importer.Service{Writer: dolt.NewCLIWriter(repoDir)}
	if _, err := svc.Import(context.Background(), importer.ImportRequest{
		PackageDir: fixtureDir,
		Branch:     branch,
	}); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
}

func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	runCmd(t, ".", "cp", "-R", src+string(filepath.Separator)+".", dst)
}
