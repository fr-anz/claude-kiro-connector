#!/usr/bin/env bash
set -euo pipefail

install_root="${CLAUDE_KIRO_INSTALL_ROOT:-$HOME/.local/share/claude-kiro}"
state_root="${CLAUDE_KIRO_STATE_ROOT:-$HOME/.claude-kiro}"
user_bin_dir="${CLAUDE_KIRO_BIN_DIR:-$HOME/.local/bin}"
purge_state=false

if [[ "${1:-}" == "--purge-state" ]]; then
  purge_state=true
elif (($# > 0)); then
  printf 'Usage: ./scripts/uninstall.sh [--purge-state]\n' >&2
  exit 2
fi

validate_removal_root() {
  local candidate="$1"
  local expected_leaf="$2"
  if [[ -z "$candidate" || "$candidate" == "/" || "$candidate" == "$HOME" || "${candidate##*/}" != "$expected_leaf" ]]; then
    printf 'refusing unsafe removal target: %s\n' "$candidate" >&2
    exit 2
  fi
}

validate_removal_root "$install_root" "claude-kiro"
rm -f "$user_bin_dir/claude-kiro" "$user_bin_dir/ck"
rm -rf "$install_root"

if [[ "$purge_state" == true ]]; then
  validate_removal_root "$state_root" ".claude-kiro"
  rm -rf "$state_root"
else
  printf 'Preserved state: %s\n' "$state_root"
fi

printf 'Removed ClaudeCode Kiro runtime. Official Claude Code was not touched.\n'
