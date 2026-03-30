# Cumasach

Cumasach defines an OCI-native packaging format for Agent Skills.

The goal is to bring versioning, provenance, dependency resolution, rollback, and enterprise policy to skills without turning skills into containers. A Cumasach package contains skill content such as `SKILL.md`, scripts, references, and templates. It does not bundle host tools like `uv`, `node`, or `jq`.

The format is designed to:

- Package a single skill version as a `tar.gz`
- Publish that package to an OCI registry
- Preserve provenance and signatures
- Resolve skill-to-skill dependencies
- Materialize exactly one active version of each skill into a flat runtime skills directory
- Work with stock OCI tooling such as `oras`

The packaging standard uses a neutral `agentskills` namespace for OCI media types and schema identifiers so it can be adopted beyond a single project or CLI.

## Status

This repository now contains:

- the v1 specification draft
- JSON Schemas and examples
- a Go reference CLI slice for `package`, `push`, `lock`, and dependency-aware `install`, including `install --lockfile`

## Repository Layout

- `docs/spec/packaging-v1.md`: normative package, registry, dependency, lockfile, and install-state specification
- `docs/spec/oci-conventions-v1.md`: OCI media types, registry layout, and ORAS transport conventions
- `docs/spec/conformance-v1.md`: conformance requirements and test matrix
- `docs/spec/cli-v1.md`: normative v1 CLI command surface and flag semantics
- `schemas/skill-manifest-v1.schema.json`: JSON Schema for `.skill/manifest.json`
- `schemas/skill-lock-v1.schema.json`: JSON Schema for lockfiles
- `schemas/install-state-v1.schema.json`: JSON Schema for local install state
- `examples/list-directory`: public demo skill used in the CLI walkthrough
- `examples/workspace-notes`: tiny dependency demo skill
- `examples/workspace-summary`: tiny root demo skill with required dependencies
- `examples/python-development`: example skill package layout
- `examples/oras`: example ORAS commands for publishing and pulling skill artifacts
- `implementation/go`: Go reference implementation of the current CLI slice

## Core Model

A Cumasach-compliant publisher produces:

1. A skill directory payload containing `SKILL.md` and related files
2. A mirrored offline manifest at `.skill/manifest.json`
3. A `tar.gz` of that directory
4. An OCI artifact whose config blob contains the canonical manifest metadata

Runtimes such as OpenClaw continue to read a flat skills directory. Cumasach sits in front of the runtime as the packaging, resolution, verification, and activation layer.

## OCI Interoperability

The format is intentionally OCI-native. A valid Cumasach artifact must be pushable and pullable with stock `oras`.

`cumasach` as a CLI is expected to add skill-aware behavior on top of OCI transport:

- `package`
- `push`
- `install`
- `lock`
- `rollback`
- `verify`

## Reference CLI

The current Go reference implementation lives in `implementation/go`.

The implemented vertical slice is:

- `cumasach package <skill-dir>`
- `cumasach push <package.tgz> <oci-repo> [--tag <tag>]`
- `cumasach lock <artifact-ref|package-name> [--from <oci-base>] [--output <file>]`
- `cumasach install <artifact-ref|package-name> --target <skills-dir> [--from <oci-base>] [--lockfile <file>]`
- `cumasach install --lockfile <file> --target <skills-dir>`
- `cumasach rollback --target <skills-dir>`

Current limitations:

- verify is specified but not implemented yet
- dependency resolution currently covers required dependencies only

## Quick Start

Install the repo-managed toolchain:

```bash
mise install
```

Run the CLI from the Go module:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach --help
```

### Demo Skills

The public demo skills are:

- `examples/list-directory`
- `examples/workspace-notes`
- `examples/workspace-summary`

Package it:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach package ../../examples/list-directory --files-sha256
```

That writes:

```text
implementation/go/dist/list-directory-1.2.3.tgz
```

Push it to an OCI registry:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach push ./dist/list-directory-1.2.3.tgz registry.example.com/agentskills/list-directory
```

The command prints a canonical digest-pinned artifact reference like:

```text
oci://registry.example.com/agentskills/list-directory@sha256:...
```

Install that exact artifact into a flat runtime-visible skills directory:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach install oci://registry.example.com/agentskills/list-directory@sha256:... --target /tmp/cumasach-skills
```

On success the target contains:

```text
/tmp/cumasach-skills/
  list-directory/
  .cumasach/install-state.json
```

The runtime-visible skill entry remains flat:

- `/tmp/cumasach-skills/list-directory`

The hidden `.cumasach/` directory is CLI metadata, not runtime-facing skill content.

### Dependency-Aware Install

The CLI can also resolve required dependencies from a repository base.

Package and push the tiny dependency demo:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach package ../../examples/workspace-notes --files-sha256
mise exec -- go run ./cmd/cumasach package ../../examples/workspace-summary --files-sha256
mise exec -- go run ./cmd/cumasach push ./dist/workspace-notes-1.0.0.tgz registry.example.com/agentskills/workspace-notes
mise exec -- go run ./cmd/cumasach push ./dist/workspace-summary-1.0.0.tgz registry.example.com/agentskills/workspace-summary
```

Then install the root skill by package name into a fresh target directory:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach install workspace-summary --from registry.example.com/agentskills --target /tmp/cumasach-skills-deps
```

On success the target contains one flat directory per active skill plus install metadata:

```text
/tmp/cumasach-skills-deps/
  workspace-notes/
  workspace-summary/
  .cumasach/install-state.json
```

### Lockfile Workflow

You can also freeze the resolved graph and install it later without live re-resolution:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach lock workspace-summary --from registry.example.com/agentskills --output ./skill.lock.json
mise exec -- go run ./cmd/cumasach install --lockfile ./skill.lock.json --target /tmp/cumasach-skills-locked
```

Mixed form is also supported for explicit root validation:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach install workspace-summary --from registry.example.com/agentskills --lockfile ./skill.lock.json --target /tmp/cumasach-skills-locked
```

In lockfile mode:

- the requested root, if provided, must match the lockfile root
- `--from` is only used for package-name root validation and does not affect fetch selection
- installs fetch exactly the digest-pinned artifacts recorded in the lockfile

## Non-Goals

- Bundling language runtimes or host binaries
- Defining container execution environments
- Replacing the Agent Skills execution model
- Requiring runtimes to understand OCI directly
