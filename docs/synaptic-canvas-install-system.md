# Synaptic Canvas — Install System

## Overview

The install system is the single point where Synaptic Canvas interacts with the user about requirements, configuration, and approval. It replaces per-invocation dependency checking (which wastes context) with a **one-time verification and configuration step** whose results are recorded in the lockfile and never rechecked until upgrade.

The install process handles three distinct concerns:
1. **Repo detection** — what kind of project is this?
2. **User configuration** — what does the user need from this skill?
3. **Dependency resolution** — what needs to be installed on this machine?

All three happen once, at install time, with user approval.

---

## Install Flow

```
synaptic install
    │
    ├─ 1. Repo Detection (automatic)
    │     Scan repo for language, framework, structure
    │     Classify repo type → suggest default skill set
    │
    ├─ 2. User Questionnaire (interactive)
    │     Ask project-specific questions
    │     Capture preferences for template rendering
    │
    ├─ 3. Skill Selection (interactive)
    │     Present recommended + available skills
    │     User selects what to install
    │
    ├─ 4. Dependency Resolution (automatic)
    │     Walk dependency graph for all selected skills
    │     Check tool versions, CLI availability, agent profile
    │
    ├─ 5. Template Rendering (automatic)
    │     Fill Jinja2 templates with repo context + user answers
    │     Show rendered output for verification
    │
    ├─ 6. Approval (interactive)
    │     Present complete install plan:
    │       - Skills to install (with variants)
    │       - Files to materialize (with destinations)
    │       - External tools/CLIs to install
    │       - Hooks to register
    │       - Rendered prompt previews
    │     [Proceed? y/N]
    │
    ├─ 7. Execute (automatic)
    │     Install external dependencies
    │     Materialize files from Dolt
    │     Render templates to final form
    │     Register hooks
    │     Generate env.toml
    │     Write lockfile
    │
    └─ 8. Verify (automatic)
          Checksum all materialized files
          Confirm hook registration
          Report success
```

---

## Phase 1: Repo Detection

The installer scans the repository to build a **repo profile** that drives skill recommendations and template defaults.

### Detection Signals

| Signal | Method | Output |
|--------|--------|--------|
| Languages | File extensions, shebang lines | `["python", "typescript"]` |
| Package managers | `package.json`, `Cargo.toml`, `go.mod`, `pyproject.toml` | `["npm", "cargo"]` |
| Frameworks | Import patterns, config files | `["react", "fastapi"]` |
| Test frameworks | `jest.config.*`, `pytest.ini`, `*_test.go` | `["jest", "pytest"]` |
| CI system | `.github/workflows/`, `.gitlab-ci.yml` | `"github-actions"` |
| Monorepo | `workspaces`, `lerna.json`, multiple `go.mod` | `true/false` |
| Git conventions | Recent commit messages pattern | `"conventional"` / `"freeform"` |
| Existing Claude config | `.claude/`, `CLAUDE.md` | Parsed for context |

### Repo Profile Output

```toml
# Generated at install time, used for template rendering and skill selection
[repo]
name = "my-api"
root = "/Users/rand/projects/my-api"
languages = ["python", "typescript"]
primary_language = "python"
package_managers = ["pip", "npm"]
frameworks = ["fastapi", "react"]
test_frameworks = ["pytest", "jest"]
ci_system = "github-actions"
monorepo = false
git_conventions = "conventional"

[detected]
python_version = "3.11.4"
node_version = "20.11.0"
has_claude_md = true
has_claude_settings = false
```

The profile is presented to the user for confirmation before proceeding. Auto-detection can be wrong — the user corrects it here, once.

---

## Phase 2: User Questionnaire

Skills can declare **install-time questions** in their package definition. Questions are asked once during install; answers are stored in the lockfile and used for template rendering.

### Question Declaration (in Dolt)

```sql
-- package_questions table
| package_id | question_id | prompt                                    | type     | default        | choices                          |
|------------|-------------|-------------------------------------------|----------|----------------|----------------------------------|
| commit-msg | lang        | What languages does this repo use?        | multi    | (auto-detect)  | python,typescript,go,rust,java   |
| commit-msg | style       | Commit message style?                     | choice   | conventional   | conventional,freeform,gitmoji    |
| pr-review  | team_size   | How many people work on this repo?        | choice   | small          | solo,small,large                 |
| pr-review  | review_tone | Preferred review tone?                    | choice   | direct         | gentle,direct,thorough           |
```

### Question Types

| Type | Behavior |
|------|----------|
| `choice` | Single selection from predefined options |
| `multi` | Multiple selections from predefined options |
| `text` | Free text input |
| `confirm` | Yes/no |
| `auto` | Filled from repo profile, shown for confirmation |

### Answer Storage

Answers are recorded in the lockfile under each skill:

```toml
[[skills]]
id = "commit-msg"

  [skills.answers]
  lang = ["python", "typescript"]
  style = "conventional"
  answered_at = "2026-02-22T10:00:00Z"
```

On upgrade, existing answers are preserved. New questions (added in a newer version) prompt the user; removed questions are silently dropped.

---

## Phase 3: Jinja2 Template Rendering

Skill files can contain Jinja2 templates that are rendered at install time using the repo profile and user answers as context. This replaces runtime guessing with install-time configuration.

### Template Syntax

Skill files with extension `.md.j2` (or `.sh.j2`, `.toml.j2`) are rendered during install. The `.j2` extension is stripped in the output.

**Example: `skills/commit-msg/main.md.j2`**

```markdown
# Commit Message Assistant

You are helping with the **{{ repo.name }}** repository.

{% if answers.style == "conventional" %}
## Commit Format
Use conventional commits: `type(scope): description`

Allowed types: feat, fix, docs, style, refactor, test, chore
{% elif answers.style == "gitmoji" %}
## Commit Format
Use gitmoji format: `:emoji: description`
{% else %}
## Commit Format
Use clear, descriptive commit messages. No specific format required.
{% endif %}

## Languages in This Repo
{% for lang in answers.lang %}
- {{ lang }}
{% endfor %}

## Context
- Primary language: {{ repo.primary_language }}
- Test framework: {{ repo.test_frameworks | join(", ") }}
- CI: {{ repo.ci_system }}
```

### Template Context

Templates receive the full repo profile and user answers:

```python
context = {
    "repo": {
        "name": "my-api",
        "root": "/Users/rand/projects/my-api",
        "primary_language": "python",
        "languages": ["python", "typescript"],
        "frameworks": ["fastapi", "react"],
        "test_frameworks": ["pytest", "jest"],
        "ci_system": "github-actions",
        "monorepo": False,
        "git_conventions": "conventional",
    },
    "answers": {
        "lang": ["python", "typescript"],
        "style": "conventional",
    },
    "env": {
        "synaptic_root": "/Users/rand/projects/my-api/.synaptic",
        "synaptic_channel": "main",
        "synaptic_agents": "claude",
    },
}
```

### Rendering Rules

1. Templates are rendered **before** file checksums are computed — the lockfile records the hash of the rendered output, not the template.
2. Rendered output is shown to the user during the approval phase. They see what the skill will actually say, not the template source.
3. Re-rendering happens on `synaptic upgrade` if answers or repo profile have changed. Unchanged templates produce identical output (deterministic rendering).
4. Non-template files (`.md`, `.sh` without `.j2`) are materialized as-is.

---

## Phase 4: Approval & Dry Run

Before any files are written, the installer presents a complete plan.

### Standard Install

```
synaptic install commit-msg claude-history

═══ Install Plan ═══

Skills to install:
  commit-msg v1.3.0 (claude variant)
    → .synaptic/skills/commit-msg/main.md         (rendered from template)
    → .synaptic/skills/commit-msg/hooks/pre-commit.sh

  claude-history v2.1.0 (claude variant)
    → .synaptic/skills/claude-history/main.md
    → .synaptic/skills/claude-history/hooks/pre-bash.sh

  common-utils v1.0.0 (dependency of claude-history)
    → .synaptic/shared/common-utils/helpers.sh

External dependencies:
  python3>=3.11     ✓ found 3.11.4
  agent-teams-mail  ✗ not found — will install via: cargo install agent-teams-mail

Hooks to register:
  PreToolUse  → commit-msg/hooks/pre-commit.sh    (priority 10, blocking)
  PreToolUse  → claude-history/hooks/pre-bash.sh   (priority 15, blocking)

[Proceed? y/N]
```

### Dry Run

`synaptic dry-run install commit-msg` shows the same plan without executing. Useful for:
- Previewing what a skill will install before committing
- Verifying template rendering without side effects
- Checking dependency status without installing anything

The dry-run output includes rendered template previews:

```
═══ Dry Run: commit-msg ═══

Rendered: .synaptic/skills/commit-msg/main.md
────────────────────────────────────────────
# Commit Message Assistant

You are helping with the **my-api** repository.

## Commit Format
Use conventional commits: `type(scope): description`
...
────────────────────────────────────────────

No files written. No dependencies installed.
```

---

## Phase 5: Lockfile Recording

After successful install, the lockfile captures everything needed to reproduce or verify the install:

```toml
[[skills]]
id = "commit-msg"
version = "1.3.0"
dolt_commit = "a3f9c21"
channel = "main"
variant = "claude"
installed_at = "2026-02-22T10:05:00Z"
install_scope = "project"
template_rendered = true

  [skills.files]
  "skills/commit-msg/main.md" = "sha256:rendered_hash..."
  "skills/commit-msg/hooks/pre-commit.sh" = "sha256:script_hash..."

  [skills.answers]
  lang = ["python", "typescript"]
  style = "conventional"
  answered_at = "2026-02-22T10:00:00Z"

  [skills.requirements]
  tools = ["python3>=3.11"]
  tools_verified = { "python3" = "3.11.4" }
  agents = ["claude"]
  cli_installed = ["agent-teams-mail"]
  cli_versions = { "agent-teams-mail" = "0.4.2" }
  acknowledged_at = "2026-02-22T10:04:30Z"

  [skills.repo_profile_snapshot]
  # Snapshot of repo profile at install time
  # Used to detect when re-rendering is needed on upgrade
  primary_language = "python"
  languages_hash = "sha256:lang_list_hash..."
```

**Key property**: after install, no skill invocation ever checks dependencies. The lockfile is the proof. Only `synaptic upgrade` or `SYNAPTIC_STARTUP_MODE=check` re-verifies.

---

## Upgrade Behavior

When `synaptic upgrade` runs:

1. Fetch latest versions from Dolt for installed skills
2. Compare against lockfile versions
3. For changed skills:
   a. Check if new questions were added → prompt user
   b. Check if repo profile changed since install → re-render templates
   c. Check if dependencies changed → verify new requirements
4. Present upgrade plan (same format as install)
5. On approval: re-materialize changed files, update lockfile
6. Unchanged skills: no action, no re-verification

### Answer Preservation

```
Upgrading commit-msg v1.3.0 → v1.4.0

Existing answers preserved:
  lang = ["python", "typescript"]
  style = "conventional"

New question (added in v1.4.0):
  Include scope in commit messages? [Y/n]: y

Updated answer:
  include_scope = true
```

---

## First-Time Repo Setup

For a brand new repo with no `.synaptic/` directory:

```bash
synaptic init
```

This runs the full flow:
1. Repo detection
2. Suggest a starter skill set based on repo type
3. User questionnaire for selected skills
4. Full install with approval

Alternatively, `synaptic install <specific-skill>` on an uninitialized repo triggers `init` automatically.

---

## Repo Profile Schema

The repo profile is stored in `.synaptic/repo-profile.toml` (gitignored, regenerated on init/upgrade):

```toml
[repo]
name = "my-api"
root = "/Users/rand/projects/my-api"
detected_at = "2026-02-22T10:00:00Z"

[languages]
detected = ["python", "typescript"]
confirmed = ["python", "typescript"]  # user-confirmed at install

[package_managers]
detected = ["pip", "npm"]

[frameworks]
detected = ["fastapi", "react"]

[testing]
frameworks = ["pytest", "jest"]

[ci]
system = "github-actions"
config_path = ".github/workflows/"

[structure]
monorepo = false
has_claude_md = true
```

The `detected` vs `confirmed` distinction allows the system to re-detect on upgrade and show the user what changed ("we now see Go files — want to add Go-specific skills?").

---

## Security Considerations

### install_cmd Execution

The `install_cmd` field in `package_deps` contains shell commands (e.g., `cargo install agent-teams-mail`). These are:

1. **Always shown to the user** before execution — never run silently
2. **Recorded in the lockfile** with the exact command that was run
3. **Only executed at install/upgrade time** — never at runtime
4. **Subject to the approval gate** — user must explicitly confirm

Future enhancements:
- Package signing (verify commands haven't been tampered with on the Dolt server)
- Command allowlisting (restrict to known safe install patterns)
- Checksum pinning for installed binaries

### Template Rendering

Jinja2 templates are rendered in a **sandboxed environment**:
- No file system access from templates
- No command execution from templates
- Only the defined context variables are available
- `{% include %}` and `{% import %}` are disabled

Templates produce text output only. They cannot execute code or access system resources.
