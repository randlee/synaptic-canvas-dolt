package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
)

type trackedInstall struct {
	Record installer.InstallRecord
	Source string
}

type validatedInstall struct {
	Package           string          `json:"package"`
	Version           string          `json:"version"`
	Branch            string          `json:"branch"`
	Scope             string          `json:"scope"`
	InstallRoot       string          `json:"install_root"`
	InstallSite       string          `json:"install_site"`
	Files             []validatedFile `json:"files"`
	AggregateExpected string          `json:"aggregate_expected"`
	AggregateActual   string          `json:"aggregate_actual,omitempty"`
	AggregatePass     bool            `json:"aggregate_pass"`
	Pass              bool            `json:"pass"`
	Status            string          `json:"status"`
}

type validatedFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

var snapshotNow = func() time.Time { return time.Now().UTC() }
var snapshotGitRemoteURL = gitRemoteURL

func loadTrackedInstalls(repoRoot string) ([]trackedInstall, error) {
	localLock, err := installer.LoadManifestLock(repoRoot)
	if err != nil {
		return nil, err
	}
	installs := make([]trackedInstall, 0, len(localLock.Installs)+4)
	for _, record := range localLock.Installs {
		installs = append(installs, trackedInstall{Record: record, Source: "project"})
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	globalLock, err := installer.LoadManifestLock(home)
	if err != nil {
		return nil, err
	}
	for _, record := range globalLock.Installs {
		installs = append(installs, trackedInstall{Record: record, Source: "global"})
	}

	sort.Slice(installs, func(i, j int) bool {
		if installs[i].Record.Package != installs[j].Record.Package {
			return installs[i].Record.Package < installs[j].Record.Package
		}
		return installs[i].Record.InstallScope < installs[j].Record.InstallScope
	})
	return installs, nil
}

func filterInstalls(installs []trackedInstall, packageID string) []trackedInstall {
	if packageID == "" {
		return installs
	}
	filtered := make([]trackedInstall, 0, len(installs))
	for _, install := range installs {
		if install.Record.Package == packageID {
			filtered = append(filtered, install)
		}
	}
	return filtered
}

func filterInstallsByScope(installs []trackedInstall, scope string) []trackedInstall {
	if scope == "" || scope == "both" {
		return installs
	}
	filtered := make([]trackedInstall, 0, len(installs))
	for _, install := range installs {
		if install.Record.InstallScope == scope {
			filtered = append(filtered, install)
		}
	}
	return filtered
}

func validateTrackedInstall(record installer.InstallRecord) (validatedInstall, error) {
	expected := make([]integrity.FileHash, 0, len(record.Files))
	for path, sha := range record.Files {
		expected = append(expected, integrity.FileHash{DestPath: path, SHA256: sha})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].DestPath < expected[j].DestPath })

	results, err := integrity.VerifyPackage(expected, record.InstallRoot)
	if err != nil {
		return validatedInstall{}, err
	}

	summary := validatedInstall{
		Package:           record.Package,
		Version:           record.Version,
		Branch:            record.Branch,
		Scope:             record.InstallScope,
		InstallRoot:       record.InstallRoot,
		InstallSite:       record.InstallSite,
		Files:             make([]validatedFile, 0, len(results)),
		AggregateExpected: integrity.ComputeAggregateSHA256(expected),
		Pass:              true,
		Status:            "PASS",
	}

	actual := make([]integrity.FileHash, 0, len(expected))
	canAggregate := true
	for _, result := range results {
		item := validatedFile{
			Path:   result.Path,
			Status: result.Status.String(),
		}
		if result.Err != nil {
			item.Error = result.Err.Error()
		}
		summary.Files = append(summary.Files, item)
		if result.Status != integrity.StatusOK {
			summary.Pass = false
			summary.Status = "FAIL"
		}
		if _, tracked := record.Files[result.Path]; tracked {
			sha, err := integrity.ComputeFileSHA256(filepath.Join(record.InstallRoot, filepath.FromSlash(result.Path)))
			if err != nil {
				canAggregate = false
				continue
			}
			actual = append(actual, integrity.FileHash{DestPath: result.Path, SHA256: sha})
		}
	}

	if canAggregate && len(actual) == len(expected) {
		summary.AggregateActual = integrity.ComputeAggregateSHA256(actual)
		summary.AggregatePass = summary.AggregateActual == summary.AggregateExpected
	} else {
		summary.AggregatePass = false
	}
	if !summary.AggregatePass {
		summary.Pass = false
		summary.Status = "FAIL"
	}

	return summary, nil
}

func scopeDisplay(branch, version string) string {
	if version == "" {
		return ""
	}
	if branch == "" || branch == "main" {
		return version
	}
	return version + " " + branch
}

func sanitizePathComponent(value string) string {
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	cleaned := strings.Trim(b.String(), "-")
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}

func repoKey(path string) string {
	base := sanitizePathComponent(filepath.Base(path))
	sum := sha256.Sum256([]byte(path))
	return base + "-" + hex.EncodeToString(sum[:4])
}

func gitRemoteURL(path string) string {
	if path == "" {
		return ""
	}
	cmd := exec.Command("git", "-C", path, "remote", "get-url", "origin") //nolint:gosec // git command and args are fixed.
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
