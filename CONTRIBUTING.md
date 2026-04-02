# Contributing to Cumasach

## Building from source

Install the repo-managed toolchain:

```bash
mise install
```

Run the CLI from the Go module:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach --help
```

## Demo skills

The repository includes demo skills for testing the full workflow:

- `examples/list-directory`
- `examples/workspace-notes`
- `examples/workspace-summary`
- `examples/python-development` (layout example, not used in walkthroughs below)

### Package, push, and install walkthrough

Package a demo skill:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach package ../../examples/list-directory --files-sha256
```

This writes `implementation/go/dist/list-directory-1.2.3.tgz`.

Push to an OCI registry (from `implementation/go`):

```bash
mise exec -- go run ./cmd/cumasach push ./dist/list-directory-1.2.3.tgz registry.example.com/agentskills/list-directory
```

Install into a flat runtime directory (from `implementation/go`):

```bash
mise exec -- go run ./cmd/cumasach install oci://registry.example.com/agentskills/list-directory@sha256:... --target /tmp/cumasach-skills
```

### Dependency-aware install walkthrough

Package and push all three demo skills, then install the root:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach package ../../examples/list-directory --files-sha256
mise exec -- go run ./cmd/cumasach package ../../examples/workspace-notes --files-sha256
mise exec -- go run ./cmd/cumasach package ../../examples/workspace-summary --files-sha256
mise exec -- go run ./cmd/cumasach push ./dist/list-directory-1.2.3.tgz registry.example.com/agentskills/list-directory
mise exec -- go run ./cmd/cumasach push ./dist/workspace-notes-1.0.0.tgz registry.example.com/agentskills/workspace-notes
mise exec -- go run ./cmd/cumasach push ./dist/workspace-summary-1.0.0.tgz registry.example.com/agentskills/workspace-summary

mise exec -- go run ./cmd/cumasach install workspace-summary --from registry.example.com/agentskills --target /tmp/cumasach-skills-deps
```

### Lockfile workflow

Freeze the resolved graph and install later:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach lock workspace-summary --from registry.example.com/agentskills --output ./skill.lock.json
mise exec -- go run ./cmd/cumasach install --lockfile ./skill.lock.json --target /tmp/cumasach-skills-locked
```

Mixed form with explicit root validation:

```bash
mise exec -- go run ./cmd/cumasach install workspace-summary --from registry.example.com/agentskills --lockfile ./skill.lock.json --target /tmp/cumasach-skills-locked
```

In lockfile mode:

- The requested root, if provided, must match the lockfile root
- `--from` is only used for package-name root validation and does not affect fetch selection
- Installs fetch exactly the digest-pinned artifacts recorded in the lockfile

## ORAS conformance

Release sign-off requires a stock-`oras` round-trip against a real registry. `scripts/run-oras-conformance.sh` is the canonical entrypoint.

Trust the repo root for mise (required before first run):

```bash
mise trust
```

Set these environment variables:

```bash
export CUMASACH_ORAS_CONFORMANCE_REPOSITORY=registry.example.com/agentskills/conformance
export CUMASACH_ORAS_CONFORMANCE_USERNAME=robot
export CUMASACH_ORAS_CONFORMANCE_PASSWORD=secret
# optional for HTTP-only test registries
export CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP=1
```

Run the conformance check:

```bash
bash scripts/run-oras-conformance.sh
```

The script self-wraps through `mise exec --` so both `go` and the test's `oras` subprocess come from the pinned toolchain in `mise.toml`. It fails fast with a targeted trust message when the repo root is untrusted.

The script runs:

```bash
cd implementation/go
mise exec -- go test ./internal/oci -run '^TestORASConformanceRoundTrip$' -count=1
```

It exits non-zero when required credentials are missing or when the test fails.

## Install behavior

Installing with Cumasach is non-destructive with respect to unrelated pre-existing skills in the target directory. The CLI manages and records the skills it installs, but does not delete unrelated user-provided skill directories outside the current install request or lockfile.
