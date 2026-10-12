#!/usr/bin/env bash
set -uo pipefail

install_root="${CLAUDE_KIRO_INSTALL_ROOT:-$HOME/.local/share/claude-kiro}"
state_root="${CLAUDE_KIRO_STATE_ROOT:-$HOME/.claude-kiro}"
user_bin_dir="${CLAUDE_KIRO_BIN_DIR:-$HOME/.local/bin}"
gateway_port="${CLAUDE_KIRO_PORT:-3457}"
gateway_url="${CLAUDE_KIRO_URL:-http://127.0.0.1:${gateway_port}}"
if [[ -n "${CLAUDE_KIRO_URL:-}" ]]; then
  gateway_url_label='<custom CLAUDE_KIRO_URL; value redacted>'
else
  gateway_url_label="$gateway_url"
fi
strict=false

usage() {
  cat <<'USAGE'
Usage: ./scripts/doctor.sh [--strict]

Read-only checks for the ClaudeCode Kiro installation. It never prints tokens,
modifies configuration, starts Claude Code, or starts/stops the gateway.

  --strict  Return non-zero when warnings are present as well as failures.
USAGE
}

while (($# > 0)); do
  case "$1" in
    --strict) strict=true ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown option: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

pass_count=0
warn_count=0
fail_count=0

pass() {
  pass_count=$((pass_count + 1))
  printf '[PASS] %s\n' "$1"
}

warn() {
  warn_count=$((warn_count + 1))
  printf '[WARN] %s\n' "$1"
}

fail() {
  fail_count=$((fail_count + 1))
  printf '[FAIL] %s\n' "$1"
}

check_command() {
  if command -v "$1" >/dev/null 2>&1; then
    pass "command available: $1"
  else
    fail "missing command: $1"
  fi
}

check_executable() {
  if [[ -x "$1" ]]; then
    pass "executable: $1"
  else
    fail "missing executable: $1"
  fi
}

check_file() {
  if [[ -f "$1" ]]; then
    pass "file: $1"
  else
    fail "missing file: $1"
  fi
}

printf 'ClaudeCode Kiro doctor (read-only)\n'
printf '  install root: %s\n' "$install_root"
printf '  state root:   %s\n' "$state_root"
printf '  gateway URL:  %s\n\n' "$gateway_url_label"

for required_command in go curl node; do
  check_command "$required_command"
done

official_claude="$(command -v claude 2>/dev/null || true)"
isolated_launcher="$(command -v claude-kiro 2>/dev/null || true)"
if [[ -n "$official_claude" ]]; then
  pass "official Claude command: $official_claude"
else
  fail 'official Claude command not found'
fi
if [[ -n "$isolated_launcher" ]]; then
  pass "isolated launcher: $isolated_launcher"
else
  fail "claude-kiro not found on PATH (expected under $user_bin_dir)"
fi
if command -v ck >/dev/null 2>&1; then
  pass "short alias: $(command -v ck)"
else
  fail "ck alias not found on PATH (expected under $user_bin_dir)"
fi
if [[ -n "$official_claude" && -n "$isolated_launcher" ]]; then
  if [[ "$official_claude" == "$isolated_launcher" ]]; then
    fail 'official claude and claude-kiro resolve to the same path'
  else
    pass 'official claude and claude-kiro paths are separate'
  fi
  case "$official_claude" in
    "$install_root"/*|"$state_root"/*) fail 'official claude resolves inside the isolated roots' ;;
    *) pass 'official claude resolves outside the isolated roots' ;;
  esac
fi

check_executable "$install_root/bin/claude-kiro-native-websearch"
check_file "$state_root/claude-config/settings.json"
check_file "$state_root/claude-config/CLAUDE.md"

if [[ -f "$state_root/claude-config/settings.json" ]]; then
  if node -e 'JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"))' \
      "$state_root/claude-config/settings.json" >/dev/null 2>&1; then
    pass 'isolated settings.json is valid JSON'
  else
    fail 'isolated settings.json is invalid JSON'
  fi
fi

if [[ -n "${KIRO_API_KEY:-}" ]]; then
  pass 'Kiro API-key mode selected (value redacted)'
  if [[ -n "${KIRO_API_REGION:-}" ]]; then
    pass 'Kiro API region is set'
  else
    warn 'KIRO_API_REGION is unset; gateway will default to us-east-1'
  fi
else
  case "$(uname -s)" in
    Darwin) default_db="$HOME/Library/Application Support/kiro-cli/data.sqlite3" ;;
    Linux) default_db="$HOME/.local/share/kiro-cli/data.sqlite3" ;;
    *) default_db="" ;;
  esac
  kiro_db="${CLAUDE_KIRO_DB_PATH:-$default_db}"
  if [[ -n "$kiro_db" && -f "$kiro_db" ]]; then
    pass "Kiro CLI database found: $kiro_db"
    if command -v kiro-cli >/dev/null 2>&1; then
      pass 'Kiro CLI command is available for login maintenance'
      if kiro-cli whoami </dev/null >/dev/null 2>&1; then
        pass 'kiro-cli whoami confirms a logged-in profile (output redacted)'
      else
        fail 'kiro-cli whoami failed; run kiro-cli login, then kiro-cli whoami'
      fi
    else
      warn 'Kiro CLI command is missing; the gateway can use the existing database, but login repair requires Kiro CLI'
    fi
  elif [[ -n "$kiro_db" ]]; then
    fail "Kiro CLI database not found: $kiro_db"
    if command -v kiro-cli >/dev/null 2>&1; then
      warn 'run kiro-cli login, then kiro-cli whoami to create a usable login database'
    else
      warn 'install Kiro CLI and log in, or set KIRO_API_KEY and KIRO_API_REGION'
    fi
  else
    warn 'no default Kiro CLI database path for this OS; use CLAUDE_KIRO_DB_PATH or KIRO_API_KEY'
  fi
fi

case "$gateway_port" in
  ''|*[!0-9]*) fail 'CLAUDE_KIRO_PORT must be numeric' ;;
  *)
    gateway_port_number=$((10#$gateway_port))
    if ((gateway_port_number < 1 || gateway_port_number > 65535)); then
      fail 'CLAUDE_KIRO_PORT must be between 1 and 65535'
    else
      pass "gateway port is valid: $gateway_port"
    fi
    ;;
esac

if curl -fsS --max-time 1 "${gateway_url%/}/health" >/dev/null 2>&1; then
  pass 'gateway health endpoint is reachable'
else
  warn 'gateway is not currently reachable; this is normal when claude-kiro is closed'
fi

printf '\nSummary: %d pass, %d warning, %d failure\n' \
  "$pass_count" "$warn_count" "$fail_count"

if ((fail_count > 0)); then
  exit 1
fi
if [[ "$strict" == true && "$warn_count" -gt 0 ]]; then
  exit 2
fi
