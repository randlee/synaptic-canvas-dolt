package operations

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

// ValidateTrackedInstall evaluates one tracked install against expected file
// hashes and tracked metadata, returning the shared readback DTO.
func ValidateTrackedInstall(
	ctx context.Context,
	record installer.InstallRecord,
	expected []integrity.FileHash,
	warnings []string,
	stateRootForScope func(repoRoot, scope string) (string, error),
) (api.ValidatedInstall, error) {
	results, err := integrity.VerifyPackage(expected, record.InstallRoot)
	if err != nil {
		return api.ValidatedInstall{}, err
	}

	summary := api.ValidatedInstall{
		Package:           record.Package,
		Version:           record.Version,
		Branch:            record.Branch,
		Scope:             record.InstallScope,
		InstallRoot:       record.InstallRoot,
		InstallSite:       record.InstallSite,
		TrackingOrigin:    record.TrackingOrigin,
		Items:             make([]api.ValidationItem, 0, len(results)+8),
		AggregateExpected: integrity.ComputeAggregateSHA256(expected),
		Warnings:          warnings,
		Pass:              true,
		Status:            "PASS",
		AggregateStatus:   string(api.ValidationSeverityInfo),
	}
	summary.DependencySummary = dependencySummary(record)

	expectedSet := make(map[string]struct{}, len(expected))
	for _, hash := range expected {
		expectedSet[hash.DestPath] = struct{}{}
	}
	actual := make([]integrity.FileHash, 0, len(expected))
	canAggregate := true
	for _, result := range results {
		item := api.ValidationItem{
			Kind:     api.ValidationKindFile,
			Path:     result.Path,
			State:    ValidationStateForIntegrityStatus(result.Status),
			Severity: SeverityForValidationState(ValidationStateForIntegrityStatus(result.Status)),
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
			appendValidationItem(&summary, api.ValidationItem{
				Kind:     api.ValidationKindAggregate,
				State:    api.ValidationStateModified,
				Severity: api.ValidationSeverityError,
				Code:     "aggregate_mismatch",
				Expected: summary.AggregateExpected,
				Actual:   summary.AggregateActual,
				Message:  "aggregate SHA256 does not match tracked package state",
			})
		}
	}

	appendStateValidationItems(ctx, record, &summary, stateRootForScope)
	return summary, nil
}

// ValidationStateForIntegrityStatus maps integrity results into the fixed CLI
// validation-state vocabulary.
func ValidationStateForIntegrityStatus(status integrity.VerifyStatus) api.ValidationState {
	switch status {
	case integrity.StatusOK:
		return api.ValidationStateOK
	case integrity.StatusModified:
		return api.ValidationStateModified
	case integrity.StatusMissing:
		return api.ValidationStateMissing
	case integrity.StatusUnreadable:
		return api.ValidationStateUnreadable
	case integrity.StatusExtra:
		return api.ValidationStateExtra
	default:
		return api.ValidationStateUnreadable
	}
}

// SeverityForValidationState maps validation-state values into the shared
// severity vocabulary.
func SeverityForValidationState(state api.ValidationState) api.ValidationSeverity {
	switch state {
	case api.ValidationStateOK, "":
		return api.ValidationSeverityInfo
	case api.ValidationStateModified:
		return api.ValidationSeverityWarn
	case api.ValidationStateExtra:
		return api.ValidationSeverityInfo
	case api.ValidationStateMissing, api.ValidationStateUnreadable:
		return api.ValidationSeverityError
	default:
		return api.ValidationSeverityError
	}
}

// HigherSeverity returns whichever severity string represents the higher
// machine-priority level.
func HigherSeverity(a, b string) string {
	if severityRank(b) > severityRank(a) {
		return b
	}
	if a == "" {
		return b
	}
	return a
}

func severityRank(severity string) int {
	switch api.ValidationSeverity(severity) {
	case api.ValidationSeverityCritical:
		return 4
	case api.ValidationSeverityError:
		return 3
	case api.ValidationSeverityWarn:
		return 2
	case api.ValidationSeverityInfo:
		return 1
	default:
		return 0
	}
}

func incrementModificationSummary(summary *api.ModificationSummary, state api.ValidationState) {
	switch state {
	case api.ValidationStateOK:
		summary.OK++
	case api.ValidationStateModified:
		summary.Modified++
	case api.ValidationStateMissing:
		summary.Missing++
	case api.ValidationStateUnreadable:
		summary.Unreadable++
	case api.ValidationStateExtra:
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

func appendStateValidationItems(
	ctx context.Context,
	record installer.InstallRecord,
	summary *api.ValidatedInstall,
	stateRootForScope func(repoRoot, scope string) (string, error),
) {
	if err := ctx.Err(); err != nil {
		appendValidationItem(summary, api.ValidationItem{
			Kind:     api.ValidationKindContext,
			State:    api.ValidationStateUnreadable,
			Severity: api.ValidationSeverityError,
			Code:     "context_unreadable",
			Message:  err.Error(),
		})
		return
	}
	appendDependencyValidationItems(record, summary)
	appendHookValidationItems(record, summary, stateRootForScope)
	appendTemplateValidationItems(record, summary)
}

func appendDependencyValidationItems(record installer.InstallRecord, summary *api.ValidatedInstall) {
	verified := record.Requirements.ToolsVerified
	for _, tool := range record.Requirements.Tools {
		if tool == "" {
			continue
		}
		if verified != nil && strings.TrimSpace(verified[tool]) != "" {
			continue
		}
		appendValidationItem(summary, api.ValidationItem{
			Kind:           api.ValidationKindDependency,
			State:          api.ValidationStateMissing,
			Severity:       api.ValidationSeverityCritical,
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
		appendValidationItem(summary, api.ValidationItem{
			Kind:           api.ValidationKindDependency,
			State:          api.ValidationStateMissing,
			Severity:       api.ValidationSeverityCritical,
			Code:           "dependency_provenance_missing",
			Message:        "installed dependency provenance is missing",
			Dependency:     dep,
			DependencyType: "cli",
		})
	}
}

func appendHookValidationItems(
	record installer.InstallRecord,
	summary *api.ValidatedInstall,
	stateRootForScope func(repoRoot, scope string) (string, error),
) {
	expectedHooks := expectedHooks(record)
	if len(expectedHooks) == 0 {
		return
	}
	summary.HookSummary.Tracked = len(expectedHooks)
	stateRoot, err := stateRootForScope(record.InstallSite, record.InstallScope)
	if err != nil {
		appendValidationItem(summary, api.ValidationItem{
			Kind:     api.ValidationKindHook,
			State:    api.ValidationStateUnreadable,
			Severity: api.ValidationSeverityWarn,
			Code:     "hook_registry_unreadable",
			Message:  err.Error(),
			Target:   "registry",
		})
		return
	}
	registry, err := installer.LoadHookRegistry(stateRoot)
	if err != nil {
		appendValidationItem(summary, api.ValidationItem{
			Kind:     api.ValidationKindHook,
			State:    api.ValidationStateUnreadable,
			Severity: api.ValidationSeverityWarn,
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
		appendValidationItem(summary, api.ValidationItem{
			Kind:        api.ValidationKindHook,
			State:       api.ValidationStateMissing,
			Severity:    api.ValidationSeverityWarn,
			Code:        "hook_not_registered",
			Message:     "tracked hook script is not registered",
			HookEvent:   expected.Event,
			HookMatcher: expected.Matcher,
			HookScript:  script,
			Scope:       expectedHookScope(record, expected),
		})
	}
}

func appendTemplateValidationItems(record installer.InstallRecord, summary *api.ValidatedInstall) {
	if len(record.TemplateValidation.Unresolved) == 0 {
		if record.TemplateRendered && len(record.TemplateValidation.TemplateFiles) == 0 {
			appendValidationItem(summary, api.ValidationItem{
				Kind:     api.ValidationKindTemplate,
				State:    api.ValidationStateModified,
				Severity: api.ValidationSeverityWarn,
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
		appendValidationItem(summary, api.ValidationItem{
			Kind:     api.ValidationKindTemplate,
			State:    api.ValidationStateModified,
			Severity: api.ValidationSeverityWarn,
			Code:     "template_invalid",
			Message:  unresolved,
			Path:     path,
		})
	}
}

func appendValidationItem(summary *api.ValidatedInstall, item api.ValidationItem) {
	if item.Severity == "" {
		item.Severity = SeverityForValidationState(item.State)
	}
	summary.Items = append(summary.Items, item)
	if item.Kind == api.ValidationKindFile {
		incrementModificationSummary(&summary.ModificationSummary, item.State)
	}
	summary.AggregateStatus = HigherSeverity(summary.AggregateStatus, string(item.Severity))
	if item.Severity == api.ValidationSeverityError || item.Severity == api.ValidationSeverityCritical {
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
		rel := NormalizeRecordPath(record, path)
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
