#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

note() {
  printf '%s\n' "$*"
}

prune_empty_dirs() {
  local current="$1"
  local stop="$2"
  while [[ -n "$current" && "$current" != "$stop" && "$current" != "/" ]]; do
    rmdir "$current" 2>/dev/null || return 0
    current="$(dirname "$current")"
  done
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SC_INSTALL_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
SRC_ROOT="$REPO_ROOT/src"
SKILL_SOURCE="$REPO_ROOT/.claude/skills/sc-plugin"
HOME_DIR="${SC_INSTALL_HOME:-${HOME:-}}"

[[ -n "$HOME_DIR" ]] || die "HOME is not set"
[[ -d "$SRC_ROOT" ]] || die "expected Go source at $SRC_ROOT"
[[ -d "$SKILL_SOURCE" ]] || die "expected sc:plugin source at $SKILL_SOURCE"
command -v go >/dev/null 2>&1 || die "go is required to build sc"

BIN_DIR="${SC_INSTALL_BIN_DIR:-$HOME_DIR/.local/bin}"
BIN_PATH="$BIN_DIR/sc"
CONFIG_DIR="$HOME_DIR/.sc"
CONFIG_PATH="$CONFIG_DIR/config.toml"
STATE_ROOT="$HOME_DIR/.synaptic/installers/sc-plugin"
SKILL_TARGET="$HOME_DIR/.claude/skills/sc-plugin"
SKILL_MANIFEST="$STATE_ROOT/managed-files.txt"
BINARY_MANIFEST="$STATE_ROOT/binary-path.txt"

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sc-install.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

mkdir -p "$BIN_DIR" "$CONFIG_DIR" "$STATE_ROOT" "$SKILL_TARGET"

BUILD_PATH="$TMP_DIR/sc"
if [[ -n "${SC_INSTALL_BINARY:-}" ]]; then
  BUILD_PATH="$SC_INSTALL_BINARY"
else
  (
    cd "$SRC_ROOT"
    go build -o "$BUILD_PATH" .
  )
fi
[[ -f "$BUILD_PATH" ]] || die "expected built binary at $BUILD_PATH"

if [[ -f "$BINARY_MANIFEST" ]]; then
  PREVIOUS_BIN="$(cat "$BINARY_MANIFEST")"
  if [[ -n "$PREVIOUS_BIN" && "$PREVIOUS_BIN" != "$BIN_PATH" && -f "$PREVIOUS_BIN" ]]; then
    rm -f "$PREVIOUS_BIN"
  fi
fi
install -m 0755 "$BUILD_PATH" "$BIN_PATH"
printf '%s\n' "$BIN_PATH" > "$BINARY_MANIFEST"

if [[ ! -f "$CONFIG_PATH" ]]; then
  cat > "$CONFIG_PATH" <<'EOF'
# Synaptic Canvas CLI configuration
# User-owned file: installer creates it once and preserves later edits.
EOF
fi

SOURCE_LIST="$TMP_DIR/source-files.txt"
(
  cd "$SKILL_SOURCE"
  find . -type f | sed 's#^\./##' | LC_ALL=C sort
) > "$SOURCE_LIST"

if [[ -f "$SKILL_MANIFEST" ]]; then
  while IFS= read -r rel; do
    [[ -n "$rel" ]] || continue
    if ! grep -Fqx "$rel" "$SOURCE_LIST"; then
      rm -f "$SKILL_TARGET/$rel"
      prune_empty_dirs "$(dirname "$SKILL_TARGET/$rel")" "$SKILL_TARGET"
    fi
  done < "$SKILL_MANIFEST"
fi

while IFS= read -r rel; do
  [[ -n "$rel" ]] || continue
  src="$SKILL_SOURCE/$rel"
  dst="$SKILL_TARGET/$rel"
  mkdir -p "$(dirname "$dst")"
  mode=0644
  if [[ -x "$src" ]]; then
    mode=0755
  fi
  install -m "$mode" "$src" "$dst"
done < "$SOURCE_LIST"
cp "$SOURCE_LIST" "$SKILL_MANIFEST"

case ":${PATH:-}:" in
  *":$BIN_DIR:"*) ;;
  *)
    note "warning: $BIN_DIR is not currently on PATH"
    ;;
esac

note "Installed sc to $BIN_PATH"
note "Installed sc:plugin to $SKILL_TARGET"
note "Preserved config at $CONFIG_PATH"
