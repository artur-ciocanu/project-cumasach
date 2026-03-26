# Cumasach OCI Conventions v1

## 1. Scope

This document defines the OCI transport and registry conventions for Cumasach v1.

It complements, but does not replace, the normative rules in [packaging-v1.md](/Users/ciocanu/personal/code/project-cumasach/.worktrees/spec-hardening/docs/spec/packaging-v1.md).

## 2. OCI Object Model

A v1 Cumasach skill artifact consists of:

- one OCI image manifest
- one config blob using the Cumasach manifest JSON
- one `tar.gz` layer containing the skill payload

Version 1 does not require additional OCI layers. Signatures, provenance, and attestations SHOULD be attached via OCI referrers when the registry supports them.

## 3. Media Types

### 3.1 Required media types

Version 1 defines these media types:

- Manifest: `application/vnd.oci.image.manifest.v1+json`
- Config: `application/vnd.agentskills.config.v1+json`
- Content layer: `application/vnd.agentskills.skill.content.v1.tar+gzip`

### 3.2 Rationale

These media types intentionally mirror Helm's pattern:

- standard OCI manifest
- custom config type for package metadata
- custom content layer type for the packaged payload

Consumers MUST NOT reuse Helm's media types for Cumasach artifacts.

## 4. Registry Naming

### 4.1 Repository layout

The recommended repository layout is one OCI repository per skill:

```text
oci://registry.example.com/agentskills/python-development
oci://registry.example.com/agentskills/tdd
oci://registry.example.com/agentskills/prd
```

This is a convention, not a hard protocol requirement. A compliant implementation MAY use a different namespace layout as long as the package and manifest rules are preserved.

### 4.2 Tags and digests

Publishers SHOULD push semver tags such as `1.2.0`.

Consumers SHOULD pin digests in lockfiles and install state.

Digests are the immutable identity of a resolved package version.

## 5. ORAS Interoperability

### 5.1 Requirement

A valid Cumasach artifact MUST be pushable and pullable with stock `oras`.

### 5.2 Reference push shape

The reference shape is:

```text
oras push <registry-repo>:<tag> \
  --config manifest.json:application/vnd.agentskills.config.v1+json \
  <skill-tarball>:application/vnd.agentskills.skill.content.v1.tar+gzip
```

The exact command-line flags MAY differ by ORAS version, but the artifact produced MUST preserve the media types and blob contents defined by this specification.

### 5.3 Reference pull shape

Consumers SHOULD be able to retrieve the payload with:

```text
oras pull <registry-repo>@sha256:<digest>
```

The output tarball and config blob MUST correspond to the published artifact.

## 6. Provenance and Signatures

### 6.1 Canonical trust source

When an artifact is fetched from OCI, the canonical trust source is:

- the OCI manifest digest
- the config blob digest
- any registry-backed signatures or attestations

### 6.2 Referrers

If the registry supports OCI referrers, implementations SHOULD publish signatures and provenance as referrers to the resolved artifact digest.

### 6.3 Offline fallback

When an artifact is used outside a registry context, `.skill/manifest.json` and `.skill/files.sha256` MAY be used for offline verification, but they are not a replacement for registry-backed signatures.

## 7. Publisher Behavior

Publishers:

- MUST produce byte-stable config JSON for a given artifact
- MUST ensure the mirrored manifest matches the OCI config blob exactly
- SHOULD publish semver tags
- SHOULD publish digests in release metadata

## 8. Consumer Behavior

Consumers:

- MUST validate the config media type
- MUST validate the content-layer media type
- MUST fail if multiple content layers are present in a v1 artifact
- SHOULD prefer digest-based installation and locking

## 9. Future Extension Points

Later versions MAY define:

- a dedicated provenance attachment media type
- SBOM attachment conventions
- bundle or collection artifact types
- lockfile artifact publication conventions
