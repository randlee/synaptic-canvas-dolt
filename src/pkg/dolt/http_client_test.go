package dolt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestHTTPClientListPackages(t *testing.T) {
	t.Parallel()

	var requestURI string
	var authHeader string
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI = r.RequestURI
		authHeader = r.Header.Get("Authorization")
		method = r.Method
		if err := r.ParseForm(); err != nil { //nolint:gosec // G120: httptest.NewServer is in-process, never exposed to untrusted traffic.
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got := r.Form.Get("q"); !strings.Contains(got, "FROM packages AS p") {
			t.Fatalf("query = %q, want package list SQL", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query_execution_status": "Success",
			"schema":                 []map[string]string{{"columnName": "id", "columnType": "text"}},
			"rows": []map[string]any{
				{
					"id":            "team-lead",
					"name":          "Team Lead",
					"version":       "1.2.3",
					"description":   nil,
					"agent_variant": true,
					"tags":          "go,workflow",
					"install_scope": "any",
					"file_count":    float64(2),
					"dep_count":     "3",
					"ignored":       "extra fields are ignored",
				},
			},
		})
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPConfig{
		Host:     server.URL,
		Database: "owner/repo",
		Branch:   "feature/http",
		Token:    "secret",
	})
	packages, err := client.ListPackages(context.Background(), ListOptions{Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}
	if method != http.MethodGet {
		t.Fatalf("method = %q, want GET", method)
	}
	if authHeader != "token secret" {
		t.Fatalf("Authorization = %q, want token secret", authHeader)
	}
	if !strings.Contains(requestURI, "/owner/repo/feature%2Fhttp?") {
		t.Fatalf("RequestURI = %q, want escaped branch path", requestURI)
	}
	if len(packages) != 1 {
		t.Fatalf("len(packages) = %d, want 1", len(packages))
	}
	pkg := packages[0]
	if pkg.ID != "team-lead" || pkg.Name != "Team Lead" || pkg.Version != "1.2.3" {
		t.Fatalf("unexpected package: %+v", pkg)
	}
	if pkg.AgentVariant != "true" {
		t.Fatalf("AgentVariant = %q, want true", pkg.AgentVariant)
	}
	if pkg.InstallScope != models.InstallScopeAny {
		t.Fatalf("InstallScope = %q, want %q", pkg.InstallScope, models.InstallScopeAny)
	}
	if pkg.FileCount != 2 || pkg.DepCount != 3 {
		t.Fatalf("counts = %d/%d, want 2/3", pkg.FileCount, pkg.DepCount)
	}
	if pkg.Description != nil {
		t.Fatalf("Description = %q, want nil for JSON null", *pkg.Description)
	}
}

func TestHTTPClientEmptyRows(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query_execution_status": "success",
			"rows":                   nil,
		})
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo"})
	packages, err := client.ListPackages(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}
	if packages == nil {
		t.Fatal("packages = nil, want empty slice")
	}
	if len(packages) != 0 {
		t.Fatalf("len(packages) = %d, want 0", len(packages))
	}
}

func TestHTTPClientPackageRelatedRows(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sql := r.URL.Query().Get("q")
		var rows []map[string]any
		switch {
		case strings.Contains(sql, "FROM package_files AS pf"):
			rows = []map[string]any{{
				"package_id": "pkg",
				"version":    "1.2.3",
				"doc_path":   "skills/pkg/SKILL.md",
				"sha256":     "abc",
			}}
		case strings.Contains(sql, "FROM package_files"):
			rows = []map[string]any{{
				"package_id":     "pkg",
				"dest_path":      "skills/pkg/SKILL.md",
				"content":        "body",
				"sha256":         "abc",
				"file_type":      "skill",
				"content_type":   "markdown",
				"is_template":    "1",
				"frontmatter":    map[string]any{"name": "pkg"},
				"fm_name":        "Pkg",
				"fm_description": nil,
				"fm_version":     "1.0.0",
				"fm_model":       nil,
			}}
		case strings.Contains(sql, "FROM package_hooks"):
			rows = []map[string]any{{
				"package_id":  "pkg",
				"event":       "PostToolUse",
				"matcher":     ".*",
				"script_path": "hooks/post.sh",
				"priority":    "7",
				"blocking":    float64(1),
			}}
		case strings.Contains(sql, "FROM package_questions"):
			rows = []map[string]any{{
				"package_id":  "pkg",
				"question_id": "q1",
				"prompt":      "Choose",
				"type":        "choice",
				"default_val": nil,
				"choices":     "a,b",
				"sort_order":  float64(2),
			}}
		default:
			t.Fatalf("unexpected query: %s", sql)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query_execution_status": "success",
			"rows":                   rows,
		})
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo"})

	files, err := client.GetPackageFiles(context.Background(), "pkg")
	if err != nil {
		t.Fatalf("GetPackageFiles() error = %v", err)
	}
	if len(files) != 1 || !files[0].IsTemplate || string(files[0].Frontmatter) != `{"name":"pkg"}` {
		t.Fatalf("unexpected files: %+v", files)
	}
	if files[0].FMDescription != nil || files[0].FMModel != nil {
		t.Fatalf("expected nil string pointers for null optional fields: %+v", files[0])
	}

	fileSHAs, err := client.GetPackageFileSHAs(context.Background(), "pkg", "skills/pkg/SKILL.md")
	if err != nil {
		t.Fatalf("GetPackageFileSHAs() error = %v", err)
	}
	if len(fileSHAs) != 1 || fileSHAs[0].Version != "1.2.3" || fileSHAs[0].DocPath != "skills/pkg/SKILL.md" {
		t.Fatalf("unexpected file SHAs: %+v", fileSHAs)
	}

	hooks, err := client.GetPackageHooks(context.Background(), "pkg")
	if err != nil {
		t.Fatalf("GetPackageHooks() error = %v", err)
	}
	if len(hooks) != 1 || hooks[0].Priority != 7 || !hooks[0].Blocking {
		t.Fatalf("unexpected hooks: %+v", hooks)
	}

	questions, err := client.GetPackageQuestions(context.Background(), "pkg")
	if err != nil {
		t.Fatalf("GetPackageQuestions() error = %v", err)
	}
	if len(questions) != 1 || questions[0].SortOrder != 2 || questions[0].DefaultVal != "" {
		t.Fatalf("unexpected questions: %+v", questions)
	}
}

func TestHTTPClientHTTPStatusErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   error
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: ErrUnauthorized},
		{name: "forbidden", status: http.StatusForbidden, want: ErrUnauthorized},
		{name: "not found", status: http.StatusNotFound, want: ErrNotFound},
		{name: "rate limited", status: http.StatusTooManyRequests, want: ErrRateLimited},
		{name: "server error", status: http.StatusInternalServerError, want: ErrServerError},
		{name: "bad query", status: http.StatusBadRequest, want: ErrBadQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, http.StatusText(tt.status), tt.status)
			}))
			defer server.Close()

			client := NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo"})
			_, err := client.ListPackages(context.Background(), ListOptions{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if tt.want == ErrUnauthorized && !strings.Contains(err.Error(), "sc config set dolt.token") {
				t.Fatalf("unauthorized error missing token guidance: %v", err)
			}
		})
	}
}

func TestHTTPClientRetriesRateLimit(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt := attempts.Add(1)
		if attempt <= maxHTTPRateLimitRetries {
			w.Header().Set("Retry-After", "0")
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query_execution_status": "success",
			"rows":                   []map[string]any{},
		})
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo"})
	if _, err := client.ListPackages(context.Background(), ListOptions{}); err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}
	if got := attempts.Load(); got != maxHTTPRateLimitRetries+1 {
		t.Fatalf("attempts = %d, want %d", got, maxHTTPRateLimitRetries+1)
	}
}

func TestHTTPClientRateLimitRetryDelay(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(2 * time.Second).UTC().Truncate(time.Second)
	tests := []struct {
		name       string
		retryAfter string
		attempt    int
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{name: "http date", retryAfter: future.Format(http.TimeFormat), wantMin: time.Second, wantMax: 3 * time.Second},
		{name: "default backoff", retryAfter: "", attempt: 1, wantMin: 200 * time.Millisecond, wantMax: 200 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rateLimitRetryDelay(tt.retryAfter, tt.attempt)
			if got < tt.wantMin || got > tt.wantMax {
				t.Fatalf("rateLimitRetryDelay() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestHTTPClientRateLimitRetrySleepHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		cancel()
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo"})
	start := time.Now()
	_, err := client.ListPackages(ctx, ListOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("retry sleep ignored cancellation; elapsed=%v", elapsed)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestHTTPClientMalformedAndFailedResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		want     error
		contains string
	}{
		{name: "malformed JSON", body: `{"rows":`, contains: "decoding DoltHub response"},
		{name: "failed query", body: `{"query_execution_status":"error","query_execution_message":"syntax failed"}`, want: ErrBadQuery, contains: "syntax failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo"})
			_, err := client.ListPackages(context.Background(), ListOptions{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("error = %v, want to contain %q", err, tt.contains)
			}
		})
	}
}

func TestHTTPClientRequestValidationAndCancellation(t *testing.T) {
	t.Parallel()

	t.Run("missing database", func(t *testing.T) {
		t.Parallel()
		hit := false
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			hit = true
		}))
		defer server.Close()

		client := NewHTTPClient(HTTPConfig{Host: server.URL})
		_, err := client.ListPackages(context.Background(), ListOptions{})
		if err == nil || !strings.Contains(err.Error(), "dolt.database is not configured") {
			t.Fatalf("error = %v, want missing database guidance", err)
		}
		if hit {
			t.Fatal("server was hit for missing database")
		}
	})

	t.Run("invalid database", func(t *testing.T) {
		t.Parallel()
		client := NewHTTPClient(HTTPConfig{Database: "repo-only"})
		_, err := client.ListPackages(context.Background(), ListOptions{})
		if !errors.Is(err, ErrBadQuery) {
			t.Fatalf("error = %v, want ErrBadQuery", err)
		}
	})

	t.Run("long query", func(t *testing.T) {
		t.Parallel()
		client := NewHTTPClient(HTTPConfig{Database: "owner/repo"})
		_, err := client.query(context.Background(), strings.Repeat("x", maxHTTPQueryURLLength))
		if !errors.Is(err, ErrBadQuery) {
			t.Fatalf("error = %v, want ErrBadQuery", err)
		}
	})

	t.Run("context canceled", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"rows": []map[string]any{}})
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := NewHTTPClient(HTTPConfig{Host: server.URL, Database: "owner/repo"})
		_, err := client.ListPackages(ctx, ListOptions{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})
}
