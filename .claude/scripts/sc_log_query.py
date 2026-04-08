#!/usr/bin/env python3
"""Historical querying for Synaptic Canvas JSON logs."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from sc_log_common import (
    compile_regex,
    default_log_path,
    entry_matches,
    iter_log_entries,
    json_dumps,
    parse_level,
    parse_time_spec,
    resolve_log_paths,
    summarize_levels,
    utc_now,
)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--log", default=str(default_log_path()), help="log file path")
    parser.add_argument("--level", help="minimum level: debug|info|warn|error")
    parser.add_argument("--component", help="filter by component")
    parser.add_argument("--operation", help="filter by operation")
    parser.add_argument("--since", help="lower-bound time filter")
    parser.add_argument("--until", help="upper-bound time filter")
    parser.add_argument("--regex", help="regex applied to the full line")
    parser.add_argument("--include-rotated", action="store_true", help="include rotated logs")
    parser.add_argument("--summary", action="store_true", help="print counts by level only")
    parser.add_argument("--json", action="store_true", help="output JSON/JSONL")
    return parser


def render_table(entries: list[dict]) -> None:
    print("TIME\tLEVEL\tCOMPONENT\tOPERATION\tMSG")
    for entry in entries:
        print(
            f"{entry.get('time','')}\t{entry.get('level','')}\t"
            f"{entry.get('component','')}\t{entry.get('operation','')}\t"
            f"{entry.get('msg','')}"
        )


def main() -> int:
    args = build_parser().parse_args()
    log_path = Path(args.log).expanduser()
    if not log_path.exists():
        print(f"log file does not exist: {log_path}", file=sys.stderr)
        return 2

    now = utc_now()
    min_level = parse_level(args.level)
    since = parse_time_spec(args.since, now)
    until = parse_time_spec(args.until, now)
    regex = compile_regex(args.regex)

    entries = [
        parsed
        for parsed in iter_log_entries(resolve_log_paths(log_path, args.include_rotated))
        if entry_matches(
            parsed,
            min_level=min_level,
            component=args.component,
            operation=args.operation,
            regex=regex,
            since=since,
            until=until,
        )
    ]
    entries.sort(key=lambda parsed: parsed.timestamp)

    if not entries:
        return 1

    if args.summary:
        counts = summarize_levels(entries)
        if args.json:
            print(json_dumps(counts))
        else:
            print("LEVEL\tCOUNT")
            for level, count in counts.items():
                if count:
                    print(f"{level}\t{count}")
        return 0

    payload = [parsed.entry for parsed in entries]
    if args.json:
        for entry in payload:
            print(json_dumps(entry))
    else:
        render_table(payload)

    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(2) from exc
