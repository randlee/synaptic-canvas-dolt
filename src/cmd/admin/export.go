package admin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/exporter"
	"github.com/spf13/cobra"
)

// NewExportCmd creates the sc admin export command.
func NewExportCmd() *cobra.Command {
	var branch string
	var outputDir string

	cmd := &cobra.Command{
		Use:   "export <package>",
		Short: "Export a package directory from Dolt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.NewConfigFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("reading config flags: %w", err)
			}
			if outputDir == "" {
				return fmt.Errorf("--output is required")
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

			absOutputDir, err := filepath.Abs(outputDir)
			if err != nil {
				return fmt.Errorf("resolving output path: %w", err)
			}

			svc := exporter.Service{Reader: client}
			summary, err := svc.Export(context.Background(), exporter.ExportRequest{
				PackageID: args[0],
				OutputDir: absOutputDir,
				Branch:    branch,
			})
			if err != nil {
				return err
			}

			formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
			if cfg.JSON {
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
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "Branch to export from")
	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory for exported package")
	return cmd
}

func resolveReadBranch(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("SC_DOLT_BRANCH"); env != "" {
		return env
	}
	return "main"
}

func openReadClient(doltDir, branch string) (*dolt.SQLClient, error) {
	cfg := dolt.DefaultConfig()
	_ = doltDir
	return dolt.OpenForBranch(cfg, branch)
}
