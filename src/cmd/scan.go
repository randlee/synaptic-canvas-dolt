package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/catalog"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/integrity"
	"github.com/spf13/cobra"
)

const trackingOriginScanReconciled = "scan-reconciled"

type scanResponse = api.ScanResponse
type scanCandidate = api.ScanCandidate
type scanCandidateFile = api.ScanCandidateFile

type scanOptions struct {
	RepoRoot string
	Branch   string
	Scope    string
	Paths    []string
	Recurse  bool
}

type scanTarget struct {
	Root        string
	RuntimeRoot string
	Scope       string
	StateRoot   string
	CatalogPath string
	Branch      string
	Recurse     bool
}

type scanMatchContext struct {
	PackageHint string
	DocPaths    []string
	InstallRoot string
	InstallSite string
	FilePath    string
}

type trackedFileInfo struct {
	Package string
	Version string
	Branch  string
	Scope   string
}

var scanComputeFileSHA = integrity.ComputeFileSHA256

// NewScanCmd creates the sc scan command.
func NewScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan [path...]",
		Short: "Discover installed packages from local SHA catalogs",
		Args:  cobra.ArbitraryArgs,
		RunE:  runScanCmd,
	}
	cmd.Flags().String("scope", "both", "scan scope: project, global, or both")
	cmd.Flags().Bool("recurse", false, "scan directory arguments recursively")
	cmd.Flags().Bool("accept-all", false, "write lockfile entries for all discovered installs")
	cmd.Flags().Bool("upgrade-all", false, "reconcile tracked installs to matched catalog versions")
	cmd.MarkFlagsMutuallyExclusive("accept-all", "upgrade-all")
	return cmd
}

func runScanCmd(cmd *cobra.Command, args []string) error {
	scope, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("reading --scope: %w", err)
	}
	recurse, err := cmd.Flags().GetBool("recurse")
	if err != nil {
		return fmt.Errorf("reading --recurse: %w", err)
	}
	acceptAll, err := cmd.Flags().GetBool("accept-all")
	if err != nil {
		return fmt.Errorf("reading --accept-all: %w", err)
	}
	upgradeAll, err := cmd.Flags().GetBool("upgrade-all")
	if err != nil {
		return fmt.Errorf("reading --upgrade-all: %w", err)
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return fmt.Errorf("reading config flags: %w", err)
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()

	if err := validateCatalogScope(scope); err != nil {
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", err.Error())
		}
		return err
	}
	if acceptAll && upgradeAll {
		message := "--accept-all cannot be combined with --upgrade-all"
		if cfg.JSON {
			return writeJSONError(formatter, "invalid_args", message)
		}
		return errors.New(message)
	}

	repoRoot, err := currentRepoRoot()
	if err != nil {
		if cfg.JSON {
			return writeClassifiedJSONError(formatter, cfg, err)
		}
		return err
	}

	result, scanErr := scanInstalledPackages(cmd.Context(), scanOptions{
		RepoRoot: repoRoot,
		Branch:   cfg.EffectiveBranch(),
		Scope:    scope,
		Paths:    args,
		Recurse:  recurse,
	})
	if scanErr != nil {
		if cfg.JSON {
			if missing, ok := catalog.MissingCatalogDetails(scanErr); ok {
				return writeJSONError(formatter, api.ErrorCodeValidationFailed, scanErr.Error(), map[string]any{
					"required_action": "sc catalog update",
					"catalog_path":    missing.Path,
				})
			}
			return writeJSONError(formatter, classifyJSONError(scanErr.Error()), scanErr.Error())
		}
		return scanErr
	}

	accepted, upgraded := 0, 0
	if acceptAll || upgradeAll {
		accepted, upgraded, err = applyScanMutations(cmd.Context(), result.Candidates, acceptAll, upgradeAll)
		if err != nil {
			if cfg.JSON {
				return writeClassifiedJSONError(formatter, cfg, err)
			}
			return err
		}
	}

	resp := scanResponse{
		OK:         true,
		Branch:     cfg.EffectiveBranch(),
		Mutated:    acceptAll || upgradeAll,
		Accepted:   accepted,
		Upgraded:   upgraded,
		Candidates: result.Candidates,
		Warnings:   result.Warnings,
	}
	if cfg.JSON {
		return formatter.WriteJSON(resp)
	}
	for _, warning := range resp.Warnings {
		writeWarning(formatter, warning)
	}
	rows := make([][]string, 0, len(resp.Candidates))
	for _, candidate := range resp.Candidates {
		status := "new"
		if candidate.NeedsUpgrade {
			status = candidate.ExistingVersion + " -> " + candidate.Version
		}
		rows = append(rows, []string{
			candidate.Package,
			candidate.Version,
			candidate.Scope,
			status,
			fmt.Sprintf("%d", len(candidate.Files)),
		})
	}
	return formatter.Table([]string{"PACKAGE", "VERSION", "SCOPE", "UPGRADE", "FILES"}, rows)
}

type scanResult struct {
	Candidates []scanCandidate
	Warnings   []string
}

func scanInstalledPackages(ctx context.Context, opts scanOptions) (scanResult, error) {
	if opts.Branch == "" {
		opts.Branch = "main"
	}
	targets, err := scanTargets(opts)
	if err != nil {
		return scanResult{}, err
	}
	locks, err := loadScanLocks(opts.RepoRoot)
	if err != nil {
		return scanResult{}, err
	}
	trackedFiles := trackedFileMap(locks)
	installedVersions := installedVersionSet(locks)

	result := scanResult{}
	grouped := map[string]*scanCandidate{}
	var walkErrs []error
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return scanResult{}, err
		}
		if _, err := os.Stat(target.Root); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			walkErrs = append(walkErrs, fmt.Errorf("stat %s: %w", target.Root, err))
			continue
		}

		cat, warnings, err := loadScanCatalog(target.CatalogPath, opts.Branch)
		if err != nil {
			return scanResult{}, err
		}
		result.Warnings = append(result.Warnings, warnings...)
		if err := scanTargetFiles(ctx, target, cat.Entries, trackedFiles, installedVersions, grouped, &walkErrs); err != nil {
			return scanResult{}, err
		}
	}
	result.Candidates = sortedScanCandidates(grouped)
	return result, errors.Join(walkErrs...)
}

func scanTargets(opts scanOptions) ([]scanTarget, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	branch := opts.Branch
	if branch == "" {
		branch = "main"
	}
	machineCatalog, err := catalog.MachinePath(branch)
	if err != nil {
		return nil, err
	}
	if len(opts.Paths) > 0 {
		targets := make([]scanTarget, 0, len(opts.Paths))
		for _, raw := range opts.Paths {
			abs, err := filepath.Abs(raw)
			if err != nil {
				return nil, fmt.Errorf("resolving scan path %q: %w", raw, err)
			}
			scope := "project"
			stateRoot := opts.RepoRoot
			catalogPath := catalog.ProjectPath(opts.RepoRoot, branch)
			if isUnderPath(abs, filepath.Join(home, ".claude")) {
				scope = "global"
				stateRoot = home
				catalogPath = machineCatalog
			}
			runtimeRoot := nearestClaudeRoot(abs)
			if runtimeRoot == "" {
				runtimeRoot = abs
			}
			recurse := opts.Recurse || filepath.Base(runtimeRoot) == ".claude"
			targets = append(targets, scanTarget{
				Root:        abs,
				RuntimeRoot: runtimeRoot,
				Scope:       scope,
				StateRoot:   stateRoot,
				CatalogPath: catalogPath,
				Branch:      branch,
				Recurse:     recurse,
			})
		}
		return targets, nil
	}

	targets := []scanTarget{}
	if opts.Scope == "" || opts.Scope == "project" || opts.Scope == "both" {
		projectRoot := filepath.Join(opts.RepoRoot, ".claude")
		targets = append(targets, scanTarget{
			Root:        projectRoot,
			RuntimeRoot: projectRoot,
			Scope:       "project",
			StateRoot:   opts.RepoRoot,
			CatalogPath: catalog.ProjectPath(opts.RepoRoot, branch),
			Branch:      branch,
			Recurse:     true,
		})
	}
	if opts.Scope == "" || opts.Scope == "global" || opts.Scope == "both" {
		globalRoot := filepath.Join(home, ".claude")
		targets = append(targets, scanTarget{
			Root:        globalRoot,
			RuntimeRoot: globalRoot,
			Scope:       "global",
			StateRoot:   home,
			CatalogPath: machineCatalog,
			Branch:      branch,
			Recurse:     true,
		})
	}
	return targets, nil
}

func loadScanCatalog(path, branch string) (catalog.Catalog, []string, error) {
	cat, warnings, err := catalog.Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return catalog.Catalog{}, nil, catalog.NewMissingCatalogError(path, branch)
	}
	if err != nil {
		return catalog.Catalog{}, nil, err
	}
	if !cat.Meta.FetchedAt.IsZero() && snapshotNow().Sub(cat.Meta.FetchedAt) > catalog.StaleAfter {
		warnings = append(warnings, "catalog is older than 24h; run sc catalog update")
	}
	return cat, warnings, nil
}

func scanTargetFiles(
	ctx context.Context,
	target scanTarget,
	entries []catalog.CatalogEntry,
	trackedFiles map[string]trackedFileInfo,
	installedVersions map[string]map[string]struct{},
	grouped map[string]*scanCandidate,
	walkErrs *[]error,
) error {
	info, err := os.Stat(target.Root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return scanOneFile(ctx, target, target.Root, entries, trackedFiles, installedVersions, grouped)
	}
	if !target.Recurse {
		return scanTargetImmediateFiles(ctx, target, entries, trackedFiles, installedVersions, grouped, walkErrs)
	}
	return filepath.WalkDir(target.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			*walkErrs = append(*walkErrs, fmt.Errorf("walking %s: %w", path, walkErr))
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Type() != 0 && !d.Type().IsRegular() {
			return nil
		}
		if err := scanOneFile(ctx, target, path, entries, trackedFiles, installedVersions, grouped); err != nil {
			*walkErrs = append(*walkErrs, err)
		}
		return nil
	})
}

func scanTargetImmediateFiles(
	ctx context.Context,
	target scanTarget,
	entries []catalog.CatalogEntry,
	trackedFiles map[string]trackedFileInfo,
	installedVersions map[string]map[string]struct{},
	grouped map[string]*scanCandidate,
	walkErrs *[]error,
) error {
	items, err := os.ReadDir(target.Root)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if item.IsDir() || item.Type()&fs.ModeSymlink != 0 || (item.Type() != 0 && !item.Type().IsRegular()) {
			continue
		}
		path := filepath.Join(target.Root, item.Name())
		if err := scanOneFile(ctx, target, path, entries, trackedFiles, installedVersions, grouped); err != nil {
			*walkErrs = append(*walkErrs, err)
		}
	}
	return nil
}

func scanOneFile(
	ctx context.Context,
	target scanTarget,
	path string,
	entries []catalog.CatalogEntry,
	trackedFiles map[string]trackedFileInfo,
	installedVersions map[string]map[string]struct{},
	grouped map[string]*scanCandidate,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	matchCtx := deriveScanMatchContext(target, abs)
	if len(matchCtx.DocPaths) == 0 {
		return nil
	}
	cleanAbs := canonicalScanPath(abs)
	trackedInfo, tracked := trackedFiles[cleanAbs]
	if tracked && matchCtx.PackageHint == "" {
		matchCtx.PackageHint = trackedInfo.Package
	}
	sha, err := scanComputeFileSHA(abs)
	if err != nil {
		return fmt.Errorf("hashing %s: %w", abs, err)
	}
	matches := catalogMatches(entries, matchCtx.PackageHint, matchCtx.DocPaths, sha)
	if len(matches) == 0 {
		return nil
	}
	selected := chooseCatalogMatch(matches, installedVersions)
	file := scanCandidateFile{
		Path:    filepath.ToSlash(matchCtx.FilePath),
		DocPath: filepath.ToSlash(selected.DocPath),
		SHA256:  sha,
	}
	if tracked {
		return addUpgradeCandidate(target, matchCtx, selected, file, trackedInfo, grouped)
	}
	addCandidate(grouped, target.StateRoot, scanCandidate{
		Package:        selected.PackageID,
		Version:        selected.Version,
		Branch:         target.Branch,
		Scope:          target.Scope,
		InstallRoot:    filepath.ToSlash(matchCtx.InstallRoot),
		InstallSite:    filepath.ToSlash(matchCtx.InstallSite),
		TrackingOrigin: trackingOriginScanReconciled,
		Files:          []scanCandidateFile{file},
	}, file)
	return nil
}

func addUpgradeCandidate(
	target scanTarget,
	matchCtx scanMatchContext,
	selected catalog.CatalogEntry,
	file scanCandidateFile,
	trackedInfo trackedFileInfo,
	grouped map[string]*scanCandidate,
) error {
	if trackedInfo.Version == "" || selected.Version <= trackedInfo.Version {
		return nil
	}
	addCandidate(grouped, target.StateRoot, scanCandidate{
		Package:         selected.PackageID,
		Version:         selected.Version,
		Branch:          target.Branch,
		Scope:           trackedInfo.Scope,
		InstallRoot:     filepath.ToSlash(matchCtx.InstallRoot),
		InstallSite:     filepath.ToSlash(matchCtx.InstallSite),
		TrackingOrigin:  trackingOriginScanReconciled,
		NeedsUpgrade:    true,
		ExistingVersion: trackedInfo.Version,
		ExistingBranch:  trackedInfo.Branch,
		Files:           []scanCandidateFile{file},
	}, file)
	return nil
}

func addCandidate(grouped map[string]*scanCandidate, stateRoot string, candidate scanCandidate, file scanCandidateFile) {
	key := strings.Join([]string{
		stateRoot,
		candidate.Scope,
		candidate.Package,
		candidate.Version,
		candidate.Branch,
		boolString(candidate.NeedsUpgrade),
	}, "\x00")
	existing := grouped[key]
	if existing == nil {
		candidate.Files = nil
		grouped[key] = &candidate
		existing = &candidate
	}
	for _, have := range existing.Files {
		if have.DocPath == file.DocPath && have.SHA256 == file.SHA256 {
			return
		}
	}
	existing.Files = append(existing.Files, file)
	sort.Slice(existing.Files, func(i, j int) bool { return existing.Files[i].DocPath < existing.Files[j].DocPath })
}

func catalogMatches(entries []catalog.CatalogEntry, packageHint string, docPaths []string, sha string) []catalog.CatalogEntry {
	docSet := make(map[string]struct{}, len(docPaths))
	for _, docPath := range docPaths {
		docSet[filepath.ToSlash(docPath)] = struct{}{}
	}
	matches := make([]catalog.CatalogEntry, 0, 1)
	for _, entry := range entries {
		if entry.SHA256 != sha {
			continue
		}
		if packageHint != "" && entry.PackageID != packageHint {
			continue
		}
		if _, ok := docSet[filepath.ToSlash(entry.DocPath)]; ok {
			matches = append(matches, entry)
		}
	}
	return matches
}

func chooseCatalogMatch(matches []catalog.CatalogEntry, installedVersions map[string]map[string]struct{}) catalog.CatalogEntry {
	sort.Slice(matches, func(i, j int) bool {
		iInstalled := hasInstalledVersion(installedVersions, matches[i])
		jInstalled := hasInstalledVersion(installedVersions, matches[j])
		if iInstalled != jInstalled {
			return iInstalled
		}
		if matches[i].Version != matches[j].Version {
			return matches[i].Version > matches[j].Version
		}
		if matches[i].PackageID != matches[j].PackageID {
			return matches[i].PackageID < matches[j].PackageID
		}
		return matches[i].DocPath < matches[j].DocPath
	})
	return matches[0]
}

func hasInstalledVersion(installedVersions map[string]map[string]struct{}, entry catalog.CatalogEntry) bool {
	versions := installedVersions[entry.PackageID]
	if versions == nil {
		return false
	}
	_, ok := versions[entry.Version]
	return ok
}

func deriveScanMatchContext(target scanTarget, abs string) scanMatchContext {
	runtimeRoot := target.RuntimeRoot
	if runtimeRoot == "" {
		runtimeRoot = nearestClaudeRoot(abs)
	}
	rel, err := filepath.Rel(runtimeRoot, abs)
	if err != nil {
		rel = filepath.Base(abs)
	}
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	ctx := scanMatchContext{
		DocPaths:    scanDedupeStrings([]string{rel, filepath.Base(rel)}),
		InstallRoot: filepath.Dir(abs),
		InstallSite: target.StateRoot,
		FilePath:    rel,
	}
	if len(parts) >= 3 && parts[0] == "skills" {
		ctx.PackageHint = parts[1]
		docPath := strings.Join(parts[2:], "/")
		ctx.DocPaths = scanDedupeStrings([]string{docPath, rel})
		ctx.InstallRoot = filepath.Join(runtimeRoot, "skills", parts[1])
		ctx.FilePath = docPath
		return ctx
	}
	if len(parts) >= 2 && (parts[0] == "agents" || parts[0] == "commands") {
		ctx.DocPaths = scanDedupeStrings([]string{rel, filepath.Base(rel)})
		ctx.InstallRoot = filepath.Dir(abs)
		ctx.FilePath = rel
	}
	return ctx
}

func loadScanLocks(repoRoot string) (map[string]installer.ManifestLock, error) {
	locks := map[string]installer.ManifestLock{}
	projectLock, err := installer.LoadManifestLock(repoRoot)
	if err != nil {
		return nil, err
	}
	locks[repoRoot] = projectLock
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	globalLock, err := installer.LoadManifestLock(home)
	if err != nil {
		return nil, err
	}
	locks[home] = globalLock
	return locks, nil
}

func trackedFileMap(locks map[string]installer.ManifestLock) map[string]trackedFileInfo {
	result := map[string]trackedFileInfo{}
	for _, lock := range locks {
		for _, record := range lock.Installs {
			for path := range record.Files {
				full := path
				if !filepath.IsAbs(full) {
					full = filepath.Join(record.InstallRoot, filepath.FromSlash(path))
				}
				if abs, err := filepath.Abs(full); err == nil {
					result[filepath.Clean(abs)] = trackedFileInfo{
						Package: record.Package,
						Version: record.Version,
						Branch:  record.Branch,
						Scope:   record.InstallScope,
					}
					result[canonicalScanPath(abs)] = trackedFileInfo{
						Package: record.Package,
						Version: record.Version,
						Branch:  record.Branch,
						Scope:   record.InstallScope,
					}
				}
			}
		}
	}
	return result
}

func installedVersionSet(locks map[string]installer.ManifestLock) map[string]map[string]struct{} {
	result := map[string]map[string]struct{}{}
	for _, lock := range locks {
		for _, record := range lock.Installs {
			if record.Package == "" || record.Version == "" {
				continue
			}
			if result[record.Package] == nil {
				result[record.Package] = map[string]struct{}{}
			}
			result[record.Package][record.Version] = struct{}{}
		}
	}
	return result
}

func applyScanMutations(ctx context.Context, candidates []scanCandidate, acceptAll, upgradeAll bool) (int, int, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}
	accepted, upgraded := 0, 0
	byStateRoot := map[string][]scanCandidate{}
	for _, candidate := range candidates {
		stateRoot := stateRootFromCandidate(candidate)
		if acceptAll && !candidate.NeedsUpgrade {
			byStateRoot[stateRoot] = append(byStateRoot[stateRoot], candidate)
		}
		if upgradeAll && candidate.NeedsUpgrade {
			byStateRoot[stateRoot] = append(byStateRoot[stateRoot], candidate)
		}
	}
	for stateRoot, scoped := range byStateRoot {
		if err := ctx.Err(); err != nil {
			return accepted, upgraded, err
		}
		if err := installer.WithManifestLock(stateRoot, func(lock *installer.ManifestLock) error {
			for _, candidate := range scoped {
				if err := ctx.Err(); err != nil {
					return err
				}
				if candidate.NeedsUpgrade {
					if upgradeScanRecord(lock, candidate) {
						upgraded++
					}
					continue
				}
				lock.UpsertInstall(scanInstallRecord(candidate))
				accepted++
			}
			return nil
		}); err != nil {
			return accepted, upgraded, err
		}
	}
	return accepted, upgraded, nil
}

func scanInstallRecord(candidate scanCandidate) installer.InstallRecord {
	return installer.InstallRecord{
		InstallID:        fmt.Sprintf(installer.InstallIDFormat, candidate.Package, candidate.Scope),
		Package:          candidate.Package,
		Version:          candidate.Version,
		DoltCommit:       "",
		Branch:           candidate.Branch,
		InstalledAt:      "",
		InstallScope:     candidate.Scope,
		InstallRoot:      candidate.InstallRoot,
		InstallSite:      candidate.InstallSite,
		TrackingOrigin:   trackingOriginScanReconciled,
		TemplateRendered: false,
		Files:            candidateFilesMap(candidate),
	}
}

func upgradeScanRecord(lock *installer.ManifestLock, candidate scanCandidate) bool {
	installID := fmt.Sprintf(installer.InstallIDFormat, candidate.Package, candidate.Scope)
	for i := range lock.Installs {
		record := &lock.Installs[i]
		if record.InstallID != installID && (record.Package != candidate.Package || record.InstallScope != candidate.Scope) {
			continue
		}
		record.Version = candidate.Version
		record.Branch = candidate.Branch
		record.DoltCommit = ""
		if record.TrackingOrigin == "" {
			record.TrackingOrigin = trackingOriginScanReconciled
		}
		if record.Files == nil {
			record.Files = map[string]string{}
		}
		for path, sha := range candidateFilesMap(candidate) {
			record.Files[path] = sha
		}
		return true
	}
	return false
}

func candidateFilesMap(candidate scanCandidate) map[string]string {
	files := make(map[string]string, len(candidate.Files))
	for _, file := range candidate.Files {
		files[file.DocPath] = file.SHA256
	}
	return files
}

func stateRootFromCandidate(candidate scanCandidate) string {
	if candidate.InstallSite != "" {
		return filepath.FromSlash(candidate.InstallSite)
	}
	if candidate.Scope == "global" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	return candidate.InstallSite
}

func sortedScanCandidates(grouped map[string]*scanCandidate) []scanCandidate {
	result := make([]scanCandidate, 0, len(grouped))
	for _, candidate := range grouped {
		result = append(result, *candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Package != result[j].Package {
			return result[i].Package < result[j].Package
		}
		if result[i].Scope != result[j].Scope {
			return result[i].Scope < result[j].Scope
		}
		if result[i].Version != result[j].Version {
			return result[i].Version < result[j].Version
		}
		return result[i].InstallRoot < result[j].InstallRoot
	})
	return result
}

func nearestClaudeRoot(path string) string {
	clean := filepath.Clean(path)
	for {
		if filepath.Base(clean) == ".claude" {
			return clean
		}
		next := filepath.Dir(clean)
		if next == clean {
			return ""
		}
		clean = next
	}
}

func isUnderPath(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func canonicalScanPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(abs)
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func scanDedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
