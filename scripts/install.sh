#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

note() {
  printf '%s\n' "$*"
}

ensure_current_path() {
  case ":${PATH:-}:" in
    *":$BIN_DIR:"*) ;;
    *)
      export PATH="$BIN_DIR:${PATH:-}"
      ;;
  esac
}

path_rc_file() {
  case "$(basename "${SHELL:-bash}")" in
    zsh) printf '%s\n' "$HOME_DIR/.zshrc" ;;
    *) printf '%s\n' "$HOME_DIR/.bashrc" ;;
  esac
}

ensure_shell_rc_path() {
  local rc_file path_line
  rc_file="$(path_rc_file)"
  path_line="export PATH=\"$BIN_DIR:\$PATH\""
  mkdir -p "$(dirname "$rc_file")"
  touch "$rc_file"
  if ! grep -Fqx "$path_line" "$rc_file"; then
    printf '%s\n' "$path_line" >> "$rc_file"
  fi
}

maybe_fail_skill_copy() {
  local threshold="${SC_INSTALL_TEST_FAIL_AFTER_SKILL_COPY:-0}"
  if [[ ! "$threshold" =~ ^[0-9]+$ ]] || (( threshold <= 0 )); then
    return 0
  fi
  SC_INSTALL_SKILL_COPY_COUNT="${SC_INSTALL_SKILL_COPY_COUNT:-0}"
  SC_INSTALL_SKILL_COPY_COUNT=$((SC_INSTALL_SKILL_COPY_COUNT + 1))
  export SC_INSTALL_SKILL_COPY_COUNT
  if (( SC_INSTALL_SKILL_COPY_COUNT >= threshold )); then
    die "simulated skill copy failure after $SC_INSTALL_SKILL_COPY_COUNT managed file copies"
  fi
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
SKILL_PARENT="$(dirname "$SKILL_TARGET")"
SKILL_MANIFEST="$STATE_ROOT/managed-files.txt"
BINARY_MANIFEST="$STATE_ROOT/binary-path.txt"

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sc-install.XXXXXX")"
STAGED_SKILL=""
BACKUP_SKILL=""
cleanup() {
  if [[ -n "$BACKUP_SKILL" && -d "$BACKUP_SKILL" && ! -e "$SKILL_TARGET" ]]; then
    mv "$BACKUP_SKILL" "$SKILL_TARGET" 2>/dev/null || true
  fi
  [[ -n "$STAGED_SKILL" ]] && rm -rf "$STAGED_SKILL" 2>/dev/null || true
  [[ -n "$BACKUP_SKILL" ]] && rm -rf "$BACKUP_SKILL" 2>/dev/null || true
  rm -rf "$TMP_DIR" 2>/dev/null || true
}
trap cleanup EXIT

mkdir -p "$BIN_DIR" "$CONFIG_DIR" "$STATE_ROOT" "$SKILL_PARENT"

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

STAGED_SKILL="$(mktemp -d "$SKILL_PARENT/.sc-plugin.stage.XXXXXX")"
if [[ -d "$SKILL_TARGET" ]]; then
  cp -a "$SKILL_TARGET/." "$STAGED_SKILL/"
fi

if [[ -f "$SKILL_MANIFEST" ]]; then
  while IFS= read -r rel; do
    [[ -n "$rel" ]] || continue
    if ! grep -Fqx "$rel" "$SOURCE_LIST"; then
      rm -f "$STAGED_SKILL/$rel"
      prune_empty_dirs "$(dirname "$STAGED_SKILL/$rel")" "$STAGED_SKILL"
    fi
  done < "$SKILL_MANIFEST"
fi

while IFS= read -r rel; do
  [[ -n "$rel" ]] || continue
  src="$SKILL_SOURCE/$rel"
  dst="$STAGED_SKILL/$rel"
  mkdir -p "$(dirname "$dst")"
  mode=0644
  if [[ -x "$src" ]]; then
    mode=0755
  fi
  install -m "$mode" "$src" "$dst"
  maybe_fail_skill_copy
done < "$SOURCE_LIST"

if [[ -d "$SKILL_TARGET" ]]; then
  BACKUP_SKILL="$SKILL_PARENT/.sc-plugin.backup.$$"
  mv "$SKILL_TARGET" "$BACKUP_SKILL"
fi
if mv "$STAGED_SKILL" "$SKILL_TARGET"; then
  STAGED_SKILL=""
  [[ -n "$BACKUP_SKILL" ]] && rm -rf "$BACKUP_SKILL"
  BACKUP_SKILL=""
else
  if [[ -n "$BACKUP_SKILL" && -d "$BACKUP_SKILL" && ! -e "$SKILL_TARGET" ]]; then
    mv "$BACKUP_SKILL" "$SKILL_TARGET"
    BACKUP_SKILL=""
  fi
  die "failed to move staged skill payload into place"
fi
cp "$SOURCE_LIST" "$SKILL_MANIFEST"
ensure_current_path
ensure_shell_rc_path

note "Installed sc to $BIN_PATH"
note "Installed sc:plugin to $SKILL_TARGET"
note "Preserved config at $CONFIG_PATH"
