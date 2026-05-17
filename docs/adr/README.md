# ADR Index

This index tracks the accepted ADR set that is normative for Synaptic Canvas.

## Accepted ADRs

- `ADR-0001` — [0001-cli-json-contract.md](./0001-cli-json-contract.md)
- `ADR-0002` — [0002-dolt-client-selection.md](./0002-dolt-client-selection.md)
- `ADR-0003` — [0003-sc-plugin-boundary.md](./0003-sc-plugin-boundary.md)
- `ADR-0004` — [0004-module-boundaries-and-operation-layer.md](./0004-module-boundaries-and-operation-layer.md)
- `ADR-0005` — [0005-atomic-mutation-and-no-stale-locks.md](./0005-atomic-mutation-and-no-stale-locks.md)

## Phase 5 Notes

- No new ADR is introduced by Phase 5 hardening.
- Phase 5 execution depends primarily on `ADR-0001` for the public `sc --json`
  contract and `ADR-0003` for the thin-wrapper boundary.
