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
	// Must include sha256 column per schema spec.
	if !strings.Contains(q, "sha256") {
		t.Error("expected sha256 column in list packages query")
	}
}

func TestGetPackageQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageQuery()
	if !strings.Contains(q, "WHERE id = ?") {
		t.Error("expected parameterized WHERE clause")
	}
	// Should select all package columns including min_claude_version.
	for _, col := range []string{"id", "name", "version", "description", "agent_variant", "author", "license", "tags", "install_scope", "variables", "options", "sha256", "min_claude_version"} {
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
	// Must include frontmatter extraction columns per schema spec.
	for _, col := range []string{"fm_name", "fm_description", "fm_version", "fm_model"} {
		if !strings.Contains(q, col) {
			t.Errorf("expected column %q in package files query", col)
		}
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
	if !strings.Contains(q, "ORDER BY dep_name") {
		t.Error("expected ORDER BY dep_name")
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
	if !strings.Contains(q, "ORDER BY sort_order, question_id") {
		t.Error("expected ORDER BY sort_order, question_id")
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
