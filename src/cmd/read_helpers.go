package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/spf13/cobra"
)

type readClient interface {
	ListPackages(context.Context, dolt.ListOptions) ([]models.Package, error)
	GetPackageDetail(context.Context, string) (*models.Package, error)
	GetPackageDeps(context.Context, string) ([]models.PackageDep, error)
	GetPackageHooks(context.Context, string) ([]models.PackageHook, error)
	GetPackageQuestions(context.Context, string) ([]models.PackageQuestion, error)
	Close() error
}

var readClientOpener = openReadClient
var detectReadDoltDir = detectReadDoltDirImpl

func openReadClient(doltDir string, branch string) (readClient, error) {
	if doltDir != "" {
		return dolt.NewCLIReader(doltDir, branch), nil
	}
	cfg := dolt.DefaultConfig()
	return dolt.OpenForBranch(cfg, branch)
}

func loadConfigAndReadDoltDir(cmd *cobra.Command) (*config.Config, string, error) {
	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return nil, "", fmt.Errorf("reading config flags: %w", err)
	}
	doltDir, err := detectReadDoltDir(cfg.DoltDirExpanded())
	if err != nil {
		return nil, "", err
	}
	return cfg, doltDir, nil
}

func withReadClient(cmd *cobra.Command, fn func(*config.Config, readClient) error) error {
	cfg, doltDir, err := loadConfigAndReadDoltDir(cmd)
	if err != nil {
		return err
	}

	client, err := readClientOpener(doltDir, cfg.EffectiveBranch())
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	return fn(cfg, client)
}

func detectReadDoltDirImpl(configured string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(filepath.Join(configured, ".dolt")); err != nil {
			return "", fmt.Errorf("invalid --dolt-dir %q: %w", configured, err)
		}
		return configured, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".dolt")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.New("could not auto-detect Dolt database directory; pass --dolt-dir")
}
