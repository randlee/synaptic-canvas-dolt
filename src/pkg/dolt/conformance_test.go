package dolt

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
)

func TestClientConformanceAcrossTransports(t *testing.T) {
	ctx := context.Background()
	for _, harness := range conformanceHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			client := harness.open(t)
			defer func() { _ = client.Close() }()

			packages, err := client.ListPackages(ctx, ListOptions{Tags: []string{"go"}})
			if err != nil {
				t.Fatalf("ListPackages() error = %v", err)
			}
			if len(packages) != 1 || packages[0].ID != "team-lead" || packages[0].FileCount != 1 || packages[0].DepCount != 1 {
				t.Fatalf("unexpected packages: %+v", packages)
			}

			pkg, err := client.GetPackageDetail(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageDetail() error = %v", err)
			}
			if pkg == nil || pkg.Name != "Team Lead" || pkg.Version != "1.2.3" || pkg.FileCount != 1 || pkg.DepCount != 1 {
				t.Fatalf("unexpected package detail: %+v", pkg)
			}

			files, err := client.GetPackageFiles(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageFiles() error = %v", err)
			}
			if len(files) != 1 || files[0].DestPath != "skills/team-lead/SKILL.md" {
				t.Fatalf("unexpected files: %+v", files)
			}

			deps, err := client.GetPackageDeps(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageDeps() error = %v", err)
			}
			if len(deps) != 1 || deps[0].DepName != "gh" {
				t.Fatalf("unexpected deps: %+v", deps)
			}

			hooks, err := client.GetPackageHooks(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageHooks() error = %v", err)
			}
			if len(hooks) != 1 || hooks[0].ScriptPath != "hooks/pre-commit.sh" || hooks[0].Priority != 5 {
				t.Fatalf("unexpected hooks: %+v", hooks)
			}

			questions, err := client.GetPackageQuestions(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageQuestions() error = %v", err)
			}
			if len(questions) != 1 || questions[0].QuestionID != "repo_name" || questions[0].SortOrder != 1 {
				t.Fatalf("unexpected questions: %+v", questions)
			}

			variant, err := client.ResolveVariant(ctx, "team-lead", "codex")
			if err != nil {
				t.Fatalf("ResolveVariant() error = %v", err)
			}
			if variant != "team-lead-codex" {
				t.Fatalf("variant = %q, want team-lead-codex", variant)
			}

			catalogRows, err := client.GetPackageCatalog(ctx)
			if err != nil {
				t.Fatalf("GetPackageCatalog() error = %v", err)
			}
			if len(catalogRows) != 1 || catalogRows[0].DocPath != "skills/team-lead/SKILL.md" || catalogRows[0].SHA256 != "sha-skill" {
				t.Fatalf("unexpected catalog rows: %+v", catalogRows)
			}
		})
	}
}

func TestClientConformanceUnavailableAcrossTransports(t *testing.T) {
	ctx := context.Background()
	for _, harness := range failingHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			client := harness.open(t)
			defer func() { _ = client.Close() }()

			_, err := client.ListPackages(ctx, ListOptions{})
			if !errors.Is(err, ErrServerError) {
				t.Fatalf("error = %v, want ErrServerError", err)
			}
		})
	}
}

type clientHarness struct {
	name string
	open func(*testing.T) Client
}

func conformanceHarnesses() []clientHarness {
	return []clientHarness{
		{name: "http", open: newHTTPConformanceClient},
		{name: "sql", open: newSQLConformanceClient},
		{name: "cli", open: newCLIConformanceClient},
	}
}

func failingHarnesses() []clientHarness {
	return []clientHarness{
		{name: "http", open: newHTTPFailingClient},
		{name: "sql", open: newSQLFailingClient},
		{name: "cli", open: newCLIFailingClient},
	}
}

func newHTTPConformanceClient(t *testing.T) Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query_execution_status": "Success",
			"schema":                 []map[string]string{{"columnName": "id", "columnType": "text"}},
			"rows":                   conformanceRowMaps(t, query),
		})
	}))
	t.Cleanup(server.Close)
	return NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo", Branch: "main"})
}

func newHTTPFailingClient(t *testing.T) Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo", Branch: "main"})
}

func newSQLConformanceClient(t *testing.T) Client {
	t.Helper()
	db := openFakeSQLDB(t, func(query string, args []driver.NamedValue) (fakeSQLRows, error) {
		return conformanceSQLRows(t, query, args), nil
	})
	return NewSQLClient(db, "owner/repo", "main")
}

func newSQLFailingClient(t *testing.T) Client {
	t.Helper()
	db := openFakeSQLDB(t, func(string, []driver.NamedValue) (fakeSQLRows, error) {
		return fakeSQLRows{}, ErrServerError
	})
	return NewSQLClient(db, "owner/repo", "main")
}

func newCLIConformanceClient(t *testing.T) Client {
	t.Helper()
	prev := cliCommandRunner
	cliCommandRunner = func(_ context.Context, _ string, _ string, query string) cliCommandResult {
		data, _ := json.Marshal(map[string]any{"rows": conformanceRowMaps(t, query)})
		return cliCommandResult{Stdout: data}
	}
	t.Cleanup(func() { cliCommandRunner = prev })
	return NewCLIReader(t.TempDir(), "main")
}

func newCLIFailingClient(t *testing.T) Client {
	t.Helper()
	prev := cliCommandRunner
	cliCommandRunner = func(_ context.Context, _ string, _ string, _ string) cliCommandResult {
		return cliCommandResult{Err: ErrServerError, Stderr: []byte("server unavailable")}
	}
	t.Cleanup(func() { cliCommandRunner = prev })
	return NewCLIReader(t.TempDir(), "main")
}

func conformanceRowMaps(t *testing.T, query string) []map[string]any {
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

func conformanceSQLRows(t *testing.T, query string, args []driver.NamedValue) fakeSQLRows {
	t.Helper()
	switch {
	case strings.Contains(query, "COUNT(*) AS file_count") && strings.Contains(query, "ORDER BY p.name"):
		return makeFakeSQLRows([]string{"id", "name", "version", "description", "agent_variant", "tags", "install_scope", "sha256", "file_count", "dep_count"}, [][]driver.Value{{"team-lead", "Team Lead", "1.2.3", "Workflow package", "claude", "go,workflow", "any", "pkg-sha", int64(1), int64(1)}})
	case strings.Contains(query, "COUNT(*) AS file_count") && strings.Contains(query, "WHERE p.id = ?"):
		if len(args) != 1 || args[0].Value != "team-lead" {
			t.Fatalf("unexpected args for package detail: %+v", args)
		}
		return makeFakeSQLRows([]string{"id", "name", "version", "description", "agent_variant", "author", "license", "tags", "install_scope", "variables", "options", "sha256", "min_claude_version", "file_count", "dep_count"}, [][]driver.Value{{"team-lead", "Team Lead", "1.2.3", "Workflow package", "claude", "Randlee", "MIT", "go,workflow", "any", []byte(`{"repo":"name"}`), []byte(`{"mode":"strict"}`), "pkg-sha", "1.0.0", int64(1), int64(1)}})
	case strings.Contains(query, "SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM") && strings.Contains(query, "WHERE id = ?"):
		return makeFakeSQLRows([]string{"id", "name", "version", "description", "agent_variant", "author", "license", "tags", "install_scope", "variables", "options", "sha256", "min_claude_version"}, [][]driver.Value{{"team-lead", "Team Lead", "1.2.3", "Workflow package", "claude", "Randlee", "MIT", "go,workflow", "any", []byte(`{"repo":"name"}`), []byte(`{"mode":"strict"}`), "pkg-sha", "1.0.0"}})
	case strings.Contains(query, "package_variants") && strings.Contains(query, "variant_package_id"):
		return makeFakeSQLRows([]string{"variant_package_id"}, [][]driver.Value{{"team-lead-codex"}})
	case strings.Contains(query, "package_questions") && strings.Contains(query, "question_id"):
		return makeFakeSQLRows([]string{"package_id", "question_id", "prompt", "type", "default_val", "choices", "sort_order"}, [][]driver.Value{{"team-lead", "repo_name", "Repo name", "text", "demo", "", int64(1)}})
	case strings.Contains(query, "package_hooks") && strings.Contains(query, "script_path"):
		return makeFakeSQLRows([]string{"package_id", "event", "matcher", "script_path", "priority", "blocking"}, [][]driver.Value{{"team-lead", "PreToolUse", "git commit", "hooks/pre-commit.sh", int64(5), true}})
	case strings.Contains(query, "package_deps") && strings.Contains(query, "dep_name"):
		return makeFakeSQLRows([]string{"package_id", "dep_type", "dep_name", "dep_spec", "install_cmd", "cmd_sha256"}, [][]driver.Value{{"team-lead", "tool", "gh", ">=2.0.0", "brew install gh", "cmd-sha"}})
	case strings.Contains(query, "package_files AS f") && strings.Contains(query, "doc_path"):
		return makeFakeSQLRows([]string{"package_id", "version", "doc_path", "sha256"}, [][]driver.Value{{"team-lead", "1.2.3", "skills/team-lead/SKILL.md", "sha-skill"}})
	case strings.Contains(query, "package_files") && strings.Contains(query, "dest_path") && strings.Contains(query, "ORDER BY dest_path"):
		return makeFakeSQLRows([]string{"package_id", "dest_path", "content", "sha256", "file_type", "content_type", "is_template", "frontmatter", "fm_name", "fm_description", "fm_version", "fm_model"}, [][]driver.Value{{"team-lead", "skills/team-lead/SKILL.md", "# Team Lead", "sha-skill", "skill", "markdown", false, []byte(`{"name":"team-lead"}`), "Team Lead", nil, "1.2.3", nil}})
	default:
		t.Fatalf("unexpected SQL query: %s args=%+v", query, args)
		return fakeSQLRows{}
	}
}

type fakeSQLRows struct {
	columns []string
	rows    [][]driver.Value
}

func makeFakeSQLRows(columns []string, rows [][]driver.Value) fakeSQLRows {
	return fakeSQLRows{columns: columns, rows: rows}
}

type fakeSQLDriver struct{}
type fakeSQLConn struct{}
type fakeSQLResultRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

var (
	fakeSQLMu      sync.Mutex
	fakeSQLHandler func(string, []driver.NamedValue) (fakeSQLRows, error)
)

func init() {
	sql.Register("synaptic-canvas-fake", fakeSQLDriver{})
}

func openFakeSQLDB(t *testing.T, handler func(string, []driver.NamedValue) (fakeSQLRows, error)) *sql.DB {
	t.Helper()
	fakeSQLMu.Lock()
	fakeSQLHandler = handler
	fakeSQLMu.Unlock()
	t.Cleanup(func() {
		fakeSQLMu.Lock()
		fakeSQLHandler = nil
		fakeSQLMu.Unlock()
	})
	db, err := sql.Open("synaptic-canvas-fake", "ignored")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func (fakeSQLDriver) Open(string) (driver.Conn, error) {
	return fakeSQLConn{}, nil
}

func (fakeSQLConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (fakeSQLConn) Close() error                        { return nil }
func (fakeSQLConn) Begin() (driver.Tx, error)           { return nil, errors.New("not implemented") }

func (fakeSQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	fakeSQLMu.Lock()
	handler := fakeSQLHandler
	fakeSQLMu.Unlock()
	if handler == nil {
		return nil, errors.New("fake SQL handler not configured")
	}
	rows, err := handler(query, args)
	if err != nil {
		return nil, err
	}
	return &fakeSQLResultRows{columns: rows.columns, rows: rows.rows}, nil
}

func (c fakeSQLConn) CheckNamedValue(*driver.NamedValue) error { return nil }

func (r *fakeSQLResultRows) Columns() []string { return r.columns }
func (r *fakeSQLResultRows) Close() error      { return nil }

func (r *fakeSQLResultRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

var _ driver.QueryerContext = fakeSQLConn{}
var _ driver.NamedValueChecker = fakeSQLConn{}

var _ = catalog.CatalogEntry{}
