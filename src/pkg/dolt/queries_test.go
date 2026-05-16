package dolt

import (
	"strings"
	"testing"
)

func TestListPackagesQuery(t *testing.T) {
	t.Parallel()
	q, args := ListPackagesQuery("synaptic_canvas", "", nil)
	if !strings.Contains(q, "SELECT") {
		t.Error("expected SELECT in query")
	}
	if !strings.Contains(q, "FROM packages") {
		t.Error("expected FROM packages in query")
	}
	for _, col := range []string{"agent_variant", "sha256", "file_count", "dep_count"} {
		if !strings.Contains(q, col) {
			t.Errorf("expected column %q in query", col)
		}
	}
	if !strings.Contains(q, "ORDER BY p.name") {
		t.Error("expected ORDER BY p.name in query")
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %d", len(args))
	}
}

func TestListPackagesQueryWithTags(t *testing.T) {
	t.Parallel()
	q, args := ListPackagesQuery("synaptic_canvas", "", []string{"Go", "cli"})
	if !strings.Contains(q, "WHERE") {
		t.Fatal("expected WHERE clause when tags are provided")
	}
	if !strings.Contains(q, "LIKE ?") {
		t.Fatal("expected LIKE predicates for tag filter")
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "%,go,%" || args[1] != "%,cli,%" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestGetPackageQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageQuery("synaptic_canvas", "")
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

func TestGetPackageDetailQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageDetailQuery("synaptic_canvas", "")
	for _, col := range []string{"file_count", "dep_count", "WHERE p.id = ?"} {
		if !strings.Contains(q, col) {
			t.Errorf("expected detail query to contain %q", col)
		}
	}
}

func TestGetPackageFilesQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageFilesQuery("synaptic_canvas", "")
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

func TestGetPackageFileSHAsQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageFileSHAsQuery("synaptic_canvas", "")
	for _, want := range []string{
		"pf.package_id",
		"p.version",
		"pf.dest_path AS doc_path",
		"pf.sha256",
		"FROM package_files AS pf",
		"JOIN packages AS p ON p.id = pf.package_id",
		"WHERE pf.package_id = ? AND pf.dest_path = ?",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("expected query to contain %q; query=%s", want, q)
		}
	}
}

func TestGetPackageDepsQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageDepsQuery("synaptic_canvas", "")
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
	q := GetPackageHooksQuery("synaptic_canvas", "")
	if !strings.Contains(q, "FROM package_hooks") {
		t.Error("expected FROM package_hooks")
	}
	if !strings.Contains(q, "ORDER BY event, priority") {
		t.Error("expected ORDER BY event, priority")
	}
}

func TestGetPackageQuestionsQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageQuestionsQuery("synaptic_canvas", "")
	if !strings.Contains(q, "FROM package_questions") {
		t.Error("expected FROM package_questions")
	}
	if !strings.Contains(q, "ORDER BY sort_order, question_id") {
		t.Error("expected ORDER BY sort_order, question_id")
	}
}

func TestResolveVariantQuery(t *testing.T) {
	t.Parallel()
	q := ResolveVariantQuery("synaptic_canvas", "")
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

func TestGetPackageCatalogQuery(t *testing.T) {
	t.Parallel()
	q := GetPackageCatalogQuery("synaptic_canvas", "beta")
	for _, want := range []string{
		"f.package_id",
		"p.version",
		"f.dest_path AS doc_path",
		"f.sha256",
		"`synaptic_canvas/beta`.package_files",
		"`synaptic_canvas/beta`.packages",
		"ORDER BY f.package_id, p.version, f.dest_path",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("query missing %q:\n%s", want, q)
		}
	}
}

func TestListPackagesTagPredicate(t *testing.T) {
	t.Parallel()
	got := ListPackagesTagPredicate([]string{"go", "", "cli"})
	if strings.Count(got, "LIKE ?") != 2 {
		t.Fatalf("expected two LIKE predicates, got %q", got)
	}
	if !strings.Contains(got, " OR ") {
		t.Fatalf("expected OR join, got %q", got)
	}
}

func TestBranchQualifiedFrom(t *testing.T) {
	t.Parallel()

	t.Run("empty branch returns empty", func(t *testing.T) {
		t.Parallel()
		got := BranchQualifiedFrom("synaptic_canvas", "", "packages")
		if got != "packages" {
			t.Errorf("got %q, want %q", got, "packages")
		}
	})

	t.Run("non-empty branch returns qualified table", func(t *testing.T) {
		t.Parallel()
		got := BranchQualifiedFrom("synaptic_canvas", "staging", "packages")
		want := "`synaptic_canvas/staging`.packages"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
