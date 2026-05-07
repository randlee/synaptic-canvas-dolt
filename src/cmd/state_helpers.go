package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

type trackedInstall struct {
	Record installer.InstallRecord
	Source string
}

type validatedInstall struct {
	Package           string          `json:"package"`
	Version           string          `json:"version"`
	Branch            string          `json:"branch"`
	Scope             string          `json:"scope"`
	InstallRoot       string          `json:"install_root"`
	InstallSite       string          `json:"install_site"`
	Files             []validatedFile `json:"items"`
	AggregateExpected string          `json:"aggregate_expected"`
	AggregateActual   string          `json:"aggregate_actual,omitempty"`
	AggregatePass     bool            `json:"aggregate_pass"`
	AggregateStatus   string          `json:"aggregate_status"`
	Warnings          []string        `json:"warnings,omitempty"`
	Pass              bool            `json:"pass"`
	Status            string          `json:"status"`
}

type ValidationSeverity string

const (
	ValidationSeverityInfo     ValidationSeverity = "info"
	ValidationSeverityWarn     ValidationSeverity = "warn"
	ValidationSeverityError    ValidationSeverity = "error"
	ValidationSeverityCritical ValidationSeverity = "critical"
)

type validatedFile struct {
	Path     string             `json:"path"`
	Status   string             `json:"status"`
	Severity ValidationSeverity `json:"severity"`
	Error    string             `json:"error,omitempty"`
}

var snapshotNow = func() time.Time { return time.Now().UTC() }
var snapshotGitRemoteURL = gitRemoteURL
var validateCatalogFetch = defaultValidateCatalogFetch

func loadTrackedInstalls(repoRoot string) ([]trackedInstall, error) {
	localLock, err := installer.LoadManifestLock(repoRoot)
	if err != nil {
		return nil, err
	}
	installs := make([]trackedInstall, 0, len(localLock.Installs)+4)
	for _, record := range localLock.Installs {
		installs = append(installs, trackedInstall{Record: record, Source: "project"})
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
		installs = append(installs, trackedInstall{Record: record, Source: "global"})
	}

	sort.Slice(installs, func(i, j int) bool {
		if installs[i].Record.Package != installs[j].Record.Package {
			return installs[i].Record.Package < installs[j].Record.Package
		}
		return scopeSortRank(installs[i].Record.InstallScope) < scopeSortRank(installs[j].Record.InstallScope)
	})
	return installs, nil
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

func filterInstalls(installs []trackedInstall, packageID string) []trackedInstall {
	if packageID == "" {
		return installs
	}
	filtered := make([]trackedInstall, 0, len(installs))
	for _, install := range installs {
		if install.Record.Package == packageID {
			filtered = append(filtered, install)
		}
	}
	return filtered
}

func filterInstallsByScope(installs []trackedInstall, scope string) []trackedInstall {
	if scope == "" || scope == "both" {
		return installs
	}
	filtered := make([]trackedInstall, 0, len(installs))
	for _, install := range installs {
		if install.Record.InstallScope == scope {
			filtered = append(filtered, install)
		}
	}
	return filtered
}

func validateTrackedInstall(record installer.InstallRecord) (validatedInstall, error) {
	expected, warnings, err := resolveExpectedHashes(record)
	if err != nil {
		return validatedInstall{}, err
	}

	results, err := integrity.VerifyPackage(expected, record.InstallRoot)
	if err != nil {
		return validatedInstall{}, err
	}

	summary := validatedInstall{
		Package:           record.Package,
		Version:           record.Version,
		Branch:            record.Branch,
		Scope:             record.InstallScope,
		InstallRoot:       record.InstallRoot,
		InstallSite:       record.InstallSite,
		Files:             make([]validatedFile, 0, len(results)),
		AggregateExpected: integrity.ComputeAggregateSHA256(expected),
		Warnings:          warnings,
		Pass:              true,
		Status:            "PASS",
		AggregateStatus:   string(ValidationSeverityInfo),
	}

	expectedSet := make(map[string]struct{}, len(expected))
	for _, hash := range expected {
		expectedSet[hash.DestPath] = struct{}{}
	}
	actual := make([]integrity.FileHash, 0, len(expected))
	canAggregate := true
	for _, result := range results {
		item := validatedFile{
			Path:     result.Path,
			Status:   result.Status.String(),
			Severity: severityForValidationStatus(result.Status.String()),
		}
		if result.Err != nil {
			item.Error = result.Err.Error()
		}
		summary.Files = append(summary.Files, item)
		summary.AggregateStatus = higherSeverity(summary.AggregateStatus, string(item.Severity))
		if result.Status != integrity.StatusOK {
			summary.Pass = false
			summary.Status = "FAIL"
		}
		if _, tracked := expectedSet[result.Path]; tracked {
			sha, err := integrity.ComputeFileSHA256(filepath.Join(record.InstallRoot, filepath.FromSlash(result.Path)))
			if err != nil {
				canAggregate = false
				continue
			}
			actual = append(actual, integrity.FileHash{DestPath: result.Path, SHA256: sha})
		}
	}

	if canAggregate && len(actual) == len(expected) {
		summary.AggregateActual = integrity.ComputeAggregateSHA256(actual)
		summary.AggregatePass = summary.AggregateActual == summary.AggregateExpected
	} else {
		summary.AggregatePass = false
	}
	if !summary.AggregatePass {
		summary.Pass = false
		summary.Status = "FAIL"
		if summary.AggregateActual != "" {
			item := validatedFile{
				Path:     "(aggregate)",
				Status:   "AGGREGATE_MISMATCH",
				Severity: ValidationSeverityError,
			}
			summary.Files = append(summary.Files, item)
			summary.AggregateStatus = higherSeverity(summary.AggregateStatus, string(item.Severity))
		}
	}

	return summary, nil
}

func severityForValidationStatus(status string) ValidationSeverity {
	switch status {
	case "OK", "":
		return ValidationSeverityInfo
	case "MODIFIED", "HOOK_NOT_REGISTERED", "TEMPLATE_INVALID":
		return ValidationSeverityWarn
	case "EXTRA":
		return ValidationSeverityInfo
	case "MISSING", "UNREADABLE", "SHA_MISMATCH", "DEPENDENCY_VERSION_INCOMPATIBLE", "AGGREGATE_MISMATCH":
		return ValidationSeverityError
	case "DEPENDENCY_MISSING":
		return ValidationSeverityCritical
	default:
		return ValidationSeverityError
	}
}

func higherSeverity(a, b string) string {
	if severityRank(b) > severityRank(a) {
		return b
	}
	if a == "" {
		return b
	}
	return a
}

func severityRank(severity string) int {
	switch ValidationSeverity(severity) {
	case ValidationSeverityCritical:
		return 4
	case ValidationSeverityError:
		return 3
	case ValidationSeverityWarn:
		return 2
	case ValidationSeverityInfo:
		return 1
	default:
		return 0
	}
}

func resolveExpectedHashes(record installer.InstallRecord) ([]integrity.FileHash, []string, error) {
	repoRoot := record.InstallSite
	if repoRoot == "" {
		root, err := currentRepoRoot()
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
	if cat, warnings, ok, err := loadValidationCatalog(localPath); err != nil {
		return nil, nil, err
	} else if ok {
		return expectedFromCatalog(record, cat, warnings)
	}

	machinePath, err := catalog.MachinePath(branch)
	if err != nil {
		return nil, nil, err
	}
	if cat, warnings, ok, err := loadValidationCatalog(machinePath); err != nil {
		return nil, nil, err
	} else if ok {
		warnings = append([]string{"project catalog absent; using machine catalog " + displayCatalogPath(machinePath)}, warnings...)
		return expectedFromCatalog(record, cat, warnings)
	}

	entries, err := validateCatalogFetch(context.Background(), repoRoot, branch)
	if err == nil {
		if _, writeErr := writeCatalogCaches(repoRoot, branch, "both", entries, snapshotNow()); writeErr != nil {
			warnings := []string{"catalog fetched but cache write failed: " + writeErr.Error()}
			return expectedFromCatalog(record, catalog.Catalog{Meta: catalog.CatalogMeta{Branch: branch, FetchedAt: snapshotNow(), SchemaVersion: catalog.SchemaVersion}, Entries: entries}, warnings)
		}
		return expectedFromCatalog(record, catalog.Catalog{Meta: catalog.CatalogMeta{Branch: branch, FetchedAt: snapshotNow(), SchemaVersion: catalog.SchemaVersion}, Entries: entries}, nil)
	}

	warnings := []string{"catalog unavailable and Dolt offline; using lockfile SHAs (may be stale - run sc catalog update when online)"}
	return expectedFromLockfile(record), warnings, nil
}

func loadValidationCatalog(path string) (catalog.Catalog, []string, bool, error) {
	cat, warnings, err := catalog.Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return catalog.Catalog{}, nil, false, nil
	}
	if err != nil {
		return catalog.Catalog{}, nil, false, err
	}
	if !cat.Meta.FetchedAt.IsZero() && snapshotNow().Sub(cat.Meta.FetchedAt) > catalog.StaleAfter {
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
		rel := normalizeRecordPath(record, path)
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
		expected = append(expected, integrity.FileHash{DestPath: normalizeRecordPath(record, path), SHA256: sha})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].DestPath < expected[j].DestPath })
	return expected
}

func expectedFromCurrentFiles(record installer.InstallRecord) []integrity.FileHash {
	expected := make([]integrity.FileHash, 0, len(record.Files))
	for path, fallbackSHA := range record.Files {
		rel := normalizeRecordPath(record, path)
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

func normalizeRecordPath(record installer.InstallRecord, path string) string {
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

func defaultValidateCatalogFetch(ctx context.Context, _ string, branch string) ([]catalog.CatalogEntry, error) {
	cfg := &config.Config{Branch: branch}
	if err := cfg.LoadFileConfig(); err != nil {
		return nil, err
	}
	client, err := readClientOpener(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	return client.GetPackageCatalog(ctx)
}

func scopeDisplay(branch, version string) string {
	if version == "" {
		return ""
	}
	if branch == "" || branch == "main" {
		return version
	}
	return version + " " + branch
}

func sanitizePathComponent(value string) string {
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	cleaned := strings.Trim(b.String(), "-")
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}

func repoKey(path string) string {
	base := sanitizePathComponent(filepath.Base(path))
	sum := sha256.Sum256([]byte(path))
	return base + "-" + hex.EncodeToString(sum[:4])
}

func gitRemoteURL(path string) string {
	if path == "" {
		return ""
	}
	cmd := exec.Command("git", "-C", path, "remote", "get-url", "origin") //nolint:gosec // git command and args are fixed.
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
