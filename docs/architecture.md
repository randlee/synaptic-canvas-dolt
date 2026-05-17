# Synaptic Canvas Architecture

This document is the top-level architecture overview for the repository.
Focused, drift-prone decisions are captured separately as ADRs under
`docs/adr/`; this overview and those ADRs are jointly normative with
`requirements.md`.

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

## 1.1 Normative Set

The normative design set for implementation and quality review is:

- `requirements.md` for cross-cutting product requirements
- this architecture overview for module boundaries and system shape
- accepted ADRs under `docs/adr/` for decisions that are narrow enough to
  decay if left only in broad prose

Sprint plans and QA reviews should cite requirement IDs and ADR IDs directly.

## 1.2 MVP Boundaries

The initial release boundary is intentionally narrow:

- the only supported MVP runtime artifact targets are `.claude/` and
  `~/.claude/`
- the canonical machine contract is `sc --json`
- higher-level ATM polling, sprint loops, team messaging, and git orchestration
  are not part of the `sc` CLI or `sc:plugin` MVP scope
- future targets such as `.agents/`, `.codex/`, or alternate machine-contract
  surfaces are deferred until promoted into the normative set

## 2. Dolt As The Source Of Truth

Dolt remains the package system database because it provides:

- relational querying for packages, files, dependencies, hooks, and variants
- branch-based release channels
- auditable promotion history
- a single authoring source for CLI reads, export pipelines, and future offline
  snapshots

The CLI is a client of Dolt. End users are not expected to reason about Dolt
session state during normal CLI use.

### Dolt Client Architecture

The `src/pkg/dolt` package exposes a `Client` interface. Multiple
implementations exist; the active implementation is selected by configuration.
See `docs/dolt-api.md` for full API contracts, source citations, and
requirement traceability.

| Implementation | Protocol | Target | MVP |
|----------------|----------|--------|-----|
| `HTTPClient`   | DoltHub REST API | dolthub.com public/private repos | **Yes** |
| `SQLClient`    | MySQL wire protocol | Hosted Dolt, local dolt sql-server | **Yes** |
| `CLIReader`    | subprocess `dolt sql -q` | local dolt clone | **Yes** |

Architecture rules:

- all three clients are first-class supported transport modes
- active client selection is explicit and deterministic, with compatibility
  inference allowed only when documented and testable
- the public `sc --json` contract is transport-invariant; backend differences
  appear in structured metadata or error details rather than top-level schema
  changes
- read-path commands may use `HTTPClient`, `SQLClient`, or `CLIReader`, but
  MVP write-path admin commands are limited to `SQLClient` and `CLIReader`;
  write commands reject `HTTPClient` explicitly rather than silently switching
  transports
- all client implementations must satisfy a shared conformance suite for
  equivalent operations
- branch is always passed as a URL path segment in HTTP requests — no session
  state exists between calls (satisfies BR-004, BR-005)
- for MySQL-protocol implementations, branch is encoded in the qualified table
  reference `` `database/branch`.table `` — `DOLT_CHECKOUT` is prohibited

## 3. Branch And Channel Model

`develop`, `beta`, and `main` remain the release branches. Read behavior must
still be explicit.

Architecture rules:

- the CLI resolves an effective branch using `--branch`, then
  `SC_DOLT_BRANCH`, then `main`
- the CLI ignores the current/checked-out Dolt branch for read-path behavior
- read operations should be explicitly branch-selected rather than relying on
  session switching. DoltHub HTTP selects the branch in the URL and uses
  unqualified SQL table names; MySQL protocol readers may use branch-qualified
  table references.
- in MVP, externally selectable branch values are the Dolt branch names
  directly; there is no separate channel-mapping layer
- branch and version are first-class, independent concepts: branch identifies
  the release track/channel and version identifies a release on that branch
- routine upgrades move to the latest compatible version on the currently
  tracked branch unless the user explicitly changes `--branch` or `--version`

This keeps CLI behavior deterministic and allows multiple readers to query
different branches safely in parallel.

## 3.1 Module Boundaries

The MVP architecture keeps boundaries narrow and explicit:

- `src/cmd`
  CLI binding layer only. Parses args, resolves process-level config, invokes
  lower-level operations, and renders output. It should not own core package
  management policy.
- `src/pkg/api`
  Shared request/response DTOs and typed error models for the public machine
  contract. This is the seam reused by CLI handlers and any future wrapper or
  MCP surface.
- `src/pkg/operations` or equivalent workflow packages
  Shared package-management use cases for install, upgrade, uninstall, status,
  validate, scan, and snapshot. This layer owns workflow policy beneath the
  command surface.
- `src/pkg/dolt`
  Transport boundary for read and write access to Dolt. `HTTPClient`,
  `SQLClient`, and `CLIReader` hide transport specifics behind shared
  interfaces and conformance tests.
- `src/pkg/installer`, `src/pkg/catalog`, `src/pkg/integrity`,
  `src/pkg/questionnaire`, `src/pkg/repo`, `src/pkg/template`
  Domain and workflow packages. Own install planning, mutation, validation,
  catalog, prompting state, repo profiling, and template validation without
  depending on Cobra or terminal rendering.
- `src/internal/config`, `src/internal/output`, `src/internal/logging`
  Infrastructure support packages for config layering, rendering, and logging.
  They support the CLI but do not own package-management rules.
- `sc:plugin` and future wrappers
  Thin callers over the public machine contract. They do not redefine package
  rules, backend selection rules, or audit semantics.

Boundary rules:

- business rules move downward into packages, not upward into command handlers
- command handlers call shared operation-layer workflows rather than each
  owning an independent business-logic path
- transport-specific behavior stays inside adapters and surfaces through typed
  results or typed error details
- wrappers never become the second implementation of the product

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
- live DoltHub checks are human/AI-driven integration verification only; CI must
  use local fixtures, mocks, or `httptest` and must not depend on external
  DoltHub state
- sprint completion reports are part of verification traceability; they record
  issues encountered, fixes completed, and deferred follow-up items so phase
  hardening work is driven by explicit evidence rather than chat memory
- future package validation may include test evidence captured alongside package
  metadata or adjacent validation records, but that evidence model is a later
  design step rather than an MVP prerequisite

## 5. AI Access Strategy

The CLI is expected to be used heavily by AI wrappers.

Architecture guidance:

- human output may remain concise and readable by default
- AI callers should invoke the CLI with `--json`
- `--json` controls output format only and must not change mutation behavior
- JSON-mode bootstrap failures still use the same typed error family as
  command-level failures
- wrappers and future MCP surfaces reuse the same business payloads and error
  families rather than translating to a second contract
- a future interactive/session mode launched with `--json` keeps JSON output
  for the whole session, and future JSON request payloads imply JSON output for
  that invocation
- any future environment-based output default is secondary to explicit
  `--json` invocation
- the package-management wrapper scope is intentionally narrow; higher-level ATM
  task polling, sprint loops, and repo orchestration remain separate concerns

This keeps machine access explicit and avoids ambiguity caused by environment
state while preserving identical command semantics for human and JSON output.

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
- the same package may legitimately be installed locally and globally on
  different branches and/or versions at the same time
- installed package artifacts under `.claude/` or `~/.claude/` must not be held
  under long-lived product locks; coordination is limited to `.synaptic/` or
  `~/.synaptic/` state mutation and the design must avoid stale lock artifacts
  entirely
- product-managed tracking must record where a package is installed, what
  version and branch it came from, what files were materialized, and which
  external dependencies were already present versus installed by Synaptic
  Canvas
- package integrity is based on `doc_path`, the package-root-relative artifact
  path. Runtime roots such as `.claude/`, `~/.claude/`, `.agents/`, and
  `~/.agents/` are install-target concerns, not immutable package identity.
- package metadata maps each `doc_path` to the runtime target path for the
  selected AI agent ecosystem. MVP implements the `.claude` target first while
  keeping the model open for `.agents` and target-specific artifacts.
- the product should support inventory reconciliation by scanning repositories
  for tracked package artifacts and reconciling them into local machine state
- `sc snapshot` exports installed package state into product-managed snapshot
  directories under machine-level Synaptic Canvas state so local modifications
  can be reviewed without mutating the active installation
- future target roots such as `.codex/` and `.agents/` are compatible with this
  model but remain post-MVP

This keeps runtime-facing files aligned with the host tool while allowing
Synaptic Canvas to own its own operational state cleanly.

## 6.1 Installer Managed/Unmanaged Boundary

The installer owns a narrow managed set and must preserve everything else.

Managed by the installer:

- the `sc` binary at the selected install location
- the `sc:plugin` managed payload under `~/.claude/skills/sc-plugin/`
- installer state under `~/.synaptic/installers/sc-plugin/`
- the initial creation of `~/.sc/config.toml` when it does not yet exist

Not managed by the installer:

- user edits to `~/.sc/config.toml` after creation
- unrelated files under `~/.claude/`, such as `~/.claude/agents/`
- unmanaged files added by the user inside `~/.claude/skills/sc-plugin/`
- any repo-local package installs or product state outside the installer-owned paths

Boundary enforcement rules:

- the installer updates managed skill payload files by reconstructing the next
  tree in a staging directory, then swapping it into place only after the copy
  succeeds
- unmanaged files inside the target skill tree are preserved into the staged
  replacement tree before the final swap
- installer reruns may delete previously managed files that are no longer part
  of the shipped `sc:plugin` payload, but must not delete unmanaged files
- failure during staged copy must leave the live installed skill tree unchanged
- managed/unmanaged behavior is repository-verified by installer tests on both
  macOS/Linux and Windows

## 7. Command Architecture

The end-user CLI surface is split into read, install, validation, and export
operations:

- `sc list` and `sc info` are read-only catalog queries
- `sc init` and `sc install` materialize package state into runtime artifact
  roots and product-managed state
- `sc validate` and `sc status` inspect installed state and report drift
- `sc snapshot` exports installed state for analysis and comparison workflows
  without writing back to Dolt
- admin commands remain opt-in and own Dolt write-path behavior

`sc snapshot` belongs with the end-user validation/inspection family rather
than the admin export pipeline because it operates on installed state already
materialized on disk.

## 8. End-User State And Validation

Phase 3 end-user commands operate against local tracked installation state plus
explicit Dolt reads.

Architecture rules:

- `install`, `status`, `validate`, `scan`, `snapshot`, `upgrade`, and
  `uninstall` must be able to reason about project-local installs, global
  installs, or both in one invocation
- scope-aware end-user commands use `--scope` with an enum and default to
  `both` when omitted; legacy scope aliases such as `--global` and `--local`
  are not accepted
- mutating commands must have audit symmetry: immediate mutation result,
  structured follow-up readback, and retained product-managed state where
  applicable
- validation is broader than file checksum verification; it includes installed
  file presence, aggregate package integrity, dependency presence, dependency
  version compatibility, hook registration state, and template-validation state
- validation should inventory local modifications rather than reducing all drift
  to corruption; locally modified files are part of the managed state picture
- validation output is list-based and severity-driven rather than a single
  monolithic pass/fail result
- local modification snapshots are exported through a separate `snapshot`
  command into product-managed global staging, organized by
  package/branch/repository, so changes can be compared across installed
  versions and prepared for re-import
- `sc snapshot <package>` is a single-target export command; if the same
  package exists in multiple install scopes and `--scope` is omitted, the CLI
  should fail with an `ambiguous_target` error rather than guessing
- scan/reconciliation should present discovered installs and the candidate
  actions first, then let the user choose `add all`, `upgrade all`, or `skip`
- mutating commands update tracked state atomically per targeted install scope;
  partial failures require explicit rollback reporting rather than silent
  half-applied state
- uninstall behavior depends on dependency provenance: dependencies installed by
  Synaptic Canvas may be offered for removal, while dependencies that predated
  the install are left untouched
- non-interactive install/upgrade flows must remain explicit. `--yolo` is the
  acknowledged "proceed without prompting" mode for install, upgrade, and
  uninstall, and still records what external dependencies were installed
- the default human-readable status view may suppress the branch label for
  `main`, but structured output must emit `"main"` explicitly
- upgrade planning must warn and skip blocked upgrades by default; force
  behavior is an explicit targeted override, not the default batch behavior

## 9. Testing And Client Conformance

Synaptic Canvas has three supported read transports, but only one public
machine contract.

Architecture rules:

- simulator-backed or equivalent deterministic adapter tests exist below the
  CLI layer for `HTTPClient`, `SQLClient`, and `CLIReader`
- shared conformance tests assert equivalent top-level success and error
  behavior across all supported client modes
- live DoltHub, hosted SQL, and local-clone verification remain manual or
  AI-driven integration checks outside normal CI
- flaky tests are treated as blocking defects, especially in contract and
  backend-conformance suites

## 10. SHA Catalog

The SHA catalog is a locally-cached, per-branch set of known immutable Dolt SHA
references for package artifacts. It enables offline validation, reconciliation
via `sc scan`, and import collision enforcement.

Architecture rules:

- one SHA per `(package_id, version, doc_path, branch)` tuple is the
  immutable invariant; no tooling produces or accepts a second SHA for an
  existing tuple
- `doc_path` is independent of install site; two repositories can validly have
  the same package installed at different versions and each validates against
  the catalog entries for its own tracked version
- the catalog is the authoritative expected-SHA source for `sc validate`;
  the lockfile records install identity but not the authoritative SHA
- the catalog is stored at `.synaptic/catalog-{branch}.toml` (project) and
  `~/.synaptic/catalog-{branch}.toml` (machine-level); schema mirrors the
  Dolt `package_files JOIN packages` result for that branch, with
  `package_files.dest_path` treated as `doc_path`
- catalog fetch is triggered explicitly by `sc catalog update` and implicitly
  by `sc install` and `sc init`; refresh merges entries and preserves older
  known versions for the same branch
- when Dolt is unreachable, commands that require the catalog use the cached
  copy and emit a warning; they do not fail unless the cache is absent
- `sc scan` uses the catalog to identify packages from on-disk SHAs without
  requiring or initiating a live Dolt connection
- `sc admin import` checks before writing; a SHA collision on an existing
  `(package_id, version, doc_path, branch)` is a hard rejection, while a new
  version may reuse the same `doc_path` with a different SHA

## 11. Agent And Script Architecture

Repository-local agent definitions and helper scripts are part of the product
surface for Claude-facing workflows.

Architecture rules:

- agents must follow the shared authoring guidelines in the sibling
  `synaptic-canvas` repository
- agents must define input/output contracts
- helper scripts must be unit-tested
- sprint plans must define how agent/script behavior is verified

## 12. Detailed Design Documents

This architecture overview is intentionally high level. The detailed subsystem
documents remain:

- `requirements.md`
- `docs/adr/`
- `dolt-api.md`
- `synaptic-canvas-cli.md`
- `synaptic-canvas-schema.md`
- `synaptic-canvas-export-pipeline.md`
- `synaptic-canvas-install-system.md`
- `synaptic-canvas-hook-system.md`
- sprint plans under `docs/`
