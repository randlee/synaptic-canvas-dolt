# Synaptic Canvas — Project Plan

## Overview

Phased plan for building the `sc` Go CLI. Each phase contains sprints. Each sprint runs through the dev-QA loop defined in `CLAUDE.md`.

**Reference project:** `claude-history` (Go + Cobra + GoReleaser conventions)
**Design docs:** See [CLAUDE.md](../CLAUDE.md) for full list

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
- No `fmt.Println` for operational output — use `internal/output/` formatters
- No `log.Fatal` — return errors up the call stack

### Error Handling
- All errors wrapped with context: `fmt.Errorf("operation: %w", err)`
- Cobra command errors surfaced to user via structured output
- `--json` output includes error details for skill integration

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
- `src/go.mod` (module: `github.com/randlee/synaptic-canvas`, Go 1.26)
- `src/main.go` with version injection via ldflags
- `src/cmd/root.go` — root Cobra command with global flags (`--dolt-dir`, `--remote`, `--json`, `--quiet`, `--verbose`)
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
- Integration test harness (optional, uses test Dolt DB in `test/fixtures/`)

**Acceptance Criteria:**
- Dolt client interface defined and mockable
- Can query packages, files, dependencies from Dolt
- Models map cleanly to schema spec tables
- All query builders tested
- Manifest reconstruction tested against known fixture data

### Sprint 1.3: Integrity Package

**Goal:** SHA256 computation and verification library.

**Deliverables:**
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
- Unit tests for import logic (mock Dolt writer)
- Integration test: import test fixture → verify Dolt state

**Acceptance Criteria:**
- Imports a package directory into Dolt on specified branch
- Computes and stores per-file SHA256
- Computes and stores aggregate package SHA256
- Creates a Dolt commit with descriptive message
- Handles manifest.yaml decomposition matching schema spec
- Refuses to import if branch doesn't exist
- `--json` output includes import summary
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

**Acceptance Criteria:**
- Verify detects OK and CORRUPT states for stored content
- Verify recomputes aggregate and compares against stored package SHA
- Diff shows file-level changes between branches
- Both commands support `--json` output

### Sprint 2.4: Admin Publish

**Goal:** `sc admin publish <package> --from <branch> --to <branch>`

**Deliverables:**
- `src/cmd/admin/publish.go` — publish command
- Dolt merge operations for targeted package promotion
- Pre-publish validation (verify SHAs before promoting)

**Acceptance Criteria:**
- Promotes package from one branch to another via Dolt merge
- Runs verify before publishing (fail if corrupt)
- `--json` output includes publish summary
- Cannot publish to same branch

---

## Phase 3: End-User Commands

The read path. These commands never write to Dolt.

### Sprint 3.1: List & Info

**Goal:** `sc list` and `sc info <package>`

**Deliverables:**
- `src/cmd/list.go` — list command with `--channel` and `--tags` filters
- `src/cmd/info.go` — info command showing package details
- Table and JSON output for both

**Acceptance Criteria:**
- Lists packages from specified channel (branch), defaults to main
- Filters by tags
- Info shows: name, version, description, dependencies, file count, SHA
- Both support `--json` output

### Sprint 3.2: Install

**Goal:** `sc install <package> [--global] [--channel <channel>]`

**Deliverables:**
- `src/cmd/install.go` — install command
- `src/pkg/installer/installer.go` — file installation logic
- `src/pkg/installer/tracking.go` — installed package tracking (local state)
- Install logic: query Dolt → verify SHAs → write files → record install

**Acceptance Criteria:**
- Installs package files to `.claude/` (local) or `~/.claude/` (global)
- Respects `install_scope` from packages table
- Verifies per-file SHA after writing each file
- Verifies aggregate SHA after install
- Fails and rolls back on any SHA mismatch
- Records installed package/version/channel for status tracking
- Handles dependencies (warn if missing, don't auto-install for MVP)
- `--json` output includes install summary

### Sprint 3.3: Validate & Status

**Goal:** `sc validate [<package>] [--all]` and `sc status`

**Deliverables:**
- `src/cmd/validate.go` — validate command
- `src/cmd/status.go` — status command
- Validate logic: recompute SHAs from installed files → compare against Dolt

**Acceptance Criteria:**
- Validate reports per-file: OK, MODIFIED, MISSING
- Validate reports extra files: EXTRA (untracked)
- Validate computes and checks aggregate SHA
- Status shows installed packages, versions, channels, validation state
- Both support `--json` output

### Sprint 3.4: Upgrade & Uninstall

**Goal:** `sc upgrade <package> [--all]` and `sc uninstall <package>`

**Deliverables:**
- `src/cmd/upgrade.go` — upgrade command
- `src/cmd/uninstall.go` — uninstall command
- Upgrade logic: check for newer version → install → verify
- Uninstall logic: remove files → update tracking

**Acceptance Criteria:**
- Upgrade checks current vs available version on channel
- Upgrade warns about local modifications before overwriting
- Uninstall removes package files and tracking record
- Both support `--json` output

---

## Phase 4: Skill & Installer

### Sprint 4.1: sc:plugin Skill

**Goal:** Claude Code skill that wraps `sc` CLI for conversational package management.

**Deliverables:**
- Skill markdown file: maps natural language → `sc` CLI commands with `--json`
- Parses JSON output for conversational presentation
- Handles error cases gracefully

**Acceptance Criteria:**
- "list packages" → `sc list --json` → conversational response
- "install delay" → `sc install sc-delay-tasks --json` → conversational response
- Skill is a thin wrapper — no business logic in the skill
- Error messages from CLI presented clearly

### Sprint 4.2: Installer Script

**Goal:** Install script that sets up both `sc` binary and `sc:plugin` skill.

**Deliverables:**
- `scripts/install.sh` — macOS/Linux installer
- `scripts/install.ps1` — Windows installer (or winget manifest)
- Installs `sc` binary to PATH
- Installs `sc:plugin` skill globally to `~/.claude/`

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
| 1. Foundation | 1.1–1.4 | Scaffold + CI pipeline, Dolt client, integrity, log-debug agent |
| 2. Admin | 2.1–2.4 | Import, export, verify, publish |
| 3. End-User | 3.1–3.4 | List, install, validate, upgrade |
| 4. Skill | 4.1–4.2 | sc:plugin skill, installer |
| 5. Release | 5.1 | GoReleaser release pipeline |

---

## Document History

| Date | Change |
|------|--------|
| 2026-02-22 | Initial project plan |
| 2026-02-22 | Move CI pipeline from Phase 5 into Sprint 1.1; Phase 5 now release-only |
