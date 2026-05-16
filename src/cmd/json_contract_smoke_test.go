package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
)

func TestJSONVER011ListShapeAcrossBackends(t *testing.T) {
	prevOpener := readClientOpener
	readClientOpener = func(cfg *config.Config) (readClient, error) {
		mock := dolt.NewMockClient()
		pkg := dolt.NewTestPackage("team-lead", "team-lead", "1.2.3", []string{"go"})
		pkg.AgentVariant = "claude"
		mock.AddPackage(pkg)
		if got := cfg.Get(config.KeyDoltClient, "http"); got == "" {
			t.Fatal("expected dolt client to be selected")
		}
		return mock, nil
	}
	defer func() { readClientOpener = prevOpener }()

	tests := []struct {
		name   string
		client string
	}{
		{name: "http", client: "http"},
		{name: "sql", client: "sql"},
		{name: "cli", client: "cli"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			setTestHome(t, home)
			t.Setenv("SC_DOLT_CLIENT", tt.client)

			var out bytes.Buffer
			cmd := NewRootCmd("test", "abc", "2025-01-01")
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"list", "--json"})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			var payload struct {
				OK       bool              `json:"ok"`
				Branch   string            `json:"branch"`
				Filters  map[string]any    `json:"filters"`
				Packages []json.RawMessage `json:"packages"`
			}
			if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
				t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
			}
			if !payload.OK || payload.Branch != "main" || len(payload.Packages) != 1 {
				t.Fatalf("unexpected payload for %s: %+v", tt.client, payload)
			}
		})
	}
}

func TestJSONBR004ListUsesConfiguredBranchWhenNoFlagProvided(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("SC_DOLT_BRANCH", "beta")

	prevOpener := readClientOpener
	readClientOpener = func(cfg *config.Config) (readClient, error) {
		if got := cfg.EffectiveBranch(); got != "beta" {
			t.Fatalf("EffectiveBranch() = %q, want beta", got)
		}
		mock := dolt.NewMockClient()
		mock.AddPackage(dolt.NewTestPackage("team-lead", "team-lead", "1.2.3", nil))
		return mock, nil
	}
	defer func() { readClientOpener = prevOpener }()

	var out bytes.Buffer
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp listResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.Branch != "beta" {
		t.Fatalf("Branch = %q, want beta", resp.Branch)
	}
}

func TestJSONBR006InfoUsesExplicitBranchSelection(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("SC_DOLT_BRANCH", "main")

	prevOpener := readClientOpener
	readClientOpener = func(cfg *config.Config) (readClient, error) {
		if got := cfg.EffectiveBranch(); got != "advanced" {
			t.Fatalf("EffectiveBranch() = %q, want advanced", got)
		}
		mock := dolt.NewMockClient()
		mock.AddPackage(dolt.NewTestPackage("team-lead", "team-lead", "2.0.0", nil))
		return mock, nil
	}
	defer func() { readClientOpener = prevOpener }()

	var out bytes.Buffer
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"info", "team-lead", "--branch", "advanced", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp infoResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.Branch != "advanced" {
		t.Fatalf("Branch = %q, want advanced", resp.Branch)
	}
}
