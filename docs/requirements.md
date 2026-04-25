# Synaptic Canvas Requirements

**Status:** Draft for implementation and QA use

This document defines the cross-cutting requirements that all design and sprint
documents in this repository must follow. Detailed subsystem documents may add
requirements, but they must not conflict with this document.

## 1. Scope And Precedence

- REQ-001 This document is the normative source for cross-cutting product
  requirements.
- REQ-002 Detailed design documents such as
  `synaptic-canvas-cli.md`, `synaptic-canvas-schema.md`,
  `synaptic-canvas-export-pipeline.md`, `synaptic-canvas-install-system.md`,
  and `synaptic-canvas-hook-system.md` shall remain consistent with this
  document.
- REQ-003 Sprint plans shall define acceptance criteria that can be traced to
  these requirements and to the relevant detailed design documents.

## 2. Structured Logging

Synaptic Canvas is a simple CLI application. Logging requirements shall remain
lightweight and operationally useful.

- LOG-001 The CLI shall emit structured log records to a file by default.
- LOG-002 Structured logging, logging failure handling, and log
  rotation/retention shall follow the same model used by the
  `sc-observability` logging-only design:
  - structured file logging
  - optional console logging
  - bounded rotation
  - bounded retention
  - sink failures after initialization are fail-open
- LOG-003 The default log file location shall be `~/.sc/logs/sc.log`.
- LOG-004 Log rotation and retention shall be documented and testable.
- LOG-005 The logging format shall be machine-readable JSON, one record per
  line.
- LOG-006 Each log record shall contain, at minimum, `time`, `level`, and
  `msg`.
- LOG-007 CLI-owned operational logs should include `component` and
  `operation` so they can be filtered by support tools and debugging agents.
- LOG-008 Logging requirements shall remain intentionally minimal; this
  repository does not require broader observability features such as OTLP,
  routing runtimes, or health endpoints.

## 3. Dolt Branch Access

Dolt branches are the release-channel mechanism, but CLI read behavior shall be
explicit and deterministic.

- BR-001 All read-path Dolt access shall resolve an explicit effective branch.
- BR-002 The effective branch shall be resolved using this precedence:
  1. `--branch`
  2. `SC_DOLT_BRANCH`
  3. `main`
- BR-003 If no branch is specified by flag or environment, the CLI shall always
  read from `main`.
- BR-004 The CLI shall ignore the current Dolt session branch or checked-out
  branch when determining read behavior.
- BR-005 Read operations shall not rely on session mutation such as `USE
  database/branch` for correctness.
- BR-006 Read operations shall use explicit branch-qualified reads so multiple
  readers can query different branches safely in parallel.
- BR-007 Admin or write-path operations may still target explicit branches, but
  those behaviors shall be documented independently in the relevant command
  design.
- BR-008 In MVP, user-selectable branch values map directly to Dolt branch
  names. There is no separate channel-to-branch translation layer.

## 4. CLI Access Strategy

The CLI is expected to be used by both humans and AI wrappers.

- CLI-001 Human-facing command output may default to concise human-readable
  rendering.
- CLI-002 Machine-oriented callers shall use `--json` for stable structured
  output.
- CLI-003 AI wrappers and Claude-facing skills should pass `--json`
  explicitly rather than relying on ambient environment defaults.
- CLI-004 If an output-mode environment variable is introduced later, it shall
  not replace `--json` as the preferred explicit machine-access contract.
- CLI-005 JSON-mode errors shall be documented and stable enough for AI wrappers
  and automation to consume predictably.

## 5. Agents And Scripts

- AG-001 Agent definitions in this repository shall comply with the shared
  Claude Code skills/agents guidelines located in the sibling
  `synaptic-canvas` repository at
  `../synaptic-canvas/docs/claude-code-skills-agents-guidelines.md`.
- AG-002 Each agent shall define a clear input contract and output contract.
- AG-003 Every repository-owned helper script shall have unit tests.
- AG-004 Script tests shall avoid nondeterministic dependence on wall-clock
  time, real user state, or production logs.
- AG-005 Agent and script behavior required by a sprint shall be documented in
  the sprint plan and verified by tests or explicit QA steps.

## 6. Install Targets And Product State

- FS-001 For MVP, package artifacts for Claude Code shall be materialized into
  `.claude/` for local installs or `~/.claude/` for global installs.
- FS-002 Synaptic Canvas product-managed state shall live under `.synaptic/`
  locally or `~/.synaptic/` for machine-level state as applicable.
- FS-003 `.synaptic/` is reserved for lockfiles, generated configuration,
  metadata, caches, logs, temporary files, and other Synaptic-managed state; it
  is not the primary runtime artifact root for Claude-targeted packages.
- FS-004 Future runtime targets such as `.codex/` and `.agents/` may be added
  later, but they are not part of the MVP package-install target surface.
- FS-005 Package install-scope enforcement shall be explicit. If a package is
  marked `local-only`, `sc install --global` shall fail with a clear error and
  shall not perform a partial install.
- FS-006 The CLI shall support exporting installed package state via
  `sc snapshot <package>`.
- FS-007 `sc snapshot` shall default to exporting only modified tracked files
  for the targeted install.
- FS-008 `sc snapshot --full` shall export the full tracked install payload for
  the targeted package.
- FS-009 Snapshot exports shall be written under machine-level product state
  rather than into the active repository tree.
- FS-010 Snapshot metadata shall be stored in a human-readable
  `snapshot.toml` file that records, at minimum, package name, branch, version,
  snapshot timestamp, source install path, and repository path when applicable.

## 7. Verification Traceability

- VER-001 Requirements shall be testable or otherwise verifiable.
- VER-002 Sprint acceptance criteria shall map to one or more concrete
  verification steps.
- VER-003 New command, agent, and script behavior shall ship with tests unless a
  documented reason prevents automation.
- VER-004 Where tests are not sufficient, the sprint plan shall identify the QA
  procedure required to validate the behavior.
- VER-005 Verification is part of the product, not just part of development
  process. MVP scope includes automated verification for the CLI, Go packages,
  and repository-owned helper scripts.
- VER-006 Branch promotion supports staged rollout, but it shall not be treated
  as a substitute for automated verification.
- VER-007 The product should evolve toward a dedicated validation system for
  packages, including a test harness and per-package eval coverage for scripts
  and agents.
- VER-008 Test evidence may later be stored in package metadata or adjacent
  validation records, but evidence storage is not required for MVP.
