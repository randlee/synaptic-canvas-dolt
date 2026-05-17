---
name: importing-sc-packages
description: Convert existing Claude Code skills, agents, commands, scripts, and package folders into Synaptic Canvas packages. Use when importing a legacy skill, merging related packages, adding install-and-troubleshooting guidance, preserving manual CLI/app install instructions, or preparing a package for sc admin import.
---

# Importing Synaptic Canvas Packages

Convert a skill or package into a Synaptic Canvas package that still works in
traditional marketplaces.

## Step 1 — Verify `sc` Installation

```bash
which sc && sc --version
```

If not found on PATH, also check common install locations:

```bash
for p in "$HOME/.local/bin/sc" "$HOME/.venvs/sc/bin/sc" \
  "$(python3 -m site --user-base 2>/dev/null)/bin/sc" \
  "/opt/homebrew/bin/sc"; do
  [ -x "$p" ] && echo "Found at: $p" && break
done
```

If found at a non-PATH location, use the full path for all commands, or export
the directory to PATH for this session:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

If not installed: **read `references/installation-and-troubleshooting.md`
before proceeding.**

## Scope

- Use the real `sc` CLI for import and validation.
- Do not invent a second schema outside Synaptic Canvas metadata.
- Preserve manual install paths for exported skills.
- Treat `sc` automation as additive, not exclusive.

## QA Baseline

Review this skill and converted skills against:
- `/Users/randlee/Documents/github/synaptic-canvas/docs/claude-code-skills-agents-guidelines.md`

## Workflow

1. Read `references/import-conversion-rules.md`.
2. Inventory artifacts, dependencies, install guidance, and package boundaries.
3. Decide topology first. Prefer one package when infra and dependencies are
   shared.
4. Normalize docs and metadata:
   - keep standard install-and-troubleshooting for non-`sc` users
   - add a short Synaptic Canvas subsection with `sc install`, `sc upgrade`,
     `sc uninstall`
   - record dependency metadata for `sc install`
   - align manifest and package metadata with actual artifacts
5. When the package is ready, use:

```bash
sc admin import <path> --branch <branch> --json
```

## Package Rules

- Keep standard manual install guidance for exported marketplaces.
- For fast-moving vendor CLIs, link to official upstream docs.
- For complex curated runtimes, keep local install guidance.
- Prefer one package with multiple skills when launch infra and dependencies are
  shared.

## Candidate Patterns

- `claude-history/.claude`
- `synaptic-canvas/packages/sc-docling-pdf`
- `synaptic-canvas/packages/sc-launchpad` + `sc-launch-term`
- `atm-core/.claude/skills/sprint-report`

## Deliverable Shape

- recommended topology
- required edits
- dependency and install guidance decisions
- missing docs
- import readiness
