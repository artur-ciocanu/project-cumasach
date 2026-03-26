# Cumasach Conformance v1

## 1. Purpose

This document defines the minimum conformance expectations for publishers, consumers, and registry tooling around Cumasach v1.

## 2. Conformance Classes

### 2.1 Publisher

A publisher implementation is conformant if it can:

- generate a valid `.skill/manifest.json`
- package a skill directory into a valid `tar.gz`
- mirror the manifest into the OCI config blob
- publish a valid OCI artifact using the required media types

### 2.2 Consumer

A consumer implementation is conformant if it can:

- validate the manifest against the schema
- validate package layout rules
- resolve dependencies according to the v1 rules
- activate one selected version per skill name into a flat runtime-visible directory
- record lock and install-state data
- perform rollback from recorded state

### 2.3 Transport

A transport implementation is conformant if it can:

- push the defined config blob and content layer to an OCI registry
- pull the artifact back without changing blob contents

Stock `oras` is the reference interoperability target for transport conformance.

## 3. Required Test Matrix

### 3.1 Artifact shape

The implementation MUST pass:

- valid single-skill tarball with exactly one top-level directory
- top-level directory name equals manifest `name`
- missing `SKILL.md` fails
- missing `.skill/manifest.json` fails
- more than one top-level directory fails
- tar entries with absolute paths fail
- packages containing symlinks, hardlinks, special files, or path traversal fail
- malformed JSON manifest fails
- schema-invalid manifest fails

### 3.2 OCI mapping

The implementation MUST pass:

- valid config media type accepted
- invalid config media type rejected
- valid content-layer media type accepted
- invalid content-layer media type rejected
- more than one content layer rejected for v1
- OCI config blob equals mirrored manifest byte-for-byte

### 3.3 Dependency resolution

The implementation MUST pass:

- satisfiable `required` dependency graph resolves
- unsatisfied `required` dependency graph fails
- dependency ranges use the Helm-compatible constraint grammar defined by the packaging spec
- `recommended` dependency can be omitted by policy
- `extends` dependency never blocks install by itself
- conflicting constraints on the same skill fail resolution

### 3.4 Activation

The implementation MUST pass:

- one active version per skill name in the runtime-visible directory
- active directory name equals the selected package `name`
- installing a new version replaces the active version in that target
- retained older versions in local cache do not appear in active view
- flat-directory runtimes see normal skill directories only

### 3.5 Locking and rollback

The implementation MUST pass:

- generated lockfile pins digests for all resolved packages
- lockfile includes the selected root package in `packages`
- lockfile preserves dependency edges and per-edge relationship types
- install from lockfile reproduces the same active set
- rollback restores the previous recorded active set

### 3.6 ORAS round-trip

The implementation MUST pass:

- artifact can be pushed with stock `oras`
- payload layer can be pulled with stock `oras`
- canonical config blob can be fetched with stock `oras`
- pulled tarball digest matches the originally published content layer digest
- pulled config blob matches the mirrored manifest

## 4. Failure Handling Requirements

Conformant consumers MUST fail closed for:

- malformed payload structure
- schema-invalid metadata
- manifest mismatch between tarball and OCI config
- unsatisfied dependency constraints
- unsupported mandatory media types

Conformant consumers SHOULD produce machine-readable errors with at least:

- error code
- short message
- path or object reference when applicable

## 5. Interoperability Notes

OpenClaw-style runtimes are the primary compatibility target for v1 activation semantics because they consume a flat skills directory. An implementation does not need to embed into OpenClaw to be conformant, but it MUST preserve the flat runtime-visible layout that such runtimes expect.

## 6. Recommended CI Checks

Implementers SHOULD automate:

- JSON Schema validation
- package layout validation
- ORAS push/pull smoke test against a disposable OCI registry
- dependency-resolution golden tests
- rollback state restoration tests
