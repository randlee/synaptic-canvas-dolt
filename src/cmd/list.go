package cmd

import (
	"fmt"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/spf13/cobra"
)

type listResponse = api.ListResponse
type listFilters = api.ListFilters
type listItem = api.ListItem

// NewListCmd creates the sc list command.
func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available packages",
		RunE:  runListCmd,
	}
	cmd.Flags().String("tags", "", "Comma-separated tag filters (case-insensitive OR match)")
	return cmd
}

func runListCmd(cmd *cobra.Command, _ []string) error {
	tagsValue, err := cmd.Flags().GetString("tags")
	if err != nil {
		return fmt.Errorf("reading --tags: %w", err)
	}
	tags := normalizeTagFilter(tagsValue)

	return withReadClient(cmd, func(cfg *config.Config, client readClient) error {
		formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
		formatter.Writer = cmd.OutOrStdout()
		formatter.ErrW = cmd.ErrOrStderr()

		packages, err := client.ListPackages(cmd.Context(), dolt.ListOptions{
			Branch: cfg.EffectiveBranch(),
			Tags:   tags,
		})
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}

		resp := listResponse{
			OK:     true,
			Branch: cfg.EffectiveBranch(),
			Filters: listFilters{
				Tags: tags,
			},
			Packages: make([]listItem, 0, len(packages)),
		}
		for _, pkg := range packages {
			resp.Packages = append(resp.Packages, listItem{
				ID:              pkg.ID,
				Name:            pkg.Name,
				Version:         pkg.Version,
				Branch:          cfg.EffectiveBranch(),
				Description:     pkg.Description,
				Tags:            pkg.TagsList(),
				Variant:         pkg.AgentVariant,
				InstallScope:    pkg.InstallScope,
				FileCount:       pkg.FileCount,
				DependencyCount: pkg.DepCount,
				SHA256:          pkg.SHA256,
			})
		}

		if cfg.JSON {
			return formatter.WriteJSON(resp)
		}

		rows := make([][]string, 0, len(resp.Packages))
		for _, pkg := range resp.Packages {
			rows = append(rows, []string{
				pkg.Name,
				pkg.Version,
				resp.Branch,
				pkg.Variant,
				string(pkg.InstallScope),
				strings.Join(pkg.Tags, ", "),
			})
		}
		return formatter.Table(
			[]string{"NAME", "VERSION", "BRANCH", "VARIANT", "SCOPE", "TAGS"},
			rows,
		)
	})
}

func normalizeTagFilter(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(part))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
