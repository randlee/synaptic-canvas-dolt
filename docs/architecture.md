# Synaptic Canvas Architecture

This document replaces the old architectural decisions log as the top-level
architecture overview for the repository.

## 1. System Shape

Synaptic Canvas is a Dolt-backed package system for Claude Code skills, agents,
commands, hooks, and related package artifacts.

The system has three main layers:

1. **AI wrapper / human caller**
   - Claude-facing skills and other automation invoke the CLI
   - machine callers should prefer `--json`
2. **`sc` CLI**
   - the product entry point
   - owns package commands, admin commands, integrity verification, file I/O,
     and structured logging
3. **Dolt database**
   - source of truth for packages and metadata
   - branches represent release channels

The CLI executable name is `sc`. `Synaptic Canvas` remains the product name,
but `synaptic` is not a separate supported command surface.
## 2. Dolt As The Source Of Truth

Dolt remains the package system database because it provides:

- relational querying for packages, files, dependencies, hooks, and variants
- branch-based release channels
- auditable promotion history
- a single authoring source for CLI reads, export pipelines, and future offline
  snapshots

The CLI is a client of Dolt. End users are not expected to reason about Dolt
session state during normal CLI use.

## 3. Branch And Channel Model

`develop`, `beta`, and `main` remain the release branches. Read behavior must
still be explicit.

Architecture rules:

- the CLI resolves an effective branch using `--branch`, then
  `SC_DOLT_BRANCH`, then `main`
- the CLI ignores the current/checked-out Dolt branch for read-path behavior
- read operations should be branch-qualified rather than relying on session
  switching
- in MVP, externally selectable branch values are the Dolt branch names
  directly; there is no separate channel-mapping layer

This keeps CLI behavior deterministic and allows multiple readers to query
different branches safely in parallel.

## 4. Promotion And Verification

Promotion across `develop`, `beta`, and `main` remains part of the product
model:

- `develop` supports active development
- `beta` supports broader pre-release testing
- `main` is the stable default read branch

However, promotion is a staged rollout mechanism, not a replacement for
testing.

The intended long-term model is:

- automated unit and integration tests for the CLI and helper scripts
- deterministic validation for repository-owned scripts and agents
- a dedicated product test harness for plugin validation
- per-plugin eval coverage for scripts and agents where automated correctness
  cannot be captured fully by conventional tests

Testing is part of the product story:

- the MVP must ship with automated verification for its CLI and helper-script
  surface
- staged rollout across `develop`, `beta`, and `main` complements testing but
  does not replace it
- future package validation may include test evidence captured alongside package
  metadata or adjacent validation records, but that evidence model is a later
  design step rather than an MVP prerequisite

## 5. AI Access Strategy

The CLI is expected to be used heavily by AI wrappers.

Architecture guidance:

- human output may remain concise and readable by default
- AI callers should invoke the CLI with `--json`
- any future environment-based output default is secondary to explicit
  `--json` invocation

This keeps machine access explicit and avoids ambiguity caused by environment
state.

## 6. Install Targets And State Layout

Synaptic Canvas separates package artifact targets from product-managed state.

Architecture rules:

- `.claude/` is the MVP runtime artifact root for installed Claude Code
  packages
- `.synaptic/` stores lockfiles, generated config, metadata, cache, logs, temp
  directories, hook registry state, and similar product-owned files
- machine-level state uses `~/.synaptic/` with the same schema as repo-local
  state where applicable
- local and global installs of the same package are a normal supported state,
  not an edge case; Synaptic Canvas tracks both independently and commands may
  target project installs, global installs, or both
- installed package artifacts under `.claude/` or `~/.claude/` must not be held
  under long-lived product locks; coordination is limited to `.synaptic/` or
  `~/.synaptic/` state mutation and the design must avoid stale lock artifacts
  entirely
- product-managed tracking must record where a package is installed, what
  version and branch it came from, what files were materialized, and which
  external dependencies were already present versus installed by Synaptic
  Canvas
- the product should support inventory reconciliation by scanning repositories
  for tracked package artifacts and reconciling them into local machine state
- future target roots such as `.codex/` and `.agents/` are compatible with this
  model but remain post-MVP

This keeps runtime-facing files aligned with the host tool while allowing
Synaptic Canvas to own its own operational state cleanly.

## 7. End-User State And Validation

Phase 3 end-user commands operate against local tracked installation state plus
explicit Dolt reads.

Architecture rules:

- `status`, `validate`, `upgrade`, and `uninstall` must be able to reason about
  project-local installs, global installs, or both in one invocation
- scope-aware end-user commands use `--scope` with an enum and default to
  `both` when omitted
- validation is broader than file checksum verification; it includes installed
  file presence, aggregate package integrity, dependency presence, dependency
  version compatibility, hook registration state, and template-validation state
- validation should inventory local modifications rather than reducing all drift
  to corruption; locally modified files are part of the managed state picture
- validation output is list-based and severity-driven rather than a single
  monolithic pass/fail result
- local modification snapshots can be exported into product-managed global
  staging, organized by package, so changes can be compared across installed
  versions and prepared for re-import
- uninstall behavior depends on dependency provenance: dependencies installed by
  Synaptic Canvas may be offered for removal, while dependencies that predated
  the install are left untouched
- non-interactive install/upgrade flows must remain explicit. `--yolo` is the
  acknowledged "proceed without prompting" mode for install, upgrade, and
  uninstall, and still records what external dependencies were installed

## 8. Agent And Script Architecture

Repository-local agent definitions and helper scripts are part of the product
surface for Claude-facing workflows.

Architecture rules:

- agents must follow the shared authoring guidelines in the sibling
  `synaptic-canvas` repository
- agents must define input/output contracts
- helper scripts must be unit-tested
- sprint plans must define how agent/script behavior is verified

## 9. Detailed Design Documents

This architecture overview is intentionally high level. The detailed subsystem
documents remain:

- `requirements.md`
- `synaptic-canvas-cli.md`
- `synaptic-canvas-schema.md`
- `synaptic-canvas-export-pipeline.md`
- `synaptic-canvas-install-system.md`
- `synaptic-canvas-hook-system.md`
- sprint plans under `docs/`
