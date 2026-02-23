# Synaptic Canvas — Dolt-Backed Skills Marketplace

## Overview

Synaptic Canvas uses Dolt as its backend database for skill storage and distribution. Dolt's Git-like branching model provides a natural release pipeline, package-level dependency management, and reproducible installs across machines. The system is designed around **local-first, project-scoped installation** with a lockfile for reproducibility and an environment-variable-driven channel selector for branch targeting.

---

## Release Channels

A single environment variable controls which Dolt branch the resolver targets at startup:

```bash
SYNAPTIC_CHANNEL=develop   # develop | beta | main
```

| Channel | Branch | Purpose |
|---------|--------|---------|
| `main` | `main` | Stable, fully tested skills |
| `beta` | `beta` | Validated but newly promoted |
| `develop` | `develop` | Active development and backlog |

Skills are promoted by merging branches in Dolt. No separate release tooling is required. This directly addresses the backlog problem: new skills land in `develop` immediately and can be tested in real projects without polluting `main`.

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

One row per file in the package. Files are stored as blobs with their destination path and hash baked in.

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

The lockfile records the precise resolved state of every installed package. It is a reproducibility artifact, not a file bundle — the actual files are materialized locally from Dolt blobs.

```toml
[metadata]
channel = "develop"
dolt_remote = "https://doltdb.example.com/rand/synaptic-canvas"
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

## Install Flow: `/skill install claude-history`

```
1. Resolve 'claude-history' against SYNAPTIC_CHANNEL in Dolt
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

The `/skills` slash command provides an interactive interface to the marketplace. It offers two rendering modes:

**Fallback — Rich markdown output** when no TUI binary is present. Renders a status table with channel, version hash, variant, and requirement health for each installed skill.

**Primary — External TUI via `textual`** (Python). The command spawns a `textual` app in a terminal split. Features:

- Arrow-key navigation across installed skills
- Install / remove / upgrade actions per skill
- Channel and variant display per entry
- Requirement status badges (tool present, CLI installed, agent match)
- Inline Dolt commit hash and last-updated timestamp

The command checks for the TUI binary on first run and falls back gracefully. The TUI itself is a Synaptic Canvas package — it installs via the same mechanism as any other skill.

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
