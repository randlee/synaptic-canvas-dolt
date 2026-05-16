package cmd

import (
	"fmt"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/spf13/cobra"
)

type infoResponse = api.InfoResponse
type infoPackageShape = api.InfoPackage
type dependencyShape = api.Dependency

// NewInfoCmd creates the sc info command.
func NewInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <package>",
		Short: "Show package details",
		Args:  cobra.ExactArgs(1),
		RunE:  runInfoCmd,
	}
}

func runInfoCmd(cmd *cobra.Command, args []string) error {
	packageID := args[0]

	return withReadClient(cmd, func(cfg *config.Config, client readClient) error {
		formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
		formatter.Writer = cmd.OutOrStdout()
		formatter.ErrW = cmd.ErrOrStderr()

		pkg, err := client.GetPackageDetail(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
		if pkg == nil {
			err := fmt.Errorf("package %q not found", packageID)
			if cfg.JSON {
				return writeJSONError(formatter, "not_found", err.Error())
			}
			return err
		}

		deps, err := client.GetPackageDeps(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
		hooks, err := client.GetPackageHooks(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
		questions, err := client.GetPackageQuestions(cmd.Context(), packageID)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}

		resp := infoResponse{
			OK:     true,
			Branch: cfg.EffectiveBranch(),
			Package: infoPackageShape{
				ID:              pkg.ID,
				Name:            pkg.Name,
				Version:         pkg.Version,
				Description:     pkg.Description,
				Variant:         pkg.AgentVariant,
				InstallScope:    apiInstallScope(pkg.InstallScope),
				SHA256:          pkg.SHA256,
				FileCount:       pkg.FileCount,
				DependencyCount: pkg.DepCount,
				HookCount:       len(hooks),
				QuestionCount:   len(questions),
				Dependencies:    make([]dependencyShape, 0, len(deps)),
			},
		}
		for _, dep := range deps {
			resp.Package.Dependencies = append(resp.Package.Dependencies, dependencyShape{
				Name: dep.DepName,
				Type: apiDependencyType(dep.DepType),
				Spec: dep.DepSpec,
			})
		}

		if cfg.JSON {
			return formatter.WriteJSON(resp)
		}

		description := ""
		if pkg.Description != nil {
			description = *pkg.Description
		}
		rows := [][]string{
			{"Name", pkg.Name},
			{"Version", pkg.Version},
			{"Branch", cfg.EffectiveBranch()},
			{"Variant", pkg.AgentVariant},
			{"Install Scope", string(pkg.InstallScope)},
			{"File Count", fmt.Sprintf("%d", pkg.FileCount)},
			{"Dependency Count", fmt.Sprintf("%d", pkg.DepCount)},
			{"SHA256", stringValue(pkg.SHA256)},
			{"Description", description},
			{"Dependencies", formatDependencies(resp.Package.Dependencies)},
		}
		return formatter.Table([]string{"FIELD", "VALUE"}, rows)
	})
}

func formatDependencies(deps []dependencyShape) string {
	if len(deps) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(deps))
	for _, dep := range deps {
		part := string(dep.Type) + ":" + dep.Name
		if dep.Spec != "" {
			part += dep.Spec
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
