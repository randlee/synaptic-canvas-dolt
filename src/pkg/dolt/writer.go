package dolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

// ImportPackageRequest contains all relational rows for one import.
type ImportPackageRequest struct {
	Branch        string
	Package       models.Package
	Files         []models.PackageFile
	Deps          []models.PackageDep
	Hooks         []models.PackageHook
	Questions     []models.PackageQuestion
	PackageSHA256 string
	CommitMessage string
}

// CLIWriter writes package data via the Dolt CLI.
type CLIWriter struct {
	DoltDir string
}

// NewCLIWriter returns a CLI-backed Dolt writer.
func NewCLIWriter(doltDir string) *CLIWriter {
	return &CLIWriter{DoltDir: doltDir}
}

// BranchExists reports whether the target branch exists.
func (w *CLIWriter) BranchExists(ctx context.Context, branch string) (bool, error) {
	out, err := w.run(ctx, "dolt", "branch", "--list", branch)
	if err != nil {
		return false, err
	}
	return strings.Contains(out, branch), nil
}

// ImportPackage writes all package rows and creates a Dolt commit.
func (w *CLIWriter) ImportPackage(ctx context.Context, req ImportPackageRequest) error {
	currentBranchOutput, err := w.run(ctx, "dolt", "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("reading current branch: %w", err)
	}
	currentBranch := strings.TrimSpace(currentBranchOutput)

	if _, err := w.run(ctx, "dolt", "checkout", req.Branch); err != nil {
		return fmt.Errorf("checking out branch %q: %w", req.Branch, err)
	}
	defer func() {
		if currentBranch != "" && currentBranch != req.Branch {
			_, _ = w.run(context.Background(), "dolt", "checkout", currentBranch)
		}
	}()

	sql := buildImportSQL(req)
	if err := w.runSQL(ctx, sql); err != nil {
		return err
	}
	if _, err := w.run(ctx, "dolt", "add", "-A"); err != nil {
		return fmt.Errorf("staging Dolt changes: %w", err)
	}
	if _, err := w.run(ctx, "dolt", "commit", "-m", req.CommitMessage); err != nil {
		return fmt.Errorf("creating Dolt commit: %w", err)
	}
	return nil
}

func (w *CLIWriter) runSQL(ctx context.Context, sql string) error {
	cmd := exec.CommandContext(ctx, "dolt", "sql")
	cmd.Dir = w.DoltDir
	cmd.Stdin = strings.NewReader(sql)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running dolt sql: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (w *CLIWriter) run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = w.DoltDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func buildImportSQL(req ImportPackageRequest) string {
	stmts := []string{
		fmt.Sprintf("DELETE FROM package_questions WHERE package_id = %s;", sqlString(req.Package.ID)),
		fmt.Sprintf("DELETE FROM package_hooks WHERE package_id = %s;", sqlString(req.Package.ID)),
		fmt.Sprintf("DELETE FROM package_deps WHERE package_id = %s;", sqlString(req.Package.ID)),
		fmt.Sprintf("DELETE FROM package_files WHERE package_id = %s;", sqlString(req.Package.ID)),
		fmt.Sprintf("DELETE FROM packages WHERE id = %s;", sqlString(req.Package.ID)),
	}

	stmts = append(stmts, fmt.Sprintf(
		"INSERT INTO packages (id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s);",
		sqlString(req.Package.ID),
		sqlString(req.Package.Name),
		sqlString(req.Package.Version),
		sqlNullable(req.Package.Description),
		sqlString(req.Package.AgentVariant),
		sqlNullable(req.Package.Author),
		sqlNullable(req.Package.License),
		sqlString(req.Package.Tags),
		sqlString(string(req.Package.InstallScope)),
		sqlJSON(req.Package.Variables),
		sqlJSON(req.Package.Options),
		sqlString(req.PackageSHA256),
		sqlNullable(req.Package.MinClaudeVer),
	))

	for _, file := range req.Files {
		stmts = append(stmts, fmt.Sprintf(
			"INSERT INTO package_files (package_id, dest_path, content, sha256, file_type, content_type, is_template, fm_name, fm_description, fm_version, fm_model, frontmatter) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s);",
			sqlString(file.PackageID),
			sqlString(file.DestPath),
			sqlString(file.Content),
			sqlString(file.SHA256),
			sqlString(string(file.FileType)),
			sqlString(string(file.ContentType)),
			sqlBool(file.IsTemplate),
			sqlNullable(file.FMName),
			sqlNullable(file.FMDescription),
			sqlNullable(file.FMVersion),
			sqlNullable(file.FMModel),
			sqlJSON(file.Frontmatter),
		))
	}

	for _, dep := range req.Deps {
		stmts = append(stmts, fmt.Sprintf(
			"INSERT INTO package_deps (package_id, dep_type, dep_name, dep_spec, install_cmd, cmd_sha256) VALUES (%s, %s, %s, %s, %s, %s);",
			sqlString(dep.PackageID),
			sqlString(string(dep.DepType)),
			sqlString(dep.DepName),
			sqlString(dep.DepSpec),
			sqlString(dep.InstallCmd),
			sqlString(dep.CmdSHA256),
		))
	}

	for _, hook := range req.Hooks {
		stmts = append(stmts, fmt.Sprintf(
			"INSERT INTO package_hooks (package_id, event, matcher, script_path, priority, blocking) VALUES (%s, %s, %s, %s, %d, %s);",
			sqlString(hook.PackageID),
			sqlString(string(hook.Event)),
			sqlString(hook.Matcher),
			sqlString(hook.ScriptPath),
			hook.Priority,
			sqlBool(hook.Blocking),
		))
	}

	for _, q := range req.Questions {
		stmts = append(stmts, fmt.Sprintf(
			"INSERT INTO package_questions (package_id, question_id, prompt, type, default_val, choices, sort_order) VALUES (%s, %s, %s, %s, %s, %s, %d);",
			sqlString(q.PackageID),
			sqlString(q.QuestionID),
			sqlString(q.Prompt),
			sqlString(string(q.Type)),
			sqlString(q.DefaultVal),
			sqlString(q.Choices),
			q.SortOrder,
		))
	}

	return strings.Join(stmts, "\n")
}

func sqlString(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}

func sqlNullable(value *string) string {
	if value == nil {
		return "NULL"
	}
	return sqlString(*value)
}

func sqlBool(value bool) string {
	if value {
		return "TRUE"
	}
	return "FALSE"
}

func sqlJSON(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "NULL"
	}
	return sqlString(string(raw))
}

func repoRootFromDoltDir(doltDir string) string {
	return filepath.Clean(doltDir)
}
