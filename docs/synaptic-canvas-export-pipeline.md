# Synaptic Canvas — Marketplace Export Pipeline

## Overview

This document describes the pipeline that exports skill packages from the Dolt database (on the `main` branch) into the directory structure expected by the Synaptic Canvas installer (`sc-install.py`) and the Claude Code marketplace.

**Direction:** Dolt `main` branch → filesystem packages → marketplace repo

The export pipeline is the inverse of ingestion: the Dolt database is the source of truth, and the marketplace repo is a materialized view.

---

## Target Directory Structure

Each package materializes to a directory under `packages/`:

```
packages/<package-id>/
├── manifest.yaml                  # Reconstructed from packages + package_deps rows
├── .claude-plugin/
│   └── plugin.json                # Reconstructed from package_files rows
├── agents/
│   └── *.md                       # file_type = 'agent'
├── commands/
│   └── *.md                       # file_type = 'command'
├── skills/
│   └── <skill-name>/
│       └── SKILL.md               # file_type = 'skill'
├── scripts/
│   └── *.py                       # file_type = 'script'
└── hooks/                          # file_type = 'hook' (if any)
    └── *.py
```

### File Placement Rules

The `dest_path` column in `package_files` stores the path relative to the package root. The export pipeline writes each file to `packages/<package_id>/<dest_path>`.

| `file_type` | Typical `dest_path` pattern | Target directory |
|-------------|---------------------------|------------------|
| `agent` | `agents/<name>.md` | `packages/<pkg>/agents/` |
| `command` | `commands/<name>.md` | `packages/<pkg>/commands/` |
| `skill` | `skills/<skill-name>/SKILL.md` | `packages/<pkg>/skills/<name>/` |
| `script` | `scripts/<name>.py` | `packages/<pkg>/scripts/` |
| `hook` | `scripts/<name>.py` (or `hooks/`) | `packages/<pkg>/scripts/` |
| `config` | `.claude-plugin/plugin.json` | `packages/<pkg>/.claude-plugin/` |

---

## Reconstruction Logic

### 1. `manifest.yaml` (reconstructed from SQL)

The manifest is NOT stored as a file in `package_files` — it is reconstructed from relational data:

```sql
-- Data for manifest reconstruction
SELECT p.id, p.name, p.version, p.description, p.author, p.license, p.tags,
       p.min_claude_version, p.install_scope, p.variables, p.options
FROM packages p
WHERE p.id = ?;

-- Artifacts (grouped by file_type)
SELECT dest_path, file_type
FROM package_files
WHERE package_id = ?
ORDER BY file_type, dest_path;

-- Requirements (with version specs)
SELECT dep_name, dep_spec, dep_type
FROM package_deps
WHERE package_id = ?;
```

**Rendered manifest.yaml (full example — sc-git-worktree):**

```yaml
name: sc-git-worktree
version: 0.9.0
description: >
  Manage git worktrees with optional tracking and protected branch safeguards.
author: randlee
license: MIT
tags:
  - git
  - worktree
  - workflow

artifacts:
  commands:
    - commands/sc-git-worktree.md
  skills:
    - skills/sc-git-worktree/SKILL.md
  agents:
    - agents/sc-git-worktree-create.md
    - agents/sc-git-worktree-scan.md
    - agents/sc-git-worktree-cleanup.md
    - agents/sc-git-worktree-abort.md
    - agents/sc-git-worktree-update.md
  scripts:
    - scripts/envelope.py
    - scripts/worktree_shared.py
    - scripts/worktree_scan.py

# From packages.variables JSON column
variables:
  REPO_NAME:
    auto: git-repo-basename
    description: Repository name for default worktree paths

# From packages.install_scope column
install:
  scope: local-only

# From packages.options JSON column
options:
  no-tracking:
    type: boolean
    default: false
    description: Disable worktree tracking document references

requires:
  - python3
  - pydantic
  - git >= 2.20
```

**Mapping rules:**
- `name`, `version`, `description`, `author`, `license` → direct from `packages` row
- `tags` → split comma-separated `packages.tags` into YAML list
- `artifacts` → group `package_files` rows by `file_type`, list `dest_path` values under their type key
- `requires` → `package_deps` rows where `dep_type = 'tool'`: format as `dep_name` or `dep_name dep_spec` if spec is non-empty
- `variables` → render `packages.variables` JSON as YAML (omit section if NULL)
- `install.scope` → from `packages.install_scope` (omit section if `'any'`)
- `options` → render `packages.options` JSON as YAML (omit section if NULL)

### 2. `plugin.json` (from package_files)

If a `config` file with `dest_path = '.claude-plugin/plugin.json'` exists, write it directly from the `content` column.

If no plugin.json exists in `package_files`, reconstruct it:

```sql
SELECT p.id, p.name, p.description, p.version, p.author, p.license,
       p.tags
FROM packages p
WHERE p.id = ?;

-- Commands, agents, skills for the plugin manifest
SELECT fm_name, fm_description, fm_version, file_type, dest_path
FROM package_files
WHERE package_id = ? AND file_type IN ('command', 'agent', 'skill');
```

**Rendered plugin.json:**

```json
{
  "name": "sc-manage",
  "description": "Manage Synaptic Canvas Claude packages.",
  "version": "0.9.0",
  "author": "synaptic-canvas",
  "license": "MIT",
  "keywords": ["management", "packages"],
  "commands": [
    { "name": "sc-manage", "description": "List, install, or uninstall packages." }
  ],
  "agents": [
    { "name": "sc-packages-list", "description": "Enumerate available packages." }
  ],
  "skills": [
    { "name": "managing-sc-packages", "description": "List, install, or uninstall packages." }
  ]
}
```

### 3. Content files (direct write)

All other files are written directly from the `content` column:

```sql
SELECT dest_path, content, sha256
FROM package_files
WHERE package_id = ? AND file_type != 'config';
```

For each row: write `content` to `packages/<package_id>/<dest_path>`, verify SHA-256 matches.

---

## Export Pipeline Steps

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  1. Query    │────→│  2. Write    │────→│  3. Verify   │────→│  4. Commit   │
│  Dolt main   │     │  Files       │     │  Integrity   │     │  to Git      │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
```

### Step 1: Query Dolt `main` branch

Connect to DoltHub and query all packages:

```sql
-- All packages on main
SELECT * FROM packages ORDER BY id;

-- All files for each package
SELECT * FROM package_files WHERE package_id = ? ORDER BY dest_path;

-- All deps for each package
SELECT * FROM package_deps WHERE package_id = ? ORDER BY dep_name;
```

### Step 2: Write files to marketplace directory

For each package:
1. Create `packages/<id>/` directory tree
2. Write content files from `package_files.content`
3. Reconstruct `manifest.yaml` from packages + package_deps + package_files metadata
4. Write or reconstruct `plugin.json`

### Step 3: Verify integrity

For each written file:
1. Compute SHA-256 of written content
2. Compare against `package_files.sha256`
3. Fail loudly on mismatch

### Step 4: Commit to marketplace repo

```bash
cd marketplace-repo
git add packages/
git commit -m "Export from Dolt main @ $(dolt log -n 1 --format='%h %s')"
```

---

## Diff-Based Validation

The export pipeline can be validated by comparing its output against the current marketplace repo:

```bash
# Export from Dolt
python3 tools/dolt-export.py --output /tmp/exported-packages/

# Compare against existing marketplace
diff -r /tmp/exported-packages/ /path/to/synaptic-canvas/packages/
```

Any differences indicate either:
- Migration missed content (bug in ingestion)
- Export reconstruction differs from source (bug in export)
- Legitimate changes made in Dolt after migration

---

## Implementation Plan

### Phase 1: Export script (`tools/dolt-export.py`)

- Connect to Dolt via MySQL wire protocol
- Query packages, files, deps
- Write directory structure
- Reconstruct manifest.yaml and plugin.json
- Verify SHA-256 integrity

### Phase 2: CI automation

- GitHub Action on marketplace repo
- Triggered by DoltHub webhook on `main` branch merge
- Runs export script
- Creates PR with changes for review

### Phase 3: Bidirectional sync (future)

- Ingestion: marketplace repo changes → Dolt `develop` branch
- Export: Dolt `main` merge → marketplace repo PR
- Conflict detection via SHA-256 comparison

---

## Configuration

```yaml
# tools/dolt-export.yaml (or env vars)
dolt:
  host: dolthub.com          # or local Dolt SQL server
  database: synaptic-canvas
  branch: main
  user: ${DOLT_USER}
  token: ${DOLT_TOKEN}

output:
  directory: ./packages/      # marketplace repo packages dir
  clean: true                 # remove existing before export
  verify_sha256: true         # fail on hash mismatch
```

---

## Future Considerations

- **Incremental export:** Only export packages changed since last export (compare Dolt commit hashes)
- **Template rendering:** If `is_template = TRUE`, the export should include the raw template, not rendered output (rendering happens at install time)
- **Variant handling:** Export only the packages matching a specific `agent_profile`, or export all variants
- **Signed manifests:** Add GPG signatures to exported manifest.yaml for supply-chain verification
