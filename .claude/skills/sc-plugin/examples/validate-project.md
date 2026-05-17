# Example: Validate Project Install

Utterance:

```text
validate team-lead in this repo
```

Command:

```bash
sc validate team-lead --scope project --json
```

Expected JSON shape:

```json
{
  "ok": true,
  "pass": false,
  "packages": [
    {
      "package": "team-lead",
      "scope": "project",
      "aggregate_status": "error",
      "dependency_summary": {
        "tracked": 2
      },
      "items": [
        {
          "kind": "file",
          "state": "modified",
          "path": "SKILL.md",
          "severity": "warn"
        },
        {
          "kind": "hook",
          "state": "missing",
          "code": "hook_not_registered",
          "hook_script": "hooks/pre-commit.sh",
          "severity": "warn"
        }
      ]
    }
  ]
}
```
