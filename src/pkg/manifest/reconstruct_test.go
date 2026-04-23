package manifest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"gopkg.in/yaml.v3"
)

func TestReconstruct(t *testing.T) {
	t.Parallel()

	desc := "Manage git worktrees with safeguards."
	author := "randlee"
	license := "MIT"
	minClaude := "1.0.32"
	pkg := &models.Package{
		ID:           "sc-git-worktree",
		Name:         "sc-git-worktree",
		Version:      "0.9.0",
		Description:  &desc,
		Author:       &author,
		License:      &license,
		Tags:         "git,worktree,workflow",
		InstallScope: models.InstallScopeLocalOnly,
		Variables:    json.RawMessage(`{"REPO_NAME":{"auto":"git-repo-basename"}}`),
		Options:      json.RawMessage(`{"no-tracking":{"type":"boolean","default":false}}`),
		MinClaudeVer: &minClaude,
	}
	files := []models.PackageFile{
		{DestPath: "scripts/worktree_scan.py", FileType: models.FileTypeScript},
		{DestPath: "commands/sc-git-worktree.md", FileType: models.FileTypeCommand},
		{DestPath: "skills/sc-git-worktree/SKILL.md", FileType: models.FileTypeSkill},
		{DestPath: "agents/sc-git-worktree-create.md", FileType: models.FileTypeAgent},
		{DestPath: ".claude-plugin/plugin.json", FileType: models.FileTypeConfig},
	}
	deps := []models.PackageDep{
		{DepType: models.DepTypeTool, DepName: "python3"},
		{DepType: models.DepTypeTool, DepName: "git", DepSpec: ">= 2.20"},
		{DepType: models.DepTypeCLI, DepName: "ignored-cli"},
	}

	got, err := Reconstruct(pkg, files, deps, nil, nil)
	if err != nil {
		t.Fatalf("Reconstruct() error = %v", err)
	}

	var decoded map[string]any
	if err := yaml.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded["name"] != "sc-git-worktree" {
		t.Fatalf("name = %v", decoded["name"])
	}
	artifacts, ok := decoded["artifacts"].(map[string]any)
	if !ok {
		t.Fatalf("artifacts missing: %#v", decoded["artifacts"])
	}
	if _, exists := artifacts["configs"]; exists {
		t.Fatalf("config files must not appear in artifacts: %#v", artifacts)
	}
	requires, ok := decoded["requires"].([]any)
	if !ok || len(requires) != 2 {
		t.Fatalf("requires = %#v", decoded["requires"])
	}
	if !strings.Contains(got, "install:\n    scope: local-only") {
		t.Fatalf("install scope missing:\n%s", got)
	}
}
