package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	ManifestLockPath = ".synaptic/manifest.lock"
	RepoProfilePath  = ".synaptic/repo-profile.toml"
	EnvPath          = ".synaptic/env.toml"
	HooksRegistry    = ".synaptic/hooks/registry.toml"
)

// ManifestLock is the normative install tracking file.
type ManifestLock struct {
	Version  int             `toml:"version"`
	Installs []InstallRecord `toml:"installs"`
}

// InstallRecord captures one tracked install.
type InstallRecord struct {
	InstallID          string                   `toml:"install_id"`
	Package            string                   `toml:"package"`
	Version            string                   `toml:"version"`
	DoltCommit         string                   `toml:"dolt_commit"`
	Branch             string                   `toml:"branch"`
	Variant            string                   `toml:"variant"`
	InstalledAt        string                   `toml:"installed_at"`
	InstallScope       string                   `toml:"install_scope"`
	InstallRoot        string                   `toml:"install_root"`
	InstallSite        string                   `toml:"install_site"`
	TrackingOrigin     string                   `toml:"tracking_origin"`
	TemplateRendered   bool                     `toml:"template_rendered"`
	Files              map[string]string        `toml:"files"`
	Answers            map[string]any           `toml:"answers"`
	QuestionSnapshot   QuestionSnapshot         `toml:"question_snapshot"`
	Requirements       RequirementSnapshot      `toml:"requirements"`
	RepoProfile        map[string]any           `toml:"repo_profile_snapshot"`
	TemplateValidation TemplateValidationRecord `toml:"template_validation"`
}

type QuestionSnapshot struct {
	QuestionIDs []string `toml:"question_ids"`
}

type RequirementSnapshot struct {
	Tools         []string          `toml:"tools"`
	ToolsVerified map[string]string `toml:"tools_verified"`
	Agents        []string          `toml:"agents"`
	CLIInstalled  []string          `toml:"cli_installed"`
	CLIProvenance map[string]string `toml:"cli_provenance"`
}

type TemplateValidationRecord struct {
	ValidatedAt       string   `toml:"validated_at"`
	TemplateFiles     []string `toml:"template_files"`
	VariablesResolved []string `toml:"variables_resolved"`
	Unresolved        []string `toml:"unresolved"`
	Warnings          []string `toml:"warnings"`
}

type HookRegistry struct {
	Hooks []HookEntry `toml:"hook"`
}

type HookEntry struct {
	Event    string `toml:"event"`
	Matcher  string `toml:"matcher"`
	Skill    string `toml:"skill"`
	Script   string `toml:"script"`
	Priority int    `toml:"priority"`
	Blocking bool   `toml:"blocking"`
}

// LoadManifestLock reads manifest.lock or returns an empty lock if it does not exist.
func LoadManifestLock(root string) (ManifestLock, error) {
	path := filepath.Join(root, ManifestLockPath)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ManifestLock{Version: 1}, nil
	}
	if err != nil {
		return ManifestLock{}, fmt.Errorf("reading manifest lock: %w", err)
	}
	var lock ManifestLock
	if err := toml.Unmarshal(data, &lock); err != nil {
		return ManifestLock{}, fmt.Errorf("decoding manifest lock: %w", err)
	}
	if lock.Version == 0 {
		lock.Version = 1
	}
	return lock, nil
}

// SaveManifestLock writes manifest.lock atomically.
func SaveManifestLock(root string, lock ManifestLock) error {
	path := filepath.Join(root, ManifestLockPath)
	lock.Version = 1
	return writeTOMLAtomic(path, lock)
}

// UpsertInstall replaces an install record with the same install id or appends it.
func (m *ManifestLock) UpsertInstall(record InstallRecord) {
	for i := range m.Installs {
		if m.Installs[i].InstallID == record.InstallID {
			m.Installs[i] = record
			return
		}
	}
	m.Installs = append(m.Installs, record)
}

// LoadHookRegistry loads the project hook registry or returns an empty registry.
func LoadHookRegistry(root string) (HookRegistry, error) {
	path := filepath.Join(root, HooksRegistry)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return HookRegistry{}, nil
	}
	if err != nil {
		return HookRegistry{}, fmt.Errorf("reading hook registry: %w", err)
	}
	var registry HookRegistry
	if err := toml.Unmarshal(data, &registry); err != nil {
		return HookRegistry{}, fmt.Errorf("decoding hook registry: %w", err)
	}
	return registry, nil
}

// SaveHookRegistry writes registry.toml atomically.
func SaveHookRegistry(root string, registry HookRegistry) error {
	return writeTOMLAtomic(filepath.Join(root, HooksRegistry), registry)
}

// SaveRepoProfile writes the repo profile TOML atomically.
func SaveRepoProfile(root string, profile any) error {
	return writeTOMLAtomic(filepath.Join(root, RepoProfilePath), profile)
}

// SaveEnv writes env.toml atomically.
func SaveEnv(root string, env any) error {
	return writeTOMLAtomic(filepath.Join(root, EnvPath), env)
}

func writeTOMLAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating parent dir: %w", err)
	}
	data, err := toml.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding toml: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic replace %s: %w", path, err)
	}
	return nil
}
