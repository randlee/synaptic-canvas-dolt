# Installation And Troubleshooting

This skill requires the `sc` CLI.

## Verify First

```bash
which sc && sc --version
```

If `sc` is present at a non-PATH location, either use the full path or export
that directory into PATH for the current session.

## Install `sc`

Use the normal Synaptic Canvas install path for your environment:
- the project installer scripts from this repository
- a local build from `src/`
- released binaries or package-manager distribution when available

Local build example:

```bash
cd /path/to/synaptic-canvas-dolt/src
go build -o /tmp/sc .
/tmp/sc --version
```

## Common Problems

### `sc: command not found`

- check PATH
- check common install locations
- use a full path for the session if needed

### Built locally but not globally installed

Use the built binary path directly:

```bash
/path/to/sc admin import <path> --branch develop --json
```

### Wrong repo or stale binary

```bash
sc --version
```

If behavior does not match the current worktree, rebuild from the current
repository checkout and use that binary explicitly.
