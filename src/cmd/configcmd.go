package cmd

import (
	"fmt"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/spf13/cobra"
)

type configGetResponse = api.ConfigGetResponse
type configSetResponse = api.ConfigSetResponse

// NewConfigCmd creates the sc config command group.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and write sc configuration",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Read a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Write a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	})
	return cmd
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	cfg, formatter, err := loadCommandConfig(cmd)
	if err != nil {
		return err
	}
	if !config.IsKnownKey(key) {
		message := fmt.Sprintf("unknown config key %q; valid keys: %s", key, strings.Join(config.KnownKeys(), ", "))
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", message)
		}
		return fmt.Errorf("%s", message)
	}

	value := cfg.Get(key, "")
	if cfg.JSON {
		return formatter.WriteJSON(configGetResponse{OK: true, Key: key, Value: value})
	}
	formatter.Success(value)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	cfg, formatter, err := loadCommandConfig(cmd)
	if err != nil {
		return err
	}
	if !config.IsKnownKey(key) {
		message := fmt.Sprintf("unknown config key %q; valid keys: %s", key, strings.Join(config.KnownKeys(), ", "))
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", message)
		}
		return fmt.Errorf("%s", message)
	}

	path, err := config.SetFileValue(key, value)
	if err != nil {
		if cfg.JSON {
			return writeClassifiedJSONError(formatter, cfg, err)
		}
		return err
	}
	if cfg.JSON {
		return formatter.WriteJSON(configSetResponse{OK: true, Key: key, Path: path})
	}
	formatter.Success(fmt.Sprintf("set %s in %s", key, path))
	return nil
}

func loadCommandConfig(cmd *cobra.Command) (*config.Config, *output.Formatter, error) {
	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("reading config flags: %w", err)
	}
	if err := cfg.LoadFileConfig(); err != nil {
		return nil, nil, err
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()
	return cfg, formatter, nil
}
