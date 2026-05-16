package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestContractSuccessEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	input := InstallResponse{
		OK:    true,
		Plan:  false,
		Scope: "project",
		Package: &InstallPackageRef{
			ID:      "team-lead",
			Version: "1.2.3",
			Branch:  "beta",
		},
		InstallRoot:  "/repo/.claude/skills/team-lead",
		FilesWritten: 1,
		HooksRegistered: []InstallHookEntry{{
			Event:    "PreToolUse",
			Matcher:  "git commit",
			Skill:    "team-lead",
			Scope:    "project",
			Script:   "/repo/.claude/skills/team-lead/hooks/pre.sh",
			Priority: 5,
			Blocking: true,
		}},
		Files: []InstallPlannedFile{{
			Path:       "SKILL.md",
			IsTemplate: false,
			Preview:    "# Team Lead",
		}},
		Answers: &InstallAnswers{
			Values: map[string]any{"repo_name": "demo"},
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var output InstallResponse
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if !reflect.DeepEqual(output, input) {
		t.Fatalf("roundtrip mismatch:\ninput=%+v\noutput=%+v", input, output)
	}
}

func TestContractErrorEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	input := ErrorEnvelope{
		OK: false,
		Error: Error{
			Code:            ErrorCodeBackendUnavailable,
			Message:         "failed to query package catalog",
			Retryable:       true,
			Details:         map[string]any{"client": "http", "cause_code": "http_timeout"},
			SuggestedAction: "retry or switch backend",
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var output ErrorEnvelope
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if !reflect.DeepEqual(output, input) {
		t.Fatalf("roundtrip mismatch:\ninput=%+v\noutput=%+v", input, output)
	}
}

func TestErrorCodeConstantsStable(t *testing.T) {
	t.Parallel()

	got := map[string]ErrorCode{
		"invalid_args":          ErrorCodeInvalidArgs,
		"not_found":             ErrorCodeNotFound,
		"ambiguous_target":      ErrorCodeAmbiguousTarget,
		"unsupported_backend":   ErrorCodeUnsupportedBackend,
		"backend_unavailable":   ErrorCodeBackendUnavailable,
		"backend_auth_failed":   ErrorCodeBackendAuthFailed,
		"confirmation_required": ErrorCodeConfirmationNeeded,
		"blocked":               ErrorCodeBlocked,
		"conflict":              ErrorCodeConflict,
		"validation_failed":     ErrorCodeValidationFailed,
		"internal_error":        ErrorCodeInternal,
	}

	if len(got) != 11 {
		t.Fatalf("len(got) = %d, want 11", len(got))
	}

	for want, code := range got {
		if string(code) != want {
			t.Fatalf("code %q = %q, want %q", want, code, want)
		}
	}
}
