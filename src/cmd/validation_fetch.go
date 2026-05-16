package cmd

import (
	"context"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
)

var validateCatalogFetch = defaultValidateCatalogFetch

func defaultValidateCatalogFetch(ctx context.Context, _ string, branch string) ([]catalog.CatalogEntry, error) {
	cfg := &config.Config{Branch: branch}
	if err := cfg.LoadFileConfig(); err != nil {
		return nil, err
	}
	client, err := readClientOpener(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	return client.GetPackageCatalog(ctx)
}
