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
)

// ClientHarness exposes deterministic transport fixtures for cross-package
// command contract tests.
type ClientHarness struct {
	Name string
	Open func(*testing.T) Client
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

func openHarnessHTTPClient(t *testing.T) Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query_execution_status": "Success",
			"schema":                 []map[string]string{{"columnName": "id", "columnType": "text"}},
			"rows":                   harnessListRows(t, query),
		})
	}))
	t.Cleanup(server.Close)
	return NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo", Branch: "main"})
}

func openHarnessHTTPFailingClient(t *testing.T) Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo", Branch: "main"})
}

func openHarnessSQLClient(t *testing.T) Client {
	t.Helper()
	db := openHarnessFakeSQLDB(t, func(query string, _ []driver.NamedValue) (harnessFakeSQLRows, error) {
		return harnessSQLRows(t, query), nil
	})
	return NewSQLClient(db, "owner/repo", "main")
}

func openHarnessSQLFailingClient(t *testing.T) Client {
	t.Helper()
	db := openHarnessFakeSQLDB(t, func(string, []driver.NamedValue) (harnessFakeSQLRows, error) {
		return harnessFakeSQLRows{}, ErrServerError
	})
	return NewSQLClient(db, "owner/repo", "main")
}

func openHarnessCLIClient(t *testing.T) Client {
	t.Helper()
	prev := cliCommandRunner
	cliCommandRunner = func(_ context.Context, _ string, _ string, query string) cliCommandResult {
		data, _ := json.Marshal(map[string]any{"rows": harnessListRows(t, query)})
		return cliCommandResult{Stdout: data}
	}
	t.Cleanup(func() { cliCommandRunner = prev })
	return NewCLIReader(t.TempDir(), "main")
}

func openHarnessCLIFailingClient(t *testing.T) Client {
	t.Helper()
	prev := cliCommandRunner
	cliCommandRunner = func(_ context.Context, _ string, _ string, _ string) cliCommandResult {
		return cliCommandResult{Err: ErrServerError, Stderr: []byte("server unavailable")}
	}
	t.Cleanup(func() { cliCommandRunner = prev })
	return NewCLIReader(t.TempDir(), "main")
}

func harnessListRows(t *testing.T, query string) []map[string]any {
	t.Helper()
	if strings.Contains(query, "ORDER BY p.name") {
		return []map[string]any{{
			"id":            "team-lead",
			"name":          "Team Lead",
			"version":       "1.2.3",
			"description":   "Workflow package",
			"agent_variant": "claude",
			"tags":          "go,workflow",
			"install_scope": "any",
			"sha256":        "pkg-sha",
			"file_count":    1,
			"dep_count":     1,
		}}
	}
	t.Fatalf("unexpected query: %s", query)
	return nil
}

func harnessSQLRows(t *testing.T, query string) harnessFakeSQLRows {
	t.Helper()
	if strings.Contains(query, "COUNT(*) AS file_count") && strings.Contains(query, "ORDER BY p.name") {
		return makeHarnessFakeSQLRows(
			[]string{"id", "name", "version", "description", "agent_variant", "tags", "install_scope", "sha256", "file_count", "dep_count"},
			[][]driver.Value{{"team-lead", "Team Lead", "1.2.3", "Workflow package", "claude", "go,workflow", "any", "pkg-sha", int64(1), int64(1)}},
		)
	}
	t.Fatalf("unexpected SQL query: %s", query)
	return harnessFakeSQLRows{}
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
	sql.Register("synaptic-canvas-harness-fake", harnessFakeSQLDriver{})
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
	db, err := sql.Open("synaptic-canvas-harness-fake", "ignored")
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

func (r *harnessFakeSQLResultRows) Columns() []string { return r.columns }
func (r *harnessFakeSQLResultRows) Close() error      { return nil }

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
