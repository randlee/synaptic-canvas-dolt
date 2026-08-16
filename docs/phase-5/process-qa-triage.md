# Phase 5 Process, QA, And Triage

Phase 5 uses completion reports as structured input to Sprint 5.6 hardening.
This document defines that triage flow.

## Inputs

Each Sprint `5.1–5.5` completion report must record:

- issues encountered
- fixes completed in that sprint
- unresolved items
- whether each unresolved item is still MVP-scope

## Triage Decision Gate

Every unresolved item from Sprint `5.1–5.5` must be assigned one of these
dispositions before Sprint 5.6 closes:

1. `fix-in-5.6`
   - MVP-scope
   - blocks release readiness or product proof

2. `defer-post-mvp`
   - valid issue, but outside approved MVP scope

3. `defer-later-phase`
   - in product scope, but requires a later planned phase such as Phase 6

4. `not-a-finding`
   - false positive or already resolved elsewhere

## QA Routing

- Sprint-specific QA validates the sprint doc and implementation evidence.
- Sprint 5.6 validates cross-sprint closure and the final readiness report.
- Skill QA for `.claude/skills/importing-sc-packages` must use
  `/Users/randlee/Documents/github/synaptic-canvas/docs/claude-code-skills-agents-guidelines.md`.

## Final Readiness Report

Sprint 5.6 writes the final readiness report to:

- `docs/phase-5/phase-5-readiness-report.md`

That report must list:

- closed follow-up items
- deferred items and rationale
- final validation reruns
- PASS/FAIL release-readiness verdict
