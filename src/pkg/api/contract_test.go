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

func TestContractInstallResponseAllScopesFailedTypedSuberrors(t *testing.T) {
	t.Parallel()

	input := InstallResponse{
		OK:      false,
		Plan:    false,
		Scope:   "both",
		Partial: false,
		Error: &Error{
			Code:            ErrorCodeBlocked,
			Message:         "install failed for all selected scopes",
			Retryable:       false,
			Details:         map[string]any{"operation": "install_scope"},
			SuggestedAction: "install required dependencies before retrying",
		},
		Failures: []InstallScopeFailure{
			{
				Package:         "team-lead",
				Scope:           "project",
				Code:            ErrorCodeBlocked,
				Error:           "dependency verification failed",
				Retryable:       false,
				Details:         map[string]any{"operation": "install_scope", "cause_code": "dependency_blocked"},
				SuggestedAction: "install required dependencies before retrying",
			},
			{
				Package:         "team-lead",
				Scope:           "global",
				Code:            ErrorCodeConfirmationNeeded,
				Error:           "interactive confirmation required; use --yolo to proceed non-interactively",
				Retryable:       false,
				Details:         map[string]any{"operation": "install_scope", "cause_code": "confirmation_required"},
				SuggestedAction: "rerun with --yolo to proceed non-interactively",
			},
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
	if len(output.Failures) != 2 {
		t.Fatalf("len(output.Failures) = %d, want 2", len(output.Failures))
	}
	if output.Failures[0].Code != ErrorCodeBlocked || output.Failures[1].Code != ErrorCodeConfirmationNeeded {
		t.Fatalf("typed per-scope codes were not preserved: %+v", output.Failures)
	}
}

func TestContractInstallResponsePartialFailureMixedTypedSuberrors(t *testing.T) {
	t.Parallel()

	input := InstallResponse{
		OK:      false,
		Plan:    false,
		Scope:   "both",
		Partial: true,
		Error: &Error{
			Code:            ErrorCodeBackendUnavailable,
			Message:         "install failed for one or more scopes",
			Retryable:       true,
			Details:         map[string]any{"operation": "install_scope", "cause_code": "rate_limited"},
			SuggestedAction: "retry or switch to a reachable backend",
		},
		Failures: []InstallScopeFailure{
			{
				Package:         "team-lead",
				Scope:           "project",
				Code:            ErrorCodeBackendUnavailable,
				Error:           "rate limited by backend",
				Retryable:       true,
				Details:         map[string]any{"operation": "install_scope", "cause_code": "rate_limited"},
				SuggestedAction: "retry or switch to a reachable backend",
			},
			{
				Package:         "team-lead",
				Scope:           "global",
				Code:            ErrorCodeBlocked,
				Error:           "local modifications block overwrite",
				Retryable:       false,
				Details:         map[string]any{"operation": "install_scope", "cause_code": "modified_files"},
				SuggestedAction: "resolve local modifications or uninstall before retrying",
			},
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
	if output.Failures[0].Code != ErrorCodeBackendUnavailable || output.Failures[1].Code != ErrorCodeBlocked {
		t.Fatalf("mixed typed sub-errors were not preserved: %+v", output.Failures)
	}
}
