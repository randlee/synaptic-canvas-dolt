# Synaptic Canvas — Architectural Decisions

This document records key architectural decisions, the reasoning behind them, and alternatives considered.

---

## ADR-001: Dolt as Server-Side Infrastructure, Not Client Dependency

**Status:** Accepted

**Context:** The system needs a backend for skill storage, dependency management, and release channel promotion. Git + flat files was considered as an alternative.

**Decision:** Dolt is the centralized server (DoltHub or self-hosted). Users never run Dolt locally. The CLI is a thin client that queries Dolt via MySQL protocol.

**Reasoning:**
- Dependency management is inherently relational. Queries like "what breaks if I upgrade X" require joins across packages, dependencies, and variants. Git + JSON requires building a query layer that reimplements a database poorly.
- Script reuse across packages is a many-to-many relationship. In Git this requires symlinks/submodules. In Dolt it's a shared row.
- Centralized promotion means conflict resolution is an admin problem, not a user problem. `dolt log` provides a built-in audit trail.
- Single source of truth feeds multiple distribution channels (CLI pull, marketplace export, offline snapshots) without maintaining separate repositories.

**Alternatives Considered:**
- **Git + JSONL index**: Simpler initially, but dependency queries become O(n) file scans. No transactional guarantees on multi-file updates. Branch merges on blob columns are painful.
- **SQLite distributed via Git LFS**: Queryable but no branching model. Every client gets the full database. No server-side merge/promotion semantics.

---

## ADR-002: Promotion-Based QA Over Automated Testing

**Status:** Accepted

**Context:** Traditional packages use CI pipelines as quality gates. Skills — which modify agent behavior through prompts, hooks, and context injection — cannot be meaningfully tested with automated tests.

**Decision:** Quality assurance uses staged branch promotion with human judgment. Skills advance `develop → beta → main` based on observed real-world usage, not test suites.

**Reasoning:**
- You can't unit test a prompt. Output depends on model version, conversation state, available tools, and user codebase.
- Integration tests are expensive (API calls), non-deterministic, and brittle across model updates.
- Regression is invisible — a skill can silently degrade without any test catching it.
- The real quality signal is "I used this across several projects and it worked." That's a human judgment, not an assertion.

**Mitigations:**
- Dolt's dependency graph is queryable before promotion — you can assess blast radius.
- Rollback is a single `dolt_reset` command.
- Promotion is an auditable merge with author and timestamp.
- The marketplace export only reads from `main`, so casual users never see unproven skills.

**What This Explicitly Does Not Provide:**
- Automated regression detection
- Guaranteed backwards compatibility
- SLA on skill behavior across model versions

These are honest limitations, not design flaws. The architecture acknowledges them rather than pretending CI solves them.

---

## ADR-003: Install-Time Dependency Verification

**Status:** Accepted

**Context:** Skills have external dependencies (CLI tools, Python versions, other skills). The current pattern in skill systems is to check dependencies on every invocation, which wastes context window on repetitive verification that almost never changes.

**Decision:** Dependencies are verified **once at install time**, recorded in the lockfile with timestamps and version snapshots, and never rechecked until upgrade. The install script is the single point where the system interacts with the user about requirements.

**Reasoning:**
- Runtime dependency checking burns agent context on every skill invocation for something that changes only when the user modifies their environment.
- Install-time verification gives the user a single, clear approval moment: "these are the requirements, these are the installs needed, proceed?"
- The lockfile records what was verified and when. If a dependency disappears later (user uninstalls Python), the startup drift check catches it without per-invocation overhead.
- This mirrors how `npm install` and `cargo build` work — resolve once, record, replay.

**Implications:**
- The `synaptic install` flow must be thorough — it's the only chance to catch problems.
- The lockfile `acknowledged_at` timestamp records user consent.
- `SYNAPTIC_STARTUP_MODE=check` provides a lightweight drift detection without full re-verification.

---

## ADR-004: Install Script with Repo-Aware Configuration

**Status:** Accepted

**Context:** Different repositories have different needs. A Python ML repo needs different skills than a Go microservice. Prompts within skills may need project-specific values (repo name, primary language, team conventions). Currently there is no mechanism to customize skills at install time.

**Decision:** The install process includes repo detection, user questionnaire, and Jinja2 template rendering for prompts and configuration files.

**Reasoning:**
- Repo detection (language, framework, structure) can auto-select relevant skills and pre-fill configuration.
- User questions at install time ("what languages do you use?", "do you use conventional commits?") capture context that would otherwise be guessed wrong or asked repeatedly at runtime.
- Jinja2 templates in skill prompts allow `{{ repo_name }}`, `{{ primary_language }}`, `{{ team_conventions }}` to be filled once and baked into the materialized skill files.
- Verification at install time means the user sees exactly what will be installed and configured before approving.

**See:** [Install System Specification](./synaptic-canvas-install-system.md)

---

## ADR-005: Multiple Distribution Channels from Single Source

**Status:** Accepted

**Context:** Users have different needs — skill authors want direct Dolt access, casual users want a marketplace, CI environments need offline access.

**Decision:** Dolt is the single authoring environment. Distribution is a separate concern with three channels:
1. **Direct CLI pull** — queries Dolt, materializes locally
2. **Claude Code marketplace export** — read-only projection from `main` branch
3. **Lightweight snapshots** — JSONL + files or SQLite for offline/CI use

**Reasoning:**
- One workflow for skill development (work in Dolt branches, promote when confident).
- The marketplace is generated, not maintained — no second repository to keep in sync.
- SQLite snapshots serve as validation artifacts: compare against Dolt to confirm export correctness.
- The export pipeline is the easiest thing to test first — you can visually confirm "everything showed up in the proper location" in a Claude Code marketplace format.

---

## ADR-006: CLI as Bootstrap Entry Point

**Status:** Accepted

**Context:** The TUI and many features are themselves Synaptic Canvas packages. This creates a bootstrapping problem — you need the system to install the system.

**Decision:** The `synaptic` CLI is a standalone binary with zero skill dependencies. It is the bootstrap entry point. All other features (TUI, marketplace export, advanced commands) are optional packages installed via the CLI.

**Reasoning:**
- Avoids circular dependency: CLI installs everything, including the TUI that provides a richer CLI experience.
- First `synaptic install` on a new machine works with just the binary.
- The CLI handles all core operations (install, remove, upgrade, list, deps, dry-run) without any installed skills.

---

## Decision Log

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| 001 | Dolt as server-side infrastructure | Accepted | 2026-02-22 |
| 002 | Promotion-based QA | Accepted | 2026-02-22 |
| 003 | Install-time dependency verification | Accepted | 2026-02-22 |
| 004 | Install script with repo-aware config | Accepted | 2026-02-22 |
| 005 | Multiple distribution channels | Accepted | 2026-02-22 |
| 006 | CLI as bootstrap entry point | Accepted | 2026-02-22 |
