package admin

import "github.com/spf13/cobra"

// NewAdminCmd creates the admin parent command.
func NewAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin commands for package authors and maintainers",
	}

	cmd.AddCommand(NewDiffCmd())
	cmd.AddCommand(NewExportCmd())
	cmd.AddCommand(NewImportCmd())
	cmd.AddCommand(NewVerifyCmd())
	return cmd
}
