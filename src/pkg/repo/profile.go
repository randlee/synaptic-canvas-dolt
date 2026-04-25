package repo

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Profile captures lightweight repository facts used for install rendering.
type Profile struct {
	Repo RepoSection `toml:"repo" json:"repo"`
}

// RepoSection stores detected repository attributes.
type RepoSection struct {
	Name            string   `toml:"name" json:"name"`
	Root            string   `toml:"root" json:"root"`
	DetectedAt      string   `toml:"detected_at" json:"detected_at"`
	Languages       []string `toml:"languages" json:"languages"`
	PrimaryLanguage string   `toml:"primary_language" json:"primary_language"`
	Frameworks      []string `toml:"frameworks" json:"frameworks"`
	TestFrameworks  []string `toml:"test_frameworks" json:"test_frameworks"`
	CISystem        string   `toml:"ci_system" json:"ci_system"`
	Monorepo        bool     `toml:"monorepo" json:"monorepo"`
	GitConventions  string   `toml:"git_conventions" json:"git_conventions"`
}

// DetectProfile builds a minimal deterministic repo profile from the filesystem.
func DetectProfile(root string, now time.Time) (Profile, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Profile{}, err
	}

	profile := Profile{
		Repo: RepoSection{
			Name:           filepath.Base(absRoot),
			Root:           absRoot,
			DetectedAt:     now.UTC().Format(time.RFC3339),
			GitConventions: "freeform",
		},
	}

	if fileExists(filepath.Join(absRoot, "go.mod")) {
		profile.Repo.Languages = append(profile.Repo.Languages, "go")
		profile.Repo.TestFrameworks = append(profile.Repo.TestFrameworks, "go-test")
	}
	if fileExists(filepath.Join(absRoot, "package.json")) {
		profile.Repo.Languages = append(profile.Repo.Languages, "javascript")
	}
	if fileExists(filepath.Join(absRoot, "pyproject.toml")) || fileExists(filepath.Join(absRoot, "requirements.txt")) {
		profile.Repo.Languages = append(profile.Repo.Languages, "python")
	}
	if fileExists(filepath.Join(absRoot, "Cargo.toml")) {
		profile.Repo.Languages = append(profile.Repo.Languages, "rust")
	}
	if fileExists(filepath.Join(absRoot, ".github", "workflows")) {
		profile.Repo.CISystem = "github-actions"
	}
	if fileExists(filepath.Join(absRoot, "pytest.ini")) {
		profile.Repo.TestFrameworks = append(profile.Repo.TestFrameworks, "pytest")
	}
	if fileExists(filepath.Join(absRoot, "jest.config.js")) || fileExists(filepath.Join(absRoot, "jest.config.ts")) {
		profile.Repo.TestFrameworks = append(profile.Repo.TestFrameworks, "jest")
	}

	if len(profile.Repo.Languages) > 0 {
		profile.Repo.PrimaryLanguage = profile.Repo.Languages[0]
	}
	profile.Repo.Frameworks = detectFrameworks(absRoot)
	profile.Repo.Monorepo = fileExists(filepath.Join(absRoot, "pnpm-workspace.yaml")) || fileExists(filepath.Join(absRoot, "lerna.json"))
	profile.Repo.Languages = dedupe(profile.Repo.Languages)
	profile.Repo.Frameworks = dedupe(profile.Repo.Frameworks)
	profile.Repo.TestFrameworks = dedupe(profile.Repo.TestFrameworks)

	return profile, nil
}

func detectFrameworks(root string) []string {
	frameworks := []string{}
	if fileContains(filepath.Join(root, "package.json"), "react") {
		frameworks = append(frameworks, "react")
	}
	if fileContains(filepath.Join(root, "package.json"), "next") {
		frameworks = append(frameworks, "next")
	}
	if fileContains(filepath.Join(root, "pyproject.toml"), "fastapi") || fileContains(filepath.Join(root, "requirements.txt"), "fastapi") {
		frameworks = append(frameworks, "fastapi")
	}
	return frameworks
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileContains(path, needle string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(needle))
}

func dedupe(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || slices.Contains(result, value) {
			continue
		}
		result = append(result, value)
	}
	return result
}
