# Cumasach Packaging Specification v1

## 1. Scope

This document defines version 1 of the Cumasach packaging format for Agent Skills.

OCI transport conventions and conformance expectations are further defined in [oci-conventions-v1.md](./oci-conventions-v1.md) and [conformance-v1.md](./conformance-v1.md).

The specification covers:

- skill package structure
- OCI artifact mapping
- metadata model
- dependency semantics
- lockfile format
- local install-state format
- activation and rollback behavior
- interoperability requirements

This specification does not define:

- how a runtime executes `SKILL.md`
- how host tools such as `uv`, `node`, or `jq` are installed
- container or sandbox execution environments

## 2. Conformance Language

The key words `MUST`, `MUST NOT`, `REQUIRED`, `SHOULD`, `SHOULD NOT`, and `MAY` in this document are to be interpreted as described in RFC 2119 and RFC 8174.

## 3. Design Goals

Version 1 is designed to satisfy these goals:

- one versioned OCI artifact per skill
- compatibility with flat-directory runtimes
- deterministic dependency resolution
- rollback to previous known-good active states
- provenance and signature support
- transport compatibility with stock `oras`

## 4. Terminology

`Skill payload`
: The runtime-facing directory containing `SKILL.md` and related files.

`Skill package`
: A versioned tarball of one skill payload plus mirrored package metadata.

`Canonical manifest`
: The package metadata stored in the OCI config blob.

`Mirrored manifest`
: The offline copy of the manifest stored at `.skill/manifest.json` inside the tarball.

`Active view`
: The flat runtime-visible skills directory containing exactly one active directory per skill name.

`Local store`
: A local cache outside the runtime-visible skills directory that may retain multiple versions of the same skill.

`Resolution graph`
: The directed graph of selected packages and dependency edges produced during dependency solving.

## 5. Package Structure

### 5.1 Required layout

A skill package MUST contain exactly one top-level directory.

The top-level directory name MUST exactly match the manifest `name` field.

Example:

```text
python-development/
  SKILL.md
  scripts/
  references/
  templates/
  assets/
  .skill/
    manifest.json
    files.sha256
```

### 5.2 Required files

The package MUST contain:

- `SKILL.md`
- `.skill/manifest.json`

The package MAY contain:

- `scripts/`
- `references/`
- `templates/`
- `assets/`
- `.skill/files.sha256`
- other support files referenced by `SKILL.md`

### 5.3 Extraction safety

When reading or extracting a package, consumers MUST reject tar entries that:

- are absolute paths
- contain `..` path traversal segments
- are symbolic links
- are hard links
- are device files, fifos, or other special files

Consumers MUST treat such packages as malformed.

### 5.4 Reserved metadata directory

The `.skill/` directory is reserved for packaging and distribution metadata.

Runtimes that execute skills:

- MUST NOT derive execution semantics from `.skill/`
- SHOULD ignore `.skill/` for runtime behavior

Packaging tools:

- MUST understand `.skill/manifest.json`
- MAY read `.skill/files.sha256`

## 6. OCI Artifact Mapping

### 6.1 Manifest type

A Cumasach artifact MUST use the standard OCI image manifest.

### 6.2 Media types

Version 1 defines these media types:

- Config blob: `application/vnd.agentskills.config.v1+json`
- Skill content layer: `application/vnd.agentskills.skill.content.v1.tar+gzip`

Future versions MAY define additional layer or attestation media types.

### 6.3 Canonical metadata location

The OCI config blob is the canonical metadata source for a package.

The config blob MUST conform to the schema defined in [../../schemas/skill-manifest-v1.schema.json](../../schemas/skill-manifest-v1.schema.json).

### 6.4 Mirrored metadata location

The file `.skill/manifest.json` MUST exist in the tarball payload and MUST be byte-for-byte identical to the canonical OCI config blob.

If the canonical and mirrored manifests differ, the consumer MUST fail the package as invalid.

### 6.5 ORAS compatibility

A valid Cumasach artifact MUST be pushable and pullable using stock `oras`.

The specification does not require `oras` to understand dependency semantics, activation, or rollback. Those behaviors are the responsibility of skill-aware tooling.

## 7. Manifest Object

### 7.1 Required top-level fields

The manifest object MUST contain:

- `schemaVersion`
- `packageType`
- `name`
- `version`
- `skill`

### 7.2 Fixed values

- `schemaVersion` MUST be `"v1"`
- `packageType` MUST be `"skill"`

### 7.3 Name

`name`:

- MUST be the logical skill identifier
- MUST be stable across versions of the same skill
- MUST match the regex `^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`

### 7.4 Version

`version` MUST be a valid Semantic Version 2.0.0 string.

Build metadata MAY be present in the manifest version, but publishers SHOULD avoid build metadata when using the same value as an OCI tag because registry tooling commonly treats tags as exact strings.

### 7.5 Skill object

The `skill` object MUST contain:

- `entrypoint`

In v1, `entrypoint` MUST be `"SKILL.md"`.

### 7.6 Description

`description` is OPTIONAL and SHOULD be a short human-readable summary.

### 7.7 Dependencies

`dependencies` is OPTIONAL.

If present, each dependency object MUST contain:

- `name`
- `version`
- `relationship`

Valid `relationship` values:

- `required`
- `recommended`
- `extends`

#### 7.7.1 Constraint language

Dependency `version` values MUST use the Helm-compatible SemVer constraint language used for chart dependencies.

Allowed forms include:

- exact versions such as `1.2.3`
- comparator expressions using `=`, `!=`, `>`, `<`, `>=`, and `<=`
- hyphen ranges such as `1.1 - 2.3.4`
- wildcard ranges using `x`, `X`, or `*`
- caret ranges such as `^2.3.0`
- tilde ranges such as `~1.4.2`
- comparator sets such as `>=1.0.0 <2.0.0`
- logical OR expressions using `||`

Whitespace-separated comparator sets MUST be interpreted as logical AND.

Consumers MUST interpret prerelease matching and comparator semantics consistently with that grammar. In particular, prerelease versions MUST NOT satisfy a constraint unless the constraint itself admits a prerelease according to the Helm-compatible semantics, for example `~1.2.3-0`.

Consumers MUST reject unsupported operators or shorthand forms rather than attempting best-effort parsing.

JSON Schema validation alone does not fully validate this constraint grammar in v1. Consumers MUST perform semantic validation of dependency and compatibility constraint strings in addition to schema validation.

#### 7.7.2 Dependency semantics

- `required`: the consumer MUST include exactly one compatible version in the resolved graph. If no such version can be resolved, installation MUST fail.
- `recommended`: the consumer SHOULD include a compatible version unless disabled by policy.
- `extends`: the consumer MAY surface this relationship for composition, but MUST NOT treat it as a hard install requirement.

Policy MUST NOT downgrade `required` into best-effort behavior.

### 7.8 Requirements

`requirements` is OPTIONAL.

If present, it MAY include:

- `binaries`: array of binary names expected on the host
- `os`: array of supported operating systems
- `env`: array of required environment variable names

Requirements are declarative only. A package MUST NOT bundle the referenced host binaries as part of v1 semantics.

### 7.9 Compatibility

`compatibility` is OPTIONAL.

If present, it MAY include:

- `agentSkills`: version constraint for the Agent Skills specification
- `runtimes`: array of runtime compatibility objects

Each runtime compatibility object MUST contain:

- `name`
- `version`

The `version` field in `compatibility` objects MUST use the same constraint language defined in section 7.7.1.

### 7.10 Source and publisher

`source` and `publisher` are OPTIONAL metadata objects intended for provenance, policy, and discovery.

## 8. Integrity Files

### 8.1 files.sha256

`.skill/files.sha256` is OPTIONAL.

If present:

- it MUST list hashes for packaged files relative to the skill root
- it MUST NOT include itself
- it SHOULD include `.skill/manifest.json`

Consumers MAY use `files.sha256` for offline verification, but MUST treat OCI digests and registry-backed signatures as the primary trust source when OCI metadata is available.

## 9. Dependency Resolution

### 9.1 Resolution source

Dependency resolution MUST be performed against published package metadata, not only against locally active skills.

### 9.2 Version selection

If no lockfile is supplied, a consumer MUST choose the highest available version that satisfies all applicable constraints.

Stable versions MUST sort ahead of prerelease versions with the same base version unless the constraint admits only prerelease matches.

For equal versions available under multiple references, policy MAY choose among allowed sources, but the selected package digest MUST be recorded.

### 9.3 Conflict handling

If dependency constraints cannot be satisfied, installation MUST fail.

Consumers MUST NOT guess, silently downgrade, or activate multiple versions of the same skill name in a single active view.

### 9.4 Recommended dependencies

Recommended dependencies SHOULD be installed by default only when policy permits.

### 9.5 Extends dependencies

`extends` dependencies are descriptive. They MUST NOT by themselves cause installation failure.

### 9.6 Dependency cycles

If the resolved graph contains a cycle of `required` dependencies, consumers MUST fail unless the cycle can be represented with one selected version per skill name and no unresolved constraints remain.

Implementations SHOULD reject self-dependencies.

## 10. Lockfile Format

A lockfile records a fully resolved dependency graph for a target active view.

The lockfile MUST conform to [../../schemas/skill-lock-v1.schema.json](../../schemas/skill-lock-v1.schema.json).

### 10.1 Required fields

- `schemaVersion`
- `root`
- `packages`
- `edges`

### 10.2 Semantics

- `root` identifies the requested package
- `packages` lists every resolved skill, including transitive dependencies
- the package selected by `root.name` and `root.version` MUST also appear in `packages`
- each package entry MUST include name, version, digest, and source reference
- `edges` records dependency edges from parent to child package names and preserves per-edge relationship semantics

### 10.3 Uniqueness

Within a lockfile:

- `packages` MUST contain at most one entry for each package `name`
- every `edge.from` and `edge.to` value MUST refer to a package in `packages`

### 10.4 Reproducibility

When a valid lockfile is present, consumers MUST prefer the lockfile over live dependency solving.

## 11. Install-State Format

Install state records what is currently active in a given runtime-visible skills directory.

The install-state file MUST conform to [../../schemas/install-state-v1.schema.json](../../schemas/install-state-v1.schema.json).

### 11.1 Purpose

Install state exists to support:

- deterministic activation
- rollback
- auditing

### 11.2 Required fields

- `schemaVersion`
- `target`
- `active`
- `history`

### 11.3 Active view

The `active` array records exactly one active version per skill name in the target runtime-visible directory.

### 11.4 History semantics

Each history entry MUST record the post-action active snapshot for that action.

Each snapshot entry in `history` MUST include enough information to re-fetch the selected artifact after local cache eviction, including its source reference.

## 12. Activation Model

### 12.1 Flat runtime requirement

The runtime-visible skills directory is flat.

For a given target directory:

- only one active directory per skill name MAY exist
- the active directory name MUST equal the manifest `name`
- multiple versions of the same skill MUST NOT coexist under the same runtime-visible root

### 12.2 Local store

Consumers MAY retain multiple downloaded versions in a local store outside the runtime-visible skills directory.

### 12.3 Activation

Activation MUST materialize exactly one selected version of each skill into the target active view.

Materialization MAY be implemented by copying, linking, or equivalent means, provided the runtime-visible result remains a normal flat skill directory layout.

### 12.4 Replacement

Installing a newer version of a skill into the same target active view replaces the previously active version of that skill name in that view.

## 13. Rollback

### 13.1 Requirement

A compliant consumer MUST retain enough recorded state to restore a previously active resolved set.

### 13.2 Mechanism

Rollback MUST use recorded install state or a lockfile-equivalent historical record.

### 13.3 Behavior

On rollback, the consumer MUST re-materialize the previously recorded active set into the runtime-visible directory.

## 14. Policy

Version 1 leaves policy expression implementation-defined, but a compliant ecosystem SHOULD support policy controls for:

- signature verification behavior
- provenance verification behavior
- allowed registries
- allowed publishers or signing identities
- handling of missing host requirements
- whether recommended dependencies are auto-installed

## 15. Failure Conditions

A consumer MUST fail a package when any of the following is true:

- the tarball contains more than one top-level skill directory
- the top-level skill directory name differs from manifest `name`
- `SKILL.md` is missing
- `.skill/manifest.json` is missing
- the mirrored manifest does not match the OCI config blob
- the manifest fails schema validation
- dependency constraints are unsatisfiable
- the package payload path layout is malformed

## 16. Example

An example package layout is provided in [../../examples/python-development](../../examples/python-development).

## 17. Future Work

These are explicitly out of scope for v1 but likely candidates for later versions:

- first-class provenance attachment schemas
- SBOM attachment conventions
- signed lockfiles
- profile- or environment-scoped activation sets
- package collections or bundles
- optional feature flags for dependencies
