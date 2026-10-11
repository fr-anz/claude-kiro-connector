#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
install_root="${CLAUDE_KIRO_INSTALL_ROOT:-$HOME/.local/share/claude-kiro}"
state_root="${CLAUDE_KIRO_STATE_ROOT:-$HOME/.claude-kiro}"
user_bin_dir="${CLAUDE_KIRO_BIN_DIR:-$HOME/.local/bin}"
gateway_port="${KIROCC_PORT:-3457}"

usage() {
  cat <<'USAGE'
Usage: ./scripts/install.sh

Environment overrides:
  CLAUDE_KIRO_INSTALL_ROOT       Gateway installation root.
  CLAUDE_KIRO_STATE_ROOT         Isolated Claude Code state/config root.
  CLAUDE_KIRO_BIN_DIR            Directory for the claude-kiro launcher.
  KIROCC_PORT                    Gateway port (default: 3457).
  KIROCC_DB_PATH                 Existing Kiro CLI login database.
  KIRO_API_KEY                   Kiro upstream API key; makes Kiro CLI optional.
  KIRO_API_REGION                Kiro API-key region (default: us-east-1).
USAGE
}

while (($# > 0)); do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown option: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

case "$gateway_port" in
  ''|*[!0-9]*) printf 'KIROCC_PORT must be numeric\n' >&2; exit 2 ;;
esac
gateway_port_number=$((10#$gateway_port))
if ((gateway_port_number < 1 || gateway_port_number > 65535)); then
  printf 'KIROCC_PORT must be between 1 and 65535\n' >&2
  exit 2
fi

for required_command in go node; do
  if ! command -v "$required_command" >/dev/null 2>&1; then
    printf 'missing prerequisite: %s\n' "$required_command" >&2
    exit 127
  fi
done

official_claude_command="$(command -v claude 2>/dev/null || true)"
if [[ -z "$official_claude_command" ]]; then
  printf 'official Claude Code command not found\n' >&2
  exit 127
fi
official_claude_real="$(node -e 'console.log(require("fs").realpathSync(process.argv[1]))' "$official_claude_command")"
case "$(uname -s)" in
  Darwin) default_kiro_db="$HOME/Library/Application Support/kiro-cli/data.sqlite3" ;;
  Linux) default_kiro_db="$HOME/.local/share/kiro-cli/data.sqlite3" ;;
  *) default_kiro_db="" ;;
esac
kiro_db_path="${KIROCC_DB_PATH:-$default_kiro_db}"
if [[ -n "${KIRO_API_KEY:-}" ]]; then
  printf 'Kiro authentication: API-key mode (Kiro CLI is not required).\n'
elif [[ -n "$kiro_db_path" && -f "$kiro_db_path" ]]; then
  if command -v kiro-cli >/dev/null 2>&1; then
    if kiro-cli whoami </dev/null >/dev/null 2>&1; then
      printf 'Kiro authentication: existing login confirmed (whoami output redacted).\n'
    else
      cat >&2 <<EOF
Kiro CLI database exists, but kiro-cli whoami did not confirm a logged-in profile.
Run:

  kiro-cli login
  kiro-cli whoami

Then rerun this installer. Request traffic will still be sent directly by the
kirocc gateway; this check only validates the credential it will read.
EOF
      exit 78
    fi
  else
    printf 'Kiro authentication: existing login database found (Kiro CLI command unavailable; live status not checked).\n'
  fi
else
  if command -v kiro-cli >/dev/null 2>&1; then
    kiro_cli_state='Kiro CLI is installed, but its login database was not found.'
  else
    kiro_cli_state='Kiro CLI is not installed and no existing login database was found.'
  fi
  cat >&2 <<EOF
No usable Kiro credential source found.
$kiro_cli_state

Kiro CLI is used only to create the local login credential; request traffic is
sent directly by the kirocc gateway. Install/login with:

  curl -fsSL https://cli.kiro.dev/install | bash
  kiro-cli login
  kiro-cli whoami

Or set KIRO_API_KEY (and optionally KIRO_API_REGION) before running this
installer and whenever claude-kiro is launched.
Expected database: ${kiro_db_path:-<no default for this OS>}
EOF
  exit 78
fi

official_claude_sha256_before="$(node -e '
const fs = require("fs");
const crypto = require("crypto");
const hash = crypto.createHash("sha256");
hash.update(fs.readFileSync(process.argv[1]));
console.log(hash.digest("hex"));
' "$official_claude_real")"

mkdir -p "$install_root/bin" "$state_root/claude-config" "$user_bin_dir"

temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/claude-kiro.XXXXXX")"
cleanup_install() {
  rm -rf "$temporary_root"
}
trap cleanup_install EXIT INT TERM HUP

printf 'Building patched kirocc...\n'
GOEXPERIMENT=jsonv2 go build -trimpath -o "$temporary_root/kirocc" "$project_root/cmd/kirocc"
install -m 0755 "$temporary_root/kirocc" "$install_root/bin/kirocc-native-websearch"

if [[ ! -f "$state_root/claude-config/CLAUDE.md" ]]; then
  install -m 0644 "$project_root/config/CLAUDE.md" "$state_root/claude-config/CLAUDE.md"
fi
if [[ ! -f "$state_root/claude-config/settings.json" ]]; then
  install -m 0644 "$project_root/config/settings.json" "$state_root/claude-config/settings.json"
fi

launcher_path="$user_bin_dir/claude-kiro"
cat > "$launcher_path" <<LAUNCHER
#!/usr/bin/env bash
set -euo pipefail

kirocc_bin="\${KIROCC_BIN:-$install_root/bin/kirocc-native-websearch}"
runtime_bin="\${CLAUDE_KIRO_RUNTIME_BIN:-$official_claude_real}"
config_dir="\${CLAUDE_KIRO_CONFIG_DIR:-$state_root/claude-config}"
gateway_port="\${KIROCC_PORT:-$gateway_port}"
gateway_url="\${KIROCC_URL:-http://127.0.0.1:\${gateway_port}}"
proxy_token="\${KIROCC_API_KEY:-dummy}"
export KIRO_API_REGION="\${KIRO_API_REGION:-us-east-1}"
log_file="\${TMPDIR:-/tmp}/claude-kiro-gateway-\${UID}-\${gateway_port}.log"
started_pid=""

cleanup() {
  if [[ -n "\$started_pid" ]] && kill -0 "\$started_pid" 2>/dev/null; then
    kill "\$started_pid" 2>/dev/null || true
    wait "\$started_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM HUP

if [[ "\${1:-}" == "update" ]]; then
  echo "claude-kiro: updates are disabled in the isolated profile; rerun the project installer" >&2
  exit 2
fi

gateway_accepts_token() {
  curl -fsS --noproxy '*' --max-time 2 \
    -H "Authorization: Bearer \$proxy_token" \
    "\$gateway_url/v1/models" >/dev/null 2>&1
}

prepare_local_kiro_credentials() {
  if [[ -n "\${KIRO_API_KEY:-}" ]]; then
    return 0
  fi

  local database_path="\${KIROCC_DB_PATH:-}"
  if [[ -z "\$database_path" ]]; then
    case "\$(uname -s)" in
      Darwin) database_path="\$HOME/Library/Application Support/kiro-cli/data.sqlite3" ;;
      Linux) database_path="\$HOME/.local/share/kiro-cli/data.sqlite3" ;;
    esac
  fi
  if [[ -n "\$database_path" && -f "\$database_path" ]]; then
    export KIROCC_DB_PATH="\$database_path"
    return 0
  fi

  cat >&2 <<EOF
claude-kiro: no usable Kiro credential source was found.
Kiro CLI is only needed to create the login credential; traffic is sent by the
kirocc gateway itself.

Install/login Kiro CLI:
  curl -fsSL https://cli.kiro.dev/install | bash
  kiro-cli login
  kiro-cli whoami

Or export KIRO_API_KEY (and optionally KIRO_API_REGION) before launching.
Managed default: KIRO_API_REGION=us-east-1 (eu-central-1 is also supported).
Expected database: \${database_path:-<no default for this OS>}
EOF
  exit 78
}

gateway_is_healthy=false
if curl -fsS --noproxy '*' --max-time 1 "\$gateway_url/health" >/dev/null 2>&1; then
  gateway_is_healthy=true
  if ! gateway_accepts_token; then
    echo "claude-kiro: a gateway is already running at \$gateway_url but rejects the current KIROCC_API_KEY" >&2
    echo "Use the matching local proxy key or choose an unused port, for example: KIROCC_PORT=3458 claude-kiro" >&2
    exit 1
  fi
fi

if [[ "\$gateway_is_healthy" != true ]]; then
  prepare_local_kiro_credentials
  "\$kirocc_bin" -port "\$gateway_port" >"\$log_file" 2>&1 &
  started_pid=\$!
  ready=false
  for _attempt in {1..50}; do
    if curl -fsS --noproxy '*' --max-time 1 "\$gateway_url/health" >/dev/null 2>&1; then
      ready=true
      break
    fi
    if ! kill -0 "\$started_pid" 2>/dev/null; then break; fi
    sleep 0.1
  done
  if [[ "\$ready" != true ]]; then
    echo "claude-kiro: gateway failed to start; log follows" >&2
    tail -n 40 "\$log_file" >&2 2>/dev/null || true
    exit 1
  fi
  if ! gateway_accepts_token; then
    echo "claude-kiro: gateway started but rejected the configured local proxy key" >&2
    exit 1
  fi
fi

runtime_proxy_args=()
if [[ "\${CLAUDE_KIRO_PRESERVE_PROXY:-0}" != 1 ]]; then
  case "\$gateway_url" in
    http://127.0.0.1:*|https://127.0.0.1:*|http://localhost:*|https://localhost:*|http://\[::1\]:*|https://\[::1\]:*)
      # The gateway already inherited proxy variables for its Kiro requests.
      # Remove them only from Claude Code so its loopback API calls cannot be
      # sent through a proxy that does not honor NO_PROXY.
      runtime_proxy_args=(
        -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY
        -u http_proxy -u https_proxy -u all_proxy
      )
      ;;
  esac
fi

/usr/bin/env \
  "\${runtime_proxy_args[@]}" \
  -u ANTHROPIC_API_KEY \
  -u CLAUDE_CODE_OAUTH_TOKEN \
  -u CLAUDE_CODE_USE_BEDROCK \
  -u CLAUDE_CODE_USE_VERTEX \
  -u CLAUDE_CODE_USE_FOUNDRY \
  CLAUDE_CONFIG_DIR="\$config_dir" \
  ANTHROPIC_BASE_URL="\$gateway_url" \
  ANTHROPIC_AUTH_TOKEN="\$proxy_token" \
  ANTHROPIC_MODEL="\${ANTHROPIC_MODEL:-claude-opus-5-5}" \
  "\$runtime_bin" "\$@"
LAUNCHER
chmod 0755 "$launcher_path"

# Short alias: ck forwards to the claude-kiro launcher next to it.
ck_path="$user_bin_dir/ck"
cat > "$ck_path" <<'CK'
#!/usr/bin/env bash
# ck is a short alias for claude-kiro.
set -euo pipefail
ck_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
if [[ -x "$ck_dir/claude-kiro" ]]; then
  exec "$ck_dir/claude-kiro" "$@"
else
  exec claude-kiro "$@"
fi
CK
chmod 0755 "$ck_path"

official_claude_sha256_after="$(node -e '
const fs = require("fs");
const crypto = require("crypto");
const hash = crypto.createHash("sha256");
hash.update(fs.readFileSync(process.argv[1]));
console.log(hash.digest("hex"));
' "$official_claude_real")"
if [[ "$official_claude_sha256_after" != "$official_claude_sha256_before" ]]; then
  printf 'official Claude Code changed during installation; refusing to report success\n' >&2
  exit 1
fi

printf '\nInstalled successfully:\n'
printf '  launcher: %s\n' "$launcher_path"
printf '  gateway:  %s\n' "$install_root/bin/kirocc-native-websearch"
printf '  runtime:  %s\n' "$official_claude_real"
printf '  config:    %s\n' "$state_root/claude-config"
printf '  command:   claude-kiro\n\n'
printf 'The official claude command was not modified by this installer.\n'
