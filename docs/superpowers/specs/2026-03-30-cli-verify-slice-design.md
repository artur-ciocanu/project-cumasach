# CLI Verify Slice Design

## Summary

This document defines the next implementation slice for the Cumasach Go reference CLI after the rollback milestone. The slice adds `cumasach verify <package.tgz|artifact-ref>` so the reference CLI can validate local packages and OCI-published artifacts against the published v1 rules.

The slice is intentionally narrow:

- `verify` accepts either a local `.tgz` package path or a canonical OCI artifact reference
- local package verification reuses the existing archive reader and manifest schema loader
- OCI verification reuses the existing fetch path and enforces the required OCI media types and mirrored-manifest rules
- if `.skill/files.sha256` is present in a local package, the implementation verifies its format and the recorded file digests

This slice does not add signature verification, installed-tree verification, policy evaluation, or cache management.

### Goals

- Add `cumasach verify <package.tgz|artifact-ref>`
- Validate local package layout, manifest schema conformance, and top-level naming rules
- Verify `.skill/files.sha256` when present
- Validate OCI artifact media types, config and mirrored manifest equality, and extracted package layout
- Reuse existing archive, manifest, and OCI code paths rather than introducing parallel validation logic

### Deferred v1 Work

The following normative or adjacent work remains outside this implementation slice:

- installed-tree verification
- signature or provenance verification
- policy evaluation
- richer machine-readable error envelopes

This slice must not change the published v1 packaging or CLI contracts. It only implements the `verify` command already defined by those contracts.

### Non-Goals

- verifying an active target directory instead of a package or OCI artifact
- validating lockfiles or install state under `verify`
- introducing a persistent local artifact cache
- expanding dependency-resolution behavior

## User-Facing Behavior

### `cumasach verify`

Supported form:

```bash
cumasach verify <package.tgz|artifact-ref>
```

Input classification:

- if the argument parses as a canonical `oci://...@sha256:...` reference, verify it as an OCI artifact
- otherwise treat the argument as a local package archive path

On success, the command prints a compact success summary and exits zero.

### Local Package Verification

For a local package archive, `verify` MUST:

- read and inspect the archive with the same extraction-safety rules already used by package readers
- require exactly one top-level directory
- require `SKILL.md`
- require `.skill/manifest.json`
- schema-validate `.skill/manifest.json`
- require the top-level directory name to equal `manifest.name`
- if `.skill/files.sha256` is present:
  - parse it as UTF-8 text
  - require `<64 lowercase hex><two spaces><relative path>` per line
  - require paths to be relative and `/`-separated
  - reject unsorted or duplicate paths
  - reject entries for `.skill/files.sha256`
  - verify that every listed file exists in the package and matches its recorded SHA-256 digest

If `.skill/files.sha256` is absent, local verification still succeeds as long as all other mandatory package rules pass.

### OCI Artifact Verification

For an OCI artifact reference, `verify` MUST:

- fetch the artifact using the existing OCI client
- require the OCI manifest descriptor media type to be the standard OCI image manifest
- require the OCI config media type to equal the v1 Cumasach config media type
- require exactly one content layer with the v1 skill content media type
- require the OCI config blob bytes to match the mirrored `.skill/manifest.json` bytes exactly
- require the extracted content layer archive to satisfy the same local package rules as `verify <package.tgz>`

This slice verifies package layout after extraction for OCI artifacts, but does not use `.skill/files.sha256` as an OCI trust root. OCI digest and config checks remain primary when registry metadata is available.

### Failure Cases

`verify` MUST fail for:

- missing input argument
- unreadable local archive path
- malformed package archive structure
- missing `SKILL.md`
- missing or schema-invalid `.skill/manifest.json`
- top-level directory name mismatch
- malformed, unsorted, or duplicate `.skill/files.sha256` entries
- checksum mismatch or missing file for any `.skill/files.sha256` entry
- unsupported OCI media types
- OCI config blob mismatch with mirrored manifest bytes
- extracted OCI payload that violates mandatory package rules

## Architecture

### New Internal Verify Package

Add `implementation/go/internal/verify` as the orchestration layer for the command. It should expose two entry points:

- `VerifyPackage(path string) (Result, error)`
- `VerifyReference(ctx context.Context, registry oci.Registry, reference string) (Result, error)`

The CLI command remains thin and delegates classification and execution to this package.

### Reuse Existing Readers

The implementation should reuse current validation surfaces wherever possible:

- `internal/archive` for tarball path safety, top-level structure, and mirrored-manifest loading
- `internal/manifest` for schema validation
- `internal/oci.Fetch` for descriptor and media-type validation plus blob retrieval

This keeps `verify`, `install`, and `push` aligned on what counts as a valid package or OCI artifact.

### Shared Local Package Validation

The local package validator should operate over archive bytes rather than a materialized working tree. That avoids new filesystem staging paths for the local `.tgz` case and keeps checksum validation tied to the actual package contents being verified.

For the OCI case:

- fetch the content layer archive
- validate config and mirrored manifest equality
- run the same archive-based local package validation against the fetched archive bytes

## Data Model

### Verify Result

The internal verify package should return a small result struct with enough information for the CLI to produce useful output without re-parsing inputs:

- mode: `package` or `oci`
- skill name
- version
- canonical reference for OCI results when available
- whether `.skill/files.sha256` was verified

This result is informational only. Any mandatory-rule violation still returns an error.

## Integrity Verification

### `.skill/files.sha256`

This slice explicitly advertises `.skill/files.sha256` verification support when the file is present in a local package.

Verification rules must follow the packaging specification:

- entries are sorted by path in ascending bytewise order
- each path appears at most once
- `.skill/files.sha256` is never listed
- each recorded path resolves to a regular package file in the archive
- each recorded digest matches the actual file bytes in the archive

The implementation does not need to require coverage of every file in the package, because the spec only says the file `SHOULD` include `.skill/manifest.json`, not that it must enumerate the entire package.

## Testing Strategy

### Internal Verify Tests

Add tests covering:

- local package verification succeeds for a valid packaged example
- local package fails on malformed archive layout
- local package fails on schema-invalid manifest
- local package fails on checksum mismatch
- local package fails on unsorted checksum entries
- local package fails on duplicate checksum entries
- OCI verification succeeds for a valid pushed artifact
- OCI verification fails on config and mirrored manifest mismatch
- OCI verification fails on wrong OCI media types

### CLI Tests

Add tests covering:

- `cumasach verify <package.tgz>` succeeds
- `cumasach verify <artifact-ref>` succeeds
- missing input fails
- root help still includes `verify`
- the verify stub is removed and replaced by real command behavior

## Milestone Boundary

This slice is done when:

- the CLI exposes a working `verify <package.tgz|artifact-ref>`
- local package verification enforces package layout and manifest schema rules
- `.skill/files.sha256` is verified when present
- OCI verification enforces media types and config/mirrored-manifest equality
- tests cover both local and OCI success and the main failure classes

This slice is not done when:

- installed directories can be verified directly
- signatures or attestations are verified
- policy evaluation is implemented
