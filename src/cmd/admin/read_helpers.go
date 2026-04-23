package admin

import (
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

func openReadClient(doltDir string, branch string) (*dolt.SQLClient, error) {
	cfg := dolt.DefaultConfig()
	_ = doltDir
	return dolt.OpenForBranch(cfg, branch)
}
