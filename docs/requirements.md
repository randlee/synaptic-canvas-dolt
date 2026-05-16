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
  `dolt-api.md`,
  `synaptic-canvas-export-pipeline.md`, `synaptic-canvas-install-system.md`,
  `synaptic-canvas-hook-system.md`, and accepted ADRs under `docs/adr/` shall
  remain consistent with this document.
- REQ-003 Sprint plans shall define acceptance criteria that can be traced to
  these requirements and to the relevant detailed design documents.
- REQ-004 Any change that alters the public CLI contract, client-selection
  model, install-state model, or wrapper boundary shall update the normative
  requirements, architecture, and relevant ADRs in the same change.
- REQ-005 Sprint plans and quality reviews shall cite the relevant requirement
  IDs and ADRs for the behavior they validate so traceability does not depend
  on reviewer memory.
- REQ-006 MVP boundaries shall be explicit. Features such as additional runtime
  targets, alternate machine-contract surfaces, or higher-level team
  orchestration are out of scope until they are promoted into the normative
  set by requirements, architecture, and any needed ADRs.

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

## 3. Dolt Client Contract

- DC-001 Synaptic Canvas shall support three first-class Dolt read clients:
  `HTTPClient` for DoltHub HTTP SQL API access, `SQLClient` for hosted or
  self-hosted MySQL-compatible Dolt servers, and `CLIReader` for local Dolt
  clones via `dolt sql -q`.
- DC-002 The `Client` interface in `src/pkg/dolt` shall be the only Dolt
  access surface for CLI commands. No command may bypass that interface and
  talk directly to HTTP, SQL, or subprocess transport code.
- DC-003 `HTTPClient` shall pass the branch as a URL path segment and shall
  issue SQL that uses unqualified table names for the repository/ref named in
  the URL. DoltHub HTTP reads shall use `GET
  https://www.dolthub.com/api/v1alpha1/{owner}/{database}/{branch}?q={sql}` and
  shall not rely on POST requests. No session state exists between HTTP calls.
  This satisfies BR-004 and BR-005.
- DC-004 For private DoltHub repos, the HTTP client shall send an
  `Authorization: token <TOKEN>` header. Tokens are stored in sc config, never
  in source control.
- DC-005 When Dolt is unreachable, commands that require a live Dolt query
  shall emit a clear error. Commands that can fall back to local cache (e.g.
  `sc validate` using the SHA catalog) shall do so and emit a warning.
- DC-006 The active client implementation shall be selectable explicitly for
  end-user and admin reads using deterministic precedence. The selection model
  shall be documented and testable. If an explicit CLI flag is added, its
  precedence shall be above environment and file config.
- DC-007 `dolt.client` shall accept the stable values `http`, `sql`, and
  `cli`. Compatibility inference such as treating `--dolt-dir` as an implicit
  CLI-reader selector may exist, but conflicting combinations shall fail with a
  typed CLI error rather than silently picking a transport.
- DC-008 DoltHub HTTP SQL responses shall be decoded from the actual response
  envelope: `query_execution_status`, `query_execution_message`, `schema`, and
  `rows`. Row values shall be decoded defensively from JSON strings, because
  DoltHub commonly returns SQL values as strings.
- DC-009 The public CLI JSON contract shall remain transport-invariant across
  `HTTPClient`, `SQLClient`, and `CLIReader`. Equivalent success cases shall use
  the same top-level success schema, and equivalent failure classes shall use
  the same top-level CLI error codes.
- DC-010 Backend-specific failure facts such as HTTP timeout, SQL auth failure,
  or missing local dolt binary shall be reported through structured error
  details without changing the top-level CLI error schema.
- DC-011 File-backed configuration keys shall resolve in this precedence:
  explicitly supplied CLI flag, environment variable, config file value, then
  default. CLI flag default values that were not explicitly supplied shall not
  override environment or file config.
- DC-012 Admin or other write-path commands that mutate Dolt state shall not
  use `HTTPClient`. MVP write-capable backends are `SQLClient` and `CLIReader`.
  If a write-path command is invoked with `--client http` or an effective
  `http` client selection, the CLI shall fail with a typed
  `unsupported_backend` error rather than silently switching transports.

See `docs/dolt-api.md` for API contracts and source citations.

## 4. Dolt Branch Access

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
- BR-006 Read operations shall use explicit branch-selected reads so multiple
  readers can query different branches safely in parallel. DoltHub HTTP selects
  the branch in the request URL and shall use unqualified table names; MySQL
  protocol readers may use branch-qualified table references.
- BR-007 Admin or write-path operations may still target explicit branches, but
  those behaviors shall be documented independently in the relevant command
  design.
- BR-008 In MVP, user-selectable branch values map directly to Dolt branch
  names. There is no separate channel-to-branch translation layer.
- BR-009 Branch and version shall be tracked and reasoned about independently.
  Branch identifies the release track/channel; version identifies a release on
  that branch.

## 5. CLI Access Strategy

The CLI is expected to be used by both humans and AI wrappers.

- CLI-001 Human-facing command output may default to concise human-readable
  rendering.
- CLI-002 Machine-oriented callers shall use `--json` for stable structured
  output.
- CLI-003 AI wrappers and Claude-facing skills should pass `--json`
  explicitly rather than relying on ambient environment defaults.
- CLI-004 If an output-mode environment variable is introduced later, it shall
  not replace `--json` as the preferred explicit machine-access contract.
- CLI-005 JSON-mode success and error contracts shall be documented and stable
  enough for AI wrappers and automation to consume predictably.
- CLI-005a Each end-user command that supports `--json` shall return a typed
  JSON success envelope rather than ad hoc maps or prose fragments.
- CLI-005b In `--json` mode, bootstrap failures such as config validation,
  client selection, branch resolution, or Dolt connection setup shall still use
  the standard JSON error envelope.
- CLI-005c JSON error envelopes shall include a stable top-level `code`, a
  human-readable `message`, structured `details`, and corrective guidance such
  as `suggested_action` when recovery is possible. Retryability shall be
  modeled explicitly when it matters for automation.
- CLI-005d Transport-specific or backend-specific facts shall appear in
  structured error details rather than by changing the top-level schema or
  forcing callers to parse free-form text.
- CLI-005e Shared failure categories shall use the fixed top-level error-code
  vocabulary `invalid_args`, `not_found`, `ambiguous_target`,
  `unsupported_backend`, `backend_unavailable`, `backend_auth_failed`,
  `confirmation_required`, `blocked`, `conflict`, `validation_failed`, and
  `internal_error`. Commands may add more specific machine-readable cause codes
  only within structured `details`.
- CLI-005f Every JSON response shall include a top-level boolean `ok`.
  Successful responses omit `error`; failed responses omit success-only payload
  objects other than stable metadata such as `command`, `scope`, or `branch`
  when those fields are needed to identify the failed target.
- CLI-006 Human-readable output may suppress the branch label for `main` when
  that improves readability, but structured output shall emit `"main"`
  explicitly.
- CLI-007 `--json` shall control output format only. It shall not change
  mutation semantics, dry-run behavior, confirmation behavior, or command
  target selection.
- CLI-008 If a future interactive/session mode is launched with `--json`, the
  entire session shall emit JSON until the session ends.
- CLI-009 Any future command surface that accepts JSON arguments or a JSON
  request payload shall imply JSON output for that invocation.
- CLI-010 Commands that can target project-local state, machine-global state, or
  both shall use `--scope <project|global|both>`. Omitted `--scope` means
  `both`. `--global` and `--local` shall not be accepted CLI flags.
- CLI-010a Commands that produce a single-target export artifact rather than a
  natural multi-target action may reject omitted `--scope` with a typed
  `ambiguous_target` error when the same package exists in multiple install
  scopes. MVP defines `sc snapshot <package>` as such a command.
- CLI-011 If a wrapper or MCP surface is added above `sc`, it shall reuse the
  same business payloads and stable error-code families rather than define a
  second reshaped contract.
- CLI-012 Human-readable mode shall not expose machine-significant state that is
  absent from the JSON contract.
- CLI-013 The MVP `sc` CLI contract covers package-management, validation,
  snapshot, and catalog workflows only. ATM polling, sprint orchestration, git
  workflow control, and team messaging are outside the MVP CLI scope.

## 6. Module Boundaries And Interface Discipline

- MB-001 `src/cmd` is a CLI binding layer. It may parse flags, select output
  mode, and adapt process-level concerns, but package-management business rules
  shall live below the Cobra command layer.
- MB-002 Transport adapters, filesystem mutation, install tracking, catalog
  state, and rendering concerns shall be isolated behind package boundaries or
  interfaces so behavior can be tested without shelling through the full CLI.
- MB-003 Shared machine-contract DTOs for public JSON payloads shall live
  outside command handlers and be reusable by wrappers or future MCP surfaces.
- MB-004 Wrapper layers shall not parse human-readable CLI output, re-encode
  business payloads into a second canonical schema, or duplicate package
  management rules that belong in `sc`.
- MB-005 Domain and adapter packages shall not depend on Cobra, terminal UI
  state, or other command-entrypoint concerns.
- MB-006 End-user package operations shall be implemented behind shared
  operation interfaces or packages that are reusable by command handlers and
  reviewable without shelling through the entire CLI surface.

## 7. Agents And Scripts

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

## 8. Install Targets And Product State

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
  marked `local-only` and requested `--scope` includes `global`, `sc install`
  shall fail with a clear error and shall not perform a partial install.
- FS-006 `doc_path` is the canonical package artifact path for integrity and
  catalog identity. It is relative to the package root and does not include
  runtime roots such as `.claude/`, `~/.claude/`, `.agents/`, or
  `~/.agents/`. The current Dolt schema stores this value in
  `package_files.dest_path`; implementation code may keep that column name but
  shall treat the value semantically as `doc_path`.
- FS-007 Package metadata shall own the mapping from `doc_path` to runtime
  target paths. MVP may implement only the `.claude` target, but the model must
  allow package metadata to support future targets such as `.agents` and
  target-specific artifacts such as `agent1.claude.md` or `agent1.agents.md`.
- FS-008 Install tracking shall distinguish the canonical package `doc_path`
  from the materialized path on disk for a particular install site.
- FS-009 The CLI shall support exporting installed package state via
  `sc snapshot <package>`.
- FS-010 `sc snapshot` shall default to exporting only modified tracked files
  for the targeted install.
- FS-011 `sc snapshot --full` shall export the full tracked install payload for
  the targeted package.
- FS-012 Snapshot exports shall be written under machine-level product state
  rather than into the active repository tree.
- FS-013 Snapshot metadata shall be stored in a human-readable
  `snapshot.toml` file that records, at minimum, package name, branch, version,
  snapshot timestamp, source install path, and repository path when applicable.

## 9. Install Tracking And State Safety

- ST-001 Synaptic Canvas shall track all local and global package installs on
  the machine in product-managed state under `.synaptic/` or `~/.synaptic/`.
- ST-002 Local and global installs of the same package shall be treated as a
  normal supported state. Status, validate, upgrade, and uninstall flows shall
  be able to target project installs, global installs, or both.
- ST-003 Install tracking shall record, at minimum, package identity, version,
  branch, Dolt source reference, install scope, materialized file inventory,
  canonical `doc_path` inventory, and dependency provenance.
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
- ST-008 Every mutating end-user command shall have a corresponding read path
  rich enough to confirm resulting state without relying only on logs or direct
  filesystem inspection.
- ST-009 Mutating end-user commands shall update tracked state atomically per
  targeted install scope. If a command fails after beginning side effects, the
  CLI shall either roll the targeted install scope back to the prior tracked
  state or return a typed failure that explicitly reports which rollback steps
  succeeded, which failed, and what manual recovery remains.

## 10. Install, Upgrade, And Uninstall Behavior

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

## 11. Validation And Inventory

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
- VA-009 The installed-state read contract shall be rich enough to confirm
  mutation results. At minimum, structured readback shall expose scope, branch,
  version, install site/root, dependency provenance summary, hook-registration
  summary, and local modification inventory for the targeted install record.
- VA-010 File-level validation state vocabulary shall be fixed as `ok`,
  `modified`, `missing`, `unreadable`, and `extra`. Higher-level failures such
  as dependency incompatibility, hook drift, template drift, or aggregate
  integrity mismatch shall be modeled as typed validation items with severities
  rather than by inventing additional file-state names.

## 12. SHA Catalog

The SHA catalog is the authoritative, local-cacheable reference for all
immutable `(package_id, version, doc_path, sha256)` tuples known for a given
branch.

- CA-001 The CLI shall maintain a local SHA catalog cache per branch at
  `.synaptic/catalog-{branch}.toml` for project installs and at
  `~/.synaptic/catalog-{branch}.toml` for machine-level state.
- CA-002 The catalog shall store, at minimum, `(package_id, version,
  doc_path, sha256)` tuples. `doc_path` is package-root-relative and independent
  of install location.
- CA-003 The catalog shall be refreshed from Dolt when a connection is
  available. Refresh shall merge newly observed package/version/doc_path tuples
  into the catalog and preserve previously observed versions so repo-local and
  global installs can validate even when they are upgraded at different times.
  When Dolt is unavailable the CLI shall fall back to the local cache and emit
  a clear warning if the cache is stale or absent.
- CA-004 `sc validate` shall use catalog SHAs as the authoritative expected
  source for file integrity comparison. The lockfile is used only to identify
  which packages are installed, at what version and branch, and which paths
  are tracked.
- CA-005 `sc catalog update [--branch <branch>]` shall explicitly fetch the
  current catalog from Dolt and write it to the local cache. Catalog refresh
  shall also be triggered implicitly by `sc install` and `sc init`.
- CA-006 The SHA immutability invariant shall be: one SHA per
  `(package_id, version, doc_path, branch)` tuple, forever. No tooling shall
  produce or accept a second SHA for an existing tuple.
- CA-007 `sc admin import` shall check the existing catalog before writing.
  If `(package_id, version, doc_path)` already exists on the target branch with
  a different SHA, the import shall be rejected with a hard error naming the
  colliding file and both SHAs. A new version of the same package may use the
  same `doc_path` with a different SHA.
- CA-008 `sc scan` shall use only on-disk files plus the local catalog cache.
  It shall not initiate a live Dolt read implicitly. If the required catalog is
  absent, `sc scan` shall fail with a typed `validation_failed` error that
  names the missing catalog and the required `sc catalog update` action.

## 13. Verification Traceability

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
- VER-010 Each supported Dolt client shall have simulator-backed or equivalent
  deterministic adapter tests below the CLI layer. Routine CI shall not depend
  on live DoltHub, a running SQL server, or a local Dolt clone.
- VER-011 Contract-conformance tests shall run equivalent command or adapter
  scenarios across `http`, `sql`, and `cli` client modes and assert the same
  top-level JSON success and error schemas where behavior is semantically
  equivalent.
- VER-012 Live DoltHub verification shall be an explicit human/AI-driven
  integration procedure, not a CI requirement. CI shall not make live network
  calls to DoltHub; live tests must be opt-in and configurable to use a
  dedicated project test repository with deterministic fixture data.
