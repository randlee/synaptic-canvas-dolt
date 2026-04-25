package cmd

import (
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

func writeJSONError(formatter *output.Formatter, code, message string) error {
	return formatter.WriteJSON(jsonErrorEnvelope{
		OK: false,
		Error: jsonErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}

func classifyJSONError(message string) string {
	if strings.Contains(strings.ToLower(message), "not found") {
		return "not_found"
	}
	return "query_failed"
}
