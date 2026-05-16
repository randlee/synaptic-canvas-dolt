package dolt_test

import (
	"context"
	"errors"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolttest"
)

func TestClientConformanceAcrossTransports(t *testing.T) {
	ctx := context.Background()
	for _, harness := range dolttest.ConformanceHarnesses() {
		t.Run(harness.Name, func(t *testing.T) {
			client := harness.Open(t)
			defer func() { _ = client.Close() }()

			packages, err := client.ListPackages(ctx, dolt.ListOptions{Tags: []string{"go"}})
			if err != nil {
				t.Fatalf("ListPackages() error = %v", err)
			}
			if len(packages) != 1 || packages[0].ID != "team-lead" || packages[0].FileCount != 1 || packages[0].DepCount != 1 {
				t.Fatalf("unexpected packages: %+v", packages)
			}

			pkg, err := client.GetPackageDetail(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageDetail() error = %v", err)
			}
			if pkg == nil || pkg.Name != "Team Lead" || pkg.Version != "1.2.3" || pkg.FileCount != 1 || pkg.DepCount != 1 {
				t.Fatalf("unexpected package detail: %+v", pkg)
			}

			files, err := client.GetPackageFiles(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageFiles() error = %v", err)
			}
			if len(files) != 1 || files[0].DestPath != "skills/team-lead/SKILL.md" {
				t.Fatalf("unexpected files: %+v", files)
			}

			deps, err := client.GetPackageDeps(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageDeps() error = %v", err)
			}
			if len(deps) != 1 || deps[0].DepName != "gh" {
				t.Fatalf("unexpected deps: %+v", deps)
			}

			hooks, err := client.GetPackageHooks(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageHooks() error = %v", err)
			}
			if len(hooks) != 1 || hooks[0].ScriptPath != "hooks/pre-commit.sh" || hooks[0].Priority != 5 {
				t.Fatalf("unexpected hooks: %+v", hooks)
			}

			questions, err := client.GetPackageQuestions(ctx, "team-lead")
			if err != nil {
				t.Fatalf("GetPackageQuestions() error = %v", err)
			}
			if len(questions) != 1 || questions[0].QuestionID != "repo_name" || questions[0].SortOrder != 1 {
				t.Fatalf("unexpected questions: %+v", questions)
			}

			variant, err := client.ResolveVariant(ctx, "team-lead", "codex")
			if err != nil {
				t.Fatalf("ResolveVariant() error = %v", err)
			}
			if variant != "team-lead-codex" {
				t.Fatalf("variant = %q, want team-lead-codex", variant)
			}

			catalogRows, err := client.GetPackageCatalog(ctx)
			if err != nil {
				t.Fatalf("GetPackageCatalog() error = %v", err)
			}
			if len(catalogRows) != 1 || catalogRows[0].DocPath != "skills/team-lead/SKILL.md" || catalogRows[0].SHA256 != "sha-skill" {
				t.Fatalf("unexpected catalog rows: %+v", catalogRows)
			}
		})
	}
}

func TestClientConformanceUnavailableAcrossTransports(t *testing.T) {
	ctx := context.Background()
	for _, harness := range dolttest.FailingHarnesses() {
		t.Run(harness.Name, func(t *testing.T) {
			client := harness.Open(t)
			defer func() { _ = client.Close() }()

			_, err := client.ListPackages(ctx, dolt.ListOptions{})
			if !errors.Is(err, dolt.ErrServerError) {
				t.Fatalf("error = %v, want ErrServerError", err)
			}
		})
	}
}
