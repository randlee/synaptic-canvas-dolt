# ADR-0004: Package-Management Policy Lives Behind Shared Operation Boundaries

## Status

Accepted

## Context

Synaptic Canvas is adding a richer machine contract, three Dolt backends, and
an AI-facing wrapper. Without a hard boundary between command-entrypoint code
and package-management policy, the implementation will drift into duplicated
logic across Cobra handlers, wrappers, tests, and transport-specific branches.

That drift is especially risky for MVP because quality review needs one place
to validate package behavior and one place to enforce the public contract.

## Decision

- `src/cmd` remains a binding layer for flags, process-level setup, and output
  rendering.
- Shared public JSON DTOs live outside command handlers.
- End-user package workflows such as install, upgrade, uninstall, status,
  validate, scan, and snapshot are implemented behind shared operation-layer
  packages or interfaces below the Cobra layer.
- Transport adapters, filesystem mutation, catalog access, and tracking state
  remain behind their own interfaces or package seams.
- Wrapper surfaces such as `sc:plugin` call the canonical CLI contract and do
  not become a second implementation of package-management policy.

## Consequences

- Quality review can verify package behavior at stable seams rather than at
  every command entrypoint.
- Contract tests, adapter tests, and workflow tests can be layered cleanly.
- Refactors to transport or rendering are less likely to change business
  behavior accidentally.
