# Phase 1 Issues

This document captures the substantive implementation issues found during a
critical review of Phase 1 work in `synaptic-canvas-dolt`, covering Sprints
1.1 through 1.4.

## Findings

### 1. Dolt branch switching is unsafe with `database/sql` pooling

Severity: Critical

`src/pkg/dolt/client.go` switches branches by issuing `USE database/branch`
through `*sql.DB`, then runs the actual query as a separate call. Because
`database/sql` may execute those statements on different pooled connections,
branch selection is not guaranteed to apply to the subsequent query.

Impact:
- `ListPackages(..., Branch: ...)` can silently read from the wrong Dolt branch
- The "branches are channels" design contract is not reliably enforced

Relevant code:
- `src/pkg/dolt/client.go:115`
- `src/pkg/dolt/client.go:129`

Recommended correction:
- Pin branch-switched operations to a single connection or transaction-scoped
  session, or redesign query execution so branch selection does not rely on a
  separate session mutation against a pooled `*sql.DB`.

### 2. Required logging context is not propagated consistently

Severity: Important

Phase 1 requires standard log attributes on every log entry, specifically
`component` and `operation`. The root command creates a contextual logger, but
that enriched logger is not installed as the global default. Lower layers, such
as the Dolt client, call package-level `slog.Debug(...)` directly, which means
those entries do not reliably include the required context fields.

Impact:
- Real log output is inconsistent with the documented logging contract
- Sprint 1.4 agent filtering by `component` and `operation` is unreliable

Relevant code:
- `src/cmd/root.go:40`
- `src/pkg/dolt/client.go:122`
- `src/pkg/dolt/client.go:135`

Recommended correction:
- Ensure the logger used across the process always includes standard context, or
  require each subsystem to derive and use contextual loggers consistently.

### 3. File logging can fail silently during setup

Severity: Important

The project plan requires file logging to be always on. In
`src/internal/logging/logger.go`, if `fileHandler()` fails, `Setup()` silently
skips it and continues with console-only or fallback logging.

Impact:
- `~/.sc/logs/sc.log` may not exist even though the CLI appears healthy
- The Sprint 1.4 log-debug agent can fail because its expected log source was
  never created
- Operators receive no explicit signal that required file logging is broken

Relevant code:
- `src/internal/logging/logger.go:43`
- `src/internal/logging/logger.go:55`

Recommended correction:
- Surface file-handler initialization failures explicitly, either by returning
  an error from setup or by emitting a clear fallback diagnostic that makes the
  degraded logging state visible.

### 4. Phase 1 Python test suite is currently non-deterministic and failing

Severity: Important

The Phase 1 Python log-script tests are not green in this environment.
`test_since_today_clock_time_filters_entries` assumes `--since 14:30` means
"today at 14:30", but the implementation rolls a future local clock time back
to the previous day. Before 14:30 local time, the test includes both fixture
rows and fails.

Impact:
- The Phase 1.4 script suite is not reliably passable
- CI and local verification can fail depending on time of day

Relevant code:
- `/.claude/scripts/test_sc_log_query.py:48`
- `/.claude/scripts/sc_log_common.py:86`

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
