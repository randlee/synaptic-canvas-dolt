# Phase 5 — Release Readiness And Product Proof

Phase 5 hardens the product from two directions at once:

- release readiness for the `sc` binary and package-management surface
- product proof through real package import and live backend smoke testing

This phase is not complete when only the publish pipeline is green. It is
complete when the release surface, first-package import path, local-clone path,
live DoltHub path, and follow-up hardening loop have all been executed and
recorded.

## Sprint Set

- [5.1 Release Pipeline](./5.1-release-pipeline.md)
- [5.2 First Import Candidate And Package Normalization](./5.2-first-import-candidate-and-package-normalization.md)
- [5.3 Local Dolt Clone Smoke Test](./5.3-local-dolt-clone-smoke-test.md)
- [5.4 DoltHub Smoke Test](./5.4-dolthub-smoke-test.md)
- [5.5 Candidate Package Expansion And Regression Pass](./5.5-candidate-package-expansion-and-regression-pass.md)
- [5.6 Phase-5 Hardening And Follow-Up Closure](./5.6-phase-5-hardening-and-follow-up-closure.md)

## Phase-Level Supporting Docs

- [Issues Inventory](./issues-inventory.md)
- [Testing And Cross-Platform Guidance](./testing-cross-platform.md)
- [Process, QA, And Triage](./process-qa-triage.md)

## Exit Condition

Phase 5 is execution-ready only when:

- every sprint doc is present and internally consistent
- the release baseline matches the merged `develop` state
- the `importing-sc-packages` skill is explicitly reviewed against
  `/Users/randlee/Documents/github/synaptic-canvas/docs/claude-code-skills-agents-guidelines.md`
- follow-up findings from Sprint `5.1–5.5` have a concrete closure path in
  Sprint `5.6`
