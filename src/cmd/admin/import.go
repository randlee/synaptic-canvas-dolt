package admin

import (
	"errors"
	"fmt"
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
			if err := cfg.LoadFileConfig(); err != nil {
				return err
			}
			formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
			formatter.Writer = cmd.OutOrStdout()
			formatter.ErrW = cmd.ErrOrStderr()
			if err := dolt.ValidateWriteClient(cfg); err != nil {
				return err
			}

			path, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolving import path: %w", err)
			}
			branch := cfg.EffectiveBranch()

			doltDir, err := dolt.DetectDoltDir(cfg.Get(config.KeyDoltDir, cfg.DoltDir))
			if err != nil {
				return err
			}
			readClient, err := openImportReadClient(cfg, doltDir, branch)
			if err != nil {
				return err
			}
			defer func() { _ = readClient.Close() }()

			svc := importer.Service{
				Writer: dolt.NewCLIWriter(doltDir),
				Client: readClient,
			}

			summary, err := svc.Import(cmd.Context(), importer.ImportRequest{
				PackageDir: path,
				Branch:     branch,
			})
			if err != nil {
				if cfg.JSON {
					if handled, writeErr := writeImportJSONError(formatter, err); handled {
						return writeErr
					}
				}
				return err
			}

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

type importSHACollisionJSON struct {
	OK    bool                        `json:"ok"`
	Error importSHACollisionErrorJSON `json:"error"`
	File  string                      `json:"file"`
}

type importSHACollisionErrorJSON struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	Branch      string `json:"branch"`
	ExistingSHA string `json:"existing_sha"`
	IncomingSHA string `json:"incoming_sha"`
}

func writeImportJSONError(formatter *output.Formatter, err error) (bool, error) {
	var collision *importer.SHACollisionError
	if !errors.As(err, &collision) {
		return false, nil
	}
	message := fmt.Sprintf("SHA collision for %s in package %s %s on branch %s", collision.File, collision.Package, collision.Version, collision.Branch)
	if writeErr := formatter.WriteJSON(importSHACollisionJSON{
		OK:   false,
		File: collision.File,
		Error: importSHACollisionErrorJSON{
			Code:        "sha_collision",
			Message:     message,
			Package:     collision.Package,
			Version:     collision.Version,
			Branch:      collision.Branch,
			ExistingSHA: collision.ExistingSHA,
			IncomingSHA: collision.IncomingSHA,
		},
	}); writeErr != nil {
		return true, writeErr
	}
	return true, err
}

func openImportReadClient(cfg *config.Config, doltDir, branch string) (dolt.Client, error) {
	selection, err := cfg.ResolveDoltClient()
	if err != nil {
		return nil, err
	}
	if selection.Client == "cli" && doltDir != "" {
		return dolt.NewCLIReader(doltDir, branch), nil
	}
	return dolt.OpenConfiguredReadClient(cfg, branch)
}

func detectDoltDir(configured string) (string, error) {
	return dolt.DetectDoltDir(configured)
}

// FormatImportAck returns a short status string for tests and future callers.
func FormatImportAck(packageID, branch string) string {
	return strings.TrimSpace(fmt.Sprintf("importing %s into %s", packageID, branch))
}
