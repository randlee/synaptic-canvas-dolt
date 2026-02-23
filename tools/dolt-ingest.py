#!/usr/bin/env python3
"""
Synaptic Canvas — Dolt Ingestion Tool
======================================

Reads a package directory from the marketplace repo and inserts it into
the local Dolt database.

Usage:
    # Ingest a single package
    python3 tools/dolt-ingest.py /path/to/packages/sc-manage

    # Ingest multiple packages
    python3 tools/dolt-ingest.py /path/to/packages/sc-manage /path/to/packages/sc-delay-tasks

    # Dry-run (show SQL without executing)
    python3 tools/dolt-ingest.py --dry-run /path/to/packages/sc-manage

    # List what would be ingested (summary only)
    python3 tools/dolt-ingest.py --list /path/to/packages/sc-manage

Requires:
    - Dolt CLI on PATH (uses `dolt sql -q`)
    - CWD must be inside a Dolt database directory (or pass --doltdb)
    - PyYAML (pip3 install pyyaml) — falls back to basic parser
"""

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any, Optional


# ---------------------------------------------------------------------------
# YAML parsing (with PyYAML fallback)
# ---------------------------------------------------------------------------

def load_yaml(path: Path) -> dict:
    """Load YAML file, preferring PyYAML but falling back to basic parser."""
    text = path.read_text(encoding="utf-8")
    try:
        import yaml
        return yaml.safe_load(text) or {}
    except ImportError:
        return _basic_yaml_parse(text)


def _basic_yaml_parse(text: str) -> dict:
    """Minimal YAML parser for flat manifest files."""
    result: dict[str, Any] = {}
    current_key = None
    current_list: list[str] | None = None

    for line in text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        # List item under a key
        if stripped.startswith("- ") and current_key:
            if current_list is None:
                current_list = []
                result[current_key] = current_list
            current_list.append(stripped[2:].strip())
            continue

        # Key: value
        match = re.match(r"^(\w[\w.-]*):\s*(.*)", stripped)
        if match:
            current_key = match.group(1)
            val = match.group(2).strip()
            current_list = None
            if val:
                # Remove quotes
                if (val.startswith('"') and val.endswith('"')) or \
                   (val.startswith("'") and val.endswith("'")):
                    val = val[1:-1]
                # Handle > (block scalar indicator — just take next non-empty lines)
                if val == ">":
                    result[current_key] = ""
                    continue
                result[current_key] = val
            else:
                result[current_key] = None
            continue

        # Continuation of block scalar
        if current_key and result.get(current_key) is not None and isinstance(result[current_key], str):
            if result[current_key] == "":
                result[current_key] = stripped
            else:
                result[current_key] += " " + stripped

    return result


# ---------------------------------------------------------------------------
# Frontmatter extraction
# ---------------------------------------------------------------------------

FRONTMATTER_RE = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.DOTALL)


def extract_frontmatter(content: str) -> Optional[dict]:
    """Extract YAML frontmatter from markdown content."""
    m = FRONTMATTER_RE.match(content)
    if not m:
        return None
    try:
        import yaml
        return yaml.safe_load(m.group(1)) or {}
    except ImportError:
        return _basic_yaml_parse(m.group(1))
    except Exception:
        return None


# ---------------------------------------------------------------------------
# SQL escaping
# ---------------------------------------------------------------------------

def sql_str(val: Optional[str]) -> str:
    """Escape a string for SQL (single-quote escaping)."""
    if val is None:
        return "NULL"
    escaped = val.replace("\\", "\\\\").replace("'", "\\'")
    return f"'{escaped}'"


def sql_json(val: Any) -> str:
    """Encode a value as a JSON SQL literal."""
    if val is None:
        return "NULL"
    return sql_str(json.dumps(val, ensure_ascii=False))


def sql_bool(val: bool) -> str:
    return "TRUE" if val else "FALSE"


# ---------------------------------------------------------------------------
# File classification
# ---------------------------------------------------------------------------

def classify_file(dest_path: str) -> tuple[str, str]:
    """Return (file_type, content_type) for a given dest_path."""
    p = dest_path.lower()

    # file_type from directory prefix
    if p.startswith("agents/"):
        file_type = "agent"
    elif p.startswith("commands/"):
        file_type = "command"
    elif p.startswith("skills/"):
        file_type = "skill"
    elif p.startswith("scripts/"):
        file_type = "script"
    elif p.startswith("hooks/"):
        file_type = "hook"
    elif p == ".claude-plugin/plugin.json" or p == "plugin.json":
        file_type = "config"
    else:
        file_type = "config"

    # content_type from extension
    if p.endswith(".md"):
        content_type = "markdown"
    elif p.endswith(".py"):
        content_type = "python"
    elif p.endswith(".json"):
        content_type = "json"
    elif p.endswith(".yaml") or p.endswith(".yml"):
        content_type = "yaml"
    elif p.endswith(".sh") or p.endswith(".bash"):
        content_type = "shell"
    else:
        content_type = "text"

    return file_type, content_type


# ---------------------------------------------------------------------------
# Package scanning
# ---------------------------------------------------------------------------

# Files/dirs to skip during ingestion
SKIP_NAMES = {
    "__pycache__", ".git", "node_modules", "tests", "test",
    ".sc-prefix-verified", "README.md", "CHANGELOG.md",
    "LICENSE", "TROUBLESHOOTING.md", "USE-CASES.md", "DESIGN.md",
}

# We skip manifest.yaml (reconstructed from DB) and the duplicate root plugin.json
SKIP_EXACT = {"manifest.yaml"}


def scan_package(pkg_dir: Path) -> dict:
    """Scan a package directory and return structured data for ingestion."""
    manifest_path = pkg_dir / "manifest.yaml"
    if not manifest_path.exists():
        raise FileNotFoundError(f"No manifest.yaml in {pkg_dir}")

    manifest = load_yaml(manifest_path)

    # Package metadata
    pkg_id = manifest.get("name", pkg_dir.name)
    pkg = {
        "id": pkg_id,
        "name": manifest.get("name", pkg_dir.name),
        "version": manifest.get("version", "0.0.0"),
        "description": manifest.get("description", ""),
        "author": manifest.get("author", ""),
        "license": manifest.get("license", "MIT"),
        "tags": "",
        "install_scope": "any",
        "variables": None,
        "options": None,
    }

    # Tags — could be list or comma-separated string
    tags = manifest.get("tags", [])
    if isinstance(tags, list):
        pkg["tags"] = ",".join(str(t) for t in tags)
    elif isinstance(tags, str):
        pkg["tags"] = tags

    # Install scope
    install = manifest.get("install", {})
    if isinstance(install, dict) and install.get("scope"):
        pkg["install_scope"] = install["scope"]

    # Variables (Tier 1 packages)
    variables = manifest.get("variables")
    if variables:
        pkg["variables"] = variables

    # Options
    options = manifest.get("options")
    if options:
        pkg["options"] = options

    # Scan files from artifacts section
    artifacts = manifest.get("artifacts", {})
    files = []

    if isinstance(artifacts, dict):
        for artifact_type, paths in artifacts.items():
            if isinstance(paths, list):
                for rel_path in paths:
                    full_path = pkg_dir / rel_path
                    if full_path.exists():
                        files.append(_scan_file(pkg_id, rel_path, full_path))
                    else:
                        print(f"  ⚠ Missing artifact: {rel_path}", file=sys.stderr)

    # Also scan for .claude-plugin/plugin.json
    plugin_json = pkg_dir / ".claude-plugin" / "plugin.json"
    if plugin_json.exists():
        files.append(_scan_file(pkg_id, ".claude-plugin/plugin.json", plugin_json))

    # Dependencies
    deps = []
    requires = manifest.get("requires", [])
    if isinstance(requires, list):
        for req in requires:
            dep_name, dep_spec = _parse_requirement(str(req))
            deps.append({
                "package_id": pkg_id,
                "dep_type": "tool",
                "dep_name": dep_name,
                "dep_spec": dep_spec,
            })

    return {"package": pkg, "files": files, "deps": deps}


def _scan_file(pkg_id: str, rel_path: str, full_path: Path) -> dict:
    """Read a single file and extract metadata."""
    content = full_path.read_text(encoding="utf-8")
    sha256 = hashlib.sha256(content.encode("utf-8")).hexdigest()
    file_type, content_type = classify_file(rel_path)
    is_template = "{{" in content and "}}" in content

    fm = None
    fm_name = fm_desc = fm_version = fm_model = None

    if content_type == "markdown":
        fm = extract_frontmatter(content)
        if fm:
            fm_name = fm.get("name")
            fm_desc = fm.get("description")
            fm_version = fm.get("version")
            fm_model = fm.get("model")

    return {
        "package_id": pkg_id,
        "dest_path": rel_path,
        "content": content,
        "sha256": sha256,
        "file_type": file_type,
        "content_type": content_type,
        "is_template": is_template,
        "fm_name": fm_name,
        "fm_description": fm_desc,
        "fm_version": fm_version,
        "fm_model": fm_model,
        "frontmatter": fm,
    }


def _parse_requirement(req: str) -> tuple[str, str]:
    """Parse 'python3' or 'git >= 2.20' into (name, spec)."""
    match = re.match(r"^([\w.+-]+)\s*(.*)", req)
    if match:
        return match.group(1), match.group(2).strip()
    return req, ""


# ---------------------------------------------------------------------------
# SQL generation
# ---------------------------------------------------------------------------

def generate_sql(data: dict) -> list[str]:
    """Generate INSERT statements for a package."""
    stmts = []
    pkg = data["package"]

    # DELETE existing (idempotent re-ingestion)
    stmts.append(f"DELETE FROM packages WHERE id = {sql_str(pkg['id'])};")

    # INSERT package
    stmts.append(
        f"INSERT INTO packages "
        f"(id, name, version, description, agent_variant, author, license, tags, "
        f"install_scope, variables, options) VALUES ("
        f"{sql_str(pkg['id'])}, "
        f"{sql_str(pkg['name'])}, "
        f"{sql_str(pkg['version'])}, "
        f"{sql_str(pkg['description'])}, "
        f"'claude', "
        f"{sql_str(pkg['author'])}, "
        f"{sql_str(pkg['license'])}, "
        f"{sql_str(pkg['tags'])}, "
        f"{sql_str(pkg['install_scope'])}, "
        f"{sql_json(pkg['variables'])}, "
        f"{sql_json(pkg['options'])}"
        f");"
    )

    # INSERT files
    for f in data["files"]:
        stmts.append(
            f"INSERT INTO package_files "
            f"(package_id, dest_path, content, sha256, file_type, content_type, "
            f"is_template, fm_name, fm_description, fm_version, fm_model, frontmatter) VALUES ("
            f"{sql_str(f['package_id'])}, "
            f"{sql_str(f['dest_path'])}, "
            f"{sql_str(f['content'])}, "
            f"{sql_str(f['sha256'])}, "
            f"{sql_str(f['file_type'])}, "
            f"{sql_str(f['content_type'])}, "
            f"{sql_bool(f['is_template'])}, "
            f"{sql_str(f['fm_name'])}, "
            f"{sql_str(f['fm_description'])}, "
            f"{sql_str(f['fm_version'])}, "
            f"{sql_str(f['fm_model'])}, "
            f"{sql_json(f['frontmatter'])}"
            f");"
        )

    # INSERT deps
    for d in data["deps"]:
        stmts.append(
            f"INSERT INTO package_deps "
            f"(package_id, dep_type, dep_name, dep_spec) VALUES ("
            f"{sql_str(d['package_id'])}, "
            f"{sql_str(d['dep_type'])}, "
            f"{sql_str(d['dep_name'])}, "
            f"{sql_str(d['dep_spec'])}"
            f");"
        )

    return stmts


# ---------------------------------------------------------------------------
# Dolt execution
# ---------------------------------------------------------------------------

def run_dolt_sql(stmts: list[str], doltdb: Path) -> bool:
    """Execute SQL statements via dolt sql."""
    combined = "\n".join(stmts)
    result = subprocess.run(
        ["dolt", "sql"],
        input=combined,
        capture_output=True,
        text=True,
        cwd=str(doltdb),
    )
    if result.returncode != 0:
        print(f"✗ Dolt SQL error:\n{result.stderr}", file=sys.stderr)
        return False
    return True


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def print_summary(data: dict) -> None:
    """Print a summary of what would be ingested."""
    pkg = data["package"]
    print(f"\n{'='*60}")
    print(f"  Package: {pkg['id']}  v{pkg['version']}")
    print(f"  Author:  {pkg['author']}")
    print(f"  Tags:    {pkg['tags']}")
    print(f"  Scope:   {pkg['install_scope']}")
    if pkg['variables']:
        print(f"  Vars:    {list(pkg['variables'].keys())}")
    if pkg['options']:
        print(f"  Options: {list(pkg['options'].keys())}")
    print(f"  Files:   {len(data['files'])}")
    for f in data["files"]:
        tpl = " [T]" if f["is_template"] else ""
        fm = " [FM]" if f["frontmatter"] else ""
        print(f"    {f['file_type']:8s} {f['dest_path']}{tpl}{fm}")
    print(f"  Deps:    {len(data['deps'])}")
    for d in data["deps"]:
        spec = f" {d['dep_spec']}" if d["dep_spec"] else ""
        print(f"    {d['dep_type']:6s} {d['dep_name']}{spec}")
    print(f"{'='*60}")


def main():
    parser = argparse.ArgumentParser(
        description="Ingest Synaptic Canvas packages into Dolt"
    )
    parser.add_argument("packages", nargs="+", help="Package directory paths")
    parser.add_argument("--dry-run", action="store_true",
                        help="Print SQL without executing")
    parser.add_argument("--list", action="store_true", dest="list_only",
                        help="Show summary only (no SQL)")
    parser.add_argument("--doltdb", type=Path, default=None,
                        help="Path to Dolt database (default: ./doltdb)")
    parser.add_argument("--commit", action="store_true",
                        help="Auto-commit after ingestion")
    parser.add_argument("--commit-msg", type=str, default=None,
                        help="Custom commit message")

    args = parser.parse_args()

    doltdb = args.doltdb or Path.cwd() / "doltdb"
    if not (doltdb / ".dolt").exists():
        # Try CWD
        if (Path.cwd() / ".dolt").exists():
            doltdb = Path.cwd()
        else:
            print(f"✗ No Dolt database at {doltdb}", file=sys.stderr)
            sys.exit(1)

    all_sql: list[str] = []
    pkg_names: list[str] = []

    for pkg_path_str in args.packages:
        pkg_dir = Path(pkg_path_str).resolve()
        if not pkg_dir.is_dir():
            print(f"✗ Not a directory: {pkg_dir}", file=sys.stderr)
            continue

        try:
            data = scan_package(pkg_dir)
        except FileNotFoundError as e:
            print(f"✗ {e}", file=sys.stderr)
            continue

        print_summary(data)
        pkg_names.append(data["package"]["id"])

        if args.list_only:
            continue

        stmts = generate_sql(data)

        if args.dry_run:
            print("\n-- SQL for", data["package"]["id"])
            for s in stmts:
                # Truncate content values for readability
                if "INSERT INTO package_files" in s and len(s) > 500:
                    print(s[:200] + "\n  ... [content truncated] ...")
                else:
                    print(s)
            continue

        all_sql.extend(stmts)

    if args.list_only or args.dry_run or not all_sql:
        return

    # Execute all SQL
    print(f"\n▶ Ingesting {len(pkg_names)} package(s) into Dolt...")
    if run_dolt_sql(all_sql, doltdb):
        print(f"✓ Ingested: {', '.join(pkg_names)}")

        # Verify
        for name in pkg_names:
            result = subprocess.run(
                ["dolt", "sql", "-q",
                 f"SELECT COUNT(*) AS file_count FROM package_files WHERE package_id = '{name}';"],
                capture_output=True, text=True, cwd=str(doltdb),
            )
            print(f"  {name}: {result.stdout.strip()}")

        # Auto-commit
        if args.commit:
            msg = args.commit_msg or f"Ingest: {', '.join(pkg_names)}"
            subprocess.run(["dolt", "add", "."], cwd=str(doltdb))
            subprocess.run(["dolt", "commit", "-m", msg], cwd=str(doltdb))
            print(f"✓ Committed: {msg}")
    else:
        sys.exit(1)


if __name__ == "__main__":
    main()
