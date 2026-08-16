# Multi-Harness Skill Package — Defining Document

> Status: defining document (draft v1) · Author: skillrx · Date: 2026-08-16
> Scope: what a "combined hermes / claude / codex skill package" looks like,
> and the best practices for authoring one skill that works across all three
> harnesses.

## 1. The core idea

A **skill** is a unit of reusable procedural knowledge: a `SKILL.md` body plus
supporting files (references, templates, scripts, formulas). Different
harnesses load skills from different locations and read different metadata.
A **multi-harness skill package** is one skill shipped once, with a thin
per-harness adapter so each harness sees it as native.

The reference implementation is the `beads` repo itself (a Claude Code
marketplace). It ships a single `skills/beads/` tree with **three** plugin
manifests — `.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`,
`.copilot-plugin/plugin.json` — and one shared body. This document generalizes
that pattern to include Hermes as a fourth target.

## 2. The anatomy of a multi-harness skill

```
my-skill/
├── SKILL.md                    # THE single source of truth (body + logic)
├── references/                 # linked docs loaded on demand
│   └── deep-dive.md
├── templates/                  # j2 / copy templates
├── scripts/                    # executable helpers
├── formulas/                   # .formula.toml workflow templates (bd)
│   └── do-a-thing.formula.toml
│
│   # --- per-harness adapters (thin) ---
├── .claude-plugin/plugin.json  # Claude Code manifest
├── .codex-plugin/plugin.json   # Codex manifest
├── .copilot-plugin/plugin.json # GitHub Copilot manifest (optional)
└── hermes/                     # Hermes is installed via its own pipeline;
    └── SKILL.md                #   see §4, no plugin.json
```

**One body, N adapters.** The knowledge lives in `SKILL.md` once. The
adapters only declare where the skill lives and how the harness should load it.

## 3. The per-harness manifest surface

### 3.1 Claude Code (`.claude-plugin/plugin.json`)

```json
{
  "name": "my-skill",
  "description": "...",
  "version": "1.0.0",
  "author": { "name": "...", "url": "..." },
  "repository": "https://github.com/org/repo",
  "license": "MIT",
  "homepage": "https://...",
  "keywords": ["..."],
  "skills": "./skills/",
  "commands": "./skills/my-skill/commands/",
  "hooks": { "SessionStart": [ { "matcher": "", "hooks": [ { "type": "command", "command": "..." } ] } ] }
}
```

Key fields: `skills` (dir pointer), `commands` (slash-command dir), `hooks`
(session lifecycle injection). `SKILL.md` frontmatter carries
`allowed-tools`, `compatible-with`, `tags`.

### 3.2 Codex (`.codex-plugin/plugin.json`)

```json
{
  "name": "my-skill",
  "version": "1.0.0",
  "description": "...",
  "author": { "name": "...", "url": "..." },
  "homepage": "...", "repository": "...", "license": "MIT",
  "keywords": ["..."],
  "skills": "./skills/",
  "hooks": "./.codex-plugin/hooks/hooks.json",
  "interface": {
    "displayName": "My Skill",
    "shortDescription": "...",
    "longDescription": "...",
    "developerName": "...",
    "category": "Developer Tools",
    "capabilities": ["Skills"],
    "websiteURL": "...",
    "defaultPrompt": ["Use $my-skill to ..."],
    "brandColor": "#4F7CAC"
  }
}
```

Key additions over Claude: the `interface` block (marketplace display
metadata) and an external `hooks.json` file. `SKILL.md` frontmatter is
minimal — often just `name` + `description`.

### 3.3 Hermes (no plugin.json — skills directory + curator)

Hermes loads skills from `~/.hermes/profiles/<profile>/skills/<category>/<name>/SKILL.md`.
No plugin manifest. The adapter is the **frontmatter**, which differs from
Claude/Codex:

```yaml
---
name: my-skill
description: "..."
version: 1.0.0
author: skillrx
license: MIT
platforms: [macos]
metadata:
  hermes:
    tags: [category, topic, ...]
---
```

Hermes-specific: `platforms` (OS gate), `metadata.hermes.tags` (curator
indexing), and lifecycle via the curator review pipeline + `skill_manage`
(create/patch/edit), not a manifest file.

### 3.4 Comparison table

| Field | Claude | Codex | Hermes |
|---|---|---|---|
| Manifest file | `.claude-plugin/plugin.json` | `.codex-plugin/plugin.json` | none (frontmatter) |
| `skills` dir pointer | ✅ | ✅ | implicit (category dir) |
| `commands` (slash) | ✅ | — | quick_commands |
| `hooks` lifecycle | ✅ | ✅ (external json) | cron / hooks |
| `interface` display block | — | ✅ | — |
| `allowed-tools` | ✅ | — | toolset config |
| `platforms` | — | — | ✅ |
| `tags` indexing | ✅ (flat) | — | ✅ (`metadata.hermes.tags`) |

## 4. Best practices for a multi-harness skill

1. **Single `SKILL.md` body.** Write the knowledge once. Never fork the body
   across harnesses — forked bodies drift.
2. **Thin adapters only.** Per-harness files are metadata, not content.
3. **Frontmatter is the divergence point.** Each harness reads a slightly
   different frontmatter schema; fill all three where they overlap (name,
   description, version, author, license) and add harness-only fields
   (`allowed-tools`, `platforms`, `interface`) alongside.
4. **Linked files live in `references/` (or `resources/`), not inline.**
   Claude and Hermes both lazy-load linked docs; keep `SKILL.md` under ~110
   lines and push detail into references.
5. **Formulas are package assets.** `.formula.toml` workflow templates ship
   with the skill so `bd cook` → `bd mol pour` can instantiate repeatable
   work from it. A formula is only usable when a skill explains *how* to pour
   it — "no one pours a formula they don't understand."
6. **Version once, everywhere.** A version bump touches the single source
   `version:` field and every manifest in lockstep.
7. **One package = one responsibility.** Don't bundle unrelated skills; the
   package is the unit of install and review.
8. **Verify each harness loads it.** Acceptance is *loaded and exercised* in
   all three (four) harnesses, not "the files exist."

## 5. How this lands in synaptic-canvas-dolt

- The package is a version-controlled unit in the sc-dolt registry; the three
  manifest files + `SKILL.md` + references + formulas are its contents.
- The registry schema (`docs/synaptic-canvas-schema.md`) already models
  packages, versions, and content-addressed files; a multi-harness package is
  a package whose manifest set spans harnesses.
- Discovery (`sc search`), install (`sc install`), and channels (branches)
  apply unchanged — the harness adapter is just more package content.

## 6. Open questions

- Does Hermes need a thin `hermes/SKILL.md` wrapper in the package, or is the
  curator pipeline (frontmatter-only) the adapter? Leaning: frontmatter-only.
- Is GitHub Copilot (`.copilot-plugin`) in scope, or claude+codex+hermes only?
- Naming: "multi-harness skill" vs "cross-harness plugin" — pick one and use
  it consistently across docs.
