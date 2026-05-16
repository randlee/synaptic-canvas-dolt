package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigSetAndGetJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("USERPROFILE", home)

	var out bytes.Buffer
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "set", "dolt.database", "randlee/synaptic-canvas", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set Execute() error = %v", err)
	}

	var setResp configSetResponse
	if err := json.Unmarshal(out.Bytes(), &setResp); err != nil {
		t.Fatalf("json.Unmarshal(set) error = %v\noutput=%s", err, out.String())
	}
	wantPath := filepath.Join(home, ".sc", "config.toml")
	if !setResp.OK || setResp.Key != "dolt.database" || setResp.Path != wantPath {
		t.Fatalf("unexpected set response: %+v", setResp)
	}
	data, err := os.ReadFile(wantPath) //nolint:gosec // G304: test reads from t.TempDir()-based path, not user input.
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "randlee/synaptic-canvas") {
		t.Fatalf("config file missing database value:\n%s", string(data))
	}

	out.Reset()
	cmd = NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "get", "dolt.database", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get Execute() error = %v", err)
	}

	var getResp configGetResponse
	if err := json.Unmarshal(out.Bytes(), &getResp); err != nil {
		t.Fatalf("json.Unmarshal(get) error = %v\noutput=%s", err, out.String())
	}
	if !getResp.OK || getResp.Key != "dolt.database" || getResp.Value != "randlee/synaptic-canvas" {
		t.Fatalf("unexpected get response: %+v", getResp)
	}
}

func TestConfigUnknownKeyJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("USERPROFILE", home)

	var out bytes.Buffer
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "get", "dolt.nope", "--json"})
	requireJSONCmdError(t, cmd.Execute())

	var resp jsonErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.OK || resp.Error.Code != "invalid_args" || !strings.Contains(resp.Error.Message, "valid keys") {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestConfigGetEnvPrecedenceJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("SC_DOLT_DATABASE", "env/repo")

	var out bytes.Buffer
	cmd := NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "set", "dolt.database", "file/repo", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set Execute() error = %v", err)
	}

	out.Reset()
	cmd = NewRootCmd("test", "abc", "2025-01-01")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "get", "dolt.database", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get Execute() error = %v", err)
	}

	var resp configGetResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	if resp.Value != "env/repo" {
		t.Fatalf("Value = %q, want env/repo", resp.Value)
	}
}
