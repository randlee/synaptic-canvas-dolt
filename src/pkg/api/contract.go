package api

import (
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

type ErrorCode = string

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
	Retryable       bool           `json:"retryable,omitempty"`
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
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Version         string              `json:"version"`
	Branch          string              `json:"branch"`
	Description     *string             `json:"description,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
	Variant         string              `json:"variant"`
	InstallScope    models.InstallScope `json:"install_scope"`
	FileCount       int                 `json:"file_count"`
	DependencyCount int                 `json:"dependency_count"`
	SHA256          *string             `json:"sha256,omitempty"`
}

type InfoResponse struct {
	OK      bool        `json:"ok"`
	Branch  string      `json:"branch"`
	Package InfoPackage `json:"package"`
}

type InfoPackage struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Version         string              `json:"version"`
	Description     *string             `json:"description,omitempty"`
	Variant         string              `json:"variant"`
	InstallScope    models.InstallScope `json:"install_scope"`
	SHA256          *string             `json:"sha256,omitempty"`
	FileCount       int                 `json:"file_count"`
	DependencyCount int                 `json:"dependency_count"`
	Dependencies    []Dependency        `json:"dependencies"`
	HookCount       int                 `json:"hook_count"`
	QuestionCount   int                 `json:"question_count"`
}

type Dependency struct {
	Name string         `json:"name"`
	Type models.DepType `json:"type"`
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
	Package string `json:"package"`
	Scope   string `json:"scope"`
	Error   string `json:"error"`
}

type InstallResponse struct {
	OK                         bool                    `json:"ok"`
	Error                      *Error                  `json:"error,omitempty"`
	Plan                       bool                    `json:"plan"`
	Scope                      string                  `json:"scope"`
	Partial                    bool                    `json:"partial,omitempty"`
	Package                    *InstallPackageRef      `json:"package,omitempty"`
	InstallRoot                string                  `json:"install_root,omitempty"`
	FilesWritten               int                     `json:"files_written,omitempty"`
	Dependencies               []string                `json:"dependencies,omitempty"`
	DependencyWarnings         []string                `json:"dependency_warnings,omitempty"`
	HooksRegistered            []installer.HookEntry   `json:"hooks_registered,omitempty"`
	TemplateValidationWarnings []string                `json:"template_validation_warnings,omitempty"`
	Warnings                   []string                `json:"warnings,omitempty"`
	Files                      []installer.PlannedFile `json:"files,omitempty"`
	Answers                    map[string]any          `json:"answers,omitempty"`
	Installs                   []installer.Summary     `json:"installs,omitempty"`
	RolledBack                 []installer.Summary     `json:"rolled_back,omitempty"`
	Failures                   []InstallScopeFailure   `json:"failures,omitempty"`
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
	Version     string `json:"version"`
	Branch      string `json:"branch"`
	Validation  string `json:"validation"`
	InstallRoot string `json:"install_root"`
}

type ValidationSeverity string

const (
	ValidationSeverityInfo     ValidationSeverity = "info"
	ValidationSeverityWarn     ValidationSeverity = "warn"
	ValidationSeverityError    ValidationSeverity = "error"
	ValidationSeverityCritical ValidationSeverity = "critical"
)

type ValidationItem struct {
	Path     string             `json:"path"`
	Status   string             `json:"status"`
	Severity ValidationSeverity `json:"severity"`
	Error    string             `json:"error,omitempty"`
}

type ValidatedInstall struct {
	Package           string           `json:"package"`
	Version           string           `json:"version"`
	Branch            string           `json:"branch"`
	Scope             string           `json:"scope"`
	InstallRoot       string           `json:"install_root"`
	InstallSite       string           `json:"install_site"`
	Files             []ValidationItem `json:"items"`
	AggregateExpected string           `json:"aggregate_expected"`
	AggregateActual   string           `json:"aggregate_actual,omitempty"`
	AggregatePass     bool             `json:"aggregate_pass"`
	AggregateStatus   string           `json:"aggregate_status"`
	Warnings          []string         `json:"warnings,omitempty"`
	Pass              bool             `json:"pass"`
	Status            string           `json:"status"`
}

type ValidateResponse struct {
	OK       bool               `json:"ok"`
	Pass     bool               `json:"pass"`
	Packages []ValidatedInstall `json:"packages"`
}

type SnapshotResponse struct {
	OK        bool     `json:"ok"`
	Package   string   `json:"package"`
	Scope     string   `json:"scope"`
	OutputDir string   `json:"output_dir"`
	Files     []string `json:"files"`
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
