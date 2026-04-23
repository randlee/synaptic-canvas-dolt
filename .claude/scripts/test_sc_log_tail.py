from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("sc_log_tail.py")


def write_jsonl(path: Path, entries: list[dict]) -> None:
    with path.open("w", encoding="utf-8") as handle:
        for entry in entries:
            handle.write(json.dumps(entry) + "\n")


class TailScriptTests(unittest.TestCase):
    def test_match_found_before_timeout(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            log_path.write_text("", encoding="utf-8")

            def append_entry() -> None:
                time.sleep(0.2)
                with log_path.open("a", encoding="utf-8") as handle:
                    handle.write(
                        json.dumps(
                            {
                                "time": "2026-04-07T14:23:46.001Z",
                                "level": "ERROR",
                                "msg": "connection refused",
                                "component": "dolt",
                                "operation": "get",
                            }
                        )
                        + "\n"
                    )

            writer = threading.Thread(target=append_entry)
            writer.start()
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--level", "warn", "--timeout", "2"],
                capture_output=True,
                text=True,
                check=False,
            )
            writer.join()

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout.strip())
            self.assertEqual(payload["level"], "ERROR")
            self.assertIn("_offset", payload)
            self.assertGreater(payload["_offset"], 0)

    def test_timeout_returns_exit_code_one(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            log_path.write_text("", encoding="utf-8")
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--timeout", "0.2"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout.strip(), "")

    def test_level_filter_and_max_matches(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            write_jsonl(
                log_path,
                [
                    {"time": "2026-04-07T14:23:45Z", "level": "INFO", "msg": "info"},
                    {"time": "2026-04-07T14:23:46Z", "level": "WARN", "msg": "warn"},
                    {"time": "2026-04-07T14:23:47Z", "level": "ERROR", "msg": "error"},
                ],
            )

            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "--log",
                    str(log_path),
                    "--since-offset",
                    "0",
                    "--level",
                    "warn",
                    "--max-matches",
                    "2",
                    "--timeout",
                    "0.2",
                ],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            lines = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
            self.assertEqual([entry["level"] for entry in lines], ["WARN", "ERROR"])

    def test_combined_filters_and_since_offset(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "sc.log"
            first = json.dumps(
                {
                    "time": "2026-04-07T14:23:45Z",
                    "level": "WARN",
                    "msg": "ignore me",
                    "component": "cli",
                    "operation": "init",
                }
            ) + "\n"
            second = json.dumps(
                {
                    "time": "2026-04-07T14:23:46Z",
                    "level": "ERROR",
                    "msg": "wanted",
                    "component": "dolt",
                    "operation": "install",
                }
            ) + "\n"
            log_path.write_text(first + second, encoding="utf-8")
            offset = len(first.encode("utf-8"))

            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "--log",
                    str(log_path),
                    "--since-offset",
                    str(offset),
                    "--component",
                    "dolt",
                    "--operation",
                    "install",
                    "--regex",
                    "wanted",
                    "--timeout",
                    "0.2",
                ],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout.strip())
            self.assertEqual(payload["component"], "dolt")
            self.assertEqual(payload["operation"], "install")

    def test_missing_log_file_returns_exit_code_two(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            log_path = Path(tmpdir) / "missing.log"
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--log", str(log_path), "--timeout", "0.1"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual(result.returncode, 2)


if __name__ == "__main__":
    unittest.main()
