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
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublishCmd(cmd, args, fromBranch, toBranch)
	}

	cmd.Flags().StringVar(&fromBranch, "from", "", "Source branch for publish")
	cmd.Flags().StringVar(&toBranch, "to", "", "Target branch for publish")
	return cmd
}

func runPublishCmd(cmd *cobra.Command, args []string, fromBranch, toBranch string) error {
	if fromBranch == "" || toBranch == "" {
		return fmt.Errorf("--from and --to are required")
	}
	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("reading config flags: %w", err)
	}
	if err := cfg.LoadFileConfig(); err != nil {
		return err
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()
	if err := dolt.ValidateWriteClient(cfg); err != nil {
		return err
	}
	doltDir, err := dolt.DetectDoltDir(cfg.Get(config.KeyDoltDir, cfg.DoltDir))
	if err != nil {
		return err
	}

	service := publisher.Service{
		Reader:   dolt.NewCLIReader(doltDir, fromBranch),
		Promoter: dolt.NewCLIPublisher(doltDir),
	}
	summary, err := runPublish(context.Background(), args[0], fromBranch, toBranch, service.Reader, service.Promoter)
	if err != nil && summary == nil {
		return err
	}

	return writePublishResult(formatter, summary, err)
}

func runPublish(ctx context.Context, packageID, fromBranch, toBranch string, reader publisher.Reader, promoter publisher.Promoter) (*publisher.Summary, error) {
	service := publisher.Service{
		Reader:   reader,
		Promoter: promoter,
	}
	return service.Publish(ctx, publisher.PublishRequest{
		PackageID:  packageID,
		FromBranch: fromBranch,
		ToBranch:   toBranch,
	})
}

func writePublishResult(formatter *output.Formatter, summary *publisher.Summary, runErr error) error {
	if formatter.JSON {
		if summary != nil {
			if err := formatter.WriteJSON(summary); err != nil {
				return err
			}
		}
		return runErr
	}

	if summary != nil {
		formatter.Success(fmt.Sprintf("Published %s %s from %s to %s", summary.PackageID, summary.Version, summary.FromBranch, summary.ToBranch))
		formatter.Success(fmt.Sprintf("Template warnings: %d  Template errors: %d", len(summary.TemplateWarnings), len(summary.TemplateValidationErrors)))
		if summary.Publish != nil {
			formatter.Success(fmt.Sprintf("Publish: %s", summary.Publish.Message))
		}
	}
	return runErr
}
