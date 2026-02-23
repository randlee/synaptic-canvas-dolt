-- =============================================================================
-- Synaptic Canvas — Example seed data
-- =============================================================================
-- Inserts one representative package (sc-manage) to validate the schema.
-- Based on the existing sc-manage package from the synaptic-canvas marketplace.
-- =============================================================================

INSERT INTO packages (id, name, version, description, agent_variant, author, license, tags)
VALUES (
    'sc-manage',
    'sc-manage',
    '0.9.0',
    'Manage Synaptic Canvas Claude packages. List available packages and their install status (local/global), and install or uninstall packages according to policy.',
    'claude',
    'synaptic-canvas',
    'MIT',
    'management,packages,installer,agents'
);

-- ---------------------------------------------------------------------------
-- Files: command, skill, agents, scripts
-- ---------------------------------------------------------------------------

-- Command file (markdown with frontmatter)
INSERT INTO package_files (package_id, dest_path, content, sha256, file_type, content_type,
    fm_name, fm_description, fm_version, frontmatter)
VALUES (
    'sc-manage',
    'commands/sc-manage.md',
    '---\nallowed-tools: Bash(python3 scripts/sc_manage_dispatch.py*)\nname: sc-manage\ndescription: List, install, or uninstall Synaptic Canvas Claude packages.\nversion: 0.9.0\n---\n\n# /sc-manage\n\nManage packages for the current machine or this repo.\n',
    'placeholder_sha256_command',
    'command',
    'markdown',
    'sc-manage',
    'List, install, or uninstall Synaptic Canvas Claude packages.',
    '0.9.0',
    JSON_OBJECT(
        'allowed-tools', 'Bash(python3 scripts/sc_manage_dispatch.py*)',
        'name', 'sc-manage',
        'description', 'List, install, or uninstall Synaptic Canvas Claude packages.',
        'version', '0.9.0'
    )
);

-- Skill file (markdown with frontmatter)
INSERT INTO package_files (package_id, dest_path, content, sha256, file_type, content_type,
    fm_name, fm_description, fm_version, frontmatter)
VALUES (
    'sc-manage',
    'skills/managing-sc-packages/SKILL.md',
    '---\nname: managing-sc-packages\ndescription: List, install, or uninstall Synaptic Canvas packages.\nversion: 0.9.0\n---\n\n# Managing Synaptic Canvas Packages\n\nUse this skill to manage packages.\n',
    'placeholder_sha256_skill',
    'skill',
    'markdown',
    'managing-sc-packages',
    'List, install, or uninstall Synaptic Canvas packages.',
    '0.9.0',
    JSON_OBJECT(
        'name', 'managing-sc-packages',
        'description', 'List, install, or uninstall Synaptic Canvas packages.',
        'version', '0.9.0'
    )
);

-- Agent file (markdown with frontmatter including model)
INSERT INTO package_files (package_id, dest_path, content, sha256, file_type, content_type,
    fm_name, fm_description, fm_version, fm_model, frontmatter)
VALUES (
    'sc-manage',
    'agents/sc-package-install.md',
    '---\nname: sc-package-install\nversion: 0.9.0\ndescription: Install a Synaptic Canvas package.\nmodel: sonnet\ncolor: green\n---\n\n# sc-package-install Agent\n\nInstall a package locally or globally.\n',
    'placeholder_sha256_agent',
    'agent',
    'markdown',
    'sc-package-install',
    'Install a Synaptic Canvas package.',
    '0.9.0',
    'sonnet',
    JSON_OBJECT(
        'name', 'sc-package-install',
        'version', '0.9.0',
        'description', 'Install a Synaptic Canvas package.',
        'model', 'sonnet',
        'color', 'green'
    )
);

-- Script file (no frontmatter)
INSERT INTO package_files (package_id, dest_path, content, sha256, file_type, content_type)
VALUES (
    'sc-manage',
    'scripts/sc_manage_dispatch.py',
    '#!/usr/bin/env python3\n"""Dispatch sc-manage subcommands."""\nimport sys\n# ... script content ...\n',
    'placeholder_sha256_script',
    'script',
    'python'
);

-- Config file (plugin.json)
INSERT INTO package_files (package_id, dest_path, content, sha256, file_type, content_type)
VALUES (
    'sc-manage',
    '.claude-plugin/plugin.json',
    '{"name":"sc-manage","version":"0.9.0","description":"Manage SC packages."}',
    'placeholder_sha256_config',
    'config',
    'json'
);

-- ---------------------------------------------------------------------------
-- Dependencies
-- ---------------------------------------------------------------------------

INSERT INTO package_deps (package_id, dep_type, dep_name, dep_spec)
VALUES
    ('sc-manage', 'tool', 'python3', '>=3.8'),
    ('sc-manage', 'tool', 'git', ''),
    ('sc-manage', 'tool', 'pydantic', '');

-- ---------------------------------------------------------------------------
-- Verify
-- ---------------------------------------------------------------------------

SELECT '--- Packages ---' AS section;
SELECT id, name, version, agent_variant FROM packages;

SELECT '--- Files by type ---' AS section;
SELECT file_type, COUNT(*) AS cnt FROM package_files GROUP BY file_type;

SELECT '--- Agents with model ---' AS section;
SELECT fm_name, fm_model FROM package_files WHERE file_type = 'agent';

SELECT '--- Dependencies ---' AS section;
SELECT dep_type, dep_name, dep_spec FROM package_deps WHERE package_id = 'sc-manage';

SELECT '--- JSON query: allowed-tools ---' AS section;
SELECT fm_name, JSON_EXTRACT(frontmatter, '$."allowed-tools"') AS allowed_tools
FROM package_files
WHERE file_type = 'command';
