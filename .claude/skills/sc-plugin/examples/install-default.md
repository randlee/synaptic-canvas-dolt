# Example: Install Package With Default Scope And Branch

Utterance:

```text
install team-lead
```

Command:

```bash
sc install team-lead --json
```

Expected JSON shape:

```json
{
  "ok": true,
  "scope": "project",
  "package": {
    "id": "team-lead",
    "version": "1.2.0",
    "branch": "main"
  },
  "install_root": "/repo/.claude/skills/team-lead"
}
```
