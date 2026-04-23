package dolt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestBuildImportSQLIncludesCoreRows(t *testing.T) {
	t.Parallel()

	req := ImportPackageRequest{
		Package: models.Package{
			ID:           "sample",
			Name:         "sample",
			Version:      "1.0.0",
			AgentVariant: "claude",
			InstallScope: models.InstallScopeLocalOnly,
			Tags:         "a,b",
		},
		Files: []models.PackageFile{
			{PackageID: "sample", DestPath: "skills/x/SKILL.md", Content: "body", SHA256: "abc", FileType: models.FileTypeSkill, ContentType: models.ContentTypeMarkdown},
		},
		Deps: []models.PackageDep{
			{PackageID: "sample", DepType: models.DepTypeTool, DepName: "python3", DepSpec: ">= 3.11"},
		},
		Questions: []models.PackageQuestion{
			{PackageID: "sample", QuestionID: "style", Prompt: "Pick style", Type: models.QuestionChoice},
		},
		PackageSHA256: "pkgsha",
	}

	sql := buildImportSQL(req)
	for _, needle := range []string{
		"INSERT INTO packages",
		"INSERT INTO package_files",
		"INSERT INTO package_deps",
		"INSERT INTO package_questions",
		"'pkgsha'",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("SQL missing %q\n%s", needle, sql)
		}
	}
}

func TestCLIWriterImportPackageInvokesDolt(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "calls.log")
	sqlPath := filepath.Join(tempDir, "sql.log")
	scriptPath := filepath.Join(tempDir, "dolt")

	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + logPath + "\"\n" +
		"case \"$1\" in\n" +
		"  branch)\n" +
		"    if [ \"$2\" = \"--list\" ]; then echo \"$3\"; exit 0; fi\n" +
		"    ;;\n" +
		"  --branch)\n" +
		"    if [ \"$3\" = \"sql\" ]; then cat > \"" + sqlPath + "\"; exit 0; fi\n" +
		"    ;;\n" +
		"  sql)\n" +
		"    cat > \"" + sqlPath + "\"\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n" +
		"exit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	writer := NewCLIWriter(tempDir)
	err := writer.ImportPackage(context.Background(), ImportPackageRequest{
		Branch: "develop",
		Package: models.Package{
			ID:           "sample",
			Name:         "sample",
			Version:      "1.0.0",
			AgentVariant: "claude",
			InstallScope: models.InstallScopeAny,
		},
		PackageSHA256: "pkgsha",
		CommitMessage: "Import package sample 1.0.0",
	})
	if err != nil {
		t.Fatalf("ImportPackage() error = %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logData)
	for _, needle := range []string{"--branch develop sql"} {
		if !strings.Contains(logText, needle) {
			t.Fatalf("call log missing %q\n%s", needle, logText)
		}
	}

	sqlData, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sqlData), "INSERT INTO packages") {
		t.Fatalf("expected package insert in SQL, got:\n%s", string(sqlData))
	}
	for _, needle := range []string{"CALL DOLT_ADD('-A')", "CALL DOLT_COMMIT('-m', 'Import package sample 1.0.0')"} {
		if !strings.Contains(string(sqlData), needle) {
			t.Fatalf("expected SQL to contain %q, got:\n%s", needle, string(sqlData))
		}
	}
}

func TestCLIWriterBranchExists(t *testing.T) {
	tests := []struct {
		name          string
		scriptBody    string
		wantExists    bool
		wantErrSubstr string
	}{
		{
			name:       "branch exists",
			scriptBody: "echo \"$3\"",
			wantExists: true,
		},
		{
			name:       "branch absent",
			scriptBody: "echo \"main\"",
			wantExists: false,
		},
		{
			name:          "dolt error",
			scriptBody:    "echo boom >&2; exit 1",
			wantErrSubstr: "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			scriptPath := filepath.Join(tempDir, "dolt")
			script := "#!/bin/sh\n" +
				"if [ \"$1\" = \"branch\" ] && [ \"$2\" = \"--list\" ]; then\n" +
				"  " + tt.scriptBody + "\n" +
				"  exit 0\n" +
				"fi\n" +
				"exit 0\n"
			if tt.wantErrSubstr != "" {
				script = "#!/bin/sh\n" +
					"if [ \"$1\" = \"branch\" ] && [ \"$2\" = \"--list\" ]; then\n" +
					"  " + tt.scriptBody + "\n" +
					"fi\n"
			}
			if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}

			oldPath := os.Getenv("PATH")
			t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

			exists, err := NewCLIWriter(tempDir).BranchExists(context.Background(), "develop")
			if tt.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("BranchExists() error = %v", err)
			}
			if exists != tt.wantExists {
				t.Fatalf("BranchExists() = %v, want %v", exists, tt.wantExists)
			}
		})
	}
}
