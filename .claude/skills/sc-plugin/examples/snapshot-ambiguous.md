# Example: Snapshot Requires Explicit Scope On Ambiguity

Utterance:

```text
snapshot team-lead
```

Command:

```bash
sc snapshot team-lead --json
```

Expected JSON shape:

```json
{
  "ok": false,
  "error": {
    "code": "ambiguous_target",
    "message": "package \"team-lead\" is installed in multiple scopes; pass --scope"
  }
}
```

Wrapper behavior:

- Surface the ambiguity instead of choosing a scope.
- Ask the user to retry with `project` or `global`.
