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
- BR-009 Branch and version shall be tracked and reasoned about independently.
  Branch identifies the release track/channel; version identifies a release on
  that branch.

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
- CLI-006 Human-readable output may suppress the branch label for `main` when
  that improves readability, but structured output shall emit `"main"`
  explicitly.

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

## 7. Install Tracking And State Safety

- ST-001 Synaptic Canvas shall track all local and global package installs on
  the machine in product-managed state under `.synaptic/` or `~/.synaptic/`.
- ST-002 Local and global installs of the same package shall be treated as a
  normal supported state. Status, validate, upgrade, and uninstall flows shall
  be able to target project installs, global installs, or both.
- ST-003 Install tracking shall record, at minimum, package identity, version,
  branch, Dolt source reference, install scope, materialized file inventory,
  and dependency provenance.
- ST-004 Synaptic Canvas shall support reconciliation of tracked state by
  scanning repositories for installed package artifacts and importing or
  updating local tracking records for installs created on another machine.
- ST-004a Scan/reconciliation flows shall present discovered installs and their
  upgrade status before mutating tracking state, and shall support the user
  choices `add all`, `upgrade all`, or `skip`.
- ST-005 Product coordination shall apply only to product-managed state
  mutation under `.synaptic/` or `~/.synaptic/`. Installed runtime artifacts
  under `.claude/` or `~/.claude/` shall not be left under persistent product
  locks.
- ST-006 The locking design shall make stale lock artifacts impossible by
  design. Synaptic Canvas shall not rely on orphanable lock files that can
  permanently block future operations after a crash or interruption.
- ST-007 Where mutual exclusion is required, the implementation shall use
  atomic replacement and transactional staging as the primary safety mechanism.
  Self-cleaning OS-backed locking may be used as an additional guard, but the
  design shall not depend on process-exit cleanup as the only stale-lock
  prevention mechanism.

## 8. Install, Upgrade, And Uninstall Behavior

- IU-001 Installing external tools or CLIs as dependencies shall require an
  explicit plan acknowledgement unless the user opts into non-interactive
  approval with `--yolo`.
- IU-002 `--yolo` shall mean "execute the computed plan without interactive
  confirmation," including dependency installation, while still recording what
  changed in tracking state.
- IU-003 Install tracking shall record whether each dependency was already
  present before install or was installed by Synaptic Canvas as part of the
  operation.
- IU-004 Uninstall shall not remove dependencies that predated the Synaptic
  Canvas install.
- IU-005 Uninstall shall ask whether to remove dependencies that were installed
  by Synaptic Canvas for the package being removed, unless a non-interactive
  mode says otherwise.
- IU-006 Uninstall and upgrade shall preserve dependencies, hooks, and shared
  artifacts still required by other tracked installs.
- IU-007 For scope-aware commands, omitting `--scope` shall default to `both`.
- IU-008 `--yolo` may be used with install, upgrade, and uninstall. In
  uninstall flows it shall only allow removal of dependencies that were
  recorded as installed by Synaptic Canvas.
- IU-009 Upgrade planning shall treat "latest" as latest available on the
  tracked branch unless the user explicitly changes branch or version.
- IU-010 If an upgrade candidate would violate dependency or compatibility
  requirements, Synaptic Canvas shall warn and skip that upgrade by default
  while continuing other valid upgrades in the same batch.
- IU-011 `--force` may override blocked-upgrade behavior only for explicitly
  targeted package upgrades; it shall not be supported as a blanket override for
  `upgrade --all`.

## 9. Validation And Inventory

- VA-001 Validation shall cover more than file checksums. At minimum it shall
  verify tracked file presence, aggregate package integrity, dependency
  presence, dependency version compatibility, and other tracked installation
  components needed for the package to function.
- VA-002 Validation shall classify local file drift distinctly from missing or
  corrupt files so local improvements and modifications can be inventoried and
  analyzed.
- VA-003 Validation output shall make the installation scope being validated
  explicit and shall support project-local, global, or combined inventory
  views.
- VA-004 Status and validation output shall be sufficient to answer "what is
  installed on this machine," including packages tracked outside the current
  repository.
- VA-004a Human-readable status output shall present one row per package with
  separate global and local version/branch views.
- VA-005 Validation output shall be list-based and each reported item shall
  carry an explicit severity level suitable for human review and automation.
- VA-005a Validation severities shall use the fixed vocabulary `info`, `warn`,
  `error`, and `critical`.
- VA-006 Validation shall support exporting local modifications to a staged
  snapshot area under product-managed global state so locally improved variants
  can be compared across versions and prepared for hardening or re-import.
- VA-007 Snapshot export shall be a separate command surface rather than an
  implicit side effect of validation. `sc snapshot <package>` shall default to
  exporting modified tracked files only and `--full` shall export the full
  installed package state.
- VA-008 Snapshot exports shall be stored under
  `~/.synaptic/mod-snapshots/<package>/<branch>/<repo>/`, where `<repo>` is a
  sanitized repository key such as `<base-folder-name>-<repo-id>`, and shall
  include a readable `snapshot.toml` file containing, at minimum, the full
  source path, repo URL, snapshot timestamp, version, branch, and scope.

## 10. Verification Traceability

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
- VER-009 Flaky tests are not acceptable. Tests that can pass or fail without a
  product change shall be treated as blocking defects and must be fixed or
  removed before the sprint is accepted.
