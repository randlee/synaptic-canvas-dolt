package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// fileTypePluralKey maps a FileType to the pluralized key used in the
// manifest artifacts map. These keys match the directory names in the
// export pipeline's target structure.
// Note: FileTypeConfig is intentionally excluded — config files are handled
// separately in the export pipeline (written as plugin.json) and are not
// part of the manifest artifacts grouping.
var fileTypePluralKey = map[FileType]string{
	FileTypeSkill:   "skills",
	FileTypeAgent:   "agents",
	FileTypeCommand: "commands",
	FileTypeScript:  "scripts",
	FileTypeHook:    "hooks",
}

// Manifest represents the full in-memory package manifest, which is a superset
// of the export pipeline's manifest.yaml format. The base fields (Name, Version,
// Description, InstallScope, MinClaudeVersion, Requires, Artifacts) correspond
// to the export pipeline spec. Hooks and Questions extend the manifest for
// install-system orchestration and are not written to manifest.yaml.
//
// Built from relational data across the packages, package_files, package_deps,
// package_hooks, and package_questions tables.
type Manifest struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Version          string              `json:"version"`
	Description      string              `json:"description,omitempty"`
	Author           string              `json:"author,omitempty"`
	License          string              `json:"license,omitempty"`
	Tags             []string            `json:"tags,omitempty"`
	MinClaudeVersion string              `yaml:"min_claude_version,omitempty" json:"min_claude_version,omitempty"`
	InstallScope     string              `json:"install_scope,omitempty"`
	Variables        map[string]any      `json:"variables,omitempty"`
	Options          map[string]any      `json:"options,omitempty"`
	SHA256           string              `json:"sha256,omitempty"`
	Artifacts        map[string][]string `json:"artifacts,omitempty"`
	Requires         []string            `json:"requires,omitempty"`
	// Hooks and Questions extend the base manifest.yaml format defined in the
	// export pipeline spec. They are populated here for use by the install
	// system (see docs/synaptic-canvas-install-system.md and
	// docs/synaptic-canvas-hook-system.md) and are not part of the core
	// manifest.yaml output.
	Hooks     []ManifestHook     `json:"hooks,omitempty"`
	Questions []ManifestQuestion `json:"questions,omitempty"`
}

// ManifestHook is the hook entry within a manifest.
type ManifestHook struct {
	Event      HookEvent `json:"event"`
	Matcher    string    `json:"matcher"`
	ScriptPath string    `json:"script_path"`
	Priority   int       `json:"priority"`
	Blocking   bool      `json:"blocking"`
}

// ManifestQuestion is the question entry within a manifest.
type ManifestQuestion struct {
	QuestionID string       `json:"question_id"`
	Prompt     string       `json:"prompt"`
	Type       QuestionType `json:"type"`
	DefaultVal string       `json:"default_val,omitempty"`
	Choices    []string     `json:"choices,omitempty" yaml:"choices,omitempty"`
	SortOrder  int          `json:"sort_order"`
}

// BuildManifest reconstructs a Manifest from a Package and its related data.
// The content of files is intentionally omitted from the manifest; the export
// pipeline writes file content separately.
//
// Artifacts are grouped by pluralized file_type key (skills, agents, etc.)
// and contain only dest_path strings, matching the export pipeline spec.
// Tool dependencies are formatted into the Requires list.
// InstallScope is omitted if the value is "any".
func BuildManifest(
	pkg *Package,
	files []PackageFile,
	deps []PackageDep,
	hooks []PackageHook,
	questions []PackageQuestion,
) (*Manifest, error) {
	if pkg == nil {
		return nil, fmt.Errorf("building manifest: package is nil")
	}

	m := &Manifest{
		ID:      pkg.ID,
		Name:    pkg.Name,
		Version: pkg.Version,
		SHA256:  pkg.SHA256,
	}

	// Omit InstallScope if "any" (per export pipeline spec).
	if pkg.InstallScope != "any" {
		m.InstallScope = pkg.InstallScope
	}

	// Copy optional scalar fields.
	if pkg.Description != nil {
		m.Description = *pkg.Description
	}
	if pkg.Author != nil {
		m.Author = *pkg.Author
	}
	if pkg.License != nil {
		m.License = *pkg.License
	}
	if pkg.MinClaudeVer != nil {
		m.MinClaudeVersion = *pkg.MinClaudeVer
	}

	// Split comma-separated tags.
	m.Tags = pkg.TagsList()

	// Parse JSON fields.
	if len(pkg.Variables) > 0 && string(pkg.Variables) != "null" {
		if err := json.Unmarshal(pkg.Variables, &m.Variables); err != nil {
			return nil, fmt.Errorf("building manifest: parsing variables: %w", err)
		}
	}

	if len(pkg.Options) > 0 && string(pkg.Options) != "null" {
		if err := json.Unmarshal(pkg.Options, &m.Options); err != nil {
			return nil, fmt.Errorf("building manifest: parsing options: %w", err)
		}
	}

	// Group files into artifacts by pluralized file_type key.
	// Artifacts contain only dest_path strings per the export pipeline spec.
	// Files with FileTypeConfig are skipped (config files are handled
	// separately as plugin.json in the export pipeline).
	if len(files) > 0 {
		m.Artifacts = make(map[string][]string)
		for _, f := range files {
			key, ok := fileTypePluralKey[f.FileType]
			if !ok {
				// Skip file types not in the artifacts map (e.g. config).
				continue
			}
			m.Artifacts[key] = append(m.Artifacts[key], f.DestPath)
		}
	}

	// Build requires list from tool dependencies.
	// Format: "dep_name dep_spec" (space-separated). Export pipeline spec examples are ambiguous; using space for readability.
	for _, d := range deps {
		if d.DepType == DepTypeTool {
			entry := d.DepName
			spec := strings.TrimSpace(d.DepSpec)
			if spec != "" {
				entry += " " + spec
			}
			m.Requires = append(m.Requires, entry)
		}
	}

	// Convert hooks.
	m.Hooks = make([]ManifestHook, 0, len(hooks))
	for _, h := range hooks {
		m.Hooks = append(m.Hooks, ManifestHook{
			Event:      h.Event,
			Matcher:    h.Matcher,
			ScriptPath: h.ScriptPath,
			Priority:   h.Priority,
			Blocking:   h.Blocking,
		})
	}

	// Convert questions.
	m.Questions = make([]ManifestQuestion, 0, len(questions))
	for _, q := range questions {
		mq := ManifestQuestion{
			QuestionID: q.QuestionID,
			Prompt:     q.Prompt,
			Type:       q.Type,
			DefaultVal: q.DefaultVal,
			SortOrder:  q.SortOrder,
		}
		choices := q.ChoicesList()
		if len(choices) > 0 {
			mq.Choices = choices
		}
		m.Questions = append(m.Questions, mq)
	}

	return m, nil
}
