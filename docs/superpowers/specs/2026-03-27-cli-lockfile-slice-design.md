## CLI Lockfile Slice Design

### Summary

This document defines the next implementation slice for the Cumasach Go reference CLI after the dependency-aware `install` milestone. The slice adds `cumasach lock` and `cumasach install --lockfile` using the existing required-only dependency resolver and install pipeline.

The slice is intentionally narrow:

- `lock` resolves the root skill plus all transitive required dependencies
- lockfiles pin the selected graph to canonical digest-pinned artifact references
- `install --lockfile` installs exactly the pinned graph without live re-resolution

This slice does not implement rollback. Rollback remains part of the normative v1 CLI and packaging model and is deferred to a later implementation slice.

### Goals

- Add `cumasach lock`
- Add `cumasach install --lockfile`
- Reuse the current resolver graph as the source for lockfile generation
- Reuse the current install pipeline for lockfile-driven installs
- Preserve the current live `install` behavior
- Keep lockfile semantics aligned with the published v1 packaging and CLI specifications

### Deferred v1 Work

The following normative v1 features remain outside this implementation slice:

- `rollback`
- lockfile refresh or update semantics beyond generating a new lockfile
- policy-driven verification or provenance enforcement beyond current package validation

This slice must not change the published v1 CLI or packaging contracts. It only implements the lockfile generation and lockfile-driven install paths that are already part of those contracts.

### Non-Goals

- partial graph locks
- non-required dependency classes
- multi-registry dependency source overrides
- emitting lockfiles as a side effect of `install`

## User-Facing Behavior

### `cumasach lock`

Supported forms:

```bash
cumasach lock <artifact-ref> [--output <file>]
cumasach lock <package-name> --from <oci-base> [--output <file>]
```

Behavior:

- resolve the root skill and all transitive required dependencies
- write a deterministic `skill.lock.json`
- pin every selected node to a canonical `oci://...@sha256:...` artifact reference
- write the resolved graph shape, not unresolved constraints
- accept both canonical `oci://...` and plain OCI forms for `<artifact-ref>` and `--from <oci-base>` inputs
- treat `--from` as an OCI repository namespace prefix and derive the root repository by appending `/<package-name>` to that prefix

Defaults:

- if `--output` is omitted, write `./skill.lock.json`

Failure cases:

- package-name input without `--from`
- malformed root artifact references
- dependency resolution failure
- invalid dependency constraints
- inability to normalize any selected node to a canonical artifact reference

### `cumasach install --lockfile`

Supported forms:

```bash
cumasach install --lockfile <file> --target <skills-dir>
cumasach install <artifact-ref|package-name> --lockfile <file> --target <skills-dir> [--from <oci-base>]
```

Behavior:

- load and validate the lockfile
- fetch exactly the pinned artifacts referenced by the lockfile
- validate fetched artifacts against the package rules
- activate the full locked graph into the flat target directory
- record install-state based on the resulting full active target view

Lockfile installs MUST NOT:

- list tags for version selection
- re-solve dependency constraints
- derive a new dependency graph from dependency manifests

### Mixed Root and Lockfile Form

If a root input is provided together with `--lockfile`, the supplied root MUST match the lockfile root before activation begins.

Matching rules:

- package-name input MUST match the lockfile root `name`
- artifact-reference input MUST match the lockfile root canonical `reference` after normalization

If `--from` is supplied in lockfile mode:

- package-name mixed form MUST still require it, because the published CLI contract requires `--from` whenever the positional root input is a package name
- it MUST be accepted for command-shape consistency
- it MUST NOT influence artifact fetching or graph selection

Root mismatch MUST fail the command before any target mutation begins.

## Lockfile Model

The lockfile is a serialization of a resolved graph, not a second dependency language.

Each lockfile MUST carry:

- `schemaVersion`
- one root package identity
- one selected package entry per skill name
- canonical digest-pinned artifact reference for each package
- version and digest metadata for each package
- resolved dependency edges as `from -> to`

`install --lockfile` MUST trust the lockfile graph shape and pinned artifact references, while still validating each fetched artifact for schema validity, extraction safety, and canonical/mirrored manifest equality.

If a fetched artifact's manifest `name` or `version` does not match the corresponding lockfile node metadata, installation MUST fail.

## Architecture

### New Internal Package

Add `implementation/go/internal/lockfile`.

This package owns:

- lockfile serialization from a resolved graph
- lockfile parsing
- semantic validation beyond JSON Schema
- conversion from lockfile data into the selected-package shape expected by the installer

### Package Responsibilities

#### `internal/resolve`

- remains the source of truth for live resolution
- produces the resolved graph used by `lock`
- does not gain lockfile-specific behavior

#### `internal/lockfile`

- serialize `resolve.Graph` into v1 `skill.lock.json`
- load lockfiles from disk
- validate semantic invariants:
  - root exists in package set
  - exactly one package per skill name
  - all edges reference known package names
  - all references are canonical digest-pinned OCI artifact references
- convert a validated lockfile into the selected-package graph shape used by install

#### `internal/install`

- continue to own activation and install-state persistence
- accept a lockfile-derived selected graph in addition to a resolver-derived selected graph
- preserve existing state-safe recovery guarantees

#### `cmd/cumasach`

- add the `lock` command
- route `install --lockfile` through lockfile loading and validation
- enforce root/lockfile identity matching before activation

## Data Model

### Shared Selected Graph

Both live installs and lockfile-driven installs should converge on the same internal selected-graph shape before activation.

That means:

- live path: root input -> resolver -> selected graph -> install
- lockfile path: lockfile -> validator -> selected graph -> install

This avoids duplicating activation logic and keeps install-state behavior identical across both paths.

### Lockfile Semantic Rules

Semantic validation MUST check:

- the `root` package exists in the `packages` set
- `root.name`, `root.version`, and `root.reference` agree with a selected package entry
- package names are unique
- every package reference is canonical and digest-pinned
- every package `digest` exactly matches the OCI manifest digest encoded in that package's `reference`
- every edge `from` and `to` refers to a known selected package
- the locked graph contains no dependency cycle

The schema already validates shape. The new lockfile package must validate these graph invariants explicitly.

## Install Semantics

Lockfile-driven installs MUST:

- activate exactly one version per locked skill name into the flat target directory
- replace any previously active directory for a selected skill name
- preserve unrelated active skills in the target directory
- record the full resulting active target view in install-state `active`
- append install-state `history` using the same rules as live install
- preserve the packaging-spec install-state invariants:
  - `history` remains ordered oldest to newest
  - new entries append at the end only
  - the newest `history` snapshot matches top-level `active`
  - each persisted snapshot includes enough artifact reference metadata to re-fetch selected artifacts after cache eviction

If target mutation succeeds but install-state persistence fails, the implementation MUST restore the previously active target view before returning failure. Lockfile mode must preserve the same target/state synchronization guarantees as live install.

## Failure Cases

`lock` MUST fail for:

- package-name input without `--from`
- malformed or invalid artifact references
- resolution failure of any required dependency
- invalid dependency constraints
- dependency cycles
- inability to produce canonical digest-pinned references for selected packages

`install --lockfile` MUST fail for:

- missing or malformed lockfile
- schema-invalid lockfile
- semantically invalid lockfile graph
- cyclic lockfile graph
- root/lockfile identity mismatch in mixed form
- missing pinned artifacts
- fetched artifact package invalidity
- fetched artifact name/version mismatch against lockfile node metadata
- fetched artifact manifest digest mismatch against lockfile package `digest` or `reference`
- install-state persistence failure

## Test Plan

### Unit Tests

- lockfile serialization from a resolved graph
- stable ordering of packages and edges in emitted lockfiles
- semantic validation for:
  - duplicate names
  - missing root package
  - invalid canonical references
  - edges to unknown packages
- root identity matching for mixed install forms
- lockfile installs ignoring live-resolution-only inputs other than root identity checks

### Command Tests

- `cumasach lock <package-name> --from ...`
- `cumasach lock <artifact-ref>`
- `cumasach install --lockfile ... --target ...`
- `cumasach install <package-name> --lockfile ... --target ...`
- `cumasach install <artifact-ref> --lockfile ... --target ...`
- root mismatch failures for both name and artifact-reference forms

### End-to-End Registry Tests

Use the existing in-memory OCI registry harness to cover:

- publish root and required dependencies
- generate a lockfile from the root
- install from that lockfile into a fresh target
- confirm flat active directories and install-state contents
- confirm live install and lockfile install converge on the same active view for the same graph

## Milestone Boundary

This slice is done when:

- `cumasach lock` emits schema-valid, semantically valid lockfiles
- `cumasach install --lockfile` installs the pinned graph without live re-resolution
- mixed root-plus-lockfile installs fail safely on identity mismatch
- existing live install behavior remains intact

This slice is not done when:

- rollback is implemented
- lockfiles can be refreshed in place
- provenance or policy behavior extends beyond current artifact/package validation
