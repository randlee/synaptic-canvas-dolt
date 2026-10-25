# sc-dolt Beads Migration — Status Record

**Status:** COMPLETE
**Executed:** 2026-08-26 · **Verified:** 2026-10-25
**Owner:** skillrx (PM)

Moves the 52 `sc-dolt-*` beads off the shared `randlee/sc-beads` DB into the
dedicated DoltHub DB `randlee/sc-dev`, per the graduation model adopted
2026-08-25 (one DoltHub DB per mature project, DB name = `ATM_TEAM`).

This document is the record of what was done and what remains. It replaces the
earlier proposal, whose execution steps no longer describe reality.

## Naming (final — do not change)

| Item | Value |
|---|---|
| Beads DB (DoltHub) | `randlee/sc-dev` |
| Beads prefix | `sc-dolt` (legacy hyphenated — pre-dates the short-acronym convention) |
| `ATM_TEAM` | `sc-dev` |
| Git repo | `randlee/synaptic-canvas-dolt` |
| Package registry (**not** beads) | `randlee/synaptic-canvas-dolt` on DoltHub |

**Critical distinction — do not conflate.** `randlee/synaptic-canvas-dolt` on
DoltHub is the sc-dolt **package registry** (`packages`, `package_files`, …
schema) — product data. The sc-dolt **beads** DB is `randlee/sc-dev`. Both are
Dolt; different databases, different schemas. Never point `.beads` at the
registry. Live mapping: `~/github/hendrix/sc-ecosystem-registry.json`.

## What was done

| Item | State |
|---|---|
| `randlee/sc-dev` created on DoltHub | done |
| 52 `sc-dolt-*` beads in `sc-dev`, IDs preserved | done — 52 issues |
| Hierarchy preserved | done — 49 molecule beads, **62** dependency edges |
| `issue_prefix` / `allowed_prefixes` | done — both `sc-dolt` |
| Synced to DoltHub | done — 0 unpushed / 0 unpulled (2026-10-25) |
| Workspace `.beads` repointed to server mode | done — PR #77, merged `51e71e5` |
| Vestigial embedded local store removed | done |

### The workspace repoint (why it mattered)

`.beads` in the git repo had never been repointed after the migration. It was a
vestigial **embedded** store: no `dolt_mode`, no `dolt_server_port`, no remote,
no `issues` table. Consequence: every `bd` invocation **auto-spawned a
per-workspace `dolt sql-server`** on a random port instead of using the single
launchd-managed shared server on `127.0.0.1:3308` — the unauthorized
stray-server pattern.

Fixed in PR #77 (`51e71e5`):

- `metadata.json` — `dolt_mode: server`, `dolt_database: sc-dev`,
  `dolt_server_user: skillrx`, `project_id` = sc-dev's `_project_id`
  (`3d84240f…`); bd refuses to connect on identity mismatch.
- `config.yaml` — `dolt.shared-server: true` (**required**: without this key bd
  falls back to embedded and spawns a server even with server mode set in
  metadata) and `sync.remote` →
  `https://doltremoteapi.dolthub.com/randlee/sc-dev`.

Verified after merge: `bd where` / `bd list` / `bd status` rc=0 against
`sc-dev` (52 issues, prefix `sc-dolt`), `bd dolt push` → *Push complete*, and no
local dolt server spawned.

## Remaining work

| # | Item | Priority |
|---|---|---|
| 1 | Remove the 5 stale local worktrees — all still carry the pre-#77 embedded `.beads` and will spawn a dolt server if `bd` runs there | Should do |
| 2 | Backfill `close_reason` on 15 of the 52 sc-beads beads | Optional (bookkeeping) |
| 3 | `sc-dolt` row in `sc-beads.project_prefixes` | **No action** — intentionally retained |

### Detail

**1. Stale worktrees.** `git worktree list` shows five under
`~/github/synaptic-canvas-dolt-worktrees/`: `add-readme-site-url1`,
`beads-migration-plan1`, `rename-team-sc-dev1`, `site-docs-index1`,
`site-sc-compose-pattern1`. All are merged except `beads-migration-plan1`, whose
only commit is the superseded proposal this record replaces. Removing them
removes the last local places where an embedded server can be spawned.

**2. Tombstone reasons.** In `sc-beads`, all 52 `sc-dolt-*` issues are `closed`,
but only **37** carry `RELOCATED to sc-dev`; 15 were closed before the migration
for unrelated reasons. No data risk — `sc-dev` is authoritative and holds all 52
with IDs and hierarchy intact. This is audit-label consistency only.

**3. Prefix registry — no action.** The original plan said to delete the
`sc-dolt` row from `sc-beads.project_prefixes`. That was **not** done and should
not be: `sc-beads` is the canonical **SC-wide prefix registry** (archive +
registry post-graduation). Dropping the row would orphan the registry — rows are
moved out, never just deleted.

## Rollback

The `sc-beads` copies are `closed`, not deleted, so they remain reopenable. The
authoritative copy is `sc-dev`; rolling back means recreating sc-dolt beads there
from the sc-beads tombstones. Note there is currently **no retained
`DOLT_BACKUP` snapshot of `sc-dev`** — take one before any destructive operation
on it.
