package admin

import (
	"context"
	"fmt"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/spf13/cobra"
)

type readClient interface {
	GetPackage(context.Context, string) (*models.Package, error)
	GetPackageFiles(context.Context, string) ([]models.PackageFile, error)
	GetPackageDeps(context.Context, string) ([]models.PackageDep, error)
	GetPackageHooks(context.Context, string) ([]models.PackageHook, error)
	GetPackageQuestions(context.Context, string) ([]models.PackageQuestion, error)
	Close() error
}

var readClientOpener = openReadClient

func openReadClient(doltDir string, branch string) (readClient, error) {
	if doltDir != "" {
		return dolt.NewCLIReader(doltDir, branch), nil
	}
	cfg := dolt.DefaultConfig()
	return dolt.OpenForBranch(cfg, branch)
}

func loadConfigAndDoltDir(cmd *cobra.Command) (*config.Config, string, error) {
	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return nil, "", fmt.Errorf("reading config flags: %w", err)
	}
	doltDir, err := detectDoltDir(cfg.DoltDirExpanded())
	if err != nil {
		return nil, "", err
	}
	return cfg, doltDir, nil
}

func resolveReadBranch(cmd *cobra.Command) (string, error) {
	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return "", fmt.Errorf("reading config flags: %w", err)
	}
	return cfg.EffectiveBranch(), nil
}

func withReadClient(cmd *cobra.Command, branch string, fn func(*config.Config, readClient) error) error {
	cfg, doltDir, err := loadConfigAndDoltDir(cmd)
	if err != nil {
		return err
	}

	client, err := readClientOpener(doltDir, branch)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	return fn(cfg, client)
}

func withReadClients(cmd *cobra.Command, branch1, branch2 string, fn func(*config.Config, readClient, readClient) error) error {
	cfg, doltDir, err := loadConfigAndDoltDir(cmd)
	if err != nil {
		return err
	}

	client1, err := readClientOpener(doltDir, branch1)
	if err != nil {
		return err
	}
	defer func() { _ = client1.Close() }()

	client2, err := readClientOpener(doltDir, branch2)
	if err != nil {
		return err
	}
	defer func() { _ = client2.Close() }()

	return fn(cfg, client1, client2)
}
