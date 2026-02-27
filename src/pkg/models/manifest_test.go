package models

import (
	"encoding/json"
	"testing"
)

func TestBuildManifestNilPackage(t *testing.T) {
	t.Parallel()
	_, err := BuildManifest(nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil package")
	}
}

func TestBuildManifestMinimal(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "any",
	}

	m, err := BuildManifest(pkg, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != "pkg-1" {
		t.Errorf("ID = %q, want %q", m.ID, "pkg-1")
	}
	if m.Name != "test" {
		t.Errorf("Name = %q, want %q", m.Name, "test")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	// InstallScope "any" should be omitted (empty string in manifest).
	if m.InstallScope != "" {
		t.Errorf("InstallScope = %q, want empty (omitted for 'any')", m.InstallScope)
	}
	if len(m.Artifacts) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(m.Artifacts))
	}
	if len(m.Requires) != 0 {
		t.Errorf("expected 0 requires, got %d", len(m.Requires))
	}
	if len(m.Hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(m.Hooks))
	}
	if len(m.Questions) != 0 {
		t.Errorf("expected 0 questions, got %d", len(m.Questions))
	}
}

func TestBuildManifestInstallScopeAnyOmitted(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-any",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "any",
	}

	m, err := BuildManifest(pkg, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.InstallScope != "" {
		t.Errorf("InstallScope = %q, want empty (omitted for 'any')", m.InstallScope)
	}
}

func TestBuildManifestOptionalFields(t *testing.T) {
	t.Parallel()

	desc := "A test package"
	author := "test-author"
	license := "MIT"
	minClaude := "1.0.32"

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		Description:  &desc,
		AgentVariant: "claude-code",
		Author:       &author,
		License:      &license,
		InstallScope: "local-only",
		Tags:         "go,cli",
		Variables:    json.RawMessage(`{"key":"val"}`),
		Options:      json.RawMessage(`{"opt":true}`),
		MinClaudeVer: &minClaude,
	}

	m, err := BuildManifest(pkg, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Description != desc {
		t.Errorf("Description = %q, want %q", m.Description, desc)
	}
	if m.Author != author {
		t.Errorf("Author = %q, want %q", m.Author, author)
	}
	if m.License != license {
		t.Errorf("License = %q, want %q", m.License, license)
	}
	if m.MinClaudeVersion != "1.0.32" {
		t.Errorf("MinClaudeVersion = %q, want %q", m.MinClaudeVersion, "1.0.32")
	}
	if m.InstallScope != "local-only" {
		t.Errorf("InstallScope = %q, want %q", m.InstallScope, "local-only")
	}
	if len(m.Tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(m.Tags))
	}
	if m.Tags[0] != "go" || m.Tags[1] != "cli" {
		t.Errorf("Tags = %v, want [go cli]", m.Tags)
	}
	if m.Variables["key"] != "val" {
		t.Errorf("Variables[key] = %v, want val", m.Variables["key"])
	}
	if m.Options["opt"] != true {
		t.Errorf("Options[opt] = %v, want true", m.Options["opt"])
	}
}

func TestBuildManifestWithFiles(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "any",
	}

	files := []PackageFile{
		{
			PackageID:   "pkg-1",
			DestPath:    "agent.md",
			Content:     "# Agent",
			SHA256:      "sha1",
			FileType:    FileTypeAgent,
			ContentType: ContentTypeMarkdown,
			IsTemplate:  false,
		},
		{
			PackageID:   "pkg-1",
			DestPath:    "config.json",
			Content:     "{}",
			SHA256:      "sha2",
			FileType:    FileTypeConfig,
			ContentType: ContentTypeJSON,
			IsTemplate:  true,
			Frontmatter: json.RawMessage(`{"title":"Config"}`),
		},
	}

	m, err := BuildManifest(pkg, files, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Agent files should be grouped into artifacts as dest_path strings.
	agents, ok := m.Artifacts["agents"]
	if !ok {
		t.Fatal("expected artifacts[agents] to exist")
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	if agents[0] != "agent.md" {
		t.Errorf("agents[0] = %q, want %q", agents[0], "agent.md")
	}

	// Config files should NOT appear in artifacts (they are handled
	// separately by the export pipeline as plugin.json).
	if _, ok := m.Artifacts["configs"]; ok {
		t.Error("config files should not appear in artifacts")
	}
}

func TestBuildManifestWithDeps(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "any",
	}

	deps := []PackageDep{
		{PackageID: "pkg-1", DepType: DepTypeTool, DepName: "tool-x", DepSpec: ">=1.0.0"},
		{PackageID: "pkg-1", DepType: DepTypeTool, DepName: "tool-y"},
		{PackageID: "pkg-1", DepType: DepTypeCLI, DepName: "cli-z", DepSpec: "^2.0"},
	}

	m, err := BuildManifest(pkg, nil, deps, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only tool deps go into Requires. CLI deps do not.
	if len(m.Requires) != 2 {
		t.Fatalf("got %d requires, want 2", len(m.Requires))
	}
	if m.Requires[0] != "tool-x >=1.0.0" {
		t.Errorf("Requires[0] = %q, want %q", m.Requires[0], "tool-x >=1.0.0")
	}
	if m.Requires[1] != "tool-y" {
		t.Errorf("Requires[1] = %q, want %q", m.Requires[1], "tool-y")
	}
}

func TestBuildManifestWithHooks(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "any",
	}

	hooks := []PackageHook{
		{PackageID: "pkg-1", Event: HookPostToolUse, Matcher: "**/*.md", ScriptPath: "hooks/post.sh", Priority: 10, Blocking: true},
	}

	m, err := BuildManifest(pkg, nil, nil, hooks, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Hooks) != 1 {
		t.Fatalf("got %d hooks, want 1", len(m.Hooks))
	}
	if m.Hooks[0].Event != HookPostToolUse {
		t.Errorf("Hooks[0].Event = %q, want %q", m.Hooks[0].Event, HookPostToolUse)
	}
	if m.Hooks[0].Priority != 10 {
		t.Errorf("Hooks[0].Priority = %d, want 10", m.Hooks[0].Priority)
	}
	if !m.Hooks[0].Blocking {
		t.Error("Hooks[0].Blocking should be true")
	}
}

func TestBuildManifestWithQuestions(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "any",
	}

	questions := []PackageQuestion{
		{
			PackageID:  "pkg-1",
			QuestionID: "q1",
			Prompt:     "Enable feature?",
			Type:       QuestionConfirm,
			DefaultVal: "yes",
			SortOrder:  1,
		},
		{
			PackageID:  "pkg-1",
			QuestionID: "q2",
			Prompt:     "Choose mode",
			Type:       QuestionChoice,
			Choices:    "fast,slow",
			SortOrder:  2,
		},
	}

	m, err := BuildManifest(pkg, nil, nil, nil, questions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Questions) != 2 {
		t.Fatalf("got %d questions, want 2", len(m.Questions))
	}
	if m.Questions[0].DefaultVal != "yes" {
		t.Errorf("Questions[0].DefaultVal = %q, want %q", m.Questions[0].DefaultVal, "yes")
	}
	// Confirm question has no choices — Choices should be nil (omitted).
	if m.Questions[0].Choices != nil {
		t.Errorf("Questions[0].Choices should be nil, got %v", m.Questions[0].Choices)
	}
	if len(m.Questions[1].Choices) != 2 {
		t.Fatalf("Questions[1].Choices = %v, want 2 choices", m.Questions[1].Choices)
	}
	if m.Questions[1].Choices[0] != "fast" {
		t.Errorf("Questions[1].Choices[0] = %q, want %q", m.Questions[1].Choices[0], "fast")
	}
}

func TestBuildManifestInvalidVariables(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "any",
		Variables:    json.RawMessage(`not json`),
	}

	_, err := BuildManifest(pkg, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid variables JSON")
	}
}

func TestBuildManifestInvalidOptions(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "any",
		Options:      json.RawMessage(`{bad`),
	}

	_, err := BuildManifest(pkg, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid options JSON")
	}
}

func TestBuildManifestFullIntegration(t *testing.T) {
	t.Parallel()

	desc := "Full integration test"
	author := "tester"
	minClaude := "1.2.0"

	pkg := &Package{
		ID:           "full-pkg",
		Name:         "full",
		Version:      "2.0.0",
		Description:  &desc,
		Author:       &author,
		InstallScope: "local-only",
		Tags:         "integration",
		MinClaudeVer: &minClaude,
	}

	files := []PackageFile{
		{PackageID: "full-pkg", DestPath: "skill.md", Content: "# Skill", SHA256: "s1", FileType: FileTypeSkill, ContentType: ContentTypeMarkdown},
	}

	deps := []PackageDep{
		{PackageID: "full-pkg", DepType: DepTypeTool, DepName: "node", DepSpec: "^1.0"},
	}

	hooks := []PackageHook{
		{PackageID: "full-pkg", Event: HookPreToolUse, Matcher: "*", ScriptPath: "pre.sh", Priority: 1, Blocking: false},
	}

	questions := []PackageQuestion{
		{PackageID: "full-pkg", QuestionID: "q1", Prompt: "Name?", Type: QuestionText, DefaultVal: "default", SortOrder: 1},
	}

	m, err := BuildManifest(pkg, files, deps, hooks, questions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.ID != "full-pkg" {
		t.Errorf("ID = %q, want %q", m.ID, "full-pkg")
	}
	if m.InstallScope != "local-only" {
		t.Errorf("InstallScope = %q, want %q", m.InstallScope, "local-only")
	}
	if m.MinClaudeVersion != "1.2.0" {
		t.Errorf("MinClaudeVersion = %q, want %q", m.MinClaudeVersion, "1.2.0")
	}

	// Files are in artifacts map as dest_path strings.
	skills, ok := m.Artifacts["skills"]
	if !ok || len(skills) != 1 {
		t.Errorf("expected 1 skill in artifacts, got %v", m.Artifacts)
	}
	if len(skills) > 0 && skills[0] != "skill.md" {
		t.Errorf("skills[0] = %q, want %q", skills[0], "skill.md")
	}

	// Tool deps become requires entries.
	if len(m.Requires) != 1 {
		t.Errorf("Requires count = %d, want 1", len(m.Requires))
	}
	if len(m.Requires) > 0 && m.Requires[0] != "node ^1.0" {
		t.Errorf("Requires[0] = %q, want %q", m.Requires[0], "node ^1.0")
	}

	if len(m.Hooks) != 1 {
		t.Errorf("Hooks count = %d, want 1", len(m.Hooks))
	}
	if len(m.Questions) != 1 {
		t.Errorf("Questions count = %d, want 1", len(m.Questions))
	}
	if len(m.Questions) > 0 && m.Questions[0].DefaultVal != "default" {
		t.Errorf("Questions[0].DefaultVal = %q, want %q", m.Questions[0].DefaultVal, "default")
	}
}
