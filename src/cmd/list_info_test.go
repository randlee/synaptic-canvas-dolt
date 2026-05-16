package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestListCommandJSON(t *testing.T) {
	mock := dolt.NewMockClient()
	desc := "Team lead workflow"
	sha := "abc123"
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"Go", "workflow"})
	pkg.Description = &desc
	pkg.AgentVariant = "claude"
	pkg.SHA256 = &sha
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "a"}, {DestPath: "b"}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepName: "gh"}, {DepName: "atm"}})

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list", "--branch", "beta", "--tags", "go", "--json"})

	restore := installReadClientTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp listResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.OK || resp.Branch != "beta" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(resp.Packages))
	}
	if resp.Packages[0].FileCount != 2 || resp.Packages[0].DependencyCount != 2 {
		t.Fatalf("unexpected counts: %+v", resp.Packages[0])
	}
}

func TestListCommandTable(t *testing.T) {
	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"go", "workflow"})
	pkg.AgentVariant = "claude"
	mock.AddPackage(pkg)

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list", "--branch", "main"})

	restore := installReadClientTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := out.String()
	for _, want := range []string{"NAME", "VERSION", "BRANCH", "team-lead", "main"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestInfoCommandJSON(t *testing.T) {
	mock := dolt.NewMockClient()
	desc := "Team lead workflow"
	sha := "abc123"
	pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.0", []string{"go", "workflow"})
	pkg.Description = &desc
	pkg.AgentVariant = "claude"
	pkg.SHA256 = &sha
	mock.AddPackage(pkg)
	mock.AddFiles("team-lead", []models.PackageFile{{DestPath: "a"}, {DestPath: "b"}, {DestPath: "c"}})
	mock.AddDeps("team-lead", []models.PackageDep{{DepType: models.DepTypeCLI, DepName: "gh", DepSpec: ">=2.0"}})
	mock.AddHooks("team-lead", []models.PackageHook{{ScriptPath: "hook.sh"}})
	mock.AddQuestions("team-lead", []models.PackageQuestion{{QuestionID: "style"}})

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"info", "team-lead", "--json"})

	restore := installReadClientTestHooks(mock)
	defer restore()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp infoResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if !resp.OK || resp.Package.Name != "team-lead" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Package.FileCount != 3 || resp.Package.DependencyCount != 1 {
		t.Fatalf("unexpected counts: %+v", resp.Package)
	}
	if resp.Package.HookCount != 1 || resp.Package.QuestionCount != 1 {
		t.Fatalf("unexpected hook/question counts: %+v", resp.Package)
	}
}

func TestInfoCommandNotFound(t *testing.T) {
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"info", "missing"})

	restore := installReadClientTestHooks(dolt.NewMockClient())
	defer restore()

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `package "missing" not found`) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestInfoCommandNotFoundJSON(t *testing.T) {
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"info", "missing", "--json"})

	restore := installReadClientTestHooks(dolt.NewMockClient())
	defer restore()

	requireJSONCmdError(t, cmd.Execute())

	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != "not_found" || !strings.Contains(resp.Error.Message, `package "missing" not found`) {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestListCommandPropagatesClientError(t *testing.T) {
	mock := dolt.NewMockClient()
	mock.ListErr = errors.New("boom")

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetArgs([]string{"list"})

	restore := installReadClientTestHooks(mock)
	defer restore()

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected list error, got %v", err)
	}
}

func TestListCommandErrorJSON(t *testing.T) {
	mock := dolt.NewMockClient()
	mock.ListErr = errors.New("boom")

	cmd := NewRootCmd("test", "abc", "2025-01-01")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list", "--json"})

	restore := installReadClientTestHooks(mock)
	defer restore()

	requireJSONCmdError(t, cmd.Execute())

	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != "query_failed" || resp.Error.Message != "boom" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func installReadClientTestHooks(mock *dolt.MockClient) func() {
	prevOpener := readClientOpener
	readClientOpener = func(_ *config.Config) (readClient, error) {
		return mock, nil
	}
	return func() {
		readClientOpener = prevOpener
	}
}
