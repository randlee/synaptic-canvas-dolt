# Synaptic Canvas (`sc`)

A Dolt-backed package management system for Claude Code skills, agents, and commands.

## Overview

Synaptic Canvas provides a CLI tool (`sc`) for discovering, installing, and managing packages that extend Claude Code's capabilities. Packages are stored in a [Dolt](https://www.dolthub.com/) database, providing Git-like versioning, branching (used as release channels), and content-addressable integrity verification.

**DoltHub:** [randlee/synaptic-canvas](https://www.dolthub.com/repositories/randlee/synaptic-canvas)

## Features

- **Package Discovery** — Browse and search available skills, agents, commands, and hooks
- **Dolt-Backed Storage** — Git-like versioning with branch-based release channels
- **SHA-256 Integrity** — Every file is hashed at ingest and verified at install
- **Manifest Reconstruction** — Database records are assembled into installable manifests
- **Interface-Based Architecture** — Clean separation between database, models, and CLI layers

## Architecture

```
src/
├── cmd/                    # Cobra CLI commands
│   └── root.go             # Root command with version info
├── internal/
│   ├── config/             # Configuration management
│   ├── logging/            # Structured logging (slog)
│   └── output/             # JSON/table output formatting
├── pkg/
│   ├── dolt/               # Database abstraction layer
│   │   ├── client.go       # Client interface + SQLClient
│   │   ├── queries.go      # Parameterized SQL builders
│   │   └── mock.go         # MockClient for testing
│   └── models/             # Data model structs
│       ├── package.go      # Package, PackageFile, PackageDep, etc.
│       └── manifest.go     # Manifest reconstruction
└── main.go                 # Entry point
```

## Requirements

- Go 1.26+
- Dolt (for local database access)

## Build

```bash
# Build from source
cd src && go build -o sc .

# Run tests
go test -race ./...

# Lint
golangci-lint run ./...
```

## Usage

```bash
# Show version
sc --version

# List packages (coming in Sprint 3.1)
sc list

# Install a package (coming in Sprint 3.2)
sc install <package-id>
```

## Design Documents

- [CLI Design](docs/synaptic-canvas-cli.md) — Command surface and architecture
- [Schema Spec](docs/synaptic-canvas-schema.md) — Dolt table definitions
- [Export Pipeline](docs/synaptic-canvas-export-pipeline.md) — Database to filesystem reconstruction
- [Install System](docs/synaptic-canvas-install-system.md) — Package installation mechanics
- [Hook System](docs/synaptic-canvas-hook-system.md) — Pre/post install hooks

## Development

Development follows a phased plan with sprint-based delivery and automated QA gates. See [CLAUDE.md](CLAUDE.md) for project conventions and dev-qa loop details.

### Current Status

| Sprint | Description | Status |
|--------|-------------|--------|
| 1.1 | Project Scaffold | ✅ Complete |
| 1.2 | Dolt Client | ✅ Complete |
| 2.1 | Export Pipeline | 🔜 Planned |
| 2.2 | Install System | 🔜 Planned |
| 3.1 | List/Info Commands | 🔜 Planned |
| 3.2 | Install Command | 🔜 Planned |

## License

MIT
