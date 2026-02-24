package models

import (
	"encoding/json"
	"fmt"
)

// Manifest represents the reconstructed manifest.yaml structure built from
// relational data across the packages, package_files, package_deps,
// package_hooks, and package_questions tables. This is used by the export
// pipeline to produce the filesystem representation of a package.
type Manifest struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Version      string             `json:"version"`
	Description  string             `json:"description,omitempty"`
	AgentVariant string             `json:"agent_variant,omitempty"`
	Author       string             `json:"author,omitempty"`
	License      string             `json:"license,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	InstallScope string             `json:"install_scope"`
	Variables    map[string]any     `json:"variables,omitempty"`
	Options      map[string]any     `json:"options,omitempty"`
	SHA256       string             `json:"sha256,omitempty"`
	Files        []ManifestFile     `json:"files,omitempty"`
	Deps         []ManifestDep      `json:"deps,omitempty"`
	Hooks        []ManifestHook     `json:"hooks,omitempty"`
	Questions    []ManifestQuestion `json:"questions,omitempty"`
}

// ManifestFile is the file entry within a manifest.
type ManifestFile struct {
	DestPath    string         `json:"dest_path"`
	SHA256      string         `json:"sha256"`
	FileType    FileType       `json:"file_type"`
	ContentType ContentType    `json:"content_type"`
	IsTemplate  bool           `json:"is_template,omitempty"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
}

// ManifestDep is the dependency entry within a manifest.
type ManifestDep struct {
	DepType    DepType `json:"dep_type"`
	DepName    string  `json:"dep_name"`
	DepSpec    string  `json:"dep_spec,omitempty"`
	InstallCmd string  `json:"install_cmd,omitempty"`
	CmdSHA256  string  `json:"cmd_sha256,omitempty"`
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
		InstallScope: pkg.InstallScope,
	}

	// Copy optional scalar fields.
	if pkg.Description != nil {
		m.Description = *pkg.Description
	}
	if pkg.AgentVariant != nil {
		m.AgentVariant = *pkg.AgentVariant
	}
	if pkg.Author != nil {
		m.Author = *pkg.Author
	}
	if pkg.License != nil {
		m.License = *pkg.License
	}
	if pkg.SHA256 != nil {
		m.SHA256 = *pkg.SHA256
	}

	// Parse JSON fields.
	tags, err := pkg.TagsList()
	if err != nil {
		return nil, fmt.Errorf("building manifest: %w", err)
	}
	m.Tags = tags

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

	// Convert files.
	m.Files = make([]ManifestFile, 0, len(files))
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
		m.Files = append(m.Files, mf)
	}

	// Convert dependencies.
	m.Deps = make([]ManifestDep, 0, len(deps))
	for _, d := range deps {
		md := ManifestDep{
			DepType: d.DepType,
			DepName: d.DepName,
		}
		if d.DepSpec != nil {
			md.DepSpec = *d.DepSpec
		}
		if d.InstallCmd != nil {
			md.InstallCmd = *d.InstallCmd
		}
		if d.CmdSHA256 != nil {
			md.CmdSHA256 = *d.CmdSHA256
		}
		m.Deps = append(m.Deps, md)
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
			SortOrder:  q.SortOrder,
		}
		if q.DefaultVal != nil {
			mq.DefaultVal = *q.DefaultVal
		}
		choices, err := q.ChoicesList()
		if err != nil {
			return nil, fmt.Errorf("building manifest: %w", err)
		}
		mq.Choices = choices
		m.Questions = append(m.Questions, mq)
	}

	return m, nil
}
