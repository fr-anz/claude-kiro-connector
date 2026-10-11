# Contributing

This repository is a downstream fork of [kirocc](https://github.com/d-kuro/kirocc).
Keep changes easy to audit against the gateway boundary (`cmd`, `internal`) and
the launcher/installer integration (`scripts`, `config`).

## Before opening an issue

- Run `./scripts/doctor.sh` (or `.\scripts\doctor.ps1`) and redact paths or
  values you don't want public.
- Check whether the problem belongs here, upstream kirocc, Kiro CLI, or
  Claude Code.
- Never post credentials, tokens, Kiro databases, private prompts, or session
  logs. Report security issues as described in [SECURITY.md](SECURITY.md).

## Development setup

Requires Go 1.26.1+ and `GOEXPERIMENT=jsonv2` (set by the Makefile). Unit tests
need no Kiro credential.

```bash
git clone https://github.com/fr-anz/claude-kiro-connector.git
cd claude-kiro-connector
make test        # go test -race ./...
make lint        # golangci-lint run
make vet
make build       # dist/claude-kiro
```

If your system Go is newer than the version in `go.mod`, pin the toolchain for
tests: `GOTOOLCHAIN=go1.26.1 make test`.

`make test-e2e` hits the live Kiro API and needs real credentials. Live tests
must stay opt-in, redact credentials, and state what remote behavior they
actually proved.

Script changes should also pass:

```bash
bash -n scripts/install.sh scripts/uninstall.sh scripts/doctor.sh
./scripts/doctor.sh --help >/dev/null
```

PowerShell scripts must parse under Windows PowerShell 5.1 and pass the
`windows-latest` CI job.

## Pull requests

- Keep changes focused; say whether they touch upstream-derived gateway code,
  fork integration code, or docs only.
- Add or update tests for protocol conversion, streaming, retry, auth, launcher,
  or installer behavior.
- Follow existing patterns, prefer explicit code, and delete dead code. Backward
  compatibility is not required unless stated.
- Update `README.md` when changing setup, flags, environment variables, or
  limitations.
- Don't commit credentials, histories, logs, or generated runtime files.
- Never modify or replace the official `claude` command or its configuration.
- Include the commands you ran and their results in the PR description.

By contributing, you agree your contribution is licensed under the repository's
[Apache-2.0 license](LICENSE) and that you have the right to submit it.
