package admin

import (
	"context"
	"os"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
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

type readClient interface {
	GetPackage(context.Context, string) (*models.Package, error)
	GetPackageFiles(context.Context, string) ([]models.PackageFile, error)
	GetPackageDeps(context.Context, string) ([]models.PackageDep, error)
	GetPackageHooks(context.Context, string) ([]models.PackageHook, error)
	GetPackageQuestions(context.Context, string) ([]models.PackageQuestion, error)
	Close() error
}

func openReadClient(doltDir string, branch string) (readClient, error) {
	if doltDir != "" {
		return dolt.NewCLIReader(doltDir, branch), nil
	}
	cfg := dolt.DefaultConfig()
	return dolt.OpenForBranch(cfg, branch)
}
