#!/usr/bin/env python3
"""Shared helpers for Synaptic Canvas log inspection scripts."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from datetime import date, datetime, timedelta
from pathlib import Path
from typing import Iterable


LEVEL_ORDER = {
    "DEBUG": 0,
    "INFO": 1,
    "WARN": 2,
    "ERROR": 3,
}

RELATIVE_TIME_RE = re.compile(r"^(?P<amount>\d+)(?P<unit>[mhd])$")
LOCAL_TIME_RE = re.compile(r"^(?P<hour>\d{2}):(?P<minute>\d{2})(?::(?P<second>\d{2}))?$")
ROTATED_LOG_RE = re.compile(r"^sc-(\d{4}-\d{2}-\d{2})\.log$")


@dataclass
class ParsedEntry:
    """Parsed log entry with metadata retained for filtering/output."""

    entry: dict
    raw_line: str
    timestamp: datetime
    source: Path


def default_log_path() -> Path:
    """Return the default current log file path."""
    return Path.home() / ".sc" / "logs" / "sc.log"


def parse_level(value: str | None) -> str | None:
    """Normalize a level string to uppercase."""
    if value is None:
        return None
    normalized = value.strip().upper()
    if normalized not in LEVEL_ORDER:
        raise ValueError(f"invalid level {value!r}; expected debug|info|warn|error")
    return normalized


def parse_log_timestamp(value: str) -> datetime:
    """Parse an RFC3339/RFC3339Nano timestamp into an aware datetime."""
    if value.endswith("Z"):
        value = value[:-1] + "+00:00"

    match = re.match(r"^(.*T\d{2}:\d{2}:\d{2})(\.\d+)?([+-]\d{2}:\d{2})$", value)
    if match:
        prefix, fraction, offset = match.groups()
        if fraction:
            digits = fraction[1:]
            fraction = "." + digits[:6].ljust(6, "0")
        else:
            fraction = ""
        value = prefix + fraction + offset

    return datetime.fromisoformat(value)


def parse_time_spec(spec: str | None, now: datetime | None = None) -> datetime | None:
    """Parse relative, local-clock, or absolute ISO8601 time specifications."""
    if spec is None:
        return None

    current = now or datetime.now().astimezone()

    relative = RELATIVE_TIME_RE.match(spec)
    if relative:
        amount = int(relative.group("amount"))
        unit = relative.group("unit")
        if unit == "m":
            return current - timedelta(minutes=amount)
        if unit == "h":
            return current - timedelta(hours=amount)
        return current - timedelta(days=amount)

    local = LOCAL_TIME_RE.match(spec)
    if local:
        parsed_local = current.replace(
            hour=int(local.group("hour")),
            minute=int(local.group("minute")),
            second=int(local.group("second") or 0),
            microsecond=0,
        )
        if parsed_local > current:
            parsed_local -= timedelta(days=1)
        return parsed_local

    try:
        parsed = parse_log_timestamp(spec)
    except ValueError as exc:
        raise ValueError(
            f"invalid time spec {spec!r}; expected 5m, 14:30, or ISO8601"
        ) from exc

    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=current.tzinfo)
    return parsed


def resolve_log_paths(
    log_path: Path,
    include_rotated: bool,
    *,
    since: datetime | None = None,
) -> list[Path]:
    """Return current log plus optional rotated logs from the same directory."""
    paths = [log_path]
    if not include_rotated:
        return paths

    earliest_date: date | None = None
    if since is not None:
        earliest_date = since.date()

    for candidate in sorted(log_path.parent.glob("sc-*.log")):
        match = ROTATED_LOG_RE.match(candidate.name)
        if not match:
            continue

        if earliest_date is not None:
            rotated_date = datetime.strptime(match.group(1), "%Y-%m-%d").date()
            if rotated_date < earliest_date:
                continue

        paths.append(candidate)
    return paths


def parse_json_line(line: str, source: Path) -> ParsedEntry | None:
    """Parse one JSON log line, skipping malformed entries."""
    try:
        entry = json.loads(line)
    except json.JSONDecodeError:
        return None

    timestamp_raw = entry.get("time")
    if not isinstance(timestamp_raw, str):
        return None

    try:
        timestamp = parse_log_timestamp(timestamp_raw)
    except ValueError:
        return None

    return ParsedEntry(entry=entry, raw_line=line, timestamp=timestamp, source=source)


def iter_log_entries(paths: Iterable[Path]) -> Iterable[ParsedEntry]:
    """Yield parsed log entries from the given files."""
    for path in paths:
        if not path.exists():
            continue
        with path.open("r", encoding="utf-8", errors="replace") as handle:
            for raw in handle:
                line = raw.rstrip("\n")
                parsed = parse_json_line(line, path)
                if parsed is not None:
                    yield parsed


def entry_matches(
    parsed: ParsedEntry,
    *,
    min_level: str | None = None,
    component: str | None = None,
    operation: str | None = None,
    regex: re.Pattern[str] | None = None,
    since: datetime | None = None,
    until: datetime | None = None,
) -> bool:
    """Return True when a parsed entry satisfies all active filters."""
    entry = parsed.entry

    level = str(entry.get("level", "")).upper()
    if min_level is not None:
        if level not in LEVEL_ORDER or LEVEL_ORDER[level] < LEVEL_ORDER[min_level]:
            return False

    if component is not None and entry.get("component") != component:
        return False

    if operation is not None and entry.get("operation") != operation:
        return False

    if regex is not None and regex.search(parsed.raw_line) is None:
        return False

    if since is not None and parsed.timestamp < since:
        return False

    if until is not None and parsed.timestamp > until:
        return False

    return True


def compile_regex(pattern: str | None) -> re.Pattern[str] | None:
    """Compile a regex if one was supplied."""
    if pattern is None:
        return None
    try:
        return re.compile(pattern)
    except re.error as exc:
        raise ValueError(f"invalid regex {pattern!r}: {exc}") from exc


def summarize_levels(entries: Iterable[ParsedEntry]) -> dict[str, int]:
    """Count entries by level in a stable key order."""
    counts = {level: 0 for level in ("DEBUG", "INFO", "WARN", "ERROR")}
    for parsed in entries:
        level = str(parsed.entry.get("level", "")).upper()
        if level in counts:
            counts[level] += 1
    return counts


def json_dumps(value: object) -> str:
    """Serialize a JSON value with deterministic formatting."""
    return json.dumps(value, sort_keys=True)


def local_now() -> datetime:
    """Return the current time as an aware local datetime."""
    return datetime.now().astimezone()
