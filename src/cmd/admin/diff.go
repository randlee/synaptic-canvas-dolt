package admin

import (
	"context"
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/differ"
	"github.com/spf13/cobra"
)

// NewDiffCmd creates the sc admin diff command.
func NewDiffCmd() *cobra.Command {
	var branch1 string
	var branch2 string

	cmd := &cobra.Command{
		Use:   "diff <package>",
		Short: "Diff a package across two Dolt branches",
		Args:  cobra.ExactArgs(1),
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDiffCmd(cmd, args, branch1, branch2)
	}

	cmd.Flags().StringVar(&branch1, "branch1", "", "First branch to compare")
	cmd.Flags().StringVar(&branch2, "branch2", "", "Second branch to compare")
	return cmd
}

func runDiffCmd(cmd *cobra.Command, args []string, branch1, branch2 string) error {
	if branch1 == "" || branch2 == "" {
		return fmt.Errorf("--branch1 and --branch2 are required")
	}
	return withReadClients(cmd, branch1, branch2, func(cfg *config.Config, client1, client2 readClient) error {
		summary, err := runDiff(context.Background(), args[0], branch1, branch2, client1, client2)
		if err != nil {
			return err
		}

		formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
		return writeDiffResult(formatter, summary)
	})
}

func runDiff(ctx context.Context, packageID, branch1, branch2 string, reader1, reader2 differ.Reader) (*differ.Summary, error) {
	return differ.Service{Reader1: reader1, Reader2: reader2}.Diff(ctx, differ.DiffRequest{
		PackageID: packageID,
		Branch1:   branch1,
		Branch2:   branch2,
	})
}

func writeDiffResult(formatter *output.Formatter, summary *differ.Summary) error {
	if formatter.JSON {
		return formatter.WriteJSON(summary)
	}
	formatter.Success(fmt.Sprintf("Diffed %s between %s and %s", summary.PackageID, summary.Branch1, summary.Branch2))
	formatter.Success(fmt.Sprintf("Changes: %d", len(summary.FileChanges)))
	for _, change := range summary.FileChanges {
		formatter.Success(fmt.Sprintf("%s %s", change.Type, change.DestPath))
	}
	return nil
}
