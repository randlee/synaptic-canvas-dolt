package dolt

import (
	"strings"
	"testing"
)

func TestListPackagesQuery(t *testing.T) {
	t.Parallel()
	q := ListPackagesQuery()
	if !strings.Contains(q, "SELECT") {
		t.Error("expected SELECT in query")
	}
	if !strings.Contains(q, "FROM packages") {
		t.Error("expected FROM packages in query")
	}
	if !strings.Contains(q, "ORDER BY name") {
		t.Error("expected ORDER BY name in query")
	}
}

func TestGetPackageQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageQuery()
	if !strings.Contains(q, "WHERE id = ?") {
		t.Error("expected parameterized WHERE clause")
	}
	// Should select all package columns.
	for _, col := range []string{"id", "name", "version", "description", "agent_variant", "author", "license", "tags", "install_scope", "variables", "options", "sha256"} {
		if !strings.Contains(q, col) {
			t.Errorf("expected column %q in query", col)
		}
	}
}

func TestGetPackageFilesQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageFilesQuery()
	if !strings.Contains(q, "FROM package_files") {
		t.Error("expected FROM package_files")
	}
	if !strings.Contains(q, "WHERE package_id = ?") {
		t.Error("expected parameterized WHERE clause")
	}
	if !strings.Contains(q, "ORDER BY dest_path") {
		t.Error("expected ORDER BY dest_path")
	}
}

func TestGetPackageDepsQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageDepsQuery()
	if !strings.Contains(q, "FROM package_deps") {
		t.Error("expected FROM package_deps")
	}
	if !strings.Contains(q, "WHERE package_id = ?") {
		t.Error("expected parameterized WHERE clause")
	}
}

func TestGetPackageHooksQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageHooksQuery()
	if !strings.Contains(q, "FROM package_hooks") {
		t.Error("expected FROM package_hooks")
	}
	if !strings.Contains(q, "ORDER BY event, priority") {
		t.Error("expected ORDER BY event, priority")
	}
}

func TestGetPackageQuestionsQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageQuestionsQuery()
	if !strings.Contains(q, "FROM package_questions") {
		t.Error("expected FROM package_questions")
	}
	if !strings.Contains(q, "ORDER BY sort_order") {
		t.Error("expected ORDER BY sort_order")
	}
}

func TestResolveVariantQuery(t *testing.T) {
	t.Parallel()
	q := ResolveVariantQuery()
	if !strings.Contains(q, "FROM package_variants") {
		t.Error("expected FROM package_variants")
	}
	if !strings.Contains(q, "logical_id = ?") {
		t.Error("expected logical_id parameter")
	}
	if !strings.Contains(q, "agent_profile = ?") {
		t.Error("expected agent_profile parameter")
	}
}

func TestSearchByTagsQuery(t *testing.T) {
	t.Parallel()
	q := SearchByTagsQuery()
	if !strings.Contains(q, "JSON_CONTAINS") {
		t.Error("expected JSON_CONTAINS in query")
	}
}

func TestBranchQuery(t *testing.T) {
	t.Parallel()

	base := "SELECT * FROM packages"

	t.Run("empty branch returns unchanged", func(t *testing.T) {
		t.Parallel()
		got := BranchQuery(base, "")
		if got != base {
			t.Errorf("got %q, want %q", got, base)
		}
	})

	t.Run("non-empty branch returns unchanged (handled at connection level)", func(t *testing.T) {
		t.Parallel()
		got := BranchQuery(base, "staging")
		if got != base {
			t.Errorf("got %q, want %q", got, base)
		}
	})
}

func TestUseBranchQuery(t *testing.T) {
	t.Parallel()

	t.Run("empty branch returns empty", func(t *testing.T) {
		t.Parallel()
		got := UseBranchQuery("synaptic_canvas", "")
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("non-empty branch returns USE statement", func(t *testing.T) {
		t.Parallel()
		got := UseBranchQuery("synaptic_canvas", "staging")
		want := "USE `synaptic_canvas/staging`"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
