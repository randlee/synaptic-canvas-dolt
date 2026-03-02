package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// Formatter controls how command output is rendered. It supports JSON mode,
// quiet mode, and human-readable table output.
type Formatter struct {
	JSON   bool
	Quiet  bool
	Writer io.Writer
	ErrW   io.Writer
}

// NewFormatter creates a Formatter that writes to stdout and errors to stderr.
func NewFormatter(jsonMode, quiet bool) *Formatter {
	return &Formatter{
		JSON:   jsonMode,
		Quiet:  quiet,
		Writer: os.Stdout,
		ErrW:   os.Stderr,
	}
}

// Table prints an aligned table with the given headers and rows.
// In JSON mode, it marshals the data as a JSON array of objects keyed by header names.
// In quiet mode, table output is suppressed entirely.
func (f *Formatter) Table(headers []string, rows [][]string) error {
	if f.Quiet {
		return nil
	}

	if f.JSON {
		return f.tableAsJSON(headers, rows)
	}

	tw := tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0)

	// Print headers.
	for i, h := range headers {
		if i > 0 {
			if _, err := fmt.Fprint(tw, "\t"); err != nil {
				return fmt.Errorf("writing table separator: %w", err)
			}
		}
		if _, err := fmt.Fprint(tw, h); err != nil {
			return fmt.Errorf("writing table header: %w", err)
		}
	}
	if _, err := fmt.Fprintln(tw); err != nil {
		return fmt.Errorf("writing table header newline: %w", err)
	}

	// Print rows.
	for _, row := range rows {
		for i, col := range row {
			if i > 0 {
				if _, err := fmt.Fprint(tw, "\t"); err != nil {
					return fmt.Errorf("writing table separator: %w", err)
				}
			}
			if _, err := fmt.Fprint(tw, col); err != nil {
				return fmt.Errorf("writing table cell: %w", err)
			}
		}
		if _, err := fmt.Fprintln(tw); err != nil {
			return fmt.Errorf("writing table row newline: %w", err)
		}
	}
	return tw.Flush()
}

// tableAsJSON converts table data to a JSON array of objects.
func (f *Formatter) tableAsJSON(headers []string, rows [][]string) error {
	result := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		obj := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(row) {
				obj[h] = row[i]
			}
		}
		result = append(result, obj)
	}
	return f.WriteJSON(result)
}

// WriteJSON marshals v to indented JSON and writes it to the formatter's writer.
func (f *Formatter) WriteJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	_, err = fmt.Fprintln(f.Writer, string(data))
	if err != nil {
		return fmt.Errorf("writing JSON output: %w", err)
	}
	return nil
}

// Success prints a success message. Suppressed in quiet mode.
func (f *Formatter) Success(msg string) {
	if f.Quiet {
		return
	}
	_, _ = fmt.Fprintln(f.Writer, msg) //nolint:errcheck // best-effort output
}

// Error prints an error message to stderr. Always shown regardless of quiet mode.
func (f *Formatter) Error(msg string) {
	w := f.ErrW
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintln(w, "Error: "+msg) //nolint:errcheck // best-effort error output
}
