#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/sc-installer-test.XXXXXX")"
trap 'chmod -R u+w "$TMP_ROOT" 2>/dev/null || true; rm -rf "$TMP_ROOT" 2>/dev/null || true' EXIT

HOME_DIR="$TMP_ROOT/home"
BIN_DIR="$TMP_ROOT/bin"
mkdir -p "$HOME_DIR" "$BIN_DIR" "$TMP_ROOT/go-cache" "$TMP_ROOT/go-mod-cache"

export HOME="$HOME_DIR"
export USERPROFILE="$HOME_DIR"
export SC_INSTALL_BIN_DIR="$BIN_DIR"
export PATH="$BIN_DIR:$PATH"
export GOCACHE="$TMP_ROOT/go-cache"
export GOMODCACHE="$TMP_ROOT/go-mod-cache"
export GOFLAGS="-modcacherw"

bash "$REPO_ROOT/scripts/install.sh"

[[ -x "$BIN_DIR/sc" ]] || { echo "missing installed sc binary" >&2; exit 1; }
[[ -f "$HOME_DIR/.claude/skills/sc-plugin/SKILL.md" ]] || { echo "missing installed skill" >&2; exit 1; }
[[ -f "$HOME_DIR/.sc/config.toml" ]] || { echo "missing config.toml" >&2; exit 1; }

VERSION_OUTPUT="$("$BIN_DIR/sc" --version)"
[[ "$VERSION_OUTPUT" == *"sc version "* ]] || { echo "unexpected version output: $VERSION_OUTPUT" >&2; exit 1; }

cat > "$HOME_DIR/.sc/config.toml" <<'EOF'
[dolt]
branch = "beta"
EOF
printf 'user note\n' > "$HOME_DIR/.claude/skills/sc-plugin/USER-NOTES.md"
mkdir -p "$HOME_DIR/.claude/agents"
printf 'assistant instructions\n' > "$HOME_DIR/.claude/agents/my-agent.md"
printf 'locally edited\n' > "$HOME_DIR/.claude/skills/sc-plugin/SKILL.md"
printf '#!/usr/bin/env bash\necho stale\n' > "$BIN_DIR/sc"
chmod +x "$BIN_DIR/sc"

bash "$REPO_ROOT/scripts/install.sh"

grep -Fq 'branch = "beta"' "$HOME_DIR/.sc/config.toml" || { echo "config was not preserved" >&2; exit 1; }
grep -Fq 'Thin Claude skill wrapper' "$HOME_DIR/.claude/skills/sc-plugin/SKILL.md" || { echo "managed skill file was not refreshed" >&2; exit 1; }
grep -Fq 'user note' "$HOME_DIR/.claude/skills/sc-plugin/USER-NOTES.md" || { echo "unmanaged file was removed" >&2; exit 1; }
grep -Fq 'assistant instructions' "$HOME_DIR/.claude/agents/my-agent.md" || { echo "unrelated file outside managed skill tree was modified" >&2; exit 1; }
VERSION_OUTPUT="$("$BIN_DIR/sc" --version)"
[[ "$VERSION_OUTPUT" == *"sc version "* ]] || { echo "unexpected version output after rerun: $VERSION_OUTPUT" >&2; exit 1; }

cp -a "$HOME_DIR/.claude/skills/sc-plugin" "$TMP_ROOT/skill-before-failure"
if SC_INSTALL_TEST_FAIL_AFTER_SKILL_COPY=1 bash "$REPO_ROOT/scripts/install.sh"; then
  echo "expected simulated installer failure" >&2
  exit 1
fi
diff -ru "$TMP_ROOT/skill-before-failure" "$HOME_DIR/.claude/skills/sc-plugin" >/dev/null || {
  echo "skill tree changed after simulated copy failure" >&2
  exit 1
}
