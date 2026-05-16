//go:build !production

package dolttest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

// ClientHarness exposes deterministic transport fixtures for cross-package
// command contract tests.
type ClientHarness struct {
	Name string
	Open func(*testing.T) dolt.Client
}

// ConformanceHarnesses returns one deterministic success harness per backend.
func ConformanceHarnesses() []ClientHarness {
	return []ClientHarness{
		{Name: "http", Open: openHarnessHTTPClient},
		{Name: "sql", Open: openHarnessSQLClient},
		{Name: "cli", Open: openHarnessCLIClient},
	}
}

// FailingHarnesses returns one deterministic failing harness per backend.
func FailingHarnesses() []ClientHarness {
	return []ClientHarness{
		{Name: "http", Open: openHarnessHTTPFailingClient},
		{Name: "sql", Open: openHarnessSQLFailingClient},
		{Name: "cli", Open: openHarnessCLIFailingClient},
	}
}

func openHarnessHTTPClient(t *testing.T) dolt.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query_execution_status": "Success",
			"schema":                 []map[string]string{{"columnName": "id", "columnType": "text"}},
			"rows":                   harnessRowMaps(t, query),
		})
	}))
	t.Cleanup(server.Close)
	return dolt.NewHTTPClient(dolt.HTTPConfig{Host: server.URL, Database: "owner/repo", Branch: "main"})
}

func openHarnessHTTPFailingClient(t *testing.T) dolt.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return dolt.NewHTTPClient(dolt.HTTPConfig{Host: server.URL, Database: "owner/repo", Branch: "main"})
}

func openHarnessSQLClient(t *testing.T) dolt.Client {
	t.Helper()
	db := openHarnessFakeSQLDB(t, func(query string, args []driver.NamedValue) (harnessFakeSQLRows, error) {
		return harnessSQLRows(t, query, args), nil
	})
	return dolt.NewSQLClient(db, "owner/repo", "main")
}

func openHarnessSQLFailingClient(t *testing.T) dolt.Client {
	t.Helper()
	db := openHarnessFakeSQLDB(t, func(string, []driver.NamedValue) (harnessFakeSQLRows, error) {
		return harnessFakeSQLRows{}, dolt.ErrServerError
	})
	return dolt.NewSQLClient(db, "owner/repo", "main")
}

func openHarnessCLIClient(t *testing.T) dolt.Client {
	t.Helper()
	doltDir := buildFakeDoltBinary(t, false)
	return dolt.NewCLIReader(doltDir, "main")
}

func openHarnessCLIFailingClient(t *testing.T) dolt.Client {
	t.Helper()
	return failingClient{}
}

func buildFakeDoltBinary(t *testing.T, failing bool) string {
	t.Helper()
	root := t.TempDir()
	srcPath := filepath.Join(root, "main.go")
	if err := os.WriteFile(srcPath, []byte(fakeDoltSource), 0o600); err != nil {
		t.Fatalf("WriteFile(main.go) error = %v", err)
	}
	binName := "dolt"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(root, binName)
	cmd := exec.Command("go", "build", "-o", binPath, srcPath) //nolint:gosec // test harness builds a controlled temp helper binary from generated source.
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build fake dolt error = %v\n%s", err, string(out))
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	if failing {
		t.Setenv("DOLTTEST_FAIL", "1")
	} else {
		t.Setenv("DOLTTEST_FAIL", "")
	}
	return t.TempDir()
}

func harnessRowMaps(t *testing.T, query string) []map[string]any {
	t.Helper()
	switch {
	case strings.Contains(query, "ORDER BY p.name"):
		return []map[string]any{{"id": "team-lead", "name": "Team Lead", "version": "1.2.3", "description": "Workflow package", "agent_variant": "claude", "tags": "go,workflow", "install_scope": "any", "sha256": "pkg-sha", "file_count": 1, "dep_count": 1, "dependency_count": 1}}
	case strings.Contains(query, "WHERE p.id = 'team-lead'"):
		return []map[string]any{{"id": "team-lead", "name": "Team Lead", "version": "1.2.3", "description": "Workflow package", "agent_variant": "claude", "author": "Randlee", "license": "MIT", "tags": "go,workflow", "install_scope": "any", "variables": map[string]any{"repo": "name"}, "options": map[string]any{"mode": "strict"}, "sha256": "pkg-sha", "min_claude_version": "1.0.0", "file_count": 1, "dep_count": 1}}
	case strings.Contains(query, "SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM packages WHERE id = 'team-lead'"):
		return []map[string]any{{"id": "team-lead", "name": "Team Lead", "version": "1.2.3", "description": "Workflow package", "agent_variant": "claude", "author": "Randlee", "license": "MIT", "tags": "go,workflow", "install_scope": "any", "variables": map[string]any{"repo": "name"}, "options": map[string]any{"mode": "strict"}, "sha256": "pkg-sha", "min_claude_version": "1.0.0"}}
	case strings.Contains(query, "SELECT id, name, version, description, agent_variant, tags, install_scope, sha256 FROM packages ORDER BY name"):
		return []map[string]any{{"id": "team-lead", "name": "Team Lead", "version": "1.2.3", "description": "Workflow package", "agent_variant": "claude", "tags": "go,workflow", "install_scope": "any", "sha256": "pkg-sha"}}
	case strings.Contains(query, "package_variants") && strings.Contains(query, "variant_package_id"):
		return []map[string]any{{"variant_package_id": "team-lead-codex"}}
	case strings.Contains(query, "package_questions") && strings.Contains(query, "question_id"):
		return []map[string]any{{"package_id": "team-lead", "question_id": "repo_name", "prompt": "Repo name", "type": "text", "default_val": "demo", "choices": "", "sort_order": 1}}
	case strings.Contains(query, "package_hooks") && strings.Contains(query, "script_path"):
		return []map[string]any{{"package_id": "team-lead", "event": "PreToolUse", "matcher": "git commit", "script_path": "hooks/pre-commit.sh", "priority": 5, "blocking": true}}
	case strings.Contains(query, "package_deps") && strings.Contains(query, "dep_name"):
		return []map[string]any{{"package_id": "team-lead", "dep_type": "tool", "dep_name": "gh", "dep_spec": ">=2.0.0", "install_cmd": "brew install gh", "cmd_sha256": "cmd-sha"}}
	case strings.Contains(query, "FROM package_files AS f"):
		return []map[string]any{{"package_id": "team-lead", "version": "1.2.3", "doc_path": "skills/team-lead/SKILL.md", "sha256": "sha-skill"}}
	case strings.Contains(query, "FROM package_files"):
		return []map[string]any{{"package_id": "team-lead", "dest_path": "skills/team-lead/SKILL.md", "content": "# Team Lead", "sha256": "sha-skill", "file_type": "skill", "content_type": "markdown", "is_template": 0, "frontmatter": map[string]any{"name": "team-lead"}, "fm_name": "Team Lead", "fm_version": "1.2.3"}}
	default:
		t.Fatalf("unexpected query: %s", query)
		return nil
	}
}

func harnessSQLRows(t *testing.T, query string, args []driver.NamedValue) harnessFakeSQLRows {
	t.Helper()
	switch {
	case strings.Contains(query, "COUNT(*) AS file_count") && strings.Contains(query, "ORDER BY p.name"):
		return makeHarnessFakeSQLRows([]string{"id", "name", "version", "description", "agent_variant", "tags", "install_scope", "sha256", "file_count", "dep_count"}, [][]driver.Value{{"team-lead", "Team Lead", "1.2.3", "Workflow package", "claude", "go,workflow", "any", "pkg-sha", int64(1), int64(1)}})
	case strings.Contains(query, "COUNT(*) AS file_count") && strings.Contains(query, "WHERE p.id = ?"):
		if len(args) != 1 || args[0].Value != "team-lead" {
			t.Fatalf("unexpected args for package detail: %+v", args)
		}
		return makeHarnessFakeSQLRows([]string{"id", "name", "version", "description", "agent_variant", "author", "license", "tags", "install_scope", "variables", "options", "sha256", "min_claude_version", "file_count", "dep_count"}, [][]driver.Value{{"team-lead", "Team Lead", "1.2.3", "Workflow package", "claude", "Randlee", "MIT", "go,workflow", "any", []byte(`{"repo":"name"}`), []byte(`{"mode":"strict"}`), "pkg-sha", "1.0.0", int64(1), int64(1)}})
	case strings.Contains(query, "SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM") && strings.Contains(query, "WHERE id = ?"):
		return makeHarnessFakeSQLRows([]string{"id", "name", "version", "description", "agent_variant", "author", "license", "tags", "install_scope", "variables", "options", "sha256", "min_claude_version"}, [][]driver.Value{{"team-lead", "Team Lead", "1.2.3", "Workflow package", "claude", "Randlee", "MIT", "go,workflow", "any", []byte(`{"repo":"name"}`), []byte(`{"mode":"strict"}`), "pkg-sha", "1.0.0"}})
	case strings.Contains(query, "package_variants") && strings.Contains(query, "variant_package_id"):
		return makeHarnessFakeSQLRows([]string{"variant_package_id"}, [][]driver.Value{{"team-lead-codex"}})
	case strings.Contains(query, "package_questions") && strings.Contains(query, "question_id"):
		return makeHarnessFakeSQLRows([]string{"package_id", "question_id", "prompt", "type", "default_val", "choices", "sort_order"}, [][]driver.Value{{"team-lead", "repo_name", "Repo name", "text", "demo", "", int64(1)}})
	case strings.Contains(query, "package_hooks") && strings.Contains(query, "script_path"):
		return makeHarnessFakeSQLRows([]string{"package_id", "event", "matcher", "script_path", "priority", "blocking"}, [][]driver.Value{{"team-lead", "PreToolUse", "git commit", "hooks/pre-commit.sh", int64(5), true}})
	case strings.Contains(query, "package_deps") && strings.Contains(query, "dep_name"):
		return makeHarnessFakeSQLRows([]string{"package_id", "dep_type", "dep_name", "dep_spec", "install_cmd", "cmd_sha256"}, [][]driver.Value{{"team-lead", "tool", "gh", ">=2.0.0", "brew install gh", "cmd-sha"}})
	case strings.Contains(query, "package_files AS f") && strings.Contains(query, "doc_path"):
		return makeHarnessFakeSQLRows([]string{"package_id", "version", "doc_path", "sha256"}, [][]driver.Value{{"team-lead", "1.2.3", "skills/team-lead/SKILL.md", "sha-skill"}})
	case strings.Contains(query, "package_files") && strings.Contains(query, "dest_path") && strings.Contains(query, "ORDER BY dest_path"):
		return makeHarnessFakeSQLRows([]string{"package_id", "dest_path", "content", "sha256", "file_type", "content_type", "is_template", "frontmatter", "fm_name", "fm_description", "fm_version", "fm_model"}, [][]driver.Value{{"team-lead", "skills/team-lead/SKILL.md", "# Team Lead", "sha-skill", "skill", "markdown", false, []byte(`{"name":"team-lead"}`), "Team Lead", nil, "1.2.3", nil}})
	default:
		t.Fatalf("unexpected SQL query: %s args=%+v", query, args)
		return harnessFakeSQLRows{}
	}
}

type harnessFakeSQLRows struct {
	columns []string
	rows    [][]driver.Value
}

type harnessFakeSQLDriver struct{}
type harnessFakeSQLConn struct{}
type harnessFakeSQLResultRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

var (
	harnessFakeSQLMu      sync.Mutex
	harnessFakeSQLHandler func(string, []driver.NamedValue) (harnessFakeSQLRows, error)
)

func init() {
	sql.Register("synaptic-canvas-dolttest-fake", harnessFakeSQLDriver{})
}

func openHarnessFakeSQLDB(t *testing.T, handler func(string, []driver.NamedValue) (harnessFakeSQLRows, error)) *sql.DB {
	t.Helper()
	harnessFakeSQLMu.Lock()
	harnessFakeSQLHandler = handler
	harnessFakeSQLMu.Unlock()
	t.Cleanup(func() {
		harnessFakeSQLMu.Lock()
		harnessFakeSQLHandler = nil
		harnessFakeSQLMu.Unlock()
	})
	db, err := sql.Open("synaptic-canvas-dolttest-fake", "ignored")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func makeHarnessFakeSQLRows(columns []string, rows [][]driver.Value) harnessFakeSQLRows {
	return harnessFakeSQLRows{columns: columns, rows: rows}
}

func (harnessFakeSQLDriver) Open(string) (driver.Conn, error) { return harnessFakeSQLConn{}, nil }
func (harnessFakeSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (harnessFakeSQLConn) Close() error              { return nil }
func (harnessFakeSQLConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }

func (harnessFakeSQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	harnessFakeSQLMu.Lock()
	handler := harnessFakeSQLHandler
	harnessFakeSQLMu.Unlock()
	if handler == nil {
		return nil, errors.New("fake SQL handler not configured")
	}
	rows, err := handler(query, args)
	if err != nil {
		return nil, err
	}
	return &harnessFakeSQLResultRows{columns: rows.columns, rows: rows.rows}, nil
}

func (harnessFakeSQLConn) CheckNamedValue(*driver.NamedValue) error { return nil }
func (r *harnessFakeSQLResultRows) Columns() []string               { return r.columns }
func (r *harnessFakeSQLResultRows) Close() error                    { return nil }
func (r *harnessFakeSQLResultRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

var _ driver.QueryerContext = harnessFakeSQLConn{}
var _ driver.NamedValueChecker = harnessFakeSQLConn{}

type failingClient struct{}

func (failingClient) Close() error { return nil }

func (failingClient) ListPackages(context.Context, dolt.ListOptions) ([]models.Package, error) {
	return nil, dolt.ErrServerError
}

func (failingClient) GetPackage(context.Context, string) (*models.Package, error) {
	return nil, dolt.ErrServerError
}

func (failingClient) GetPackageDetail(context.Context, string) (*models.Package, error) {
	return nil, dolt.ErrServerError
}

func (failingClient) GetPackageFiles(context.Context, string) ([]models.PackageFile, error) {
	return nil, dolt.ErrServerError
}

func (failingClient) GetPackageFileSHAs(context.Context, string, string) ([]dolt.PackageFileSHA, error) {
	return nil, dolt.ErrServerError
}

func (failingClient) GetPackageDeps(context.Context, string) ([]models.PackageDep, error) {
	return nil, dolt.ErrServerError
}

func (failingClient) GetPackageHooks(context.Context, string) ([]models.PackageHook, error) {
	return nil, dolt.ErrServerError
}

func (failingClient) GetPackageQuestions(context.Context, string) ([]models.PackageQuestion, error) {
	return nil, dolt.ErrServerError
}

func (failingClient) ResolveVariant(context.Context, string, string) (string, error) {
	return "", dolt.ErrServerError
}

func (failingClient) GetPackageCatalog(context.Context) ([]catalog.CatalogEntry, error) {
	return nil, dolt.ErrServerError
}

const fakeDoltSource = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	if os.Getenv("DOLTTEST_FAIL") == "1" {
		fmt.Fprintln(os.Stderr, "server unavailable")
		os.Exit(1)
	}
	query := ""
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "-q" && i+1 < len(os.Args) {
			query = os.Args[i+1]
			break
		}
	}
	rows := rowMaps(query)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"rows": rows})
}

func rowMaps(query string) []map[string]any {
	switch {
	case strings.Contains(query, "ORDER BY p.name"):
		return []map[string]any{{"id": "team-lead", "name": "Team Lead", "version": "1.2.3", "description": "Workflow package", "agent_variant": "claude", "tags": "go,workflow", "install_scope": "any", "sha256": "pkg-sha", "file_count": 1, "dep_count": 1, "dependency_count": 1}}
	case strings.Contains(query, "WHERE p.id = 'team-lead'"):
		return []map[string]any{{"id": "team-lead", "name": "Team Lead", "version": "1.2.3", "description": "Workflow package", "agent_variant": "claude", "author": "Randlee", "license": "MIT", "tags": "go,workflow", "install_scope": "any", "variables": map[string]any{"repo": "name"}, "options": map[string]any{"mode": "strict"}, "sha256": "pkg-sha", "min_claude_version": "1.0.0", "file_count": 1, "dep_count": 1}}
	case strings.Contains(query, "SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM packages WHERE id = 'team-lead'"):
		return []map[string]any{{"id": "team-lead", "name": "Team Lead", "version": "1.2.3", "description": "Workflow package", "agent_variant": "claude", "author": "Randlee", "license": "MIT", "tags": "go,workflow", "install_scope": "any", "variables": map[string]any{"repo": "name"}, "options": map[string]any{"mode": "strict"}, "sha256": "pkg-sha", "min_claude_version": "1.0.0"}}
	case strings.Contains(query, "SELECT id, name, version, description, agent_variant, tags, install_scope, sha256 FROM packages ORDER BY name"):
		return []map[string]any{{"id": "team-lead", "name": "Team Lead", "version": "1.2.3", "description": "Workflow package", "agent_variant": "claude", "tags": "go,workflow", "install_scope": "any", "sha256": "pkg-sha"}}
	case strings.Contains(query, "package_variants") && strings.Contains(query, "variant_package_id"):
		return []map[string]any{{"variant_package_id": "team-lead-codex"}}
	case strings.Contains(query, "package_questions") && strings.Contains(query, "question_id"):
		return []map[string]any{{"package_id": "team-lead", "question_id": "repo_name", "prompt": "Repo name", "type": "text", "default_val": "demo", "choices": "", "sort_order": 1}}
	case strings.Contains(query, "package_hooks") && strings.Contains(query, "script_path"):
		return []map[string]any{{"package_id": "team-lead", "event": "PreToolUse", "matcher": "git commit", "script_path": "hooks/pre-commit.sh", "priority": 5, "blocking": true}}
	case strings.Contains(query, "package_deps") && strings.Contains(query, "dep_name"):
		return []map[string]any{{"package_id": "team-lead", "dep_type": "tool", "dep_name": "gh", "dep_spec": ">=2.0.0", "install_cmd": "brew install gh", "cmd_sha256": "cmd-sha"}}
	case strings.Contains(query, "FROM package_files AS f"):
		return []map[string]any{{"package_id": "team-lead", "version": "1.2.3", "doc_path": "skills/team-lead/SKILL.md", "sha256": "sha-skill"}}
	case strings.Contains(query, "FROM package_files"):
		return []map[string]any{{"package_id": "team-lead", "dest_path": "skills/team-lead/SKILL.md", "content": "# Team Lead", "sha256": "sha-skill", "file_type": "skill", "content_type": "markdown", "is_template": 0, "frontmatter": map[string]any{"name": "team-lead"}, "fm_name": "Team Lead", "fm_version": "1.2.3"}}
	default:
		fmt.Fprintf(os.Stderr, "unexpected query: %s\n", query)
		os.Exit(1)
		return nil
	}
}
`
