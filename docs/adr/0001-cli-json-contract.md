# ADR-0001: `sc --json` Is The Canonical Automation Contract

## Status

Accepted

## Context

Synaptic Canvas is expected to be used heavily by AI wrappers. Human-readable
CLI output is useful, but it cannot be the source of truth for automation,
quality review, or future wrapper or MCP surfaces.

Phase 4 also depends on a thin `sc:plugin` wrapper. Without a canonical
machine contract, the wrapper will either become brittle or absorb business
logic that belongs in `sc`.

## Decision

- `sc --json` is the canonical public automation contract for end-user CLI
  operations.
- Success and error payloads are typed and documented before wrapper behavior
  is considered complete.
- Bootstrap failures in `--json` mode use the same error-envelope family as
  command-level failures.
- Human-readable output is a renderer over the canonical machine contract, not
  a richer parallel surface.
- Wrappers and future MCP layers reuse the same business payloads and stable
  error-code families rather than translating to a second canonical schema.

## Consequences

- Command handlers should use shared DTOs rather than ad hoc maps.
- Quality review can assert JSON contracts directly.
- Wrapper work is blocked on contract hardening rather than papering over CLI
  ambiguity in skill logic.
