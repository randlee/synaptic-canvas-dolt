#!/usr/bin/env python3
"""Build a basic docs site for synaptic-canvas-dolt.

Renders docs/*.md (recursive, depth 1) into a single self-contained
site/index.html with a sidebar nav and inline content. No external
dependencies beyond the `markdown` package.

Usage:
    python3 scripts/build_site.py
Output:
    site/index.html
"""
import os
import re
import html
from pathlib import Path

import markdown

REPO_ROOT = Path(__file__).resolve().parent.parent
DOCS_DIR = REPO_ROOT / "docs"
SITE_DIR = REPO_ROOT / "site"

NAV_TITLE = "synaptic-canvas-dolt"


def slugify(name: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-")


def collect_docs() -> list[dict]:
    docs = []
    for path in sorted(DOCS_DIR.rglob("*.md")):
        rel = path.relative_to(DOCS_DIR)
        # skip phase-* deeply nested files for now? No — include all.
        title = path.stem.replace("-", " ").replace("_", " ").title()
        docs.append({
            "rel": str(rel),
            "title": title,
            "anchor": slugify(str(rel)),
            "path": path,
        })
    return docs


def render_md(path: Path) -> str:
    text = path.read_text(encoding="utf-8", errors="replace")
    return markdown.markdown(text, extensions=["fenced_code", "tables", "toc"])


def build() -> None:
    docs = collect_docs()
    SITE_DIR.mkdir(exist_ok=True)

    nav_items = []
    body_sections = []
    for d in docs:
        nav_items.append(
            f'<li><a href="#{d["anchor"]}">{html.escape(d["title"])}</a></li>'
        )
        body_sections.append(
            f'<section id="{d["anchor"]}">\n'
            f'<h1>{html.escape(d["title"])}</h1>\n'
            f'<p class="src">source: docs/{html.escape(d["rel"])}</p>\n'
            f'{render_md(d["path"])}\n'
            f'</section>\n'
        )

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{NAV_TITLE} — docs</title>
<style>
:root {{ --bg:#0d1117; --fg:#c9d1d9; --accent:#58a6ff; --border:#21262d; }}
* {{ box-sizing:border-box; }}
body {{ margin:0; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;
  background:var(--bg); color:var(--fg); line-height:1.6; }}
.layout {{ display:flex; min-height:100vh; }}
nav {{ width:260px; flex-shrink:0; border-right:1px solid var(--border);
  padding:1rem; position:sticky; top:0; height:100vh; overflow-y:auto; background:#161b22; }}
nav h2 {{ font-size:1rem; color:var(--accent); margin:0 0 .5rem; }}
nav ul {{ list-style:none; padding:0; margin:0; }}
nav li {{ margin:2px 0; }}
nav a {{ color:var(--fg); text-decoration:none; font-size:.9rem; display:block; padding:3px 6px; border-radius:4px; }}
nav a:hover {{ background:#21262d; color:var(--accent); }}
main {{ flex:1; padding:2rem; max-width:900px; }}
section {{ margin-bottom:3rem; padding-bottom:2rem; border-bottom:1px solid var(--border); }}
section h1 {{ color:var(--accent); border-bottom:1px solid var(--border); padding-bottom:.4rem; }}
.src {{ color:#8b949e; font-size:.8rem; }}
pre {{ background:#161b22; padding:1rem; border-radius:6px; overflow-x:auto; border:1px solid var(--border); }}
code {{ font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; font-size:.85em; }}
p code, li code {{ background:#161b22; padding:1px 5px; border-radius:3px; }}
a {{ color:var(--accent); }}
table {{ border-collapse:collapse; width:100%; }}
th,td {{ border:1px solid var(--border); padding:6px 10px; text-align:left; }}
th {{ background:#161b22; }}
img {{ max-width:100%; }}
@media (max-width:768px) {{
  .layout {{ flex-direction:column; }}
  nav {{ width:100%; height:auto; position:static; border-right:none; border-bottom:1px solid var(--border); }}
}}
</style>
</head>
<body>
<div class="layout">
  <nav>
    <h2>{NAV_TITLE}</h2>
    <ul>
{chr(10).join(nav_items)}
    </ul>
  </nav>
  <main>
{chr(10).join(body_sections)}
  </main>
</div>
</body>
</html>
"""
    out = SITE_DIR / "index.html"
    out.write_text(page, encoding="utf-8")
    print(f"Wrote {out} ({len(docs)} docs rendered)")


if __name__ == "__main__":
    build()
