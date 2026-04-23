package admin

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/exporter"
	"github.com/spf13/cobra"
)

// NewExportCmd creates the sc admin export command.
func NewExportCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "export <package>",
		Short: "Export a package directory from Dolt",
		Args:  cobra.ExactArgs(1),
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExportCmd(cmd, args, outputDir)
	}

	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory for exported package")
	return cmd
}

func runExportCmd(cmd *cobra.Command, args []string, outputDir string) error {
	if outputDir == "" {
		return fmt.Errorf("--output is required")
	}
	return withReadClient(cmd, "", func(cfg *config.Config, client readClient) error {
		summary, err := runExport(context.Background(), cfg, args[0], outputDir, client)
		if err != nil {
			return err
		}

		formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
		return writeExportResult(formatter, summary)
	})
}

func runExport(ctx context.Context, cfg *config.Config, packageID, outputDir string, reader exporter.Reader) (*exporter.Summary, error) {
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("resolving output path: %w", err)
	}

	svc := exporter.Service{Reader: reader}
	return svc.Export(ctx, exporter.ExportRequest{
		PackageID: packageID,
		OutputDir: absOutputDir,
		Branch:    cfg.EffectiveBranch(),
	})
}

func writeExportResult(formatter *output.Formatter, summary *exporter.Summary) error {
	if formatter.JSON {
		return formatter.WriteJSON(summary)
	}

	formatter.Success(fmt.Sprintf("Exported %s %s from %s", summary.PackageID, summary.Version, summary.Branch))
	formatter.Success(fmt.Sprintf("Output: %s", summary.OutputDir))
	formatter.Success(fmt.Sprintf("Files written: %d  SHA verified: %d", summary.FilesWritten, summary.FileSHAVerified))
	formatter.Success(fmt.Sprintf("Package SHA256: %s", summary.PackageSHA256))
	if summary.PluginReconstructed {
		formatter.Success("Plugin manifest reconstructed from package metadata")
	}
	return nil
}
