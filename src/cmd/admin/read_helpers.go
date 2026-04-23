package admin

import (
	"context"
	"fmt"
	"os"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
)

func resolveReadBranch(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("SC_DOLT_BRANCH"); env != "" {
		return env
	}
	return "main"
}

func openReadClient(_ string, branch string) (*dolt.SQLClient, error) {
	cfg := dolt.DefaultConfig()
	client, err := dolt.Open(cfg)
	if err != nil {
		return nil, err
	}
	if err := client.UseBranch(context.Background(), branch); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("switching read client to %s: %w", branch, err)
	}
	return client, nil
}
