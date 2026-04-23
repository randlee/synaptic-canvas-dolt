package admin

import "testing"

func TestPublishRequiresBranches(t *testing.T) {
	t.Parallel()

	cmd := NewPublishCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	cmd.SetArgs([]string{"pkg"})
	err := cmd.Execute()
	if err == nil || err.Error() != "--from and --to are required" {
		t.Fatalf("unexpected error: %v", err)
	}
}
