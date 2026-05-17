package cmd

import (
	"errors"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
)

type jsonErrorEnvelope = api.ErrorEnvelope
type jsonErrorPayload = api.Error

// jsonCmdError is a sentinel error for JSON-mode failures that have already
// rendered their standard error envelope and should not be printed again.
type jsonCmdError struct {
	cause error
}

func (e jsonCmdError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return "json command failed"
}

func (e jsonCmdError) Unwrap() error {
	return e.cause
}

func isJSONCmdError(err error) bool {
	var target jsonCmdError
	return errors.As(err, &target)
}

// IsJSONCmdError reports whether err represents a JSON-mode command failure
// that has already been rendered to stdout/stderr.
func IsJSONCmdError(err error) bool {
	return isJSONCmdError(err)
}

// JSONErrorExitCode is intentionally shared with all command failures so scripts
// only need to check non-zero status while reading the structured JSON envelope.
func JSONErrorExitCode(error) int {
	return 1
}

func writeJSONError(formatter *output.Formatter, code api.ErrorCode, message string, details ...map[string]any) error {
	var detailMap map[string]any
	if len(details) > 0 {
		detailMap = details[0]
	}
	return writeStructuredJSONError(formatter, api.NewError(code, message, api.ErrorOptions{Details: detailMap}))
}

func writeStructuredJSONError(formatter *output.Formatter, payload api.Error) error {
	envelope := jsonErrorEnvelope{
		OK:    false,
		Error: payload,
	}
	if err := formatter.WriteJSON(envelope); err != nil {
		return err
	}
	return jsonCmdError{cause: errors.New(payload.Message)}
}

func writeClassifiedJSONError(formatter *output.Formatter, cfg *config.Config, err error, operation ...string) error {
	return writeStructuredJSONError(formatter, classifyRuntimeJSONError(cfg, err, operation...))
}

func classifyRuntimeJSONError(cfg *config.Config, err error, operation ...string) api.Error {
	code := classifyJSONErr(err)
	metadata := jsonErrorMetadata(cfg, code, err, operation...)
	return api.NewError(code, err.Error(), api.ErrorOptions{
		Retryable:       metadata.Retryable,
		Details:         metadata.Details,
		SuggestedAction: metadata.SuggestedAction,
	})
}

type runtimeJSONErrorMetadata struct {
	Retryable       bool
	Details         map[string]any
	SuggestedAction string
}

func jsonErrorMetadata(cfg *config.Config, code api.ErrorCode, err error, operation ...string) runtimeJSONErrorMetadata {
	metadata := runtimeJSONErrorMetadata{
		Retryable: false,
	}
	switch code {
	case api.ErrorCodeBackendUnavailable, api.ErrorCodeBackendAuthFailed, api.ErrorCodeUnsupportedBackend:
		metadata.Details = jsonErrorDetails(cfg, code, err, operation...)
	}
	switch code {
	case api.ErrorCodeBackendUnavailable:
		metadata.Retryable = jsonErrorRetryable(err)
		if metadata.Retryable {
			metadata.SuggestedAction = "retry or switch to a reachable backend"
		} else {
			metadata.SuggestedAction = "verify backend configuration and retry"
		}
	case api.ErrorCodeBackendAuthFailed:
		metadata.Retryable = false
		metadata.SuggestedAction = "configure backend credentials and retry"
	case api.ErrorCodeConfirmationNeeded:
		metadata.Retryable = false
		metadata.SuggestedAction = "rerun with --yolo to proceed non-interactively"
	}
	return metadata
}

func jsonErrorRetryable(err error) bool {
	switch {
	case errors.Is(err, dolt.ErrRateLimited), errors.Is(err, dolt.ErrServerError):
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "timeout") || strings.Contains(lower, "connection")
}

func jsonErrorCauseCode(err error) string {
	switch {
	case errors.Is(err, dolt.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, dolt.ErrServerError):
		return "server_error"
	case errors.Is(err, dolt.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, dolt.ErrBadQuery):
		return "bad_query"
	case errors.Is(err, dolt.ErrNotFound):
		return "not_found"
	case errors.Is(err, dolt.ErrUnsupportedBackend):
		return "unsupported_backend"
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "timeout"):
		return "timeout"
	case strings.Contains(lower, "connection"):
		return "connection_failed"
	case strings.Contains(lower, "dsn"):
		return "dsn_invalid"
	case strings.Contains(lower, "database"):
		return "database_unconfigured"
	default:
		return ""
	}
}

func effectiveOperation(operation []string) string {
	if len(operation) == 0 {
		return ""
	}
	return strings.TrimSpace(operation[0])
}

func jsonErrorDetails(cfg *config.Config, code api.ErrorCode, err error, operation ...string) map[string]any {
	var detailMap map[string]any
	switch code {
	case api.ErrorCodeUnsupportedBackend, api.ErrorCodeBackendUnavailable, api.ErrorCodeBackendAuthFailed:
	default:
		return nil
	}
	client := "http"
	if cfg != nil {
		if selection, resolveErr := cfg.ResolveDoltClient(); resolveErr == nil && selection.Client != "" {
			client = selection.Client
		}
	}
	detailMap = map[string]any{"client": client}
	if causeCode := jsonErrorCauseCode(err); causeCode != "" {
		detailMap["cause_code"] = causeCode
	}
	if op := effectiveOperation(operation); op != "" {
		detailMap["operation"] = op
	}
	return detailMap
}

func classifyJSONError(message string) api.ErrorCode {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "not found"):
		return api.ErrorCodeNotFound
	case strings.Contains(lower, "invalid --scope"),
		strings.Contains(lower, "requires <package> or --all"),
		strings.Contains(lower, "cannot be used with --all"),
		strings.Contains(lower, "cannot be installed globally"),
		strings.Contains(lower, "no install scopes were eligible"):
		return api.ErrorCodeInvalidArgs
	case strings.Contains(lower, "--dolt-dir may only be used with client=cli"):
		return api.ErrorCodeInvalidArgs
	case strings.Contains(lower, "unsupported dolt.client"), strings.Contains(lower, "unsupported backend"):
		return api.ErrorCodeUnsupportedBackend
	case strings.Contains(lower, "unauthorized"), strings.Contains(lower, "forbidden"), strings.Contains(lower, "access denied"), strings.Contains(lower, "authentication"):
		return api.ErrorCodeBackendAuthFailed
	case strings.Contains(lower, "interactive confirmation required"), strings.Contains(lower, "use --yolo"):
		return api.ErrorCodeConfirmationNeeded
	case strings.Contains(lower, "multiple scopes"), strings.Contains(lower, "pass --scope"), strings.Contains(lower, "ambiguous"):
		return api.ErrorCodeAmbiguousTarget
	case strings.Contains(lower, "validation failed"):
		return api.ErrorCodeValidationFailed
	case strings.Contains(lower, "blocked"), strings.Contains(lower, "cannot be upgraded"), strings.Contains(lower, "incompatible dependency"):
		return api.ErrorCodeBlocked
	case strings.Contains(lower, "conflict"):
		return api.ErrorCodeConflict
	case strings.Contains(lower, "dolt."), strings.Contains(lower, "dolthub"), strings.Contains(lower, "timeout"), strings.Contains(lower, "connection"), strings.Contains(lower, "dsn"), strings.Contains(lower, "database"):
		return api.ErrorCodeBackendUnavailable
	default:
		return api.ErrorCodeInternal
	}
}

func classifyJSONErr(err error) api.ErrorCode {
	switch {
	case errors.Is(err, dolt.ErrUnauthorized):
		return api.ErrorCodeBackendAuthFailed
	case errors.Is(err, dolt.ErrServerError), errors.Is(err, dolt.ErrRateLimited):
		return api.ErrorCodeBackendUnavailable
	case errors.Is(err, dolt.ErrBadQuery):
		return api.ErrorCodeBackendUnavailable
	case errors.Is(err, dolt.ErrUnsupportedBackend):
		return api.ErrorCodeUnsupportedBackend
	case errors.Is(err, dolt.ErrNotFound):
		return api.ErrorCodeNotFound
	default:
		return classifyJSONError(err.Error())
	}
}
