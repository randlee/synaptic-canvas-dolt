# Synaptic Canvas — Dolt-Backed Skills Platform

## Overview

Synaptic Canvas uses Dolt as the **centralized authoring and management backbone** for skill storage, dependency management, and distribution. Dolt is server-side infrastructure — users never run Dolt locally. The system provides multiple distribution paths: direct pull for power users, Claude Code marketplace export, and lightweight local snapshots.

The core insight driving this architecture: **skills cannot be effectively tested with traditional automated tests**. Quality assurance is therefore promotion-based — skills advance through staged confidence branches with queryable dependency impact and fast rollback — rather than CI-gated.

---

## Architecture: Server / Client Boundary

```
┌─────────────────────────────────────────────────────────────┐
│  Dolt Server (DoltHub or self-hosted)                       │
│  ─────────────────────────────────────────                  │
│  Single source of truth. All authoring, promotion,          │
│  dependency management, and variant tracking happens here.  │
│                                                             │
│  Branches:  develop → beta → main                           │
│  Tables:    packages, package_files, package_deps,          │
│             package_variants, package_hooks                  │
└──────────────────┬──────────────────────────────────────────┘
                   │
        ┌──────────┼──────────────┐
        ▼          ▼              ▼
   ┌─────────┐ ┌──────────┐ ┌────────────────┐
   │  Direct  │ │  Claude  │ │  Lightweight   │
   │  Pull    │ │  Code    │ │  Snapshot      │
   │  (CLI)   │ │  Market  │ │  (SQLite/JSON) │
   └─────────┘ └──────────┘ └────────────────┘
```

**Dolt is the server, not a client-side dependency.** Users interact via:
- **`synaptic` CLI** — thin client that speaks to Dolt remote or REST API
- **Claude Code marketplace** — read-only projection exported from a Dolt branch
- **Filtered snapshots** — lightweight local format for offline/CI use

---

## Why Dolt (Not Git + Files)

### Dependency management is inherently relational

```sql
-- What breaks if I upgrade claude-history?
SELECT p.name, d.version_constraint
FROM package_deps d JOIN packages p ON d.package_id = p.id
WHERE d.depends_on = 'claude-history';

-- Everything on beta that needs python3
SELECT name, version FROM packages
WHERE channel = 'beta' AND install_cmd LIKE '%python3%';
```

Git + JSON can't do this without building a query layer — at which point you're reinventing a database poorly. The dependency graph is relational; as the catalog grows, flat-file approaches collapse.

### Script reuse across packages

A single script can be referenced by multiple packages without symlinks, submodules, or copy-paste:

```sql
-- Same script used by two packages (many-to-many via shared hash)
package_files:
  package_id  |  dest_path           |  file_hash
  3           |  scripts/git-sync.sh |  abc123...
  7           |  scripts/git-sync.sh |  abc123...
```

Update the script once; both packages reference the new version on next promotion.

### Centralized promotion with audit trail

Promotion is a merge on the server. `dolt log` gives you who published what, when. Conflict resolution is an admin problem, not a user problem. Rollback is `dolt_reset('--hard', 'HEAD~1')`.

### Quality assurance without automated tests

Skills modify agent behavior in context-dependent ways — no unit test can validate this. Dolt's branch model provides staged human confidence:

| Branch | Meaning |
|--------|---------|
| `develop` | "I wrote this, it seems to work" |
| `beta` | "I've used this across several projects" |
| `main` | "This is proven, ship it" |

Promotion records a human judgment as a merge. The dependency graph is queryable before promotion to assess blast radius.

---

## Release Channels

A single environment variable controls which Dolt branch the resolver targets:

```bash
SYNAPTIC_CHANNEL=develop   # develop | beta | main
```

| Channel | Branch | Purpose |
|---------|--------|---------|
| `main` | `main` | Stable, proven skills |
| `beta` | `beta` | Validated but newly promoted |
| `develop` | `develop` | Active development and backlog |

Skills are promoted by merging branches on the Dolt server. No separate release tooling is required.

---

## Database Schema

### `packages`

The top-level unit of installation. One package may contain multiple files and declare multiple dependencies.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `varchar` | Unique package identifier |
| `name` | `varchar` | Human-readable name |
| `version` | `varchar` | Semver string |
| `channel` | `varchar` | Source channel (`main`, `beta`, `develop`) |
| `description` | `text` | Package description |
| `agent_variant` | `varchar` | `claude`, `codex`, or `codex+claude` |

### `package_files`

One row per file in the package. Files are stored as blobs with their destination path and hash.

| Column | Type | Description |
|--------|------|-------------|
| `package_id` | `varchar` | FK to `packages` |
| `dest_path` | `varchar` | Relative destination path (e.g., `skills/my-skill/main.md`) |
| `blob` | `longblob` | File contents |
| `sha256` | `varchar` | SHA-256 of blob for integrity verification |
| `file_type` | `varchar` | `skill`, `script`, `hook`, or `config` |

### `package_deps`

Declares all dependencies a package requires. The installer walks this graph before materializing any files.

| Column | Type | Description |
|--------|------|-------------|
| `package_id` | `varchar` | FK to `packages` |
| `dep_type` | `varchar` | `tool`, `cli`, or `skill` |
| `dep_name` | `varchar` | Name of the dependency |
| `dep_spec` | `varchar` | Version spec (e.g., `>=3.11`) or empty |
| `install_cmd` | `varchar` | Command to install if absent (e.g., `cargo install agent-teams-mail`) |

### `package_variants`

Maps a logical package name to agent-profile-specific implementations.

| Column | Type | Description |
|--------|------|-------------|
| `package_id` | `varchar` | Logical package ID |
| `agent_profile` | `varchar` | `claude`, `codex`, or `codex+claude` |
| `variant_package_id` | `varchar` | Concrete package implementing this profile |

### `package_hooks`

Hook declarations for packages. See [Hook System](./synaptic-canvas-hook-system.md) for full architecture.

| Column | Type | Description |
|--------|------|-------------|
| `package_id` | `varchar` | FK to `packages` |
| `event` | `varchar` | `PreToolUse`, `PostToolUse`, etc. |
| `matcher` | `varchar` | Tool matcher pattern |
| `script_path` | `varchar` | Relative path to hook script within package |
| `priority` | `int` | Execution order (lower runs first) |
| `blocking` | `boolean` | Whether hook can return decisions |

---

## Dolt API

The Dolt server is accessed via DoltHub's hosted SQL interface or a self-hosted Dolt SQL server. The CLI communicates using standard MySQL wire protocol.

### Connection

```bash
# DoltHub hosted
DOLT_HOST=dolthub.com
DOLT_USER=randlee
DOLT_DB=synaptic-canvas

# Self-hosted
DOLT_HOST=dolt.example.com
DOLT_PORT=3306
DOLT_USER=readonly
DOLT_DB=synaptic-canvas
```

### Core Queries (CLI → Server)

**List available packages on a channel:**
```sql
SELECT id, name, version, description, agent_variant
FROM packages
WHERE channel = ?
ORDER BY name;
```

**Resolve a package with dependencies:**
```sql
SELECT p.id, p.name, p.version,
       d.dep_type, d.dep_name, d.dep_spec, d.install_cmd
FROM packages p
LEFT JOIN package_deps d ON p.id = d.package_id
WHERE p.id = ? AND p.channel = ?;
```

**Fetch files for a package:**
```sql
SELECT dest_path, blob, sha256, file_type
FROM package_files
WHERE package_id = ?;
```

**Check for upgrades (compare local lockfile versions):**
```sql
SELECT id, name, version
FROM packages
WHERE channel = ? AND id IN (?, ?, ?)
  AND version != ?;  -- compare against lockfile versions
```

**Dependency impact analysis (before promotion):**
```sql
SELECT p.name, p.channel, d.version_constraint
FROM package_deps d JOIN packages p ON d.package_id = p.id
WHERE d.depends_on = ?;
```

### Branch Operations (Admin Only)

```sql
-- Promote develop → beta
CALL dolt_checkout('beta');
CALL dolt_merge('develop');
CALL dolt_commit('-m', 'Promote develop to beta: claude-history v2.1');

-- Rollback last promotion
CALL dolt_checkout('main');
CALL dolt_reset('--hard', 'HEAD~1');

-- View promotion history
SELECT * FROM dolt_log ORDER BY date DESC LIMIT 20;
```

---

## Distribution Channels

### 1. Direct Pull (CLI)

The `synaptic` CLI queries Dolt directly, resolves dependencies, and materializes files locally. This is the primary workflow for skill authors and power users.

```bash
synaptic install claude-history    # resolve, fetch, materialize
synaptic upgrade                   # check all installed for updates
synaptic list --available          # browse channel catalog
```

### 2. Claude Code Marketplace Export

A Dolt branch (typically `main`) is exported as a Claude Code marketplace format:

```
Dolt main branch
    ↓ filtered SQL query
    ↓ export transform
Claude Code marketplace format (JSONL index + file blobs)
    ↓ publish
Claude Code package registry
```

The Dolt DB stays the authoring environment. The marketplace is a **read-only projection** — generated, not maintained. This gives one workflow for development and multiple distribution channels.

### 3. Lightweight Snapshot

For offline development or CI environments, a filtered branch can be exported as:
- **JSONL manifest + flat files** — human-readable, diffable
- **SQLite export** — queryable offline, useful for validation testing

```bash
synaptic snapshot --channel main --format files --output .synaptic/cache/
synaptic snapshot --channel main --format sqlite --output skills.db
```

The SQLite export is particularly useful as a **validation artifact**: compare SQLite snapshot against Dolt to confirm export correctness.

---

## Dependency Tiers

### Tier 1 — Hard Tool Requirements

Binary tools that must be present on `PATH` with version constraints satisfied. These are **blocking**: installation fails with a clear diagnostic if unmet.

```
python3>=3.11 required — found 3.9.2. Install a newer Python before proceeding.
```

### Tier 2 — Agent Capabilities

Soft requirements that drive variant selection rather than blocking install. The installer detects the agent environment and selects the best matching variant automatically.

```bash
SYNAPTIC_AGENTS=codex+claude   # claude | codex | codex+claude
```

A logical skill like `claude-history` might have three variants:

```
claude-history/
  variant: claude          # uses Claude Code SDK, MCP tools
  variant: codex           # uses codex CLI patterns, different prompts
  variant: codex+claude    # richer coordination between both agents
```

The installed variant is recorded in the lockfile. If `SYNAPTIC_AGENTS` changes, `skill upgrade` can swap variants automatically.

### Tier 3 — Skill Dependencies

A package can declare `dep_type = skill` to pull in another skill as a prerequisite. Shared utility packages (type `script`, no slash command) materialize to `.synaptic/shared/` and are referenced by all dependents via absolute paths.

---

## Local Install Layout

All installation is **project-scoped by default**. Nothing is installed globally unless explicitly requested.

```
{repo-root}/
  .synaptic/
    manifest.lock          # committed to git — source of truth
    env.toml               # generated at install time — gitignored
    skills/
      claude-history/      # materialized skill files
        main.md
        hooks/
          pre-bash.sh
    shared/                # shared script dependencies
      common-utils/
        helpers.sh
    hooks/
      registry.toml        # hook dispatcher registry
      dispatch              # dispatcher binary or script
```

`manifest.lock` is committed. All other contents of `.synaptic/` are gitignored and regenerated by the installer from the lockfile on a new machine.

---

## The Lockfile (`manifest.lock`)

The lockfile records the precise resolved state of every installed package. It is a reproducibility artifact — actual files are materialized locally from Dolt blobs.

```toml
[metadata]
channel = "develop"
dolt_remote = "dolthub/randlee/synaptic-canvas"
resolved_at = "2026-02-21T14:32:00Z"

[[skills]]
id = "claude-history"
logical_id = "claude-history"
variant = "claude"
version = "a3f9c21"                  # Dolt commit hash
channel = "develop"
installed_at = "2026-02-21T14:32:00Z"
install_scope = "project"

  [skills.files]
  "skills/claude-history/main.md" = "sha256:abc123..."
  "skills/claude-history/hooks/pre-bash.sh" = "sha256:def456..."

  [skills.requirements]
  tools = ["python3>=3.11"]
  agents = ["claude"]
  cli_installed = ["agent-teams-mail"]
  acknowledged_at = "2026-02-21T14:31:45Z"

[[skills]]
id = "common-utils"
dep_of = "claude-history"
install_scope = "shared"
# ...
```

On startup, the installer checksums materialized files against the lockfile. Drift (missing, modified, or version-mismatched files) triggers a re-install or a warning depending on `SYNAPTIC_STARTUP_MODE`.

---

## Generated Environment (`env.toml`)

Generated at install time from the resolved repo root. Gitignored. Regenerated on every install or upgrade.

```toml
# .synaptic/env.toml — machine-local, do not commit
SYNAPTIC_ROOT = "/Users/rand/projects/myproject/.synaptic"
SYNAPTIC_SHARED = "/Users/rand/projects/myproject/.synaptic/shared"
SYNAPTIC_SKILLS = "/Users/rand/projects/myproject/.synaptic/skills"
SYNAPTIC_PROJECT_ROOT = "/Users/rand/projects/myproject"
SYNAPTIC_CHANNEL = "develop"
SYNAPTIC_AGENTS = "claude"
```

All skill scripts source this file. Absolute paths eliminate the fragility of relative path resolution when Claude `cd`s during a session.

---

## CLI Commands

The `synaptic` CLI is a standalone binary (no skills dependency). It is the bootstrap entry point.

### Package Management

```bash
synaptic install <package>         # resolve, fetch, materialize
synaptic remove <package>          # remove files, update lockfile
synaptic upgrade [package]         # upgrade one or all packages
synaptic list                      # installed packages
synaptic list --available          # all packages on current channel
synaptic list --deps <package>     # dependency tree for a package
synaptic dry-run install <package> # show what files land where, no action
```

### Inspection

```bash
synaptic info <package>            # metadata, version, description
synaptic deps <package>            # full dependency graph
synaptic deps --reverse <package>  # what depends on this package
synaptic diff <package>            # changes between installed and latest
```

### Channel & Snapshot

```bash
synaptic channel                   # show current channel
synaptic channel set beta          # switch channel
synaptic snapshot --format files   # export to flat files
synaptic snapshot --format sqlite  # export to SQLite
```

### Marketplace Export

```bash
synaptic export marketplace        # export main branch → Claude Code format
```

---

## Install Flow: `synaptic install claude-history`

```
1. Query Dolt for 'claude-history' on SYNAPTIC_CHANNEL
2. Detect SYNAPTIC_AGENTS → select variant (e.g., claude)
3. Walk dependency graph:
     claude-history → common-utils (shared script)
                    → python3>=3.11 (tool)
                    → agent-teams-mail (cli, cargo install)
4. Present acknowledgement summary:
     Skills to install:  claude-history (claude variant), common-utils
     Tools required:     python3>=3.11 ✓ (found 3.11.4)
     CLIs to install:    agent-teams-mail  via: cargo install agent-teams-mail
     Hooks to register:  PreToolUse → pre-bash.sh
   [Proceed? y/N]
5. Run CLI installs in declared order
6. Materialize files from Dolt blobs to dest_path, verify sha256
7. Register hooks in .synaptic/hooks/registry.toml
8. Regenerate .synaptic/env.toml
9. Write lockfile entries for all resolved packages
10. Confirm: skill + CLI dropped and ready
```

User acknowledgement is recorded once per install with a timestamp. Re-acknowledgement is only required if requirements change on upgrade.

---

## Startup Upgrade Behavior

Controlled by environment variable:

```bash
SYNAPTIC_STARTUP_MODE=check   # check | auto | off
```

| Mode | Behavior |
|------|----------|
| `off` | No network activity on startup |
| `check` | Compare lockfile against Dolt HEAD, report drift, prompt to upgrade |
| `auto` | Pull and re-materialize automatically (recommended for `main`/`beta` only) |

The `develop` channel should default to `check` to avoid silent behavior changes mid-session. `main` and `beta` are safer candidates for `auto`.

---

## `/skills` Command — TUI

The `/skills` slash command provides an interactive interface. Two rendering modes:

**Fallback — Rich markdown output** when no TUI binary is present. Renders a status table with channel, version hash, variant, and requirement health for each installed skill.

**Primary — External TUI via `textual`** (Python). Features:

- Arrow-key navigation across installed skills
- Install / remove / upgrade actions per skill
- Channel and variant display per entry
- Requirement status badges (tool present, CLI installed, agent match)
- Dependency tree visualization
- Dry-run preview before install/upgrade
- Inline Dolt commit hash and last-updated timestamp

The TUI itself is a Synaptic Canvas package — it installs via the same mechanism as any other skill. The CLI (`synaptic`) provides the bootstrap path that doesn't depend on any installed skills.

---

## Key Environment Variables

| Variable | Values | Purpose |
|----------|--------|---------|
| `SYNAPTIC_CHANNEL` | `main`, `beta`, `develop` | Dolt branch to resolve against |
| `SYNAPTIC_AGENTS` | `claude`, `codex`, `codex+claude` | Agent capability profile for variant selection |
| `SYNAPTIC_STARTUP_MODE` | `off`, `check`, `auto` | Upgrade behavior on session start |
| `SYNAPTIC_ROOT` | absolute path | Generated; set in `env.toml` |
| `SYNAPTIC_SHARED` | absolute path | Generated; set in `env.toml` |
| `SYNAPTIC_PROJECT_ROOT` | absolute path | Generated; set in `env.toml` |
