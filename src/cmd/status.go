package cmd

import (
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/spf13/cobra"
)

type statusResponse struct {
	OK       bool               `json:"ok"`
	Packages []statusPackageRow `json:"packages"`
}

type statusPackageRow struct {
	Package string            `json:"package"`
	Global  *statusScopeState `json:"global,omitempty"`
	Local   *statusScopeState `json:"local,omitempty"`
}

type statusScopeState struct {
	Version     string `json:"version"`
	Branch      string `json:"branch"`
	Validation  string `json:"validation"`
	InstallRoot string `json:"install_root"`
}

// NewStatusCmd creates the sc status command.
func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show tracked install status",
		RunE:  runStatusCmd,
	}
	cmd.Flags().String("scope", "both", "status scope: project, global, or both")
	return cmd
}

func runStatusCmd(cmd *cobra.Command, _ []string) error {
	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("reading config flags: %w", err)
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()
	scope, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("reading --scope: %w", err)
	}
	if err := validateScope(scope); err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", err.Error())
		}
		return err
	}

	repoRoot, err := currentRepoRoot()
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	installs, err := loadTrackedInstalls(repoRoot)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "query_failed", err.Error())
		}
		return err
	}
	installs = filterInstallsByScope(installs, scope)

	grouped := map[string]*statusPackageRow{}
	order := []string{}
	for _, install := range installs {
		summary, err := validateTrackedInstall(cmd.Context(), install.Record)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, "query_failed", err.Error())
			}
			return err
		}
		row := grouped[install.Record.Package]
		if row == nil {
			row = &statusPackageRow{Package: install.Record.Package}
			grouped[install.Record.Package] = row
			order = append(order, install.Record.Package)
		}
		state := &statusScopeState{
			Version:     install.Record.Version,
			Branch:      install.Record.Branch,
			Validation:  summary.Status,
			InstallRoot: install.Record.InstallRoot,
		}
		if install.Record.InstallScope == "global" {
			row.Global = state
		} else {
			row.Local = state
		}
	}

	rows := make([]statusPackageRow, 0, len(order))
	for _, pkg := range order {
		rows = append(rows, *grouped[pkg])
	}

	if cfg.JSON {
		return formatter.WriteJSON(statusResponse{
			OK:       true,
			Packages: rows,
		})
	}

	table := make([][]string, 0, len(rows))
	for _, row := range rows {
		global := ""
		if row.Global != nil {
			global = scopeDisplay(row.Global.Branch, row.Global.Version)
		}
		local := ""
		if row.Local != nil {
			local = scopeDisplay(row.Local.Branch, row.Local.Version)
		}
		validation := ""
		switch {
		case row.Global != nil && row.Local != nil:
			validation = "global:" + row.Global.Validation + " local:" + row.Local.Validation
		case row.Global != nil:
			validation = "global:" + row.Global.Validation
		case row.Local != nil:
			validation = "local:" + row.Local.Validation
		}
		table = append(table, []string{row.Package, global, local, validation})
	}

	return formatter.Table([]string{"PACKAGE", "GLOBAL", "LOCAL", "VALIDATION"}, table)
}
