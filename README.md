# claudecode-kiro

[![CI](https://github.com/fr-anz/claudecode-kiro/actions/workflows/ci.yml/badge.svg)](https://github.com/fr-anz/claudecode-kiro/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Run Claude Code against Kiro credentials through a local gateway, without
touching your official `claude` installation.

This repository contains two things:

- **`kirocc`** — a local HTTP gateway that translates the Anthropic Messages API
  to the Kiro backend (AWS Event Stream) and back. It is based on
  [d-kuro/kirocc](https://github.com/d-kuro/kirocc) and adds Kiro-native
  WebSearch, image-input fixes, and installer tooling.
- **`claude-kiro`** — an isolated launcher (alias `ck`) that starts the gateway
  and runs the official Claude Code runtime with its own configuration
  directory, pointed at the gateway.

> Not affiliated with Anthropic, Amazon, Kiro, or d-kuro. This repository
> contains no Claude Code binaries or source, credentials, or private prompts.

## Features

- `/v1/messages` (streaming and non-streaming), `/v1/messages/count_tokens`
  (approximate), `/v1/models`, and `/health`
- Claude Code's built-in WebSearch (`web_search_20250305`) served through Kiro's
  regional MCP endpoint (`https://q.<region>.amazonaws.com/mcp`)
- Extended thinking, native reasoning effort, and Tool Search emulation
- Base64 image input across conversation history and tool results (request
  bodies up to 32 MiB)
- Kiro CLI SQLite credentials with automatic Social/OIDC refresh, or a static
  Kiro API key (`ksk_...`)
- Retries for upstream throttling, 5xx, and transient stream errors, invisible
  to the client until output starts
- Optional OpenTelemetry tracing and rotating file logs

## Requirements

- macOS, Linux, or Windows 11 x64
- Go 1.26.1+ (built with `GOEXPERIMENT=jsonv2`; the installer sets it)
- Node.js 18+ and an installed official Claude Code (`claude` on `PATH`)
- One Kiro credential source:
  - a Kiro CLI login (`kiro-cli login`), or
  - `KIRO_API_KEY` plus optional `KIRO_API_REGION` (default `us-east-1`)
- `~/.local/bin` on `PATH` (the Windows installer adds it)

Kiro CLI is not a traffic proxy. It only creates the login database; the gateway
calls Kiro directly and refreshes tokens itself.

## Install the managed launcher

macOS/Linux:

```bash
git clone https://github.com/fr-anz/claudecode-kiro.git
cd claudecode-kiro
./scripts/install.sh
claude-kiro        # or: ck
```

Windows PowerShell:

```powershell
git clone https://github.com/fr-anz/claudecode-kiro.git
Set-Location claudecode-kiro
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\install.ps1
claude-kiro        # or: ck
```

The installer refuses to continue without a usable credential source. To use an
API key, export it before installing and before each launch; it is never written
to disk by this project:

```bash
export KIRO_API_KEY=ksk_...
export KIRO_API_REGION=us-east-1
```

### What gets installed

| Purpose | macOS/Linux | Windows |
| --- | --- | --- |
| Launchers | `~/.local/bin/claude-kiro`, `ck` | `%USERPROFILE%\.local\bin\claude-kiro.cmd`, `ck.cmd`, `ck.ps1` |
| Gateway binary | `~/.local/share/claude-kiro/bin/kirocc-native-websearch` | `%LOCALAPPDATA%\claude-kiro\bin\kirocc-native-websearch.exe` |
| Isolated Claude config | `~/.claude-kiro/claude-config/` | `%USERPROFILE%\.claude-kiro\claude-config\` |

The launcher reuses a healthy gateway at `KIROCC_URL` (default
`http://127.0.0.1:3457`); otherwise it starts one for the lifetime of the
session. It sets `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, and
`ANTHROPIC_MODEL` (default `claude-opus-5-5` when unset), and strips proxy
variables from the Claude process unless `CLAUDE_KIRO_PRESERVE_PROXY=1`.
The profile ships [`config/CLAUDE.md`](config/CLAUDE.md) and
[`config/settings.json`](config/settings.json) as defaults.

### Installer overrides

| Variable | Purpose |
| --- | --- |
| `CLAUDE_KIRO_INSTALL_ROOT` | Gateway installation root |
| `CLAUDE_KIRO_STATE_ROOT` | Isolated state root (default `~/.claude-kiro`) |
| `CLAUDE_KIRO_BIN_DIR` | Launcher directory (default `~/.local/bin`) |
| `CLAUDE_KIRO_RUNTIME_BIN` | Claude runtime used by the launcher |
| `CLAUDE_KIRO_CONFIG_DIR` | Claude config directory used by the launcher |
| `KIROCC_PORT` | Managed gateway port (default `3457`) |
| `KIROCC_DB_PATH` | Kiro CLI login database path |

### Verify, update, uninstall

```bash
./scripts/doctor.sh            # read-only checks; --strict fails on warnings
git pull --ff-only && ./scripts/install.sh
./scripts/uninstall.sh         # keeps isolated state
./scripts/uninstall.sh --purge-state
```

Windows equivalents: `.\scripts\doctor.ps1`, `.\scripts\install.ps1`,
`.\scripts\uninstall.ps1 [-PurgeState]`.

## Run the gateway standalone

```bash
make build
./dist/kirocc                       # listens on 127.0.0.1:3456
ANTHROPIC_BASE_URL=http://127.0.0.1:3456 ANTHROPIC_AUTH_TOKEN=dummy claude
```

Every flag has an environment equivalent; run `kirocc -h` for the full list.

| Flag | Environment | Default |
| --- | --- | --- |
| `-host` / `-port` | `KIROCC_HOST` / `KIROCC_PORT` | `127.0.0.1` / `3456` |
| `-db` | `KIROCC_DB_PATH` | Kiro CLI default path |
| `-api-key` | `KIROCC_API_KEY` | none (local gateway auth, `Authorization: Bearer`) |
| `-kiro-api-key` | `KIRO_API_KEY` | none (use DB credential) |
| `-kiro-api-region` | `KIRO_API_REGION` | credential region |
| `-debug` | `KIROCC_DEBUG` | `false` |
| `-otel`, `-otel-body-limit` | `KIROCC_OTEL`, `KIROCC_OTEL_BODY_LIMIT` | off |
| `-keepalive-interval` | `KIROCC_KEEPALIVE_INTERVAL` | `15s` (`0` disables) |
| `-log-file` and `-log-*` | `KIROCC_LOG_*` | console only |

Model names are mapped to Kiro IDs in
[`internal/models/models.go`](internal/models/models.go). Override them with
`KIROCC_MODEL_MAPPINGS`, a JSON array of
`{"anthropic": "...", "kiro": "..."}` objects.

## Security notes

- The gateway binds to loopback and has no authentication by default. Set
  `-api-key` before binding to any other address; the server warns if you don't.
- Only `Authorization: Bearer` is accepted for the gateway key, so use
  `ANTHROPIC_AUTH_TOKEN`, not `ANTHROPIC_API_KEY`.
- Never commit Kiro databases, tokens, Claude session history, or logs. See
  [SECURITY.md](SECURITY.md) for the local data boundary and reporting.

## Known limitations

- Upstream errors are currently all returned as HTTP 502, so client-side
  rate-limit and prompt-too-long handling does not trigger.
- `count_tokens` is a local approximation, not Kiro's count.
- Model availability, quota, and billing are enforced by Kiro; this gateway does
  not change them.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Apache License 2.0](LICENSE). Based on
[kirocc](https://github.com/d-kuro/kirocc) by d-kuro, also Apache-2.0; modified
files carry changes by the contributors of this repository.
