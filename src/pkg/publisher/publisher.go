package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/template"
)

// Reader loads source package data from Dolt.
type Reader interface {
	GetPackage(ctx context.Context, id string) (*models.Package, error)
	GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error)
	GetPackageQuestions(ctx context.Context, packageID string) ([]models.PackageQuestion, error)
}

// Merger promotes one branch into another.
type Merger interface {
	Merge(ctx context.Context, fromBranch, toBranch string) (*MergeResult, error)
}

// MergeResult captures Dolt merge metadata.
type MergeResult struct {
	Hash        string `json:"hash,omitempty"`
	FastForward bool   `json:"fast_forward"`
	Conflicts   int    `json:"conflicts"`
	Message     string `json:"message,omitempty"`
}

// PublishRequest defines one publish operation.
type PublishRequest struct {
	PackageID  string
	FromBranch string
	ToBranch   string
}

// Summary is the publish command output.
type Summary struct {
	PackageID                string             `json:"package_id"`
	Version                  string             `json:"version"`
	FromBranch               string             `json:"from_branch"`
	ToBranch                 string             `json:"to_branch"`
	Verify                   *VerifySummary     `json:"verify"`
	TemplateValidationErrors []template.Finding `json:"template_validation_errors,omitempty"`
	TemplateWarnings         []template.Finding `json:"template_validation_warnings,omitempty"`
	Merge                    *MergeResult       `json:"merge,omitempty"`
}

type FileResult struct {
	DestPath    string `json:"dest_path"`
	ExpectedSHA string `json:"expected_sha"`
	ActualSHA   string `json:"actual_sha"`
	Status      string `json:"status"`
}

type VerifySummary struct {
	PackageID       string       `json:"package_id"`
	Version         string       `json:"version"`
	Branch          string       `json:"branch"`
	FilesChecked    int          `json:"files_checked"`
	CorruptFiles    int          `json:"corrupt_files"`
	AggregateStatus string       `json:"aggregate_status"`
	AggregateSHA    string       `json:"aggregate_sha"`
	ExpectedSHA     string       `json:"expected_package_sha,omitempty"`
	FileResults     []FileResult `json:"file_results"`
}

// Service publishes a package from one branch to another.
type Service struct {
	Reader Reader
	Merger Merger
}

// Publish validates and merges a package promotion.
func (s Service) Publish(ctx context.Context, req PublishRequest) (*Summary, error) {
	if s.Reader == nil {
		return nil, fmt.Errorf("publish reader is required")
	}
	if s.Merger == nil {
		return nil, fmt.Errorf("publish merger is required")
	}
	if strings.TrimSpace(req.PackageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}
	if req.FromBranch == "" || req.ToBranch == "" {
		return nil, fmt.Errorf("--from and --to are required")
	}
	if req.FromBranch == req.ToBranch {
		return nil, fmt.Errorf("cannot publish to same branch")
	}

	pkg, err := s.Reader.GetPackage(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading package %q from %s: %w", req.PackageID, req.FromBranch, err)
	}
	if pkg == nil {
		return nil, fmt.Errorf("package %q not found on %s", req.PackageID, req.FromBranch)
	}

	verifySummary, err := verifyPackage(ctx, s.Reader, req.PackageID, req.FromBranch)
	if err != nil {
		return nil, err
	}
	if verifySummary.CorruptFiles > 0 || verifySummary.AggregateStatus == "CORRUPT" {
		return &Summary{
			PackageID:  pkg.ID,
			Version:    pkg.Version,
			FromBranch: req.FromBranch,
			ToBranch:   req.ToBranch,
			Verify:     verifySummary,
		}, fmt.Errorf("publish blocked: verify failed")
	}

	files, err := s.Reader.GetPackageFiles(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading package files for %q: %w", req.PackageID, err)
	}
	questions, err := s.Reader.GetPackageQuestions(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading package questions for %q: %w", req.PackageID, err)
	}

	templateFiles := map[string]string{}
	for _, file := range files {
		if file.IsTemplate {
			templateFiles[file.DestPath] = file.Content
		}
	}
	questionIDs := make([]string, 0, len(questions))
	for _, question := range questions {
		questionIDs = append(questionIDs, question.QuestionID)
	}
	report := template.Validate(templateFiles, questionIDs)
	if len(report.Errors) > 0 {
		return &Summary{
			PackageID:                pkg.ID,
			Version:                  pkg.Version,
			FromBranch:               req.FromBranch,
			ToBranch:                 req.ToBranch,
			Verify:                   verifySummary,
			TemplateValidationErrors: report.Errors,
			TemplateWarnings:         report.Warnings,
		}, fmt.Errorf("publish blocked: template validation failed")
	}

	mergeResult, err := s.Merger.Merge(ctx, req.FromBranch, req.ToBranch)
	if err != nil {
		return nil, err
	}

	return &Summary{
		PackageID:                pkg.ID,
		Version:                  pkg.Version,
		FromBranch:               req.FromBranch,
		ToBranch:                 req.ToBranch,
		Verify:                   verifySummary,
		TemplateValidationErrors: report.Errors,
		TemplateWarnings:         report.Warnings,
		Merge:                    mergeResult,
	}, nil
}

func verifyPackage(ctx context.Context, reader Reader, packageID, branch string) (*VerifySummary, error) {
	pkg, err := reader.GetPackage(ctx, packageID)
	if err != nil {
		return nil, fmt.Errorf("loading package %q: %w", packageID, err)
	}
	if pkg == nil {
		return nil, fmt.Errorf("package %q not found", packageID)
	}
	files, err := reader.GetPackageFiles(ctx, packageID)
	if err != nil {
		return nil, fmt.Errorf("loading package files for %q: %w", packageID, err)
	}

	results := make([]FileResult, 0, len(files))
	aggregateParts := make([]string, 0, len(files))
	corruptFiles := 0
	for _, file := range files {
		actual := shaText(file.Content)
		status := "OK"
		if actual != file.SHA256 {
			status = "CORRUPT"
			corruptFiles++
		}
		results = append(results, FileResult{
			DestPath:    file.DestPath,
			ExpectedSHA: file.SHA256,
			ActualSHA:   actual,
			Status:      status,
		})
		aggregateParts = append(aggregateParts, file.DestPath+":"+actual)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].DestPath < results[j].DestPath
	})

	aggregateSHA := computePackageSHA(aggregateParts)
	aggregateStatus := "OK"
	expectedSHA := ""
	if pkg.SHA256 != nil {
		expectedSHA = *pkg.SHA256
	}
	if expectedSHA != "" && expectedSHA != aggregateSHA {
		aggregateStatus = "CORRUPT"
	}

	return &VerifySummary{
		PackageID:       pkg.ID,
		Version:         pkg.Version,
		Branch:          branch,
		FilesChecked:    len(files),
		CorruptFiles:    corruptFiles,
		AggregateStatus: aggregateStatus,
		AggregateSHA:    aggregateSHA,
		ExpectedSHA:     expectedSHA,
		FileResults:     results,
	}, nil
}

func shaText(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func computePackageSHA(parts []string) string {
	sorted := append([]string(nil), parts...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}
