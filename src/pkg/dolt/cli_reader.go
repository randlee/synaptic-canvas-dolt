package dolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

type cliQueryResult[T any] struct {
	Rows []T `json:"rows"`
}

type cliPackageFile struct {
	PackageID     string             `json:"package_id"`
	DestPath      string             `json:"dest_path"`
	Content       string             `json:"content"`
	SHA256        string             `json:"sha256"`
	FileType      models.FileType    `json:"file_type"`
	ContentType   models.ContentType `json:"content_type"`
	IsTemplateRaw int                `json:"is_template"`
	Frontmatter   json.RawMessage    `json:"frontmatter,omitempty"`
	FMName        *string            `json:"fm_name,omitempty"`
	FMDescription *string            `json:"fm_description,omitempty"`
	FMVersion     *string            `json:"fm_version,omitempty"`
	FMModel       *string            `json:"fm_model,omitempty"`
}

type CLIReader struct {
	DoltDir string
	Branch  string
}

func NewCLIReader(doltDir, branch string) *CLIReader {
	return &CLIReader{DoltDir: doltDir, Branch: branch}
}

func (r *CLIReader) Close() error {
	return nil
}

func (r *CLIReader) ListPackages(ctx context.Context, opts ListOptions) ([]models.Package, error) {
	rows, err := runCLIQuery[models.Package](ctx, r.DoltDir, opts.Branch, fmt.Sprintf(
		"SELECT id, name, version, description, agent_variant, tags, install_scope, sha256 FROM packages ORDER BY name",
	))
	if err != nil {
		return nil, err
	}

	filtered := make([]models.Package, 0, len(rows))
	for _, row := range rows {
		if !matchesTags(row.TagsList(), opts.Tags) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered, nil
}

func (r *CLIReader) GetPackage(ctx context.Context, id string) (*models.Package, error) {
	rows, err := runCLIQuery[models.Package](ctx, r.DoltDir, r.Branch, fmt.Sprintf(
		"SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM packages WHERE id = %s",
		cliSQLString(id),
	))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (r *CLIReader) GetPackageDetail(ctx context.Context, id string) (*models.Package, error) {
	pkg, err := r.GetPackage(ctx, id)
	if err != nil || pkg == nil {
		return pkg, err
	}
	files, err := r.GetPackageFiles(ctx, id)
	if err != nil {
		return nil, err
	}
	deps, err := r.GetPackageDeps(ctx, id)
	if err != nil {
		return nil, err
	}
	pkg.FileCount = len(files)
	pkg.DepCount = len(deps)
	return pkg, nil
}

func (r *CLIReader) GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error) {
	rows, err := runCLIQuery[cliPackageFile](ctx, r.DoltDir, r.Branch, fmt.Sprintf(
		"SELECT package_id, dest_path, content, sha256, file_type, content_type, is_template, frontmatter, fm_name, fm_description, fm_version, fm_model FROM package_files WHERE package_id = %s ORDER BY dest_path",
		cliSQLString(packageID),
	))
	if err != nil {
		return nil, err
	}
	files := make([]models.PackageFile, 0, len(rows))
	for _, row := range rows {
		files = append(files, models.PackageFile{
			PackageID:     row.PackageID,
			DestPath:      row.DestPath,
			Content:       row.Content,
			SHA256:        row.SHA256,
			FileType:      row.FileType,
			ContentType:   row.ContentType,
			IsTemplate:    row.IsTemplateRaw != 0,
			Frontmatter:   row.Frontmatter,
			FMName:        row.FMName,
			FMDescription: row.FMDescription,
			FMVersion:     row.FMVersion,
			FMModel:       row.FMModel,
		})
	}
	return files, nil
}

func (r *CLIReader) GetPackageDeps(ctx context.Context, packageID string) ([]models.PackageDep, error) {
	return runCLIQuery[models.PackageDep](ctx, r.DoltDir, r.Branch, fmt.Sprintf(
		"SELECT package_id, dep_type, dep_name, dep_spec, install_cmd, cmd_sha256 FROM package_deps WHERE package_id = %s ORDER BY dep_name",
		cliSQLString(packageID),
	))
}

func (r *CLIReader) GetPackageHooks(ctx context.Context, packageID string) ([]models.PackageHook, error) {
	return runCLIQuery[models.PackageHook](ctx, r.DoltDir, r.Branch, fmt.Sprintf(
		"SELECT package_id, event, matcher, script_path, priority, blocking FROM package_hooks WHERE package_id = %s ORDER BY event, priority",
		cliSQLString(packageID),
	))
}
func (r *CLIReader) GetPackageQuestions(ctx context.Context, packageID string) ([]models.PackageQuestion, error) {
	return runCLIQuery[models.PackageQuestion](ctx, r.DoltDir, r.Branch, fmt.Sprintf(
		"SELECT package_id, question_id, prompt, type, default_val, choices, sort_order FROM package_questions WHERE package_id = %s ORDER BY sort_order, question_id",
		cliSQLString(packageID),
	))
}

func (r *CLIReader) ResolveVariant(ctx context.Context, logicalID, agentProfile string) (string, error) {
	rows, err := runCLIQuery[models.PackageVariant](ctx, r.DoltDir, r.Branch, fmt.Sprintf(
		"SELECT logical_id, agent_profile, variant_package_id FROM package_variants WHERE logical_id = %s AND agent_profile = %s",
		cliSQLString(logicalID), cliSQLString(agentProfile),
	))
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0].VariantPackageID, nil
}

func runCLIQuery[T any](ctx context.Context, doltDir, branch, query string) ([]T, error) {
	cmd := exec.CommandContext(ctx, doltCommand, "--branch", branch, "sql", "-q", query, "-r", "json") //nolint:gosec // G204: dolt binary is hardcoded constant.
	cmd.Dir = doltDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running dolt query on %s: %w: %s", branch, err, strings.TrimSpace(stderr.String()))
	}
	var result cliQueryResult[T]
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("decoding dolt query json: %w", err)
	}
	return result.Rows, nil
}

var _ Client = (*CLIReader)(nil)

func cliSQLString(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
