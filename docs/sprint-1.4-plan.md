# Sprint 1.4 — Log Debug Agent: Implementation Plan

## Goal

Claude Code agent that monitors `sc` log output and surfaces warnings/errors.
Supports two deployment modes: background (deterministic turns) and persistent
(named agent in tmux pane, runs indefinitely via Claude agent teams).

---

## Files to Create

```
.claude/agents/sc-log-debug.md          # Agent definition
.claude/agents/registry.yaml            # Optional runner registry
.claude/scripts/
  sc_log_tail.py                        # Blocking tail w/ filter + timeout
  sc_log_query.py                       # Historical query (level/component/operation/time)
  sc_log_correlate.py                   # Group entries by operation + time proximity
  sc_log_common.py                      # Shared parsing/filter/time helpers
  test_sc_log_tail.py
  test_sc_log_query.py
  test_sc_log_correlate.py
```

---

## Log Format

Source: `src/internal/logging/logger.go` — `slog.NewJSONHandler`, one JSON object per
line, RFC3339Nano timestamp.

```json
{"time":"2026-04-07T14:23:45.123456789Z","level":"INFO","msg":"listing packages","component":"dolt","operation":"list"}
{"time":"2026-04-07T14:23:46.001Z","level":"ERROR","msg":"getting package \"foo\": connection refused","component":"dolt","operation":"get"}
```

### Known Fields

| Field | Type | Values |
|-------|------|--------|
| `time` | RFC3339Nano | always present |
| `level` | string | `INFO` `WARN` `ERROR` today; parser should also tolerate `DEBUG` for forward compatibility |
| `msg` | string | human-readable |
| `component` | string | arbitrary slog attr; currently `cli` is present, future values may include `dolt` `config` `output` `integrity` `installer` |
| `operation` | string | arbitrary slog attr; currently `init` is present, future values may include `list` `get` `get-files` `get-deps` `install` `import` `export` `verify` |

Additional context fields are arbitrary slog attrs appended per call site
(e.g. `"package_id":"foo"`, `"count":3`).

### Current Logging Constraints

- `src/internal/logging/logger.go` writes file logs at `INFO` level today.
- Helper scripts should still accept a `debug` filter for forward compatibility,
  even though current `sc.log` content should be expected to contain
  `INFO`/`WARN`/`ERROR` entries only.

### Log File Paths

- Current: `~/.sc/logs/sc.log`
- Rotated: `~/.sc/logs/sc-YYYY-MM-DD.log` (7-day retention)

---

## Script 1: `sc_log_tail.py` — Blocking tail with filter

Blocking read on `sc.log`. Unblocks on match OR timeout. Core primitive the
agent calls in a loop.

### Interface

```
python3 sc_log_tail.py [OPTIONS]

Options:
  --log PATH            Log file path (default: ~/.sc/logs/sc.log)
  --level LEVEL         Minimum level to match: debug|info|warn|error
  --component COMPONENT Filter by component field value
  --operation OPERATION Filter by operation field value
  --regex PATTERN       Regex applied to full JSON line
  --timeout SECS        Seconds before giving up (default: 30)
  --max-matches N       Stop after N matches (default: 1)
  --since-offset BYTES  Start reading from byte offset (enables cross-turn resumption)

Exit codes:
  0   One or more matches found
  1   Timeout — no matches within --timeout seconds
  2   Log file does not exist

Stdout: JSONL — one matched entry per line, with injected "_offset" field
Stderr: human-readable status (e.g. "Watching sc.log... timeout after 30s")
```

### Implementation Notes

- Seek to end of file before entering the blocking loop (tail behaviour)
- On each new line: parse JSON, apply all active filters, emit on match
- `_offset` field: current file byte position after reading the matched line;
  allows the agent to resume from exactly this point on the next turn
- Filters are AND-combined (level AND component AND operation AND regex)
- Level filter is `>=`: `--level warn` matches WARN and ERROR, not INFO/DEBUG
- Level comparison order: DEBUG=0, INFO=1, WARN=2, ERROR=3

### CLI Commands

```json
{
  "watch_errors":    "python3 ~/.claude/scripts/sc_log_tail.py --level warn --timeout 30",
  "watch_component": "python3 ~/.claude/scripts/sc_log_tail.py --level warn --component dolt --timeout 30",
  "watch_operation": "python3 ~/.claude/scripts/sc_log_tail.py --operation install --timeout 60",
  "watch_regex":     "python3 ~/.claude/scripts/sc_log_tail.py --regex 'sha256|integrity' --timeout 30",
  "watch_resume":    "python3 ~/.claude/scripts/sc_log_tail.py --level warn --timeout 30 --since-offset {offset}"
}
```

---

## Script 2: `sc_log_query.py` — Historical query

Reads current and optionally rotated logs. Used for "show me errors from the
last install" type queries.

### Interface

```
python3 sc_log_query.py [OPTIONS]

Options:
  --log PATH            Log file path (default: ~/.sc/logs/sc.log)
  --level LEVEL         Minimum level: debug|info|warn|error
  --component COMPONENT Filter by component
  --operation OPERATION Filter by operation
  --since SPEC          Time filter (see Parsing below)
  --until SPEC          Optional upper-bound time filter (same syntax as --since)
  --regex PATTERN       Regex applied to full line
  --include-rotated     Also search sc-YYYY-MM-DD.log files within time range
  --summary             Print counts by level only, no full entries
  --json                Output JSONL (default: human-readable table)

Exit codes:
  0   Results found
  1   No results matched filters
  2   Log file does not exist
```

### `--since` Parsing

| Input | Meaning |
|-------|---------|
| `5m` / `2h` / `1d` | Relative: N minutes/hours/days before now |
| `14:30` / `14:30:00` | Today at HH:MM[:SS] local time |
| `2026-04-07T14:00:00Z` | Absolute ISO8601 timestamp |

### Rotated File Handling (`--include-rotated`)

Scans `~/.sc/logs/sc-*.log` files whose date falls within the query window.
Required for queries that span midnight.

### CLI Commands

```json
{
  "query_recent_errors": "python3 ~/.claude/scripts/sc_log_query.py --level error --since 5m",
  "query_last_hour":     "python3 ~/.claude/scripts/sc_log_query.py --level warn --since 1h --include-rotated",
  "query_component":     "python3 ~/.claude/scripts/sc_log_query.py --component dolt --since 30m",
  "query_operation":     "python3 ~/.claude/scripts/sc_log_query.py --operation install --since 2h --include-rotated",
  "query_since_time":    "python3 ~/.claude/scripts/sc_log_query.py --level warn --since 14:30",
  "query_summary":       "python3 ~/.claude/scripts/sc_log_query.py --since 1h --summary",
  "query_context":       "python3 ~/.claude/scripts/sc_log_query.py --since 2026-04-07T14:23:15Z --until 2026-04-07T14:24:15Z --json"
}
```

---

## Script 3: `sc_log_correlate.py` — Operation correlation

Groups entries by `operation` field + time proximity. Reconstructs the full
lifecycle of a single `sc install` or `sc import` invocation.

### Interface

```
python3 sc_log_correlate.py [OPTIONS]

Options:
  --log PATH            Log file path (default: ~/.sc/logs/sc.log)
  --operation OPERATION Operation name to correlate (required)
  --since SPEC          Time window to search (same syntax as sc_log_query.py)
  --window SECS         Max gap between entries in the same run (default: 5)
  --include-rotated     Search rotated files
  --json                Output structured JSON per correlated run

Exit codes:
  0   Results found
  1   No results matched filters
  2   Log file does not exist
```

### Output (`--json`)

```json
[
  {
    "start": "2026-04-07T14:23:45Z",
    "end":   "2026-04-07T14:23:46Z",
    "entries": [ ...log entries... ],
    "outcome": "error"
  }
]
```

`outcome` values: `"error"` (any ERROR entry), `"warn"` (any WARN, no ERROR),
`"ok"` (INFO/DEBUG only).

### Correlation Algorithm

1. Pull all entries matching `--operation` within the time window
2. Sort by `time`
3. Split into runs: new run starts when gap between consecutive entries
   exceeds `--window` seconds
4. Annotate each run with `outcome` (error > warn > ok precedence)

### CLI Commands

```json
{
  "correlate_install": "python3 ~/.claude/scripts/sc_log_correlate.py --operation install --since 1h --json",
  "correlate_import":  "python3 ~/.claude/scripts/sc_log_correlate.py --operation import --since 30m --json",
  "correlate_get":     "python3 ~/.claude/scripts/sc_log_correlate.py --operation get --since 15m --json"
}
```

---

## Recommended Filters

| User intent | Invocation |
|-------------|-----------|
| Watch for any errors | `sc_log_tail.py --level error --timeout 30` |
| Watch dolt warnings | `sc_log_tail.py --level warn --component dolt --timeout 30` |
| Watch install progress | `sc_log_tail.py --operation install --timeout 60` |
| Watch integrity issues | `sc_log_tail.py --regex 'sha256\|MISSING\|CORRUPT' --timeout 30` |
| Errors from last install | `sc_log_correlate.py --operation install --since 1h --json` |
| Last 5 minutes | `sc_log_query.py --level warn --since 5m` |
| Errors since 2pm | `sc_log_query.py --level error --since 14:00` |
| Today's summary | `sc_log_query.py --since 24h --summary --include-rotated` |

---

## Test Strategy

Tests use synthetic JSONL fixtures — never touch the real `sc.log`.

### `test_sc_log_tail.py`

- Match found before timeout → exit 0, correct JSONL on stdout with `_offset`
- Timeout (no match within N seconds) → exit 1, empty stdout
- Level filter: `--level warn` matches WARN and ERROR, not INFO or DEBUG
- Component filter: `--component dolt` excludes entries with different component
- Operation filter: `--component install` excludes non-matching entries
- AND semantics: component + level combined → only entries matching both
- `--since-offset`: skips bytes before offset, matches only new entries
- `--max-matches 3`: stops after 3 matches
- Log file missing → exit 2

### `test_sc_log_query.py`

- `--since 5m` returns only entries within last 5 minutes
- `--since 14:30` parses as today at 14:30 local time
- ISO8601 `--since` parses correctly
- `--include-rotated` picks up entries from dated log files
- `--until` applies an upper-bound time filter
- `--summary` outputs counts by level, no individual entries
- No results → exit 1
- Log file missing → exit 2

### `test_sc_log_correlate.py`

- Single run: consecutive entries within window → one group, correct outcome
- Two runs: gap > window → two groups
- `outcome` precedence: ERROR wins over WARN wins over INFO
- Empty result when no entries match `--operation`
- `--window` respected: entries outside window start a new run
- Log file missing → exit 2

---

## Agent Definition (`sc-log-debug.md`)

### Frontmatter

```yaml
---
name: sc-log-debug
version: 1.0.0
description: >
  Monitors ~/.sc/logs/sc.log for warnings and errors. Supports two modes:
  background (fixed turns) and persistent (named agent in tmux, indefinite).
  Invokes Python helper scripts for blocking tail, historical queries, and
  operation correlation.
---
```

### Output Contract

Agent responses should follow the shared skills/agents guidance:

- Return fenced JSON for machine-readable result payloads.
- Prefer the minimal response envelope for normal operations:

```json
{
  "success": true,
  "data": {
    "mode": "background",
    "summary": "1 warning from dolt/install",
    "matches": []
  },
  "error": null
}
```

- The CLI is expected to be available on the command line in later sprints, but
  Sprint 1.4 must not depend on `sc` being invokable yet. The agent should
  interact directly with `~/.sc/logs/sc.log` via the Python helper scripts.

### Background Mode

Agent receives `mode=background`, `turns=N`, optional initial filter args.

Each turn:
1. Call `sc_log_tail.py` with stored `--since-offset` (or seek to end on
   first turn)
2. Exit 0 → format matches, store new `_offset`, report to user
3. Exit 1 (timeout) → report "quiet — no warnings/errors"
4. Decrement turn counter; exit when 0

### Persistent Mode (tmux / named agent)

Agent receives `mode=persistent`. Runs indefinitely:

1. Same tail loop, no turn budget
2. On match: call `SendMessage` to the main conversation with a concise
   summary (level, component, operation, msg, timestamp)
3. On user message: handle requests — filter changes, "explain this error",
   "show context around this entry", "summarize last hour"

### Explaining Error Context

When asked to explain an error, call `sc_log_query.py` with `--since` and
`--until` spanning a ±30s window around the error's `time` field to retrieve
surrounding entries, then present the full operation sequence.

---

## Acceptance Criteria (from `docs/project-plan.md`)

- [ ] Agent can be launched to monitor logs during development/testing
- [ ] Detects and reports new warnings/errors as they appear
- [ ] Filters work: `level:error`, `component:dolt`, `operation:install`, custom regex
- [ ] Time-range filtering: "last 5 minutes", "since 14:30"
- [ ] Output is concise — summarizes patterns, does not dump raw logs
- [ ] Can be asked to explain error context (surrounding log lines)
- [ ] All Python scripts have passing unit tests (synthetic fixtures, no real sc.log)

---

## Review Notes / Resolved Gaps

- Script paths were standardized to `.claude/scripts/` so the plan matches the
  repository layout for local helper scripts.
- Added `--until` to `sc_log_query.py`; without it, bounded context lookup for a
  specific error timestamp was underspecified.
- Tightened the agent definition to include `version` and a structured output
  contract aligned with the shared skills/agents architecture guidance.
- Clarified that Sprint 1.4 must not depend on `sc` being on `PATH` yet; the
  agent operates directly on the log files and helper scripts.
- Clarified that current file logs are `INFO`+ only, while helper filters still
  accept `debug` for forward compatibility.
