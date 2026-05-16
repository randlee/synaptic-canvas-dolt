# Dolt API Reference for Synaptic Canvas

This document records how `sc` connects to Dolt, with direct citations from
official Dolt documentation. It is the authoritative trace from requirements
BR-001 through BR-009 to Dolt-supplied API contracts.

It supplements the normative requirements and architecture documents and should
be reviewed alongside ADR-0002 when transport behavior or client selection is
changed.

All information below is sourced verbatim from official Dolt documentation.
URLs are provided for every claim.

---

## 1. Deployment Models

Three Dolt deployment models are supported by Synaptic Canvas. They share one
public CLI contract but use different transport adapters.

For MVP, these modes are not symmetrical for writes: `HTTPClient` is a
supported read transport only, while write-path admin commands require
`SQLClient` or `CLIReader` against a writable server or local clone.

### 1.1 DoltHub.com — HTTP SQL API

DoltHub public repos expose an HTTP REST endpoint only. MySQL wire protocol
is **not** available on dolthub.com.

- **Source:** https://docs.dolthub.com/products/dolthub/api/sql

SQL query endpoint — GET with `?q=` query parameter:
```
GET https://www.dolthub.com/api/v1alpha1/{owner}/{database}/{ref}?q={url-encoded-sql}
```

Example for `main` branch:
```
GET https://www.dolthub.com/api/v1alpha1/randlee/synaptic-canvas/main?q=SELECT+*+FROM+packages
```

**Note:** The `{ref}` path segment must be URL-path-encoded. Use `url.PathEscape(branch)`
for any branch name that may contain `/` or other URL-unsafe characters.

**Request method:** DoltHub SQL API reads use GET with the SQL in the `q`
query parameter. `HTTPClient` must keep generated SQL short enough for GET and
must not switch to POST for long queries.

Response envelope:
```json
{
  "query_execution_status": "Success",
  "query_execution_message": "",
  "repository_owner": "owner",
  "repository_name": "database",
  "commit_ref": "main",
  "sql_query": "SELECT ...",
  "schema": [
    {"columnName": "id", "columnType": "varchar"}
  ],
  "rows": [
    {"id": "team-lead"}
  ]
}
```

DoltHub commonly returns SQL row values as JSON strings, including numeric SQL
types. `HTTPClient` decoders must coerce from strings where needed instead of
assuming JSON numbers or Go booleans.

Authentication for private repos (source: https://docs.dolthub.com/products/dolthub/api/authentication):
```
Authorization: token <TOKEN>
```

> "API tokens can be used to authenticate calls to the SQL API over Basic
> Authentication. First, create an API token in your settings on DoltHub."

> "You must include a ref name (branch, tag, commit hash, etc) when making
> authenticated calls to the SQL API using a token. Unauthenticated API
> requests do not require this. They use the default branch (main or master)."

Branch targeting is in the URL path — no session mutation.

### 1.2 Hosted Dolt — MySQL Protocol

Hosted Dolt or another MySQL-compatible Dolt server can be accessed through
`SQLClient`.

- **Source:** https://docs.dolthub.com/products/hosted/getting-started

Connection format:
```
mysql -h"{deployment}.dbs.hosted.doltdb.com" -u"{user}" -p"{password}"
```

Go DSN (source: https://github.com/dolthub/dolt/.../sql-driver-mysql-test.go):
```go
dsn = user + "@tcp(host:port)/" + db
```

Branch in DSN path (source: https://docs.dolthub.com/sql-reference/version-control/branches):
```
user@tcp(host:port)/database/feature-branch
```

Branch-qualified table reference (no session mutation):
```sql
SELECT * FROM `randlee/synaptic-canvas/main`.packages WHERE id = ?
```

> "You can also use fully-qualified names with database revisions in your
> queries."
> — https://docs.dolthub.com/sql-reference/version-control/branches

### 1.3 Local dolt sql-server — MySQL Protocol

Self-hosted `dolt sql-server` on the developer's machine or a private server.
Same MySQL protocol as Hosted Dolt. Synaptic Canvas may use `SQLClient`
against this deployment model when configured to do so.

- **Source:** https://docs.dolthub.com/sql-reference/supported-clients/clients

> "The `dolt sql-server` command starts a MySQL compatible server for the Dolt
> database on port 3306 with no authentication."
>
> "Once a server is running, any MySQL client should be able to connect to Dolt
> SQL Server in the exact same way it connects to a standard MySQL database."

### 1.4 CLIReader — Local dolt binary

Shells out to `dolt sql -q` with the local dolt binary. Requires dolt
installed in PATH and a local repo clone. Synaptic Canvas supports this mode as
the local-clone adapter for repositories that use a checked-out Dolt database.
When `cli` is the effective transport, `sc` resolves the clone root from
`--dolt-dir`, then `SC_DOLT_DIR`, then `dolt.dir`, then upward auto-detection
of a local `.dolt/` directory from the current working directory. An explicit
`--dolt-dir` supplied alongside effective `http` or `sql` client selection is
treated as an invalid conflicting transport input.

### 1.5 Client Selection Precedence

Synaptic Canvas resolves the active Dolt client mode using a fixed five-level
precedence chain:

1. explicit CLI flag
2. environment variable
3. project config
4. machine config
5. compiled default

For the current CLI, the effective source priority is:

- `--client`
- `SC_DOLT_CLIENT`
- project `dolt.client` from repo-local config
- machine `dolt.client` from `~/.sc/config.toml`
- compiled default client mode

The same pattern applies to related transport selectors such as `--dolt-dir`
and `--branch`, but client mode precedence is the authoritative transport
selection chain for Phase 4.

---

## 2. Branch Access — Per-Query, No Session Mutation

Requirements BR-004 and BR-005 prohibit session mutation for branch selection.

### 2.1 HTTP API — Branch in URL

Branch is a path segment. Each request targets a specific ref independently.
No session state exists between HTTP calls. SQL sent through this endpoint
should use unqualified table names such as `packages` and `package_files`; the
URL path selects the repository and ref.

```
GET https://www.dolthub.com/api/v1alpha1/randlee/synaptic-canvas/main?q=...
GET https://www.dolthub.com/api/v1alpha1/randlee/synaptic-canvas/develop?q=...
```

### 2.2 MySQL Protocol — Qualified Table Reference

Branch encoded in table qualifier. No `USE` or `DOLT_CHECKOUT` required.

```sql
-- main branch
SELECT * FROM `randlee/synaptic-canvas/main`.packages;
-- develop branch
SELECT * FROM `randlee/synaptic-canvas/develop`.package_files;
```

Source: https://docs.dolthub.com/sql-reference/version-control/branches

### 2.3 MySQL Protocol — AS OF Clause

Branch or commit hash in `AS OF` — reads point-in-time without session change.

```sql
SELECT * FROM packages AS OF 'main';
SELECT * FROM packages AS OF 'develop';
SELECT * FROM packages AS OF 'ia1ibijq8hq1llr7u85uivsi5lh3310p';
```

Source: https://docs.dolthub.com/sql-reference/version-control/querying-history

> "`AS OF` always names a revision at a specific Dolt commit."
> "Each table in a query can use a different `AS OF` clause."

### 2.4 What NOT to Use

`DOLT_CHECKOUT()` mutates session state and is explicitly prohibited by BR-004:

> "`DOLT_CHECKOUT()` with a branch argument has two side effects on your session
> state: 1. The session's current database is now the unqualified database name.
> 2. For the remainder of this session, references to the unqualified name of
> this database will resolve to the branch checked out."
> — https://docs.dolthub.com/sql-reference/version-control/dolt-sql-procedures

---

## 3. Requirement Traceability

| Requirement | Dolt API Contract | Section |
|-------------|------------------|---------|
| BR-001 All reads use explicit branch | HTTP: branch in URL path; MySQL: qualified table or AS OF | §2 |
| BR-002 Branch precedence: --branch → env → main | Enforced in sc; Dolt ref accepts any valid branch name | §2.1 |
| BR-003 Default branch = main | HTTP default ref = main when unauthenticated | §1.1 |
| BR-004 Ignore session branch | Do not use DOLT_CHECKOUT; use qualified refs | §2.4 |
| BR-005 No session mutation | HTTP stateless by design; MySQL qualified refs stateless | §2.1–2.3 |
| BR-006 Explicit branch-selected parallel reads | HTTP: branch in URL with unqualified table names; MySQL: separate qualifiers per query | §2 |
| BR-008 Branch values = Dolt branch names | URL path and SQL qualifier accept Dolt branch names directly | §2.1–2.2 |
| BR-009 Branch and version independent | Branch = ref in URL/qualifier; version = package metadata field | §2 |

---

## 4. MVP vs Future Deployment

| Mode | Supported | Notes |
|------|-----------|-------|
| DoltHub HTTP API | **Yes** | Use `HTTPClient`; public repo = no auth; private = token header |
| Hosted Dolt MySQL | **Yes** | Use `SQLClient`; branch-qualified table references |
| Local dolt sql-server | **Yes** | Use `SQLClient`; dev/private deployments |
| CLIReader (dolt binary) | **Yes** | Use `CLIReader`; local clone / subprocess mode |

Private repo access in MVP requires a DoltHub API token stored in sc config.
Public repo requires no credentials.

Live DoltHub verification is manual/AI-driven integration testing only. CI must
not depend on a live DoltHub repository, branch, network path, or mutable remote
state. Any live test must be opt-in, skipped by default, and configurable with a
dedicated project test repository and branch containing deterministic fixture
data.

Routine CI instead uses simulator-backed or equivalent adapter harnesses for all
three transports:
- `httptest.Server` for `HTTPClient`
- fake `database/sql` driver coverage for `SQLClient`
- injected subprocess shim coverage for `CLIReader`
