package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/spf13/cobra"
)

type catalogUpdateResponse struct {
	OK      bool     `json:"ok"`
	Branch  string   `json:"branch"`
	Entries int      `json:"entries"`
	Path    string   `json:"path"`
	Paths   []string `json:"paths,omitempty"`
}

// NewCatalogCmd creates the sc catalog command group.
func NewCatalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage local SHA catalogs",
	}
	update := &cobra.Command{
		Use:   "update",
		Short: "Refresh local SHA catalog caches",
		Args:  cobra.NoArgs,
		RunE:  runCatalogUpdateCmd,
	}
	update.Flags().String("scope", "both", "catalog scope to update: project, global, or both")
	cmd.AddCommand(update)
	return cmd
}

func runCatalogUpdateCmd(cmd *cobra.Command, _ []string) error {
	scope, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("reading --scope: %w", err)
	}
	if err := validateCatalogScope(scope); err != nil {
		return err
	}
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()

	repoRoot, err := currentRepoRoot()
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	client, err := readClientOpener(cfg)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	defer func() { _ = client.Close() }()

	paths, entries, err := updateCatalogCaches(cmd.Context(), repoRoot, cfg.EffectiveBranch(), scope, client)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	displayPaths := displayCatalogPaths(paths)
	if cfg.JSON {
		path := ""
		if len(displayPaths) > 0 {
			path = displayPaths[0]
		}
		return formatter.WriteJSON(catalogUpdateResponse{
			OK:      true,
			Branch:  cfg.EffectiveBranch(),
			Entries: entries,
			Path:    path,
			Paths:   displayPaths,
		})
	}
	rows := [][]string{
		{"branch", cfg.EffectiveBranch()},
		{"entries", fmt.Sprintf("%d", entries)},
		{"paths", strings.Join(displayPaths, ", ")},
	}
	return formatter.Table([]string{"FIELD", "VALUE"}, rows)
}

func updateCatalogCaches(ctx context.Context, repoRoot, branch, scope string, client readClient) ([]string, int, error) {
	entries, err := client.GetPackageCatalog(ctx)
	if err != nil {
		return nil, 0, err
	}
	paths, err := writeCatalogCaches(repoRoot, branch, scope, entries, time.Now().UTC())
	if err != nil {
		return nil, 0, err
	}
	return paths, len(entries), nil
}

func writeCatalogCaches(repoRoot, branch, scope string, entries []catalog.CatalogEntry, fetchedAt time.Time) ([]string, error) {
	if err := validateCatalogScope(scope); err != nil {
		return nil, err
	}
	paths := []string{}
	if scope == "project" || scope == "both" || scope == "" {
		paths = append(paths, catalog.ProjectPath(repoRoot, branch))
	}
	if scope == "global" || scope == "both" || scope == "" {
		path, err := catalog.MachinePath(branch)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	for _, path := range paths {
		if _, err := catalog.Refresh(path, branch, entries, fetchedAt); err != nil {
			return paths, err
		}
	}
	return paths, nil
}

func refreshCatalogNonFatal(ctx context.Context, formatter *output.Formatter, repoRoot, branch string, client readClient) []string {
	_, _, err := updateCatalogCaches(ctx, repoRoot, branch, "both", client)
	if err == nil {
		return nil
	}
	warnings := []string{"catalog refresh failed: " + err.Error()}
	if !formatter.JSON {
		for _, warning := range warnings {
			writeWarning(formatter, warning)
		}
	}
	return warnings
}

func refreshCatalogWithConfigNonFatal(ctx context.Context, formatter *output.Formatter, repoRoot string, cfg *config.Config) []string {
	client, err := readClientOpener(cfg)
	if err != nil {
		warnings := []string{"catalog refresh failed: " + err.Error()}
		if !formatter.JSON {
			for _, warning := range warnings {
				writeWarning(formatter, warning)
			}
		}
		return warnings
	}
	defer func() { _ = client.Close() }()
	return refreshCatalogNonFatal(ctx, formatter, repoRoot, cfg.EffectiveBranch(), client)
}

func validateCatalogScope(scope string) error {
	if scope == "" {
		return nil
	}
	return validateScope(scope)
}

func displayCatalogPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		result = append(result, displayCatalogPath(path))
	}
	return result
}

func displayCatalogPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	prefix := home + string(os.PathSeparator)
	if strings.HasPrefix(path, prefix) {
		return "~" + string(os.PathSeparator) + strings.TrimPrefix(path, prefix)
	}
	return path
}

func writeWarning(formatter *output.Formatter, warning string) {
	if formatter.Quiet {
		return
	}
	w := formatter.ErrW
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintln(w, "warning: "+warning)
}
