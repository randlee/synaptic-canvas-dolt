package admin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewImportCmdRequiresBranch(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tempDir, ".dolt"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := NewImportCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	cmd.SetArgs([]string{"--dolt-dir", tempDir, "."})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--branch is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatImportAck(t *testing.T) {
	t.Parallel()
	if got := FormatImportAck("pkg", "develop"); got != "importing pkg into develop" {
		t.Fatalf("FormatImportAck() = %q", got)
	}
}

var _ = cobra.ShellCompDirectiveDefault
