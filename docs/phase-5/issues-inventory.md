# Phase 5 Issues Inventory

This document is the Phase 5 hardening task list and the running inventory of
known planning issues.

## Audit Iteration 1 Findings

| Finding | Document / Section | Type | Description | Status |
|---|---|---|---|---|
| F5-001 | `docs/phase-5/5.1-release-pipeline.md` and merged release baseline | `CONTRA` | Sprint 5.1 still describes Winget as disabled/deferred even though the merged `develop` baseline enables the Winget publish path in `.goreleaser.yml`. | Resolved |
| F5-002 | missing `docs/phase-5/README.md` | `GAP` | Phase 5 has sprint docs but no dedicated phase plan document covering the whole remaining phase. | Resolved |
| F5-003 | missing `docs/adr/README.md` | `GAP` | The hardening-required ADR index does not exist as a document even though accepted ADRs are referenced throughout the plan set. | Resolved |
| F5-004 | missing `docs/phase-5/issues-inventory.md` | `GAP` | Phase 5 has no issues inventory tracking known findings and their disposition. | Resolved |
| F5-005 | missing `docs/phase-5/testing-cross-platform.md` | `GAP` | Phase 5 touches cross-platform release and installer behavior but lacks a dedicated testing/cross-platform guidance doc. | Resolved |
| F5-006 | missing `docs/phase-5/process-qa-triage.md` | `GAP` | Phase 5 now depends on completion-report triage and Sprint 5.6 follow-up closure but lacks a process/QA/triage document defining that flow. | Resolved |
| F5-007 | `docs/phase-5/5.6-phase-5-hardening-and-follow-up-closure.md` | `VAGUE` | Sprint 5.6 names a final readiness report artifact but does not define its target path or the triage decision points for in-scope versus deferred findings. | Resolved |

## Current Disposition

- No open findings remain after Iteration 1 fixes.
- New findings discovered during Phase 5 implementation should be appended
  below with sprint source, disposition, and follow-up target.

## Audit Iteration 2 Result

- Finding count: `0`
- Additional fixes required: `none`

## Hardening Complete

HARDENING COMPLETE  
Iterations: `2`  
Total findings resolved: `7`  
Final finding count: `0`  
Dev-agent decision points eliminated: `7`  
Documents modified:
- `docs/requirements.md`
- `docs/architecture.md`
- `docs/project-plan.md`
- `docs/synaptic-canvas-cli.md`
- `docs/phase-5/5.1-release-pipeline.md`
- `docs/phase-5/5.6-phase-5-hardening-and-follow-up-closure.md`
- `templates/sprint-plan.md.j2`

Documents created:
- `docs/adr/README.md`
- `docs/phase-5/README.md`
- `docs/phase-5/issues-inventory.md`
- `docs/phase-5/testing-cross-platform.md`
- `docs/phase-5/process-qa-triage.md`
- `docs/phase-5/phase-5-readiness-report.md`
- `docs/phase-5/5.2-first-import-candidate-and-package-normalization.md`
- `docs/phase-5/5.3-local-dolt-clone-smoke-test.md`
- `docs/phase-5/5.4-dolthub-smoke-test.md`
- `docs/phase-5/5.5-candidate-package-expansion-and-regression-pass.md`
- `docs/phase-5/5.6-phase-5-hardening-and-follow-up-closure.md`
