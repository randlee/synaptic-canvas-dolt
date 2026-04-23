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
)

func TestImportWritesRowsToRealDoltRepo(t *testing.T) {
	if _, err := exec.LookPath("dolt"); err != nil {
		t.Skip("dolt not installed")
	}

	repoDir := t.TempDir()
	runCmd(t, repoDir, "dolt", "init", "--name", "test", "--email", "test@example.com")
	runSQLFile(t, repoDir, filepath.Join("..", "..", "..", "sql", "001-create-tables.sql"))
	runQuery(t, repoDir, "main", "alter table packages add column sha256 varchar(64);")
	runCmd(t, repoDir, "dolt", "add", ".")
	runCmd(t, repoDir, "dolt", "commit", "-m", "init schema")
	runCmd(t, repoDir, "dolt", "branch", "develop")

	svc := importer.Service{Writer: dolt.NewCLIWriter(repoDir)}
	fixtureDir := filepath.Join("..", "..", "pkg", "importer", "testdata", "basic-package")
	summary, err := svc.Import(context.Background(), importer.ImportRequest{
		PackageDir: fixtureDir,
		Branch:     "develop",
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if summary.PackageID != "sample-skill" {
		t.Fatalf("PackageID = %q", summary.PackageID)
	}

	pkgRow := runQuery(t, repoDir, "develop", "select id, version, sha256 from packages where id = 'sample-skill';")
	if !strings.Contains(pkgRow, "sample-skill") || !strings.Contains(pkgRow, "1.2.3") {
		t.Fatalf("unexpected package row output:\n%s", pkgRow)
	}

	fileCount := runQuery(t, repoDir, "develop", "select count(*) as c from package_files where package_id = 'sample-skill';")
	if !strings.Contains(fileCount, "4") {
		t.Fatalf("unexpected file count output:\n%s", fileCount)
	}
}

func runSQLFile(t *testing.T, repoDir, sqlFile string) {
	t.Helper()
	cmd := exec.Command("dolt", "sql")
	cmd.Dir = repoDir
	file, err := os.Open(sqlFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	cmd.Stdin = file
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dolt sql < %s failed: %v\n%s", sqlFile, err, string(output))
	}
}

func runQuery(t *testing.T, repoDir, branch, query string) string {
	t.Helper()
	cmd := exec.Command("dolt", "--branch", branch, "sql", "-q", query, "-r", "csv")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("query failed: %v\n%s", err, string(output))
	}
	return string(output)
}

func runCmd(t *testing.T, repoDir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
}
