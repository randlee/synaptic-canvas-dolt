package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

func TestLoadTrackedInstallsAndFilters(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	local := installer.ManifestLock{Installs: []installer.InstallRecord{
		{InstallID: "pkg_b_project", Package: "b", InstallScope: "project"},
		{InstallID: "pkg_a_project", Package: "a", InstallScope: "project"},
	}}
	global := installer.ManifestLock{Installs: []installer.InstallRecord{
		{InstallID: "pkg_a_global", Package: "a", InstallScope: "global"},
	}}
	if err := installer.SaveManifestLock(root, local); err != nil {
		t.Fatalf("SaveManifestLock(local) error = %v", err)
	}
	if err := installer.SaveManifestLock(home, global); err != nil {
		t.Fatalf("SaveManifestLock(global) error = %v", err)
	}

	installs, err := LoadTrackedInstalls(root)
	if err != nil {
		t.Fatalf("LoadTrackedInstalls() error = %v", err)
	}
	if len(installs) != 3 {
		t.Fatalf("len(installs) = %d, want 3", len(installs))
	}
	gotOrder := []string{
		installs[0].Record.Package + ":" + installs[0].Record.InstallScope,
		installs[1].Record.Package + ":" + installs[1].Record.InstallScope,
		installs[2].Record.Package + ":" + installs[2].Record.InstallScope,
	}
	wantOrder := []string{"a:project", "a:global", "b:project"}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("order = %v, want %v", gotOrder, wantOrder)
	}

	filtered := FilterInstalls(installs, "a")
	if len(filtered) != 2 {
		t.Fatalf("FilterInstalls() len = %d, want 2", len(filtered))
	}
	projectOnly := FilterInstallsByScope(installs, "project")
	if len(projectOnly) != 2 {
		t.Fatalf("FilterInstallsByScope(project) len = %d, want 2", len(projectOnly))
	}
}

func TestValidateTrackedInstallSurfacesRecordedPackageAggregateMismatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeTestFile(t, filepath.Join(installRoot, "SKILL.md"), "current")

	record := installer.InstallRecord{
		Package:                  "team-lead",
		Version:                  "1.0.0",
		Branch:                   "main",
		InstallScope:             "project",
		InstallRoot:              installRoot,
		InstallSite:              root,
		Files:                    map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("current"))},
		PackageAggregateExpected: "package-sha",
		PackageAggregateActual:   "installed-sha",
	}
	expected := []integrity.FileHash{
		{DestPath: "SKILL.md", SHA256: integrity.ComputeContentSHA256([]byte("current"))},
	}

	summary, err := ValidateTrackedInstall(context.Background(), record, expected, nil, func(repoRoot, scope string) (string, error) {
		return repoRoot, nil
	})
	if err != nil {
		t.Fatalf("ValidateTrackedInstall() error = %v", err)
	}
	if !summary.Pass {
		t.Fatalf("summary.Pass = false, want true for advisory package aggregate mismatch")
	}
	if summary.Status != "PASS" {
		t.Fatalf("Status = %q, want PASS", summary.Status)
	}
	if len(summary.Warnings) != 1 || !strings.Contains(summary.Warnings[0], "package aggregate differs") {
		t.Fatalf("Warnings = %+v", summary.Warnings)
	}
	found := false
	for _, item := range summary.Items {
		if item.Code == "package_aggregate_mismatch" {
			found = true
			if item.Severity != api.ValidationSeverityWarn {
				t.Fatalf("item.Severity = %q, want warn", item.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected package_aggregate_mismatch item in %+v", summary.Items)
	}
}

func TestResolveExpectedHashesProjectAndMachineCatalogFallbacks(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeTestFile(t, filepath.Join(installRoot, "SKILL.md"), "local")

	record := installer.InstallRecord{
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallRoot:  installRoot,
		InstallSite:  root,
		InstallScope: "project",
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("lock"))},
	}
	now := time.Date(2026, 5, 16, 5, 0, 0, 0, time.UTC)

	projectPath := catalog.ProjectPath(root, "main")
	if err := catalog.Save(projectPath, catalog.Catalog{
		Meta: catalog.CatalogMeta{
			Branch:        "main",
			FetchedAt:     now.Add(-25 * time.Hour),
			SchemaVersion: catalog.SchemaVersion,
		},
		Entries: []catalog.CatalogEntry{{
			PackageID: "team-lead",
			Version:   "1.0.0",
			DocPath:   "SKILL.md",
			SHA256:    "project-sha",
		}},
	}); err != nil {
		t.Fatalf("catalog.Save(project) error = %v", err)
	}

	expected, warnings, err := ResolveExpectedHashes(context.Background(), record, ExpectedHashOptions{
		Now: nowFunc(now),
		FetchCatalog: func(context.Context, string, string) ([]catalog.CatalogEntry, error) {
			t.Fatal("FetchCatalog should not be called when project catalog exists")
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("ResolveExpectedHashes(project) error = %v", err)
	}
	if len(expected) != 1 || expected[0].SHA256 != "project-sha" {
		t.Fatalf("project expected = %+v", expected)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "older than 24h") {
		t.Fatalf("project warnings = %+v, want stale warning", warnings)
	}

	if err := os.Remove(projectPath); err != nil {
		t.Fatalf("Remove(projectPath) error = %v", err)
	}
	machinePath, err := catalog.MachinePath("main")
	if err != nil {
		t.Fatalf("MachinePath() error = %v", err)
	}
	if err := catalog.Save(machinePath, catalog.Catalog{
		Meta: catalog.CatalogMeta{
			Branch:        "main",
			FetchedAt:     now,
			SchemaVersion: catalog.SchemaVersion,
		},
		Entries: []catalog.CatalogEntry{{
			PackageID: "team-lead",
			Version:   "1.0.0",
			DocPath:   "SKILL.md",
			SHA256:    "machine-sha",
		}},
	}); err != nil {
		t.Fatalf("catalog.Save(machine) error = %v", err)
	}

	expected, warnings, err = ResolveExpectedHashes(context.Background(), record, ExpectedHashOptions{
		Now:                nowFunc(now),
		DisplayCatalogPath: func(string) string { return "~/catalog-main.toml" },
	})
	if err != nil {
		t.Fatalf("ResolveExpectedHashes(machine) error = %v", err)
	}
	if len(expected) != 1 || expected[0].SHA256 != "machine-sha" {
		t.Fatalf("machine expected = %+v", expected)
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "using machine catalog ~/catalog-main.toml") {
		t.Fatalf("machine warnings = %+v", warnings)
	}
}

func TestResolveExpectedHashesFetchFallbacks(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeTestFile(t, filepath.Join(installRoot, "SKILL.md"), "current")

	record := installer.InstallRecord{
		Package:      "team-lead",
		Version:      "1.0.0",
		InstallRoot:  installRoot,
		InstallScope: "project",
		Files:        map[string]string{"SKILL.md": "lock-sha"},
	}
	now := time.Date(2026, 5, 16, 6, 0, 0, 0, time.UTC)

	expected, warnings, err := ResolveExpectedHashes(context.Background(), record, ExpectedHashOptions{
		ResolveRepoRoot: func() (string, error) { return root, nil },
		Now:             nowFunc(now),
		FetchCatalog: func(context.Context, string, string) ([]catalog.CatalogEntry, error) {
			return []catalog.CatalogEntry{{
				PackageID: "team-lead",
				Version:   "1.0.0",
				DocPath:   "SKILL.md",
				SHA256:    "fetched-sha",
			}}, nil
		},
		WriteCatalogCaches: func(string, string, string, []catalog.CatalogEntry, time.Time) ([]string, error) {
			return nil, errors.New("disk full")
		},
	})
	if err != nil {
		t.Fatalf("ResolveExpectedHashes(fetch) error = %v", err)
	}
	if len(expected) != 1 || expected[0].SHA256 != "fetched-sha" {
		t.Fatalf("fetched expected = %+v", expected)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "cache write failed") {
		t.Fatalf("fetched warnings = %+v", warnings)
	}

	expected, warnings, err = ResolveExpectedHashes(context.Background(), record, ExpectedHashOptions{
		ResolveRepoRoot: func() (string, error) { return root, nil },
		FetchCatalog: func(context.Context, string, string) ([]catalog.CatalogEntry, error) {
			return nil, errors.New("offline")
		},
	})
	if err != nil {
		t.Fatalf("ResolveExpectedHashes(offline) error = %v", err)
	}
	if len(expected) != 1 || expected[0].SHA256 != "lock-sha" {
		t.Fatalf("offline expected = %+v", expected)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "catalog unavailable and Dolt offline") {
		t.Fatalf("offline warnings = %+v", warnings)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = ResolveExpectedHashes(ctx, record, ExpectedHashOptions{
		ResolveRepoRoot: func() (string, error) { return root, nil },
		FetchCatalog: func(ctx context.Context, _ string, _ string) ([]catalog.CatalogEntry, error) {
			return nil, ctx.Err()
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveExpectedHashes(canceled) error = %v, want context.Canceled", err)
	}
}

func TestResolveExpectedHashesCatalogShapes(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeTestFile(t, filepath.Join(installRoot, "SKILL.md"), "current")

	record := installer.InstallRecord{
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallRoot:  installRoot,
		InstallSite:  root,
		InstallScope: "project",
		Files:        map[string]string{"SKILL.md": "lock-sha"},
	}
	now := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)
	projectPath := catalog.ProjectPath(root, "main")

	if err := catalog.Save(projectPath, catalog.Catalog{
		Meta: catalog.CatalogMeta{Branch: "main", FetchedAt: now, SchemaVersion: catalog.SchemaVersion},
	}); err != nil {
		t.Fatalf("catalog.Save(empty) error = %v", err)
	}
	expected, warnings, err := ResolveExpectedHashes(context.Background(), record, ExpectedHashOptions{Now: nowFunc(now)})
	if err != nil {
		t.Fatalf("ResolveExpectedHashes(empty catalog) error = %v", err)
	}
	if len(expected) != 1 || expected[0].SHA256 != integrity.ComputeContentSHA256([]byte("current")) {
		t.Fatalf("empty catalog expected = %+v", expected)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "catalog is empty") {
		t.Fatalf("empty catalog warnings = %+v", warnings)
	}

	if err := catalog.Save(projectPath, catalog.Catalog{
		Meta: catalog.CatalogMeta{Branch: "main", FetchedAt: now, SchemaVersion: catalog.SchemaVersion},
		Entries: []catalog.CatalogEntry{{
			PackageID: "other",
			Version:   "1.0.0",
			DocPath:   "SKILL.md",
			SHA256:    "other-sha",
		}},
	}); err != nil {
		t.Fatalf("catalog.Save(mismatch) error = %v", err)
	}
	expected, warnings, err = ResolveExpectedHashes(context.Background(), record, ExpectedHashOptions{Now: nowFunc(now)})
	if err != nil {
		t.Fatalf("ResolveExpectedHashes(mismatch catalog) error = %v", err)
	}
	if len(expected) != 1 || expected[0].SHA256 != "lock-sha" {
		t.Fatalf("mismatch catalog expected = %+v", expected)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "no entries for installed package/version") {
		t.Fatalf("mismatch catalog warnings = %+v", warnings)
	}
}

func TestNormalizeRecordPath(t *testing.T) {
	record := installer.InstallRecord{InstallRoot: filepath.Join(string(filepath.Separator), "repo", ".claude", "skills", "team-lead")}
	abs := filepath.Join(record.InstallRoot, "nested", "SKILL.md")
	if got := NormalizeRecordPath(record, abs); got != "nested/SKILL.md" {
		t.Fatalf("NormalizeRecordPath(abs) = %q", got)
	}
	if got := NormalizeRecordPath(record, filepath.ToSlash(abs)); got != "nested/SKILL.md" {
		t.Fatalf("NormalizeRecordPath(prefixed) = %q", got)
	}
	if got := NormalizeRecordPath(record, "plain/SKILL.md"); got != "plain/SKILL.md" {
		t.Fatalf("NormalizeRecordPath(rel) = %q", got)
	}
}

func TestValidateTrackedInstallAndSubfunctions(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeTestFile(t, filepath.Join(installRoot, "hooks", "pre.sh"), "#!/bin/sh\n")
	writeTestFile(t, filepath.Join(installRoot, "SKILL.md"), "changed")

	record := installer.InstallRecord{
		Package:          "team-lead",
		Version:          "1.0.0",
		Branch:           "main",
		InstallScope:     "project",
		InstallRoot:      installRoot,
		InstallSite:      root,
		TemplateRendered: true,
		Files: map[string]string{
			"hooks/pre.sh": integrity.ComputeContentSHA256([]byte("#!/bin/sh\n")),
			"SKILL.md":     integrity.ComputeContentSHA256([]byte("original")),
		},
		Hooks: []installer.HookEntry{{
			Event:    "PreToolUse",
			Matcher:  "git commit",
			Skill:    "team-lead",
			Scope:    "project",
			Script:   "hooks/pre.sh",
			Priority: 5,
			Blocking: true,
		}},
		Requirements: installer.RequirementSnapshot{
			Tools:        []string{"gh>=2"},
			CLIInstalled: []string{"atm"},
		},
		TemplateValidation: installer.TemplateValidationRecord{
			TemplateFiles: []string{"SKILL.md.j2"},
			Unresolved:    []string{"SKILL.md unresolved"},
		},
	}
	expected := []integrity.FileHash{
		{DestPath: "SKILL.md", SHA256: integrity.ComputeContentSHA256([]byte("original"))},
		{DestPath: "hooks/pre.sh", SHA256: integrity.ComputeContentSHA256([]byte("#!/bin/sh\n"))},
	}

	summary, err := ValidateTrackedInstall(context.Background(), record, expected, []string{"warning"}, func(repoRoot, scope string) (string, error) {
		return repoRoot, nil
	})
	if err != nil {
		t.Fatalf("ValidateTrackedInstall() error = %v", err)
	}
	if summary.Pass {
		t.Fatalf("summary.Pass = true, want false due to aggregate + dependency/template issues")
	}
	if summary.AggregateStatus != "critical" {
		t.Fatalf("AggregateStatus = %q, want critical", summary.AggregateStatus)
	}
	if len(summary.Warnings) != 1 || summary.Warnings[0] != "warning" {
		t.Fatalf("Warnings = %+v", summary.Warnings)
	}
	codes := map[string]bool{}
	for _, item := range summary.Items {
		codes[item.Code] = true
	}
	for _, code := range []string{"aggregate_mismatch", "dependency_verification_missing", "dependency_provenance_missing", "hook_not_registered", "template_invalid"} {
		if !codes[code] {
			t.Fatalf("missing validation item %q in %+v", code, summary.Items)
		}
	}
	if summary.HookSummary.Tracked != 1 || summary.HookSummary.Missing != 1 {
		t.Fatalf("HookSummary = %+v", summary.HookSummary)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled := api.ValidatedInstall{Pass: true, Status: "PASS", AggregateStatus: string(api.ValidationSeverityInfo)}
	appendStateValidationItems(ctx, record, &canceled, func(string, string) (string, error) { return root, nil })
	if len(canceled.Items) != 1 || canceled.Items[0].Code != "context_unreadable" {
		t.Fatalf("context item = %+v", canceled.Items)
	}

	ok := api.ValidatedInstall{Pass: true, Status: "PASS", AggregateStatus: string(api.ValidationSeverityInfo)}
	registry := installer.HookRegistry{Hooks: []installer.HookEntry{{
		Event:    "PreToolUse",
		Matcher:  "git commit",
		Skill:    "team-lead",
		Scope:    "project",
		Script:   filepath.Join(root, ".claude", "skills", "team-lead", "hooks", "pre.sh"),
		Priority: 5,
		Blocking: true,
	}}}
	if err := installer.SaveHookRegistry(root, registry); err != nil {
		t.Fatalf("SaveHookRegistry() error = %v", err)
	}
	appendHookValidationItems(record, &ok, func(string, string) (string, error) { return root, nil })
	if ok.HookSummary.Registered != 1 || ok.HookSummary.Missing != 0 {
		t.Fatalf("registered HookSummary = %+v", ok.HookSummary)
	}

	deps := api.ValidatedInstall{Pass: true, Status: "PASS", AggregateStatus: string(api.ValidationSeverityInfo)}
	appendDependencyValidationItems(record, &deps)
	if len(deps.Items) != 2 {
		t.Fatalf("dependency items = %+v", deps.Items)
	}

	templates := api.ValidatedInstall{Pass: true, Status: "PASS", AggregateStatus: string(api.ValidationSeverityInfo)}
	appendTemplateValidationItems(record, &templates)
	if len(templates.Items) != 1 || templates.Items[0].Code != "template_invalid" {
		t.Fatalf("template items = %+v", templates.Items)
	}
}

func nowFunc(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
