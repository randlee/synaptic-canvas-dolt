package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/differ"
	"github.com/randlee/synaptic-canvas-dolt/pkg/dolt"
	"github.com/randlee/synaptic-canvas-dolt/pkg/exporter"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/publisher"
	"github.com/randlee/synaptic-canvas-dolt/pkg/verifier"
	"github.com/spf13/cobra"
)

type adminMockReader struct {
	pkg       *models.Package
	files     []models.PackageFile
	questions []models.PackageQuestion
	err       error
}

func (m adminMockReader) GetPackage(_ context.Context, _ string) (*models.Package, error) {
	return m.pkg, m.err
}

func (m adminMockReader) GetPackageFiles(_ context.Context, _ string) ([]models.PackageFile, error) {
	return m.files, m.err
}

func (m adminMockReader) GetPackageQuestions(_ context.Context, _ string) ([]models.PackageQuestion, error) {
	return m.questions, m.err
}

type adminMockPromoter struct {
	result *publisher.PublishResult
	err    error
}

func (m adminMockPromoter) PublishPackage(_ context.Context, _, _, _ string) (*publisher.PublishResult, error) {
	return m.result, m.err
}

func TestNewAdminCmdIncludesPhase2Subcommands(t *testing.T) {
	cmd := NewAdminCmd()
	names := []string{"diff", "export", "import", "publish", "verify"}
	for _, name := range names {
		if _, _, err := cmd.Find([]string{name}); err != nil {
			t.Fatalf("expected subcommand %q: %v", name, err)
		}
	}
}

func TestCommandConstructors(t *testing.T) {
	if NewExportCmd() == nil || NewVerifyCmd() == nil || NewDiffCmd() == nil || NewPublishCmd() == nil {
		t.Fatal("expected command constructors to return commands")
	}
}

func TestDetectDoltDir(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".dolt"), 0o755); err != nil { //nolint:gosec // G301: test temp directory permissions are intentional.
		t.Fatalf("Mkdir(.dolt): %v", err)
	}

	got, err := detectDoltDir(repoDir)
	if err != nil || got != repoDir {
		t.Fatalf("detectDoltDir(configured) = %q, %v", got, err)
	}

	child := filepath.Join(repoDir, "nested")
	if err := os.Mkdir(child, 0o755); err != nil { //nolint:gosec // G301: test temp directory permissions are intentional.
		t.Fatalf("Mkdir(nested): %v", err)
	}
	t.Chdir(child)
	got, err = detectDoltDir("")
	if err != nil || got != repoDir {
		t.Fatalf("detectDoltDir(auto) = %q, %v", got, err)
	}

	if _, err := detectDoltDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected invalid configured dolt dir error")
	}
}

func TestOpenReadClientWithDoltDir(t *testing.T) {
	client, err := openReadClient(t.TempDir(), "main")
	if err != nil {
		t.Fatalf("openReadClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestLoadConfigAndDoltDir(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".dolt"), 0o755); err != nil { //nolint:gosec // G301: test temp directory permissions are intentional.
		t.Fatalf("Mkdir(.dolt): %v", err)
	}

	cmd := &cobra.Command{}
	cmd.PersistentFlags().String("dolt-dir", "", "")
	cmd.PersistentFlags().String("remote", "", "")
	cmd.PersistentFlags().String("branch", "", "")
	cmd.PersistentFlags().Bool("json", false, "")
	cmd.PersistentFlags().Bool("quiet", false, "")
	cmd.PersistentFlags().Bool("verbose", false, "")
	if err := cmd.PersistentFlags().Set("dolt-dir", repoDir); err != nil {
		t.Fatalf("Set(dolt-dir): %v", err)
	}
	if err := cmd.PersistentFlags().Set("branch", "develop"); err != nil {
		t.Fatalf("Set(branch): %v", err)
	}

	cfg, gotDir, err := loadConfigAndDoltDir(cmd)
	if err != nil {
		t.Fatalf("loadConfigAndDoltDir() error = %v", err)
	}
	if gotDir != repoDir || cfg.Branch != "develop" {
		t.Fatalf("unexpected config/dir: cfg=%#v dir=%q", cfg, gotDir)
	}
}

func TestWithReadClientAndClients(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".dolt"), 0o755); err != nil { //nolint:gosec // G301: test temp directory permissions are intentional.
		t.Fatalf("Mkdir(.dolt): %v", err)
	}

	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{}
		cmd.PersistentFlags().String("dolt-dir", "", "")
		cmd.PersistentFlags().String("remote", "", "")
		cmd.PersistentFlags().String("branch", "", "")
		cmd.PersistentFlags().Bool("json", false, "")
		cmd.PersistentFlags().Bool("quiet", false, "")
		cmd.PersistentFlags().Bool("verbose", false, "")
		if err := cmd.PersistentFlags().Set("dolt-dir", repoDir); err != nil {
			t.Fatalf("Set(dolt-dir): %v", err)
		}
		return cmd
	}

	called := false
	if err := withReadClient(newCmd(), "main", func(_ *config.Config, client readClient) error {
		called = client != nil
		return nil
	}); err != nil {
		t.Fatalf("withReadClient() error = %v", err)
	}
	if !called {
		t.Fatal("expected withReadClient callback to run")
	}

	called = false
	if err := withReadClients(newCmd(), "main", "develop", func(_ *config.Config, client1, client2 readClient) error {
		called = client1 != nil && client2 != nil
		return nil
	}); err != nil {
		t.Fatalf("withReadClients() error = %v", err)
	}
	if !called {
		t.Fatal("expected withReadClients callback to run")
	}
}

func TestRunExportCmdRequiresOutput(t *testing.T) {
	cmd := NewExportCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	if err := runExportCmd(cmd, []string{"pkg"}, ""); err == nil || err.Error() != "--output is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunVerifyCmdDetectsMissingDoltDir(t *testing.T) {
	cmd := NewVerifyCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	t.Chdir(t.TempDir())
	if err := runVerifyCmd(cmd, []string{"pkg"}); err == nil || !strings.Contains(err.Error(), "could not auto-detect Dolt database directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPublishCmdRequiresBranches(t *testing.T) {
	cmd := NewPublishCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	if err := runPublishCmd(cmd, []string{"pkg"}, "", ""); err == nil || err.Error() != "--from and --to are required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDiffCmdRequiresBranches(t *testing.T) {
	cmd := NewDiffCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	if err := runDiffCmd(cmd, []string{"pkg"}, "", ""); err == nil || err.Error() != "--branch1 and --branch2 are required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunExportUsesEffectiveBranch(t *testing.T) {
	t.Setenv("SC_DOLT_BRANCH", "develop")

	mock := dolt.NewMockClient()
	fileSHA := shaTextForAdmin("alpha")
	pkgSHA := computePackageSHAForAdmin([]string{"agents/a.md:" + fileSHA})
	mock.AddPackage(&models.Package{ID: "pkg", Name: "pkg", Version: "1.0.0", SHA256: &pkgSHA})
	mock.AddFiles("pkg", []models.PackageFile{{
		PackageID: "pkg",
		DestPath:  "agents/a.md",
		Content:   "alpha",
		SHA256:    fileSHA,
	}})
	mock.AddDeps("pkg", nil)
	mock.AddHooks("pkg", nil)
	mock.AddQuestions("pkg", nil)

	summary, err := runExport(context.Background(), &config.Config{}, "pkg", t.TempDir(), mock)
	if err != nil {
		t.Fatalf("runExport() error = %v", err)
	}
	if summary.Branch != "develop" {
		t.Fatalf("summary.Branch = %q, want develop", summary.Branch)
	}
}

func TestRunExportCmdUsesEffectiveBranchForReader(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".dolt"), 0o755); err != nil { //nolint:gosec // G301: test temp directory permissions are intentional.
		t.Fatalf("Mkdir(.dolt): %v", err)
	}

	cmd := NewExportCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	if err := cmd.Root().PersistentFlags().Set("dolt-dir", repoDir); err != nil {
		t.Fatalf("Set(dolt-dir): %v", err)
	}
	t.Setenv("SC_DOLT_BRANCH", "develop")

	fileSHA := shaTextForAdmin("alpha")
	pkgSHA := computePackageSHAForAdmin([]string{"agents/a.md:" + fileSHA})

	originalOpener := readClientOpener
	t.Cleanup(func() { readClientOpener = originalOpener })

	var openedBranch string
	readClientOpener = func(_ string, branch string) (readClient, error) {
		openedBranch = branch
		mock := dolt.NewMockClient()
		mock.AddPackage(&models.Package{ID: "pkg", Name: "pkg", Version: "1.0.0", SHA256: &pkgSHA})
		mock.AddFiles("pkg", []models.PackageFile{{
			PackageID: "pkg",
			DestPath:  "agents/a.md",
			Content:   "alpha",
			SHA256:    fileSHA,
		}})
		mock.AddDeps("pkg", nil)
		mock.AddHooks("pkg", nil)
		mock.AddQuestions("pkg", nil)
		return mock, nil
	}

	if err := runExportCmd(cmd, []string{"pkg"}, t.TempDir()); err != nil {
		t.Fatalf("runExportCmd() error = %v", err)
	}
	if openedBranch != "develop" {
		t.Fatalf("openedBranch = %q, want develop", openedBranch)
	}
}

func TestWriteExportResultHuman(t *testing.T) {
	formatter := output.NewFormatter(false, false)
	var out bytes.Buffer
	formatter.Writer = &out

	summary := &exporter.Summary{
		PackageID:           "pkg",
		Version:             "1.0.0",
		Branch:              "main",
		OutputDir:           "/tmp/pkg",
		FilesWritten:        4,
		FileSHAVerified:     3,
		PluginReconstructed: true,
		PackageSHA256:       "abc123",
	}
	if err := writeExportResult(formatter, summary); err != nil {
		t.Fatalf("writeExportResult() error = %v", err)
	}
	text := out.String()
	for _, want := range []string{"Exported pkg 1.0.0 from main", "Output: /tmp/pkg", "Plugin manifest reconstructed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected output to contain %q, got %q", want, text)
		}
	}
}

func TestRunVerifyUsesEffectiveBranch(t *testing.T) {
	t.Setenv("SC_DOLT_BRANCH", "develop")

	mock := dolt.NewMockClient()
	pkg := dolt.NewTestPackage("pkg", "pkg", "1.0.0", nil)
	fileSHA := shaTextForAdmin("alpha")
	agg := computePackageSHAForAdmin([]string{"agents/a.md:" + fileSHA})
	pkg.SHA256 = &agg
	mock.AddPackage(pkg)
	mock.AddFiles("pkg", []models.PackageFile{{PackageID: "pkg", DestPath: "agents/a.md", Content: "alpha", SHA256: fileSHA}})

	summary, err := runVerify(context.Background(), &config.Config{}, "pkg", mock)
	if err != nil {
		t.Fatalf("runVerify() error = %v", err)
	}
	if summary.Branch != "develop" {
		t.Fatalf("summary.Branch = %q, want develop", summary.Branch)
	}
}

func TestRunVerifyCmdUsesEffectiveBranchForReader(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".dolt"), 0o755); err != nil { //nolint:gosec // G301: test temp directory permissions are intentional.
		t.Fatalf("Mkdir(.dolt): %v", err)
	}

	cmd := NewVerifyCmd()
	cmd.Root().PersistentFlags().String("dolt-dir", "", "")
	cmd.Root().PersistentFlags().String("remote", "", "")
	cmd.Root().PersistentFlags().String("branch", "", "")
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.Root().PersistentFlags().Bool("quiet", false, "")
	cmd.Root().PersistentFlags().Bool("verbose", false, "")
	if err := cmd.Root().PersistentFlags().Set("dolt-dir", repoDir); err != nil {
		t.Fatalf("Set(dolt-dir): %v", err)
	}
	t.Setenv("SC_DOLT_BRANCH", "develop")

	fileSHA := shaTextForAdmin("alpha")
	pkg := dolt.NewTestPackage("pkg", "pkg", "1.0.0", nil)
	agg := computePackageSHAForAdmin([]string{"agents/a.md:" + fileSHA})
	pkg.SHA256 = &agg

	originalOpener := readClientOpener
	t.Cleanup(func() { readClientOpener = originalOpener })

	var openedBranch string
	readClientOpener = func(_ string, branch string) (readClient, error) {
		openedBranch = branch
		mock := dolt.NewMockClient()
		mock.AddPackage(pkg)
		mock.AddFiles("pkg", []models.PackageFile{{
			PackageID: "pkg",
			DestPath:  "agents/a.md",
			Content:   "alpha",
			SHA256:    fileSHA,
		}})
		return mock, nil
	}

	if err := runVerifyCmd(cmd, []string{"pkg"}); err != nil {
		t.Fatalf("runVerifyCmd() error = %v", err)
	}
	if openedBranch != "develop" {
		t.Fatalf("openedBranch = %q, want develop", openedBranch)
	}
}

func TestWriteVerifyResultWritesCorruptFileToErr(t *testing.T) {
	formatter := output.NewFormatter(false, false)
	var out bytes.Buffer
	var errOut bytes.Buffer
	formatter.Writer = &out
	formatter.ErrW = &errOut

	summary := &verifier.Summary{
		PackageID:       "pkg",
		Version:         "1.0.0",
		Branch:          "main",
		FilesChecked:    1,
		CorruptFiles:    1,
		AggregateStatus: verifier.StatusCorrupt,
		FileResults: []verifier.FileResult{{
			DestPath:    "agents/a.md",
			ExpectedSHA: "old",
			ActualSHA:   "new",
			Status:      verifier.StatusCorrupt,
		}},
	}
	if err := writeVerifyResult(formatter, summary); err != nil {
		t.Fatalf("writeVerifyResult() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "CORRUPT: agents/a.md") {
		t.Fatalf("expected corrupt output on stderr, got %q", errOut.String())
	}
	if !strings.Contains(out.String(), "Aggregate status: CORRUPT") {
		t.Fatalf("expected summary output, got %q", out.String())
	}
}

func TestRunDiffAndWriteDiffResult(t *testing.T) {
	left := dolt.NewMockClient()
	right := dolt.NewMockClient()
	left.AddPackage(&models.Package{ID: "pkg", Version: "1.0.0"})
	right.AddPackage(&models.Package{ID: "pkg", Version: "1.1.0"})
	left.AddFiles("pkg", []models.PackageFile{{PackageID: "pkg", DestPath: "a.txt", Content: "one", SHA256: "sha1"}})
	right.AddFiles("pkg", []models.PackageFile{{PackageID: "pkg", DestPath: "a.txt", Content: "two", SHA256: "sha2"}})

	summary, err := runDiff(context.Background(), "pkg", "main", "develop", left, right)
	if err != nil {
		t.Fatalf("runDiff() error = %v", err)
	}
	if len(summary.FileChanges) != 1 || summary.FileChanges[0].Type != differ.ChangeModified {
		t.Fatalf("unexpected diff summary: %#v", summary)
	}

	formatter := output.NewFormatter(true, false)
	var out bytes.Buffer
	formatter.Writer = &out
	if err := writeDiffResult(formatter, summary); err != nil {
		t.Fatalf("writeDiffResult() error = %v", err)
	}
	if !strings.Contains(out.String(), "\"package_id\": \"pkg\"") {
		t.Fatalf("expected JSON output, got %q", out.String())
	}
}

func TestWriteDiffResultHuman(t *testing.T) {
	formatter := output.NewFormatter(false, false)
	var out bytes.Buffer
	formatter.Writer = &out

	err := writeDiffResult(formatter, &differ.Summary{
		PackageID: "pkg",
		Branch1:   "main",
		Branch2:   "develop",
		FileChanges: []differ.FileChange{{
			DestPath: "a.txt",
			Type:     differ.ChangeModified,
		}},
	})
	if err != nil {
		t.Fatalf("writeDiffResult() error = %v", err)
	}
	if !strings.Contains(out.String(), "MODIFIED a.txt") {
		t.Fatalf("expected human diff output, got %q", out.String())
	}
}

func TestRunPublishAndWritePublishResult(t *testing.T) {
	fileSHA := shaTextForAdmin("alpha")
	pkgSHA := computePackageSHAForAdmin([]string{"skills/x/SKILL.md:" + fileSHA})
	summary, err := runPublish(context.Background(), "pkg", "main", "develop", adminMockReader{
		pkg: &models.Package{ID: "pkg", Name: "pkg", Version: "1.0.0", SHA256: &pkgSHA},
		files: []models.PackageFile{{
			PackageID: "pkg",
			DestPath:  "skills/x/SKILL.md",
			Content:   "alpha",
			SHA256:    fileSHA,
		}},
	}, adminMockPromoter{result: &publisher.PublishResult{Hash: "abc", Message: "ok"}})
	if err != nil {
		t.Fatalf("runPublish() error = %v", err)
	}
	if summary.Publish == nil || summary.Publish.Hash != "abc" {
		t.Fatalf("unexpected publish summary: %#v", summary)
	}

	formatter := output.NewFormatter(true, false)
	var out bytes.Buffer
	formatter.Writer = &out
	if err := writePublishResult(formatter, summary, errors.New("boom")); err == nil || err.Error() != "boom" {
		t.Fatalf("expected propagated error, got %v", err)
	}
	if !strings.Contains(out.String(), "\"publish\"") {
		t.Fatalf("expected JSON output, got %q", out.String())
	}
}

func TestWritePublishResultHuman(t *testing.T) {
	formatter := output.NewFormatter(false, false)
	var out bytes.Buffer
	formatter.Writer = &out

	err := writePublishResult(formatter, &publisher.Summary{
		PackageID:  "pkg",
		Version:    "1.0.0",
		FromBranch: "main",
		ToBranch:   "develop",
		Publish:    &publisher.PublishResult{Message: "ok"},
	}, nil)
	if err != nil {
		t.Fatalf("writePublishResult() error = %v", err)
	}
	if !strings.Contains(out.String(), "Published pkg 1.0.0 from main to develop") {
		t.Fatalf("expected publish output, got %q", out.String())
	}
}

func shaTextForAdmin(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func computePackageSHAForAdmin(parts []string) string {
	sorted := append([]string(nil), parts...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}
