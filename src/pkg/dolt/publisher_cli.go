package dolt

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/publisher"
)

// CLIPublisher promotes packages using the local dolt CLI.
type CLIPublisher struct {
	DoltDir string
}

func NewCLIPublisher(doltDir string) *CLIPublisher {
	return &CLIPublisher{DoltDir: doltDir}
}

func (p *CLIPublisher) PublishPackage(ctx context.Context, packageID, fromBranch, toBranch string) (*publisher.PublishResult, error) {
	databaseName, err := p.currentDatabase(ctx, toBranch)
	if err != nil {
		return nil, err
	}
	commitMessage := fmt.Sprintf("Publish package %s from %s to %s", packageID, fromBranch, toBranch)
	sql := buildPublishSQL(databaseName, packageID, fromBranch, commitMessage)

	cmd := exec.CommandContext(ctx, doltCommand, "--branch", toBranch, "sql") //nolint:gosec // G204: dolt binary is hardcoded constant.
	cmd.Dir = p.DoltDir
	cmd.Stdin = strings.NewReader(sql)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("publishing %s from %s to %s: %w: %s", packageID, fromBranch, toBranch, err, strings.TrimSpace(stderr.String()))
	}

	commitHash, err := p.headCommitHash(ctx, toBranch)
	if err != nil {
		return nil, err
	}
	return &publisher.PublishResult{
		Hash:    commitHash,
		Message: commitMessage,
	}, nil
}

func (p *CLIPublisher) currentDatabase(ctx context.Context, branch string) (string, error) {
	raw, err := p.runSQLQuery(ctx, branch, "SELECT DATABASE();")
	if err != nil {
		return "", fmt.Errorf("querying current database on %s: %w", branch, err)
	}

	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(raw)))
	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("parsing database query output: %w", err)
	}
	if len(records) < 2 || len(records[1]) < 1 || strings.TrimSpace(records[1][0]) == "" {
		return "", fmt.Errorf("unexpected database query output: %q", raw)
	}
	name := strings.TrimSpace(records[1][0])
	suffix := "/" + branch
	name = strings.TrimSuffix(name, suffix)
	return name, nil
}

func (p *CLIPublisher) headCommitHash(ctx context.Context, branch string) (string, error) {
	cmd := exec.CommandContext(ctx, doltCommand, "--branch", branch, "log", "-n", "1") //nolint:gosec // G204: dolt binary is hardcoded constant.
	cmd.Dir = p.DoltDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("reading head commit on %s: %w: %s", branch, err, strings.TrimSpace(stderr.String()))
	}

	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "commit ") {
			continue
		}
		hash := strings.TrimSpace(strings.TrimPrefix(line, "commit "))
		if hash != "" {
			return hash, nil
		}
	}
	return "", fmt.Errorf("unexpected dolt log output: %s", strings.TrimSpace(stdout.String()))
}

func (p *CLIPublisher) runSQLQuery(ctx context.Context, branch, query string) (string, error) {
	cmd := exec.CommandContext(ctx, doltCommand, "--branch", branch, "sql", "-q", query, "-r", "csv") //nolint:gosec // G204: dolt binary is hardcoded constant.
	cmd.Dir = p.DoltDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running sql query on %s: %w: %s", branch, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func buildPublishSQL(databaseName, packageID, fromBranch, commitMessage string) string {
	source := func(table string) string {
		return fmt.Sprintf("`%s/%s`.%s", databaseName, fromBranch, table)
	}

	return strings.Join([]string{
		fmt.Sprintf("DELETE FROM package_variants WHERE variant_package_id = %s;", sqlString(packageID)),
		fmt.Sprintf("DELETE FROM package_questions WHERE package_id = %s;", sqlString(packageID)),
		fmt.Sprintf("DELETE FROM package_hooks WHERE package_id = %s;", sqlString(packageID)),
		fmt.Sprintf("DELETE FROM package_deps WHERE package_id = %s;", sqlString(packageID)),
		fmt.Sprintf("DELETE FROM package_files WHERE package_id = %s;", sqlString(packageID)),
		fmt.Sprintf("DELETE FROM packages WHERE id = %s;", sqlString(packageID)),
		fmt.Sprintf("INSERT INTO packages (id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version) SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM %s WHERE id = %s;", source("packages"), sqlString(packageID)),
		fmt.Sprintf("INSERT INTO package_files (package_id, dest_path, content, sha256, file_type, content_type, is_template, fm_name, fm_description, fm_version, fm_model, frontmatter) SELECT package_id, dest_path, content, sha256, file_type, content_type, is_template, fm_name, fm_description, fm_version, fm_model, frontmatter FROM %s WHERE package_id = %s;", source("package_files"), sqlString(packageID)),
		fmt.Sprintf("INSERT INTO package_deps (package_id, dep_type, dep_name, dep_spec, install_cmd, cmd_sha256) SELECT package_id, dep_type, dep_name, dep_spec, install_cmd, cmd_sha256 FROM %s WHERE package_id = %s;", source("package_deps"), sqlString(packageID)),
		fmt.Sprintf("INSERT INTO package_hooks (package_id, event, matcher, script_path, priority, blocking) SELECT package_id, event, matcher, script_path, priority, blocking FROM %s WHERE package_id = %s;", source("package_hooks"), sqlString(packageID)),
		fmt.Sprintf("INSERT INTO package_questions (package_id, question_id, prompt, type, default_val, choices, sort_order) SELECT package_id, question_id, prompt, type, default_val, choices, sort_order FROM %s WHERE package_id = %s;", source("package_questions"), sqlString(packageID)),
		fmt.Sprintf("INSERT INTO package_variants (logical_id, agent_profile, variant_package_id) SELECT logical_id, agent_profile, variant_package_id FROM %s WHERE variant_package_id = %s;", source("package_variants"), sqlString(packageID)),
		buildCommitSQL(commitMessage),
	}, "\n")
}
