// Package dolt provides a database abstraction layer for querying the
// Synaptic Canvas Dolt database. It uses database/sql with the MySQL
// driver since Dolt exposes a MySQL-compatible SQL interface.
package dolt

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	// MySQL driver for database/sql — Dolt exposes a MySQL-compatible interface.
	_ "github.com/go-sql-driver/mysql"

	"github.com/randlee/synaptic-canvas/pkg/models"
)

// ListOptions controls filtering and pagination for list operations.
type ListOptions struct {
	// Branch specifies the Dolt branch (channel) to query.
	// Empty string means use the current/default branch.
	Branch string
}

// Client defines the interface for querying the Synaptic Canvas Dolt database.
// All methods accept a context for cancellation and timeout support.
type Client interface {
	// ListPackages returns all packages, optionally filtered by branch.
	ListPackages(ctx context.Context, opts ListOptions) ([]models.Package, error)

	// GetPackage retrieves a single package by ID.
	GetPackage(ctx context.Context, id string) (*models.Package, error)

	// GetPackageFiles retrieves all files belonging to a package.
	GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error)

	// GetPackageDeps retrieves all dependencies for a package.
	GetPackageDeps(ctx context.Context, packageID string) ([]models.PackageDep, error)

	// GetPackageHooks retrieves all hooks for a package.
	GetPackageHooks(ctx context.Context, packageID string) ([]models.PackageHook, error)

	// GetPackageQuestions retrieves all questions for a package.
	GetPackageQuestions(ctx context.Context, packageID string) ([]models.PackageQuestion, error)

	// ResolveVariant resolves a logical package ID and agent profile to a
	// concrete variant package ID. Returns empty string if no variant exists.
	ResolveVariant(ctx context.Context, logicalID, agentProfile string) (string, error)

	// Close releases database resources.
	Close() error
}

// SQLClient implements Client using database/sql with a MySQL-compatible driver.
type SQLClient struct {
	db       *sql.DB
	database string
}

// Config holds connection parameters for the Dolt SQL server.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string //nolint:gosec // Not a hardcoded credential; holds runtime config.
	Database string
}

// DefaultConfig returns a Config with Dolt's default local settings.
func DefaultConfig() Config {
	return Config{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "",
		Database: "synaptic_canvas",
	}
}

// DSN returns the MySQL-format data source name for the configuration.
func (c Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

// NewSQLClient creates a new SQLClient connected to the Dolt SQL server.
// The caller must call Close() when done.
func NewSQLClient(db *sql.DB, database string) *SQLClient {
	return &SQLClient{db: db, database: database}
}

// Open creates a new SQLClient by opening a database connection using the
// provided Config. The caller must call Close() when done.
func Open(cfg Config) (*SQLClient, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("opening dolt connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging dolt server: %w", err)
	}
	return NewSQLClient(db, cfg.Database), nil
}

// Close releases the database connection.
func (c *SQLClient) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

// switchBranch executes a USE statement to switch to the specified Dolt branch.
// If branch is empty, this is a no-op.
func (c *SQLClient) switchBranch(ctx context.Context, branch string) error {
	stmt := UseBranchQuery(c.database, branch)
	if stmt == "" {
		return nil
	}
	slog.Debug("switching dolt branch", "branch", branch)
	if _, err := c.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("switching to branch %q: %w", branch, err)
	}
	return nil
}

// ListPackages returns all packages, optionally filtered by branch.
func (c *SQLClient) ListPackages(ctx context.Context, opts ListOptions) ([]models.Package, error) {
	if err := c.switchBranch(ctx, opts.Branch); err != nil {
		return nil, err
	}

	slog.Debug("listing packages", "branch", opts.Branch)
	rows, err := c.db.QueryContext(ctx, ListPackagesQuery())
	if err != nil {
		return nil, fmt.Errorf("listing packages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var packages []models.Package
	for rows.Next() {
		var p models.Package
		if err := rows.Scan(&p.ID, &p.Name, &p.Version, &p.Description, &p.Tags, &p.InstallScope); err != nil {
			return nil, fmt.Errorf("scanning package row: %w", err)
		}
		packages = append(packages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating packages: %w", err)
	}
	slog.Debug("listed packages", "count", len(packages))
	return packages, nil
}

// GetPackage retrieves a single package by ID.
func (c *SQLClient) GetPackage(ctx context.Context, id string) (*models.Package, error) {
	slog.Debug("getting package", "id", id)
	var p models.Package
	err := c.db.QueryRowContext(ctx, GetPackageQuery(), id).Scan(
		&p.ID, &p.Name, &p.Version, &p.Description, &p.AgentVariant,
		&p.Author, &p.License, &p.Tags, &p.InstallScope,
		&p.Variables, &p.Options, &p.MinClaudeVer,
	)
	if err == sql.ErrNoRows {
		slog.Debug("package not found", "id", id)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting package %q: %w", id, err)
	}
	return &p, nil
}

// GetPackageFiles retrieves all files belonging to a package.
func (c *SQLClient) GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error) {
	slog.Debug("getting package files", "package_id", packageID)
	rows, err := c.db.QueryContext(ctx, GetPackageFilesQuery(), packageID)
	if err != nil {
		return nil, fmt.Errorf("getting files for package %q: %w", packageID, err)
	}
	defer func() { _ = rows.Close() }()

	var files []models.PackageFile
	for rows.Next() {
		var f models.PackageFile
		if err := rows.Scan(
			&f.PackageID, &f.DestPath, &f.Content, &f.SHA256,
			&f.FileType, &f.ContentType, &f.IsTemplate, &f.Frontmatter,
			&f.FMName, &f.FMDescription, &f.FMVersion, &f.FMModel,
		); err != nil {
			return nil, fmt.Errorf("scanning file row: %w", err)
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating files: %w", err)
	}
	slog.Debug("got package files", "package_id", packageID, "count", len(files))
	return files, nil
}

// GetPackageDeps retrieves all dependencies for a package.
func (c *SQLClient) GetPackageDeps(ctx context.Context, packageID string) ([]models.PackageDep, error) {
	slog.Debug("getting package deps", "package_id", packageID)
	rows, err := c.db.QueryContext(ctx, GetPackageDepsQuery(), packageID)
	if err != nil {
		return nil, fmt.Errorf("getting deps for package %q: %w", packageID, err)
	}
	defer func() { _ = rows.Close() }()

	var deps []models.PackageDep
	for rows.Next() {
		var d models.PackageDep
		if err := rows.Scan(
			&d.PackageID, &d.DepType, &d.DepName,
			&d.DepSpec, &d.InstallCmd, &d.CmdSHA256,
		); err != nil {
			return nil, fmt.Errorf("scanning dep row: %w", err)
		}
		deps = append(deps, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating deps: %w", err)
	}
	slog.Debug("got package deps", "package_id", packageID, "count", len(deps))
	return deps, nil
}

// GetPackageHooks retrieves all hooks for a package.
func (c *SQLClient) GetPackageHooks(ctx context.Context, packageID string) ([]models.PackageHook, error) {
	slog.Debug("getting package hooks", "package_id", packageID)
	rows, err := c.db.QueryContext(ctx, GetPackageHooksQuery(), packageID)
	if err != nil {
		return nil, fmt.Errorf("getting hooks for package %q: %w", packageID, err)
	}
	defer func() { _ = rows.Close() }()

	var hooks []models.PackageHook
	for rows.Next() {
		var h models.PackageHook
		if err := rows.Scan(
			&h.PackageID, &h.Event, &h.Matcher,
			&h.ScriptPath, &h.Priority, &h.Blocking,
		); err != nil {
			return nil, fmt.Errorf("scanning hook row: %w", err)
		}
		hooks = append(hooks, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating hooks: %w", err)
	}
	slog.Debug("got package hooks", "package_id", packageID, "count", len(hooks))
	return hooks, nil
}

// GetPackageQuestions retrieves all questions for a package.
func (c *SQLClient) GetPackageQuestions(ctx context.Context, packageID string) ([]models.PackageQuestion, error) {
	slog.Debug("getting package questions", "package_id", packageID)
	rows, err := c.db.QueryContext(ctx, GetPackageQuestionsQuery(), packageID)
	if err != nil {
		return nil, fmt.Errorf("getting questions for package %q: %w", packageID, err)
	}
	defer func() { _ = rows.Close() }()

	var questions []models.PackageQuestion
	for rows.Next() {
		var q models.PackageQuestion
		if err := rows.Scan(
			&q.PackageID, &q.QuestionID, &q.Prompt, &q.Type,
			&q.DefaultVal, &q.Choices, &q.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("scanning question row: %w", err)
		}
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating questions: %w", err)
	}
	slog.Debug("got package questions", "package_id", packageID, "count", len(questions))
	return questions, nil
}

// ResolveVariant resolves a logical package ID and agent profile to a
// concrete variant package ID. Returns empty string if no variant exists.
func (c *SQLClient) ResolveVariant(ctx context.Context, logicalID, agentProfile string) (string, error) {
	slog.Debug("resolving variant", "logical_id", logicalID, "agent_profile", agentProfile)
	var variantID string
	err := c.db.QueryRowContext(ctx, ResolveVariantQuery(), logicalID, agentProfile).Scan(&variantID)
	if err == sql.ErrNoRows {
		slog.Debug("variant not found", "logical_id", logicalID, "agent_profile", agentProfile)
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("resolving variant %q/%q: %w", logicalID, agentProfile, err)
	}
	return variantID, nil
}
