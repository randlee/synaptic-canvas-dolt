package differ

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

const (
	ChangeAdded    = "ADDED"
	ChangeRemoved  = "REMOVED"
	ChangeModified = "MODIFIED"
)

// Reader loads package rows from Dolt.
type Reader interface {
	GetPackage(ctx context.Context, id string) (*models.Package, error)
	GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error)
}

// Service diffs package state across two branches.
type Service struct {
	Reader1 Reader
	Reader2 Reader
}

// DiffRequest defines one diff operation.
type DiffRequest struct {
	PackageID string
	Branch1   string
	Branch2   string
}

// FileChange describes a file-level delta between two branches.
type FileChange struct {
	DestPath string `json:"dest_path"`
	Type     string `json:"type"`
	SHA1     string `json:"sha1,omitempty"`
	SHA2     string `json:"sha2,omitempty"`
}

// Summary is the command output.
type Summary struct {
	PackageID      string       `json:"package_id"`
	Branch1        string       `json:"branch1"`
	Branch2        string       `json:"branch2"`
	Version1       string       `json:"version1,omitempty"`
	Version2       string       `json:"version2,omitempty"`
	PackageChanged bool         `json:"package_changed"`
	FileChanges    []FileChange `json:"file_changes"`
}

// Diff compares package state between two branches.
func (s Service) Diff(ctx context.Context, req DiffRequest) (*Summary, error) {
	if s.Reader1 == nil || s.Reader2 == nil {
		return nil, fmt.Errorf("diff readers are required")
	}
	if strings.TrimSpace(req.PackageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}

	pkg1, err := s.Reader1.GetPackage(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading %s from %s: %w", req.PackageID, req.Branch1, err)
	}
	pkg2, err := s.Reader2.GetPackage(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading %s from %s: %w", req.PackageID, req.Branch2, err)
	}
	if pkg1 == nil && pkg2 == nil {
		return nil, fmt.Errorf("package %q not found on either branch", req.PackageID)
	}

	files1, err := s.loadFiles(ctx, s.Reader1, req.PackageID, pkg1 != nil)
	if err != nil {
		return nil, fmt.Errorf("loading files from %s: %w", req.Branch1, err)
	}
	files2, err := s.loadFiles(ctx, s.Reader2, req.PackageID, pkg2 != nil)
	if err != nil {
		return nil, fmt.Errorf("loading files from %s: %w", req.Branch2, err)
	}

	changes := compareFiles(files1, files2)
	summary := &Summary{
		PackageID:      req.PackageID,
		Branch1:        req.Branch1,
		Branch2:        req.Branch2,
		PackageChanged: len(changes) > 0,
		FileChanges:    changes,
	}
	if pkg1 != nil {
		summary.Version1 = pkg1.Version
	}
	if pkg2 != nil {
		summary.Version2 = pkg2.Version
	}
	if pkg1 == nil || pkg2 == nil {
		summary.PackageChanged = true
	}
	if pkg1 != nil && pkg2 != nil && pkg1.Version != pkg2.Version {
		summary.PackageChanged = true
	}
	return summary, nil
}

func (s Service) loadFiles(ctx context.Context, reader Reader, packageID string, ok bool) ([]models.PackageFile, error) {
	if !ok {
		return nil, nil
	}
	return reader.GetPackageFiles(ctx, packageID)
}

func compareFiles(files1, files2 []models.PackageFile) []FileChange {
	left := make(map[string]models.PackageFile, len(files1))
	right := make(map[string]models.PackageFile, len(files2))
	keys := make(map[string]struct{}, len(files1)+len(files2))
	for _, file := range files1 {
		left[file.DestPath] = file
		keys[file.DestPath] = struct{}{}
	}
	for _, file := range files2 {
		right[file.DestPath] = file
		keys[file.DestPath] = struct{}{}
	}

	changes := make([]FileChange, 0)
	for path := range keys {
		file1, ok1 := left[path]
		file2, ok2 := right[path]
		switch {
		case ok1 && !ok2:
			changes = append(changes, FileChange{DestPath: path, Type: ChangeRemoved, SHA1: file1.SHA256})
		case !ok1 && ok2:
			changes = append(changes, FileChange{DestPath: path, Type: ChangeAdded, SHA2: file2.SHA256})
		case ok1 && ok2 && (file1.SHA256 != file2.SHA256 || file1.Content != file2.Content):
			changes = append(changes, FileChange{DestPath: path, Type: ChangeModified, SHA1: file1.SHA256, SHA2: file2.SHA256})
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].DestPath < changes[j].DestPath
	})
	return changes
}
