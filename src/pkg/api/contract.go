package api

type ErrorCode string

const (
	ErrorCodeInvalidArgs        ErrorCode = "invalid_args"
	ErrorCodeNotFound           ErrorCode = "not_found"
	ErrorCodeAmbiguousTarget    ErrorCode = "ambiguous_target"
	ErrorCodeUnsupportedBackend ErrorCode = "unsupported_backend"
	ErrorCodeBackendUnavailable ErrorCode = "backend_unavailable"
	ErrorCodeBackendAuthFailed  ErrorCode = "backend_auth_failed"
	ErrorCodeConfirmationNeeded ErrorCode = "confirmation_required"
	ErrorCodeBlocked            ErrorCode = "blocked"
	ErrorCodeConflict           ErrorCode = "conflict"
	ErrorCodeValidationFailed   ErrorCode = "validation_failed"
	ErrorCodeInternal           ErrorCode = "internal_error"
)

type Error struct {
	Code            ErrorCode      `json:"code"`
	Message         string         `json:"message"`
	Retryable       bool           `json:"retryable"`
	Details         map[string]any `json:"details,omitempty"`
	SuggestedAction string         `json:"suggested_action,omitempty"`
}

type ErrorEnvelope struct {
	OK    bool  `json:"ok"`
	Error Error `json:"error"`
}

type ListResponse struct {
	OK       bool        `json:"ok"`
	Branch   string      `json:"branch"`
	Filters  ListFilters `json:"filters"`
	Packages []ListItem  `json:"packages"`
}

type ListFilters struct {
	Tags []string `json:"tags,omitempty"`
}

type ListItem struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Version         string       `json:"version"`
	Branch          string       `json:"branch"`
	Description     *string      `json:"description,omitempty"`
	Tags            []string     `json:"tags,omitempty"`
	Variant         string       `json:"variant"`
	InstallScope    InstallScope `json:"install_scope"`
	FileCount       int          `json:"file_count"`
	DependencyCount int          `json:"dependency_count"`
	SHA256          *string      `json:"sha256,omitempty"`
}

type InfoResponse struct {
	OK      bool        `json:"ok"`
	Branch  string      `json:"branch"`
	Package InfoPackage `json:"package"`
}

type InfoPackage struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Version         string       `json:"version"`
	Description     *string      `json:"description,omitempty"`
	Variant         string       `json:"variant"`
	InstallScope    InstallScope `json:"install_scope"`
	SHA256          *string      `json:"sha256,omitempty"`
	FileCount       int          `json:"file_count"`
	DependencyCount int          `json:"dependency_count"`
	Dependencies    []Dependency `json:"dependencies"`
	HookCount       int          `json:"hook_count"`
	QuestionCount   int          `json:"question_count"`
}

type Dependency struct {
	Name string         `json:"name"`
	Type DependencyType `json:"type"`
	Spec string         `json:"spec,omitempty"`
}

type InitResponse struct {
	OK        bool     `json:"ok"`
	Root      string   `json:"root"`
	Created   []string `json:"created"`
	Refreshed []string `json:"refreshed"`
	Warnings  []string `json:"warnings,omitempty"`
}

type InstallPackageRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Branch  string `json:"branch"`
}

type InstallScopeFailure struct {
	Package         string         `json:"package"`
	Scope           string         `json:"scope"`
	Code            ErrorCode      `json:"code,omitempty"`
	Error           string         `json:"error"`
	Retryable       bool           `json:"retryable"`
	Details         map[string]any `json:"details,omitempty"`
	SuggestedAction string         `json:"suggested_action,omitempty"`
}

type InstallScope string

type DependencyType string

type InstallHookEntry struct {
	Event    string `json:"event"`
	Matcher  string `json:"matcher"`
	Skill    string `json:"skill"`
	Scope    string `json:"scope,omitempty"`
	Script   string `json:"script"`
	Priority int    `json:"priority"`
	Blocking bool   `json:"blocking"`
}

type InstallPlannedFile struct {
	Path       string `json:"path"`
	IsTemplate bool   `json:"is_template"`
	Preview    string `json:"preview,omitempty"`
}

type InstallAnswers struct {
	Values map[string]any `json:"values,omitempty"`
}

type InstallSummary struct {
	PackageID                  string               `json:"package_id"`
	Version                    string               `json:"version"`
	Branch                     string               `json:"branch"`
	Scope                      string               `json:"scope"`
	InstallRoot                string               `json:"install_root,omitempty"`
	FilesWritten               int                  `json:"files_written,omitempty"`
	Dependencies               []string             `json:"dependencies,omitempty"`
	DependencyWarnings         []string             `json:"dependency_warnings,omitempty"`
	HooksRegistered            []InstallHookEntry   `json:"hooks_registered,omitempty"`
	TemplateValidationWarnings []string             `json:"template_validation_warnings,omitempty"`
	Warnings                   []string             `json:"warnings,omitempty"`
	Files                      []InstallPlannedFile `json:"files,omitempty"`
	Answers                    *InstallAnswers      `json:"answers,omitempty"`
}

type InstallResponse struct {
	OK                         bool                  `json:"ok"`
	Error                      *Error                `json:"error,omitempty"`
	Plan                       bool                  `json:"plan"`
	Scope                      string                `json:"scope"`
	Partial                    bool                  `json:"partial,omitempty"`
	Package                    *InstallPackageRef    `json:"package,omitempty"`
	InstallRoot                string                `json:"install_root,omitempty"`
	FilesWritten               int                   `json:"files_written,omitempty"`
	Dependencies               []string              `json:"dependencies,omitempty"`
	DependencyWarnings         []string              `json:"dependency_warnings,omitempty"`
	HooksRegistered            []InstallHookEntry    `json:"hooks_registered,omitempty"`
	TemplateValidationWarnings []string              `json:"template_validation_warnings,omitempty"`
	Warnings                   []string              `json:"warnings,omitempty"`
	Files                      []InstallPlannedFile  `json:"files,omitempty"`
	Answers                    *InstallAnswers       `json:"answers,omitempty"`
	Installs                   []InstallSummary      `json:"installs,omitempty"`
	RolledBack                 []InstallSummary      `json:"rolled_back,omitempty"`
	Failures                   []InstallScopeFailure `json:"failures,omitempty"`
}

type StatusResponse struct {
	OK       bool            `json:"ok"`
	Packages []StatusPackage `json:"packages"`
}

type StatusPackage struct {
	Package string       `json:"package"`
	Global  *StatusScope `json:"global,omitempty"`
	Local   *StatusScope `json:"local,omitempty"`
}

type StatusScope struct {
	Scope               string              `json:"scope"`
	Version             string              `json:"version"`
	Branch              string              `json:"branch"`
	Validation          string              `json:"validation"`
	AggregateStatus     string              `json:"aggregate_status,omitempty"`
	InstallRoot         string              `json:"install_root"`
	InstallSite         string              `json:"install_site"`
	TrackingOrigin      string              `json:"tracking_origin,omitempty"`
	DependencySummary   DependencySummary   `json:"dependency_summary"`
	HookSummary         HookSummary         `json:"hook_summary"`
	ModificationSummary ModificationSummary `json:"modification_summary"`
	Modifications       []ValidationItem    `json:"modifications,omitempty"`
	Issues              []ValidationItem    `json:"issues,omitempty"`
}

type ValidationSeverity string

type ValidationKind string

type ValidationState string

const (
	ValidationSeverityInfo     ValidationSeverity = "info"
	ValidationSeverityWarn     ValidationSeverity = "warn"
	ValidationSeverityError    ValidationSeverity = "error"
	ValidationSeverityCritical ValidationSeverity = "critical"

	ValidationKindFile       ValidationKind = "file"
	ValidationKindDependency ValidationKind = "dependency"
	ValidationKindHook       ValidationKind = "hook"
	ValidationKindTemplate   ValidationKind = "template"
	ValidationKindAggregate  ValidationKind = "aggregate"
	ValidationKindContext    ValidationKind = "context"

	ValidationStateOK         ValidationState = "ok"
	ValidationStateModified   ValidationState = "modified"
	ValidationStateMissing    ValidationState = "missing"
	ValidationStateUnreadable ValidationState = "unreadable"
	ValidationStateExtra      ValidationState = "extra"
)

type ValidationItem struct {
	Kind           ValidationKind     `json:"kind"`
	Severity       ValidationSeverity `json:"severity"`
	State          ValidationState    `json:"state"`
	Code           string             `json:"code,omitempty"`
	Message        string             `json:"message,omitempty"`
	Path           string             `json:"path,omitempty"`
	Dependency     string             `json:"dependency,omitempty"`
	DependencyType string             `json:"dependency_type,omitempty"`
	Provenance     string             `json:"provenance,omitempty"`
	InstalledBySC  bool               `json:"installed_by_sc,omitempty"`
	HookEvent      string             `json:"hook_event,omitempty"`
	HookMatcher    string             `json:"hook_matcher,omitempty"`
	HookScript     string             `json:"hook_script,omitempty"`
	Scope          string             `json:"scope,omitempty"`
	Expected       string             `json:"expected,omitempty"`
	Actual         string             `json:"actual,omitempty"`
	Target         string             `json:"target,omitempty"`
}

type DependencyReadback struct {
	Name           string `json:"name"`
	DependencyType string `json:"dependency_type"`
	Verified       bool   `json:"verified"`
	Provenance     string `json:"provenance,omitempty"`
	InstalledBySC  bool   `json:"installed_by_sc"`
}

type DependencySummary struct {
	Tracked  int                  `json:"tracked"`
	Verified int                  `json:"verified"`
	Missing  int                  `json:"missing"`
	Items    []DependencyReadback `json:"items,omitempty"`
}

type HookValidationState struct {
	Event      string `json:"hook_event,omitempty"`
	Matcher    string `json:"hook_matcher,omitempty"`
	Script     string `json:"hook_script"`
	Scope      string `json:"scope,omitempty"`
	Priority   int    `json:"priority,omitempty"`
	Blocking   bool   `json:"blocking,omitempty"`
	Registered bool   `json:"registered"`
}

type HookSummary struct {
	Tracked    int                   `json:"tracked"`
	Registered int                   `json:"registered"`
	Missing    int                   `json:"missing"`
	Hooks      []HookValidationState `json:"hooks,omitempty"`
}

type ModificationSummary struct {
	OK         int `json:"ok"`
	Modified   int `json:"modified"`
	Missing    int `json:"missing"`
	Unreadable int `json:"unreadable"`
	Extra      int `json:"extra"`
}

type ValidatedInstall struct {
	Package             string              `json:"package"`
	Version             string              `json:"version"`
	Branch              string              `json:"branch"`
	Scope               string              `json:"scope"`
	InstallRoot         string              `json:"install_root"`
	InstallSite         string              `json:"install_site"`
	TrackingOrigin      string              `json:"tracking_origin,omitempty"`
	Items               []ValidationItem    `json:"items"`
	AggregateExpected   string              `json:"aggregate_expected"`
	AggregateActual     string              `json:"aggregate_actual,omitempty"`
	AggregatePass       bool                `json:"aggregate_pass"`
	AggregateStatus     string              `json:"aggregate_status"`
	DependencySummary   DependencySummary   `json:"dependency_summary"`
	HookSummary         HookSummary         `json:"hook_summary"`
	ModificationSummary ModificationSummary `json:"modification_summary"`
	Warnings            []string            `json:"warnings,omitempty"`
	Pass                bool                `json:"pass"`
	Status              string              `json:"status"`
}

type ValidateResponse struct {
	OK       bool               `json:"ok"`
	Pass     bool               `json:"pass"`
	Packages []ValidatedInstall `json:"packages"`
}

type SnapshotResponse struct {
	OK           bool     `json:"ok"`
	Package      string   `json:"package"`
	Version      string   `json:"version"`
	Branch       string   `json:"branch"`
	Scope        string   `json:"scope"`
	InstallRoot  string   `json:"install_root"`
	InstallSite  string   `json:"install_site"`
	OutputDir    string   `json:"output_dir"`
	MetadataPath string   `json:"metadata_path"`
	Files        []string `json:"files"`
}

type UpgradeResult struct {
	Package            string   `json:"package"`
	Scope              string   `json:"scope"`
	FromVersion        string   `json:"from_version"`
	ToVersion          string   `json:"to_version"`
	FromBranch         string   `json:"from_branch"`
	ToBranch           string   `json:"to_branch"`
	InstallRoot        string   `json:"install_root"`
	Warnings           []string `json:"warnings,omitempty"`
	Skipped            bool     `json:"skipped,omitempty"`
	FilesWritten       int      `json:"files_written"`
	TemplateWarnings   []string `json:"template_warnings,omitempty"`
	DependencyWarnings []string `json:"dependency_warnings,omitempty"`
}

type UpgradeResponse struct {
	OK       bool            `json:"ok"`
	Upgrades []UpgradeResult `json:"upgrades"`
}

type UninstallResult struct {
	Package             string   `json:"package"`
	Scope               string   `json:"scope"`
	RemovedFiles        []string `json:"removed_files"`
	RemovedDependencies []string `json:"removed_dependencies,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
	HooksRemoved        int      `json:"hooks_removed"`
}

type UninstallResponse struct {
	OK         bool              `json:"ok"`
	Removed    UninstallResult   `json:"removed"`
	RemovedAll []UninstallResult `json:"removed_all,omitempty"`
}

type ConfigGetResponse struct {
	OK    bool   `json:"ok"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ConfigSetResponse struct {
	OK   bool   `json:"ok"`
	Key  string `json:"key"`
	Path string `json:"path"`
}

type CatalogUpdateResponse struct {
	OK      bool     `json:"ok"`
	Branch  string   `json:"branch"`
	Entries int      `json:"entries"`
	Path    string   `json:"path"`
	Paths   []string `json:"paths,omitempty"`
}

type ScanCandidateFile struct {
	Path    string `json:"path"`
	DocPath string `json:"doc_path"`
	SHA256  string `json:"sha256"`
}

type ScanCandidate struct {
	Package         string              `json:"package"`
	Version         string              `json:"version"`
	Branch          string              `json:"branch"`
	Scope           string              `json:"scope"`
	InstallRoot     string              `json:"install_root"`
	InstallSite     string              `json:"install_site"`
	TrackingOrigin  string              `json:"tracking_origin"`
	NeedsUpgrade    bool                `json:"needs_upgrade"`
	ExistingVersion string              `json:"existing_version,omitempty"`
	ExistingBranch  string              `json:"existing_branch,omitempty"`
	Files           []ScanCandidateFile `json:"files"`
}

type ScanResponse struct {
	OK         bool            `json:"ok"`
	Branch     string          `json:"branch"`
	Mutated    bool            `json:"mutated"`
	Accepted   int             `json:"accepted"`
	Upgraded   int             `json:"upgraded"`
	Candidates []ScanCandidate `json:"candidates"`
	Warnings   []string        `json:"warnings,omitempty"`
}
