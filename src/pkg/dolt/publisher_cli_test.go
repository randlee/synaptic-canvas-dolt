package dolt

import (
	"strings"
	"testing"
)

func TestBuildPublishSQL(t *testing.T) {
	t.Parallel()

	sql := buildPublishSQL("synaptic_canvas", "pkg-1", "develop", "publish commit")

	expectedSnippets := []string{
		"DELETE FROM package_variants WHERE variant_package_id = 'pkg-1';",
		"DELETE FROM package_questions WHERE package_id = 'pkg-1';",
		"DELETE FROM package_hooks WHERE package_id = 'pkg-1';",
		"DELETE FROM package_deps WHERE package_id = 'pkg-1';",
		"DELETE FROM package_files WHERE package_id = 'pkg-1';",
		"DELETE FROM packages WHERE id = 'pkg-1';",
		"FROM `synaptic_canvas/develop`.packages WHERE id = 'pkg-1';",
		"FROM `synaptic_canvas/develop`.package_files WHERE package_id = 'pkg-1';",
		"FROM `synaptic_canvas/develop`.package_deps WHERE package_id = 'pkg-1';",
		"FROM `synaptic_canvas/develop`.package_hooks WHERE package_id = 'pkg-1';",
		"FROM `synaptic_canvas/develop`.package_questions WHERE package_id = 'pkg-1';",
		"FROM `synaptic_canvas/develop`.package_variants WHERE variant_package_id = 'pkg-1';",
		"CALL DOLT_ADD('-A');",
		"CALL DOLT_COMMIT('-m', 'publish commit');",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(sql, snippet) {
			t.Fatalf("expected SQL to contain %q, got:\n%s", snippet, sql)
		}
	}
}
