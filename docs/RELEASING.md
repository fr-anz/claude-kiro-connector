# Release process

This is the `fr-anz/claudecode-kiro` fork. Upstream `d-kuro/kirocc`
release history through `v0.6.0` has been pruned from `docs/release-notes/`;
only fork prerelease notes (`v0.6.0-clawgod.*`) are kept. Never republish
inherited tags to `origin`.

Creating a tag or GitHub Release is a maintainer decision and is intentionally
separate from preparing code and release notes.

## Artifact boundary

GoReleaser publishes standalone `claude-kiro` gateway binaries for macOS, Linux,
and Windows on amd64 and arm64. Windows artifacts use ZIP; macOS/Linux use
tar.gz. Those archives do not contain Claude Code, credentials, or the
complete-profile installer output.

Users who want `claude-kiro` must clone the source and run
`./scripts/install.sh` (or `scripts\install.ps1` on Windows).

## Tag scheme

Use plain semantic versions:

```text
v<major>.<minor>.<patch>
```

The next release must sort after every inherited tag, so start at `v0.7.0`.
The release workflow runs on any `v*` tag.

## Prepare a release

1. Update and verify `main`:

   ```bash
   git switch main
   git pull --ff-only origin main
   git status --short
   ```

2. Confirm that the candidate tag does not already exist locally or remotely:

   ```bash
   git show-ref --tags --verify refs/tags/v0.7.0 || true
   git ls-remote --exit-code --tags origin refs/tags/v0.7.0 || true
   ```

3. Update `CHANGELOG.md` and copy `docs/release-notes/NEXT.md` to a versioned
   release-note file.

4. Run the complete local validation:

   ```bash
   make test
   GOEXPERIMENT=jsonv2 go vet ./...
   bash -n scripts/install.sh scripts/uninstall.sh scripts/doctor.sh
   ./scripts/doctor.sh --help >/dev/null
   python3 -m json.tool config/settings.json >/dev/null
   xmllint --noout docs/assets/comparison.svg
   GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOEXPERIMENT=jsonv2 go build -o /tmp/claude-kiro.exe ./cmd/claude-kiro
   ```

5. Push the release-preparation commit and wait for CI to pass on that exact
   commit. Do not push `--tags`.

## Publish

Replace the candidate below with the approved version:

```bash
git switch main
git pull --ff-only origin main
git tag -a v0.7.0 -m "ClaudeCode Kiro v0.7.0"
git push origin refs/tags/v0.7.0
```

The tag triggers `.github/workflows/release.yml`. After it succeeds, replace
the automatically generated notes with the reviewed version:

```bash
gh release edit v0.7.0 --notes-file docs/release-notes/v0.7.0.md
```

Do not use `git push --tags`: it can publish inherited upstream tags and create
ambiguous releases.

## Release checklist

- [ ] Candidate tag is unique locally and on `origin`.
- [ ] `CHANGELOG.md` and versioned release notes match the code.
- [ ] Local validation passes.
- [ ] GitHub CI passes on the exact commit.
- [ ] Only the single new tag was pushed.
- [ ] GoReleaser completed for all six OS/architecture targets.
- [ ] Checksums and extracted `claude-kiro --help` were verified.
- [ ] GitHub Release notes were replaced with the reviewed markdown.
