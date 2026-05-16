# ADR-0002: Three First-Class Dolt Clients Behind One Contract

## Status

Accepted

## Context

Synaptic Canvas must support:

- `HTTPClient` for DoltHub HTTP SQL API access
- `SQLClient` for hosted or self-hosted MySQL-compatible Dolt servers
- `CLIReader` for local Dolt clones via `dolt sql -q`

These backends expose different transports and failure modes, but AI callers
must see one stable CLI contract.

MVP admin and other write-path operations are narrower: the DoltHub HTTP SQL
API is read-oriented for this product, while writable workflows depend on a
MySQL-compatible server or a writable local clone.

## Decision

- All three client modes are first-class supported transports.
- All three client modes are first-class supported read transports.
- Client selection is explicit and deterministic, with documented precedence.
- Compatibility inference may exist, but conflicting transport inputs fail with
  typed CLI errors rather than silently choosing a backend.
- MVP write-path admin commands use `SQLClient` or `CLIReader`; they reject
  `HTTPClient` with a typed `unsupported_backend` error rather than silently
  switching transports.
- Equivalent success and failure classes normalize to the same top-level CLI
  schemas regardless of backend.
- Backend-specific facts surface through structured details, including client
  mode and backend-local cause codes.
- Shared conformance tests and adapter-level simulator-backed tests are
  required for each client mode.

## Consequences

- The CLI cannot treat HTTP as the only production path.
- Backend selection rules must distinguish read-path parity from write-path
  capability explicitly.
- Backend work must be designed around conformance, not just local adapter
  correctness.
- Live backend verification remains a manual or AI-driven integration activity,
  not routine CI.
