//go:build integration

package dolt

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestHTTPClientLive(t *testing.T) {
	if os.Getenv("SC_RUN_LIVE_DOLTHUB") != "1" {
		t.Skip("set SC_RUN_LIVE_DOLTHUB=1 for live DoltHub HTTP API integration testing")
	}
	database := os.Getenv("SC_TEST_DOLT_DATABASE")
	if database == "" {
		t.Skip("set SC_TEST_DOLT_DATABASE=<owner/database>")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewHTTPClient(HTTPConfig{
		Host:     os.Getenv("SC_DOLT_HOST"),
		Database: database,
		Branch:   envDefault("SC_TEST_DOLT_BRANCH", "main"),
		Token:    os.Getenv("SC_DOLT_TOKEN"),
		Timeout:  30 * time.Second,
	})
	if _, err := client.ListPackages(ctx, ListOptions{}); err != nil {
		t.Fatalf("ListPackages() live HTTP error = %v", err)
	}
}

func TestSQLClientLive(t *testing.T) {
	if os.Getenv("SC_RUN_SQL_DOLT") != "1" {
		t.Skip("set SC_RUN_SQL_DOLT=1 for live Dolt SQL server integration testing")
	}
	dsn := os.Getenv("SC_TEST_DOLT_DSN")
	if dsn == "" {
		t.Skip("set SC_TEST_DOLT_DSN, for example root:@tcp(127.0.0.1:3306)/synaptic_canvas?parseTime=true")
	}

	cfg, err := ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN() error = %v", err)
	}
	client, err := OpenForBranch(cfg, envDefault("SC_TEST_DOLT_BRANCH", "main"))
	if err != nil {
		t.Fatalf("OpenForBranch() live SQL error = %v", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := client.ListPackages(ctx, ListOptions{}); err != nil {
		t.Fatalf("ListPackages() live SQL error = %v", err)
	}
}

func TestCLIReaderLive(t *testing.T) {
	if os.Getenv("SC_RUN_CLI_DOLT") != "1" {
		t.Skip("set SC_RUN_CLI_DOLT=1 for live Dolt CLI integration testing")
	}
	if _, err := exec.LookPath("dolt"); err != nil {
		t.Skip("dolt CLI not found in PATH")
	}
	doltDir := os.Getenv("SC_TEST_DOLT_DIR")
	if doltDir == "" {
		t.Skip("set SC_TEST_DOLT_DIR to a local Dolt database clone")
	}

	reader := NewCLIReader(doltDir, envDefault("SC_TEST_DOLT_BRANCH", "main"))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := reader.ListPackages(ctx, ListOptions{}); err != nil {
		t.Fatalf("ListPackages() live CLI error = %v", err)
	}
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
