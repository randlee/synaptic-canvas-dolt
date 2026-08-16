# Import Conversion Rules

Use this checklist when converting a skill or package into Synaptic Canvas.

## 1. Package Topology

- Prefer one package with multiple skills when they share:
  - install-and-troubleshooting guidance
  - runtime scripts or launch helpers
  - the same external CLI/app dependencies
  - the same package identity from a user point of view
- Split packages only when:
  - install/runtime dependencies differ materially
  - users would reasonably install one without the other
  - exported marketplace identity is already distinct

## 2. Dependency Rules

- Treat CLI and app dependencies as the same class of install dependency.
- Record them in package metadata for `sc install`.
- Keep normal manual installation instructions for non-`sc` users.
- For simple vendor-owned CLIs, point to official upstream docs.
- For complex curated runtimes, keep a local install guide with the tested
  subset, version ceilings, extras, and validation steps.

## 3. Install-And-Troubleshooting Rules

Every imported skill that depends on an external CLI/app should end up with:

- standard install/troubleshooting instructions that work without Synaptic Canvas
- a short Synaptic Canvas subsection such as:
  - `sc install <package>`
  - `sc upgrade <package>`
  - `sc uninstall <package>`

That `sc` subsection is additive. It does not replace the normal install path.

## 4. Export Compatibility

Imported packages must still work in traditional marketplaces:
- do not make package docs depend on Synaptic Canvas-only concepts
- do not remove standard human installation instructions
- do not assume the consumer has `sc`

## 5. Skill QA Reference

When a converted package includes skills or agents, review them against:
- `/Users/randlee/Documents/github/synaptic-canvas/docs/claude-code-skills-agents-guidelines.md`

## 6. Candidate-Specific Rules

### `claude-history/.claude`

- Add install-and-troubleshooting guidance; it is missing as a dedicated
  reference file.
- Preserve agent delegation structure.
- Keep manual `claude-history` CLI install guidance.
- Add a short Synaptic Canvas subsection for package lifecycle commands.

### `sc-docling-pdf`

- Treat current package shape as a reference-quality baseline.
- Preserve detailed local install instructions because Docling runtime setup is
  complex and version-sensitive.

### `sc-launchpad` + `sc-launch-term`

- Prefer merging into one package with multiple skills.
- Shared install guidance should cover Claude Code, Codex CLI, and Gemini CLI.
- Point to official upstream install sources for those fast-moving CLIs.
- Keep one shared install-and-troubleshooting guide plus per-skill launch notes.

### `sprint-report`

- Preserve templates/assets as package artifacts.
- Make sure manifest metadata reflects template files actually used by the skill.

## 7. Import Readiness Gate

Ready for `sc admin import` when:
- topology is settled
- manifest/plugin metadata matches artifacts
- dependency guidance exists
- install-and-troubleshooting works for non-`sc` users
- a short Synaptic Canvas subsection exists
- no obvious artifact is untracked
