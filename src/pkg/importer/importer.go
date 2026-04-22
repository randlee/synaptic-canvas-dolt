package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/template"
	"gopkg.in/yaml.v3"
)

var frontmatterPattern = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)

// Writer persists imported packages.
type Writer interface {
	BranchExists(ctx context.Context, branch string) (bool, error)
	ImportPackage(ctx context.Context, req dolt.ImportPackageRequest) error
}

// Service imports package directories into Dolt.
type Service struct {
	Writer Writer
}

// ImportRequest defines one import operation.
type ImportRequest struct {
	PackageDir string
	Branch     string
}

// Summary is the command output.
type Summary struct {
	PackageID                  string   `json:"package_id"`
	Version                    string   `json:"version"`
	Branch                     string   `json:"branch"`
	FilesImported              int      `json:"files_imported"`
	DepsImported               int      `json:"deps_imported"`
	HooksImported              int      `json:"hooks_imported"`
	QuestionsImported          int      `json:"questions_imported"`
	PackageSHA256              string   `json:"package_sha256"`
	CommitMessage              string   `json:"commit_message"`
	TemplateValidationWarnings []string `json:"template_validation_warnings,omitempty"`
}

type manifestFile struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description any    `yaml:"description"`
	Author      any    `yaml:"author"`
	License     string `yaml:"license"`
	Tags        any    `yaml:"tags"`
	Install     struct {
		Scope string `yaml:"scope"`
	} `yaml:"install"`
	Variables map[string]any      `yaml:"variables"`
	Options   map[string]any      `yaml:"options"`
	Artifacts map[string][]string `yaml:"artifacts"`
	Requires  []string            `yaml:"requires"`
	Questions []manifestQuestion  `yaml:"questions"`
	Hooks     []manifestHook      `yaml:"hooks"`
}

type manifestQuestion struct {
	QuestionID string   `yaml:"question_id"`
	Prompt     string   `yaml:"prompt"`
	Type       string   `yaml:"type"`
	DefaultVal string   `yaml:"default_val"`
	Choices    []string `yaml:"choices"`
	SortOrder  int      `yaml:"sort_order"`
}

type manifestHook struct {
	Event    string `yaml:"event"`
	Matcher  string `yaml:"matcher"`
	Script   string `yaml:"script"`
	Priority int    `yaml:"priority"`
	Blocking bool   `yaml:"blocking"`
}

// Import scans, validates, and writes a package.
func (s Service) Import(ctx context.Context, req ImportRequest) (*Summary, error) {
	if s.Writer == nil {
		return nil, fmt.Errorf("import writer is required")
	}
	exists, err := s.Writer.BranchExists(ctx, req.Branch)
	if err != nil {
		return nil, fmt.Errorf("checking branch %q: %w", req.Branch, err)
	}
	if !exists {
		return nil, fmt.Errorf("branch %q does not exist", req.Branch)
	}

	data, warnings, err := scanPackage(req.PackageDir)
	if err != nil {
		return nil, err
	}

	commitMessage := fmt.Sprintf("Import package %s %s", data.Package.ID, data.Package.Version)
	writeReq := dolt.ImportPackageRequest{
		Branch:        req.Branch,
		Package:       data.Package,
		Files:         data.Files,
		Deps:          data.Deps,
		Hooks:         data.Hooks,
		Questions:     data.Questions,
		PackageSHA256: data.PackageSHA256,
		CommitMessage: commitMessage,
	}
	if err := s.Writer.ImportPackage(ctx, writeReq); err != nil {
		return nil, err
	}

	return &Summary{
		PackageID:                  data.Package.ID,
		Version:                    data.Package.Version,
		Branch:                     req.Branch,
		FilesImported:              len(data.Files),
		DepsImported:               len(data.Deps),
		HooksImported:              len(data.Hooks),
		QuestionsImported:          len(data.Questions),
		PackageSHA256:              data.PackageSHA256,
		CommitMessage:              commitMessage,
		TemplateValidationWarnings: warnings,
	}, nil
}

type scannedPackage struct {
	Package       models.Package
	Files         []models.PackageFile
	Deps          []models.PackageDep
	Hooks         []models.PackageHook
	Questions     []models.PackageQuestion
	PackageSHA256 string
}

func scanPackage(dir string) (*scannedPackage, []string, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading manifest.yaml: %w", err)
	}

	var mf manifestFile
	if err := yaml.Unmarshal(rawManifest, &mf); err != nil {
		return nil, nil, fmt.Errorf("parsing manifest.yaml: %w", err)
	}
	if mf.Name == "" {
		return nil, nil, fmt.Errorf("manifest.yaml missing name")
	}
	if mf.Version == "" {
		return nil, nil, fmt.Errorf("manifest.yaml missing version")
	}

	pkg := models.Package{
		ID:           mf.Name,
		Name:         mf.Name,
		Version:      mf.Version,
		AgentVariant: "claude",
		InstallScope: models.InstallScopeAny,
	}
	if desc := normalizeString(mf.Description); desc != "" {
		pkg.Description = stringPtr(desc)
	}
	if author := normalizeString(mf.Author); author != "" {
		pkg.Author = stringPtr(author)
	}
	if mf.License != "" {
		pkg.License = stringPtr(mf.License)
	}
	if scope := strings.TrimSpace(mf.Install.Scope); scope != "" {
		pkg.InstallScope = models.InstallScope(scope)
	}
	pkg.Tags = normalizeTags(mf.Tags)
	if len(mf.Variables) > 0 {
		raw, err := json.Marshal(mf.Variables)
		if err != nil {
			return nil, nil, fmt.Errorf("encoding manifest variables: %w", err)
		}
		pkg.Variables = raw
	}
	if len(mf.Options) > 0 {
		raw, err := json.Marshal(mf.Options)
		if err != nil {
			return nil, nil, fmt.Errorf("encoding manifest options: %w", err)
		}
		pkg.Options = raw
	}

	files, templateContents, err := scanArtifacts(dir, mf.Name, mf.Artifacts)
	if err != nil {
		return nil, nil, err
	}

	deps := make([]models.PackageDep, 0, len(mf.Requires))
	for _, req := range mf.Requires {
		name, spec := parseRequirement(req)
		deps = append(deps, models.PackageDep{
			PackageID: mf.Name,
			DepType:   models.DepTypeTool,
			DepName:   name,
			DepSpec:   spec,
		})
	}

	questions := make([]models.PackageQuestion, 0, len(mf.Questions))
	questionIDs := make([]string, 0, len(mf.Questions))
	for _, q := range mf.Questions {
		choices := strings.Join(q.Choices, ",")
		questions = append(questions, models.PackageQuestion{
			PackageID:  mf.Name,
			QuestionID: q.QuestionID,
			Prompt:     q.Prompt,
			Type:       models.QuestionType(q.Type),
			DefaultVal: q.DefaultVal,
			Choices:    choices,
			SortOrder:  q.SortOrder,
		})
		questionIDs = append(questionIDs, q.QuestionID)
	}

	hooks := make([]models.PackageHook, 0, len(mf.Hooks))
	for _, h := range mf.Hooks {
		hooks = append(hooks, models.PackageHook{
			PackageID:  mf.Name,
			Event:      models.HookEvent(h.Event),
			Matcher:    h.Matcher,
			ScriptPath: h.Script,
			Priority:   h.Priority,
			Blocking:   h.Blocking,
		})
	}

	report := template.Validate(templateContents, questionIDs)
	warnings := make([]string, 0, len(report.Errors)+len(report.Warnings))
	for _, finding := range report.Errors {
		warnings = append(warnings, fmt.Sprintf("%s: %s", finding.File, finding.Message))
	}
	for _, finding := range report.Warnings {
		if finding.File != "" {
			warnings = append(warnings, fmt.Sprintf("%s: %s", finding.File, finding.Message))
		} else {
			warnings = append(warnings, finding.Message)
		}
	}

	aggregate := computePackageSHA(files)
	pkg.SHA256 = stringPtr(aggregate)

	return &scannedPackage{
		Package:       pkg,
		Files:         files,
		Deps:          deps,
		Hooks:         hooks,
		Questions:     questions,
		PackageSHA256: aggregate,
	}, warnings, nil
}

func scanArtifacts(root, packageID string, artifacts map[string][]string) ([]models.PackageFile, map[string]string, error) {
	var files []models.PackageFile
	templateContents := map[string]string{}
	for _, paths := range artifacts {
		for _, rel := range paths {
			full := filepath.Join(root, rel)
			content, err := os.ReadFile(full)
			if err != nil {
				return nil, nil, fmt.Errorf("reading artifact %s: %w", rel, err)
			}
			fileType, contentType := classifyFile(rel)
			text := string(content)
			sha := sha256.Sum256(content)
			pf := models.PackageFile{
				PackageID:   packageID,
				DestPath:    rel,
				Content:     text,
				SHA256:      hex.EncodeToString(sha[:]),
				FileType:    fileType,
				ContentType: contentType,
				IsTemplate:  strings.HasSuffix(rel, ".j2"),
			}
			if pf.ContentType == models.ContentTypeMarkdown {
				frontmatter, meta := extractFrontmatter(text)
				pf.Frontmatter = frontmatter
				pf.FMName = meta.name
				pf.FMDescription = meta.description
				pf.FMVersion = meta.version
				pf.FMModel = meta.model
			}
			files = append(files, pf)
			if pf.IsTemplate {
				templateContents[rel] = text
			}
		}
	}

	pluginPath := filepath.Join(root, ".claude-plugin", "plugin.json")
	if content, err := os.ReadFile(pluginPath); err == nil {
		sha := sha256.Sum256(content)
		files = append(files, models.PackageFile{
			PackageID:   packageID,
			DestPath:    ".claude-plugin/plugin.json",
			Content:     string(content),
			SHA256:      hex.EncodeToString(sha[:]),
			FileType:    models.FileTypeConfig,
			ContentType: models.ContentTypeJSON,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].DestPath < files[j].DestPath
	})
	return files, templateContents, nil
}

func classifyFile(path string) (models.FileType, models.ContentType) {
	lower := strings.ToLower(path)
	var ft models.FileType
	switch {
	case strings.HasPrefix(lower, "agents/"):
		ft = models.FileTypeAgent
	case strings.HasPrefix(lower, "commands/"):
		ft = models.FileTypeCommand
	case strings.HasPrefix(lower, "skills/"):
		ft = models.FileTypeSkill
	case strings.HasPrefix(lower, "scripts/"):
		ft = models.FileTypeScript
	case strings.HasPrefix(lower, "hooks/"):
		ft = models.FileTypeHook
	case lower == ".claude-plugin/plugin.json":
		ft = models.FileTypeConfig
	default:
		ft = models.FileTypeConfig
	}

	switch {
	case strings.HasSuffix(lower, ".md"), strings.HasSuffix(lower, ".md.j2"):
		return ft, models.ContentTypeMarkdown
	case strings.HasSuffix(lower, ".py"), strings.HasSuffix(lower, ".py.j2"):
		return ft, models.ContentTypePython
	case strings.HasSuffix(lower, ".json"), strings.HasSuffix(lower, ".json.j2"):
		return ft, models.ContentTypeJSON
	case strings.HasSuffix(lower, ".yaml"), strings.HasSuffix(lower, ".yml"), strings.HasSuffix(lower, ".yaml.j2"), strings.HasSuffix(lower, ".yml.j2"):
		return ft, models.ContentTypeYAML
	default:
		return ft, models.ContentTypeText
	}
}

type frontmatterMeta struct {
	name        *string
	description *string
	version     *string
	model       *string
}

func extractFrontmatter(content string) (json.RawMessage, frontmatterMeta) {
	match := frontmatterPattern.FindStringSubmatch(content)
	if len(match) != 2 {
		return nil, frontmatterMeta{}
	}
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(match[1]), &raw); err != nil {
		return nil, frontmatterMeta{}
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, frontmatterMeta{}
	}
	meta := frontmatterMeta{
		name:        optionalString(raw["name"]),
		description: optionalString(raw["description"]),
		version:     optionalString(raw["version"]),
		model:       optionalString(raw["model"]),
	}
	return encoded, meta
}

func computePackageSHA(files []models.PackageFile) string {
	parts := make([]string, 0, len(files))
	for _, file := range files {
		parts = append(parts, file.DestPath+":"+file.SHA256)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func parseRequirement(req string) (string, string) {
	fields := strings.Fields(strings.TrimSpace(req))
	if len(fields) == 0 {
		return "", ""
	}
	if len(fields) == 1 {
		return fields[0], ""
	}
	return fields[0], strings.Join(fields[1:], " ")
}

func normalizeString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return strings.TrimSpace(name)
		}
	}
	return ""
}

func normalizeTags(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := normalizeString(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(v, ",")
	default:
		return ""
	}
}

func optionalString(value any) *string {
	if text := normalizeString(value); text != "" {
		return &text
	}
	return nil
}

func stringPtr(value string) *string {
	return &value
}
