package cmd

import (
	"errors"
	"strings"

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

func writeJSONError(formatter *output.Formatter, code api.ErrorCode, message string) error {
	if err := formatter.WriteJSON(jsonErrorEnvelope{
		OK: false,
		Error: jsonErrorPayload{
			Code:    code,
			Message: message,
		},
	}); err != nil {
		return err
	}
	return jsonCmdError{cause: errors.New(message)}
}

func classifyJSONError(message string) api.ErrorCode {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "not found"):
		return api.ErrorCodeNotFound
	case strings.Contains(lower, "unsupported dolt.client"), strings.Contains(lower, "unsupported backend"):
		return api.ErrorCodeUnsupportedBackend
	case strings.Contains(lower, "unauthorized"), strings.Contains(lower, "forbidden"), strings.Contains(lower, "access denied"), strings.Contains(lower, "authentication"):
		return api.ErrorCodeBackendAuthFailed
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
	case errors.Is(err, dolt.ErrNotFound):
		return api.ErrorCodeNotFound
	default:
		return classifyJSONError(err.Error())
	}
}
