package dolt

import "fmt"

// SQL query constants for the Synaptic Canvas database.
// These correspond to the schema defined in docs/synaptic-canvas-schema.md.

func BranchQualifiedFrom(database, branch, table string) string {
	if branch == "" {
		return table
	}
	return fmt.Sprintf("`%s/%s`.%s", database, branch, table)
}

// ListPackagesQuery returns the SQL for listing packages.
func ListPackagesQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT id, name, version, description, tags, install_scope FROM %s ORDER BY name",
		BranchQualifiedFrom(database, branch, "packages"),
	)
}

// GetPackageQuery returns the SQL for fetching a single package.
func GetPackageQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM %s WHERE id = ?",
		BranchQualifiedFrom(database, branch, "packages"),
	)
}

// GetPackageFilesQuery returns the SQL for fetching package files.
func GetPackageFilesQuery(database, branch string) string {
	return fmt.Sprintf(
		"SELECT package_id, dest_path, content, sha256, file_type, content_type, is_template, frontmatter, fm_name, fm_description, fm_version, fm_model FROM %s WHERE package_id = ? ORDER BY dest_path",
		BranchQualifiedFrom(database, branch, "package_files"),
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
