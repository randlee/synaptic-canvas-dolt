package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// fileTypePluralKey maps a FileType to the pluralized key used in the
// manifest artifacts map. These keys match the directory names in the
// export pipeline's target structure.
var fileTypePluralKey = map[FileType]string{
	FileTypeSkill:   "skills",
	FileTypeAgent:   "agents",
	FileTypeCommand: "commands",
	FileTypeScript:  "scripts",
	FileTypeHook:    "hooks",
	FileTypeConfig:  "configs",
}

// Manifest represents the reconstructed manifest.yaml structure built from
// relational data across the packages, package_files, package_deps,
// package_hooks, and package_questions tables. This is used by the export
// pipeline to produce the filesystem representation of a package.
type Manifest struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	Version      string                    `json:"version"`
	Description  string                    `json:"description,omitempty"`
	AgentVariant string                    `json:"agent_variant,omitempty"`
	Author       string                    `json:"author,omitempty"`
	License      string                    `json:"license,omitempty"`
	Tags         []string                  `json:"tags,omitempty"`
	InstallScope string                    `json:"install_scope,omitempty"`
	Variables    map[string]any            `json:"variables,omitempty"`
	Options      map[string]any            `json:"options,omitempty"`
	SHA256       string                    `json:"sha256,omitempty"`
	Artifacts    map[string][]ManifestFile `json:"artifacts,omitempty"`
	Requires     []string                  `json:"requires,omitempty"`
	Hooks        []ManifestHook            `json:"hooks,omitempty"`
	Questions    []ManifestQuestion        `json:"questions,omitempty"`
}

// ManifestFile is the file entry within a manifest artifacts group.
type ManifestFile struct {
	DestPath    string         `json:"dest_path"`
	SHA256      string         `json:"sha256"`
	FileType    FileType       `json:"file_type"`
	ContentType ContentType    `json:"content_type"`
	IsTemplate  bool           `json:"is_template,omitempty"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
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
	Choices    []string     `json:"choices,omitempty"`
	SortOrder  int          `json:"sort_order"`
}

// BuildManifest reconstructs a Manifest from a Package and its related data.
// The content of files is intentionally omitted from the manifest; the export
// pipeline writes file content separately.
//
// Artifacts are grouped by pluralized file_type key (skills, agents, etc.).
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
		ID:           pkg.ID,
		Name:         pkg.Name,
		Version:      pkg.Version,
		AgentVariant: pkg.AgentVariant,
		SHA256:       pkg.SHA256,
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
	if len(files) > 0 {
		m.Artifacts = make(map[string][]ManifestFile)
		for _, f := range files {
			mf := ManifestFile{
				DestPath:    f.DestPath,
				SHA256:      f.SHA256,
				FileType:    f.FileType,
				ContentType: f.ContentType,
				IsTemplate:  f.IsTemplate,
			}
			if len(f.Frontmatter) > 0 && string(f.Frontmatter) != "null" {
				if err := json.Unmarshal(f.Frontmatter, &mf.Frontmatter); err != nil {
					return nil, fmt.Errorf("building manifest: parsing frontmatter for %s: %w", f.DestPath, err)
				}
			}
			key, ok := fileTypePluralKey[f.FileType]
			if !ok {
				key = string(f.FileType) + "s"
			}
			m.Artifacts[key] = append(m.Artifacts[key], mf)
		}
	}

	// Build requires list from tool dependencies.
	// Format: "dep_name" or "dep_name dep_spec" if spec is non-empty.
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
		mq.Choices = q.ChoicesList()
		m.Questions = append(m.Questions, mq)
	}

	return m, nil
}
