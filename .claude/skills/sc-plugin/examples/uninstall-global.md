# Example: Uninstall Global Package

Utterance:

```text
uninstall team-lead globally
```

Command:

```bash
sc uninstall team-lead --scope global --json
```

Expected JSON shape:

```json
{
  "ok": true,
  "removed": {
    "package": "team-lead",
    "scope": "global",
    "hooks_removed": 1
  }
}
```
