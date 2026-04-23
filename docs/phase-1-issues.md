# Phase 1 Issues

This document captures the substantive implementation issues found during a
critical review of Phase 1 work in `synaptic-canvas-dolt`, covering Sprints
1.1 through 1.4.

The review was performed against the Phase 1 review branch at commit
`cb6bd52512115024bfe81623396d4b27a659589d` (`fix/phase-1-issues`). Some of the
issues below may have been addressed by later Phase 2 work; this document is a
record of what the review found on that reviewed commit, not a claim that every
finding still applies to the current `phase2/integration` tree.

## Findings

### 1. Dolt branch switching is unsafe with `database/sql` pooling

Severity: Critical

At the reviewed commit, `src/pkg/dolt/client.go` switched branches by issuing
`USE database/branch` through `*sql.DB`, then ran the actual query as a
separate call. Because `database/sql` may execute those statements on
different pooled connections, branch selection was not guaranteed to apply to
the subsequent query.

Impact:
- `ListPackages(..., Branch: ...)` can silently read from the wrong Dolt branch
- The "branches are channels" design contract is not reliably enforced

Relevant code at review time:
- `src/pkg/dolt/client.go`

Recommended correction:
- Pin branch-switched operations to a single connection or transaction-scoped
  session, or redesign query execution so branch selection does not rely on a
  separate session mutation against a pooled `*sql.DB`.

### 2. Required logging context is not propagated consistently

Severity: Important

At the reviewed commit, Phase 1 required standard log attributes on every log
entry, specifically `component` and `operation`. The root command created a
contextual logger, but that enriched logger was not installed as the global
default. Lower layers, such as the Dolt client, called package-level
`slog.Debug(...)` directly, which meant those entries did not reliably include
the required context fields.

Impact:
- Real log output is inconsistent with the documented logging contract
- Sprint 1.4 agent filtering by `component` and `operation` is unreliable

Relevant code at review time:
- `src/cmd/root.go`
- `src/pkg/dolt/client.go`

Recommended correction:
- Ensure the logger used across the process always includes standard context, or
  require each subsystem to derive and use contextual loggers consistently.

### 3. File logging can fail silently during setup

Severity: Important

At the reviewed commit, the project plan required file logging to be always on.
In `src/internal/logging/logger.go`, if `fileHandler()` failed, `Setup()`
silently skipped it and continued with console-only or fallback logging.

Impact:
- `~/.sc/logs/sc.log` may not exist even though the CLI appears healthy
- The Sprint 1.4 log-debug agent can fail because its expected log source was
  never created
- Operators receive no explicit signal that required file logging is broken

Relevant code at review time:
- `src/internal/logging/logger.go`

Recommended correction:
- Surface file-handler initialization failures explicitly, either by returning
  an error from setup or by emitting a clear fallback diagnostic that makes the
  degraded logging state visible.

### 4. Phase 1 Python test suite is currently non-deterministic and failing

Severity: Important

The Phase 1 Python log-script tests were not green in the review environment.
`test_since_today_clock_time_filters_entries` assumes `--since 14:30` means
"today at 14:30", but the implementation rolls a future local clock time back
to the previous day. Before 14:30 local time, the test includes both fixture
rows and fails.

Impact:
- The Phase 1.4 script suite is not reliably passable
- CI and local verification can fail depending on time of day

Relevant code at review time:
- `.claude/scripts/test_sc_log_query.py`
- `.claude/scripts/sc_log_common.py`

Observed failure:

```text
FAIL: test_since_today_clock_time_filters_entries
AssertionError: ['before', 'after'] != ['after']
```

Recommended correction:
- Make the test deterministic by fixing the reference time it uses, or update
  the fixture/setup so it validates the intended behavior independent of the
  current wall clock.

## Verification Notes

Commands run during review:

```bash
cd src && go test ./...
python3 -m unittest discover -s .claude/scripts -p 'test_*.py'
```

Results:
- Go tests passed
- Python unit tests failed due to the time-dependent query test described above
