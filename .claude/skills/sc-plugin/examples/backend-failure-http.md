# Example: Backend Failure With Corrective Guidance

Utterance:

```text
list packages
```

Command:

```bash
sc list --json
```

Representative failure JSON:

```json
{
  "ok": false,
  "error": {
    "code": "backend_unavailable",
    "message": "failed to query package catalog",
    "details": {
      "client": "http",
      "cause_code": "http_timeout"
    },
    "suggested_action": "retry or switch to a reachable backend"
  }
}
```

Wrapper behavior:

- Preserve the top-level `error.code`.
- Mention that the active client was `http`.
- Surface the suggested action instead of inventing wrapper-specific advice.
