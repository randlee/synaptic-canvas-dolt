# Synaptic Canvas — `sc` CLI Design

## Overview

`sc` is a Go CLI that serves as the primary interface for the Synaptic Canvas package management system. It provides end-user commands for installing and managing Claude Code skill packages, and admin commands for ingesting packages into the Dolt database.

**DoltHub:** https://www.dolthub.com/repositories/randlee/synaptic-canvas

### Related Documents

- [Requirements](./requirements.md) — cross-cutting normative requirements
- [Architecture](./architecture.md) — module boundaries and system shape
- [ADRs](./adr/) — focused accepted design decisions
- [Schema Spec](./synaptic-canvas-schema.md) — Dolt table definitions and design rationale
- [Export Pipeline](./synaptic-canvas-export-pipeline.md) — Dolt → filesystem reconstruction logic
- [Install System](./synaptic-canvas-install-system.md) — Package installation mechanics
- [Hook System](./synaptic-canvas-hook-system.md) — Pre/post install hooks

---

## Architecture

### Three-Layer Design

```
┌─────────────────────────────────────────────────┐
│  Claude Code                                    │
│  ┌───────────────────────────────────────────┐  │
│  │  sc:plugin skill (Claude skill wrapper)   │  │
│  │  - Conversational interface to sc CLI     │  │
│  │  - Installed globally with sc             │  │
│  └──────────────────┬────────────────────────┘  │
│                     │ shells out                 │
│  ┌──────────────────▼────────────────────────┐  │
│  │  sc CLI (Go binary)                       │  │
│  │  - Package management commands            │  │
│  │  - Admin commands (opt-in)                │  │
│  │  - SHA validation, integrity checks       │  │
│  └──────────────────┬────────────────────────┘  │
│                     │ queries                    │
│  ┌──────────────────▼────────────────────────┐  │
│  │  Dolt Database (local or DoltHub remote)  │  │
│  │  - Branches = release channels            │  │
│  │  - Per-file SHA256 integrity              │  │
│  │  - Package-level aggregate SHA            │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

**Layer 1 — `sc` Go CLI:** The compiled binary. Binds command input/output to
shared package-management workflows, typed JSON DTOs, Dolt client adapters,
and file/state services. Distributed via GoReleaser release archives,
Homebrew, and `go install`. Winget publication is deferred pending publisher
registration (gh issue `#54`).

**Layer 2 — `sc:plugin` skill:** A Claude Code skill that wraps the `sc` CLI.
Allows Claude to manage packages conversationally ("install the delay
package"). Thin wrapper — delegates all logic to the CLI. It is not the place
for ATM polling, sprint loops, git orchestration, or other higher-level team
workflow control.

**Layer 3 — Dolt Database:** Source of truth. Packages, files, dependencies, and metadata stored in relational tables. Branches (`develop`, `beta`, `main`) serve as release channels. Package promotion copies specific package rows between branches via targeted SQL (DELETE + INSERT ... SELECT) followed by a Dolt commit. Whole-branch promotion uses `dolt_merge`.

### Installer

The `sc` installer installs both:
1. The `sc` Go binary (to PATH)
2. The `sc:plugin` skill globally (to `~/.claude/`)

This means any repo immediately has both the CLI and Claude's ability to use it.

Managed install roots for the MVP installer are:
- `sc` binary at the selected executable target, defaulting to `~/.local/bin/sc`
  on macOS/Linux and `%USERPROFILE%/AppData/Local/Programs/SynapticCanvas/bin/sc.exe`
  on Windows unless an explicit installer bin directory override is supplied
- `sc:plugin` under `~/.claude/skills/sc-plugin/`
- CLI config at `~/.sc/config.toml`
- installer-owned reconciliation metadata under `~/.synaptic/installers/sc-plugin/`

Reruns preserve a strict managed/unmanaged split:
- managed: the installed `sc` binary, tracked `sc:plugin` files, and the
  installer manifest under `~/.synaptic/`
- unmanaged: user edits to `~/.sc/config.toml`, unrelated files under
  `~/.claude/`, and repository-local package installs

Installer reruns overwrite managed files from source, remove no-longer-managed
`sc:plugin` files listed in the installer manifest, and leave unrelated user
files untouched.

---

## Command Surface

### End-User Commands (default)

Available to all users. These commands interact with installed packages and the Dolt database as a consumer.

```
sc list [--branch <branch>] [--tags <tag,...>]
    List available packages. Defaults to the `main` branch.

sc info <package> [--branch <branch>]
    Show package details: version, description, dependencies, file count, SHA.

sc init
    Initialize `.synaptic/` state for the current repository.

sc install <package> [--scope <project|global|both>] [--branch <branch>] [--dry-run] [--yolo]
    Install a package from Dolt.
    --scope     Install to project .claude/, global ~/.claude/, or both (default: both)
    --branch    Install from specific branch (default: main)
    --dry-run   Show the install plan and template preview without side effects
    --yolo      Execute the computed install plan without interactive confirmation

sc upgrade <package> [--all] [--scope <project|global|both>] [--branch <branch>] [--version <version>] [--yolo] [--force]
    Upgrade installed package(s) to latest version on their tracked branch and scope by default.
    --branch    Explicitly retarget upgrade to a different branch
    --version   Explicitly target a specific version on the chosen branch
    --force     Override blocked-upgrade checks for a single targeted package.
                May not be combined with --all; exits with error if both are supplied.

sc uninstall <package> [--scope <project|global|both>] [--yolo] [--force]
    Remove an installed package from the selected tracked install scope.
    --force     Remove package files even when local modifications are detected.
                Without --force, locally modified files cause a prompt [y/N] or error in non-TTY mode.

sc validate [<package>] [--all] [--scope <project|global|both>]
    Verify installed files, dependency state, and tracked install health.
    Reports: ok, modified (local edits), missing, unreadable (permission denied or I/O error), extra (untracked files).
    `extra` is limited to files inside the installed package's managed target
    paths that are not tracked by that package manifest.
    Validation results are list-based and each item carries an explicit severity.

sc status [--scope <project|global|both>]
    Show installed packages, their versions, branches, scopes, and validation state.

sc scan [<path> ...] [--recurse] [--scope <project|global|both>] [--accept-all] [--upgrade-all]
    Scan for installed packages and reconcile them into local tracking state.
    Default mode: walks .claude/ (project) and ~/.claude/ (global).
    Custom paths as positional args override scope. Discovers installs by mapping on-disk files
    to package doc_path values and SHA-matching against the local catalog.
    Default mode lists candidates only — no tracking state mutation.
    --accept-all   Write lockfile entries for all discovered untracked installs
    --upgrade-all  Upgrade existing tracked installs to the catalog version
    --accept-all and --upgrade-all are mutually exclusive.
    --json controls output format only; mutation still depends on explicit action flags.
    Requires local catalog; never triggers a live Dolt connection. Run sc catalog update first.

sc catalog update [--branch <branch>] [--scope <project|global|both>]
    Fetch the SHA catalog for the effective branch from Dolt and write it to the local cache
    at .synaptic/catalog-{branch}.toml (local) or ~/.synaptic/catalog-{branch}.toml (global).
    --scope controls write target:
      project → .synaptic/catalog-{branch}.toml
      global  → ~/.synaptic/catalog-{branch}.toml
      both    → writes both locations
    Default scope: both.
    Also triggered implicitly by sc install and sc init.

sc config get <key>
    Read a configuration value. Prints the resolved value using explicit flag,
    environment, ~/.sc/config.toml, then default precedence. Unknown keys are
    rejected with an error.

sc config set <key> <value>
    Write a configuration value to ~/.sc/config.toml, creating the file if absent.
    Unknown keys are rejected. Valid keys: dolt.client, dolt.host, dolt.database,
    dolt.token, dolt.dsn, dolt.dir, dolt.timeout.

sc snapshot <package> [--scope <project|global|both>] [--full]
    Export local modifications for the selected package into global snapshot staging.
    By default exports modified tracked files only; `--full` captures the full installed package state.
    If the package is installed in more than one scope and --scope is omitted,
    the command fails with `ambiguous_target` rather than guessing.
```

### Admin Commands (opt-in)

Not enabled by default. Intended for package authors and maintainers. These commands modify the Dolt database.

```
sc admin import <path> --branch <branch>
    Ingest a package directory into Dolt on the specified branch.
    Computes SHA256 per file and aggregate package SHA.
    Creates a Dolt commit with package metadata.
    Runs template variable validation on .j2 files (warning, non-blocking).
    Enforces CA-007 before writing: hard-fails if any file's
    (package_id, version, doc_path, branch) tuple already exists in the
    catalog with a different SHA. The error names the colliding file plus
    existing and incoming SHAs.

sc admin export <package> --output <dir> [--branch <branch>]
    Export a package from Dolt to filesystem.
    Defaults to the effective branch (`--branch`, then SC_DOLT_BRANCH, then main).
    Reconstructs manifest.yaml and plugin.json from relational data.
    Verifies SHA on each exported file.

sc admin publish <package> --from <branch> --to <branch>
    Promote a package between channels (e.g., develop → beta → main).
    Promotes a package by copying its rows (packages, package_files, deps, hooks, questions) from the source branch to the target branch via targeted SQL, then commits.
    Runs template variable validation as a BLOCKING gate — publish
    fails if any .j2 template references undeclared variables.

sc admin promote <package> --from <branch> --to <branch>
    Targeted package promotion. Copies only the named package's rows from source
    branch to target branch using DELETE + INSERT ... SELECT targeted SQL, then
    creates a Dolt commit. This does not merge unrelated branch changes.

sc admin promote all --from <branch> --to <branch>
    Whole-branch promotion. Runs a Dolt branch merge (`dolt_merge`) from source
    into target and commits the branch-level result. Use only when the entire
    source branch is ready to promote.

sc admin verify <package> [--branch <branch>]
    Full integrity check within Dolt: recompute all SHA256 hashes
    Defaults to the effective branch (`--branch`, then SC_DOLT_BRANCH, then main).
    from stored content and compare against stored hashes.

sc admin diff <package> --branch1 <b1> --branch2 <b2>
    Show differences between package versions across branches.
```

### Global Flags / Environment

```
--client <http|sql|cli>
                      Dolt read-client override. Selects DoltHub HTTP API,
                      MySQL-compatible SQL transport, or local `dolt sql`
                      subprocess mode.
--dolt-client <http|sql|cli>
                      Deprecated hidden alias for `--client`, retained for
                      backward compatibility.
--dolt-dir <path>     Path to Dolt database directory for `cli` mode or
                      compatibility inference (default: auto-detect when
                      `--client=cli`)
--remote <url>        Optional Dolt remote override for commands that connect to a non-default Dolt host
--branch <branch>     Read/query branch override (default: SC_DOLT_BRANCH or main)
--json                Output as JSON (for scripting/skill integration)
--quiet               Suppress non-essential output
--verbose             Detailed output including SHA hashes
```

`--json` is strictly an output-format selector. It must not change whether a
command mutates state; mutation is controlled by command action flags such as
`--accept-all`, `--upgrade-all`, `--yolo`, and `--dry-run`. If a future
interactive/session mode is started with `--json`, all commands in that session
inherit JSON output. Any future command that accepts JSON arguments or a JSON
request payload also emits JSON output for that invocation even if `--json` is
omitted.

Read-path branch resolution order:

1. `--branch`
2. `SC_DOLT_BRANCH`
3. `main`

The CLI should ignore the current Dolt session branch for read behavior and use
the resolved branch explicitly on each read operation.

There is no separate user-facing `--channel` abstraction in MVP. End-user and
admin flows both use `--branch`, and those values map directly to Dolt branch
names.

`--remote` is primarily for admin and explicit remote-read workflows. Local
operations may rely on configured defaults; commands that require a non-default
remote should document that requirement explicitly.

Client selection rules:

1. `--client`, if supplied
2. environment override, if defined
3. `dolt.client` from file config
4. compatibility inference such as `--dolt-dir` implying `cli`
5. documented default

The public JSON contract must remain the same across `http`, `sql`, and `cli`
client modes. Backend-specific differences belong in structured metadata or
error details, not in top-level schema drift.

Write-path admin commands are different: MVP admin mutations may use `sql` or
`cli` only. If an admin mutation command resolves to `http`, the CLI returns a
typed `unsupported_backend` error and does not attempt a silent transport
switch.

---

## JSON Contracts

Structured JSON is the canonical automation contract. Human-readable rendering
may simplify presentation, but `--json` remains explicit and stable. Shared
request/response DTOs for end-user commands live in `src/pkg/api/`; Cobra
handlers render those DTOs directly rather than inventing command-local JSON
shapes.

### Error Envelope

All commands that support `--json` return errors in this envelope:

```json
{"ok": false, "error": {"code": "backend_unavailable", "message": "failed to query package catalog"}}
```

Rules:
- `ok` is always present
- success responses set `ok: true` and omit `error`
- `error.code` is stable for automation
- `error.message` is human-readable
- `details` may vary by command or backend but must remain structured JSON
- `retryable` is explicit when retry behavior matters for automation
- `suggested_action` is included when caller recovery is possible
- `details.operation` names the CLI workflow step that raised the failure, such
  as `get_package`, `list_packages`, `validate_scope`, or `install_scope`
- bootstrap failures in `--json` mode use the same error envelope family
- root-level config, branch, and argument failures must render through the same
  envelope instead of escaping as raw Go/Cobra stderr

Shared top-level `error.code` vocabulary:
- `invalid_args`
- `not_found`
- `ambiguous_target`
- `unsupported_backend`
- `backend_unavailable`
- `backend_auth_failed`
- `confirmation_required`
- `blocked`
- `conflict`
- `validation_failed`
- `internal_error`

Backend- or command-specific detail belongs in `error.details.cause_code`,
never in a new top-level error-code family invented by one command.

### `sc list --json`

```json
{
  "ok": true,
  "branch": "main",
  "filters": {
    "tags": ["git", "review"]
  },
  "packages": [
    {
      "id": "team-lead",
      "name": "team-lead",
      "version": "1.2.0",
      "branch": "main",
      "description": "Team lead workflow skill",
      "tags": ["team", "workflow"],
      "variant": "claude",
      "install_scope": "any",
      "file_count": 4,
      "dependency_count": 2,
      "sha256": "abc123"
    }
  ]
}
```

### `sc install --json`

Single-scope installs and dry-runs return one typed install payload. `plan:
true` indicates a dry-run response and no files were written.

```json
{
  "ok": true,
  "plan": false,
  "scope": "project",
  "package": {
    "id": "team-lead",
    "version": "1.2.0",
    "branch": "beta"
  },
  "install_root": "/repo/.claude/skills/team-lead",
  "files_written": 4,
  "dependencies": ["gh", "agent-teams-mail"],
  "dependency_warnings": [],
  "hooks_registered": [
    {
      "event": "PreToolUse",
      "matcher": "git commit",
      "script_path": ".claude/skills/team-lead/hooks/pre-commit.sh"
    }
  ],
  "template_validation_warnings": [],
  "warnings": [],
  "files": [],
  "answers": {}
}
```

Multi-scope installs use the same response type with `installs`,
`rolled_back`, and `failures`. Partial multi-scope failures keep the standard
top-level schema and set `error.code` from the shared vocabulary rather than
inventing a command-specific top-level code.

### `sc upgrade --json`

```json
{
  "ok": true,
  "upgrades": [
    {
      "package": "team-lead",
      "scope": "project",
      "from_version": "1.2.0",
      "to_version": "1.3.0",
      "from_branch": "main",
      "to_branch": "main",
      "install_root": "/repo/.claude/skills/team-lead",
      "files_written": 4,
      "warnings": [],
      "dependency_warnings": []
    }
  ]
}
```

### `sc status --json`

```json
{
  "ok": true,
  "packages": [
    {
      "package": "team-lead",
      "global": {
        "scope": "global",
        "branch": "main",
        "version": "1.4.0",
        "install_site": "/Users/randlee",
        "install_root": "/Users/randlee/.claude/skills/team-lead",
        "validation": "PASS",
        "aggregate_status": "info",
        "dependency_summary": {
          "tracked": 2,
          "verified": 2,
          "missing": 0,
          "items": [
            {
              "name": "gh>=2",
              "dependency_type": "tool",
              "verified": true,
              "provenance": "verified-at-install"
            }
          ]
        },
        "hook_summary": {
          "tracked": 1,
          "registered": 1,
          "missing": 0,
          "hooks": [
            {
              "hook_event": "PreToolUse",
              "hook_matcher": "git commit",
              "hook_script": "hooks/pre-commit.sh",
              "scope": "global",
              "registered": true
            }
          ]
        },
        "modification_summary": {
          "ok": 4,
          "modified": 0,
          "missing": 0,
          "unreadable": 0,
          "extra": 0
        }
      },
      "local": {
        "scope": "project",
        "branch": "main",
        "version": "1.3.0",
        "install_root": "/repo/.claude/skills/team-lead",
        "install_site": "/repo",
        "validation": "FAIL",
        "aggregate_status": "error",
        "modification_summary": {
          "ok": 3,
          "modified": 1,
          "missing": 0,
          "unreadable": 0,
          "extra": 1
        },
        "modifications": [
          {
            "kind": "file",
            "severity": "warn",
            "state": "modified",
            "path": "SKILL.md"
          },
          {
            "kind": "file",
            "severity": "info",
            "state": "extra",
            "path": "NOTES.md"
          }
        ],
        "issues": [
        {
          "kind": "hook",
          "severity": "warn",
          "state": "missing",
          "code": "hook_not_registered",
          "hook_event": "PreToolUse",
          "hook_script": "hooks/pre-commit.sh",
          "scope": "project",
          "message": "tracked hook script is not registered"
          }
        ]
      }
    }
  ]
}
```

### `sc validate --json`

```json
{
  "ok": true,
  "pass": false,
  "packages": [
    {
      "package": "team-lead",
      "version": "1.3.0",
      "branch": "main",
      "scope": "project",
      "install_root": "/repo/.claude/skills/team-lead",
      "install_site": "/repo",
      "tracking_origin": "local-install",
      "aggregate_expected": "abc123",
      "aggregate_actual": "def456",
      "aggregate_pass": false,
      "aggregate_status": "error",
      "dependency_summary": {
        "tracked": 2,
        "verified": 2,
        "missing": 0
      },
      "hook_summary": {
        "tracked": 1,
        "registered": 0,
        "missing": 1
      },
      "modification_summary": {
        "ok": 2,
        "modified": 1,
        "missing": 0,
        "unreadable": 0,
        "extra": 0
      },
      "status": "FAIL",
      "warnings": [],
      "items": [
        {
          "kind": "file",
          "state": "modified",
          "path": "SKILL.md",
          "severity": "warn"
        },
        {
          "kind": "hook",
          "state": "missing",
          "code": "hook_not_registered",
          "hook_event": "PreToolUse",
          "hook_script": "hooks/pre-commit.sh",
          "scope": "project",
          "message": "tracked hook script is not registered",
          "severity": "warn"
        },
        {
          "kind": "aggregate",
          "state": "modified",
          "code": "aggregate_mismatch",
          "expected": "abc123",
          "actual": "def456",
          "message": "aggregate SHA256 does not match tracked package state",
          "severity": "error"
        }
      ]
    }
  ]
}
```

`items` is a flat, typed validation list. File items use the fixed
file-state vocabulary only: `ok`, `modified`, `missing`, `unreadable`, and
`extra`. Higher-level problems carry a `kind`, severity, optional `code`, and
target-identification fields such as `dependency`, `hook_event`, or
`hook_script`.

The validation severity vocabulary is fixed and normative: `info`, `warn`,
`error`, and `critical`.

Representative dependency item:

```json
{
  "kind": "dependency",
  "state": "missing",
  "code": "dependency_provenance_missing",
  "dependency": "atm",
  "dependency_type": "cli",
  "severity": "critical",
  "message": "installed dependency provenance is missing"
}
```

### Mutation Outcome And Rollback Reporting

Mutating commands use atomic per-scope tracking updates. When a command fails
after side effects begin, JSON errors must still match the standard error
envelope. The emitted message must report the relevant recovery facts directly
rather than relying on extra rollback-only fields or external logs.

Representative uninstall file-removal failure:

```json
{
  "ok": false,
  "error": {
    "code": "conflict",
    "message": "conflict removing tracked files: SKILL.md; manifest record preserved"
  }
}
```

### File Mode Policy

Script and hook files (`file_type: script` or `hook`) are written with mode
`0755`. All other files are written with mode `0644`. Mode is set atomically
alongside the file content write. This behavior is stable and automation may
depend on it.

Other end-user commands such as `snapshot`, `scan`, `catalog update`,
`config get`, and `config set` follow the same top-level `ok` convention and
emit command-specific typed payloads from `src/pkg/api/`.

### `sc uninstall --json`

```json
{
  "ok": true,
  "removed": {
    "package": "team-lead",
    "scope": "project",
    "removed_files": [
      "/repo/.claude/skills/team-lead/SKILL.md"
    ],
    "removed_dependencies": [
      "gh"
    ],
    "hooks_removed": 1
  },
  "removed_all": [
    {
      "package": "team-lead",
      "scope": "project",
      "removed_files": [
        "/repo/.claude/skills/team-lead/SKILL.md"
      ],
      "removed_dependencies": [
        "gh"
      ],
      "hooks_removed": 1
    }
  ]
}
```

### `sc scan --json`

Successful scan:

```json
{
  "ok": true,
  "branch": "main",
  "mutated": false,
  "accepted": 0,
  "upgraded": 0,
  "candidates": [
    {
      "package": "team-lead",
      "version": "1.3.0",
      "branch": "main",
      "scope": "project",
      "install_root": "/repo/.claude/skills/team-lead",
      "install_site": "/repo",
      "tracking_origin": "scan-reconciled",
      "needs_upgrade": false,
      "files": [
        {
          "path": "/repo/.claude/skills/team-lead/SKILL.md",
          "doc_path": "SKILL.md",
          "sha256": "abc123"
        }
      ]
    }
  ],
  "warnings": []
}
```

Missing-catalog failure:

```json
{
  "ok": false,
  "error": {
    "code": "validation_failed",
    "message": "catalog not found for branch main at /repo/.synaptic/catalog-main.toml; run: sc catalog update",
    "details": {
      "required_action": "sc catalog update",
      "catalog_path": "/repo/.synaptic/catalog-main.toml"
    }
  }
}
```

### `sc catalog update --json`

```json
{
  "ok": true,
  "branch": "beta",
  "entries": 42,
  "path": "~/.synaptic/catalog-beta.toml",
  "paths": [
    "/repo/.synaptic/catalog-beta.toml",
    "~/.synaptic/catalog-beta.toml"
  ]
}
```

### `sc config get --json`

```json
{
  "ok": true,
  "key": "dolt.database",
  "value": "randlee/synaptic-canvas"
}
```

### `sc config set --json`

```json
{
  "ok": true,
  "key": "dolt.database",
  "path": "/Users/example/.sc/config.toml"
}
```

### `sc snapshot --json`

```json
{
  "ok": true,
  "package": "team-lead",
  "version": "1.3.0",
  "branch": "main",
  "scope": "project",
  "install_root": "/repo/.claude/skills/team-lead",
  "install_site": "/repo",
  "output_dir": "/Users/example/.synaptic/mod-snapshots/team-lead/main/repo/20260425T120000Z",
  "metadata_path": "/Users/example/.synaptic/mod-snapshots/team-lead/main/repo/20260425T120000Z/snapshot.toml",
  "files": [
    "SKILL.md",
    "README.md"
  ]
}
```
---

## Integrity Model

### SHA256 Hierarchy

```
Package SHA (aggregate)
├── file1.md  → SHA256(content)
├── file2.py  → SHA256(content)
├── file3.md  → SHA256(content)
└── ...
```

**Per-file SHA256:** Computed over the raw file content bytes at ingest time. Stored in `package_files.sha256`. Verified on export and install.

**Package-level SHA256:** Deterministic aggregate hash computed over sorted `(doc_path, sha256)` pairs. Stored in `packages.sha256` (column to be added). Provides a single value for quick "has anything changed?" checks. `doc_path` is the package-root-relative artifact path, not the materialized install path under `.claude/`, `~/.claude/`, `.agents/`, or another target root.

```
package_sha = SHA256(
    sorted([f"{doc_path}:{sha256}" for each file])
    joined by newline
)
```

This is a Merkle-like construction — changing any file changes the package SHA.

### When SHAs Are Computed

| Event | Per-file SHA | Package SHA | Action |
|-------|-------------|-------------|--------|
| `sc admin import` | Computed from source files | Computed from all file SHAs | Both stored in Dolt |
| `sc install` | Verified against Dolt | Verified against Dolt | Fail on mismatch |
| `sc validate` | Recomputed from installed files | Recomputed from file SHAs | Report drift |
| `sc admin verify` | Recomputed from DB content | Recomputed from file SHAs | Report corruption |
| `sc admin export` | Verified on write | Verified after export | Fail on mismatch |

### Validation Scenarios

Per-file validation states:
- `ok` — file exists and SHA matches
- `modified` — file exists but SHA does not match
- `missing` — file does not exist
- `unreadable` — file exists but cannot be read (permission denied or I/O error); `Err` field contains the underlying cause
- `extra` — file exists on disk but has no entry in the package

These file states are the complete file-level vocabulary. Higher-level problems
such as aggregate integrity mismatch, dependency incompatibility, hook drift,
or template drift are separate validation items with explicit severities rather
than additional file-state names.

**`sc validate <package>`** (end-user):
```
For each installed file:
  local_sha = SHA256(read file from disk)
  expected_sha = read package_files.sha256 from local catalog cache
                 (.synaptic/catalog-{branch}.toml or ~/.synaptic/catalog-{branch}.toml)
  Compare → ok | modified | missing | unreadable

For extra files in the installed package's managed target paths that are not tracked
by the installed package file inventory:
  Report → extra (untracked)

Compute aggregate from local file SHAs:
  Compare against catalog aggregate/package SHA when available → PASS | FAIL

Verify tracked dependency and component state:
  required_tools present and version-compatible → PASS | FAIL
  required_clis present and version-compatible → PASS | FAIL
  hook registrations and template-validation state consistent with lockfile → PASS | FAIL

Inventory local modifications:
  Record modified tracked files separately from corrupt or missing files
  Optionally export modification snapshots into product-managed staging under
  global state for comparison across versions
```

**`sc admin verify <package>`** (admin):
```
For each file in package_files:
  stored_sha = package_files.sha256
  recomputed_sha = SHA256(package_files.content)
  Compare → OK | CORRUPT

Recompute aggregate from stored file SHAs:
  Compare against packages.sha256 → PASS | FAIL
```

---

## Security Considerations

### Current (MVP)

- **SHA256 per file** — tamper detection for installed files
- **SHA256 per package** — quick integrity check
- **Dolt commit history** — full audit trail of every package change
- **Branch isolation** — develop/beta/main are separate database states

### Future: Package Signing

Two complementary approaches:

**Option A — Package-level signing:**
- Sign the aggregate package SHA256 with a private key
- Public key ships with `sc` binary (or fetched from DoltHub)
- On install: verify signature over package SHA → proves trusted publisher
- Supports multiple signers (author + reviewer)

**Option B — Dolt commit signing:**
- Dolt supports GPG-signed commits natively
- Every `sc admin import` creates a signed commit
- Signature covers entire database state at that point
- Provides auditable, signed history for free

Both build on the SHA foundation. Option A is package-granular, Option B is database-granular. They're not mutually exclusive.

### Future: Security Scanning

The per-file content storage in Dolt enables automated scanning:
- Pattern matching for known-bad content (exfiltration, injection)
- Template variable validation — **implemented** as a three-point check (dry-run, pre-publish gate, post-install). See [Install System](./synaptic-canvas-install-system.md#template-variable-validation)
- Permission analysis (what hooks/scripts request)
- Dependency chain verification

None of this is MVP, but the schema supports it without changes.

---

## Project Structure

Following the `claude-history` conventions:

```
synaptic-canvas-dolt/
├── src/                          # Go source root
│   ├── main.go                   # Entry point, version injection
│   ├── go.mod                    # Module: github.com/randlee/synaptic-canvas-dolt
│   ├── cmd/                      # Cobra commands
│   │   ├── root.go               # Root command, global flags
│   │   ├── list.go               # sc list
│   │   ├── info.go               # sc info
│   │   ├── install.go            # sc install
│   │   ├── scan.go               # sc scan
│   │   ├── catalog.go            # sc catalog
│   │   ├── configcmd.go          # sc config get/set
│   │   ├── snapshot.go           # sc snapshot
│   │   ├── upgrade.go            # sc upgrade
│   │   ├── uninstall.go          # sc uninstall
│   │   ├── validate.go           # sc validate
│   │   ├── status.go             # sc status
│   │   └── admin/                # Admin subcommands
│   │       ├── admin.go          # sc admin (parent)
│   │       ├── import.go         # sc admin import
│   │       ├── export.go         # sc admin export
│   │       ├── publish.go        # sc admin publish
│   │       ├── verify.go         # sc admin verify
│   │       └── diff.go           # sc admin diff
│   ├── pkg/                      # Public packages
│   │   ├── api/                  # Shared JSON DTOs + typed error models
│   │   ├── operations/           # Shared end-user workflow/use-case layer
│   │   ├── dolt/                 # Dolt database client
│   │   │   ├── http_client.go    # HTTPClient — DoltHub REST API
│   │   │   ├── client.go         # SQLClient + shared client interface
│   │   │   ├── cli_reader.go     # CLIReader — local dolt clone adapter
│   │   │   ├── errors.go         # Sentinel errors (ErrNotFound, ErrUnauthorized, etc.)
│   │   ├── integrity/            # SHA computation and verification
│   │   ├── manifest/             # manifest.yaml reconstruction
│   │   ├── plugin/               # plugin.json reconstruction
│   │   ├── catalog/              # SHA catalog: TOML cache, writeTOMLAtomic
│   │   ├── installer/            # File installation logic
│   │   ├── questionnaire/        # Install/upgrade question prompting + answer tracking
│   │   ├── repo/                 # Repo detection/profile generation and scan helpers
│   │   ├── template/             # Template validation/render helpers
│   │   └── models/               # Domain data structures when DTO-specific types are not needed
│   └── internal/                 # Private implementation
│       ├── config/               # CLI configuration
│       │   ├── config.go         # Config struct, NewConfigFromFlags, EffectiveBranch
│       │   ├── fileconfig.go     # Layered file config (~/.sc/config.toml)
│       │   └── keys.go           # Config key constants (KeyDoltClient, etc.)
│       ├── logging/              # Structured logger setup
│       └── output/               # Output formatters (table, JSON)
├── sql/                          # DDL scripts
│   └── 001-create-tables.sql
├── docs/                         # Design documents
├── scripts/                      # Utility scripts
├── tools/                        # Prototype scripts (Python)
├── test/                         # Test fixtures
├── .github/
│   └── workflows/
│       ├── test.yml              # CI: lint + test + build
│       └── release.yml           # Tag-triggered GoReleaser
├── .goreleaser.yml               # Build configuration
├── .golangci.yml                 # Linter configuration
└── CLAUDE.md                     # Project developer instructions
```

---

## Build & Release

### GoReleaser

Following `claude-history` patterns:

- **Source directory:** `./src`
- **Binary name:** `sc`
- **Targets:** linux/darwin (amd64, arm64), windows (amd64)
- **CGO_ENABLED=0** (static binaries)
- **Ldflags:** version, commit, date injection
- **Homebrew:** `randlee/homebrew-tap` → `Formula/sc.rb`
- **Winget:** deferred pending publisher registration (gh issue `#54`)
- **Checksums:** SHA256

### CI Workflows

**test.yml** (PR and push to main/develop):
- golangci-lint with gosec
- `go test ./... -v -race` on ubuntu/macOS/windows matrix
- Build verification
- Coverage to Codecov

**release.yml** (tag push `v*`):
- Full test suite
- GoReleaser build + publish
- Homebrew tap update
- GitHub release archives, checksums, and generated notes

**release-validate.yml** (PR/push):
- `goreleaser check`
- Non-publishing validation of the release configuration before tags are cut

Winget publication is intentionally out of MVP release scope until publisher
registration is complete (tracked in gh issue `#54`).

---

## Skill Integration

### `sc:plugin` Skill

A Claude Code skill installed globally by the `sc` installer. Replaces the current `sc-manage` skill.

**Invocation:** `/sc:plugin` or natural language ("install the delay package")

**Commands mapped to CLI:**
```
"list packages"        → sc list --json
"show team-lead"       → sc info team-lead --json
"install <pkg>"        → sc install <pkg> --json
"install <pkg> from <branch> globally"
                       → sc install <pkg> --branch <branch> --scope global --json
"upgrade <pkg>"        → sc upgrade <pkg> --json
"upgrade <pkg> to <version> on <branch> in this repo"
                       → sc upgrade <pkg> --branch <branch> --version <version> --scope project --json
"uninstall <pkg>"      → sc uninstall <pkg> --json
"validate <pkg>"       → sc validate <pkg> --json
"show status"          → sc status --json
"snapshot <pkg>"       → sc snapshot <pkg> --json
```

The skill parses `--json` output from the CLI and presents it conversationally.
The skill itself is a thin markdown file with tool definitions — all package
logic lives in the `sc` binary. ATM message polling, autonomous task loops, and
repository orchestration remain out of scope for this skill and belong to
higher-level orchestration layers.

The authoritative in-repo skill package lives under
`.claude/skills/sc-plugin/`. Its `examples/` directory is the manual-QA
fixture set for utterance-to-command mapping and expected structured outcomes.

### Admin Skill (separate, opt-in)

For package authors who want Claude to help with admin operations:

```
"import this package"  → sc admin import . --branch develop --json
"publish to beta"      → sc admin publish <pkg> --from develop --to beta --json
```

Not installed by default. Available via `sc install sc-admin-skill --scope global`.

---

## Schema Additions

The following columns need to be added to support the CLI:

### `packages` table

```sql
ALTER TABLE packages ADD COLUMN sha256 VARCHAR(64) AFTER options;
```

Stores the aggregate package SHA256 computed from sorted file SHAs.

### Future: `packages` table (signing)

```sql
-- Not MVP, but reserved for future use
ALTER TABLE packages ADD COLUMN signature TEXT AFTER sha256;
ALTER TABLE packages ADD COLUMN signed_by VARCHAR(256) AFTER signature;
```

---

## Resolved MVP Decisions

1. **Read backends:** MVP read-path commands support `http`, `sql`, and `cli`
   client modes behind one stable `sc --json` contract.
2. **Write backends:** MVP admin/write-path commands use `sql` or `cli` only.
   `http` is read-only and rejected for write-path commands with
   `unsupported_backend`.
3. **Channel defaults:** Read-path commands resolve branches using `--branch`,
   then `SC_DOLT_BRANCH`, then `main`. The CLI ignores the current Dolt
   session branch.
4. **Dependency resolution:** `sc install` computes a dependency plan, shows
   external installs for approval, and may install them when approved or when
   `--yolo` is used. Dependency provenance is always recorded.
5. **Template expansion:** `sc` handles Jinja2 rendering at install time.
   Templates are validated at dry-run, pre-publish, and post-install stages.
6. **Upgrade strategy:** `sc upgrade` warns and skips blocked upgrades by
   default and only allows force as a targeted override.

---

## Document History

| Date | Change |
|------|--------|
| 2026-02-22 | Initial design document |
| 2026-02-22 | Add template variable validation to admin publish (blocking gate) and import (warning) |
| 2026-05-15 | Sprint 4.1 hardening: shared JSON contract, typed error coverage, warning formatter, and command JSON shape examples |
| 2026-05-15 | Sprint 4.3 readback/audit symmetry: richer status and validate JSON, typed validation items, snapshot metadata path, and scan recovery contract |
