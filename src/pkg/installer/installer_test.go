package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestExecuteDryRun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	summary, err := (Service{}).Execute(context.Background(), Request{
		Package: &models.Package{
			ID:           "team-lead",
			Version:      "1.2.0",
			InstallScope: models.InstallScopeAny,
			AgentVariant: "claude",
		},
		Files: []models.PackageFile{{
			DestPath:   "SKILL.md.j2",
			Content:    "Hello {{ repo.name }}",
			SHA256:     "ignored",
			FileType:   models.FileTypeSkill,
			IsTemplate: true,
		}},
		Branch:   "main",
		DryRun:   true,
		RepoRoot: root,
		Now:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if summary.FilesWritten != 0 {
		t.Fatalf("expected dry run to avoid writes, got %d", summary.FilesWritten)
	}
	if len(summary.Files) != 1 || !strings.Contains(summary.Files[0].Preview, filepath.Base(root)) {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestExecuteWritesFilesAndTracking(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	resolvedRoot := mustEvalSymlinks(t, root)
	content := "# static"
	sum := shaHex([]byte(content))
	summary, err := (Service{}).Execute(context.Background(), Request{
		Package: &models.Package{
			ID:           "team-lead",
			Version:      "1.2.0",
			InstallScope: models.InstallScopeAny,
			AgentVariant: "claude",
		},
		Files: []models.PackageFile{
			{DestPath: "SKILL.md", Content: content, SHA256: sum, FileType: models.FileTypeSkill},
			{DestPath: "hooks/pre.sh", Content: "#!/bin/sh\n", SHA256: shaHex([]byte("#!/bin/sh\n")), FileType: models.FileTypeHook},
		},
		Hooks: []models.PackageHook{{
			Event:      models.HookPreToolUse,
			Matcher:    ".*",
			ScriptPath: "hooks/pre.sh",
			Priority:   10,
			Blocking:   true,
		}},
		Branch:   "main",
		RepoRoot: root,
		Now:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if summary.FilesWritten != 2 {
		t.Fatalf("FilesWritten = %d", summary.FilesWritten)
	}
	if _, err := os.Stat(filepath.Join(resolvedRoot, ".synaptic", "manifest.lock")); err != nil {
		t.Fatalf("manifest.lock missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mustEvalSymlinks(t, summary.InstallRoot), "SKILL.md")); err != nil {
		t.Fatalf("installed skill missing: %v", err)
	}
	registry, err := LoadHookRegistry(root)
	if err != nil {
		t.Fatalf("LoadHookRegistry() error = %v", err)
	}
	if len(registry.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %+v", registry)
	}
}

func TestExecuteRollbackOnAggregateMismatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content := "# static"
	sum := shaHex([]byte(content))
	_, err := (Service{}).Execute(context.Background(), Request{
		Package: &models.Package{
			ID:           "team-lead",
			Version:      "1.2.0",
			InstallScope: models.InstallScopeAny,
			AgentVariant: "claude",
			SHA256:       ptr("wrong-aggregate"),
		},
		Files: []models.PackageFile{
			{DestPath: "SKILL.md", Content: content, SHA256: sum, FileType: models.FileTypeSkill},
		},
		Branch:   "main",
		RepoRoot: root,
		Now:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "aggregate sha mismatch") {
		t.Fatalf("expected aggregate mismatch, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".claude", "skills", "team-lead", "SKILL.md")); !os.IsNotExist(statErr) {
		t.Fatalf("expected rollback to remove installed file, got err=%v", statErr)
	}
}

func TestExecuteGlobalWritesTrackingUnderHome(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	resolvedHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	content := "# static"
	sum := shaHex([]byte(content))
	_, err = (Service{}).Execute(context.Background(), Request{
		Package: &models.Package{
			ID:           "team-lead",
			Version:      "1.2.0",
			InstallScope: models.InstallScopeAny,
			AgentVariant: "claude",
		},
		Files: []models.PackageFile{
			{DestPath: "SKILL.md", Content: content, SHA256: sum, FileType: models.FileTypeSkill},
		},
		Global:   true,
		Branch:   "main",
		RepoRoot: root,
		Now:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(resolvedHome, ".synaptic", "manifest.lock")); statErr != nil {
		t.Fatalf("expected global manifest.lock under home, got %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".synaptic", "manifest.lock")); !os.IsNotExist(statErr) {
		t.Fatalf("did not expect project manifest.lock for global install, got %v", statErr)
	}
}

func TestRequirementSnapshotIsInstalledBySC(t *testing.T) {
	t.Parallel()

	snapshot := RequirementSnapshot{CLIProvenance: map[string]string{
		"owned":       "installed-by-synaptic",
		"preexisting": "already-present",
	}}
	if !snapshot.IsInstalledBySC("owned") {
		t.Fatal("expected owned dependency to be Synaptic-installed")
	}
	if snapshot.IsInstalledBySC("preexisting") || snapshot.IsInstalledBySC("missing") {
		t.Fatal("expected preexisting and missing dependencies to be ignored")
	}
}

func ptr(v string) *string { return &v }

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", path, err)
	}
	return resolved
}
