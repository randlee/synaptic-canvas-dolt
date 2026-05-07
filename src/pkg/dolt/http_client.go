package dolt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

const DefaultHTTPTimeout = 30 * time.Second
const maxHTTPQueryURLLength = 1800

// HTTPConfig holds all parameters for NewHTTPClient.
type HTTPConfig struct {
	Host     string
	Database string
	Branch   string
	Token    string
	Timeout  time.Duration
}

// HTTPClient implements Client using DoltHub's HTTP SQL API.
type HTTPClient struct {
	baseURL  string
	database string
	branch   string
	token    string
	http     *http.Client
}

type apiResponse struct {
	QueryExecutionStatus  string           `json:"query_execution_status"`
	QueryExecutionMessage string           `json:"query_execution_message"`
	RepositoryOwner       string           `json:"repository_owner"`
	RepositoryName        string           `json:"repository_name"`
	CommitRef             string           `json:"commit_ref"`
	SQLQuery              string           `json:"sql_query"`
	Schema                []apiColumn      `json:"schema"`
	Rows                  []map[string]any `json:"rows"`
}

type apiColumn struct {
	Name string `json:"columnName"`
	Type string `json:"columnType"`
}

// NewHTTPClient creates a DoltHub HTTP SQL client.
func NewHTTPClient(cfg HTTPConfig) *HTTPClient {
	host := strings.TrimRight(cfg.Host, "/")
	if host == "" {
		host = "www.dolthub.com"
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}
	return &HTTPClient{
		baseURL:  host + "/api/v1alpha1",
		database: cfg.Database,
		branch:   cfg.Branch,
		token:    cfg.Token,
		http:     &http.Client{Timeout: timeout},
	}
}

func (c *HTTPClient) Close() error {
	return nil
}

func (c *HTTPClient) query(ctx context.Context, sql string) ([]map[string]any, error) {
	if c.database == "" {
		return nil, fmt.Errorf("dolt.database is not configured; run: sc config set dolt.database <owner/database>")
	}
	owner, database, err := splitDatabaseSlug(c.database)
	if err != nil {
		return nil, err
	}
	branch := c.branch
	if branch == "" {
		branch = "main"
	}

	endpoint := fmt.Sprintf("%s/%s/%s/%s",
		c.baseURL,
		url.PathEscape(owner),
		url.PathEscape(database),
		url.PathEscape(branch),
	)
	values := url.Values{"q": []string{sql}}
	requestURL := endpoint + "?" + values.Encode()
	if len(requestURL) > maxHTTPQueryURLLength {
		return nil, fmt.Errorf("%w: generated DoltHub GET query exceeds URL length budget", ErrBadQuery)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating DoltHub request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	resp, err := c.http.Do(req) //nolint:gosec // G704: URL comes from validated config, and requests are only issued by explicit CLI configuration.
	if err != nil {
		return nil, fmt.Errorf("querying DoltHub: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("%w: configure dolt.token with sc config set dolt.token <token>", ErrUnauthorized)
	case http.StatusNotFound:
		return nil, fmt.Errorf("%w: DoltHub repository or branch not found", ErrNotFound)
	case http.StatusTooManyRequests:
		retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After"))
		if retryAfter != "" {
			return nil, fmt.Errorf("%w: DoltHub returned HTTP 429; retry_after=%s", ErrRateLimited, retryAfter)
		}
		return nil, fmt.Errorf("%w: DoltHub returned HTTP 429", ErrRateLimited)
	default:
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("%w: DoltHub returned HTTP %d", ErrServerError, resp.StatusCode)
		}
		return nil, fmt.Errorf("%w: DoltHub returned HTTP %d", ErrBadQuery, resp.StatusCode)
	}

	var parsed apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decoding DoltHub response: %w", err)
	}
	if parsed.QueryExecutionStatus != "" && !strings.EqualFold(parsed.QueryExecutionStatus, "success") {
		return nil, fmt.Errorf("%w: %s", ErrBadQuery, parsed.QueryExecutionMessage)
	}
	if parsed.Rows == nil {
		return []map[string]any{}, nil
	}
	return parsed.Rows, nil
}

func (c *HTTPClient) ListPackages(ctx context.Context, opts ListOptions) ([]models.Package, error) {
	rows, err := c.query(ctx, httpListPackagesSQL(opts.Tags))
	if err != nil {
		return nil, err
	}
	packages := make([]models.Package, 0, len(rows))
	for _, row := range rows {
		packages = append(packages, packageFromRow(row, true))
	}
	return packages, nil
}

func (c *HTTPClient) GetPackage(ctx context.Context, id string) (*models.Package, error) {
	rows, err := c.query(ctx, "SELECT id, name, version, description, agent_variant, author, license, tags, install_scope, variables, options, sha256, min_claude_version FROM packages WHERE id = "+httpSQLString(id))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	pkg := packageFromRow(rows[0], false)
	return &pkg, nil
}

func (c *HTTPClient) GetPackageDetail(ctx context.Context, id string) (*models.Package, error) {
	rows, err := c.query(ctx, httpPackageDetailSQL(id))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	pkg := packageFromRow(rows[0], true)
	return &pkg, nil
}

func (c *HTTPClient) GetPackageFiles(ctx context.Context, packageID string) ([]models.PackageFile, error) {
	rows, err := c.query(ctx, "SELECT package_id, dest_path, content, sha256, file_type, content_type, is_template, frontmatter, fm_name, fm_description, fm_version, fm_model FROM package_files WHERE package_id = "+httpSQLString(packageID)+" ORDER BY dest_path")
	if err != nil {
		return nil, err
	}
	files := make([]models.PackageFile, 0, len(rows))
	for _, row := range rows {
		files = append(files, models.PackageFile{
			PackageID:     stringValue(row, "package_id"),
			DestPath:      stringValue(row, "dest_path"),
			Content:       stringValue(row, "content"),
			SHA256:        stringValue(row, "sha256"),
			FileType:      models.FileType(stringValue(row, "file_type")),
			ContentType:   models.ContentType(stringValue(row, "content_type")),
			IsTemplate:    boolValue(row, "is_template"),
			Frontmatter:   rawMessageValue(row, "frontmatter"),
			FMName:        stringPointerValue(row, "fm_name"),
			FMDescription: stringPointerValue(row, "fm_description"),
			FMVersion:     stringPointerValue(row, "fm_version"),
			FMModel:       stringPointerValue(row, "fm_model"),
		})
	}
	return files, nil
}

func (c *HTTPClient) GetPackageFileSHAs(ctx context.Context, packageID, docPath string) ([]PackageFileSHA, error) {
	rows, err := c.query(ctx, `SELECT pf.package_id, p.version, pf.dest_path AS doc_path, pf.sha256
FROM package_files AS pf
JOIN packages AS p ON p.id = pf.package_id
WHERE pf.package_id = `+httpSQLString(packageID)+` AND pf.dest_path = `+httpSQLString(docPath)+`
ORDER BY p.version`)
	if err != nil {
		return nil, err
	}
	result := make([]PackageFileSHA, 0, len(rows))
	for _, row := range rows {
		result = append(result, PackageFileSHA{
			PackageID: stringValue(row, "package_id"),
			Version:   stringValue(row, "version"),
			DocPath:   stringValue(row, "doc_path"),
			SHA256:    stringValue(row, "sha256"),
		})
	}
	return result, nil
}

func (c *HTTPClient) GetPackageDeps(ctx context.Context, packageID string) ([]models.PackageDep, error) {
	rows, err := c.query(ctx, "SELECT package_id, dep_type, dep_name, dep_spec, install_cmd, cmd_sha256 FROM package_deps WHERE package_id = "+httpSQLString(packageID)+" ORDER BY dep_name")
	if err != nil {
		return nil, err
	}
	deps := make([]models.PackageDep, 0, len(rows))
	for _, row := range rows {
		deps = append(deps, models.PackageDep{
			PackageID:  stringValue(row, "package_id"),
			DepType:    models.DepType(stringValue(row, "dep_type")),
			DepName:    stringValue(row, "dep_name"),
			DepSpec:    stringValue(row, "dep_spec"),
			InstallCmd: stringValue(row, "install_cmd"),
			CmdSHA256:  stringValue(row, "cmd_sha256"),
		})
	}
	return deps, nil
}

func (c *HTTPClient) GetPackageHooks(ctx context.Context, packageID string) ([]models.PackageHook, error) {
	rows, err := c.query(ctx, "SELECT package_id, event, matcher, script_path, priority, blocking FROM package_hooks WHERE package_id = "+httpSQLString(packageID)+" ORDER BY event, priority")
	if err != nil {
		return nil, err
	}
	hooks := make([]models.PackageHook, 0, len(rows))
	for _, row := range rows {
		hooks = append(hooks, models.PackageHook{
			PackageID:  stringValue(row, "package_id"),
			Event:      models.HookEvent(stringValue(row, "event")),
			Matcher:    stringValue(row, "matcher"),
			ScriptPath: stringValue(row, "script_path"),
			Priority:   intValue(row, "priority"),
			Blocking:   boolValue(row, "blocking"),
		})
	}
	return hooks, nil
}

func (c *HTTPClient) GetPackageQuestions(ctx context.Context, packageID string) ([]models.PackageQuestion, error) {
	rows, err := c.query(ctx, "SELECT package_id, question_id, prompt, type, default_val, choices, sort_order FROM package_questions WHERE package_id = "+httpSQLString(packageID)+" ORDER BY sort_order, question_id")
	if err != nil {
		return nil, err
	}
	questions := make([]models.PackageQuestion, 0, len(rows))
	for _, row := range rows {
		questions = append(questions, models.PackageQuestion{
			PackageID:  stringValue(row, "package_id"),
			QuestionID: stringValue(row, "question_id"),
			Prompt:     stringValue(row, "prompt"),
			Type:       models.QuestionType(stringValue(row, "type")),
			DefaultVal: stringValue(row, "default_val"),
			Choices:    stringValue(row, "choices"),
			SortOrder:  intValue(row, "sort_order"),
		})
	}
	return questions, nil
}

func (c *HTTPClient) ResolveVariant(ctx context.Context, logicalID, agentProfile string) (string, error) {
	rows, err := c.query(ctx, "SELECT variant_package_id FROM package_variants WHERE logical_id = "+httpSQLString(logicalID)+" AND agent_profile = "+httpSQLString(agentProfile))
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	return stringValue(rows[0], "variant_package_id"), nil
}

func (c *HTTPClient) GetPackageCatalog(ctx context.Context) ([]catalog.CatalogEntry, error) {
	rows, err := c.query(ctx, `SELECT f.package_id, p.version, f.dest_path AS doc_path, f.sha256
FROM package_files AS f
JOIN packages AS p ON p.id = f.package_id
WHERE COALESCE(f.sha256, '') <> ''
ORDER BY f.package_id, p.version, f.dest_path`)
	if err != nil {
		return nil, err
	}
	entries := make([]catalog.CatalogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, catalog.CatalogEntry{
			PackageID: stringValue(row, "package_id"),
			Version:   stringValue(row, "version"),
			DocPath:   stringValue(row, "doc_path"),
			SHA256:    stringValue(row, "sha256"),
		})
	}
	return catalog.SortedEntries(entries), nil
}

func httpListPackagesSQL(tags []string) string {
	query := `SELECT p.id, p.name, p.version, p.description, p.agent_variant, p.tags, p.install_scope, p.sha256,
COALESCE(fc.file_count, 0) AS file_count, COALESCE(dc.dep_count, 0) AS dep_count
FROM packages AS p
LEFT JOIN (
  SELECT package_id, COUNT(*) AS file_count FROM package_files GROUP BY package_id
) AS fc ON p.id = fc.package_id
LEFT JOIN (
  SELECT package_id, COUNT(*) AS dep_count FROM package_deps GROUP BY package_id
) AS dc ON p.id = dc.package_id`
	if predicate := httpTagPredicate(tags); predicate != "" {
		query += " WHERE " + predicate
	}
	return query + " ORDER BY p.name"
}

func httpPackageDetailSQL(id string) string {
	return `SELECT p.id, p.name, p.version, p.description, p.agent_variant, p.author, p.license, p.tags, p.install_scope, p.variables, p.options, p.sha256, p.min_claude_version,
COALESCE(fc.file_count, 0) AS file_count, COALESCE(dc.dep_count, 0) AS dep_count
FROM packages AS p
LEFT JOIN (
  SELECT package_id, COUNT(*) AS file_count FROM package_files GROUP BY package_id
) AS fc ON p.id = fc.package_id
LEFT JOIN (
  SELECT package_id, COUNT(*) AS dep_count FROM package_deps GROUP BY package_id
) AS dc ON p.id = dc.package_id
WHERE p.id = ` + httpSQLString(id)
}

func httpTagPredicate(tags []string) string {
	clauses := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			continue
		}
		clauses = append(clauses, "LOWER(CONCAT(',', REPLACE(COALESCE(p.tags, ''), ' ', ''), ',')) LIKE "+httpSQLString("%,"+normalized+",%"))
	}
	return strings.Join(clauses, " OR ")
}

func packageFromRow(row map[string]any, withCounts bool) models.Package {
	return models.Package{
		ID:           stringValue(row, "id"),
		Name:         stringValue(row, "name"),
		Version:      stringValue(row, "version"),
		Description:  stringPointerValue(row, "description"),
		AgentVariant: stringValue(row, "agent_variant"),
		Author:       stringPointerValue(row, "author"),
		License:      stringPointerValue(row, "license"),
		Tags:         stringValue(row, "tags"),
		InstallScope: models.InstallScope(stringValue(row, "install_scope")),
		Variables:    rawMessageValue(row, "variables"),
		Options:      rawMessageValue(row, "options"),
		SHA256:       stringPointerValue(row, "sha256"),
		MinClaudeVer: stringPointerValue(row, "min_claude_version"),
		FileCount:    countValue(row, "file_count", withCounts),
		DepCount:     countValue(row, "dep_count", withCounts),
	}
}

func splitDatabaseSlug(database string) (string, string, error) {
	parts := strings.Split(database, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w: dolt.database must be <owner/database>", ErrBadQuery)
	}
	return parts[0], parts[1], nil
}

func httpSQLString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func stringValue(row map[string]any, key string) string {
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}

func stringPointerValue(row map[string]any, key string) *string {
	value := stringValue(row, key)
	if value == "" {
		return nil
	}
	return &value
}

func intValue(row map[string]any, key string) int {
	value, ok := row[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case string:
		parsed, _ := strconv.Atoi(v)
		return parsed
	case json.Number:
		parsed, _ := strconv.Atoi(v.String())
		return parsed
	case float64:
		return int(v)
	case int:
		return v
	default:
		parsed, _ := strconv.Atoi(fmt.Sprint(v))
		return parsed
	}
}

func countValue(row map[string]any, key string, enabled bool) int {
	if !enabled {
		return 0
	}
	return intValue(row, key)
}

func boolValue(row map[string]any, key string) bool {
	value, ok := row[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "1" || strings.EqualFold(v, "true")
	case json.Number:
		return v.String() != "0"
	case float64:
		return v != 0
	default:
		return fmt.Sprint(v) == "1"
	}
}

func rawMessageValue(row map[string]any, key string) json.RawMessage {
	value, ok := row[key]
	if !ok || value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return json.RawMessage(v)
	case json.RawMessage:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return data
	}
}

var _ Client = (*HTTPClient)(nil)
