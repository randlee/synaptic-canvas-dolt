# Synaptic Canvas — Hook System

## Overview

The current Claude Code hook mechanism has three problems that make skill-based hook registration impractical:

1. **`settings.json` is shared mutable state** — skills cannot safely add or remove hooks without risking conflicts or corrupting settings for other tools
2. **No composability** — multiple skills hooking the same event have no ordering, coordination, or isolation guarantees
3. **`claude-project-root` asymmetry** — hook context provides this variable, but bash tools spawned from skills do not receive it, creating two different mental models for path resolution

The Synaptic Canvas hook system solves all three with a **dispatcher architecture**: a single permanent entry in `settings.json` delegates to a registry file that skills manage independently.

---

## Core Architecture

```
settings.json
  └── one permanent dispatcher hook (never changes after install)
        └── .synaptic/hooks/dispatch
              └── reads .synaptic/hooks/registry.toml
                    ├── skill-a/hooks/pre-bash.sh       (priority 10)
                    ├── skill-b/hooks/pre-bash.sh       (priority 20)
                    └── skill-c/hooks/post-fs-audit.sh  (priority 50)
```

Installing or removing a skill only modifies `registry.toml`. `settings.json` is touched exactly once — at initial Synaptic Canvas bootstrap — and never again.

---

## `settings.json` — One-Time Registration

During initial bootstrap, the installer adds a single dispatcher entry per hook event class. This is idempotent: running bootstrap on a new machine either adds the entry or finds it already present.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "~/.synaptic/bin/dispatch PreToolUse"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "~/.synaptic/bin/dispatch PostToolUse"
          }
        ]
      }
    ]
  }
}
```

The dispatcher binary (`dispatch`) is installed globally at `~/.synaptic/bin/` so it is available regardless of working directory. It reads the project-local registry at runtime.

---

## Hook Declaration in Packages

Hooks are declared in the Dolt package definition alongside files and dependencies. The installer materializes hook scripts and registers them; the skill author never touches `settings.json` or the registry directly.

```toml
# Package definition in Dolt (package_hooks table)

[[hooks]]
event = "PreToolUse"
matcher = "Bash"
script = "hooks/pre-bash-inject-env.sh"
priority = 10
blocking = true     # can return decision (approve/block/modify)

[[hooks]]
event = "PostToolUse"
matcher = "mcp__filesystem__.*"
script = "hooks/post-fs-audit.sh"
priority = 50
blocking = false    # fire-and-forget
```

At install time the installer:

1. Materializes hook scripts to `.synaptic/skills/{skill-name}/hooks/` using absolute paths resolved from `env.toml`
2. Appends entries to `.synaptic/hooks/registry.toml`
3. Marks hook scripts executable

At removal time the installer removes the skill's entries from the registry. No `settings.json` surgery at any point.

---

## Hook Registry (`registry.toml`)

The registry is the dispatcher's source of truth. It is written by the installer and read at runtime by the dispatcher.

```toml
# .synaptic/hooks/registry.toml

[[hook]]
event = "PreToolUse"
matcher = "Bash"
skill = "env-injector"
script = "/Users/rand/projects/myproject/.synaptic/skills/env-injector/hooks/pre-bash.sh"
priority = 10
blocking = true

[[hook]]
event = "PreToolUse"
matcher = "Bash"
skill = "policy-guard"
script = "/Users/rand/projects/myproject/.synaptic/skills/policy-guard/hooks/pre-bash.sh"
priority = 20
blocking = true

[[hook]]
event = "PostToolUse"
matcher = "mcp__filesystem__.*"
skill = "audit-logger"
script = "/Users/rand/projects/myproject/.synaptic/skills/audit-logger/hooks/post-fs.sh"
priority = 50
blocking = false
```

All paths are absolute. The dispatcher does not need to know the project root — it reads paths directly from the registry.

---

## The Dispatcher

The dispatcher (`~/.synaptic/bin/dispatch`) is invoked by Claude Code for every matching hook event. It:

1. Receives the Claude Code hook context on `stdin` (JSON)
2. Extracts `claude-project-root` from the context
3. Locates `{project-root}/.synaptic/hooks/registry.toml`
4. Filters registered hooks by event and matcher
5. Sorts by priority (ascending — lower number runs first)
6. Injects environment variables before each hook script invocation
7. Collects and forwards any blocking decisions

### Environment Injection

Before invoking each hook script, the dispatcher sets:

```bash
SYNAPTIC_PROJECT_ROOT="/Users/rand/projects/myproject"
SYNAPTIC_ROOT="/Users/rand/projects/myproject/.synaptic"
SYNAPTIC_SHARED="/Users/rand/projects/myproject/.synaptic/shared"
SYNAPTIC_SKILLS="/Users/rand/projects/myproject/.synaptic/skills"
SYNAPTIC_CHANNEL="develop"
SYNAPTIC_AGENTS="claude"

# Hook-specific context
HOOK_EVENT="PreToolUse"
HOOK_TOOL="Bash"
HOOK_INPUT='{"command": "ls -la"}'   # serialized tool input
```

This is the same set of variables available via `.synaptic/env.toml`. Hook scripts and bash tool scripts now share an identical path resolution model — the asymmetry is gone.

### Blocking vs Non-Blocking Execution

**Non-blocking hooks** (`blocking = false`) run as fire-and-forget. The dispatcher launches them, does not wait for exit, and forwards no decision to Claude Code.

**Blocking hooks** (`blocking = true`) run sequentially in priority order. Each hook can emit a JSON decision to `stdout`:

```json
{ "action": "approve" }
{ "action": "block", "reason": "Banned command pattern detected." }
{ "action": "modify", "input": { "command": "ls -la --color=never" } }
```

The dispatcher short-circuits on the first `block` decision and returns it to Claude Code without running lower-priority hooks. If no hook blocks, the final aggregated decision (or implicit approval) is returned.

---

## Solving the `claude-project-root` Asymmetry

The asymmetry existed because:

- Hook scripts receive `claude-project-root` via Claude Code's hook context
- Bash tools spawned by skills have no such context — they only know their working directory, which Claude may have changed

The dispatcher eliminates this gap. Because the dispatcher runs in hook context (where `claude-project-root` is available), it can inject `SYNAPTIC_PROJECT_ROOT` into every hook script's environment. Skills that also use bash tools source `.synaptic/env.toml` at script start, which contains the same value burned in at install time.

Both paths arrive at the same variable with the same value:

```bash
# In a hook script — set by dispatcher at runtime
echo $SYNAPTIC_PROJECT_ROOT

# In a bash tool script — sourced from env.toml at script start
source "$HOME/.synaptic/env.toml"   # or project-local if known
echo $SYNAPTIC_PROJECT_ROOT
```

Skills always have a reliable, absolute project root reference regardless of how they were invoked.

---

## Hook Script Conventions

Hook scripts should follow a minimal convention to stay composable:

```bash
#!/usr/bin/env bash
# Skills should source env for consistent path access
# SYNAPTIC_PROJECT_ROOT is already set by the dispatcher

set -euo pipefail

# Read tool input from environment
INPUT="${HOOK_INPUT:-}"

# Do work using absolute paths
LOG_FILE="$SYNAPTIC_ROOT/logs/audit.log"
echo "$(date -u +%FT%TZ) [$HOOK_TOOL] $INPUT" >> "$LOG_FILE"

# For blocking hooks: emit decision or nothing (implicit approve)
# echo '{"action": "approve"}'
```

Non-blocking hooks can exit with any code — the dispatcher ignores it. Blocking hooks should exit 0 and emit a JSON decision, or exit non-zero to signal an unexpected error (treated as approve-with-warning to avoid over-blocking).

---

## Priority and Ordering

Priority is an integer where lower values run first. Conventions:

| Range | Purpose |
|-------|---------|
| 1–19 | Environment setup, variable injection |
| 20–39 | Policy enforcement, blocking guards |
| 40–59 | Logging, auditing |
| 60–79 | Notification, telemetry |
| 80+ | Post-processing, cleanup |

Package authors declare a priority in their hook definition. Users can override per-installation by editing the registry directly — the installer preserves manual edits when upgrading, only adding or removing the entry for the upgraded skill.

---

## Install and Removal Flow

### On `skill install my-skill`

```
1. Materialize hook scripts from Dolt blobs to:
     .synaptic/skills/my-skill/hooks/*.sh
2. chmod +x each hook script
3. Append entries to .synaptic/hooks/registry.toml
4. Verify dispatcher is registered in settings.json (bootstrap if not)
```

### On `skill remove my-skill`

```
1. Remove .synaptic/skills/my-skill/hooks/ directory
2. Remove my-skill's entries from registry.toml
3. Leave settings.json untouched
```

The dispatcher gracefully skips missing scripts (logs a warning, continues). This means a partially-failed removal never breaks the hook system for other installed skills.

---

## Bootstrap (First Install)

The Synaptic Canvas bootstrap sequence runs once per machine and is idempotent:

```
1. Install dispatcher binary to ~/.synaptic/bin/dispatch
2. Add dispatcher entries to settings.json for PreToolUse and PostToolUse
   (check for existence first — skip if already present)
3. Create ~/.synaptic/bin/ if absent
4. Add ~/.synaptic/bin to PATH in shell profile if absent
```

Bootstrap is triggered automatically on the first `skill install` command, or can be run explicitly with `/skill bootstrap`.

---

## What This Unlocks

Skills can now do things that were previously impossible without manual settings.json coordination:

- **Auto-inject environment variables** before every bash tool invocation (env-injector skill)
- **Enforce path policies** — block bash commands that reference paths outside the project (policy-guard skill)
- **Audit file system operations** performed via MCP filesystem tools (audit-logger skill)
- **Pre-flight checks** before expensive tool calls — verify API keys present, confirm network available
- **Automatic context priming** — inject project-specific context into every tool call silently

All of this composes cleanly. Multiple skills hook the same events, run in priority order, and have no awareness of each other. Adding or removing a skill never affects others.
