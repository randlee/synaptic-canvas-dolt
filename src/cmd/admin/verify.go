package admin

import (
	"context"
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/verifier"
	"github.com/spf13/cobra"
)

// NewVerifyCmd creates the sc admin verify command.
func NewVerifyCmd() *cobra.Command {
	var branch string

	cmd := &cobra.Command{
		Use:   "verify <package>",
		Short: "Verify stored package integrity in Dolt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.NewConfigFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("reading config flags: %w", err)
			}
			doltDir, err := detectDoltDir(cfg.DoltDirExpanded())
			if err != nil {
				return err
			}
			branch = resolveReadBranch(branch)

			client, err := openReadClient(doltDir, branch)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			summary, err := verifier.Service{Reader: client}.Verify(context.Background(), verifier.VerifyRequest{
				PackageID: args[0],
				Branch:    branch,
			})
			if err != nil {
				return err
			}

			formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
			if cfg.JSON {
				return formatter.WriteJSON(summary)
			}
			formatter.Success(fmt.Sprintf("Verified %s %s on %s", summary.PackageID, summary.Version, summary.Branch))
			formatter.Success(fmt.Sprintf("Files checked: %d  Corrupt files: %d", summary.FilesChecked, summary.CorruptFiles))
			formatter.Success(fmt.Sprintf("Aggregate status: %s", summary.AggregateStatus))
			return nil
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "Branch to verify from")
	return cmd
}
