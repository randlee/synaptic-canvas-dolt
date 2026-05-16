# sc:plugin

`sc:plugin` is the Claude-facing wrapper for Synaptic Canvas package
management. It does not implement package logic itself. It delegates to
`sc --json`, interprets only the structured JSON contract, and presents the
result conversationally.

## Boundaries

- In scope: package list, info, install, upgrade, uninstall, validate,
  status, snapshot
- Out of scope: ATM polling, autonomous sprint loops, git branching, PR
  orchestration, repository task routing

## Verification Model

Manual QA for this skill is fixture-backed. Review each example in
`examples/` and verify:

1. The utterance maps to the exact `sc --json` command shown.
2. Explicit `--branch`, `--version`, and `--scope` selections are preserved.
3. The expected success or error shape comes from the CLI contract rather than
   a wrapper-owned schema.
4. Ambiguity and backend failures are surfaced, not hidden.

## Examples

- `examples/install-beta-global.md`
- `examples/upgrade-version-project.md`
- `examples/uninstall-global.md`
- `examples/status-local-global.md`
- `examples/validate-project.md`
- `examples/snapshot-ambiguous.md`
- `examples/backend-failure-http.md`
