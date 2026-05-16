# ADR-0003: `sc:plugin` Remains A Thin Package-Management Wrapper

## Status

Accepted

## Context

The project needs an AI-facing wrapper so Claude can use Synaptic Canvas
conversationally. At the same time, team workflows may include ATM polling,
sprint loops, repository orchestration, and other autonomous behaviors that are
broader than package management.

If `sc:plugin` absorbs those concerns, package-management behavior and team
orchestration will become coupled and difficult to review or validate.

## Decision

- `sc:plugin` is a thin package-management wrapper over `sc --json`.
- It may map natural-language package intents to explicit CLI invocations and
  render structured results for the user.
- It does not redefine package-management business rules, backend-selection
  policy, mutation safety, or audit semantics.
- It does not own ATM polling, autonomous sprint execution, git orchestration,
  or general team workflow control.
- Higher-level orchestration belongs in separate skills or agents above the
  package-management layer.

## Consequences

- Package-management correctness remains testable in the CLI itself.
- ATM and other orchestration behavior can evolve independently.
- Quality review can reject wrapper changes that sneak business logic or team
  workflow policy into `sc:plugin`.
