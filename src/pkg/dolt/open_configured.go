package dolt

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
)

func DetectDoltDir(configured string) (string, error) {
	if configured != "" {
		expanded := config.ExpandPath(configured)
		if _, err := os.Stat(filepath.Join(expanded, ".dolt")); err != nil {
			return "", fmt.Errorf("invalid dolt.dir %q: %w", expanded, err)
		}
		return expanded, nil
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
	return "", fmt.Errorf("could not auto-detect Dolt database directory; pass --dolt-dir")
}

func OpenConfiguredReadClient(cfg *config.Config, branch string) (Client, error) {
	selection, err := cfg.ResolveDoltClient()
	if err != nil {
		return nil, err
	}
	if branch == "" {
		branch = cfg.EffectiveBranch()
	}

	switch selection.Client {
	case "http":
		database := cfg.Get(config.KeyDoltDatabase, "")
		if database == "" {
			return nil, fmt.Errorf("dolt.database is not configured; run: sc config set dolt.database <owner/database>")
		}
		return NewHTTPClient(HTTPConfig{
			Host:     cfg.Get(config.KeyDoltHost, "www.dolthub.com"),
			Database: database,
			Branch:   branch,
			Token:    cfg.Get(config.KeyDoltToken, ""),
			Timeout:  time.Duration(cfg.GetInt(config.KeyDoltTimeout, 30)) * time.Second,
		}), nil
	case "sql":
		dsn := cfg.Get(config.KeyDoltDSN, "")
		if dsn == "" {
			return nil, fmt.Errorf("dolt.dsn is not configured; run: sc config set dolt.dsn <dsn>")
		}
		sqlCfg, err := ParseDSN(dsn)
		if err != nil {
			return nil, err
		}
		return OpenForBranch(sqlCfg, branch)
	case "cli":
		doltDir, err := DetectDoltDir(selection.DoltDir)
		if err != nil {
			return nil, err
		}
		return NewCLIReader(doltDir, branch), nil
	default:
		return nil, fmt.Errorf("%w: unsupported dolt.client %q", ErrUnsupportedBackend, selection.Client)
	}
}

func ValidateWriteClient(cfg *config.Config) error {
	selection, err := cfg.ResolveDoltClient()
	if err != nil {
		return err
	}
	if selection.Client == "http" {
		return fmt.Errorf("%w: admin write commands require client sql or cli; effective client is http", ErrUnsupportedBackend)
	}
	return nil
}
