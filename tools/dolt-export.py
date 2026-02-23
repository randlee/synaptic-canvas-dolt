#!/usr/bin/env python3
"""
Synaptic Canvas — Dolt Export Tool
====================================

Exports packages from the local Dolt database into the marketplace
directory structure (manifest.yaml, plugin.json, content files).

Usage:
    # Export all packages to a directory
    python3 tools/dolt-export.py --output /tmp/exported-packages

    # Export specific package(s)
    python3 tools/dolt-export.py --output /tmp/exported --packages sc-manage sc-delay-tasks

    # Dry-run (show what would be written)
    python3 tools/dolt-export.py --dry-run

    # Verify round-trip against source
    python3 tools/dolt-export.py --output /tmp/exported --verify-against /path/to/packages/

Requires:
    - Dolt CLI on PATH (uses `dolt sql -q -r json`)
    - CWD must be inside a Dolt database directory (or pass --doltdb)
"""

import argparse
import hashlib
import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any, Optional


# ---------------------------------------------------------------------------
# Dolt query helpers
# ---------------------------------------------------------------------------

def dolt_query(sql: str, doltdb: Path) -> list[dict]:
    """Execute SQL and return rows as list of dicts."""
    result = subprocess.run(
        ["dolt", "sql", "-q", sql, "-r", "json"],
        capture_output=True, text=True, cwd=str(doltdb),
    )
    if result.returncode != 0:
        print(f"✗ SQL error: {result.stderr}", file=sys.stderr)
        return []
    try:
        data = json.loads(result.stdout)
        # dolt sql -r json returns {"rows": [...]}
        return data.get("rows", [])
    except json.JSONDecodeError:
        return []


# ---------------------------------------------------------------------------
# YAML rendering (no PyYAML dependency for export)
# ---------------------------------------------------------------------------

def render_yaml_value(val: Any, indent: int = 0) -> str:
    """Render a value as YAML."""
    prefix = "  " * indent
    if val is None:
        return "null"
    if isinstance(val, bool):
        return "true" if val else "false"
    if isinstance(val, (int, float)):
        return str(val)
    if isinstance(val, str):
        # Use block scalar for multi-line, quoted for special chars
        if "\n" in val:
            lines = val.rstrip("\n").split("\n")
            return ">\n" + "\n".join(f"{prefix}  {line}" for line in lines)
        if any(c in val for c in ":{}[]#&*!|>'\"%@`"):
            return f"'{val}'"
        return val
    if isinstance(val, list):
        if not val:
            return "[]"
        lines = []
        for item in val:
            lines.append(f"\n{prefix}- {render_yaml_value(item, indent + 1)}")
        return "".join(lines)
    if isinstance(val, dict):
        if not val:
            return "{}"
        lines = []
        for k, v in val.items():
            rendered = render_yaml_value(v, indent + 1)
            if isinstance(v, (dict, list)) and v:
                lines.append(f"\n{prefix}{k}:{rendered}")
            else:
                lines.append(f"\n{prefix}{k}: {rendered}")
        return "".join(lines)
    return str(val)


def build_manifest_yaml(pkg: dict, files: list[dict], deps: list[dict]) -> str:
    """Reconstruct manifest.yaml from database rows."""
    lines = []

    # Core fields
    lines.append(f"name: {pkg['name']}")
    lines.append(f"version: {pkg['version']}")
    desc = pkg.get("description", "")
    if desc:
        lines.append(f"description: >")
        # Wrap at ~78 chars
        words = desc.split()
        current = "  "
        for w in words:
            if len(current) + len(w) + 1 > 78:
                lines.append(current)
                current = "  " + w
            else:
                current += (" " if len(current) > 2 else "") + w
        if current.strip():
            lines.append(current)
    lines.append(f"author: {pkg.get('author', '')}")
    lines.append(f"license: {pkg.get('license', 'MIT')}")

    # Tags
    tags = pkg.get("tags", "")
    if tags:
        tag_list = [t.strip() for t in tags.split(",") if t.strip()]
        lines.append(f"tags: [{', '.join(tag_list)}]")

    lines.append("")

    # Artifacts grouped by file_type
    type_order = ["commands", "skills", "agents", "scripts"]
    # Map DB file_type to manifest section name (pluralized)
    type_map = {"command": "commands", "skill": "skills", "agent": "agents",
                "script": "scripts", "hook": "scripts"}

    grouped: dict[str, list[str]] = {}
    for f in files:
        ft = f.get("file_type", "")
        section = type_map.get(ft)
        if section:
            grouped.setdefault(section, []).append(f["dest_path"])

    if grouped:
        lines.append("# Files to install (relative to package root)")
        lines.append("artifacts:")
        for section in type_order:
            if section in grouped:
                lines.append(f"  {section}:")
                for path in sorted(grouped[section]):
                    lines.append(f"    - {path}")

    lines.append("")

    # Variables
    variables = pkg.get("variables")
    if variables:
        if isinstance(variables, str):
            try:
                variables = json.loads(variables)
            except json.JSONDecodeError:
                variables = None
    if variables:
        lines.append("# Token substitution (Tier 1 package)")
        lines.append("variables:")
        for var_name, var_def in variables.items():
            lines.append(f"  {var_name}:")
            if isinstance(var_def, dict):
                for k, v in var_def.items():
                    lines.append(f"    {k}: {v}")
            else:
                lines.append(f"    value: {var_def}")

    # Install scope
    install_scope = pkg.get("install_scope", "any")
    if install_scope and install_scope != "any":
        lines.append("")
        lines.append("# Installation policy/metadata")
        lines.append("install:")
        lines.append(f"  scope: {install_scope}")

    # Options
    options = pkg.get("options")
    if options:
        if isinstance(options, str):
            try:
                options = json.loads(options)
            except json.JSONDecodeError:
                options = None
    if options:
        lines.append("")
        lines.append("# Install-time options")
        lines.append("options:")
        for opt_name, opt_def in options.items():
            lines.append(f"  {opt_name}:")
            if isinstance(opt_def, dict):
                for k, v in opt_def.items():
                    val = "true" if v is True else "false" if v is False else str(v)
                    lines.append(f"    {k}: {val}")
            else:
                lines.append(f"    value: {opt_def}")

    # Requirements
    if deps:
        lines.append("")
        lines.append("# Runtime requirements")
        lines.append("requires:")
        for d in deps:
            spec = d.get("dep_spec", "")
            name = d.get("dep_name", "")
            if spec:
                lines.append(f"  - {name} {spec}")
            else:
                lines.append(f"  - {name}")

    return "\n".join(lines) + "\n"


def build_plugin_json(pkg: dict, files: list[dict]) -> str:
    """Reconstruct plugin.json from database rows."""
    tags = pkg.get("tags", "")
    keywords = [t.strip() for t in tags.split(",") if t.strip()] if tags else []

    # Author as object (Claude Code format)
    author = pkg.get("author", "")
    author_obj = {"name": author} if author else {"name": "synaptic-canvas"}

    plugin = {
        "name": pkg["name"],
        "description": pkg.get("description", ""),
        "version": pkg["version"],
        "author": author_obj,
        "license": pkg.get("license", "MIT"),
        "keywords": keywords,
    }

    # Group file references by type
    commands = []
    agents = []
    skills = []

    for f in files:
        ft = f.get("file_type", "")
        dp = f.get("dest_path", "")
        if ft == "command":
            commands.append(f"./{dp}")
        elif ft == "agent":
            agents.append(f"./{dp}")
        elif ft == "skill":
            skills.append(f"./{dp}")

    if commands:
        plugin["commands"] = sorted(commands)
    if agents:
        plugin["agents"] = sorted(agents)
    if skills:
        plugin["skills"] = sorted(skills)

    return json.dumps(plugin, indent=2, ensure_ascii=False) + "\n"


# ---------------------------------------------------------------------------
# Export logic
# ---------------------------------------------------------------------------

def export_package(pkg_id: str, output_dir: Path, doltdb: Path,
                   dry_run: bool = False) -> dict:
    """Export a single package to the output directory. Returns stats."""
    # Query package
    pkgs = dolt_query(
        f"SELECT * FROM packages WHERE id = '{pkg_id}';", doltdb
    )
    if not pkgs:
        print(f"✗ Package not found: {pkg_id}", file=sys.stderr)
        return {"error": f"not found: {pkg_id}"}

    pkg = pkgs[0]

    # Query files
    files = dolt_query(
        f"SELECT * FROM package_files WHERE package_id = '{pkg_id}' "
        f"ORDER BY dest_path;", doltdb
    )

    # Query deps
    deps = dolt_query(
        f"SELECT * FROM package_deps WHERE package_id = '{pkg_id}' "
        f"ORDER BY dep_name;", doltdb
    )

    pkg_dir = output_dir / pkg_id
    stats = {"id": pkg_id, "files_written": 0, "sha_ok": 0, "sha_fail": 0}

    if dry_run:
        print(f"\n  Package: {pkg_id} v{pkg['version']}")
        print(f"  Would write {len(files)} content files + manifest.yaml + plugin.json")
        return stats

    # Create directory
    pkg_dir.mkdir(parents=True, exist_ok=True)

    # Write manifest.yaml (reconstructed)
    manifest_content = build_manifest_yaml(pkg, files, deps)
    (pkg_dir / "manifest.yaml").write_text(manifest_content, encoding="utf-8")
    stats["files_written"] += 1

    # Write content files
    has_plugin_json = False
    for f in files:
        dest_path = f["dest_path"]
        content = f.get("content", "")
        expected_sha = f.get("sha256", "")

        file_path = pkg_dir / dest_path
        file_path.parent.mkdir(parents=True, exist_ok=True)
        file_path.write_text(content, encoding="utf-8")
        stats["files_written"] += 1

        # Verify SHA-256
        actual_sha = hashlib.sha256(content.encode("utf-8")).hexdigest()
        if expected_sha and actual_sha == expected_sha:
            stats["sha_ok"] += 1
        elif expected_sha:
            stats["sha_fail"] += 1
            print(f"  ⚠ SHA mismatch: {dest_path}", file=sys.stderr)

        if dest_path == ".claude-plugin/plugin.json":
            has_plugin_json = True

    # If no plugin.json was stored as a file, reconstruct it
    if not has_plugin_json:
        plugin_content = build_plugin_json(pkg, files)
        plugin_dir = pkg_dir / ".claude-plugin"
        plugin_dir.mkdir(parents=True, exist_ok=True)
        (plugin_dir / "plugin.json").write_text(plugin_content, encoding="utf-8")
        stats["files_written"] += 1

    return stats


# ---------------------------------------------------------------------------
# Verification
# ---------------------------------------------------------------------------

def verify_against_source(exported_dir: Path, source_dir: Path, pkg_id: str) -> dict:
    """Compare exported files against the source marketplace directory."""
    export_pkg = exported_dir / pkg_id
    source_pkg = source_dir / pkg_id

    if not source_pkg.exists():
        return {"status": "source_missing", "pkg": pkg_id}

    results = {"matched": 0, "differed": 0, "export_only": 0, "source_only": 0, "details": []}

    # Compare exported files against source
    for root, _, filenames in os.walk(export_pkg):
        for fname in filenames:
            export_file = Path(root) / fname
            rel = export_file.relative_to(export_pkg)
            source_file = source_pkg / rel

            if source_file.exists():
                try:
                    e_content = export_file.read_text(encoding="utf-8")
                    s_content = source_file.read_text(encoding="utf-8")
                    if e_content == s_content:
                        results["matched"] += 1
                    else:
                        results["differed"] += 1
                        results["details"].append(f"DIFF {rel}")
                except UnicodeDecodeError:
                    results["details"].append(f"BINARY {rel}")
            else:
                results["export_only"] += 1
                results["details"].append(f"EXPORT_ONLY {rel}")

    return results


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Export Synaptic Canvas packages from Dolt"
    )
    parser.add_argument("--output", type=Path, required=True,
                        help="Output directory for exported packages")
    parser.add_argument("--packages", nargs="*", default=None,
                        help="Specific package IDs to export (default: all)")
    parser.add_argument("--dry-run", action="store_true",
                        help="Show what would be exported without writing")
    parser.add_argument("--doltdb", type=Path, default=None,
                        help="Path to Dolt database (default: ./doltdb)")
    parser.add_argument("--verify-against", type=Path, default=None,
                        help="Compare export against source marketplace dir")

    args = parser.parse_args()

    doltdb = args.doltdb or Path.cwd() / "doltdb"
    if not (doltdb / ".dolt").exists():
        if (Path.cwd() / ".dolt").exists():
            doltdb = Path.cwd()
        else:
            print(f"✗ No Dolt database at {doltdb}", file=sys.stderr)
            sys.exit(1)

    # Get package list
    if args.packages:
        pkg_ids = args.packages
    else:
        rows = dolt_query("SELECT id FROM packages ORDER BY id;", doltdb)
        pkg_ids = [r["id"] for r in rows]

    if not pkg_ids:
        print("No packages found in database.", file=sys.stderr)
        sys.exit(1)

    print(f"▶ Exporting {len(pkg_ids)} package(s) from Dolt...")

    if not args.dry_run:
        args.output.mkdir(parents=True, exist_ok=True)

    total_files = 0
    total_sha_ok = 0
    total_sha_fail = 0

    for pkg_id in pkg_ids:
        stats = export_package(pkg_id, args.output, doltdb, args.dry_run)
        if "error" not in stats:
            total_files += stats.get("files_written", 0)
            total_sha_ok += stats.get("sha_ok", 0)
            total_sha_fail += stats.get("sha_fail", 0)
            if not args.dry_run:
                print(f"  ✓ {pkg_id}: {stats['files_written']} files, "
                      f"{stats['sha_ok']} SHA verified")

    if not args.dry_run:
        print(f"\n✓ Export complete: {total_files} files, "
              f"{total_sha_ok} SHA verified, {total_sha_fail} SHA failures")

    # Verification
    if args.verify_against and not args.dry_run:
        print(f"\n▶ Verifying against {args.verify_against}...")
        for pkg_id in pkg_ids:
            vr = verify_against_source(args.output, args.verify_against, pkg_id)
            if vr.get("status") == "source_missing":
                print(f"  ⚠ {pkg_id}: source not found")
            else:
                m = vr["matched"]
                d = vr["differed"]
                eo = vr["export_only"]
                print(f"  {pkg_id}: {m} matched, {d} differed, {eo} export-only")
                for detail in vr.get("details", []):
                    print(f"    {detail}")


if __name__ == "__main__":
    main()
