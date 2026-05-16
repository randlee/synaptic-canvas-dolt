# ADR-0005: Atomic Mutation And No-Stale-Lock Coordination

## Status

Accepted

## Context

Synaptic Canvas mutates both runtime artifact trees and product-managed
tracking state. The project explicitly requires that stale lock artifacts be
impossible by design and that interruptions not leave the system permanently
blocked.

That requirement is too important to leave as broad prose only. Without a
focused decision, implementations may drift toward ad hoc lock files, partial
tracking updates, or mutation flows that require manual cleanup after failure.

## Decision

- Product-managed mutation coordination is centered on atomic staging and
  replacement, not persistent lock files.
- Runtime artifact trees such as `.claude/` and `~/.claude/` are never held
  under long-lived product locks.
- Where mutual exclusion is still needed, self-cleaning OS-backed locks may be
  used as a secondary guard, but the design must not depend on process-exit
  cleanup as the sole stale-lock prevention mechanism.
- Tracked state is committed atomically per targeted install scope only after
  the corresponding mutation workflow reaches a consistent checkpoint.
- If a mutation fails mid-flight, the CLI either rolls the targeted scope back
  to the prior tracked state or returns an explicit rollback summary naming the
  remaining recovery work.

## Consequences

- Mutation workflows need a clear staging/commit/rollback model.
- Quality review can reject implementations that introduce orphanable lock
  files or silent partial state.
- Readback and error reporting must expose rollback outcomes rather than hiding
  them in logs.
