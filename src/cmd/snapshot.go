package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
	"github.com/spf13/cobra"
)

type snapshotMetadata struct {
	Package    string `toml:"package"`
	Branch     string `toml:"branch"`
	Version    string `toml:"version"`
	Scope      string `toml:"scope"`
	SourcePath string `toml:"source_path"`
	RepoPath   string `toml:"repo_path"`
	RepoURL    string `toml:"repo_url"`
	SnapshotAt string `toml:"snapshot_at"`
}

type snapshotResponse = api.SnapshotResponse

// NewSnapshotCmd creates the sc snapshot command.
func NewSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot <package>",
		Short: "Export modified or full installed package contents",
		Args:  cobra.ExactArgs(1),
		RunE:  runSnapshotCmd,
	}
	cmd.Flags().String("scope", "both", "snapshot scope: project, global, or both")
	cmd.Flags().Bool("full", false, "snapshot the full installed package")
	return cmd
}

func runSnapshotCmd(cmd *cobra.Command, args []string) error {
	scope, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("reading --scope: %w", err)
	}
	full, err := cmd.Flags().GetBool("full")
	if err != nil {
		return fmt.Errorf("reading --full: %w", err)
	}
	packageID := args[0]

	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("reading config flags: %w", err)
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()
	if err := validateScope(scope); err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", err.Error())
		}
		return err
	}

	repoRoot, err := currentRepoRoot()
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, classifyJSONErr(err), err.Error())
		}
		return err
	}
	installs, err := loadTrackedInstalls(repoRoot)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, classifyJSONErr(err), err.Error())
		}
		return err
	}
	selected := filterInstallsByScope(filterInstalls(installs, packageID), scope)
	if len(selected) == 0 {
		message := fmt.Sprintf("package %q is not installed", packageID)
		if cfg.JSON {
			return writeJSONError(formatter, "not_found", message)
		}
		return errors.New(message)
	}
	if len(selected) > 1 && scope == "both" {
		message := fmt.Sprintf("package %q is installed in multiple scopes; pass --scope", packageID)
		if cfg.JSON {
			return writeJSONError(formatter, api.ErrorCodeAmbiguousTarget, message)
		}
		return errors.New(message)
	}

	record := selected[0].Record
	files, err := snapshotFiles(cmd.Context(), record, full)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, classifyJSONErr(err), err.Error())
		}
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, classifyJSONErr(err), err.Error())
		}
		return err
	}
	now := snapshotNow()
	outputDir := filepath.Join(
		home,
		".synaptic",
		"mod-snapshots",
		record.Package,
		record.Branch,
		repoKey(record.InstallSite),
		now.Format("20060102T150405Z"),
	)
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("creating snapshot dir: %w", err)
	}

	copied := make([]string, 0, len(files))
	for _, rel := range files {
		src := filepath.Join(record.InstallRoot, filepath.FromSlash(rel))
		dst := filepath.Join(outputDir, filepath.FromSlash(rel))
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copying %s: %w", rel, err)
		}
		copied = append(copied, rel)
	}

	meta := snapshotMetadata{
		Package:    record.Package,
		Branch:     record.Branch,
		Version:    record.Version,
		Scope:      record.InstallScope,
		SourcePath: record.InstallRoot,
		RepoPath:   record.InstallSite,
		RepoURL:    snapshotGitRemoteURL(record.InstallSite),
		SnapshotAt: now.Format(time.RFC3339),
	}
	data, err := toml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encoding snapshot metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "snapshot.toml"), data, 0o600); err != nil {
		return fmt.Errorf("writing snapshot metadata: %w", err)
	}

	if cfg.JSON {
		return formatter.WriteJSON(snapshotResponse{
			OK:        true,
			Package:   record.Package,
			Scope:     record.InstallScope,
			OutputDir: outputDir,
			Files:     copied,
		})
	}

	rows := [][]string{
		{"package", record.Package},
		{"scope", record.InstallScope},
		{"output_dir", outputDir},
		{"files", fmt.Sprintf("%d", len(copied))},
	}
	return formatter.Table([]string{"FIELD", "VALUE"}, rows)
}

func snapshotFiles(ctx context.Context, record installer.InstallRecord, full bool) ([]string, error) {
	if full {
		files := []string{}
		err := filepath.Walk(record.InstallRoot, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(record.InstallRoot, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		return files, err
	}

	summary, err := validateTrackedInstall(ctx, record)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, file := range summary.Files {
		if file.Status != integrity.StatusOK.String() {
			path := filepath.Join(record.InstallRoot, filepath.FromSlash(file.Path))
			if _, err := os.Stat(path); err == nil {
				files = append(files, file.Path)
			}
		}
	}
	return files, nil
}

func copyFile(src, dst string) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	in, err := os.Open(src) //nolint:gosec // src path is derived from tracked install roots.
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst) //nolint:gosec // destination is created under the validated snapshot output directory.
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
