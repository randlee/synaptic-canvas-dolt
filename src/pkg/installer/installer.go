package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/questionnaire"
	"github.com/randlee/synaptic-canvas-dolt/pkg/repo"
)

var unresolvedTemplatePattern = regexp.MustCompile(`(?s)\{\{.*?\}\}|\{%.*?%\}`)
var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// Request describes one install execution.
type Request struct {
	Package   *models.Package
	Files     []models.PackageFile
	Deps      []models.PackageDep
	Hooks     []models.PackageHook
	Questions []models.PackageQuestion
	Branch    string
	Global    bool
	DryRun    bool
	Now       time.Time
	RepoRoot  string
}

// Summary is returned to the CLI.
type Summary struct {
	PackageID                  string         `json:"package_id"`
	Version                    string         `json:"version"`
	Branch                     string         `json:"branch"`
	Scope                      string         `json:"scope"`
	InstallRoot                string         `json:"install_root"`
	FilesWritten               int            `json:"files_written"`
	Dependencies               []string       `json:"dependencies"`
	DependencyWarnings         []string       `json:"dependency_warnings"`
	HooksRegistered            []HookEntry    `json:"hooks_registered"`
	TemplateValidationWarnings []string       `json:"template_validation_warnings"`
	Files                      []PlannedFile  `json:"files"`
	Answers                    map[string]any `json:"answers"`
}

// PlannedFile is one materialized file in the install summary.
type PlannedFile struct {
	Path       string `json:"path"`
	IsTemplate bool   `json:"is_template"`
	Preview    string `json:"preview,omitempty"`
}

// Service executes installs against the filesystem.
type Service struct{}

// Execute performs one install or dry-run operation.
func (Service) Execute(_ context.Context, req Request) (Summary, error) {
	if req.Package == nil {
		return Summary{}, fmt.Errorf("missing package")
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}

	profile, err := repo.DetectProfile(req.RepoRoot, req.Now)
	if err != nil {
		return Summary{}, fmt.Errorf("detecting repo profile: %w", err)
	}
	answers := questionnaire.CollectDefaults(req.Questions, profile)
	scope := "project"
	installBase := filepath.Join(req.RepoRoot, ".claude")
	if req.Global {
		scope = "global"
		home, err := os.UserHomeDir()
		if err != nil {
			return Summary{}, fmt.Errorf("getting home dir: %w", err)
		}
		installBase = filepath.Join(home, ".claude")
	}
	if req.Package.InstallScope == models.InstallScopeLocalOnly && req.Global {
		return Summary{}, fmt.Errorf("package %s cannot be installed globally", req.Package.ID)
	}

	planned, warnings, resolvedVars, err := renderFiles(req.Package.ID, installBase, req.Files, profile, answers.Values, req.Branch, req.RepoRoot)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		PackageID:                  req.Package.ID,
		Version:                    req.Package.Version,
		Branch:                     req.Branch,
		Scope:                      scope,
		InstallRoot:                packageInstallRoot(installBase, req.Package.ID),
		Dependencies:               dependencyNames(req.Deps),
		DependencyWarnings:         dependencyWarnings(req.Deps),
		HooksRegistered:            make([]HookEntry, 0, len(req.Hooks)),
		TemplateValidationWarnings: warnings,
		Files:                      make([]PlannedFile, 0, len(planned)),
		Answers:                    answers.Values,
	}
	for _, file := range planned {
		summary.Files = append(summary.Files, PlannedFile{
			Path:       file.DestPath,
			IsTemplate: file.Source.IsTemplate,
			Preview:    preview(file.Rendered),
		})
	}
	for _, hook := range req.Hooks {
		summary.HooksRegistered = append(summary.HooksRegistered, HookEntry{
			Event:    string(hook.Event),
			Matcher:  hook.Matcher,
			Skill:    req.Package.ID,
			Script:   hookAbsolutePath(installBase, req.Package.ID, hook.ScriptPath),
			Priority: hook.Priority,
			Blocking: hook.Blocking,
		})
	}

	if req.DryRun {
		return summary, nil
	}

	if err := ensureProjectState(req.RepoRoot); err != nil {
		return Summary{}, err
	}

	renderedFiles := make([]integrity.FileHash, 0, len(planned))
	for _, file := range planned {
		if err := writeFileAtomic(filepath.FromSlash(file.DestPath), []byte(file.Rendered), file.Mode); err != nil {
			return Summary{}, fmt.Errorf("writing %s: %w", file.DestPath, err)
		}
		actual := shaHex([]byte(file.Rendered))
		if actual != file.Source.SHA256 && !file.Source.IsTemplate {
			return Summary{}, fmt.Errorf("sha mismatch for %s", file.DestPath)
		}
		renderedFiles = append(renderedFiles, integrity.FileHash{DestPath: file.DestPath, SHA256: actual})
	}

	lock, err := LoadManifestLock(req.RepoRoot)
	if err != nil {
		return Summary{}, err
	}
	record := InstallRecord{
		InstallID:        fmt.Sprintf("pkg_%s_%s", req.Package.ID, scope),
		Package:          req.Package.ID,
		Version:          req.Package.Version,
		DoltCommit:       stringValue(req.Package.SHA256),
		Branch:           req.Branch,
		Variant:          req.Package.AgentVariant,
		InstalledAt:      req.Now.UTC().Format(time.RFC3339),
		InstallScope:     scope,
		InstallRoot:      summary.InstallRoot,
		InstallSite:      req.RepoRoot,
		TrackingOrigin:   "local-install",
		TemplateRendered: hasTemplates(req.Files),
		Files:            make(map[string]string, len(renderedFiles)),
		Answers:          answers.Values,
		QuestionSnapshot: QuestionSnapshot{QuestionIDs: questionIDs(req.Questions)},
		Requirements: RequirementSnapshot{
			Tools:         dependencyNames(req.Deps),
			ToolsVerified: map[string]string{},
			Agents:        []string{req.Package.AgentVariant},
			CLIInstalled:  []string{},
			CLIProvenance: map[string]string{},
		},
		RepoProfile: map[string]any{
			"name":             profile.Repo.Name,
			"primary_language": profile.Repo.PrimaryLanguage,
		},
		TemplateValidation: TemplateValidationRecord{
			ValidatedAt:       req.Now.UTC().Format(time.RFC3339),
			TemplateFiles:     templateFiles(req.Files),
			VariablesResolved: resolvedVars,
			Unresolved:        warnings,
			Warnings:          warnings,
		},
	}
	for _, hash := range renderedFiles {
		record.Files[hash.DestPath] = hash.SHA256
	}
	lock.UpsertInstall(record)
	if err := SaveManifestLock(req.RepoRoot, lock); err != nil {
		return Summary{}, err
	}

	registry, err := LoadHookRegistry(req.RepoRoot)
	if err != nil {
		return Summary{}, err
	}
	for _, hook := range summary.HooksRegistered {
		registry.Hooks = append(registry.Hooks, hook)
	}
	if err := SaveHookRegistry(req.RepoRoot, registry); err != nil {
		return Summary{}, err
	}

	summary.FilesWritten = len(planned)
	return summary, nil
}

type renderedFile struct {
	Source   models.PackageFile
	DestPath string
	Rendered string
	Mode     os.FileMode
}

func renderFiles(packageID, installBase string, files []models.PackageFile, profile repo.Profile, answers map[string]any, branch, repoRoot string) ([]renderedFile, []string, []string, error) {
	result := make([]renderedFile, 0, len(files))
	warnings := []string{}
	resolved := []string{}
	env := map[string]string{
		"synaptic_root":         filepath.Join(repoRoot, ".synaptic"),
		"synaptic_project_root": repoRoot,
		"synaptic_skills":       filepath.Join(installBase, "skills"),
		"synaptic_shared":       filepath.Join(installBase, "shared"),
		"sc_dolt_branch":        branch,
		"synaptic_agents":       "claude",
	}
	for _, file := range files {
		rendered := file.Content
		dest := installPathForFile(installBase, packageID, file)
		if file.IsTemplate {
			rendered, resolved = renderTemplate(file.Content, profile, answers, env, resolved)
			dest = strings.TrimSuffix(dest, ".j2")
			if unresolvedTemplatePattern.MatchString(rendered) {
				warnings = append(warnings, dest+": contains unresolved template markers")
			}
		}
		mode := os.FileMode(0o644)
		if file.FileType == models.FileTypeScript || file.FileType == models.FileTypeHook {
			mode = 0o755
		}
		result = append(result, renderedFile{
			Source:   file,
			DestPath: dest,
			Rendered: rendered,
			Mode:     mode,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DestPath < result[j].DestPath })
	return result, warnings, dedupeStrings(resolved), nil
}

func installPathForFile(installBase, packageID string, file models.PackageFile) string {
	switch file.FileType {
	case models.FileTypeSkill:
		return filepath.ToSlash(filepath.Join(installBase, "skills", packageID, file.DestPath))
	case models.FileTypeHook:
		return filepath.ToSlash(filepath.Join(installBase, "skills", packageID, file.DestPath))
	case models.FileTypeScript:
		return filepath.ToSlash(filepath.Join(installBase, "skills", packageID, file.DestPath))
	case models.FileTypeAgent:
		return filepath.ToSlash(filepath.Join(installBase, "agents", filepath.Base(file.DestPath)))
	case models.FileTypeCommand:
		return filepath.ToSlash(filepath.Join(installBase, "commands", filepath.Base(file.DestPath)))
	default:
		return filepath.ToSlash(filepath.Join(installBase, "skills", packageID, file.DestPath))
	}
}

func packageInstallRoot(installBase, packageID string) string {
	return filepath.ToSlash(filepath.Join(installBase, "skills", packageID))
}

func hookAbsolutePath(installBase, packageID, scriptPath string) string {
	return filepath.ToSlash(filepath.Join(installBase, "skills", packageID, scriptPath))
}

func renderTemplate(content string, profile repo.Profile, answers map[string]any, env map[string]string, resolved []string) (string, []string) {
	rendered := placeholderPattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := placeholderPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		ns, key := parts[1], parts[2]
		switch ns {
		case "repo":
			resolved = append(resolved, ns+"."+key)
			return repoValue(profile, key)
		case "answers":
			if value, ok := answers[key]; ok {
				resolved = append(resolved, ns+"."+key)
				return stringify(value)
			}
		case "env":
			if value, ok := env[key]; ok {
				resolved = append(resolved, ns+"."+key)
				return value
			}
		}
		return match
	})
	return rendered, resolved
}

func repoValue(profile repo.Profile, key string) string {
	switch key {
	case "name":
		return profile.Repo.Name
	case "root":
		return profile.Repo.Root
	case "primary_language":
		return profile.Repo.PrimaryLanguage
	case "ci_system":
		return profile.Repo.CISystem
	default:
		return ""
	}
}

func stringify(value any) string {
	switch v := value.(type) {
	case []string:
		return strings.Join(v, ", ")
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func dependencyNames(deps []models.PackageDep) []string {
	names := make([]string, 0, len(deps))
	for _, dep := range deps {
		names = append(names, dep.DepName+dep.DepSpec)
	}
	return names
}

func dependencyWarnings(deps []models.PackageDep) []string {
	warnings := make([]string, 0, len(deps))
	for _, dep := range deps {
		if dep.DepType == models.DepTypeCLI || dep.DepType == models.DepTypeTool {
			warnings = append(warnings, "missing dependency verification for "+dep.DepName)
		}
	}
	return warnings
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func shaHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ensureProjectState(root string) error {
	for _, rel := range []string{ManifestLockPath, RepoProfilePath, EnvPath, HooksRegistry} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			return fmt.Errorf("creating state dir: %w", err)
		}
	}
	return nil
}

func hasTemplates(files []models.PackageFile) bool {
	for _, file := range files {
		if file.IsTemplate {
			return true
		}
	}
	return false
}

func questionIDs(questions []models.PackageQuestion) []string {
	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.QuestionID)
	}
	sort.Strings(ids)
	return ids
}

func templateFiles(files []models.PackageFile) []string {
	paths := []string{}
	for _, file := range files {
		if file.IsTemplate {
			paths = append(paths, file.DestPath)
		}
	}
	sort.Strings(paths)
	return paths
}

func preview(rendered string) string {
	rendered = strings.TrimSpace(rendered)
	if len(rendered) > 120 {
		return rendered[:120]
	}
	return rendered
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
