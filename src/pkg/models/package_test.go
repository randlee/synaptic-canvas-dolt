package models

import (
	"encoding/json"
	"testing"
)

func TestPackageTagsList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags string
		want []string
	}{
		{
			name: "multiple tags",
			tags: "go,cli,tool",
			want: []string{"go", "cli", "tool"},
		},
		{
			name: "empty tags",
			tags: "",
			want: []string{},
		},
		{
			name: "single tag",
			tags: "agent",
			want: []string{"agent"},
		},
		{
			name: "tags with spaces",
			tags: "go , cli , tool",
			want: []string{"go", "cli", "tool"},
		},
		{
			name: "trailing comma",
			tags: "go,cli,",
			want: []string{"go", "cli"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Package{Tags: tt.tags}
			got := p.TagsList()
			if len(got) != len(tt.want) {
				t.Fatalf("got %d tags, want %d", len(got), len(tt.want))
			}
			for i, tag := range got {
				if tag != tt.want[i] {
					t.Errorf("tag[%d] = %q, want %q", i, tag, tt.want[i])
				}
			}
		})
	}
}

func TestPackageQuestionChoicesList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		choices string
		want    []string
	}{
		{
			name:    "valid choices",
			choices: "yes,no,maybe",
			want:    []string{"yes", "no", "maybe"},
		},
		{
			name:    "empty choices",
			choices: "",
			want:    []string{},
		},
		{
			name:    "single choice",
			choices: "only",
			want:    []string{"only"},
		},
		{
			name:    "choices with spaces",
			choices: "fast , slow , medium",
			want:    []string{"fast", "slow", "medium"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := &PackageQuestion{Choices: tt.choices}
			got := q.ChoicesList()
			if len(got) != len(tt.want) {
				t.Fatalf("got %d choices, want %d", len(got), len(tt.want))
			}
			for i, choice := range got {
				if choice != tt.want[i] {
					t.Errorf("choice[%d] = %q, want %q", i, choice, tt.want[i])
				}
			}
		})
	}
}

func TestFileTypeConstants(t *testing.T) {
	t.Parallel()

	expected := map[FileType]string{
		FileTypeSkill:   "skill",
		FileTypeAgent:   "agent",
		FileTypeCommand: "command",
		FileTypeScript:  "script",
		FileTypeHook:    "hook",
		FileTypeConfig:  "config",
	}

	for ft, want := range expected {
		if string(ft) != want {
			t.Errorf("FileType %v = %q, want %q", ft, string(ft), want)
		}
	}

	// Verify exactly 6 file types are defined.
	if len(expected) != 6 {
		t.Errorf("expected 6 FileType constants, got %d", len(expected))
	}
}

func TestContentTypeConstants(t *testing.T) {
	t.Parallel()

	expected := map[ContentType]string{
		ContentTypeMarkdown: "markdown",
		ContentTypePython:   "python",
		ContentTypeJSON:     "json",
		ContentTypeYAML:     "yaml",
		ContentTypeText:     "text",
	}

	for ct, want := range expected {
		if string(ct) != want {
			t.Errorf("ContentType %v = %q, want %q", ct, string(ct), want)
		}
	}
}

func TestDepTypeConstants(t *testing.T) {
	t.Parallel()

	expected := map[DepType]string{
		DepTypeTool:  "tool",
		DepTypeCLI:   "cli",
		DepTypeSkill: "skill",
	}

	for dt, want := range expected {
		if string(dt) != want {
			t.Errorf("DepType %v = %q, want %q", dt, string(dt), want)
		}
	}
}

func TestHookEventConstants(t *testing.T) {
	t.Parallel()

	expected := map[HookEvent]string{
		HookPreToolUse:  "PreToolUse",
		HookPostToolUse: "PostToolUse",
	}

	for he, want := range expected {
		if string(he) != want {
			t.Errorf("HookEvent %v = %q, want %q", he, string(he), want)
		}
	}
}

func TestQuestionTypeConstants(t *testing.T) {
	t.Parallel()

	expected := map[QuestionType]string{
		QuestionChoice:  "choice",
		QuestionMulti:   "multi",
		QuestionText:    "text",
		QuestionConfirm: "confirm",
		QuestionAuto:    "auto",
	}

	for qt, want := range expected {
		if string(qt) != want {
			t.Errorf("QuestionType %v = %q, want %q", qt, string(qt), want)
		}
	}
}

func TestPackageJSONSerialization(t *testing.T) {
	t.Parallel()

	desc := "A test package"
	p := Package{
		ID:           "test-pkg",
		Name:         "test",
		Version:      "1.0.0",
		Description:  &desc,
		InstallScope: "local",
		Tags:         "go,test",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Package
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != p.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, p.ID)
	}
	if decoded.Name != p.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, p.Name)
	}
	if decoded.Description == nil || *decoded.Description != desc {
		t.Errorf("Description mismatch")
	}
	if decoded.Tags != "go,test" {
		t.Errorf("Tags = %q, want %q", decoded.Tags, "go,test")
	}
}
