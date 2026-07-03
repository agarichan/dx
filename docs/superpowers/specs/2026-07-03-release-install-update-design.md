# Release binaries + one-liner install + dx update — design

Date: 2026-07-03
Status: approved

## Goal

Distribute dx as prebuilt single binaries: a tag push produces a GitHub
Release with per-platform binaries, users install with one curl line, and the
binary can replace itself via `dx update`.

## Decisions

- Hand-rolled release workflow (no goreleaser): dx is CGO-free, so a 4-target
  `go build` matrix in ~20 lines of YAML covers it. Go's linker ad-hoc-signs
  darwin/arm64 automatically. Revisit goreleaser only if a Homebrew tap is
  wanted.
- Raw binaries (no archives): assets `dx-<os>-<arch>` for darwin/linux ×
  arm64/amd64, plus a `SHA256SUMS` file (`shasum -a 256` format).
- Release procedure = `git tag vX.Y.Z && git push origin vX.Y.Z`.

## Components

### 1. Version injection
- `internal/cli/dispatch.go`: `const version` → `var version = "dev"`.
- Release builds pass
  `-ldflags "-s -w -X github.com/agarichan/dx/internal/cli.version=<tag without v>"`.
- Local builds (mise / go install) show `dev`.

### 2. `.github/workflows/release.yml`
On `push: tags: [v*]`: checkout → setup-go → `go test ./... -short` → build
the 4 targets with `CGO_ENABLED=0 -trimpath` → `SHA256SUMS` → `gh release
create "$TAG" dist/* --generate-notes` (needs `permissions: contents: write`).

### 3. `install.sh` (repo root)
`curl -fsSL https://raw.githubusercontent.com/agarichan/dx/main/install.sh | sh`
- `uname -s/-m` → asset name (x86_64→amd64, aarch64/arm64→arm64); unsupported
  → clear error.
- Downloads `releases/latest/download/<asset>` + `SHA256SUMS`, verifies the
  checksum (`shasum -a 256` or `sha256sum`, whichever exists).
- Installs to `${DX_INSTALL_DIR:-$HOME/.local/bin}/dx`; prints the installed
  version; warns when the dir is not on PATH.

### 4. `dx update` — internal/selfupdate
`Run(opts, stdout)`, everything injectable for tests:

```go
type Options struct {
	Repo    string // "agarichan/dx"
	Current string // cli version var
	Force   bool
	APIBase string // default https://api.github.com   (httptest override)
	DLBase  string // default https://github.com       (httptest override)
	OS, Arch string // default runtime.GOOS/GOARCH
	ExePath  string // default os.Executable()
	Client   *http.Client
}
```

Flow:
1. `GET {APIBase}/repos/{Repo}/releases/latest` → `tag_name` (unauthenticated;
   rate limits are irrelevant at this frequency).
2. `latest == Current` → "already up to date", exit 0.
3. `Current == "dev"` and not Force → error: not a release build, use
   `--force` to overwrite.
4. Download `{DLBase}/{Repo}/releases/download/{tag}/SHA256SUMS` and the
   `dx-<os>-<arch>` asset; verify SHA256.
5. Write to a temp file in `Dir(ExePath)` (0755) and `rename` over ExePath —
   atomic on the same filesystem; the original is untouched on any failure.
6. Print `updated: <current> -> <latest>`.

CLI: `dx update [--force]` + help text + COMMANDS entry.

### 5. README
Install section leads with the curl one-liner; `go install` and source build
remain as alternatives. `dx update` documented.

## Testing

- selfupdate: httptest server serving the API JSON, SHA256SUMS, and binary —
  happy path replaces the exe file; up-to-date path; checksum mismatch leaves
  the exe untouched; dev-without-force errors.
- install.sh: shellcheck-clean by inspection; real E2E after the first
  release.
- E2E (after merging): tag `v0.2.0` → release workflow → run the one-liner
  into a scratch dir → `dx version` = 0.2.0 → `dx update` says up to date →
  copy of a dev binary + `dx update --force` becomes 0.2.0.

## Out of scope

Homebrew tap, Windows, release-notes curation, notarization/signing beyond
Go's ad-hoc signature.
