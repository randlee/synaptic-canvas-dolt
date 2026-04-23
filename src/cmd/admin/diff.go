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
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.NewConfigFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("reading config flags: %w", err)
			}
			if branch1 == "" || branch2 == "" {
				return fmt.Errorf("--branch1 and --branch2 are required")
			}
			doltDir, err := detectDoltDir(cfg.DoltDirExpanded())
			if err != nil {
				return err
			}

			client1, err := openReadClient(doltDir, branch1)
			if err != nil {
				return err
			}
			defer func() { _ = client1.Close() }()

			client2, err := openReadClient(doltDir, branch2)
			if err != nil {
				return err
			}
			defer func() { _ = client2.Close() }()

			summary, err := differ.Service{Reader1: client1, Reader2: client2}.Diff(context.Background(), differ.DiffRequest{
				PackageID: args[0],
				Branch1:   branch1,
				Branch2:   branch2,
			})
			if err != nil {
				return err
			}

			formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
			if cfg.JSON {
				return formatter.WriteJSON(summary)
			}
			formatter.Success(fmt.Sprintf("Diffed %s between %s and %s", summary.PackageID, summary.Branch1, summary.Branch2))
			formatter.Success(fmt.Sprintf("Changes: %d", len(summary.FileChanges)))
			for _, change := range summary.FileChanges {
				formatter.Success(fmt.Sprintf("%s %s", change.Type, change.DestPath))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&branch1, "branch1", "", "First branch to compare")
	cmd.Flags().StringVar(&branch2, "branch2", "", "Second branch to compare")
	return cmd
}
