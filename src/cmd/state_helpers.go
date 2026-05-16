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
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

type trackedInstall struct {
	Record installer.InstallRecord
	Source string
}

type validatedInstall = api.ValidatedInstall
type ValidationSeverity = api.ValidationSeverity
type ValidationKind = api.ValidationKind
type ValidationState = api.ValidationState
type validatedItem = api.ValidationItem

const (
	ValidationSeverityInfo     ValidationSeverity = api.ValidationSeverityInfo
	ValidationSeverityWarn     ValidationSeverity = api.ValidationSeverityWarn
	ValidationSeverityError    ValidationSeverity = api.ValidationSeverityError
	ValidationSeverityCritical ValidationSeverity = api.ValidationSeverityCritical

	ValidationKindFile       ValidationKind = api.ValidationKindFile
	ValidationKindDependency ValidationKind = api.ValidationKindDependency
	ValidationKindHook       ValidationKind = api.ValidationKindHook
	ValidationKindTemplate   ValidationKind = api.ValidationKindTemplate
	ValidationKindAggregate  ValidationKind = api.ValidationKindAggregate
	ValidationKindContext    ValidationKind = api.ValidationKindContext

	ValidationStateOK         ValidationState = api.ValidationStateOK
	ValidationStateModified   ValidationState = api.ValidationStateModified
	ValidationStateMissing    ValidationState = api.ValidationStateMissing
	ValidationStateUnreadable ValidationState = api.ValidationStateUnreadable
	ValidationStateExtra      ValidationState = api.ValidationStateExtra
)

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

func validateTrackedInstall(ctx context.Context, record installer.InstallRecord) (validatedInstall, error) {
	expected, warnings, err := resolveExpectedHashes(ctx, record)
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
		TrackingOrigin:    record.TrackingOrigin,
		Items:             make([]validatedItem, 0, len(results)+8),
		AggregateExpected: integrity.ComputeAggregateSHA256(expected),
		Warnings:          warnings,
		Pass:              true,
		Status:            "PASS",
		AggregateStatus:   string(ValidationSeverityInfo),
	}
	summary.DependencySummary = dependencySummary(record)

	expectedSet := make(map[string]struct{}, len(expected))
	for _, hash := range expected {
		expectedSet[hash.DestPath] = struct{}{}
	}
	actual := make([]integrity.FileHash, 0, len(expected))
	canAggregate := true
	for _, result := range results {
		item := validatedItem{
			Kind:     ValidationKindFile,
			Path:     result.Path,
			State:    validationStateForIntegrityStatus(result.Status),
			Severity: severityForValidationState(validationStateForIntegrityStatus(result.Status)),
		}
		if result.Err != nil {
			item.Message = result.Err.Error()
			item.Code = "file_unreadable"
		}
		appendValidationItem(&summary, item)
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
			item := validatedItem{
				Kind:     ValidationKindAggregate,
				State:    ValidationStateModified,
				Severity: ValidationSeverityError,
				Code:     "aggregate_mismatch",
				Expected: summary.AggregateExpected,
				Actual:   summary.AggregateActual,
				Message:  "aggregate SHA256 does not match tracked package state",
			}
			appendValidationItem(&summary, item)
		}
	}

	appendStateValidationItems(ctx, record, &summary)
	return summary, nil
}

func validationStateForIntegrityStatus(status integrity.VerifyStatus) ValidationState {
	switch status {
	case integrity.StatusOK:
		return ValidationStateOK
	case integrity.StatusModified:
		return ValidationStateModified
	case integrity.StatusMissing:
		return ValidationStateMissing
	case integrity.StatusUnreadable:
		return ValidationStateUnreadable
	case integrity.StatusExtra:
		return ValidationStateExtra
	default:
		return ValidationStateUnreadable
	}
}

func severityForValidationState(state ValidationState) ValidationSeverity {
	switch state {
	case ValidationStateOK, "":
		return ValidationSeverityInfo
	case ValidationStateModified:
		return ValidationSeverityWarn
	case ValidationStateExtra:
		return ValidationSeverityInfo
	case ValidationStateMissing, ValidationStateUnreadable:
		return ValidationSeverityError
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

func incrementModificationSummary(summary *api.ModificationSummary, state ValidationState) {
	switch state {
	case ValidationStateOK:
		summary.OK++
	case ValidationStateModified:
		summary.Modified++
	case ValidationStateMissing:
		summary.Missing++
	case ValidationStateUnreadable:
		summary.Unreadable++
	case ValidationStateExtra:
		summary.Extra++
	}
}

func dependencySummary(record installer.InstallRecord) api.DependencySummary {
	items := make([]api.DependencyReadback, 0, len(record.Requirements.Tools)+len(record.Requirements.CLIInstalled))
	verifiedCount := 0
	missingCount := 0
	for _, tool := range record.Requirements.Tools {
		if tool == "" {
			continue
		}
		provenance := strings.TrimSpace(record.Requirements.ToolsVerified[tool])
		verified := provenance != ""
		if verified {
			verifiedCount++
		} else {
			missingCount++
		}
		items = append(items, api.DependencyReadback{
			Name:           tool,
			DependencyType: "tool",
			Verified:       verified,
			Provenance:     provenance,
		})
	}
	for _, dep := range record.Requirements.CLIInstalled {
		if dep == "" {
			continue
		}
		provenance := strings.TrimSpace(record.Requirements.CLIProvenance[dep])
		verified := provenance != ""
		if verified {
			verifiedCount++
		} else {
			missingCount++
		}
		items = append(items, api.DependencyReadback{
			Name:           dep,
			DependencyType: "cli",
			Verified:       verified,
			Provenance:     provenance,
			InstalledBySC:  record.Requirements.IsInstalledBySC(dep),
		})
	}
	return api.DependencySummary{
		Tracked:  len(items),
		Verified: verifiedCount,
		Missing:  missingCount,
		Items:    items,
	}
}

func appendStateValidationItems(ctx context.Context, record installer.InstallRecord, summary *validatedInstall) {
	if err := ctx.Err(); err != nil {
		appendValidationItem(summary, validatedItem{
			Kind:     ValidationKindContext,
			State:    ValidationStateUnreadable,
			Severity: ValidationSeverityError,
			Code:     "context_unreadable",
			Message:  err.Error(),
		})
		return
	}
	appendDependencyValidationItems(record, summary)
	appendHookValidationItems(record, summary)
	appendTemplateValidationItems(record, summary)
}

func appendDependencyValidationItems(record installer.InstallRecord, summary *validatedInstall) {
	verified := record.Requirements.ToolsVerified
	for _, tool := range record.Requirements.Tools {
		if tool == "" {
			continue
		}
		if verified != nil && strings.TrimSpace(verified[tool]) != "" {
			continue
		}
		appendValidationItem(summary, validatedItem{
			Kind:           ValidationKindDependency,
			State:          ValidationStateMissing,
			Severity:       ValidationSeverityCritical,
			Code:           "dependency_verification_missing",
			Message:        "dependency is not verified in install record",
			Dependency:     tool,
			DependencyType: "tool",
		})
	}
	provenance := record.Requirements.CLIProvenance
	for _, dep := range record.Requirements.CLIInstalled {
		if dep == "" {
			continue
		}
		if provenance != nil && strings.TrimSpace(provenance[dep]) != "" {
			continue
		}
		appendValidationItem(summary, validatedItem{
			Kind:           ValidationKindDependency,
			State:          ValidationStateMissing,
			Severity:       ValidationSeverityCritical,
			Code:           "dependency_provenance_missing",
			Message:        "installed dependency provenance is missing",
			Dependency:     dep,
			DependencyType: "cli",
		})
	}
}

func appendHookValidationItems(record installer.InstallRecord, summary *validatedInstall) {
	expectedHooks := expectedHooks(record)
	if len(expectedHooks) == 0 {
		return
	}
	summary.HookSummary.Tracked = len(expectedHooks)
	stateRoot, err := stateRootForScope(record.InstallSite, record.InstallScope)
	if err != nil {
		appendValidationItem(summary, validatedItem{
			Kind:     ValidationKindHook,
			State:    ValidationStateUnreadable,
			Severity: ValidationSeverityWarn,
			Code:     "hook_registry_unreadable",
			Message:  err.Error(),
			Target:   "registry",
		})
		return
	}
	registry, err := installer.LoadHookRegistry(stateRoot)
	if err != nil {
		appendValidationItem(summary, validatedItem{
			Kind:     ValidationKindHook,
			State:    ValidationStateUnreadable,
			Severity: ValidationSeverityWarn,
			Code:     "hook_registry_unreadable",
			Message:  err.Error(),
			Target:   "registry",
		})
		return
	}
	for _, expected := range expectedHooks {
		script := filepath.ToSlash(expected.Script)
		if hook, ok := registeredHook(registry, record, script); ok {
			summary.HookSummary.Registered++
			summary.HookSummary.Hooks = append(summary.HookSummary.Hooks, api.HookValidationState{
				Event:      hook.Event,
				Matcher:    hook.Matcher,
				Script:     script,
				Scope:      hook.Scope,
				Priority:   hook.Priority,
				Blocking:   hook.Blocking,
				Registered: true,
			})
			continue
		}
		summary.HookSummary.Missing++
		summary.HookSummary.Hooks = append(summary.HookSummary.Hooks, api.HookValidationState{
			Event:      expected.Event,
			Matcher:    expected.Matcher,
			Script:     script,
			Scope:      expectedHookScope(record, expected),
			Priority:   expected.Priority,
			Blocking:   expected.Blocking,
			Registered: false,
		})
		appendValidationItem(summary, validatedItem{
			Kind:        ValidationKindHook,
			State:       ValidationStateMissing,
			Severity:    ValidationSeverityWarn,
			Code:        "hook_not_registered",
			Message:     "tracked hook script is not registered",
			HookEvent:   expected.Event,
			HookMatcher: expected.Matcher,
			HookScript:  script,
			Scope:       expectedHookScope(record, expected),
		})
	}
}

func appendTemplateValidationItems(record installer.InstallRecord, summary *validatedInstall) {
	if len(record.TemplateValidation.Unresolved) == 0 {
		if record.TemplateRendered && len(record.TemplateValidation.TemplateFiles) == 0 {
			appendValidationItem(summary, validatedItem{
				Kind:     ValidationKindTemplate,
				State:    ValidationStateModified,
				Severity: ValidationSeverityWarn,
				Code:     "template_metadata_missing",
				Message:  "template render was tracked without template file metadata",
				Target:   record.Package,
			})
		}
		return
	}
	path := record.Package
	if len(record.TemplateValidation.TemplateFiles) > 0 {
		path = filepath.ToSlash(record.TemplateValidation.TemplateFiles[0])
	}
	for _, unresolved := range record.TemplateValidation.Unresolved {
		appendValidationItem(summary, validatedItem{
			Kind:     ValidationKindTemplate,
			State:    ValidationStateModified,
			Severity: ValidationSeverityWarn,
			Code:     "template_invalid",
			Message:  unresolved,
			Path:     path,
		})
	}
}

func appendValidationItem(summary *validatedInstall, item validatedItem) {
	if item.Severity == "" {
		item.Severity = severityForValidationState(item.State)
	}
	summary.Items = append(summary.Items, item)
	if item.Kind == ValidationKindFile {
		incrementModificationSummary(&summary.ModificationSummary, item.State)
	}
	summary.AggregateStatus = higherSeverity(summary.AggregateStatus, string(item.Severity))
	if item.Severity == ValidationSeverityError || item.Severity == ValidationSeverityCritical {
		summary.Pass = false
		summary.Status = "FAIL"
	}
}

func expectedHooks(record installer.InstallRecord) []installer.HookEntry {
	if len(record.Hooks) > 0 {
		hooks := make([]installer.HookEntry, len(record.Hooks))
		copy(hooks, record.Hooks)
		sort.Slice(hooks, func(i, j int) bool {
			left := filepath.ToSlash(hooks[i].Script) + "\x00" + hooks[i].Event + "\x00" + hooks[i].Matcher
			right := filepath.ToSlash(hooks[j].Script) + "\x00" + hooks[j].Event + "\x00" + hooks[j].Matcher
			return left < right
		})
		return hooks
	}
	hooks := make([]installer.HookEntry, 0, len(record.Files))
	for path := range record.Files {
		rel := normalizeRecordPath(record, path)
		if isHookScriptPath(rel) {
			hooks = append(hooks, installer.HookEntry{
				Skill:  record.Package,
				Scope:  record.InstallScope,
				Script: filepath.ToSlash(rel),
			})
		}
	}
	sort.Slice(hooks, func(i, j int) bool {
		return filepath.ToSlash(hooks[i].Script) < filepath.ToSlash(hooks[j].Script)
	})
	return hooks
}

func expectedHookScope(record installer.InstallRecord, hook installer.HookEntry) string {
	if strings.TrimSpace(hook.Scope) != "" {
		return hook.Scope
	}
	return record.InstallScope
}

func isHookScriptPath(path string) bool {
	slashPath := filepath.ToSlash(path)
	parts := strings.Split(slashPath, "/")
	for _, part := range parts {
		if part == "hooks" {
			return true
		}
	}
	return strings.HasPrefix(filepath.Base(slashPath), "hook-") || strings.Contains(filepath.Base(slashPath), ".hook.")
}

func registeredHook(registry installer.HookRegistry, record installer.InstallRecord, script string) (installer.HookEntry, bool) {
	absScript := filepath.ToSlash(filepath.Join(record.InstallRoot, filepath.FromSlash(script)))
	for _, hook := range registry.Hooks {
		if hook.Skill != record.Package {
			continue
		}
		if hook.Scope != "" && hook.Scope != record.InstallScope {
			continue
		}
		hookScript := filepath.ToSlash(hook.Script)
		if hookScript == absScript || hookScript == script || strings.HasSuffix(hookScript, "/"+script) {
			return hook, true
		}
	}
	return installer.HookEntry{}, false
}

func resolveExpectedHashes(ctx context.Context, record installer.InstallRecord) ([]integrity.FileHash, []string, error) {
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

	entries, err := validateCatalogFetch(ctx, repoRoot, branch)
	if err == nil {
		if _, writeErr := writeCatalogCaches(repoRoot, branch, "both", entries, snapshotNow()); writeErr != nil {
			warnings := []string{"catalog fetched but cache write failed: " + writeErr.Error()}
			return expectedFromCatalog(record, catalog.Catalog{Meta: catalog.CatalogMeta{Branch: branch, FetchedAt: snapshotNow(), SchemaVersion: catalog.SchemaVersion}, Entries: entries}, warnings)
		}
		return expectedFromCatalog(record, catalog.Catalog{Meta: catalog.CatalogMeta{Branch: branch, FetchedAt: snapshotNow(), SchemaVersion: catalog.SchemaVersion}, Entries: entries}, nil)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, nil, err
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
