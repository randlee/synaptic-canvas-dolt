# Phase 3 Plan Review

## Scope

Reviewed:

- `docs/project-plan.md` Phase 3 sprints 3.1 through 3.4
- `docs/requirements.md`
- `docs/synaptic-canvas-cli.md`
- `docs/synaptic-canvas-install-system.md`
- `docs/synaptic-canvas-schema.md`
- current `develop` read-path code under `src/pkg/dolt/`, `src/internal/config/`, and `src/cmd/admin/`

## Resolution Addendum

The findings below drove the hardening pass on `feature/phase3-plan-hardening`.
The current doc set resolves the major blockers by:

- defining one normative `manifest.lock` schema based on flat `[[installs]]`
  records and deprecating legacy `[[skills]]` examples
- making `status` the merged presentation view while validate/upgrade/uninstall
  operate on concrete install records
- adding `sc snapshot` to the CLI command surface
- aligning `sc upgrade` with `--branch` and `--version`
- defining the JSON error envelope plus concrete `list` and `install` JSON
  success shapes
- defining `sc init` minimum artifacts explicitly
- defining new-question detection via tracked `question_snapshot.question_ids`
  instead of a schema change to `package_questions`
- making atomic replacement and transactional staging part of the accepted
  state-safety contract

## Findings

### 1. Sprint 3.2 install tracking format is still under-specified at the plan level

- Sprint: 3.2
- Severity: blocker
- Sources:
  - `docs/project-plan.md:323-345`
  - `docs/synaptic-canvas-install-system.md:317-354`
  - `docs/synaptic-canvas-install-system.md:579-593`

The plan says to add `src/pkg/installer/tracking.go` and to record install state in a lockfile, but Sprint 3.2 acceptance only says "Records installed package/version/branch for status tracking, including `template_validation` in lockfile." That is not enough to implement `status`, `validate`, `upgrade`, and `uninstall` without re-deriving behavior from other docs.

The install-system doc contains a lockfile example, but the project plan does not make that example normative. It also does not say whether the Phase 3 implementation must support exactly one lockfile format, whether project-local and global state share one schema, or whether `skills.files` is authoritative for uninstall.

Recommended fix:

- Add an explicit Sprint 3.2 acceptance item that the initial lockfile schema is defined by the install-system example.
- Promote the minimum required fields into project-plan acceptance:
  - package id
  - version
  - Dolt commit
  - branch
  - variant
  - install scope
  - materialized file map
  - answers
  - requirements snapshot
  - repo-profile snapshot
  - template-validation record
- State explicitly that uninstall and validate operate from tracking state, not from filesystem discovery alone.

### 2. Phase 3 does not define how local and global installs coexist for the same package

- Sprint: 3.2, 3.3, 3.4
- Severity: blocker
- Sources:
  - `docs/project-plan.md:331-350`
  - `docs/synaptic-canvas-install-system.md:352-366`
  - `docs/requirements.md:103-111`

The docs distinguish project-local state (`.synaptic/manifest.lock`) and global state (`~/.synaptic/manifest.lock`), but the plan never defines command behavior when the same package is installed in both places. That leaves `status`, `validate`, `upgrade`, and `uninstall` ambiguous:

- Does `sc status` show only repo-local installs when run inside a repo?
- Does it also show global installs?
- If both exist, which one does `sc uninstall <package>` remove by default?
- Does `sc validate --all` mean all packages in the current repo, all global packages, or both?

Recommended fix:

- Add explicit precedence and targeting rules:
  - default command scope when run in a repo
  - default command scope when run outside a repo
  - how users target global vs project installs when both exist
- Add one explicit scope enum flag to `status`, `validate`, `upgrade`, and `uninstall`.

### 3. Sprint 3.3 acceptance is missing `UNREADABLE`, aggregate result shape, and mixed-scope status behavior

- Sprint: 3.3
- Severity: blocker
- Sources:
  - `docs/project-plan.md:356-368`
  - `docs/synaptic-canvas-cli.md:88-97`
  - `docs/synaptic-canvas-cli.md:199-218`

The CLI design already defines `UNREADABLE` and limits `EXTRA` to managed install paths, but Sprint 3.3 acceptance only requires `OK`, `MODIFIED`, `MISSING`, and `EXTRA`. That is a direct coverage gap against the CLI design.

The plan also says "Validate computes and checks aggregate SHA" but never specifies the user-visible result shape:

- pass/fail field name
- behavior when one file is unreadable
- behavior when files are missing
- JSON contract for aggregate results

`status` is also underspecified. "Status shows installed packages, versions, branches, validation state" does not define:

- whether install scope is shown
- whether variant is shown
- whether requirement-health or template-validation status is shown
- whether global and project installs can both appear

Recommended fix:

- Add `UNREADABLE` explicitly to Sprint 3.3 acceptance.
- Add a required aggregate result field and JSON contract.
- Add `install_scope` to `status` acceptance at minimum.
- Clarify whether `status` also surfaces template-validation and requirement-health data that the install-system already records.

### 4. Sprint 3.4 upgrade and uninstall are not specific enough to be safe

- Sprint: 3.4
- Severity: blocker
- Sources:
  - `docs/project-plan.md:374-385`
  - `docs/synaptic-canvas-install-system.md:370-388`

The project plan reduces upgrade to "check for newer version → install → verify" and uninstall to "remove files → update tracking." That omits several behaviors already defined elsewhere:

- prompt for new questions
- re-render if repo profile changed
- verify changed dependencies
- re-resolve variant selection
- preserve unchanged skills without re-verification

Uninstall is also not safe as written. "Remove package files → update tracking" does not define:

- what happens to shared files
- what happens to hooks shared across packages
- whether uninstall refuses to remove files modified locally
- whether empty package directories and empty `.synaptic` subdirectories are cleaned up

Recommended fix:

- Expand Sprint 3.4 acceptance to include the upgrade rules already specified in the install-system doc.
- Add uninstall acceptance for:
  - only removing files owned by the target package
  - preserving shared dependencies still referenced by other installed packages
  - removing hook registrations only when no remaining package owns them
  - clear behavior when tracked files are locally modified or missing

### 5. Sprint 3.1 tag filtering is underspecified relative to the schema

- Sprint: 3.1
- Severity: minor
- Sources:
  - `docs/project-plan.md:303-314`
  - `docs/synaptic-canvas-schema.md:72-94`

Sprint 3.1 requires `--tags` filtering, but the schema stores tags as comma-separated text, not a normalized table. The plan never defines:

- AND vs OR behavior for multiple tags
- case sensitivity
- whitespace normalization
- exact match vs substring

Without that, implementations can diverge while still technically "filtering by tags."

Recommended fix:

- Add a precise rule, for example:
  - split stored tags on commas
  - trim whitespace
  - compare case-insensitively
  - treat multiple requested tags as OR or AND explicitly

### 6. The plan does not say whether list/info/install must use the existing `Client` interface or extend it

- Sprint: 3.1, 3.2
- Severity: minor
- Sources:
  - `docs/project-plan.md:302-328`
  - `src/pkg/dolt/client.go`
  - `src/pkg/dolt/queries.go`

Current `develop` already has a branch-qualified read client and package/file/dependency accessors. That is good Phase 2 groundwork. But the Phase 3 plan does not say whether new end-user commands are expected to reuse that client directly, or whether they need additional read methods and query shapes.

That matters because `info` wants dependency and file-count detail, `list` wants tag filtering, and install will need variant resolution plus question/hook/file reads. The interface likely needs to be treated as a stable dependency instead of an implementation detail.

Recommended fix:

- Add an implementation note that Phase 3 must extend or reuse the existing branch-qualified read client rather than introducing a second read path.
- Name the expected additional query needs for Phase 3 explicitly:
  - list by tags
  - package detail with counts
  - question retrieval
  - hook retrieval
  - variant resolution for install

### 7. `sc init` scope is split across multiple docs but not anchored to a single acceptance contract

- Sprint: 3.2
- Severity: minor
- Sources:
  - `docs/project-plan.md:322`
  - `docs/project-plan.md:348-350`
  - `docs/synaptic-canvas-cli.md:77`
  - `docs/synaptic-canvas-install-system.md:386-388`
  - `docs/synaptic-canvas-dolt.md:441-443`

The plan says `sc init` bootstraps `.synaptic/` and is idempotent, but it never enumerates the minimum files it must create or refresh. The supporting docs imply at least:

- `.synaptic/manifest.lock`
- `.synaptic/repo-profile.toml`
- `.synaptic/env.toml`
- hook-registry scaffolding

That is enough surface area that "bootstraps `.synaptic/` state" is too loose for QA.

Recommended fix:

- Add an acceptance item listing the minimum initialized artifacts and which are required immediately vs lazily generated on first install.

### 8. Phase 3 still lacks an explicit JSON contract for end-user commands

- Sprint: 3.1, 3.2, 3.3, 3.4
- Severity: minor
- Sources:
  - `docs/project-plan.md:305-314`
  - `docs/project-plan.md:347`
  - `docs/project-plan.md:367-385`
  - `docs/requirements.md:76-83`

The plan repeatedly says commands support `--json`, but it does not define result fields or error-shape expectations for Phase 3 user commands. The requirements doc says JSON-mode errors should be stable enough for AI wrappers to consume, and Phase 4 depends on this.

Recommended fix:

- Add a short JSON-contract note per sprint, or one cross-cutting Phase 3 note, covering:
  - list result shape
  - info result shape
  - install summary shape
  - validate result shape
  - status result shape
  - upgrade/uninstall summary shape
  - error envelope rules

## Recommended Plan Changes By Sprint

### Sprint 3.1

- Define `--tags` semantics precisely.
- State whether `info` must surface variant, install scope, and package SHA.
- Add JSON response shape expectations.

### Sprint 3.2

- Make the lockfile/tracking schema explicit and normative.
- Define project vs global coexistence behavior.
- Define `sc init` minimum artifacts.
- Clarify whether dependency execution is warning-only for missing requirements, but still blocking for `local-only` scope violations and SHA failures.

### Sprint 3.3

- Add `UNREADABLE` explicitly.
- Define aggregate validation output and JSON shape.
- Define how status and validate behave across project vs global installs.

### Sprint 3.4

- Pull the upgrade rules from the install-system doc into sprint acceptance.
- Define uninstall ownership and shared-dependency behavior explicitly.
- Define default targeting when the same package is installed both locally and globally.

## Summary

Phase 3 is directionally sound and mostly aligned with the requirements and CLI/install docs, but the plan is still too loose in the areas that make end-user stateful commands risky:

- tracking format
- local vs global coexistence
- validate/status semantics
- uninstall ownership rules
- JSON contract stability

Those should be tightened before implementation starts so Phase 3 does not fragment into multiple incompatible interpretations.
