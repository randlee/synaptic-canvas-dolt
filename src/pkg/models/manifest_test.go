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
		InstallScope: "local",
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
	if m.InstallScope != "local" {
		t.Errorf("InstallScope = %q, want %q", m.InstallScope, "local")
	}
	if len(m.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.Files))
	}
	if len(m.Deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(m.Deps))
	}
	if len(m.Hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(m.Hooks))
	}
	if len(m.Questions) != 0 {
		t.Errorf("expected 0 questions, got %d", len(m.Questions))
	}
}

func TestBuildManifestOptionalFields(t *testing.T) {
	t.Parallel()

	desc := "A test package"
	variant := "claude-code"
	author := "test-author"
	license := "MIT"
	sha := "abc123"

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		Description:  &desc,
		AgentVariant: &variant,
		Author:       &author,
		License:      &license,
		InstallScope: "global",
		SHA256:       &sha,
		Tags:         json.RawMessage(`["go","cli"]`),
		Variables:    json.RawMessage(`{"key":"val"}`),
		Options:      json.RawMessage(`{"opt":true}`),
	}

	m, err := BuildManifest(pkg, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Description != desc {
		t.Errorf("Description = %q, want %q", m.Description, desc)
	}
	if m.AgentVariant != variant {
		t.Errorf("AgentVariant = %q, want %q", m.AgentVariant, variant)
	}
	if m.Author != author {
		t.Errorf("Author = %q, want %q", m.Author, author)
	}
	if m.License != license {
		t.Errorf("License = %q, want %q", m.License, license)
	}
	if m.SHA256 != sha {
		t.Errorf("SHA256 = %q, want %q", m.SHA256, sha)
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
		InstallScope: "local",
	}

	files := []PackageFile{
		{
			PackageID:   "pkg-1",
			DestPath:    "agent.md",
			Content:     "# Agent",
			SHA256:      "sha1",
			FileType:    FileTypeAgent,
			ContentType: ContentTypeText,
			IsTemplate:  false,
		},
		{
			PackageID:   "pkg-1",
			DestPath:    "config.json",
			Content:     "{}",
			SHA256:      "sha2",
			FileType:    FileTypeConfig,
			ContentType: ContentTypeText,
			IsTemplate:  true,
			Frontmatter: json.RawMessage(`{"title":"Config"}`),
		},
	}

	m, err := BuildManifest(pkg, files, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(m.Files))
	}
	if m.Files[0].DestPath != "agent.md" {
		t.Errorf("Files[0].DestPath = %q, want %q", m.Files[0].DestPath, "agent.md")
	}
	if m.Files[1].IsTemplate != true {
		t.Error("Files[1].IsTemplate should be true")
	}
	if m.Files[1].Frontmatter["title"] != "Config" {
		t.Errorf("Files[1].Frontmatter[title] = %v, want Config", m.Files[1].Frontmatter["title"])
	}
}

func TestBuildManifestWithDeps(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "local",
	}

	spec := ">=1.0.0"
	installCmd := "go install example.com/tool@latest"
	cmdSHA := "sha-cmd"

	deps := []PackageDep{
		{PackageID: "pkg-1", DepType: DepTypePackage, DepName: "other-pkg", DepSpec: &spec},
		{PackageID: "pkg-1", DepType: DepTypeTool, DepName: "tool-x", InstallCmd: &installCmd, CmdSHA256: &cmdSHA},
	}

	m, err := BuildManifest(pkg, nil, deps, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Deps) != 2 {
		t.Fatalf("got %d deps, want 2", len(m.Deps))
	}
	if m.Deps[0].DepSpec != spec {
		t.Errorf("Deps[0].DepSpec = %q, want %q", m.Deps[0].DepSpec, spec)
	}
	if m.Deps[1].InstallCmd != installCmd {
		t.Errorf("Deps[1].InstallCmd = %q, want %q", m.Deps[1].InstallCmd, installCmd)
	}
	if m.Deps[1].CmdSHA256 != cmdSHA {
		t.Errorf("Deps[1].CmdSHA256 = %q, want %q", m.Deps[1].CmdSHA256, cmdSHA)
	}
}

func TestBuildManifestWithHooks(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "local",
	}

	hooks := []PackageHook{
		{PackageID: "pkg-1", Event: HookPostInstall, Matcher: "**/*.md", ScriptPath: "hooks/post.sh", Priority: 10, Blocking: true},
	}

	m, err := BuildManifest(pkg, nil, nil, hooks, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Hooks) != 1 {
		t.Fatalf("got %d hooks, want 1", len(m.Hooks))
	}
	if m.Hooks[0].Event != HookPostInstall {
		t.Errorf("Hooks[0].Event = %q, want %q", m.Hooks[0].Event, HookPostInstall)
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
		InstallScope: "local",
	}

	defaultVal := "yes"
	questions := []PackageQuestion{
		{
			PackageID:  "pkg-1",
			QuestionID: "q1",
			Prompt:     "Enable feature?",
			Type:       QuestionBoolean,
			DefaultVal: &defaultVal,
			SortOrder:  1,
		},
		{
			PackageID:  "pkg-1",
			QuestionID: "q2",
			Prompt:     "Choose mode",
			Type:       QuestionChoice,
			Choices:    json.RawMessage(`["fast","slow"]`),
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
	if m.Questions[0].DefaultVal != defaultVal {
		t.Errorf("Questions[0].DefaultVal = %q, want %q", m.Questions[0].DefaultVal, defaultVal)
	}
	if len(m.Questions[1].Choices) != 2 {
		t.Fatalf("Questions[1].Choices = %v, want 2 choices", m.Questions[1].Choices)
	}
	if m.Questions[1].Choices[0] != "fast" {
		t.Errorf("Questions[1].Choices[0] = %q, want %q", m.Questions[1].Choices[0], "fast")
	}
}

func TestBuildManifestInvalidTags(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "local",
		Tags:         json.RawMessage(`not json`),
	}

	_, err := BuildManifest(pkg, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid tags JSON")
	}
}

func TestBuildManifestInvalidVariables(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "local",
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
		InstallScope: "local",
		Options:      json.RawMessage(`{bad`),
	}

	_, err := BuildManifest(pkg, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid options JSON")
	}
}

func TestBuildManifestInvalidFrontmatter(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "local",
	}

	files := []PackageFile{
		{PackageID: "pkg-1", DestPath: "bad.md", SHA256: "x", FileType: FileTypeDoc, ContentType: ContentTypeText, Frontmatter: json.RawMessage(`{bad`)},
	}

	_, err := BuildManifest(pkg, files, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid frontmatter JSON")
	}
}

func TestBuildManifestInvalidChoices(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		ID:           "pkg-1",
		Name:         "test",
		Version:      "1.0.0",
		InstallScope: "local",
	}

	questions := []PackageQuestion{
		{PackageID: "pkg-1", QuestionID: "q1", Prompt: "Bad", Type: QuestionChoice, Choices: json.RawMessage(`not json`), SortOrder: 1},
	}

	_, err := BuildManifest(pkg, nil, nil, nil, questions)
	if err == nil {
		t.Fatal("expected error for invalid choices JSON")
	}
}

func TestBuildManifestFullIntegration(t *testing.T) {
	t.Parallel()

	desc := "Full integration test"
	author := "tester"

	pkg := &Package{
		ID:           "full-pkg",
		Name:         "full",
		Version:      "2.0.0",
		Description:  &desc,
		Author:       &author,
		InstallScope: "global",
		Tags:         json.RawMessage(`["integration"]`),
	}

	files := []PackageFile{
		{PackageID: "full-pkg", DestPath: "skill.md", Content: "# Skill", SHA256: "s1", FileType: FileTypeSkill, ContentType: ContentTypeText},
	}

	spec := "^1.0"
	deps := []PackageDep{
		{PackageID: "full-pkg", DepType: DepTypeRuntime, DepName: "node", DepSpec: &spec},
	}

	hooks := []PackageHook{
		{PackageID: "full-pkg", Event: HookPreInstall, Matcher: "*", ScriptPath: "pre.sh", Priority: 1, Blocking: false},
	}

	defVal := "default"
	questions := []PackageQuestion{
		{PackageID: "full-pkg", QuestionID: "q1", Prompt: "Name?", Type: QuestionText, DefaultVal: &defVal, SortOrder: 1},
	}

	m, err := BuildManifest(pkg, files, deps, hooks, questions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.ID != "full-pkg" {
		t.Errorf("ID = %q, want %q", m.ID, "full-pkg")
	}
	if len(m.Files) != 1 {
		t.Errorf("Files count = %d, want 1", len(m.Files))
	}
	if len(m.Deps) != 1 {
		t.Errorf("Deps count = %d, want 1", len(m.Deps))
	}
	if len(m.Hooks) != 1 {
		t.Errorf("Hooks count = %d, want 1", len(m.Hooks))
	}
	if len(m.Questions) != 1 {
		t.Errorf("Questions count = %d, want 1", len(m.Questions))
	}
	if m.Questions[0].DefaultVal != defVal {
		t.Errorf("Questions[0].DefaultVal = %q, want %q", m.Questions[0].DefaultVal, defVal)
	}
}
