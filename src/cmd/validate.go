package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/spf13/cobra"
)

type validateResponse = api.ValidateResponse

// NewValidateCmd creates the sc validate command.
func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [package]",
		Short: "Validate tracked installs",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runValidateCmd,
	}
	cmd.Flags().Bool("all", false, "validate all tracked installs")
	cmd.Flags().String("scope", "both", "validation scope: project, global, or both")
	return cmd
}

func runValidateCmd(cmd *cobra.Command, args []string) error {
	packageID := ""
	if len(args) == 1 {
		packageID = args[0]
	}
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return fmt.Errorf("reading --all: %w", err)
	}
	scope, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("reading --scope: %w", err)
	}

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

	if packageID == "" && !all {
		message := "specify a package name or use --all to validate all installs"
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", message)
		}
		return errors.New(message)
	}

	repoRoot, err := currentRepoRoot()
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
		}
		return err
	}
	installs, err := loadTrackedInstalls(repoRoot)
	if err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
		}
		return err
	}
	filtered := filterInstallsByScope(filterInstalls(installs, packageID), scope)
	if len(filtered) == 0 {
		message := "no tracked installs found"
		if packageID != "" {
			message = fmt.Sprintf("package %q is not installed", packageID)
		}
		if cfg.JSON {
			return writeJSONError(formatter, "not_found", message)
		}
		return errors.New(message)
	}
	summaries := make([]validatedInstall, 0, len(filtered))
	pass := true
	for _, install := range filtered {
		summary, err := validateTrackedInstall(cmd.Context(), install.Record)
		if err != nil {
			if cfg.JSON {
				return writeJSONError(formatter, classifyJSONError(err.Error()), err.Error())
			}
			return err
		}
		if !summary.Pass {
			pass = false
		}
		summaries = append(summaries, summary)
	}

	if cfg.JSON {
		if err := formatter.WriteJSON(validateResponse{
			OK:       true,
			Pass:     pass,
			Packages: summaries,
		}); err != nil {
			return err
		}
		if !pass {
			return jsonCmdError{cause: errors.New("validation failed")}
		}
		return nil
	}

	rows := make([][]string, 0, len(summaries))
	for _, summary := range summaries {
		rows = append(rows, []string{
			summary.Package,
			summary.Scope,
			summary.Status,
			fmt.Sprintf("%d", visibleValidatedFileCount(summary.Files, cfg.Verbose)),
			summary.AggregateStatus,
		})
	}
	if err := formatter.Table([]string{"PACKAGE", "SCOPE", "STATUS", "ITEMS", "AGGREGATE"}, rows); err != nil {
		return err
	}
	for _, summary := range summaries {
		formatter.Success(fmt.Sprintf("%s [%s]", summary.Package, summary.Scope))
		for _, warning := range summary.Warnings {
			writeWarning(formatter, warning)
		}
		fileRows := make([][]string, 0, len(summary.Files))
		for _, file := range summary.Files {
			if !cfg.Verbose && file.Severity == ValidationSeverityInfo {
				continue
			}
			status := file.Status
			if file.Error != "" {
				status += ": " + file.Error
			}
			fileRows = append(fileRows, []string{file.Path, status, string(file.Severity)})
		}
		if err := formatter.Table([]string{"FILE", "STATUS", "SEVERITY"}, fileRows); err != nil {
			return err
		}
	}
	if !pass {
		return errors.New("validation failed")
	}
	return nil
}

func visibleValidatedFileCount(files []validatedFile, verbose bool) int {
	if verbose {
		return len(files)
	}
	count := 0
	for _, file := range files {
		if file.Severity != ValidationSeverityInfo {
			count++
		}
	}
	return count
}

func currentRepoRoot() (string, error) {
	root, err := osGetwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	return root, nil
}

var osGetwd = func() (string, error) {
	return os.Getwd()
}
