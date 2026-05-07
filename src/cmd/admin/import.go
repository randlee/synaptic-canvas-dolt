package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

			path, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolving import path: %w", err)
			}
			branch := cfg.EffectiveBranch()

			doltDir, err := detectDoltDir(config.ExpandPath(cfg.Get(config.KeyDoltDir, cfg.DoltDir)))
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

			summary, err := svc.Import(context.Background(), importer.ImportRequest{
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
	Error       string `json:"error"`
	File        string `json:"file"`
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
	return true, formatter.WriteJSON(importSHACollisionJSON{
		Error:       "sha_collision",
		File:        collision.File,
		Package:     collision.Package,
		Version:     collision.Version,
		Branch:      collision.Branch,
		ExistingSHA: collision.ExistingSHA,
		IncomingSHA: collision.IncomingSHA,
	})
}

func openImportReadClient(cfg *config.Config, doltDir, branch string) (dolt.Client, error) {
	clientType := cfg.Get(config.KeyDoltClient, "http")
	switch clientType {
	case "http":
		database := cfg.Get(config.KeyDoltDatabase, "")
		if database == "" {
			return nil, fmt.Errorf("dolt.database is not configured; run: sc config set dolt.database <owner/database>")
		}
		return dolt.NewHTTPClient(dolt.HTTPConfig{
			Host:     cfg.Get(config.KeyDoltHost, "www.dolthub.com"),
			Database: database,
			Branch:   branch,
			Token:    cfg.Get(config.KeyDoltToken, ""),
			Timeout:  time.Duration(cfg.GetInt(config.KeyDoltTimeout, 30)) * time.Second,
		}), nil
	case "sql":
		dsn := cfg.Get(config.KeyDoltDSN, "")
		if dsn == "" {
			return nil, fmt.Errorf("dolt.dsn is not configured; run: sc config set dolt.dsn <dsn>")
		}
		sqlCfg, err := dolt.ParseDSN(dsn)
		if err != nil {
			return nil, err
		}
		return dolt.OpenForBranch(sqlCfg, branch)
	case "cli":
		if doltDir == "" {
			return nil, fmt.Errorf("dolt.dir is not configured; run: sc config set dolt.dir <path>")
		}
		return dolt.NewCLIReader(doltDir, branch), nil
	default:
		return nil, fmt.Errorf("unsupported dolt.client %q", clientType)
	}
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
