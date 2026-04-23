from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("sc_log_correlate.py")


def write_jsonl(path: Path, entries: list[dict]) -> None:
    with path.open("w", encoding="utf-8") as handle:
        for entry in entries:
            handle.write(json.dumps(entry) + "\n")


class CorrelateScriptTests(unittest.TestCase):
    def test_single_run_groups_consecutive_entries(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(
                log_path,
                [
                    {"time": "2026-04-07T14:23:45Z", "level": "INFO", "operation": "install", "msg": "start"},
                    {"time": "2026-04-07T14:23:48Z", "level": "WARN", "operation": "install", "msg": "warn"},
                ],
            )
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--operation", "install", "--json"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout.strip())
            self.assertEqual(len(payload), 1)
            self.assertEqual(payload[0]["outcome"], "warn")

    def test_gap_larger_than_window_starts_new_run(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(
                log_path,
                [
                    {"time": "2026-04-07T14:23:45Z", "level": "INFO", "operation": "install", "msg": "one"},
                    {"time": "2026-04-07T14:23:46Z", "level": "INFO", "operation": "install", "msg": "two"},
                    {"time": "2026-04-07T14:23:55Z", "level": "INFO", "operation": "install", "msg": "three"},
                ],
            )
            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "--log",
                    str(log_path),
                    "--operation",
                    "install",
                    "--window",
                    "5",
                    "--json",
                ],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout.strip())
            self.assertEqual(len(payload), 2)

    def test_error_beats_warn_and_info(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(
                log_path,
                [
                    {"time": "2026-04-07T14:23:45Z", "level": "INFO", "operation": "install", "msg": "one"},
                    {"time": "2026-04-07T14:23:46Z", "level": "WARN", "operation": "install", "msg": "two"},
                    {"time": "2026-04-07T14:23:47Z", "level": "ERROR", "operation": "install", "msg": "three"},
                ],
            )
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--operation", "install", "--json"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout.strip())
            self.assertEqual(payload[0]["outcome"], "error")

    def test_existing_timestamp_like_field_is_preserved(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(
                log_path,
                [
                    {
                        "time": "2026-04-07T14:23:45Z",
                        "level": "INFO",
                        "operation": "install",
                        "msg": "one",
                        "_timestamp": "keep-me",
                    }
                ],
            )
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--operation", "install", "--json"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout.strip())
            self.assertEqual(payload[0]["entries"][0]["_timestamp"], "keep-me")

    def test_no_matches_returns_exit_code_one(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(log_path, [{"time": "2026-04-07T14:23:45Z", "level": "INFO", "operation": "get"}])
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--operation", "install"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 1)

    def test_missing_log_file_returns_exit_code_two(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "missing.log"
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--operation", "install"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 2)


if __name__ == "__main__":
    unittest.main()
