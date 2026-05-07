package cmd

import "testing"

func requireJSONCmdError(t *testing.T, err error) {
	t.Helper()
	if err == nil || !isJSONCmdError(err) {
		t.Fatalf("Execute() error = %v, want json command error", err)
	}
}
