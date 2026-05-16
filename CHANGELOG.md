# Changelog

All notable changes to Synaptic Canvas are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.4.0] - 2026-05-15

### Added

- **Install system** (`sc install`, `sc init`) — materialize packages from DoltHub into
  `.claude/` and `~/.claude/`; scope-aware (`--scope local|global|both`); cross-scope
  rollback on partial failure; `--yolo` for non-interactive flows
- **Uninstall** (`sc uninstall`) — removes owned files and hook registry entries;
  offers removal of SC-installed dependencies; respects pre-existing deps
- **Upgrade** (`sc upgrade`) — upgrades installed packages to latest compatible version
  on their tracked branch; per-install branch tracking (IU-009); `--yolo` support
- **Validation** (`sc validate`, `sc status`) — severity-driven findings
  (info/warn/error/critical); checks file presence, SHA integrity, dependency presence
  and version compatibility, hook registration, and template validity
- **SHA catalog** (`sc catalog update`) — local TOML cache of immutable
  `(package_id, version, doc_path, branch)` → SHA mappings; offline validation support;
  per-branch files at `.synaptic/catalog-{branch}.toml` and `~/.synaptic/catalog-{branch}.toml`
- **Scan / reconciliation** (`sc scan`) — discovers installed packages from on-disk SHAs
  without a Dolt connection; presents candidate actions (add/upgrade/skip)
- **HTTPClient** — DoltHub REST API client with 429 retry (up to 3×, Retry-After backoff),
  context cancellation, and branch-in-URL stateless design
- **Admin import SHA collision enforcement** — hard rejection on duplicate
  `(package_id, version, doc_path, branch)` tuples; standard JSON error envelope
- **JSON error envelope** — all command errors emit `{ok:false,error:{code,message}}`
  on `--json`; non-zero exit on failure
- **Hook registry** — upsert keyed by `(package_id, scope, hook_type)`; init merges
  not overwrites; upgrade and uninstall only touch owning package entries
- **Concurrent state safety** — in-process `sync.Map` mutex per canonical manifest path
  combined with OS-level advisory flock for cross-process protection

### Changed

- `HTTPClient` is now the sole MVP Dolt client; `SQLClient` and `CLIReader` retained
  as documented alternatives
- Branch always passed as URL path segment — no session state between calls
- `--scope` enum replaces `--global`/`--local` flags across all end-user commands

### Fixed

- Uninstall write ordering: manifest lock saved before file removal
- Upgrade uses each install record's tracked branch when `--branch` not explicitly set
- Admin import uses `cmd.Context()` throughout (no `context.Background()`)
- Multi-scope batch failure emits standard error envelope
- SHA collision response normalized to standard envelope with `error.details` fields

## [0.3.0] - 2026-05-06

### Added

- Phase 2 admin commands: `sc admin import`, `sc admin export`
- DoltHub HTTP API client (initial implementation)
- `sc list` and `sc info` catalog queries
- GoReleaser cross-platform build pipeline (linux, darwin, windows; amd64, arm64)
- golangci-lint with gosec enabled in CI

## [0.2.2] - 2026-04-01

### Fixed

- Minor CLI flag parsing corrections

## [0.2.0] - 2026-03-15

### Added

- Initial `sc` CLI skeleton
- Dolt schema and SQL bootstrap scripts
- Phase 1 infrastructure: config, logging, output formatting

[Unreleased]: https://github.com/randlee/synaptic-canvas-dolt/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/randlee/synaptic-canvas-dolt/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/randlee/synaptic-canvas-dolt/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/randlee/synaptic-canvas-dolt/compare/v0.2.0...v0.2.2
[0.2.0]: https://github.com/randlee/synaptic-canvas-dolt/releases/tag/v0.2.0
