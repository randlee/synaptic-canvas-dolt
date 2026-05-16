# Example: Install From Beta Globally

Utterance:

```text
install team-lead from beta globally
```

Command:

```bash
sc install team-lead --branch beta --scope global --json
```

Expected JSON shape:

```json
{
  "ok": true,
  "scope": "global",
  "package": {
    "id": "team-lead",
    "version": "1.2.0",
    "branch": "beta"
  },
  "install_root": "/Users/example/.claude/skills/team-lead"
}
```
