# claude-kiro-connector

[![CI](https://github.com/fr-anz/claude-kiro-connector/actions/workflows/ci.yml/badge.svg)](https://github.com/fr-anz/claude-kiro-connector/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Run Claude Code against Kiro credentials through a local gateway, without
touching your official `claude` installation.

This repository contains two things:

- **`claude-kiro` gateway** — a local HTTP gateway that translates the Anthropic Messages API
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
- PDF and text `document` blocks (including PDFs returned by the Read tool),
  converted to text locally because Kiro has no document input; scanned
  image-only PDFs have no extractable text
- `tool_choice` (`any`, `tool`, `none`) emulated with a system-prompt
  directive; Kiro does not enforce it
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
Refreshed OAuth tokens are cached separately in `~/.claude-kiro/auth-cache.json`
(the managed state root when overridden), so refresh-token rotation survives
gateway restarts. The Kiro database stays read-only. Treat this cache as a secret;
API keys are never cached. Use `-auth-cache ""` to disable persistence.

## Install the managed launcher

macOS/Linux:

```bash
git clone https://github.com/fr-anz/claude-kiro-connector.git
cd claude-kiro-connector
./scripts/install.sh
claude-kiro        # or: ck
```

Windows PowerShell:

```powershell
git clone https://github.com/fr-anz/claude-kiro-connector.git
Set-Location claude-kiro-connector
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
| Gateway binary | `~/.local/share/claude-kiro/bin/claude-kiro-native-websearch` | `%LOCALAPPDATA%\claude-kiro\bin\claude-kiro-native-websearch.exe` |
| Isolated Claude config | `~/.claude-kiro/claude-config/` | `%USERPROFILE%\.claude-kiro\claude-config\` |

The launcher reuses a healthy gateway at `CLAUDE_KIRO_URL` (default
`http://127.0.0.1:3457`); otherwise it starts one for the lifetime of the
session. It sets `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, and
`ANTHROPIC_MODEL` (default `claude-opus-5-5` when unset),
`ANTHROPIC_DEFAULT_HAIKU_MODEL` (background tasks; default `claude-sonnet-5-5`),
and `DISABLE_AUTOUPDATER=1` so the isolated profile never updates the official
runtime. It strips proxy
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
| `CLAUDE_KIRO_PORT` | Managed gateway port (default `3457`) |
| `CLAUDE_KIRO_DB_PATH` | Kiro CLI login database path |

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
./dist/claude-kiro                  # listens on 127.0.0.1:3456
ANTHROPIC_BASE_URL=http://127.0.0.1:3456 ANTHROPIC_AUTH_TOKEN=dummy claude
```

Every flag has an environment equivalent; run `claude-kiro -h` for the full list.

| Flag | Environment | Default |
| --- | --- | --- |
| `-host` / `-port` | `CLAUDE_KIRO_HOST` / `CLAUDE_KIRO_PORT` | `127.0.0.1` / `3456` |
| `-db` | `CLAUDE_KIRO_DB_PATH` | Kiro CLI default path |
| `-auth-cache` | `CLAUDE_KIRO_AUTH_CACHE` | `~/.claude-kiro/auth-cache.json` |
| `-api-key` | `CLAUDE_KIRO_API_KEY` | none (local gateway auth, `Authorization: Bearer`) |
| `-kiro-api-key` | `KIRO_API_KEY` | none (use DB credential) |
| `-kiro-api-region` | `KIRO_API_REGION` | credential region |
| `-debug` | `CLAUDE_KIRO_DEBUG` | `false` |
| `-otel`, `-otel-body-limit` | `CLAUDE_KIRO_OTEL`, `CLAUDE_KIRO_OTEL_BODY_LIMIT` | off |
| `-keepalive-interval` | `CLAUDE_KIRO_KEEPALIVE_INTERVAL` | `15s` (`0` disables) |
| `-log-file` and `-log-*` | `CLAUDE_KIRO_LOG_*` | console only |

Model names are mapped to Kiro IDs in
[`internal/models/models.go`](internal/models/models.go). Override them with
`CLAUDE_KIRO_MODEL_MAPPINGS`, a JSON array of
`{"anthropic": "...", "kiro": "..."}` objects.

## Security notes

- The gateway binds to loopback and has no authentication by default. Set
  `-api-key` before binding to any other address; the server warns if you don't.
- Only `Authorization: Bearer` is accepted for the gateway key, so use
  `ANTHROPIC_AUTH_TOKEN`, not `ANTHROPIC_API_KEY`.
- Never commit Kiro databases, tokens, Claude session history, or logs. See
  [SECURITY.md](SECURITY.md) for the local data boundary and reporting.

## Known limitations

Messages require a nonempty `model`, a positive `max_tokens`, and `user` or
`assistant` message roles. `count_tokens` does not require an output budget.

- Kiro throttling and unavailability are returned as `429 rate_limit_error`
  and `529 overloaded_error`, and context overflow as Anthropic's
  `prompt is too long` error so Claude Code retries and auto-compacts. Other
  upstream failures are returned as HTTP 502.
- `count_tokens` and usage without Kiro metadata are local tokenizer
  estimates, not Kiro's count.
- URL-sourced images and documents are not fetched by the gateway.
- Model availability, quota, and billing are enforced by Kiro; this gateway does
  not change them.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Apache License 2.0](LICENSE). Based on
[kirocc](https://github.com/d-kuro/kirocc) by d-kuro, also Apache-2.0; modified
files carry changes by the contributors of this repository.
