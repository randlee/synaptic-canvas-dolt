package dolt

import (
	"fmt"
	"strings"
)

// SQL query constants for the Synaptic Canvas database.
// These correspond to the schema defined in docs/synaptic-canvas-schema.md.

func BranchQualifiedFrom(database, branch, table string) string {
	if branch == "" {
		return table
	}
	return fmt.Sprintf("`%s/%s`.%s", database, branch, table)
}

// ListPackagesQuery returns the SQL and args for listing packages with counts and optional tag filters.
func ListPackagesQuery(database, branch string, tags []string) (string, []any) {
	packagesTable := BranchQualifiedFrom(database, branch, "packages")
	filesTable := BranchQualifiedFrom(database, branch, "package_files")
	depsTable := BranchQualifiedFrom(database, branch, "package_deps")

	query := fmt.Sprintf(
		`SELECT p.id, p.name, p.version, p.description, p.agent_variant, p.tags, p.install_scope, p.sha256,
COALESCE(fc.file_count, 0) AS file_count, COALESCE(dc.dep_count, 0) AS dep_count
FROM %s AS p
LEFT JOIN (
  SELECT package_id, COUNT(*) AS file_count FROM %s GROUP BY package_id
) AS fc ON p.id = fc.package_id
LEFT JOIN (
  SELECT package_id, COUNT(*) AS dep_count FROM %s GROUP BY package_id
) AS dc ON p.id = dc.package_id`,
		packagesTable, filesTable, depsTable,
	)

	args := make([]any, 0, len(tags))
	if predicate := ListPackagesTagPredicate(tags); predicate != "" {
		query += " WHERE " + predicate
		for _, tag := range tags {
			normalized := strings.ToLower(strings.TrimSpace(tag))
			if normalized != "" {
				args = append(args, "%,"+normalized+",%")
			}
		}
	}

	query += " ORDER BY p.name"
	return query, args
}

// ListPackagesTagPredicate returns the SQL predicate for case-insensitive OR tag matching.
func ListPackagesTagPredicate(tags []string) string {
	clauses := make([]string, 0, len(tags))
	for _, tag := range tags {
		if strings.TrimSpace(tag) == "" {
			continue
		}
		clauses = append(clauses, "LOWER(CONCAT(',', REPLACE(COALESCE(p.tags, ''), ' ', ''), ',')) LIKE ?")
	}
	return strings.Join(clauses, " OR ")
}

// GetPackageQuery returns the SQL for fetching a single package.
func GetPackageQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM %s WHERE id = ?",
		BranchQualifiedFrom(database, branch, "packages"),
	)
}

// GetPackageDetailQuery returns the SQL for fetching a package plus file/dependency counts.
func GetPackageDetailQuery(database, branch string) string {
	packagesTable := BranchQualifiedFrom(database, branch, "packages")
	filesTable := BranchQualifiedFrom(database, branch, "package_files")
	depsTable := BranchQualifiedFrom(database, branch, "package_deps")

	return fmt.Sprintf(
		`SELECT p.id, p.name, p.version, p.description, p.agent_variant, p.author, p.license, p.tags, p.install_scope, p.variables, p.options, p.sha256, p.min_claude_version,
COALESCE(fc.file_count, 0) AS file_count, COALESCE(dc.dep_count, 0) AS dep_count
FROM %s AS p
LEFT JOIN (
  SELECT package_id, COUNT(*) AS file_count FROM %s GROUP BY package_id
) AS fc ON p.id = fc.package_id
LEFT JOIN (
  SELECT package_id, COUNT(*) AS dep_count FROM %s GROUP BY package_id
) AS dc ON p.id = dc.package_id
WHERE p.id = ?`,
		packagesTable, filesTable, depsTable,
	)
}

// GetPackageFilesQuery returns the SQL for fetching package files.
func GetPackageFilesQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT package_id, dest_path, content, sha256, file_type, content_type, is_template, frontmatter, fm_name, fm_description, fm_version, fm_model FROM %s WHERE package_id = ? ORDER BY dest_path",
		BranchQualifiedFrom(database, branch, "package_files"),
	)
}

// GetPackageFileSHAsQuery returns SQL for checking immutable package file SHAs.
func GetPackageFileSHAsQuery(database, branch string) string {
	return fmt.Sprintf(
		`SELECT pf.package_id, p.version, pf.dest_path AS doc_path, pf.sha256
FROM %s AS pf
JOIN %s AS p ON p.id = pf.package_id
WHERE pf.package_id = ? AND pf.dest_path = ?
ORDER BY p.version`,
		BranchQualifiedFrom(database, branch, "package_files"),
		BranchQualifiedFrom(database, branch, "packages"),
	)
}

// GetPackageDepsQuery returns the SQL for fetching package dependencies.
func GetPackageDepsQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT package_id, dep_type, dep_name, dep_spec, install_cmd, cmd_sha256 FROM %s WHERE package_id = ? ORDER BY dep_name",
		BranchQualifiedFrom(database, branch, "package_deps"),
	)
}

// GetPackageHooksQuery returns the SQL for fetching package hooks.
func GetPackageHooksQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT package_id, event, matcher, script_path, priority, blocking FROM %s WHERE package_id = ? ORDER BY event, priority",
		BranchQualifiedFrom(database, branch, "package_hooks"),
	)
}

// GetPackageQuestionsQuery returns the SQL for fetching package questions.
func GetPackageQuestionsQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT package_id, question_id, prompt, type, default_val, choices, sort_order FROM %s WHERE package_id = ? ORDER BY sort_order, question_id",
		BranchQualifiedFrom(database, branch, "package_questions"),
	)
}

// ResolveVariantQuery returns the SQL for resolving a variant.
func ResolveVariantQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT variant_package_id FROM %s WHERE logical_id = ? AND agent_profile = ?",
		BranchQualifiedFrom(database, branch, "package_variants"),
	)
}

// GetPackageCatalogQuery returns package asset hashes for catalog refresh.
func GetPackageCatalogQuery(database, branch string) string {
	packagesTable := BranchQualifiedFrom(database, branch, "packages")
	filesTable := BranchQualifiedFrom(database, branch, "package_files")
	return fmt.Sprintf(
		`SELECT f.package_id, p.version, f.dest_path AS doc_path, f.sha256
FROM %s AS f
JOIN %s AS p ON p.id = f.package_id
WHERE COALESCE(f.sha256, '') <> ''
ORDER BY f.package_id, p.version, f.dest_path`,
		filesTable, packagesTable,
	)
}
