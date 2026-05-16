package cmd

import (
	"errors"
	"testing"
)

func requireJSONCmdError(t *testing.T, err error) {
	t.Helper()
	if err == nil || !isJSONCmdError(err) {
		t.Fatalf("Execute() error = %v, want json command error", err)
	}
}

func TestJSONCmdErrorExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "sentinel", err: jsonCmdError{cause: errors.New("boom")}, want: 1},
		{name: "wrapped sentinel", err: errors.Join(errors.New("outer"), jsonCmdError{cause: errors.New("boom")}), want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !isJSONCmdError(tt.err) {
				t.Fatalf("isJSONCmdError(%v) = false, want true", tt.err)
			}
			if got := JSONErrorExitCode(tt.err); got != tt.want {
				t.Fatalf("JSONErrorExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}
