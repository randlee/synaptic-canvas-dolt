package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

func TestJSONValidateAllFileStates(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setTestHome(t, home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "good.txt"), "good")
	writeCmdFile(t, filepath.Join(installRoot, "modified.txt"), "changed")
	writeCmdFile(t, filepath.Join(installRoot, "extra.txt"), "extra")
	if err := os.MkdirAll(filepath.Join(installRoot, "unreadable.txt"), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	lock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.2.0",
			Branch:       "main",
			InstallScope: "project",
			InstallRoot:  installRoot,
			InstallSite:  root,
			Files: map[string]string{
				"good.txt":       integrity.ComputeContentSHA256([]byte("good")),
				"modified.txt":   integrity.ComputeContentSHA256([]byte("original")),
				"missing.txt":    integrity.ComputeContentSHA256([]byte("missing")),
				"unreadable.txt": integrity.ComputeContentSHA256([]byte("anything")),
			},
		}},
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "team-lead", "--json"})

	if err := cmd.Execute(); err == nil || err.Error() != "validation failed" {
		t.Fatalf("Execute() error = %v, want validation failed", err)
	}

	var resp validateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.Pass {
		t.Fatalf("expected validate to fail, got %+v", resp)
	}
	if len(resp.Packages) != 1 {
		t.Fatalf("expected 1 package, got %+v", resp)
	}
	if resp.Packages[0].AggregateStatus != "error" {
		t.Fatalf("aggregate_status = %q, want error", resp.Packages[0].AggregateStatus)
	}
	got := map[string]ValidationState{}
	severity := map[string]ValidationSeverity{}
	for _, file := range resp.Packages[0].Items {
		got[file.Path] = file.State
		severity[file.Path] = file.Severity
	}
	for path, want := range map[string]ValidationState{
		"good.txt":       ValidationStateOK,
		"modified.txt":   ValidationStateModified,
		"missing.txt":    ValidationStateMissing,
		"unreadable.txt": ValidationStateUnreadable,
		"extra.txt":      ValidationStateExtra,
	} {
		if got[path] != want {
			t.Fatalf("status[%s] = %q, want %q (all=%+v)", path, got[path], want, got)
		}
	}
	if severity["good.txt"] != ValidationSeverityInfo || severity["modified.txt"] != ValidationSeverityWarn || severity["missing.txt"] != ValidationSeverityError {
		t.Fatalf("unexpected severities: %+v", severity)
	}
}

func TestValidationSeverityMappings(t *testing.T) {
	for state, want := range map[ValidationState]ValidationSeverity{
		ValidationStateOK:         ValidationSeverityInfo,
		ValidationStateModified:   ValidationSeverityWarn,
		ValidationStateExtra:      ValidationSeverityInfo,
		ValidationStateMissing:    ValidationSeverityError,
		ValidationStateUnreadable: ValidationSeverityError,
	} {
		if got := severityForValidationState(state); got != want {
			t.Fatalf("severityForValidationState(%q) = %q, want %q", state, got, want)
		}
	}
}

func TestValidateEmitsDependencyHookAndTemplateItems(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	hookContent := "#!/bin/sh\n"
	writeCmdFile(t, filepath.Join(installRoot, "hooks", "pre.sh"), hookContent)
	lock := installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:        "pkg_team-lead_project",
		Package:          "team-lead",
		Version:          "1.0.0",
		Branch:           "main",
		InstallScope:     "project",
		InstallRoot:      installRoot,
		InstallSite:      root,
		TemplateRendered: true,
		Files:            map[string]string{"hooks/pre.sh": integrity.ComputeContentSHA256([]byte(hookContent))},
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
			Unresolved:    []string{"SKILL.md: contains unresolved template markers"},
		},
	}}}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "team-lead", "--json"})
	if err := cmd.Execute(); err == nil || err.Error() != "validation failed" {
		t.Fatalf("Execute() error = %v, want validation failed", err)
	}

	var resp validateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	statuses := map[string]int{}
	for _, item := range resp.Packages[0].Items {
		statuses[item.Code]++
	}
	for _, want := range []string{"dependency_verification_missing", "dependency_provenance_missing", "hook_not_registered", "template_invalid"} {
		if statuses[want] == 0 {
			t.Fatalf("expected validation status %s, got %+v", want, resp.Packages[0].Items)
		}
	}
	for _, item := range resp.Packages[0].Items {
		if item.Code != "hook_not_registered" {
			continue
		}
		if item.HookEvent != "PreToolUse" || item.HookMatcher != "git commit" || item.HookScript != "hooks/pre.sh" {
			t.Fatalf("unexpected hook validation item: %+v", item)
		}
	}
	if len(resp.Packages[0].HookSummary.Hooks) != 1 {
		t.Fatalf("expected one hook summary entry, got %+v", resp.Packages[0].HookSummary)
	}
	if hook := resp.Packages[0].HookSummary.Hooks[0]; hook.Event != "PreToolUse" || hook.Matcher != "git commit" || hook.Script != "hooks/pre.sh" || hook.Registered {
		t.Fatalf("unexpected hook summary entry: %+v", hook)
	}
}

func TestValidateScopeBothMixedPassFailReturnsExitOneWithResults(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	localRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	globalRoot := filepath.Join(home, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(localRoot, "SKILL.md"), "local")
	writeCmdFile(t, filepath.Join(globalRoot, "SKILL.md"), "changed")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  localRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("local"))},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock(local) error = %v", err)
	}
	if err := installer.SaveManifestLock(home, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_global",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "global",
		InstallRoot:  globalRoot,
		InstallSite:  home,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("global"))},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock(global) error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "--all", "--scope", "both", "--json"})
	if err := cmd.Execute(); err == nil || err.Error() != "validation failed" {
		t.Fatalf("Execute() error = %v, want validation failed", err)
	}

	var resp validateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.Pass || len(resp.Packages) != 2 {
		t.Fatalf("expected mixed validation failure with 2 packages, got %+v", resp)
	}
	passByScope := map[string]bool{}
	for _, pkg := range resp.Packages {
		passByScope[pkg.Scope] = pkg.Pass
	}
	if !passByScope["project"] || passByScope["global"] {
		t.Fatalf("expected project pass and global fail, got %+v", passByScope)
	}
}

func TestJSONStatusMergedScopes(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setTestHome(t, home)
	resolvedHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	localRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	globalRoot := filepath.Join(resolvedHome, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(localRoot, "SKILL.md"), "local")
	writeCmdFile(t, filepath.Join(globalRoot, "SKILL.md"), "global")

	localLock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.0.0",
			Branch:       "main",
			InstallScope: "project",
			InstallRoot:  localRoot,
			InstallSite:  root,
			Files: map[string]string{
				"SKILL.md": integrity.ComputeContentSHA256([]byte("local")),
			},
		}},
	}
	globalLock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_global",
			Package:      "team-lead",
			Version:      "1.1.0",
			Branch:       "beta",
			InstallScope: "global",
			InstallRoot:  globalRoot,
			InstallSite:  resolvedHome,
			Files: map[string]string{
				"SKILL.md": integrity.ComputeContentSHA256([]byte("global")),
			},
		}},
	}
	if err := installer.SaveManifestLock(root, localLock); err != nil {
		t.Fatalf("SaveManifestLock(local) error = %v", err)
	}
	if err := installer.SaveManifestLock(resolvedHome, globalLock); err != nil {
		t.Fatalf("SaveManifestLock(global) error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"status", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp statusResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if len(resp.Packages) != 1 {
		t.Fatalf("expected 1 package, got %+v", resp)
	}
	row := resp.Packages[0]
	if row.Global == nil || row.Local == nil {
		t.Fatalf("expected both scopes, got %+v", row)
	}
	if row.Global.Branch != "beta" || row.Global.Version != "1.1.0" || row.Global.Validation != "PASS" {
		t.Fatalf("unexpected global row: %+v", row.Global)
	}
	if row.Local.Branch != "main" || row.Local.Version != "1.0.0" || row.Local.Validation != "PASS" {
		t.Fatalf("unexpected local row: %+v", row.Local)
	}
	if row.Global.Scope != "global" || row.Local.Scope != "project" {
		t.Fatalf("unexpected scope readback: global=%+v local=%+v", row.Global, row.Local)
	}
	if row.Global.InstallSite != resolvedHome || row.Local.InstallSite != root {
		t.Fatalf("unexpected install sites: global=%q local=%q", row.Global.InstallSite, row.Local.InstallSite)
	}
	if row.Global.ModificationSummary.OK != 1 || row.Local.ModificationSummary.OK != 1 {
		t.Fatalf("unexpected modification summary: global=%+v local=%+v", row.Global.ModificationSummary, row.Local.ModificationSummary)
	}
}

func TestJSONSnapshotModifiedOnly(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "good.txt"), "good")
	writeCmdFile(t, filepath.Join(installRoot, "modified.txt"), "changed")
	writeCmdFile(t, filepath.Join(installRoot, "extra.txt"), "extra")

	lock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.2.0",
			Branch:       "advanced",
			InstallScope: "project",
			InstallRoot:  installRoot,
			InstallSite:  root,
			Files: map[string]string{
				"good.txt":     integrity.ComputeContentSHA256([]byte("good")),
				"modified.txt": integrity.ComputeContentSHA256([]byte("original")),
			},
		}},
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	snapshotNow = func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { snapshotNow = func() time.Time { return time.Now().UTC() } })
	snapshotGitRemoteURL = func(string) string { return "https://example.com/repo.git" }
	t.Cleanup(func() { snapshotGitRemoteURL = gitRemoteURL })

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "team-lead", "--scope", "project", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp snapshotResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.Version != "1.2.0" || resp.Branch != "advanced" || resp.InstallRoot != installRoot || resp.InstallSite != root {
		t.Fatalf("unexpected snapshot metadata: %+v", resp)
	}
	if len(resp.Files) != 2 {
		t.Fatalf("expected 2 copied files, got %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(resp.OutputDir, "modified.txt")); err != nil {
		t.Fatalf("modified snapshot missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resp.OutputDir, "extra.txt")); err != nil {
		t.Fatalf("extra snapshot missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resp.OutputDir, "good.txt")); !os.IsNotExist(err) {
		t.Fatalf("good file should not be copied, got err=%v", err)
	}
	if resp.MetadataPath == "" {
		t.Fatalf("expected metadata path, got %+v", resp)
	}
	if _, err := os.Stat(resp.MetadataPath); err != nil {
		t.Fatalf("snapshot.toml missing: %v", err)
	}
}

func TestJSONSnapshotFull(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "good.txt"), "good")
	writeCmdFile(t, filepath.Join(installRoot, "modified.txt"), "changed")
	writeCmdFile(t, filepath.Join(installRoot, "extra.txt"), "extra")

	lock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.2.0",
			Branch:       "advanced",
			InstallScope: "project",
			InstallRoot:  installRoot,
			InstallSite:  root,
			Files: map[string]string{
				"good.txt":     integrity.ComputeContentSHA256([]byte("good")),
				"modified.txt": integrity.ComputeContentSHA256([]byte("original")),
			},
		}},
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}
	snapshotNow = func() time.Time { return time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { snapshotNow = func() time.Time { return time.Now().UTC() } })
	snapshotGitRemoteURL = func(string) string { return "https://example.com/repo.git" }
	t.Cleanup(func() { snapshotGitRemoteURL = gitRemoteURL })

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "team-lead", "--scope", "project", "--full", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp snapshotResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.MetadataPath == "" || resp.Version != "1.2.0" || resp.Branch != "advanced" {
		t.Fatalf("unexpected full snapshot metadata: %+v", resp)
	}
	if len(resp.Files) != 3 {
		t.Fatalf("expected 3 files (full snapshot), got %d: %+v", len(resp.Files), resp.Files)
	}
	for _, name := range []string{"good.txt", "modified.txt", "extra.txt"} {
		if _, err := os.Stat(filepath.Join(resp.OutputDir, name)); err != nil {
			t.Fatalf("file %s missing from full snapshot: %v", name, err)
		}
	}
	if _, err := os.Stat(resp.MetadataPath); err != nil {
		t.Fatalf("snapshot.toml missing: %v", err)
	}
}

func TestJSONSnapshotAmbiguousTargetRequiresScope(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	setTestHome(t, home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	localRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	globalRoot := filepath.Join(home, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(localRoot, "SKILL.md"), "local")
	writeCmdFile(t, filepath.Join(globalRoot, "SKILL.md"), "global")
	if err := installer.SaveManifestLock(root, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_project",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "project",
		InstallRoot:  localRoot,
		InstallSite:  root,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("local"))},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock(local) error = %v", err)
	}
	if err := installer.SaveManifestLock(home, installer.ManifestLock{Installs: []installer.InstallRecord{{
		InstallID:    "pkg_team-lead_global",
		Package:      "team-lead",
		Version:      "1.0.0",
		Branch:       "main",
		InstallScope: "global",
		InstallRoot:  globalRoot,
		InstallSite:  home,
		Files:        map[string]string{"SKILL.md": integrity.ComputeContentSHA256([]byte("global"))},
	}}}); err != nil {
		t.Fatalf("SaveManifestLock(global) error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "team-lead", "--json"})
	requireJSONCmdError(t, cmd.Execute())

	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != api.ErrorCodeAmbiguousTarget {
		t.Fatalf("unexpected ambiguous snapshot response: %+v", resp)
	}
}

func TestValidateCommandAllFlag(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("USERPROFILE", home)
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	installRoot := filepath.Join(root, ".claude", "skills", "team-lead")
	writeCmdFile(t, filepath.Join(installRoot, "good.txt"), "good")

	lock := installer.ManifestLock{
		Installs: []installer.InstallRecord{{
			InstallID:    "pkg_team-lead_project",
			Package:      "team-lead",
			Version:      "1.0.0",
			Branch:       "main",
			InstallScope: "project",
			InstallRoot:  installRoot,
			InstallSite:  root,
			Files: map[string]string{
				"good.txt": integrity.ComputeContentSHA256([]byte("good")),
			},
		}},
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		t.Fatalf("SaveManifestLock() error = %v", err)
	}

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "--all", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp validateResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got %+v", resp)
	}
	if !resp.Pass {
		t.Fatalf("expected pass=true, got %+v", resp)
	}
	if len(resp.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(resp.Packages))
	}
}
