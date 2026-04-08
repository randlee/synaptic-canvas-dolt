from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta
from pathlib import Path


SCRIPT = Path(__file__).with_name("sc_log_query.py")


def write_jsonl(path: Path, entries: list[dict]) -> None:
    with path.open("w", encoding="utf-8") as handle:
        for entry in entries:
            handle.write(json.dumps(entry) + "\n")


class QueryScriptTests(unittest.TestCase):
    def test_since_relative_time_filters_recent_entries(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            now = datetime.now().astimezone()
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(
                log_path,
                [
                    {"time": (now - timedelta(minutes=10)).isoformat(), "level": "WARN", "msg": "old"},
                    {"time": (now - timedelta(minutes=2)).isoformat(), "level": "ERROR", "msg": "recent"},
                ],
            )

            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--since", "5m", "--json"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            lines = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
            self.assertEqual(len(lines), 1)
            self.assertEqual(lines[0]["msg"], "recent")

    def test_since_today_clock_time_filters_entries(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            now = datetime.now().astimezone()
            log_path = Path(tmpdir) / "sc.log"
            today = now.replace(hour=0, minute=0, second=0, microsecond=0)
            write_jsonl(
                log_path,
                [
                    {"time": today.replace(hour=14, minute=29).isoformat(), "level": "WARN", "msg": "before"},
                    {"time": today.replace(hour=14, minute=31).isoformat(), "level": "WARN", "msg": "after"},
                ],
            )

            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--since", "14:30", "--json"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            lines = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
            self.assertEqual([line["msg"] for line in lines], ["after"])

    def test_iso8601_and_until_filters(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(
                log_path,
                [
                    {"time": "2026-04-07T14:23:45Z", "level": "WARN", "msg": "before"},
                    {"time": "2026-04-07T14:23:46Z", "level": "ERROR", "msg": "inside"},
                    {"time": "2026-04-07T14:23:47Z", "level": "ERROR", "msg": "after"},
                ],
            )

            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "--log",
                    str(log_path),
                    "--since",
                    "2026-04-07T14:23:45.500Z",
                    "--until",
                    "2026-04-07T14:23:46.500Z",
                    "--json",
                ],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            lines = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
            self.assertEqual([line["msg"] for line in lines], ["inside"])

    def test_include_rotated_picks_up_dated_logs(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp = Path(tmpdir)
            log_path = tmp / "sc.log"
            write_jsonl(log_path, [])
            write_jsonl(
                tmp / "sc-2026-04-06.log",
                [{"time": "2026-04-06T14:23:46Z", "level": "ERROR", "msg": "rotated"}],
            )

            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "--log",
                    str(log_path),
                    "--since",
                    "2026-04-06T00:00:00Z",
                    "--include-rotated",
                    "--json",
                ],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            lines = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
            self.assertEqual(lines[0]["msg"], "rotated")

    def test_summary_outputs_counts_only(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(
                log_path,
                [
                    {"time": "2026-04-07T14:23:45Z", "level": "INFO", "msg": "i"},
                    {"time": "2026-04-07T14:23:46Z", "level": "WARN", "msg": "w"},
                    {"time": "2026-04-07T14:23:47Z", "level": "ERROR", "msg": "e"},
                ],
            )

            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--summary", "--json"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout.strip())
            self.assertEqual(payload["INFO"], 1)
            self.assertEqual(payload["WARN"], 1)
            self.assertEqual(payload["ERROR"], 1)

    def test_no_results_returns_exit_code_one(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(log_path, [{"time": "2026-04-07T14:23:45Z", "level": "INFO", "msg": "i"}])
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--level", "error"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 1)

    def test_missing_log_file_returns_exit_code_two(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "missing.log"
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path)],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 2)


if __name__ == "__main__":
    unittest.main()
