package admin

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewImportCmdRequiresBranch(t *testing.T) {
	t.Parallel()

	cmd := NewImportCmd()
	cmd.SetArgs([]string{"."})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatImportAck(t *testing.T) {
	t.Parallel()
	if got := FormatImportAck("pkg", "develop"); got != "importing pkg into develop" {
		t.Fatalf("FormatImportAck() = %q", got)
	}
}

var _ = cobra.ShellCompDirectiveDefault
