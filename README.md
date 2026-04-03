<h1 align="center">Cumasach</h1>

<p align="center">
  <em>Cumasach</em> (pronounced <strong>KUM-uh-sahk</strong>) is the Irish word for "capable" or "powerful,"<br>
  from <em>cumas</em> meaning capability. A fitting name for a tool that makes agent skills production-capable.
</p>

**OCI-native packaging for Agent Skills.**

Agent skills are everywhere — Claude Code, Cursor, Copilot, Codex, OpenClaw — but distribution is still git clones and folder copies. No versioning beyond "whatever's on main." No dependency resolution. No provenance. Security audits routinely surface critical findings — command injection, credential exposure, excessive permissions — across the ecosystem.

Cumasach brings the packaging infrastructure that every other software ecosystem already has. It implements the [Agent Skills specification](https://agentskills.io) and uses OCI registries you already run — GHCR, Artifactory, ECR — so there's nothing new to adopt for transport.

## What you get

| Problem | Today | With Cumasach |
|---------|-------|---------------|
| **Versioning** | Git refs or nothing | SemVer with digest-pinned artifacts |
| **Dependencies** | Manual, undeclared | Declared in manifest, resolved on install |
| **Reproducibility** | Hope the repo hasn't changed | Lockfiles freeze the exact dependency graph |
| **Rollback** | `git revert` and pray | `cumasach rollback` restores previous state |
| **Provenance** | Trust the author | Cryptographic verification of package integrity |
| **Discovery** | Search GitHub | Query any OCI registry with standard tooling |

## Quick start

```bash
# Package a skill directory
cumasach package ./my-skill --files-sha256

# Push to your registry
cumasach push ./dist/my-skill-1.0.0.tgz ghcr.io/my-org/skills/my-skill

# Install it anywhere
cumasach install oci://ghcr.io/my-org/skills/my-skill@sha256:... --target ./skills
```

> Cumasach doesn't have prebuilt binaries yet. See [Building from source](#building-from-source) for how to run the CLI from the Go module.

The result is a flat directory that any agent runtime reads as-is. Claude Code, Cursor, OpenClaw — they all expect a skills folder with `SKILL.md` files. Cumasach materializes exactly that. It sits in front of the runtime as the packaging and verification layer. It doesn't change how skills work. It changes how they ship.

## Dependency resolution

Install a skill and Cumasach resolves its full dependency tree into a flat runtime directory:

```bash
cumasach install workspace-summary \
  --from ghcr.io/my-org/skills \
  --target ./skills
```

```
./skills/
  list-directory/
  workspace-notes/
  workspace-summary/
  .cumasach/install-state.json
```

### Lockfile workflow

Freeze the resolved graph, install later without live re-resolution:

```bash
cumasach lock workspace-summary --from ghcr.io/my-org/skills --output skill.lock.json
cumasach install --lockfile skill.lock.json --target ./skills
```

## Design decisions

**Spec-first, not CLI-first.** This repository contains normative specifications, JSON Schemas, and conformance tests. The Go CLI is a reference implementation — the spec is the product. If someone wants to build a Rust or Python implementation, the spec should be sufficient.

**Builds on what exists.** OCI registries are battle-tested infrastructure with authentication, access control, replication, and audit logging. Dependency version constraints use Helm-compatible SemVer syntax. A Cumasach package is a standard OCI artifact, pushable and pullable with stock [ORAS](https://oras.land/) tooling. Less to learn, less to break.

**Strict v1 schema.** `additionalProperties: false` everywhere. Extensibility goes through the explicit `metadata` field, not through loose schema validation. The schema can loosen in v2; it can never tighten.

**Neutral namespace.** OCI media types and schema identifiers use `agentskills`, not `cumasach`. The format is designed to be adopted beyond a single project.

**No bundled runtimes.** Packages contain skill content — `SKILL.md`, scripts, references, templates. The `requirements` field declares what the host needs, but the package doesn't ship it.

## Status

This repository contains:

- The v1 specification draft
- JSON Schemas and examples
- A Go reference CLI for `package`, `push`, `install`, `lock`, `rollback`, and `verify`
- ORAS conformance tests validated against GHCR and JFrog Artifactory

Current limitations: dependency resolution covers required dependencies only. Optional dependencies and version-range resolution are planned but not yet implemented.

## Repository layout

| Path | Description |
|------|-------------|
| `docs/spec/` | Normative v1 specifications (packaging, OCI conventions, conformance, CLI) |
| `schemas/` | JSON Schemas for manifest, lockfile, and install state |
| `examples/` | Demo skill packages including dependency chains |
| `implementation/go/` | Go reference implementation |

## Building from source

Install the repo-managed toolchain, then run the CLI from the Go module:

```bash
mise install
cd implementation/go
mise exec -- go run ./cmd/cumasach --help
```

## Non-goals

- Bundling language runtimes or host binaries
- Defining container execution environments
- Replacing the agent skills execution model
- Requiring runtimes to understand OCI directly
