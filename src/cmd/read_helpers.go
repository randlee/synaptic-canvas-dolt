package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/spf13/cobra"
)

type readClient interface {
	ListPackages(ctx context.Context, opts dolt.ListOptions) ([]models.Package, error)
	GetPackage(ctx context.Context, id string) (*models.Package, error)
	GetPackageDetail(ctx context.Context, id string) (*models.Package, error)
	GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error)
	GetPackageDeps(ctx context.Context, packageID string) ([]models.PackageDep, error)
	GetPackageHooks(ctx context.Context, packageID string) ([]models.PackageHook, error)
	GetPackageQuestions(ctx context.Context, packageID string) ([]models.PackageQuestion, error)
	ResolveVariant(ctx context.Context, logicalID, agentProfile string) (string, error)
	Close() error
}

var readClientOpener = openReadClient

func openReadClient(cfg *config.Config) (readClient, error) {
	clientType := cfg.Get(config.KeyDoltClient, "http")
	switch clientType {
	case "http":
		database := cfg.Get(config.KeyDoltDatabase, "")
		if database == "" {
			return nil, fmt.Errorf("dolt.database is not configured; run: sc config set dolt.database <owner/database>")
		}
		return dolt.NewHTTPClient(dolt.HTTPConfig{
			Host:     cfg.Get(config.KeyDoltHost, "www.dolthub.com"),
			Database: database,
			Branch:   cfg.EffectiveBranch(),
			Token:    cfg.Get(config.KeyDoltToken, ""),
			Timeout:  time.Duration(cfg.GetInt(config.KeyDoltTimeout, 30)) * time.Second,
		}), nil
	case "sql":
		dsn := cfg.Get(config.KeyDoltDSN, "")
		if dsn == "" {
			return nil, fmt.Errorf("dolt.dsn is not configured; run: sc config set dolt.dsn <dsn>")
		}
		sqlCfg, err := dolt.ParseDSN(dsn)
		if err != nil {
			return nil, err
		}
		return dolt.OpenForBranch(sqlCfg, cfg.EffectiveBranch())
	case "cli":
		doltDir := config.ExpandPath(cfg.Get(config.KeyDoltDir, ""))
		if doltDir == "" {
			return nil, fmt.Errorf("dolt.dir is not configured; run: sc config set dolt.dir <path>")
		}
		return dolt.NewCLIReader(doltDir, cfg.EffectiveBranch()), nil
	default:
		return nil, fmt.Errorf("unsupported dolt.client %q", clientType)
	}
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
