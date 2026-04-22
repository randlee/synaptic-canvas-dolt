package admin

import "testing"

func TestResolveReadBranch(t *testing.T) {
	t.Setenv("SC_DOLT_BRANCH", "")
	if got := resolveReadBranch("beta"); got != "beta" {
		t.Fatalf("resolveReadBranch(flag) = %q", got)
	}

	t.Setenv("SC_DOLT_BRANCH", "develop")
	if got := resolveReadBranch(""); got != "develop" {
		t.Fatalf("resolveReadBranch(env) = %q", got)
	}

	t.Setenv("SC_DOLT_BRANCH", "")
	if got := resolveReadBranch(""); got != "main" {
		t.Fatalf("resolveReadBranch(default) = %q", got)
	}
}
