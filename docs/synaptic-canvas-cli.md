# Synaptic Canvas — `sc` CLI Design

## Overview

`sc` is a Go CLI that serves as the primary interface for the Synaptic Canvas package management system. It provides end-user commands for installing and managing Claude Code skill packages, and admin commands for ingesting packages into the Dolt database.

**DoltHub:** https://www.dolthub.com/repositories/randlee/synaptic-canvas

### Related Documents

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

**Layer 1 — `sc` Go CLI:** The compiled binary. Handles all Dolt operations, file I/O, SHA computation, and validation. Distributed via GoReleaser (Homebrew, winget, `go install`).

**Layer 2 — `sc:plugin` skill:** A Claude Code skill that wraps the `sc` CLI. Allows Claude to manage packages conversationally ("install the delay package"). Thin wrapper — delegates all logic to the CLI.

**Layer 3 — Dolt Database:** Source of truth. Packages, files, dependencies, and metadata stored in relational tables. Branches (`develop`, `beta`, `main`) serve as release channels. Promotion is `dolt_merge`.

### Installer

The `sc` installer installs both:
1. The `sc` Go binary (to PATH)
2. The `sc:plugin` skill globally (to `~/.claude/`)

This means any repo immediately has both the CLI and Claude's ability to use it.

---

## Command Surface

### End-User Commands (default)

Available to all users. These commands interact with installed packages and the Dolt database as a consumer.

```
sc list [--channel <channel>] [--tags <tag,...>]
    List available packages. Defaults to main channel.

sc info <package>
    Show package details: version, description, dependencies, file count, SHA.

sc install <package> [--global] [--channel <channel>]
    Install a package from Dolt.
    --global    Install to ~/.claude/ (default: .claude/ in current repo)
    --channel   Install from specific channel (default: main)

sc upgrade <package> [--all]
    Upgrade installed package(s) to latest version on their channel.

sc uninstall <package>
    Remove an installed package.

sc validate [<package>] [--all]
    Verify installed files match Dolt SHA256 hashes.
    Reports: OK, MODIFIED (local edits), MISSING, EXTRA (untracked files).

sc status
    Show installed packages, their versions, channels, and validation state.
```

### Admin Commands (opt-in)

Not enabled by default. Intended for package authors and maintainers. These commands modify the Dolt database.

```
sc admin import <path> --branch <branch>
    Ingest a package directory into Dolt on the specified branch.
    Computes SHA256 per file and aggregate package SHA.
    Creates a Dolt commit with package metadata.
    Runs template variable validation on .j2 files (warning, non-blocking).

sc admin export <package> --output <dir> [--branch <branch>]
    Export a package from Dolt to filesystem.
    Reconstructs manifest.yaml and plugin.json from relational data.
    Verifies SHA on each exported file.

sc admin publish <package> --from <branch> --to <branch>
    Promote a package between channels (e.g., develop → beta → main).
    Executes a targeted dolt_merge for the package data.
    Runs template variable validation as a BLOCKING gate — publish
    fails if any .j2 template references undeclared variables.

sc admin verify <package> [--branch <branch>]
    Full integrity check within Dolt: recompute all SHA256 hashes
    from stored content and compare against stored hashes.

sc admin diff <package> --branch1 <b1> --branch2 <b2>
    Show differences between package versions across branches.
```

### Global Flags

```
--dolt-dir <path>     Path to Dolt database directory (default: auto-detect)
--remote <url>        DoltHub remote URL (for remote operations)
--json                Output as JSON (for scripting/skill integration)
--quiet               Suppress non-essential output
--verbose             Detailed output including SHA hashes
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

**Package-level SHA256:** Deterministic aggregate hash computed over sorted `(dest_path, sha256)` pairs. Stored in `packages.sha256` (column to be added). Provides a single value for quick "has anything changed?" checks.

```
package_sha = SHA256(
    sorted([f"{dest_path}:{sha256}" for each file])
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

**`sc validate <package>`** (end-user):
```
For each installed file:
  local_sha = SHA256(read file from disk)
  expected_sha = query package_files.sha256 from Dolt
  Compare → OK | MODIFIED | MISSING

For extra files in package directory not in Dolt:
  Report → EXTRA (untracked)

Compute aggregate from local file SHAs:
  Compare against packages.sha256 → PASS | FAIL
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
│   ├── go.mod                    # Module: github.com/randlee/synaptic-canvas
│   ├── cmd/                      # Cobra commands
│   │   ├── root.go               # Root command, global flags
│   │   ├── list.go               # sc list
│   │   ├── info.go               # sc info
│   │   ├── install.go            # sc install
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
│   │   ├── dolt/                 # Dolt database client
│   │   ├── integrity/            # SHA computation and verification
│   │   ├── manifest/             # manifest.yaml reconstruction
│   │   ├── plugin/               # plugin.json reconstruction
│   │   ├── installer/            # File installation logic
│   │   └── models/               # Data structures (Package, File, Dep)
│   └── internal/                 # Private implementation
│       ├── config/               # CLI configuration
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
- **Winget:** `randlee.sc`
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
- Winget manifest update

---

## Skill Integration

### `sc:plugin` Skill

A Claude Code skill installed globally by the `sc` installer. Replaces the current `sc-manage` skill.

**Invocation:** `/sc:plugin` or natural language ("install the delay package")

**Commands mapped to CLI:**
```
"list packages"        → sc list --json
"install <pkg>"        → sc install <pkg> --json
"upgrade <pkg>"        → sc upgrade <pkg> --json
"uninstall <pkg>"      → sc uninstall <pkg> --json
"validate <pkg>"       → sc validate <pkg> --json
"show status"          → sc status --json
```

The skill parses `--json` output from the CLI and presents it conversationally. The skill itself is a thin markdown file with tool definitions — all logic lives in the `sc` binary.

### Admin Skill (separate, opt-in)

For package authors who want Claude to help with admin operations:

```
"import this package"  → sc admin import . --branch develop --json
"publish to beta"      → sc admin publish <pkg> --from develop --to beta --json
```

Not installed by default. Available via `sc install sc-admin-skill --global`.

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

## Open Questions

1. **Remote vs local Dolt:** MVP uses local Dolt database. When does DoltHub remote come into play? Read-only pull for end users? Push for admins?

2. **Channel defaults:** Should `sc install` default to `main` channel, or should users configure a preferred channel?

3. **Dependency resolution:** When installing a package with dependencies, should `sc` auto-install deps? Or just warn?

4. **Template expansion:** ~~Resolved.~~ `sc` handles Jinja2 rendering at install time. Templates are validated at three points: dry-run (preview), pre-publish (blocking gate), and post-install (rendered output scan). See [Install System — Template Variable Validation](./synaptic-canvas-install-system.md#template-variable-validation).

5. **Upgrade strategy:** On `sc upgrade`, what happens to local modifications? Warn and skip? Force overwrite? Stash?

6. **Admin authentication:** How does `sc admin import` authenticate to write to Dolt? Local-only for MVP, DoltHub credentials later?

---

## Document History

| Date | Change |
|------|--------|
| 2026-02-22 | Initial design document |
| 2026-02-22 | Add template variable validation to admin publish (blocking gate) and import (warning) |
