package differ

import (
	"context"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestDiffReportsAddedRemovedModified(t *testing.T) {
	t.Parallel()

	left := dolt.NewMockClient()
	right := dolt.NewMockClient()
	left.AddPackage(dolt.NewTestPackage("pkg", "pkg", "1.0.0", nil))
	right.AddPackage(dolt.NewTestPackage("pkg", "pkg", "1.1.0", nil))
	left.AddFiles("pkg", []models.PackageFile{
		{DestPath: "a.md", SHA256: "a1", Content: "old"},
		{DestPath: "b.md", SHA256: "b1", Content: "same"},
		{DestPath: "c.md", SHA256: "c1", Content: "remove"},
	})
	right.AddFiles("pkg", []models.PackageFile{
		{DestPath: "a.md", SHA256: "a2", Content: "new"},
		{DestPath: "b.md", SHA256: "b1", Content: "same"},
		{DestPath: "d.md", SHA256: "d1", Content: "add"},
	})

	summary, err := Service{Reader1: left, Reader2: right}.Diff(context.Background(), DiffRequest{
		PackageID: "pkg",
		Branch1:   "develop",
		Branch2:   "main",
	})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if !summary.PackageChanged {
		t.Fatalf("PackageChanged = false, want true")
	}
	if len(summary.FileChanges) != 3 {
		t.Fatalf("len(FileChanges) = %d, want 3", len(summary.FileChanges))
	}
	if summary.FileChanges[0].Type != ChangeModified {
		t.Fatalf("first change type = %q", summary.FileChanges[0].Type)
	}
	if summary.FileChanges[1].Type != ChangeRemoved {
		t.Fatalf("second change type = %q", summary.FileChanges[1].Type)
	}
	if summary.FileChanges[2].Type != ChangeAdded {
		t.Fatalf("third change type = %q", summary.FileChanges[2].Type)
	}
}

func TestDiffMissingOnOneBranch(t *testing.T) {
	t.Parallel()

	left := dolt.NewMockClient()
	right := dolt.NewMockClient()
	right.AddPackage(dolt.NewTestPackage("pkg", "pkg", "1.0.0", nil))
	right.AddFiles("pkg", []models.PackageFile{{DestPath: "a.md", SHA256: "a1", Content: "only"}})

	summary, err := Service{Reader1: left, Reader2: right}.Diff(context.Background(), DiffRequest{
		PackageID: "pkg",
		Branch1:   "beta",
		Branch2:   "main",
	})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if len(summary.FileChanges) != 1 || summary.FileChanges[0].Type != ChangeAdded {
		t.Fatalf("unexpected file changes: %#v", summary.FileChanges)
	}
}
