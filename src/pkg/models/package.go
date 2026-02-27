// Package models defines the data structures that map to the Synaptic Canvas
// Dolt database schema. Each struct corresponds to a table defined in the
// schema specification (docs/synaptic-canvas-schema.md).
package models

import (
	"encoding/json"
	"strings"
)

// Package represents a row in the packages table.
type Package struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Description  *string         `json:"description,omitempty"`
	AgentVariant string          `json:"agent_variant"`
	Author       *string         `json:"author,omitempty"`
	License      *string         `json:"license,omitempty"`
	Tags         string          `json:"tags,omitempty"`
	InstallScope string          `json:"install_scope"`
	Variables    json.RawMessage `json:"variables,omitempty"`
	Options      json.RawMessage `json:"options,omitempty"`
	MinClaudeVer *string         `json:"min_claude_version,omitempty"`
}

// TagsList splits the comma-separated tags field into a string slice.
// Returns an empty slice if tags is empty.
func (p *Package) TagsList() []string {
	if p.Tags == "" {
		return []string{}
	}
	parts := strings.Split(p.Tags, ",")
	result := make([]string, 0, len(parts))
	for _, t := range parts {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

// FileType enumerates the allowed values for package_files.file_type.
type FileType string

const (
	FileTypeSkill   FileType = "skill"
	FileTypeAgent   FileType = "agent"
	FileTypeCommand FileType = "command"
	FileTypeScript  FileType = "script"
	FileTypeHook    FileType = "hook"
	FileTypeConfig  FileType = "config"
)

// ContentType enumerates the allowed values for package_files.content_type.
type ContentType string

const (
	ContentTypeMarkdown ContentType = "markdown"
	ContentTypePython   ContentType = "python"
	ContentTypeJSON     ContentType = "json"
	ContentTypeYAML     ContentType = "yaml"
	ContentTypeText     ContentType = "text"
)

// PackageFile represents a row in the package_files table.
type PackageFile struct {
	PackageID     string          `json:"package_id"`
	DestPath      string          `json:"dest_path"`
	Content       string          `json:"content"`
	SHA256        string          `json:"sha256"`
	FileType      FileType        `json:"file_type"`
	ContentType   ContentType     `json:"content_type"`
	IsTemplate    bool            `json:"is_template"`
	Frontmatter   json.RawMessage `json:"frontmatter,omitempty"`
	FMName        *string         `json:"fm_name,omitempty"`
	FMDescription *string         `json:"fm_description,omitempty"`
	FMVersion     *string         `json:"fm_version,omitempty"`
	FMModel       *string         `json:"fm_model,omitempty"`
}

// DepType enumerates the allowed values for package_deps.dep_type.
type DepType string

const (
	DepTypeTool  DepType = "tool"
	DepTypeCLI   DepType = "cli"
	DepTypeSkill DepType = "skill"
)

// PackageDep represents a row in the package_deps table.
type PackageDep struct {
	PackageID  string  `json:"package_id"`
	DepType    DepType `json:"dep_type"`
	DepName    string  `json:"dep_name"`
	DepSpec    string  `json:"dep_spec,omitempty"`
	InstallCmd string  `json:"install_cmd,omitempty"`
	CmdSHA256  string  `json:"cmd_sha256,omitempty"`
}

// PackageVariant represents a row in the package_variants table.
type PackageVariant struct {
	LogicalID        string `json:"logical_id"`
	AgentProfile     string `json:"agent_profile"`
	VariantPackageID string `json:"variant_package_id"`
}

// HookEvent enumerates the allowed values for package_hooks.event.
type HookEvent string

const (
	HookPreToolUse  HookEvent = "PreToolUse"
	HookPostToolUse HookEvent = "PostToolUse"
)

// PackageHook represents a row in the package_hooks table.
type PackageHook struct {
	PackageID string    `json:"package_id"`
	Event     HookEvent `json:"event"`
	// Matcher is a regex pattern. Default is ".*" per schema spec.
	Matcher string `json:"matcher"`
	// ScriptPath must match a dest_path in package_files where file_type = "hook".
	ScriptPath string `json:"script_path"`
	Priority   int    `json:"priority"`
	Blocking   bool   `json:"blocking"`
}

// QuestionType enumerates the allowed values for package_questions.type.
type QuestionType string

const (
	QuestionChoice  QuestionType = "choice"
	QuestionMulti   QuestionType = "multi"
	QuestionText    QuestionType = "text"
	QuestionConfirm QuestionType = "confirm"
	QuestionAuto    QuestionType = "auto"
)

// PackageQuestion represents a row in the package_questions table.
type PackageQuestion struct {
	PackageID  string       `json:"package_id"`
	QuestionID string       `json:"question_id"`
	Prompt     string       `json:"prompt"`
	Type       QuestionType `json:"type"`
	DefaultVal string       `json:"default_val,omitempty"`
	Choices    string       `json:"choices,omitempty"`
	SortOrder  int          `json:"sort_order"`
}

// ChoicesList splits the comma-separated choices field into a string slice.
// Returns an empty slice if choices is empty.
func (q *PackageQuestion) ChoicesList() []string {
	if q.Choices == "" {
		return []string{}
	}
	parts := strings.Split(q.Choices, ",")
	result := make([]string, 0, len(parts))
	for _, c := range parts {
		c = strings.TrimSpace(c)
		if c != "" {
			result = append(result, c)
		}
	}
	return result
}
