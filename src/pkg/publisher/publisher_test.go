package publisher

import (
	"context"
	"errors"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

type mockReader struct {
	pkg       *models.Package
	files     []models.PackageFile
	questions []models.PackageQuestion
	err       error
}

func (m mockReader) GetPackage(_ context.Context, _ string) (*models.Package, error) {
	return m.pkg, m.err
}
func (m mockReader) GetPackageFiles(_ context.Context, _ string) ([]models.PackageFile, error) {
	return m.files, m.err
}
func (m mockReader) GetPackageQuestions(_ context.Context, _ string) ([]models.PackageQuestion, error) {
	return m.questions, m.err
}

type mockMerger struct {
	result *MergeResult
	err    error
}

func (m mockMerger) Merge(_ context.Context, _, _ string) (*MergeResult, error) {
	return m.result, m.err
}

func TestPublishRejectsSameBranch(t *testing.T) {
	t.Parallel()

	_, err := Service{}.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "main",
		ToBranch:   "main",
	})
	if err == nil || err.Error() != "publish reader is required" {
		t.Fatalf("unexpected error order: %v", err)
	}
}

func TestPublishBlocksOnTemplateErrors(t *testing.T) {
	t.Parallel()

	desc := "desc"
	fileSHA := shaText("{{ answers.missing }}")
	pkgSHA := computePackageSHA([]string{"skills/x/SKILL.md.j2:" + fileSHA})
	svc := Service{
		Reader: mockReader{
			pkg: &models.Package{ID: "pkg", Name: "pkg", Version: "1.0.0", Description: &desc, SHA256: &pkgSHA},
			files: []models.PackageFile{{
				PackageID:  "pkg",
				DestPath:   "skills/x/SKILL.md.j2",
				Content:    "{{ answers.missing }}",
				SHA256:     fileSHA,
				IsTemplate: true,
			}},
		},
		Merger: mockMerger{result: &MergeResult{}},
	}

	summary, err := svc.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err == nil || summary == nil {
		t.Fatalf("expected blocking error, got summary=%#v err=%v", summary, err)
	}
	if len(summary.TemplateValidationErrors) == 0 {
		t.Fatalf("expected template validation errors")
	}
}

func TestPublishRunsMerge(t *testing.T) {
	t.Parallel()

	desc := "desc"
	fileSHA := shaText("alpha")
	pkgSHA := computePackageSHA([]string{"skills/x/SKILL.md:" + fileSHA})
	svc := Service{
		Reader: mockReader{
			pkg: &models.Package{ID: "pkg", Name: "pkg", Version: "1.0.0", Description: &desc, SHA256: &pkgSHA},
			files: []models.PackageFile{{
				PackageID: "pkg",
				DestPath:  "skills/x/SKILL.md",
				Content:   "alpha",
				SHA256:    fileSHA,
			}},
		},
		Merger: mockMerger{result: &MergeResult{Hash: "abc", FastForward: true, Message: "merge successful"}},
	}

	summary, err := svc.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if summary.Merge == nil || summary.Merge.Hash != "abc" {
		t.Fatalf("unexpected merge result: %#v", summary.Merge)
	}
}

func TestPublishPropagatesMergeError(t *testing.T) {
	t.Parallel()

	desc := "desc"
	fileSHA := shaText("alpha")
	pkgSHA := computePackageSHA([]string{"skills/x/SKILL.md:" + fileSHA})
	svc := Service{
		Reader: mockReader{
			pkg: &models.Package{ID: "pkg", Name: "pkg", Version: "1.0.0", Description: &desc, SHA256: &pkgSHA},
			files: []models.PackageFile{{
				PackageID: "pkg",
				DestPath:  "skills/x/SKILL.md",
				Content:   "alpha",
				SHA256:    fileSHA,
			}},
		},
		Merger: mockMerger{err: errors.New("merge failed")},
	}

	_, err := svc.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err == nil || err.Error() != "merge failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}
