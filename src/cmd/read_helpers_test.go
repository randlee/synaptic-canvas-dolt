package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
)

func TestOpenReadClientConfigSelection(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		setup      func(*testing.T)
		wantErr    string
		wantClient bool
	}{
		{
			name:    "default http requires database",
			env:     map[string]string{},
			wantErr: "dolt.database is not configured",
		},
		{
			name: "http from env",
			env: map[string]string{
				"SC_DOLT_DATABASE": "owner/repo",
			},
			wantClient: true,
		},
		{
			name: "sql requires dsn",
			env: map[string]string{
				"SC_DOLT_CLIENT": "sql",
			},
			wantErr: "dolt.dsn is not configured",
		},
		{
			name: "cli requires dir",
			env: map[string]string{
				"SC_DOLT_CLIENT": "cli",
			},
			setup: func(t *testing.T) {
				t.Chdir(t.TempDir())
			},
			wantErr: "could not auto-detect Dolt database directory",
		},
		{
			name: "cli auto detects repo dir",
			env: map[string]string{
				"SC_DOLT_CLIENT": "cli",
			},
			setup: func(t *testing.T) {
				repoDir := t.TempDir()
				if err := os.Mkdir(filepath.Join(repoDir, ".dolt"), 0o750); err != nil {
					t.Fatalf("Mkdir(.dolt) error = %v", err)
				}
				child := filepath.Join(repoDir, "nested")
				if err := os.MkdirAll(child, 0o750); err != nil {
					t.Fatalf("MkdirAll(nested) error = %v", err)
				}
				t.Chdir(child)
			},
			wantClient: true,
		},
		{
			name: "unsupported client",
			env: map[string]string{
				"SC_DOLT_CLIENT": "ftp",
			},
			wantErr: `unsupported dolt.client "ftp"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearReadClientEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			if tt.setup != nil {
				tt.setup(t)
			}

			client, err := openReadClient(&config.Config{})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("openReadClient() error = %v", err)
			}
			defer func() { _ = client.Close() }()
			if tt.wantClient {
				switch tt.env["SC_DOLT_CLIENT"] {
				case "cli":
					if _, ok := client.(*dolt.CLIReader); !ok {
						t.Fatalf("client type = %T, want *dolt.CLIReader", client)
					}
				default:
					if _, ok := client.(*dolt.HTTPClient); !ok {
						t.Fatalf("client type = %T, want *dolt.HTTPClient", client)
					}
				}
			}
		})
	}
}

func clearReadClientEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"SC_DOLT_CLIENT",
		"SC_DOLT_HOST",
		"SC_DOLT_DATABASE",
		"SC_DOLT_TOKEN",
		"SC_DOLT_DSN",
		"SC_DOLT_DIR",
		"SC_DOLT_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}
