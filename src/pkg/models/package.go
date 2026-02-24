// Package models defines the data structures that map to the Synaptic Canvas
// Dolt database schema. Each struct corresponds to a table defined in the
// schema specification (docs/synaptic-canvas-schema.md).
package models

import (
	"encoding/json"
	"fmt"
)

// Package represents a row in the packages table.
type Package struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Description  *string         `json:"description,omitempty"`
	AgentVariant *string         `json:"agent_variant,omitempty"`
	Author       *string         `json:"author,omitempty"`
	License      *string         `json:"license,omitempty"`
	Tags         json.RawMessage `json:"tags,omitempty"`
	InstallScope string          `json:"install_scope"`
	Variables    json.RawMessage `json:"variables,omitempty"`
	Options      json.RawMessage `json:"options,omitempty"`
	SHA256       *string         `json:"sha256,omitempty"`
}

// TagsList parses the JSON tags field into a string slice.
// Returns an empty slice if tags is nil or empty.
func (p *Package) TagsList() ([]string, error) {
	if len(p.Tags) == 0 || string(p.Tags) == "null" {
		return []string{}, nil
	}
	var tags []string
	if err := json.Unmarshal(p.Tags, &tags); err != nil {
		return nil, fmt.Errorf("parsing tags: %w", err)
	}
	return tags, nil
}

// FileType enumerates the allowed values for package_files.file_type.
type FileType string

const (
	FileTypeAgent   FileType = "agent"
	FileTypeCommand FileType = "command"
	FileTypeSkill   FileType = "skill"
	FileTypeHook    FileType = "hook"
	FileTypeSnippet FileType = "snippet"
	FileTypeConfig  FileType = "config"
	FileTypeDoc     FileType = "doc"
	FileTypeOther   FileType = "other"
)

// ContentType enumerates the allowed values for package_files.content_type.
type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeBinary   ContentType = "binary"
	ContentTypeTemplate ContentType = "template"
)

// PackageFile represents a row in the package_files table.
type PackageFile struct {
	PackageID   string          `json:"package_id"`
	DestPath    string          `json:"dest_path"`
	Content     string          `json:"content"`
	SHA256      string          `json:"sha256"`
	FileType    FileType        `json:"file_type"`
	ContentType ContentType     `json:"content_type"`
	IsTemplate  bool            `json:"is_template"`
	Frontmatter json.RawMessage `json:"frontmatter,omitempty"`
}

// DepType enumerates the allowed values for package_deps.dep_type.
type DepType string

const (
	DepTypePackage DepType = "package"
	DepTypeTool    DepType = "tool"
	DepTypeRuntime DepType = "runtime"
)

// PackageDep represents a row in the package_deps table.
type PackageDep struct {
	PackageID  string  `json:"package_id"`
	DepType    DepType `json:"dep_type"`
	DepName    string  `json:"dep_name"`
	DepSpec    *string `json:"dep_spec,omitempty"`
	InstallCmd *string `json:"install_cmd,omitempty"`
	CmdSHA256  *string `json:"cmd_sha256,omitempty"`
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
	HookPreInstall    HookEvent = "pre-install"
	HookPostInstall   HookEvent = "post-install"
	HookPreUninstall  HookEvent = "pre-uninstall"
	HookPostUninstall HookEvent = "post-uninstall"
	HookPreUpgrade    HookEvent = "pre-upgrade"
	HookPostUpgrade   HookEvent = "post-upgrade"
)

// PackageHook represents a row in the package_hooks table.
type PackageHook struct {
	PackageID  string    `json:"package_id"`
	Event      HookEvent `json:"event"`
	Matcher    string    `json:"matcher"`
	ScriptPath string    `json:"script_path"`
	Priority   int       `json:"priority"`
	Blocking   bool      `json:"blocking"`
}

// QuestionType enumerates the allowed values for package_questions.type.
type QuestionType string

const (
	QuestionText        QuestionType = "text"
	QuestionBoolean     QuestionType = "boolean"
	QuestionChoice      QuestionType = "choice"
	QuestionMultiChoice QuestionType = "multi-choice"
)

// PackageQuestion represents a row in the package_questions table.
type PackageQuestion struct {
	PackageID  string          `json:"package_id"`
	QuestionID string          `json:"question_id"`
	Prompt     string          `json:"prompt"`
	Type       QuestionType    `json:"type"`
	DefaultVal *string         `json:"default_val,omitempty"`
	Choices    json.RawMessage `json:"choices,omitempty"`
	SortOrder  int             `json:"sort_order"`
}

// ChoicesList parses the JSON choices field into a string slice.
// Returns an empty slice if choices is nil or empty.
func (q *PackageQuestion) ChoicesList() ([]string, error) {
	if len(q.Choices) == 0 || string(q.Choices) == "null" {
		return []string{}, nil
	}
	var choices []string
	if err := json.Unmarshal(q.Choices, &choices); err != nil {
		return nil, fmt.Errorf("parsing choices: %w", err)
	}
	return choices, nil
}
