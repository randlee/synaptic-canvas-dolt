# Example: Status With Local And Global Coexistence

Utterance:

```text
show package status
```

Command:

```bash
sc status --json
```

Expected JSON shape:

```json
{
  "ok": true,
  "packages": [
    {
      "package": "team-lead",
      "global": {
        "scope": "global",
        "version": "1.4.0",
        "branch": "main",
        "modification_summary": {
          "modified": 0
        }
      },
      "local": {
        "scope": "project",
        "version": "1.3.0",
        "branch": "main",
        "modifications": [
          {
            "kind": "file",
            "state": "modified",
            "path": "SKILL.md"
          }
        ]
      }
    }
  ]
}
```
