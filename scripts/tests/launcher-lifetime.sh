#!/usr/bin/env bash
set -euo pipefail
gateway_source="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
smoke_root="$(mktemp -d)"
cleanup() {
  if [[ -f "$smoke_root/state/gateway-$CLAUDE_KIRO_PORT.pid" ]]; then
    kill "$(cat "$smoke_root/state/gateway-$CLAUDE_KIRO_PORT.pid")" 2>/dev/null || true
  fi
  rm -rf "$smoke_root"
}
trap cleanup EXIT
mkdir -p "$smoke_root/fake-bin"
cat > "$smoke_root/fake-bin/go" <<'GO'
#!/usr/bin/env bash
while (($#)); do
  if [[ "$1" == -o ]]; then cp "$SMOKE_GATEWAY" "$2"; exit 0; fi
  shift
done
exit 1
GO
printf '#!/usr/bin/env bash\necho lifetime-runtime-ok\n' > "$smoke_root/fake-bin/claude"
chmod +x "$smoke_root/fake-bin/"*
export SMOKE_GATEWAY="$gateway_source"
export PATH="$smoke_root/fake-bin:$PATH"
export CLAUDE_KIRO_INSTALL_ROOT="$smoke_root/install"
export CLAUDE_KIRO_STATE_ROOT="$smoke_root/state"
export CLAUDE_KIRO_BIN_DIR="$smoke_root/user-bin"
export CLAUDE_KIRO_PORT="${CLAUDE_KIRO_TEST_PORT:-45679}"
export CLAUDE_KIRO_URL="http://127.0.0.1:$CLAUDE_KIRO_PORT"
export CLAUDE_KIRO_API_KEY=local_lifetime_placeholder
export KIRO_API_KEY=ksk_lifetime_placeholder
if curl --noproxy '*' -fsS --max-time 1 "$CLAUDE_KIRO_URL/health" >/dev/null 2>&1; then
  echo 'test port is already occupied' >&2; exit 1
fi
bash "$project_root/scripts/install.sh"
"$smoke_root/user-bin/claude-kiro" --version
curl --noproxy '*' -fsS --max-time 2 "$CLAUDE_KIRO_URL/health"
"$smoke_root/user-bin/claude-kiro" --version
curl --noproxy '*' -fsS --max-time 2 "$CLAUDE_KIRO_URL/health"
echo 'gateway survives launcher exit and is reused'
