package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
)

func TestClientSelectionPrecedenceMatrix(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		useFlagDir bool
		envClient  string
		fileClient string
		wantClient string
		wantSource config.ValueSource
		wantErr    string
	}{
		{
			name:       "explicit client flag wins",
			args:       []string{"--client", "http"},
			envClient:  "cli",
			wantClient: "http",
			wantSource: config.ValueSourceExplicit,
		},
		{
			name:       "env beats file",
			envClient:  "cli",
			fileClient: "sql",
			wantClient: "cli",
			wantSource: config.ValueSourceEnvironment,
		},
		{
			name:       "file beats default",
			fileClient: "sql",
			wantClient: "sql",
			wantSource: config.ValueSourceFile,
		},
		{
			name:       "dolt dir flag implies cli",
			useFlagDir: true,
			wantClient: "cli",
			wantSource: config.ValueSourceCompatibility,
		},
		{
			name:       "dolt dir conflicts with explicit non cli",
			useFlagDir: true,
			envClient:  "http",
			wantErr:    "--dolt-dir may only be used with client=cli",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("SC_DOLT_CLIENT", tt.envClient)
			repoDir := filepath.Join(home, "repo")
			if err := os.MkdirAll(filepath.Join(repoDir, ".dolt"), 0o750); err != nil {
				t.Fatalf("MkdirAll(.dolt) error = %v", err)
			}
			if tt.fileClient != "" {
				if _, err := config.SetFileValue(config.KeyDoltClient, tt.fileClient); err != nil {
					t.Fatalf("SetFileValue(client) error = %v", err)
				}
			}
			if _, err := config.SetFileValue(config.KeyDoltDir, repoDir); err != nil {
				t.Fatalf("SetFileValue(dir) error = %v", err)
			}

			root := NewRootCmd("test", "abc", "2025-01-01")
			args := append([]string{}, tt.args...)
			if tt.useFlagDir {
				args = append(args, "--dolt-dir", repoDir)
			}
			root.SetArgs(args)
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			if err := root.Execute(); err != nil {
				t.Fatalf("root.Execute() error = %v", err)
			}

			cfg, err := config.NewConfigFromFlags(root)
			if err != nil {
				t.Fatalf("NewConfigFromFlags() error = %v", err)
			}
			if err := cfg.LoadFileConfig(); err != nil {
				t.Fatalf("LoadFileConfig() error = %v", err)
			}

			selection, err := cfg.ResolveDoltClient()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveDoltClient() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveDoltClient() error = %v", err)
			}
			if selection.Client != tt.wantClient || selection.ClientSource != tt.wantSource {
				t.Fatalf("selection = %+v, want client=%s source=%s", selection, tt.wantClient, tt.wantSource)
			}
		})
	}
}

func TestConformanceJSONErrorCodesAcrossBackends(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "http unavailable", err: fmt.Errorf("http failure: %w", dolt.ErrServerError), want: "backend_unavailable"},
		{name: "sql unavailable", err: fmt.Errorf("sql failure: %w", dolt.ErrServerError), want: "backend_unavailable"},
		{name: "cli unavailable", err: fmt.Errorf("cli failure: %w", dolt.ErrServerError), want: "backend_unavailable"},
		{name: "http auth", err: fmt.Errorf("http failure: %w", dolt.ErrUnauthorized), want: "backend_auth_failed"},
		{name: "sql auth", err: fmt.Errorf("sql failure: %w", dolt.ErrUnauthorized), want: "backend_auth_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := readClientOpener
			readClientOpener = func(*config.Config) (readClient, error) { return nil, tt.err }
			defer func() { readClientOpener = prev }()

			root := NewRootCmd("test", "abc", "2025-01-01")
			root.SetArgs([]string{"list", "--json"})
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)

			requireJSONCmdError(t, executeCommand(root))

			var envelope jsonErrorEnvelope
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
			}
			if envelope.OK || envelope.Error.Code != tt.want {
				t.Fatalf("unexpected envelope: %+v", envelope)
			}
		})
	}
}

func TestWritePathBackendRejectsHTTPJSON(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "admin import", args: []string{"--client", "http", "admin", "import", t.TempDir(), "--json"}},
		{name: "admin publish", args: []string{"--client", "http", "admin", "publish", "team-lead", "--from", "develop", "--to", "beta", "--json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCmd("test", "abc", "2025-01-01")
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(tt.args)

			requireJSONCmdError(t, executeCommand(root))

			var envelope jsonErrorEnvelope
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
			}
			if envelope.OK || envelope.Error.Code != "unsupported_backend" {
				t.Fatalf("unexpected envelope: %+v", envelope)
			}
		})
	}
}
