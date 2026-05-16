package operations

import (
	"reflect"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
)

func TestModificationsFiltersOnlyNonOKFiles(t *testing.T) {
	t.Parallel()

	items := []api.ValidationItem{
		{Kind: api.ValidationKindFile, State: api.ValidationStateOK, Path: "ok.txt"},
		{Kind: api.ValidationKindFile, State: api.ValidationStateModified, Path: "mod.txt"},
		{Kind: api.ValidationKindHook, State: api.ValidationStateMissing, HookScript: "hooks/pre.sh"},
	}

	got := Modifications(items)
	want := []api.ValidationItem{{Kind: api.ValidationKindFile, State: api.ValidationStateModified, Path: "mod.txt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Modifications() = %+v, want %+v", got, want)
	}
}

func TestIssuesFiltersNonFileItems(t *testing.T) {
	t.Parallel()

	items := []api.ValidationItem{
		{Kind: api.ValidationKindFile, State: api.ValidationStateModified, Path: "mod.txt"},
		{Kind: api.ValidationKindDependency, State: api.ValidationStateMissing, Dependency: "gh"},
	}

	got := Issues(items)
	want := []api.ValidationItem{{Kind: api.ValidationKindDependency, State: api.ValidationStateMissing, Dependency: "gh"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Issues() = %+v, want %+v", got, want)
	}
}
