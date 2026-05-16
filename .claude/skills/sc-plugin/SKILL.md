---
name: sc-plugin
description: Thin Claude skill wrapper for Synaptic Canvas package management through `sc --json`.
---

# sc:plugin

Use this skill when the user wants to manage Synaptic Canvas packages through
natural language instead of typing `sc` commands directly.

## Scope

This skill is a thin wrapper over the `sc` CLI.

- Always shell out to `sc` with `--json`.
- Treat CLI JSON as the only machine contract.
- Never parse human-readable CLI output.
- Never recreate package-management business rules in the skill.
- Never absorb ATM polling, sprint orchestration, git workflow, or team-level
  execution control. Those belong to higher-level orchestration layers.

## Command Mapping Rules

Translate the user request into the narrowest matching `sc --json` command.

- "list packages" -> `sc list --json`
- "show team-lead" -> `sc info team-lead --json`
- "install team-lead" -> `sc install team-lead --json`
- "install team-lead from beta globally" -> `sc install team-lead --branch beta --scope global --json`
- "upgrade team-lead in this repo" -> `sc upgrade team-lead --scope project --json`
- "upgrade team-lead to 1.2.0 on beta locally" -> `sc upgrade team-lead --branch beta --version 1.2.0 --scope project --json`
- "upgrade team-lead to 1.3.0 on beta in this repo" -> `sc upgrade team-lead --branch beta --version 1.3.0 --scope project --json`
- "uninstall team-lead globally" -> `sc uninstall team-lead --scope global --json`
- "validate team-lead here" -> `sc validate team-lead --scope project --json`
- "validate team-lead in this repo" -> `sc validate team-lead --scope project --json`
- "show package status" -> `sc status --json`
- "snapshot team-lead" -> `sc snapshot team-lead --json`
- "snapshot team-lead globally" -> `sc snapshot team-lead --scope global --json`

## Selection Rules

- Preserve explicit user selections for `--branch`, `--version`, and `--scope`.
- If the user does not specify a scope for scope-aware commands, allow the CLI
  to enforce its own default or ambiguity behavior.
- Do not silently choose a scope when the CLI returns `ambiguous_target`.
- Do not rewrite backend-specific failures. Surface the top-level error plus
  any structured corrective guidance from the JSON payload.

## Response Rules

For successful commands:
- summarize the outcome from the JSON payload
- keep package, scope, branch, and version explicit when present
- use structured readback fields like validation items, dependency summary, or
  snapshot metadata when they exist

For failures:
- show the CLI `error.code`
- summarize `error.message`
- include corrective fields from `error.details` or `suggested_action` when present
- ask for missing disambiguation only when the CLI contract requires it

## Forbidden Behavior

- No parsing of table output.
- No hidden fallback to non-JSON commands.
- No synthetic success/failure schema separate from the CLI contract.
- No inference that overrides explicit CLI ambiguity or backend errors.

## Review Aids

Use the example files in `examples/` as the normative utterance-to-command
mapping set for manual QA and regression review.
