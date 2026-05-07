package cmd

import (
	"errors"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/internal/output"
)

type jsonErrorEnvelope struct {
	OK    bool             `json:"ok"`
	Error jsonErrorPayload `json:"error"`
}

type jsonErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

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

func writeJSONError(formatter *output.Formatter, code, message string) error {
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

func classifyJSONError(message string) string {
	if strings.Contains(strings.ToLower(message), "not found") {
		return "not_found"
	}
	return "query_failed"
}
