package importer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
)

type mockWriter struct {
	branchExists bool
	err          error
	req          *dolt.ImportPackageRequest
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
	return nil
}

func TestImportBuildsWriteRequest(t *testing.T) {
	t.Parallel()

	writer := &mockWriter{branchExists: true}
	svc := Service{Writer: writer}
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

	svc := Service{Writer: &mockWriter{branchExists: false}}
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

	svc := Service{Writer: &mockWriter{branchExists: true, err: errors.New("boom")}}
	_, err := svc.Import(context.Background(), ImportRequest{
		PackageDir: filepath.Join("testdata", "basic-package"),
		Branch:     "develop",
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected writer error, got %v", err)
	}
}
