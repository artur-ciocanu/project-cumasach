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

This repository currently contains the v1 specification draft, JSON Schemas, and example artifacts.

## Repository Layout

- `docs/spec/packaging-v1.md`: normative package, registry, dependency, lockfile, and install-state specification
- `docs/spec/oci-conventions-v1.md`: OCI media types, registry layout, and ORAS transport conventions
- `docs/spec/conformance-v1.md`: conformance requirements and test matrix
- `schemas/skill-manifest-v1.schema.json`: JSON Schema for `.skill/manifest.json`
- `schemas/skill-lock-v1.schema.json`: JSON Schema for lockfiles
- `schemas/install-state-v1.schema.json`: JSON Schema for local install state
- `examples/python-development`: example skill package layout
- `examples/oras`: example ORAS commands for publishing and pulling skill artifacts

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
- `pull`
- `resolve`
- `verify`
- `install`
- `rollback`
- `lock`

## Non-Goals

- Bundling language runtimes or host binaries
- Defining container execution environments
- Replacing the Agent Skills execution model
- Requiring runtimes to understand OCI directly
