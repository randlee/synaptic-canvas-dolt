# Synaptic Canvas — Project Instructions

## Project

Synaptic Canvas is a Dolt-backed package management system for Claude Code skills. The `sc` Go CLI is the primary interface. DoltHub: https://www.dolthub.com/repositories/randlee/synaptic-canvas

## Design Documents

Requirements and architecture are documented. Read before making changes:

- [Requirements](docs/requirements.md) — cross-cutting product requirements
- [Architecture](docs/architecture.md) — top-level system architecture
- [CLI Design](docs/synaptic-canvas-cli.md) — `sc` command surface, architecture, integrity model
- [Schema Spec](docs/synaptic-canvas-schema.md) — Dolt table definitions and rationale
- [Export Pipeline](docs/synaptic-canvas-export-pipeline.md) — Dolt → filesystem reconstruction
- [Install System](docs/synaptic-canvas-install-system.md) — Package installation mechanics
- [Hook System](docs/synaptic-canvas-hook-system.md) — Pre/post install hooks

## Dev-QA Loop

All development follows a phased plan split into sprints. Each sprint runs a dev-qa loop that repeats until all QA agents are 100% satisfied.

### QA Agents

**1. Code QA Agent**
- Verifies implementation matches the sprint plan
- Checks adequate test coverage exists for new/changed code
- Runs full test suite, demands 100% pass
- Validates Go conventions (lint, formatting, vet)

**2. Requirements QA Agent** — `.claude/agents/sc-qa.md`
- Verifies work matches design documents (schema spec, CLI design, pipeline spec)
- **Stops work outside of documented requirements** — no scope creep
- Validates CLI commands match the command surface defined in the CLI design doc
- Checks schema changes are reflected in design docs (and vice versa)
- Ensures no drift between documentation and implementation

### Loop Structure

All agent runs use `Task` tool with `run_in_background=true`.

```
iteration = 1
WHILE iteration <= 3:
    Run Dev Phase (developer agent)
        - First iteration: full sprint dev prompt
        - Subsequent iterations: fix prompt incorporating QA findings
    Run Code QA Agent                    ← Task(subagent_type, run_in_background=true)
    Run Requirements QA Agent (sc-qa)    ← Task(subagent_type="sc-qa", run_in_background=true)
    IF BOTH QA verdicts are PASS:
        BREAK → proceed to PR
    IF EITHER QA verdict is FAIL:
        Extract specific findings from both QA outputs
        Write a NEW dev prompt that:
          - Lists the exact QA failures
          - Quotes the specific error messages or code issues
          - Provides clear fix instructions
          - References the relevant design documents
        iteration += 1
IF iteration > 3 and QA still FAIL:
    ESCALATE:
      - Sprint ID and deliverables
      - All QA failure reports across iterations
      - What was tried in each iteration
      - Request architecture review
    STOP — do not proceed to PR
```

**NEVER fix code yourself during QA.** Every fix goes through a developer agent. Dev agents fix. QA agents judge. Never mixed.

## Build & CI

- Go 1.26, source in `src/` (following claude-history conventions)
- GoReleaser for cross-platform builds
- CI: `test.yml` (lint + test matrix + build), `release.yml` (tag-triggered publish)
- All tests must pass with `-race` flag
- golangci-lint with gosec enabled

## Key Rules

1. **Design docs are the source of truth.** If code doesn't match the docs, the code is wrong (or the docs need updating first with explicit approval).
2. **No scope creep.** Only build what's in the current sprint plan. If something seems needed but isn't planned, flag it — don't build it.
3. **Dolt database is read-only for end-user commands.** Only admin commands write to Dolt.
4. **SHA integrity is non-negotiable.** Every file gets SHA256 at ingest. Every install verifies SHA. No exceptions.
5. **Branches are channels.** No `channel` column in the database. The Dolt branch IS the channel.

## Codex Orchestration Rules

These rules apply whenever `/codex-orchestration` governs a phase (csc as sole developer). Violations cause process drift and missed ACKs.

1. **QA runs parallel with CI — never wait for CI green first.** As soon as csc pushes and you create the PR, immediately assign QA to quality-mgr via `SendMessage`. The merge gate requires both QA PASS and CI green, but they must run concurrently.

2. **All csc assignments MUST use sc-compose + Jinja2 templates.** Never send raw `atm send csc "..."` strings. Always:
   ```bash
   sc-compose render --file .claude/skills/codex-orchestration/dev-template.xml.j2 --var-file /tmp/vars.json | atm send csc
   ```
   Without the structured XML template, csc will not ACK.

3. **Clear ATM inbox locks after every send and before every read.**
   ```bash
   rm -f ~/.claude/teams/sc-dev/inboxes/*.lock
   ```
   Locks reappear on every ATM write. Stale locks silently block context injection.

4. **Prepare the next sprint worktree before csc finishes the current sprint.** Create S+1 worktree branching from S as soon as csc ACKs the current assignment. Do not wait for csc to complete.

5. **Re-read `.claude/skills/codex-orchestration/SKILL.md` at the start of every session** where a phase is in progress. Context compaction causes process drift — re-reading is mandatory.

6. **quality-mgr assignments use qa-template.xml.j2 via SendMessage.** Send the rendered template body as the SendMessage content, not a raw text summary.
