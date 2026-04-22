package exporter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/manifest"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	pluginmanifest "github.com/randlee/synaptic-canvas-dolt/pkg/plugin"
)

// Reader loads package data from Dolt.
type Reader interface {
	GetPackage(ctx context.Context, id string) (*models.Package, error)
	GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error)
	GetPackageDeps(ctx context.Context, packageID string) ([]models.PackageDep, error)
	GetPackageHooks(ctx context.Context, packageID string) ([]models.PackageHook, error)
	GetPackageQuestions(ctx context.Context, packageID string) ([]models.PackageQuestion, error)
}

// Service exports packages from Dolt to the filesystem.
type Service struct {
	Reader Reader
}

// ExportRequest defines one export operation.
type ExportRequest struct {
	PackageID  string
	OutputDir  string
	Branch     string
	WriteFiles bool
}

// Summary is the command output.
type Summary struct {
	PackageID           string   `json:"package_id"`
	Version             string   `json:"version"`
	Branch              string   `json:"branch"`
	OutputDir           string   `json:"output_dir"`
	FilesWritten        int      `json:"files_written"`
	FileSHAVerified     int      `json:"file_sha_verified"`
	PluginReconstructed bool     `json:"plugin_reconstructed"`
	PackageSHA256       string   `json:"package_sha256"`
	Warnings            []string `json:"warnings,omitempty"`
}

// Export reconstructs and writes a package from Dolt to the filesystem.
func (s Service) Export(ctx context.Context, req ExportRequest) (*Summary, error) {
	if s.Reader == nil {
		return nil, fmt.Errorf("export reader is required")
	}
	if strings.TrimSpace(req.PackageID) == "" {
		return nil, fmt.Errorf("package id is required")
	}
	if strings.TrimSpace(req.OutputDir) == "" {
		return nil, fmt.Errorf("output dir is required")
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
	deps, err := s.Reader.GetPackageDeps(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading package deps for %q: %w", req.PackageID, err)
	}
	hooks, err := s.Reader.GetPackageHooks(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading package hooks for %q: %w", req.PackageID, err)
	}
	questions, err := s.Reader.GetPackageQuestions(ctx, req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("loading package questions for %q: %w", req.PackageID, err)
	}

	manifestContent, err := manifest.Reconstruct(pkg, files, deps, hooks, questions)
	if err != nil {
		return nil, fmt.Errorf("reconstructing manifest: %w", err)
	}

	packageDir := filepath.Join(req.OutputDir, pkg.ID)
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	if err := writeTextFile(filepath.Join(packageDir, "manifest.yaml"), manifestContent); err != nil {
		return nil, err
	}

	written := 1
	verified := 0
	hasPlugin := false
	aggregateParts := make([]string, 0, len(files))

	for _, file := range files {
		filePath := filepath.Join(packageDir, filepath.FromSlash(file.DestPath))
		if err := writeTextFile(filePath, file.Content); err != nil {
			return nil, err
		}
		written++

		actualSHA := shaText(file.Content)
		if actualSHA != file.SHA256 {
			return nil, fmt.Errorf("sha mismatch for %s: expected %s got %s", file.DestPath, file.SHA256, actualSHA)
		}
		verified++
		aggregateParts = append(aggregateParts, file.DestPath+":"+actualSHA)

		if file.DestPath == ".claude-plugin/plugin.json" {
			hasPlugin = true
		}
	}

	reconstructedPlugin := false
	if !hasPlugin {
		pluginContent, err := pluginmanifest.Reconstruct(pkg, files)
		if err != nil {
			return nil, fmt.Errorf("reconstructing plugin: %w", err)
		}
		if err := writeTextFile(filepath.Join(packageDir, ".claude-plugin", "plugin.json"), pluginContent); err != nil {
			return nil, err
		}
		written++
		reconstructedPlugin = true
	}

	aggregate := computePackageSHA(aggregateParts)
	if pkg.SHA256 != nil && *pkg.SHA256 != "" && aggregate != *pkg.SHA256 {
		return nil, fmt.Errorf("aggregate sha mismatch for %s: expected %s got %s", pkg.ID, *pkg.SHA256, aggregate)
	}

	return &Summary{
		PackageID:           pkg.ID,
		Version:             pkg.Version,
		Branch:              req.Branch,
		OutputDir:           packageDir,
		FilesWritten:        written,
		FileSHAVerified:     verified,
		PluginReconstructed: reconstructedPlugin,
		PackageSHA256:       aggregate,
	}, nil
}

func writeTextFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating parent dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
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
