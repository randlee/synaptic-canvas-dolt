package admin

import "testing"

func TestNewDiffCmdRequiresBranches(t *testing.T) {
	t.Parallel()

	cmd := NewDiffCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	cmd.SetArgs([]string{"pkg"})
	err := cmd.Execute()
	if err == nil || err.Error() != "--branch1 and --branch2 are required" {
		t.Fatalf("unexpected error: %v", err)
	}
}
