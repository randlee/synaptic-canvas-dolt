# Synaptic Canvas — Project Plan

## Overview

Phased plan for building the `sc` Go CLI. Each phase contains sprints. Each sprint runs through the dev-QA loop defined in `CLAUDE.md`.

**Reference project:** `claude-history` (Go + Cobra + GoReleaser conventions)
**Design docs:** See [CLAUDE.md](../CLAUDE.md) for full list
**Normative docs:** [`requirements.md`](./requirements.md) and
[`architecture.md`](./architecture.md)

---

## Cross-Cutting Requirements

These apply to every sprint in every phase:

### Unit Testing
- Every exported function must have unit tests
- Table-driven tests preferred (Go convention)
- Tests run with `-race` flag
- Target: 80%+ line coverage, 100% coverage on integrity/SHA code paths
- Test fixtures in `test/` directory
- Mocks for Dolt database interactions (interface-based)
- Repository-owned helper scripts must have unit tests
- Tests must avoid nondeterministic dependence on wall-clock time, user state, or production logs
- Flaky tests are not tolerated; any nondeterministic test is a blocking defect
- Corner-case and interruption-path coverage is required for state mutation,
  install tracking, dependency resolution, and validation inventory logic

### Structured Logging
- **Package:** `log/slog` (stdlib, Go 1.21+; project targets Go 1.26)
- Centralized logger initialization in `internal/logging/`
- **Always enabled** — logging is on by default, never opt-in
- **Log destination:** `~/.sc/logs/sc.log` (file), rotated by date
- Console output: `--verbose` prints human-readable logs to stderr; `--quiet` suppresses console logging
- File logging always active regardless of `--quiet`/`--verbose` flags
- JSON format in log files; text format on console when `--verbose`
- Standard attributes on every log entry: `component`, `operation`, `timestamp`
- Levels: `Debug` (internal detail), `Info` (operations), `Warn` (recoverable), `Error` (failures)
- Default file level: `Info`; `--verbose` sets console to `Debug`
- Log failures after initialization are fail-open; they do not break normal CLI command flow
- No `fmt.Println` for operational output — use `internal/output/` formatters
- No `log.Fatal` — return errors up the call stack

### Error Handling
- All errors wrapped with context: `fmt.Errorf("operation: %w", err)`
- Cobra command errors surfaced to user via structured output
- `--json` output includes error details for skill integration
- AI-facing wrappers should call the CLI with `--json` explicitly

### Branch Resolution
- Read-path commands resolve branches using:
  1. `--branch`
  2. `SC_DOLT_BRANCH`
  3. `main`
- User-selectable branch values map directly to Dolt branch names in MVP
- The CLI ignores the current Dolt session branch for read behavior
- Read-path commands must not rely on session branch switching for correctness

### Verification Traceability
- Sprint acceptance criteria must map cleanly to tests or explicit QA checks
- Agent and script requirements must identify how they will be verified
- MVP verification is part of the product surface, not just development process
- Promotion across `develop`, `beta`, and `main` complements testing but does
  not replace it

### Code Quality
- `golangci-lint` with `gosec` enabled
- `go vet` clean
- No ignored errors (enforced by linter)
- Consistent code formatting (`gofmt`)

---

## Phase 1: Foundation

Scaffold the Go project, establish patterns, connect to Dolt.

### Sprint 1.1: Project Scaffold

**Goal:** Buildable Go binary with root command and global flags.

**Deliverables:**
- `src/go.mod` (module: `github.com/randlee/synaptic-canvas-dolt`, Go 1.26)
- `src/main.go` with version injection via ldflags
- `src/cmd/root.go` — root Cobra command with global flags (`--dolt-dir`, `--remote`, `--branch`, `--json`, `--quiet`, `--verbose`)
- `src/internal/logging/logger.go` — centralized `slog` setup
- `src/internal/output/formatter.go` — table + JSON output formatters
- `src/internal/config/config.go` — CLI configuration (flag parsing, defaults)
- `.golangci.yml` — linter configuration
- `.goreleaser.yml` — build configuration (following claude-history patterns)
- `.github/workflows/test.yml` — CI pipeline (PR and push triggers)
- Unit tests for config, logging, output formatters

**Acceptance Criteria:**
- `go build ./...` succeeds
- `go test ./... -race` passes
- `golangci-lint run` clean
- `sc --help` prints usage
- `sc --version` prints injected version
- `--branch` is available as a global flag
- JSON and table output formatters tested
- CI runs on PR and push to main/develop
- CI matrix: ubuntu/macOS/windows × Go 1.26
- CI steps: lint (`golangci-lint` with `gosec`), test (with `-race`), build, coverage
- Coverage reported to Codecov

### Sprint 1.2: Dolt Client

**Goal:** Database abstraction layer that can query Dolt.

**Deliverables:**
- `src/pkg/dolt/client.go` — Dolt client interface and implementation
- `src/pkg/dolt/queries.go` — SQL query builders for package operations
- `src/pkg/models/package.go` — Package, File, Dependency structs
- `src/pkg/models/manifest.go` — Manifest reconstruction from relational data
- Unit tests with mock Dolt client (interface-based testing)
- Integration test harness using test Dolt DB fixtures in `test/fixtures/`

**Acceptance Criteria:**
- Dolt client interface defined and mockable
- Can query packages, files, dependencies from Dolt
- Read-path queries resolve branch using `--branch`, then `SC_DOLT_BRANCH`, then `main`
- Read-path queries treat the supplied branch value as the Dolt branch name directly
- Read-path queries do not rely on session branch switching for correctness
- Models map cleanly to schema spec tables
- All query builders tested
- Manifest reconstruction tested against known fixture data

### Sprint 1.3: Integrity Package

**Goal:** SHA256 computation and verification library.

**Deliverables:**
- `src/pkg/integrity/types.go` — shared types: FileHash, VerifyStatus, VerifyResult
- `src/pkg/integrity/sha.go` — per-file SHA256 computation
- `src/pkg/integrity/aggregate.go` — package-level aggregate SHA (sorted dest_path:sha256 pairs)
- `src/pkg/integrity/verify.go` — comparison functions (OK, MODIFIED, MISSING, EXTRA)
- 100% test coverage on all integrity functions
- Test vectors with known SHA values

**Acceptance Criteria:**
- Per-file SHA matches `sha256sum` output for test fixtures
- Aggregate SHA is deterministic (same files in any order → same hash)
- Verify functions correctly classify OK/MODIFIED/MISSING/EXTRA
- Edge cases tested: empty files, binary content, unicode filenames
- 100% line coverage on integrity package

### Sprint 1.4: Log Debug Agent

**Goal:** Claude Code agent that tails `sc` logs and surfaces warnings/errors.

**Deliverables:**
- `.claude/agents/sc-log-debug.md` — log monitoring agent
- Agent definition aligned with `../synaptic-canvas/docs/claude-code-skills-agents-guidelines.md`
- Python helper scripts under `.claude/scripts/`
- Unit tests for all helper scripts
- Tails `~/.sc/logs/sc.log` in background
- Notifies when warnings or errors are encountered (count + summary)
- Supports on-demand filtering: by level, component, operation, time range, or regex pattern
- Can correlate log entries across a single operation (e.g., all logs from one `sc install` run)
- Formats findings for conversational presentation

**Acceptance Criteria:**
- Agent can be launched to monitor logs during development/testing
- Detects and reports new warnings/errors as they appear
- Filters work: `level:error`, `component:dolt`, `operation:install`, custom regex
- Time-range filtering: "last 5 minutes", "since 14:30"
- Output is concise — summarizes patterns, doesn't dump raw logs
- Can be asked to explain error context (surrounding log lines)
- All helper scripts have passing unit tests
- Sprint artifacts include explicit traceability from requirements to tests/QA checks

### Sprint 1.5: Phase 1 Gap Closure

**Goal:** Close Phase 1 implementation gaps discovered after the Phase 1
foundation work was completed.

**Deliverables:**
- Branch-safe read-path Dolt access that does not rely on session branch
  switching for correctness
- Consistent structured logging context across CLI layers, including
  `component` and `operation`
- Explicit handling of file-logging degradation so missing file logging is
  visible and testable
- Deterministic fixes for Phase 1 Python log-script tests
- Updated tests covering the fixed branch-resolution, logging, and script
  behavior

**Acceptance Criteria:**
- Read-path Dolt queries are correct under connection pooling and parallel
  branch reads
- Phase 1 logging output includes the documented context fields consistently
- File logging failure or degradation is surfaced explicitly rather than failing
  silently
- Python log-script tests pass deterministically without dependence on time of
  day or real user state
- Gap-closure changes trace back to the documented Phase 1 review findings and
  have corresponding tests or explicit QA checks

---

## Phase 2: Admin Commands

Import/export — the write path. Python prototypes (`tools/dolt-ingest.py`, `tools/dolt-export.py`) serve as reference implementations.

### Sprint 2.1: Admin Import

**Goal:** `sc admin import <path> --branch <branch>`

**Deliverables:**
- `src/cmd/admin/admin.go` — admin parent command
- `src/cmd/admin/import.go` — import command
- `src/pkg/dolt/writer.go` — Dolt write operations (INSERT/UPDATE)
- Import logic: scan directory → compute SHAs → write to Dolt → create commit
- Manifest parsing: read `manifest.yaml` → decompose into relational inserts
- `src/pkg/template/validator.go` — Jinja2 template variable validator (parse `.j2` files, cross-reference against `package_questions`, known `repo.*` schema, known `env.*` schema)
- Unit tests for import logic (mock Dolt writer)
- Unit tests for template validator (known-good and known-bad templates)
- Integration test: import test fixture → verify Dolt state

**Acceptance Criteria:**
- Imports a package directory into Dolt on specified branch
- Computes and stores per-file SHA256
- Computes and stores aggregate package SHA256
- Creates a Dolt commit with descriptive message
- Handles manifest.yaml decomposition matching schema spec
- Refuses to import if branch doesn't exist
- Runs template variable validation on `.j2` files — reports warnings for undeclared variables (non-blocking)
- `--json` output includes import summary (with template validation warnings if any)
- Matches behavior of `tools/dolt-ingest.py` for same input

### Sprint 2.2: Admin Export

**Goal:** `sc admin export <package> --output <dir> [--branch <branch>]`

**Deliverables:**
- `src/cmd/admin/export.go` — export command
- `src/pkg/manifest/reconstruct.go` — manifest.yaml reconstruction from DB
- `src/pkg/plugin/reconstruct.go` — plugin.json reconstruction from DB
- Export logic: query Dolt → write files → verify SHAs → reconstruct manifest
- Unit tests for export and reconstruction logic

**Acceptance Criteria:**
- Exports package from Dolt to filesystem
- If `--branch` is omitted, export reads from the effective branch resolved as
  `--branch`, then `SC_DOLT_BRANCH`, then `main`
- Reconstructs manifest.yaml from relational data
- Reconstructs plugin.json from relational data
- Verifies per-file SHA on each written file
- Verifies aggregate SHA after export
- Fails on any SHA mismatch
- `--json` output includes export summary
- Round-trip test: import → export → diff shows no content changes (manifest formatting may differ)

### Sprint 2.3: Admin Verify & Diff

**Goal:** `sc admin verify <package>` and `sc admin diff <package>`

**Deliverables:**
- `src/cmd/admin/verify.go` — verify command
- `src/cmd/admin/diff.go` — diff command
- Verify logic: recompute SHAs from stored content → compare against stored hashes
- Diff logic: compare package data across two branches
- Unit tests for verify and diff behavior

**Acceptance Criteria:**
- Verify detects OK and CORRUPT states for stored content
- If `--branch` is omitted, verify reads from the effective branch resolved as
  `--branch`, then `SC_DOLT_BRANCH`, then `main`
- Verify recomputes aggregate and compares against stored package SHA
- Diff shows file-level changes between branches
- Both commands support `--json` output

### Sprint 2.4: Admin Publish

**Goal:** `sc admin publish <package> --from <branch> --to <branch>`

**Deliverables:**
- `src/cmd/admin/publish.go` — publish command
- Dolt merge operations for targeted package promotion
- Pre-publish validation (verify SHAs before promoting)
- Pre-publish template variable validation (reuses `src/pkg/template/validator.go` from Sprint 2.1)
- Unit and integration tests for publish behavior and gating

**Acceptance Criteria:**
- Promotes package from one branch to another via Dolt merge
- Runs verify before publishing (fail if corrupt)
- Runs template variable validation as a **BLOCKING gate** — publish fails if any `.j2` template references undeclared variables
- Template validation errors include: unknown namespace, `answers.*` variable with no matching `package_questions` row, `repo.*` or `env.*` variable not in known schema
- `--json` output includes publish summary (with template validation results)
- Cannot publish to same branch

---

## Phase 3: End-User Commands

The read path. These commands never write to Dolt.

### Sprint 3.1: List & Info

**Goal:** `sc list` and `sc info <package>`

**Deliverables:**
- `src/cmd/list.go` — list command with `--branch` and `--tags` filters
- `src/cmd/info.go` — info command showing package details
- `src/pkg/dolt/queries.go` extensions for:
  - tag-filtered package listing
  - variant resolution lookups
  - package detail queries with file/dependency counts
  - package questions retrieval (required by `src/pkg/questionnaire/`)
  - package hooks retrieval (required by Sprint 3.2 hook registration)
- Table and JSON output for both
- Unit tests for list/info command behavior and output

**Acceptance Criteria:**
- Lists packages from specified branch, defaults to main
- Branch resolution follows `--branch`, then `SC_DOLT_BRANCH`, then `main`
- Branch values map directly to Dolt branch names
- `--tags` filtering semantics are explicit: split stored tags on commas, trim
  whitespace, compare case-insensitively, and treat multiple requested tags as
  OR filters
- Info shows: name, version, description, dependencies, file count, SHA,
  install scope, and variant when present
- Both support `--json` output
- Phase 3 end-user read commands reuse or extend the existing branch-qualified
  read client rather than introducing a second read path
- Human-readable output may render the `main` branch as an empty string for UX,
  but JSON output must emit `"main"` explicitly

### Sprint 3.2: Install

**Goal:** `sc install <package> [--global] [--branch <branch>] [--dry-run] [--yolo]`

**Deliverables:**
- `src/cmd/install.go` — install command
- `src/cmd/init.go` — repository bootstrap command for first-time setup
- `src/pkg/installer/installer.go` — file installation logic
- `src/pkg/installer/tracking.go` — installed package tracking (local state)
- `src/pkg/repo/` — repo detection, repo-profile generation, and init helpers
- `src/pkg/questionnaire/` — install and upgrade question prompting plus
  tracked-answer comparison
- Reconciliation-ready tracking primitives for repository install discovery,
  consumed by `sc scan` in Sprint 3.3
- Install logic: query Dolt → verify SHAs → write files → render templates → record install
- Dry-run mode for install planning and template preview
- `--yolo` mode for non-interactive approval of the computed install plan
- Post-install template verification: scan rendered `.j2` output for unresolved `{{ }}` patterns
- Unit and integration tests for install and tracking behavior

**Acceptance Criteria:**
- Installs package files to `.claude/` (local) or `~/.claude/` (global)
- Stores lockfiles, repo profile, hook registry state, cache, and temp files
  under `.synaptic/`
- Branch resolution follows `--branch`, then `SC_DOLT_BRANCH`, then `main`
- Branch values map directly to Dolt branch names
- Respects `install_scope` from packages table
- `local-only` packages fail fast if the user requests `--global`
- The initial lockfile/tracking schema is explicit and normative for Phase 3,
  including package identity, version, Dolt commit, branch, variant, install
  scope, materialized file inventory, answers, requirements snapshot,
  repo-profile snapshot, and template-validation state
- Install tracking records all repo-local and global installs on the machine;
  local and global installs of the same package are treated as normal
  independent tracked installs
- Install records dependency provenance: whether each dependency was already
  present or was installed by Synaptic Canvas during the operation
- Verifies per-file SHA after writing each file
- Verifies aggregate SHA after install
- Fails and rolls back on any SHA mismatch
- Renders `.j2` templates with repo profile + user answers context
- `sc install --dry-run` shows the install plan and template preview without
  side effects
- Standard install prompts the user to acknowledge external dependency
  installation before any CLI/tool dependency is installed
- `sc install --yolo` executes the computed plan without interactive prompts
  and still records dependency provenance and the final install plan in
  tracking state
- Post-install scan: warns if any rendered output contains unresolved `{{ }}` patterns (safety net)
- Records installed package/version/branch for status tracking, including `template_validation` in lockfile
- `sc init` creates or refreshes the minimum `.synaptic/` state artifacts
  required for Phase 3:
  - `.synaptic/manifest.lock`
  - `.synaptic/repo-profile.toml`
  - `.synaptic/env.toml`
  - `.synaptic/hooks/registry.toml`
- Install step 7 registers hooks and records the resulting hook registry state
  in tracking data
- State mutation uses atomic replacement and transactional staging for
  `.synaptic/` or `~/.synaptic/`; package artifacts under `.claude/` are never
  left under persistent product locks
- `--json` output includes install summary (with template validation results)
- `sc init` bootstraps `.synaptic/` state for a new repository and can be
  triggered implicitly by first install
- `sc init` is idempotent on an already initialized repository

**Implementation Sketch: tracking record**

```toml
version = 1

[[installs]]
install_id = "pkg_team-lead_project_ab12cd34"
package = "team-lead"
version = "1.2.0"
branch = "beta"
dolt_commit = "abc123def456"
scope = "project"
install_root = "/repo/.claude/skills/team-lead"
repo_path = "/repo"
variant = ""
template_validation = "ok"

[installs.requirements]
repo_profile_sha = "9d5a..."
answers_sha = "5a9d..."

[installs.question_snapshot]
question_ids = ["lang", "style"]

[installs.files]
".claude/skills/team-lead/SKILL.md" = "c0ffee..."

[installs.requirements.cli_provenance]
gh = "preexisting"
```

### Sprint 3.3: Validate, Status, Scan & Snapshot

**Goal:** `sc validate [<package>] [--all]`, `sc status`, `sc scan`, and `sc snapshot <package>`

**Deliverables:**
- `src/cmd/validate.go` — validate command
- `src/cmd/status.go` — status command
- `src/cmd/scan.go` — scan command for machine/repo inventory reconciliation
- `src/cmd/snapshot.go` — snapshot command for exporting local modifications
- Validate logic: recompute SHAs from installed files → compare against Dolt
  plus validate tracked dependency and component state
- Unit tests for validate, status, scan, and snapshot behavior

**Acceptance Criteria:**
- Validate supports project-local installs, global installs, or both in one
  invocation
- Scope-aware commands use `--scope`, and omitting `--scope` defaults to `both`
- Validate reports per-file: OK, MODIFIED, MISSING, UNREADABLE
- Validate reports extra files inside the package's managed install paths as
  EXTRA (untracked)
- Validate computes and checks aggregate SHA
- Validate also verifies tracked dependency presence, dependency version
  compatibility, hook registration state, and template-validation state needed
  for the package to function
- Validate inventories local modifications distinctly from corruption so local
  improvements can be analyzed later
- Validate output is list-based and each item includes an explicit severity
  level from `info`, `warn`, `error`, or `critical`
- Validate computes an aggregate package result and emits it in JSON with a
  stable field such as `aggregate_status`
- Status shows one row per package with columns for package, global
  version/branch, and local version/branch; human-readable output suppresses
  `main` to an empty branch string while JSON emits `"main"`
- `sc scan` discovers repository installs found on disk, shows version and
  upgrade status, and then offers `add all`, `upgrade all`, or `skip`
- `sc scan` defaults to the current folder, accepts a path list, and supports
  `--recurse`
- `sc snapshot <package>` exports modified tracked files by default and supports
  `--full` for full-package snapshots
- Snapshot exports are written under
  `~/.synaptic/mod-snapshots/<package>/<branch>/<repo>/`, where `<repo>` is a
  sanitized repository key such as `<base-folder-name>-<repo-id>`
- Snapshot metadata is recorded in `snapshot.toml`, including source path, repo
  URL, snapshot timestamp, version, branch, and scope
- Both support `--json` output
- JSON output is stable enough for Phase 4 wrappers to consume, including
  aggregate validation result shape and install-scope targeting
- Validate/upgrade/uninstall target concrete install records; status is the
  merged presentation view over those records
- Scan, snapshot, and validation tests cover duplicate repo names, unreadable
  files, mixed local/global installs, modified-vs-missing classification, and
  reconcile behavior for installs created on another machine

**Implementation Sketch: validation JSON item**

```json
{
  "package": "team-lead",
  "scope": "project",
  "branch": "main",
  "version": "1.2.0",
  "items": [
    {
      "type": "file",
      "path": ".claude/skills/team-lead/SKILL.md",
      "status": "MODIFIED",
      "severity": "warn",
      "expected_sha256": "abc123",
      "actual_sha256": "def456"
    },
    {
      "type": "dependency",
      "name": "gh",
      "status": "OK",
      "severity": "info",
      "version": "2.77.0"
    }
  ]
}
```

**Implementation Sketch: snapshot metadata**

```toml
package = "team-lead"
branch = "beta"
version = "1.2.0"
scope = "project"
source_path = "/repo/.claude/skills/team-lead"
repo_path = "/repo"
repo_url = "git@github.com:org/repo.git"
snapshot_time = "2026-04-24T18:42:00Z"
```

**Implementation Sketch: scan findings**

```json
[
  {
    "package": "team-lead",
    "scope": "project",
    "branch": "beta",
    "version": "1.2.0",
    "latest_on_branch": "1.3.0",
    "needs_upgrade": true,
    "repo": "client-repo-7f3a9c"
  }
]
```

### Sprint 3.4: Upgrade & Uninstall

**Goal:** `sc upgrade <package> [--all] [--scope <project|global|both>] [--branch <branch>] [--version <version>] [--yolo]` and `sc uninstall <package> [--scope <project|global|both>] [--yolo]`

**Deliverables:**
- `src/cmd/upgrade.go` — upgrade command
- `src/cmd/uninstall.go` — uninstall command
- Upgrade logic: check for newer version → refresh questions/profile/deps as
  needed → install → verify
- Uninstall logic: remove files → update tracking
- Unit and integration tests for upgrade and uninstall behavior

**Acceptance Criteria:**
- Upgrade supports project-local installs, global installs, or both in one
  invocation
- Scope-aware commands use `--scope`, and omitting `--scope` defaults to `both`
- Upgrade checks current vs available version on the tracked branch by default;
  branch changes are explicit via `--branch`
- Upgrade prompts for new questions added since install, re-renders templates
  if repo profile changed, verifies changed dependencies, and re-resolves
  variant selection
- Upgrade warns about local modifications before overwriting
- Upgrade supports the policy goal of keeping local/global installs up to date
  while still allowing users to target only local or only global installs
- `upgrade --all` warns and skips blocked upgrades that would violate
  dependency or compatibility requirements and continues valid upgrades
- `--force` is only available for explicitly targeted package upgrades; it is
  not supported as a blanket override for `upgrade --all`
- Uninstall removes only files and hooks owned exclusively by the target
  tracked install and preserves shared dependencies still needed elsewhere
- Uninstall asks whether dependencies installed by Synaptic Canvas should be
  removed and ignores dependencies that predated the install
- `--yolo` is supported for upgrade and uninstall; in uninstall it may only
  remove dependencies recorded as installed by Synaptic Canvas
- Uninstall behavior is explicit when tracked files are locally modified,
  missing, or discovered by scan rather than original local tracking
- Both support `--json` output
- Upgrade and uninstall tests cover dependency-order planning, blocked upgrade
  skip behavior, local/global mixed-state upgrades, branch changes, targeted
  version pinning, and interruption-safe state mutation

**Implementation Sketch: dependency-safe upgrade planning**

```go
func PlanBatchUpgrade(installs []TrackedInstall, req UpgradeRequest) ([]UpgradePlan, []BlockedUpgrade) {
    candidates := resolveCandidates(installs, req) // latest on tracked branch unless overridden
    ordered := topoSortByDependencies(candidates)

    var plans []UpgradePlan
    var blocked []BlockedUpgrade

    for _, candidate := range ordered {
        if err := validateDependencyClosure(candidate, plans); err != nil {
            blocked = append(blocked, BlockedUpgrade{
                Package: candidate.Package,
                Reason:  err.Error(),
            })
            continue
        }
        plans = append(plans, candidate)
    }

    return plans, blocked
}
```

### Sprint 3.5: SHA Catalog Subsystem

**Goal:** `sc catalog update [--branch <branch>]` and catalog-backed `sc validate`

**Deliverables:**
- `GetPackageCatalogQuery(database, branch)` in `src/pkg/dolt/queries.go` —
  returns `(id, version, dest_path, sha256)` for all packages on a branch
- `GetPackageCatalog(ctx)` on the `Client` interface in `src/pkg/dolt/client.go`
- `src/pkg/catalog/` — catalog struct, TOML read/write, per-branch cache file
- Cache written to `.synaptic/catalog-{branch}.toml` (local) and
  `~/.synaptic/catalog-{branch}.toml` (machine-level)
- `src/cmd/catalog.go` — `sc catalog update` command with explicit Dolt fetch
- Implicit catalog refresh triggered by `sc install` and `sc init`
- Update `validateTrackedInstall()` in `src/cmd/state_helpers.go` to look up
  expected SHAs from the local catalog rather than from `record.Files`
- Offline fallback: use cached catalog with a clear warning when Dolt is
  unavailable
- Unit tests for catalog read/write, Dolt fetch, and validate integration

**Acceptance Criteria:**
- `sc catalog update` fetches `(package_id, version, dest_path, sha256)` from
  Dolt and writes `.synaptic/catalog-{branch}.toml`
- `sc validate` compares recomputed on-disk SHAs against catalog SHAs as the
  authoritative source; lockfile is used only for install identity
- Validate emits a clear warning and uses cached catalog when Dolt is offline
- Catalog storage paths are cross-platform
- `sc install` and `sc init` refresh the catalog after completing their primary
  operation

---

### Sprint 3.6: sc scan Implementation

**Goal:** `sc scan [<path>...] [--recurse] [--scope <project|global|both>]`

**Deliverables:**
- `src/cmd/scan.go` — scan command
- Walk `.claude/` and `~/.claude/` dirs, compute SHA per file
- Look up computed SHAs in local catalog to identify
  `(package_id, version, dest_path)`
- Group matched files into discovered install candidates
- Present candidates before mutating state (ST-004a): `add all`, `upgrade all`,
  or `skip`
- Write valid lockfile entries for accepted installs
- `--json` output and `--scope` flag
- Unit tests covering: SHA match, no catalog hit, mixed local/global, stale
  catalog warning, reconcile accepted/skipped, interrupted reconcile safety

**Acceptance Criteria:**
- Discovers installed packages not in local tracking by SHA matching against catalog
- Shows version and upgrade status for each discovered install before
  mutating state
- Supports `add all` / `upgrade all` / `skip` choices (ST-004a)
- Writes valid `[[installs]]` records for accepted installs
- Works fully offline using cached catalog
- Does not mutate tracking state until user confirms

---

### Sprint 3.7: Import Collision Enforcement

**Goal:** Enforce the SHA immutability invariant (`CA-006`, `CA-007`) at import time.

**Deliverables:**
- Pre-import catalog check in `src/pkg/importer/importer.go`
- Before writing any package row, query existing catalog for `(package_id,
  dest_path)` on the target branch
- If entry exists with a different SHA → hard reject with error naming the
  colliding file and both SHAs
- Same version + different content = blocking error
- New version on the same branch = allowed (replaces current row; old state
  preserved in Dolt history via Dolt commit)
- Unit tests: collision detected, new version allowed, first import allowed,
  identical re-import allowed

**Acceptance Criteria:**
- Re-import of an existing `(package_id, dest_path, branch)` with different SHA
  is rejected before any write occurs
- Error message names the colliding file, the existing SHA, and the incoming SHA
- Import of a new version (different `package_id` version field) succeeds
- Identical re-import (same SHA) is a no-op and succeeds

---

### Sprint 3.8: Scope Flags, --yolo, Dep Acknowledgement, Validation Severity

**Goal:** Complete the behavioral requirements from IU-* and VA-* sections that
were planned in Sprints 3.2–3.4 but not yet implemented.

**Deliverables:**
- Replace `--global` bool with `--scope <project|global|both>` on: `validate`,
  `status`, `upgrade`, `uninstall` (IU-007)
- `--yolo` flag on `install`, `upgrade`, `uninstall` (IU-002, IU-008)
- Interactive dependency acknowledgement in `install`: prompt before installing
  any external CLI/tool dependency unless `--yolo` (IU-001)
- `severity` field on each validate output item using fixed vocabulary
  `info`, `warn`, `error`, `critical` (VA-005, VA-005a)
- Upgrade warn-and-skip for blocked upgrades with continuation of valid upgrades
  (IU-010)
- `--force` for single targeted upgrade override only; not valid with
  `upgrade --all` (IU-011)
- Dep provenance in uninstall: offer removal only for deps recorded as
  installed by Synaptic Canvas; leave predating deps untouched (IU-004, IU-005)
- JSON output emits branch as `"main"` explicitly, never as empty string
  (CLI-006)
- Unit tests for each new flag, severity classification, blocked-upgrade skip,
  and provenance-aware uninstall

**Acceptance Criteria:**
- All scope-aware commands accept `--scope` enum; default is `both`
- `--yolo` skips interactive prompts while still recording full provenance
- Install presents external dep plan for acknowledgement before installing,
  skipped only with `--yolo`
- Each validate output item has a `severity` field from the fixed vocabulary
- `upgrade --all` warns on each blocked candidate and skips it; continues
  remaining valid upgrades
- `--force` accepted only on single targeted upgrade; rejected for `--all`
- Uninstall asks about SC-installed deps; ignores predating deps silently

---

## Phase 4: Skill & Installer

### Sprint 4.1: sc:plugin Skill

**Goal:** Claude Code skill that wraps `sc` CLI for conversational package management.

**Deliverables:**
- Skill markdown file: maps natural language → `sc` CLI commands with `--json`
- Parses JSON output for conversational presentation
- Handles error cases gracefully
- Verification fixtures or golden examples for the wrapper behavior

**Design Constraints:**
- The skill is an AI wrapper and should invoke `sc` with `--json` explicitly
- The skill must remain a thin wrapper with no business logic
- The skill should pass `--branch` explicitly when operating against a
  non-default branch

**Acceptance Criteria:**
- "list packages" → `sc list --json` → conversational response
- "install delay" → `sc install sc-delay-tasks --json` → conversational response
- Skill is a thin wrapper — no business logic in the skill
- Error messages from CLI presented clearly
- Non-default branch workflows remain explicit in the CLI invocation rather
  than hidden in skill logic

### Sprint 4.2: Installer Script

**Goal:** Install script that sets up both `sc` binary and `sc:plugin` skill.

**Deliverables:**
- `scripts/install.sh` — macOS/Linux installer
- `scripts/install.ps1` — Windows installer (or winget manifest)
- Installs `sc` binary to PATH
- Installs `sc:plugin` skill globally to `~/.claude/`
- Installation verification on supported platforms

**Acceptance Criteria:**
- Fresh install works on macOS, Linux, Windows
- Upgrade preserves configuration
- `sc --version` works after install
- `sc:plugin` skill available in Claude Code after install

---

## Phase 5: Release Pipeline

### Sprint 5.1: Release Pipeline

**Goal:** Tag-triggered GoReleaser publish.

**Deliverables:**
- `.github/workflows/release.yml` — tag-triggered release
- GoReleaser configuration for cross-platform builds
- Homebrew tap update (`randlee/homebrew-tap`)
- Checksums and release notes

**Acceptance Criteria:**
- Tag push `v*` triggers release
- Binaries for linux/darwin (amd64, arm64), windows (amd64)
- Homebrew formula updated automatically
- SHA256 checksums published
- Release notes generated from commits

---

## Phase Summary

| Phase | Sprints | Focus |
|-------|---------|-------|
| 1. Foundation | 1.1–1.5 | Scaffold + CI pipeline, Dolt client, integrity, log-debug agent, gap closure |
| 2. Admin | 2.1–2.4 | Import, export, verify, publish |
| 3. End-User | 3.1–3.4 | List, install, validate, upgrade |
| 4. Skill | 4.1–4.2 | sc:plugin skill, installer |
| 5. Release | 5.1 | GoReleaser release pipeline |

---

## MVP Release Gate

The MVP should not be considered production-ready until all of the following are
true:

- Phase 1 through Phase 5 deliverables are complete
- Cross-cutting branch-resolution rules are implemented consistently
- Structured logging and integrity verification behave as documented
- All documented CLI JSON contracts required for AI wrappers are implemented
- Repository-owned helper scripts and CLI/package code have passing automated
  tests
- Remaining verification gaps are documented explicitly rather than implied away

---

## Future Validation Considerations

These are not required for MVP completion, but they should be considered in
later planning so verification remains part of the product story:

- Dedicated package test harness for scripts and agents
- Per-package eval suites for behavior that cannot be covered fully by
  conventional tests
- Optional storage of validation evidence in package metadata, schema-adjacent
  records, or other auditable package-associated state

---

## Document History

| Date | Change |
|------|--------|
| 2026-02-22 | Initial project plan |
| 2026-02-22 | Move CI pipeline from Phase 5 into Sprint 1.1; Phase 5 now release-only |
| 2026-02-22 | Add template variable validation to Sprints 2.1 (validator + warning), 2.4 (blocking gate), 3.2 (post-install scan) |
| 2026-04-08 | Align plan with `requirements.md` and `architecture.md`, including explicit branch resolution and AI JSON access |
| 2026-04-08 | Add Sprint 1.5 for Phase 1 gap closure and strengthen verification/test expectations across later phases |
| 2026-04-08 | Add MVP release gate and future validation considerations (test harness, evals, validation evidence) |
