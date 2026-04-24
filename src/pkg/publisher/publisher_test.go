package publisher

import (
	"context"
	"errors"
	"strings"
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

type mockPromoter struct {
	result    *PublishResult
	err       error
	packageID string
	from      string
	to        string
}

func (m *mockPromoter) PublishPackage(_ context.Context, packageID, fromBranch, toBranch string) (*PublishResult, error) {
	m.packageID = packageID
	m.from = fromBranch
	m.to = toBranch
	return m.result, m.err
}

func TestPublishRejectsSameBranch(t *testing.T) {
	t.Parallel()

	fileSHA := shaText("alpha")
	pkgSHA := computePackageSHA([]string{"skills/x/SKILL.md:" + fileSHA})
	svc := Service{
		Reader: mockReader{
			pkg: &models.Package{ID: "pkg", Version: "1.0.0", SHA256: &pkgSHA},
			files: []models.PackageFile{{
				PackageID: "pkg",
				DestPath:  "skills/x/SKILL.md",
				Content:   "alpha",
				SHA256:    fileSHA,
			}},
		},
		Promoter: &mockPromoter{},
	}

	_, err := svc.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "main",
		ToBranch:   "main",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot publish to same branch") {
		t.Fatalf("unexpected error: %v", err)
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
		Promoter: &mockPromoter{result: &PublishResult{}},
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

func TestPublishRunsPromoter(t *testing.T) {
	t.Parallel()

	desc := "desc"
	fileSHA := shaText("alpha")
	pkgSHA := computePackageSHA([]string{"skills/x/SKILL.md:" + fileSHA})
	promoter := &mockPromoter{result: &PublishResult{Hash: "abc", Message: "publish successful"}}
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
		Promoter: promoter,
	}

	summary, err := svc.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if summary.Publish == nil || summary.Publish.Hash != "abc" {
		t.Fatalf("unexpected publish result: %#v", summary.Publish)
	}
	if promoter.packageID != "pkg" || promoter.from != "develop" || promoter.to != "beta" {
		t.Fatalf("unexpected promoter request: %#v", promoter)
	}
}

func TestPublishPropagatesPromoterError(t *testing.T) {
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
		Promoter: &mockPromoter{err: errors.New("publish failed")},
	}

	_, err := svc.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err == nil || err.Error() != "publish failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublishBlocksWhenVerifyFindsCorruptFile(t *testing.T) {
	t.Parallel()

	desc := "desc"
	expectedFileSHA := shaText("expected")
	pkgSHA := computePackageSHA([]string{"skills/x/SKILL.md:" + expectedFileSHA})
	svc := Service{
		Reader: mockReader{
			pkg: &models.Package{ID: "pkg", Name: "pkg", Version: "1.0.0", Description: &desc, SHA256: &pkgSHA},
			files: []models.PackageFile{{
				PackageID: "pkg",
				DestPath:  "skills/x/SKILL.md",
				Content:   "corrupt",
				SHA256:    expectedFileSHA,
			}},
		},
		Promoter: &mockPromoter{result: &PublishResult{}},
	}

	summary, err := svc.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err == nil || !strings.Contains(err.Error(), "publish blocked: verify failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary == nil || summary.Verify == nil || summary.Verify.CorruptFiles != 1 {
		t.Fatalf("unexpected verify summary: %#v", summary)
	}
}

func TestPublishBlocksWhenPackageSHAIsMissing(t *testing.T) {
	t.Parallel()

	desc := "desc"
	fileSHA := shaText("alpha")
	svc := Service{
		Reader: mockReader{
			pkg: &models.Package{ID: "pkg", Name: "pkg", Version: "1.0.0", Description: &desc},
			files: []models.PackageFile{{
				PackageID: "pkg",
				DestPath:  "skills/x/SKILL.md",
				Content:   "alpha",
				SHA256:    fileSHA,
			}},
		},
		Promoter: &mockPromoter{result: &PublishResult{}},
	}

	_, err := svc.Publish(context.Background(), PublishRequest{
		PackageID:  "pkg",
		FromBranch: "develop",
		ToBranch:   "beta",
	})
	if err == nil || !strings.Contains(err.Error(), "has no aggregate SHA256") {
		t.Fatalf("unexpected error: %v", err)
	}
}
