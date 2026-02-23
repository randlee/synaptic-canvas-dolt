# Synaptic Canvas — Dolt Database Schema

## Overview

This document defines the finalized Dolt database schema for the `synaptic-canvas` database hosted on DoltHub. It is the canonical reference for table definitions, relationships, and design rationale.

**DoltHub:** https://www.dolthub.com/repositories/randlee/synaptic-canvas

---

## Design Decisions Reflected in Schema

### 1. No `channel` column on `packages`

Channels are **branches, not data**. The `develop`, `beta`, and `main` branches each contain a complete copy of all tables. A package exists on a channel because it exists on that branch. No column needed.

```sql
-- "What's on main?" = query the main branch
-- "What's on beta?" = query the beta branch
-- No WHERE channel = 'main' — the branch IS the channel
```

This eliminates data duplication and makes promotion a pure `dolt_merge`, not a data update.

### 2. Text + JSON storage in `package_files`

File contents are stored as `LONGTEXT` (not LONGBLOB) because all skill files are text (markdown, Python, JSON, YAML). This enables:
- Full-text searching across file content via SQL `LIKE` or `MATCH`
- No encoding/decoding overhead — content is human-readable in query results

For markdown files with YAML frontmatter (agents, commands, skills), the frontmatter is also extracted into:
- **Dedicated columns** (`fm_name`, `fm_description`, `fm_model`) for the most-queried fields
- **A `JSON` column** (`frontmatter`) containing the complete YAML header as JSON for flexible querying via `JSON_EXTRACT`

This dual approach gives fast SQL filtering on common fields while preserving full frontmatter richness. Non-markdown files (scripts, configs) leave the frontmatter columns NULL.

The `sha256` column provides integrity verification independent of Dolt internals.

### 3. Composite primary keys

Most tables use composite PKs that naturally express their relationships:
- `package_files`: (`package_id`, `dest_path`) — one file per destination per package
- `package_deps`: (`package_id`, `dep_name`) — one dep entry per name per package
- `package_questions`: (`package_id`, `question_id`) — one question per ID per package
- `package_hooks`: (`package_id`, `event`, `script_path`) — one hook per event+script per package

### 4. `is_template` flag on `package_files`

Files with `.j2` extension are Jinja2 templates rendered at install time. The `is_template` flag makes this queryable without relying on filename conventions.

### 5. Security: `cmd_sha256` on `package_deps`

The `install_cmd` field contains shell commands run during install. The `cmd_sha256` column enables future command verification — compare against a signed allowlist before execution. Currently advisory; enforcement comes later.

---

## Tables

### `packages`

The top-level unit of installation. One package may contain multiple files and declare multiple dependencies.

```sql
CREATE TABLE packages (
    id              VARCHAR(128) NOT NULL,
    name            VARCHAR(256) NOT NULL,
    version         VARCHAR(64)  NOT NULL,
    description     TEXT,
    agent_variant   VARCHAR(64)  NOT NULL DEFAULT 'claude',
    -- Metadata
    author          VARCHAR(256),
    license         VARCHAR(64)  DEFAULT 'MIT',
    tags            TEXT,                          -- comma-separated: "git,commit,workflow"
    min_claude_version VARCHAR(32),                -- minimum Claude Code version, e.g. "1.0.32"
    -- Installation policy
    install_scope   VARCHAR(32)  NOT NULL DEFAULT 'any',  -- any|local-only
    variables       JSON,                          -- token expansion config (Tier 1 packages)
    options         JSON,                          -- install-time options (e.g. no-tracking)
    created_at      TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
);
```

**Notes:**
- `id` is the unique package identifier (e.g., `commit-msg`, `claude-history-claude`)
- `agent_variant` indicates the target agent: `claude`, `codex`, or `codex+claude`
- `tags` is comma-separated for simplicity; a join table is over-engineering at this scale
- `version` is semver (e.g., `1.3.0`). The Dolt commit hash provides the immutable snapshot reference; semver provides the human-readable version
- `install_scope`: `any` (default, can install globally or locally) or `local-only` (repo `.claude` only)
- `variables`: JSON object for Tier 1 token expansion, e.g. `{"REPO_NAME": {"auto": "git-repo-basename", "description": "..."}}`
- `options`: JSON object for install-time boolean/string options, e.g. `{"no-tracking": {"type": "boolean", "default": false}}`

### `package_files`

One row per file in the package. File content is stored as text with YAML frontmatter extracted into searchable columns and a JSON column.

```sql
CREATE TABLE package_files (
    package_id      VARCHAR(128) NOT NULL,
    dest_path       VARCHAR(512) NOT NULL,
    -- Content
    content         LONGTEXT     NOT NULL,      -- full file content (markdown, python, yaml, etc.)
    sha256          VARCHAR(64)  NOT NULL,
    -- Classification
    file_type       VARCHAR(32)  NOT NULL DEFAULT 'skill',
    content_type    VARCHAR(32)  NOT NULL DEFAULT 'markdown',  -- markdown, python, json, yaml, text
    is_template     BOOLEAN      NOT NULL DEFAULT FALSE,
    -- Extracted frontmatter (searchable columns for common queries)
    fm_name         VARCHAR(256),               -- YAML: name
    fm_description  TEXT,                        -- YAML: description
    fm_version      VARCHAR(64),                 -- YAML: version
    fm_model        VARCHAR(64),                 -- YAML: model (agents only)
    -- Full frontmatter as JSON (for flexible querying via JSON_EXTRACT)
    frontmatter     JSON,                        -- complete YAML header as structured JSON
    PRIMARY KEY (package_id, dest_path),
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
);
```

**`file_type` values:**

| Value | Materialized To | Purpose |
|-------|----------------|---------|
| `skill` | `.claude/skills/{pkg}/` | Prompt/skill markdown files |
| `agent` | `.claude/agents/` | Agent definition markdown files |
| `command` | `.claude/commands/` | Slash command markdown files |
| `script` | `.claude/skills/{pkg}/scripts/` | Executable scripts (Python, shell) |
| `hook` | `.claude/skills/{pkg}/hooks/` | Hook scripts registered with dispatcher |
| `config` | `.claude/skills/{pkg}/` | Configuration files (JSON, YAML) |

**`content_type` values:**

| Value | Has Frontmatter? | Description |
|-------|:-:|---------|
| `markdown` | ✓ | Markdown with YAML frontmatter (agents, commands, skills) |
| `python` | | Python source code |
| `json` | | JSON configuration (plugin.json, etc.) |
| `yaml` | | YAML configuration (manifest.yaml, etc.) |
| `text` | | Plain text or other formats |

**Frontmatter extraction:** When `content_type = 'markdown'`, the YAML block between `---` markers is:
1. Parsed to JSON and stored in the `frontmatter` column
2. Key fields extracted into `fm_name`, `fm_description`, `fm_version`, `fm_model`

Non-markdown files have `frontmatter = NULL` and all `fm_*` columns NULL.

**Querying frontmatter:**
```sql
-- Find all agents using the 'sonnet' model
SELECT package_id, dest_path, fm_name
FROM package_files
WHERE file_type = 'agent' AND fm_model = 'sonnet';

-- Find commands with specific allowed-tools (via JSON)
SELECT package_id, fm_name,
       JSON_EXTRACT(frontmatter, '$."allowed-tools"') AS allowed_tools
FROM package_files
WHERE file_type = 'command'
  AND JSON_EXTRACT(frontmatter, '$."allowed-tools"') IS NOT NULL;

-- Find agents that declare hooks
SELECT package_id, fm_name,
       JSON_EXTRACT(frontmatter, '$.hooks') AS hooks
FROM package_files
WHERE file_type = 'agent'
  AND JSON_EXTRACT(frontmatter, '$.hooks') IS NOT NULL;
```

**`is_template`:** When `TRUE`, the `content` column contains Jinja2 template source. The installer renders it at install time using repo profile + user answers, then stores the rendered output. The lockfile records the hash of the rendered output, not the template source.

### `package_deps`

Declares all dependencies a package requires. The installer walks this graph before materializing any files.

```sql
CREATE TABLE package_deps (
    package_id  VARCHAR(128) NOT NULL,
    dep_type    VARCHAR(32)  NOT NULL,
    dep_name    VARCHAR(256) NOT NULL,
    dep_spec    VARCHAR(128) DEFAULT '',
    install_cmd VARCHAR(1024) DEFAULT '',
    cmd_sha256  VARCHAR(64)  DEFAULT '',
    PRIMARY KEY (package_id, dep_name),
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
);
```

**`dep_type` values:**

| Value | Meaning | Example |
|-------|---------|---------|
| `tool` | Binary on PATH with version constraint | `python3>=3.11` |
| `cli` | External CLI to install if absent | `agent-teams-mail` via cargo |
| `skill` | Another Synaptic Canvas package | `common-utils` |

**`cmd_sha256`:** SHA-256 of the `install_cmd` string. Future use: verify against signed allowlist before executing install commands. Currently populated but not enforced.

### `package_variants`

Maps a logical package name to agent-profile-specific implementations. This enables `synaptic install claude-history` to automatically resolve to the correct variant based on `SYNAPTIC_AGENTS`.

```sql
CREATE TABLE package_variants (
    logical_id          VARCHAR(128) NOT NULL,
    agent_profile       VARCHAR(64)  NOT NULL,
    variant_package_id  VARCHAR(128) NOT NULL,
    PRIMARY KEY (logical_id, agent_profile),
    FOREIGN KEY (variant_package_id) REFERENCES packages(id) ON DELETE CASCADE
);
```

**Example:**

| logical_id | agent_profile | variant_package_id |
|------------|---------------|--------------------|
| claude-history | claude | claude-history-claude |
| claude-history | codex | claude-history-codex |
| claude-history | codex+claude | claude-history-dual |

When a user runs `synaptic install claude-history`, the resolver:
1. Reads `SYNAPTIC_AGENTS` (e.g., `claude`)
2. Looks up `package_variants WHERE logical_id = 'claude-history' AND agent_profile = 'claude'`
3. Resolves to `claude-history-claude` as the concrete package to install

If no variant mapping exists, the `logical_id` is treated as the concrete `package_id` directly.

### `package_hooks`

Hook declarations for packages. See [Hook System](./synaptic-canvas-hook-system.md) for the full dispatcher architecture.

```sql
CREATE TABLE package_hooks (
    package_id  VARCHAR(128) NOT NULL,
    event       VARCHAR(64)  NOT NULL,
    matcher     VARCHAR(256) NOT NULL DEFAULT '.*',
    script_path VARCHAR(512) NOT NULL,
    priority    INT          NOT NULL DEFAULT 50,
    blocking    BOOLEAN      NOT NULL DEFAULT FALSE,
    PRIMARY KEY (package_id, event, script_path),
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
);
```

**`event` values:** `PreToolUse`, `PostToolUse` (extensible as Claude Code adds more hook points).

**`script_path`:** Relative path within the package's file set. Must match a `dest_path` in `package_files` where `file_type = 'hook'`.

**`priority`:** Lower runs first. Conventions:

| Range | Purpose |
|-------|---------|
| 1–19 | Environment setup, variable injection |
| 20–39 | Policy enforcement, blocking guards |
| 40–59 | Logging, auditing |
| 60–79 | Notification, telemetry |
| 80+ | Post-processing, cleanup |

### `package_questions`

Install-time questions declared by packages. Asked once during install; answers stored in the lockfile and used for Jinja2 template rendering.

```sql
CREATE TABLE package_questions (
    package_id  VARCHAR(128) NOT NULL,
    question_id VARCHAR(128) NOT NULL,
    prompt      TEXT         NOT NULL,
    type        VARCHAR(32)  NOT NULL DEFAULT 'choice',
    default_val VARCHAR(512) DEFAULT '',
    choices     TEXT         DEFAULT '',
    sort_order  INT          NOT NULL DEFAULT 0,
    PRIMARY KEY (package_id, question_id),
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
);
```

**`type` values:**

| Type | Behavior |
|------|----------|
| `choice` | Single selection from predefined options |
| `multi` | Multiple selections from predefined options |
| `text` | Free text input |
| `confirm` | Yes/no |
| `auto` | Filled from repo profile, shown for confirmation |

**`choices`:** Comma-separated option list for `choice` and `multi` types. Empty for `text`, `confirm`, and `auto`.

**`default_val`:** Default value. For `auto` type, this is the repo profile key to read (e.g., `repo.languages`). For `confirm`, use `true` or `false`.

**`sort_order`:** Controls question presentation order. Lower values appear first. Questions with the same `sort_order` are presented in `question_id` alphabetical order.

---

## Entity Relationship Diagram

```
packages (1) ──────< package_files (many)
    │                   PK: (package_id, dest_path)
    │                   content: LONGTEXT, frontmatter: JSON
    │
    ├──────< package_deps (many)
    │           PK: (package_id, dep_name)
    │
    ├──────< package_hooks (many)
    │           PK: (package_id, event, script_path)
    │
    ├──────< package_questions (many)
    │           PK: (package_id, question_id)
    │
    └──────< package_variants (many, as variant_package_id)
                PK: (logical_id, agent_profile)
                FK: variant_package_id → packages.id

package_variants maps:
    logical_id (abstract name) → variant_package_id (concrete packages.id)
```

All relationships cascade on delete: removing a package removes its files, deps, hooks, questions, and variant mappings.

---

## Key Queries

### List available packages

```sql
SELECT id, name, version, description, agent_variant, tags
FROM packages
ORDER BY name;
```

### Resolve a package with dependencies

```sql
SELECT p.id, p.name, p.version,
       d.dep_type, d.dep_name, d.dep_spec, d.install_cmd
FROM packages p
LEFT JOIN package_deps d ON p.id = d.package_id
WHERE p.id = ?;
```

### Fetch files for a package

```sql
SELECT dest_path, content, sha256, file_type, content_type, is_template,
       fm_name, fm_description, frontmatter
FROM package_files
WHERE package_id = ?;
```

### Search files by frontmatter field

```sql
-- Find all agents across all packages
SELECT pf.package_id, p.name AS package_name, pf.fm_name, pf.fm_model, pf.fm_description
FROM package_files pf
JOIN packages p ON pf.package_id = p.id
WHERE pf.file_type = 'agent'
ORDER BY p.name, pf.fm_name;

-- Full-text search across file content
SELECT package_id, dest_path, fm_name
FROM package_files
WHERE content LIKE '%PreToolUse%';
```

### Resolve variant

```sql
SELECT variant_package_id
FROM package_variants
WHERE logical_id = ? AND agent_profile = ?;
```

### Get install questions

```sql
SELECT question_id, prompt, type, default_val, choices
FROM package_questions
WHERE package_id = ?
ORDER BY sort_order, question_id;
```

### Dependency impact analysis (what depends on X?)

```sql
SELECT p.name, p.version, d.dep_spec
FROM package_deps d
JOIN packages p ON d.package_id = p.id
WHERE d.dep_name = ? AND d.dep_type = 'skill';
```

### Full dependency tree (recursive)

```sql
-- Walk the dependency graph for a package
WITH RECURSIVE dep_tree AS (
    SELECT package_id, dep_name, dep_type, dep_spec, 0 AS depth
    FROM package_deps
    WHERE package_id = ?

    UNION ALL

    SELECT d.package_id, d.dep_name, d.dep_type, d.dep_spec, dt.depth + 1
    FROM package_deps d
    JOIN dep_tree dt ON d.package_id = dt.dep_name
    WHERE d.dep_type = 'skill' AND dt.depth < 10
)
SELECT * FROM dep_tree;
```

### Packages with hooks on a specific event

```sql
SELECT p.name, h.matcher, h.script_path, h.priority, h.blocking
FROM package_hooks h
JOIN packages p ON h.package_id = p.id
WHERE h.event = 'PreToolUse'
ORDER BY h.priority;
```

---

## Frontmatter JSON Schema

The `frontmatter` JSON column in `package_files` stores the complete YAML header from markdown files as structured JSON. The schema varies by `file_type`:

### Agent frontmatter

```json
{
  "name": "ci-build-agent",
  "version": "0.9.0",
  "description": "Run build and classify failures.",
  "model": "sonnet",
  "color": "blue",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "python3 scripts/validate_hook.py" }
        ]
      }
    ]
  }
}
```

**Required fields:** `name`, `description`
**Optional fields:** `version`, `model`, `color`, `hooks`

### Command frontmatter

```json
{
  "name": "sc-manage",
  "version": "0.9.0",
  "description": "List, install, or uninstall packages.",
  "allowed-tools": "Bash(python3 scripts/sc_manage_dispatch.py*)",
  "options": [
    { "name": "--list", "description": "List available packages." },
    { "name": "--install", "description": "Install a package.", "args": [
      { "name": "package", "description": "Package name to install." }
    ]}
  ]
}
```

**Required fields:** `name`, `description`
**Optional fields:** `version`, `allowed-tools`, `options`

### Skill frontmatter

```json
{
  "name": "managing-sc-packages",
  "version": "0.9.0",
  "description": "List, install, or uninstall Synaptic Canvas packages."
}
```

**Required fields:** `name`, `description`
**Optional fields:** `version`

### Ingestion rules

When inserting a markdown file into `package_files`:

1. **Parse** the YAML block between `---` markers at the top of the file
2. **Convert** the YAML to JSON and store in the `frontmatter` column
3. **Extract** `name` → `fm_name`, `description` → `fm_description`, `version` → `fm_version`, `model` → `fm_model`
4. **Store** the full original file (including the YAML block) in `content`
5. **Detect** `content_type` from file extension: `.md` → `markdown`, `.py` → `python`, `.json` → `json`, `.yaml`/`.yml` → `yaml`

The `content` column always contains the exact original file — the frontmatter columns and JSON are denormalized indexes for queryability, not replacements.

---

## Branch Model

| Branch | Role | Who writes | Who reads |
|--------|------|-----------|-----------|
| `develop` | Active development | Skill authors | Authors, testers |
| `beta` | Validated, needs broader testing | Promoted from develop | Early adopters |
| `main` | Proven, stable | Promoted from beta | All users, marketplace export |

**Promotion:** `dolt_merge('develop')` on the beta branch; `dolt_merge('beta')` on main. Each merge is a Dolt commit with author and message — full audit trail.

**Rollback:** `dolt_reset('--hard', 'HEAD~1')` on any branch reverts the last promotion.

---

## Migration Notes

### From current marketplace repo

The existing `synaptic-canvas` GitHub marketplace repo contains skills in a directory structure. Migration to Dolt involves:

1. Each `packages/<name>/manifest.yaml` → one `packages` row (id, name, version, description, tags, etc.)
2. Each file in the package → one `package_files` row:
   - `content` = full file text, `sha256` = computed hash
   - For `.md` files: parse YAML frontmatter → `frontmatter` JSON + `fm_*` columns
   - `file_type` determined by directory: `agents/` → `agent`, `commands/` → `command`, `skills/` → `skill`, `scripts/` → `script`
   - `content_type` determined by extension: `.md` → `markdown`, `.py` → `python`, `.json` → `json`, `.yaml` → `yaml`
3. Hook declarations from agent frontmatter `hooks` field → `package_hooks` rows
4. `manifest.yaml` `requires` section → `package_deps` rows
5. Variant information (if any) → `package_variants` rows
6. `.claude-plugin/plugin.json` content stored as a `config` file type row in `package_files`

The marketplace export pipeline (Dolt → Claude Code format) should produce output structurally identical to the current marketplace repo, enabling a diff-based validation.

---

## Future Considerations

### Not yet implemented (tracked for later)

- **Package signing:** Verify `install_cmd` against a signed allowlist using `cmd_sha256`
- **Download counts / telemetry:** Separate from the core schema; could be a DoltHub API integration
- **Deprecation flags:** `deprecated BOOLEAN`, `successor_id VARCHAR` on packages
- **Changelogs:** Dolt commit messages serve as the changelog for now; a dedicated `package_changelog` table may be warranted later
- **Rating/reviews:** Out of scope for v1; the quality signal is promotion, not user ratings
