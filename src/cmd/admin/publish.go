package admin

import (
	"context"
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/publisher"
	"github.com/spf13/cobra"
)

// NewPublishCmd creates the sc admin publish command.
func NewPublishCmd() *cobra.Command {
	var fromBranch string
	var toBranch string

	cmd := &cobra.Command{
		Use:   "publish <package>",
		Short: "Promote a package from one Dolt branch to another",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.NewConfigFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("reading config flags: %w", err)
			}
			if fromBranch == "" || toBranch == "" {
				return fmt.Errorf("--from and --to are required")
			}

			doltDir, err := detectDoltDir(cfg.DoltDirExpanded())
			if err != nil {
				return err
			}

			service := publisher.Service{
				Reader:   dolt.NewCLIReader(doltDir, fromBranch),
				Promoter: dolt.NewCLIPublisher(doltDir),
			}
			summary, err := service.Publish(context.Background(), publisher.PublishRequest{
				PackageID:  args[0],
				FromBranch: fromBranch,
				ToBranch:   toBranch,
			})
			if err != nil && summary == nil {
				return err
			}

			formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
			if cfg.JSON {
				if summary != nil {
					_ = formatter.WriteJSON(summary)
				}
				return err
			}

			if summary != nil {
				formatter.Success(fmt.Sprintf("Published %s %s from %s to %s", summary.PackageID, summary.Version, summary.FromBranch, summary.ToBranch))
				formatter.Success(fmt.Sprintf("Template warnings: %d  Template errors: %d", len(summary.TemplateWarnings), len(summary.TemplateValidationErrors)))
				if summary.Publish != nil {
					formatter.Success(fmt.Sprintf("Publish: %s", summary.Publish.Message))
				}
			}
			return err
		},
	}

	cmd.Flags().StringVar(&fromBranch, "from", "", "Source branch for publish")
	cmd.Flags().StringVar(&toBranch, "to", "", "Target branch for publish")
	return cmd
}
