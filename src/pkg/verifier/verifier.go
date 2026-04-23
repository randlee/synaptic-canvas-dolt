package verifier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

const (
	StatusOK      = "OK"
	StatusCorrupt = "CORRUPT"
)

// Reader loads package data from Dolt.
type Reader interface {
	GetPackage(ctx context.Context, id string) (*models.Package, error)
	GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error)
}

// Service verifies package integrity from stored Dolt content.
type Service struct {
	Reader Reader
}

// VerifyRequest defines one verify operation.
type VerifyRequest struct {
	PackageID string
	Branch    string
}

// FileResult describes the verification state for one stored file.
type FileResult struct {
	DestPath    string `json:"dest_path"`
	ExpectedSHA string `json:"expected_sha"`
	ActualSHA   string `json:"actual_sha"`
	Status      string `json:"status"`
}

// Summary is the command output.
type Summary struct {
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

// Verify recomputes file and package hashes from stored content.
func (s Service) Verify(ctx context.Context, req VerifyRequest) (*Summary, error) {
	if s.Reader == nil {
		return nil, fmt.Errorf("verify reader is required")
	}
	if strings.TrimSpace(req.PackageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}

	pkg, err := s.Reader.GetPackage(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading package %q: %w", req.PackageID, err)
	}
	if pkg == nil {
		return nil, fmt.Errorf("package %q not found", req.PackageID)
	}
	files, err := s.Reader.GetPackageFiles(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading package files for %q: %w", req.PackageID, err)
	}

	results := make([]FileResult, 0, len(files))
	aggregateParts := make([]string, 0, len(files))
	corruptFiles := 0
	for _, file := range files {
		actual := shaText(file.Content)
		status := StatusOK
		if actual != file.SHA256 {
			status = StatusCorrupt
			corruptFiles++
		}
		results = append(results, FileResult{
			DestPath:    file.DestPath,
			ExpectedSHA: file.SHA256,
			ActualSHA:   actual,
			Status:      status,
		})
		aggregateParts = append(aggregateParts, file.DestPath+":"+file.SHA256)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].DestPath < results[j].DestPath
	})

	aggregateSHA := computePackageSHA(aggregateParts)
	aggregateStatus := StatusOK
	expectedSHA := ""
	if pkg.SHA256 != nil {
		expectedSHA = *pkg.SHA256
	}
	if expectedSHA != "" && expectedSHA != aggregateSHA {
		aggregateStatus = StatusCorrupt
	}

	return &Summary{
		PackageID:       pkg.ID,
		Version:         pkg.Version,
		Branch:          req.Branch,
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
