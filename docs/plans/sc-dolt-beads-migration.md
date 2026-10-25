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
| 5 stale local worktrees removed | done — 2026-10-25 |
| Legacy duplicates purged from `sc-beads` | done — 57 issues, 66 deps, 7 counters |

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

**None.** The migration is complete, and `sc-beads` no longer carries stale
duplicates (see *Legacy copy purge* below).

The two items previously listed here were resolved by **deletion rather than
bookkeeping**:

- **No backfill of `RELOCATED` reasons.** Stamping where 15 legacy copies went
  has no SSOT value — those rows no longer exist.
- **No `project_prefixes` change.** The `sc-dolt` row is retained deliberately:
  `sc-beads` is the canonical SC-wide prefix registry, and rows are moved out,
  never dropped.

## Legacy copy purge (done 2026-10-25)

`sc-beads` held **57 issues, all stale duplicates** of the SSOT databases:

| Set | Count | What `sc-beads` said | Truth |
|---|---|---|---|
| `sc-dolt-*` | 52 | `closed` | perfect duplicate of `sc-dev` — ID sets diffed, 52 vs 52, no gaps either way |
| `atm-x7y*` | 5 | `closed` | `atm-dev` has them **open / in_progress** — the copies were actively misleading |

All 57 were deleted, plus their `dependencies` (66) and `child_counters` (7)
rows. `project_prefixes` (16 rows), `metadata` and `config` were retained.

Post-purge state, verified by a **fresh `dolt clone` from DoltHub**:

- issues **0**, registry rows **16**, history **165 commits** (nothing rewritten)
- `sc-dev` remains SSOT for sc-dolt beads; `atm-dev` for atm beads
- `sc-beads` is now purely the canonical SC-wide prefix registry

Cold backup taken before the purge:
`~/sc-beads-backup-pre-purge-20261025T172947Z`

## Worktree cleanup (done 2026-10-25)

All five worktrees under `~/github/synaptic-canvas-dolt-worktrees/` were removed
— `add-readme-site-url1`, `beads-migration-plan1`, `rename-team-sc-dev1`,
`site-docs-index1`, `site-sc-compose-pattern1` — along with the parent scaffolding
directory. All were clean (no uncommitted work); four were merged branches and
`beads-migration-plan1` was superseded by this record.

They mattered because each still carried the **pre-#77 embedded `.beads`**, so
any `bd` invocation inside one would auto-spawn a per-workspace dolt server. With
them gone, `git worktree list` shows only the main checkout on `develop`, and the
only dolt server is the launchd-managed shared one on `127.0.0.1:3308`.

Branches were left intact (local + remote); only the working directories were
removed. `sc/beads-migration-plan1` can be deleted whenever convenient.

## Rollback

The purge is reversible: Dolt keeps full history, and a cold backup was taken
first — `~/sc-beads-backup-pre-purge-20261025T172947Z`. To restore, clone that
backup and push its `main` back to DoltHub.

`sc-dev` (52 sc-dolt beads) and `atm-dev` (atm beads) were **not touched** by the
purge and remain SSOT. Note there is still **no retained `DOLT_BACKUP` snapshot of
`sc-dev`** — take one before any destructive operation on it.
