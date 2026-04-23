package template

import "testing"

func TestValidateKnownGoodTemplate(t *testing.T) {
	report := Validate(map[string]string{
		"skills/example.md.j2": `{{ repo.name }} {% if answers.style == "x" %}{{ env.synaptic_channel }}{% endif %}`,
	}, []string{"style"})

	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", report.Errors)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", report.Warnings)
	}
}

func TestValidateKnownBadTemplate(t *testing.T) {
	report := Validate(map[string]string{
		"skills/example.md.j2": `{{ answers.missing }} {{ repo.unknown }} {{ foo.bar }}`,
	}, []string{"style"})

	if len(report.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d (%+v)", len(report.Errors), report.Errors)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("expected 1 warning for unused question, got %d", len(report.Warnings))
	}
}
