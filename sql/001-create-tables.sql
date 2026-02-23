-- =============================================================================
-- Synaptic Canvas — DoltHub Database Schema
-- =============================================================================
-- Repository: https://www.dolthub.com/repositories/randlee/synaptic-canvas
-- Schema doc: docs/synaptic-canvas-schema.md
--
-- Run on each branch (main, beta, develop) — the branch IS the channel.
-- Promotion is dolt_merge, not a data update.
--
-- Compatible with: Dolt (MySQL 8 wire protocol)
-- =============================================================================

-- ---------------------------------------------------------------------------
-- packages: top-level unit of installation
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS packages (
    id                  VARCHAR(128)  NOT NULL,
    name                VARCHAR(256)  NOT NULL,
    version             VARCHAR(64)   NOT NULL,
    description         TEXT,
    agent_variant       VARCHAR(64)   NOT NULL DEFAULT 'claude',

    -- Metadata
    author              VARCHAR(256),
    license             VARCHAR(64)   DEFAULT 'MIT',
    tags                TEXT,                             -- comma-separated: "git,commit,workflow"
    min_claude_version  VARCHAR(32),                      -- e.g. "1.0.32"
    created_at          TIMESTAMP     DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id)
);

-- ---------------------------------------------------------------------------
-- package_files: one row per file in a package
--   - content stored as LONGTEXT (all skill files are text)
--   - markdown files get YAML frontmatter extracted to JSON + fm_* columns
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS package_files (
    package_id      VARCHAR(128)  NOT NULL,
    dest_path       VARCHAR(512)  NOT NULL,

    -- Content
    content         LONGTEXT      NOT NULL,               -- full original file
    sha256          VARCHAR(64)   NOT NULL,

    -- Classification
    file_type       VARCHAR(32)   NOT NULL DEFAULT 'skill',   -- skill|agent|command|script|hook|config
    content_type    VARCHAR(32)   NOT NULL DEFAULT 'markdown', -- markdown|python|json|yaml|text
    is_template     BOOLEAN       NOT NULL DEFAULT FALSE,

    -- Extracted frontmatter (denormalized for fast SQL filtering)
    fm_name         VARCHAR(256),                         -- YAML: name
    fm_description  TEXT,                                  -- YAML: description
    fm_version      VARCHAR(64),                           -- YAML: version
    fm_model        VARCHAR(64),                           -- YAML: model (agents only)

    -- Full frontmatter as structured JSON
    frontmatter     JSON,                                  -- complete YAML header as JSON

    PRIMARY KEY (package_id, dest_path),
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- package_deps: dependency declarations
--   dep_type: 'tool' (binary on PATH), 'cli' (install if absent), 'skill' (SC pkg)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS package_deps (
    package_id  VARCHAR(128)  NOT NULL,
    dep_type    VARCHAR(32)   NOT NULL,                   -- tool|cli|skill
    dep_name    VARCHAR(256)  NOT NULL,
    dep_spec    VARCHAR(128)  DEFAULT '',                  -- version spec e.g. ">=3.11"
    install_cmd VARCHAR(1024) DEFAULT '',                  -- shell command to install
    cmd_sha256  VARCHAR(64)   DEFAULT '',                  -- hash of install_cmd for future verification

    PRIMARY KEY (package_id, dep_name),
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- package_variants: logical name → agent-profile-specific implementation
--   Enables `synaptic install claude-history` to resolve per SYNAPTIC_AGENTS
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS package_variants (
    logical_id          VARCHAR(128)  NOT NULL,
    agent_profile       VARCHAR(64)   NOT NULL,           -- claude|codex|codex+claude
    variant_package_id  VARCHAR(128)  NOT NULL,

    PRIMARY KEY (logical_id, agent_profile),
    FOREIGN KEY (variant_package_id) REFERENCES packages(id) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- package_hooks: hook declarations for the dispatcher
--   See docs/synaptic-canvas-hook-system.md for architecture
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS package_hooks (
    package_id  VARCHAR(128)  NOT NULL,
    event       VARCHAR(64)   NOT NULL,                   -- PreToolUse|PostToolUse|...
    matcher     VARCHAR(256)  NOT NULL DEFAULT '.*',      -- regex for tool/context matching
    script_path VARCHAR(512)  NOT NULL,                   -- must match a dest_path in package_files
    priority    INT           NOT NULL DEFAULT 50,        -- lower runs first
    blocking    BOOLEAN       NOT NULL DEFAULT FALSE,

    PRIMARY KEY (package_id, event, script_path),
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- package_questions: install-time prompts for template rendering
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS package_questions (
    package_id  VARCHAR(128)  NOT NULL,
    question_id VARCHAR(128)  NOT NULL,
    prompt      TEXT          NOT NULL,
    type        VARCHAR(32)   NOT NULL DEFAULT 'choice',  -- choice|multi|text|confirm|auto
    default_val VARCHAR(512)  DEFAULT '',
    choices     TEXT          DEFAULT '',                   -- comma-separated for choice/multi
    sort_order  INT           NOT NULL DEFAULT 0,          -- lower appears first

    PRIMARY KEY (package_id, question_id),
    FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
);

-- =============================================================================
-- Indexes for common query patterns
-- =============================================================================

-- Find files by type across all packages (agent listing, command listing)
CREATE INDEX idx_package_files_type ON package_files (file_type);

-- Find agents by model
CREATE INDEX idx_package_files_model ON package_files (fm_model)
;

-- Find packages by variant
CREATE INDEX idx_package_variants_logical ON package_variants (logical_id);

-- Find hooks by event for dispatcher ordering
CREATE INDEX idx_package_hooks_event ON package_hooks (event, priority);

-- Find deps of a specific type (e.g., all skill deps for graph walks)
CREATE INDEX idx_package_deps_type ON package_deps (dep_type, dep_name);
