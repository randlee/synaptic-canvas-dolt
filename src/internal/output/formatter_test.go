package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestTableOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	f := &Formatter{JSON: false, Quiet: false, Writer: &buf}

	headers := []string{"Name", "Version"}
	rows := [][]string{
		{"foo", "1.0.0"},
		{"bar", "2.3.1"},
	}
	if err := f.Table(headers, rows); err != nil {
		t.Fatalf("Table returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Name") {
		t.Error("table output should contain header 'Name'")
	}
	if !strings.Contains(output, "Version") {
		t.Error("table output should contain header 'Version'")
	}
	if !strings.Contains(output, "foo") {
		t.Error("table output should contain row value 'foo'")
	}
	if !strings.Contains(output, "2.3.1") {
		t.Error("table output should contain row value '2.3.1'")
	}
}

func TestTableOutputQuiet(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	f := &Formatter{JSON: false, Quiet: true, Writer: &buf}

	if err := f.Table([]string{"Name"}, [][]string{{"foo"}}); err != nil {
		t.Fatalf("Table returned error: %v", err)
	}

	if buf.Len() > 0 {
		t.Error("table output should be suppressed in quiet mode")
	}
}

func TestTableOutputJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	f := &Formatter{JSON: true, Quiet: false, Writer: &buf}

	headers := []string{"Name", "Version"}
	rows := [][]string{{"foo", "1.0.0"}}
	if err := f.Table(headers, rows); err != nil {
		t.Fatalf("Table returned error: %v", err)
	}

	var result []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("table JSON output should be valid JSON: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result))
	}
	if result[0]["Name"] != "foo" {
		t.Errorf("expected Name=foo, got %s", result[0]["Name"])
	}
	if result[0]["Version"] != "1.0.0" {
		t.Errorf("expected Version=1.0.0, got %s", result[0]["Version"])
	}
}

func TestWriteJSONRoundtrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	f := &Formatter{JSON: true, Quiet: false, Writer: &buf}

	type payload struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	input := payload{Name: "test-pkg", Version: "0.1.0"}
	if err := f.WriteJSON(input); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	var output payload
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}

	if output.Name != input.Name {
		t.Errorf("roundtrip name mismatch: got %s, want %s", output.Name, input.Name)
	}
	if output.Version != input.Version {
		t.Errorf("roundtrip version mismatch: got %s, want %s", output.Version, input.Version)
	}
}

func TestSuccessMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	f := &Formatter{JSON: false, Quiet: false, Writer: &buf}
	f.Success("operation completed")

	if !strings.Contains(buf.String(), "operation completed") {
		t.Error("success message should be printed in normal mode")
	}
}

func TestSuccessMessageQuiet(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	f := &Formatter{JSON: false, Quiet: true, Writer: &buf}
	f.Success("operation completed")

	if buf.Len() > 0 {
		t.Error("success message should be suppressed in quiet mode")
	}
}

func TestErrorMessage(t *testing.T) {
	t.Parallel()

	var errBuf bytes.Buffer
	f := &Formatter{JSON: false, Quiet: false, Writer: &bytes.Buffer{}, ErrW: &errBuf}
	f.Error("something went wrong")

	if !strings.Contains(errBuf.String(), "something went wrong") {
		t.Error("error message should always be printed to stderr writer")
	}
}

func TestErrorMessageQuiet(t *testing.T) {
	t.Parallel()

	var errBuf bytes.Buffer
	f := &Formatter{JSON: false, Quiet: true, Writer: &bytes.Buffer{}, ErrW: &errBuf}
	f.Error("something went wrong")

	if !strings.Contains(errBuf.String(), "something went wrong") {
		t.Error("error message should be printed even in quiet mode")
	}
}

func TestErrorWritesToErrWriter(t *testing.T) {
	t.Parallel()

	var stdBuf, errBuf bytes.Buffer
	f := &Formatter{JSON: false, Quiet: false, Writer: &stdBuf, ErrW: &errBuf}
	f.Error("test error")

	if stdBuf.Len() > 0 {
		t.Error("error should not write to stdout writer")
	}
	if !strings.Contains(errBuf.String(), "Error: test error") {
		t.Errorf("error should write to stderr writer, got: %q", errBuf.String())
	}
}

func TestNewFormatter(t *testing.T) {
	t.Parallel()

	f := NewFormatter(true, true)
	if !f.JSON {
		t.Error("JSON should be true")
	}
	if !f.Quiet {
		t.Error("Quiet should be true")
	}
	if f.Writer == nil {
		t.Error("Writer should not be nil")
	}
	if f.ErrW == nil {
		t.Error("ErrW should not be nil")
	}
}
