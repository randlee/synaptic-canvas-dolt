package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/importer"
	"github.com/spf13/cobra"
)

// NewImportCmd creates the sc admin import command.
func NewImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Import a package directory into Dolt on a specific branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.NewConfigFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("reading config flags: %w", err)
			}

			path, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolving import path: %w", err)
			}
			branch := cfg.EffectiveBranch()

			doltDir, err := detectDoltDir(cfg.DoltDirExpanded())
			if err != nil {
				return err
			}

			svc := importer.Service{
				Writer: dolt.NewCLIWriter(doltDir),
			}

			summary, err := svc.Import(context.Background(), importer.ImportRequest{
				PackageDir: path,
				Branch:     branch,
			})
			if err != nil {
				return err
			}

			formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
			if cfg.JSON {
				return formatter.WriteJSON(summary)
			}

			lines := []string{
				fmt.Sprintf("Imported %s %s into %s", summary.PackageID, summary.Version, summary.Branch),
				fmt.Sprintf("Files: %d  Deps: %d  Hooks: %d  Questions: %d", summary.FilesImported, summary.DepsImported, summary.HooksImported, summary.QuestionsImported),
				fmt.Sprintf("Package SHA256: %s", summary.PackageSHA256),
				fmt.Sprintf("Commit: %s", summary.CommitMessage),
			}
			for _, line := range lines {
				formatter.Success(line)
			}
			if len(summary.TemplateValidationWarnings) > 0 {
				formatter.Success("Template validation warnings:")
				for _, warning := range summary.TemplateValidationWarnings {
					formatter.Success("- " + warning)
				}
			}
			return nil
		},
	}

	return cmd
}

func detectDoltDir(configured string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(filepath.Join(configured, ".dolt")); err != nil {
			return "", fmt.Errorf("invalid --dolt-dir %q: %w", configured, err)
		}
		return configured, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".dolt")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.New("could not auto-detect Dolt database directory; pass --dolt-dir")
}

// FormatImportAck returns a short status string for tests and future callers.
func FormatImportAck(packageID, branch string) string {
	return strings.TrimSpace(fmt.Sprintf("importing %s into %s", packageID, branch))
}
