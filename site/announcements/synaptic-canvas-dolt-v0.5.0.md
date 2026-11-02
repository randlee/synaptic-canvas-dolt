# synaptic-canvas-dolt v0.5.0 — Distribution, Lifecycle, and Contract Hardening

**Released:** May 17, 2026 · **Tag:** `v0.5.0` · **Previous:** `v0.4.0` (May 16, 2026)

[Release notes](https://github.com/randlee/synaptic-canvas-dolt/releases/tag/v0.5.0) · [Repository](https://github.com/randlee/synaptic-canvas-dolt) · [Documentation site](https://randlee.github.io/synaptic-canvas-dolt/)

---

## Consumer (Claude Code user) — install, upgrade, and remove without surprises

**As a Claude Code user, I want to discover and install packages by name with SHA-256 verification at install, so that I can extend my agent with trusted, reproducible skills.**

v0.5.0 is the release where the `sc` CLI stops being a development checkout and starts behaving like an installed tool. The Phase 4 work rebuilt the install surface end to end:

- **An installer and local-distribution path.** Sprint 4.5 gave the CLI its own installer and local distribution flow, so `sc` can be installed, upgraded and removed as a managed artifact rather than a hand-copied binary.
- **Atomic uninstall and PATH hardening.** Sprint 4.9 made uninstall atomic and hardened how the installer manipulates `PATH`, so a failed or partial removal does not leave a half-registered tool behind or corrupt a shell profile.
- **Operations layer extraction.** Sprint 4.10 pulled install, upgrade and uninstall into an explicit operations layer, so each lifecycle step is a named, testable operation instead of logic scattered across commands.

The distribution matrix widened at the same time. Release archives now ship for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64 and — new at this release — windows/arm64. Homebrew users can install from the `randlee/tap` formula (`brew install randlee/tap/sc`), and the release pipeline now opens a winget manifest pull request against `microsoft/winget-pkgs`; availability there depends on Microsoft's review, so the release archives remain the guaranteed path. SHA-256 checksums are published alongside every archive.

Existing package installs keep the same integrity guarantee as before: per-file SHA-256 verification at install time. The difference is the surrounding machinery — cleaner upgrades, a real uninstall, and platform coverage for ARM64 Windows.

## Release Maintainer / Curator — a release pipeline that cannot silently degrade

**As a release maintainer, I want to manage branch-based release channels and curate the authorized catalog, so that consumers only see stable, approved packages.**

The release engineering in this version is as significant as the user-facing surface. The goreleaser pipeline gained a production build tag and a windows/arm64 secondary release target, and the release workflow no longer falls back to alternative token sources — if the publishing credential is missing, the job fails loudly instead of publishing from an unintended identity. Winget publishing was enabled as an explicit channel, with the pull request against `microsoft/winget-pkgs` opened from the release workflow rather than by hand.

Error-contract completeness and fixture verification landed alongside it. Sprint 4.6 closed the error-contract gaps and Sprint 4.7 hardened the `sc:plugin` fixture verification with an architecture cleanup, so the release gates check the same contracts the shipped binary relies on. Sprint 4.8 then hardened the installer and closed the documentation gaps that had accumulated across the earlier sprints.

The net effect for a maintainer is that a release is reproducible from the workflow definition: the tag, the archives, the checksums and the channel submissions all come from one pipeline. Branch-based release channels and the authorized-catalog discipline are unchanged; what changed is the confidence that a release either completes or fails visibly.

## Registry Operator / DBA — client selection and backend parity

**As a registry operator, I want to administer the Dolt backend (schema, remotes, sync, sandbox isolation), so that the store stays consistent, backed-up, and available.**

Sprint 4.2 added client selection and brought the backend contract into parity, so the CLI can target a chosen Dolt client without each command re-deciding how to connect. Sprint 4.3 added installed-state readback and audit symmetry: what the CLI reports as installed is read back from the store rather than inferred, and the audit surface reports the same state the installer acted on. The sandbox-DB discipline and the wrong-schema guard remain in place — pointing `sc` at a beads database is still rejected.

## Package Author — a stricter JSON and error contract

**As a package author, I want to publish a skill/agent/command/hook package with a manifest and SHA-hashed files, so that consumers can install an exact, verified version.**

Sprint 4.1 hardened the JSON contract and the typed error surface. For an author, that means `sc admin import` and the manifest-reconstruction pipeline now reject malformed manifests and mismatched contracts with typed errors instead of loose string failures. The manifest and SHA-hash model itself is unchanged: a package is still an exact set of hashed files, and a consumer still verifies each file at install. The stricter contract is there to make a bad import fail at the boundary rather than at a consumer's install.

## Evaluator — skipped

No change to the diff-before-promote or branch-as-eval-gate workflow in this release. The version/branch/SHA tuple immutability rule is unchanged. Nothing for this persona to adopt.

## Not this release

- **A public winget listing.** The pipeline opens the manifest PR; Microsoft's review gates when `winget install` works. Until then, use the release archives or Homebrew.
- **Scoop.** The `randlee/scoop-bucket` exists for other SC tools but carries no `sc` manifest at this tag; there is no Scoop install path to document.
- **New discovery/authoring surfaces.** v0.5.0 is a distribution and lifecycle release. The `sc:plugin` thin wrapper is the one new command surface; package discovery and authoring commands are not expanded here.

## Install

Homebrew (macOS/Linux):

```bash
brew install randlee/tap/sc
```

Release archives (all platforms, with `checksums.txt`):

```bash
# https://github.com/randlee/synaptic-canvas-dolt/releases/tag/v0.5.0
```

Build from source:

```bash
git clone https://github.com/randlee/synaptic-canvas-dolt && cd synaptic-canvas-dolt/src
go build -o sc .
```

## Upgrading from v0.4.0

v0.4.0 shipped one day earlier and had no installed-tool surface to speak of. If you built from source at v0.4.0, rebuild at `v0.5.0`; the JSON and error contracts are stricter, so a manifest that imported cleanly at v0.4.0 may now fail with a typed error that names the offending field. That is intentional — fix the manifest rather than pinning the older tag. There is no in-place binary upgrade from v0.4.0 because there was no installer to upgrade.
