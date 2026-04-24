#!/usr/bin/env python3
"""Correlate Synaptic Canvas log entries into operation runs."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from sc_log_common import (
    default_log_path,
    entry_matches,
    iter_log_entries,
    json_dumps,
    local_now,
    parse_time_spec,
    resolve_log_paths,
)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--log", default=str(default_log_path()), help="log file path")
    parser.add_argument("--operation", required=True, help="operation name to correlate")
    parser.add_argument("--since", help="time filter")
    parser.add_argument("--window", type=float, default=5.0, help="max gap in seconds")
    parser.add_argument("--include-rotated", action="store_true", help="include rotated logs")
    parser.add_argument("--json", action="store_true", help="output structured JSON")
    return parser


def outcome_for(entries: list[dict]) -> str:
    levels = {str(entry.get("level", "")).upper() for entry in entries}
    if "ERROR" in levels:
        return "error"
    if "WARN" in levels:
        return "warn"
    return "ok"


def correlate(entries: list[tuple[object, dict]], window_seconds: float) -> list[dict]:
    runs: list[dict] = []
    current: list[tuple[object, dict]] = []
    last_timestamp = None

    for timestamp, entry in entries:
        if last_timestamp is None or (timestamp - last_timestamp).total_seconds() <= window_seconds:
            current.append((timestamp, entry))
        else:
            runs.append(build_run(current))
            current = [(timestamp, entry)]
        last_timestamp = timestamp

    if current:
        runs.append(build_run(current))

    return runs


def build_run(entries: list[tuple[object, dict]]) -> dict:
    clean_entries = [entry for _, entry in entries]
    return {
        "start": clean_entries[0]["time"],
        "end": clean_entries[-1]["time"],
        "entries": clean_entries,
        "outcome": outcome_for(clean_entries),
    }


def render_human(runs: list[dict]) -> None:
    print("START\tEND\tOUTCOME\tCOUNT")
    for run in runs:
        print(f"{run['start']}\t{run['end']}\t{run['outcome']}\t{len(run['entries'])}")


def main() -> int:
    args = build_parser().parse_args()
    log_path = Path(args.log).expanduser()
    if not log_path.exists():
        print(f"log file does not exist: {log_path}", file=sys.stderr)
        return 2

    since = parse_time_spec(args.since, local_now())
    entries: list[tuple[object, dict]] = []
    for parsed in iter_log_entries(resolve_log_paths(log_path, args.include_rotated, since=since)):
        if entry_matches(parsed, operation=args.operation, since=since):
            entries.append((parsed.timestamp, dict(parsed.entry)))

    entries.sort(key=lambda item: item[0])
    if not entries:
        return 1

    runs = correlate(entries, args.window)
    if args.json:
        print(json_dumps(runs))
    else:
        render_human(runs)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(2) from exc
