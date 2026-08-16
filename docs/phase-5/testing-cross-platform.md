# Phase 5 Testing And Cross-Platform Guidance

Phase 5 touches release packaging, installer behavior, and smoke testing across
multiple backend and OS combinations. This document defines the required
cross-platform test posture.

## Release Matrix

Primary targets:

- `darwin/arm64`
- `windows/amd64`
- `linux/amd64`

Secondary targets:

- `darwin/amd64`
- `windows/arm64`
- `linux/arm64`

## Required Test Layers

1. CI validation
   - `go test ./...`
   - `go test ./... -race`
   - `golangci-lint run ./...`
   - `goreleaser check`

2. Release packaging validation
   - `goreleaser release --snapshot --clean`
   - verify archive names, checksums, and release-note generation

3. Installer validation
   - shell installer path on macOS/Linux
   - PowerShell installer path on Windows
   - rerun behavior with managed/unmanaged split preserved

4. Product smoke validation
   - local-clone backend via native `dolt`
   - live DoltHub-backed path
   - local-scope and global-scope install lifecycles

## Platform Rules

- `amd64` is the Go/GoReleaser architecture name for x64.
- `arm64` is the Go/GoReleaser architecture name for Apple Silicon and Windows/Linux ARM64.
- Winget publication status must be validated separately from Windows binary
  generation. A successful `windows/arm64` archive build does not imply Winget
  readiness.

## Non-CI Live Testing

- Live DoltHub verification is manual or AI-driven release evidence, not a CI
  requirement.
- Local clone smoke tests use the native `dolt` CLI directly for clone/pull/push
  and use `sc --json` for product behavior validation.
