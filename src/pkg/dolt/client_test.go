package dolt

import (
	"context"
	"errors"
	"testing"

	"github.com/randlee/synaptic-canvas/pkg/models"
)

func TestMockClientListPackages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.AddPackage(NewTestPackage("pkg-1", "alpha", "1.0.0", []string{"go"}))
	m.AddPackage(NewTestPackage("pkg-2", "beta", "2.0.0", nil))

	pkgs, err := m.ListPackages(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("got %d packages, want 2", len(pkgs))
	}
}

func TestMockClientListPackagesError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.ListErr = errors.New("list failed")

	_, err := m.ListPackages(ctx, ListOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockClientGetPackage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.AddPackage(NewTestPackage("pkg-1", "alpha", "1.0.0", nil))

	t.Run("existing package", func(t *testing.T) {
		t.Parallel()
		p, err := m.GetPackage(ctx, "pkg-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p == nil {
			t.Fatal("expected package, got nil")
		}
		if p.Name != "alpha" {
			t.Errorf("Name = %q, want %q", p.Name, "alpha")
		}
	})

	t.Run("missing package", func(t *testing.T) {
		t.Parallel()
		p, err := m.GetPackage(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p != nil {
			t.Errorf("expected nil, got %+v", p)
		}
	})

	t.Run("error injection", func(t *testing.T) {
		t.Parallel()
		m2 := NewMockClient()
		m2.GetErr = errors.New("get failed")
		_, err := m2.GetPackage(ctx, "pkg-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestMockClientGetPackageFiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.AddFiles("pkg-1", []models.PackageFile{
		{PackageID: "pkg-1", DestPath: "agent.md", SHA256: "abc123", FileType: models.FileTypeAgent, ContentType: models.ContentTypeMarkdown},
	})

	files, err := m.GetPackageFiles(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if files[0].DestPath != "agent.md" {
		t.Errorf("DestPath = %q, want %q", files[0].DestPath, "agent.md")
	}
}

func TestMockClientGetPackageFilesError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.FilesErr = errors.New("files failed")

	_, err := m.GetPackageFiles(ctx, "pkg-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockClientGetPackageDeps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.AddDeps("pkg-1", []models.PackageDep{
		{PackageID: "pkg-1", DepType: models.DepTypeTool, DepName: "other-pkg", DepSpec: ">=1.0.0"},
	})

	deps, err := m.GetPackageDeps(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("got %d deps, want 1", len(deps))
	}
	if deps[0].DepName != "other-pkg" {
		t.Errorf("DepName = %q, want %q", deps[0].DepName, "other-pkg")
	}
}

func TestMockClientGetPackageDepsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.DepsErr = errors.New("deps failed")

	_, err := m.GetPackageDeps(ctx, "pkg-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockClientGetPackageHooks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.AddHooks("pkg-1", []models.PackageHook{
		{PackageID: "pkg-1", Event: models.HookPostToolUse, Matcher: "**/*.md", ScriptPath: "hooks/post.sh", Priority: 10, Blocking: true},
	})

	hooks, err := m.GetPackageHooks(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("got %d hooks, want 1", len(hooks))
	}
	if hooks[0].Event != models.HookPostToolUse {
		t.Errorf("Event = %q, want %q", hooks[0].Event, models.HookPostToolUse)
	}
}

func TestMockClientGetPackageHooksError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.HooksErr = errors.New("hooks failed")

	_, err := m.GetPackageHooks(ctx, "pkg-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockClientGetPackageQuestions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.AddQuestions("pkg-1", []models.PackageQuestion{
		{PackageID: "pkg-1", QuestionID: "q1", Prompt: "Choose mode", Type: models.QuestionChoice, SortOrder: 1},
	})

	questions, err := m.GetPackageQuestions(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("got %d questions, want 1", len(questions))
	}
	if questions[0].QuestionID != "q1" {
		t.Errorf("QuestionID = %q, want %q", questions[0].QuestionID, "q1")
	}
}

func TestMockClientGetPackageQuestionsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.QuestionsErr = errors.New("questions failed")

	_, err := m.GetPackageQuestions(ctx, "pkg-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockClientResolveVariant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := NewMockClient()
	m.AddVariant("logical-1", "claude-code", "variant-pkg-1")

	t.Run("existing variant", func(t *testing.T) {
		t.Parallel()
		id, err := m.ResolveVariant(ctx, "logical-1", "claude-code")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "variant-pkg-1" {
			t.Errorf("got %q, want %q", id, "variant-pkg-1")
		}
	})

	t.Run("missing variant returns empty", func(t *testing.T) {
		t.Parallel()
		id, err := m.ResolveVariant(ctx, "nonexistent", "profile")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "" {
			t.Errorf("got %q, want empty string", id)
		}
	})

	t.Run("error injection", func(t *testing.T) {
		t.Parallel()
		m2 := NewMockClient()
		m2.VariantErr = errors.New("variant failed")
		_, err := m2.ResolveVariant(ctx, "x", "y")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestMockClientClose(t *testing.T) {
	t.Parallel()

	t.Run("successful close", func(t *testing.T) {
		t.Parallel()
		m := NewMockClient()
		if m.Closed {
			t.Error("expected Closed=false before close")
		}
		if err := m.Close(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !m.Closed {
			t.Error("expected Closed=true after close")
		}
	})

	t.Run("close error", func(t *testing.T) {
		t.Parallel()
		m := NewMockClient()
		m.CloseErr = errors.New("close failed")
		if err := m.Close(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestNewTestPackage(t *testing.T) {
	t.Parallel()

	t.Run("with tags", func(t *testing.T) {
		t.Parallel()
		p := NewTestPackage("id-1", "test", "1.0.0", []string{"a", "b"})
		if p.ID != "id-1" {
			t.Errorf("ID = %q, want %q", p.ID, "id-1")
		}
		if p.InstallScope != "local" {
			t.Errorf("InstallScope = %q, want %q", p.InstallScope, "local")
		}
		tags := p.TagsList()
		if len(tags) != 2 {
			t.Fatalf("got %d tags, want 2", len(tags))
		}
	})

	t.Run("without tags", func(t *testing.T) {
		t.Parallel()
		p := NewTestPackage("id-2", "test", "1.0.0", nil)
		if p.Tags != "" {
			t.Errorf("expected empty Tags, got %q", p.Tags)
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 3306 {
		t.Errorf("Port = %d, want %d", cfg.Port, 3306)
	}
	if cfg.Database != "synaptic_canvas" {
		t.Errorf("Database = %q, want %q", cfg.Database, "synaptic_canvas")
	}
}

func TestConfigDSN(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	dsn := cfg.DSN()

	// Should contain user, host:port, and database.
	if dsn == "" {
		t.Fatal("DSN should not be empty")
	}
	// Basic format check: user@tcp(host:port)/database
	if !contains(dsn, "tcp(127.0.0.1:3306)") {
		t.Errorf("DSN %q missing host:port", dsn)
	}
	if !contains(dsn, "synaptic_canvas") {
		t.Errorf("DSN %q missing database name", dsn)
	}
}

func TestListOptions(t *testing.T) {
	t.Parallel()

	opts := ListOptions{Branch: "staging"}
	if opts.Branch != "staging" {
		t.Errorf("Branch = %q, want %q", opts.Branch, "staging")
	}
}

// contains is a simple helper to avoid importing strings in test.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Verify that MockClient satisfies Client interface at compile time.
var _ Client = (*MockClient)(nil)
