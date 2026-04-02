# README Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite README.md as a marketing-forward landing page for skill authors, and extract developer-operations content into CONTRIBUTING.md.

**Architecture:** Two-file change. README.md is rewritten top-to-bottom following the approved section structure (hook → value table → quick start → dependency resolution → design decisions → status → repo layout → build → non-goals). CONTRIBUTING.md is created from content removed from the README (ORAS conformance, detailed demo walkthrough, mise workflow details).

**Tech Stack:** Markdown only.

---

### Task 1: Rewrite README.md

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Replace README.md with the new content**

```markdown
# Cumasach

**OCI-native packaging for Agent Skills.**

Agent skills are everywhere — Claude Code, Cursor, Copilot, Codex, OpenClaw — but distribution is still git clones and folder copies. No versioning beyond "whatever's on main." No dependency resolution. No provenance. A recent audit found that [66% of published skills have at least one security finding](https://dev.to/ecap0/the-state-of-mcp-server-security-in-2026-118-findings-across-68-packages-4fkd).

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
```

- [ ] **Step 2: Review the rendered README locally**

Run: `cat README.md | head -5` to verify the header is correct.
Expected: First line is `# Cumasach`, third line is `**OCI-native packaging for Agent Skills.**`

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: rewrite README as marketing-forward landing page"
```

---

### Task 2: Create CONTRIBUTING.md

**Files:**
- Create: `CONTRIBUTING.md`

- [ ] **Step 1: Create CONTRIBUTING.md with developer-operations content**

This file captures the detailed build, test, and conformance workflows that were removed from the README. Source the ORAS conformance content from the current README (prior to rewrite — reference git history or the spec at `docs/spec/conformance-v1.md` for accuracy).

```markdown
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

### Package, push, and install walkthrough

Package a demo skill:

```bash
cd implementation/go
mise exec -- go run ./cmd/cumasach package ../../examples/list-directory --files-sha256
```

This writes `implementation/go/dist/list-directory-1.2.3.tgz`.

Push to an OCI registry:

```bash
mise exec -- go run ./cmd/cumasach push ./dist/list-directory-1.2.3.tgz registry.example.com/agentskills/list-directory
```

Install into a flat runtime directory:

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

Set these environment variables:

```bash
export CUMASACH_ORAS_CONFORMANCE_REPOSITORY=registry.example.com/agentskills/conformance
export CUMASACH_ORAS_CONFORMANCE_USERNAME=robot
export CUMASACH_ORAS_CONFORMANCE_PASSWORD=secret
# optional for HTTP-only test registries
export CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP=1
```

Trust the repo root for mise:

```bash
mise trust
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
```

- [ ] **Step 2: Verify the file renders correctly**

Run: `head -3 CONTRIBUTING.md`
Expected: First line is `# Contributing to Cumasach`

- [ ] **Step 3: Commit**

```bash
git add CONTRIBUTING.md
git commit -m "docs: add CONTRIBUTING.md with developer workflow details"
```
