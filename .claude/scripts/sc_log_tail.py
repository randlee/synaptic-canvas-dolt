#!/usr/bin/env python3
"""Blocking tail with filters for Synaptic Canvas JSON logs."""

from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

from sc_log_common import compile_regex, default_log_path, entry_matches, parse_json_line, parse_level


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--log", default=str(default_log_path()), help="log file path")
    parser.add_argument("--level", help="minimum level: debug|info|warn|error")
    parser.add_argument("--component", help="filter by component")
    parser.add_argument("--operation", help="filter by operation")
    parser.add_argument("--regex", help="regex applied to full JSON line")
    parser.add_argument("--timeout", type=float, default=30.0, help="seconds before timing out")
    parser.add_argument("--max-matches", type=int, default=1, help="stop after N matches")
    parser.add_argument("--since-offset", type=int, help="resume from byte offset")
    return parser


def main() -> int:
    args = build_parser().parse_args()
    log_path = Path(args.log).expanduser()
    if not log_path.exists():
        print(f"log file does not exist: {log_path}", file=sys.stderr)
        return 2

    min_level = parse_level(args.level)
    regex = compile_regex(args.regex)
    deadline = time.monotonic() + args.timeout
    matches = 0

    print(
        f"Watching {log_path} for up to {args.timeout:g}s",
        file=sys.stderr,
    )

    with log_path.open("rb") as handle:
        if args.since_offset is not None:
            handle.seek(args.since_offset)
        else:
            handle.seek(0, 2)

        while time.monotonic() <= deadline:
            line = handle.readline()
            if not line:
                time.sleep(0.1)
                continue

            offset = handle.tell()
            decoded = line.decode("utf-8", errors="replace").rstrip("\n")
            parsed = parse_json_line(decoded, log_path)
            if parsed is None:
                continue

            if not entry_matches(
                parsed,
                min_level=min_level,
                component=args.component,
                operation=args.operation,
                regex=regex,
            ):
                continue

            entry = dict(parsed.entry)
            entry["_offset"] = offset
            print(json.dumps(entry, sort_keys=True), flush=True)
            matches += 1
            if matches >= args.max_matches:
                return 0

    print(f"timeout after {args.timeout:g}s with no matches", file=sys.stderr)
    return 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(2) from exc
