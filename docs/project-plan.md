# Synaptic Canvas — Project Plan

## Overview

Phased plan for building the `sc` Go CLI. Each phase contains sprints. Each sprint runs through the dev-QA loop defined in `CLAUDE.md`.

**Reference project:** `claude-history` (Go + Cobra + GoReleaser conventions)
**Design docs:** See [CLAUDE.md](../CLAUDE.md) for full list
**Normative docs:** [`requirements.md`](./requirements.md),
[`architecture.md`](./architecture.md), and accepted ADRs under
[`docs/adr/`](./adr/)

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
- Sprint plans and QA reviews should cite the relevant requirement IDs and ADR
  IDs for the behavior under review
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
- `src/pkg/integrity/aggregate.go` — package-level aggregate SHA (sorted doc_path:sha256 pairs)
- `src/pkg/integrity/verify.go` — comparison functions (`ok`, `modified`, `missing`, `extra`)
- 100% test coverage on all integrity functions
- Test vectors with known SHA values

**Acceptance Criteria:**
- Per-file SHA matches `sha256sum` output for test fixtures
- Aggregate SHA is deterministic (same files in any order → same hash)
- Verify functions correctly classify `ok`/`modified`/`missing`/`extra`
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

**Goal:** `sc list` and `sc info <package> [--branch <branch>]`

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

**Goal:** `sc install <package> [--scope <project|global|both>] [--branch <branch>] [--dry-run] [--yolo]`

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
- Installs package files to `.claude/` (project), `~/.claude/` (global), or
  both according to `--scope`; omitted `--scope` defaults to `both`
- Stores lockfiles, repo profile, hook registry state, cache, and temp files
  under `.synaptic/`
- Branch resolution follows `--branch`, then `SC_DOLT_BRANCH`, then `main`
- Branch values map directly to Dolt branch names
- Respects `install_scope` from packages table
- `local-only` packages fail fast with no partial install if the requested
  `--scope` includes `global`
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
install_scope = "project"
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
".claude/skills/team-lead/SKILL.md" = "c0ffee..." # key is materialized path

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
- Validate reports per-file: `ok`, `modified`, `missing`, `unreadable`
- Validate reports extra files inside the package's managed install paths as
  `extra` (untracked)
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
      "doc_path": "skills/team-lead/SKILL.md",
      "materialized_path": ".claude/skills/team-lead/SKILL.md",
      "status": "modified",
      "severity": "warn",
      "expected_sha256": "abc123",
      "actual_sha256": "def456"
    },
    {
      "type": "dependency",
      "name": "gh",
      "status": "ok",
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
### Sprint 3.5: HTTPClient, File Config, and read_helpers Rewire

**Goal:** Three tightly coupled deliverables that must land together: (1) a
file-based layered config system, (2) `HTTPClient` implementing `dolt.Client`
via the DoltHub REST API, (3) `read_helpers.go` rewired to default to
`HTTPClient` instead of requiring a local `.dolt` directory.

**Background:** DoltHub.com exposes HTTP REST only — not MySQL wire protocol.
`openReadClient` currently requires a `.dolt` directory and uses `CLIReader` or
`SQLClient`. Without this sprint, every end-user command fails with
"could not auto-detect Dolt database directory." See `docs/dolt-api.md` for
API contracts (DC-001 through DC-009).

---

#### Deliverable A: File-based Config System

`src/internal/config/fileconfig.go` — layered config: explicit CLI flags → env → file → default.
`src/internal/config/keys.go` — all config key constants (see Config Constants section below).

`fileconfig.go` **extends** the existing `config.Config` struct. The existing
`NewConfigFromFlags`, `EffectiveBranch`, `Validate`, and `DoltDirExpanded`
functions are preserved, but `NewConfigFromFlags` must also record which CLI
flags were explicitly supplied. `fileconfig.go` adds `LoadFileConfig() error`
method that reads `~/.sc/config.toml` and stores values in a
`fileValues map[string]string` field added to `Config`. `Get` and `GetInt`
resolve values in this order:

1. explicitly supplied CLI flag mapped to that config key
2. environment variable mapped to that key
3. config file value
4. supplied default

Unset CLI flag defaults must not override environment or file config. Existing
struct fields (`DoltDir`, `Branch`, etc.) continue to carry CLI flag values for
existing command code.

Config file location: `~/.sc/config.toml` (created on first `sc config set`).
Format: TOML. Schema:

```toml
[dolt]
client   = "http"                      # "http" | "sql" | "cli"; default "http"
host     = "www.dolthub.com"           # DoltHub host
database = "randlee/synaptic-canvas"   # owner/database slug
token    = ""                          # DoltHub API token; empty = public read
dsn      = ""                          # MySQL DSN; required when client = "sql"
dir      = ""                          # local dolt dir; required when client = "cli"
timeout  = 30                          # HTTP request timeout in seconds; default 30
```

New methods on `config.Config`:

```go
// Get returns the string value for key from the merged config.
// Precedence: explicit CLI flag > environment > config file > defaultVal.
// Returns defaultVal if key is absent or empty.
func (c *Config) Get(key, defaultVal string) string

// GetInt returns the int value for key. Returns defaultVal on parse error or absent.
func (c *Config) GetInt(key string, defaultVal int) int
```

Required implementation shape:

```go
func (c *Config) Get(key, defaultVal string) string {
    if val, ok := c.explicitFlagValue(key); ok && val != "" {
        return val
    }
    if val := os.Getenv(EnvNameForKey(key)); val != "" {
        return val
    }
    if val := c.fileValues[key]; val != "" {
        return val
    }
    return defaultVal
}
```

Load precedence: explicit CLI flags → environment variables → config file → defaults.
Environment variable naming: `SC_DOLT_CLIENT`, `SC_DOLT_HOST`, etc.

`src/cmd/configcmd.go` — `sc config get <key>` and `sc config set <key> <value>`.
Both human and `--json` output. Validates key against known schema; rejects
unknown keys with clear error.

---

#### Deliverable B: HTTPClient

`src/pkg/dolt/http_client.go`:

```go
type HTTPClient struct {
    baseURL  string        // https://www.dolthub.com/api/v1alpha1
    database string        // randlee/synaptic-canvas
    branch   string        // resolved effective branch
    token    string        // empty for public repos
    http     *http.Client  // custom client with timeout; never http.DefaultClient
}

// HTTPConfig holds all parameters for NewHTTPClient.
type HTTPConfig struct {
    Host     string        // e.g. "www.dolthub.com"
    Database string        // e.g. "randlee/synaptic-canvas"
    Branch   string        // effective branch
    Token    string        // empty = unauthenticated
    Timeout  time.Duration // 0 uses DefaultHTTPTimeout (30s)
}

const DefaultHTTPTimeout = 30 * time.Second

func NewHTTPClient(cfg HTTPConfig) *HTTPClient

// query executes SQL against the DoltHub API.
// Uses GET with ?q=. The DoltHub SQL API read endpoint does not support POST.
// Branch is URL-path-encoded via url.PathEscape.
func (c *HTTPClient) query(ctx context.Context, sql string) ([]map[string]any, error)
```

DoltHub REST API response envelope:

```go
type apiResponse struct {
    QueryExecutionStatus  string           `json:"query_execution_status"`
    QueryExecutionMessage string           `json:"query_execution_message"`
    RepositoryOwner       string           `json:"repository_owner"`
    RepositoryName        string           `json:"repository_name"`
    CommitRef             string           `json:"commit_ref"`
    SQLQuery              string           `json:"sql_query"`
    Schema                []apiColumn      `json:"schema"`
    Rows                  []map[string]any `json:"rows"`
}

type apiColumn struct {
    Name       string `json:"columnName"`
    Type       string `json:"columnType"`
}
```

**Response type mapping** — DoltHub commonly returns SQL values as JSON strings.
Decode rules:
- String columns → `string`; absent key or JSON `null` → empty string `""`
- Integer columns → accept JSON string or JSON number; parse/cast to `int64`;
  JSON `null` → `0`
- Boolean columns (e.g. `is_template`) → accept `"0"`, `"1"`, `0`, `1`,
  `false`, or `true`
- `json.RawMessage` columns (e.g. `frontmatter`) → kept as raw string;
  caller decodes further

**HTTP error handling:**
- 200 → parse response
- 400 → bad SQL; return `ErrBadQuery` with body text
- 401/403 → authentication failure; return `ErrUnauthorized`
- 404 → branch or database not found; return `ErrNotFound`
- 429 → rate limited; return `ErrRateLimited` with `Retry-After` seconds if present
- 5xx → server error; return `ErrServerError` with status code
- Timeout (context deadline) → context error propagated as-is

**Query dialect strategy:**
- HTTP queries run against the repository/ref selected by the URL and shall use
  unqualified table names such as `packages` and `package_files`.
- HTTP queries cannot use database/sql parameter placeholders. They must be
  built with a small, reviewed set of query-builder functions that escape
  string literals with the existing SQL literal escaping rules.
- SQLClient queries may keep branch-qualified table references and
  database/sql placeholders.
- CLIReader may keep subprocess SQL with escaped literals.
- Tests must cover that HTTP builders do not emit `` `database/branch`.table ``
  references and do not leave `?` placeholders in the SQL sent to DoltHub.

All sentinel errors defined in `src/pkg/dolt/errors.go`:
```go
var (
    ErrNotFound     = errors.New("dolt: not found")
    ErrUnauthorized = errors.New("dolt: unauthorized")
    ErrBadQuery     = errors.New("dolt: bad query")
    ErrRateLimited  = errors.New("dolt: rate limited")
    ErrServerError  = errors.New("dolt: server error")
)
```

---

#### Deliverable C: read_helpers.go Rewire

Replace `.dolt`-directory detection with config-driven client construction.

`src/cmd/read_helpers.go` — new `openReadClient`:

```go
func openReadClient(cfg *config.Config) (readClient, error) {
    clientType := cfg.Get("dolt.client", "http")
    branch := cfg.EffectiveBranch()

    switch clientType {
    case "http":
        return dolt.NewHTTPClient(dolt.HTTPConfig{
            Host:     cfg.Get("dolt.host", "www.dolthub.com"),
            Database: cfg.Get("dolt.database", ""),
            Branch:   branch,
            Token:    cfg.Get("dolt.token", ""),
            Timeout:  time.Duration(cfg.GetInt("dolt.timeout", 30)) * time.Second,
        }), nil
    case "sql":
        dsn := cfg.Get("dolt.dsn", "")
        if dsn == "" {
            return nil, errors.New("dolt.client=sql requires dolt.dsn to be set")
        }
        sqlCfg, err := dolt.ParseDSN(dsn) // see ParseDSN below
        if err != nil {
            return nil, fmt.Errorf("parsing dolt.dsn: %w", err)
        }
        return dolt.OpenForBranch(sqlCfg, branch)
    case "cli":
        dir := cfg.Get("dolt.dir", "")
        if dir == "" {
            return nil, errors.New("dolt.client=cli requires dolt.dir to be set")
        }
        return dolt.NewCLIReader(dir, branch), nil
    default:
        return nil, fmt.Errorf("unknown dolt.client value %q; must be http, sql, or cli", clientType)
    }
}
```

**Call site migration** — every call site using the 2-arg `readClientOpener` must
be updated. Known call sites as of this sprint:
- `src/cmd/read_helpers.go:31` — `openReadClient` definition (replaced)
- `src/cmd/read_helpers.go` — `readClientOpener` var (updated to new signature)
- `src/cmd/upgrade.go:96` — uses `readClientOpener` (migrated)
- `src/cmd/install_mutation_helpers.go:77` — uses `readClientOpener` (migrated)
- Any test file that sets `readClientOpener` directly (all must be updated)

`detectReadDoltDir`, `detectReadDoltDirImpl`, and `loadConfigAndReadDoltDir` are
removed. `withReadClient` calls `openReadClient(cfg)` directly. Signature changes:

```go
func withReadClient(cmd *cobra.Command, fn func(*config.Config, readClient) error) error
```

`dolt.client = ""` (missing from config file) defaults to `"http"`.
If `dolt.database` is empty and client type is `http`, return a clear error:
`"dolt.database is not configured; run: sc config set dolt.database <owner/database>"`.

---

#### Config Constants and DSN Parser

`src/internal/config/keys.go` (new file — list as deliverable):

```go
// src/internal/config/keys.go
const (
    KeyDoltClient   = "dolt.client"
    KeyDoltHost     = "dolt.host"
    KeyDoltDatabase = "dolt.database"
    KeyDoltToken    = "dolt.token"
    KeyDoltDSN      = "dolt.dsn"
    KeyDoltDir      = "dolt.dir"
    KeyDoltTimeout  = "dolt.timeout"
)
```

`src/pkg/dolt/client.go` — add `ParseDSN(dsn string) (Config, error)`:

```go
// ParseDSN parses a MySQL DSN string (user:pass@tcp(host:port)/dbname)
// into a dolt.Config. Returns error if DSN is malformed.
func ParseDSN(dsn string) (Config, error)
```

Uses `github.com/go-sql-driver/mysql` DSN parser:
`cfg, err := mysqldrv.ParseDSN(dsn)` and maps fields to `dolt.Config`.

---

#### readClient / dolt.Client Interface Note

Sprint 3.5 **defers** the `type readClient = dolt.Client` alias to Sprint 3.6.
Reason: `GetPackageCatalog` method is added to `dolt.Client` in Sprint 3.6; the
alias cannot be created without it. Sprint 3.5 leaves `readClient` as the
existing local interface definition — no breaking change. Sprint 3.6 acceptance
criteria include creating the alias once `GetPackageCatalog` is added.

---

#### Test Requirements

Unit tests (`httptest.NewServer` — no live network calls):

- Empty response (zero rows) → returns empty slice, no error
- HTTP 404 (branch not found) → returns `ErrNotFound`
- HTTP 401/403 → returns `ErrUnauthorized`
- HTTP 500 → returns `ErrServerError`
- Malformed JSON response → returns wrapped parse error
- Context timeout → returns context error
- NULL column values → decoded to zero values (empty string, 0, false)
- Unexpected extra JSON fields → ignored, no error
- Branch name with `/` → URL-path-encoded correctly in request URL
- Empty `dolt.database` → clear error before any HTTP call
- HTTP query builder emits GET requests only and returns a clear error if a
  generated query would exceed the supported URL length budget
- Rate limit (429) → returns `ErrRateLimited`

Manual/AI-driven live integration test (tagged `//go:build integration`,
excluded from CI and never run by default):

```go
//go:build integration
// Run only when a human/AI explicitly opts in:
// SC_RUN_LIVE_DOLTHUB=1 \
// SC_TEST_DOLT_DATABASE=randlee/synaptic-canvas-dolt-test \
// SC_TEST_DOLT_BRANCH=main \
// go test -tags integration ./src/pkg/dolt/ -run TestHTTPClientLive
func TestHTTPClientLive(t *testing.T) {
    // Skips unless SC_RUN_LIVE_DOLTHUB=1.
    // Uses SC_TEST_DOLT_DATABASE and SC_TEST_DOLT_BRANCH so the live target is
    // not hard-coded. A dedicated project test repo with deterministic package
    // fixture data is preferred.
}
```

Additional live integration tests for retained Dolt clients:

```go
//go:build integration
// Run only when a human/AI explicitly opts in:
// SC_RUN_SQL_DOLT=1 \
// SC_TEST_DOLT_DSN='user:pass@tcp(127.0.0.1:3306)/synaptic_canvas' \
// go test -tags integration ./src/pkg/dolt/ -run TestSQLClientLive
func TestSQLClientLive(t *testing.T) {
    // Skips unless SC_RUN_SQL_DOLT=1.
    // Requires a local dolt sql-server process running from dolt 1.88.0 at
    // /opt/homebrew/bin/dolt and a DSN supplied through SC_TEST_DOLT_DSN.
}

//go:build integration
// Run only when a human/AI explicitly opts in:
// SC_RUN_CLI_DOLT=1 \
// SC_TEST_DOLT_DIR=/path/to/local/synaptic-canvas-dolt-repo \
// go test -tags integration ./src/pkg/dolt/ -run TestCLIReaderLive
func TestCLIReaderLive(t *testing.T) {
    // Skips unless SC_RUN_CLI_DOLT=1.
    // Requires the dolt binary in PATH and SC_TEST_DOLT_DIR pointing at a local
    // repository clone with deterministic fixture data.
}
```

Flaky test prevention:
- All unit tests use `httptest.NewServer` exclusively
- Live integration test tagged `integration`, additionally gated by
  `SC_RUN_LIVE_DOLTHUB=1`, `SC_RUN_SQL_DOLT=1`, or `SC_RUN_CLI_DOLT=1`, and
  excluded from default CI via `.github/workflows/test.yml` (no
  `-tags integration` in CI matrix)

`readClient` interface duplication: Sprint 3.5 must consolidate or alias
`cmd.readClient` with `dolt.Client`. Preferred approach: add `GetPackageCatalog`
to `dolt.Client` in Sprint 3.6 and make `readClient` a type alias
`type readClient = dolt.Client`.

**Acceptance Criteria:**

- `sc list` returns correct results against `httptest` fixtures in automated
  tests; live DoltHub verification is a manual/AI-driven integration procedure
  against a configured project test repo, not a CI requirement
- `sc list` works with httptest mock in unit tests (no network)
- Branch resolution (`--branch`, `SC_DOLT_BRANCH`, `main`) works via HTTP API
- `dolt.database` not set → clear error message naming the config key
- `sc config set dolt.database randlee/synaptic-canvas` writes `~/.sc/config.toml`
- `sc config get dolt.database` reads it back
- Config precedence is tested: explicit CLI flag overrides environment,
  environment overrides file config, file config overrides defaults, and unset
  CLI flag defaults do not override lower layers
- HTTP 401/403 → error message mentions token configuration
- `SQLClient` and `CLIReader` still compile and pass existing tests
- No automated test makes a live network call
- `test.yml` CI workflow does NOT pass `-tags integration`; `TestHTTPClientLive`,
  `TestSQLClientLive`, and `TestCLIReaderLive` also skip unless their explicit
  env gates are set
- `TestSQLClientLive` documents and uses `SC_TEST_DOLT_DSN`; required manual
  setup is local `dolt sql-server` from dolt 1.88.0 at `/opt/homebrew/bin/dolt`
- `TestCLIReaderLive` documents and uses `SC_TEST_DOLT_DIR`; required manual
  setup is `dolt` in PATH and a local repository clone with fixture data
- `src/internal/config/keys.go` exists with all seven constants

**Requirements:** DC-001, DC-002, DC-003, DC-004, DC-005, DC-006, DC-007, DC-008, DC-009,
BR-001 through BR-006

---

### Sprint 3.6: SHA Catalog Subsystem

**Goal:** `sc catalog update [--branch <branch>]` and catalog-backed `sc validate`

**Sprint dependency:** Sprint 3.6 depends on Sprint 3.5 completion. Do not begin
until Sprint 3.5 is merged to the integration branch.

**Background:** The catalog is a locally-cached, branch-scoped set of known
immutable `(package_id, version, doc_path, sha256)` tuples from Dolt. `doc_path`
is the package-root-relative artifact path, not the installed filesystem path.
The catalog preserves previously observed versions when refreshed so global and
repo-local installs can remain valid while different machines or repositories
upgrade at different times. It enables offline validation and is the
authoritative expected-SHA source for `sc validate`. `record.Files` SHAs in the
lockfile are NOT the authoritative source — the catalog is. See CA-001 through
CA-007 in requirements.md.

---

#### Deliverable A: Client Interface Extension

Add `GetPackageCatalog(ctx context.Context) ([]CatalogEntry, error)` to the
`dolt.Client` interface in `src/pkg/dolt/client.go`.

**All four implementations must be updated:**
- `HTTPClient` (`src/pkg/dolt/http_client.go`) — query DoltHub REST API
- `SQLClient` (`src/pkg/dolt/client.go`) — query via MySQL protocol
- `CLIReader` (`src/pkg/dolt/cli_reader.go`) — query via subprocess
- `MockClient` (`src/pkg/dolt/mock.go`) — return configurable test data via new
  `CatalogEntries []CatalogEntry` field

`GetPackageCatalogQuery(database, branch string) string` added to `queries.go`.
Returns all current `(package_id, version, doc_path, sha256)` tuples visible on
the branch. For the current schema, `package_files.dest_path` is selected as
`doc_path`.

Sprint 3.6 creates the `readClient` type alias:
```go
// src/cmd/read_helpers.go
type readClient = dolt.Client
```
This is possible only after `GetPackageCatalog` is added to `dolt.Client` in this sprint.

Also update `src/cmd/root.go` `NewRootCmd` to register `NewCatalogCmd()` as a subcommand.

---

#### Deliverable B: Catalog Package

`src/pkg/catalog/catalog.go`:

```go
type CatalogEntry struct {
    PackageID string `toml:"package_id"`
    Version   string `toml:"version"`
    DocPath   string `toml:"doc_path"`
    SHA256    string `toml:"sha256"`
}

type Catalog struct {
    Meta    CatalogMeta    `toml:"meta"`
    Entries []CatalogEntry `toml:"entries"`
}

type CatalogMeta struct {
    Branch        string    `toml:"branch"`
    FetchedAt     time.Time `toml:"fetched_at"`
    SchemaVersion int       `toml:"schema_version"` // current = 1
}
```

TOML file format:

```toml
[meta]
branch         = "main"
fetched_at     = "2026-04-25T12:00:00Z"
schema_version = 1

[[entries]]
package_id = "team-lead"
version    = "1.2.0"
doc_path   = "skills/team-lead/SKILL.md"
sha256     = "abc123..."

[[entries]]
package_id = "team-lead"
version    = "1.2.0"
doc_path   = "skills/team-lead/README.md"
sha256     = "def456..."
```

Storage paths:
- Local (project): `.synaptic/catalog-{sanitized-branch}.toml`
- Machine: `~/.synaptic/catalog-{sanitized-branch}.toml`

Branch name sanitization for path component: replace `/` with `_` and any
character outside `[a-zA-Z0-9._-]` with `_`. Use
`catalog.SanitizeBranchName(branch) string`. This prevents path traversal
(e.g. `../../etc/evil` → `______etc_evil`).

---

#### Deliverable C: Catalog Write/Read

Write target and read precedence (explicit):

- `sc catalog update` with no `--scope`: writes both project and machine
  catalogs. With `--scope global`: writes machine catalog only. With
  `--scope project`: writes local catalog only.
- Catalog refresh merges new entries into the existing catalog and preserves
  older versions. It must not truncate historical entries for the same branch
  just because the branch currently points at a newer package version.
- `validateTrackedInstall()` reads local catalog first; falls back to machine
  catalog if local absent.
- Concurrent writes: catalog file writes use `writeTOMLAtomic` (temp file +
  rename). Last writer wins — catalogs are idempotent caches. No advisory
  locking required.

---

#### Deliverable D: Offline Fallback Chain

When `validateTrackedInstall()` needs catalog SHAs:

1. Local catalog exists → use it (emit warn if `fetched_at` > 24 hours ago)
2. Local absent, machine catalog exists → use machine catalog with warning
3. Both absent, Dolt reachable → fetch and cache, then use
4. Both absent, Dolt unreachable → **fall back to lockfile `record.Files` SHAs**
   with warning: `"catalog unavailable and Dolt offline; using lockfile SHAs
   (may be stale — run sc catalog update when online)"`

Option 4 must NOT silently pass validation — it emits a warning that appears in
both human and JSON output.

**CA-004 qualification:** CA-004 states the catalog is the authoritative SHA
source. Step 4 is the only exception — it applies exclusively when both catalog
and Dolt are unavailable. CA-004 applies in all other scenarios.

---

#### Deliverable E: sc catalog update command

`src/cmd/catalog.go`:
- `sc catalog update [--branch <branch>] [--scope <project|global|both>]`
- Fetches from Dolt via `client.GetPackageCatalog(ctx)`
- Writes to appropriate path(s) per scope
- `--json` output: `{"branch":"main","entries":42,"path":"~/.synaptic/catalog-main.toml"}`
- Implicit refresh triggered by `sc install` and `sc init` (after primary op
  completes; failure is non-fatal — emit warning, continue)
- Register `NewCatalogCmd()` in `src/cmd/root.go` `NewRootCmd`

`src/pkg/catalog/atomic.go` — `writeTOMLAtomic(path string, v any) error`.

**Note:** An identical private `writeTOMLAtomic` already exists at
`src/pkg/installer/tracking.go:161`. Sprint 3.6 adds a second copy in the
catalog package rather than extracting a shared utility. Rationale: both packages
are internal implementation details with no external consumers; extracting to
`src/internal/atomicfile/` would require modifying the installer package in this
sprint. Add a comment in both copies: `// NOTE: duplicate of tracking.writeTOMLAtomic;
// a future refactor may extract this to src/internal/atomicfile/`.
If the sprint developer prefers extraction, that is acceptable provided
installer tests still pass.

`src/pkg/catalog/atomic.go` spec:
- Marshal `v` to TOML using `github.com/BurntSushi/toml` (or `github.com/pelletier/go-toml/v2`)
- Write to a temp file in the same directory as `path` (same filesystem, no cross-device rename)
- `os.Rename` temp → path (atomic on linux/darwin; best-effort on Windows)
- Temp file name: `path + "." + random hex + ".tmp"`
- On any error: remove temp file before returning

`src/cmd/state_helpers.go` — refactor `validateTrackedInstall()`:
- Current: uses `record.Files[destPath].SHA256` as expected SHA
- New: load catalog via `catalog.Load(synapticDir, branch)`, look up
  `entry.SHA256` by `(package_id, version, doc_path)`; fall back to lockfile
  SHAs per the four-step chain in Deliverable D if catalog absent

---

#### Test Requirements

Each test must use its own `t.TempDir()`. Do not share temp directories across
parallel tests.

Mandatory test cases:
- Corrupt/invalid TOML in catalog file → returns wrapped parse error, not panic
- `schema_version` mismatch (future schema) → warning + best-effort parse
- Branch name with path-unsafe characters → sanitized correctly in filename
- Empty catalog (zero entries) → valid; validate skips SHA check with info log
- Catalog entry with no matching lockfile entry → ignored (catalog may contain
  older or newer versions than the current install)
- Validate with catalog absent + Dolt unreachable → falls back to lockfile SHAs
  with warning in output
- Catalog refresh failure during `sc install` → install succeeds, warning emitted
- Concurrent writes: two goroutines writing same catalog file → no corruption
  (atomic rename provides last-writer-wins safety)

**Acceptance Criteria:**

- `sc catalog update` fetches and writes `(package_id, version, doc_path, sha256)`
- `sc validate` uses catalog SHAs as authoritative source
- Validate emits a warning and uses fallback when catalog absent
- Branch names with `/` produce valid, non-colliding catalog filenames
- All four `dolt.Client` implementations compile with `GetPackageCatalog`
- `type readClient = dolt.Client` alias in `src/cmd/read_helpers.go`
- `NewCatalogCmd()` registered in `src/cmd/root.go`
- `writeTOMLAtomic` exists in `src/pkg/catalog/atomic.go` with temp+rename logic
- Parallel tests each use independent `t.TempDir()`

**Requirements:** CA-001 through CA-007

---

### Sprint 3.7: sc scan Implementation

**Goal:** `sc scan [<path>...] [--recurse] [--scope <project|global|both>]`

---

#### Deliverable A: Scan Walk and SHA Matching

`src/cmd/scan.go`:

- Walk target directories based on `--scope`:
  - `project`: walk `.claude/` in current directory
  - `global`: walk `~/.claude/`
  - `both` (default): walk both
- Custom paths override scope when provided as positional args
- For each file encountered: compute SHA256, derive a candidate `doc_path` from
  the package metadata/runtime-target mapping, and look up `(doc_path, sha256)`
  in the local catalog. MVP implements the `.claude` mapping first.

Catalog lookup key is `(doc_path, sha256)`. When multiple catalog entries
match (same file content, same path, different versions): **prefer the entry
whose version matches any existing lockfile record for that package; if no
match, prefer the latest version (lexicographic descending).**

A file that matches the catalog but is already tracked in the lockfile is
**skipped silently** — not re-presented as a discovery candidate.

---

#### Deliverable B: Scan-Derived Install Records

Scan-discovered installs produce partial lockfile records. All fields not
derivable from the catalog must use explicit defaults:

```go
// Scan-derived install record structure
InstallRecord{
    PackageID:          from catalog match,
    Version:            from catalog match,
    Branch:             from catalog match (branch of catalog used),
    DoltCommit:         "",                  // unknown — not in catalog
    InstallScope:       derived from path (local vs global),
    InstalledAt:        time.Time{},         // zero — original time unknown
    TrackingOrigin:     "scan-reconciled",   // sentinel: identifies scan-derived records
    Files:              all matched doc_path files for package,
    Answers:            nil,
    QuestionSnapshot:   nil,
    Requirements:       nil,
    TemplateValidation: nil,
    CLIVersions:        nil,
    Dependencies:       nil,                 // not recoverable from scan
}
```

`TrackingOrigin` string field **already exists** at `src/pkg/installer/tracking.go:38`.
Sprint 3.7 establishes `"scan-reconciled"` as its canonical sentinel value for
scan-derived records. Do NOT add the field again; update the documentation comment
to list all valid values: `"local-install"` (existing), `"scan-reconciled"` (new).
Validate and upgrade must handle records where `DoltCommit` is empty — these
are valid but cannot be pinned to a specific Dolt history point.

---

#### Deliverable C: Non-Interactive Confirmation (MVP)

ST-004a requires presenting discovered installs before mutating state. MVP
implementation uses flags, not TTY prompts. Interactive TTY prompting is
deferred to a future sprint.

```
sc scan                 # lists candidates only; no mutation (default dry-run)
sc scan --accept-all    # writes lockfile entries for all discovered installs
sc scan --upgrade-all   # upgrades existing tracked installs to catalog version
sc scan --json          # machine-readable candidate list; no mutation
sc scan --json --accept-all  # machine-readable output and lockfile mutation
```

`--accept-all` and `--upgrade-all` are mutually exclusive. Neither is the default.
`--json` controls output format only; it must not alter mutation semantics.
JSON consumers that want mutation must pass the same explicit action flags as
human users, such as `--accept-all` or `--upgrade-all`.

Future interactive/session mode rule: starting the session with `--json` implies
JSON output for the entire session. Any future command surface that accepts JSON
arguments or a JSON request payload also implies JSON output for that invocation.

Behavior when stdin is not a TTY and no flag given: list candidates and exit
with code 0. This is safe for scripted environments. This default-no-mutation
mode satisfies the "skip" choice from ST-004a.

`sc scan` **never triggers a live Dolt connection**. If the catalog is absent,
emit error: `"catalog not found for branch {branch}; run: sc catalog update"` and
exit code 1. Do not attempt to fetch the catalog from Dolt during scan.

Register `NewScanCmd()` in `src/cmd/root.go` `NewRootCmd`.

---

#### Test Requirements

All tests use `t.TempDir()` for filesystem fixtures. Symlinks and special files
are excluded from walk (skip with `fs.ModeSymlink` check).

On Windows: skip file permission tests with `if runtime.GOOS == "windows" { t.Skip(...) }`.

Mandatory test cases:
- No `.claude/` directory → returns empty candidate list, no error
- `.claude/` contains only non-package files → zero candidates
- File permission error during walk → error logged, walk continues, error returned
- SHA matches multiple catalog entries → prefer installed version, then latest
- Interrupted scan (context cancel mid-walk) → no partial lockfile writes
- File already tracked in lockfile → not presented as candidate
- Stale catalog warning (fetched_at > 24h) → warning in output
- `--accept-all` on zero candidates → no-op, exit 0
- `--json` alone → no lockfile mutation because no action flag was supplied
- `--json --accept-all` writes the same lockfile records as `--accept-all` and
  returns the mutation summary as JSON

**Acceptance Criteria:**

- Discovers untracked installed packages by SHA matching against catalog
- Shows package ID, version, scope, and upgrade status for each candidate
- `--accept-all` writes valid `[[installs]]` records with `tracking_origin = "scan-reconciled"`
- Already-tracked packages are not re-presented
- Works fully offline using cached catalog
- No lockfile mutation without explicit `--accept-all` or `--upgrade-all`,
  regardless of output mode
- `TrackingOrigin` field documents `"scan-reconciled"` as a valid value
- `NewScanCmd()` registered in `src/cmd/root.go`
- `sc scan` with absent catalog → error naming the missing branch, exit 1
- `sc scan` with no flags → lists candidates only, no lockfile mutation

**Requirements:** ST-004, ST-004a

---

### Sprint 3.8: Import Collision Enforcement

**Goal:** Enforce the SHA immutability invariant (`CA-006`, `CA-007`) at import time.

**Background:** `sc admin import` is a write-path admin command. The collision
check must query **Dolt directly** (not the local catalog cache, which may be
stale). The importer currently uses `pkg/importer` which has a `dolt.Writer`
dependency. Sprint 3.8 adds a `dolt.Client` (read) dependency to the importer
so it can query existing catalog entries before writing.

---

#### Deliverable A: Read Client in Importer

`src/pkg/importer/importer.go` — add `Client dolt.Client` field alongside
existing `Writer`. Import now requires both read and write access.

`src/cmd/admin/import.go` (correct path — not `src/cmd/admin_import.go`) —
construct both client and writer, pass both to importer. If the read query
fails (network error, auth failure), **block the import** with error:
`"catalog check failed: %w; import aborted to protect SHA immutability"`.
Do not write partial data.

---

#### Deliverable B: Collision Check Logic

Before writing any `package_files` row, query the current package version and
file SHA for the same package/doc_path through the Sprint 3.5 `HTTPClient`.
This is a DoltHub HTTP GET read path: the branch is selected by the URL and the
SQL uses unqualified table names. Do not use MySQL-style
`` `database/branch`.table `` references or `?` placeholders in the HTTP query.

```sql
SELECT p.version, pf.dest_path AS doc_path, pf.sha256
FROM package_files AS pf
JOIN packages AS p ON p.id = pf.package_id
WHERE pf.package_id = '{{escaped_package_id}}'
  AND pf.dest_path = '{{escaped_doc_path}}'
```

HTTP request shape:

```text
GET https://www.dolthub.com/api/v1alpha1/{owner}/{database}/{branch}?q={urlencoded_sql}
```

**Ordering:** The collision check READ query must execute BEFORE any DELETE or
INSERT in the writer. `buildImportSQL` (in `dolt/writer.go`) issues DELETE then
INSERT. Run `importer.checkCollision(ctx, packageID, destPath)` in the importer
loop before calling `writer.ImportPackage`. This prevents the race where existing
rows are deleted before the check can read them.

Decision table:

| Existing row for `(package_id, version, doc_path)` | Incoming SHA | Action |
|----------------------------------------------------|-------------|--------|
| None | any | Allow (first import or new version) |
| Exists, same SHA as incoming | same | Allow (idempotent re-import, no-op) |
| Exists, different SHA from incoming | different | **Reject** — SHA collision |

A version bump may reuse the same `doc_path` with different content and SHA.
The collision check rejects mutation of an existing package/version/doc_path
tuple, not legitimate new package versions.

Error message format for rejection:
```
SHA collision: file "{{doc_path}}" for {{package_id}} {{version}} on branch "{{branch}}" already exists
  existing SHA: {{existing_sha}}
  incoming SHA: {{incoming_sha}}
Import aborted. No data was written.
```

JSON error format (`--json` mode):
```json
{
  "error": "sha_collision",
  "file": "{{doc_path}}",
  "package": "{{package_id}}",
  "version": "{{version}}",
  "branch": "{{branch}}",
  "existing_sha": "{{existing_sha}}",
  "incoming_sha": "{{incoming_sha}}"
}
```

---

#### Test Requirements

Mandatory test cases:
- First import of new package (no existing rows) → allowed, all files written
- Identical re-import (same SHA all files) → allowed, idempotent, no error
- Same package, same version, same doc_path, different SHA on same branch →
  rejected before any write; no partial writes
- Version bump on same branch (different package version, new files) → allowed
- Read query failure → import aborted, clear error, no writes
- Error message contains both SHAs and the colliding file path
- `--json` mode → JSON error shape matches spec above

**Acceptance Criteria:**

- Re-import of the same package/version/doc_path with different SHA rejected
  before any Dolt write occurs
- Error message names the colliding file and both SHAs exactly
- Identical re-import is a no-op and succeeds
- Read query failure blocks import with clear error
- No partial imports: either all files written or none

**Requirements:** CA-006, CA-007

---

### Sprint 3.9: Scope Flags, --yolo, Dep Acknowledgement, Validation Severity

**Goal:** Complete behavioral requirements from IU-* and VA-* sections deferred
from Sprints 3.2–3.4. These items appeared in earlier sprint acceptance criteria
but were not implemented — Sprint 3.9 is the authoritative implementation sprint.

**Note:** Sprint 3.3 acceptance criteria included severity and aggregate_status.
Sprint 3.4 acceptance criteria included `--scope` and `--yolo`. Both are
**deferred to this sprint**. QA for 3.3 and 3.4 should not fail on these items;
they are tracked as deferred here.

---

#### Deliverable A: --scope Flag

Replace all scope-specific booleans with `--scope <project|global|both>` on
every command that can target project-local state, machine-global state, or both:
`install`, `validate`, `status`, `scan`, `catalog update`, `snapshot`,
`upgrade`, and `uninstall`.

```go
// Valid values; use a string enum with explicit validation.
const (
    ScopeProject = "project"
    ScopeGlobal  = "global"
    ScopeBoth    = "both"     // default when --scope omitted
)
```

`config.Config` has no `Global` or `Local` field. Remove legacy scope aliases
from all command registrations, documentation, help text, and tests. Known
implementation cleanup points include:
- `src/cmd/install.go:21`
- `src/cmd/upgrade.go:28`
- `src/cmd/uninstall.go:26`

Also update `selectInstall` in `src/cmd/install_mutation_helpers.go:50` which
reads the `global` flag to derive install scope.

Default when `--scope` omitted: `both`.

Command audit requirement: Sprint 3.9 must audit all command registrations,
help output, docs, JSON examples, and tests to ensure `--scope` is the only
local/global selection interface. Legacy scope aliases must not appear as
accepted CLI flags after this sprint.

---

#### Deliverable B: --yolo Flag

`--yolo` on `install`, `upgrade`, `uninstall`:
- Skips all interactive prompts (dep acknowledgement, uninstall dep removal)
- Still records full provenance in tracking state
- `--yolo` + `--dry-run` together → `--dry-run` wins; no mutation, no prompts

---

#### Deliverable C: Dependency Acknowledgement

`sc install` with external CLI/tool dependencies and no `--yolo`:
- Print the install plan listing each external dep before installing
- Prompt: `"Proceed? [y/N]"` — default N (safe)
- Non-TTY stdin without `--yolo`: print plan and exit 1 with error:
  `"interactive confirmation required; use --yolo to proceed non-interactively"`

---

#### Deliverable D: Validation Severity

**Actual type names in source:** The validate structs are `validatedFile`
(`src/cmd/state_helpers.go:42`) and `validatedInstall`. Use these names when
modifying source. `ValidationSeverity` is a new type defined in
`src/cmd/state_helpers.go` alongside these structs.

Each `validatedFile` gains a `Severity ValidationSeverity` field:

```go
type ValidationSeverity string
const (
    SeverityInfo     ValidationSeverity = "info"
    SeverityWarn     ValidationSeverity = "warn"
    SeverityError    ValidationSeverity = "error"
    SeverityCritical ValidationSeverity = "critical"
)
```

Severity mapping (authoritative):

| Validation item/state | Severity |
|-----------------------|----------|
| ok | info |
| modified (local change) | warn |
| extra (untracked file in package dir) | info |
| missing (tracked file absent) | error |
| unreadable (file exists, can't read) | error |
| DEPENDENCY_MISSING | critical |
| DEPENDENCY_VERSION_INCOMPATIBLE | error |
| HOOK_NOT_REGISTERED | warn |
| TEMPLATE_INVALID | warn |
| AGGREGATE_MISMATCH | error |

Aggregate status emitted in JSON output:
```json
{
  "package": "team-lead",
  "aggregate_status": "error",
  "items": [
    {"path": "...", "status": "missing", "severity": "error"}
  ]
}
```

`aggregate_status` = highest severity across all items (`critical` > `error` >
`warn` > `info`). Human output suppresses `info`-only items by default;
`--verbose` shows all.

---

#### Deliverable E: Upgrade Warn-and-Skip

`upgrade --all`:
- For each candidate, compute upgrade plan independently
- If candidate has incompatible dependency → warn: `"skipping {pkg}: {reason}"`
- Continue remaining valid upgrades
- Exit code 0 if any upgrade succeeded; exit code 1 only if ALL upgrades failed

`upgrade <pkg> --force`:
- Overrides blocked-upgrade behavior for single targeted package only
- `upgrade --all --force` → rejected with error: `"--force cannot be used with
  --all; target a specific package"`

---

#### Deliverable F: Provenance-Aware Uninstall

`RequirementSnapshot` in `src/pkg/installer/tracking.go` has
`CLIProvenance map[string]string` (not a bool field). Sprint 3.9 adds:

```go
// IsInstalledBySC returns true if the named dependency was installed by
// Synaptic Canvas (not pre-existing). Reads CLIProvenance["depName"].
func (r *RequirementSnapshot) IsInstalledBySC(depName string) bool {
    return r.CLIProvenance[depName] == "installed-by-synaptic"
}
```

Do NOT change TOML on disk — `CLIProvenance` already stores provenance strings.

`uninstall` uses `IsInstalledBySC(depName)` for each dep:
- returns `true` → offer removal (prompt or `--yolo`)
- returns `false` → leave untouched, no mention unless `--verbose`

`--force` on uninstall: removes package files even if local modifications
detected. Without `--force`, local modifications produce a warning and a prompt:
`"Package has locally modified files. Proceed anyway? [y/N]"`. Non-TTY + no
`--yolo` + no `--force` → error: `"locally modified files detected; use
--force to proceed or --yolo in non-interactive mode"`.

---

#### Backward Compatibility

Sprint 3.9 code must handle lockfile records created by Sprint 3.2–3.4 code:
- Missing `tracking_origin` field → treat as `"local-install"` (matches installer.go:187)
- Missing `severity` on validate items → emit as `""` in JSON (not null); human
  output omits severity label
- Missing `dependency.installed_by_sc` → treat as `false` (do not offer removal)
- Missing `aggregate_status` → compute from items; if no items, emit `"info"`

No migration required. Zero-value fallback is sufficient.

---

#### Test Requirements

Mandatory test cases:
- `sc install <pkg>` with omitted `--scope` installs both project and global
  targets or fails atomically before writing either target
- `sc install <pkg> --scope project` writes only project-local artifacts
- `local-only` package with `--scope both` fails before any project/global
  files are written
- `--scope project` when only global install exists → empty results, no error
- `--scope both` with one local PASS, one global FAIL → mixed output, exit 1
- `--yolo` + `--dry-run` → no mutation, no prompts
- `--force` with `--all` → rejected before any upgrade attempt
- Dependency provenance missing from lockfile → treated as `false`, not offered
- `--yolo` uninstall only removes deps where `installed_by_sc = true`
- Non-TTY stdin + dep acknowledgement required + no `--yolo` → exit 1 with error
- All severity mappings produce correct severity value
- `aggregate_status` = highest severity across items
- Old lockfile record (missing `tracking_origin`) → read without panic

**Acceptance Criteria:**

- All scope-aware commands accept `--scope` enum; omitting defaults to `both`
- Legacy scope aliases removed from all CLI commands
- Scope audit confirms command registrations, help text, docs, JSON examples,
  and tests use `--scope` consistently
- `--yolo` skips interactive prompts while recording full provenance
- `--yolo` + `--dry-run` → dry-run wins
- Install presents external dep plan before installing; `--yolo` skips prompt
- Each validate output item has `severity` from fixed vocabulary
- `aggregate_status` field emitted in JSON validate output
- `upgrade --all` warns and skips blocked candidates; continues valid upgrades
- `upgrade --all --force` → rejected with clear error
- Uninstall warns on locally modified files; `--force` overrides
- Uninstall offers SC-installed deps for removal; ignores predating deps
- Old lockfile records (pre-3.9) handled without crash or data loss

**VA-001 through VA-004a disposition:**
- VA-001 (validation covers more than checksums): **partially addressed in Sprint 3.3**
  for file presence and checksum. Dep presence, hook registration, template validation
  are **open and addressed in Sprint 3.9** via the severity mapping and validate expansion.
- VA-002 (`modified` distinct from `missing`): addressed in Sprint 3.3 — `modified` status exists.
- VA-003 (explicit scope in output): addressed in Sprint 3.3 — scope label in output.
- VA-004/VA-004a (status output sufficient for "what is installed"): addressed in Sprint 3.3.
  Sprint 3.9 adds `aggregate_status` to JSON output.

`upgrade --all --force` error contract:
- Exit code: `1`
- `--json` mode output: `{"ok":false,"error":{"code":"invalid_args","message":"--force cannot be used with --all; target a specific package"}}`

**Requirements:** IU-001 through IU-011, VA-001 through VA-005a, CLI-006 through CLI-010

---

## Phase 4: AI Surface, Backend Parity, And Distribution

Phase 4 should not begin with a skill wrapper alone. The wrapper depends on a
stable machine contract, explicit backend selection, and readback-rich state
queries. The sequence for this phase is therefore:

1. Harden the public `sc --json` contract and typed error surface.
2. Support all three first-class Dolt backends behind one CLI contract:
   `HTTPClient` for DoltHub, `SQLClient` for hosted SQL-compatible servers, and
   `CLIReader` for local Dolt clones.
3. Make mutating commands auditable through corresponding read commands.
4. Build the `sc:plugin` skill as a thin wrapper over the hardened CLI.
5. Add installer/distribution scripts once the CLI and wrapper contracts are stable.

Quality review for this phase should anchor to `REQ-004`, `REQ-005`,
`REQ-006`, `CLI-002` through `CLI-013`, `DC-001` through `DC-012`,
`ST-009`, `CA-008`, `VA-010`, `MB-001` through `MB-006`, and
`ADR-0001` through `ADR-0005`. The sprint
documents under `docs/phase-4/` are the detailed review checklists.

### Sprint 4.1: JSON Contract And Typed Error Hardening

**Goal:** Make `sc --json` the deterministic machine contract for all end-user
commands.

**Deliverables:**
- Shared typed JSON response models for end-user command families
- Shared typed JSON error envelope with stable codes and structured details
- Root-level JSON failure handling for bootstrap/config/client-selection errors
- Contract tests that assert JSON success and failure shapes directly

**Key Requirements:**
- No end-user command may emit raw prose-only failures in `--json` mode
- `map[string]any` response payloads are replaced with typed response structs
- Error categories are stable enough for AI callers to branch on without parsing
  free-form text

**Acceptance Criteria:**
- All end-user commands return typed JSON success envelopes in `--json` mode
- Failures that occur before command business logic still return the standard
  JSON error envelope when `--json` is set
- Typed error categories cover invalid args, ambiguous targets, backend
  failures, local-modification blocks, confirmation-required states, and
  internal failures
- Command-family tests assert JSON schema shape across success and failure paths

### Sprint 4.2: Client Selection And Backend Contract Parity

**Goal:** Support `HTTPClient`, `SQLClient`, and `CLIReader` behind one stable
CLI contract.

**Deliverables:**
- Explicit backend selection via config/flag precedence
- Implemented `HTTPClient` for DoltHub reads
- Normalized backend error mapping into the shared CLI JSON error contract
- Shared client conformance tests across all three backends

**Key Requirements:**
- `http`, `sql`, and `cli` are all supported client modes
- Backend-specific failure facts are reported in structured `details`, but the
  top-level error contract remains consistent
- Routine CI does not depend on live DoltHub, a live SQL server, or a local
  Dolt clone

**Acceptance Criteria:**
- The CLI can select `http`, `sql`, or `cli` explicitly without changing the
  public JSON schema
- Equivalent backend failure classes map to the same top-level CLI error codes
- Adapter-level simulator or harness coverage exists for each client mode
- Live backend verification remains manual/AI-driven integration testing only

### Sprint 4.3: Installed-State Readback And Audit Symmetry

**Goal:** Make install, upgrade, uninstall, and snapshot results confirmable
through read commands rather than direct filesystem inspection alone.

**Deliverables:**
- Richer `status --json` and `validate --json` installed-state DTOs
- Readback coverage for scope, version, branch, install site, dependency
  provenance, hook state, and local modification inventory
- Snapshot metadata/readback aligned to install records and validation output
- Mutation tests that assert follow-up state through read commands

**Key Requirements:**
- Read commands must be rich enough to answer "what changed?" after a mutation
- Validation remains list-based and severity-driven for AI and human review
- Human-readable output may stay concise, but JSON is the authoritative machine
  contract

**Acceptance Criteria:**
- `status --json` and `validate --json` are rich enough to confirm mutation
  effects without re-deriving state from logs
- Readback includes dependency provenance and hook-registration summaries
- Mutation-family tests verify post-mutation state using shared DTOs and read
  commands

### Sprint 4.4: sc:plugin Thin Wrapper

**Goal:** Build the `sc:plugin` skill as a thin AI wrapper over the hardened
`sc --json` contract.

**Deliverables:**
- `sc:plugin` package/skill source in-repo
- Explicit natural-language to `sc --json` command mapping
- Fixture-backed wrapper verification for representative success and error cases
- Documentation clarifying that orchestration behavior stays outside the skill

**Key Requirements:**
- The skill delegates business logic to the CLI rather than re-implementing it
- Non-default branch, version, and scope choices stay explicit in generated CLI
  invocations
- The skill does not absorb ATM/task-loop orchestration responsibilities

**Acceptance Criteria:**
- Wrapper actions shell out to `sc` with `--json`
- Wrapper verification covers success, ambiguity, backend failure, and
  corrective-error presentation
- The wrapper does not create a second business-payload schema separate from
  the CLI contract

### Sprint 4.5: Installer And Local Distribution

**Goal:** Install or upgrade the `sc` binary and `sc:plugin` skill on supported
platforms with predictable config behavior.

**Deliverables:**
- `scripts/install.sh` for macOS/Linux
- `scripts/install.ps1` for Windows
- Installer verification owned by the repository
- Preserved user configuration on rerun/upgrade

**Key Requirements:**
- Installer reruns are predictable and sufficiently idempotent for support and
  development workflows
- The installer updates managed assets without clobbering user-owned config
- Release publication logic remains in Phase 5 rather than in installer scripts

**Acceptance Criteria:**
- Fresh install makes `sc --version` work and installs `sc:plugin` globally
- Upgrade reruns preserve config values while updating managed binaries and
  skill assets
- Repository-owned verification exists for installer behavior on supported
  platforms

---

### Sprint 4.9: Uninstall Atomicity And Installer PATH Hardening

Detailed sprint plan: [4.9 Uninstall Atomicity And Installer PATH Hardening](./phase-4/4.9-uninstall-atomicity-and-installer-path-hardening.md)

**Goal:** Fix uninstall manifest-removal ordering so partial file-deletion failures leave tracked state intact. Fix installer to actually add the bin directory to PATH on fresh install (not merely warn).

**Deliverables:**
- `src/cmd/uninstall.go` — reordered to file-delete → hook-remove → manifest-remove; structured error on partial failure
- `src/cmd/uninstall_test.go` — corrected atomicity assertions
- `scripts/install.sh` — adds bin dir to rc file and session PATH on fresh install
- `scripts/install.ps1` — adds bin dir to Windows user PATH via SetEnvironmentVariable
- `tests/installer/test_install.sh` and `test_install.ps1` — fresh-PATH scenario without pre-seeded PATH
- `docs/phase-4/4.8-installer-hardening-and-doc-gaps.md` added to branch

**Key Requirements:** ST-009, REQ-004, FS-001, ADR-0005

**Acceptance Criteria:**
- Uninstall removes manifest record only after all managed files successfully deleted
- File-removal failure preserves manifest and returns structured error listing unremoved files
- Fresh `install.sh` run modifies rc file and adds bin dir to PATH without pre-seeded PATH in tests
- Fresh `install.ps1` run adds bin dir to Windows user PATH
- All installer and Go tests pass

---

### Sprint 4.10: Operations Layer: Install, Upgrade, Uninstall Workflow Extraction

Detailed sprint plan: [4.10 Operations Layer Workflow Extraction](./phase-4/4.10-operations-layer-workflow-extraction.md)

**Goal:** Move install, upgrade, and uninstall workflow policy out of Cobra handlers into `src/pkg/operations`, satisfying MB-001 and MB-006 module boundary requirements.

**Deliverables:**
- `src/pkg/operations/install.go` — install scope-loop and multi-scope aggregation
- `src/pkg/operations/upgrade.go` — upgrade workflow
- `src/pkg/operations/uninstall.go` — uninstall workflow
- `src/cmd/install.go`, `src/cmd/upgrade.go`, `src/cmd/uninstall.go` — thin bindings only (flag parse + operation call + output format)

**Key Requirements:** MB-001, MB-006, REQ-005, ADR-0004

**Acceptance Criteria:**
- No package-management business logic in `src/cmd/` handlers
- No `cobra` imports in `src/pkg/operations/`
- `go test ./src/pkg/operations/...` passes with direct workflow coverage
- All existing tests pass; no regressions

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
| 3. End-User | 3.1–3.9 | List, install, validate, upgrade, HTTP client, SHA catalog, scan, import collision, scope/yolo/severity |
| 4. AI Surface | 4.1–4.10 | JSON contract, backend parity, readback, sc:plugin, installer, error/fixture hardening, uninstall atomicity, PATH, operations layer |
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
