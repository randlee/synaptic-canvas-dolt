package manifest

import (
	"fmt"
	"sort"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"gopkg.in/yaml.v3"
)

type installConfig struct {
	Scope string `yaml:"scope"`
}

type renderedManifest struct {
	Name        string              `yaml:"name"`
	Version     string              `yaml:"version"`
	Description string              `yaml:"description,omitempty"`
	Author      string              `yaml:"author,omitempty"`
	License     string              `yaml:"license,omitempty"`
	Tags        []string            `yaml:"tags,omitempty"`
	Artifacts   map[string][]string `yaml:"artifacts,omitempty"`
	Variables   map[string]any      `yaml:"variables,omitempty"`
	Install     *installConfig      `yaml:"install,omitempty"`
	Options     map[string]any      `yaml:"options,omitempty"`
	Requires    []string            `yaml:"requires,omitempty"`
	Hooks       []renderedHook      `yaml:"hooks,omitempty"`
	Questions   []renderedQuestion  `yaml:"questions,omitempty"`
}

type renderedHook struct {
	Event      models.HookEvent `yaml:"event"`
	Matcher    string           `yaml:"matcher,omitempty"`
	ScriptPath string           `yaml:"script"`
	Priority   int              `yaml:"priority,omitempty"`
	Blocking   bool             `yaml:"blocking,omitempty"`
}

type renderedQuestion struct {
	QuestionID string              `yaml:"question_id"`
	Prompt     string              `yaml:"prompt"`
	Type       models.QuestionType `yaml:"type"`
	DefaultVal string              `yaml:"default_val,omitempty"`
	Choices    []string            `yaml:"choices,omitempty"`
	SortOrder  int                 `yaml:"sort_order,omitempty"`
}

// Reconstruct renders manifest.yaml content from relational package data.
func Reconstruct(
	pkg *models.Package,
	files []models.PackageFile,
	deps []models.PackageDep,
	hooks []models.PackageHook,
	questions []models.PackageQuestion,
) (string, error) {
	m, err := models.BuildManifest(pkg, files, deps, hooks, questions)
	if err != nil {
		return "", err
	}

	rendered := renderedManifest{
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
		Author:      m.Author,
		License:     m.License,
		Tags:        append([]string(nil), m.Tags...),
		Variables:   m.Variables,
		Options:     m.Options,
		Requires:    append([]string(nil), m.Requires...),
	}

	if len(m.Artifacts) > 0 {
		rendered.Artifacts = make(map[string][]string, len(m.Artifacts))
		for key, paths := range m.Artifacts {
			copied := append([]string(nil), paths...)
			sort.Strings(copied)
			rendered.Artifacts[key] = copied
		}
	}

	if m.InstallScope != "" {
		rendered.Install = &installConfig{Scope: m.InstallScope}
	}
	if len(m.Hooks) > 0 {
		rendered.Hooks = make([]renderedHook, 0, len(m.Hooks))
		for _, hook := range m.Hooks {
			rendered.Hooks = append(rendered.Hooks, renderedHook{
				Event:      hook.Event,
				Matcher:    hook.Matcher,
				ScriptPath: hook.ScriptPath,
				Priority:   hook.Priority,
				Blocking:   hook.Blocking,
			})
		}
	}
	if len(m.Questions) > 0 {
		rendered.Questions = make([]renderedQuestion, 0, len(m.Questions))
		for _, question := range m.Questions {
			rendered.Questions = append(rendered.Questions, renderedQuestion{
				QuestionID: question.QuestionID,
				Prompt:     question.Prompt,
				Type:       question.Type,
				DefaultVal: question.DefaultVal,
				Choices:    append([]string(nil), question.Choices...),
				SortOrder:  question.SortOrder,
			})
		}
	}

	data, err := yaml.Marshal(&rendered)
	if err != nil {
		return "", fmt.Errorf("rendering manifest yaml: %w", err)
	}
	return string(data), nil
}
