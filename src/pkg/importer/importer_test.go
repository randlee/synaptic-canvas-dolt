package importer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

type mockWriter struct {
	branchExists bool
	err          error
	req          *dolt.ImportPackageRequest
	writes       int
}

func (m *mockWriter) BranchExists(_ context.Context, _ string) (bool, error) {
	return m.branchExists, m.err
}

func (m *mockWriter) ImportPackage(_ context.Context, req dolt.ImportPackageRequest) error {
	if m.err != nil {
		return m.err
	}
	copied := req
	m.req = &copied
	m.writes++
	return nil
}

func TestImportBuildsWriteRequest(t *testing.T) {
	t.Parallel()

	writer := &mockWriter{branchExists: true}
	svc := Service{Writer: writer, Client: dolt.NewMockClient()}
	summary, err := svc.Import(context.Background(), ImportRequest{
		PackageDir: filepath.Join("testdata", "basic-package"),
		Branch:     "develop",
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if summary.PackageID != "sample-skill" {
		t.Fatalf("PackageID = %q", summary.PackageID)
	}
	if writer.req == nil {
		t.Fatal("writer was not called")
	}
	if writer.req.Package.ID != "sample-skill" {
		t.Fatalf("writer package id = %q", writer.req.Package.ID)
	}
	if got := len(writer.req.Files); got != 4 {
		t.Fatalf("files imported = %d, want 4", got)
	}
	if got := len(writer.req.Questions); got != 2 {
		t.Fatalf("questions imported = %d, want 2", got)
	}
	if got := len(summary.TemplateValidationWarnings); got != 1 {
		t.Fatalf("warnings = %d, want 1", got)
	}
	if !strings.Contains(summary.TemplateValidationWarnings[0], "declared but not referenced") {
		t.Fatalf("unexpected warning: %+v", summary.TemplateValidationWarnings)
	}
}

func TestImportFailsWhenBranchMissing(t *testing.T) {
	t.Parallel()

	svc := Service{Writer: &mockWriter{branchExists: false}, Client: dolt.NewMockClient()}
	_, err := svc.Import(context.Background(), ImportRequest{
		PackageDir: filepath.Join("testdata", "basic-package"),
		Branch:     "missing",
	})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected missing branch error, got %v", err)
	}
}

func TestImportPropagatesWriterError(t *testing.T) {
	t.Parallel()

	svc := Service{Writer: &mockWriter{branchExists: true, err: errors.New("boom")}, Client: dolt.NewMockClient()}
	_, err := svc.Import(context.Background(), ImportRequest{
		PackageDir: filepath.Join("testdata", "basic-package"),
		Branch:     "develop",
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected writer error, got %v", err)
	}
}

func TestImportIdenticalReimportAllowed(t *testing.T) {
	t.Parallel()

	scanned, _, err := scanPackage(filepath.Join("testdata", "basic-package"))
	if err != nil {
		t.Fatalf("scanPackage() error = %v", err)
	}
	client := dolt.NewMockClient()
	client.AddPackage(&scanned.Package)
	client.AddFiles(scanned.Package.ID, scanned.Files)
	writer := &mockWriter{branchExists: true}
	svc := Service{Writer: writer, Client: client}

	if _, err := svc.Import(context.Background(), ImportRequest{
		PackageDir: filepath.Join("testdata", "basic-package"),
		Branch:     "develop",
	}); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if writer.writes != 1 {
		t.Fatalf("writer writes = %d, want 1", writer.writes)
	}
}

func TestImportSHACollisionRejectedBeforeWrite(t *testing.T) {
	t.Parallel()

	scanned, _, err := scanPackage(filepath.Join("testdata", "basic-package"))
	if err != nil {
		t.Fatalf("scanPackage() error = %v", err)
	}
	existingFiles := append([]models.PackageFile(nil), scanned.Files...)
	existingFiles[0].SHA256 = "existing-different-sha"
	client := dolt.NewMockClient()
	client.AddPackage(&scanned.Package)
	client.AddFiles(scanned.Package.ID, existingFiles)
	writer := &mockWriter{branchExists: true}
	svc := Service{Writer: writer, Client: client}

	_, err = svc.Import(context.Background(), ImportRequest{
		PackageDir: filepath.Join("testdata", "basic-package"),
		Branch:     "develop",
	})
	var collision *SHACollisionError
	if !errors.As(err, &collision) {
		t.Fatalf("Import() error = %v, want SHACollisionError", err)
	}
	if writer.writes != 0 || writer.req != nil {
		t.Fatalf("writer should not be called on collision: writes=%d req=%+v", writer.writes, writer.req)
	}
	if collision.File != existingFiles[0].DestPath || collision.ExistingSHA != "existing-different-sha" || collision.IncomingSHA != scanned.Files[0].SHA256 {
		t.Fatalf("unexpected collision metadata: %+v", collision)
	}
	message := err.Error()
	for _, want := range []string{existingFiles[0].DestPath, "existing-different-sha", scanned.Files[0].SHA256} {
		if !strings.Contains(message, want) {
			t.Fatalf("collision message %q missing %q", message, want)
		}
	}
}

func TestImportVersionBumpAllowsDifferentSHA(t *testing.T) {
	t.Parallel()

	scanned, _, err := scanPackage(filepath.Join("testdata", "basic-package"))
	if err != nil {
		t.Fatalf("scanPackage() error = %v", err)
	}
	existingPackage := scanned.Package
	existingPackage.Version = "1.2.2"
	existingFiles := append([]models.PackageFile(nil), scanned.Files...)
	existingFiles[0].SHA256 = "previous-version-sha"
	client := dolt.NewMockClient()
	client.AddPackage(&existingPackage)
	client.AddFiles(existingPackage.ID, existingFiles)
	writer := &mockWriter{branchExists: true}
	svc := Service{Writer: writer, Client: client}

	if _, err := svc.Import(context.Background(), ImportRequest{
		PackageDir: filepath.Join("testdata", "basic-package"),
		Branch:     "develop",
	}); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if writer.writes != 1 {
		t.Fatalf("writer writes = %d, want 1", writer.writes)
	}
}

func TestImportReadFailureAbortsBeforeWrite(t *testing.T) {
	t.Parallel()

	client := dolt.NewMockClient()
	client.FilesErr = errors.New("read failed")
	writer := &mockWriter{branchExists: true}
	svc := Service{Writer: writer, Client: client}

	_, err := svc.Import(context.Background(), ImportRequest{
		PackageDir: filepath.Join("testdata", "basic-package"),
		Branch:     "develop",
	})
	if err == nil || !strings.Contains(err.Error(), "catalog check failed: read failed; import aborted to protect SHA immutability") {
		t.Fatalf("Import() error = %v, want catalog check failure", err)
	}
	if writer.writes != 0 {
		t.Fatalf("writer writes = %d, want 0", writer.writes)
	}
}
