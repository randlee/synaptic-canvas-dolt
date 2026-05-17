package cmd

import (
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/spf13/cobra"
)

type readClient = dolt.Client

var readClientOpener = openReadClient

func openReadClient(cfg *config.Config) (readClient, error) {
	return dolt.OpenConfiguredReadClient(cfg, cfg.EffectiveBranch())
}

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return nil, fmt.Errorf("reading config flags: %w", err)
	}
	if err := cfg.LoadFileConfig(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func withReadClient(cmd *cobra.Command, fn func(*config.Config, readClient) error) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	client, err := readClientOpener(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	return fn(cfg, client)
}
