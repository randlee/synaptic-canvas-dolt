package verifier

import (
	"context"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestVerifyOK(t *testing.T) {
	t.Parallel()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("pkg-1", "pkg-1", "1.0.0", nil)
	files := []models.PackageFile{
		{PackageID: "pkg-1", DestPath: "agents/a.md", Content: "alpha"},
		{PackageID: "pkg-1", DestPath: "commands/b.md", Content: "beta"},
	}
	for i := range files {
		files[i].SHA256 = shaText(files[i].Content)
	}
	aggregate := computePackageSHA([]string{
		files[0].DestPath + ":" + files[0].SHA256,
		files[1].DestPath + ":" + files[1].SHA256,
	})
	pkg.SHA256 = &aggregate
	mock.AddPackage(pkg)
	mock.AddFiles("pkg-1", files)

	summary, err := Service{Reader: mock}.Verify(context.Background(), VerifyRequest{
		PackageID: "pkg-1",
		Branch:    "main",
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if summary.CorruptFiles != 0 {
		t.Fatalf("CorruptFiles = %d, want 0", summary.CorruptFiles)
	}
	if summary.AggregateStatus != StatusOK {
		t.Fatalf("AggregateStatus = %q, want %q", summary.AggregateStatus, StatusOK)
	}
}

func TestVerifyCorruptFileAndAggregate(t *testing.T) {
	t.Parallel()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("pkg-1", "pkg-1", "1.0.0", nil)
	badPackageSHA := "bad"
	pkg.SHA256 = &badPackageSHA
	mock.AddPackage(pkg)
	mock.AddFiles("pkg-1", []models.PackageFile{
		{
			PackageID: "pkg-1",
			DestPath:  "agents/a.md",
			Content:   "alpha",
			SHA256:    "wrong",
		},
	})

	summary, err := Service{Reader: mock}.Verify(context.Background(), VerifyRequest{
		PackageID: "pkg-1",
		Branch:    "develop",
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if summary.CorruptFiles != 1 {
		t.Fatalf("CorruptFiles = %d, want 1", summary.CorruptFiles)
	}
	if summary.FileResults[0].Status != StatusCorrupt {
		t.Fatalf("FileResults[0].Status = %q", summary.FileResults[0].Status)
	}
	if summary.AggregateStatus != StatusCorrupt {
		t.Fatalf("AggregateStatus = %q, want %q", summary.AggregateStatus, StatusCorrupt)
	}
}

func TestVerifyFailsWhenPackageSHAIsMissing(t *testing.T) {
	t.Parallel()

	mock := dolt.NewMockClient()
	mock.AddPackage(&models.Package{ID: "pkg-1", Version: "1.0.0"})
	mock.AddFiles("pkg-1", []models.PackageFile{{
		PackageID: "pkg-1",
		DestPath:  "agents/a.md",
		Content:   "alpha",
		SHA256:    shaText("alpha"),
	}})

	_, err := Service{Reader: mock}.Verify(context.Background(), VerifyRequest{
		PackageID: "pkg-1",
		Branch:    "main",
	})
	if err == nil || err.Error() != "package pkg-1 is missing aggregate SHA256" {
		t.Fatalf("unexpected error: %v", err)
	}
}
