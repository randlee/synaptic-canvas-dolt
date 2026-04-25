package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestInitCommandJSON(t *testing.T) {
	root := t.TempDir()
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp initResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}
	for _, rel := range []string{".synaptic/manifest.lock", ".synaptic/repo-profile.toml", ".synaptic/env.toml", ".synaptic/hooks/registry.toml"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("%s missing: %v", rel, err)
		}
	}
}

func TestInitCommandErrorJSON(t *testing.T) {
	root := t.TempDir()
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	prev := initializeRepoFunc
	initializeRepoFunc = func(string) (initResponse, error) {
		return initResponse{}, errors.New("boom")
	}
	defer func() { initializeRepoFunc = prev }()

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != "query_failed" || resp.Error.Message != "boom" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestInstallCommandDryRunJSON(t *testing.T) {
	root := t.TempDir()
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath:   "SKILL.md.j2",
		Content:    "Hello {{ repo.name }}",
		SHA256:     "ignored",
		FileType:   models.FileTypeSkill,
		IsTemplate: true,
	}})

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--dry-run", "--json"})

	restore := installReadTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".synaptic", "manifest.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run should not create manifest.lock, got err=%v", err)
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("unexpected json output: %s", out.String())
	}
}

func TestInstallCommandWritesFiles(t *testing.T) {
	root := t.TempDir()
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()
	resolvedRoot := mustEvalSymlinks(t, root)

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)
	static := "# title\n"
	mock.AddFiles("team-lead", []models.PackageFile{{
		DestPath: "SKILL.md", Content: static, SHA256: testSHA(static), FileType: models.FileTypeSkill,
	}})

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetArgs([]string{"install", "team-lead"})

	restore := installReadTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(resolvedRoot, ".synaptic", "manifest.lock")); err != nil {
		t.Fatalf("manifest.lock missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resolvedRoot, ".claude", "skills", "team-lead", "SKILL.md")); err != nil {
		t.Fatalf("installed file missing: %v", err)
	}
}

func TestInstallCommandErrorJSON(t *testing.T) {
	root := t.TempDir()
	writeCmdFile(t, filepath.Join(root, "go.mod"), "module test\n")
	restoreDir := chdirForTest(t, root)
	defer restoreDir()

	mock := dolt.NewMockClient()
	mock.GetErr = errors.New("boom")

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"install", "team-lead", "--json"})

	restore := installReadTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != "query_failed" || resp.Error.Message != "boom" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func installReadTestHooks(mock *dolt.MockClient) func() {
	prevOpener := readClientOpener
	prevDetect := detectReadDoltDir
	readClientOpener = func(_ string, _ string) (readClient, error) { return mock, nil }
	detectReadDoltDir = func(string) (string, error) { return "", nil }
	return func() {
		readClientOpener = prevOpener
		detectReadDoltDir = prevDetect
	}
}

func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	return func() { _ = os.Chdir(prev) }
}

func writeCmdFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func testSHA(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", path, err)
	}
	return resolved
}
