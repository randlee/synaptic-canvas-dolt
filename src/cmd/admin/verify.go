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
	cmd := &cobra.Command{
		Use:   "verify <package>",
		Short: "Verify stored package integrity in Dolt",
		Args:  cobra.ExactArgs(1),
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runVerifyCmd(cmd, args)
	}

	return cmd
}

func runVerifyCmd(cmd *cobra.Command, args []string) error {
	branch, err := resolveReadBranch(cmd)
	if err != nil {
		return err
	}
	return withReadClient(cmd, branch, func(cfg *config.Config, client readClient) error {
		summary, err := runVerify(context.Background(), cfg, args[0], client)
		if err != nil {
			return err
		}

		formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
		return writeVerifyResult(formatter, summary)
	})
}

func runVerify(ctx context.Context, cfg *config.Config, packageID string, reader verifier.Reader) (*verifier.Summary, error) {
	return verifier.Service{Reader: reader}.Verify(ctx, verifier.VerifyRequest{
		PackageID: packageID,
		Branch:    cfg.EffectiveBranch(),
	})
}

func writeVerifyResult(formatter *output.Formatter, summary *verifier.Summary) error {
	if formatter.JSON {
		return formatter.WriteJSON(summary)
	}
	for _, result := range summary.FileResults {
		if result.Status == verifier.StatusOK {
			formatter.Success(fmt.Sprintf("OK: %s", result.DestPath))
			continue
		}
		formatter.Error(fmt.Sprintf("CORRUPT: %s (expected %s, got %s)", result.DestPath, result.ExpectedSHA, result.ActualSHA))
	}
	formatter.Success(fmt.Sprintf("Verified %s %s on %s", summary.PackageID, summary.Version, summary.Branch))
	formatter.Success(fmt.Sprintf("Files checked: %d  Corrupt files: %d", summary.FilesChecked, summary.CorruptFiles))
	formatter.Success(fmt.Sprintf("Aggregate status: %s", summary.AggregateStatus))
	return nil
}
