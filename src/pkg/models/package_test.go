package models

import (
	"encoding/json"
	"testing"
)

func TestPackageTagsList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tags    json.RawMessage
		want    []string
		wantErr bool
	}{
		{
			name: "valid tags",
			tags: json.RawMessage(`["go","cli","tool"]`),
			want: []string{"go", "cli", "tool"},
		},
		{
			name: "empty tags",
			tags: nil,
			want: []string{},
		},
		{
			name: "null tags",
			tags: json.RawMessage(`null`),
			want: []string{},
		},
		{
			name: "single tag",
			tags: json.RawMessage(`["agent"]`),
			want: []string{"agent"},
		},
		{
			name:    "invalid json",
			tags:    json.RawMessage(`not json`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Package{Tags: tt.tags}
			got, err := p.TagsList()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
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
		choices json.RawMessage
		want    []string
		wantErr bool
	}{
		{
			name:    "valid choices",
			choices: json.RawMessage(`["yes","no","maybe"]`),
			want:    []string{"yes", "no", "maybe"},
		},
		{
			name:    "empty choices",
			choices: nil,
			want:    []string{},
		},
		{
			name:    "null choices",
			choices: json.RawMessage(`null`),
			want:    []string{},
		},
		{
			name:    "invalid json",
			choices: json.RawMessage(`{bad`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := &PackageQuestion{Choices: tt.choices}
			got, err := q.ChoicesList()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
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
		FileTypeAgent:   "agent",
		FileTypeCommand: "command",
		FileTypeSkill:   "skill",
		FileTypeHook:    "hook",
		FileTypeSnippet: "snippet",
		FileTypeConfig:  "config",
		FileTypeDoc:     "doc",
		FileTypeOther:   "other",
	}

	for ft, want := range expected {
		if string(ft) != want {
			t.Errorf("FileType %v = %q, want %q", ft, string(ft), want)
		}
	}
}

func TestContentTypeConstants(t *testing.T) {
	t.Parallel()

	expected := map[ContentType]string{
		ContentTypeText:     "text",
		ContentTypeBinary:   "binary",
		ContentTypeTemplate: "template",
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
		DepTypePackage: "package",
		DepTypeTool:    "tool",
		DepTypeRuntime: "runtime",
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
		HookPreInstall:    "pre-install",
		HookPostInstall:   "post-install",
		HookPreUninstall:  "pre-uninstall",
		HookPostUninstall: "post-uninstall",
		HookPreUpgrade:    "pre-upgrade",
		HookPostUpgrade:   "post-upgrade",
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
		QuestionText:        "text",
		QuestionBoolean:     "boolean",
		QuestionChoice:      "choice",
		QuestionMultiChoice: "multi-choice",
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
		Tags:         json.RawMessage(`["go","test"]`),
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
}
