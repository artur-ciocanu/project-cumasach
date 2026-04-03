# Go Release Pipeline Design

**Date:** 2026-04-03
**Status:** Approved
**Scope:** GoReleaser configuration, GitHub Actions CI/CD, SLSA provenance, security hardening

## Overview

Set up a production release pipeline for the `cumasach` Go CLI using GoReleaser and GitHub Actions. The pipeline cross-compiles for macOS, Linux, and Windows, publishes GitHub Releases on tag push, and generates SLSA Level 3 provenance attestations for supply chain security.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Versioning | Semver, starting at `v0.1.0` | Go ecosystem expects semver; spec is v1 but CLI is still maturing |
| Release trigger | Tag-driven (`v*` tags) | Go community standard, simple, idiomatic |
| Release tool | GoReleaser | De facto standard for Go CLI releases |
| Signing | SLSA Level 3 via Sigstore (keyless) | GitHub OIDC + Sigstore requires no key management |
| Homebrew tap | Deferred | Keep initial release simple; easy to add later |
| Distribution model | GitHub Releases only | Sufficient for v0.x; additional channels later |

## Target Platforms

| OS | Arch | Binary Name | Archive Format |
|----|------|-------------|----------------|
| linux | amd64 | `cumasach` | `.tar.gz` |
| linux | arm64 | `cumasach` | `.tar.gz` |
| darwin | amd64 | `cumasach` | `.tar.gz` |
| darwin | arm64 | `cumasach` | `.tar.gz` |
| windows | amd64 | `cumasach.exe` | `.zip` |

- No Windows arm64 (negligible demand for Go CLIs on Windows ARM)
- No 32-bit targets
- Archive naming: `cumasach_<version>_<os>_<arch>`
- SHA256 checksums file attached to every release

## GitHub Actions Workflows

### `ci.yml` -- Continuous Integration

- **Triggers:** push to `main`, pull requests targeting `main`
- **Go version:** pinned to `1.25.x` (matches `go.mod`)
- **Working directory:** `implementation/go/`
- **Permissions:** `contents: read` only
- **Jobs:**
  - **test** -- `go test ./...`
  - **lint** -- `golangci-lint run`
  - **build** -- `go build ./cmd/cumasach` (compile check, discard binary)

### `release.yml` -- Build, Publish & Attest

- **Triggers:** push of tags matching `v*`
- **Permissions:** `contents: write` (create releases), `id-token: write` (SLSA/Sigstore)
- **Jobs:**
  - **goreleaser** -- checks out code, runs GoReleaser, uploads binaries + checksums to GitHub Release
  - **provenance** -- calls `slsa-framework/slsa-github-generator` reusable workflow to generate and attach SLSA Level 3 provenance attestation (signed via Sigstore keyless/GitHub OIDC)

## GoReleaser Configuration

Location: `implementation/go/.goreleaser.yml`

### Build

- Single binary: `cumasach`
- Entry point: `./cmd/cumasach`
- CGO disabled (pure Go, clean cross-compilation)
- ldflags inject version metadata:
  ```
  -X main.version={{.Version}}
  -X main.commit={{.ShortCommit}}
  -X main.date={{.Date}}
  ```

### Archives

- Default format: `tar.gz`
- Windows override: `zip`
- Name template: `cumasach_{{ .Version }}_{{ .Os }}_{{ .Arch }}`
- Contents: binary only (no README/LICENSE in archive)

### Checksum

- Algorithm: SHA256
- Output: `checksums.txt` attached to release

### Changelog

- Auto-generated from commits between tags
- Grouped by: features, fixes, other
- Filtered: docs-only commits, merge commits excluded

### Release

- GitHub Release created automatically
- Draft: false (publishes immediately)
- Prerelease: auto (tags like `v0.1.0-rc1` marked as prerelease)

## Code Changes

### Version Injection

Add version variables to `cmd/cumasach/main.go` (the `main` package, where ldflags target `main.*` variables):

```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

Wire into cobra's root command `Version` field so `cumasach --version` prints:

```
cumasach 0.1.0 (abc1234, 2026-04-03)
```

GoReleaser populates these via ldflags at build time. Local `go build` uses the defaults above.

## Security Hardening

### Workflow Permissions

- Every workflow explicitly declares `permissions` (no reliance on repo defaults)
- Default: `contents: read`
- Only `release.yml` escalates to `contents: write` and `id-token: write`
- No `permissions: write-all` anywhere

### Action Pinning

All third-party actions pinned to full commit SHAs, not tags:

```yaml
actions/checkout@<full-sha>             # v4.x.x
actions/setup-go@<full-sha>             # v5.x.x
goreleaser/goreleaser-action@<full-sha> # v6.x.x
```

Version noted in comments for readability.

### Branch Protection on `main`

Configured in GitHub repo settings (manual step):

- Require status checks to pass (`test`, `lint`, `build`)
- Prevent force pushes to `main`
- Prevent deletion of `main`
- No bypass for admins

### SLSA Provenance

- SLSA Level 3 attestation generated on every release
- Uses `slsa-framework/slsa-github-generator` reusable workflow
- Keyless signing via GitHub OIDC + Sigstore
- Provenance JSON attached to the GitHub Release
- Users can verify with `slsa-verifier`

### Additional Security Files

- **`.github/CODEOWNERS`** -- maps `*` to maintainer, auto-assigns PR reviews
- **`.github/SECURITY.md`** -- vulnerability reporting policy

### Not Included (deferred)

- Dependabot / Renovate (can layer on later)
- CLA bot (solo maintainer)
- Required PR reviews (solo maintainer)
- Disabling forks (counterproductive for public open source)

## New Files

```
project-cumasach/
├── .github/
│   ├── workflows/
│   │   ├── ci.yml
│   │   └── release.yml
│   ├── CODEOWNERS
│   └── SECURITY.md
└── implementation/go/
    └── .goreleaser.yml
```

## Manual Steps

After implementation:

1. **GitHub repo settings:** enable branch protection on `main` per the security section
2. **First release:** `git tag v0.1.0 && git push --tags`

## Release Flow

When a `v*` tag is pushed:

1. CI runs (test, lint, build)
2. GoReleaser cross-compiles 5 binaries, creates archives, generates changelog
3. GitHub Release published with archives + `checksums.txt`
4. SLSA provenance attestation generated and attached to the release
5. Users download from GitHub Releases, verify checksums, optionally verify provenance
