package admin

import (
	"os/exec"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	cmd.SetArgs([]string{"--dolt-dir", tempDir, filepath.Join("..", "..", "pkg", "importer", "testdata", "basic-package")})
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

var _ = cobra.ShellCompDirectiveDefault
