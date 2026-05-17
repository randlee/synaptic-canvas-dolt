# Example: Upgrade To Explicit Version In Project Scope

Utterance:

```text
upgrade team-lead to 1.3.0 on beta in this repo
```

Command:

```bash
sc upgrade team-lead --branch beta --version 1.3.0 --scope project --json
```

Expected JSON shape:

```json
{
  "ok": true,
  "upgrades": [
    {
      "package": "team-lead",
      "scope": "project",
      "from_version": "1.2.0",
      "to_version": "1.3.0",
      "to_branch": "beta"
    }
  ]
}
```
