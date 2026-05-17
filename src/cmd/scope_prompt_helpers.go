package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/spf13/cobra"
)

func validateScope(scope string) error {
	switch scope {
	case "project", "global", "both":
		return nil
	default:
		return fmt.Errorf("invalid --scope %q; expected project, global, or both", scope)
	}
}

func externalDeps(deps []models.PackageDep) []models.PackageDep {
	result := make([]models.PackageDep, 0, len(deps))
	for _, dep := range deps {
		if dep.DepType == models.DepTypeCLI || dep.DepType == models.DepTypeTool {
			result = append(result, dep)
		}
	}
	return result
}

func confirmExternalDeps(cmd *cobra.Command, formatter *output.Formatter, deps []models.PackageDep, yolo, dryRun bool) error {
	external := externalDeps(deps)
	if len(external) == 0 || yolo || dryRun {
		return nil
	}
	if formatter.JSON {
		return fmt.Errorf("interactive confirmation required; use --yolo to proceed non-interactively")
	}
	for _, dep := range external {
		line := fmt.Sprintf("dependency: %s %s %s", dep.DepType, dep.DepName, dep.DepSpec)
		formatter.Success(strings.TrimSpace(line))
	}
	if !isCommandInputTTY(cmd) {
		return fmt.Errorf("interactive confirmation required; use --yolo to proceed non-interactively")
	}
	_, _ = fmt.Fprint(cmd.ErrOrStderr(), "Proceed? [y/N] ")
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("installation cancelled")
	}
	return nil
}

func confirmProceed(cmd *cobra.Command, prompt, nonInteractiveError string, yolo, force bool) error {
	if yolo || force {
		return nil
	}
	if !isCommandInputTTY(cmd) {
		return errors.New(nonInteractiveError)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N] ", prompt)
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("operation cancelled")
	}
	return nil
}

var removeSCDependency = func(string) error { return nil }

func confirmRemoveDependency(cmd *cobra.Command, dep string) (bool, error) {
	if !isCommandInputTTY(cmd) {
		return false, nil
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Remove SC-installed dependency %s? [y/N] ", dep)
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("reading dependency removal confirmation: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}

func isCommandInputTTY(cmd *cobra.Command) bool {
	file, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
