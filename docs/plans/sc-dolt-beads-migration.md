# sc-dolt Beads Migration Plan — 52 beads → `randlee/sc-dev`

**Author:** skillrx (PM/user)
**Status:** proposed — awaiting `randlee/sc-dev` DoltHub repo creation + execution approval
**Date:** 2026-08-26

## Goal

Migrate all `sc-dolt-*` beads out of the shared `randlee/sc-beads` DB into a
dedicated DoltHub DB `randlee/sc-dev`, following the established graduation
process. This is the test case that validates the process end-to-end before
pater runs it for the remaining projects.

## Naming decisions (finalized)

| Item | Value |
|------|-------|
| DoltHub repo (beads) | `randlee/sc-dev` |
| beads prefix | `sc-dolt` (legacy hyphenated — pre-dates short-acronym convention) |
| ATM_TEAM | `sc-dev` |
| GitHub repo | `randlee/synaptic-canvas-dolt` |
| Package registry (NOT beads) | `randlee/synaptic-canvas-dolt` on DoltHub — primary public-facing sc-dolt package registry |

**Critical distinction:** `randlee/synaptic-canvas-dolt` (DoltHub) is the
sc-dolt **package registry** (`packages`, `package_files`, … schema) — NOT
beads. The sc-dolt **beads** DB is `randlee/sc-dev`. Do not conflate.

## Source data

- `randlee/sc-beads` on DoltHub currently holds **57** issues:
  - `sc-dolt-*` → **52** (these are what we migrate)
  - `atm-*` → 5 (already migrated to `randlee/atm-dev` on 2026-08-25)

The 52 `sc-dolt-*` beads are a rich hierarchy:

- **Epics / molecules:**
  - `sc-dolt-mol-459` — create-github-pages-site (molecule) — closed
  - `sc-dolt-mol-459.1`–`.12` — docs moves + user guides
  - `sc-dolt-mol-459.13`–`.15` — eval architecture, OpenShell, sc-dolt-sandbox
  - `sc-dolt-mol-459.16`–`.21` — Codex marketplace workflow (research→spec→plan→execute→verify)
  - `sc-dolt-mol-459.22`–`.24` — immutability, package overwrite, separate sandbox DB
  - `sc-dolt-mol-459.25`–`.28` — bugs (silent exit, beads collision, manifest docs, SHA mismatch)
  - `sc-dolt-mol-459.29`–`.30` — sc-sha crate unification
  - `sc-dolt-mol-459.31` — **EPIC: migrate sc-publish → sc-dolt** (open)
  - `sc-dolt-mol-459.31.1`–`.31.5` — P1–P5 phases + children
- **Loose tasks:** `sc-dolt-dnn`, `sc-dolt-j9b`, `sc-dolt-m7z`

## Execution steps

### 0. Precondition (manual, blocked on Rand)

Create the empty DoltHub repo `randlee/sc-dev` at
<https://www.dolthub.com/repositories/new>.

### 1. Export the 52 sc-dolt beads

```bash
T=$(mktemp -d)
dolt clone randlee/sc-beads "$T"
cd "$T"
dolt sql -r json -q "SELECT * FROM issues WHERE id LIKE 'sc-dolt-%'" > /tmp/sc-dolt-issues.json
```

Transform to `bd import` JSONL (id-preserving, RFC3339 timestamps, dependency
map from the `dependencies` table — same generator pattern as the atm
migration).

### 2. Initialize the dedicated DB

```bash
python3 ~/Documents/github/hendrix/skills/beads-management/scripts/init-beads-db.py \
  <workspace-dir> sc-dolt synaptic-canvas-dolt randlee/sc-dev "SC product: Dolt-backed package management"
```

This: creates `sc-dev` DB on the shared server (127.0.0.1:3308), runs
`bd init --shared-server --external`, sets `allowed_prefixes=sc-dolt` +
`status.custom` lifecycle, creates `project_prefixes` row, sets DoltHub remote,
commits.

### 3. Import

```bash
bd import -i /tmp/sc-dolt-migration.jsonl
bd dolt commit -m "migrate: import 52 sc-dolt beads from sc-beads"
bd dolt push
```

### 4. Verify on DoltHub

Fresh clone of `randlee/sc-dev` → assert:
- 52 issues present, IDs preserved (`sc-dolt-*`)
- parent-child + blocks links intact (count = source count)
- `child_counters` preserved (molecule `.1`–`.31` numbering)
- descriptions byte-identical
- `issue_prefix=sc-dolt`, `allowed_prefixes=sc-dolt`, `status.custom` set

### 5. Graduate out of sc-beads

```bash
# Close (tombstone) the 52 sc-dolt-* beads in sc-beads
dolt sql -q "UPDATE issues SET status='closed', close_reason='RELOCATED to sc-dev', closed_at=NOW() WHERE id LIKE 'sc-dolt-%'"
# Remove sc-dolt from project_prefixes registry
dolt sql -q "DELETE FROM project_prefixes WHERE prefix='sc-dolt'"
dolt add -A && dolt commit -m "graduate sc-dolt -> sc-dev" && dolt push origin main
```

### 6. Remove embedded DB (if present)

Check `~/Documents/github/synaptic-canvas-dolt/.beads/` — if an embedded
`embeddeddolt/` dir exists with sc-dolt beads, remove it after migration is
confirmed (data is in sc-dev).

## Verification checklist

- [ ] `randlee/sc-dev` exists on DoltHub
- [ ] 52 beads imported with IDs + hierarchy intact
- [ ] sc-beads tombstones `closed` with `RELOCATED to sc-dev`
- [ ] `sc-dolt` removed from sc-beads `project_prefixes`
- [ ] No embedded DB left in synaptic-canvas-dolt

## Rollback

sc-beads tombstones are `closed` (reopenable), not `deleted`. If the migration
is wrong, `bd reopen` the sc-beads beads and force-push sc-dev back to empty.

## Dependencies / blockers

1. **Rand creates `randlee/sc-dev`** on DoltHub (manual — no programmatic create).
2. None else — process is already validated on the atm-dev migration.
