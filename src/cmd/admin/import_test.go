package admin

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/importer"
	"github.com/spf13/cobra"
)

func TestNewImportCmdUsesEffectiveBranch(t *testing.T) {
	if _, err := exec.LookPath("dolt"); err != nil {
		t.Skip("dolt binary not found in PATH")
	}

	tempDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tempDir, ".dolt"), 0o755); err != nil { //nolint:gosec // G301: test directory permissions are intentional.
		t.Fatal(err)
	}
	logPath := filepath.Join(tempDir, "calls.log")
	sqlPath := filepath.Join(tempDir, "sql.log")
	scriptPath := filepath.Join(tempDir, "dolt")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + logPath + "\"\n" +
		"case \"$1\" in\n" +
		"  branch)\n" +
		"    if [ \"$2\" = \"--list\" ]; then echo \"$3\"; exit 0; fi\n" +
		"    ;;\n" +
		"  --branch)\n" +
		"    if [ \"$3\" = \"sql\" ] && [ \"$4\" = \"-q\" ]; then echo '{\"rows\":[]}'; exit 0; fi\n" +
		"    if [ \"$3\" = \"sql\" ]; then cat > \"" + sqlPath + "\"; exit 0; fi\n" +
		"    ;;\n" +
		"esac\n" +
		"exit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil { //nolint:gosec // G306: test helper script permissions are intentional.
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)
	t.Setenv("SC_DOLT_BRANCH", "develop")

	cmd := NewImportCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("dolt-client", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	cmd.SetArgs([]string{"--dolt-dir", tempDir, "--dolt-client", "cli", filepath.Join("..", "..", "pkg", "importer", "testdata", "basic-package")})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logData, err := os.ReadFile(logPath) //nolint:gosec // G304: test log path is controlled by the test harness.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), "--branch develop sql") {
		t.Fatalf("expected effective branch from env in call log, got:\n%s", string(logData))
	}
}

func TestFormatImportAck(t *testing.T) {
	t.Parallel()
	if got := FormatImportAck("pkg", "develop"); got != "importing pkg into develop" {
		t.Fatalf("FormatImportAck() = %q", got)
	}
}

func TestWriteImportJSONErrorShape(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	formatter := output.NewFormatter(true, false)
	formatter.Writer = &out
	handled, err := writeImportJSONError(formatter, &importer.SHACollisionError{
		File:        "skills/sample-skill/SKILL.md.j2",
		Package:     "sample-skill",
		Version:     "1.2.3",
		Branch:      "develop",
		ExistingSHA: "existing",
		IncomingSHA: "incoming",
	})
	if err != nil {
		t.Fatalf("writeImportJSONError() error = %v", err)
	}
	if !handled {
		t.Fatal("collision error was not handled")
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, out.String())
	}
	want := map[string]string{
		"error":        "sha_collision",
		"file":         "skills/sample-skill/SKILL.md.j2",
		"package":      "sample-skill",
		"version":      "1.2.3",
		"branch":       "develop",
		"existing_sha": "existing",
		"incoming_sha": "incoming",
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("json field %s = %q, want %q; full=%+v", key, got[key], value, got)
		}
	}
}

var _ = cobra.ShellCompDirectiveDefault
