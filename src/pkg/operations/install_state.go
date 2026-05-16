package operations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

// TrackedInstall identifies one lockfile-tracked install and its origin scope.
type TrackedInstall struct {
	Record installer.InstallRecord
	Source string
}

// ExpectedHashOptions supplies cmd-owned integrations needed to resolve the
// authoritative SHA source without creating a package cycle back into cmd/.
type ExpectedHashOptions struct {
	ResolveRepoRoot    func() (string, error)
	FetchCatalog       func(context.Context, string, string) ([]catalog.CatalogEntry, error)
	WriteCatalogCaches func(string, string, string, []catalog.CatalogEntry, time.Time) ([]string, error)
	Now                func() time.Time
	DisplayCatalogPath func(string) string
}

// LoadTrackedInstalls loads both project and global lockfiles, preserving the
// stable package/scope sort order used by read commands and mutations.
func LoadTrackedInstalls(repoRoot string) ([]TrackedInstall, error) {
	localLock, err := installer.LoadManifestLock(repoRoot)
	if err != nil {
		return nil, err
	}
	installs := make([]TrackedInstall, 0, len(localLock.Installs)+4)
	for _, record := range localLock.Installs {
		installs = append(installs, TrackedInstall{Record: record, Source: "project"})
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	globalLock, err := installer.LoadManifestLock(home)
	if err != nil {
		return nil, err
	}
	for _, record := range globalLock.Installs {
		installs = append(installs, TrackedInstall{Record: record, Source: "global"})
	}

	sort.Slice(installs, func(i, j int) bool {
		if installs[i].Record.Package != installs[j].Record.Package {
			return installs[i].Record.Package < installs[j].Record.Package
		}
		return scopeSortRank(installs[i].Record.InstallScope) < scopeSortRank(installs[j].Record.InstallScope)
	})
	return installs, nil
}

// FilterInstalls narrows tracked installs by package id when one is supplied.
func FilterInstalls(installs []TrackedInstall, packageID string) []TrackedInstall {
	if packageID == "" {
		return installs
	}
	filtered := make([]TrackedInstall, 0, len(installs))
	for _, install := range installs {
		if install.Record.Package == packageID {
			filtered = append(filtered, install)
		}
	}
	return filtered
}

// FilterInstallsByScope narrows tracked installs to one scope or returns both.
func FilterInstallsByScope(installs []TrackedInstall, scope string) []TrackedInstall {
	if scope == "" || scope == "both" {
		return installs
	}
	filtered := make([]TrackedInstall, 0, len(installs))
	for _, install := range installs {
		if install.Record.InstallScope == scope {
			filtered = append(filtered, install)
		}
	}
	return filtered
}

// ResolveExpectedHashes resolves the expected package SHAs from the local
// project cache, machine cache, live Dolt fetch, or finally the lockfile.
func ResolveExpectedHashes(ctx context.Context, record installer.InstallRecord, opts ExpectedHashOptions) ([]integrity.FileHash, []string, error) {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	displayCatalogPath := opts.DisplayCatalogPath
	if displayCatalogPath == nil {
		displayCatalogPath = func(path string) string { return path }
	}

	repoRoot := record.InstallSite
	if repoRoot == "" {
		if opts.ResolveRepoRoot == nil {
			return nil, nil, errors.New("resolve repo root callback is required when install site is empty")
		}
		root, err := opts.ResolveRepoRoot()
		if err != nil {
			return nil, nil, err
		}
		repoRoot = root
	}
	branch := record.Branch
	if branch == "" {
		branch = "main"
	}

	localPath := catalog.ProjectPath(repoRoot, branch)
	if cat, warnings, ok, err := loadValidationCatalog(localPath, now); err != nil {
		return nil, nil, err
	} else if ok {
		return expectedFromCatalog(record, cat, warnings)
	}

	machinePath, err := catalog.MachinePath(branch)
	if err != nil {
		return nil, nil, err
	}
	if cat, warnings, ok, err := loadValidationCatalog(machinePath, now); err != nil {
		return nil, nil, err
	} else if ok {
		warnings = append([]string{"project catalog absent; using machine catalog " + displayCatalogPath(machinePath)}, warnings...)
		return expectedFromCatalog(record, cat, warnings)
	}

	if opts.FetchCatalog == nil {
		return expectedFromLockfile(record), []string{"catalog unavailable and Dolt offline; using lockfile SHAs (may be stale - run sc catalog update when online)"}, nil
	}
	entries, err := opts.FetchCatalog(ctx, repoRoot, branch)
	if err == nil {
		fetchedAt := now()
		if opts.WriteCatalogCaches != nil {
			if _, writeErr := opts.WriteCatalogCaches(repoRoot, branch, "both", entries, fetchedAt); writeErr != nil {
				warnings := []string{"catalog fetched but cache write failed: " + writeErr.Error()}
				return expectedFromCatalog(record, catalog.Catalog{
					Meta:    catalog.CatalogMeta{Branch: branch, FetchedAt: fetchedAt, SchemaVersion: catalog.SchemaVersion},
					Entries: entries,
				}, warnings)
			}
		}
		return expectedFromCatalog(record, catalog.Catalog{
			Meta:    catalog.CatalogMeta{Branch: branch, FetchedAt: fetchedAt, SchemaVersion: catalog.SchemaVersion},
			Entries: entries,
		}, nil)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, nil, err
	}

	warnings := []string{"catalog unavailable and Dolt offline; using lockfile SHAs (may be stale - run sc catalog update when online)"}
	return expectedFromLockfile(record), warnings, nil
}

// NormalizeRecordPath converts an install record path into the stable
// install-root-relative path used by catalogs, hashes, and validation output.
func NormalizeRecordPath(record installer.InstallRecord, path string) string {
	slashPath := filepath.ToSlash(path)
	slashRoot := filepath.ToSlash(record.InstallRoot)
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(record.InstallRoot, path); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	if slashRoot != "" && strings.HasPrefix(slashPath, slashRoot+"/") {
		return strings.TrimPrefix(slashPath, slashRoot+"/")
	}
	return slashPath
}

func scopeSortRank(scope string) int {
	if scope == "project" {
		return 0
	}
	if scope == "global" {
		return 1
	}
	return 2
}

func loadValidationCatalog(path string, now func() time.Time) (catalog.Catalog, []string, bool, error) {
	cat, warnings, err := catalog.Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return catalog.Catalog{}, nil, false, nil
	}
	if err != nil {
		return catalog.Catalog{}, nil, false, err
	}
	if !cat.Meta.FetchedAt.IsZero() && now().Sub(cat.Meta.FetchedAt) > catalog.StaleAfter {
		warnings = append(warnings, "catalog is older than 24h; run sc catalog update")
	}
	return cat, warnings, true, nil
}

func expectedFromCatalog(record installer.InstallRecord, cat catalog.Catalog, warnings []string) ([]integrity.FileHash, []string, error) {
	if len(cat.Entries) == 0 {
		warnings = append(warnings, "catalog is empty; skipping authoritative SHA check")
		return expectedFromCurrentFiles(record), warnings, nil
	}
	entries := matchingCatalogEntries(record, cat.Entries)
	if len(entries) == 0 {
		warnings = append(warnings, "catalog has no entries for installed package/version; using lockfile SHAs")
		return expectedFromLockfile(record), warnings, nil
	}

	expected := make([]integrity.FileHash, 0, len(record.Files))
	for path, lockSHA := range record.Files {
		rel := NormalizeRecordPath(record, path)
		sha := matchingCatalogSHA(rel, entries)
		if sha == "" {
			sha = lockSHA
			warnings = append(warnings, "catalog missing SHA for "+rel+"; using lockfile SHA")
		}
		expected = append(expected, integrity.FileHash{DestPath: rel, SHA256: sha})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].DestPath < expected[j].DestPath })
	return expected, warnings, nil
}

func expectedFromLockfile(record installer.InstallRecord) []integrity.FileHash {
	expected := make([]integrity.FileHash, 0, len(record.Files))
	for path, sha := range record.Files {
		expected = append(expected, integrity.FileHash{DestPath: NormalizeRecordPath(record, path), SHA256: sha})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].DestPath < expected[j].DestPath })
	return expected
}

func expectedFromCurrentFiles(record installer.InstallRecord) []integrity.FileHash {
	expected := make([]integrity.FileHash, 0, len(record.Files))
	for path, fallbackSHA := range record.Files {
		rel := NormalizeRecordPath(record, path)
		sha, err := integrity.ComputeFileSHA256(filepath.Join(record.InstallRoot, filepath.FromSlash(rel)))
		if err != nil {
			sha = fallbackSHA
		}
		expected = append(expected, integrity.FileHash{DestPath: rel, SHA256: sha})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].DestPath < expected[j].DestPath })
	return expected
}

func matchingCatalogEntries(record installer.InstallRecord, entries []catalog.CatalogEntry) []catalog.CatalogEntry {
	matches := make([]catalog.CatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.PackageID == record.Package && entry.Version == record.Version {
			matches = append(matches, entry)
		}
	}
	return matches
}

func matchingCatalogSHA(rel string, entries []catalog.CatalogEntry) string {
	for _, entry := range entries {
		docPath := filepath.ToSlash(entry.DocPath)
		if docPath == rel || strings.HasSuffix(rel, "/"+docPath) || filepath.Base(docPath) == filepath.Base(rel) {
			return entry.SHA256
		}
	}
	return ""
}
