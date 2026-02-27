package dolt

import "fmt"

// SQL query constants for the Synaptic Canvas database.
// These correspond to the schema defined in docs/synaptic-canvas-schema.md.
//
// NOTE: sha256 column is defined in cli.md schema additions but not yet in schema.md DDL. Tracked for schema.md update.

// listPackagesQuery returns packages ordered by name.
const listPackagesBaseQuery = `SELECT id, name, version, description, tags, install_scope FROM packages ORDER BY name`

// getPackageQuery retrieves a single package by ID.
const getPackageBaseQuery = `SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, min_claude_version FROM packages WHERE id = ?`

// getPackageFilesQuery retrieves all files for a package.
const getPackageFilesBaseQuery = `SELECT package_id, dest_path, content, sha256, file_type, content_type, is_template, frontmatter, fm_name, fm_description, fm_version, fm_model FROM package_files WHERE package_id = ? ORDER BY dest_path`

// getPackageDepsQuery retrieves all dependencies for a package.
const getPackageDepsBaseQuery = `SELECT package_id, dep_type, dep_name, dep_spec, install_cmd, cmd_sha256 FROM package_deps WHERE package_id = ? ORDER BY dep_name`

// getPackageHooksQuery retrieves all hooks for a package.
const getPackageHooksBaseQuery = `SELECT package_id, event, matcher, script_path, priority, blocking FROM package_hooks WHERE package_id = ? ORDER BY event, priority`

// getPackageQuestionsQuery retrieves all questions for a package.
const getPackageQuestionsBaseQuery = `SELECT package_id, question_id, prompt, type, default_val, choices, sort_order FROM package_questions WHERE package_id = ? ORDER BY sort_order, question_id`

// resolveVariantQuery resolves a variant package ID from a logical ID and agent profile.
const resolveVariantBaseQuery = `SELECT variant_package_id FROM package_variants WHERE logical_id = ? AND agent_profile = ?`

// Branch switching is handled at the connection level via UseBranchQuery/switchBranch,
// not via query modification.

// UseBranchQuery returns a USE statement for switching to a Dolt branch.
// Returns empty string if branch is empty (use default branch).
func UseBranchQuery(database, branch string) string {
	if branch == "" {
		return ""
	}
	// Dolt branch syntax: USE `database/branch`
	return fmt.Sprintf("USE `%s/%s`", database, branch)
}

// ListPackagesQuery returns the SQL for listing packages.
func ListPackagesQuery() string {
	return listPackagesBaseQuery
}

// GetPackageQuery returns the SQL for fetching a single package.
func GetPackageQuery() string {
	return getPackageBaseQuery
}

// GetPackageFilesQuery returns the SQL for fetching package files.
func GetPackageFilesQuery() string {
	return getPackageFilesBaseQuery
}

// GetPackageDepsQuery returns the SQL for fetching package dependencies.
func GetPackageDepsQuery() string {
	return getPackageDepsBaseQuery
}

// GetPackageHooksQuery returns the SQL for fetching package hooks.
func GetPackageHooksQuery() string {
	return getPackageHooksBaseQuery
}

// GetPackageQuestionsQuery returns the SQL for fetching package questions.
func GetPackageQuestionsQuery() string {
	return getPackageQuestionsBaseQuery
}

// ResolveVariantQuery returns the SQL for resolving a variant.
func ResolveVariantQuery() string {
	return resolveVariantBaseQuery
}
